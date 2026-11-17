package tiktok

import (
	"os"
	"testing"
	"time"
)

// Network check against real TikTok. Run with GENT_LIVE=1 and
// GENT_TIKTOK=<handle of a busy live room>.
func TestLiveChat(t *testing.T) {
	if os.Getenv("GENT_LIVE") == "" {
		t.Skip("set GENT_LIVE=1 to run network checks")
	}

	handle := os.Getenv("GENT_TIKTOK")
	if handle == "" {
		t.Skip("set GENT_TIKTOK to the handle of a busy live room")
	}

	rec := &recorder{}
	c := NewClient(rec.onMessage, rec.onEvent, rec.onViewers)

	if err := c.Connect(handle); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	defer c.Disconnect()

	deadline := time.Now().Add(90 * time.Second)

	for time.Now().Before(deadline) {
		_, msgs, viewers := rec.snapshot()
		if len(msgs) >= 3 && len(viewers) >= 2 {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	events, msgs, viewers := rec.snapshot()

	t.Logf("events=%v messages=%d viewerUpdates=%d", events, len(msgs), len(viewers))

	for i, m := range msgs {
		if i < 3 {
			t.Logf("chat %q: %q", m.Username, m.Text)
		}
	}

	if len(viewers) > 0 {
		t.Logf("latest viewers=%d", viewers[len(viewers)-1])
	}

	if len(events) == 0 || events[0] != "chat:connected@tiktok" {
		t.Fatalf("events = %v, want connected (is %s live?)", events, handle)
	}

	if len(msgs) == 0 {
		t.Error("no chat messages received")
	}

	if len(viewers) < 2 {
		t.Error("no viewer updates from the socket")
	}
}
