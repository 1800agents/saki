package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestRedactsTokenAndPassword(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf)

	logger.Info("control plane request", map[string]any{
		"url":     "https://saki.internal/api?token=abc123&v=1",
		"details": "password=hunter2",
	})

	line := buf.String()
	if strings.Contains(line, "abc123") {
		t.Fatalf("token leaked in log line: %s", line)
	}
	if strings.Contains(line, "hunter2") {
		t.Fatalf("password leaked in log line: %s", line)
	}
	if !strings.Contains(line, "token=<redacted>") {
		t.Fatalf("expected redacted token, got: %s", line)
	}
	if !strings.Contains(line, "password=<redacted>") {
		t.Fatalf("expected redacted password, got: %s", line)
	}
}
