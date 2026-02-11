package cli

import (
	"strings"
)

const frontmatterDelimiter = "---"

type Frontmatter struct {
	NotionID string
}

// ParseFrontmatter extracts frontmatter and body from a markdown string.
// Returns the parsed frontmatter (if any) and the body without frontmatter.
func ParseFrontmatter(content string) (Frontmatter, string) {
	trimmed := strings.TrimLeft(content, " \t")
	if !strings.HasPrefix(trimmed, frontmatterDelimiter) {
		return Frontmatter{}, content
	}

	rest := trimmed[len(frontmatterDelimiter):]
	if len(rest) == 0 || (rest[0] != '\n' && rest[0] != '\r') {
		return Frontmatter{}, content
	}
	rest = consumeNewline(rest)

	if strings.HasPrefix(rest, frontmatterDelimiter) {
		afterClose := rest[len(frontmatterDelimiter):]
		body := strings.TrimLeft(afterClose, "\r\n")
		return Frontmatter{}, body
	}

	endIdx := strings.Index(rest, "\n"+frontmatterDelimiter)
	if endIdx == -1 {
		return Frontmatter{}, content
	}

	fmBlock := rest[:endIdx]
	afterClose := rest[endIdx+1+len(frontmatterDelimiter):]

	body := strings.TrimLeft(afterClose, "\r\n")

	fm := Frontmatter{}
	for _, line := range strings.Split(fmBlock, "\n") {
		trimLine := strings.TrimRight(line, " \t\r")
		if trimLine == "" || strings.HasPrefix(trimLine, "#") {
			continue
		}
		if strings.HasPrefix(trimLine, " ") || strings.HasPrefix(trimLine, "\t") {
			continue
		}
		k, v, ok := strings.Cut(trimLine, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "notion-id" {
			fm.NotionID = v
		}
	}

	return fm, body
}

// SetFrontmatterID returns the content with notion-id set in frontmatter.
// If frontmatter already exists, it updates or adds the notion-id field.
// If no frontmatter exists, it prepends a new frontmatter block.
func SetFrontmatterID(content string, notionID string) string {
	hasTrailingNewline := strings.HasSuffix(content, "\n")
	_, body := ParseFrontmatter(content)

	fmBlock := extractFrontmatterBlock(content)
	if fmBlock == "" {
		return ensureTrailingNewline(frontmatterDelimiter+"\nnotion-id: "+notionID+"\n"+frontmatterDelimiter+"\n\n"+body, hasTrailingNewline)
	}

	var newLines []string
	replaced := false
	for _, line := range strings.Split(fmBlock, "\n") {
		trimLine := strings.TrimRight(line, " \t\r")
		isTopLevel := !strings.HasPrefix(trimLine, " ") && !strings.HasPrefix(trimLine, "\t")
		if isTopLevel {
			if k, _, ok := strings.Cut(trimLine, ":"); ok && strings.TrimSpace(k) == "notion-id" {
				newLines = append(newLines, "notion-id: "+notionID)
				replaced = true
				continue
			}
		}
		newLines = append(newLines, line)
	}
	if !replaced {
		newLines = append(newLines, "notion-id: "+notionID)
	}

	return ensureTrailingNewline(frontmatterDelimiter+"\n"+strings.Join(newLines, "\n")+"\n"+frontmatterDelimiter+"\n\n"+body, hasTrailingNewline)
}

func ensureTrailingNewline(s string, want bool) string {
	has := strings.HasSuffix(s, "\n")
	if want && !has {
		return s + "\n"
	}
	if !want && has {
		return strings.TrimRight(s, "\n")
	}
	return s
}

func extractFrontmatterBlock(content string) string {
	trimmed := strings.TrimLeft(content, " \t")
	if !strings.HasPrefix(trimmed, frontmatterDelimiter) {
		return ""
	}
	rest := trimmed[len(frontmatterDelimiter):]
	if len(rest) == 0 || (rest[0] != '\n' && rest[0] != '\r') {
		return ""
	}
	rest = consumeNewline(rest)

	endIdx := strings.Index(rest, "\n"+frontmatterDelimiter)
	if endIdx == -1 {
		return ""
	}
	return rest[:endIdx]
}

func consumeNewline(s string) string {
	if len(s) > 0 && s[0] == '\r' {
		s = s[1:]
	}
	if len(s) > 0 && s[0] == '\n' {
		s = s[1:]
	}
	return s
}
