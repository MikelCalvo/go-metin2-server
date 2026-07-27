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
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
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
	if !strings.Contains(string(body), `"static_actors"`) || !strings.Contains(string(body), `"interaction_definitions"`) || !strings.Contains(string(body), `"ref": "npc:village_guard"`) {
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
	wantRaw, err := contentbundle.CanonicalJSON(want)
	if err != nil {
		t.Fatalf("canonicalize expected exporter response: %v", err)
	}
	if !reflect.DeepEqual(rec.Body.Bytes(), wantRaw) {
		t.Fatalf("expected exported bundle response to use canonical JSON\n--- got ---\n%s\n--- want ---\n%s", rec.Body.String(), string(wantRaw))
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
	wantRaw, err := contentbundle.CanonicalJSON(importer.bundle)
	if err != nil {
		t.Fatalf("canonicalize imported bundle response: %v", err)
	}
	if !reflect.DeepEqual(rec.Body.Bytes(), wantRaw) {
		t.Fatalf("expected import response to use canonical JSON\n--- got ---\n%s\n--- want ---\n%s", rec.Body.String(), string(wantRaw))
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
	importer := &stubContentBundleImporter{}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"static_actors":[`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid body, got %d", http.StatusBadRequest, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected invalid import body not to call importer, got %d calls", importer.calls)
	}
}

func TestLocalContentBundleEndpointsRejectNullRootBeforeCallbacks(t *testing.T) {
	t.Run("import", func(t *testing.T) {
		importer := &stubContentBundleImporter{}
		mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

		req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`null`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for null import body, got %d", http.StatusBadRequest, rec.Code)
		}
		if importer.calls != 0 {
			t.Fatalf("expected null import body not to call importer, got %d calls", importer.calls)
		}
	})

	t.Run("summary", func(t *testing.T) {
		summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
		mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

		req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(`null`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for null summary body, got %d", http.StatusBadRequest, rec.Code)
		}
		if summaryer.calls != 0 {
			t.Fatalf("expected null dry-run summary not to call live exporter, got %d calls", summaryer.calls)
		}
	})

	t.Run("import_preview", func(t *testing.T) {
		previewer := &stubContentBundleImportPreviewer{}
		mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

		req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`null`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for null import-preview body, got %d", http.StatusBadRequest, rec.Code)
		}
		if previewer.calls != 0 {
			t.Fatalf("expected null import preview not to call previewer, got %d calls", previewer.calls)
		}
	})

	t.Run("validate", func(t *testing.T) {
		mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))

		req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", strings.NewReader(`null`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for null validation body, got %d", http.StatusBadRequest, rec.Code)
		}
	})
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

func TestLocalContentBundleEndpointRejectsRewardDropsWithoutBundledItemTemplatesBeforeImport(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"spawn_groups":[{"ref":"practice.reward_mob","name":"Reward Mob","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected reward-drop bundle without item templates to be rejected before importer call, got %d calls", importer.calls)
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
	if !reflect.DeepEqual(rec.Body.Bytes(), raw) {
		t.Fatalf("expected example validation response to be byte-for-byte canonical\n--- got ---\n%s\n--- want ---\n%s", rec.Body.String(), string(raw))
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

func TestLocalContentBundleValidateEndpointRejectsConflictingRegisteredCombatProfileSnapshot(t *testing.T) {
	const profile = "practice_ops_conflict_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		AttackValue:           7,
		DefenseValue:          4,
		RespawnDelay:          1500 * time.Millisecond,
	}) {
		t.Fatalf("expected local combat profile %q to register", profile)
	}
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))

	body := `{"spawn_groups":[{"ref":"practice.ops_conflict_wolf","name":"Ops Conflict Wolf","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_ops_conflict_wolf"}],"combat_profiles":[{"profile":"practice_ops_conflict_wolf","max_hp":30,"damage_per_normal_attack":3,"attack_value":7,"defense_value":4,"respawn_delay_ms":1500}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for conflicting registered combat profile snapshot, got %d", http.StatusBadRequest, rec.Code)
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

func TestLocalContentBundleSummaryEndpointReturnsSummaryJSONForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{
		StaticActorCount:             2,
		InteractableStaticActorCount: 2,
		SpawnGroupCount:              1,
		CombatProfileCount:           1,
		ItemTemplateCount:            2,
		ShopCatalogEntryCount:        2,
		ShopCatalogs: []contentbundle.ShopCatalogSummary{{
			Kind:       interactionstore.KindShopPreview,
			Ref:        "npc:merchant",
			Title:      "Village Merchant",
			EntryCount: 2,
			Entries: []contentbundle.ShopCatalogEntrySummary{
				{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
				{Slot: 1, ItemVnum: 11200, ItemName: "Wooden Sword", Count: 1, Price: 500, Stackable: false, MaxCount: 1},
			},
		}},
		InteractionDefinitionCount:             3,
		ReferencedInteractionDefinitionCount:   2,
		UnreferencedInteractionDefinitionCount: 1,
		InteractionKinds: []contentbundle.InteractionKindSummary{
			{Kind: interactionstore.KindInfo, Count: 1, ReferencedCount: 0, UnreferencedCount: 1},
			{Kind: interactionstore.KindShopPreview, Count: 1, ReferencedCount: 1, UnreferencedCount: 0},
			{Kind: interactionstore.KindTalk, Count: 1, ReferencedCount: 1, UnreferencedCount: 0},
		},
		InteractionDefinitionPreviews: []contentbundle.InteractionDefinitionPreviewSummary{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused", Preview: "Unused lore."},
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Preview: "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"},
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Keep your blade sharp."},
		},
		ReferencedInteractionDefinitions: []contentbundle.InteractionDefinitionReferenceSummary{
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant"},
			{Kind: interactionstore.KindTalk, Ref: "npc:guide"},
		},
		UnreferencedInteractionDefinitions: []contentbundle.InteractionDefinitionReferenceSummary{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused"},
		},
		SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{
			{Ref: "practice.reward_mob", Name: "Reward Mob", MapIndex: 42, CombatProfile: "practice_mob", RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001}},
		},
		Maps: []contentbundle.MapContentSummary{
			{MapIndex: 1, StaticActorCount: 1, InteractableStaticActorCount: 1},
			{MapIndex: 42, StaticActorCount: 1, InteractableStaticActorCount: 1, SpawnGroupCount: 1},
		},
	}}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/summary", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode summary response body: %v", err)
	}
	if !reflect.DeepEqual(got, summaryer.summary) {
		t.Fatalf("unexpected summary response:\n got: %#v\nwant: %#v", got, summaryer.summary)
	}
}

func TestLocalContentBundleSummaryEndpointReturnsDryRunSummaryForLoopbackPost(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"static_actors":[{"name":"VillageGuide","map_index":1,"x":469350,"y":964200,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:qa_guide"},{"name":"Merchant","map_index":1,"x":469500,"y":964200,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:qa_merchant"},{"name":"Teleporter","map_index":1,"x":469650,"y":964200,"race_num":20300,"interaction_kind":"warp","interaction_ref":"npc:qa_teleporter"}],"spawn_groups":[{"ref":"practice.qa_reward_mob","name":"QARewardMob","map_index":1,"x":469800,"y":964200,"race_num":20350,"combat_profile":"practice_qa_profile"}],"combat_profiles":[{"profile":"practice_qa_profile","max_hp":24,"damage_per_normal_attack":3,"attack_value":7,"defense_value":4,"level":4,"rank":1,"respawn_delay_ms":1500,"death_reward":{"experience":75,"gold":60,"drop_vnums":[27001]}}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5}],"interaction_definitions":[{"kind":"talk","ref":"npc:qa_guide","text":"Welcome."},{"kind":"shop_preview","ref":"npc:qa_merchant","title":"QA Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1}]},{"kind":"info","ref":"lore:unused","text":"Unused lore."},{"kind":"warp","ref":"npc:qa_teleporter","text":"Step through the gate.","map_index":7,"x":1300,"y":2300}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected dry-run summary not to call live exporter, got %d calls", summaryer.calls)
	}
	var got contentbundle.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode dry-run summary response body: %v", err)
	}
	want := contentbundle.Summary{
		StaticActorCount:             3,
		InteractableStaticActorCount: 3,
		SpawnGroupCount:              1,
		CombatProfileCount:           1,
		ItemTemplateCount:            1,
		ShopCatalogEntryCount:        1,
		WarpDestinationCount:         1,
		RewardExperienceTotal:        75,
		RewardGoldTotal:              60,
		RewardDropItemCount:          1,
		RewardDrops: []contentbundle.RewardDropAggregateSummary{
			{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		StaticActors: []contentbundle.StaticActor{
			{Name: "Merchant", MapIndex: 1, X: 469500, Y: 964200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:qa_merchant"},
			{Name: "Teleporter", MapIndex: 1, X: 469650, Y: 964200, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:qa_teleporter"},
			{Name: "VillageGuide", MapIndex: 1, X: 469350, Y: 964200, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:qa_guide"},
		},
		ShopCatalogs: []contentbundle.ShopCatalogSummary{{
			Kind:       interactionstore.KindShopPreview,
			Ref:        "npc:qa_merchant",
			Title:      "QA Merchant",
			EntryCount: 1,
			Entries: []contentbundle.ShopCatalogEntrySummary{
				{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
			},
		}},
		ShopRouteCount:                         1,
		ShopRoutes:                             []contentbundle.ShopRouteSummary{{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 469500, SourceY: 964200, Ref: "npc:qa_merchant", Title: "QA Merchant", EntryCount: 1}},
		WarpDestinations:                       []contentbundle.WarpDestinationSummary{{Kind: interactionstore.KindWarp, Ref: "npc:qa_teleporter", Text: "Step through the gate.", MapIndex: 7, X: 1300, Y: 2300}},
		WarpRouteCount:                         1,
		WarpRoutes:                             []contentbundle.WarpRouteSummary{{ActorName: "Teleporter", SourceMapIndex: 1, SourceX: 469650, SourceY: 964200, Ref: "npc:qa_teleporter", Text: "Step through the gate.", TargetMapIndex: 7, TargetX: 1300, TargetY: 2300}},
		InteractionDefinitionCount:             4,
		ReferencedInteractionDefinitionCount:   3,
		UnreferencedInteractionDefinitionCount: 1,
		InteractionKinds: []contentbundle.InteractionKindSummary{
			{Kind: interactionstore.KindInfo, Count: 1, ReferencedCount: 0, UnreferencedCount: 1},
			{Kind: interactionstore.KindShopPreview, Count: 1, ReferencedCount: 1, UnreferencedCount: 0},
			{Kind: interactionstore.KindTalk, Count: 1, ReferencedCount: 1, UnreferencedCount: 0},
			{Kind: interactionstore.KindWarp, Count: 1, ReferencedCount: 1, UnreferencedCount: 0},
		},
		InteractionDefinitionPreviews: []contentbundle.InteractionDefinitionPreviewSummary{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused", Preview: "Unused lore."},
			{Kind: interactionstore.KindShopPreview, Ref: "npc:qa_merchant", Preview: "QA Merchant: [0] Small Red Potion x1 @ 50g"},
			{Kind: interactionstore.KindTalk, Ref: "npc:qa_guide", Preview: "Welcome."},
			{Kind: interactionstore.KindWarp, Ref: "npc:qa_teleporter", Preview: "Step through the gate. [warp -> map 7 @ 1300,2300]"},
		},
		ReferencedInteractionDefinitions: []contentbundle.InteractionDefinitionReferenceSummary{
			{Kind: interactionstore.KindShopPreview, Ref: "npc:qa_merchant"},
			{Kind: interactionstore.KindTalk, Ref: "npc:qa_guide"},
			{Kind: interactionstore.KindWarp, Ref: "npc:qa_teleporter"},
		},
		UnreferencedInteractionDefinitions: []contentbundle.InteractionDefinitionReferenceSummary{
			{Kind: interactionstore.KindInfo, Ref: "lore:unused"},
		},
		InteractableStaticActors: []contentbundle.InteractableStaticActorSummary{
			{Name: "Merchant", MapIndex: 1, X: 469500, Y: 964200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:qa_merchant", Preview: "QA Merchant: [0] Small Red Potion x1 @ 50g"},
			{Name: "Teleporter", MapIndex: 1, X: 469650, Y: 964200, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:qa_teleporter", Preview: "Step through the gate. [warp -> map 7 @ 1300,2300]"},
			{Name: "VillageGuide", MapIndex: 1, X: 469350, Y: 964200, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:qa_guide", Preview: "VillageGuide:\nWelcome."},
		},
		SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{
			{Ref: "practice.qa_reward_mob", Name: "QARewardMob", MapIndex: 1, X: 469800, Y: 964200, RaceNum: 20350, CombatProfile: "practice_qa_profile", RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{Profile: "practice_qa_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 4, Rank: 1, RespawnDelayMs: 1500, DeathReward: worldruntime.StaticActorDeathReward{Experience: 75, Gold: 60, DropVnums: []uint32{27001}}}},
		ItemTemplates:  []contentbundle.ItemTemplateReferenceSummary{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		Maps:           []contentbundle.MapContentSummary{{MapIndex: 1, StaticActorCount: 3, InteractableStaticActorCount: 3, TalkActorCount: 1, ShopPreviewActorCount: 1, ShopCatalogEntryCount: 1, WarpActorCount: 1, SpawnGroupCount: 1, RewardExperienceTotal: 75, RewardGoldTotal: 60, RewardDropItemCount: 1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected dry-run summary response:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleImportPreviewEndpointReturnsDeltaJSONForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{Name: "VillageGuide", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "Old notice."},
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
		},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"static_actors":[{"name":"  Merchant  ","map_index":42,"x":1800,"y":2900,"race_num":20302,"interaction_kind":" shop_preview ","interaction_ref":" npc:merchant "}],"item_templates":[{"vnum":27001,"name":" Small Red Potion ","stackable":true,"max_count":200,"shop_buy_price":5},{"vnum":11200,"name":" Wooden Sword ","stackable":false,"max_count":1}],"interaction_definitions":[{"kind":"info","ref":"lore:notice","text":"New notice."},{"kind":" shop_preview ","ref":" npc:merchant ","title":" Village Merchant ","catalog":[{"slot":1,"item_vnum":11200,"price":500,"count":1},{"slot":0,"item_vnum":27001,"price":50,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	wantCandidate := contentbundle.Bundle{
		StaticActors:  []contentbundle.StaticActor{{Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		ItemTemplates: testOpsMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "New notice."},
			testOpsMerchantCatalogDefinition(),
		},
	}
	if !reflect.DeepEqual(previewer.lastBundle, wantCandidate) {
		t.Fatalf("expected canonical candidate bundle passed to previewer:\n got: %#v\nwant: %#v", previewer.lastBundle, wantCandidate)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode import preview response: %v", err)
	}
	if got.Deltas.ShopCatalogEntryCount != (contentbundle.SummaryCountDelta{Current: 0, Candidate: 2, Delta: 2}) {
		t.Fatalf("unexpected shop catalog entry delta: %+v", got.Deltas.ShopCatalogEntryCount)
	}
	if got.Deltas.ShopRouteCount != (contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1}) {
		t.Fatalf("unexpected shop route delta: %+v", got.Deltas.ShopRouteCount)
	}
	wantShopCatalogs := []contentbundle.ShopCatalogDelta{{
		Kind:   interactionstore.KindShopPreview,
		Ref:    "npc:merchant",
		Change: "added",
		Candidate: &contentbundle.ShopCatalogSummary{
			Kind:       interactionstore.KindShopPreview,
			Ref:        "npc:merchant",
			Title:      "Village Merchant",
			EntryCount: 2,
			Entries: []contentbundle.ShopCatalogEntrySummary{
				{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
				{Slot: 1, ItemVnum: 11200, ItemName: "Wooden Sword", Count: 1, Price: 500, Stackable: false, MaxCount: 1},
			},
		},
	}}
	if !reflect.DeepEqual(got.Deltas.ShopCatalogs, wantShopCatalogs) {
		t.Fatalf("unexpected shop catalog import preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.ShopCatalogs, wantShopCatalogs)
	}
	wantStaticActors := []contentbundle.StaticActorDelta{
		{Change: "added", Candidate: &contentbundle.StaticActor{Name: "Merchant", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		{Change: "removed", Current: &contentbundle.StaticActor{Name: "VillageGuide", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
	}
	if !reflect.DeepEqual(got.Deltas.StaticActors, wantStaticActors) {
		t.Fatalf("unexpected static-actor import preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.StaticActors, wantStaticActors)
	}
	wantKinds := []contentbundle.InteractionKindDelta{
		{Kind: interactionstore.KindShopPreview, Count: contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1}, ReferencedCount: contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1}, UnreferencedCount: contentbundle.SummaryCountDelta{}},
		{Kind: interactionstore.KindTalk, Count: contentbundle.SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1}, ReferencedCount: contentbundle.SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1}, UnreferencedCount: contentbundle.SummaryCountDelta{}},
	}
	if !reflect.DeepEqual(got.Deltas.InteractionKinds, wantKinds) {
		t.Fatalf("unexpected interaction-kind import preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.InteractionKinds, wantKinds)
	}
	wantDefinitions := []contentbundle.InteractionDefinitionDelta{
		{Kind: interactionstore.KindInfo, Ref: "lore:notice", Change: "changed", CurrentPreview: "Old notice.", CandidatePreview: "New notice."},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Change: "added", CandidatePreview: "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g"},
		{Kind: interactionstore.KindTalk, Ref: "npc:guide", Change: "removed", CurrentPreview: "Welcome."},
	}
	if !reflect.DeepEqual(got.Deltas.InteractionDefinitions, wantDefinitions) {
		t.Fatalf("unexpected interaction-definition import preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.InteractionDefinitions, wantDefinitions)
	}
	wantItemTemplates := []contentbundle.ItemTemplateDelta{
		{Vnum: 11200, Change: "added", Candidate: &itemcatalog.Template{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1}},
		{Vnum: 27001, Change: "added", Candidate: &itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
	}
	if !reflect.DeepEqual(got.Deltas.ItemTemplates, wantItemTemplates) {
		t.Fatalf("unexpected item-template import preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.ItemTemplates, wantItemTemplates)
	}
}

func TestLocalContentBundleImportPreviewEndpointReturnsServiceRouteDeltaJSONForLoopbackPost(t *testing.T) {
	currentShop := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Old Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
		},
	}
	currentGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{
			{Name: "Merchant", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: currentShop.Ref},
			{Name: "Gate", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: currentGate.Ref},
		},
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		InteractionDefinitions: []interactionstore.Definition{
			currentShop,
			currentGate,
		},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	body := `{"static_actors":[{"name":"Merchant","map_index":1,"x":1000,"y":2000,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:merchant"},{"name":"Gate","map_index":1,"x":1100,"y":2100,"race_num":20300,"interaction_kind":"warp","interaction_ref":"npc:gate"},{"name":"RemoteMerchant","map_index":3,"x":3000,"y":4000,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:remote_merchant"}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5},{"vnum":11200,"name":"Wooden Sword","stackable":false,"max_count":1}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1},{"slot":1,"item_vnum":11200,"price":500,"count":1}]},{"kind":"shop_preview","ref":"npc:remote_merchant","title":"Remote Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":75,"count":1}]},{"kind":"warp","ref":"npc:gate","text":"New gate.","map_index":3,"x":2100,"y":3100}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode import preview service route response: %v", err)
	}
	currentMerchantRoute := contentbundle.ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Title: "Old Merchant", EntryCount: 1}
	candidateMerchantRoute := contentbundle.ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 2}
	candidateRemoteRoute := contentbundle.ShopRouteSummary{ActorName: "RemoteMerchant", SourceMapIndex: 3, SourceX: 3000, SourceY: 4000, Ref: "npc:remote_merchant", Title: "Remote Merchant", EntryCount: 1}
	wantShopRoutes := []contentbundle.ShopRouteDelta{
		{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Change: "changed", Current: &currentMerchantRoute, Candidate: &candidateMerchantRoute},
		{ActorName: "RemoteMerchant", SourceMapIndex: 3, SourceX: 3000, SourceY: 4000, Ref: "npc:remote_merchant", Change: "added", Candidate: &candidateRemoteRoute},
	}
	if !reflect.DeepEqual(got.Deltas.ShopRoutes, wantShopRoutes) {
		t.Fatalf("unexpected shop route import-preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.ShopRoutes, wantShopRoutes)
	}
	currentGateRoute := contentbundle.WarpRouteSummary{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Text: "Old gate.", TargetMapIndex: 2, TargetX: 2000, TargetY: 3000}
	candidateGateRoute := contentbundle.WarpRouteSummary{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Text: "New gate.", TargetMapIndex: 3, TargetX: 2100, TargetY: 3100}
	wantWarpRoutes := []contentbundle.WarpRouteDelta{{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Change: "changed", Current: &currentGateRoute, Candidate: &candidateGateRoute}}
	if !reflect.DeepEqual(got.Deltas.WarpRoutes, wantWarpRoutes) {
		t.Fatalf("unexpected warp route import-preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.WarpRoutes, wantWarpRoutes)
	}
}

func TestLocalContentBundleImportPreviewEndpointReturnsWarpDestinationDeltaJSONForLoopbackPost(t *testing.T) {
	currentGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		InteractionDefinitions: []interactionstore.Definition{
			currentGate,
			{Kind: interactionstore.KindWarp, Ref: "npc:old_gate", Text: "Old route.", MapIndex: 4, X: 2200, Y: 3200},
		},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"interaction_definitions":[{"kind":"warp","ref":"npc:gate","text":"New gate.","map_index":3,"x":2100,"y":3100},{"kind":"warp","ref":"npc:remote_gate","text":"Remote route.","map_index":9,"x":9000,"y":9100}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode import preview warp-destination response: %v", err)
	}
	currentGateDestination := contentbundle.WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}
	candidateGateDestination := contentbundle.WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "New gate.", MapIndex: 3, X: 2100, Y: 3100}
	currentOldGateDestination := contentbundle.WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:old_gate", Text: "Old route.", MapIndex: 4, X: 2200, Y: 3200}
	candidateRemoteGateDestination := contentbundle.WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:remote_gate", Text: "Remote route.", MapIndex: 9, X: 9000, Y: 9100}
	want := []contentbundle.WarpDestinationDelta{
		{Kind: interactionstore.KindWarp, Ref: "npc:gate", Change: "changed", Current: &currentGateDestination, Candidate: &candidateGateDestination},
		{Kind: interactionstore.KindWarp, Ref: "npc:old_gate", Change: "removed", Current: &currentOldGateDestination},
		{Kind: interactionstore.KindWarp, Ref: "npc:remote_gate", Change: "added", Candidate: &candidateRemoteGateDestination},
	}
	if !reflect.DeepEqual(got.Deltas.WarpDestinations, want) {
		t.Fatalf("unexpected warp destination import-preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.WarpDestinations, want)
	}
}

func TestLocalContentBundleImportPreviewEndpointReturnsPerMapDeltaJSONForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"static_actors":[{"name":"Merchant","map_index":1,"x":1200,"y":2200,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:merchant"},{"name":"Teleporter","map_index":7,"x":1300,"y":2300,"race_num":20303,"interaction_kind":"warp","interaction_ref":"npc:teleporter"}],"spawn_groups":[{"ref":"practice.reward_mob","name":"Reward Mob","map_index":7,"x":1400,"y":2400,"race_num":101,"combat_profile":"practice_mob","reward_experience":75,"reward_gold":60,"reward_drop_vnums":[27001]}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5},{"vnum":11200,"name":"Wooden Sword","stackable":false,"max_count":1}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1},{"slot":1,"item_vnum":11200,"price":500,"count":1}]},{"kind":"warp","ref":"npc:teleporter","text":"Step through the gate.","map_index":7,"x":1300,"y":2300}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode import preview per-map response: %v", err)
	}
	if got.Deltas.RewardExperienceTotal != (contentbundle.SummaryAmountDelta{Current: 0, Candidate: 75, Delta: 75}) {
		t.Fatalf("unexpected reward experience delta: %+v", got.Deltas.RewardExperienceTotal)
	}
	if got.Deltas.RewardGoldTotal != (contentbundle.SummaryAmountDelta{Current: 0, Candidate: 60, Delta: 60}) {
		t.Fatalf("unexpected reward gold delta: %+v", got.Deltas.RewardGoldTotal)
	}
	want := []contentbundle.MapContentDelta{
		{
			MapIndex:                     1,
			StaticActorCount:             contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
			InteractableStaticActorCount: contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
			TalkActorCount:               contentbundle.SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1},
			ShopPreviewActorCount:        contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			ShopCatalogEntryCount:        contentbundle.SummaryCountDelta{Current: 0, Candidate: 2, Delta: 2},
			StaticActors: []contentbundle.StaticActorDelta{
				{Change: "added", Candidate: &contentbundle.StaticActor{Name: "Merchant", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
				{Change: "removed", Current: &contentbundle.StaticActor{Name: "VillageGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
			},
		},
		{
			MapIndex:                     7,
			StaticActorCount:             contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			InteractableStaticActorCount: contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			WarpActorCount:               contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			SpawnGroupCount:              contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			RewardExperienceTotal:        contentbundle.SummaryAmountDelta{Current: 0, Candidate: 75, Delta: 75},
			RewardGoldTotal:              contentbundle.SummaryAmountDelta{Current: 0, Candidate: 60, Delta: 60},
			RewardDropItemCount:          contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
			StaticActors:                 []contentbundle.StaticActorDelta{{Change: "added", Candidate: &contentbundle.StaticActor{Name: "Teleporter", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20303, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:teleporter"}}},
			SpawnGroups:                  []contentbundle.SpawnGroupDelta{{Ref: "practice.reward_mob", Change: "added", Candidate: &contentbundle.SpawnGroupReferenceSummary{Ref: "practice.reward_mob", Name: "Reward Mob", MapIndex: 7, X: 1400, Y: 2400, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}}},
		},
	}
	if !reflect.DeepEqual(got.Deltas.Maps, want) {
		t.Fatalf("unexpected per-map import-preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.Maps, want)
	}
}

func TestLocalContentBundleImportPreviewEndpointReturnsRewardDropDeltaJSONForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
		},
		SpawnGroups: []contentbundle.SpawnGroup{
			{Ref: "practice.red", Name: "Red Drop Mob", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27001}},
			{Ref: "practice.blue", Name: "Blue Drop Mob", MapIndex: 42, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27002}},
		},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5},{"vnum":27003,"name":"Small Green Potion","stackable":true,"max_count":200,"shop_buy_price":9}],"spawn_groups":[{"ref":"practice.red","name":"Red Drop Mob","map_index":42,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]},{"ref":"practice.red_bonus","name":"Bonus Red Drop Mob","map_index":42,"x":1200,"y":2200,"race_num":103,"combat_profile":"practice_mob","reward_drop_vnums":[27001]},{"ref":"practice.green","name":"Green Drop Mob","map_index":42,"x":1300,"y":2300,"race_num":104,"combat_profile":"practice_mob","reward_drop_vnums":[27003]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode import preview reward-drop response: %v", err)
	}
	currentRed := contentbundle.RewardDropAggregateSummary{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	candidateRed := contentbundle.RewardDropAggregateSummary{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 2, Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	currentBlue := contentbundle.RewardDropAggregateSummary{ItemVnum: 27002, ItemName: "Small Blue Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 7}
	candidateGreen := contentbundle.RewardDropAggregateSummary{ItemVnum: 27003, ItemName: "Small Green Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 9}
	want := []contentbundle.RewardDropDelta{
		{ItemVnum: 27001, Change: "changed", Current: &currentRed, Candidate: &candidateRed},
		{ItemVnum: 27002, Change: "removed", Current: &currentBlue},
		{ItemVnum: 27003, Change: "added", Candidate: &candidateGreen},
	}
	if !reflect.DeepEqual(got.Deltas.RewardDrops, want) {
		t.Fatalf("unexpected reward-drop import-preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.RewardDrops, want)
	}
}

func TestLocalContentBundleImportPreviewEndpointReturnsSpawnGroupDeltaJSONForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		SpawnGroups: []contentbundle.SpawnGroup{
			{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 10, RewardGold: 5, RewardDropVnums: []uint32{27001}},
			{Ref: "practice.remove", Name: "Removed Mob", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 3, RewardGold: 1},
		},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5},{"vnum":27002,"name":"Small Blue Potion","stackable":true,"max_count":200,"shop_buy_price":7}],"spawn_groups":[{"ref":"practice.add","name":"Added Mob","map_index":2,"x":1300,"y":2300,"race_num":103,"combat_profile":"practice_mob","reward_experience":7,"reward_gold":2,"reward_drop_vnums":[27002]},{"ref":"practice.keep","name":"Keep Mob","map_index":1,"x":1200,"y":2200,"race_num":101,"combat_profile":"practice_mob","reward_experience":20,"reward_gold":8,"reward_drop_vnums":[27001]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode import preview spawn-group response: %v", err)
	}
	currentKeep := contentbundle.SpawnGroupReferenceSummary{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 10, RewardGold: 5, RewardDropVnums: []uint32{27001}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}
	currentRemoved := contentbundle.SpawnGroupReferenceSummary{Ref: "practice.remove", Name: "Removed Mob", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 3, RewardGold: 1}
	candidateAdded := contentbundle.SpawnGroupReferenceSummary{Ref: "practice.add", Name: "Added Mob", MapIndex: 2, X: 1300, Y: 2300, RaceNum: 103, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 7, RewardGold: 2, RewardDropVnums: []uint32{27002}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27002, ItemName: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7}}}
	candidateKeep := contentbundle.SpawnGroupReferenceSummary{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 20, RewardGold: 8, RewardDropVnums: []uint32{27001}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}
	want := []contentbundle.SpawnGroupDelta{
		{Ref: "practice.add", Change: "added", Candidate: &candidateAdded},
		{Ref: "practice.keep", Change: "changed", Current: &currentKeep, Candidate: &candidateKeep},
		{Ref: "practice.remove", Change: "removed", Current: &currentRemoved},
	}
	if !reflect.DeepEqual(got.Deltas.SpawnGroups, want) {
		t.Fatalf("unexpected spawn-group import-preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.SpawnGroups, want)
	}
}

func TestLocalContentBundleImportPreviewEndpointReturnsCombatProfileDeltaJSONForLoopbackPost(t *testing.T) {
	currentKeepProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_keep_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 2, Rank: 1, RespawnDelayMs: 1500}
	currentRemovedProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_remove_profile", MaxHP: 20, DamagePerNormalAttack: 2, AttackValue: 6, DefenseValue: 4, Level: 1, RespawnDelayMs: 1500}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{currentRemovedProfile, currentKeepProfile},
		SpawnGroups: []contentbundle.SpawnGroup{
			{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: currentKeepProfile.Profile},
			{Ref: "practice.remove", Name: "Removed Mob", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: currentRemovedProfile.Profile},
		},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"combat_profiles":[{"profile":"practice_keep_profile","max_hp":28,"damage_per_normal_attack":3,"attack_value":7,"defense_value":4,"level":2,"rank":1,"respawn_delay_ms":1500},{"profile":"practice_add_profile","max_hp":30,"damage_per_normal_attack":4,"attack_value":8,"defense_value":4,"level":3,"rank":1,"respawn_delay_ms":2000}],"spawn_groups":[{"ref":"practice.add","name":"Added Mob","map_index":2,"x":1300,"y":2300,"race_num":103,"combat_profile":"practice_add_profile"},{"ref":"practice.keep","name":"Keep Mob","map_index":1,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_keep_profile"}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode import preview combat-profile response: %v", err)
	}
	candidateAddedProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_add_profile", MaxHP: 30, DamagePerNormalAttack: 4, AttackValue: 8, DefenseValue: 4, Level: 3, Rank: 1, RespawnDelayMs: 2000}
	candidateKeepProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_keep_profile", MaxHP: 28, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 2, Rank: 1, RespawnDelayMs: 1500}
	want := []contentbundle.CombatProfileDelta{
		{Profile: "practice_add_profile", Change: "added", Candidate: &candidateAddedProfile},
		{Profile: "practice_keep_profile", Change: "changed", Current: &currentKeepProfile, Candidate: &candidateKeepProfile},
		{Profile: "practice_remove_profile", Change: "removed", Current: &currentRemovedProfile},
	}
	if !reflect.DeepEqual(got.Deltas.CombatProfiles, want) {
		t.Fatalf("unexpected combat-profile import-preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.CombatProfiles, want)
	}
}

func TestLocalContentBundleImportPreviewEndpointReturnsCombatProfileDeltaJSONForSpawnReferencedProfiles(t *testing.T) {
	currentAlpha := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_alpha_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 4, Rank: 1, RespawnDelayMs: 1500}
	currentBeta := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_beta_profile", MaxHP: 30, DamagePerNormalAttack: 5, AttackValue: 8, DefenseValue: 3, Level: 6, Rank: 2, RespawnDelayMs: 2500}
	candidateAlpha := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_alpha_profile", MaxHP: 24, DamagePerNormalAttack: 5, AttackValue: 9, DefenseValue: 4, Level: 4, Rank: 1, RespawnDelayMs: 1500}
	candidateGamma := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_gamma_profile", MaxHP: 40, DamagePerNormalAttack: 6, AttackValue: 10, DefenseValue: 4, Level: 7, Rank: 3, RespawnDelayMs: 3000}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{
			{Ref: "practice.alpha", Name: "Alpha Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: currentAlpha.Profile},
			{Ref: "practice.beta", Name: "Beta Mob", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: currentBeta.Profile},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{currentBeta, currentAlpha},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"spawn_groups":[{"ref":"practice.alpha","name":"Alpha Mob","map_index":1,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_alpha_profile"},{"ref":"practice.gamma","name":"Gamma Mob","map_index":2,"x":1200,"y":2200,"race_num":103,"combat_profile":"practice_gamma_profile"}],"combat_profiles":[{"profile":"practice_alpha_profile","max_hp":24,"damage_per_normal_attack":5,"attack_value":9,"defense_value":4,"level":4,"rank":1,"respawn_delay_ms":1500},{"profile":"practice_gamma_profile","max_hp":40,"damage_per_normal_attack":6,"attack_value":10,"defense_value":4,"level":7,"rank":3,"respawn_delay_ms":3000}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode import preview combat-profile response: %v", err)
	}
	want := []contentbundle.CombatProfileDelta{
		{Profile: "practice_alpha_profile", Change: "changed", Current: &currentAlpha, Candidate: &candidateAlpha},
		{Profile: "practice_beta_profile", Change: "removed", Current: &currentBeta},
		{Profile: "practice_gamma_profile", Change: "added", Candidate: &candidateGamma},
	}
	if !reflect.DeepEqual(got.Deltas.CombatProfiles, want) {
		t.Fatalf("unexpected combat-profile import-preview delta JSON:\n got: %#v\nwant: %#v", got.Deltas.CombatProfiles, want)
	}
}

func TestLocalContentBundleImportPreviewEndpointRejectsInvalidCandidateBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"static_actors":[{"name":"BrokenActor","race_num":20300}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid candidate import preview, got %d", http.StatusBadRequest, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected invalid candidate import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func testOpsMerchantCatalogDefinition() interactionstore.Definition {
	return interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
		},
	}
}

func testOpsMerchantItemTemplates() []itemcatalog.Template {
	return []itemcatalog.Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
	}
}

func TestLocalContentBundleSummaryEndpointReturnsPerMapInfoTalkActorCountsForLoopbackPost(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"static_actors":[{"name":"NoticeBoard","map_index":1,"x":469200,"y":964200,"race_num":20303,"interaction_kind":"info","interaction_ref":"lore:qa_square"},{"name":"VillageGuide","map_index":1,"x":469350,"y":964200,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:qa_guide"}],"interaction_definitions":[{"kind":"info","ref":"lore:qa_square","text":"Read the notices."},{"kind":"talk","ref":"npc:qa_guide","text":"Welcome."}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected dry-run summary not to call live exporter, got %d calls", summaryer.calls)
	}
	var got contentbundle.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode per-map info/talk summary response body: %v", err)
	}
	wantMaps := []contentbundle.MapContentSummary{{MapIndex: 1, StaticActorCount: 2, InteractableStaticActorCount: 2, InfoActorCount: 1, TalkActorCount: 1}}
	if !reflect.DeepEqual(got.Maps, wantMaps) {
		t.Fatalf("unexpected per-map info/talk summary counts:\n got: %#v\nwant: %#v", got.Maps, wantMaps)
	}
}

func TestLocalContentBundleSummaryEndpointRejectsInvalidDryRunBundle(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(`{"static_actors":[{"name":"BrokenActor","race_num":20300}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid dry-run bundle, got %d", http.StatusBadRequest, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected invalid dry-run summary not to call live exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSummaryEndpointRejectsOversizedDryRunBundle(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"interaction_definitions":[]}` + strings.Repeat(" ", 1<<20)
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d for oversized dry-run bundle, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected oversized dry-run summary not to call live exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSummaryEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/summary", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback caller, got %d", http.StatusForbidden, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSummaryEndpointRejectsDryRunNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback dry-run caller, got %d", http.StatusForbidden, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected dry-run summary not to call live exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSummaryEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodDelete, "/local/content-bundle/summary", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called, got %d calls", summaryer.calls)
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

type stubContentBundleSummaryExporter struct {
	summary contentbundle.Summary
	status  int
	calls   int
}

func (s *stubContentBundleSummaryExporter) ExportContentBundleSummary() (any, int) {
	s.calls++
	return s.summary, s.status
}

type stubContentBundleImportPreviewer struct {
	current    contentbundle.Bundle
	status     int
	calls      int
	lastBundle contentbundle.Bundle
}

func (s *stubContentBundleImportPreviewer) PreviewContentBundleImport(bundle contentbundle.Bundle) (any, int) {
	s.calls++
	s.lastBundle = bundle
	preview, err := contentbundle.BuildImportPreview(s.current, bundle)
	if err != nil {
		return nil, http.StatusBadRequest
	}
	if s.status != 0 {
		return preview, s.status
	}
	return preview, http.StatusOK
}
