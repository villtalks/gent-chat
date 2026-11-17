package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReturnsDefaultsWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "ghost-chat", "config.json")

	config, err := Load(path)

	if err != nil {
		t.Fatal(err)
	}

	defaultConfig := DefaultConfig()

	if config.General.Language != defaultConfig.General.Language {
		t.Errorf("got %v, want %v", config.General.Language, defaultConfig.General.Language)
	}

	if config.WindowState.Width != defaultConfig.WindowState.Width {
		t.Errorf("got %v, want %v", config.WindowState.Width, defaultConfig.WindowState.Width)
	}

	if config.YouTube.FadeTimeout != defaultConfig.YouTube.FadeTimeout {
		t.Errorf("got %v, want %v", config.YouTube.FadeTimeout, defaultConfig.YouTube.FadeTimeout)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "ghost-chat", "config.json")

	defaultConfig := DefaultConfig()

	defaultConfig.General.Language = "de-DE"
	defaultConfig.Twitch.DefaultChannel = "streamer"

	if err := Save(&defaultConfig, path); err != nil {
		t.Fatal("Failed to save config file", err)
	}

	config, err := Load(path)

	if err != nil {
		t.Fatal("Failed to load config", err)
	}

	if config.General.Language != defaultConfig.General.Language {
		t.Errorf("got %v, want %v", config.General.Language, defaultConfig.General.Language)
	}

	if config.Twitch.DefaultChannel != defaultConfig.Twitch.DefaultChannel {
		t.Errorf("got %v, want %v", config.Twitch.DefaultChannel, defaultConfig.Twitch.DefaultChannel)
	}
}

func TestLoadRecoversFromInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "ghost-chat", "config.json")

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal("Failed to create tmp dir", err)
	}

	garbageBytes := []byte("not json")

	if err := os.WriteFile(path, garbageBytes, 0644); err != nil {
		t.Fatal("Failed to write garbage data", err)
	}

	config, err := Load(path)

	if err != nil {
		t.Fatal("Expected nil error for invalid JSON, got", err)
	}

	if config == nil {
		t.Fatal("Expected default config, got nil")
	}

	defaultConfig := DefaultConfig()

	if config.General.Language != defaultConfig.General.Language {
		t.Errorf("got %v, want %v", config.General.Language, defaultConfig.General.Language)
	}

	backupBytes, err := os.ReadFile(path + ".corrupted")

	if err != nil {
		t.Fatal("Expected corrupted backup file to exist, got", err)
	}

	if string(backupBytes) != string(garbageBytes) {
		t.Errorf("got backup bytes %q, want %q", backupBytes, garbageBytes)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Expected corrupted file at %s to be removed, but it still exists", path)
	}
}

func TestHideFromTaskbarDefaultsToFalse(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.General.HideFromTaskbar {
		t.Error("hide from taskbar should default to false")
	}
}

func TestHideFromTaskbarRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ghost-chat", "config.json")

	cfg := DefaultConfig()
	cfg.General.HideFromTaskbar = true

	if err := Save(&cfg, path); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(raw), `"hide_from_taskbar": true`) {
		t.Errorf("expected hide_from_taskbar key in saved json, got %s", raw)
	}

	loaded, err := Load(path)

	if err != nil {
		t.Fatal(err)
	}

	if !loaded.General.HideFromTaskbar {
		t.Error("hide from taskbar did not survive round trip")
	}
}

func TestDefaultConfigNewPlatforms(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.TikTok.FadeTimeout != 30 {
		t.Errorf("TikTok.FadeTimeout = %d, want 30", cfg.TikTok.FadeTimeout)
	}

	if cfg.YouTubeVertical.FadeTimeout != 30 {
		t.Errorf("YouTubeVertical.FadeTimeout = %d, want 30", cfg.YouTubeVertical.FadeTimeout)
	}

	if !cfg.General.ShowViewerCount || !cfg.General.ShowPlatformViewers {
		t.Errorf("viewer flags = %v/%v, want true/true", cfg.General.ShowViewerCount, cfg.General.ShowPlatformViewers)
	}
}

func TestLoadUpstreamConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	upstream := `{"version":"4.9.0","install_id":"abc","general":{"language":"de-DE"},"twitch":{"default_channel":"streamer","account":{"login":"x"},"events":{"redemptions":true}},"youtube":{"channel_id":"@somechannel","fade_timeout":12}}`

	if err := os.WriteFile(path, []byte(upstream), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Twitch.DefaultChannel != "streamer" || cfg.YouTube.FadeTimeout != 12 || cfg.General.Language != "de-DE" {
		t.Errorf("upstream values lost: %+v", cfg)
	}

	if cfg.TikTok.FadeTimeout != 30 || cfg.YouTubeVertical.FadeTimeout != 30 {
		t.Errorf("new blocks lack defaults: tiktok=%+v vertical=%+v", cfg.TikTok, cfg.YouTubeVertical)
	}

	if !cfg.General.ShowViewerCount || !cfg.General.ShowPlatformViewers {
		t.Error("viewer flags should default to true when absent from the file")
	}
}

func TestGetConfigPathUsesGentChat(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(path, "gent-chat") || strings.Contains(path, "ghost-chat") {
		t.Errorf("config path = %q, want it under gent-chat", path)
	}
}
