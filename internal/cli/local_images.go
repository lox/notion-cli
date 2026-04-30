package cli

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type LocalImagePlacement struct {
	Alt         string
	Original    string
	Resolved    string
	Placeholder string
}

var uriSchemeRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)

func RewriteStandaloneLocalImages(markdown, sourceFile string) (string, []LocalImagePlacement, error) {
	sourceFileAbs, err := filepath.Abs(sourceFile)
	if err != nil {
		return "", nil, fmt.Errorf("resolve source file path: %w", err)
	}
	sourceDir := filepath.Dir(sourceFileAbs)

	return scanStandaloneLocalImages(markdown, func(dest string) (string, error) {
		resolvedPath, err := resolveLocalPath(dest, sourceDir)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("local image %q not found (from %s): %w", dest, sourceFile, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("local image %q resolves to a directory: %s", dest, resolvedPath)
		}
		return resolvedPath, nil
	})
}

// FindStandaloneLocalImageLines rewrites standalone local image lines into
// placeholders without validating that the referenced files exist on disk. It
// still rejects inline or mixed-content local image syntax with the same error
// as RewriteStandaloneLocalImages, preserving the "local images must appear on
// their own line" invariant. Use this when the caller only needs to identify
// and strip local image lines (e.g. page upload/sync --skip-local-images) and
// does not need the resolved file path.
func FindStandaloneLocalImageLines(markdown string) (string, []LocalImagePlacement, error) {
	return scanStandaloneLocalImages(markdown, nil)
}

// scanStandaloneLocalImages walks markdown line-by-line, skipping fenced and
// indented code blocks, rejecting inline local images, and replacing each
// standalone local image line with a placeholder. When resolvePath is non-nil,
// it is invoked for every local destination and its returned path is stored on
// the placement's Resolved field; a non-nil error from resolvePath aborts the
// scan. When resolvePath is nil, placements are recorded without a Resolved
// path.
func scanStandaloneLocalImages(markdown string, resolvePath func(dest string) (string, error)) (string, []LocalImagePlacement, error) {
	normalizedMarkdown := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(markdown)
	lines := strings.Split(normalizedMarkdown, "\n")
	placements := make([]LocalImagePlacement, 0)

	var inFence bool
	var fenceChar byte
	var fenceLen int
	var inIndented bool
	prevBlank := true

	for i, line := range lines {
		isBlank := strings.TrimSpace(line) == ""

		if inFence {
			if closesFencedCodeBlock(line, fenceChar, fenceLen) {
				inFence = false
			}
			prevBlank = isBlank
			continue
		}
		if c, n := opensFencedCodeBlock(line); n > 0 {
			inFence = true
			fenceChar = c
			fenceLen = n
			prevBlank = false
			continue
		}
		// Blockquotes typically hold documentation, examples, or quoted
		// code including fenced blocks with literal `![...](...)` samples.
		// The fence detector above ignores `>`-prefixed lines, so any
		// image syntax inside a quoted fence would otherwise be scanned
		// as a real local image and either uploaded by mistake or hit
		// the "must appear on their own line" error path. Treat the
		// whole blockquote line as opaque to image scanning.
		if isBlockquoteLine(line) {
			prevBlank = isBlank
			continue
		}

		if inIndented {
			if isBlank || startsWithCodeIndent(line) {
				prevBlank = isBlank
				continue
			}
			inIndented = false
		} else if prevBlank && !isBlank && startsWithCodeIndent(line) {
			inIndented = true
			prevBlank = false
			continue
		}

		scanLine := maskInlineCodeSpans(line)
		matches := findMarkdownImages(scanLine)
		if len(matches) == 0 {
			prevBlank = isBlank
			continue
		}

		if !isStandaloneImageLine(line, matches) {
			for _, m := range matches {
				dest, ok := parseMarkdownDestination(m.dest)
				if ok && isLocalDestination(dest) {
					return "", nil, fmt.Errorf("unsupported local image syntax on line %d: local images must appear on their own line", i+1)
				}
			}
			prevBlank = isBlank
			continue
		}

		m := matches[0]
		dest, ok := parseMarkdownDestination(m.dest)
		if !ok || !isLocalDestination(dest) {
			prevBlank = isBlank
			continue
		}

		var resolvedPath string
		if resolvePath != nil {
			r, err := resolvePath(dest)
			if err != nil {
				return "", nil, err
			}
			resolvedPath = r
		}

		placeholder := "NOTION_CLI_LOCAL_IMAGE_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
		// Replace the entire line with the bare placeholder at column 0. We
		// deliberately drop any surrounding whitespace so the placeholder
		// always lands in a paragraph block after replace_content, even
		// when the original image sat inside a list continuation or
		// blockquote. Keeping the leading whitespace would let Notion nest
		// the placeholder inside the enclosing block (list_item, quote,
		// etc.), and substituteUploadedLocalImages only indexes paragraph
		// blocks, so substitution would fail and force rollback. The
		// tradeoff is cosmetic: uploaded images render as standalone
		// blocks rather than nested under the list or quote they were
		// written under.
		lines[i] = placeholder
		placements = append(placements, LocalImagePlacement{
			Alt:         m.alt,
			Original:    dest,
			Resolved:    resolvedPath,
			Placeholder: placeholder,
		})
		prevBlank = false
	}

	return strings.Join(lines, "\n"), placements, nil
}

