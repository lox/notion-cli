package output

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
	"golang.org/x/term"
)

type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
}

func NewMarkdownRenderer() (*MarkdownRenderer, error) {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
		if width > 120 {
			width = 120
		}
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, fmt.Errorf("creating markdown renderer: %w", err)
	}

	return &MarkdownRenderer{renderer: r}, nil
}

func (m *MarkdownRenderer) Render(content string) (string, error) {
	content = preprocessNotionMarkdown(content)

	out, err := m.renderer.Render(content)
	if err != nil {
		return "", fmt.Errorf("rendering markdown: %w", err)
	}

	return strings.TrimSpace(out), nil
}

func (m *MarkdownRenderer) RenderAndPrint(content string) error {
	out, err := m.Render(content)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func preprocessNotionMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inCallout := false
	calloutContent := []string{}

	for _, line := range lines {
		if strings.HasPrefix(line, "> ℹ️") || strings.HasPrefix(line, "> ⚠️") ||
			strings.HasPrefix(line, "> 💡") || strings.HasPrefix(line, "> 📌") ||
			strings.HasPrefix(line, "> ❗") || strings.HasPrefix(line, "> 🔥") {
			inCallout = true
			calloutContent = append(calloutContent, line)
			continue
		}

		if inCallout {
			if strings.HasPrefix(line, "> ") {
				calloutContent = append(calloutContent, line)
				continue
			} else {
				result = append(result, calloutContent...)
				result = append(result, "")
				calloutContent = nil
				inCallout = false
			}
		}

		result = append(result, line)
	}

	if len(calloutContent) > 0 {
		result = append(result, calloutContent...)
	}

	return strings.Join(result, "\n")
}

func RenderMarkdown(content string) error {
	r, err := NewMarkdownRenderer()
	if err != nil {
		return err
	}
	return r.RenderAndPrint(content)
}

// RenderPage renders a Notion page with pretty metadata header
func RenderPage(content string) error {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	meta, body := parseNotionResponse(content)

	if meta != nil {
		renderPageHeader(meta, isTTY)
	}

	if body != "" {
		r, err := NewMarkdownRenderer()
		if err != nil {
			return err
		}
		return r.RenderAndPrint(body)
	}

	return nil
}

type pageMetadata struct {
	Title     string
	URL       string
	Created   string
	Author    string
	Type      string
	ExtraInfo string
}

func parseNotionResponse(content string) (*pageMetadata, string) {
	meta := &pageMetadata{}

	// Extract properties JSON from <properties> tag
	if start := strings.Index(content, "<properties>"); start != -1 {
		if end := strings.Index(content[start:], "</properties>"); end != -1 {
			propsContent := content[start+len("<properties>") : start+end]
			propsContent = strings.TrimSpace(propsContent)
			var data map[string]any
			if err := json.Unmarshal([]byte(propsContent), &data); err == nil {
				if name, ok := data["Name"].(string); ok {
					meta.Title = name
				}
				if url, ok := data["url"].(string); ok {
					meta.URL = cleanNotionURL(url)
				}
				if created, ok := data["Created"].(string); ok {
					meta.Created = created
				}
			}
		}
	}

	// Extract content from <content> tag
	contentRe := regexp.MustCompile(`(?s)<content>\s*(.*?)\s*</content>`)
	if match := contentRe.FindStringSubmatch(content); len(match) > 1 {
		body := match[1]
		// Clean up Notion-specific markup
		body = cleanNotionMarkup(body)
		return meta, body
	}

	// Check for database title
	if strings.Contains(content, "The title of this Database is:") {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "The title of this Database is:") {
				meta.Title = strings.TrimSpace(strings.TrimPrefix(line, "The title of this Database is:"))
				meta.Type = "database"
				break
			}
		}
		return meta, content
	}

	// Fallback: return raw content
	return meta, content
}

func cleanNotionMarkup(content string) string {
	// Remove <span discussion-urls="...">...</span> wrappers but keep content
	spanRe := regexp.MustCompile(`<span[^>]*>([^<]*)</span>`)
	content = spanRe.ReplaceAllString(content, "$1")

	// Remove other Notion-specific tags
	content = regexp.MustCompile(`<mention-[^>]*>[^<]*</mention-[^>]*>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`<omitted\s*/>`).ReplaceAllString(content, "")

	return content
}

func cleanNotionURL(url string) string {
	// Remove {{ }} wrappers
	url = strings.TrimPrefix(url, "{{")
	url = strings.TrimSuffix(url, "}}")
	return url
}

func renderPageHeader(meta *pageMetadata, isTTY bool) {
	if meta.Title == "" && meta.URL == "" {
		return
	}

	if isTTY {
		titleStyle := color.New(color.Bold, color.FgWhite)
		urlStyle := color.New(color.Faint)
		labelStyle := color.New(color.Faint)

		fmt.Println()
		if meta.Title != "" {
			titleStyle.Println(meta.Title)
		}
		if meta.URL != "" {
			urlStyle.Println(meta.URL)
		}
		if meta.Type != "" {
			labelStyle.Printf("Type: ")
			fmt.Println(meta.Type)
		}
		fmt.Println()
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println()
	} else {
		if meta.Title != "" {
			fmt.Printf("Title: %s\n", meta.Title)
		}
		if meta.URL != "" {
			fmt.Printf("URL: %s\n", meta.URL)
		}
		fmt.Println()
	}
}
