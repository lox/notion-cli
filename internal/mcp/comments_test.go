package mcp

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildGetCommentsToolArgs(t *testing.T) {
	req := GetCommentsRequest{
		PageID:           "page-123",
		Cursor:           "cursor-1",
		PageSize:         50,
		IncludeAllBlocks: true,
		IncludeResolved:  true,
		DiscussionID:     "discussion://page/block/discussion",
	}

	got := buildGetCommentsToolArgs(req)
	want := map[string]any{
		"page_id":            "page-123",
		"cursor":             "cursor-1",
		"page_size":          50,
		"include_all_blocks": true,
		"include_resolved":   true,
		"discussion_id":      "discussion://page/block/discussion",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildFetchToolArgs(t *testing.T) {
	tests := []struct {
		name               string
		includeDiscussions bool
		want               map[string]any
	}{
		{
			name:               "basic fetch",
			includeDiscussions: false,
			want: map[string]any{
				"id": "page-123",
			},
		},
		{
			name:               "fetch with discussion markers",
			includeDiscussions: true,
			want: map[string]any{
				"id":                  "page-123",
				"include_discussions": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFetchToolArgs("page-123", tt.includeDiscussions)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}

func TestParseCommentsResponseXMLWrapper(t *testing.T) {
	raw := `{"text":"<discussions total-count=\"1\" shown-count=\"1\"><discussion id=\"discussion://page/block/discussion\" comment-count=\"1\" resolved=\"false\" type=\"comment\" context=\"inline\"><comment id=\"comment-1\" url=\"https://example.com\" user-url=\"user://user-123\" datetime=\"2026-03-29T22:31:40.086Z\">Hello &amp; goodbye</comment></discussion></discussions>"}`

	resp, err := parseCommentsResponse(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(resp.Comments))
	}

	comment := resp.Comments[0]
	if comment.ID != "comment-1" {
		t.Fatalf("expected comment ID %q, got %q", "comment-1", comment.ID)
	}
	if comment.DiscussionID != "discussion://page/block/discussion" {
		t.Fatalf("expected discussion ID %q, got %q", "discussion://page/block/discussion", comment.DiscussionID)
	}
	if comment.Context != "" {
		t.Fatalf("expected empty context, got %q", comment.Context)
	}
	if comment.CreatedBy.ID != "user-123" {
		t.Fatalf("expected author ID %q, got %q", "user-123", comment.CreatedBy.ID)
	}
	if got := comment.RichText[0].PlainText; got != "Hello & goodbye" {
		t.Fatalf("expected comment text %q, got %q", "Hello & goodbye", got)
	}

	wantTime := time.Date(2026, time.March, 29, 22, 31, 40, 86_000_000, time.UTC)
	if !comment.CreatedTime.Equal(wantTime) {
		t.Fatalf("expected created time %s, got %s", wantTime, comment.CreatedTime)
	}
}

func TestParseCommentsResponseXMLWrapperWithBareAmpersands(t *testing.T) {
	raw := `{"text":"<discussions total-count=\"1\" shown-count=\"1\"><discussion id=\"discussion://page/block/discussion\" comment-count=\"1\" resolved=\"false\" type=\"comment\" context=\"inline\"><comment id=\"comment-1\" url=\"https://example.com?d=discussion&pvs=42\" user-url=\"user://user-123\" datetime=\"2026-03-29T22:31:40.086Z\">URL has bare ampersands</comment></discussion></discussions>"}`

	resp, err := parseCommentsResponse(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(resp.Comments))
	}
}

func TestParseCommentsResponseXMLWrapperWithContext(t *testing.T) {
	raw := `{"text":"<discussions total-count=\"1\" shown-count=\"1\"><discussion id=\"discussion://page/block/discussion\" comment-count=\"1\" resolved=\"true\" type=\"comment\" context=\"inline\" text-context=\"Security, Legal, and Risk\"><comment id=\"comment-1\" user-url=\"user://user-123\" datetime=\"2026-03-29T22:31:40.086Z\">Hello</comment></discussion></discussions>"}`

	resp, err := parseCommentsResponse(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	comment := resp.Comments[0]
	if comment.Context != "Security, Legal, and Risk" {
		t.Fatalf("expected context %q, got %q", "Security, Legal, and Risk", comment.Context)
	}
	if !comment.Resolved {
		t.Fatalf("expected resolved comment")
	}
}

func TestParseCommentsResponseXMLWrapperWithUnknownNamedEntity(t *testing.T) {
	raw := `{"text":"<discussions total-count=\"1\" shown-count=\"1\"><discussion id=\"discussion://page/block/discussion\" comment-count=\"1\" resolved=\"false\" type=\"comment\" context=\"inline\"><comment id=\"comment-1\" user-url=\"user://user-123\" datetime=\"2026-03-29T22:31:40.086Z\">Hello &nbsp; goodbye</comment></discussion></discussions>"}`

	resp, err := parseCommentsResponse(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(resp.Comments))
	}
	if got := resp.Comments[0].RichText[0].PlainText; got != "Hello &nbsp; goodbye" {
		t.Fatalf("expected literal unknown entity text, got %q", got)
	}
}

func TestParseCommentsResponseEmptyObject(t *testing.T) {
	resp, err := parseCommentsResponse(`{}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Comments) != 0 {
		t.Fatalf("expected no comments, got %d", len(resp.Comments))
	}
}
