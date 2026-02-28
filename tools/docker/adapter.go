package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"

	"github.com/1800agents/saki/tools/internal/apperrors"
)

// Logger receives structured log events from the Docker adapter.
type Logger interface {
	Info(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
}

// CommandRunner runs shell commands and returns process output.
type CommandRunner interface {
	Run(ctx context.Context, req CommandRequest) (CommandResult, error)
}

// CommandRequest describes a command execution.
type CommandRequest struct {
	Name  string
	Args  []string
	Dir   string
	Stdin string
}

// CommandResult captures command output and exit information.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Adapter wraps Docker CLI actions used by the deploy flow.
type Adapter struct {
	runner CommandRunner
	logger Logger
}

// CommandError is a structured error from a failed Docker command.
type CommandError struct {
	Op       string
	Command  string
	ExitCode int
	Stderr   string
	Err      error
}

func (e *CommandError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.ExitCode >= 0 {
		return fmt.Sprintf("docker %s failed (exit=%d): %v", e.Op, e.ExitCode, e.Err)
	}
	return fmt.Sprintf("docker %s failed: %v", e.Op, e.Err)
}

func (e *CommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *CommandError) ErrorCode() apperrors.Code {
	if e != nil && errors.Is(e.Err, context.DeadlineExceeded) {
		return apperrors.CodeTimeout
	}
	return apperrors.CodeDocker
}

// NewAdapter creates a Docker CLI adapter with optional logger/runner overrides.
func NewAdapter(logger Logger, runner CommandRunner) *Adapter {
	if logger == nil {
		logger = noopLogger{}
	}
	if runner == nil {
		runner = execRunner{}
	}

	return &Adapter{runner: runner, logger: logger}
}

// Login runs `docker login` using stdin for the password.
func (a *Adapter) Login(ctx context.Context, registry, username, password string) error {
	stdin := password
	if !strings.HasSuffix(stdin, "\n") {
		stdin += "\n"
	}

	return a.run(ctx, "login", CommandRequest{
		Name:  "docker",
		Args:  []string{"login", registry, "--username", username, "--password-stdin"},
		Stdin: stdin,
	})
}

// Build runs `docker build -t <image> .` in workDir.
func (a *Adapter) Build(ctx context.Context, workDir, image string) error {
	return a.run(ctx, "build", CommandRequest{
		Name: "docker",
		Args: []string{"build", "-t", image, "."},
		Dir:  workDir,
	})
}

// Push runs `docker push <image>`.
func (a *Adapter) Push(ctx context.Context, image string) error {
	return a.run(ctx, "push", CommandRequest{
		Name: "docker",
		Args: []string{"push", image},
	})
}

func (a *Adapter) run(ctx context.Context, op string, req CommandRequest) error {
	redacted := redactedCommand(req.Name, req.Args)
	a.logger.Info("docker command", map[string]any{
		"op":      op,
		"command": redacted,
	})

	res, err := a.runner.Run(ctx, req)
	if err == nil {
		return nil
	}

	cmdErr := &CommandError{
		Op:       op,
		Command:  redacted,
		ExitCode: res.ExitCode,
		Stderr:   strings.TrimSpace(res.Stderr),
		Err:      err,
	}

	a.logger.Error("docker command failed", map[string]any{
		"op":        op,
		"command":   redacted,
		"exit_code": cmdErr.ExitCode,
		"stderr":    cmdErr.Stderr,
	})

	return cmdErr
}

func redactedCommand(name string, args []string) string {
	clean := make([]string, 0, len(args)+1)
	clean = append(clean, name)

	for i := range args {
		if shouldRedactArg(args, i) {
			clean = append(clean, "<redacted>")
			continue
		}
		clean = append(clean, redactURLUserInfo(args[i]))
	}

	return strings.Join(clean, " ")
}

func shouldRedactArg(args []string, idx int) bool {
	arg := strings.ToLower(args[idx])

	if idx > 0 {
		prev := strings.ToLower(args[idx-1])
		if prev == "--password" || prev == "-p" {
			return true
		}
	}

	return strings.Contains(arg, "token=") ||
		strings.Contains(arg, "password=") ||
		strings.Contains(arg, "passwd=") ||
		strings.Contains(arg, "secret=")
}

func redactURLUserInfo(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}

	u.User = nil
	return u.String()
}

type noopLogger struct{}

func (noopLogger) Info(string, map[string]any)  {}
func (noopLogger) Error(string, map[string]any) {}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, req CommandRequest) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, req.Name, req.Args...)
	cmd.Dir = req.Dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	} else {
		cmd.Stdin = io.Reader(nil)
	}

	err := cmd.Run()
	result := CommandResult{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}

	if err == nil {
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}

	return result, err
}
