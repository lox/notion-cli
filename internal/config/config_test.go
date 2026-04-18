package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lox/notion-cli/internal/profile"
)

func TestLoadWithMetaDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.BaseURL != defaultAPIBaseURL {
		t.Fatalf("BaseURL = %q, want %q", loaded.Config.API.BaseURL, defaultAPIBaseURL)
	}
	if loaded.Config.API.NotionVersion != defaultNotionAPIVer {
		t.Fatalf("NotionVersion = %q, want %q", loaded.Config.API.NotionVersion, defaultNotionAPIVer)
	}
	if loaded.APITokenSource != APITokenSourceNone {
		t.Fatalf("APITokenSource = %q, want %q", loaded.APITokenSource, APITokenSourceNone)
	}
}

func TestLoadWithMetaReportsConfigTokenSource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetAPIToken("secret-token"); err != nil {
		t.Fatalf("SetAPIToken: %v", err)
	}

	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.Token != "secret-token" {
		t.Fatalf("Token = %q, want secret-token", loaded.Config.API.Token)
	}
	if loaded.APITokenSource != APITokenSourceConfig {
		t.Fatalf("APITokenSource = %q, want %q", loaded.APITokenSource, APITokenSourceConfig)
	}
}

func TestLoadWithMetaEnvOverrideWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetAPIToken("config-token"); err != nil {
		t.Fatalf("SetAPIToken: %v", err)
	}
	loaded, err := LoadWithMeta(APIOverrides{
		Token:         "env-token",
		BaseURL:       "https://example.test/v1/",
		NotionVersion: "2026-04-01",
	})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.Token != "env-token" {
		t.Fatalf("Token = %q, want env-token", loaded.Config.API.Token)
	}
	if loaded.Config.API.BaseURL != "https://example.test/v1" {
		t.Fatalf("BaseURL = %q", loaded.Config.API.BaseURL)
	}
	if loaded.Config.API.NotionVersion != "2026-04-01" {
		t.Fatalf("NotionVersion = %q", loaded.Config.API.NotionVersion)
	}
	if loaded.APITokenSource != APITokenSourceEnv {
		t.Fatalf("APITokenSource = %q, want %q", loaded.APITokenSource, APITokenSourceEnv)
	}
}

func TestUnsetAPITokenClearsStoredToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetAPIToken("secret-token"); err != nil {
		t.Fatalf("SetAPIToken: %v", err)
	}
	if err := UnsetAPIToken(); err != nil {
		t.Fatalf("UnsetAPIToken: %v", err)
	}

	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.Token != "" {
		t.Fatalf("Token = %q, want empty", loaded.Config.API.Token)
	}
	if loaded.APITokenSource != APITokenSourceNone {
		t.Fatalf("APITokenSource = %q, want %q", loaded.APITokenSource, APITokenSourceNone)
	}
}

func TestSetAPITokenIsIsolatedPerProfile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))

	work := profile.Profile{Name: "work", Source: profile.SourceFlag}
	home := profile.Profile{Name: "home", Source: profile.SourceFlag}

	if err := SetAPITokenForProfile(work, "work-token"); err != nil {
		t.Fatalf("SetAPITokenForProfile(work): %v", err)
	}
	if err := SetAPITokenForProfile(home, "home-token"); err != nil {
		t.Fatalf("SetAPITokenForProfile(home): %v", err)
	}

	loadedWork, err := LoadWithMetaForProfile(work, APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMetaForProfile(work): %v", err)
	}
	if loadedWork.Config.API.Token != "work-token" {
		t.Fatalf("work token = %q, want work-token", loadedWork.Config.API.Token)
	}

	loadedHome, err := LoadWithMetaForProfile(home, APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMetaForProfile(home): %v", err)
	}
	if loadedHome.Config.API.Token != "home-token" {
		t.Fatalf("home token = %q, want home-token", loadedHome.Config.API.Token)
	}

	if loadedWork.Path == loadedHome.Path {
		t.Fatalf("work and home configs share path: %q", loadedWork.Path)
	}
}

func TestSaveSecuresConfigFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := Default()
	cfg.API.Token = "secret-token"
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config perm = %o, want 600", perm)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("config dir perm = %o, want 700", perm)
	}
}
