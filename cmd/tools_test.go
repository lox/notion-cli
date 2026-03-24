package cmd

import (
	"bytes"
	"testing"
)

func TestPrintToolsText(t *testing.T) {
	var buf bytes.Buffer
	err := printTools(&buf, []toolSummary{
		{Name: "notion-search", Description: "Search Notion content"},
		{Name: "notion-fetch", Description: "Fetch a page or database"},
	}, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "notion-search\n  Search Notion content\n\nnotion-fetch\n  Fetch a page or database\n\n"
	if buf.String() != want {
		t.Fatalf("printTools() text output mismatch\nwant:\n%q\ngot:\n%q", want, buf.String())
	}
}

func TestPrintToolsJSON(t *testing.T) {
	var buf bytes.Buffer
	err := printTools(&buf, []toolSummary{
		{Name: "notion-search", Description: "Search Notion content"},
	}, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := "[\n  {\n    \"name\": \"notion-search\",\n    \"description\": \"Search Notion content\"\n  }\n]\n"
	if buf.String() != want {
		t.Fatalf("printTools() JSON output mismatch\nwant:\n%q\ngot:\n%q", want, buf.String())
	}
}
