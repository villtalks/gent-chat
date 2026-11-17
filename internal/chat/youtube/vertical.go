package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
)

const (
	liveBadgeStyle   = "THUMBNAIL_OVERLAY_BADGE_STYLE_LIVE"
	legacyLiveMarker = `"style":"LIVE"`
)

// parseLiveVideoIDs returns the ids of the videos that a channel /streams page
// marks as live right now, in page order (most recently started first).
func parseLiveVideoIDs(html string) ([]string, error) {
	raw, err := extractJSONObject(html, "var ytInitialData = ")
	if err != nil {
		return nil, fmt.Errorf("streams page has no initial data: %w", err)
	}

	var data any

	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to parse streams page data: %w", err)
	}

	var ids []string

	seen := map[string]bool{}

	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	var walk func(v any)

	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			// map order is random; sorted keys keep the result deterministic
			keys := make([]string, 0, len(node))

			for key := range node {
				keys = append(keys, key)
			}

			sort.Strings(keys)

			for _, key := range keys {
				item, isObject := node[key].(map[string]any)

				switch {
				case isObject && key == "lockupViewModel":
					if containsMarker(item, liveBadgeStyle) {
						id, _ := item["contentId"].(string)

						add(id)
					}
				case isObject && (key == "videoRenderer" || key == "gridVideoRenderer"):
					if containsMarker(item, legacyLiveMarker) {
						id, _ := item["videoId"].(string)

						add(id)
					}
				default:
					walk(node[key])
				}
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}

	walk(data)

	return ids, nil
}

func containsMarker(item map[string]any, marker string) bool {
	b, err := json.Marshal(item)
	if err != nil {
		return false
	}

	return bytes.Contains(b, []byte(marker))
}

// pickVertical chooses the vertical stream: the first live video that is not the
// one the channel /live page points at. It only answers when the landscape video
// is itself among at least two live videos; anything else would be a guess that
// could put the landscape chat on the vertical card, so the caller keeps waiting.
func pickVertical(live []string, landscapeID string) (string, bool) {
	if len(live) < 2 || !slices.Contains(live, landscapeID) {
		return "", false
	}

	for _, id := range live {
		if id != landscapeID {
			return id, true
		}
	}

	return "", false
}

// resolveVerticalURL finds the second simultaneous live stream of a channel. It
// returns ErrChannelOffline while there is no live video besides the /live one.
func resolveVerticalURL(ctx context.Context, hc *http.Client, liveURL string) (string, error) {
	landscapeURL, err := resolveLiveURL(ctx, hc, liveURL)
	if err != nil {
		return "", err
	}

	streamsURL := strings.TrimSuffix(liveURL, "/live") + "/streams"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamsURL, nil)
	if err != nil {
		return "", err
	}

	setBrowserHeaders(req)

	resp, err := hc.Do(req)
	if err != nil {
		if errors.Is(err, ErrRateLimited) {
			return "", ErrRateLimited
		}

		return "", fmt.Errorf("failed to fetch streams page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("streams page returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read streams page: %w", err)
	}

	live, err := parseLiveVideoIDs(string(body))
	if err != nil {
		return "", err
	}

	id, ok := pickVertical(live, extractVideoID(landscapeURL))
	if !ok {
		return "", ErrChannelOffline
	}

	return watchURL(id), nil
}
