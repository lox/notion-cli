package profile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// isolateProfileDirs redirects HOME and UserConfigDir to a temp dir so the
// resolver only sees state we set up for the test.
func isolateProfileDirs(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// os.UserConfigDir on Linux reads XDG_CONFIG_HOME, on macOS it reads
	// nothing and composes from HOME. Set both so the lookup is deterministic.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))
	return tmp
}

func TestValidateRejectsEmpty(t *testing.T) {
	if err := Validate(""); err == nil {
		t.Fatalf("Validate(\"\") returned nil, expected error")
	}
}

func TestValidateRejectsBadCharacters(t *testing.T) {
	bad := []string{
		"Work",              // uppercase
		"work/sub",          // slash
		"-leading-dash",     // leading dash
		"_leading-under",    // leading underscore
		"has spaces",        // whitespace
		"has.dot",           // dot
		"has!bang",          // punctuation
		"../escape",         // path traversal
		"work/../other",     // path traversal
		"",                  // empty
	}
	for _, name := range bad {
		if err := Validate(name); err == nil {
			t.Errorf("Validate(%q) returned nil, expected error", name)
		}
	}
}

func TestValidateAcceptsCommonNames(t *testing.T) {
	good := []string{"work", "home", "default", "brianle", "proj1", "proj_1", "a-b-c", "0xble"}
	for _, name := range good {
		if err := Validate(name); err != nil {
			t.Errorf("Validate(%q) returned %v, expected nil", name, err)
		}
	}
}

func TestResolvePrefersFlagOverEnvAndSettings(t *testing.T) {
	isolateProfileDirs(t)
	t.Setenv(EnvVar, "env-profile")
	writeSettings(t, "settings-profile")

	p, err := Resolve("flag-profile")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name != "flag-profile" || p.Source != SourceFlag {
		t.Fatalf("Resolve = %+v, want flag-profile/SourceFlag", p)
	}
}

func TestResolvePrefersEnvOverSettings(t *testing.T) {
	isolateProfileDirs(t)
	t.Setenv(EnvVar, "env-profile")
	writeSettings(t, "settings-profile")

	p, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name != "env-profile" || p.Source != SourceEnv {
		t.Fatalf("Resolve = %+v, want env-profile/SourceEnv", p)
	}
}

func TestResolveUsesSettingsWhenFlagAndEnvUnset(t *testing.T) {
	isolateProfileDirs(t)
	t.Setenv(EnvVar, "")
	writeSettings(t, "settings-profile")

	p, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name != "settings-profile" || p.Source != SourceSettings {
		t.Fatalf("Resolve = %+v, want settings-profile/SourceSettings", p)
	}
}

func TestResolveFallsBackToImplicitDefaultWhenLegacyFilesExist(t *testing.T) {
	tmp := isolateProfileDirs(t)
	t.Setenv(EnvVar, "")

	tokenRoot := filepath.Join(tmp, ".config", configDirName)
	if err := os.MkdirAll(tokenRoot, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tokenRoot, TokenFileName), []byte(`{"access_token":"legacy"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name != DefaultName || p.Source != SourceDefault {
		t.Fatalf("Resolve = %+v, want default/SourceDefault", p)
	}
}

func TestResolveReturnsErrNoProfileWhenNothingResolves(t *testing.T) {
	isolateProfileDirs(t)
	t.Setenv(EnvVar, "")

	_, err := Resolve("")
	if !errors.Is(err, ErrNoProfile) {
		t.Fatalf("Resolve = %v, want ErrNoProfile", err)
	}
}

func TestResolveValidatesFlagValue(t *testing.T) {
	isolateProfileDirs(t)
	t.Setenv(EnvVar, "")

	if _, err := Resolve("BadName"); err == nil {
		t.Fatalf("Resolve(\"BadName\") returned nil, expected validation error")
	}
}

func TestResolveValidatesEnvValue(t *testing.T) {
	isolateProfileDirs(t)
	t.Setenv(EnvVar, "BadName")

	if _, err := Resolve(""); err == nil {
		t.Fatalf("Resolve with bad env returned nil, expected validation error")
	}
}

func TestTokenPathDefaultPointsToLegacyTopLevel(t *testing.T) {
	tmp := isolateProfileDirs(t)

	path, err := TokenPath(Profile{Name: DefaultName, Source: SourceDefault})
	if err != nil {
		t.Fatalf("TokenPath: %v", err)
	}
	want := filepath.Join(tmp, ".config", configDirName, TokenFileName)
	if path != want {
		t.Fatalf("TokenPath = %q, want %q", path, want)
	}
}

func TestTokenPathNamedProfileUsesSubdir(t *testing.T) {
	tmp := isolateProfileDirs(t)

	path, err := TokenPath(Profile{Name: "work", Source: SourceFlag})
	if err != nil {
		t.Fatalf("TokenPath: %v", err)
	}
	want := filepath.Join(tmp, ".config", configDirName, "work", TokenFileName)
	if path != want {
		t.Fatalf("TokenPath = %q, want %q", path, want)
	}
}

func TestConfigPathNamedProfileIsolatedFromDefault(t *testing.T) {
	isolateProfileDirs(t)

	defaultPath, err := ConfigPath(Profile{Name: DefaultName})
	if err != nil {
		t.Fatalf("ConfigPath default: %v", err)
	}
	workPath, err := ConfigPath(Profile{Name: "work"})
	if err != nil {
		t.Fatalf("ConfigPath work: %v", err)
	}
	if defaultPath == workPath {
		t.Fatalf("default and named profile share config path: %q", defaultPath)
	}
}

func writeSettings(t *testing.T, defaultProfile string) {
	t.Helper()
	path, err := SettingsPath()
	if err != nil {
		t.Fatalf("SettingsPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(settingsFile{DefaultProfile: defaultProfile})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
