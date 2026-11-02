package cmd

import (
	"strings"
	"testing"

	"github.com/lox/notion-cli/internal/mcp"
)

func TestRunPageMoveRequiresExactlyOneParent(t *testing.T) {
	err := runPageMove(&Context{}, "My Page", "", "")
	if err == nil {
		t.Fatal("expected error when no parent is given")
	}
	if !strings.Contains(err.Error(), "--parent") {
		t.Fatalf("error = %q", err.Error())
	}

	err = runPageMove(&Context{}, "My Page", "Parent Page", "Some DB")
	if err == nil {
		t.Fatal("expected error when both parents are given")
	}
	if !strings.Contains(err.Error(), "--parent") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestDuplicatedPageRecoversIDFromURL(t *testing.T) {
	page := duplicatedPage(&mcp.DuplicatePageResponse{
		URL: "https://www.notion.so/workspace/Copy-12345678abcdef1234567890abcdef12",
	})

	if page.ID != "12345678-abcd-ef12-3456-7890abcdef12" {
		t.Fatalf("id = %q", page.ID)
	}
}

func TestDuplicatedPageKeepsServerID(t *testing.T) {
	page := duplicatedPage(&mcp.DuplicatePageResponse{ID: "page-2", URL: "https://www.notion.so/page-2"})

	if page.ID != "page-2" {
		t.Fatalf("id = %q", page.ID)
	}
}
