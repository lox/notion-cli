package mcp

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	DefaultEndpoint = "https://mcp.notion.com/mcp"
)

type Client struct {
	mcpClient  *client.Client
	tokenStore *FileTokenStore
}

type ClientOption func(*clientConfig)

type clientConfig struct {
	endpoint    string
	accessToken string
}

func WithEndpoint(endpoint string) ClientOption {
	return func(c *clientConfig) {
		c.endpoint = endpoint
	}
}

func WithAccessToken(token string) ClientOption {
	return func(c *clientConfig) {
		c.accessToken = token
	}
}

func NewClient(opts ...ClientOption) (*Client, error) {
	cfg := &clientConfig{
		endpoint: DefaultEndpoint,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	tokenStore, err := NewFileTokenStore()
	if err != nil {
		return nil, fmt.Errorf("create token store: %w", err)
	}

	// If access token provided directly, use a static token store
	var store transport.TokenStore = tokenStore
	if cfg.accessToken != "" {
		store = &staticTokenStore{token: cfg.accessToken}
	}

	oauthConfig := transport.OAuthConfig{
		TokenStore:  store,
		PKCEEnabled: true,
	}

	trans, err := transport.NewStreamableHTTP(
		cfg.endpoint,
		transport.WithHTTPOAuth(oauthConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("create transport: %w", err)
	}

	return &Client{
		mcpClient:  client.NewClient(trans),
		tokenStore: tokenStore,
	}, nil
}

func (c *Client) Start(ctx context.Context) error {
	if err := c.mcpClient.Start(ctx); err != nil {
		if client.IsOAuthAuthorizationRequiredError(err) {
			return &AuthRequiredError{
				Handler: client.GetOAuthHandler(err),
			}
		}
		return err
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "notion-cli",
		Version: "0.1.0",
	}

	_, err := c.mcpClient.Initialize(ctx, initReq)
	if err != nil {
		if client.IsOAuthAuthorizationRequiredError(err) {
			return &AuthRequiredError{
				Handler: client.GetOAuthHandler(err),
			}
		}
		return fmt.Errorf("initialize: %w", err)
	}

	return nil
}

func (c *Client) Close() error {
	return c.mcpClient.Close()
}

func (c *Client) TokenStore() *FileTokenStore {
	return c.tokenStore
}

func (c *Client) GetOAuthHandler() *transport.OAuthHandler {
	trans := c.mcpClient.GetTransport()
	if st, ok := trans.(*transport.StreamableHTTP); ok {
		return st.GetOAuthHandler()
	}
	return nil
}

type AuthRequiredError struct {
	Handler *transport.OAuthHandler
}

func (e *AuthRequiredError) Error() string {
	return "authentication required - run 'notion-cli auth login'"
}

func IsAuthRequired(err error) bool {
	var authErr *AuthRequiredError
	return errors.As(err, &authErr)
}

func GetOAuthHandler(err error) *transport.OAuthHandler {
	var authErr *AuthRequiredError
	if errors.As(err, &authErr) {
		return authErr.Handler
	}
	return nil
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	return c.mcpClient.CallTool(ctx, req)
}

func (c *Client) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	resp, err := c.mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Tools, nil
}

type SearchOptions struct {
	ContentSearchMode string // "workspace_search" or "ai_search" or "" (auto)
}

func (c *Client) Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResponse, error) {
	args := buildSearchToolArgs(query, opts)
	result, err := c.CallTool(ctx, "notion-search", args)
	if err != nil {
		return nil, err
	}
	if err := checkToolError(result); err != nil {
		return nil, err
	}

	text := extractText(result)
	var resp SearchResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	return &resp, nil
}

func buildSearchToolArgs(query string, opts *SearchOptions) map[string]any {
	args := map[string]any{}
	if strings.TrimSpace(query) != "" {
		args["query"] = query
	}
	if opts != nil && opts.ContentSearchMode != "" {
		args["content_search_mode"] = opts.ContentSearchMode
	}
	return args
}

type FetchResult struct {
	Content string
	Title   string
	URL     string
}

type fetchResponse struct {
	Metadata struct {
		Type string `json:"type"`
	} `json:"metadata"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text"`
}

func (c *Client) Fetch(ctx context.Context, id string) (*FetchResult, error) {
	return c.fetch(ctx, id, false)
}

func (c *Client) FetchWithDiscussions(ctx context.Context, id string) (*FetchResult, error) {
	return c.fetch(ctx, id, true)
}

