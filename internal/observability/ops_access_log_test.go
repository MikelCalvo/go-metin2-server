package observability

import (
	"bytes"
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
