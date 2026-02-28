package apperrors

import (
	"errors"
	"testing"
)

func TestWrapAndCodeOf(t *testing.T) {
	base := errors.New("boom")
	err := Wrap(CodeDocker, "docker build", base)

	if got := CodeOf(err); got != CodeDocker {
		t.Fatalf("expected %q, got %q", CodeDocker, got)
	}

	var appErr *Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected wrapped *Error")
	}
	if appErr.Op != "docker build" {
		t.Fatalf("unexpected op: %q", appErr.Op)
	}
}

func TestCodeOfUnknownErrorDefaultsInternal(t *testing.T) {
	if got := CodeOf(errors.New("plain")); got != CodeInternal {
		t.Fatalf("expected %q, got %q", CodeInternal, got)
	}
}
