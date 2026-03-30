package output

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// notionToMarkdown converts Notion's XML-like content to Markdown.
// It uses an HTML parser which is lenient with malformed markup.
func notionToMarkdown(content string) string {
	rendered, _ := notionToMarkdownWithComments(content, nil)
	return rendered
}

func notionToMarkdownWithComments(content string, comments []Comment) (string, map[string]bool) {
	// Preprocess: remove self-closing tags that HTML parser mishandles
	// These become nested containers otherwise
	content = regexp.MustCompile(`<empty-block\s*/>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`<unknown[^>]*/>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`<omitted\s*/>`).ReplaceAllString(content, "")

	// Convert self-closing mention-page to paired tags for proper parsing
	content = regexp.MustCompile(`<mention-page([^>]*)/>`).ReplaceAllString(content, "<mention-page$1></mention-page>")

	// Wrap in a root element to ensure valid parsing
	wrapped := "<root>" + content + "</root>"

	doc, err := html.Parse(strings.NewReader(wrapped))
	if err != nil {
		return content, nil
	}

	var out strings.Builder
	ctx := &renderContext{
		out:             &out,
		inQuote:         false,
		inlineComments:  buildInlineCommentIndex(comments),
		usedDiscussions: make(map[string]bool),
	}

	// Find <root> element (will be under html > body) and process its children
	var findAndProcess func(*html.Node) bool
	findAndProcess = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "root" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				ctx.renderNode(c)
			}
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if findAndProcess(c) {
				return true
			}
		}
		return false
	}
	findAndProcess(doc)

	result := out.String()

	// Clean up excess blank lines
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result), ctx.usedDiscussions
}

type renderContext struct {
	out             *strings.Builder
	inQuote         bool
	inlineComments  map[string][]Comment
	usedDiscussions map[string]bool
}

func (ctx *renderContext) renderNode(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		ctx.renderText(n.Data)
	case html.ElementNode:
		ctx.renderElement(n)
	default:
		// Recurse into children for other node types
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			ctx.renderNode(c)
		}
	}
}

// Precompiled regexes for text cleaning
var (
	colorAnnotationRe = regexp.MustCompile(`\s*\{color="[^"]+"\}`)
	slackLinkRe       = regexp.MustCompile(`\[([^\]]+)\]\(\{\{slackChannel://[^}]+\}\}\)`)
	notionURLRe       = regexp.MustCompile(`\{\{([^}]+)\}\}`)
)

func (ctx *renderContext) renderText(text string) {
	// Clean color annotations like {color="gray"}
	text = colorAnnotationRe.ReplaceAllString(text, "")

	// Clean Slack channel links in markdown format: [#channel]({{slackChannel://...}}) -> #channel
	text = slackLinkRe.ReplaceAllString(text, "$1")

	// Clean remaining {{url}} wrappers in markdown links
	text = notionURLRe.ReplaceAllString(text, "$1")

	if ctx.inQuote {
		// Add quote prefix to each line
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if i > 0 {
				ctx.out.WriteString("\n> ")
			}
			ctx.out.WriteString(line)
		}
	} else {
		ctx.out.WriteString(text)
	}
}

func (ctx *renderContext) renderElement(n *html.Node) {
	switch n.Data {
	case "callout":
		ctx.renderCallout(n)
	case "columns":
		ctx.renderColumns(n)
	case "column":
		ctx.renderColumn(n)
	case "page":
		ctx.renderPageLink(n)
	case "database":
		ctx.renderDatabaseLink(n)
	case "mention-page":
		ctx.renderMentionPage(n)
	case "span":
		ctx.renderSpan(n)
	case "empty-block", "unknown", "omitted":
		// Skip these elements entirely
	case "p", "div":
		ctx.renderChildren(n)
		ctx.out.WriteString("\n")
	case "br":
		ctx.out.WriteString("\n")
	case "a":
		ctx.renderLink(n)
	case "strong", "b":
		ctx.out.WriteString("**")
		ctx.renderChildren(n)
		ctx.out.WriteString("**")
	case "em", "i":
		ctx.out.WriteString("*")
		ctx.renderChildren(n)
		ctx.out.WriteString("*")
	case "code":
		ctx.out.WriteString("`")
		ctx.renderChildren(n)
		ctx.out.WriteString("`")
	case "h1":
		ctx.out.WriteString("\n# ")
		ctx.renderChildren(n)
		ctx.out.WriteString("\n")
	case "h2":
		ctx.out.WriteString("\n## ")
		ctx.renderChildren(n)
		ctx.out.WriteString("\n")
	case "h3":
		ctx.out.WriteString("\n### ")
		ctx.renderChildren(n)
		ctx.out.WriteString("\n")
	case "ul", "ol":
		ctx.out.WriteString("\n")
		ctx.renderChildren(n)
	case "li":
		ctx.out.WriteString("- ")
		ctx.renderChildren(n)
		ctx.out.WriteString("\n")
	default:
		// For unknown elements, just render children
		ctx.renderChildren(n)
	}
}

func (ctx *renderContext) renderChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		ctx.renderNode(c)
	}
}

func (ctx *renderContext) renderSpan(n *html.Node) {
	ids := splitOutputDiscussionURLs(getAttr(n, "discussion-urls"))
	if len(ids) == 0 {
		ctx.renderChildren(n)
		return
	}

	renderedComments := make([]Comment, 0)
	for _, id := range ids {
		canonicalID := canonicalOutputDiscussionID(id)
		comments := ctx.inlineComments[canonicalID]
		if len(comments) == 0 {
			continue
		}
		ctx.usedDiscussions[canonicalID] = true
		renderedComments = append(renderedComments, comments...)
	}

	if len(renderedComments) == 0 {
		ctx.renderChildren(n)
		return
	}

	highlighted := strings.TrimSpace(ctx.renderNodeToString(n))
	if highlighted != "" {
		ctx.out.WriteString("[[" + highlighted + "]]")
	} else {
		ctx.renderChildren(n)
	}

	ctx.out.WriteString("\n")
	ctx.renderInlineCommentBlock(renderedComments)
}

