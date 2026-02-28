package template

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1800agents/saki/tools/internal/apperrors"
)

const envFileName = ".env"

// PrepareResponse captures the template location returned by the prepare API.
type PrepareResponse struct {
	TemplateRepository string
	TemplateRef        string
}

// CloneFromPrepare clones the template repository into destinationDir.
func CloneFromPrepare(ctx context.Context, prepare PrepareResponse, destinationDir string) error {
	if strings.TrimSpace(prepare.TemplateRepository) == "" {
		return apperrors.New(apperrors.CodeInvalidInput, "clone template", "template repository is required")
	}

	if strings.TrimSpace(destinationDir) == "" {
		return apperrors.New(apperrors.CodeInvalidInput, "clone template", "destination directory is required")
	}

	cloneCmd := exec.CommandContext(
		ctx,
		"git",
		"clone",
		"--depth",
		"1",
		"--",
		prepare.TemplateRepository,
		destinationDir,
	)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		wrapped := fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		return apperrors.Wrap(apperrors.CodeTemplate, "clone template", wrapped)
	}

	if strings.TrimSpace(prepare.TemplateRef) != "" {
		checkoutCmd := exec.CommandContext(ctx, "git", "-C", destinationDir, "checkout", "--detach", prepare.TemplateRef)
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			wrapped := fmt.Errorf("ref %q: %w: %s", prepare.TemplateRef, err, strings.TrimSpace(string(output)))
			return apperrors.Wrap(apperrors.CodeTemplate, "checkout template", wrapped)
		}
	}

	return nil
}

// WriteEnv writes the app .env file with only NAME and DESCRIPTION keys.
func WriteEnv(appDir, name, description string) error {
	if strings.TrimSpace(appDir) == "" {
		return apperrors.New(apperrors.CodeInvalidInput, "write env", "app directory is required")
	}

	if strings.ContainsAny(name, "\r\n") {
		return apperrors.New(apperrors.CodeInvalidInput, "write env", "name cannot contain newlines")
	}

	if strings.ContainsAny(description, "\r\n") {
		return apperrors.New(apperrors.CodeInvalidInput, "write env", "description cannot contain newlines")
	}

	envContent := fmt.Sprintf("NAME=%s\nDESCRIPTION=%s\n", name, description)
	envPath := filepath.Join(appDir, envFileName)

	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		return apperrors.Wrap(apperrors.CodeTemplate, "write env", fmt.Errorf("write %s: %w", envFileName, err))
	}

	return nil
}
