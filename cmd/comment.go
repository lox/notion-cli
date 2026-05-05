package cmd

import (
	"context"
	"strings"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
	"golang.org/x/net/html"
)

type CommentCmd struct {
	List   CommentListCmd   `cmd:"" help:"List comments and discussions on a page"`
	Create CommentCreateCmd `cmd:"" help:"Create a comment on a page"`
}

type CommentListCmd struct {
	Page     string `arg:"" help:"Page URL, name, or ID"`
	Resolved bool   `help:"Include resolved discussions"`
	JSON     bool   `help:"Output as JSON" short:"j"`
}

type commentsGetter interface {
	GetComments(context.Context, mcp.GetCommentsRequest) (*mcp.CommentsResponse, error)
}

func (c *CommentListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runCommentList(ctx, c.Page, c.Resolved)
}

func runCommentList(ctx *Context, page string, includeResolved bool) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()
	pageID, err := resolveCommentPageID(bgCtx, page, client, cli.ResolvePageID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	req := buildCommentListRequest(pageID, includeResolved)

	mcpComments, err := loadAllComments(bgCtx, client, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	comments := convertComments(mcpComments)
	if len(comments) > 0 {
		if pageResult, err := client.FetchWithDiscussions(bgCtx, pageID); err == nil {
			hydrateCommentContextsFromPageContent(pageResult.Content, comments)
		}
	}
	hydrateCommentAuthors(bgCtx, client, comments)
	return output.PrintComments(comments, ctx.JSON)
}

func buildCommentListRequest(pageID string, includeResolved bool) mcp.GetCommentsRequest {
	return mcp.GetCommentsRequest{
		PageID:           pageID,
		IncludeAllBlocks: true,
		IncludeResolved:  includeResolved,
	}
}

func loadAllComments(ctx context.Context, client commentsGetter, req mcp.GetCommentsRequest) ([]mcp.Comment, error) {
	comments := make([]mcp.Comment, 0)
	for {
		resp, err := client.GetComments(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return comments, nil
		}

		comments = append(comments, resp.Comments...)
		if !resp.HasMore || resp.NextCursor == "" {
			return comments, nil
		}

		req.Cursor = resp.NextCursor
	}
}

func convertComments(mcpComments []mcp.Comment) []output.Comment {
	comments := make([]output.Comment, 0, len(mcpComments))
	for _, c := range mcpComments {
		comments = append(comments, output.Comment{
			ID:             c.ID,
			DiscussionID:   c.DiscussionID,
			Context:        c.Context,
			Resolved:       c.Resolved,
			CreatedTime:    c.CreatedTime,
			LastEditedTime: c.LastEditedTime,
			CreatedBy:      c.CreatedBy.ID,
			Content:        extractRichText(c.RichText),
		})
	}
	return comments
}

func extractRichText(richText []mcp.RichText) string {
	var content string
	for _, rt := range richText {
		content += rt.PlainText
	}
	return content
}

func resolveCommentPageID(ctx context.Context, page string, client *mcp.Client, resolve pageIDResolver) (string, error) {
	ref := cli.ParsePageRef(page)
	if ref.Kind == cli.RefID {
		return ref.ID, nil
	}

	return resolve(ctx, client, page)
}

func hydrateCommentAuthors(ctx context.Context, client *mcp.Client, comments []output.Comment) {
	seen := make(map[string]string)
	for i := range comments {
		authorID := comments[i].CreatedBy
		if authorID == "" {
			continue
		}
		if name, ok := seen[authorID]; ok {
			comments[i].CreatedByName = name
			continue
		}

		user, err := client.GetUser(ctx, authorID)
		if err != nil || user == nil || user.Name == "" {
			continue
		}

		seen[authorID] = user.Name
		comments[i].CreatedByName = user.Name
	}
}

func hydrateCommentContextsFromPageContent(pageContent string, comments []output.Comment) {
	contextByDiscussion := extractDiscussionContexts(pageContent)
	if len(contextByDiscussion) == 0 {
		return
	}

	for i := range comments {
		if context := contextByDiscussion[comments[i].DiscussionID]; context != "" {
			comments[i].Context = context
			continue
		}
		if context := contextByDiscussion[canonicalDiscussionID(comments[i].DiscussionID)]; context != "" {
			comments[i].Context = context
		}
	}
}

func extractDiscussionContexts(pageContent string) map[string]string {
	if pageContent == "" {
		return nil
	}

	doc, err := html.Parse(strings.NewReader("<root>" + pageContent + "</root>"))
	if err != nil {
		return nil
	}

	contexts := make(map[string]string)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "span" {
			if ids := htmlAttr(n, "discussion-urls"); ids != "" {
				if text := normaliseDiscussionContext(htmlTextContent(n)); text != "" {
					for _, id := range splitDiscussionURLs(ids) {
						contexts[id] = text
						contexts[canonicalDiscussionID(id)] = text
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return contexts
}

func splitDiscussionURLs(value string) []string {
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

func normaliseDiscussionContext(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func canonicalDiscussionID(id string) string {
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

func htmlAttr(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func htmlTextContent(n *html.Node) string {
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return text.String()
}

type CommentCreateCmd struct {
	Page    string `arg:"" help:"Page URL, name, or ID"`
	Content string `help:"Comment content" short:"c" required:""`
	JSON    bool   `help:"Output as JSON" short:"j"`
}

func (c *CommentCreateCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runCommentCreate(ctx, c.Page, c.Content)
}

func runCommentCreate(ctx *Context, page, content string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()
	pageID, err := resolveCommentPageID(bgCtx, page, client, cli.ResolvePageID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	req := mcp.CreateCommentRequest{
		PageID: pageID,
		Text:   content,
	}

	comment, err := client.CreateComment(bgCtx, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if ctx.JSON {
		outComments := []output.Comment{{
			ID:             comment.ID,
			DiscussionID:   comment.DiscussionID,
			CreatedTime:    comment.CreatedTime,
			LastEditedTime: comment.LastEditedTime,
			CreatedBy:      comment.CreatedBy.ID,
			Content:        extractRichText(comment.RichText),
		}}
		return output.PrintComments(outComments, true)
	}

	output.PrintSuccess("Comment created")
	return nil
}
