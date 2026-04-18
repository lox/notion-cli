package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lox/notion-cli/internal/profile"
	"github.com/mark3labs/mcp-go/client/transport"
)

var ErrNoToken = errors.New("no token available")

type FileTokenStore struct {
	path string
	mu   sync.RWMutex
}

// NewFileTokenStore returns a token store backed by the default profile's
// token.json, matching the legacy behavior before multi-profile support.
func NewFileTokenStore() (*FileTokenStore, error) {
	return NewFileTokenStoreForProfile(profile.Profile{Name: profile.DefaultName, Source: profile.SourceDefault})
}

// NewFileTokenStoreForProfile returns a token store rooted at the given
// profile's on-disk location.
func NewFileTokenStoreForProfile(p profile.Profile) (*FileTokenStore, error) {
	path, err := profile.TokenPath(p)
	if err != nil {
		return nil, err
	}
	return &FileTokenStore{path: path}, nil
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

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0600)
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

	data, err = json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0600)
}
