package cmd

import "testing"

func TestShouldLoadPageViewComments(t *testing.T) {
	tests := []struct {
		name            string
		raw             bool
		includeComments bool
		want            bool
	}{
		{
			name:            "plain view loads comments by default",
			raw:             false,
			includeComments: true,
			want:            true,
		},
		{
			name:            "comments can be disabled",
			raw:             false,
			includeComments: false,
			want:            false,
		},
		{
			name:            "raw view skips comments",
			raw:             true,
			includeComments: true,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldLoadPageViewComments(tt.raw, tt.includeComments)
			if got != tt.want {
				t.Fatalf("shouldLoadPageViewComments() = %v, want %v", got, tt.want)
			}
		})
	}
}
