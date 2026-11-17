package tiktok

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ghost-chat/internal/chat"

	"github.com/gorilla/websocket"
)

type fakeTikTok struct {
	t        *testing.T
	mu       sync.Mutex
	roomBody string
	blocked  int
	hangUp   bool
	cookies  int
	dials    int
	sent     [][]byte
	conns    []*websocket.Conn
	received []pushFrame
	srv      *httptest.Server
}

const liveRoomBody = `{"statusCode":0,"data":{"user":{"roomId":"42","status":2},"liveRoom":{"liveRoomStats":{"userCount":7}}}}`
const offlineRoomBody = `{"statusCode":0,"data":{"user":{"roomId":"42","status":4},"liveRoom":{"liveRoomStats":{"userCount":1}}}}`
const notFoundRoomBody = `{"data":null,"message":"user_not_found","statusCode":19881007}`

func newFakeTikTok(t *testing.T, roomBody string) *fakeTikTok {
	t.Helper()

	f := &fakeTikTok{t: t, roomBody: roomBody}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	mux := http.NewServeMux()

	mux.HandleFunc("/room", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		_, _ = w.Write([]byte(f.roomBody))
	})

	mux.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.cookies++
		f.mu.Unlock()

		http.SetCookie(w, &http.Cookie{Name: "ttwid", Value: "cookie-value"})
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.dials++

		if f.blocked > 0 {
			f.blocked--
			f.mu.Unlock()

			w.Header().Set("Handshake-Msg", "DEVICE_BLOCKED")
			w.WriteHeader(http.StatusForbidden)

			return
		}

		f.mu.Unlock()

		if !strings.Contains(r.Header.Get("Cookie"), "ttwid=cookie-value") || r.URL.Query().Get("room_id") != "42" {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		f.mu.Lock()
		f.conns = append(f.conns, conn)
		pending := f.sent
		hangUp := f.hangUp
		f.mu.Unlock()

		if hangUp {
			_, _, _ = conn.ReadMessage()
			_, _, _ = conn.ReadMessage()
			_ = conn.Close()

			return
		}

		for _, frame := range pending {
			_ = conn.WriteMessage(websocket.BinaryMessage, frame)
		}

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}

			frame, err := decodePushFrame(data)
			if err != nil {
				continue
			}

			f.mu.Lock()
			f.received = append(f.received, frame)
			f.mu.Unlock()
		}
	})

	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	return f
}

func (f *fakeTikTok) setRoom(body string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.roomBody = body
}

func (f *fakeTikTok) queue(frames ...[]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sent = append(f.sent, frames...)
}

func (f *fakeTikTok) push(frame []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.conns) == 0 {
		f.t.Fatal("no open connection to push to")
	}

	_ = f.conns[len(f.conns)-1].WriteMessage(websocket.BinaryMessage, frame)
}

func (f *fakeTikTok) dropConnection() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.conns) > 0 {
		_ = f.conns[len(f.conns)-1].Close()
	}
}

func (f *fakeTikTok) receivedTypes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	types := make([]string, 0, len(f.received))

	for _, frame := range f.received {
		types = append(types, frame.PayloadType)
	}

	return types
}

func (f *fakeTikTok) dialCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.dials
}

type recorder struct {
	mu       sync.Mutex
	events   []string
	messages []chat.ChatMessage
	viewers  []int
}

func (r *recorder) onEvent(event string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	platform := ""
	if m, ok := data.(map[string]string); ok {
		platform = m["platform"]
	}

	r.events = append(r.events, event+"@"+platform)
}

func (r *recorder) onMessage(m chat.ChatMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.messages = append(r.messages, m)
}

func (r *recorder) onViewers(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.viewers = append(r.viewers, n)
}

func (r *recorder) snapshot() ([]string, []chat.ChatMessage, []int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.events...), append([]chat.ChatMessage(nil), r.messages...), append([]int(nil), r.viewers...)
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", what)
}

func newTestClient(t *testing.T, f *fakeTikTok) (*Client, *recorder) {
	t.Helper()

	rec := &recorder{}
	c := NewClient(rec.onMessage, rec.onEvent, rec.onViewers)

	c.roomAPI = f.srv.URL + "/room"
	c.homeURL = f.srv.URL + "/home"
	c.wsURL = "ws" + strings.TrimPrefix(f.srv.URL, "http") + "/ws"
	c.heartbeatEvery = 20 * time.Millisecond
	c.waitEvery = 10 * time.Millisecond
	c.minBackoff = 5 * time.Millisecond
	c.maxBackoff = 20 * time.Millisecond
	c.cookieRetryDelay = time.Millisecond

	t.Cleanup(c.Disconnect)

	return c, rec
}

