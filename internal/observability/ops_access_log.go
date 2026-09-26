package observability

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WrapOpsAccessLog returns an http.Handler that emits one metadata-only JSON
// access line for each /local/* request after the downstream handler returns.
//
// Non-/local/ paths (including /healthz and /debug/pprof/*) are never logged.
// Query strings, fragments, and request/response bodies are never logged.
// A nil logger or nil next is a passthrough (nil next remains nil).
//
// When next is the gamed ops mux, the wrapper also records the completed
// request on the OpsMetrics document mounted at LocalMetricsPath. Reading
// that document does not count itself. authd has no mount, so it is not
// counted. Bodies are never read.
func WrapOpsAccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	if next == nil {
		return nil
	}
	metrics := mountedOpsMetrics(next)
	if logger == nil && metrics == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.URL == nil || !strings.HasPrefix(r.URL.Path, "/local/") {
			next.ServeHTTP(w, r)
			return
		}

		recorder := &opsAccessResponseRecorder{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r)
		if metrics != nil && r.URL.Path != LocalMetricsPath {
			metrics.ObserveLocalRequest(r.Method, r.URL.Path, recorder.status)
		}
		if logger == nil {
			return
		}
		elapsed := time.Since(started)
		durationMS := elapsed.Milliseconds()
		if durationMS < 0 {
			durationMS = 0
		}

		logger.Info("ops local request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"status", recorder.status,
			"duration_ms", durationMS,
		)
	})
}

// MountLocalMetrics serves the already-owned loopback JSON document and marks
// the handler so WrapOpsAccessLog can record other /local/<name> requests on
// the same snapshot. A nil metrics returns nil.
func MountLocalMetrics(metrics *OpsMetrics) http.Handler {
	if metrics == nil {
		return nil
	}
	return &localMetricsMount{metrics: metrics, handler: metrics.Handler()}
}

func mountedOpsMetrics(next http.Handler) *OpsMetrics {
	mux, ok := next.(*http.ServeMux)
	if !ok || mux == nil {
		return nil
	}
	handler, pattern := mux.Handler(&http.Request{
		Method:     http.MethodGet,
		URL:        localMetricsURL(),
		RemoteAddr: "127.0.0.1:1",
	})
	if pattern != LocalMetricsPath {
		return nil
	}
	mount, ok := handler.(*localMetricsMount)
	if !ok || mount == nil {
		return nil
	}
	return mount.metrics
}

func localMetricsURL() *url.URL {
	return &url.URL{Path: LocalMetricsPath}
}

type localMetricsMount struct {
	metrics *OpsMetrics
	handler http.Handler
}

func (m *localMetricsMount) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.handler == nil {
		http.NotFound(w, r)
		return
	}
	m.handler.ServeHTTP(w, r)
}

type opsAccessResponseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *opsAccessResponseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *opsAccessResponseRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(p)
}
