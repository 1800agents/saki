package template

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		return errors.New("template repository is required")
	}

	if strings.TrimSpace(destinationDir) == "" {
		return errors.New("destination directory is required")
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
		return fmt.Errorf("clone template: %w: %s", err, strings.TrimSpace(string(output)))
	}

	if strings.TrimSpace(prepare.TemplateRef) != "" {
		checkoutCmd := exec.CommandContext(ctx, "git", "-C", destinationDir, "checkout", "--detach", prepare.TemplateRef)
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("checkout template ref %q: %w: %s", prepare.TemplateRef, err, strings.TrimSpace(string(output)))
		}
	}

	return nil
}

// WriteEnv writes the app .env file with only NAME and DESCRIPTION keys.
func WriteEnv(appDir, name, description string) error {
	if strings.TrimSpace(appDir) == "" {
		return errors.New("app directory is required")
	}

	if strings.ContainsAny(name, "\r\n") {
		return errors.New("name cannot contain newlines")
	}

	if strings.ContainsAny(description, "\r\n") {
		return errors.New("description cannot contain newlines")
	}

	envContent := fmt.Sprintf("NAME=%s\nDESCRIPTION=%s\n", name, description)
	envPath := filepath.Join(appDir, envFileName)

	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", envFileName, err)
	}

	return nil
}
