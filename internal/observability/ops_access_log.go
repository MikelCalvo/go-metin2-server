package observability

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AccessLogSampler is an opt-in rate limit for metadata-only /local/* JSON
// access lines. A nil sampler, a non-positive interval, or any other unset
// config stays disabled: every eligible request still emits one line.
//
// The sampler never stores a path, query, body, header, or remote address.
// It is not a Prometheus exporter, not an OpenTelemetry exporter, and not a
// remote log sink. Metrics and trace recording stay on every request.
type AccessLogSampler struct {
	// Every is the minimum wall-clock gap between emitted access lines.
	// Zero and negative values stay disabled.
	Every time.Duration
}

// Configured reports whether Every is a positive interval. Missing config
// stays false so the current every-request line is unchanged.
func (s AccessLogSampler) Configured() bool {
	return s.Every > 0
}

var (
	accessLogSampleMu       sync.Mutex
	accessLogSampleEvery    time.Duration
	accessLogSampleLastEmit time.Time
)

// SetAccessLogSampler accepts only a positive interval. A nil argument or a
// non-positive Every clears the sampler. Missing config stays on the current
// every-request access line. The interval is never written into a log record.
//
// Setting the flag does not attach a logger. Only WrapOpsAccessLog consults
// it, and only for the JSON access line.
func SetAccessLogSampler(sampler *AccessLogSampler) {
	accessLogSampleMu.Lock()
	defer accessLogSampleMu.Unlock()
	accessLogSampleEvery = 0
	accessLogSampleLastEmit = time.Time{}
	if sampler == nil || !sampler.Configured() {
		return
	}
	accessLogSampleEvery = sampler.Every
}

// AccessLogSamplerConfigured reports whether a positive interval is set.
// Missing config stays false.
func AccessLogSamplerConfigured() bool {
	accessLogSampleMu.Lock()
	defer accessLogSampleMu.Unlock()
	return accessLogSampleEvery > 0
}

// accessLogLineAllowed reports whether this request may emit one JSON access
// line. Missing sampler config always allows the line. An enabled sampler
// allows the first line and then at most one line per interval. The decision
// does not look at the request.
func accessLogLineAllowed(now time.Time) bool {
	accessLogSampleMu.Lock()
	defer accessLogSampleMu.Unlock()
	if accessLogSampleEvery <= 0 {
		return true
	}
	if !accessLogSampleLastEmit.IsZero() && now.Sub(accessLogSampleLastEmit) < accessLogSampleEvery {
		return false
	}
	accessLogSampleLastEmit = now
	return true
}

// WrapOpsAccessLog returns an http.Handler that emits one metadata-only JSON
// access line for each /local/* request after the downstream handler returns.
//
// Non-/local/ paths (including /healthz and /debug/pprof/*) are never logged.
// Query strings, fragments, and request/response bodies are never logged.
// A nil logger or nil next is a passthrough (nil next remains nil).
//
// Missing sampler config keeps that every-request line. A positive
// SetAccessLogSampler interval drops later lines inside the window and does
// not change which requests run.
//
// When next is the gamed ops mux, the wrapper also records the completed
// request on the OpsMetrics document mounted at LocalMetricsPath and one
// in-memory span on the OpsTrace document mounted at LocalTracePath.
// Reading either document does not record itself. The Prometheus text
// mount at LocalPrometheusPath is multi-segment, so metricsPathKey already
// leaves it uncounted and untraced. authd has no mount, so it is not
// counted or traced. Bodies are never read. Sampling does not skip those
// records. Missing trace exporter config stays fail-closed: the span stays
// in memory. Missing Prometheus exporter config stays fail-closed too.
func WrapOpsAccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	if next == nil {
		return nil
	}
	metrics := mountedOpsMetrics(next)
	trace := mountedOpsTrace(next)
	if logger == nil && metrics == nil && trace == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.URL == nil || !strings.HasPrefix(r.URL.Path, "/local/") {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		recordTrace := false
		if trace != nil && r.URL.Path != LocalTracePath {
			if _, ok := metricsPathKey(r.URL.Path); ok {
				recordTrace = true
			}
		}
		if recordTrace {
			ctx, _ = trace.StartSpan(ctx, otelTraceSpanName, map[string]string{
				"http.method": r.Method,
				"http.target": r.URL.Path,
			})
			r = r.WithContext(ctx)
		}
		recorder := &opsAccessResponseRecorder{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r)
		if metrics != nil && r.URL.Path != LocalMetricsPath {
			metrics.ObserveLocalRequest(r.Method, r.URL.Path, recorder.status)
		}
		if recordTrace {
			trace.FinishSpan(ctx, recorder.status)
		}
		if logger == nil || !accessLogLineAllowed(time.Now()) {
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

// MountLocalPrometheus serves the already-owned loopback Prometheus text
// document for the same OpsMetrics snapshot as MountLocalMetrics.
//
// A nil metrics returns nil. This mount does not call SetPrometheusExporter.
// Missing or non-loopback exporter config stays fail-closed inside
// PrometheusHandler (404, empty body). The text path is not a clean
// /local/<name> segment, so WrapOpsAccessLog does not count or trace it.
func MountLocalPrometheus(metrics *OpsMetrics) http.Handler {
	if metrics == nil {
		return nil
	}
	return metrics.PrometheusHandler()
}

func mountedOpsMetrics(next http.Handler) *OpsMetrics {
	mux, ok := next.(*http.ServeMux)
	if !ok || mux == nil {
		return nil
	}
	return MountedOpsMetrics(mux)
}

// MountedOpsMetrics returns the OpsMetrics document already registered at
// LocalMetricsPath, or nil when that path is not the JSON metrics mount.
// Callers use it to attach Prometheus text to the same counters. It does
// not create a second counter set and does not set an exporter.
func MountedOpsMetrics(mux *http.ServeMux) *OpsMetrics {
	if mux == nil {
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

// MountLocalTrace serves the already-owned loopback JSON document and marks
// the handler so WrapOpsAccessLog can record other /local/<name> requests
// on the same in-memory spans. A nil trace returns nil. Missing exporter
// config stays fail-closed inside the document; this mount never dials.
func MountLocalTrace(trace *OpsTrace) http.Handler {
	if trace == nil {
		return nil
	}
	return &localTraceMount{trace: trace, handler: trace.Handler()}
}

func mountedOpsTrace(next http.Handler) *OpsTrace {
	mux, ok := next.(*http.ServeMux)
	if !ok || mux == nil {
		return nil
	}
	handler, pattern := mux.Handler(&http.Request{
		Method:     http.MethodGet,
		URL:        localTraceURL(),
		RemoteAddr: "127.0.0.1:1",
	})
	if pattern != LocalTracePath {
		return nil
	}
	mount, ok := handler.(*localTraceMount)
	if !ok || mount == nil {
		return nil
	}
	return mount.trace
}

func localTraceURL() *url.URL {
	return &url.URL{Path: LocalTracePath}
}

type localTraceMount struct {
	trace   *OpsTrace
	handler http.Handler
}

func (m *localTraceMount) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
