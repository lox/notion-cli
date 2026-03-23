package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lox/notion-cli/internal/cli"
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

func TestBuildPageEditRequestFindAppendUsesInsertContentAfter(t *testing.T) {
	req, err := buildPageEditRequest("", "## Section", "", "\nExtra details")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if req.Command != "insert_content_after" {
		t.Fatalf("expected insert_content_after command, got %q", req.Command)
	}

	if req.Selection != "## Section" {
		t.Fatalf("unexpected selection: %q", req.Selection)
	}
	if req.NewStr != "\nExtra details" {
		t.Fatalf("unexpected new string: %q", req.NewStr)
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

func TestResolveFetchIDUsesCanonicalIDForKnownRefKinds(t *testing.T) {
	resolver := func(_ context.Context, _ *mcp.Client, _ string) (string, error) {
		return "resolved-by-name", nil
	}

	tests := []struct {
		name     string
		page     string
		ref      cli.PageRef
		wantID   string
		wantErr  bool
		errMatch string
		resolver pageIDResolver
	}{
		{
			name:   "canonical id for ref id",
			page:   "https://www.notion.so/workspace/Page-12345678abcdef1234567890abcdef12",
			ref:    cli.PageRef{Kind: cli.RefID, ID: "12345678-abcd-ef12-3456-7890abcdef12"},
			wantID: "12345678-abcd-ef12-3456-7890abcdef12",
			resolver: func(_ context.Context, _ *mcp.Client, _ string) (string, error) {
				return "", errors.New("resolver should not be called")
			},
		},
		{
			name:   "url ref keeps original for fetch",
			page:   "https://example.com/page",
			ref:    cli.PageRef{Kind: cli.RefURL},
			wantID: "https://example.com/page",
			resolver: func(_ context.Context, _ *mcp.Client, _ string) (string, error) {
				return "", errors.New("resolver should not be called")
			},
		},
		{
			name:     "name ref resolves via resolver",
			page:     "Meeting Notes",
			ref:      cli.PageRef{Kind: cli.RefName},
			wantID:   "resolved-by-name",
			resolver: resolver,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveFetchID(context.Background(), tt.page, tt.ref, nil, tt.resolver)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Fatalf("expected error to contain %q, got %q", tt.errMatch, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got != tt.wantID {
				t.Fatalf("resolveFetchID() = %q, want %q", got, tt.wantID)
			}
		})
	}
}
