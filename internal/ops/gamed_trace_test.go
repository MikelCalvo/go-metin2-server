package ops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/observability"
)

func TestGamedOpsMuxMountsLocalTrace(t *testing.T) {
	mux := NewPprofMux("gamed")
	handler := observability.WrapOpsAccessLog(nil, mux)

	status, body := getLocal(handler, "/local/build-info")
	if status != http.StatusOK {
		t.Fatalf("build-info status = %d, want 200", status)
	}
	if !strings.Contains(body, "{") {
		t.Fatal("build-info response is not JSON")
	}

	mux.HandleFunc("/local/late-probe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	status, _ = getLocal(handler, "/local/late-probe")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("late probe status = %d, want 503", status)
	}

	status, nested := getLocal(handler, "/local/persistence/status")
	if status != http.StatusNotFound {
		t.Fatalf("nested status = %d, want 404", status)
	}
	if strings.Contains(nested, "span_count") {
		t.Fatal("nested miss served the trace document")
	}

	status, _ = getLocal(handler, "/healthz")
	if status != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", status)
	}
	status, _ = getLocal(handler, "/debug/pprof/")
	if status == http.StatusNotFound {
		t.Fatal("pprof index was not registered")
	}

	status, secret := getLocalRaw(handler, "/local/trace?token=secret", "secret-body")
	if status != http.StatusOK {
		t.Fatalf("trace status = %d, want 200", status)
	}
	if strings.Contains(secret, "secret") {
		t.Fatalf("trace document echoed the request: %s", secret)
	}
	var snap observability.OpsTraceSnapshot
	if err := json.Unmarshal([]byte(secret), &snap); err != nil {
		t.Fatalf("decode trace: %v", err)
	}
	if snap.Service != "gamed" || snap.Exporter != "fail-closed" || snap.SpanCount != 2 || len(snap.Spans) != 2 {
		t.Fatalf("snapshot = %+v, want gamed fail-closed span_count 2", snap)
	}
	if snap.Spans[0].Attributes["http.target"] != "/local/build-info" || snap.Spans[0].StatusCode != 1 {
		t.Fatalf("first span = %+v", snap.Spans[0])
	}
	last := snap.Spans[1]
	if last.Attributes["http.target"] != "/local/late-probe" || last.StatusCode != 2 || last.Name != "ops.local.request" {
		t.Fatalf("error span = %+v", last)
	}
	for _, span := range snap.Spans {
		if span.Attributes["http.target"] == "/local/trace" || span.Attributes["http.target"] == "/healthz" || strings.Contains(span.Attributes["http.target"], "persistence") {
			t.Fatalf("unexpected span target %+v", span)
		}
	}

	status, prom := getLocal(handler, "/local/metrics/prometheus")
	if status != http.StatusNotFound || prom != "" && strings.Contains(prom, "metin2_ops") {
		t.Fatalf("prometheus mount status = %d body %q", status, prom)
	}

	status, forbidden := getLocalRaw(handler, "/local/trace", "")
	req := httptest.NewRequest(http.MethodGet, "/local/trace", nil)
	req.RemoteAddr = "10.1.2.3:9"
	forbiddenRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenRec, req)
	if forbiddenRec.Code != http.StatusForbidden || forbiddenRec.Body.Len() != 0 {
		t.Fatalf("non-loopback trace = %d %q", forbiddenRec.Code, forbiddenRec.Body.String())
	}
	if status != http.StatusOK || strings.Contains(forbidden, "10.1.2.3") {
		t.Fatal("loopback trace read changed")
	}
}

func TestAuthdOpsMuxDoesNotMountLocalTrace(t *testing.T) {
	mux := NewPprofMux("authd")
	handler := observability.WrapOpsAccessLog(nil, mux)

	status, body := getLocal(handler, "/local/trace")
	if status != http.StatusNotFound {
		t.Fatalf("authd trace status = %d, want 404", status)
	}
	if strings.Contains(body, "span_count") {
		t.Fatal("authd served the gamed trace document")
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
