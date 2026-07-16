package ops

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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

func TestLocalContentBundleEndpointCanonicalizesExporterBundleForLoopbackGet(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "  VillageGuard  ",
			MapIndex:        42,
			X:               1700,
			Y:               2800,
			RaceNum:         20300,
			InteractionKind: " talk ",
			InteractionRef:  " npc:village_guard ",
		}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: " talk ", Ref: " npc:village_guard ", Text: " Keep your blade sharp. "}},
	}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.Bundle
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	want := contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected exported bundle to be canonicalized:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleEndpointReturnsServerErrorWhenExporterBundleIsInvalid(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{Name: "VillageGuard", RaceNum: 20300}},
	}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d for invalid exporter bundle, got %d", http.StatusInternalServerError, rec.Code)
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

func TestLocalContentBundleEndpointImportsSpawnGroupsForLoopbackPost(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK, bundle: contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{Ref: "practice.wolf_1", Name: "Practice Wolf", MapIndex: 3, X: 1200, Y: 2200, RaceNum: 101, CombatProfile: "practice_mob"}},
	}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"spawn_groups":[{"ref":"practice.wolf_1","name":"Practice Wolf","map_index":3,"x":1200,"y":2200,"race_num":101,"combat_profile":"practice_mob"}],"interaction_definitions":[]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if importer.calls != 1 || len(importer.lastBundle.SpawnGroups) != 1 || importer.lastBundle.SpawnGroups[0].Ref != "practice.wolf_1" {
		t.Fatalf("unexpected spawn-group importer call state: %+v", importer)
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

func TestLocalContentBundleEndpointRejectsDuplicateStaticActorsBeforeImport(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"static_actors":[{"name":"VillageGuard","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":"talk","interaction_ref":"npc:village_guard"},{"name":" VillageGuard ","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":" talk ","interaction_ref":" npc:village_guard "}],"interaction_definitions":[{"kind":"talk","ref":"npc:village_guard","text":"Keep your blade sharp."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected duplicate bundle to be rejected before importer call, got %d calls", importer.calls)
	}
}

func TestLocalContentBundleEndpointRejectsDuplicateCombatProfilesBeforeImport(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"spawn_groups":[{"ref":"practice.imported_wolf","name":"Imported Wolf","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_imported_wolf"}],"combat_profiles":[{"profile":"practice_imported_wolf","max_hp":24,"attack_value":8,"respawn_delay_ms":1500},{"profile":" practice_imported_wolf ","max_hp":24,"attack_value":8,"respawn_delay_ms":1500}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected duplicate combat profiles to be rejected before importer call, got %d calls", importer.calls)
	}
}

func TestLocalContentBundleEndpointRejectsDanglingRewardDropsBeforeImport(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200}],"spawn_groups":[{"ref":"practice.reward_mob","name":"Reward Mob","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27002]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected dangling reward-drop bundle to be rejected before importer call, got %d calls", importer.calls)
	}
}

func TestLocalContentBundleEndpointRejectsOversizedBodyBeforeImport(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	body := `{"interaction_definitions":[]}` + strings.Repeat(" ", 1<<20)
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d for oversized content bundle body, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected oversized bundle to be rejected before importer call, got %d calls", importer.calls)
	}
}

func TestLocalContentBundleValidateEndpointReturnsCanonicalBundleForLoopbackPost(t *testing.T) {
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", strings.NewReader(`{"static_actors":[{"name":"  VillageGuard  ","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":" talk ","interaction_ref":" npc:village_guard "}],"interaction_definitions":[{"kind":" talk ","ref":" npc:village_guard ","text":" Keep your blade sharp. "}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.Bundle
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	want := contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected canonical validation response:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleValidateEndpointAcceptsExampleBundle(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate ops contentbundle test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "examples", "bootstrap-npc-service-bundle.json"))
	if err != nil {
		t.Fatalf("read example content bundle: %v", err)
	}
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", strings.NewReader(string(raw)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for example bundle validation, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.Bundle
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode example validation response: %v", err)
	}
	if len(got.StaticActors) != 4 || len(got.SpawnGroups) != 1 || len(got.ItemTemplates) != 2 || len(got.InteractionDefinitions) != 4 {
		t.Fatalf("unexpected canonical example validation response: %+v", got)
	}
}

func TestLocalContentBundleValidateEndpointRejectsInvalidBundle(t *testing.T) {
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", strings.NewReader(`{"static_actors":[{"name":"VillageGuard","race_num":20300}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid bundle, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestLocalContentBundleValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback caller, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestLocalContentBundleValidateEndpointRejectsWrongMethod(t *testing.T) {
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/validate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method, got %d", http.StatusMethodNotAllowed, rec.Code)
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