func chatFrame(msgID uint64, nickname, text string, needsAck bool) []byte {
	user := appendBytesField(nil, 3, []byte(nickname))
	payload := appendBytesField(nil, 2, user)
	payload = appendBytesField(payload, 3, []byte(text))

	return msgFrame(77, methodChat, payload, msgID, needsAck)
}

func viewersFrame(count uint64) []byte {
	return msgFrame(78, methodRoomUserSeq, appendVarintField(nil, 3, count), 1, false)
}

func msgFrame(logID uint64, method string, payload []byte, msgID uint64, needsAck bool) []byte {
	msg := appendBytesField(nil, 1, []byte(method))
	msg = appendBytesField(msg, 2, payload)
	msg = appendVarintField(msg, 3, msgID)

	resp := appendBytesField(nil, 1, msg)
	resp = appendBytesField(resp, 5, []byte("ext-data"))

	if needsAck {
		resp = appendVarintField(resp, 9, 1)
	}

	return encodePushFrame(pushFrame{LogID: logID, PayloadEncoding: "pb", PayloadType: "msg", Payload: resp})
}

func TestConnectLive(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	f.queue(chatFrame(1, "Nick", "hello", false), viewersFrame(321))

	c, rec := newTestClient(t, f)

	if err := c.Connect("@Streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if c.Handle() != "streamer" {
		t.Errorf("Handle = %q, want streamer", c.Handle())
	}

	eventually(t, "chat message and viewer count", func() bool {
		_, msgs, viewers := rec.snapshot()

		return len(msgs) == 1 && len(viewers) >= 1
	})

	events, msgs, viewers := rec.snapshot()

	if events[0] != "chat:connected@tiktok" {
		t.Errorf("events = %v, want chat:connected@tiktok first", events)
	}

	if msgs[0].Username != "Nick" || msgs[0].Text != "hello" || msgs[0].Platform != chat.PlatformTikTok {
		t.Errorf("message = %+v", msgs[0])
	}

	if viewers[len(viewers)-1] != 321 {
		t.Errorf("viewers = %v, want last 321", viewers)
	}

	eventually(t, "heartbeat and enter room", func() bool {
		types := f.receivedTypes()

		return len(types) >= 3
	})

	types := f.receivedTypes()
	if types[0] != "hb" || types[1] != "im_enter_room" || types[2] != "hb" {
		t.Errorf("first frames = %v, want hb, im_enter_room, hb...", types)
	}
}

func TestInitialViewerCountComesFromRoomLookup(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	c, rec := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "initial viewer count", func() bool {
		_, _, viewers := rec.snapshot()

		return len(viewers) >= 1 && viewers[0] == 7
	})
}

func TestAckSent(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	f.queue(chatFrame(1, "Nick", "hello", true))

	c, _ := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "ack frame", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()

		for _, frame := range f.received {
			if frame.PayloadType == "ack" && frame.LogID == 77 && string(frame.Payload) == "ext-data" {
				return true
			}
		}

		return false
	})
}

func TestGarbageFrameIgnored(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	f.queue([]byte{0xff, 0xff, 0xff}, []byte("not protobuf at all"), msgFrame(1, methodChat, []byte{0x12, 0xff}, 9, false), chatFrame(2, "Nick", "still here", false))

	c, rec := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "good frame after garbage", func() bool {
		_, msgs, _ := rec.snapshot()

		return len(msgs) == 1 && msgs[0].Text == "still here"
	})

	if f.dialCount() != 1 {
		t.Errorf("dials = %d, want 1 (garbage must not drop the connection)", f.dialCount())
	}
}

func TestDuplicateMessagesDelivereOnce(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	f.queue(chatFrame(5, "Nick", "once", false), chatFrame(5, "Nick", "once", false), chatFrame(6, "Nick", "twice", false))

	c, rec := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "two distinct messages", func() bool {
		_, msgs, _ := rec.snapshot()

		return len(msgs) >= 2
	})

	time.Sleep(30 * time.Millisecond)

	_, msgs, _ := rec.snapshot()
	if len(msgs) != 2 || msgs[0].Text != "once" || msgs[1].Text != "twice" {
		t.Errorf("messages = %+v, want once then twice", msgs)
	}
}

func TestOfflineWaitsThenConnects(t *testing.T) {
	f := newFakeTikTok(t, offlineRoomBody)
	c, rec := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "waiting event", func() bool {
		events, _, _ := rec.snapshot()

		return len(events) == 1 && events[0] == "chat:waiting@tiktok"
	})

	if f.dialCount() != 0 {
		t.Errorf("dials = %d, want 0 while offline", f.dialCount())
	}

	f.setRoom(liveRoomBody)

	eventually(t, "connected after going live", func() bool {
		events, _, _ := rec.snapshot()

		return len(events) == 2 && events[1] == "chat:connected@tiktok"
	})
}

