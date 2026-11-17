package youtube

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"ghost-chat/internal/chat"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()

	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func TestParseLiveVideoIDs(t *testing.T) {
	cases := []struct {
		fixture string
		want    []string
	}{
		{"streams_two_live.html", []string{"LANDSCAPE01", "VERTICAL001"}},
		{"streams_one_live.html", []string{"LANDSCAPE01"}},
		{"streams_none.html", nil},
	}

	for _, tc := range cases {
		got, err := parseLiveVideoIDs(readFixture(t, tc.fixture))
		if err != nil {
			t.Fatalf("%s: %v", tc.fixture, err)
		}

		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: ids = %v, want %v", tc.fixture, got, tc.want)
		}
	}
}

func TestParseLiveVideoIDsLegacyRenderer(t *testing.T) {
	html := `<script>var ytInitialData = {"items":[{"videoRenderer":{"videoId":"aaaaaaaaaaa","thumbnailOverlays":[{"thumbnailOverlayTimeStatusRenderer":{"style":"LIVE"}}]}},{"videoRenderer":{"videoId":"bbbbbbbbbbb","thumbnailOverlays":[{"thumbnailOverlayTimeStatusRenderer":{"style":"DEFAULT"}}]}}]};</script>`

	got, err := parseLiveVideoIDs(html)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0] != "aaaaaaaaaaa" {
		t.Errorf("ids = %v, want [aaaaaaaaaaa]", got)
	}
}

func TestParseLiveVideoIDsRejectsPageWithoutData(t *testing.T) {
	if _, err := parseLiveVideoIDs("<html>consent wall</html>"); err == nil {
		t.Error("want error for a page without ytInitialData")
	}
}

func TestPickVertical(t *testing.T) {
	cases := []struct {
		live      []string
		landscape string
		want      string
		ok        bool
	}{
		{[]string{"A", "B"}, "A", "B", true},
		{[]string{"A", "B"}, "B", "A", true},
		{[]string{"A"}, "A", "", false},
		{[]string{"A", "B", "C"}, "A", "B", true},
		{[]string{"A", "B"}, "", "", false},
		{[]string{"B"}, "A", "", false},
		{[]string{"A", "B"}, "X", "", false},
		{[]string{"A", "B", "C"}, "C", "A", true},
		{[]string{"B"}, "", "", false},
		{nil, "A", "", false},
	}

	for _, tc := range cases {
		got, ok := pickVertical(tc.live, tc.landscape)
		if got != tc.want || ok != tc.ok {
			t.Errorf("pickVertical(%v, %q) = %q, %v; want %q, %v", tc.live, tc.landscape, got, ok, tc.want, tc.ok)
		}
	}
}

type verticalFake struct {
	mu          sync.Mutex
	streamsPage string
	liveID      string
}

func (f *verticalFake) set(streamsPage, liveID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.streamsPage = streamsPage
	f.liveID = liveID
}

func (f *verticalFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == http.MethodPost:
		_, _ = io.WriteString(w, `{}`)
	case strings.HasSuffix(r.URL.Path, "/streams"):
		_, _ = io.WriteString(w, f.streamsPage)
	case strings.HasSuffix(r.URL.Path, "/live"):
		if f.liveID == "" {
			_, _ = io.WriteString(w, `<link rel="canonical" href="https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa">`)

			return
		}

		_, _ = io.WriteString(w, `<link rel="canonical" href="https://www.youtube.com/watch?v=`+f.liveID+`">`)
	default:
		_, _ = io.WriteString(w, watchPage(false))
	}
}

func newVerticalClient(t *testing.T, f *verticalFake) (*Client, *eventLog, *http.Client) {
	t.Helper()

	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	target, _ := url.Parse(srv.URL)
	log := &eventLog{}
	hc := &http.Client{Transport: rewriteTransport{target: target}}

	c := NewClient(chat.PlatformYouTubeVertical, func(chat.ChatMessage) {}, log.add)
	c.hc = hc
	c.waitInterval = func() time.Duration { return 5 * time.Millisecond }

	t.Cleanup(c.Disconnect)

	return c, log, hc
}

