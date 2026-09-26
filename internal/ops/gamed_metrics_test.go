package ops

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/observability"
)

func TestGamedOpsMuxMountsLocalMetrics(t *testing.T) {
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
	if strings.Contains(nested, "local_requests_total") {
		t.Fatal("nested miss served the metrics document")
	}

	status, _ = getLocal(handler, "/healthz")
	if status != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", status)
	}
	status, _ = getLocal(handler, "/debug/pprof/")
	if status == http.StatusNotFound {
		t.Fatal("pprof index was not registered")
	}

	status, secret := getLocalRaw(handler, "/local/metrics?token=secret", "secret-body")
	if status != http.StatusOK {
		t.Fatalf("metrics status = %d, want 200", status)
	}
	if strings.Contains(secret, "secret") {
		t.Fatalf("metrics document echoed the request: %s", secret)
	}
	var snap observability.OpsMetricsSnapshot
	if err := json.Unmarshal([]byte(secret), &snap); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if snap.Service != "gamed" || snap.LocalRequestsTotal != 2 || snap.LocalErrorsTotal != 1 {
		t.Fatalf("snapshot = %+v, want gamed total 2 errors 1", snap)
	}
	if snap.LocalRequestsByPath["/local/build-info"] != 1 || snap.LocalRequestsByPath["/local/late-probe"] != 1 {
		t.Fatalf("paths = %+v", snap.LocalRequestsByPath)
	}
	if _, ok := snap.LocalRequestsByPath["/local/metrics"]; ok {
		t.Fatal("reading metrics counted itself")
	}
	if _, ok := snap.LocalRequestsByPath["/healthz"]; ok {
		t.Fatal("healthz was counted")
	}
	if _, ok := snap.LocalRequestsByPath["/local/persistence/status"]; ok {
		t.Fatal("nested path was counted")
	}

	status, forbidden := getLocalRaw(handler, "/local/metrics", "")
	req := httptest.NewRequest(http.MethodGet, "/local/metrics", nil)
	req.RemoteAddr = "10.1.2.3:9"
	forbiddenRec := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenRec, req)
	if forbiddenRec.Code != http.StatusForbidden || forbiddenRec.Body.Len() != 0 {
		t.Fatalf("non-loopback metrics = %d %q", forbiddenRec.Code, forbiddenRec.Body.String())
	}
	if status != http.StatusOK || strings.Contains(forbidden, "10.1.2.3") {
		t.Fatal("loopback metrics read changed")
	}
}

func TestAuthdOpsMuxDoesNotMountLocalMetrics(t *testing.T) {
	mux := NewPprofMux("authd")
	handler := observability.WrapOpsAccessLog(nil, mux)

	status, body := getLocal(handler, "/local/metrics")
	if status != http.StatusNotFound {
		t.Fatalf("authd metrics status = %d, want 404", status)
	}
	if strings.Contains(body, "local_requests_total") {
		t.Fatal("authd served the gamed metrics document")
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

func getLocal(handler http.Handler, path string) (int, string) {
	return getLocalRaw(handler, path, "")
}

func getLocalRaw(handler http.Handler, path, body string) (int, string) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(http.MethodGet, path, reader)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}
