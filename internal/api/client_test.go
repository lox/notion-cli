package api

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lox/notion-cli/internal/config"
)

func TestGetSelf(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/users/me" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Notion-Version"); got != "2026-03-11" {
			t.Fatalf("Notion-Version = %q", got)
		}
		_, _ = w.Write([]byte(`{"object":"user","id":"user_123","type":"bot","name":"Notion CLI","bot":{"workspace_name":"Workspace"}}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{
		BaseURL:       srv.URL + "/v1",
		NotionVersion: "2026-03-11",
	}, "secret-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	self, err := client.GetSelf(context.Background())
	if err != nil {
		t.Fatalf("GetSelf: %v", err)
	}
	if self.ID != "user_123" || self.Bot == nil || self.Bot.WorkspaceName != "Workspace" {
		t.Fatalf("unexpected self: %#v", self)
	}
}

func TestUploadFileAndAppendAfter(t *testing.T) {
	createCalls := 0
	sendCalls := 0
	getCalls := 0
	appendCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads":
			createCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads/upload_123/send":
			sendCalls++
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data;") {
				t.Fatalf("Content-Type = %q", ct)
			}
			mediaType, params, err := mime.ParseMediaType(ct)
			if err != nil {
				t.Fatalf("ParseMediaType: %v", err)
			}
			if mediaType != "multipart/form-data" {
				t.Fatalf("mediaType = %q", mediaType)
			}
			reader := multipart.NewReader(r.Body, params["boundary"])
			part, err := reader.NextPart()
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			if got := part.FileName(); got != `diag"ram.png` {
				t.Fatalf("part FileName = %q", got)
			}
			if got := part.Header.Get("Content-Type"); got != "image/png" {
				t.Fatalf("part Content-Type = %q", got)
			}
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/file_uploads/upload_123":
			getCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/blocks/page_123/children":
			appendCalls++
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
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL + "/v1"}, "secret-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	uploadID, err := client.UploadFileBytes(context.Background(), `diag"ram.png`, []byte("PNGDATA"))
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if uploadID != "upload_123" {
		t.Fatalf("UploadFile = %q, want upload_123", uploadID)
	}

	if err := client.AppendUploadedImageAfter(context.Background(), "page_123", "block_123", UploadedImageBlock{
		FileUploadID: uploadID,
		Caption:      "Diagram",
	}); err != nil {
		t.Fatalf("AppendUploadedImageAfter: %v", err)
	}

	if createCalls != 1 || sendCalls != 1 || getCalls != 1 || appendCalls != 1 {
		t.Fatalf("unexpected call counts create=%d send=%d get=%d append=%d", createCalls, sendCalls, getCalls, appendCalls)
	}
}

func TestUploadFileRetriesEmptyAndPendingStatuses(t *testing.T) {
	oldPollInterval := fileUploadPollInterval
	fileUploadPollInterval = time.Millisecond
	t.Cleanup(func() {
		fileUploadPollInterval = oldPollInterval
	})

	getCalls := 0
	statuses := make([]string, 0, 23)
	statuses = append(statuses, "")
	for i := 0; i < 21; i++ {
		statuses = append(statuses, "pending")
	}
	statuses = append(statuses, "uploaded")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads":
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads/upload_123/send":
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/file_uploads/upload_123":
			status := statuses[getCalls]
			getCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"` + status + `"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL + "/v1"}, "secret-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	uploadID, err := client.UploadFileBytes(context.Background(), "diagram.png", []byte("PNGDATA"))
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if uploadID != "upload_123" {
		t.Fatalf("UploadFile = %q, want upload_123", uploadID)
	}
	if getCalls != len(statuses) {
		t.Fatalf("getCalls = %d, want %d", getCalls, len(statuses))
	}
}

func TestListAllBlockChildrenPaginates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.RawQuery {
		case "page_size=100":
			_, _ = w.Write([]byte(`{"results":[{"id":"one","type":"paragraph","paragraph":{"rich_text":[{"plain_text":"A"}]}}],"has_more":true,"next_cursor":"next"}`))
		case "page_size=100&start_cursor=next":
			_, _ = w.Write([]byte(`{"results":[{"id":"two","type":"paragraph","paragraph":{"rich_text":[{"plain_text":"B"}]}}],"has_more":false}`))
		default:
			t.Fatalf("unexpected query: %q", r.URL.RawQuery)
		}
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	blocks, err := client.ListAllBlockChildren(context.Background(), "page_123")
	if err != nil {
		t.Fatalf("ListAllBlockChildren: %v", err)
	}
	if len(blocks) != 2 || blocks[0].ID != "one" || blocks[1].ID != "two" {
		t.Fatalf("unexpected blocks: %#v", blocks)
	}
}

func TestNewClientRejectsEmptyToken(t *testing.T) {
	_, err := NewClient(config.APIConfig{}, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientErrorIncludesAPIMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"bad token"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.GetSelf(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("expected bad token error, got %v", err)
	}
}

func TestTrashPageUsesPatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/pages/page_123" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		defer func() { _ = r.Body.Close() }()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !strings.Contains(string(body), `"in_trash":true`) {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL + "/v1"}, "secret-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.TrashPage(context.Background(), "page_123"); err != nil {
		t.Fatalf("TrashPage: %v", err)
	}
}

func TestUploadFileRejectsOversizedSinglePart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL + "/v1"}, "secret-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	oversize := int64(SinglePartUploadMaxBytes + 1)
	_, err = client.UploadFile(context.Background(), "big.png", oversize, strings.NewReader(""))
	if err == nil {
		t.Fatalf("UploadFile returned nil error; expected size-limit error")
	}
	if !strings.Contains(err.Error(), "single_part upload limit") {
		t.Fatalf("UploadFile error = %q, want single_part limit message", err.Error())
	}
}