type markdownImageMatch struct {
	start int
	end   int
	alt   string
	dest  string
}

// maskInlineCodeSpans replaces the contents of every inline code span in
// `line` with spaces so the image scanner does not pick up markdown tokens
// inside the span. Length is preserved so match offsets still map back to
// the original line. An opening run of N backticks closes on the first
// matching run of exactly N backticks (CommonMark rule). Backticks inside
// a span cannot be escaped.
func maskInlineCodeSpans(line string) string {
	b := []byte(line)
	i := 0
	for i < len(b) {
		if b[i] != '`' {
			i++
			continue
		}
		runStart := i
		for i < len(b) && b[i] == '`' {
			i++
		}
		runLen := i - runStart
		for i < len(b) {
			if b[i] != '`' {
				i++
				continue
			}
			closeStart := i
			for i < len(b) && b[i] == '`' {
				i++
			}
			if i-closeStart == runLen {
				for k := runStart + runLen; k < closeStart; k++ {
					if b[k] != '\n' {
						b[k] = ' '
					}
				}
				break
			}
		}
	}
	return string(b)
}

// findMarkdownImages returns every `![alt](dest)` span on a single line.
// Destinations may contain balanced parentheses and `\)` / `\(` escapes, and
// may optionally be wrapped in angle brackets (`<dest>`). A backslash before
// `!` escapes the image marker so `\![...]` is treated as literal text.
// The returned `dest` is the raw text between the opening `(` and the
// matching `)`, preserving escapes for parseMarkdownDestination to normalize.
func findMarkdownImages(line string) []markdownImageMatch {
	var matches []markdownImageMatch
	i := 0
	for i < len(line) {
		if line[i] == '\\' && i+1 < len(line) {
			i += 2
			continue
		}
		if line[i] != '!' || i+1 >= len(line) || line[i+1] != '[' {
			i++
			continue
		}
		altStart := i + 2
		altEnd, ok := findLinkTextEnd(line, altStart)
		if !ok {
			i = altStart
			continue
		}
		if altEnd+1 >= len(line) || line[altEnd+1] != '(' {
			i = altEnd + 1
			continue
		}
		destStart := altEnd + 2
		destEnd, ok := findDestinationEnd(line, destStart)
		if !ok {
			i = altEnd + 1
			continue
		}
		matches = append(matches, markdownImageMatch{
			start: i,
			end:   destEnd + 1,
			alt:   line[altStart:altEnd],
			dest:  line[destStart:destEnd],
		})
		i = destEnd + 1
	}
	return matches
}

// findLinkTextEnd walks `line` from `start` and returns the offset of the
// unescaped `]` that closes the link text, honoring `\]` escapes and balanced
// nested `[`/`]` pairs so alt text like `Architecture [v2]` parses as a single
// image.
func findLinkTextEnd(line string, start int) (int, bool) {
	depth := 0
	for i := start; i < len(line); i++ {
		c := line[i]
		if c == '\\' && i+1 < len(line) {
			i++
			continue
		}
		switch c {
		case '[':
			depth++
		case ']':
			if depth == 0 {
				return i, true
			}
			depth--
		}
	}
	return 0, false
}

