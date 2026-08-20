package observability

import (
	"io"
	"log/slog"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
)

// NewServiceLogger returns the shared daemon JSON logger with release-identity
// baseline attrs and fail-closed redaction for sensitive attribute keys.
//
// The logger never includes DSNs, passwords, tickets, or similar secrets as
// attribute values. Callers must still avoid embedding those secrets inside
// free-form message text they control.
func NewServiceLogger(serviceName string, w io.Writer) *slog.Logger {
	identity := buildinfo.Current()
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		ReplaceAttr: redactSensitiveAttr,
	})
	return slog.New(handler).With(
		"service", serviceName,
		"version", identity.Version,
		"commit", identity.Commit,
		"build_date", identity.BuildDate,
	)
}

func redactSensitiveAttr(_ []string, attr slog.Attr) slog.Attr {
	if isSensitiveAttrKey(attr.Key) {
		return slog.String(attr.Key, "<redacted>")
	}
	return attr
}

func isSensitiveAttrKey(key string) bool {
	normalized := normalizeAttrKey(key)
	for _, sensitive := range []string{"dsn", "password", "secret", "token", "ticket", "loginkey", "apikey"} {
		if normalized == sensitive || strings.HasSuffix(normalized, sensitive) {
			return true
		}
	}
	return false
}

func normalizeAttrKey(key string) string {
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range strings.ToLower(strings.TrimSpace(key)) {
		if r == '-' || r == '_' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
