package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoopbackOTelSpanSitsBesideJSONLogsAndLocalMetrics(t *testing.T) {
	var buf bytes.Buffer
	logger := NewServiceLogger("gamed", &buf)
	metrics := NewOpsMetrics()
	metrics.SetService("gamed")
	trace := NewOpsTrace()
	trace.SetService("gamed")

	if got, ok := trace.Export(); ok || got != nil {
		t.Fatalf("missing exporter exported spans: ok=%v spans=%v", ok, got)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/local/notice":
			http.Error(w, "nope", http.StatusConflict)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	outer := trace.Wrap(metrics.Wrap(WrapOpsAccessLog(logger, next)))
	serve := func(method, target, remote string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		outer.ServeHTTP(rec, req)
		return rec
	}

	if rec := serve(http.MethodGet, "/local/build-info", "127.0.0.1:9"); rec.Code != http.StatusOK {
		t.Fatalf("build-info status = %d", rec.Code)
	}
	if rec := serve(http.MethodGet, "/local/build-info?token=should-not-trace", "127.0.0.1:9"); rec.Code != http.StatusOK {
		t.Fatalf("query status = %d", rec.Code)
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

	traceReq := httptest.NewRequest(http.MethodGet, LocalTracePath, nil)
	traceReq.RemoteAddr = "127.0.0.1:13"
	traceRec := httptest.NewRecorder()
	trace.Handler().ServeHTTP(traceRec, traceReq)
	if traceRec.Code != http.StatusOK {
		t.Fatalf("trace status = %d body %s", traceRec.Code, traceRec.Body.String())
	}
	if got := traceRec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("content-type = %q", got)
	}
	body := traceRec.Body.String()
	for _, forbidden := range []string{"should-not-trace", "token", "/healthz", "pprof", "prometheus", "otlp"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("trace document leaked %q: %s", forbidden, body)
		}
	}

	var snap OpsTraceSnapshot
	if err := json.Unmarshal(traceRec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode trace: %v\n%s", err, body)
	}
	if snap.Service != "gamed" {
		t.Fatalf("service = %q", snap.Service)
	}
	if snap.Scope != otelTraceScopeName {
		t.Fatalf("scope = %q", snap.Scope)
	}
	if snap.Exporter != "fail-closed" {
		t.Fatalf("exporter = %q, want fail-closed without config", snap.Exporter)
	}
	if snap.SpanCount != 3 || len(snap.Spans) != 3 {
		t.Fatalf("span_count = %d spans=%d, want 3", snap.SpanCount, len(snap.Spans))
	}
	last := snap.Spans[2]
	if last.Name != otelTraceSpanName || last.Kind != "SPAN_KIND_SERVER" || last.StatusCode != 2 {
		t.Fatalf("error span = %+v", last)
	}
	if last.Attributes["http.method"] != http.MethodPost || last.Attributes["http.target"] != "/local/notice" {
		t.Fatalf("error attrs = %#v", last.Attributes)
	}
	if last.Attributes["service.name"] != "gamed" {
		t.Fatalf("service.name = %q", last.Attributes["service.name"])
	}
	if last.TraceID == "" || last.SpanID == "" || last.EndUnix < last.StartUnix {
		t.Fatalf("span identity/time = %+v", last)
	}
	if snap.Spans[0].Attributes["http.target"] != "/local/build-info" || snap.Spans[0].StatusCode != 1 {
		t.Fatalf("first span = %+v", snap.Spans[0])
	}
	if snap.Spans[1].Attributes["http.target"] != "/local/build-info" {
		t.Fatalf("query was not stripped: %+v", snap.Spans[1])
	}

	metricsReq := httptest.NewRequest(http.MethodGet, LocalMetricsPath, nil)
	metricsReq.RemoteAddr = "127.0.0.1:14"
	metricsRec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", metricsRec.Code)
	}
	if !strings.Contains(metricsRec.Body.String(), `"local_requests_total":3`) {
		t.Fatalf("metrics lost the same /local traffic: %s", metricsRec.Body.String())
	}
	if !strings.Contains(buf.String(), `"msg":"ops local request"`) {
		t.Fatalf("access log missing beside the span: %s", buf.String())
	}
	if strings.Contains(buf.String(), "should-not-trace") {
		t.Fatalf("access log leaked query: %s", buf.String())
	}
}

