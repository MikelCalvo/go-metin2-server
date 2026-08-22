package observability

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// WrapOpsAccessLog returns an http.Handler that emits one metadata-only JSON
// access line for each /local/* request after the downstream handler returns.
//
// Non-/local/ paths (including /healthz and /debug/pprof/*) are never logged.
// Query strings, fragments, and request/response bodies are never logged.
// A nil logger or nil next is a passthrough (nil next remains nil).
func WrapOpsAccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	if next == nil {
		return nil
	}
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || !strings.HasPrefix(r.URL.Path, "/local/") {
			next.ServeHTTP(w, r)
			return
		}

		recorder := &opsAccessResponseRecorder{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r)
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