func TestUserNotFound(t *testing.T) {
	f := newFakeTikTok(t, notFoundRoomBody)
	c, rec := newTestClient(t, f)

	err := c.Connect("nobody")
	if err == nil || !strings.Contains(err.Error(), "tiktok user not found") {
		t.Fatalf("err = %v, want tiktok user not found", err)
	}

	if events, _, _ := rec.snapshot(); len(events) != 0 {
		t.Errorf("events = %v, want none", events)
	}

	if c.Handle() != "" {
		t.Errorf("Handle = %q, want empty after a failed connect", c.Handle())
	}
}

func TestInvalidInput(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	c, _ := newTestClient(t, f)

	if err := c.Connect("two words"); err == nil {
		t.Fatal("want error for invalid input")
	}
}

func TestDisconnect(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	c, rec := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "connected", func() bool {
		events, _, _ := rec.snapshot()

		return len(events) == 1
	})

	c.Disconnect()

	events, _, _ := rec.snapshot()
	if len(events) != 2 || events[1] != "chat:disconnected@tiktok" {
		t.Fatalf("events = %v, want connected then disconnected", events)
	}

	if c.Handle() != "" {
		t.Errorf("Handle = %q, want empty", c.Handle())
	}

	dials := f.dialCount()
	time.Sleep(60 * time.Millisecond)

	if f.dialCount() != dials {
		t.Error("client kept dialing after Disconnect")
	}

	c.Disconnect()

	if events, _, _ := rec.snapshot(); len(events) != 2 {
		t.Errorf("events = %v, second Disconnect must not emit", events)
	}
}

func TestReconnectAfterDrop(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	c, rec := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "first dial", func() bool { return f.dialCount() == 1 && len(f.receivedTypes()) > 0 })

	f.dropConnection()

	eventually(t, "second dial", func() bool { return f.dialCount() >= 2 })
	eventually(t, "second connection ready", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()

		return len(f.conns) >= 2
	})

	f.push(chatFrame(10, "Nick", "after drop", false))

	eventually(t, "message after reconnect", func() bool {
		_, msgs, _ := rec.snapshot()

		return len(msgs) == 1 && msgs[0].Text == "after drop"
	})

	events, _, _ := rec.snapshot()
	if len(events) != 1 {
		t.Errorf("events = %v, want a single connected event across a silent reconnect", events)
	}
}

func TestStreamEndReturnsToWaiting(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	c, rec := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "first dial", func() bool { return f.dialCount() == 1 && len(f.receivedTypes()) > 0 })

	f.setRoom(offlineRoomBody)
	f.dropConnection()

	eventually(t, "waiting after stream end", func() bool {
		events, _, _ := rec.snapshot()

		return len(events) == 2 && events[1] == "chat:waiting@tiktok"
	})
}

func TestDeviceBlockedFetchesFreshCookie(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	f.blocked = 1

	c, rec := newTestClient(t, f)

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	eventually(t, "connected after a blocked handshake", func() bool {
		events, _, _ := rec.snapshot()

		return len(events) >= 1 && events[0] == "chat:connected@tiktok" && f.dialCount() >= 2
	})

	f.mu.Lock()
	cookies := f.cookies
	f.mu.Unlock()

	if cookies < 2 {
		t.Errorf("cookie fetches = %d, want a fresh ttwid after DEVICE_BLOCKED", cookies)
	}
}

func TestBackoffGrowsWhenSocketOpensThenDrops(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	f.hangUp = true

	c, _ := newTestClient(t, f)

	c.minBackoff = 10 * time.Millisecond
	c.maxBackoff = 400 * time.Millisecond

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	time.Sleep(600 * time.Millisecond)

	// 10+20+40+80+160+320 ms of growing waits allow about 6 dials in 600 ms;
	// a backoff that resets on every open would allow dozens.
	if dials := f.dialCount(); dials < 2 || dials > 9 {
		t.Errorf("dials = %d in 600 ms, want 2-9 (backoff must grow while sessions die at once)", dials)
	}
}

func TestBackoffResetsAfterAHealthySession(t *testing.T) {
	f := newFakeTikTok(t, liveRoomBody)
	f.queue(viewersFrame(5))

	c, _ := newTestClient(t, f)

	c.minBackoff = 10 * time.Millisecond
	c.maxBackoff = 2 * time.Second

	if err := c.Connect("streamer"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	for want := 1; want <= 5; want++ {
		eventually(t, "dial after a healthy session", func() bool {
			f.mu.Lock()
			defer f.mu.Unlock()

			return f.dials >= want && len(f.conns) >= want
		})

		time.Sleep(15 * time.Millisecond)
		f.dropConnection()
	}
}
