package tiktok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	userAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	userAgentAlt   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0"
	roomStatusLive = 2
	codeNotFound   = 19881007
)

var defaultRoomAPI = "https://www.tiktok.com/api-live/user/room/"

var ErrUserNotFound = errors.New("tiktok user not found")

var reHandle = regexp.MustCompile(`^[a-z0-9._]{1,64}$`)

// ResolveHandle normalises user input to a bare lowercase TikTok handle.
// Accepts "@name", "name" and tiktok.com/@name[/live] links.
func ResolveHandle(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("empty input")
	}

	if strings.Contains(input, "/") {
		if !strings.Contains(input, "://") {
			input = "https://" + input
		}

		u, err := url.Parse(input)
		if err != nil {
			return "", fmt.Errorf("invalid tiktok link: %w", err)
		}

		host := strings.ToLower(u.Hostname())
		if host != "tiktok.com" && !strings.HasSuffix(host, ".tiktok.com") {
			return "", fmt.Errorf("not a tiktok link: %s", host)
		}

		input = ""

		for _, part := range strings.Split(u.Path, "/") {
			if strings.HasPrefix(part, "@") {
				input = part

				break
			}
		}

		if input == "" {
			return "", errors.New("no @name in tiktok link")
		}
	}

	handle := strings.ToLower(strings.TrimPrefix(input, "@"))
	if !reHandle.MatchString(handle) {
		return "", fmt.Errorf("invalid tiktok name %q", input)
	}

	return handle, nil
}

type RoomInfo struct {
	RoomID  string
	Live    bool
	Viewers int
}

type roomResponse struct {
	StatusCode int `json:"statusCode"`
	Data       *struct {
		User struct {
			RoomID string `json:"roomId"`
			Status int    `json:"status"`
		} `json:"user"`
		LiveRoom struct {
			LiveRoomStats struct {
				UserCount int `json:"userCount"`
			} `json:"liveRoomStats"`
		} `json:"liveRoom"`
	} `json:"data"`
}

func parseRoomResponse(body []byte) (RoomInfo, error) {
	var resp roomResponse

	if err := json.Unmarshal(body, &resp); err != nil {
		return RoomInfo{}, fmt.Errorf("failed to parse room response: %w", err)
	}

	if resp.StatusCode == codeNotFound {
		return RoomInfo{}, ErrUserNotFound
	}

	if resp.StatusCode != 0 || resp.Data == nil {
		return RoomInfo{}, fmt.Errorf("room lookup failed with status %d", resp.StatusCode)
	}

	info := RoomInfo{
		RoomID: resp.Data.User.RoomID,
		Live:   resp.Data.User.Status == roomStatusLive && resp.Data.User.RoomID != "",
	}

	if info.Live {
		info.Viewers = max(resp.Data.LiveRoom.LiveRoomStats.UserCount, 0)
	}

	return info, nil
}

// FetchRoom looks up the live room of a handle through the public web endpoint.
func FetchRoom(ctx context.Context, hc *http.Client, handle string) (RoomInfo, error) {
	return fetchRoom(ctx, hc, defaultRoomAPI, handle)
}

func fetchRoom(ctx context.Context, hc *http.Client, api, handle string) (RoomInfo, error) {
	q := url.Values{
		"aid":              {"1988"},
		"app_name":         {"tiktok_web"},
		"device_platform":  {"web_pc"},
		"app_language":     {"en"},
		"browser_language": {"en-US"},
		"region":           {"US"},
		"user_is_login":    {"false"},
		"sourceType":       {"54"},
		"staleTime":        {"600000"},
		"uniqueId":         {handle},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"?"+q.Encode(), nil)
	if err != nil {
		return RoomInfo{}, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://www.tiktok.com/")

	resp, err := hc.Do(req)
	if err != nil {
		return RoomInfo{}, fmt.Errorf("failed to fetch room: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return RoomInfo{}, fmt.Errorf("room lookup returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return RoomInfo{}, fmt.Errorf("failed to read room response: %w", err)
	}

	return parseRoomResponse(body)
}
