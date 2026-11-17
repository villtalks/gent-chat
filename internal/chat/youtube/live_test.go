package youtube

import (
	"context"
	"os"
	"testing"
)

// Network checks against real YouTube. Run with GENT_LIVE=1 and
// GENT_YOUTUBE_CHANNEL=<a channel's /live link that has two streams live>.
func TestLiveVerticalResolve(t *testing.T) {
	if os.Getenv("GENT_LIVE") == "" {
		t.Skip("set GENT_LIVE=1 to run network checks")
	}

	channel := os.Getenv("GENT_YOUTUBE_CHANNEL")
	if channel == "" {
		t.Skip("set GENT_YOUTUBE_CHANNEL to a channel /live link with two live streams")
	}

	landscape, err := resolveLiveURL(context.Background(), httpClient, channel)
	if err != nil {
		t.Fatalf("landscape: %v", err)
	}

	vertical, err := resolveVerticalURL(context.Background(), httpClient, channel)
	if err != nil {
		t.Fatalf("vertical: %v", err)
	}

	t.Logf("landscape=%s vertical=%s", landscape, vertical)

	if vertical == landscape {
		t.Errorf("vertical resolved to the landscape video %s", landscape)
	}

	if _, _, err := fetchInitialData(context.Background(), httpClient, vertical); err != nil {
		t.Errorf("second stream chat bootstrap failed: %v", err)
	}
}
