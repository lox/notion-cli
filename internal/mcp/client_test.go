package mcp

import (
	"reflect"
	"testing"
)

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
