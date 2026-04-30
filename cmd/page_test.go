package cmd

import (
	"testing"
)

func TestStripLocalImagesRemovesStandaloneLocalImageLines(t *testing.T) {
	markdown := "# Title\n\n![Local](./diagram.png)\n\nParagraph\n"

	got, err := stripLocalImages(markdown)
	if err != nil {
		t.Fatalf("stripLocalImages: %v", err)
	}

	want := "# Title\n\n\n\nParagraph\n"
	if got != want {
		t.Fatalf("stripLocalImages() = %q, want %q", got, want)
	}
}

func TestStripLocalImagesPreservesRemoteImages(t *testing.T) {
	markdown := "![Remote](https://example.com/remote.png)\n![Local](./diagram.png)\n"

	got, err := stripLocalImages(markdown)
	if err != nil {
		t.Fatalf("stripLocalImages: %v", err)
	}

	want := "![Remote](https://example.com/remote.png)\n\n"
	if got != want {
		t.Fatalf("stripLocalImages() = %q, want %q", got, want)
	}
}

func TestStripLocalImagesStripsMissingLocalFiles(t *testing.T) {
	markdown := "# Title\n\n![Local](./does-not-exist.png)\n\nParagraph\n"

	got, err := stripLocalImages(markdown)
	if err != nil {
		t.Fatalf("stripLocalImages: %v", err)
	}

	want := "# Title\n\n\n\nParagraph\n"
	if got != want {
		t.Fatalf("stripLocalImages() = %q, want %q", got, want)
	}
}
