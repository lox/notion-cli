package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client/transport"
)

func TestNewFileTokenStoreUsesProfilePath(t *testing.T) {
	isolateMCPConfig(t)

	store, err := NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}

	if got := filepath.Base(store.Path()); got != "token.json" {
		t.Fatalf("token filename = %q, want token.json", got)
	}
	if !strings.Contains(store.Path(), filepath.Join("profiles", "work")) {
		t.Fatalf("token path = %q, want profiles/work segment", store.Path())
	}
}

func TestSaveTokenWritesAtomicallyAndPreservesClientID(t *testing.T) {
	isolateMCPConfig(t)

	store, err := NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}

	if err := store.SaveClientID(context.Background(), "client-123"); err != nil {
		t.Fatalf("SaveClientID: %v", err)
	}
	if err := store.SaveToken(context.Background(), &transport.Token{
		AccessToken:  "access-123",
		TokenType:    "bearer",
		RefreshToken: "refresh-123",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("token file mode = %o, want 0600", got)
	}

	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	var stored storedToken
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal token: %v", err)
	}
	if stored.ClientID != "client-123" {
		t.Fatalf("client ID = %q, want client-123", stored.ClientID)
	}
	if stored.RefreshToken != "refresh-123" {
		t.Fatalf("refresh token = %q, want refresh-123", stored.RefreshToken)
	}
}
