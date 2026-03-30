package output

import "testing"

func TestCommentAuthorName(t *testing.T) {
	if got := commentAuthorName(Comment{CreatedBy: "user-123", CreatedByName: "Person Example"}); got != "Person Example" {
		t.Fatalf("expected display name, got %q", got)
	}
	if got := commentAuthorName(Comment{CreatedBy: "user-123"}); got != "user-123" {
		t.Fatalf("expected fallback author ID, got %q", got)
	}
}

func TestCommentStatusLabel(t *testing.T) {
	if got := commentStatusLabel(Comment{Resolved: true}); got != "resolved" {
		t.Fatalf("expected resolved label, got %q", got)
	}
	if got := commentStatusLabel(Comment{}); got != "" {
		t.Fatalf("expected empty label, got %q", got)
	}
}
