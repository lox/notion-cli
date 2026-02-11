package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
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
	return "authentication required - run 'notion config auth'"
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
	args := map[string]any{
		"query": query,
	}
	if opts != nil && opts.ContentSearchMode != "" {
		args["content_search_mode"] = opts.ContentSearchMode
	}
	result, err := c.CallTool(ctx, "notion-search", args)
	if err != nil {
		return nil, err
	}

	text := extractText(result)
	var resp SearchResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	return &resp, nil
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
	result, err := c.CallTool(ctx, "notion-fetch", map[string]any{
		"id": id,
	})
	if err != nil {
		return nil, err
	}

	text := extractText(result)

	var resp fetchResponse
	if err := json.Unmarshal([]byte(text), &resp); err == nil && resp.Text != "" {
		return &FetchResult{Content: resp.Text, Title: resp.Title, URL: resp.URL}, nil
	}

	return &FetchResult{Content: text}, nil
}

type CreatePageRequest struct {
	ParentPageID     string
	ParentDatabaseID string
	Title            string
	Content          string
}

type CreatePageResponse struct {
	URL string `json:"url"`
	ID  string `json:"id"`
}

func (c *Client) CreatePage(ctx context.Context, req CreatePageRequest) (*CreatePageResponse, error) {
	pageSpec := map[string]any{
		"properties": map[string]any{
			"title": req.Title,
		},
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

type UpdatePageRequest struct {
	PageID  string
	Command string // "replace_content", "replace_content_range", "insert_content_after", "update_properties"

	// For replace_content
	NewContent string

	// For replace_content_range and insert_content_after
	Selection string
	NewStr    string

	// For update_properties
	Properties map[string]any
}

func (c *Client) UpdatePage(ctx context.Context, req UpdatePageRequest) error {
	data := map[string]any{
		"page_id": req.PageID,
		"command": req.Command,
	}

	switch req.Command {
	case "replace_content":
		data["new_str"] = req.NewContent
	case "replace_content_range", "insert_content_after":
		data["selection_with_ellipsis"] = req.Selection
		data["new_str"] = req.NewStr
	case "update_properties":
		data["properties"] = req.Properties
	}

	args := map[string]any{"data": data}

	_, err := c.CallTool(ctx, "notion-update-page", args)
	return err
}

type GetCommentsRequest struct {
	PageID   string `json:"page_id,omitempty"`
	BlockID  string `json:"block_id,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

func (c *Client) GetComments(ctx context.Context, req GetCommentsRequest) (*CommentsResponse, error) {
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

	result, err := c.CallTool(ctx, "notion-get-comments", args)
	if err != nil {
		return nil, err
	}

	text := extractText(result)
	var resp CommentsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return nil, fmt.Errorf("parse comments: %w", err)
	}

	return &resp, nil
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
