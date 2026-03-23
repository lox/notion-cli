package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPrintAuthRefreshGuidanceWritesToStderr(t *testing.T) {
	var buf bytes.Buffer
	oldWriter := authRefreshNoticeWriter
	authRefreshNoticeWriter = &buf
	t.Cleanup(func() {
		authRefreshNoticeWriter = oldWriter
	})

	printAuthRefreshGuidance(errors.New("token expired and no refresh token available"))

	out := buf.String()
	if !strings.Contains(out, "Auth token refresh skipped") {
		t.Fatalf("expected guidance prefix in output, got: %q", out)
	}

	if !strings.Contains(out, "token expired and no refresh token available") {
		t.Fatalf("expected underlying error details in output, got: %q", out)
	}

	if !strings.Contains(out, "notion-cli auth status") {
		t.Fatalf("expected actionable command guidance in output, got: %q", out)
	}
}
