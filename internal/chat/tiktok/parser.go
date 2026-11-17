package tiktok

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"ghost-chat/internal/chat"
)

const (
	methodChat        = "WebcastChatMessage"
	methodRoomUserSeq = "WebcastRoomUserSeqMessage"
)

// parseChat maps a WebcastChatMessage (user = 2, content = 3) to a ChatMessage.
func parseChat(payload []byte, msgID uint64, now time.Time) (chat.ChatMessage, error) {
	fields, err := readFields(payload)
	if err != nil {
		return chat.ChatMessage{}, err
	}

	var text, nickname, uniqueID, avatar string

	for _, f := range fields {
		if f.Wire != wireBytes {
			continue
		}

		switch f.Num {
		case 2:
			nickname, uniqueID, avatar = parseUser(f.Bytes)
		case 3:
			text = strings.ToValidUTF8(string(f.Bytes), "")
		}
	}

	if strings.TrimSpace(text) == "" {
		return chat.ChatMessage{}, errors.New("chat message has no text")
	}

	username := nickname
	if username == "" {
		username = uniqueID
	}

	return chat.ChatMessage{
		ID:        "tiktok-" + strconv.FormatUint(msgID, 10),
		Platform:  chat.PlatformTikTok,
		Username:  username,
		Text:      text,
		Badges:    []chat.Badge{},
		Fragments: []chat.MessageFragment{{Type: chat.FragmentText, Text: text}},
		Timestamp: now,
		Tags:      map[string]string{},
		Avatar:    avatar,
	}, nil
}

// parseUser reads nickname = 3, avatar_thumb = 9 (url_list = 1) and unique_id = 38.
func parseUser(b []byte) (nickname, uniqueID, avatar string) {
	fields, err := readFields(b)
	if err != nil {
		return "", "", ""
	}

	for _, f := range fields {
		if f.Wire != wireBytes {
			continue
		}

		switch f.Num {
		case 3:
			nickname = strings.TrimSpace(strings.ToValidUTF8(string(f.Bytes), ""))
		case 38:
			uniqueID = strings.ToValidUTF8(string(f.Bytes), "")
		case 9:
			if avatar == "" {
				avatar = firstImageURL(f.Bytes)
			}
		}
	}

	return nickname, uniqueID, avatar
}

func firstImageURL(b []byte) string {
	fields, err := readFields(b)
	if err != nil {
		return ""
	}

	for _, f := range fields {
		if f.Num == 1 && f.Wire == wireBytes && strings.HasPrefix(string(f.Bytes), "https://") {
			return string(f.Bytes)
		}
	}

	return ""
}

// parseViewerCount reads viewer_count = 3 from a WebcastRoomUserSeqMessage.
func parseViewerCount(payload []byte) (int, error) {
	fields, err := readFields(payload)
	if err != nil {
		return 0, err
	}

	for _, f := range fields {
		if f.Num == 3 && f.Wire == wireVarint {
			if f.Varint > 1<<40 {
				return 0, errors.New("viewer count out of range")
			}

			return int(f.Varint), nil
		}
	}

	return 0, errors.New("room user seq message has no viewer count")
}
