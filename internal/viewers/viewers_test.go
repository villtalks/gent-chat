package viewers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"ghost-chat/internal/chat"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()

	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestParseTwitch(t *testing.T) {
	count, live, err := parseTwitch(fixture(t, "twitch_live.json"))
	if err != nil || !live || count != 1200 {
		t.Errorf("live = %d, %v, %v; want 1200, true, nil", count, live, err)
	}

	for _, name := range []string{"twitch_offline.json", "twitch_unknown.json"} {
		count, live, err := parseTwitch(fixture(t, name))
		if err != nil || live || count != 0 {
			t.Errorf("%s = %d, %v, %v; want 0, false, nil", name, count, live, err)
		}
	}

	for _, body := range []string{"", "<html>nope</html>", `{"errors":[{"message":"service error"}]}`, `{}`} {
		if _, _, err := parseTwitch([]byte(body)); err == nil {
			t.Errorf("parseTwitch(%q): want error", body)
		}
	}
}

func TestParseYouTube(t *testing.T) {
	count, live, err := parseYouTube(fixture(t, "youtube_live.json"))
	if err != nil || !live || count != 560 {
		t.Errorf("live = %d, %v, %v; want 560, true, nil", count, live, err)
	}

	count, live, err = parseYouTube(fixture(t, "youtube_ended.json"))
	if err != nil || live || count != 0 {
		t.Errorf("ended = %d, %v, %v; want 0, false, nil (total views are not viewers)", count, live, err)
	}

	for _, body := range []string{"", "<html>sorry</html>", `{"actions":[]}`, `{}`} {
		if _, _, err := parseYouTube([]byte(body)); err == nil {
			t.Errorf("parseYouTube(%q): want error", body)
		}
	}
}

func TestParseKick(t *testing.T) {
	count, live, err := parseKick(fixture(t, "kick_live.json"))
	if err != nil || !live || count != 3400 {
		t.Errorf("live = %d, %v, %v; want 3400, true, nil", count, live, err)
	}

	count, live, err = parseKick(fixture(t, "kick_offline.json"))
	if err != nil || live || count != 0 {
		t.Errorf("offline = %d, %v, %v; want 0, false, nil", count, live, err)
	}

	for _, body := range []string{"", "<html>Just a moment...</html>", `{}`, `[]`} {
		if _, _, err := parseKick([]byte(body)); err == nil {
			t.Errorf("parseKick(%q): want error", body)
		}
	}
}

