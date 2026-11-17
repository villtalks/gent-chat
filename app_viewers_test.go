package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"ghost-chat/internal/chat"
	"ghost-chat/internal/config"
	"ghost-chat/internal/viewers"
)

type viewerEmits struct {
	mu      sync.Mutex
	updates []viewers.Update
	others  []string
}

func (v *viewerEmits) emit(event string, data any) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if u, ok := data.(viewers.Update); ok && event == "viewers:update" {
		v.updates = append(v.updates, u)

		return
	}

	v.others = append(v.others, event)
}

func (v *viewerEmits) waitFor(t *testing.T, want ...viewers.Update) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		v.mu.Lock()
		got := append([]viewers.Update(nil), v.updates...)
		v.mu.Unlock()

		if len(got) >= len(want) {
			match := true

			for i := range want {
				if got[i] != want[i] {
					match = false
				}
			}

			if match {
				return
			}
		}

		time.Sleep(2 * time.Millisecond)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	t.Fatalf("viewer updates = %+v, want prefix %+v", v.updates, want)
}

func newViewerApp(t *testing.T) (*App, *viewerEmits, *[]string) {
	t.Helper()

	a := NewApp(&config.Config{}, "", "test")
	emits := &viewerEmits{}

	a.emit = emits.emit

	var mu sync.Mutex

	inputs := []string{}

	a.viewerSource = func(p chat.Platform, input string) (viewers.Source, time.Duration) {
		mu.Lock()
		defer mu.Unlock()

		inputs = append(inputs, string(p)+"="+input)

		return func(context.Context) (int, bool, error) { return 10, true, nil }, 5 * time.Millisecond
	}

	a.clients = map[chat.Platform]chat.Client{chat.PlatformTwitch: &fakeClient{}}

	t.Cleanup(a.viewers.StopAll)

	return a, emits, &inputs
}

func platformData(p chat.Platform) map[string]string {
	return map[string]string{"platform": string(p)}
}

func TestViewerPollersFollowConnection(t *testing.T) {
	a, emits, inputs := newViewerApp(t)

	if err := a.Connect(chat.PlatformTwitch, "streamer"); err != nil {
		t.Fatal(err)
	}

	a.handleChatEvent("chat:connected", platformData(chat.PlatformTwitch))
	emits.waitFor(t, viewers.Update{Platform: chat.PlatformTwitch, Count: 10, Live: true})

	if len(*inputs) != 1 || (*inputs)[0] != "twitch=streamer" {
		t.Errorf("source inputs = %v, want [twitch=streamer]", *inputs)
	}

	if got := a.GetViewers(); len(got) != 1 || got[0].Count != 10 {
		t.Errorf("GetViewers = %+v, want the twitch count", got)
	}

	a.handleChatEvent("chat:waiting", platformData(chat.PlatformTwitch))
	emits.waitFor(t,
		viewers.Update{Platform: chat.PlatformTwitch, Count: 10, Live: true},
		viewers.Update{Platform: chat.PlatformTwitch},
	)

	a.handleChatEvent("chat:connected", platformData(chat.PlatformTwitch))
	emits.waitFor(t,
		viewers.Update{Platform: chat.PlatformTwitch, Count: 10, Live: true},
		viewers.Update{Platform: chat.PlatformTwitch},
		viewers.Update{Platform: chat.PlatformTwitch, Count: 10, Live: true},
	)

	a.handleChatEvent("chat:disconnected", platformData(chat.PlatformTwitch))
	emits.waitFor(t,
		viewers.Update{Platform: chat.PlatformTwitch, Count: 10, Live: true},
		viewers.Update{Platform: chat.PlatformTwitch},
		viewers.Update{Platform: chat.PlatformTwitch, Count: 10, Live: true},
		viewers.Update{Platform: chat.PlatformTwitch},
	)

	if got := a.GetViewers(); len(got) != 0 {
		t.Errorf("GetViewers = %+v, want empty after disconnect", got)
	}
}

func TestChatEventsStillReachTheFrontend(t *testing.T) {
	a, emits, _ := newViewerApp(t)

	a.handleChatEvent("chat:connected", platformData(chat.PlatformTwitch))
	a.handleChatEvent("chat:clear", []string{"#chan", "user"})
	a.handleChatEvent("chat:delete-message", "msg-id")
	a.handleChatEvent("chat:connected", nil)

	emits.mu.Lock()
	defer emits.mu.Unlock()

	want := []string{"chat:connected", "chat:clear", "chat:delete-message", "chat:connected"}

	if len(emits.others) != len(want) {
		t.Fatalf("forwarded events = %v, want %v", emits.others, want)
	}

	for i := range want {
		if emits.others[i] != want[i] {
			t.Errorf("forwarded events = %v, want %v", emits.others, want)
		}
	}
}

func TestViewerSourceSkippedForUnknownPlatform(t *testing.T) {
	a, emits, _ := newViewerApp(t)

	a.viewerSource = func(chat.Platform, string) (viewers.Source, time.Duration) { return nil, 0 }

	a.handleChatEvent("chat:connected", platformData("custom"))

	time.Sleep(20 * time.Millisecond)

	emits.mu.Lock()
	defer emits.mu.Unlock()

	if len(emits.updates) != 0 {
		t.Errorf("viewer updates = %+v, want none", emits.updates)
	}
}

func TestDefaultViewerSourceCoversEveryPlatform(t *testing.T) {
	a := NewApp(&config.Config{}, "", "test")

	a.emit = func(string, any) {}
	a.wireClients()

	t.Cleanup(a.viewers.StopAll)

	want := map[chat.Platform]time.Duration{
		chat.PlatformTwitch:          30 * time.Second,
		chat.PlatformYouTube:         15 * time.Second,
		chat.PlatformYouTubeVertical: 15 * time.Second,
		chat.PlatformKick:            30 * time.Second,
		chat.PlatformTikTok:          30 * time.Second,
	}

	for p, every := range want {
		src, got := a.defaultViewerSource(p, "streamer")
		if src == nil || got != every {
			t.Errorf("%s: source nil=%v interval=%v, want interval %v", p, src == nil, got, every)
		}
	}

	if src, _ := a.defaultViewerSource("custom", "x"); src != nil {
		t.Error("unknown platform must have no viewer source")
	}
}
