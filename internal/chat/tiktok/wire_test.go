package tiktok

import (
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"ghost-chat/internal/chat"
)

func fixture(t testing.TB, name string) []byte {
	t.Helper()

	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestResolveHandle(t *testing.T) {
	good := []string{
		"@SomeName",
		" somename ",
		"somename",
		"https://www.tiktok.com/@somename/live?lang=en",
		"tiktok.com/@somename",
		"https://www.tiktok.com/@SomeName/",
		"https://m.tiktok.com/@somename/live",
	}

	for _, input := range good {
		got, err := ResolveHandle(input)
		if err != nil {
			t.Errorf("ResolveHandle(%q) error: %v", input, err)

			continue
		}

		if got != "somename" {
			t.Errorf("ResolveHandle(%q) = %q, want somename", input, got)
		}
	}

	if got, err := ResolveHandle("@some.name_99"); err != nil || got != "some.name_99" {
		t.Errorf("ResolveHandle with dots and underscores = %q, %v", got, err)
	}

	bad := []string{"", "   ", "@", "https://www.tiktok.com/", "https://www.tiktok.com/foryou", "two words", "https://example.com/@somename"}

	for _, input := range bad {
		if got, err := ResolveHandle(input); err == nil {
			t.Errorf("ResolveHandle(%q) = %q, want error", input, got)
		}
	}
}

func TestParseRoomResponse(t *testing.T) {
	live, err := parseRoomResponse(fixture(t, "room_api_live.json"))
	if err != nil {
		t.Fatalf("live: %v", err)
	}

	if !live.Live || live.RoomID == "" || live.Viewers <= 0 {
		t.Errorf("live room = %+v, want live with room id and viewers", live)
	}

	offline, err := parseRoomResponse(fixture(t, "room_api_offline.json"))
	if err != nil {
		t.Fatalf("offline: %v", err)
	}

	if offline.Live || offline.Viewers != 0 {
		t.Errorf("offline room = %+v, want not live with 0 viewers", offline)
	}

	if _, err := parseRoomResponse(fixture(t, "room_api_notfound.json")); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("not found err = %v, want ErrUserNotFound", err)
	}

	for _, body := range []string{"<html>blocked</html>", "", "{"} {
		if _, err := parseRoomResponse([]byte(body)); err == nil || errors.Is(err, ErrUserNotFound) {
			t.Errorf("parseRoomResponse(%q) err = %v, want a parse error", body, err)
		}
	}
}

func TestReadFieldsRoundTrip(t *testing.T) {
	var b []byte

	b = appendVarintField(b, 1, 300)
	b = appendBytesField(b, 2, []byte("hello"))
	b = appendVarintField(b, 99, 0)

	fields, err := readFields(b)
	if err != nil {
		t.Fatal(err)
	}

	if len(fields) != 3 {
		t.Fatalf("fields = %+v, want 3", fields)
	}

	if fields[0].Num != 1 || fields[0].Varint != 300 {
		t.Errorf("field 0 = %+v", fields[0])
	}

	if fields[1].Num != 2 || string(fields[1].Bytes) != "hello" {
		t.Errorf("field 1 = %+v", fields[1])
	}

	if fields[2].Num != 99 || fields[2].Varint != 0 {
		t.Errorf("field 2 = %+v", fields[2])
	}
}

func TestReadFieldsRejectsBrokenInput(t *testing.T) {
	good := appendBytesField(nil, 2, []byte("hello world"))

	broken := [][]byte{
		good[:len(good)-3],
		{0x0a, 0xff, 0xff, 0xff, 0xff, 0x0f},
		{0x08},
		{0x0b},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0x09, 1, 2, 3},
		{0x0d, 1, 2},
	}

	for i, b := range broken {
		if _, err := readFields(b); err == nil {
			t.Errorf("case %d: want error for % x", i, b)
		}
	}
}

func FuzzReadFields(f *testing.F) {
	f.Add(fixture(f, "frame_chat.bin"))
	f.Add(fixture(f, "frame_roomuserseq.bin"))
	f.Add([]byte{0x0a, 0xff, 0xff, 0xff, 0xff, 0x0f})

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = readFields(data)

		if frame, err := decodePushFrame(data); err == nil {
			if resp, err := decodeResponse(frame.Payload); err == nil {
				for _, m := range resp.Messages {
					_, _ = parseChat(m.Payload, m.MsgID, time.Unix(0, 0))
					_, _ = parseViewerCount(m.Payload)
				}
			}
		}
	})
}

func decodeFixture(t *testing.T, name string) response {
	t.Helper()

	frame, err := decodePushFrame(fixture(t, name))
	if err != nil {
		t.Fatalf("decodePushFrame: %v", err)
	}

	if frame.PayloadType != "msg" {
		t.Fatalf("payload type = %q, want msg", frame.PayloadType)
	}

	resp, err := decodeResponse(frame.Payload)
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}

	return resp
}

func TestDecodeFixtureChat(t *testing.T) {
	resp := decodeFixture(t, "frame_chat.bin")
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	found := 0

	for _, m := range resp.Messages {
		if m.Method != methodChat {
			continue
		}

		msg, err := parseChat(m.Payload, m.MsgID, now)
		if err != nil {
			t.Fatalf("parseChat: %v", err)
		}

		found++

		if msg.Platform != chat.PlatformTikTok {
			t.Errorf("platform = %q, want tiktok", msg.Platform)
		}

		if msg.Username == "" || msg.Text == "" {
			t.Errorf("username/text empty: %+v", msg)
		}

		if !strings.HasPrefix(msg.ID, "tiktok-") || msg.ID == "tiktok-0" {
			t.Errorf("id = %q, want tiktok-<msg id>", msg.ID)
		}

		if len(msg.Fragments) != 1 || msg.Fragments[0].Type != chat.FragmentText || msg.Fragments[0].Text != msg.Text {
			t.Errorf("fragments = %+v, want one text fragment equal to the text", msg.Fragments)
		}

		if !msg.Timestamp.Equal(now) {
			t.Errorf("timestamp = %v, want %v", msg.Timestamp, now)
		}

		if msg.Badges == nil || msg.Tags == nil {
			t.Error("badges and tags must be non-nil so they serialise as empty collections")
		}
	}

	if found == 0 {
		t.Fatal("fixture contains no chat message")
	}
}

