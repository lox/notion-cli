package mcp

import (
	"reflect"
	"testing"
)

func TestBuildSearchToolArgsSendsEmptyQueryWhenBlank(t *testing.T) {
	got := buildSearchToolArgs("", &SearchOptions{ContentSearchMode: "workspace_search"})
	want := map[string]any{
		"query":               "",
		"content_search_mode": "workspace_search",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildSearchToolArgsSendsEmptyQueryWhenWhitespace(t *testing.T) {
	got := buildSearchToolArgs("  \t\n", &SearchOptions{ContentSearchMode: "workspace_search"})
	want := map[string]any{
		"query":               "",
		"content_search_mode": "workspace_search",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildSearchToolArgsIncludesTrimmedQueryWhenPresent(t *testing.T) {
	got := buildSearchToolArgs("  Tasks  ", &SearchOptions{ContentSearchMode: "workspace_search"})
	want := map[string]any{
		"query":               "Tasks",
		"content_search_mode": "workspace_search",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}
