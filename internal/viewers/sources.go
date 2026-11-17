package viewers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ghost-chat/internal/chat/tiktok"
)

const (
	browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

	twitchEndpoint  = "https://gql.twitch.tv/gql"
	twitchClientID  = "kimne78kx3ncx6brgo4mv6wki5h1ko"
	twitchQuery     = "query($login:String!){user(login:$login){stream{viewersCount}}}"
	youtubeEndpoint = "https://www.youtube.com/youtubei/v1/updated_metadata?prettyPrint=false"
	youtubeVersion  = "2.20241101.00.00"
	kickEndpoint    = "https://kick.com/api/v2/channels/"
)

func fetch(ctx context.Context, hc *http.Client, req *http.Request) ([]byte, error) {
	resp, err := hc.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
}

func normalizeTwitchLogin(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimRight(input, "/")

	if i := strings.LastIndex(input, "/"); i >= 0 {
		input = input[i+1:]
	}

	return strings.ToLower(strings.TrimPrefix(input, "#"))
}

// TwitchSource reads the viewer count through the public web GraphQL endpoint.
func TwitchSource(hc *http.Client, channel string) Source {
	return twitchSource(hc, twitchEndpoint, channel)
}

func twitchSource(hc *http.Client, endpoint, channel string) Source {
	login := normalizeTwitchLogin(channel)

	return func(ctx context.Context) (int, bool, error) {
		body, err := json.Marshal(map[string]any{
			"query":     twitchQuery,
			"variables": map[string]string{"login": login},
		})
		if err != nil {
			return 0, false, err
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return 0, false, err
		}

		req.Header.Set("Client-ID", twitchClientID)
		req.Header.Set("Content-Type", "application/json")

		raw, err := fetch(ctx, hc, req)
		if err != nil {
			return 0, false, fmt.Errorf("twitch viewers: %w", err)
		}

		return parseTwitch(raw)
	}
}

func parseTwitch(body []byte) (int, bool, error) {
	var resp struct {
		Data *struct {
			User *struct {
				Stream *struct {
					ViewersCount int `json:"viewersCount"`
				} `json:"stream"`
			} `json:"user"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, fmt.Errorf("failed to parse twitch viewers: %w", err)
	}

	if resp.Data == nil {
		return 0, false, errors.New("twitch viewers response has no data")
	}

	if resp.Data.User == nil || resp.Data.User.Stream == nil {
		return 0, false, nil
	}

	return resp.Data.User.Stream.ViewersCount, true, nil
}

// YouTubeSource reads the concurrent viewers of whichever video videoID
// currently returns. An empty id means no stream is being followed.
func YouTubeSource(hc *http.Client, videoID func() string) Source {
	return youtubeSource(hc, youtubeEndpoint, videoID)
}

func youtubeSource(hc *http.Client, endpoint string, videoID func() string) Source {
	return func(ctx context.Context) (int, bool, error) {
		id := videoID()
		if id == "" {
			return 0, false, nil
		}

		body, err := json.Marshal(map[string]any{
			"context": map[string]any{
				"client": map[string]string{"clientName": "WEB", "clientVersion": youtubeVersion, "hl": "en"},
			},
			"videoId": id,
		})
		if err != nil {
			return 0, false, err
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return 0, false, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", browserUA)

		raw, err := fetch(ctx, hc, req)
		if err != nil {
			return 0, false, fmt.Errorf("youtube viewers: %w", err)
		}

		return parseYouTube(raw)
	}
}

func parseYouTube(body []byte) (int, bool, error) {
	var resp struct {
		Actions []struct {
			UpdateViewershipAction *struct {
				ViewCount struct {
					VideoViewCountRenderer struct {
						IsLive            bool   `json:"isLive"`
						OriginalViewCount string `json:"originalViewCount"`
					} `json:"videoViewCountRenderer"`
				} `json:"viewCount"`
			} `json:"updateViewershipAction"`
		} `json:"actions"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, fmt.Errorf("failed to parse youtube viewers: %w", err)
	}

	for _, action := range resp.Actions {
		if action.UpdateViewershipAction == nil {
			continue
		}

		renderer := action.UpdateViewershipAction.ViewCount.VideoViewCountRenderer

		// once a stream ends the same field carries total views, not viewers
		if !renderer.IsLive {
			return 0, false, nil
		}

		count, err := strconv.Atoi(renderer.OriginalViewCount)
		if err != nil {
			return 0, false, fmt.Errorf("unexpected youtube viewer count %q", renderer.OriginalViewCount)
		}

		return count, true, nil
	}

	return 0, false, errors.New("youtube viewers response has no viewership action")
}

// KickSource reads the viewer count from the channel endpoint the Kick site
// itself uses. Browser-like headers are required to pass Cloudflare.
func KickSource(hc *http.Client, channel string) Source {
	return kickSource(hc, kickEndpoint, channel)
}

func kickSource(hc *http.Client, endpoint, channel string) Source {
	slug := normalizeTwitchLogin(channel)

	return func(ctx context.Context) (int, bool, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint+url.PathEscape(slug), nil)
		if err != nil {
			return 0, false, err
		}

		req.Header.Set("User-Agent", browserUA)
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Referer", "https://kick.com/")
		req.Header.Set("Origin", "https://kick.com")

		raw, err := fetch(ctx, hc, req)
		if err != nil {
			return 0, false, fmt.Errorf("kick viewers: %w", err)
		}

		return parseKick(raw)
	}
}

func parseKick(body []byte) (int, bool, error) {
	var resp struct {
		Slug       *string `json:"slug"`
		Livestream *struct {
			IsLive      bool `json:"is_live"`
			ViewerCount int  `json:"viewer_count"`
		} `json:"livestream"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, fmt.Errorf("failed to parse kick viewers: %w", err)
	}

	if resp.Slug == nil {
		return 0, false, errors.New("kick viewers response is not a channel")
	}

	if resp.Livestream == nil || !resp.Livestream.IsLive {
		return 0, false, nil
	}

	return resp.Livestream.ViewerCount, true, nil
}

// TikTokSource is the backup for the count TikTok pushes on the chat socket.
func TikTokSource(hc *http.Client, handle func() string) Source {
	return func(ctx context.Context) (int, bool, error) {
		name := handle()
		if name == "" {
			return 0, false, nil
		}

		room, err := tiktok.FetchRoom(ctx, hc, name)
		if err != nil {
			return 0, false, fmt.Errorf("tiktok viewers: %w", err)
		}

		return room.Viewers, room.Live, nil
	}
}
