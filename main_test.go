package main

import "testing"

func TestShouldPrintVersionAndExit(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "no args", args: nil, want: false},
		{name: "long version top level", args: []string{"--version"}, want: true},
		{name: "short version top level", args: []string{"-v"}, want: true},
		{name: "version subcommand", args: []string{"version"}, want: false},
		{name: "subcommand with version literal value", args: []string{"page", "edit", "p", "--find", "x", "--replace-with", "--version"}, want: false},
		{name: "subcommand with short value", args: []string{"page", "edit", "p", "--find", "x", "--replace-with", "-v"}, want: false},
		{name: "help top level", args: []string{"--help"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldPrintVersionAndExit(tt.args)
			if got != tt.want {
				t.Fatalf("shouldPrintVersionAndExit(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
