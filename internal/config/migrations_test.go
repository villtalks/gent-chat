package config

import (
	"testing"
)

func TestAppliesMigration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = "1.0.0"

	migs := []Migration{
		{Version: "1.1.0", Migrate: func(cfg *Config) {
			cfg.General.Language = "de-DE"
		}},
	}

	err := runMigrations(&cfg, migs, "2.0.0")

	if err != nil {
		t.Fatal(err)
	}

	if cfg.Version != "1.1.0" {
		t.Errorf("got version %s, want 1.1.0", cfg.Version)
	}

	if cfg.General.Language != "de-DE" {
		t.Errorf("got language %s, want de-DE", cfg.General.Language)
	}
}

func TestSkipsMigration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = "1.1.0"

	migs := []Migration{
		{Version: "1.0.0", Migrate: func(cfg *Config) {
			cfg.General.Language = "de-DE"
		}},
	}

	err := runMigrations(&cfg, migs, "2.0.0")

	if err != nil {
		t.Fatal(err)
	}

	if cfg.Version != "1.1.0" {
		t.Errorf("got version %s, want 1.1.0", cfg.Version)
	}

	if cfg.General.Language == "de-DE" {
		t.Errorf("got language %s, want not de-DE", cfg.General.Language)
	}
}

func TestAppliesMultipleMigrations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = "1.0.0"

	migs := []Migration{
		{Version: "1.1.0", Migrate: func(cfg *Config) {}},
		{Version: "1.2.0", Migrate: func(cfg *Config) {}},
	}

	err := runMigrations(&cfg, migs, "2.0.0")

	if err != nil {
		t.Fatal(err)
	}

	if cfg.Version != "1.2.0" {
		t.Errorf("got version %s, want 1.2.0", cfg.Version)
	}
}

func TestThrowsErrorOnInvalidVersion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = "invalid"

	migs := []Migration{
		{Version: "1.1.0", Migrate: func(cfg *Config) {}},
	}

	err := runMigrations(&cfg, migs, "2.0.0")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestShowWaitingMessageMigration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = "4.0.1"
	cfg.General.ShowWaitingMessage = false

	err := RunMigrations(&cfg, "4.0.2")

	if err != nil {
		t.Fatal(err)
	}

	if !cfg.General.ShowWaitingMessage {
		t.Error("expected ShowWaitingMessage to be true after migration")
	}

	if cfg.Version != "4.0.2" {
		t.Errorf("got version %s, want 4.0.2", cfg.Version)
	}
}

func TestSkipsMigrationsNewerThanTarget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = "1.0.0"

	migs := []Migration{
		{Version: "3.0.0", Migrate: func(cfg *Config) {}},
	}

	err := runMigrations(&cfg, migs, "2.0.0")

	if err != nil {
		t.Fatal(err)
	}

	if cfg.Version != "1.0.0" {
		t.Errorf("got version %s, want 1.0.0", cfg.Version)
	}
}

func TestSkipsMigrationsWhenVersionEmpty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = ""

	ran := false

	migs := []Migration{
		{Version: "1.1.0", Migrate: func(cfg *Config) {
			ran = true
		}},
	}

	err := runMigrations(&cfg, migs, "2.0.0")

	if err != nil {
		t.Fatal(err)
	}

	if ran {
		t.Error("expected no migrations to run")
	}

	if cfg.Version != "" {
		t.Errorf("got version %s, want empty", cfg.Version)
	}
}

func TestMigration4_10_0(t *testing.T) {
	cfg := &Config{Version: "4.9.0"}

	if err := RunMigrations(cfg, "4.10.0"); err != nil {
		t.Fatal(err)
	}

	if cfg.TikTok.FadeTimeout != 30 || cfg.YouTubeVertical.FadeTimeout != 30 {
		t.Errorf("fade timeouts = %d/%d, want 30/30", cfg.TikTok.FadeTimeout, cfg.YouTubeVertical.FadeTimeout)
	}

	if !cfg.General.ShowViewerCount || !cfg.General.ShowPlatformViewers {
		t.Error("viewer flags should be enabled by the 4.10.0 migration")
	}

	if cfg.Version != "4.10.0" {
		t.Errorf("Version = %q, want 4.10.0", cfg.Version)
	}
}

func TestMigration4_10_0KeepsExistingTimeouts(t *testing.T) {
	cfg := &Config{Version: "4.9.0"}
	cfg.TikTok.FadeTimeout = 45

	if err := RunMigrations(cfg, "4.10.0"); err != nil {
		t.Fatal(err)
	}

	if cfg.TikTok.FadeTimeout != 45 {
		t.Errorf("TikTok.FadeTimeout = %d, want 45 kept", cfg.TikTok.FadeTimeout)
	}
}
