package ops

import (
	"io"
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

func TestLocalNoticeEndpointQueuesBroadcastForLoopbackPost(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 2}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader("server maintenance"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if broadcaster.calls != 1 || broadcaster.lastMessage != "server maintenance" {
		t.Fatalf("unexpected broadcaster call state: %+v", broadcaster)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if string(body) != "queued 2\n" {
		t.Fatalf("unexpected response body %q", string(body))
	}
}

func TestLocalNoticeEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 2}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader("server maintenance"))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d calls", broadcaster.calls)
	}
}

func TestLocalNoticeEndpointRejectsEmptyMessage(t *testing.T) {
	broadcaster := &stubNoticeBroadcaster{delivered: 2}
	mux := NewPprofMuxWithLocalNotice("gamed", broadcaster.BroadcastNotice)

	req := httptest.NewRequest(http.MethodPost, "/local/notice", strings.NewReader("   \n"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected broadcaster not to be called, got %d calls", broadcaster.calls)
	}
}

func TestLocalRelocateEndpointRelocatesConnectedCharacterForLoopbackPost(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: true}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if relocator.calls != 1 || relocator.lastName != "PeerTwo" || relocator.lastMapIndex != 42 || relocator.lastX != 1700 || relocator.lastY != 2800 {
		t.Fatalf("unexpected relocator call state: %+v", relocator)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if string(body) != "relocated 1\n" {
		t.Fatalf("unexpected response body %q", string(body))
	}
}

func TestLocalRelocateEndpointRejectsInvalidBody(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: true}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"PeerTwo","map_index":0,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if relocator.calls != 0 {
		t.Fatalf("expected relocator not to be called, got %d calls", relocator.calls)
	}
}

func TestLocalRelocateEndpointRejectsTrailingJSON(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: true}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}{"extra":1}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if relocator.calls != 0 {
		t.Fatalf("expected relocator not to be called, got %d calls", relocator.calls)
	}
}

func TestLocalRelocateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: true}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if relocator.calls != 0 {
		t.Fatalf("expected relocator not to be called, got %d calls", relocator.calls)
	}
}

func TestLocalRelocateEndpointReturnsNotFoundForUnknownTarget(t *testing.T) {
	relocator := &stubCharacterRelocator{relocated: false}
	mux := NewPprofMuxWithLocalRelocation("gamed", nil, relocator.RelocateCharacter)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate", strings.NewReader(`{"name":"MissingPeer","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if relocator.calls != 1 {
		t.Fatalf("expected relocator to be called once, got %d calls", relocator.calls)
	}
}

func TestLocalTransferEndpointReturnsStructuredJSONForLoopbackPost(t *testing.T) {
	transferer := &stubCharacterTransferer{found: true, result: map[string]any{
		"applied":               true,
		"character":             map[string]any{"name": "PeerTwo", "map_index": uint32(1), "x": int32(1300), "y": int32(2300)},
		"target":                map[string]any{"name": "PeerTwo", "map_index": uint32(42), "x": int32(1700), "y": int32(2800)},
		"removed_visible_peers": []map[string]any{{"name": "PeerOne"}},
		"added_visible_peers":   []map[string]any{{"name": "PeerThree"}},
		"map_occupancy_changes": []map[string]any{{"map_index": uint32(1), "before_count": 2, "after_count": 1}, {"map_index": uint32(42), "before_count": 1, "after_count": 2}},
		"before_map_occupancy":  []map[string]any{{"map_index": uint32(1), "character_count": 2, "characters": []map[string]any{{"name": "PeerOne"}, {"name": "PeerTwo"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}}}, {"map_index": uint32(42), "character_count": 1, "characters": []map[string]any{{"name": "PeerThree"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}}}},
		"after_map_occupancy":   []map[string]any{{"map_index": uint32(1), "character_count": 1, "characters": []map[string]any{{"name": "PeerOne"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}}}, {"map_index": uint32(42), "character_count": 2, "characters": []map[string]any{{"name": "PeerThree"}, {"name": "PeerTwo"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}}}},
	}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/transfer", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if transferer.calls != 1 || transferer.lastName != "PeerTwo" || transferer.lastMapIndex != 42 || transferer.lastX != 1700 || transferer.lastY != 2800 {
		t.Fatalf("unexpected transferer call state: %+v", transferer)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"applied":true`) || !strings.Contains(string(body), `"map_occupancy_changes"`) || !strings.Contains(string(body), `"before_map_occupancy"`) || !strings.Contains(string(body), `"after_map_occupancy"`) || !strings.Contains(string(body), `"static_actor_count":1`) || !strings.Contains(string(body), `"name":"PeerThree"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalTransferEndpointRejectsInvalidBody(t *testing.T) {
	transferer := &stubCharacterTransferer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/transfer", strings.NewReader(`{"name":"PeerTwo","map_index":0,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if transferer.calls != 0 {
		t.Fatalf("expected transferer not to be called, got %d calls", transferer.calls)
	}
}

func TestLocalTransferEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	transferer := &stubCharacterTransferer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/transfer", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if transferer.calls != 0 {
		t.Fatalf("expected transferer not to be called, got %d calls", transferer.calls)
	}
}

