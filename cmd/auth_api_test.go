package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lox/notion-cli/internal/config"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = oldStdout
	return <-done
}

func TestReadOfficialAPITokenFromReader(t *testing.T) {
	token, err := readOfficialAPIToken(strings.NewReader(" secret-token \n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("readOfficialAPIToken: %v", err)
	}
	if token != "secret-token" {
		t.Fatalf("token = %q, want secret-token", token)
	}
}

func TestLooksLikeNotionAPIToken(t *testing.T) {
	valid := "ntn_Q757783358158rIe2zWtQzaPS0AERzY88Mi7HR3KruTbof"
	if !looksLikeNotionAPIToken(valid) {
		t.Fatalf("expected valid token format: %q", valid)
	}

	invalid := []string{
		"",
		"secret-token",
		"ntn-short",
		"ntn_bad-token",
	}
	for _, token := range invalid {
		if looksLikeNotionAPIToken(token) {
			t.Fatalf("expected invalid token format: %q", token)
		}
	}
}

func TestPrintOfficialAPITokenSetupHintIncludesURL(t *testing.T) {
	var out bytes.Buffer

	printOfficialAPITokenSetupHint(&out, false)

	text := out.String()
	if !strings.Contains(text, officialAPIIntegrationsURL) {
		t.Fatalf("hint should include integrations URL: %q", text)
	}
	if !strings.Contains(text, "copy the token from Configuration") {
		t.Fatalf("hint should explain where to find the token: %q", text)
	}
	if !strings.Contains(text, "Paste is hidden. Press Enter when done.") {
		t.Fatalf("hint should explain hidden paste behavior: %q", text)
	}
}

func TestPrintOfficialAPITokenSetupHintOpensBrowserWhenRequested(t *testing.T) {
	var out bytes.Buffer
	var openedURL string

	oldOpenBrowser := openOfficialAPIBrowser
	openOfficialAPIBrowser = func(url string) error {
		openedURL = url
		return nil
	}
	t.Cleanup(func() {
		openOfficialAPIBrowser = oldOpenBrowser
	})

	printOfficialAPITokenSetupHint(&out, true)

	if openedURL != officialAPIIntegrationsURL {
		t.Fatalf("opened URL = %q, want %q", openedURL, officialAPIIntegrationsURL)
	}
	if !strings.Contains(out.String(), "Opening that page in your browser") {
		t.Fatalf("expected browser open message in output: %q", out.String())
	}
}

func TestPrintOfficialAPITokenSetupHintReportsBrowserOpenFailure(t *testing.T) {
	var out bytes.Buffer

	oldOpenBrowser := openOfficialAPIBrowser
	openOfficialAPIBrowser = func(string) error {
		return errors.New("boom")
	}
	t.Cleanup(func() {
		openOfficialAPIBrowser = oldOpenBrowser
	})

	printOfficialAPITokenSetupHint(&out, true)

	if !strings.Contains(out.String(), "Could not open browser automatically: boom") {
		t.Fatalf("expected browser open failure in output: %q", out.String())
	}
}

func TestAuthAPIStatusJSONUsesLoadedConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_TOKEN", "env-token")

	var out bytes.Buffer
	oldOut := authAPIOutput
	authAPIOutput = &out
	t.Cleanup(func() {
		authAPIOutput = oldOut
	})

	cmd := &AuthAPIStatusCmd{JSON: true}
	if err := cmd.Run(&Context{Profile: "work", APIToken: "env-token"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), `"configured": true`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"profile": "work"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"token_source": "env"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestAuthAPIVerifyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/users/me" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"object":"user","id":"user_123","type":"bot","name":"Notion CLI","bot":{"workspace_name":"Workspace"}}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	oldOut := authAPIOutput
	authAPIOutput = &out
	t.Cleanup(func() {
		authAPIOutput = oldOut
	})

	cmd := &AuthAPIVerifyCmd{JSON: true}
	if err := cmd.Run(&Context{
		APIToken:   "env-token",
		APIBaseURL: srv.URL + "/v1",
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), `"verified": true`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"workspace_name": "Workspace"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestAuthAPISetupWarnsWhenTokenFormatLooksWrong(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var out bytes.Buffer
	oldIn := authAPIInput
	oldOut := authAPIOutput
	authAPIInput = strings.NewReader("not-a-notion-token\n")
	authAPIOutput = &out
	t.Cleanup(func() {
		authAPIInput = oldIn
		authAPIOutput = oldOut
	})

	cmd := &AuthAPISetupCmd{}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if !strings.Contains(stdout, "Official API token does not match the expected Notion token format") {
		t.Fatalf("expected format warning: %q", stdout)
	}
	if !strings.Contains(out.String(), "Expected format: ntn_<letters-and-numbers>") {
		t.Fatalf("expected token format hint: %q", out.String())
	}

	loaded, err := config.LoadWithMeta(config.APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.Token != "not-a-notion-token" {
		t.Fatalf("token = %q, want not-a-notion-token", loaded.Config.API.Token)
	}
}

func TestAuthAPIUnsetWarnsWhenEnvTokenStillActive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_TOKEN", "env-token")
	if err := config.SetAPIToken("stored-token"); err != nil {
		t.Fatalf("SetAPIToken: %v", err)
	}

	var out bytes.Buffer
	oldOut := authAPIOutput
	authAPIOutput = &out
	t.Cleanup(func() {
		authAPIOutput = oldOut
	})

	cmd := &AuthAPIUnsetCmd{}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{APIToken: "env-token"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	text := out.String()
	if !strings.Contains(stdout, "Saved official API token removed") {
		t.Fatalf("expected saved-token removal message: %q", stdout)
	}
	if !strings.Contains(text, "Effective token still comes from NOTION_API_TOKEN.") {
		t.Fatalf("expected env override note: %q", text)
	}

	loaded, err := config.LoadWithMeta(config.APIOverrides{Token: "env-token"})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.HasConfigToken {
		t.Fatalf("expected stored token to be removed")
	}
}

func TestAuthAPIUnsetWarnsWhenOnlyEnvTokenExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_TOKEN", "env-token")

	var out bytes.Buffer
	oldOut := authAPIOutput
	authAPIOutput = &out
	t.Cleanup(func() {
		authAPIOutput = oldOut
	})

	cmd := &AuthAPIUnsetCmd{}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{APIToken: "env-token"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	text := out.String()
	if !strings.Contains(stdout, "No saved official API token to remove") {
		t.Fatalf("expected no-saved-token warning: %q", stdout)
	}
	if !strings.Contains(text, "Effective token still comes from NOTION_API_TOKEN.") {
		t.Fatalf("expected env override note: %q", text)
	}

	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if _, err := os.Stat(cfgPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error: %v", err)
	}
}
