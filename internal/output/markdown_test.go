package output

import (
	"testing"
)

func TestParseNotionResponse_Ancestors(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantTitle     string
		wantAncestors []ancestor
	}{
		{
			name: "page with ancestors",
			content: `<page url="{{https://www.notion.so/abc123}}">
<ancestor-path>
<parent-page url="https://www.notion.so/parent1" title="Pipelines"/>
<ancestor-2-page url="https://www.notion.so/grandparent1" title="Teams"/>
</ancestor-path>
<properties>
{"title":"My Page"}
</properties>
<content>Hello world</content>`,
			wantTitle: "My Page",
			wantAncestors: []ancestor{
				{Title: "Teams", URL: "https://www.notion.so/grandparent1"},
				{Title: "Pipelines", URL: "https://www.notion.so/parent1"},
			},
		},
		{
			name: "page with single parent",
			content: `<page url="{{https://www.notion.so/abc123}}">
<ancestor-path>
<parent-page url="https://www.notion.so/parent1" title="Engineering"/>
</ancestor-path>
<properties>
{"title":"Docs"}
</properties>
<content>Content here</content>`,
			wantTitle: "Docs",
			wantAncestors: []ancestor{
				{Title: "Engineering", URL: "https://www.notion.so/parent1"},
			},
		},
		{
			name: "page without ancestors",
			content: `<page url="{{https://www.notion.so/abc123}}">
<properties>
{"title":"Top Level"}
</properties>
<content>Content here</content>`,
			wantTitle:     "Top Level",
			wantAncestors: nil,
		},
		{
			name: "deeply nested page",
			content: `<page url="{{https://www.notion.so/abc123}}">
<ancestor-path>
<parent-page url="https://www.notion.so/p1" title="Parent"/>
<ancestor-2-page url="https://www.notion.so/p2" title="Grandparent"/>
<ancestor-3-page url="https://www.notion.so/p3" title="Root"/>
</ancestor-path>
<properties>
{"title":"Leaf"}
</properties>
<content>Deep content</content>`,
			wantTitle: "Leaf",
			wantAncestors: []ancestor{
				{Title: "Root", URL: "https://www.notion.so/p3"},
				{Title: "Grandparent", URL: "https://www.notion.so/p2"},
				{Title: "Parent", URL: "https://www.notion.so/p1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, _ := parseNotionResponse(tt.content)

			if meta.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", meta.Title, tt.wantTitle)
			}

			if len(meta.Ancestors) != len(tt.wantAncestors) {
				t.Fatalf("Ancestors count = %d, want %d", len(meta.Ancestors), len(tt.wantAncestors))
			}

			for i, want := range tt.wantAncestors {
				got := meta.Ancestors[i]
				if got.Title != want.Title {
					t.Errorf("Ancestors[%d].Title = %q, want %q", i, got.Title, want.Title)
				}
				if got.URL != want.URL {
					t.Errorf("Ancestors[%d].URL = %q, want %q", i, got.URL, want.URL)
				}
			}
		})
	}
}
