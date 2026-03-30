package cmd

import "testing"

func TestShouldLoadPageViewComments(t *testing.T) {
	tests := []struct {
		name            string
		raw             bool
		includeComments bool
		asJSON          bool
		want            bool
	}{
		{
			name:            "plain view loads comments by default",
			raw:             false,
			includeComments: true,
			asJSON:          false,
			want:            true,
		},
		{
			name:            "comments can be disabled",
			raw:             false,
			includeComments: false,
			asJSON:          false,
			want:            false,
		},
		{
			name:            "raw view skips comments",
			raw:             true,
			includeComments: true,
			asJSON:          false,
			want:            false,
		},
		{
			name:            "json keeps comments enabled even with raw output",
			raw:             true,
			includeComments: true,
			asJSON:          true,
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldLoadPageViewComments(tt.raw, tt.includeComments, tt.asJSON)
			if got != tt.want {
				t.Fatalf("shouldLoadPageViewComments() = %v, want %v", got, tt.want)
			}
		})
	}
}
