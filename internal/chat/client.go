package chat

type Platform string

const (
	PlatformTwitch          Platform = "twitch"
	PlatformYouTube         Platform = "youtube"
	PlatformYouTubeVertical Platform = "youtube_vertical"
	PlatformKick            Platform = "kick"
	PlatformTikTok          Platform = "tiktok"
)

type Client interface {
	Connect(input string) error
	Disconnect()
}
