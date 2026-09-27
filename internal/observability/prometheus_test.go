package observability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrometheusTextStaysFailClosedUntilLoopbackExporter(t *testing.T) {
	metrics := NewOpsMetrics()
	metrics.SetService("gamed")
	metrics.ObserveLocalRequest(http.MethodGet, "/local/build-info?token=should-not-appear", http.StatusOK)
	metrics.ObserveLocalRequest(http.MethodGet, "/local/build-info", http.StatusOK)
	metrics.ObserveLocalRequest(http.MethodPost, "/local/notice", http.StatusConflict)
	metrics.ObserveLocalRequest(http.MethodGet, "/healthz", http.StatusOK)
	metrics.ObserveLocalRequest(http.MethodGet, "/debug/pprof/heap", http.StatusOK)

	if metrics.PrometheusConfigured() {
		t.Fatal("missing exporter config was treated as enabled")
	}
	text, ok := metrics.PrometheusText()
	if ok || text != "" {
		t.Fatalf("missing config text = %q ok=%v, want empty fail-closed", text, ok)
	}

	disabled := httptest.NewRequest(http.MethodGet, LocalPrometheusPath+"?token=secret", strings.NewReader(`{"dsn":"postgres://secret"}`))
	disabled.RemoteAddr = "127.0.0.1:9"
	rec := httptest.NewRecorder()
	metrics.PrometheusHandler().ServeHTTP(rec, disabled)
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Fatalf("disabled exposition = %d %q", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "token") || strings.Contains(rec.Body.String(), "secret") || strings.Contains(rec.Body.String(), "postgres") {
		t.Fatalf("fail-closed body leaked request material: %s", rec.Body.String())
	}

	jsonReq := httptest.NewRequest(http.MethodGet, LocalMetricsPath, nil)
	jsonReq.RemoteAddr = "127.0.0.1:10"
	jsonRec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(jsonRec, jsonReq)
	if jsonRec.Code != http.StatusOK || !strings.Contains(jsonRec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("JSON metrics changed while Prometheus stayed disabled: %d %s", jsonRec.Code, jsonRec.Body.String())
	}
	if strings.Contains(jsonRec.Body.String(), "metin2_ops_") {
		t.Fatalf("JSON document became Prometheus text: %s", jsonRec.Body.String())
	}

	for _, endpoint := range []string{
		"",
		"https://127.0.0.1:9090/metrics",
		"http://10.1.2.3:9090/metrics",
		"http://0.0.0.0:9090/metrics",
		"http://127.0.0.1:9090/metrics?token=secret",
		"http://127.0.0.1:9090/v1/traces",
		"http://example.com/metrics",
	} {
		metrics.SetPrometheusExporter(&LoopbackPrometheusExporter{Endpoint: endpoint})
		if metrics.PrometheusConfigured() {
			t.Fatalf("endpoint %q was accepted", endpoint)
		}
		if _, ok := metrics.PrometheusText(); ok {
			t.Fatalf("endpoint %q unlocked exposition", endpoint)
		}
	}

	metrics.SetPrometheusExporter(&LoopbackPrometheusExporter{Endpoint: "http://127.0.0.1:9090/metrics"})
	if !metrics.PrometheusConfigured() {
		t.Fatal("loopback exporter stayed disabled")
	}
	enabled := httptest.NewRequest(http.MethodGet, LocalPrometheusPath, nil)
	enabled.RemoteAddr = "127.0.0.1:11"
	rec = httptest.NewRecorder()
	metrics.PrometheusHandler().ServeHTTP(rec, enabled)
	if rec.Code != http.StatusOK {
		t.Fatalf("enabled status = %d, body %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") || !strings.Contains(got, "version=0.0.4") {
		t.Fatalf("content-type = %q", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`metin2_ops_local_requests_total{service="gamed"} 3`,
		`metin2_ops_local_errors_total{service="gamed"} 1`,
		`metin2_ops_local_requests_by_path{service="gamed",path="/local/build-info"} 2`,
		`metin2_ops_local_requests_by_path{service="gamed",path="/local/notice"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("exposition missing %q:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{"should-not-appear", "token", "secret", "postgres", "/healthz", "pprof", "9090", "127.0.0.1:9090"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("exposition leaked %q:\n%s", forbidden, body)
		}
	}

	post := httptest.NewRequest(http.MethodPost, LocalPrometheusPath, strings.NewReader(`{"dsn":"postgres://secret"}`))
	post.RemoteAddr = "127.0.0.1:12"
	rec = httptest.NewRecorder()
	metrics.PrometheusHandler().ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("POST = %d %q", rec.Code, rec.Body.String())
	}
	remote := httptest.NewRequest(http.MethodGet, LocalPrometheusPath, nil)
	remote.RemoteAddr = "203.0.113.8:9"
	rec = httptest.NewRecorder()
	metrics.PrometheusHandler().ServeHTTP(rec, remote)
	if rec.Code != http.StatusForbidden || rec.Body.Len() != 0 {
		t.Fatalf("non-loopback = %d %q", rec.Code, rec.Body.String())
	}

	metrics.SetPrometheusExporter(nil)
	if metrics.PrometheusConfigured() {
		t.Fatal("clearing the exporter left exposition enabled")
	}
	cleared := httptest.NewRequest(http.MethodGet, LocalPrometheusPath, nil)
	cleared.RemoteAddr = "[::1]:9"
	rec = httptest.NewRecorder()
	metrics.PrometheusHandler().ServeHTTP(rec, cleared)
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Fatalf("cleared exporter = %d %q", rec.Code, rec.Body.String())
	}

	var nilMetrics *OpsMetrics
	nilMetrics.SetPrometheusExporter(&LoopbackPrometheusExporter{Endpoint: "http://127.0.0.1:9090/metrics"})
	if _, ok := nilMetrics.PrometheusText(); ok {
		t.Fatal("nil metrics unlocked exposition")
	}
	nilRec := httptest.NewRecorder()
	nilReq := httptest.NewRequest(http.MethodGet, LocalPrometheusPath, nil)
	nilReq.RemoteAddr = "localhost:9"
	nilMetrics.PrometheusHandler().ServeHTTP(nilRec, nilReq)
	if nilRec.Code != http.StatusNotFound {
		t.Fatalf("nil handler status = %d", nilRec.Code)
	}

	var snap map[string]any
	if err := json.Unmarshal(jsonRec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("JSON metrics no longer decodes: %v", err)
	}
	if snap["local_requests_total"] != float64(3) {
		t.Fatalf("JSON total changed: %#v", snap)
	}
}

func TestLoopbackPrometheusEndpointAcceptsOnlyLocalMetricsTarget(t *testing.T) {
	accepted := []string{
		"http://127.0.0.1:9090/metrics",
		"http://127.0.0.1:9090",
		"http://[::1]:9090/metrics",
		"http://localhost:9090/metrics",
	}
	for _, endpoint := range accepted {
		if !(&LoopbackPrometheusExporter{Endpoint: endpoint}).Configured() {
			t.Fatalf("expected %q to configure", endpoint)
		}
	}
}
