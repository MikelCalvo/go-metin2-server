package ops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sqlDriversEnvelopeGot struct {
	Format  string   `json:"format"`
	Drivers []string `json:"drivers"`
}

func TestLocalSQLDriversEndpointReturnsEnvelopeForLoopbackGet(t *testing.T) {
	calls := 0
	mux := RegisterLocalSQLDriversEndpoint(NewPprofMux("gamed"), func() []string {
		calls++
		return []string{"sqlite", "go_metin2_ops_driver_test"}
	})

	req := httptest.NewRequest(http.MethodGet, "/local/db/drivers", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("expected drivers provider to be called once, got %d", calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	got := decodeSQLDriversEnvelope(t, rec.Body.Bytes())
	if len(got.Drivers) != 2 || got.Drivers[0] != "go_metin2_ops_driver_test" || got.Drivers[1] != "sqlite" {
		t.Fatalf("expected sorted drivers copy in envelope, got %#v body=%s", got, rec.Body.String())
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "UpSQL", "DownSQL", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("sql-drivers endpoint must not expose %q, got %s", forbidden, body)
		}
	}
}

func TestLocalSQLDriversEndpointReturnsEmptyArrayWhenProviderReturnsNil(t *testing.T) {
	mux := RegisterLocalSQLDriversEndpoint(NewPprofMux("gamed"), func() []string { return nil })
	req := httptest.NewRequest(http.MethodGet, "/local/db/drivers", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected empty linked list to be 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	got := decodeSQLDriversEnvelope(t, rec.Body.Bytes())
	if got.Drivers == nil {
		t.Fatalf("drivers must be [] not null, body=%s", rec.Body.String())
	}
	if len(got.Drivers) != 0 {
		t.Fatalf("expected empty drivers list, got %#v", got)
	}
}

func TestLocalSQLDriversEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	calls := 0
	mux := RegisterLocalSQLDriversEndpoint(NewPprofMux("gamed"), func() []string {
		calls++
		return []string{"sqlite"}
	})
	req := httptest.NewRequest(http.MethodGet, "/local/db/drivers", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if calls != 0 {
		t.Fatalf("expected drivers provider not to be called, got %d", calls)
	}
}

func TestLocalSQLDriversEndpointRejectsWrongMethod(t *testing.T) {
	calls := 0
	mux := RegisterLocalSQLDriversEndpoint(NewPprofMux("gamed"), func() []string {
		calls++
		return []string{"sqlite"}
	})
	req := httptest.NewRequest(http.MethodPost, "/local/db/drivers", strings.NewReader(`{}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if calls != 0 {
		t.Fatalf("expected drivers provider not to be called, got %d", calls)
	}
}

func TestRegisterLocalSQLDriversEndpointNoopsOnNilMuxOrProvider(t *testing.T) {
	if got := RegisterLocalSQLDriversEndpoint(nil, func() []string { return []string{"sqlite"} }); got != nil {
		t.Fatalf("expected nil mux to stay nil, got %#v", got)
	}
	mux := NewPprofMux("gamed")
	if got := RegisterLocalSQLDriversEndpoint(mux, nil); got != mux {
		t.Fatalf("expected nil provider to return the same mux")
	}
	req := httptest.NewRequest(http.MethodGet, "/local/db/drivers", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected unregistered route 404, got %d", rec.Code)
	}
}

func decodeSQLDriversEnvelope(t *testing.T, raw []byte) sqlDriversEnvelopeGot {
	t.Helper()
	var got sqlDriversEnvelopeGot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode sql-drivers JSON: %v\nbody:\n%s", err, raw)
	}
	if got.Format != "go-metin2-sql-drivers-v1" {
		t.Fatalf("unexpected sql-drivers format: %#v body=%s", got, raw)
	}
	if string(raw) == "null" || strings.Contains(string(raw), `"drivers":null`) {
		t.Fatalf("drivers must be a JSON array, got %s", raw)
	}
	if got.Drivers == nil {
		t.Fatalf("drivers must decode as empty slice not nil, body=%s", raw)
	}
	return got
}
