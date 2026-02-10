package cli

import (
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantID   string
		wantBody string
	}{
		{
			name:     "no frontmatter",
			input:    "# Hello\n\nWorld",
			wantID:   "",
			wantBody: "# Hello\n\nWorld",
		},
		{
			name:     "with notion-id",
			input:    "---\nnotion-id: abc123\n---\n\n# Hello\n\nWorld",
			wantID:   "abc123",
			wantBody: "# Hello\n\nWorld",
		},
		{
			name:     "with other fields",
			input:    "---\ntitle: My Page\nnotion-id: def456\ntags: test\n---\n\n# Hello",
			wantID:   "def456",
			wantBody: "# Hello",
		},
		{
			name:     "empty frontmatter",
			input:    "---\n---\n\n# Hello",
			wantID:   "",
			wantBody: "# Hello",
		},
		{
			name:     "no closing delimiter",
			input:    "---\nnotion-id: abc\n# Hello",
			wantID:   "",
			wantBody: "---\nnotion-id: abc\n# Hello",
		},
		{
			name:     "triple dash in code block is not frontmatter",
			input:    "Some text\n---\nnotion-id: abc\n---\n",
			wantID:   "",
			wantBody: "Some text\n---\nnotion-id: abc\n---\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := ParseFrontmatter(tt.input)
			if fm.NotionID != tt.wantID {
				t.Errorf("NotionID = %q, want %q", fm.NotionID, tt.wantID)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestSetFrontmatterID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		id    string
		want  string
	}{
		{
			name:  "add to file without frontmatter",
			input: "# Hello\n\nWorld",
			id:    "abc123",
			want:  "---\nnotion-id: abc123\n---\n\n# Hello\n\nWorld",
		},
		{
			name:  "update existing notion-id",
			input: "---\nnotion-id: old-id\n---\n\n# Hello",
			id:    "new-id",
			want:  "---\nnotion-id: new-id\n---\n\n# Hello",
		},
		{
			name:  "add to existing frontmatter without notion-id",
			input: "---\ntitle: My Page\n---\n\n# Hello",
			id:    "abc123",
			want:  "---\ntitle: My Page\nnotion-id: abc123\n---\n\n# Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetFrontmatterID(tt.input, tt.id)
			if got != tt.want {
				t.Errorf("SetFrontmatterID():\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}
