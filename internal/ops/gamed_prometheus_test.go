package ops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/observability"
)

func TestGamedOpsMuxMountsLocalPrometheusFailClosed(t *testing.T) {
	mux := NewPprofMux("gamed")
	handler := observability.WrapOpsAccessLog(nil, mux)

	status, body := getLocal(handler, "/local/build-info")
	if status != http.StatusOK || !strings.Contains(body, "{") {
		t.Fatalf("build-info = %d %q", status, body)
	}
	mux.HandleFunc("/local/late-probe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	status, _ = getLocal(handler, "/local/late-probe")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("late probe status = %d, want 503", status)
	}

	status, _ = getLocal(handler, "/healthz")
	if status != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", status)
	}
	status, _ = getLocal(handler, "/debug/pprof/")
	if status == http.StatusNotFound {
		t.Fatal("pprof index was not registered")
	}

	status, secret := getLocalRaw(handler, "/local/metrics/prometheus?token=secret", "secret-body")
	if status != http.StatusNotFound || secret != "" {
		t.Fatalf("unset exporter = %d %q, want 404 empty", status, secret)
	}

	status, jsonBody := getLocalRaw(handler, "/local/metrics?token=secret", "secret-body")
	if status != http.StatusOK {
		t.Fatalf("json metrics status = %d, want 200", status)
	}
	if strings.Contains(jsonBody, "secret") || strings.Contains(jsonBody, "metin2_ops_") {
		t.Fatalf("json document changed: %s", jsonBody)
	}
	var snap observability.OpsMetricsSnapshot
	if err := json.Unmarshal([]byte(jsonBody), &snap); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if snap.Service != "gamed" || snap.LocalRequestsTotal != 2 || snap.LocalErrorsTotal != 1 {
		t.Fatalf("snapshot = %+v, want gamed total 2 errors 1", snap)
	}
	if snap.LocalRequestsByPath["/local/build-info"] != 1 || snap.LocalRequestsByPath["/local/late-probe"] != 1 {
		t.Fatalf("paths = %+v", snap.LocalRequestsByPath)
	}
	for _, path := range []string{"/local/metrics", "/local/metrics/prometheus", "/healthz", "/debug/pprof/"} {
		if _, ok := snap.LocalRequestsByPath[path]; ok {
			t.Fatalf("counted quiet path %s: %+v", path, snap.LocalRequestsByPath)
		}
	}

	metrics := observability.MountedOpsMetrics(mux)
	if metrics == nil {
		t.Fatal("gamed mux lost the metrics document")
	}
	metrics.SetPrometheusExporter(&observability.LoopbackPrometheusExporter{Endpoint: "http://10.1.2.3:9090/metrics"})
	status, remote := getLocalRaw(handler, "/local/metrics/prometheus", "secret-body")
	if status != http.StatusNotFound || remote != "" {
		t.Fatalf("non-loopback exporter = %d %q, want 404 empty", status, remote)
	}
	if snap = readMetrics(t, handler); snap.LocalRequestsTotal != 2 {
		t.Fatalf("refused exporter changed json counters: %+v", snap)
	}

	metrics.SetPrometheusExporter(&observability.LoopbackPrometheusExporter{Endpoint: "http://127.0.0.1:9090/metrics"})
	status, text := getLocalRaw(handler, "/local/metrics/prometheus?token=secret", "secret-body")
	if status != http.StatusOK {
		t.Fatalf("loopback exporter status = %d body %q", status, text)
	}
	for _, want := range []string{
		`metin2_ops_local_requests_total{service="gamed"} 2`,
		`metin2_ops_local_errors_total{service="gamed"} 1`,
		`metin2_ops_local_requests_by_path{service="gamed",path="/local/build-info"} 1`,
		`metin2_ops_local_requests_by_path{service="gamed",path="/local/late-probe"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("exposition missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"secret", "token", "9090", "/healthz", "pprof", "postgres"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("exposition leaked %q:\n%s", forbidden, text)
		}
	}
	if snap = readMetrics(t, handler); snap.LocalRequestsTotal != 2 || snap.LocalRequestsByPath["/local/metrics/prometheus"] != 0 {
		t.Fatalf("prometheus read changed json counters: %+v", snap)
	}

	req := httptest.NewRequest(http.MethodGet, "/local/metrics/prometheus", nil)
	req.RemoteAddr = "10.1.2.3:9"
	forbiddenRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenRec, req)
	if forbiddenRec.Code != http.StatusForbidden || forbiddenRec.Body.Len() != 0 {
		t.Fatalf("non-loopback caller = %d %q", forbiddenRec.Code, forbiddenRec.Body.String())
	}
	post := httptest.NewRequest(http.MethodPost, "/local/metrics/prometheus", strings.NewReader(`{"dsn":"postgres://secret"}`))
	post.RemoteAddr = "127.0.0.1:9"
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, post)
	if postRec.Code != http.StatusMethodNotAllowed || strings.Contains(postRec.Body.String(), "secret") {
		t.Fatalf("POST = %d %q", postRec.Code, postRec.Body.String())
	}
}

func TestAuthdOpsMuxDoesNotMountLocalPrometheus(t *testing.T) {
	mux := NewPprofMux("authd")
	handler := observability.WrapOpsAccessLog(nil, mux)

	status, body := getLocal(handler, "/local/metrics/prometheus")
	if status != http.StatusNotFound {
		t.Fatalf("authd prometheus status = %d, want 404", status)
	}
	if strings.Contains(body, "metin2_ops") {
		t.Fatal("authd served prometheus text")
	}
	if observability.MountedOpsMetrics(mux) != nil {
		t.Fatal("authd registered the gamed metrics document")
	}
	status, _ = getLocal(handler, "/local/build-info")
	if status != http.StatusOK {
		t.Fatalf("authd build-info status = %d, want 200", status)
	}
	status, _ = getLocal(handler, "/healthz")
	if status != http.StatusOK {
		t.Fatalf("authd healthz status = %d, want 200", status)
	}
}

func readMetrics(t *testing.T, handler http.Handler) observability.OpsMetricsSnapshot {
	t.Helper()
	status, body := getLocal(handler, "/local/metrics")
	if status != http.StatusOK {
		t.Fatalf("metrics status = %d", status)
	}
	var snap observability.OpsMetricsSnapshot
	if err := json.Unmarshal([]byte(body), &snap); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	return snap
}
