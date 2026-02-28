package logging

import (
	"io"
	"os"
	"sort"
	"strings"

	"log/slog"
)

// Logger wraps slog.Logger with map-based helper methods used by adapters.
type Logger struct {
	logger *slog.Logger
}

func New() *Logger {
	return NewWithWriter(io.Writer(os.Stderr))
}

func NewWithWriter(w io.Writer) *Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Value.Kind() != slog.KindString {
				return a
			}
			return slog.String(a.Key, redactSecrets(a.Value.String()))
		},
	})

	return &Logger{
		logger: slog.New(handler),
	}
}

func (l *Logger) Slog() *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l.logger
}

func (l *Logger) Info(msg string, fields map[string]any) {
	l.Slog().Info(msg, attrs(fields)...)
}

func (l *Logger) Error(msg string, fields map[string]any) {
	l.Slog().Error(msg, attrs(fields)...)
}

func attrs(fields map[string]any) []any {
	if len(fields) == 0 {
		return nil
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]any, 0, len(keys)*2)
	for _, key := range keys {
		value := fields[key]
		if s, ok := value.(string); ok {
			value = redactSecrets(s)
		}
		out = append(out, key, value)
	}

	return out
}

func redactSecrets(s string) string {
	replacements := []struct {
		key string
	}{
		{key: "token="},
		{key: "password="},
		{key: "passwd="},
		{key: "secret="},
	}

	redacted := s
	for _, item := range replacements {
		redacted = redactValue(redacted, item.key)
	}

	return redacted
}

func redactValue(input, key string) string {
	lower := strings.ToLower(input)
	search := strings.ToLower(key)
	start := 0

	for {
		idx := strings.Index(lower[start:], search)
		if idx < 0 {
			break
		}

		tokenStart := start + idx + len(search)
		tokenEnd := tokenStart
		for tokenEnd < len(input) {
			switch input[tokenEnd] {
			case '&', ' ', '\t', '\n', '\r':
				goto done
			default:
				tokenEnd++
			}
		}
	done:
		input = input[:tokenStart] + "<redacted>" + input[tokenEnd:]
		lower = strings.ToLower(input)
		start = tokenStart + len("<redacted>")
	}

	return input
}
