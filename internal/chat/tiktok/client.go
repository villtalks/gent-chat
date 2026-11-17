package tiktok

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"ghost-chat/internal/chat"
	"ghost-chat/internal/logging"

	"github.com/gorilla/websocket"
)

const (
	defaultHomeURL = "https://www.tiktok.com/"
	defaultWSURL   = "wss://webcast-ws.tiktok.com/webcast/im/ws_proxy/ws_reuse_supplement/"

	cookieAttempts  = 8
	readTimeout     = 45 * time.Second
	dedupeWindow    = 512
	lookupTimeout   = 10 * time.Second
	blockedHeader   = "Handshake-Msg"
	blockedValue    = "DEVICE_BLOCKED"
	heartbeatMillis = 10000
)

var errDeviceBlocked = errors.New("tiktok rejected the device cookie")

type MessageHandler func(chat.ChatMessage)
type EventHandler func(event string, data any)
type ViewersHandler func(count int)

// Client reads the chat of one TikTok LIVE room straight from TikTok's own
// webcast socket. It follows the handle across streams: while the handle is
// offline it waits, and it reconnects silently when the socket drops mid-stream.
type Client struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	handle string

	hc *http.Client

	roomAPI          string
	homeURL          string
	wsURL            string
	heartbeatEvery   time.Duration
	waitEvery        time.Duration
	minBackoff       time.Duration
	maxBackoff       time.Duration
	cookieRetryDelay time.Duration

	OnMessage MessageHandler
	OnEvent   EventHandler
	OnViewers ViewersHandler
}

func NewClient(onMessage MessageHandler, onEvent EventHandler, onViewers ViewersHandler) *Client {
	return &Client{
		hc: &http.Client{
			Timeout: lookupTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		roomAPI:          defaultRoomAPI,
		homeURL:          defaultHomeURL,
		wsURL:            defaultWSURL,
		heartbeatEvery:   heartbeatMillis * time.Millisecond,
		waitEvery:        30 * time.Second,
		minBackoff:       2 * time.Second,
		maxBackoff:       30 * time.Second,
		cookieRetryDelay: 750 * time.Millisecond,
		OnMessage:        onMessage,
		OnEvent:          onEvent,
		OnViewers:        onViewers,
	}
}

// Handle returns the handle being followed, or "" when disconnected.
func (c *Client) Handle() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.handle
}

