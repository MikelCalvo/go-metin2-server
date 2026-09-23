package observability

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
)

func TestNewServiceLoggerIncludesBuildIdentityAttrs(t *testing.T) {
	originalVersion := buildinfo.Version
	originalCommit := buildinfo.Commit
	originalBuildDate := buildinfo.BuildDate
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
		buildinfo.Commit = originalCommit
		buildinfo.BuildDate = originalBuildDate
	})

	buildinfo.Version = "v0.1.0-test"
	buildinfo.Commit = "abcdef012345"
	buildinfo.BuildDate = "2026-08-20T15:30:45Z"

	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	logger.Info("ops server listening", "addr", "127.0.0.1:6060")

	record := decodeLastJSONLog(t, buf.Bytes())
	if got := record["service"]; got != "gamed" {
		t.Fatalf("service = %v, want gamed", got)
	}
	if got := record["version"]; got != buildinfo.Version {
		t.Fatalf("version = %v, want %s", got, buildinfo.Version)
	}
	if got := record["commit"]; got != buildinfo.Commit {
		t.Fatalf("commit = %v, want %s", got, buildinfo.Commit)
	}
	if got := record["build_date"]; got != buildinfo.BuildDate {
		t.Fatalf("build_date = %v, want %s", got, buildinfo.BuildDate)
	}
	if got := record["addr"]; got != "127.0.0.1:6060" {
		t.Fatalf("addr = %v, want 127.0.0.1:6060", got)
	}
	if got := record["msg"]; got != "ops server listening" {
		t.Fatalf("msg = %v, want ops server listening", got)
	}
}

func TestNewServiceLoggerRedactsSensitiveAttributeKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := NewServiceLogger("authd", &buf)
	logger.Error("open database failed",
		"dsn", "postgres://operator:s3cret@127.0.0.1:5432/metin2?sslmode=disable",
		"DB_DSN", "should-also-redact",
		"password", "hunter2",
		"login-key", uint32(42),
		"ticket", "raw-ticket-bytes",
		"secret", "top-secret",
		"api_key", "abcd",
		"token", "bearer-token",
		"addr", "127.0.0.1:6061",
		"err", "connection refused",
	)

	body := buf.String()
	for _, forbidden := range []string{
		"postgres://",
		"s3cret",
		"hunter2",
		"raw-ticket-bytes",
		"top-secret",
		"bearer-token",
		"should-also-redact",
		"abcd",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("log leaked sensitive material %q: %s", forbidden, body)
		}
	}

	record := decodeLastJSONLog(t, buf.Bytes())
	for _, key := range []string{"dsn", "DB_DSN", "password", "login-key", "ticket", "secret", "api_key", "token"} {
		if got := record[key]; got != "<redacted>" {
			t.Fatalf("%s = %v, want <redacted>", key, got)
		}
	}
	if got := record["addr"]; got != "127.0.0.1:6061" {
		t.Fatalf("addr = %v, want 127.0.0.1:6061", got)
	}
	if got := record["err"]; got != "connection refused" {
		t.Fatalf("err = %v, want connection refused", got)
	}
	if got := record["service"]; got != "authd" {
		t.Fatalf("service = %v, want authd", got)
	}
}

