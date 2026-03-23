package mcp

import (
	"reflect"
	"testing"
)

func TestBuildUpdatePageToolArgsReplaceContent(t *testing.T) {
	req := UpdatePageRequest{
		PageID:     "page-123",
		Command:    "replace_content",
		NewContent: "hello",
	}

	got := buildUpdatePageToolArgs(req)
	want := map[string]any{
		"page_id": "page-123",
		"command": "replace_content",
		"new_str": "hello",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildUpdatePageToolArgsUpdateContent(t *testing.T) {
	req := UpdatePageRequest{
		PageID:  "page-123",
		Command: "update_content",
		ContentUpdates: []ContentUpdate{
			{OldStr: "old", NewStr: "new"},
		},
	}

	got := buildUpdatePageToolArgs(req)
	want := map[string]any{
		"page_id": "page-123",
		"command": "update_content",
		"content_updates": []any{
			map[string]any{"old_str": "old", "new_str": "new"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildUpdatePageToolArgsUpdateProperties(t *testing.T) {
	req := UpdatePageRequest{
		PageID:     "page-123",
		Command:    "update_properties",
		Properties: map[string]any{"title": "New title"},
	}

	got := buildUpdatePageToolArgs(req)
	want := map[string]any{
		"page_id":    "page-123",
		"command":    "update_properties",
		"properties": map[string]any{"title": "New title"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildUpdatePageToolArgsInsertContentAfter(t *testing.T) {
	req := UpdatePageRequest{
		PageID:    "page-123",
		Command:   "insert_content_after",
		Selection: "start...end",
		NewStr:    "extra",
	}

	got := buildUpdatePageToolArgs(req)
	want := map[string]any{
		"page_id":                 "page-123",
		"command":                 "insert_content_after",
		"selection_with_ellipsis": "start...end",
		"new_str":                 "extra",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}
