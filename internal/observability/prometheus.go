package observability

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// LocalPrometheusPath is the loopback-only Prometheus text exposition that
// sits beside GET /local/metrics.
//
// It is not mounted on authd or gamed. Operators cannot curl it on a running
// daemon until a later slice registers Handler on the ops mux. It is not an
// OpenTelemetry exporter, not remote log shipping, and not a remote admin
// surface. With no loopback exporter configured, Handler refuses the document.
const LocalPrometheusPath = "/local/metrics/prometheus"

// LoopbackPrometheusExporter is the only config this slice accepts. A nil
// exporter, any other name, or a non-loopback endpoint is fail-closed: the
// text document is not served.
type LoopbackPrometheusExporter struct {
	Endpoint string
}

// Configured reports whether Endpoint is an explicit loopback HTTP target.
// Empty config and every remote or wildcard target stay false.
func (e LoopbackPrometheusExporter) Configured() bool {
	return loopbackPrometheusEndpoint(e.Endpoint)
}

// SetPrometheusExporter accepts only a configured loopback exporter. A nil
// argument or any other endpoint clears the exporter and leaves later
// exposition calls fail-closed. JSON /local/metrics is unchanged.
func (m *OpsMetrics) SetPrometheusExporter(exporter *LoopbackPrometheusExporter) {
	if m == nil {
		return
	}
	var next LoopbackPrometheusExporter
	if exporter != nil && exporter.Configured() {
		next = *exporter
	}
	m.mu.Lock()
	m.prometheus = next
	m.mu.Unlock()
}

// PrometheusConfigured reports whether a loopback exporter is set. Missing
// config stays false.
func (m *OpsMetrics) PrometheusConfigured() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prometheus.Configured()
}

// PrometheusText renders the current counters as Prometheus text only when a
// loopback exporter is configured. Missing or remote config returns ok=false
// and an empty body. The text never includes query strings, bodies, remote
// addresses, or the exporter endpoint.
func (m *OpsMetrics) PrometheusText() (text string, ok bool) {
	if m == nil || !m.PrometheusConfigured() {
		return "", false
	}
	snap := m.Snapshot()
	var b strings.Builder
	service := prometheusLabelValue(snap.Service)
	writeCounter(&b, "metin2_ops_local_requests_total", "Completed loopback /local requests.", service, snap.LocalRequestsTotal)
	writeCounter(&b, "metin2_ops_local_errors_total", "Completed loopback /local requests with HTTP status >= 400.", service, snap.LocalErrorsTotal)
	b.WriteString("# HELP metin2_ops_local_requests_by_path Completed loopback /local requests by path.\n")
	b.WriteString("# TYPE metin2_ops_local_requests_by_path counter\n")
	paths := make([]string, 0, len(snap.LocalRequestsByPath))
	for path := range snap.LocalRequestsByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		b.WriteString("metin2_ops_local_requests_by_path{service=\"")
		b.WriteString(service)
		b.WriteString("\",path=\"")
		b.WriteString(prometheusLabelValue(path))
		b.WriteString("\"} ")
		b.WriteString(strconv.FormatUint(snap.LocalRequestsByPath[path], 10))
		b.WriteByte('\n')
	}
	return b.String(), true
}

// PrometheusHandler serves GET LocalPrometheusPath to loopback callers only
// when a loopback exporter is configured.
//
// Missing exporter config, non-GET methods, and non-loopback callers all
// return an empty body (404, 405, and 403). The text never echoes the request
// URL, query, or body. A nil receiver still returns a handler that stays
// fail-closed.
func (m *OpsMetrics) PrometheusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !metricsLoopback(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		text, ok := m.PrometheusText()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(text))
	})
}

func writeCounter(b *strings.Builder, name, help, service string, value uint64) {
	b.WriteString("# HELP ")
	b.WriteString(name)
	b.WriteByte(' ')
	b.WriteString(help)
	b.WriteByte('\n')
	b.WriteString("# TYPE ")
	b.WriteString(name)
	b.WriteString(" counter\n")
	b.WriteString(name)
	b.WriteString("{service=\"")
	b.WriteString(service)
	b.WriteString("\"} ")
	b.WriteString(strconv.FormatUint(value, 10))
	b.WriteByte('\n')
}

func prometheusLabelValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch r {
		case '\\', '"', '\n':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func loopbackPrometheusEndpoint(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, " ") || strings.ContainsAny(raw, "?#") {
		return false
	}
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") {
		return false
	}
	rest := raw[len("http://"):]
	hostport, path, _ := strings.Cut(rest, "/")
	if hostport == "" || path != "" && path != "metrics" {
		return false
	}
	return metricsLoopback(hostport)
}
