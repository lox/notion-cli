package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"

	"github.com/lox/notion-cli/internal/config"
)

const defaultHTTPTimeout = 20 * time.Second

// Notion block type strings used when appending or inspecting blocks.
const (
	BlockTypeParagraph  = "paragraph"
	BlockTypeImage      = "image"
	BlockTypeFileUpload = "file_upload"
)

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

// SinglePartUploadMaxBytes is Notion's single_part file-upload size limit.
// Files larger than this require the multi_part upload flow, which this
// client does not yet implement.
const SinglePartUploadMaxBytes = 20 * 1024 * 1024

// UploadFile streams a file upload to the Notion API without buffering the whole
// payload in memory. The reader is consumed through a multipart writer piped
// directly into the HTTP request body. Pass the exact byte size so the server
// can validate the stream.
func (c *Client) UploadFile(ctx context.Context, name string, size int64, body io.Reader) (string, error) {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" {
		return "", fmt.Errorf("filename is required")
	}
	if size <= 0 {
		return "", fmt.Errorf("file size must be positive")
	}
	if size > SinglePartUploadMaxBytes {
		return "", fmt.Errorf("file %q is %d bytes; Notion's single_part upload limit is %d bytes (~20MB) and multi_part uploads are not yet supported by this client", name, size, SinglePartUploadMaxBytes)
	}
	if body == nil {
		return "", fmt.Errorf("file body is required")
	}

	var created FileUpload
	createPayload := map[string]any{
		"mode":     "single_part",
		"filename": name,
	}
	if err := c.doJSON(ctx, http.MethodPost, "/file_uploads", createPayload, &created); err != nil {
		return "", err
	}
	if strings.TrimSpace(created.ID) == "" {
		return "", fmt.Errorf("create file upload failed: empty upload ID")
	}

	if _, err := c.sendFileUploadPart(ctx, created.ID, name, size, body); err != nil {
		return "", err
	}

	uploaded, err := c.waitForFileUploadUploaded(ctx, created.ID)
	if err != nil {
		return "", err
	}
	return uploaded.ID, nil
}

// UploadFileBytes is a convenience wrapper for callers that already have the
// file contents in memory.
func (c *Client) UploadFileBytes(ctx context.Context, name string, data []byte) (string, error) {
	return c.UploadFile(ctx, name, int64(len(data)), bytes.NewReader(data))
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
		"type": BlockTypeFileUpload,
		BlockTypeFileUpload: map[string]any{
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

	child := map[string]any{
		"object":       "block",
		"type":         BlockTypeImage,
		BlockTypeImage: image,
	}
	payload := map[string]any{
		"children": []map[string]any{child},
		"position": map[string]any{
			"type": "after_block",
			"after_block": map[string]any{
				"id": afterBlockID,
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

func (c *Client) sendFileUploadPart(ctx context.Context, fileUploadID, filename string, size int64, body io.Reader) (*FileUpload, error) {
	// Peek the first 512 bytes so we can detect the content type without
	// buffering the full payload.
	br := bufio.NewReaderSize(body, 4096)
	peek, _ := br.Peek(512)
	contentType := detectUploadContentType(filename, peek)

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	header := make(textproto.MIMEHeader)
	contentDisposition := mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": filename,
	})
	if strings.TrimSpace(contentDisposition) == "" {
		_ = pw.Close()
		return nil, fmt.Errorf("format multipart content disposition: empty result")
	}
	header.Set("Content-Disposition", contentDisposition)
	header.Set("Content-Type", contentType)

	go func() {
		part, err := writer.CreatePart(header)
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("create multipart file part: %w", err))
			return
		}
		if _, err := io.Copy(part, br); err != nil {
			_ = pw.CloseWithError(fmt.Errorf("write multipart file data: %w", err))
			return
		}
		if err := writer.Close(); err != nil {
			_ = pw.CloseWithError(fmt.Errorf("close multipart writer: %w", err))
			return
		}
		_ = pw.Close()
	}()

	var out FileUpload
	path := "/file_uploads/" + strings.TrimSpace(fileUploadID) + "/send"
	if err := c.doRequest(ctx, http.MethodPost, path, pr, writer.FormDataContentType(), &out); err != nil {
		return nil, err
	}
	_ = size // size is currently advisory; the multipart writer sets its own framing.
	if strings.TrimSpace(out.ID) == "" {
		out.ID = strings.TrimSpace(fileUploadID)
	}
	return &out, nil
}

func detectUploadContentType(filename string, data []byte) string {
	if ext := strings.TrimSpace(filepath.Ext(filename)); ext != "" {
		if contentType := strings.TrimSpace(mime.TypeByExtension(strings.ToLower(ext))); contentType != "" {
			return contentType
		}
	}
	if len(data) > 0 {
		return http.DetectContentType(data)
	}
	return "application/octet-stream"
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