func TestIsSensitiveAttrKeyNormalizesSeparatorsAndCase(t *testing.T) {
	cases := map[string]bool{
		"dsn":       true,
		"DB_DSN":    true,
		"db-dsn":    true,
		"Login_Key": true,
		"login-key": true,
		"api key":   true,
		"addr":      false,
		"remote":    false,
		"phase":     false,
	}
	for key, want := range cases {
		if got := isSensitiveAttrKey(key); got != want {
			t.Fatalf("isSensitiveAttrKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestRedactSensitiveAttrLeavesOrdinaryValues(t *testing.T) {
	attr := redactSensitiveAttr(nil, slog.String("remote_addr", "10.0.0.8:1234"))
	if attr.Value.String() != "10.0.0.8:1234" {
		t.Fatalf("unexpected attr %#v", attr)
	}
}

func TestLocalMetricsEndpointReportsLoopbackRequestCounts(t *testing.T) {
	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	metrics := NewOpsMetrics()
	metrics.SetService("gamed")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/local/notice":
			http.Error(w, "nope", http.StatusConflict)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	outer := metrics.Wrap(WrapOpsAccessLog(logger, next))
	serve := func(method, path, remote string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		outer.ServeHTTP(rec, req)
		return rec
	}

	if rec := serve(http.MethodGet, "/local/build-info", "127.0.0.1:9"); rec.Code != http.StatusOK {
		t.Fatalf("build-info status = %d", rec.Code)
	}
	if rec := serve(http.MethodGet, "/local/build-info?token=should-not-count", "127.0.0.1:9"); rec.Code != http.StatusOK {
		t.Fatalf("build-info query status = %d", rec.Code)
	}
	if rec := serve(http.MethodPost, "/local/notice", "127.0.0.1:10"); rec.Code != http.StatusConflict {
		t.Fatalf("notice status = %d", rec.Code)
	}
	if rec := serve(http.MethodGet, "/healthz", "127.0.0.1:11"); rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d", rec.Code)
	}
	if rec := serve(http.MethodGet, "/debug/pprof/heap", "127.0.0.1:12"); rec.Code != http.StatusOK {
		t.Fatalf("pprof status = %d", rec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, LocalMetricsPath, nil)
	metricsReq.RemoteAddr = "127.0.0.1:13"
	metricsRec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, body %s", metricsRec.Code, metricsRec.Body.String())
	}
	if got := metricsRec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("content-type = %q", got)
	}

	var body map[string]any
	if err := json.Unmarshal(metricsRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metrics: %v\n%s", err, metricsRec.Body.String())
	}
	if got := body["service"]; got != "gamed" {
		t.Fatalf("service = %v, want gamed", got)
	}
	requests, _ := body["local_requests_total"].(float64)
	if requests != 3 {
		t.Fatalf("local_requests_total = %v, want 3", requests)
	}
	errors, _ := body["local_errors_total"].(float64)
	if errors != 1 {
		t.Fatalf("local_errors_total = %v, want 1", errors)
	}
	if strings.Contains(metricsRec.Body.String(), "should-not-count") || strings.Contains(metricsRec.Body.String(), "token") {
		t.Fatalf("metrics leaked query material: %s", metricsRec.Body.String())
	}
	if strings.Contains(metricsRec.Body.String(), "/healthz") || strings.Contains(metricsRec.Body.String(), "pprof") {
		t.Fatalf("metrics named quiet paths: %s", metricsRec.Body.String())
	}

	paths, ok := body["local_requests_by_path"].(map[string]any)
	if !ok {
		t.Fatalf("local_requests_by_path = %#v", body["local_requests_by_path"])
	}
	if got := paths["/local/build-info"]; got != float64(2) {
		t.Fatalf("build-info count = %v, want 2", got)
	}
	if got := paths["/local/notice"]; got != float64(1) {
		t.Fatalf("notice count = %v, want 1", got)
	}
	if _, ok := paths["/healthz"]; ok {
		t.Fatal("healthz was counted")
	}

	// The access log still records the same /local traffic and still drops the query.
	if strings.Contains(buf.String(), "should-not-count") || strings.Contains(buf.String(), "token=") {
		t.Fatalf("access log leaked query string: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"msg":"ops local request"`) {
		t.Fatalf("expected access log beside metrics, got %s", buf.String())
	}
}

func TestLocalMetricsEndpointRejectsNonLoopbackAndNonGet(t *testing.T) {
	metrics := NewOpsMetrics()
	metrics.ObserveLocalRequest(http.MethodGet, "/local/build-info", http.StatusOK)

	nonLoopback := httptest.NewRequest(http.MethodGet, LocalMetricsPath, nil)
	nonLoopback.RemoteAddr = "203.0.113.8:4242"
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, nonLoopback)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback status = %d, want 403", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("non-loopback body = %q, want empty", rec.Body.String())
	}

	post := httptest.NewRequest(http.MethodPost, LocalMetricsPath, strings.NewReader(`{"dsn":"postgres://secret"}`))
	post.RemoteAddr = "127.0.0.1:9"
	rec = httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "postgres") || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("POST body leaked: %s", rec.Body.String())
	}

	localhost := httptest.NewRequest(http.MethodGet, LocalMetricsPath, nil)
	localhost.RemoteAddr = "localhost:9"
	rec = httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, localhost)
	if rec.Code != http.StatusOK {
		t.Fatalf("localhost status = %d, want 200", rec.Code)
	}
}

