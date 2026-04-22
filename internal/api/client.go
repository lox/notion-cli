package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/lox/notion-cli/internal/config"
)

const defaultHTTPTimeout = 20 * time.Second

var (
	fileUploadPollInterval = 250 * time.Millisecond
	fileUploadMaxChecks    = 240
)

type Client struct {
	httpClient    *http.Client
	baseURL       string
	notionVersion string
	token         string
}

type Self struct {
	Object string   `json:"object"`
	ID     string   `json:"id"`
	Type   string   `json:"type"`
	Name   string   `json:"name,omitempty"`
	Bot    *SelfBot `json:"bot,omitempty"`
}

type SelfBot struct {
	WorkspaceName string `json:"workspace_name,omitempty"`
}

type FileUpload struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type UploadedImageBlock struct {
	FileUploadID string
	Caption      string
}

type PageMarkdown struct {
	Object          string   `json:"object"`
	ID              string   `json:"id"`
	Markdown        string   `json:"markdown"`
	Truncated       bool     `json:"truncated"`
	UnknownBlockIDs []string `json:"unknown_block_ids,omitempty"`
}

type PageMetadata struct {
	Object         string      `json:"object"`
	ID             string      `json:"id"`
	CreatedTime    time.Time   `json:"created_time"`
	LastEditedTime time.Time   `json:"last_edited_time"`
	CreatedBy      PartialUser `json:"created_by"`
	LastEditedBy   PartialUser `json:"last_edited_by"`
	Archived       bool        `json:"archived"`
	InTrash        bool        `json:"in_trash"`
	Icon           *PageIcon   `json:"icon,omitempty"`
	Parent         PageParent  `json:"parent"`
	URL            string      `json:"url"`
	PublicURL      string      `json:"public_url,omitempty"`
}

type PartialUser struct {
	Object string `json:"object"`
	ID     string `json:"id"`
}

type PageIcon struct {
	Type     string `json:"type"`
	Emoji    string `json:"emoji,omitempty"`
	External *struct {
		URL string `json:"url"`
	} `json:"external,omitempty"`
	File *struct {
		URL string `json:"url"`
	} `json:"file,omitempty"`
}

type PageParent struct {
	Type       string `json:"type"`
	PageID     string `json:"page_id,omitempty"`
	DatabaseID string `json:"database_id,omitempty"`
	BlockID    string `json:"block_id,omitempty"`
	Workspace  bool   `json:"workspace,omitempty"`
}

type Block struct {
	ID        string          `json:"id"`
	Object    string          `json:"object"`
	Type      string          `json:"type"`
	Paragraph *ParagraphBlock `json:"paragraph,omitempty"`
}

type ParagraphBlock struct {
	RichText []RichText `json:"rich_text"`
}

type RichText struct {
	PlainText string `json:"plain_text"`
}