func (c *Client) Connect(input string) error {
	handle, err := ResolveHandle(input)
	if err != nil {
		return fmt.Errorf("failed to resolve tiktok name: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
		c.handle = ""
	}

	lookupCtx, cancelLookup := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancelLookup()

	room, err := fetchRoom(lookupCtx, c.hc, c.roomAPI, handle)
	if err != nil {
		return fmt.Errorf("failed to look up tiktok live: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.cancel = cancel
	c.handle = handle

	if room.Live {
		c.emit("chat:connected")
	} else {
		c.emit("chat:waiting")
	}

	go c.run(ctx, handle, room)

	return nil
}

func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel == nil {
		return
	}

	c.cancel()
	c.cancel = nil
	c.handle = ""

	c.emit("chat:disconnected")
}

func (c *Client) emit(event string) {
	if c.OnEvent != nil {
		c.OnEvent(event, map[string]string{"platform": string(chat.PlatformTikTok)})
	}
}

// emitIfActive emits only while this run is still the current one, so a run
// that was cancelled by Disconnect or a new Connect can never report state.
func (c *Client) emitIfActive(ctx context.Context, event string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ctx.Err() != nil {
		return false
	}

	c.emit(event)

	return true
}

func (c *Client) reportViewers(ctx context.Context, count int) {
	if ctx.Err() == nil && c.OnViewers != nil {
		c.OnViewers(count)
	}
}

func (c *Client) run(ctx context.Context, handle string, room RoomInfo) {
	log := logging.Platform("tiktok")

	for {
		if !room.Live {
			var ok bool

			room, ok = c.waitForLive(ctx, handle)
			if !ok {
				return
			}

			if !c.emitIfActive(ctx, "chat:connected") {
				return
			}
		}

		c.reportViewers(ctx, room.Viewers)
		c.followRoom(ctx, handle, room)

		if ctx.Err() != nil {
			return
		}

		log.Info("stream ended, waiting for next stream")

		room = RoomInfo{}

		if !c.emitIfActive(ctx, "chat:waiting") {
			return
		}
	}
}

func (c *Client) waitForLive(ctx context.Context, handle string) (RoomInfo, bool) {
	for {
		if !sleep(ctx, c.waitEvery) {
			return RoomInfo{}, false
		}

		room, err := fetchRoom(ctx, c.hc, c.roomAPI, handle)
		if err != nil {
			if ctx.Err() != nil {
				return RoomInfo{}, false
			}

			logging.Platform("tiktok").Warn("live check failed", "error", err)

			continue
		}

		if room.Live {
			return room, true
		}
	}
}

// followRoom keeps a socket open to the room, redialling after drops, and
// returns once the room is no longer live or ctx is cancelled.
func (c *Client) followRoom(ctx context.Context, handle string, room RoomInfo) {
	log := logging.Platform("tiktok")
	seen := newDedupe(dedupeWindow)
	backoff := c.minBackoff
	cookie := ""
	agent := userAgent

	for ctx.Err() == nil {
		if cookie == "" {
			var err error

			cookie, err = c.fetchCookie(ctx, agent)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				log.Warn("failed to get tiktok cookie", "error", err)
			}
		}

		var err error

		if cookie != "" {
			err = c.session(ctx, room.RoomID, cookie, agent, seen, func() { backoff = c.minBackoff })
		}

		if ctx.Err() != nil {
			return
		}

		if errors.Is(err, errDeviceBlocked) {
			log.Warn("device blocked, refreshing cookie")

			cookie = ""

			if agent == userAgent {
				agent = userAgentAlt
			} else {
				agent = userAgent
			}
		} else if err != nil {
			log.Warn("tiktok socket closed", "error", err)
		}

		if !sleep(ctx, backoff) {
			return
		}

		backoff = min(backoff*2, c.maxBackoff)

		current, err := fetchRoom(ctx, c.hc, c.roomAPI, handle)
		if err != nil {
			continue
		}

		if !current.Live {
			return
		}

		room = current
	}
}

func (c *Client) fetchCookie(ctx context.Context, agent string) (string, error) {
	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.homeURL, nil)
		if err != nil {
			return "", err
		}

		req.Header.Set("User-Agent", agent)

		resp, err := c.hc.Do(req)
		if err == nil {
			_ = resp.Body.Close()

			for _, cookie := range resp.Cookies() {
				if cookie.Name == "ttwid" && cookie.Value != "" {
					return strings.TrimSpace(cookie.Value), nil
				}
			}
		}

		if attempt >= cookieAttempts {
			if err != nil {
				return "", fmt.Errorf("failed to fetch tiktok cookie: %w", err)
			}

			return "", fmt.Errorf("no ttwid cookie after %d attempts", attempt)
		}

		if !sleep(ctx, c.cookieRetryDelay) {
			return "", ctx.Err()
		}
	}
}

func (c *Client) socketURL(roomID, agent string) string {
	q := url.Values{
		"version_code":          {"180800"},
		"device_platform":       {"web"},
		"cookie_enabled":        {"true"},
		"screen_width":          {"1920"},
		"screen_height":         {"1080"},
		"browser_language":      {"en-US"},
		"browser_platform":      {"Win32"},
		"browser_name":          {"Mozilla"},
		"browser_version":       {strings.TrimPrefix(agent, "Mozilla/")},
		"browser_online":        {"true"},
		"tz_name":               {"UTC"},
		"app_name":              {"tiktok_web"},
		"sup_ws_ds_opt":         {"1"},
		"update_version_code":   {"2.0.0"},
		"compress":              {"gzip"},
		"webcast_language":      {"en"},
		"ws_direct":             {"1"},
		"aid":                   {"1988"},
		"live_id":               {"12"},
		"app_language":          {"en"},
		"client_enter":          {"1"},
		"room_id":               {roomID},
		"identity":              {"audience"},
		"history_comment_count": {"6"},
		"last_rtt":              {fmt.Sprintf("%.3f", 100+rand.Float64()*100)},
		"heartbeat_duration":    {strconv.Itoa(heartbeatMillis)},
		"resp_content_type":     {"protobuf"},
		"did_rule":              {"3"},
	}

	return c.wsURL + "?" + q.Encode()
}

