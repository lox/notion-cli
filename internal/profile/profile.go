// Package profile resolves which Notion account a notion-cli invocation
// targets and returns the on-disk locations for that account's credentials.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// EnvVar is the environment variable that selects a profile when no
	// --profile flag is passed.
	EnvVar = "NOTION_CLI_PROFILE"
	// DefaultName is the implicit profile name used when the caller has
	// not selected one and legacy top-level credentials exist.
	DefaultName = "default"
	// SettingsFileName is the cross-profile settings file at the top of
	// the notion-cli config dir.
	SettingsFileName = "settings.json"
	// TokenFileName is the OAuth token filename inside a profile dir.
	TokenFileName = "token.json"
	// ConfigFileName is the API config filename inside a profile dir.
	ConfigFileName = "config.json"
	configDirName  = "notion-cli"
)

// Source records where the active profile came from, so auth status can
// tell the user how it resolved.
type Source string

const (
	SourceFlag     Source = "--profile flag"
	SourceEnv      Source = EnvVar
	SourceSettings Source = SettingsFileName
	SourceDefault  Source = "implicit default"
)

// Profile identifies a selected notion-cli profile and where it was
// selected from.
type Profile struct {
	Name   string
	Source Source
}

// ErrNoProfile is returned by Resolve when no profile is selected by flag,
// env, or settings, and no legacy top-level credentials exist to fall back
// to.
var ErrNoProfile = errors.New("no profile specified; pass --profile <name> or set " + EnvVar)

// nameRE enforces the accepted profile-name alphabet: lowercase ASCII,
// leading letter or digit, `_` and `-` allowed in the tail.
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Validate reports whether name is acceptable as a profile identifier.
func Validate(name string) error {
	if name == "" {
		return errors.New("profile name must not be empty")
	}
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: must match %s", name, nameRE.String())
	}
	return nil
}

// Resolve picks the active profile using the precedence:
//
//  1. --profile flag value
//  2. NOTION_CLI_PROFILE environment variable
//  3. default_profile in ~/.config/notion-cli/settings.json
//  4. legacy implicit default (top-level ~/.config/notion-cli/{token,config}.json)
//
// If none of those apply, Resolve returns ErrNoProfile.
func Resolve(flagValue string) (Profile, error) {
	if v := strings.TrimSpace(flagValue); v != "" {
		if err := Validate(v); err != nil {
			return Profile{}, err
		}
		return Profile{Name: v, Source: SourceFlag}, nil
	}
	if v := strings.TrimSpace(os.Getenv(EnvVar)); v != "" {
		if err := Validate(v); err != nil {
			return Profile{}, fmt.Errorf("%s: %w", EnvVar, err)
		}
		return Profile{Name: v, Source: SourceEnv}, nil
	}
	if v, err := loadSettingsDefault(); err != nil {
		return Profile{}, err
	} else if v != "" {
		if err := Validate(v); err != nil {
			return Profile{}, fmt.Errorf("settings.json default_profile: %w", err)
		}
		return Profile{Name: v, Source: SourceSettings}, nil
	}
	if legacyTopLevelExists() {
		return Profile{Name: DefaultName, Source: SourceDefault}, nil
	}
	return Profile{}, ErrNoProfile
}

// TokenRoot returns the top-level directory that contains OAuth token files.
// notion-cli historically stores tokens under $HOME/.config/notion-cli on
// every OS, so we keep that path here to avoid orphaning existing tokens on
// macOS where UserConfigDir would otherwise resolve to
// ~/Library/Application Support.
func TokenRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", configDirName), nil
}

// ConfigRoot returns the top-level directory that contains API config and
// cross-profile settings files. This follows os.UserConfigDir, matching the
// existing config.json layout.
func ConfigRoot() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, configDirName), nil
}

// TokenPath returns the OAuth token file path for the given profile. The
// implicit default profile keeps using the top-level token.json.
func TokenPath(p Profile) (string, error) {
	root, err := TokenRoot()
	if err != nil {
		return "", err
	}
	if p.Name == DefaultName {
		return filepath.Join(root, TokenFileName), nil
	}
	return filepath.Join(root, p.Name, TokenFileName), nil
}

// ConfigPath returns the API config file path for the given profile. The
// implicit default profile keeps using the top-level config.json.
func ConfigPath(p Profile) (string, error) {
	root, err := ConfigRoot()
	if err != nil {
		return "", err
	}
	if p.Name == DefaultName {
		return filepath.Join(root, ConfigFileName), nil
	}
	return filepath.Join(root, p.Name, ConfigFileName), nil
}

// SettingsPath returns the cross-profile settings.json path. It lives next
// to the API config files.
func SettingsPath() (string, error) {
	root, err := ConfigRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, SettingsFileName), nil
}

type settingsFile struct {
	DefaultProfile string `json:"default_profile,omitempty"`
}

func loadSettingsDefault() (string, error) {
	path, err := SettingsPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var s settingsFile
	if err := json.Unmarshal(data, &s); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	return strings.TrimSpace(s.DefaultProfile), nil
}

func legacyTopLevelExists() bool {
	tokenRoot, err := TokenRoot()
	if err == nil {
		if _, err := os.Stat(filepath.Join(tokenRoot, TokenFileName)); err == nil {
			return true
		}
	}
	configRoot, err := ConfigRoot()
	if err == nil {
		if _, err := os.Stat(filepath.Join(configRoot, ConfigFileName)); err == nil {
			return true
		}
	}
	return false
}
