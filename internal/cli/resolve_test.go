package cli

import (
	"testing"
)

func TestParsePageRef(t *testing.T) {
	tests := []struct {
		input    string
		wantKind PageRefKind
		wantID   string
	}{
		// Plain UUIDs
		{"12345678abcdef1234567890abcdef12", RefID, "12345678-abcd-ef12-3456-7890abcdef12"},
		{"12345678-abcd-ef12-3456-7890abcdef12", RefID, "12345678-abcd-ef12-3456-7890abcdef12"},

		// Uppercase hex
		{"12345678ABCDEF1234567890ABCDEF12", RefID, "12345678-abcd-ef12-3456-7890abcdef12"},

		// Notion URLs
		{"https://www.notion.so/My-Page-12345678abcdef1234567890abcdef12", RefID, "12345678-abcd-ef12-3456-7890abcdef12"},
		{"https://notion.so/workspace/Page-Title-12345678abcdef1234567890abcdef12?v=abc", RefID, "12345678-abcd-ef12-3456-7890abcdef12"},

		// URL without extractable ID
		{"https://example.com/some-page", RefURL, ""},

		// Names
		{"Meeting Notes", RefName, ""},
		{"Engineering", RefName, ""},
		{"short", RefName, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref := ParsePageRef(tt.input)
			if ref.Kind != tt.wantKind {
				t.Errorf("ParsePageRef(%q).Kind = %d, want %d", tt.input, ref.Kind, tt.wantKind)
			}
			if tt.wantID != "" && ref.ID != tt.wantID {
				t.Errorf("ParsePageRef(%q).ID = %q, want %q", tt.input, ref.ID, tt.wantID)
			}
		})
	}
}

func TestLooksLikeID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"12345678abcdef1234567890abcdef12", true},
		{"12345678-abcd-ef12-3456-7890abcdef12", true},
		{"ABCDEF1234567890ABCDEF1234567890", true},
		{"not-an-id", false},
		{"Meeting Notes", false},
		{"https://notion.so/page", false},
		{"12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := LooksLikeID(tt.input)
			if got != tt.want {
				t.Errorf("LooksLikeID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractNotionUUID(t *testing.T) {
	tests := []struct {
		input  string
		wantID string
		wantOK bool
	}{
		{"12345678abcdef1234567890abcdef12", "12345678-abcd-ef12-3456-7890abcdef12", true},
		{"https://www.notion.so/Page-12345678abcdef1234567890abcdef12", "12345678-abcd-ef12-3456-7890abcdef12", true},
		{"no-id-here", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			id, ok := ExtractNotionUUID(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ExtractNotionUUID(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if id != tt.wantID {
				t.Errorf("ExtractNotionUUID(%q) = %q, want %q", tt.input, id, tt.wantID)
			}
		})
	}
}
