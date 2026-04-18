package mcp

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lox/notion-cli/internal/profile"
)

func TestNewFileTokenStoreForProfileIsolatesPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	defaultStore, err := NewFileTokenStoreForProfile(profile.Profile{Name: profile.DefaultName, Source: profile.SourceDefault})
	if err != nil {
		t.Fatalf("NewFileTokenStoreForProfile(default): %v", err)
	}
	workStore, err := NewFileTokenStoreForProfile(profile.Profile{Name: "work", Source: profile.SourceFlag})
	if err != nil {
		t.Fatalf("NewFileTokenStoreForProfile(work): %v", err)
	}

	defaultWant := filepath.Join(tmp, ".config", "notion-cli", "token.json")
	workWant := filepath.Join(tmp, ".config", "notion-cli", "work", "token.json")
	if defaultStore.Path() != defaultWant {
		t.Fatalf("default store path = %q, want %q", defaultStore.Path(), defaultWant)
	}
	if workStore.Path() != workWant {
		t.Fatalf("work store path = %q, want %q", workStore.Path(), workWant)
	}
}

func TestBuildSearchToolArgsOmitsBlankQuery(t *testing.T) {
	got := buildSearchToolArgs("", &SearchOptions{ContentSearchMode: "workspace_search"})
	want := map[string]any{
		"content_search_mode": "workspace_search",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildSearchToolArgsIncludesQueryWhenPresent(t *testing.T) {
	got := buildSearchToolArgs("Tasks", &SearchOptions{ContentSearchMode: "workspace_search"})
	want := map[string]any{
		"query":               "Tasks",
		"content_search_mode": "workspace_search",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}