// findDestinationEnd returns the offset of the `)` that closes a markdown
// destination starting at `start`. Balanced parens and `\(` / `\)` escapes
// inside the destination are preserved; angle-bracketed destinations end at
// the first unescaped `>`, optionally followed by a whitespace-separated title
// in `"..."`, `'...'`, or `(...)` form before the closing `)`.
func findDestinationEnd(line string, start int) (int, bool) {
	if start < len(line) && line[start] == '<' {
		for i := start + 1; i < len(line); i++ {
			c := line[i]
			if c == '\\' && i+1 < len(line) {
				i++
				continue
			}
			if c == '>' {
				return skipOptionalTitleAndClose(line, i+1)
			}
		}
		return 0, false
	}
	depth := 0
	for i := start; i < len(line); i++ {
		c := line[i]
		switch c {
		case '\\':
			if i+1 < len(line) {
				i++
				continue
			}
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i, true
			}
			depth--
		case ' ', '\t':
			// Whitespace at depth 0 terminates a non-angle destination; the
			// rest is an optional quoted title, then the closing `)`. Skip
			// through the title so a `)` inside it doesn't look like the end
			// of the image span.
			if depth == 0 {
				return skipOptionalTitleAndClose(line, i)
			}
		}
	}
	return 0, false
}

// skipOptionalTitleAndClose returns the offset of the `)` that terminates a
// markdown image after an optional whitespace-separated title, starting at
// the first character after the destination (or after the closing `>` of an
// angle-bracketed destination).
func skipOptionalTitleAndClose(line string, i int) (int, bool) {
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return 0, false
	}
	if line[i] == ')' {
		return i, true
	}
	var closeQuote byte
	switch line[i] {
	case '"':
		closeQuote = '"'
	case '\'':
		closeQuote = '\''
	case '(':
		closeQuote = ')'
	default:
		return 0, false
	}
	for i++; i < len(line); i++ {
		if line[i] == '\\' && i+1 < len(line) {
			i++
			continue
		}
		if line[i] == closeQuote {
			break
		}
	}
	if i >= len(line) {
		return 0, false
	}
	for i++; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			continue
		}
		if line[i] == ')' {
			return i, true
		}
		return 0, false
	}
	return 0, false
}

// isStandaloneImageLine reports whether `line` contains exactly one image and
// no other non-whitespace content.
func isStandaloneImageLine(line string, matches []markdownImageMatch) bool {
	if len(matches) != 1 {
		return false
	}
	m := matches[0]
	return strings.TrimSpace(line[:m.start]) == "" && strings.TrimSpace(line[m.end:]) == ""
}

// isBlockquoteLine reports whether `line` opens or continues a CommonMark
// blockquote (a `>` optionally preceded by up to three spaces of indent).
func isBlockquoteLine(line string) bool {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	return i < len(line) && line[i] == '>'
}

// opensFencedCodeBlock reports whether `line` opens a CommonMark fenced code
// block. It returns the fence character (`` ` `` or `~`) and the fence length
// (>= 3) on a match, and (0, 0) otherwise.
func opensFencedCodeBlock(line string) (byte, int) {
	i := 0
	for i < len(line) && i < 4 && line[i] == ' ' {
		i++
	}
	if i >= 4 || i >= len(line) {
		return 0, 0
	}
	c := line[i]
	if c != '`' && c != '~' {
		return 0, 0
	}
	n := 0
	for i < len(line) && line[i] == c {
		i++
		n++
	}
	if n < 3 {
		return 0, 0
	}
	// Backtick info strings may not contain additional backticks.
	if c == '`' && strings.ContainsRune(line[i:], '`') {
		return 0, 0
	}
	return c, n
}

// closesFencedCodeBlock reports whether `line` closes a fenced code block that
// was opened with `openChar` repeated `openLen` times. A closing fence must be
// the same character, at least as long as the opener, indented no more than 3
// spaces, and followed only by whitespace.
func closesFencedCodeBlock(line string, openChar byte, openLen int) bool {
	i := 0
	for i < len(line) && i < 4 && line[i] == ' ' {
		i++
	}
	if i >= 4 || i >= len(line) || line[i] != openChar {
		return false
	}
	n := 0
	for i < len(line) && line[i] == openChar {
		i++
		n++
	}
	if n < openLen {
		return false
	}
	for ; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			return false
		}
	}
	return true
}