func TestNormalizeTwitchLogin(t *testing.T) {
	cases := map[string]string{
		"Streamer":                         "streamer",
		"#streamer":                        "streamer",
		" https://www.twitch.tv/Streamer ": "streamer",
		"twitch.tv/streamer/":              "streamer",
	}

	for input, want := range cases {
		if got := normalizeTwitchLogin(input); got != want {
			t.Errorf("normalizeTwitchLogin(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSourcesOverHTTP(t *testing.T) {
	var mu sync.Mutex

	status := http.StatusOK
	body := ""
	gotBodies := []string{}
	gotHeaders := []http.Header{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		b, _ := io.ReadAll(r.Body)
		gotBodies = append(gotBodies, r.URL.Path+" "+string(b))
		gotHeaders = append(gotHeaders, r.Header.Clone())

		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	set := func(s int, b string) {
		mu.Lock()
		defer mu.Unlock()

		status, body = s, b
	}

	ctx := context.Background()
	hc := srv.Client()

	set(http.StatusOK, string(fixture(t, "twitch_live.json")))

	if count, live, err := twitchSource(hc, srv.URL+"/gql", "#Streamer")(ctx); err != nil || !live || count != 1200 {
		t.Errorf("twitch = %d, %v, %v", count, live, err)
	}

	set(http.StatusOK, string(fixture(t, "youtube_live.json")))

	if count, live, err := youtubeSource(hc, srv.URL+"/yt", func() string { return "abcdefghijk" })(ctx); err != nil || !live || count != 560 {
		t.Errorf("youtube = %d, %v, %v", count, live, err)
	}

	set(http.StatusOK, string(fixture(t, "kick_live.json")))

	if count, live, err := kickSource(hc, srv.URL+"/kick/", "Streamer")(ctx); err != nil || !live || count != 3400 {
		t.Errorf("kick = %d, %v, %v", count, live, err)
	}

	mu.Lock()

	if !strings.Contains(gotBodies[0], `"login":"streamer"`) || gotHeaders[0].Get("Client-ID") == "" {
		t.Errorf("twitch request = %q headers %v", gotBodies[0], gotHeaders[0])
	}

	if !strings.Contains(gotBodies[1], `"videoId":"abcdefghijk"`) {
		t.Errorf("youtube request = %q", gotBodies[1])
	}

	if !strings.HasPrefix(gotBodies[2], "/kick/streamer") {
		t.Errorf("kick request = %q", gotBodies[2])
	}

	requests := len(gotBodies)

	mu.Unlock()

	if count, live, err := youtubeSource(hc, srv.URL+"/yt", func() string { return "" })(ctx); err != nil || live || count != 0 {
		t.Errorf("youtube without a video id = %d, %v, %v; want 0, false, nil", count, live, err)
	}

	mu.Lock()

	if len(gotBodies) != requests {
		t.Error("youtube source must not send a request without a video id")
	}

	mu.Unlock()

	for _, failure := range []struct {
		status int
		body   string
	}{
		{http.StatusForbidden, "<html>Just a moment...</html>"},
		{http.StatusOK, ""},
		{http.StatusOK, "<html>blocked</html>"},
		{http.StatusTooManyRequests, "{}"},
	} {
		set(failure.status, failure.body)

		sources := map[string]Source{
			"twitch":  twitchSource(hc, srv.URL+"/gql", "streamer"),
			"youtube": youtubeSource(hc, srv.URL+"/yt", func() string { return "abcdefghijk" }),
			"kick":    kickSource(hc, srv.URL+"/kick/", "streamer"),
		}

		for name, src := range sources {
			if _, _, err := src(ctx); err == nil {
				t.Errorf("%s with http %d body %q: want error", name, failure.status, failure.body)
			}
		}
	}
}

type updateLog struct {
	mu      sync.Mutex
	updates []Update
}

func (l *updateLog) add(u Update) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.updates = append(l.updates, u)
}

func (l *updateLog) snapshot() []Update {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]Update(nil), l.updates...)
}

func (l *updateLog) waitFor(t *testing.T, want ...Update) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		got := l.snapshot()

		if len(got) >= len(want) {
			match := true

			for i := range want {
				if got[i] != want[i] {
					match = false
				}
			}

			if match {
				return
			}
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("updates = %+v, want prefix %+v", l.snapshot(), want)
}

type scripted struct {
	mu    sync.Mutex
	count int
	live  bool
	err   error
	calls int
}

func (s *scripted) set(count int, live bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.count, s.live, s.err = count, live, err
}

func (s *scripted) source(context.Context) (int, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls++

	return s.count, s.live, s.err
}

func (s *scripted) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls
}

func newTestManager(t *testing.T) (*Manager, *updateLog) {
	t.Helper()

	log := &updateLog{}
	m := NewManager(log.add)

	m.staleAfter = 60 * time.Millisecond

	t.Cleanup(m.StopAll)

	return m, log
}

const tick = 5 * time.Millisecond

func TestManagerEmitsImmediatelyAndOnChange(t *testing.T) {
	m, log := newTestManager(t)
	src := &scripted{count: 10, live: true}

	m.Start(chat.PlatformTwitch, src.source, tick)
	log.waitFor(t, Update{chat.PlatformTwitch, 10, true})

	src.set(12, true, nil)
	log.waitFor(t, Update{chat.PlatformTwitch, 10, true}, Update{chat.PlatformTwitch, 12, true})

	time.Sleep(5 * tick)

	if got := log.snapshot(); len(got) != 2 {
		t.Errorf("updates = %+v, want unchanged counts not re-emitted", got)
	}
}

func TestManagerHoldsLastValueThenExpiresOnce(t *testing.T) {
	m, log := newTestManager(t)
	src := &scripted{count: 10, live: true}

	m.Start(chat.PlatformKick, src.source, tick)
	log.waitFor(t, Update{chat.PlatformKick, 10, true})

	src.set(0, false, errors.New("http 403"))

	time.Sleep(25 * time.Millisecond)

	if got := log.snapshot(); len(got) != 1 {
		t.Fatalf("updates = %+v, want the last value held while the source fails", got)
	}

	if latest := m.Latest(); len(latest) != 1 || latest[0].Count != 10 || !latest[0].Live {
		t.Errorf("Latest = %+v, want held value", latest)
	}

	log.waitFor(t, Update{chat.PlatformKick, 10, true}, Update{chat.PlatformKick, 0, false})

	time.Sleep(10 * tick)

	if got := log.snapshot(); len(got) != 2 {
		t.Fatalf("updates = %+v, want a single expiry update", got)
	}

	src.set(15, true, nil)
	log.waitFor(t, Update{chat.PlatformKick, 10, true}, Update{chat.PlatformKick, 0, false}, Update{chat.PlatformKick, 15, true})
}