func (c *Client) fetch(ctx context.Context, id string, includeDiscussions bool) (*FetchResult, error) {
	result, err := c.CallTool(ctx, "notion-fetch", buildFetchToolArgs(id, includeDiscussions))
	if err != nil {
		return nil, err
	}
	if err := checkToolError(result); err != nil {
		return nil, err
	}

	text := extractText(result)

	var resp fetchResponse
	if err := json.Unmarshal([]byte(text), &resp); err == nil && resp.Text != "" {
		return &FetchResult{Content: resp.Text, Title: resp.Title, URL: resp.URL}, nil
	}

	return &FetchResult{Content: text}, nil
}

func buildFetchToolArgs(id string, includeDiscussions bool) map[string]any {
	args := map[string]any{
		"id": id,
	}
	if includeDiscussions {
		args["include_discussions"] = true
	}
	return args
}

type CreatePageRequest struct {
	ParentPageID     string
	ParentDatabaseID string
	Title            string
	Content          string
	Properties       map[string]string
}

type CreatePageResponse struct {
	URL string `json:"url"`
	ID  string `json:"id"`
}

func (c *Client) CreatePage(ctx context.Context, req CreatePageRequest) (*CreatePageResponse, error) {
	props := map[string]any{}
	for k, v := range req.Properties {
		props[k] = v
	}
	props["title"] = req.Title

	pageSpec := map[string]any{
		"properties": props,
	}

	if req.Content != "" {
		pageSpec["content"] = req.Content
	}

	args := map[string]any{
		"pages": []any{pageSpec},
	}

	if req.ParentPageID != "" {
		args["parent"] = map[string]any{
			"page_id": req.ParentPageID,
		}
	} else if req.ParentDatabaseID != "" {
		args["parent"] = map[string]any{
			"data_source_id": req.ParentDatabaseID,
		}
	}

	result, err := c.CallTool(ctx, "notion-create-pages", args)
	if err != nil {
		return nil, err
	}
	if err := checkToolError(result); err != nil {
		return nil, err
	}

	text := extractText(result)

	var resp CreatePageResponse
	if err := json.Unmarshal([]byte(text), &resp); err == nil && resp.URL != "" {
		return &resp, nil
	}

	url := extractURLFromText(text)
	return &CreatePageResponse{URL: url}, nil
}

func extractURLFromText(text string) string {
	if idx := strings.Index(text, "https://www.notion.so/"); idx >= 0 {
		end := idx
		for end < len(text) && text[end] != ' ' && text[end] != '\n' && text[end] != '"' && text[end] != ')' && text[end] != '>' {
			end++
		}
		return text[idx:end]
	}
	return ""
}

// ResolveDataSourceID fetches a database by ID and extracts the data source ID
// from the collection:// URL in the content. If the ID is already a data source ID,
// it's returned as-is (the fetch will fail, and we fall back).
func (c *Client) ResolveDataSourceID(ctx context.Context, id string) (string, error) {
	result, err := c.Fetch(ctx, id)
	if err != nil {
		return id, nil // assume it's already a data source ID
	}

	// Look for collection://UUID pattern in the content
	re := regexp.MustCompile(`collection://([a-fA-F0-9-]{32,36})`)
	if m := re.FindStringSubmatch(result.Content); m != nil {
		return m[1], nil
	}

	return id, nil // fallback to original ID
}

type UpdatePageRequest struct {
	PageID  string
	Command string // "replace_content", "update_content", "insert_content_after", "update_properties", "apply_template", "update_verification"

	// For replace_content
	NewContent           string
	AllowDeletingContent bool

	// For update_content
	ContentUpdates []ContentUpdate

	// For insert_content_after
	Selection string
	NewStr    string

	// For update_properties
	Properties map[string]any
}

type ContentUpdate struct {
	OldStr string
	NewStr string
}

func (c *Client) UpdatePage(ctx context.Context, req UpdatePageRequest) error {
	data := buildUpdatePageToolArgs(req)

	result, err := c.CallTool(ctx, "notion-update-page", data)
	if err != nil {
		return err
	}
	return checkToolError(result)
}