// startsWithCodeIndent reports whether `line` begins with an indented-code
// indentation (a tab or four spaces).
func startsWithCodeIndent(line string) bool {
	if strings.HasPrefix(line, "\t") {
		return true
	}
	if len(line) >= 4 && line[:4] == "    " {
		return true
	}
	return false
}

func parseMarkdownDestination(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}

	if strings.HasPrefix(s, "<") {
		// Match the scanner's findDestinationEnd: walk past `\>` escapes
		// so inputs like `<./foo\>bar.png>` keep the full destination
		// instead of truncating at the first `>`.
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if s[i] == '>' {
				if i > 1 {
					return unescapeMarkdownPunctuation(s[1:i]), true
				}
				break
			}
		}
	}

	escaped := false
	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			s = s[:i]
			break
		}
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return unescapeMarkdownPunctuation(s), true
}

// unescapeMarkdownPunctuation turns CommonMark backslash escapes of ASCII
// punctuation (e.g. `\)`, `\(`, `\\`) into their literal characters. It also
// decodes `\<space>` because the destination scanner treats a backslash as
// an escape across whitespace so users can embed spaces in unbracketed paths
// like `./diagram\ 1.png`; if we left the backslash in place, the resolved
// path would not match the real filename.
func unescapeMarkdownPunctuation(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (isASCIIPunctuation(s[i+1]) || s[i+1] == ' ') {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isASCIIPunctuation(b byte) bool {
	switch b {
	case '!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '/',
		':', ';', '<', '=', '>', '?', '@', '[', '\\', ']', '^', '_', '`', '{', '|', '}', '~':
		return true
	}
	return false
}

func isLocalDestination(dest string) bool {
	d := strings.TrimSpace(dest)
	if d == "" {
		return false
	}

	lower := strings.ToLower(d)
	switch {
	case strings.HasPrefix(lower, "#"),
		strings.HasPrefix(lower, "//"),
		strings.HasPrefix(lower, "http://"),
		strings.HasPrefix(lower, "https://"),
		strings.HasPrefix(lower, "mailto:"),
		strings.HasPrefix(lower, "tel:"),
		strings.HasPrefix(lower, "data:"):
		return false
	case strings.HasPrefix(lower, "file://"):
		return true
	}

	// Windows drive paths like `C:`, `C:\foo`, or `C:/foo`. Require the
	// leading character to be an ASCII letter and the character after the
	// colon to be a separator (or end-of-string) so one-letter URI schemes
	// such as `a:foo` fall through to the scheme check below instead of
	// being misclassified as filesystem paths. Exclude `x://...` because
	// that is a URI with authority, not a Windows drive path.
	if len(d) >= 2 && isASCIILetter(d[0]) && d[1] == ':' {
		hasAuthority := len(d) >= 4 && d[2] == '/' && d[3] == '/'
		if !hasAuthority && (len(d) == 2 || d[2] == '\\' || d[2] == '/') {
			return true
		}
	}

	return !uriSchemeRE.MatchString(d)
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func resolveLocalPath(dest, sourceDir string) (string, error) {
	d := strings.TrimSpace(dest)
	if strings.HasPrefix(strings.ToLower(d), "file://") {
		parsed, err := url.Parse(d)
		if err != nil {
			return "", fmt.Errorf("invalid file URL %q: %w", d, err)
		}
		unescaped, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return "", fmt.Errorf("invalid file URL path %q: %w", d, err)
		}
		// Preserve the authority for UNC-style file URLs
		// (file://server/share/path) so the resolved local path points at
		// the right share instead of silently dropping the host.
		if parsed.Host != "" && parsed.Host != "localhost" {
			d = "//" + parsed.Host + unescaped
		} else {
			d = unescaped
		}
	}

	// Accept both `~/` and `~\` so markdown paths written with forward
	// slashes still expand on Windows, where filepath.Separator is `\`.
	if strings.HasPrefix(d, "~/") || strings.HasPrefix(d, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home path %q: %w", d, err)
		}
		d = filepath.Join(home, d[2:])
	}

	if !filepath.IsAbs(d) {
		d = filepath.Join(sourceDir, d)
	}

	abs, err := filepath.Abs(d)
	if err != nil {
		return "", fmt.Errorf("resolve local path %q: %w", dest, err)
	}
	return filepath.Clean(abs), nil
}
