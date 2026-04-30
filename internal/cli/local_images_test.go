package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteStandaloneLocalImagesRewritesStandaloneLocalLines(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages("# Title\n\n![Diagram](./diagram.png)\n\nDone\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if !strings.Contains(rewritten, placements[0].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholder: %q", rewritten)
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
}

func TestRewriteStandaloneLocalImagesHandlesCRLF(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages("# Title\r\n\r\n![Diagram](./diagram.png)\r\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if !strings.Contains(rewritten, placements[0].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholder: %q", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesRejectsInlineLocalImage(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := RewriteStandaloneLocalImages("before ![Diagram](./diagram.png) after\n", doc)
	if err == nil || !strings.Contains(err.Error(), "must appear on their own line") {
		t.Fatalf("expected unsupported syntax error, got %v", err)
	}
}

func TestRewriteStandaloneLocalImagesIgnoresRemoteImages(t *testing.T) {
	doc := filepath.Join(t.TempDir(), "doc.md")

	rewritten, placements, err := RewriteStandaloneLocalImages("![Diagram](https://example.test/diagram.png)\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0", len(placements))
	}
	if rewritten != "![Diagram](https://example.test/diagram.png)\n" {
		t.Fatalf("rewritten = %q", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesAcceptsMissingFiles(t *testing.T) {
	rewritten, placements, err := FindStandaloneLocalImageLines("# Title\n\n![Diagram](./does-not-exist.png)\n\nDone\n")
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Resolved != "" {
		t.Fatalf("Resolved = %q, want empty (strip mode does not resolve paths)", placements[0].Resolved)
	}
	if placements[0].Placeholder == "" {
		t.Fatalf("Placeholder was not set")
	}
	if !strings.Contains(rewritten, placements[0].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholder: %q", rewritten)
	}
	if strings.Contains(rewritten, "does-not-exist.png") {
		t.Fatalf("rewritten markdown still contains original image line: %q", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesRejectsInlineLocalImage(t *testing.T) {
	_, _, err := FindStandaloneLocalImageLines("before ![Diagram](./diagram.png) after\n")
	if err == nil || !strings.Contains(err.Error(), "must appear on their own line") {
		t.Fatalf("expected unsupported syntax error, got %v", err)
	}
}

func TestFindStandaloneLocalImageLinesIgnoresRemoteImages(t *testing.T) {
	rewritten, placements, err := FindStandaloneLocalImageLines("![Diagram](https://example.test/diagram.png)\n")
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0", len(placements))
	}
	if rewritten != "![Diagram](https://example.test/diagram.png)\n" {
		t.Fatalf("rewritten = %q", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesSupportsBalancedParensInDestination(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram(1).png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages("![Diagram](./diagram(1).png)\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
	if !strings.Contains(rewritten, placements[0].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholder: %q", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesSupportsEscapedParensInDestination(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram(final).png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages(`![Diagram](./diagram\(final\).png)`+"\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
	if placements[0].Original != "./diagram(final).png" {
		t.Fatalf("Original = %q, want unescaped destination", placements[0].Original)
	}
	if !strings.Contains(rewritten, placements[0].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholder: %q", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesIgnoresInlineCodeSpans(t *testing.T) {
	input := "Use `![Demo](./example.png)` in docs.\n"

	rewritten, placements, err := FindStandaloneLocalImageLines(input)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0 (image inside inline code span should be ignored)", len(placements))
	}
	if rewritten != input {
		t.Fatalf("rewritten = %q, want input unchanged", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesNormalizesToColumnZero(t *testing.T) {
	// Indented standalone image lines have their leading whitespace dropped
	// so the placeholder always lands in a paragraph block after
	// replace_content. substituteUploadedLocalImages only indexes paragraph
	// blocks, so keeping the indent would let Notion nest the placeholder
	// inside the surrounding list_item/quote and break substitution.
	input := "   ![img](./a.png)\n"

	rewritten, placements, err := FindStandaloneLocalImageLines(input)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	want := placements[0].Placeholder + "\n"
	if rewritten != want {
		t.Fatalf("rewritten = %q, want %q (placeholder must sit at column 0)", rewritten, want)
	}
}

func TestFindStandaloneLocalImageLinesSkipsBacktickFencedCodeBlock(t *testing.T) {
	markdown := strings.Join([]string{
		"before",
		"",
		"```",
		"![Demo](./example.png)",
		"```",
		"",
		"after",
		"",
	}, "\n")

	rewritten, placements, err := FindStandaloneLocalImageLines(markdown)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0 (image inside fenced code block should be ignored)", len(placements))
	}
	if rewritten != markdown {
		t.Fatalf("rewritten mutated code block content:\n%s", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesSkipsTildeFencedCodeBlock(t *testing.T) {
	markdown := strings.Join([]string{
		"~~~markdown",
		"![Demo](./example.png)",
		"~~~",
		"",
	}, "\n")

	_, placements, err := FindStandaloneLocalImageLines(markdown)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0 (image inside tilde-fenced code block should be ignored)", len(placements))
	}
}

func TestFindStandaloneLocalImageLinesSkipsIndentedCodeBlock(t *testing.T) {
	markdown := strings.Join([]string{
		"before paragraph",
		"",
		"    ![Demo](./example.png)",
		"",
		"after paragraph",
		"",
	}, "\n")

	rewritten, placements, err := FindStandaloneLocalImageLines(markdown)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0 (image inside indented code block should be ignored)", len(placements))
	}
	if rewritten != markdown {
		t.Fatalf("rewritten mutated indented code block:\n%s", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesSupportsNestedBracketsInAltText(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages("![Architecture [v2]](./diagram.png)\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Alt != "Architecture [v2]" {
		t.Fatalf("Alt = %q, want \"Architecture [v2]\"", placements[0].Alt)
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
	if strings.Contains(rewritten, "./diagram.png") {
		t.Fatalf("rewritten should have replaced nested-bracket image line: %q", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesIgnoresProtocolRelativeURLs(t *testing.T) {
	rewritten, placements, err := FindStandaloneLocalImageLines("![CDN](//cdn.example.com/image.png)\n")
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0 (protocol-relative URL should be treated as remote)", len(placements))
	}
	if rewritten != "![CDN](//cdn.example.com/image.png)\n" {
		t.Fatalf("rewritten = %q, want input unchanged", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesSupportsAngleBracketDestinationWithTitle(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram 1.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages(`![Alt](<./diagram 1.png> "caption")`+"\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
	if strings.Contains(rewritten, "./diagram 1.png") {
		t.Fatalf("rewritten should have replaced angle-bracket image line: %q", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesHandlesEscapedGTInAngleBrackets(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "foo>bar.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// CommonMark allows `\>` inside angle-bracket destinations.
	rewritten, placements, err := RewriteStandaloneLocalImages(`![Alt](<./foo\>bar.png>)`+"\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
	if strings.Contains(rewritten, `foo\>bar.png`) {
		t.Fatalf("rewritten should have replaced image line, got %q", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesIgnoresEscapedImageMarker(t *testing.T) {
	input := `\![Demo](./example.png)` + "\n"
	rewritten, placements, err := FindStandaloneLocalImageLines(input)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0 (escaped ! should be literal text)", len(placements))
	}
	if rewritten != input {
		t.Fatalf("rewritten = %q, want input unchanged", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesTreatsImageOutsideFenceAsStandalone(t *testing.T) {
	markdown := strings.Join([]string{
		"```",
		"fenced example",
		"```",
		"",
		"![Real](./diagram.png)",
		"",
	}, "\n")

	rewritten, placements, err := FindStandaloneLocalImageLines(markdown)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if strings.Contains(rewritten, "./diagram.png") {
		t.Fatalf("rewritten should have replaced post-fence image line with a placeholder: %q", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesDecodesEscapedSpaceInDestination(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram 1.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages(`![Alt](./diagram\ 1.png)`+"\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
	if strings.Contains(rewritten, `diagram\ 1.png`) {
		t.Fatalf("rewritten still contains escaped destination: %q", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesHandlesParenInTitle(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "img.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// The `)` inside the title must not terminate the image span.
	rewritten, placements, err := RewriteStandaloneLocalImages(`![Alt](./img.png "v1) draft")`+"\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1; rewritten=%q", len(placements), rewritten)
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
}

func TestFindStandaloneLocalImageLinesIgnoresQuotedFencedCodeBlock(t *testing.T) {
	// Image syntax inside a quoted fenced code block must not be
	// scanned; it's documentation, not an upload target.
	markdown := strings.Join([]string{
		"> ```md",
		"> ![Demo](./img.png)",
		"> ```",
		"",
		"![Real](./real.png)",
		"",
	}, "\n")

	rewritten, placements, err := FindStandaloneLocalImageLines(markdown)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1 (quoted fence contents should be ignored)", len(placements))
	}
	if !strings.Contains(rewritten, "> ![Demo](./img.png)") {
		t.Fatalf("rewritten lost the quoted example: %q", rewritten)
	}
	if strings.Contains(rewritten, "./real.png") {
		t.Fatalf("rewritten should have replaced the real image line: %q", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesIgnoresBlockquotedImage(t *testing.T) {
	// A blockquoted image line isn't standalone; the `>` marker means
	// the user is documenting rather than uploading.
	markdown := "> ![Quoted](./quoted.png)\n"

	rewritten, placements, err := FindStandaloneLocalImageLines(markdown)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0 (blockquoted image should be ignored)", len(placements))
	}
	if rewritten != markdown {
		t.Fatalf("rewritten = %q, want input unchanged", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesHonorsBlockquoteIndentLimit(t *testing.T) {
	blockquote := "   > ![Quoted](./quoted.png)\n"

	rewritten, placements, err := FindStandaloneLocalImageLines(blockquote)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0", len(placements))
	}
	if rewritten != blockquote {
		t.Fatalf("rewritten = %q, want input unchanged", rewritten)
	}

	_, _, err = FindStandaloneLocalImageLines("text\n    > ![Code](./code.png)\n")
	if err == nil {
		t.Fatal("expected local image syntax error for 4-space-indented > line")
	}
}

func TestFindStandaloneLocalImageLinesIgnoresRawHTMLBlock(t *testing.T) {
	markdown := strings.Join([]string{
		"<div>",
		"![Demo](./example.png)",
		"</div>",
		"",
		"![Real](./real.png)",
		"",
	}, "\n")

	rewritten, placements, err := FindStandaloneLocalImageLines(markdown)
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if !strings.Contains(rewritten, "![Demo](./example.png)") {
		t.Fatalf("rewritten lost raw HTML image syntax: %q", rewritten)
	}
	if strings.Contains(rewritten, "![Real](./real.png)") {
		t.Fatalf("rewritten should have replaced the real image line: %q", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesIgnoresMissingImageInRawHTMLBlock(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "doc.md")
	real := filepath.Join(dir, "real.png")
	if err := os.WriteFile(source, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	markdown := strings.Join([]string{
		"<div>",
		"![Missing](./missing.png)",
		"</div>",
		"",
		"![Real](./real.png)",
		"",
	}, "\n")

	_, placements, err := RewriteStandaloneLocalImages(markdown, source)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Resolved != real {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, real)
	}
}

func TestIsLocalDestinationHandlesWindowsAndURISchemes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"windows drive only", `C:`, true},
		{"windows backslash path", `C:\Users\foo\img.png`, true},
		{"windows forward-slash path", `C:/Users/foo/img.png`, true},
		{"lowercase drive", `d:\tmp\img.png`, true},
		{"single-letter URI scheme", `a://example.com/img.png`, false},
		{"single-letter URI no authority", `a:example`, false},
		{"non-letter colon prefix", `::foo`, true},
		{"digit colon prefix", `1:relative`, true},
		{"empty string", ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isLocalDestination(tc.in)
			if got != tc.want {
				t.Fatalf("isLocalDestination(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
