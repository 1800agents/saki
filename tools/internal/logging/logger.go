package logging

import (
	"fmt"
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

const defaultDebugLogPath = "/tmp/saki.log"

func New() *Logger {
	return NewWithWriter(defaultWriter(
		os.Stderr,
		os.Getenv,
		func(path string) (io.Writer, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		},
	))
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

func defaultWriter(
	stderr io.Writer,
	getenv func(string) string,
	openDebugLog func(path string) (io.Writer, error),
) io.Writer {
	if !debugLoggingEnabled(getenv) {
		return stderr
	}

	path := strings.TrimSpace(getenv("SAKI_TOOLS_LOG_PATH"))
	if path == "" {
		path = defaultDebugLogPath
	}

	fileWriter, err := openDebugLog(path)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open debug log file %q: %v\n", path, err)
		return stderr
	}

	return io.MultiWriter(stderr, fileWriter)
}

func debugLoggingEnabled(getenv func(string) string) bool {
	raw := firstNonEmpty(
		getenv("SAKI_TOOLS_DEBUG"),
		getenv("SAKI_TOOLS_MCP_DEBUG"),
	)
	if raw == "" {
		return true
	}
	return parseBool(raw)
}

func parseBool(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.EqualFold(trimmed, "1") || strings.EqualFold(trimmed, "true")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
