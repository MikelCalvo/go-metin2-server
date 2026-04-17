package ops

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestHealthzEndpointIncludesServiceName(t *testing.T) {
    mux := NewPprofMux("gamed")

    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rec := httptest.NewRecorder()

    mux.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
    }

    if !strings.Contains(rec.Body.String(), "gamed ok") {
        t.Fatalf("unexpected health body %q", rec.Body.String())
    }
}

func TestPprofIndexIsReachable(t *testing.T) {
    mux := NewPprofMux("gamed")

    req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
    rec := httptest.NewRecorder()

    mux.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
    }

    if !strings.Contains(rec.Body.String(), "profile") {
        t.Fatalf("expected pprof index page, got %q", rec.Body.String())
    }
}
