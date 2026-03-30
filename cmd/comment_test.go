package cmd

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

func TestBuildCommentListRequest(t *testing.T) {
	tests := []struct {
		name            string
		includeResolved bool
		want            mcp.GetCommentsRequest
	}{
		{
			name:            "includes block discussions by default",
			includeResolved: false,
			want: mcp.GetCommentsRequest{
				PageID:           "page-123",
				IncludeAllBlocks: true,
			},
		},
		{
			name:            "can include resolved discussions",
			includeResolved: true,
			want: mcp.GetCommentsRequest{
				PageID:           "page-123",
				IncludeAllBlocks: true,
				IncludeResolved:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCommentListRequest("page-123", tt.includeResolved)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected request\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}

type stubCommentsGetter struct {
	responses []*mcp.CommentsResponse
	requests  []mcp.GetCommentsRequest
}

func (s *stubCommentsGetter) GetComments(_ context.Context, req mcp.GetCommentsRequest) (*mcp.CommentsResponse, error) {
	s.requests = append(s.requests, req)
	if len(s.responses) == 0 {
		return nil, errors.New("unexpected GetComments call")
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func TestLoadAllCommentsPaginates(t *testing.T) {
	getter := &stubCommentsGetter{
		responses: []*mcp.CommentsResponse{
			{
				Comments:   []mcp.Comment{{ID: "comment-1"}},
				HasMore:    true,
				NextCursor: "cursor-1",
			},
			{
				Comments: []mcp.Comment{{ID: "comment-2"}},
			},
		},
	}

	comments, err := loadAllComments(context.Background(), getter, mcp.GetCommentsRequest{PageID: "page-123", IncludeAllBlocks: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].ID != "comment-1" || comments[1].ID != "comment-2" {
		t.Fatalf("unexpected comments: %#v", comments)
	}
	if len(getter.requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(getter.requests))
	}
	if getter.requests[0].Cursor != "" {
		t.Fatalf("expected first request without cursor, got %q", getter.requests[0].Cursor)
	}
	if getter.requests[1].Cursor != "cursor-1" {
		t.Fatalf("expected second request to use next cursor, got %q", getter.requests[1].Cursor)
	}
}

func TestConvertCommentsIncludesContext(t *testing.T) {
	comments := convertComments([]mcp.Comment{{
		ID:           "comment-1",
		DiscussionID: "discussion-1",
		Context:      "Security, Legal, and Risk",
		Resolved:     true,
		CreatedBy:    mcp.UserRef{ID: "user-123"},
		RichText:     []mcp.RichText{{PlainText: "Hello world"}},
	}})

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Context != "Security, Legal, and Risk" {
		t.Fatalf("expected context %q, got %q", "Security, Legal, and Risk", comments[0].Context)
	}
	if !comments[0].Resolved {
		t.Fatalf("expected resolved flag to be set")
	}
}

func TestHydrateCommentContextsFromPageContent(t *testing.T) {
	comments := []output.Comment{
		{DiscussionID: "discussion://page/block/discussion-1", Context: "Existing c...r example:"},
		{DiscussionID: "discussion://page/block/discussion-2", Context: "Buildkite ... of work."},
	}

	pageContent := `<page><content><p><span discussion-urls="discussion://discussion-1">Existing company controls remain the source of truth for regulated work.</span></p><p><span discussion-urls="discussion://discussion-2,discussion://discussion-3">Buildkite Personnel may adopt and trial new AI tools in support of work.</span></p></content></page>`

	hydrateCommentContextsFromPageContent(pageContent, comments)

	if comments[0].Context != "Existing company controls remain the source of truth for regulated work." {
		t.Fatalf("unexpected context for first discussion: %q", comments[0].Context)
	}
	if comments[1].Context != "Buildkite Personnel may adopt and trial new AI tools in support of work." {
		t.Fatalf("unexpected context for second discussion: %q", comments[1].Context)
	}
}

func TestCanonicalDiscussionID(t *testing.T) {
	if got := canonicalDiscussionID("discussion://32bb8dbc/page-block/discussion-1"); got != "discussion://discussion-1" {
		t.Fatalf("unexpected canonical discussion id: %q", got)
	}
	if got := canonicalDiscussionID("discussion://discussion-1"); got != "discussion://discussion-1" {
		t.Fatalf("unexpected passthrough discussion id: %q", got)
	}
}

func TestResolveCommentPageID(t *testing.T) {
	resolver := func(_ context.Context, _ *mcp.Client, _ string) (string, error) {
		return "resolved-by-name", nil
	}

	tests := []struct {
		name     string
		page     string
		wantID   string
		wantErr  bool
		errMatch string
		resolver pageIDResolver
	}{
		{
			name:   "url resolves to canonical id without resolver",
			page:   "https://www.notion.so/buildkite/Draft-Policy-32bb8dbc2c8981bf9406fd122784324f",
			wantID: "32bb8dbc-2c89-81bf-9406-fd122784324f",
			resolver: func(_ context.Context, _ *mcp.Client, _ string) (string, error) {
				return "", errors.New("resolver should not be called")
			},
		},
		{
			name:   "plain id resolves to canonical id without resolver",
			page:   "32bb8dbc2c8981bf9406fd122784324f",
			wantID: "32bb8dbc-2c89-81bf-9406-fd122784324f",
			resolver: func(_ context.Context, _ *mcp.Client, _ string) (string, error) {
				return "", errors.New("resolver should not be called")
			},
		},
		{
			name:     "name resolves via resolver",
			page:     "Draft Buildkite Artificial Intelligence (AI) Usage Policy",
			wantID:   "resolved-by-name",
			resolver: resolver,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCommentPageID(context.Background(), tt.page, nil, tt.resolver)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if tt.errMatch != "" && err.Error() != tt.errMatch {
					t.Fatalf("expected error %q, got %q", tt.errMatch, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got != tt.wantID {
				t.Fatalf("resolveCommentPageID() = %q, want %q", got, tt.wantID)
			}
		})
	}
}