func TestLoopbackTraceExporterFailClosedUnlessLoopback(t *testing.T) {
	trace := NewOpsTrace()
	ctx, span := trace.StartSpan(context.Background(), otelTraceSpanName, map[string]string{
		"http.target": "/local/build-info?ticket=raw-ticket",
		"dsn":         "postgres://operator:secret@10.0.0.8/metin2",
		"password":    "hunter2",
	})
	if span == nil {
		t.Fatal("expected span")
	}
	if _, ok := span.Attributes["dsn"]; ok {
		t.Fatalf("span kept dsn: %#v", span.Attributes)
	}
	if span.Attributes["http.target"] != "/local/build-info" {
		t.Fatalf("http.target = %#v", span.Attributes)
	}
	trace.FinishSpan(ctx, http.StatusOK)

	for _, endpoint := range []string{
		"",
		"https://collector.example:4318/v1/traces",
		"http://10.1.2.3:4318/v1/traces",
		"http://0.0.0.0:4318/v1/traces",
		"http://127.0.0.1:4318/v1/traces?token=raw",
	} {
		refused := NewOpsTrace()
		refused.SetExporter(&LoopbackTraceExporter{Endpoint: endpoint})
		refused.FinishSpan(mustSpan(t, refused), http.StatusOK)
		if _, ok := refused.Export(); ok {
			t.Fatalf("endpoint %q was accepted", endpoint)
		}
		if refused.Snapshot().Exporter != "fail-closed" {
			t.Fatalf("endpoint %q exporter = %q", endpoint, refused.Snapshot().Exporter)
		}
	}

	accepted := NewOpsTrace()
	accepted.SetExporter(&LoopbackTraceExporter{Endpoint: "http://127.0.0.1:4318/v1/traces"})
	accepted.FinishSpan(mustSpan(t, accepted), http.StatusCreated)
	spans, ok := accepted.Export()
	if !ok || len(spans) != 1 {
		t.Fatalf("loopback export ok=%v spans=%d", ok, len(spans))
	}
	if accepted.Snapshot().Exporter != "loopback" {
		t.Fatalf("exporter = %q", accepted.Snapshot().Exporter)
	}
	encoded, err := json.Marshal(spans)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "127.0.0.1:4318") {
		t.Fatalf("exported spans named the endpoint: %s", encoded)
	}
}

func TestLocalTraceEndpointRejectsNonLoopbackAndNonGet(t *testing.T) {
	trace := NewOpsTrace()
	trace.FinishSpan(mustSpan(t, trace), http.StatusOK)

	nonLoopback := httptest.NewRequest(http.MethodGet, LocalTracePath, nil)
	nonLoopback.RemoteAddr = "203.0.113.8:4242"
	rec := httptest.NewRecorder()
	trace.Handler().ServeHTTP(rec, nonLoopback)
	if rec.Code != http.StatusForbidden || rec.Body.Len() != 0 {
		t.Fatalf("non-loopback = %d body %q", rec.Code, rec.Body.String())
	}

	post := httptest.NewRequest(http.MethodPost, LocalTracePath, strings.NewReader(`{"dsn":"postgres://secret"}`))
	post.RemoteAddr = "127.0.0.1:9"
	rec = httptest.NewRecorder()
	trace.Handler().ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "postgres") || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("POST leaked body: %s", rec.Body.String())
	}
}

func TestOpsTraceNilWrapIsPassthrough(t *testing.T) {
	var trace *OpsTrace
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if trace.Wrap(next) == nil {
		t.Fatal("nil trace Wrap returned nil")
	}
	req := httptest.NewRequest(http.MethodGet, "/local/build-info", nil)
	req.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	trace.Wrap(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("passthrough status = %d", rec.Code)
	}
	if trace.Wrap(nil) != nil {
		t.Fatal("nil next should stay nil")
	}
	ctx, span := trace.StartSpan(context.Background(), otelTraceSpanName, nil)
	if span != nil || ctx == nil {
		t.Fatal("nil tracer started a span")
	}
	if trace.FinishSpan(context.Background(), http.StatusOK) != nil {
		t.Fatal("nil tracer finished a span")
	}
}

func mustSpan(t *testing.T, trace *OpsTrace) context.Context {
	t.Helper()
	ctx, span := trace.StartSpan(context.Background(), otelTraceSpanName, map[string]string{
		"http.method": http.MethodGet,
		"http.target": "/local/build-info",
	})
	if span == nil {
		t.Fatal("expected span")
	}
	return ctx
}
