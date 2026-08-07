package ops

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalCharacterVisibilityEndpointReturnsExactSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{snapshots: map[string]any{"Mkmk Sura": map[string]any{"name": "Mkmk Sura", "visible_static_actors": []map[string]any{{"entity_id": uint64(7), "name": "VillageGuard"}}}}}
	mux := RegisterLocalCharacterVisibilityEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility/Mkmk%20Sura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "Mkmk Sura" {
		t.Fatalf("expected decoded visibility snapshot name Mkmk Sura, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"visible_static_actors"`) || !strings.Contains(string(body), `"name":"VillageGuard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalCharacterVisibilityEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalCharacterVisibilityEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility/MkmkSura", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected visibility snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalCharacterVisibilityEndpointRejectsInvalidCharacterName(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalCharacterVisibilityEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility/Mkmk%2FSura", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected visibility snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalCharacterVisibilityEndpointReturnsNotFoundForMissingCharacter(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{snapshots: map[string]any{}}
	mux := RegisterLocalCharacterVisibilityEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility/Missing", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if snapshotter.calls != 1 || snapshotter.lastName != "Missing" {
		t.Fatalf("expected visibility snapshotter to be called for Missing, got calls=%d name=%q", snapshotter.calls, snapshotter.lastName)
	}
}

func TestLocalCharacterVisibilityEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubNamedSnapshotter{}
	mux := RegisterLocalCharacterVisibilityEndpoint(NewPprofMux("gamed"), snapshotter.Snapshot)

	req := httptest.NewRequest(http.MethodPost, "/local/visibility/MkmkSura", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected visibility snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}