// session runs one socket connection until it fails or ctx is cancelled.
// onHealthy is called once, when the first event batch has been decoded: a
// socket that opens and is then dropped at once must not count as a success.
func (c *Client) session(ctx context.Context, roomID, cookie, agent string, seen *dedupe, onHealthy func()) error {
	id, err := strconv.ParseUint(roomID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid room id %q: %w", roomID, err)
	}

	header := http.Header{}

	header.Set("User-Agent", agent)
	header.Set("Cookie", "ttwid="+cookie)
	header.Set("Origin", "https://www.tiktok.com")
	header.Set("Referer", "https://www.tiktok.com/")

	dialer := websocket.Dialer{HandshakeTimeout: lookupTimeout}

	conn, resp, err := dialer.DialContext(ctx, c.socketURL(roomID, agent), header)
	if err != nil {
		if resp != nil && resp.Header.Get(blockedHeader) == blockedValue {
			return errDeviceBlocked
		}

		return fmt.Errorf("failed to open tiktok socket: %w", err)
	}

	var writeMu sync.Mutex

	write := func(frame []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()

		_ = conn.SetWriteDeadline(time.Now().Add(lookupTimeout))

		return conn.WriteMessage(websocket.BinaryMessage, frame)
	}

	sessionCtx, stop := context.WithCancel(ctx)

	var wg sync.WaitGroup

	defer func() {
		stop()
		_ = conn.Close()
		wg.Wait()
	}()

	heartbeat := heartbeatFrame(id)

	if err := write(heartbeat); err != nil {
		return err
	}

	if err := write(enterRoomFrame(id)); err != nil {
		return err
	}

	wg.Add(2)

	go func() {
		defer wg.Done()

		<-sessionCtx.Done()

		_ = conn.Close()
	}()

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(c.heartbeatEvery)
		defer ticker.Stop()

		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-ticker.C:
				if write(heartbeat) != nil {
					return
				}
			}
		}
	}()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))

		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		if c.handleFrame(ctx, data, seen, write) && onHealthy != nil {
			onHealthy()

			onHealthy = nil
		}
	}
}

// handleFrame processes one socket frame and reports whether it carried a
// decodable event batch.
func (c *Client) handleFrame(ctx context.Context, data []byte, seen *dedupe, write func([]byte) error) bool {
	frame, err := decodePushFrame(data)
	if err != nil || frame.PayloadType != "msg" {
		return false
	}

	resp, err := decodeResponse(frame.Payload)
	if err != nil {
		return false
	}

	if resp.NeedsAck && len(resp.InternalExt) > 0 {
		_ = write(ackFrame(frame.LogID, resp.InternalExt))
	}

	for _, m := range resp.Messages {
		switch m.Method {
		case methodChat:
			if m.MsgID != 0 && !seen.add(m.MsgID) {
				continue
			}

			msg, err := parseChat(m.Payload, m.MsgID, time.Now())
			if err != nil {
				continue
			}

			if ctx.Err() == nil && c.OnMessage != nil {
				c.OnMessage(msg)
			}
		case methodRoomUserSeq:
			count, err := parseViewerCount(m.Payload)
			if err != nil {
				continue
			}

			c.reportViewers(ctx, count)
		}
	}

	return true
}

// dedupe remembers the most recent message ids. TikTok replays the last few
// comments on every (re)connect, and those must not show twice.
type dedupe struct {
	ids   map[uint64]struct{}
	order []uint64
	limit int
}

func newDedupe(limit int) *dedupe {
	return &dedupe{ids: make(map[uint64]struct{}, limit), limit: limit}
}

func (d *dedupe) add(id uint64) bool {
	if _, ok := d.ids[id]; ok {
		return false
	}

	if len(d.order) >= d.limit {
		delete(d.ids, d.order[0])
		d.order = d.order[1:]
	}

	d.ids[id] = struct{}{}
	d.order = append(d.order, id)

	return true
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