func (ctx *renderContext) renderInlineCommentBlock(comments []Comment) {
	for _, comment := range comments {
		header := commentAuthorName(comment)
		if timeLabel := formatTime(comment.CreatedTime); timeLabel != "" {
			header += " · " + timeLabel
		}
		if statusLabel := commentStatusLabel(comment); statusLabel != "" {
			header += " · " + statusLabel
		}

		ctx.out.WriteString("\n> Comment — " + header + "\n")
		for _, line := range strings.Split(strings.TrimSpace(comment.Content), "\n") {
			ctx.out.WriteString("> " + line + "\n")
		}
		ctx.out.WriteString("\n\n")
	}
}

func (ctx *renderContext) renderNodeToString(n *html.Node) string {
	var out strings.Builder
	childCtx := &renderContext{
		out:             &out,
		inQuote:         ctx.inQuote,
		inlineComments:  ctx.inlineComments,
		usedDiscussions: ctx.usedDiscussions,
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		childCtx.renderNode(c)
	}
	return strings.TrimSpace(out.String())
}

func buildInlineCommentIndex(comments []Comment) map[string][]Comment {
	index := make(map[string][]Comment)
	for _, comment := range comments {
		canonicalID := canonicalOutputDiscussionID(comment.DiscussionID)
		if canonicalID == "" {
			continue
		}
		index[canonicalID] = append(index[canonicalID], comment)
	}
	return index
}

func splitOutputDiscussionURLs(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		ids = append(ids, part)
	}
	return ids
}

func canonicalOutputDiscussionID(id string) string {
	if !strings.HasPrefix(id, "discussion://") {
		return id
	}

	body := strings.TrimPrefix(id, "discussion://")
	parts := strings.Split(body, "/")
	if len(parts) == 0 {
		return id
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return id
	}
	return "discussion://" + last
}

func (ctx *renderContext) renderCallout(n *html.Node) {
	icon := getAttr(n, "icon")

	// Handle custom emoji (notion://custom_emoji/...) - use a generic icon
	if strings.HasPrefix(icon, "notion://") || icon == "" {
		icon = "💡"
	}

	ctx.out.WriteString("\n> " + icon + " ")

	// Render children in quote context
	oldQuote := ctx.inQuote
	ctx.inQuote = true
	ctx.renderChildren(n)
	ctx.inQuote = oldQuote

	ctx.out.WriteString("\n")
}

func (ctx *renderContext) renderColumns(n *html.Node) {
	// Just render children - columns will add separators
	ctx.renderChildren(n)
}

func (ctx *renderContext) renderColumn(n *html.Node) {
	// Collect column content
	var colOut strings.Builder
	colCtx := &renderContext{out: &colOut}
	colCtx.renderChildren(n)

	// Dedent and add to output
	content := dedentContent(colOut.String())
	ctx.out.WriteString("\n")
	ctx.out.WriteString(content)
	ctx.out.WriteString("\n")
}

func (ctx *renderContext) renderPageLink(n *html.Node) {
	url := cleanNotionURL(getAttr(n, "url"))
	title := getTextContent(n)

	if title == "" {
		title = "page"
	}

	if ctx.inQuote {
		// Inline in callout
		ctx.out.WriteString("**[" + title + "](" + url + ")**")
	} else {
		// Block context - render as list item
		ctx.out.WriteString("\n- [📄 " + title + "](" + url + ")")
	}
}

func (ctx *renderContext) renderDatabaseLink(n *html.Node) {
	url := cleanNotionURL(getAttr(n, "url"))
	title := getTextContent(n)

	if title == "" {
		title = "database"
	}

	ctx.out.WriteString("\n**[📊 " + title + "](" + url + ")**\n")
}

func (ctx *renderContext) renderMentionPage(n *html.Node) {
	url := cleanNotionURL(getAttr(n, "url"))
	title := getTextContent(n)

	if ctx.inQuote {
		// Inline in callout
		if title == "" {
			ctx.out.WriteString("[→ page](" + url + ")")
		} else {
			ctx.out.WriteString("[" + title + "](" + url + ")")
		}
	} else {
		// Block context - render as list item
		if title == "" {
			ctx.out.WriteString("\n- [→ page](" + url + ")")
		} else {
			ctx.out.WriteString("\n- [" + title + "](" + url + ")")
		}
	}
}

func (ctx *renderContext) renderLink(n *html.Node) {
	href := getAttr(n, "href")
	text := getTextContent(n)

	// Skip Slack channel links - just show the text
	if strings.HasPrefix(href, "slackChannel://") {
		ctx.out.WriteString(text)
		return
	}

	ctx.out.WriteString("[" + text + "](" + cleanNotionURL(href) + ")")
}

// getAttr returns the value of an attribute on a node
func getAttr(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

// getTextContent returns all text content within a node
func getTextContent(n *html.Node) string {
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(text.String())
}

// dedentContent removes common leading whitespace from all lines
func dedentContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	// Find minimum indentation (excluding empty lines)
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := 0
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				indent++
			} else {
				break
			}
		}
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return strings.TrimSpace(content)
	}

	// Remove common indentation
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result = append(result, "")
			continue
		}
		runes := []rune(line)
		if len(runes) >= minIndent {
			result = append(result, string(runes[minIndent:]))
		} else {
			result = append(result, strings.TrimSpace(line))
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}
