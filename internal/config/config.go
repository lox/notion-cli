package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const (
	configDirName       = "notion-cli"
	configFileName      = "config.json"
	tokenFileName       = "token.json"
	stateFileName       = "state.json"
	profilesDirName     = "profiles"
	defaultProfileName  = "default"
	defaultAPIBaseURL   = "https://api.notion.com/v1"
	defaultNotionAPIVer = "2026-03-11"
)

type Config struct {
	API APIConfig `json:"api,omitempty"`
}

type APIConfig struct {
	BaseURL       string `json:"base_url,omitempty"`
	NotionVersion string `json:"notion_version,omitempty"`
	Token         string `json:"token,omitempty"`
}

type LoadedConfig struct {
	Config         Config
	Profile        string
	Path           string
	APITokenSource string
	HasConfigToken bool
}

type APIOverrides struct {
	Profile       string
	BaseURL       string
	NotionVersion string
	Token         string
}

type ProfilePaths struct {
	Profile    string
	ConfigPath string
	TokenPath  string
}

type State struct {
	ActiveProfile string `json:"active_profile,omitempty"`
}

const (
	APITokenSourceNone   = "none"
	APITokenSourceConfig = "config"
	APITokenSourceEnv    = "env"
)

func Default() Config {
	return Config{
		API: APIConfig{
			BaseURL:       defaultAPIBaseURL,
			NotionVersion: defaultNotionAPIVer,
		},
	}
}

func Path() (string, error) {
	return PathForProfile("")
}

func ConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, configDirName), nil
}

func ProfilesDir() (string, error) {
	baseDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, profilesDirName), nil
}

func StatePath() (string, error) {
	baseDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, stateFileName), nil
}

func PathForProfile(profile string) (string, error) {
	paths, err := PathsForProfile(profile)
	if err != nil {
		return "", err
	}
	return paths.ConfigPath, nil
}

func PathsForProfile(profile string) (ProfilePaths, error) {
	baseDir, err := ConfigDir()
	if err != nil {
		return ProfilePaths{}, err
	}

	resolvedProfile, err := ResolveProfile(profile)
	if err != nil {
		return ProfilePaths{}, err
	}

	profileDir := baseDir
	if resolvedProfile != defaultProfileName {
		profileDir = filepath.Join(baseDir, profilesDirName, resolvedProfile)
	}

	return ProfilePaths{
		Profile:    resolvedProfile,
		ConfigPath: filepath.Join(profileDir, configFileName),
		TokenPath:  filepath.Join(profileDir, tokenFileName),
	}, nil
}

func DefaultProfile() string {
	return defaultProfileName
}

func ResolveProfile(profile string) (string, error) {
	normalized := strings.TrimSpace(profile)
	if normalized == "" {
		return defaultProfileName, nil
	}
	if normalized == "." || normalized == ".." {
		return "", fmt.Errorf("invalid profile %q", profile)
	}
	for _, r := range normalized {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' || r == '@' {
			continue
		}
		return "", fmt.Errorf("invalid profile %q: use letters, numbers, at sign, dot, underscore, and hyphen", profile)
	}
	return normalized, nil
}

func LoadState() (State, error) {
	path, err := StatePath()
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}
	if len(data) == 0 {
		return State{}, nil
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	if state.ActiveProfile != "" {
		resolved, err := ResolveProfile(state.ActiveProfile)
		if err != nil {
			return State{}, err
		}
		state.ActiveProfile = resolved
	}
	return state, nil
}

func SaveState(state State) error {
	if state.ActiveProfile != "" {
		resolved, err := ResolveProfile(state.ActiveProfile)
		if err != nil {
			return err
		}
		state.ActiveProfile = resolved
	}

	path, err := StatePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("secure state dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, stateFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}

	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temp state: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("replace state: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure state file: %w", err)
	}
	return nil
}

func ResolveSelectedProfile(requested string) (string, error) {
	if strings.TrimSpace(requested) != "" {
		return ResolveProfile(requested)
	}
	state, err := LoadState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveProfile) != "" {
		return ResolveProfile(state.ActiveProfile)
	}
	return DefaultProfile(), nil
}

func SetActiveProfile(profile string) error {
	resolved, err := ResolveProfile(profile)
	if err != nil {
		return err
	}
	return SaveState(State{ActiveProfile: resolved})
}

func ActiveProfile() (string, error) {
	state, err := LoadState()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.ActiveProfile) != "" {
		return state.ActiveProfile, nil
	}
	return DefaultProfile(), nil
}

