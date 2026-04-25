package ops

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalInteractionDefinitionsEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubInteractionDefinitionSnapshotter{definitions: []map[string]any{{"kind": "info", "ref": "lore:alchemist", "text": "The alchemist studies forgotten herbs."}, {"kind": "talk", "ref": "npc:village_guard", "text": "Keep your blade sharp."}}}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), snapshotter.InteractionDefinitions, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/interactions", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected interaction definition snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"info"`) || !strings.Contains(string(body), `"ref":"npc:village_guard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInteractionDefinitionsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubInteractionDefinitionSnapshotter{}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), snapshotter.InteractionDefinitions, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/interactions", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected interaction definition snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalInteractionDefinitionsEndpointCreatesDefinitionForLoopbackPost(t *testing.T) {
	creator := &stubInteractionDefinitionCreator{status: http.StatusOK, definition: map[string]any{"kind": "info", "ref": "lore:alchemist", "text": "The alchemist studies forgotten herbs."}}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), nil, creator.CreateInteractionDefinition)

	req := httptest.NewRequest(http.MethodPost, "/local/interactions", strings.NewReader(`{"kind":"info","ref":"lore:alchemist","text":"The alchemist studies forgotten herbs."}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if creator.calls != 1 || creator.lastKind != "info" || creator.lastRef != "lore:alchemist" || creator.lastText != "The alchemist studies forgotten herbs." {
		t.Fatalf("unexpected interaction definition creator call state: %+v", creator)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"info"`) || !strings.Contains(string(body), `"ref":"lore:alchemist"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInteractionDefinitionsEndpointPropagatesCreateStatusForLoopbackPost(t *testing.T) {
	creator := &stubInteractionDefinitionCreator{status: http.StatusConflict}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), nil, creator.CreateInteractionDefinition)

	req := httptest.NewRequest(http.MethodPost, "/local/interactions", strings.NewReader(`{"kind":"info","ref":"lore:alchemist","text":"duplicate"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if creator.calls != 1 {
		t.Fatalf("expected interaction definition creator to be called once, got %d calls", creator.calls)
	}
}

func TestLocalInteractionDefinitionUpdateEndpointUpsertsDefinitionForLoopbackPatch(t *testing.T) {
	updater := &stubInteractionDefinitionUpdater{status: http.StatusOK, definition: map[string]any{"kind": "talk", "ref": "npc:village_guard", "text": "Keep your blade sharp."}}
	mux := RegisterLocalInteractionDefinitionUpdateEndpoint(NewPprofMux("gamed"), updater.UpsertInteractionDefinition)

	req := httptest.NewRequest(http.MethodPatch, "/local/interactions/talk/npc:village_guard", strings.NewReader(`{"kind":"talk","ref":"npc:village_guard","text":"Keep your blade sharp."}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if updater.calls != 1 || updater.lastKind != "talk" || updater.lastRef != "npc:village_guard" || updater.lastText != "Keep your blade sharp." {
		t.Fatalf("unexpected interaction definition updater call state: %+v", updater)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"talk"`) || !strings.Contains(string(body), `"ref":"npc:village_guard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInteractionDefinitionUpdateEndpointRejectsIdentityMismatch(t *testing.T) {
	updater := &stubInteractionDefinitionUpdater{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionUpdateEndpoint(NewPprofMux("gamed"), updater.UpsertInteractionDefinition)

	req := httptest.NewRequest(http.MethodPatch, "/local/interactions/talk/npc:village_guard", strings.NewReader(`{"kind":"info","ref":"lore:alchemist","text":"wrong identity"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected interaction definition updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalInteractionDefinitionDeleteEndpointRemovesDefinitionForLoopbackDelete(t *testing.T) {
	remover := &stubInteractionDefinitionRemover{status: http.StatusOK, definition: map[string]any{"kind": "info", "ref": "lore:alchemist", "text": "The alchemist studies forgotten herbs."}}
	mux := RegisterLocalInteractionDefinitionDeleteEndpoint(NewPprofMux("gamed"), remover.RemoveInteractionDefinition)

	req := httptest.NewRequest(http.MethodDelete, "/local/interactions/info/lore:alchemist", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if remover.calls != 1 || remover.lastKind != "info" || remover.lastRef != "lore:alchemist" {
		t.Fatalf("unexpected interaction definition remover call state: %+v", remover)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"info"`) || !strings.Contains(string(body), `"ref":"lore:alchemist"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInteractionDefinitionDeleteEndpointPropagatesConflictStatus(t *testing.T) {
	remover := &stubInteractionDefinitionRemover{status: http.StatusConflict}
	mux := RegisterLocalInteractionDefinitionDeleteEndpoint(NewPprofMux("gamed"), remover.RemoveInteractionDefinition)

	req := httptest.NewRequest(http.MethodDelete, "/local/interactions/talk/npc:village_guard", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	if remover.calls != 1 || remover.lastKind != "talk" || remover.lastRef != "npc:village_guard" {
		t.Fatalf("expected interaction definition remover to be called for conflict path, got %+v", remover)
	}
}

type stubInteractionDefinitionSnapshotter struct {
	definitions any
	calls       int
}

func (s *stubInteractionDefinitionSnapshotter) InteractionDefinitions() any {
	s.calls++
	return s.definitions
}

type stubInteractionDefinitionCreator struct {
	definition map[string]any
	status     int
	calls      int
	lastKind   string
	lastRef    string
	lastText   string
}

func (s *stubInteractionDefinitionCreator) CreateInteractionDefinition(kind string, ref string, text string) (any, int) {
	s.calls++
	s.lastKind = kind
	s.lastRef = ref
	s.lastText = text
	return s.definition, s.status
}

type stubInteractionDefinitionUpdater struct {
	definition map[string]any
	status     int
	calls      int
	lastKind   string
	lastRef    string
	lastText   string
}

func (s *stubInteractionDefinitionUpdater) UpsertInteractionDefinition(kind string, ref string, text string) (any, int) {
	s.calls++
	s.lastKind = kind
	s.lastRef = ref
	s.lastText = text
	return s.definition, s.status
}

type stubInteractionDefinitionRemover struct {
	definition map[string]any
	status     int
	calls      int
	lastKind   string
	lastRef    string
}

func (s *stubInteractionDefinitionRemover) RemoveInteractionDefinition(kind string, ref string) (any, int) {
	s.calls++
	s.lastKind = kind
	s.lastRef = ref
	return s.definition, s.status
}
