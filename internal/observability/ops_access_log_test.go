package observability

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWrapOpsAccessLogEmitsLocalRequestMetadata(t *testing.T) {
	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})

	handler := WrapOpsAccessLog(logger, next)
	req := httptest.NewRequest(http.MethodGet, "/local/build-info?token=should-not-log", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if body := rec.Body.String(); body != `{"ok":true}` {
		t.Fatalf("unexpected response body %q", body)
	}

	record := decodeLastJSONLog(t, buf.Bytes())
	if got := record["msg"]; got != "ops local request" {
		t.Fatalf("msg = %v, want ops local request", got)
	}
	if got := record["method"]; got != http.MethodGet {
		t.Fatalf("method = %v, want %s", got, http.MethodGet)
	}
	if got := record["path"]; got != "/local/build-info" {
		t.Fatalf("path = %v, want /local/build-info", got)
	}
	if got := record["remote_addr"]; got != "127.0.0.1:54321" {
		t.Fatalf("remote_addr = %v, want 127.0.0.1:54321", got)
	}
	if got := record["status"]; got != float64(http.StatusCreated) {
		t.Fatalf("status = %v, want %d", got, http.StatusCreated)
	}
	duration, ok := record["duration_ms"].(float64)
	if !ok || duration < 0 {
		t.Fatalf("duration_ms = %v, want non-negative number", record["duration_ms"])
	}
	if strings.Contains(buf.String(), "token=") || strings.Contains(buf.String(), "should-not-log") {
		t.Fatalf("access log leaked query string: %s", buf.String())
	}
	if strings.Contains(buf.String(), `{"ok":true}`) {
		t.Fatalf("access log leaked response body: %s", buf.String())
	}
	if got := record["service"]; got != "gamed" {
		t.Fatalf("service = %v, want gamed", got)
	}
}

func TestWrapOpsAccessLogDefaultsStatusWhenHandlerOmitsWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := NewServiceLogger("authd", &buf)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok\n")
	})

	req := httptest.NewRequest(http.MethodGet, "/local/runtime-config", nil)
	req.RemoteAddr = "127.0.0.1:9"
	rec := httptest.NewRecorder()
	WrapOpsAccessLog(logger, next).ServeHTTP(rec, req)

	record := decodeLastJSONLog(t, buf.Bytes())
	if got := record["status"]; got != float64(http.StatusOK) {
		t.Fatalf("status = %v, want %d", got, http.StatusOK)
	}
	if got := record["path"]; got != "/local/runtime-config" {
		t.Fatalf("path = %v, want /local/runtime-config", got)
	}
}

func TestWrapOpsAccessLogSkipsNonLocalPaths(t *testing.T) {
	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := WrapOpsAccessLog(logger, next)

	for _, path := range []string{"/healthz", "/debug/pprof/", "/debug/pprof/heap", "/"} {
		buf.Reset()
		called = false
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:1"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if !called {
			t.Fatalf("expected next to run for %s", path)
		}
		if strings.TrimSpace(buf.String()) != "" {
			t.Fatalf("expected no access log for %s, got %s", path, buf.String())
		}
	}
}

func TestWrapOpsAccessLogNilLoggerOrHandlerIsPassthrough(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if WrapOpsAccessLog(nil, next) == nil {
		t.Fatal("expected non-nil handler when logger is nil")
	}
	req := httptest.NewRequest(http.MethodGet, "/local/build-info", nil)
	rec := httptest.NewRecorder()
	WrapOpsAccessLog(nil, next).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("nil logger passthrough status = %d", rec.Code)
	}
	if WrapOpsAccessLog(NewServiceLogger("gamed", io.Discard), nil) != nil {
		t.Fatal("expected nil handler when next is nil")
	}
}

func TestWrapOpsAccessLogRecordsDurationAfterSlowHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/local/notice", nil)
	req.RemoteAddr = "127.0.0.1:2"
	rec := httptest.NewRecorder()
	WrapOpsAccessLog(logger, next).ServeHTTP(rec, req)

	record := decodeLastJSONLog(t, buf.Bytes())
	duration, ok := record["duration_ms"].(float64)
	if !ok || duration < 1 {
		t.Fatalf("duration_ms = %v, want >= 1 after sleep", record["duration_ms"])
	}
	if got := record["method"]; got != http.MethodPost {
		t.Fatalf("method = %v, want POST", got)
	}
}

func TestMountLocalMetricsNilIsNil(t *testing.T) {
	if MountLocalMetrics(nil) != nil {
		t.Fatal("nil metrics mounted a handler")
	}
	if MountLocalTrace(nil) != nil {
		t.Fatal("nil trace mounted a handler")
	}
}