func buildUpdatePageToolArgs(req UpdatePageRequest) map[string]any {
	data := map[string]any{
		"page_id": req.PageID,
		"command": req.Command,
	}
	if req.AllowDeletingContent {
		data["allow_deleting_content"] = true
	}

	switch req.Command {
	case "replace_content":
		data["new_str"] = req.NewContent
	case "update_content":
		updates := make([]any, 0, len(req.ContentUpdates))
		for _, u := range req.ContentUpdates {
			updates = append(updates, map[string]any{
				"old_str": u.OldStr,
				"new_str": u.NewStr,
			})
		}
		data["content_updates"] = updates
	case "insert_content_after":
		data["selection_with_ellipsis"] = req.Selection
		data["new_str"] = req.NewStr
	case "update_properties":
		data["properties"] = req.Properties
	}

	return data
}

type GetCommentsRequest struct {
	PageID           string `json:"page_id,omitempty"`
	BlockID          string `json:"block_id,omitempty"`
	Cursor           string `json:"cursor,omitempty"`
	PageSize         int    `json:"page_size,omitempty"`
	IncludeAllBlocks bool   `json:"include_all_blocks,omitempty"`
	IncludeResolved  bool   `json:"include_resolved,omitempty"`
	DiscussionID     string `json:"discussion_id,omitempty"`
}

func (c *Client) GetComments(ctx context.Context, req GetCommentsRequest) (*CommentsResponse, error) {
	args := buildGetCommentsToolArgs(req)

	result, err := c.CallTool(ctx, "notion-get-comments", args)
	if err != nil {
		return nil, err
	}
	if err := checkToolError(result); err != nil {
		return nil, err
	}

	return parseCommentsResponse(extractText(result))
}

func buildGetCommentsToolArgs(req GetCommentsRequest) map[string]any {
	args := make(map[string]any)
	if req.PageID != "" {
		args["page_id"] = req.PageID
	}
	if req.BlockID != "" {
		args["block_id"] = req.BlockID
	}
	if req.Cursor != "" {
		args["cursor"] = req.Cursor
	}
	if req.PageSize > 0 {
		args["page_size"] = req.PageSize
	}
	if req.IncludeAllBlocks {
		args["include_all_blocks"] = true
	}
	if req.IncludeResolved {
		args["include_resolved"] = true
	}
	if req.DiscussionID != "" {
		args["discussion_id"] = req.DiscussionID
	}
	return args
}

type commentsXMLWrapper struct {
	Text string `json:"text"`
}

type discussionsXML struct {
	XMLName     xml.Name        `xml:"discussions"`
	Discussions []discussionXML `xml:"discussion"`
}

type discussionXML struct {
	ID          string       `xml:"id,attr"`
	Resolved    bool         `xml:"resolved,attr"`
	TextContext string       `xml:"text-context,attr"`
	Comments    []commentXML `xml:"comment"`
}

type commentXML struct {
	ID       string `xml:"id,attr"`
	UserURL  string `xml:"user-url,attr"`
	Datetime string `xml:"datetime,attr"`
	Body     string `xml:",innerxml"`
}

func parseCommentsResponse(text string) (*CommentsResponse, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || trimmed == "{}" {
		return &CommentsResponse{}, nil
	}

	if strings.HasPrefix(trimmed, "<discussions") {
		return parseCommentsXML(trimmed)
	}

	var wrapper commentsXMLWrapper
	if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil && wrapper.Text != "" {
		return parseCommentsXML(wrapper.Text)
	}

	var resp CommentsResponse
	if err := json.Unmarshal([]byte(trimmed), &resp); err == nil {
		if strings.Contains(trimmed, `"comments"`) || strings.Contains(trimmed, `"next_cursor"`) || strings.Contains(trimmed, `"has_more"`) {
			return &resp, nil
		}
	}

	return nil, fmt.Errorf("parse comments: unsupported response format")
}

func parseCommentsXML(text string) (*CommentsResponse, error) {
	var doc discussionsXML
	if err := xml.Unmarshal([]byte(sanitiseCommentsXML(text)), &doc); err != nil {
		return nil, fmt.Errorf("parse comments xml: %w", err)
	}

	comments := make([]Comment, 0)
	for _, discussion := range doc.Discussions {
		for _, comment := range discussion.Comments {
			createdAt, _ := time.Parse(time.RFC3339Nano, comment.Datetime)
			body := extractCommentBodyText(comment.Body)
			comments = append(comments, Comment{
				ID:           comment.ID,
				Object:       "comment",
				DiscussionID: discussion.ID,
				Context:      strings.TrimSpace(discussion.TextContext),
				Resolved:     discussion.Resolved,
				CreatedTime:  createdAt,
				CreatedBy: UserRef{
					ID: extractCommentUserID(comment.UserURL),
				},
				RichText: []RichText{{
					Type:      "text",
					PlainText: body,
					Text: &TextContent{
						Content: body,
					},
				}},
			})
		}
	}

	return &CommentsResponse{Comments: comments}, nil
}

