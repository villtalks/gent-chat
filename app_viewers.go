package main

import (
	"net/http"
	"time"

	"ghost-chat/internal/chat"
	"ghost-chat/internal/viewers"
)

const (
	twitchViewerInterval  = 30 * time.Second
	youtubeViewerInterval = 15 * time.Second
	kickViewerInterval    = 30 * time.Second
	tiktokViewerInterval  = 30 * time.Second
)

var viewerHTTP = &http.Client{Timeout: 10 * time.Second}

// handleChatEvent forwards a chat client event to the frontend and keeps the
// viewer pollers in step with each platform's connection state.
func (a *App) handleChatEvent(event string, data any) {
	if a.emit != nil {
		a.emit(event, data)
	}

	payload, ok := data.(map[string]string)
	if !ok {
		return
	}

	platform := chat.Platform(payload["platform"])
	if platform == "" {
		return
	}

	switch event {
	case "chat:connected":
		a.inputsMu.Lock()
		input := a.inputs[platform]
		a.inputsMu.Unlock()

		if src, every := a.viewerSource(platform, input); src != nil {
			a.viewers.Start(platform, src, every)
		}
	case "chat:waiting", "chat:disconnected":
		a.viewers.Stop(platform)
	}
}

func (a *App) rememberInput(platform chat.Platform, input string) {
	a.inputsMu.Lock()
	defer a.inputsMu.Unlock()

	a.inputs[platform] = input
}

func (a *App) defaultViewerSource(platform chat.Platform, input string) (viewers.Source, time.Duration) {
	switch platform {
	case chat.PlatformTwitch:
		return viewers.TwitchSource(viewerHTTP, input), twitchViewerInterval
	case chat.PlatformKick:
		return viewers.KickSource(viewerHTTP, input), kickViewerInterval
	case chat.PlatformYouTube:
		if a.youtube == nil {
			return nil, 0
		}

		return viewers.YouTubeSource(viewerHTTP, a.youtube.VideoID), youtubeViewerInterval
	case chat.PlatformYouTubeVertical:
		if a.youtubeVertical == nil {
			return nil, 0
		}

		return viewers.YouTubeSource(viewerHTTP, a.youtubeVertical.VideoID), youtubeViewerInterval
	case chat.PlatformTikTok:
		if a.tiktok == nil {
			return nil, 0
		}

		return viewers.TikTokSource(viewerHTTP, a.tiktok.Handle), tiktokViewerInterval
	}

	return nil, 0
}

// GetViewers returns the latest viewer count of every connected platform, for
// a view that mounts after the updates were emitted.
func (a *App) GetViewers() []viewers.Update {
	return a.viewers.Latest()
}
