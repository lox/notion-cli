package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

// localImageUploadConcurrency caps concurrent local image uploads. Each
// upload fires at least three sequential Notion API calls (create file
// upload, send, then poll for upload status), so even serial work lives
// right at Notion's documented ~3 req/sec average limit. Fanning out
// in parallel without a shared rate limiter and 429/Retry-After backoff
// reliably trips rate limits on pages with several images.
const localImageUploadConcurrency = 1

type uploadedLocalImage struct {
	Alt          string
	FileUploadID string
	Placeholder  string
	ResolvedPath string
}

func prepareLocalImageUploads(cmdCtx *Context, ctx context.Context, sourceFile, markdown string) (string, []uploadedLocalImage, error) {
	rewritten, placements, err := cli.RewriteStandaloneLocalImages(markdown, sourceFile)
	if err != nil {
		return "", nil, err
	}
	if len(placements) == 0 {
		return markdown, nil, nil
	}

	apiClient, err := cli.RequireOfficialAPIClient(officialAPIOverrides(cmdCtx))
	if err != nil {
		return "", nil, err
	}

	// Collect the unique set of resolved paths to upload so we do one upload
	// per distinct file even when the same image appears in multiple
	// placements.
	distinctPaths := make([]string, 0, len(placements))
	seen := make(map[string]struct{}, len(placements))
	for _, placement := range placements {
		if _, ok := seen[placement.Resolved]; ok {
			continue
		}
		seen[placement.Resolved] = struct{}{}
		distinctPaths = append(distinctPaths, placement.Resolved)
	}

	uploadIDByPath := make(map[string]string, len(distinctPaths))
	var mu sync.Mutex

	group, gctx := errgroup.WithContext(ctx)
	group.SetLimit(localImageUploadConcurrency)
	for _, path := range distinctPaths {
		path := path
		group.Go(func() error {
			uploadID, err := uploadLocalImage(gctx, apiClient, path)
			if err != nil {
				return err
			}
			mu.Lock()
			uploadIDByPath[path] = uploadID
			mu.Unlock()
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return "", nil, err
	}

	uploads := make([]uploadedLocalImage, 0, len(placements))
	for _, placement := range placements {
		uploads = append(uploads, uploadedLocalImage{
			Alt:          placement.Alt,
			FileUploadID: uploadIDByPath[placement.Resolved],
			Placeholder:  placement.Placeholder,
			ResolvedPath: placement.Resolved,
		})
	}

	return rewritten, uploads, nil
}

func uploadLocalImage(ctx context.Context, apiClient *api.Client, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open local image %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat local image %q: %w", path, err)
	}

	uploadID, err := apiClient.UploadFile(ctx, path, info.Size(), f)
	if err != nil {
		return "", fmt.Errorf("upload local image %q: %w", path, err)
	}
	return uploadID, nil
}

func stripLocalImages(markdown string) (string, error) {
	rewritten, placements, err := cli.FindStandaloneLocalImageLines(markdown)
	if err != nil {
		return "", err
	}
	if len(placements) == 0 {
		return markdown, nil
	}

	lines := strings.Split(rewritten, "\n")
	placeholders := make(map[string]struct{}, len(placements))
	for _, placement := range placements {
		placeholders[placement.Placeholder] = struct{}{}
	}
	for i, line := range lines {
		// scanStandaloneLocalImages preserves the whitespace that surrounded
		// the image so block context is not lost during upload. In strip
		// mode we need to match on the trimmed placeholder so indented or
		// trailing-whitespace image lines are also cleared rather than left
		// as NOTION_CLI_LOCAL_IMAGE_* text in the published page.
		if _, ok := placeholders[strings.TrimSpace(line)]; ok {
			lines[i] = ""
		}
	}

	return strings.Join(lines, "\n"), nil
}

// checkLocalImageParent returns a parent-required error when markdown has
// standalone local images but the caller provided neither --parent nor
// --parent-db.
func checkLocalImageParent(markdown, parent, parentDB string) error {
	_, placements, err := cli.FindStandaloneLocalImageLines(markdown)
	if err != nil {
		return err
	}
	if len(placements) == 0 {
		return nil
	}
	if strings.TrimSpace(parent) != "" || strings.TrimSpace(parentDB) != "" {
		return nil
	}
	return &output.UserError{
		Message: "standalone local image upload requires --parent or --parent-db shared with your Notion integration",
	}
}

// substituteOrCleanup substitutes uploaded local images into the page and
// trashes the page on failure so it doesn't linger with placeholders. When
// pageID is empty (the create response omitted a parseable ID), we cannot
// trash the page automatically, so the error message includes createdURL so
// the user can delete the orphan manually.
func substituteOrCleanup(cmdCtx *Context, ctx context.Context, pageID, createdURL string, uploads []uploadedLocalImage) error {
	if err := substituteUploadedLocalImages(cmdCtx, ctx, pageID, uploads); err != nil {
		finalErr := fmt.Errorf("insert uploaded local images: %w", err)
		if pageID != "" {
			if apiClient, apiErr := cli.RequireOfficialAPIClient(officialAPIOverrides(cmdCtx)); apiErr == nil {
				if cleanupErr := apiClient.TrashPage(ctx, pageID); cleanupErr != nil {
					finalErr = fmt.Errorf("%w (cleanup failed: %v)", finalErr, cleanupErr)
				}
			} else {
				finalErr = fmt.Errorf("%w (cleanup client init failed: %v)", finalErr, apiErr)
			}
		} else if len(uploads) > 0 {
			ref := strings.TrimSpace(createdURL)
			if ref == "" {
				ref = "(no URL returned; check your workspace)"
			}
			finalErr = fmt.Errorf("%w (page was created but its ID could not be parsed, so placeholders remain and automatic cleanup is not possible; delete it manually: %s)", finalErr, ref)
		}
		return finalErr
	}
	return nil
}

func substituteUploadedLocalImages(cmdCtx *Context, ctx context.Context, pageID string, uploads []uploadedLocalImage) error {
	if len(uploads) == 0 {
		return nil
	}
	if strings.TrimSpace(pageID) == "" {
		return fmt.Errorf("cannot substitute %d local image(s): missing page ID", len(uploads))
	}

	apiClient, err := cli.RequireOfficialAPIClient(officialAPIOverrides(cmdCtx))
	if err != nil {
		return err
	}

	blocks, err := apiClient.ListAllBlockChildren(ctx, pageID)
	if err != nil {
		return err
	}

	blocksByPlaceholder := make(map[string]api.Block, len(uploads))
	for _, block := range blocks {
		if block.Type != api.BlockTypeParagraph || block.Paragraph == nil {
			continue
		}
		text := paragraphPlainText(block)
		if text == "" {
			continue
		}
		blocksByPlaceholder[text] = block
	}

	for _, upload := range uploads {
		block, ok := blocksByPlaceholder[upload.Placeholder]
		if !ok {
			return fmt.Errorf("could not find placeholder block for %q", upload.ResolvedPath)
		}
		if err := apiClient.AppendUploadedImageAfter(ctx, pageID, block.ID, api.UploadedImageBlock{
			FileUploadID: upload.FileUploadID,
			Caption:      upload.Alt,
		}); err != nil {
			return err
		}
		if err := apiClient.DeleteBlock(ctx, block.ID); err != nil {
			return err
		}
	}

	return nil
}

type pageUpdater interface {
	UpdatePage(ctx context.Context, req mcp.UpdatePageRequest) error
}

func rollbackSyncedPage(ctx context.Context, client pageUpdater, pageID string, snapshot *api.PageMarkdown) error {
	if snapshot == nil {
		return nil
	}
	if snapshot.Truncated {
		return fmt.Errorf("skipped rollback: page markdown snapshot was truncated; replaying would lose content")
	}
	if len(snapshot.UnknownBlockIDs) > 0 {
		return fmt.Errorf("skipped rollback: page markdown snapshot omits %d block(s) that cannot be represented in markdown; replaying would drop them", len(snapshot.UnknownBlockIDs))
	}
	return client.UpdatePage(ctx, mcp.UpdatePageRequest{
		PageID:     pageID,
		Command:    "replace_content",
		NewContent: snapshot.Markdown,
	})
}

func pageIDFromCreateResponse(resp *mcp.CreatePageResponse) string {
	if resp == nil {
		return ""
	}
	if strings.TrimSpace(resp.ID) != "" {
		return strings.TrimSpace(resp.ID)
	}
	if strings.TrimSpace(resp.URL) == "" {
		return ""
	}
	id, _ := cli.ExtractNotionUUID(resp.URL)
	return id
}

func paragraphPlainText(block api.Block) string {
	if block.Paragraph == nil {
		return ""
	}

	var builder strings.Builder
	for _, part := range block.Paragraph.RichText {
		builder.WriteString(part.PlainText)
	}
	return builder.String()
}
