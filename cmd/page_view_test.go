package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lox/notion-cli/internal/api"
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

func TestRenderFetchedPageViewJSONEnrichesWithAPIMetadata(t *testing.T) {
	originalLoad := loadPageViewCommentsFn
	originalPrintViewedPage := printViewedPageFn
	originalLoadMetadata := loadPageMetadataFn
	defer func() {
		loadPageViewCommentsFn = originalLoad
		printViewedPageFn = originalPrintViewedPage
		loadPageMetadataFn = originalLoadMetadata
	}()

	loadPageViewCommentsFn = func(_ context.Context, _ *mcp.Client, _ string, _ string, _ bool, _ bool, _ bool) ([]output.Comment, error) {
		return nil, nil
	}

	createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	editedAt := time.Date(2025, 6, 7, 8, 9, 10, 0, time.UTC)
	loadPageMetadataFn = func(_ context.Context, _ *Context, pageID string) *api.PageMetadata {
		if pageID != "page-123" {
			t.Fatalf("unexpected page id %q", pageID)
		}
		return &api.PageMetadata{
			CreatedTime:    createdAt,
			LastEditedTime: editedAt,
			CreatedBy:      api.PartialUser{ID: "user-creator"},
			LastEditedBy:   api.PartialUser{ID: "user-editor"},
			Archived:       true,
			Icon:           &api.PageIcon{Type: "emoji", Emoji: "📝"},
			Parent:         api.PageParent{Type: "database_id", DatabaseID: "db-parent"},
		}
	}

	var captured output.Page
	printViewedPageFn = func(page output.Page, _ []output.Comment, asJSON bool) error {
		if !asJSON {
			t.Fatalf("expected JSON rendering")
		}
		captured = page
		return nil
	}

	err := renderFetchedPageView(context.Background(), &Context{JSON: true}, nil, "page-123", &mcp.FetchResult{Content: "page body"}, false, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !captured.CreatedTime.Equal(createdAt) {
		t.Fatalf("CreatedTime = %v, want %v", captured.CreatedTime, createdAt)
	}
	if !captured.LastEditedTime.Equal(editedAt) {
		t.Fatalf("LastEditedTime = %v, want %v", captured.LastEditedTime, editedAt)
	}
	if captured.CreatedBy != "user-creator" {
		t.Fatalf("CreatedBy = %q, want user-creator", captured.CreatedBy)
	}
	if captured.LastEditedBy != "user-editor" {
		t.Fatalf("LastEditedBy = %q, want user-editor", captured.LastEditedBy)
	}
	if !captured.Archived {
		t.Fatalf("Archived = false, want true")
	}
	if captured.Icon != "📝" {
		t.Fatalf("Icon = %q, want 📝", captured.Icon)
	}
	if captured.ParentType != "database" || captured.ParentID != "db-parent" {
		t.Fatalf("parent = (%q,%q), want (database,db-parent)", captured.ParentType, captured.ParentID)
	}
	if captured.Content != "page body" {
		t.Fatalf("Content not preserved from MCP, got %q", captured.Content)
	}
}

func TestRenderFetchedPageViewJSONFallsBackWhenNoAPIToken(t *testing.T) {
	originalLoad := loadPageViewCommentsFn
	originalPrintViewedPage := printViewedPageFn
	originalLoadMetadata := loadPageMetadataFn
	defer func() {
		loadPageViewCommentsFn = originalLoad
		printViewedPageFn = originalPrintViewedPage
		loadPageMetadataFn = originalLoadMetadata
	}()

	loadPageViewCommentsFn = func(_ context.Context, _ *mcp.Client, _ string, _ string, _ bool, _ bool, _ bool) ([]output.Comment, error) {
		return nil, nil
	}

	loadPageMetadataFn = func(_ context.Context, _ *Context, _ string) *api.PageMetadata {
		return nil
	}

	var captured output.Page
	printViewedPageFn = func(page output.Page, _ []output.Comment, _ bool) error {
		captured = page
		return nil
	}

	err := renderFetchedPageView(context.Background(), &Context{JSON: true}, nil, "page-123", &mcp.FetchResult{Content: "page body", Title: "Hello", URL: "https://notion.so/x"}, false, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !captured.CreatedTime.IsZero() || !captured.LastEditedTime.IsZero() {
		t.Fatalf("expected metadata to be empty when API unavailable, got created=%v edited=%v", captured.CreatedTime, captured.LastEditedTime)
	}
	if captured.Title != "Hello" || captured.URL != "https://notion.so/x" || captured.Content != "page body" {
		t.Fatalf("MCP-sourced fields not preserved: %+v", captured)
	}
}

func TestRenderFetchedPageViewTextModeSkipsMetadataFetch(t *testing.T) {
	originalLoad := loadPageViewCommentsFn
	originalPrintViewedPage := printViewedPageFn
	originalLoadMetadata := loadPageMetadataFn
	defer func() {
		loadPageViewCommentsFn = originalLoad
		printViewedPageFn = originalPrintViewedPage
		loadPageMetadataFn = originalLoadMetadata
	}()

	loadPageViewCommentsFn = func(_ context.Context, _ *mcp.Client, _ string, _ string, _ bool, _ bool, _ bool) ([]output.Comment, error) {
		return nil, nil
	}

	metadataCalls := 0
	loadPageMetadataFn = func(_ context.Context, _ *Context, _ string) *api.PageMetadata {
		metadataCalls++
		return nil
	}

	printViewedPageFn = func(_ output.Page, _ []output.Comment, _ bool) error {
		return nil
	}

	err := renderFetchedPageView(context.Background(), &Context{JSON: false}, nil, "page-123", &mcp.FetchResult{Content: "page body"}, false, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if metadataCalls != 0 {
		t.Fatalf("metadata should not be fetched in text mode, got %d calls", metadataCalls)
	}
}

func TestApplyPageMetadataHandlesParentVariants(t *testing.T) {
	tests := []struct {
		name     string
		parent   api.PageParent
		wantType string
		wantID   string
	}{
		{"page parent", api.PageParent{Type: "page_id", PageID: "p1"}, "page", "p1"},
		{"database parent", api.PageParent{Type: "database_id", DatabaseID: "d1"}, "database", "d1"},
		{"block parent", api.PageParent{Type: "block_id", BlockID: "b1"}, "block", "b1"},
		{"workspace parent", api.PageParent{Type: "workspace", Workspace: true}, "workspace", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotID := parentFromMetadata(tc.parent)
			if gotType != tc.wantType || gotID != tc.wantID {
				t.Fatalf("parent = (%q,%q), want (%q,%q)", gotType, gotID, tc.wantType, tc.wantID)
			}
		})
	}
}

func TestIconFromMetadataSupportsAllIconTypes(t *testing.T) {
	emoji := &api.PageIcon{Type: "emoji", Emoji: "🚀"}
	if got := iconFromMetadata(emoji); got != "🚀" {
		t.Fatalf("emoji icon = %q, want 🚀", got)
	}

	ext := &api.PageIcon{Type: "external"}
	ext.External = &struct {
		URL string `json:"url"`
	}{URL: "https://example.com/icon.png"}
	if got := iconFromMetadata(ext); got != "https://example.com/icon.png" {
		t.Fatalf("external icon = %q", got)
	}

	file := &api.PageIcon{Type: "file"}
	file.File = &struct {
		URL string `json:"url"`
	}{URL: "https://notion.so/file.png"}
	if got := iconFromMetadata(file); got != "https://notion.so/file.png" {
		t.Fatalf("file icon = %q", got)
	}

	if got := iconFromMetadata(nil); got != "" {
		t.Fatalf("nil icon = %q, want empty", got)
	}
}
