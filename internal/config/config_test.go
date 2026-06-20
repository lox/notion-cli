package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func isolateConfigDir(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func TestLoadWithMetaDefaults(t *testing.T) {
	isolateConfigDir(t)

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
	if loaded.Profile != DefaultProfile() {
		t.Fatalf("Profile = %q, want %q", loaded.Profile, DefaultProfile())
	}
}

func TestLoadWithMetaReportsConfigTokenSource(t *testing.T) {
	isolateConfigDir(t)
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

func TestLoadWithMetaDefaultDoesNotRequireHome(t *testing.T) {
	xdgConfigHome := t.TempDir()
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)

	// macOS and Windows resolve UserConfigDir from HOME, so this regression
	// only surfaces on platforms (Linux containers, etc.) where ConfigDir
	// can be satisfied via XDG_CONFIG_HOME without HOME.
	configRoot, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("UserConfigDir requires HOME on this platform: %v", err)
	}

	configDir := filepath.Join(configRoot, configDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	configPath := filepath.Join(configDir, configFileName)
	if err := os.WriteFile(configPath, []byte(`{"api":{"token":"config-token"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Profile != DefaultProfile() {
		t.Fatalf("Profile = %q, want %q", loaded.Profile, DefaultProfile())
	}
	if loaded.Path != configPath {
		t.Fatalf("Path = %q, want %q", loaded.Path, configPath)
	}
	if loaded.Config.API.Token != "config-token" {
		t.Fatalf("Token = %q, want config-token", loaded.Config.API.Token)
	}
}

func TestLoadWithMetaEnvOverrideWins(t *testing.T) {
	isolateConfigDir(t)
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
	isolateConfigDir(t)
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

func TestSaveSecuresConfigFile(t *testing.T) {
	isolateConfigDir(t)

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

func TestPathsForProfileDefaultAndNamed(t *testing.T) {
	isolateConfigDir(t)

	defaultPaths, err := PathsForProfile("")
	if err != nil {
		t.Fatalf("PathsForProfile default: %v", err)
	}
	if defaultPaths.Profile != DefaultProfile() {
		t.Fatalf("default profile = %q, want %q", defaultPaths.Profile, DefaultProfile())
	}
	if got := filepath.Base(defaultPaths.ConfigPath); got != configFileName {
		t.Fatalf("default config filename = %q, want %q", got, configFileName)
	}
	if got := filepath.Base(defaultPaths.TokenPath); got != tokenFileName {
		t.Fatalf("default token filename = %q, want %q", got, tokenFileName)
	}
	if !strings.Contains(defaultPaths.TokenPath, filepath.Join(".config", configDirName, tokenFileName)) {
		t.Fatalf("default token path = %q, want legacy .config/notion-cli path", defaultPaths.TokenPath)
	}

	workPaths, err := PathsForProfile("work")
	if err != nil {
		t.Fatalf("PathsForProfile work: %v", err)
	}
	if workPaths.Profile != "work" {
		t.Fatalf("work profile = %q, want work", workPaths.Profile)
	}
	if !strings.Contains(workPaths.ConfigPath, filepath.Join(profilesDirName, "work")) {
		t.Fatalf("work config path = %q, want profiles/work segment", workPaths.ConfigPath)
	}
	if !strings.Contains(workPaths.TokenPath, filepath.Join(profilesDirName, "work")) {
		t.Fatalf("work token path = %q, want profiles/work segment", workPaths.TokenPath)
	}
}

func TestProfileSpecificAPITokensAreIsolated(t *testing.T) {
	isolateConfigDir(t)

	if err := SetAPIToken("default-token"); err != nil {
		t.Fatalf("SetAPIToken default: %v", err)
	}
	if err := SetAPITokenForProfile("work", "work-token"); err != nil {
		t.Fatalf("SetAPITokenForProfile: %v", err)
	}

	defaultLoaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta default: %v", err)
	}
	if defaultLoaded.Config.API.Token != "default-token" {
		t.Fatalf("default token = %q, want default-token", defaultLoaded.Config.API.Token)
	}

	workLoaded, err := LoadWithMeta(APIOverrides{Profile: "work"})
	if err != nil {
		t.Fatalf("LoadWithMeta work: %v", err)
	}
	if workLoaded.Config.API.Token != "work-token" {
		t.Fatalf("work token = %q, want work-token", workLoaded.Config.API.Token)
	}
	if workLoaded.Path == defaultLoaded.Path {
		t.Fatalf("profile config path should differ from default path")
	}
}

func TestResolveProfileRejectsInvalidNames(t *testing.T) {
	for _, value := range []string{
		"../oops",
		"work/team",
		"two words",
		"Work",
		".work",
		"work.",
		"work-",
		"ümlaut",
	} {
		if _, err := ResolveProfile(value); err == nil {
			t.Fatalf("ResolveProfile(%q) should fail", value)
		}
	}
}

func TestResolveProfileRejectsWindowsReservedNames(t *testing.T) {
	for _, value := range []string{
		"con",
		"prn",
		"aux",
		"nul",
		"com1",
		"com9",
		"lpt1",
		"lpt9",
		"con.txt",
	} {
		if _, err := ResolveProfile(value); err == nil {
			t.Fatalf("ResolveProfile(%q) should fail as Windows reserved", value)
		}
	}
}

func TestResolveProfileAllowsPortableNames(t *testing.T) {
	for _, value := range []string{"work", "work-1", "personal_2", "brian@brianle.xyz"} {
		got, err := ResolveProfile(value)
		if err != nil {
			t.Fatalf("ResolveProfile(%q): %v", value, err)
		}
		if got != value {
			t.Fatalf("profile = %q, want %q", got, value)
		}
	}
}

func TestResolveSelectedProfileUsesActiveStateWhenUnset(t *testing.T) {
	isolateConfigDir(t)
	if err := SetActiveProfile("work"); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}

	profile, err := ResolveSelectedProfile("")
	if err != nil {
		t.Fatalf("ResolveSelectedProfile: %v", err)
	}
	if profile != "work" {
		t.Fatalf("profile = %q, want work", profile)
	}
}

func TestResolveSelectedProfilePrefersExplicitValue(t *testing.T) {
	isolateConfigDir(t)
	if err := SetActiveProfile("work"); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}

	profile, err := ResolveSelectedProfile("personal")
	if err != nil {
		t.Fatalf("ResolveSelectedProfile: %v", err)
	}
	if profile != "personal" {
		t.Fatalf("profile = %q, want personal", profile)
	}
}

func TestListProfilesIncludesActiveDefaultAndNamedProfiles(t *testing.T) {
	isolateConfigDir(t)
	if err := SetActiveProfile("work"); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}
	if err := SetAPITokenForProfile("personal", "personal-token"); err != nil {
		t.Fatalf("SetAPITokenForProfile: %v", err)
	}

	got, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	want := []string{"work", "default", "personal"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("profiles = %#v, want %#v", got, want)
	}
}

func TestPathForProfileDefaultDoesNotRequireHome(t *testing.T) {
	isolateConfigDir(t)
	t.Setenv("HOME", "")

	// macOS and Windows resolve UserConfigDir from HOME, so this regression
	// only surfaces on platforms (Linux containers, etc.) where ConfigDir
	// can be satisfied via XDG_CONFIG_HOME without HOME.
	if _, err := os.UserConfigDir(); err != nil {
		t.Skipf("UserConfigDir requires HOME on this platform: %v", err)
	}

	path, err := PathForProfile("")
	if err != nil {
		t.Fatalf("PathForProfile(default): %v", err)
	}
	if path == "" {
		t.Fatalf("expected non-empty config path for default profile")
	}

	path, err = PathForProfile("work")
	if err != nil {
		t.Fatalf("PathForProfile(work): %v", err)
	}
	if path == "" {
		t.Fatalf("expected non-empty config path for named profile")
	}
}
