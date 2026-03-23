package cmd

import (
	"testing"

	"github.com/lox/notion-cli/internal/mcp"
)

func TestBuildPageEditRequestReplace(t *testing.T) {
	req, err := buildPageEditRequest("new content", "", "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if req.Command != "replace_content" {
		t.Fatalf("expected replace_content command, got %q", req.Command)
	}

	if req.NewContent != "new content" {
		t.Fatalf("expected new content to be set")
	}
}

func TestBuildPageEditRequestFindReplaceUsesUpdateContent(t *testing.T) {
	req, err := buildPageEditRequest("", "old text", "new text", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if req.Command != "update_content" {
		t.Fatalf("expected update_content command, got %q", req.Command)
	}

	want := []mcp.ContentUpdate{{OldStr: "old text", NewStr: "new text"}}
	if len(req.ContentUpdates) != len(want) {
		t.Fatalf("expected %d content updates, got %d", len(want), len(req.ContentUpdates))
	}
	if req.ContentUpdates[0] != want[0] {
		t.Fatalf("unexpected content update: %#v", req.ContentUpdates[0])
	}
}

func TestBuildPageEditRequestFindAppendUsesUpdateContent(t *testing.T) {
	req, err := buildPageEditRequest("", "## Section", "", "\nExtra details")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if req.Command != "update_content" {
		t.Fatalf("expected update_content command, got %q", req.Command)
	}

	if len(req.ContentUpdates) != 1 {
		t.Fatalf("expected one content update, got %d", len(req.ContentUpdates))
	}

	if req.ContentUpdates[0].OldStr != "## Section" {
		t.Fatalf("unexpected old string: %q", req.ContentUpdates[0].OldStr)
	}
	if req.ContentUpdates[0].NewStr != "## Section\nExtra details" {
		t.Fatalf("unexpected new string: %q", req.ContentUpdates[0].NewStr)
	}
}

func TestBuildPageEditRequestInvalidCombinations(t *testing.T) {
	tests := []struct {
		name        string
		replace     string
		find        string
		replaceWith string
		appendText  string
	}{
		{
			name:        "replace cannot be combined",
			replace:     "all",
			find:        "old",
			replaceWith: "new",
		},
		{
			name:        "replace with requires find",
			replaceWith: "new",
		},
		{
			name:       "append requires find",
			appendText: "extra",
		},
		{
			name: "requires an action",
		},
		{
			name:        "find requires either replace-with or append",
			find:        "old",
			replaceWith: "",
			appendText:  "",
		},
		{
			name:        "replace and append are mutually exclusive",
			find:        "old",
			replaceWith: "new",
			appendText:  "extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := buildPageEditRequest(tt.replace, tt.find, tt.replaceWith, tt.appendText); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}
