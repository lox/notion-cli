package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintPageViewJSONOmitsCommentsWhenEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := printPageViewJSON(&buf, Page{ID: "page-123", Title: "Example"}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.Contains(buf.String(), `"Comments"`) {
		t.Fatalf("expected comments to be omitted, got %q", buf.String())
	}
}

func TestPrintPageViewJSONIncludesCommentsWhenRequested(t *testing.T) {
	var buf bytes.Buffer
	err := printPageViewJSON(&buf, Page{ID: "page-123", Title: "Example"}, []Comment{{CreatedByName: "Person Example", Content: "Hello"}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(buf.String(), `"Comments"`) {
		t.Fatalf("expected comments field, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), `Person Example`) {
		t.Fatalf("expected comment author in JSON, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), `"Content": "Hello"`) {
		t.Fatalf("expected comment content in JSON, got %q", buf.String())
	}
}