func TestOpsMetricsNilWrapIsPassthroughAndDoesNotPanic(t *testing.T) {
	var metrics *OpsMetrics
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if metrics.Wrap(next) == nil {
		t.Fatal("nil metrics Wrap returned nil")
	}
	req := httptest.NewRequest(http.MethodGet, "/local/build-info", nil)
	req.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	metrics.Wrap(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("passthrough status = %d", rec.Code)
	}
	if metrics.Wrap(nil) != nil {
		t.Fatal("nil next should stay nil")
	}
	metrics.ObserveLocalRequest(http.MethodGet, "/local/build-info", http.StatusOK)

	empty := NewOpsMetrics()
	if empty.Handler() == nil {
		t.Fatal("expected metrics handler")
	}
	req = httptest.NewRequest(http.MethodGet, LocalMetricsPath+"?token=nope", nil)
	req.RemoteAddr = "[::1]:9"
	rec = httptest.NewRecorder()
	empty.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ipv6 loopback status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "token") || strings.Contains(rec.Body.String(), "nope") {
		t.Fatalf("query leaked into metrics: %s", rec.Body.String())
	}
}

func TestOpsMetricsSnapshotStaysMetadataOnly(t *testing.T) {
	metrics := NewOpsMetrics()
	metrics.SetService("authd")
	metrics.ObserveLocalRequest(http.MethodGet, "/local/build-info?ticket=raw-ticket", http.StatusOK)
	metrics.ObserveLocalRequest("GET", "/local/../etc/passwd", http.StatusOK)
	metrics.ObserveLocalRequest(http.MethodPost, "/local/notice", http.StatusInternalServerError)

	snap := metrics.Snapshot()
	encoded, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(encoded)
	for _, forbidden := range []string{"raw-ticket", "ticket", "passwd", "postgres://", "dsn", "password", "secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("snapshot leaked %q: %s", forbidden, body)
		}
	}
	if snap.Service != "authd" {
		t.Fatalf("service = %q", snap.Service)
	}
	if snap.LocalRequestsTotal != 2 {
		t.Fatalf("local_requests_total = %d, want 2", snap.LocalRequestsTotal)
	}
	if snap.LocalErrorsTotal != 1 {
		t.Fatalf("local_errors_total = %d, want 1", snap.LocalErrorsTotal)
	}
	if snap.LocalRequestsByPath["/local/build-info"] != 1 {
		t.Fatalf("by path = %#v", snap.LocalRequestsByPath)
	}
	if _, ok := snap.LocalRequestsByPath["/local/notice"]; !ok {
		t.Fatalf("notice missing from %#v", snap.LocalRequestsByPath)
	}
}

func TestWrapOpsMetricsRecordsOnlyLocalPaths(t *testing.T) {
	metrics := NewOpsMetrics()
	var saw io.Reader
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = r.Body
		w.WriteHeader(http.StatusCreated)
	})
	handler := metrics.Wrap(next)
	req := httptest.NewRequest(http.MethodPut, "/local/runtime-config", strings.NewReader("not-a-secret-body"))
	req.RemoteAddr = "10.1.2.3:9"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if saw == nil {
		t.Fatal("wrapper dropped the request")
	}
	snap := metrics.Snapshot()
	if snap.LocalRequestsTotal != 1 || snap.LocalErrorsTotal != 0 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.LocalRequestsByPath["/local/runtime-config"] != 1 {
		t.Fatalf("by path = %#v", snap.LocalRequestsByPath)
	}
	encoded, _ := json.Marshal(snap)
	if strings.Contains(string(encoded), "not-a-secret-body") || strings.Contains(string(encoded), "10.1.2.3") {
		t.Fatalf("snapshot captured body or remote addr: %s", encoded)
	}
}

func decodeLastJSONLog(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	if len(lines) == 0 || len(bytes.TrimSpace(lines[len(lines)-1])) == 0 {
		t.Fatalf("expected JSON log output, got %q", raw)
	}
	var record map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &record); err != nil {
		t.Fatalf("decode log JSON: %v\nraw=%s", err, raw)
	}
	return record
}
