package ops

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
)

func TestLocalContentBundleEndpointReturnsDeterministicJSONForLoopbackGet(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if exporter.calls != 1 {
		t.Fatalf("expected content bundle exporter to be called once, got %d calls", exporter.calls)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), `"static_actors"`) || !strings.Contains(string(body), `"interaction_definitions"`) || !strings.Contains(string(body), `"ref":"npc:village_guard"`) {
		t.Fatalf("unexpected JSON response body %q", string(body))
	}
}

func TestLocalContentBundleEndpointReturnsServerErrorWhenExporterFails(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusInternalServerError}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if exporter.calls != 1 {
		t.Fatalf("expected content bundle exporter to be called once on failure path, got %d calls", exporter.calls)
	}
}

func TestLocalContentBundleEndpointImportsBundleForLoopbackPost(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK, bundle: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"static_actors":[{"name":"VillageGuard","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":"talk","interaction_ref":"npc:village_guard"}],"interaction_definitions":[{"kind":"talk","ref":"npc:village_guard","text":"Keep your blade sharp."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if importer.calls != 1 || len(importer.lastBundle.StaticActors) != 1 || importer.lastBundle.StaticActors[0].Name != "VillageGuard" || len(importer.lastBundle.InteractionDefinitions) != 1 {
		t.Fatalf("unexpected content bundle importer call state: %+v", importer)
	}
}

func TestLocalContentBundleEndpointRejectsInvalidBody(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"static_actors":[{"name":"VillageGuard"}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected content bundle importer not to be called, got %d calls", importer.calls)
	}
}

type stubContentBundleExporter struct {
	bundle contentbundle.Bundle
	status int
	calls  int
}

func (s *stubContentBundleExporter) ExportContentBundle() (any, int) {
	s.calls++
	return s.bundle, s.status
}

type stubContentBundleImporter struct {
	bundle     contentbundle.Bundle
	status     int
	calls      int
	lastBundle contentbundle.Bundle
}

func (s *stubContentBundleImporter) ImportContentBundle(bundle contentbundle.Bundle) (any, int) {
	s.calls++
	s.lastBundle = bundle
	return s.bundle, s.status
}
