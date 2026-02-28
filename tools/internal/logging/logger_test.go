package logging

import (
	"bytes"
	"errors"
	"io"
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

func TestDefaultWriter_DebugOnByDefaultWritesToFile(t *testing.T) {
	var stderr bytes.Buffer
	var file bytes.Buffer
	var openedPath string

	writer := defaultWriter(
		&stderr,
		func(string) string { return "" },
		func(path string) (io.Writer, error) {
			openedPath = path
			return &file, nil
		},
	)

	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if openedPath != defaultDebugLogPath {
		t.Fatalf("expected default log path %q, got %q", defaultDebugLogPath, openedPath)
	}
	if !strings.Contains(stderr.String(), "hello") {
		t.Fatalf("expected stderr to receive logs, got %q", stderr.String())
	}
	if !strings.Contains(file.String(), "hello") {
		t.Fatalf("expected file writer to receive logs, got %q", file.String())
	}
}

func TestDefaultWriter_DebugOffSkipsFile(t *testing.T) {
	var stderr bytes.Buffer
	opened := false

	writer := defaultWriter(
		&stderr,
		func(key string) string {
			if key == "SAKI_TOOLS_DEBUG" {
				return "false"
			}
			return ""
		},
		func(string) (io.Writer, error) {
			opened = true
			return nil, errors.New("should not open")
		},
	)

	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if opened {
		t.Fatal("expected debug log file not to be opened when debug is disabled")
	}
}

func TestDefaultWriter_UsesCustomPath(t *testing.T) {
	var stderr bytes.Buffer
	var file bytes.Buffer
	var openedPath string

	writer := defaultWriter(
		&stderr,
		func(key string) string {
			if key == "SAKI_TOOLS_LOG_PATH" {
				return "/tmp/custom-saki.log"
			}
			return ""
		},
		func(path string) (io.Writer, error) {
			openedPath = path
			return &file, nil
		},
	)

	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if openedPath != "/tmp/custom-saki.log" {
		t.Fatalf("expected custom log path, got %q", openedPath)
	}
}