func TestWrapOpsAccessLogRecordsMountedLocalMetrics(t *testing.T) {
	metrics := NewOpsMetrics()
	metrics.SetService("gamed")
	mux := http.NewServeMux()
	mux.HandleFunc("/local/build-info", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle(LocalMetricsPath, MountLocalMetrics(metrics))
	handler := WrapOpsAccessLog(nil, mux)

	for _, path := range []string{"/local/build-info", "/healthz", "/debug/pprof/", "/local/persistence/status", LocalMetricsPath} {
		req := httptest.NewRequest(http.MethodGet, path, strings.NewReader("secret-body"))
		req.RemoteAddr = "127.0.0.1:9"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	snap := metrics.Snapshot()
	if snap.LocalRequestsTotal != 1 || snap.LocalRequestsByPath["/local/build-info"] != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.LocalErrorsTotal != 0 {
		t.Fatalf("errors = %d, want 0", snap.LocalErrorsTotal)
	}
}

func TestAccessLogSamplerMissingConfigKeepsEveryRequestLine(t *testing.T) {
	SetAccessLogSampler(nil)
	t.Cleanup(func() { SetAccessLogSampler(nil) })
	if AccessLogSamplerConfigured() {
		t.Fatal("missing sampler config was treated as enabled")
	}
	if (AccessLogSampler{}).Configured() || (AccessLogSampler{Every: 0}).Configured() || (AccessLogSampler{Every: -time.Second}).Configured() {
		t.Fatal("non-positive interval was treated as configured")
	}

	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WrapOpsAccessLog(logger, next)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/local/build-info?token=should-not-log", strings.NewReader("secret-body"))
		req.RemoteAddr = "127.0.0.1:9"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	lines := accessLogLines(t, buf.Bytes())
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3 with sampler unset", len(lines))
	}
	if strings.Contains(buf.String(), "token=") || strings.Contains(buf.String(), "secret-body") || strings.Contains(buf.String(), "should-not-log") {
		t.Fatalf("access log leaked request detail: %s", buf.String())
	}

	SetAccessLogSampler(&AccessLogSampler{Every: 0})
	if AccessLogSamplerConfigured() {
		t.Fatal("zero interval enabled the sampler")
	}
}

func TestAccessLogSamplerDropsLinesInsideInterval(t *testing.T) {
	SetAccessLogSampler(&AccessLogSampler{Every: time.Hour})
	t.Cleanup(func() { SetAccessLogSampler(nil) })
	if !AccessLogSamplerConfigured() {
		t.Fatal("positive interval did not enable the sampler")
	}

	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	handler := WrapOpsAccessLog(logger, next)
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/local/notice?token=should-not-log", strings.NewReader("secret-body"))
		req.RemoteAddr = "127.0.0.1:9"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated || rec.Body.String() != `{"ok":true}` {
			t.Fatalf("handler changed under sampler: status=%d body=%q", rec.Code, rec.Body.String())
		}
	}
	lines := accessLogLines(t, buf.Bytes())
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1 inside the interval", len(lines))
	}
	if got := lines[0]["path"]; got != "/local/notice" {
		t.Fatalf("path = %v, want /local/notice", got)
	}
	if strings.Contains(buf.String(), "token=") || strings.Contains(buf.String(), "secret-body") || strings.Contains(buf.String(), "Every") {
		t.Fatalf("sampled line leaked request or config: %s", buf.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:1"
	handler.ServeHTTP(httptest.NewRecorder(), req)
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil)
	req.RemoteAddr = "127.0.0.1:1"
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if got := len(accessLogLines(t, buf.Bytes())); got != 1 {
		t.Fatalf("quiet paths emitted a line: %d", got)
	}
}

func TestAccessLogSamplerAllowsALineAfterTheInterval(t *testing.T) {
	SetAccessLogSampler(&AccessLogSampler{Every: 20 * time.Millisecond})
	t.Cleanup(func() { SetAccessLogSampler(nil) })

	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WrapOpsAccessLog(logger, next)
	hit := func() {
		req := httptest.NewRequest(http.MethodGet, "/local/build-info", nil)
		req.RemoteAddr = "127.0.0.1:9"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	hit()
	hit()
	if got := len(accessLogLines(t, buf.Bytes())); got != 1 {
		t.Fatalf("lines before wait = %d, want 1", got)
	}
	time.Sleep(30 * time.Millisecond)
	hit()
	if got := len(accessLogLines(t, buf.Bytes())); got != 2 {
		t.Fatalf("lines after interval = %d, want 2", got)
	}
}

func TestAccessLogSamplerKeepsMetricsAndTraceOnDroppedLines(t *testing.T) {
	SetAccessLogSampler(&AccessLogSampler{Every: time.Hour})
	t.Cleanup(func() { SetAccessLogSampler(nil) })

	metrics := NewOpsMetrics()
	metrics.SetService("gamed")
	trace := NewOpsTrace()
	trace.SetService("gamed")
	mux := http.NewServeMux()
	mux.HandleFunc("/local/build-info", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle(LocalMetricsPath, MountLocalMetrics(metrics))
	mux.Handle(LocalTracePath, MountLocalTrace(trace))

	var buf bytes.Buffer
	handler := WrapOpsAccessLog(NewServiceLogger("gamed", &buf), mux)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/local/build-info?token=should-not-log", strings.NewReader("secret-body"))
		req.RemoteAddr = "127.0.0.1:9"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:9"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got := len(accessLogLines(t, buf.Bytes())); got != 1 {
		t.Fatalf("lines = %d, want 1", got)
	}
	snap := metrics.Snapshot()
	if snap.LocalRequestsTotal != 3 || snap.LocalRequestsByPath["/local/build-info"] != 3 {
		t.Fatalf("metrics snapshot = %+v, want 3 build-info counts", snap)
	}
	doc := trace.Snapshot()
	if doc.SpanCount != 3 {
		t.Fatalf("span_count = %d, want 3", doc.SpanCount)
	}
	if strings.Contains(buf.String(), "token=") || strings.Contains(buf.String(), "secret-body") {
		t.Fatalf("sampled line leaked request detail: %s", buf.String())
	}
}

func accessLogLines(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode access line: %v\n%s", err, line)
		}
		if record["msg"] == "ops local request" {
			lines = append(lines, record)
		}
	}
	return lines
}
