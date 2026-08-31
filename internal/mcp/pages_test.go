package mcp

import "testing"

func TestBuildMovePageToolArgsPageParent(t *testing.T) {
	args, err := buildMovePageToolArgs(MovePageRequest{PageID: "page-1", ParentPageID: "parent-1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ids, ok := args["page_or_database_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "page-1" {
		t.Fatalf("expected page_or_database_ids [page-1], got %#v", args["page_or_database_ids"])
	}

	parent, ok := args["new_parent"].(map[string]any)
	if !ok || parent["page_id"] != "parent-1" {
		t.Fatalf("expected new_parent page_id parent-1, got %#v", args["new_parent"])
	}
	if _, exists := parent["data_source_id"]; exists {
		t.Fatalf("expected no data_source_id, got %#v", parent)
	}
}

func TestBuildMovePageToolArgsDatabaseParent(t *testing.T) {
	args, err := buildMovePageToolArgs(MovePageRequest{PageID: "page-1", ParentDatabaseID: "ds-1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	parent, ok := args["new_parent"].(map[string]any)
	if !ok || parent["data_source_id"] != "ds-1" {
		t.Fatalf("expected new_parent data_source_id ds-1, got %#v", args["new_parent"])
	}
}

func TestBuildMovePageToolArgsRejectsAmbiguousParent(t *testing.T) {
	if _, err := buildMovePageToolArgs(MovePageRequest{PageID: "page-1"}); err == nil {
		t.Fatal("expected an error when no parent is given")
	}

	req := MovePageRequest{PageID: "page-1", ParentPageID: "parent-1", ParentDatabaseID: "ds-1"}
	if _, err := buildMovePageToolArgs(req); err == nil {
		t.Fatal("expected an error when both parents are given")
	}
}

func TestParseDuplicatePageResponseJSON(t *testing.T) {
	resp := parseDuplicatePageResponse(`{"page_id":"page-2","page_url":"https://www.notion.so/page-2"}`)

	if resp.ID != "page-2" {
		t.Fatalf("expected id page-2, got %q", resp.ID)
	}
	if resp.URL != "https://www.notion.so/page-2" {
		t.Fatalf("expected url https://www.notion.so/page-2, got %q", resp.URL)
	}
}

func TestParseDuplicatePageResponseFallsBackToURLInText(t *testing.T) {
	resp := parseDuplicatePageResponse("Duplicated page https://www.notion.so/page-2 successfully")

	if resp.URL != "https://www.notion.so/page-2" {
		t.Fatalf("expected url https://www.notion.so/page-2, got %q", resp.URL)
	}
}

func TestParseCreatePageResponseReadsPagesArray(t *testing.T) {
	raw := `{"pages":[{"id":"page-1","url":"https://www.notion.so/page-1","properties":{"title":"T"}}]}`

	resp := parseCreatePageResponse(raw)

	if resp.ID != "page-1" {
		t.Fatalf("id = %q", resp.ID)
	}
	if resp.URL != "https://www.notion.so/page-1" {
		t.Fatalf("url = %q", resp.URL)
	}
}

func TestParseCreatePageResponseReadsFlatObject(t *testing.T) {
	resp := parseCreatePageResponse(`{"id":"page-1","url":"https://www.notion.so/page-1"}`)

	if resp.ID != "page-1" || resp.URL != "https://www.notion.so/page-1" {
		t.Fatalf("resp = %#v", resp)
	}
}

func TestExtractURLFromTextAcceptsBothNotionHosts(t *testing.T) {
	cases := map[string]string{
		"see https://app.notion.com/p/abc?pvs=204 now": "https://app.notion.com/p/abc?pvs=204",
		`{"url":"https://www.notion.so/abc"}`:          "https://www.notion.so/abc",
		"no link here":                                 "",
	}

	for text, want := range cases {
		if got := extractURLFromText(text); got != want {
			t.Fatalf("extractURLFromText(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestNextUsersCursor(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "has more",
			text: `{"results":[],"has_more":true,"next_cursor":"cursor-2"}`,
			want: "cursor-2",
		},
		{
			name: "last page",
			text: `{"results":[],"has_more":false,"next_cursor":"cursor-2"}`,
			want: "",
		},
		{
			name: "prose fallback",
			text: "Found 3 users in the workspace",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextUsersCursor(tt.text); got != tt.want {
				t.Errorf("nextUsersCursor() = %q, want %q", got, tt.want)
			}
		})
	}
}