type listBlocksResponse struct {
	Results    []Block `json:"results"`
	NextCursor string  `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
}

func NewClient(cfg config.APIConfig, token string) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("official API token is required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.notion.com/v1"
	}
	notionVersion := strings.TrimSpace(cfg.NotionVersion)
	if notionVersion == "" {
		notionVersion = "2026-03-11"
	}

	return &Client{
		httpClient:    &http.Client{Timeout: defaultHTTPTimeout},
		baseURL:       strings.TrimRight(baseURL, "/"),
		notionVersion: notionVersion,
		token:         token,
	}, nil
}

func (c *Client) GetSelf(ctx context.Context) (*Self, error) {
	var out Self
	if err := c.doJSON(ctx, http.MethodGet, "/users/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPageMarkdown(ctx context.Context, pageID string) (*PageMarkdown, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return nil, fmt.Errorf("page ID is required")
	}

	var out PageMarkdown
	if err := c.doJSON(ctx, http.MethodGet, "/pages/"+pageID+"/markdown", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPageMetadata retrieves page metadata (timestamps, authors, parent, icon,
// archived state) via the Notion REST API. The MCP tool notion-fetch does not
// expose these fields, so this method is used to enrich page view output when
// an official API token is configured.
func (c *Client) GetPageMetadata(ctx context.Context, pageID string) (*PageMetadata, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return nil, fmt.Errorf("page ID is required")
	}

	var out PageMetadata
	if err := c.doJSON(ctx, http.MethodGet, "/pages/"+pageID, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UploadFile(ctx context.Context, filename string, data []byte) (string, error) {
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("file data is required")
	}

	var created FileUpload
	createPayload := map[string]any{
		"mode":     "single_part",
		"filename": filename,
	}
	if err := c.doJSON(ctx, http.MethodPost, "/file_uploads", createPayload, &created); err != nil {
		return "", err
	}
	if strings.TrimSpace(created.ID) == "" {
		return "", fmt.Errorf("create file upload failed: empty upload ID")
	}

	if _, err := c.sendFileUploadPart(ctx, created.ID, filename, data); err != nil {
		return "", err
	}

	uploaded, err := c.waitForFileUploadUploaded(ctx, created.ID)
	if err != nil {
		return "", err
	}
	return uploaded.ID, nil
}

func (c *Client) AppendUploadedImageAfter(ctx context.Context, parentID, afterBlockID string, block UploadedImageBlock) error {
	parentID = strings.TrimSpace(parentID)
	afterBlockID = strings.TrimSpace(afterBlockID)
	fileUploadID := strings.TrimSpace(block.FileUploadID)
	if parentID == "" {
		return fmt.Errorf("parent ID is required")
	}
	if afterBlockID == "" {
		return fmt.Errorf("after block ID is required")
	}
	if fileUploadID == "" {
		return fmt.Errorf("file upload ID is required")
	}

	image := map[string]any{
		"type": "file_upload",
		"file_upload": map[string]any{
			"id": fileUploadID,
		},
	}
	if caption := strings.TrimSpace(block.Caption); caption != "" {
		image["caption"] = []map[string]any{
			{
				"type": "text",
				"text": map[string]any{
					"content": caption,
				},
			},
		}
	}

	payload := map[string]any{
		"position": map[string]any{
			"type": "after_block",
			"after_block": map[string]any{
				"id": afterBlockID,
			},
		},
		"children": []map[string]any{
			{
				"object": "block",
				"type":   "image",
				"image":  image,
			},
		},
	}

	return c.doJSON(ctx, http.MethodPatch, "/blocks/"+parentID+"/children", payload, nil)
}

func (c *Client) ListAllBlockChildren(ctx context.Context, blockID string) ([]Block, error) {
	blockID = strings.TrimSpace(blockID)
	if blockID == "" {
		return nil, fmt.Errorf("block ID is required")
	}

	var all []Block
	cursor := ""
	for {
		path := "/blocks/" + blockID + "/children?page_size=100"
		if cursor != "" {
			path += "&start_cursor=" + cursor
		}

		var out listBlocksResponse
		if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Results...)
		if !out.HasMore || strings.TrimSpace(out.NextCursor) == "" {
			return all, nil
		}
		cursor = out.NextCursor
	}
}

func (c *Client) DeleteBlock(ctx context.Context, blockID string) error {
	blockID = strings.TrimSpace(blockID)
	if blockID == "" {
		return fmt.Errorf("block ID is required")
	}
	return c.doJSON(ctx, http.MethodDelete, "/blocks/"+blockID, nil, nil)
}

func (c *Client) TrashPage(ctx context.Context, pageID string) error {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return fmt.Errorf("page ID is required")
	}
	return c.doJSON(ctx, http.MethodPatch, "/pages/"+pageID, map[string]any{"in_trash": true}, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	contentType := ""
	if payload != nil {
		contentType = "application/json"
	}
	return c.doRequest(ctx, method, path, bodyReader, contentType, out)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", c.notionVersion)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(respBody))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		} else {
			var errResp struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(respBody, &errResp); err == nil && strings.TrimSpace(errResp.Message) != "" {
				message = strings.TrimSpace(errResp.Message)
			}
		}
		return fmt.Errorf("official API %s %s failed (%d): %s", method, path, resp.StatusCode, message)
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("parse official API response for %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) sendFileUploadPart(ctx context.Context, fileUploadID, filename string, data []byte) (*FileUpload, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create multipart file part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write multipart file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	var out FileUpload
	path := "/file_uploads/" + strings.TrimSpace(fileUploadID) + "/send"
	if err := c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(body.Bytes()), writer.FormDataContentType(), &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.ID) == "" {
		out.ID = strings.TrimSpace(fileUploadID)
	}
	return &out, nil
}

func (c *Client) waitForFileUploadUploaded(ctx context.Context, fileUploadID string) (*FileUpload, error) {
	id := strings.TrimSpace(fileUploadID)
	if id == "" {
		return nil, fmt.Errorf("file upload ID is required")
	}

	for i := 0; i < fileUploadMaxChecks; i++ {
		var upload FileUpload
		if err := c.doJSON(ctx, http.MethodGet, "/file_uploads/"+id, nil, &upload); err != nil {
			return nil, err
		}
		if strings.TrimSpace(upload.ID) == "" {
			upload.ID = id
		}
		status := strings.ToLower(strings.TrimSpace(upload.Status))
		switch status {
		case "uploaded":
			return &upload, nil
		case "", "pending":
			if i == fileUploadMaxChecks-1 {
				return nil, fmt.Errorf("file upload %s did not reach uploaded status in time", id)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(fileUploadPollInterval):
			}
		default:
			return nil, fmt.Errorf("file upload %s failed with status %q", id, upload.Status)
		}
	}
	return nil, fmt.Errorf("file upload %s did not reach uploaded status in time", id)
}
