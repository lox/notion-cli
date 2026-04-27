package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/mcp"
)

type fakePageUpdater struct {
	calls []mcp.UpdatePageRequest
	err   error
}

func (f *fakePageUpdater) UpdatePage(_ context.Context, req mcp.UpdatePageRequest) error {
	f.calls = append(f.calls, req)
	return f.err
}

func TestPrepareLocalImageUploadsUploadsAndDeduplicates(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	createCalls := 0
	sendCalls := 0
	getCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads":
			createCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads/upload_123/send":
			sendCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/file_uploads/upload_123":
			getCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	cmdCtx := &Context{
		APIToken:   "secret-token",
		APIBaseURL: srv.URL + "/v1",
	}

	rewritten, uploads, err := prepareLocalImageUploads(cmdCtx, context.Background(), doc, "![One](./diagram.png)\n![Two](./diagram.png)\n")
	if err != nil {
		t.Fatalf("prepareLocalImageUploads: %v", err)
	}
	if len(uploads) != 2 {
		t.Fatalf("len(uploads) = %d, want 2", len(uploads))
	}
	if createCalls != 1 || sendCalls != 1 || getCalls != 1 {
		t.Fatalf("unexpected call counts create=%d send=%d get=%d", createCalls, sendCalls, getCalls)
	}
	if uploads[0].FileUploadID != "upload_123" || uploads[1].FileUploadID != "upload_123" {
		t.Fatalf("unexpected upload ids: %#v", uploads)
	}
	if !strings.Contains(rewritten, uploads[0].Placeholder) || !strings.Contains(rewritten, uploads[1].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholders: %q", rewritten)
	}
}

