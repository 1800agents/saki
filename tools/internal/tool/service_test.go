package tool

import "testing"

func TestResolveTemplateRepository(t *testing.T) {
	t.Run("uses prepare repository when provided", func(t *testing.T) {
		got := resolveTemplateRepository("https://example.com/prepare.git", "https://example.com/env.git")
		if got != "https://example.com/prepare.git" {
			t.Fatalf("expected prepare repository, got %q", got)
		}
	})

	t.Run("falls back to env repository when prepare repository is empty", func(t *testing.T) {
		got := resolveTemplateRepository(" ", "https://example.com/env.git")
		if got != "https://example.com/env.git" {
			t.Fatalf("expected env repository, got %q", got)
		}
	})

	t.Run("falls back to default repository when neither prepare nor env repository is set", func(t *testing.T) {
		got := resolveTemplateRepository(" ", " ")
		if got != defaultTemplateRepository {
			t.Fatalf("expected default repository %q, got %q", defaultTemplateRepository, got)
		}
	})
}

func TestFirstNonEmpty(t *testing.T) {
	got := firstNonEmpty(" ", "\n", "value", "later")
	if got != "value" {
		t.Fatalf("expected first non-empty value, got %q", got)
	}

	got = firstNonEmpty(" ", "\n")
	if got != "" {
		t.Fatalf("expected empty string when all values are empty, got %q", got)
	}
}
