package observability

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"

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

// LocalMetricsPath is the loopback-only JSON metrics companion that sits
// beside daemon JSON logging and metadata-only /local/* access logs.
//
// It is not a Prometheus exporter, not an OpenTelemetry endpoint, and not a
// remote admin surface. Daemons do not register it until a later slice mounts
// Handler on the ops mux. Loopback trace spans live on OpsTrace
// (LocalTracePath), not here.
const LocalMetricsPath = "/local/metrics"

const maxMetricsPathLen = 128

// OpsMetricsSnapshot is the metadata-only counters document returned by
// GET LocalMetricsPath. It never includes remote addresses, query strings,
// header values, bodies, or secrets.
type OpsMetricsSnapshot struct {
	Service             string            `json:"service"`
	LocalRequestsTotal  uint64            `json:"local_requests_total"`
	LocalErrorsTotal    uint64            `json:"local_errors_total"`
	LocalRequestsByPath map[string]uint64 `json:"local_requests_by_path"`
}

// OpsMetrics counts completed /local/* HTTP requests. Nil receivers are safe
// no-ops so callers can wire the companion beside an optional process logger.
type OpsMetrics struct {
	mu      sync.Mutex
	service string
	total   uint64
	errors  uint64
	byPath  map[string]uint64
}

// NewOpsMetrics returns an empty counter set. Service stays blank until
// SetService names the daemon ("authd" or "gamed").
func NewOpsMetrics() *OpsMetrics {
	return &OpsMetrics{byPath: map[string]uint64{}}
}

// SetService records the daemon name included in every snapshot. Empty and
// whitespace-only names are ignored. The value is metadata only.
func (m *OpsMetrics) SetService(service string) {
	if m == nil {
		return
	}
	service = strings.TrimSpace(service)
	if service == "" || len(service) > 64 {
		return
	}
	m.mu.Lock()
	m.service = service
	m.mu.Unlock()
}

// ObserveLocalRequest counts one completed /local/* request.
//
// Non-/local/ paths, including /healthz and /debug/pprof/*, are ignored.
// Only URL.Path is retained, after dropping raw query text and rejecting
// paths that are not a single clean /local/<name> segment. Status >= 400
// also increments the error counter. Method and remote address are not stored.
func (m *OpsMetrics) ObserveLocalRequest(method, path string, status int) {
	if m == nil {
		return
	}
	_ = method
	clean, ok := metricsPathKey(path)
	if !ok {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byPath == nil {
		m.byPath = map[string]uint64{}
	}
	m.total++
	m.byPath[clean]++
	if status >= 400 {
		m.errors++
	}
}

// Snapshot copies the current counters. Callers must not mutate the returned map.
func (m *OpsMetrics) Snapshot() OpsMetricsSnapshot {
	if m == nil {
		return OpsMetricsSnapshot{LocalRequestsByPath: map[string]uint64{}}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	byPath := make(map[string]uint64, len(m.byPath))
	for path, count := range m.byPath {
		byPath[path] = count
	}
	return OpsMetricsSnapshot{
		Service:             m.service,
		LocalRequestsTotal:  m.total,
		LocalErrorsTotal:    m.errors,
		LocalRequestsByPath: byPath,
	}
}

// Handler serves GET LocalMetricsPath to loopback callers only.
//
// Non-GET methods return 405 with an empty body. Non-loopback callers return
// 403 with an empty body. The JSON document is Snapshot and never echoes the
// request URL, query, or body.
func (m *OpsMetrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !metricsLoopback(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		snap := m.Snapshot()
		if snap.LocalRequestsByPath == nil {
			snap.LocalRequestsByPath = map[string]uint64{}
		}
		encoded, err := json.Marshal(snap)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encoded)
	})
}

// Wrap returns a handler that records completed /local/* responses on m and
// leaves the downstream handler unchanged.
//
// A nil receiver returns next. A nil next stays nil. /healthz, /debug/pprof/*,
// and every other non-/local/ path are not counted. Bodies are never read.
func (m *OpsMetrics) Wrap(next http.Handler) http.Handler {
	if m == nil || next == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.URL == nil || !strings.HasPrefix(r.URL.Path, "/local/") {
			next.ServeHTTP(w, r)
			return
		}
		recorder := &opsAccessResponseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		m.ObserveLocalRequest(r.Method, r.URL.Path, recorder.status)
	})
}

func metricsPathKey(raw string) (string, bool) {
	path, _, _ := strings.Cut(raw, "?")
	path, _, _ = strings.Cut(path, "#")
	if !strings.HasPrefix(path, "/local/") || strings.Contains(path, "..") {
		return "", false
	}
	if len(path) > maxMetricsPathLen {
		return "", false
	}
	rest := strings.TrimPrefix(path, "/local/")
	if rest == "" || strings.ContainsAny(rest, "/\\?# \t") {
		return "", false
	}
	for _, r := range rest {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return "", false
		}
	}
	return path, true
}

func metricsLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