func TestManagerNeverEmitsForASourceThatNeverWorked(t *testing.T) {
	m, log := newTestManager(t)
	src := &scripted{err: errors.New("blocked")}

	m.Start(chat.PlatformKick, src.source, tick)

	time.Sleep(100 * time.Millisecond)

	if got := log.snapshot(); len(got) != 0 {
		t.Errorf("updates = %+v, want none", got)
	}
}

func TestManagerOfflineSourceReportsNotLive(t *testing.T) {
	m, log := newTestManager(t)
	src := &scripted{count: 0, live: false}

	m.Start(chat.PlatformTwitch, src.source, tick)
	log.waitFor(t, Update{chat.PlatformTwitch, 0, false})
}

func TestManagerStop(t *testing.T) {
	m, log := newTestManager(t)
	src := &scripted{count: 10, live: true}

	m.Start(chat.PlatformTwitch, src.source, tick)
	log.waitFor(t, Update{chat.PlatformTwitch, 10, true})

	m.Stop(chat.PlatformTwitch)
	log.waitFor(t, Update{chat.PlatformTwitch, 10, true}, Update{chat.PlatformTwitch, 0, false})

	calls := src.callCount()
	src.set(99, true, nil)

	time.Sleep(10 * tick)

	if src.callCount() != calls {
		t.Error("source polled after Stop")
	}

	if got := log.snapshot(); len(got) != 2 {
		t.Errorf("updates = %+v, want nothing after Stop", got)
	}

	if latest := m.Latest(); len(latest) != 0 {
		t.Errorf("Latest = %+v, want empty after Stop", latest)
	}

	m.Stop(chat.PlatformTwitch)

	if got := log.snapshot(); len(got) != 2 {
		t.Errorf("updates = %+v, second Stop must not emit", got)
	}
}

func TestManagerStartTwiceKeepsOnePoller(t *testing.T) {
	m, log := newTestManager(t)
	first := &scripted{count: 10, live: true}
	second := &scripted{count: 20, live: true}

	m.Start(chat.PlatformTwitch, first.source, tick)
	log.waitFor(t, Update{chat.PlatformTwitch, 10, true})

	m.Start(chat.PlatformTwitch, second.source, tick)
	log.waitFor(t, Update{chat.PlatformTwitch, 10, true}, Update{chat.PlatformTwitch, 20, true})

	calls := first.callCount()

	time.Sleep(10 * tick)

	if first.callCount() != calls {
		t.Error("first poller still running after a second Start")
	}
}

func TestManagerPush(t *testing.T) {
	m, log := newTestManager(t)
	src := &scripted{err: errors.New("backup endpoint down")}

	m.Push(chat.PlatformTikTok, 5)

	if got := log.snapshot(); len(got) != 0 {
		t.Fatalf("updates = %+v, want Push ignored for a platform that is not started", got)
	}

	m.Start(chat.PlatformTikTok, src.source, tick)
	m.Push(chat.PlatformTikTok, 700)
	log.waitFor(t, Update{chat.PlatformTikTok, 700, true})

	for i := 0; i < 8; i++ {
		time.Sleep(15 * time.Millisecond)
		m.Push(chat.PlatformTikTok, 700+i)
	}

	for _, u := range log.snapshot() {
		if !u.Live {
			t.Fatalf("updates = %+v, pushes must keep the value fresh", log.snapshot())
		}
	}
}

func TestManagerLatestIsSortedAndStopAllClears(t *testing.T) {
	m, log := newTestManager(t)

	m.Start(chat.PlatformTwitch, (&scripted{count: 1, live: true}).source, tick)
	m.Start(chat.PlatformKick, (&scripted{count: 2, live: true}).source, tick)

	deadline := time.Now().Add(2 * time.Second)
	for len(m.Latest()) < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}

	latest := m.Latest()
	if len(latest) != 2 || latest[0].Platform != chat.PlatformKick || latest[1].Platform != chat.PlatformTwitch {
		t.Fatalf("Latest = %+v, want kick then twitch", latest)
	}

	m.StopAll()

	if len(m.Latest()) != 0 {
		t.Errorf("Latest = %+v, want empty after StopAll", m.Latest())
	}

	count := len(log.snapshot())

	time.Sleep(10 * tick)

	if len(log.snapshot()) != count {
		t.Error("updates emitted after StopAll")
	}
}
