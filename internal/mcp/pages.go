package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// MovePageRequest identifies a page and the parent it is moved under.
// Exactly one of ParentPageID or ParentDatabaseID must be set.
type MovePageRequest struct {
	PageID           string
	ParentPageID     string
	ParentDatabaseID string
}

// MovePage moves a page under a new parent page or data source.
func (c *Client) MovePage(ctx context.Context, req MovePageRequest) error {
	args, err := buildMovePageToolArgs(req)
	if err != nil {
		return err
	}

	result, err := c.CallTool(ctx, "notion-move-pages", args)
	if err != nil {
		return err
	}
	return checkToolError(result)
}

func buildMovePageToolArgs(req MovePageRequest) (map[string]any, error) {
	parent := map[string]any{}
	switch {
	case req.ParentPageID != "" && req.ParentDatabaseID != "":
		return nil, fmt.Errorf("move page: both a page and a database parent given")
	case req.ParentPageID != "":
		parent["page_id"] = req.ParentPageID
	case req.ParentDatabaseID != "":
		parent["data_source_id"] = req.ParentDatabaseID
	default:
		return nil, fmt.Errorf("move page: no parent given")
	}

	return map[string]any{
		"page_or_database_ids": []any{req.PageID},
		"new_parent":           parent,
	}, nil
}

// DuplicatePageResponse holds the identity of the page produced by a duplication.
type DuplicatePageResponse struct {
	ID  string `json:"page_id"`
	URL string `json:"page_url"`
}

// DuplicatePage copies a page, its content, its child pages, and its explicit
// permissions under the same parent.
func (c *Client) DuplicatePage(ctx context.Context, pageID string) (*DuplicatePageResponse, error) {
	result, err := c.CallTool(ctx, "notion-duplicate-page", map[string]any{
		"page_id": pageID,
	})
	if err != nil {
		return nil, err
	}
	if err := checkToolError(result); err != nil {
		return nil, err
	}

	return parseDuplicatePageResponse(extractText(result)), nil
}

func parseDuplicatePageResponse(text string) *DuplicatePageResponse {
	var resp DuplicatePageResponse
	if err := json.Unmarshal([]byte(text), &resp); err == nil && (resp.URL != "" || resp.ID != "") {
		return &resp
	}

	return &DuplicatePageResponse{URL: extractURLFromText(text)}
}

// ListUsersOptions filters and bounds a user listing.
type ListUsersOptions struct {
	Query string
	Limit int
}

// ListUsers returns the workspace users, following the server's cursor until
// the listing is exhausted or Limit is reached.
func (c *Client) ListUsers(ctx context.Context, opts *ListUsersOptions) ([]User, error) {
	var (
		all    []User
		cursor string
	)

	for {
		args := map[string]any{}
		if opts != nil && opts.Query != "" {
			args["query"] = opts.Query
		}
		if cursor != "" {
			args["start_cursor"] = cursor
		}

		result, err := c.CallTool(ctx, "notion-get-users", args)
		if err != nil {
			return nil, err
		}
		if err := checkToolError(result); err != nil {
			return nil, err
		}

		text := extractText(result)

		users, err := parseUsersResponse(text)
		if err != nil {
			return nil, err
		}
		all = append(all, users...)

		if opts != nil && opts.Limit > 0 && len(all) >= opts.Limit {
			return all[:opts.Limit], nil
		}

		cursor = nextUsersCursor(text)
		if cursor == "" {
			return all, nil
		}
	}
}

func nextUsersCursor(text string) string {
	var resp getUsersResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil || !resp.HasMore {
		return ""
	}
	return resp.NextCursor
}

// CurrentUser returns the user the access token authenticates as.
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	return c.GetUser(ctx, "self")
}
