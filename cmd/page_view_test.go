package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

func TestShouldLoadPageViewComments(t *testing.T) {
	tests := []struct {
		name            string
		raw             bool
		includeComments bool
		asJSON          bool
		want            bool
	}{
		{
			name:            "plain view loads comments by default",
			raw:             false,
			includeComments: true,
			asJSON:          false,
			want:            true,
		},
		{
			name:            "comments can be disabled",
			raw:             false,
			includeComments: false,
			asJSON:          false,
			want:            false,
		},
		{
			name:            "raw view skips comments",
			raw:             true,
			includeComments: true,
			asJSON:          false,
			want:            false,
		},
		{
			name:            "json keeps comments enabled even with raw output",
			raw:             true,
			includeComments: true,
			asJSON:          true,
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldLoadPageViewComments(tt.raw, tt.includeComments, tt.asJSON)
			if got != tt.want {
				t.Fatalf("shouldLoadPageViewComments() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderFetchedPageViewContinuesWhenCommentsFail(t *testing.T) {
	originalLoad := loadPageViewCommentsFn
	originalPrintViewedPage := printViewedPageFn
	originalPrintWarning := printWarningFn
	defer func() {
		loadPageViewCommentsFn = originalLoad
		printViewedPageFn = originalPrintViewedPage
		printWarningFn = originalPrintWarning
	}()

	loadPageViewCommentsFn = func(_ context.Context, _ *mcp.Client, _ string, _ string, _ bool, _ bool, _ bool) ([]output.Comment, error) {
		return nil, errors.New("comments unavailable")
	}

	var rendered bool
	printViewedPageFn = func(page output.Page, comments []output.Comment, asJSON bool) error {
		rendered = true
		if asJSON {
			t.Fatalf("expected text rendering")
		}
		if page.Content != "page body" {
			t.Fatalf("expected page content to be preserved, got %q", page.Content)
		}
		if comments != nil {
			t.Fatalf("expected comments to be dropped on failure, got %#v", comments)
		}
		return nil
	}

	warnings := 0
	printWarningFn = func(message string) {
		warnings++
		if message != "Unable to load comments: comments unavailable" {
			t.Fatalf("unexpected warning %q", message)
		}
	}

	err := renderFetchedPageView(context.Background(), &Context{}, nil, "page-123", &mcp.FetchResult{Content: "page body"}, false, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !rendered {
		t.Fatalf("expected page to still render")
	}
	if warnings != 1 {
		t.Fatalf("expected one warning, got %d", warnings)
	}
}

func TestRenderFetchedPageViewJSONSkipsWarningsWhenCommentsFail(t *testing.T) {
	originalLoad := loadPageViewCommentsFn
	originalPrintViewedPage := printViewedPageFn
	originalPrintWarning := printWarningFn
	defer func() {
		loadPageViewCommentsFn = originalLoad
		printViewedPageFn = originalPrintViewedPage
		printWarningFn = originalPrintWarning
	}()

	loadPageViewCommentsFn = func(_ context.Context, _ *mcp.Client, _ string, _ string, _ bool, _ bool, _ bool) ([]output.Comment, error) {
		return nil, errors.New("comments unavailable")
	}

	var renderedJSON bool
	printViewedPageFn = func(_ output.Page, comments []output.Comment, asJSON bool) error {
		renderedJSON = true
		if !asJSON {
			t.Fatalf("expected JSON rendering")
		}
		if comments != nil {
			t.Fatalf("expected comments to be dropped on failure, got %#v", comments)
		}
		return nil
	}

	printWarningFn = func(message string) {
		t.Fatalf("unexpected warning in JSON mode: %q", message)
	}

	err := renderFetchedPageView(context.Background(), &Context{JSON: true}, nil, "page-123", &mcp.FetchResult{Content: "page body"}, false, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !renderedJSON {
		t.Fatalf("expected JSON page output")
	}
}