func TestParseChatFallsBackToUniqueID(t *testing.T) {
	user := appendBytesField(nil, 38, []byte("some.handle"))
	payload := appendBytesField(nil, 2, user)
	payload = appendBytesField(payload, 3, []byte("hi"))

	msg, err := parseChat(payload, 5, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if msg.Username != "some.handle" || msg.Text != "hi" || msg.ID != "tiktok-5" {
		t.Errorf("msg = %+v", msg)
	}
}

func TestParseChatReadsAvatar(t *testing.T) {
	image := appendBytesField(nil, 1, []byte("https://example.com/a.webp"))
	user := appendBytesField(nil, 3, []byte("Nick"))
	user = appendBytesField(user, 9, image)
	payload := appendBytesField(nil, 2, user)
	payload = appendBytesField(payload, 3, []byte("hello"))

	msg, err := parseChat(payload, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if msg.Username != "Nick" || msg.Avatar != "https://example.com/a.webp" {
		t.Errorf("msg = %+v", msg)
	}
}

func TestParseChatRejectsEmptyMessage(t *testing.T) {
	if _, err := parseChat(appendBytesField(nil, 3, nil), 1, time.Now()); err == nil {
		t.Error("want error for a chat message without text")
	}
}

func TestDecodeFixtureViewers(t *testing.T) {
	resp := decodeFixture(t, "frame_roomuserseq.bin")
	found := false

	for _, m := range resp.Messages {
		if m.Method != methodRoomUserSeq {
			continue
		}

		n, err := parseViewerCount(m.Payload)
		if err != nil {
			t.Fatalf("parseViewerCount: %v", err)
		}

		if n <= 0 {
			t.Errorf("viewer count = %d, want > 0", n)
		}

		found = true
	}

	if !found {
		t.Fatal("fixture contains no room user seq message")
	}
}

func TestParseViewerCountRequiresField(t *testing.T) {
	if _, err := parseViewerCount(appendVarintField(nil, 7, 10)); err == nil {
		t.Error("want error when viewer_count is absent")
	}
}

func TestDecodeResponseGunzips(t *testing.T) {
	inner := appendBytesField(nil, 1, appendBytesField(nil, 1, []byte("WebcastChatMessage")))
	inner = appendBytesField(inner, 5, []byte("ext"))
	inner = appendVarintField(inner, 9, 1)

	var zipped bytes.Buffer

	zw := gzip.NewWriter(&zipped)
	_, _ = zw.Write(inner)
	_ = zw.Close()

	for name, payload := range map[string][]byte{"plain": inner, "gzip": zipped.Bytes()} {
		resp, err := decodeResponse(payload)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if len(resp.Messages) != 1 || resp.Messages[0].Method != "WebcastChatMessage" || string(resp.InternalExt) != "ext" || !resp.NeedsAck {
			t.Errorf("%s: resp = %+v", name, resp)
		}
	}

	if _, err := decodeResponse([]byte{0x1f, 0x8b, 0x00, 0x01}); err == nil {
		t.Error("want error for a corrupt gzip payload")
	}
}

func TestFrames(t *testing.T) {
	ack, err := decodePushFrame(ackFrame(7, []byte("x")))
	if err != nil {
		t.Fatal(err)
	}

	if ack.LogID != 7 || ack.PayloadType != "ack" || ack.PayloadEncoding != "pb" || string(ack.Payload) != "x" {
		t.Errorf("ack = %+v", ack)
	}

	hb, err := decodePushFrame(heartbeatFrame(42))
	if err != nil {
		t.Fatal(err)
	}

	hbFields, _ := readFields(hb.Payload)
	if hb.PayloadType != "hb" || len(hbFields) != 1 || hbFields[0].Num != 1 || hbFields[0].Varint != 42 {
		t.Errorf("heartbeat = %+v fields %+v", hb, hbFields)
	}

	enter, err := decodePushFrame(enterRoomFrame(42))
	if err != nil {
		t.Fatal(err)
	}

	if enter.PayloadType != "im_enter_room" || enter.PayloadEncoding != "pb" {
		t.Errorf("enter frame = %+v", enter)
	}

	got := map[int]string{}
	fields, _ := readFields(enter.Payload)

	for _, f := range fields {
		if f.Wire == wireVarint {
			got[f.Num] = string(rune('0' + f.Varint%10))

			if f.Num == 1 && f.Varint != 42 {
				t.Errorf("room id = %d, want 42", f.Varint)
			}

			if f.Num == 4 && f.Varint != 12 {
				t.Errorf("live id = %d, want 12", f.Varint)
			}
		} else {
			got[f.Num] = string(f.Bytes)
		}
	}

	if got[5] != "audience" || got[9] != "0" {
		t.Errorf("enter room fields = %v, want 5=audience 9=0", got)
	}

	if _, ok := got[1]; !ok {
		t.Error("enter room lacks room id")
	}

	if _, ok := got[4]; !ok {
		t.Error("enter room lacks live id")
	}
}
