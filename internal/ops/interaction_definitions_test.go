package ops

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
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

func TestLocalInteractionDefinitionLookupEndpointReturnsDefinitionForLoopbackGet(t *testing.T) {
	lookup := &stubInteractionDefinitionLookup{status: http.StatusOK, definition: map[string]any{"kind": "talk", "ref": "npc:village_guard", "text": "Keep your blade sharp."}}
	mux := RegisterLocalInteractionDefinitionLookupEndpoint(NewPprofMux("gamed"), lookup.InteractionDefinition)

	req := httptest.NewRequest(http.MethodGet, "/local/interactions/talk/npc:village_guard", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if lookup.calls != 1 || lookup.lastKind != "talk" || lookup.lastRef != "npc:village_guard" {
		t.Fatalf("unexpected interaction definition lookup call state: %+v", lookup)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"talk"`) || !strings.Contains(string(body), `"ref":"npc:village_guard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInteractionDefinitionLookupEndpointReturnsNotFoundForMissingDefinition(t *testing.T) {
	lookup := &stubInteractionDefinitionLookup{status: http.StatusNotFound}
	mux := RegisterLocalInteractionDefinitionLookupEndpoint(NewPprofMux("gamed"), lookup.InteractionDefinition)

	req := httptest.NewRequest(http.MethodGet, "/local/interactions/info/lore:missing", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing definition, got %d", http.StatusNotFound, rec.Code)
	}
	if lookup.calls != 1 || lookup.lastKind != "info" || lookup.lastRef != "lore:missing" {
		t.Fatalf("unexpected missing-definition lookup call state: %+v", lookup)
	}
}

func TestLocalInteractionDefinitionLookupEndpointRejectsInvalidIdentityBeforeCallback(t *testing.T) {
	lookup := &stubInteractionDefinitionLookup{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionLookupEndpoint(NewPprofMux("gamed"), lookup.InteractionDefinition)

	for _, path := range []string{"/local/interactions/info", "/local/interactions/info/lore%2Falchemist", "/local/interactions/info/lore:alchemist/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid identity path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if lookup.calls != 0 {
		t.Fatalf("expected invalid lookup paths not to call lookup callback, got %d calls", lookup.calls)
	}
}

func TestLocalInteractionDefinitionLookupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	lookup := &stubInteractionDefinitionLookup{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionLookupEndpoint(NewPprofMux("gamed"), lookup.InteractionDefinition)

	req := httptest.NewRequest(http.MethodGet, "/local/interactions/info/lore:alchemist", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if lookup.calls != 0 {
		t.Fatalf("expected non-loopback lookup not to call callback, got %d calls", lookup.calls)
	}
}

func TestLocalInteractionDefinitionLookupEndpointCoexistsWithInteractionCollectionGet(t *testing.T) {
	snapshotter := &stubInteractionDefinitionSnapshotter{definitions: []map[string]any{{"kind": "info", "ref": "lore:alchemist", "text": "The alchemist studies forgotten herbs."}}}
	lookup := &stubInteractionDefinitionLookup{status: http.StatusOK, definition: map[string]any{"kind": "info", "ref": "lore:alchemist", "text": "The alchemist studies forgotten herbs."}}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), snapshotter.InteractionDefinitions, nil)
	mux = RegisterLocalInteractionDefinitionLookupEndpoint(mux, lookup.InteractionDefinition)

	collectionReq := httptest.NewRequest(http.MethodGet, "/local/interactions", nil)
	collectionReq.RemoteAddr = "127.0.0.1:12345"
	collectionRec := httptest.NewRecorder()
	mux.ServeHTTP(collectionRec, collectionReq)

	if collectionRec.Code != http.StatusOK {
		t.Fatalf("expected collection status %d, got %d", http.StatusOK, collectionRec.Code)
	}
	if snapshotter.calls != 1 || lookup.calls != 0 {
		t.Fatalf("expected collection GET to use snapshotter only, got snapshotter=%d lookup=%d", snapshotter.calls, lookup.calls)
	}

	lookupReq := httptest.NewRequest(http.MethodGet, "/local/interactions/info/lore:alchemist", nil)
	lookupReq.RemoteAddr = "127.0.0.1:12345"
	lookupRec := httptest.NewRecorder()
	mux.ServeHTTP(lookupRec, lookupReq)

	if lookupRec.Code != http.StatusOK {
		t.Fatalf("expected lookup status %d, got %d", http.StatusOK, lookupRec.Code)
	}
	if snapshotter.calls != 1 || lookup.calls != 1 {
		t.Fatalf("expected item GET to use lookup only after collection GET, got snapshotter=%d lookup=%d", snapshotter.calls, lookup.calls)
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
	if creator.calls != 1 || creator.lastDefinition.Kind != "info" || creator.lastDefinition.Ref != "lore:alchemist" || creator.lastDefinition.Text != "The alchemist studies forgotten herbs." {
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

func TestLocalInteractionDefinitionsEndpointRejectsOversizedCreateBody(t *testing.T) {
	creator := &stubInteractionDefinitionCreator{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), nil, creator.CreateInteractionDefinition)
	oversizedText := strings.Repeat("a", maxLocalInteractionDefinitionBodyBytes+1)

	req := httptest.NewRequest(http.MethodPost, "/local/interactions", strings.NewReader(`{"kind":"info","ref":"lore:oversized","text":"`+oversizedText+`"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if creator.calls != 0 {
		t.Fatalf("expected interaction definition creator not to be called for oversized body, got %d calls", creator.calls)
	}
}

func TestLocalInteractionDefinitionsEndpointRejectsPathAmbiguousRefBeforeCreate(t *testing.T) {
	creator := &stubInteractionDefinitionCreator{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), nil, creator.CreateInteractionDefinition)

	req := httptest.NewRequest(http.MethodPost, "/local/interactions", strings.NewReader(`{"kind":"info","ref":"lore/alchemist","text":"The alchemist studies forgotten herbs."}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if creator.calls != 0 {
		t.Fatalf("expected interaction definition creator not to be called for path-ambiguous ref, got %d calls", creator.calls)
	}
}

func TestLocalInteractionDefinitionsEndpointRejectsNULTextBeforeCreate(t *testing.T) {
	creator := &stubInteractionDefinitionCreator{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), nil, creator.CreateInteractionDefinition)

	req := httptest.NewRequest(http.MethodPost, "/local/interactions", strings.NewReader(`{"kind":"info","ref":"lore:alchemist","text":"visible\u0000hidden"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for NUL interaction text, got %d", http.StatusBadRequest, rec.Code)
	}
	if creator.calls != 0 {
		t.Fatalf("expected interaction definition creator not to be called for NUL text, got %d calls", creator.calls)
	}
}

func TestLocalInteractionDefinitionUpdateEndpointRejectsNULTitleBeforeUpdate(t *testing.T) {
	updater := &stubInteractionDefinitionUpdater{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionUpdateEndpoint(NewPprofMux("gamed"), updater.UpsertInteractionDefinition)

	req := httptest.NewRequest(http.MethodPut, "/local/interactions/shop_preview/npc:merchant", strings.NewReader(`{"kind":"shop_preview","ref":"npc:merchant","title":"Village\u0000Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for NUL interaction title, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected interaction definition updater not to be called for NUL title, got %d calls", updater.calls)
	}
}

func TestLocalInteractionDefinitionsEndpointRejectsInvalidUTF8BodyBeforeCreate(t *testing.T) {
	creator := &stubInteractionDefinitionCreator{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), nil, creator.CreateInteractionDefinition)
	body := []byte(`{"kind":"info","ref":"lore:invalid_utf8","text":"visible`)
	body = append(body, 0xff)
	body = append(body, []byte(`hidden"}`)...)

	req := httptest.NewRequest(http.MethodPost, "/local/interactions", strings.NewReader(string(body)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid UTF-8 interaction definition body, got %d", http.StatusBadRequest, rec.Code)
	}
	if creator.calls != 0 {
		t.Fatalf("expected interaction definition creator not to be called for invalid UTF-8 body, got %d calls", creator.calls)
	}
}

func TestLocalInteractionDefinitionUpdateEndpointRejectsInvalidUTF8BodyBeforeUpdate(t *testing.T) {
	updater := &stubInteractionDefinitionUpdater{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionUpdateEndpoint(NewPprofMux("gamed"), updater.UpsertInteractionDefinition)
	body := []byte(`{"kind":"info","ref":"lore:invalid_utf8","text":"visible`)
	body = append(body, 0xff)
	body = append(body, []byte(`hidden"}`)...)

	req := httptest.NewRequest(http.MethodPut, "/local/interactions/info/lore:invalid_utf8", strings.NewReader(string(body)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid UTF-8 interaction definition body, got %d", http.StatusBadRequest, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected interaction definition updater not to be called for invalid UTF-8 body, got %d calls", updater.calls)
	}
}

func TestLocalInteractionDefinitionUpdateEndpointRejectsOversizedBody(t *testing.T) {
	updater := &stubInteractionDefinitionUpdater{status: http.StatusOK}
	mux := RegisterLocalInteractionDefinitionUpdateEndpoint(NewPprofMux("gamed"), updater.UpsertInteractionDefinition)
	oversizedText := strings.Repeat("a", maxLocalInteractionDefinitionBodyBytes+1)

	req := httptest.NewRequest(http.MethodPut, "/local/interactions/info/lore:oversized", strings.NewReader(`{"kind":"info","ref":"lore:oversized","text":"`+oversizedText+`"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if updater.calls != 0 {
		t.Fatalf("expected interaction definition updater not to be called for oversized body, got %d calls", updater.calls)
	}
}

func TestLocalInteractionDefinitionsEndpointCreatesWarpDefinitionForLoopbackPost(t *testing.T) {
	creator := &stubInteractionDefinitionCreator{status: http.StatusOK, definition: map[string]any{"kind": "warp", "ref": "npc:teleporter", "map_index": float64(42), "x": float64(1700), "y": float64(2800), "text": "Step through the gate."}}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), nil, creator.CreateInteractionDefinition)

	req := httptest.NewRequest(http.MethodPost, "/local/interactions", strings.NewReader(`{"kind":"warp","ref":"npc:teleporter","map_index":42,"x":1700,"y":2800,"text":"Step through the gate."}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if creator.calls != 1 || creator.lastDefinition.Kind != interactionstore.KindWarp || creator.lastDefinition.Ref != "npc:teleporter" || creator.lastDefinition.MapIndex != 42 || creator.lastDefinition.X != 1700 || creator.lastDefinition.Y != 2800 || creator.lastDefinition.Text != "Step through the gate." {
		t.Fatalf("unexpected warp interaction definition creator call state: %+v", creator)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"warp"`) || !strings.Contains(string(body), `"map_index":42`) || !strings.Contains(string(body), `"x":1700`) || !strings.Contains(string(body), `"y":2800`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInteractionDefinitionsEndpointCreatesShopPreviewDefinitionForLoopbackPost(t *testing.T) {
	creator := &stubInteractionDefinitionCreator{status: http.StatusOK, definition: map[string]any{"kind": "shop_preview", "ref": "npc:merchant", "title": "Village Merchant", "catalog": []map[string]any{{"slot": float64(0), "item_vnum": float64(27001), "price": float64(50), "count": float64(1)}, {"slot": float64(1), "item_vnum": float64(11200), "price": float64(500), "count": float64(1)}}}}
	mux := RegisterLocalInteractionDefinitionEndpoints(NewPprofMux("gamed"), nil, creator.CreateInteractionDefinition)

	req := httptest.NewRequest(http.MethodPost, "/local/interactions", strings.NewReader(`{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1},{"slot":1,"item_vnum":11200,"price":500,"count":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if creator.calls != 1 || creator.lastDefinition.Kind != interactionstore.KindShopPreview || creator.lastDefinition.Ref != "npc:merchant" || creator.lastDefinition.Title != "Village Merchant" || len(creator.lastDefinition.Catalog) != 2 || creator.lastDefinition.Catalog[0].Slot != 0 || creator.lastDefinition.Catalog[0].ItemVnum != 27001 || creator.lastDefinition.Catalog[0].Price != 50 || creator.lastDefinition.Catalog[0].Count != 1 || creator.lastDefinition.Catalog[1].Slot != 1 || creator.lastDefinition.Catalog[1].ItemVnum != 11200 || creator.lastDefinition.Catalog[1].Price != 500 || creator.lastDefinition.Catalog[1].Count != 1 {
		t.Fatalf("unexpected shop preview interaction definition creator call state: %+v", creator)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"shop_preview"`) || !strings.Contains(string(body), `"ref":"npc:merchant"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
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
	if updater.calls != 1 || updater.lastDefinition.Kind != "talk" || updater.lastDefinition.Ref != "npc:village_guard" || updater.lastDefinition.Text != "Keep your blade sharp." {
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

func TestLocalInteractionDefinitionUpdateEndpointUpsertsWarpDefinitionForLoopbackPut(t *testing.T) {
	updater := &stubInteractionDefinitionUpdater{status: http.StatusOK, definition: map[string]any{"kind": "warp", "ref": "npc:teleporter", "map_index": float64(42), "x": float64(1700), "y": float64(2800), "text": "Step through the gate."}}
	mux := RegisterLocalInteractionDefinitionUpdateEndpoint(NewPprofMux("gamed"), updater.UpsertInteractionDefinition)

	req := httptest.NewRequest(http.MethodPut, "/local/interactions/warp/npc:teleporter", strings.NewReader(`{"kind":"warp","ref":"npc:teleporter","map_index":42,"x":1700,"y":2800,"text":"Step through the gate."}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if updater.calls != 1 || updater.lastDefinition.Kind != interactionstore.KindWarp || updater.lastDefinition.Ref != "npc:teleporter" || updater.lastDefinition.MapIndex != 42 || updater.lastDefinition.X != 1700 || updater.lastDefinition.Y != 2800 || updater.lastDefinition.Text != "Step through the gate." {
		t.Fatalf("unexpected warp interaction definition updater call state: %+v", updater)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"warp"`) || !strings.Contains(string(body), `"map_index":42`) || !strings.Contains(string(body), `"x":1700`) || !strings.Contains(string(body), `"y":2800`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalInteractionDefinitionUpdateEndpointUpsertsShopPreviewDefinitionForLoopbackPut(t *testing.T) {
	updater := &stubInteractionDefinitionUpdater{status: http.StatusOK, definition: map[string]any{"kind": "shop_preview", "ref": "npc:merchant", "title": "Village Merchant", "catalog": []map[string]any{{"slot": float64(0), "item_vnum": float64(27001), "price": float64(50), "count": float64(1)}, {"slot": float64(1), "item_vnum": float64(11200), "price": float64(500), "count": float64(1)}}}}
	mux := RegisterLocalInteractionDefinitionUpdateEndpoint(NewPprofMux("gamed"), updater.UpsertInteractionDefinition)

	req := httptest.NewRequest(http.MethodPut, "/local/interactions/shop_preview/npc:merchant", strings.NewReader(`{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1},{"slot":1,"item_vnum":11200,"price":500,"count":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if updater.calls != 1 || updater.lastDefinition.Kind != interactionstore.KindShopPreview || updater.lastDefinition.Ref != "npc:merchant" || updater.lastDefinition.Title != "Village Merchant" || len(updater.lastDefinition.Catalog) != 2 || updater.lastDefinition.Catalog[0].Slot != 0 || updater.lastDefinition.Catalog[0].ItemVnum != 27001 || updater.lastDefinition.Catalog[0].Price != 50 || updater.lastDefinition.Catalog[0].Count != 1 || updater.lastDefinition.Catalog[1].Slot != 1 || updater.lastDefinition.Catalog[1].ItemVnum != 11200 || updater.lastDefinition.Catalog[1].Price != 500 || updater.lastDefinition.Catalog[1].Count != 1 {
		t.Fatalf("unexpected shop preview interaction definition updater call state: %+v", updater)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"shop_preview"`) || !strings.Contains(string(body), `"ref":"npc:merchant"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
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
	definition     map[string]any
	status         int
	calls          int
	lastDefinition interactionstore.Definition
}

func (s *stubInteractionDefinitionCreator) CreateInteractionDefinition(definition interactionstore.Definition) (any, int) {
	s.calls++
	s.lastDefinition = definition
	return s.definition, s.status
}

type stubInteractionDefinitionLookup struct {
	definition map[string]any
	status     int
	calls      int
	lastKind   string
	lastRef    string
}

func (s *stubInteractionDefinitionLookup) InteractionDefinition(kind string, ref string) (any, int) {
	s.calls++
	s.lastKind = kind
	s.lastRef = ref
	return s.definition, s.status
}

type stubInteractionDefinitionUpdater struct {
	definition     map[string]any
	status         int
	calls          int
	lastDefinition interactionstore.Definition
}

func (s *stubInteractionDefinitionUpdater) UpsertInteractionDefinition(definition interactionstore.Definition) (any, int) {
	s.calls++
	s.lastDefinition = definition
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