func TestVerticalNeverReturnsLandscape(t *testing.T) {
	f := &verticalFake{}
	f.set(readFixture(t, "streams_one_live.html"), "LANDSCAPE01")

	_, _, hc := newVerticalClient(t, f)

	_, err := resolveVerticalURL(context.Background(), hc, "https://www.youtube.com/@somechannel/live")
	if !errors.Is(err, ErrChannelOffline) {
		t.Fatalf("err = %v, want ErrChannelOffline", err)
	}
}

func TestVerticalResolvesSecondStream(t *testing.T) {
	f := &verticalFake{}
	f.set(readFixture(t, "streams_two_live.html"), "LANDSCAPE01")

	_, _, hc := newVerticalClient(t, f)

	got, err := resolveVerticalURL(context.Background(), hc, "https://www.youtube.com/@somechannel/live")
	if err != nil {
		t.Fatal(err)
	}

	if got != watchURL("VERTICAL001") {
		t.Errorf("vertical url = %q, want the non-landscape stream", got)
	}
}

func TestVerticalClientWaitsUntilSecondStreamThenConnects(t *testing.T) {
	f := &verticalFake{}
	f.set(readFixture(t, "streams_one_live.html"), "LANDSCAPE01")

	c, log, _ := newVerticalClient(t, f)

	if err := c.Connect("@somechannel"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	log.waitFor(t, "chat:waiting")

	if got := c.VideoID(); got != "" {
		t.Errorf("VideoID while waiting = %q, want empty", got)
	}

	f.set(readFixture(t, "streams_two_live.html"), "LANDSCAPE01")
	log.waitFor(t, "chat:waiting", "chat:connected")

	deadline := time.Now().Add(2 * time.Second)
	for c.VideoID() == "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if got := c.VideoID(); got != "VERTICAL001" {
		t.Errorf("VideoID = %q, want VERTICAL001", got)
	}
}

func TestVerticalClientAcceptsVideoLink(t *testing.T) {
	f := &verticalFake{}
	c, log, _ := newVerticalClient(t, f)

	if err := c.Connect("https://www.youtube.com/watch?v=abcdefghijk"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	log.waitFor(t, "chat:connected")
}

type platformLog struct {
	mu        sync.Mutex
	platforms []string
}

func (l *platformLog) add(_ string, data any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if m, ok := data.(map[string]string); ok {
		l.platforms = append(l.platforms, m["platform"])
	}
}

func TestClientEmitsOwnPlatform(t *testing.T) {
	log := &platformLog{}

	var got []chat.ChatMessage

	c := NewClient(chat.PlatformYouTubeVertical, func(m chat.ChatMessage) { got = append(got, m) }, log.add)

	c.emit("chat:connected")
	c.deliver([]chat.ChatMessage{{ID: "1", Platform: chat.PlatformYouTube}})

	if len(log.platforms) != 1 || log.platforms[0] != "youtube_vertical" {
		t.Errorf("event platforms = %v, want [youtube_vertical]", log.platforms)
	}

	if len(got) != 1 || got[0].Platform != chat.PlatformYouTubeVertical {
		t.Errorf("messages = %+v, want platform youtube_vertical", got)
	}
}

func TestVideoIDClearedOnDisconnect(t *testing.T) {
	f := &fakeYouTube{}
	c, log := newTestClient(t, f)

	if err := c.Connect("https://www.youtube.com/watch?v=abcdefghijk"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	log.waitFor(t, "chat:connected")

	deadline := time.Now().Add(2 * time.Second)
	for c.VideoID() == "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if got := c.VideoID(); got != "abcdefghijk" {
		t.Fatalf("VideoID = %q, want abcdefghijk", got)
	}

	c.Disconnect()

	if got := c.VideoID(); got != "" {
		t.Errorf("VideoID after disconnect = %q, want empty", got)
	}
}
