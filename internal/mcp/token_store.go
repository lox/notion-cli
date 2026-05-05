package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lox/notion-cli/internal/config"
	"github.com/mark3labs/mcp-go/client/transport"
)

var ErrNoToken = errors.New("no token available")

var (
	refreshLocksMu sync.Mutex
	refreshLocks   = map[string]*sync.Mutex{}
)

type FileTokenStore struct {
	path string
	mu   sync.RWMutex
}

func NewFileTokenStore(profile string) (*FileTokenStore, error) {
	paths, err := config.PathsForProfile(profile)
	if err != nil {
		return nil, err
	}
	return &FileTokenStore{path: paths.TokenPath}, nil
}

func (s *FileTokenStore) GetToken(ctx context.Context) (*transport.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoToken
		}
		return nil, err
	}

	var stored storedToken
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}

	return &transport.Token{
		AccessToken:  stored.AccessToken,
		TokenType:    stored.TokenType,
		RefreshToken: stored.RefreshToken,
		ExpiresAt:    stored.ExpiresAt,
	}, nil
}

func (s *FileTokenStore) SaveToken(ctx context.Context, token *transport.Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Preserve existing client_id if present
	var existing storedToken
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	stored := storedToken{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
		SavedAt:      time.Now(),
		ClientID:     existing.ClientID,
	}

	return s.writeStoredToken(ctx, stored)
}

func (s *FileTokenStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *FileTokenStore) Path() string {
	return s.path
}

func (s *FileTokenStore) LockPath() string {
	return s.path + ".lock"
}

type storedToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	SavedAt      time.Time `json:"saved_at,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
}

func (s *FileTokenStore) GetClientID(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var stored storedToken
	if err := json.Unmarshal(data, &stored); err != nil {
		return "", err
	}

	return stored.ClientID, nil
}

func (s *FileTokenStore) SaveClientID(ctx context.Context, clientID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	var stored storedToken
	data, err := os.ReadFile(s.path)
	if err == nil {
		_ = json.Unmarshal(data, &stored)
	}

	stored.ClientID = clientID

	return s.writeStoredToken(ctx, stored)
}

func (s *FileTokenStore) writeStoredToken(ctx context.Context, stored storedToken) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".token-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func (s *FileTokenStore) WithLock(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	processLock := processRefreshLock(s.LockPath())
	processLock.Lock()
	defer processLock.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	lockFile, err := acquireFileLock(s.LockPath())
	if err != nil {
		return err
	}
	defer func() { _ = releaseFileLock(lockFile) }()

	if err := ctx.Err(); err != nil {
		return err
	}
	return fn()
}

func processRefreshLock(path string) *sync.Mutex {
	refreshLocksMu.Lock()
	defer refreshLocksMu.Unlock()

	lock, ok := refreshLocks[path]
	if !ok {
		lock = &sync.Mutex{}
		refreshLocks[path] = lock
	}
	return lock
}
