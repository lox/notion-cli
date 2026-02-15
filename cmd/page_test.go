package cmd

import "testing"

func TestExtractDataSourceID(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "typical database fetch response",
			content: `{"url":"collection://308b8dbc-2c89-80da-9070-000be5b57575"}`,
			want:    "308b8dbc-2c89-80da-9070-000be5b57575",
		},
		{
			name:    "collection URL in prose",
			content: `CREATE TABLE "collection://aaaabbbb-cccc-dddd-eeee-ffffffffffff" (url TEXT)`,
			want:    "aaaabbbb-cccc-dddd-eeee-ffffffffffff",
		},
		{
			name:    "no collection URL",
			content: `{"title":"Some Page","text":"hello"}`,
			want:    "",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "collection prefix but no valid UUID",
			content: `collection://not-a-uuid`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDataSourceID(tt.content)
			if got != tt.want {
				t.Errorf("extractDataSourceID() = %q, want %q", got, tt.want)
			}
		})
	}
}