func TestRunPageSyncPreflightsMCPBeforeLocalImageUpload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatalf("WriteFile image: %v", err)
	}
	if err := os.WriteFile(doc, []byte("---\nnotion-id: page_123\n---\n\n![Diagram](./diagram.png)\n"), 0o644); err != nil {
		t.Fatalf("WriteFile doc: %v", err)
	}

	uploadCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/pages/page_123/markdown":
			_, _ = w.Write([]byte(`{"markdown":"# Previous\n","truncated":false}`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/file_uploads"):
			uploadCalls++
			http.Error(w, "upload should not start before MCP auth", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	wantErr := errors.New("mcp unavailable")
	oldRequirePageClientFn := requirePageClientFn
	requirePageClientFn = func() (*mcp.Client, error) {
		return nil, wantErr
	}
	t.Cleanup(func() {
		requirePageClientFn = oldRequirePageClientFn
	})

	err := runPageSync(&Context{
		APIToken:   "secret-token",
		APIBaseURL: srv.URL + "/v1",
	}, doc, "", "", "", "", false)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runPageSync error = %v, want %v", err, wantErr)
	}
	if uploadCalls != 0 {
		t.Fatalf("uploadCalls = %d, want 0", uploadCalls)
	}
}

func TestSubstituteUploadedLocalImagesAppendsAfterPlaceholderAndDeletes(t *testing.T) {
	var sawAppend bool
	var sawDelete bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/blocks/page_123/children":
			_, _ = w.Write([]byte(`{"results":[{"id":"block_123","type":"paragraph","paragraph":{"rich_text":[{"plain_text":"PLACEHOLDER"}]}}],"has_more":false}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/blocks/page_123/children":
			sawAppend = true
			defer func() { _ = r.Body.Close() }()
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode: %v", err)
			}
			position, ok := payload["position"].(map[string]any)
			if !ok {
				t.Fatalf("position = %#v", payload["position"])
			}
			if position["type"] != "after_block" {
				t.Fatalf("position.type = %#v", position["type"])
			}
			afterBlock, ok := position["after_block"].(map[string]any)
			if !ok {
				t.Fatalf("position.after_block = %#v", position["after_block"])
			}
			if afterBlock["id"] != "block_123" {
				t.Fatalf("position.after_block.id = %#v", afterBlock["id"])
			}
			_, _ = w.Write([]byte(`{"results":[]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/blocks/block_123":
			sawDelete = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	cmdCtx := &Context{
		APIToken:   "secret-token",
		APIBaseURL: srv.URL + "/v1",
	}

	err := substituteUploadedLocalImages(cmdCtx, context.Background(), "page_123", []uploadedLocalImage{{
		Alt:          "Diagram",
		FileUploadID: "upload_123",
		Placeholder:  "PLACEHOLDER",
		ResolvedPath: "/tmp/diagram.png",
	}})
	if err != nil {
		t.Fatalf("substituteUploadedLocalImages: %v", err)
	}
	if !sawAppend || !sawDelete {
		t.Fatalf("expected append and delete, saw append=%v delete=%v", sawAppend, sawDelete)
	}
}

func TestSubstituteUploadedLocalImagesErrorsWhenPageIDMissing(t *testing.T) {
	cmdCtx := &Context{APIToken: "secret-token"}
	uploads := []uploadedLocalImage{{
		Alt:          "Diagram",
		FileUploadID: "upload_123",
		Placeholder:  "PLACEHOLDER",
		ResolvedPath: "/tmp/diagram.png",
	}}

	err := substituteUploadedLocalImages(cmdCtx, context.Background(), "   ", uploads)
	if err == nil {
		t.Fatalf("expected error when pageID is empty with pending uploads")
	}
	if !strings.Contains(err.Error(), "missing page ID") {
		t.Fatalf("error = %v, want missing-page-ID message", err)
	}
}

func TestSubstituteOrCleanupReportsOrphanURLWhenPageIDMissing(t *testing.T) {
	cmdCtx := &Context{APIToken: "secret-token"}
	uploads := []uploadedLocalImage{{
		Alt:          "Diagram",
		FileUploadID: "upload_123",
		Placeholder:  "PLACEHOLDER",
		ResolvedPath: "/tmp/diagram.png",
	}}

	orphanURL := "https://www.notion.so/Page-1234567890abcdef1234567890abcdef"
	err := substituteOrCleanup(cmdCtx, context.Background(), "", orphanURL, uploads)
	if err == nil {
		t.Fatalf("expected error when pageID is empty with pending uploads")
	}
	if !strings.Contains(err.Error(), orphanURL) {
		t.Fatalf("error = %v, want orphan URL in message", err)
	}
	if !strings.Contains(err.Error(), "delete it manually") {
		t.Fatalf("error = %v, want manual-delete guidance", err)
	}
}

func TestSubstituteUploadedLocalImagesSkipsWithoutUploads(t *testing.T) {
	cmdCtx := &Context{APIToken: "secret-token"}
	if err := substituteUploadedLocalImages(cmdCtx, context.Background(), "", nil); err != nil {
		t.Fatalf("expected nil when no uploads, got %v", err)
	}
}

func TestCheckLocalImageParent(t *testing.T) {
	const markdownWithImage = "![Diagram](./diagram.png)\n"
	const markdownWithoutImage = "# Just text\n"

	if err := checkLocalImageParent(markdownWithoutImage, "", ""); err != nil {
		t.Fatalf("expected nil without local images, got %v", err)
	}
	if err := checkLocalImageParent(markdownWithImage, "parent-id", ""); err != nil {
		t.Fatalf("expected nil with parent, got %v", err)
	}
	if err := checkLocalImageParent(markdownWithImage, "", "db-id"); err != nil {
		t.Fatalf("expected nil with parent db, got %v", err)
	}

	err := checkLocalImageParent(markdownWithImage, "", "")
	if err == nil {
		t.Fatal("expected error without parent or parent db")
	}
	if !strings.Contains(err.Error(), "--parent or --parent-db") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A nil *mcp.Client is passed on purpose: if the guard failed to short-circuit
// on truncated snapshots, UpdatePage would dereference the nil client and panic.
func TestRollbackSyncedPageSkipsTruncatedSnapshot(t *testing.T) {
	snapshot := &api.PageMarkdown{
		Markdown:  "# Title\n\nPartial content\n",
		Truncated: true,
	}

	err := rollbackSyncedPage(context.Background(), nil, "page-id", snapshot)
	if err == nil {
		t.Fatalf("rollbackSyncedPage returned nil error; expected truncation error")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("rollbackSyncedPage error = %q, want it to mention truncation", err.Error())
	}
}

func TestRollbackSyncedPageSkipsUnknownBlocks(t *testing.T) {
	snapshot := &api.PageMarkdown{
		Markdown:        "# Title\n\nLossy content\n",
		UnknownBlockIDs: []string{"block-1", "block-2"},
	}

	err := rollbackSyncedPage(context.Background(), nil, "page-id", snapshot)
	if err == nil {
		t.Fatalf("rollbackSyncedPage returned nil error; expected unknown-blocks error")
	}
	if !strings.Contains(err.Error(), "cannot be represented in markdown") {
		t.Fatalf("rollbackSyncedPage error = %q, want it to mention unrepresentable blocks", err.Error())
	}
}

func TestRollbackSyncedPageSkipsNilSnapshot(t *testing.T) {
	if err := rollbackSyncedPage(context.Background(), nil, "page-id", nil); err != nil {
		t.Fatalf("rollbackSyncedPage returned %v, want nil", err)
	}
}

func TestRollbackSyncedPageReplacesEmptyMarkdownSnapshot(t *testing.T) {
	snapshot := &api.PageMarkdown{Markdown: ""}
	updater := &fakePageUpdater{}

	if err := rollbackSyncedPage(context.Background(), updater, "page-id", snapshot); err != nil {
		t.Fatalf("rollbackSyncedPage returned %v, want nil", err)
	}
	if len(updater.calls) != 1 {
		t.Fatalf("len(updater.calls) = %d, want 1 (empty snapshot should still restore previous empty state)", len(updater.calls))
	}
	got := updater.calls[0]
	if got.PageID != "page-id" || got.Command != "replace_content" || got.NewContent != "" {
		t.Fatalf("unexpected UpdatePage request: %#v", got)
	}
}

func TestRollbackSyncedPageReplacesWhitespaceSnapshot(t *testing.T) {
	snapshot := &api.PageMarkdown{Markdown: "   \n"}
	updater := &fakePageUpdater{}

	if err := rollbackSyncedPage(context.Background(), updater, "page-id", snapshot); err != nil {
		t.Fatalf("rollbackSyncedPage returned %v, want nil", err)
	}
	if len(updater.calls) != 1 {
		t.Fatalf("len(updater.calls) = %d, want 1", len(updater.calls))
	}
	if got := updater.calls[0].NewContent; got != "   \n" {
		t.Fatalf("NewContent = %q, want whitespace snapshot preserved verbatim", got)
	}
}

func TestRollbackSyncedPageReplacesNonEmptySnapshot(t *testing.T) {
	snapshot := &api.PageMarkdown{Markdown: "# Previous\n\nbody\n"}
	updater := &fakePageUpdater{}

	if err := rollbackSyncedPage(context.Background(), updater, "page-id", snapshot); err != nil {
		t.Fatalf("rollbackSyncedPage returned %v, want nil", err)
	}
	if len(updater.calls) != 1 {
		t.Fatalf("len(updater.calls) = %d, want 1", len(updater.calls))
	}
	if got := updater.calls[0].NewContent; got != "# Previous\n\nbody\n" {
		t.Fatalf("NewContent = %q, want snapshot markdown", got)
	}
}