func extractCommentUserID(userURL string) string {
	return strings.TrimPrefix(userURL, "user://")
}

func extractCommentBodyText(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	nodes, err := html.ParseFragment(strings.NewReader(body), &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"})
	if err != nil {
		return body
	}

	var out strings.Builder
	for i, node := range nodes {
		if shouldSkipCommentWhitespaceFragmentNode(nodes, i) {
			continue
		}
		appendCommentBodyText(&out, node)
	}

	return strings.TrimSpace(normaliseCommentBodyLineEndings(out.String()))
}

func appendCommentBodyText(out *strings.Builder, node *html.Node) {
	if node == nil {
		return
	}

	switch node.Type {
	case html.TextNode:
		if shouldSkipCommentWhitespaceTextNode(node) {
			return
		}
		out.WriteString(node.Data)
		return
	case html.ElementNode:
		if node.DataAtom == atom.Br {
			out.WriteByte('\n')
			return
		}
		if isCommentBlockNode(node.DataAtom) {
			ensureTrailingCommentNewline(out)
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendCommentBodyText(out, child)
	}

	if node.Type == html.ElementNode && isCommentBlockNode(node.DataAtom) {
		ensureTrailingCommentNewline(out)
	}
}

func shouldSkipCommentWhitespaceTextNode(node *html.Node) bool {
	if node == nil || node.Type != html.TextNode || strings.TrimSpace(node.Data) != "" {
		return false
	}
	return isCommentBlockHTMLNode(adjacentNonWhitespaceSibling(node, true)) || isCommentBlockHTMLNode(adjacentNonWhitespaceSibling(node, false))
}

func shouldSkipCommentWhitespaceFragmentNode(nodes []*html.Node, index int) bool {
	if index < 0 || index >= len(nodes) {
		return false
	}
	node := nodes[index]
	if node == nil || node.Type != html.TextNode || strings.TrimSpace(node.Data) != "" {
		return false
	}
	return isCommentBlockHTMLNode(adjacentNonWhitespaceFragmentNode(nodes, index, true)) || isCommentBlockHTMLNode(adjacentNonWhitespaceFragmentNode(nodes, index, false))
}

func adjacentNonWhitespaceFragmentNode(nodes []*html.Node, index int, previous bool) *html.Node {
	for i := index + offsetForDirection(previous); i >= 0 && i < len(nodes); i += offsetForDirection(previous) {
		node := nodes[i]
		if node == nil {
			continue
		}
		if node.Type == html.TextNode && strings.TrimSpace(node.Data) == "" {
			continue
		}
		return node
	}
	return nil
}

func offsetForDirection(previous bool) int {
	if previous {
		return -1
	}
	return 1
}

func adjacentNonWhitespaceSibling(node *html.Node, previous bool) *html.Node {
	for sibling := siblingNode(node, previous); sibling != nil; sibling = siblingNode(sibling, previous) {
		if sibling.Type == html.TextNode && strings.TrimSpace(sibling.Data) == "" {
			continue
		}
		return sibling
	}
	return nil
}

func siblingNode(node *html.Node, previous bool) *html.Node {
	if previous {
		return node.PrevSibling
	}
	return node.NextSibling
}

func isCommentBlockHTMLNode(node *html.Node) bool {
	return node != nil && node.Type == html.ElementNode && isCommentBlockNode(node.DataAtom)
}

func isCommentBlockNode(tag atom.Atom) bool {
	switch tag {
	case atom.P, atom.Div, atom.Li, atom.Ul, atom.Ol, atom.Blockquote:
		return true
	default:
		return false
	}
}

func ensureTrailingCommentNewline(out *strings.Builder) {
	if out.Len() == 0 {
		return
	}
	if strings.HasSuffix(out.String(), "\n") {
		return
	}
	out.WriteByte('\n')
}

func normaliseCommentBodyLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

func sanitiseCommentsXML(text string) string {
	text = strings.NewReplacer(
		"<br></br>", "<br/>",
		"<br ></br>", "<br/>",
	).Replace(text)
	text = strings.NewReplacer(
		"<br>", "<br/>",
		"<br >", "<br/>",
	).Replace(text)

	var out strings.Builder
	out.Grow(len(text))

	for i := 0; i < len(text); i++ {
		if text[i] != '&' {
			out.WriteByte(text[i])
			continue
		}

		if entity, width, ok := parseXMLEntity(text[i+1:]); ok {
			out.WriteByte('&')
			out.WriteString(entity)
			out.WriteByte(';')
			i += width
			continue
		}

		out.WriteString("&amp;")
	}

	return out.String()
}

func parseXMLEntity(text string) (entity string, width int, ok bool) {
	semi := strings.IndexByte(text, ';')
	if semi <= 0 {
		return "", 0, false
	}

	entity = text[:semi]
	if !isValidXMLEntity(entity) {
		return "", 0, false
	}

	return entity, semi + 1, true
}

func isValidXMLEntity(entity string) bool {
	if entity == "" {
		return false
	}

	if entity[0] == '#' {
		if len(entity) == 1 {
			return false
		}
		if entity[1] == 'x' || entity[1] == 'X' {
			if len(entity) == 2 {
				return false
			}
			for i := 2; i < len(entity); i++ {
				if !isHexDigit(entity[i]) {
					return false
				}
			}
			return true
		}
		for i := 1; i < len(entity); i++ {
			if entity[i] < '0' || entity[i] > '9' {
				return false
			}
		}
		return true
	}

	switch entity {
	case "amp", "lt", "gt", "quot", "apos":
		return true
	default:
		return false
	}
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

type getUsersResponse struct {
	Results    []getUserResult `json:"results"`
	NextCursor string          `json:"next_cursor,omitempty"`
	HasMore    bool            `json:"has_more"`
}

type getUserResult struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	result, err := c.CallTool(ctx, "notion-get-users", map[string]any{
		"user_id": userID,
	})
	if err != nil {
		return nil, err
	}
	if err := checkToolError(result); err != nil {
		return nil, err
	}

	users, err := parseUsersResponse(extractText(result))
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func parseUsersResponse(text string) ([]User, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || trimmed == "{}" {
		return nil, nil
	}

	var resp getUsersResponse
	if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
		return nil, fmt.Errorf("parse users: %w", err)
	}

	users := make([]User, 0, len(resp.Results))
	for _, result := range resp.Results {
		user := User{
			ID:   result.ID,
			Type: result.Type,
			Name: result.Name,
		}
		if result.Email != "" {
			user.Person = &Person{Email: result.Email}
		}
		users = append(users, user)
	}

	return users, nil
}

type CreateCommentRequest struct {
	PageID       string `json:"page_id,omitempty"`
	DiscussionID string `json:"discussion_id,omitempty"`
	Text         string `json:"text"`
}

func (c *Client) CreateComment(ctx context.Context, req CreateCommentRequest) (*Comment, error) {
	args := map[string]any{
		"text": req.Text,
	}
	if req.PageID != "" {
		args["page_id"] = req.PageID
	}
	if req.DiscussionID != "" {
		args["discussion_id"] = req.DiscussionID
	}

	result, err := c.CallTool(ctx, "notion-create-comment", args)
	if err != nil {
		return nil, err
	}
	if err := checkToolError(result); err != nil {
		return nil, err
	}

	text := extractText(result)
	var comment Comment
	if err := json.Unmarshal([]byte(text), &comment); err != nil {
		return nil, fmt.Errorf("parse comment: %w", err)
	}

	return &comment, nil
}

// staticTokenStore provides a token from a fixed string (for CI/env var usage)
type staticTokenStore struct {
	token string
}

func (s *staticTokenStore) GetToken(ctx context.Context) (*transport.Token, error) {
	return &transport.Token{
		AccessToken: s.token,
		TokenType:   "Bearer",
	}, nil
}

func (s *staticTokenStore) SaveToken(ctx context.Context, token *transport.Token) error {
	return nil // no-op for static tokens
}

// checkToolError returns an error if the MCP tool result indicates failure.
// The Notion MCP server signals errors via IsError=true with the error message
// in the text content, rather than returning a transport-level error.
func checkToolError(result *mcp.CallToolResult) error {
	if result == nil || !result.IsError {
		return nil
	}
	msg := extractText(result)
	if msg == "" {
		msg = "tool call failed"
	}
	return fmt.Errorf("notion API error: %s", msg)
}

func extractText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			return textContent.Text
		}
	}
	return ""
}
