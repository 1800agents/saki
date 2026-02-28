package docker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLogin_UsesPasswordStdinAndRedactsLogs(t *testing.T) {
	runner := &stubRunner{}
	logger := &captureLogger{}
	adapter := NewAdapter(logger, runner)

	registry := "https://user:supersecret@registry.internal"
	if err := adapter.Login(context.Background(), registry, "robot", "password-123"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if runner.last.Name != "docker" {
		t.Fatalf("expected docker command, got %q", runner.last.Name)
	}
	wantArgs := []string{"login", registry, "--username", "robot", "--password-stdin"}
	if strings.Join(runner.last.Args, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("unexpected args: got %v want %v", runner.last.Args, wantArgs)
	}
	if runner.last.Stdin != "password-123\n" {
		t.Fatalf("expected password newline in stdin, got %q", runner.last.Stdin)
	}

	cmd := logger.lastCommand(t)
	if strings.Contains(cmd, "supersecret") {
		t.Fatalf("log command leaked registry credential: %q", cmd)
	}
	if strings.Contains(cmd, "password-123") {
		t.Fatalf("log command leaked password: %q", cmd)
	}
}

func TestBuild_SetsWorkingDirectory(t *testing.T) {
	runner := &stubRunner{}
	adapter := NewAdapter(nil, runner)

	if err := adapter.Build(context.Background(), "/tmp/app", "registry.internal/me/app:123"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if runner.last.Dir != "/tmp/app" {
		t.Fatalf("expected work dir to be set, got %q", runner.last.Dir)
	}
	if got := strings.Join(runner.last.Args, " "); got != "build -t registry.internal/me/app:123 ." {
		t.Fatalf("unexpected build args: %q", got)
	}
}

func TestPush_ReturnsStructuredCommandError(t *testing.T) {
	runner := &stubRunner{
		result: CommandResult{ExitCode: 1, Stderr: "denied"},
		err:    errors.New("exit status 1"),
	}
	adapter := NewAdapter(nil, runner)

	err := adapter.Push(context.Background(), "registry.internal/me/app:123")
	if err == nil {
		t.Fatalf("expected error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T", err)
	}

	if cmdErr.Op != "push" {
		t.Fatalf("expected op push, got %q", cmdErr.Op)
	}
	if cmdErr.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", cmdErr.ExitCode)
	}
	if cmdErr.Stderr != "denied" {
		t.Fatalf("expected stderr denied, got %q", cmdErr.Stderr)
	}
}

type stubRunner struct {
	last   CommandRequest
	result CommandResult
	err    error
}

func (s *stubRunner) Run(_ context.Context, req CommandRequest) (CommandResult, error) {
	s.last = req
	return s.result, s.err
}

type logEntry struct {
	message string
	fields  map[string]any
}

type captureLogger struct {
	entries []logEntry
}

func (c *captureLogger) Info(msg string, fields map[string]any) {
	c.entries = append(c.entries, logEntry{message: msg, fields: fields})
}

func (c *captureLogger) Error(msg string, fields map[string]any) {
	c.entries = append(c.entries, logEntry{message: msg, fields: fields})
}

func (c *captureLogger) lastCommand(t *testing.T) string {
	t.Helper()
	if len(c.entries) == 0 {
		t.Fatalf("expected at least one log entry")
	}

	v, ok := c.entries[len(c.entries)-1].fields["command"]
	if !ok {
		t.Fatalf("expected command field in log")
	}

	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected command field to be string")
	}
	return s
}
