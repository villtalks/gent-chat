package viewers

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// Network checks against the real platforms. Run with GENT_LIVE=1 and name a
// live channel per platform with GENT_TWITCH, GENT_KICK, GENT_YOUTUBE_VIDEO and
// GENT_TIKTOK. A platform without a channel is skipped.
func TestLiveSources(t *testing.T) {
	if os.Getenv("GENT_LIVE") == "" {
		t.Skip("set GENT_LIVE=1 to run network checks")
	}

	hc := &http.Client{Timeout: 10 * time.Second}

	sources := map[string]Source{}

	if v := os.Getenv("GENT_TWITCH"); v != "" {
		sources["twitch"] = TwitchSource(hc, v)
	}

	if v := os.Getenv("GENT_KICK"); v != "" {
		sources["kick"] = KickSource(hc, v)
	}

	if v := os.Getenv("GENT_YOUTUBE_VIDEO"); v != "" {
		sources["youtube"] = YouTubeSource(hc, func() string { return v })
	}

	if v := os.Getenv("GENT_TIKTOK"); v != "" {
		sources["tiktok"] = TikTokSource(hc, func() string { return v })
	}

	if len(sources) == 0 {
		t.Skip("set GENT_TWITCH, GENT_KICK, GENT_YOUTUBE_VIDEO or GENT_TIKTOK to a live channel")
	}

	for name, src := range sources {
		count, live, err := src(context.Background())

		t.Logf("%s: count=%d live=%v err=%v", name, count, live, err)

		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