func ListProfiles() ([]string, error) {
	baseDir, err := ProfilesDir()
	if err != nil {
		return nil, err
	}

	names := map[string]struct{}{
		DefaultProfile(): {},
	}
	active, err := ActiveProfile()
	if err != nil {
		return nil, err
	}
	names[active] = struct{}{}

	entries, err := os.ReadDir(baseDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		resolved, err := ResolveProfile(entry.Name())
		if err != nil {
			return nil, err
		}
		names[resolved] = struct{}{}
	}

	var rest []string
	for name := range names {
		if name == active || name == DefaultProfile() {
			continue
		}
		rest = append(rest, name)
	}
	slices.Sort(rest)

	profiles := []string{active}
	if active != DefaultProfile() {
		profiles = append(profiles, DefaultProfile())
	}
	profiles = append(profiles, rest...)
	return profiles, nil
}

func Load() (Config, error) {
	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		return Config{}, err
	}
	return loaded.Config, nil
}

func LoadWithMeta(overrides APIOverrides) (LoadedConfig, error) {
	cfg := Default()
	paths, err := PathsForProfile(overrides.Profile)
	if err != nil {
		return LoadedConfig{}, err
	}
	path := paths.ConfigPath

	fileCfg, err := loadFile(path)
	if err != nil {
		return LoadedConfig{}, err
	}
	cfg = merge(cfg, fileCfg)
	source := APITokenSourceNone
	if strings.TrimSpace(fileCfg.API.Token) != "" {
		source = APITokenSourceConfig
	}

	applyOverrides(&cfg, overrides)
	if strings.TrimSpace(overrides.Token) != "" {
		source = APITokenSourceEnv
	}

	normalize(&cfg)
	return LoadedConfig{
		Config:         cfg,
		Profile:        paths.Profile,
		Path:           path,
		APITokenSource: source,
		HasConfigToken: strings.TrimSpace(fileCfg.API.Token) != "",
	}, nil
}

func Save(cfg Config) error {
	return SaveForProfile("", cfg)
}

func SaveForProfile(profile string, cfg Config) error {
	path, err := PathForProfile(profile)
	if err != nil {
		return err
	}

	normalize(&cfg)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("secure config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, configFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}

	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temp config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("replace config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure config file: %w", err)
	}
	return nil
}

func SetAPIToken(token string) error {
	return SetAPITokenForProfile("", token)
}

func SetAPITokenForProfile(profile, token string) error {
	cfg, err := loadForMutation(profile)
	if err != nil {
		return err
	}
	cfg.API.Token = strings.TrimSpace(token)
	return SaveForProfile(profile, cfg)
}

func UnsetAPIToken() error {
	return UnsetAPITokenForProfile("")
}

func UnsetAPITokenForProfile(profile string) error {
	cfg, err := loadForMutation(profile)
	if err != nil {
		return err
	}
	cfg.API.Token = ""
	return SaveForProfile(profile, cfg)
}

func loadForMutation(profile string) (Config, error) {
	path, err := PathForProfile(profile)
	if err != nil {
		return Config{}, err
	}

	cfg := Default()
	fileCfg, err := loadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg = merge(cfg, fileCfg)
	normalize(&cfg)
	return cfg, nil
}

func loadFile(path string) (Config, error) {
	cfg := Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func merge(base, overlay Config) Config {
	if strings.TrimSpace(overlay.API.BaseURL) != "" {
		base.API.BaseURL = overlay.API.BaseURL
	}
	if strings.TrimSpace(overlay.API.NotionVersion) != "" {
		base.API.NotionVersion = overlay.API.NotionVersion
	}
	if strings.TrimSpace(overlay.API.Token) != "" {
		base.API.Token = overlay.API.Token
	}
	return base
}

func applyOverrides(cfg *Config, overrides APIOverrides) {
	if cfg == nil {
		return
	}

	if s := strings.TrimSpace(overrides.BaseURL); s != "" {
		cfg.API.BaseURL = s
	}
	if s := strings.TrimSpace(overrides.NotionVersion); s != "" {
		cfg.API.NotionVersion = s
	}
	if s := strings.TrimSpace(overrides.Token); s != "" {
		cfg.API.Token = s
	}
}

func normalize(cfg *Config) {
	if cfg == nil {
		return
	}
	cfg.API.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.API.BaseURL), "/")
	if cfg.API.BaseURL == "" {
		cfg.API.BaseURL = defaultAPIBaseURL
	}
	cfg.API.NotionVersion = strings.TrimSpace(cfg.API.NotionVersion)
	if cfg.API.NotionVersion == "" {
		cfg.API.NotionVersion = defaultNotionAPIVer
	}
	cfg.API.Token = strings.TrimSpace(cfg.API.Token)
}