func TestLocalTransferEndpointReturnsNotFoundForUnknownTarget(t *testing.T) {
	transferer := &stubCharacterTransferer{found: false}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/transfer", strings.NewReader(`{"name":"MissingPeer","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if transferer.calls != 1 {
		t.Fatalf("expected transferer to be called once, got %d calls", transferer.calls)
	}
}

func TestLocalTransferEndpointRejectsWrongMethod(t *testing.T) {
	transferer := &stubCharacterTransferer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, transferer.TransferCharacter, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/transfer", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if transferer.calls != 0 {
		t.Fatalf("expected transferer not to be called, got %d calls", transferer.calls)
	}
}

func TestLocalRelocatePreviewEndpointReturnsJSONSnapshotForLoopbackPost(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: true, preview: map[string]any{
		"character":             map[string]any{"name": "PeerTwo", "map_index": uint32(1), "x": int32(1300), "y": int32(2300)},
		"target":                map[string]any{"name": "PeerTwo", "map_index": uint32(42), "x": int32(1700), "y": int32(2800)},
		"removed_visible_peers": []map[string]any{{"name": "PeerOne"}},
		"added_visible_peers":   []map[string]any{{"name": "PeerThree"}},
		"map_occupancy_changes": []map[string]any{{"map_index": uint32(1), "before_count": 2, "after_count": 1}, {"map_index": uint32(42), "before_count": 1, "after_count": 2}},
		"before_map_occupancy":  []map[string]any{{"map_index": uint32(1), "character_count": 2, "characters": []map[string]any{{"name": "PeerOne"}, {"name": "PeerTwo"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}}}, {"map_index": uint32(42), "character_count": 1, "characters": []map[string]any{{"name": "PeerThree"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}}}},
		"after_map_occupancy":   []map[string]any{{"map_index": uint32(1), "character_count": 1, "characters": []map[string]any{{"name": "PeerOne"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(1), "name": "Blacksmith"}}}, {"map_index": uint32(42), "character_count": 2, "characters": []map[string]any{{"name": "PeerThree"}, {"name": "PeerTwo"}}, "static_actor_count": 1, "static_actors": []map[string]any{{"entity_id": uint64(2), "name": "VillageGuard"}}}},
	}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate-preview", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 || previewer.lastName != "PeerTwo" || previewer.lastMapIndex != 42 || previewer.lastX != 1700 || previewer.lastY != 2800 {
		t.Fatalf("unexpected previewer call state: %+v", previewer)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"removed_visible_peers"`) || !strings.Contains(string(body), `"map_occupancy_changes"`) || !strings.Contains(string(body), `"before_map_occupancy"`) || !strings.Contains(string(body), `"after_map_occupancy"`) || !strings.Contains(string(body), `"static_actor_count":1`) || !strings.Contains(string(body), `"name":"PeerThree"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalRelocatePreviewEndpointRejectsInvalidBody(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate-preview", strings.NewReader(`{"name":"PeerTwo","map_index":0,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d calls", previewer.calls)
	}
}

func TestLocalRelocatePreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate-preview", strings.NewReader(`{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d calls", previewer.calls)
	}
}

func TestLocalRelocatePreviewEndpointReturnsNotFoundForUnknownTarget(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: false}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/relocate-preview", strings.NewReader(`{"name":"MissingPeer","map_index":42,"x":1700,"y":2800}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected previewer to be called once, got %d calls", previewer.calls)
	}
}

func TestLocalRelocatePreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubRelocationPreviewer{found: true}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, previewer.PreviewRelocation, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/relocate-preview", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected previewer not to be called, got %d calls", previewer.calls)
	}
}

func TestLocalPlayersEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubConnectedCharactersSnapshotter{characters: []map[string]any{{"name": "Alpha", "map_index": 42, "x": int32(1700), "y": int32(2800)}, {"name": "Zulu", "map_index": uint32(1), "x": int32(1100), "y": int32(2100)}}}
	mux := NewPprofMuxWithLocalRuntimeSnapshot("gamed", nil, nil, snapshotter.ConnectedCharacters)

	req := httptest.NewRequest(http.MethodGet, "/local/players", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"name":"Alpha"`) || !strings.Contains(string(body), `"name":"Zulu"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalPlayersEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubConnectedCharactersSnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeSnapshot("gamed", nil, nil, snapshotter.ConnectedCharacters)

	req := httptest.NewRequest(http.MethodGet, "/local/players", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalPlayersEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubConnectedCharactersSnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeSnapshot("gamed", nil, nil, snapshotter.ConnectedCharacters)

	req := httptest.NewRequest(http.MethodPost, "/local/players", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalVisibilityEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubCharacterVisibilitySnapshotter{snapshots: []map[string]any{{"name": "Alpha", "map_index": 42, "visible_peers": []map[string]any{{"name": "PeerTwo", "map_index": 42}}}, {"name": "Zulu", "map_index": uint32(1), "visible_peers": []map[string]any{}}}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, snapshotter.CharacterVisibility, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected visibility snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"name":"Alpha"`) || !strings.Contains(string(body), `"visible_peers":[`) || !strings.Contains(string(body), `"name":"PeerTwo"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalVisibilityEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubCharacterVisibilitySnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, snapshotter.CharacterVisibility, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/visibility", nil)
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

func TestLocalVisibilityEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubCharacterVisibilitySnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, snapshotter.CharacterVisibility, nil)

	req := httptest.NewRequest(http.MethodPost, "/local/visibility", strings.NewReader("ignored"))
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

func TestLocalRuntimeConfigEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubRuntimeConfigSnapshotter{snapshot: map[string]any{"local_channel_id": 1, "visibility_mode": "radius", "visibility_radius": int32(400), "visibility_sector_size": int32(200)}}
	mux := RegisterLocalRuntimeConfigEndpoint(NewPprofMux("gamed"), snapshotter.RuntimeConfig)

	req := httptest.NewRequest(http.MethodGet, "/local/runtime-config", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected runtime config snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"visibility_mode":"radius"`) || !strings.Contains(string(body), `"visibility_radius":400`) || !strings.Contains(string(body), `"visibility_sector_size":200`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalRuntimeConfigEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubRuntimeConfigSnapshotter{}
	mux := RegisterLocalRuntimeConfigEndpoint(NewPprofMux("gamed"), snapshotter.RuntimeConfig)

	req := httptest.NewRequest(http.MethodGet, "/local/runtime-config", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected runtime config snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalRuntimeConfigEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubRuntimeConfigSnapshotter{}
	mux := RegisterLocalRuntimeConfigEndpoint(NewPprofMux("gamed"), snapshotter.RuntimeConfig)

	req := httptest.NewRequest(http.MethodPost, "/local/runtime-config", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected runtime config snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalMapsEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubMapOccupancySnapshotter{snapshots: []map[string]any{{"map_index": uint32(1), "character_count": 1, "characters": []map[string]any{{"name": "Zulu"}}}, {"map_index": uint32(42), "character_count": 2, "characters": []map[string]any{{"name": "Alpha"}, {"name": "PeerTwo"}}}}}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, nil, snapshotter.MapOccupancy)

	req := httptest.NewRequest(http.MethodGet, "/local/maps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected map snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"map_index":42`) || !strings.Contains(string(body), `"character_count":2`) || !strings.Contains(string(body), `"name":"PeerTwo"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalMapsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubMapOccupancySnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, nil, snapshotter.MapOccupancy)

	req := httptest.NewRequest(http.MethodGet, "/local/maps", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected map snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalMapsEndpointRejectsWrongMethod(t *testing.T) {
	snapshotter := &stubMapOccupancySnapshotter{}
	mux := NewPprofMuxWithLocalRuntimeIntrospection("gamed", nil, nil, nil, nil, nil, nil, snapshotter.MapOccupancy)

	req := httptest.NewRequest(http.MethodPost, "/local/maps", strings.NewReader("ignored"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected map snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalStaticActorsEndpointReturnsJSONSnapshotForLoopbackGet(t *testing.T) {
	snapshotter := &stubStaticActorSnapshotter{actors: []map[string]any{{"entity_id": uint64(2), "name": "Blacksmith", "map_index": uint32(42), "x": int32(1900), "y": int32(3000), "race_num": uint32(20301)}, {"entity_id": uint64(1), "name": "VillageGuard", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20300)}}}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), snapshotter.StaticActors, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actors", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("expected static actor snapshotter to be called once, got %d calls", snapshotter.calls)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"name":"Blacksmith"`) || !strings.Contains(string(body), `"race_num":20300`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorsEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	snapshotter := &stubStaticActorSnapshotter{}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), snapshotter.StaticActors, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/static-actors", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected static actor snapshotter not to be called, got %d calls", snapshotter.calls)
	}
}

func TestLocalStaticActorsEndpointRegistersActorForLoopbackPost(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true, actor: map[string]any{"entity_id": uint64(1), "name": "VillageGuard", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20300)}}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(`{"name":"VillageGuard","map_index":42,"x":1700,"y":2800,"race_num":20300}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if registrar.calls != 1 || registrar.lastName != "VillageGuard" || registrar.lastMapIndex != 42 || registrar.lastX != 1700 || registrar.lastY != 2800 || registrar.lastRaceNum != 20300 {
		t.Fatalf("unexpected static actor registrar call state: %+v", registrar)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":1`) || !strings.Contains(string(body), `"name":"VillageGuard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorsEndpointRejectsInvalidSeedBody(t *testing.T) {
	registrar := &stubStaticActorRegistrar{registered: true}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), nil, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodPost, "/local/static-actors", strings.NewReader(`{"name":"VillageGuard","map_index":0,"x":1700,"y":2800,"race_num":20300}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if registrar.calls != 0 {
		t.Fatalf("expected static actor registrar not to be called, got %d calls", registrar.calls)
	}
}

