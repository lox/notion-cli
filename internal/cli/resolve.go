package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type PageRefKind int

const (
	RefID   PageRefKind = iota
	RefURL
	RefName
)

type PageRef struct {
	Kind PageRefKind
	Raw  string
	ID   string // canonical UUID if Kind==RefID
}

var hexPattern = regexp.MustCompile(`[0-9a-fA-F]{32}`)

// ParsePageRef classifies an input string as a URL, ID, or name.
func ParsePageRef(s string) PageRef {
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if id, ok := extractNotionUUID(s); ok {
			return PageRef{Kind: RefID, Raw: s, ID: id}
		}
		return PageRef{Kind: RefURL, Raw: s}
	}
	if LooksLikeID(s) {
		id, _ := extractNotionUUID(s)
		return PageRef{Kind: RefID, Raw: s, ID: id}
	}
	return PageRef{Kind: RefName, Raw: s}
}

// ResolvePageID resolves any page reference (URL, ID, or name) to a page ID.
// For URLs, it extracts the embedded UUID. For names, it searches and requires
// an exact unique match.
func ResolvePageID(ctx context.Context, client *mcp.Client, input string) (string, error) {
	ref := ParsePageRef(input)
	switch ref.Kind {
	case RefID:
		return ref.ID, nil
	case RefURL:
		if id, ok := extractNotionUUID(input); ok {
			return id, nil
		}
		return "", &output.UserError{Message: fmt.Sprintf("could not extract page ID from URL: %s\nUse the page ID directly instead.", input)}
	case RefName:
		return resolvePageByName(ctx, client, input)
	}
	return "", &output.UserError{Message: "invalid page reference: " + input}
}

// extractNotionUUID finds exactly 32 hex digits in a string and returns
// the canonical UUID format (8-4-4-4-12).
func extractNotionUUID(s string) (string, bool) {
	cleaned := strings.ReplaceAll(s, "-", "")

	if len(cleaned) == 32 && isAllHex(cleaned) {
		return formatUUID(cleaned), true
	}

	match := hexPattern.FindString(s)
	if match == "" {
		return "", false
	}
	return formatUUID(match), true
}

func isAllHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func formatUUID(hex string) string {
	hex = strings.ToLower(hex)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
}

// LooksLikeID returns true if the string is a valid Notion UUID (32 or 36 hex chars).
func LooksLikeID(s string) bool {
	_, ok := extractNotionUUID(s)
	if !ok {
		return false
	}
	cleaned := strings.ReplaceAll(s, "-", "")
	return len(cleaned) == 32 && isAllHex(cleaned)
}

func resolvePageByName(ctx context.Context, client *mcp.Client, name string) (string, error) {
	resp, err := client.Search(ctx, name, &mcp.SearchOptions{ContentSearchMode: "workspace_search"})
	if err != nil {
		return "", err
	}

	var exactMatches []mcp.SearchResult
	for _, r := range resp.Results {
		if r.ObjectType != "page" && r.Object != "page" {
			continue
		}
		if strings.EqualFold(r.Title, name) {
			exactMatches = append(exactMatches, r)
		}
	}

	if len(exactMatches) == 1 {
		return exactMatches[0].ID, nil
	}

	if len(exactMatches) > 1 {
		return "", ambiguousError(name, exactMatches)
	}

	// No exact match — check for partial matches to give a helpful error
	var partialMatches []mcp.SearchResult
	for _, r := range resp.Results {
		if r.ObjectType != "page" && r.Object != "page" {
			continue
		}
		if strings.Contains(strings.ToLower(r.Title), strings.ToLower(name)) {
			partialMatches = append(partialMatches, r)
		}
	}

	if len(partialMatches) == 0 {
		return "", &output.UserError{Message: "page not found: " + name}
	}

	return "", ambiguousError(name, partialMatches)
}

func ambiguousError(name string, matches []mcp.SearchResult) error {
	var b strings.Builder
	fmt.Fprintf(&b, "ambiguous page name %q, matching pages:\n", name)
	limit := len(matches)
	if limit > 5 {
		limit = 5
	}
	for _, m := range matches[:limit] {
		id := m.ID
		if m.URL != "" {
			fmt.Fprintf(&b, "  %s (%s)\n", m.Title, m.URL)
		} else {
			fmt.Fprintf(&b, "  %s (%s)\n", m.Title, id)
		}
	}
	if len(matches) > 5 {
		fmt.Fprintf(&b, "  ... and %d more\n", len(matches)-5)
	}
	b.WriteString("Use a page URL or ID to be specific.")
	return &output.UserError{Message: b.String()}
}

// IsEmoji returns true if the rune is an emoji character.
func IsEmoji(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) && !unicode.IsPunct(r) && r > 127
}
