package template

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCloneFromPrepare(t *testing.T) {
	srcRepo := t.TempDir()
	writeFile(t, filepath.Join(srcRepo, "Dockerfile"), "FROM scratch\n")
	writeFile(t, filepath.Join(srcRepo, "README.md"), "# template\n")

	runCommand(t, "git", "-C", srcRepo, "init")
	runCommand(t, "git", "-C", srcRepo, "add", ".")
	runCommand(t, "git", "-C", srcRepo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "init")

	dest := filepath.Join(t.TempDir(), "app")
	err := CloneFromPrepare(context.Background(), PrepareResponse{
		TemplateRepository: srcRepo,
	}, dest)
	if err != nil {
		t.Fatalf("CloneFromPrepare() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "Dockerfile")); err != nil {
		t.Fatalf("expected cloned Dockerfile, got error: %v", err)
	}
}

func TestCloneFromPrepare_RequiresRepository(t *testing.T) {
	err := CloneFromPrepare(context.Background(), PrepareResponse{}, filepath.Join(t.TempDir(), "app"))
	if err == nil {
		t.Fatal("expected error for missing template repository")
	}
}

func TestWriteEnv_WritesOnlyNameAndDescription(t *testing.T) {
	appDir := t.TempDir()
	writeFile(t, filepath.Join(appDir, ".env"), "EXTRA=1\n")

	if err := WriteEnv(appDir, "my-app", "Internal app"); err != nil {
		t.Fatalf("WriteEnv() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(appDir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}

	want := "NAME=my-app\nDESCRIPTION=Internal app\n"
	if string(got) != want {
		t.Fatalf("unexpected .env content:\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}

func TestWriteEnv_RejectsMultilineValues(t *testing.T) {
	appDir := t.TempDir()
	if err := WriteEnv(appDir, "my-app", "line1\nline2"); err == nil {
		t.Fatal("expected error for multiline description")
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func runCommand(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("command %q failed: %v\noutput: %s", cmd.String(), err, string(output))
	}
}