func TestLocalStaticActorsEndpointRejectsUnsupportedMethod(t *testing.T) {
	snapshotter := &stubStaticActorSnapshotter{}
	registrar := &stubStaticActorRegistrar{registered: true}
	mux := RegisterLocalStaticActorEndpoints(NewPprofMux("gamed"), snapshotter.StaticActors, registrar.RegisterStaticActor)

	req := httptest.NewRequest(http.MethodDelete, "/local/static-actors", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("expected static actor snapshotter not to be called, got %d calls", snapshotter.calls)
	}
	if registrar.calls != 0 {
		t.Fatalf("expected static actor registrar not to be called, got %d calls", registrar.calls)
	}
}

func TestLocalStaticActorUpdateEndpointUpdatesActorForLoopbackPatch(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true, actor: map[string]any{"entity_id": uint64(7), "name": "Merchant", "map_index": uint32(99), "x": int32(900), "y": int32(1200), "race_num": uint32(9001)}}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if updater.calls != 1 || updater.lastEntityID != 7 || updater.lastName != "Merchant" || updater.lastMapIndex != 99 || updater.lastX != 900 || updater.lastY != 1200 || updater.lastRaceNum != 9001 {
		t.Fatalf("unexpected static actor updater call state: %+v", updater)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":7`) || !strings.Contains(string(body), `"name":"Merchant"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorUpdateEndpointRejectsInvalidBody(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":0,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointRejectsInvalidEntityID(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/not-a-number", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointReturnsNotFoundForUnknownActor(t *testing.T) {
	updater := &stubStaticActorUpdater{}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if updater.calls != 1 || updater.lastEntityID != 7 {
		t.Fatalf("expected static actor updater to be called once for not-found path, got %+v", updater)
	}
}

func TestLocalStaticActorUpdateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodPatch, "/local/static-actors/7", strings.NewReader(`{"name":"Merchant","map_index":99,"x":900,"y":1200,"race_num":9001}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorUpdateEndpointRejectsUnsupportedMethod(t *testing.T) {
	updater := &stubStaticActorUpdater{updated: true}
	mux := RegisterLocalStaticActorUpdateEndpoint(NewPprofMux("gamed"), updater.UpdateStaticActor)

	req := httptest.NewRequest(http.MethodDelete, "/local/static-actors/7", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected static actor updater not to be called, got %d calls", updater.calls)
	}
}

func TestLocalStaticActorDeleteEndpointRemovesActorForLoopbackDelete(t *testing.T) {
	remover := &stubStaticActorRemover{removed: true, actor: map[string]any{"entity_id": uint64(7), "name": "VillageGuard", "map_index": uint32(42), "x": int32(1700), "y": int32(2800), "race_num": uint32(20300)}}
	mux := RegisterLocalStaticActorDeleteEndpoint(NewPprofMux("gamed"), remover.RemoveStaticActor)

	req := httptest.NewRequest(http.MethodDelete, "/local/static-actors/7", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if remover.calls != 1 || remover.lastEntityID != 7 {
		t.Fatalf("unexpected static actor remover call state: %+v", remover)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"entity_id":7`) || !strings.Contains(string(body), `"name":"VillageGuard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalStaticActorDeleteEndpointRejectsInvalidEntityID(t *testing.T) {
	remover := &stubStaticActorRemover{removed: true}
	mux := RegisterLocalStaticActorDeleteEndpoint(NewPprofMux("gamed"), remover.RemoveStaticActor)

	req := httptest.NewRequest(http.MethodDelete, "/local/static-actors/not-a-number", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if remover.calls != 0 {
		t.Fatalf("expected static actor remover not to be called, got %d calls", remover.calls)
	}
}

type stubNoticeBroadcaster struct {
	delivered   int
	calls       int
	lastMessage string
}

func (b *stubNoticeBroadcaster) BroadcastNotice(message string) int {
	b.calls++
	b.lastMessage = message
	return b.delivered
}

type stubCharacterRelocator struct {
	relocated    bool
	calls        int
	lastName     string
	lastMapIndex uint32
	lastX        int32
	lastY        int32
}

func (r *stubCharacterRelocator) RelocateCharacter(name string, mapIndex uint32, x int32, y int32) bool {
	r.calls++
	r.lastName = name
	r.lastMapIndex = mapIndex
	r.lastX = x
	r.lastY = y
	return r.relocated
}

type stubConnectedCharactersSnapshotter struct {
	characters []map[string]any
	calls      int
}

func (s *stubConnectedCharactersSnapshotter) ConnectedCharacters() any {
	s.calls++
	return s.characters
}

type stubCharacterVisibilitySnapshotter struct {
	snapshots []map[string]any
	calls     int
}

func (s *stubCharacterVisibilitySnapshotter) CharacterVisibility() any {
	s.calls++
	return s.snapshots
}

type stubMapOccupancySnapshotter struct {
	snapshots []map[string]any
	calls     int
}

func (s *stubMapOccupancySnapshotter) MapOccupancy() any {
	s.calls++
	return s.snapshots
}

type stubRuntimeConfigSnapshotter struct {
	snapshot map[string]any
	calls    int
}

func (s *stubRuntimeConfigSnapshotter) RuntimeConfig() any {
	s.calls++
	return s.snapshot
}

type stubStaticActorSnapshotter struct {
	actors []map[string]any
	calls  int
}

func (s *stubStaticActorSnapshotter) StaticActors() any {
	s.calls++
	return s.actors
}

type stubStaticActorRegistrar struct {
	actor        map[string]any
	registered   bool
	calls        int
	lastName     string
	lastMapIndex uint32
	lastX        int32
	lastY        int32
	lastRaceNum  uint32
}

func (r *stubStaticActorRegistrar) RegisterStaticActor(name string, mapIndex uint32, x int32, y int32, raceNum uint32) (any, bool) {
	r.calls++
	r.lastName = name
	r.lastMapIndex = mapIndex
	r.lastX = x
	r.lastY = y
	r.lastRaceNum = raceNum
	return r.actor, r.registered
}

type stubStaticActorUpdater struct {
	actor        map[string]any
	updated      bool
	calls        int
	lastEntityID uint64
	lastName     string
	lastMapIndex uint32
	lastX        int32
	lastY        int32
	lastRaceNum  uint32
}

func (r *stubStaticActorUpdater) UpdateStaticActor(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32) (any, bool) {
	r.calls++
	r.lastEntityID = entityID
	r.lastName = name
	r.lastMapIndex = mapIndex
	r.lastX = x
	r.lastY = y
	r.lastRaceNum = raceNum
	return r.actor, r.updated
}

type stubStaticActorRemover struct {
	actor        map[string]any
	removed      bool
	calls        int
	lastEntityID uint64
}

func (r *stubStaticActorRemover) RemoveStaticActor(entityID uint64) (any, bool) {
	r.calls++
	r.lastEntityID = entityID
	return r.actor, r.removed
}

type stubRelocationPreviewer struct {
	preview      map[string]any
	found        bool
	calls        int
	lastName     string
	lastMapIndex uint32
	lastX        int32
	lastY        int32
}

func (p *stubRelocationPreviewer) PreviewRelocation(name string, mapIndex uint32, x int32, y int32) (any, bool) {
	p.calls++
	p.lastName = name
	p.lastMapIndex = mapIndex
	p.lastX = x
	p.lastY = y
	return p.preview, p.found
}

type stubCharacterTransferer struct {
	result       map[string]any
	found        bool
	calls        int
	lastName     string
	lastMapIndex uint32
	lastX        int32
	lastY        int32
}

func (t *stubCharacterTransferer) TransferCharacter(name string, mapIndex uint32, x int32, y int32) (any, bool) {
	t.calls++
	t.lastName = name
	t.lastMapIndex = mapIndex
	t.lastX = x
	t.lastY = y
	return t.result, t.found
}
