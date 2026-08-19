package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
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

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"static_actors":[{"name":"VillageGuard","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":"talk","interaction_ref":"npc:village_guard"}],"quest_state":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":1}],"interaction_definitions":[{"kind":"talk","ref":"npc:village_guard","text":"Keep your blade sharp."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if importer.calls != 1 || len(importer.lastBundle.StaticActors) != 1 || importer.lastBundle.StaticActors[0].Name != "VillageGuard" || len(importer.lastBundle.QuestState) != 1 || importer.lastBundle.QuestState[0].Character != "QuestHero" || len(importer.lastBundle.InteractionDefinitions) != 1 {
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

func TestLocalContentBundleEndpointRejectsInvalidUTF8BodyBeforeImport(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	body := []byte(`{"static_actors":[{"name":"Visible`)
	body = append(body, 0xff)
	body = append(body, []byte(`Hidden","map_index":42,"x":1700,"y":2800,"race_num":20300}]}`)...)
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid UTF-8 body, got %d", http.StatusBadRequest, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected invalid UTF-8 body not to call importer, got %d calls", importer.calls)
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

func TestLocalContentBundleEndpointRejectsNullCollectionFieldsBeforeImport(t *testing.T) {
	for _, field := range []string{"static_actors", "spawn_groups", "regen_spawns", "drop_tables", "combat_profiles", "item_templates", "quest_state", "interaction_definitions"} {
		t.Run(field, func(t *testing.T) {
			importer := &stubContentBundleImporter{status: http.StatusOK}
			mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

			req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(fmt.Sprintf(`{"%s":null}`, field)))
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d for null %s, got %d", http.StatusBadRequest, field, rec.Code)
			}
			if importer.calls != 0 {
				t.Fatalf("expected null %s bundle not to call importer, got %d calls", field, importer.calls)
			}
		})
	}
}

func TestLocalContentBundleImportPreviewEndpointRejectsNullCollectionFieldBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{status: http.StatusOK}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(`{"interaction_definitions":null}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for null interaction_definitions, got %d", http.StatusBadRequest, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected null import preview body not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleDryRunEndpointsRejectNullCollectionFieldsBeforeCallbacks(t *testing.T) {
	t.Run("summary", func(t *testing.T) {
		summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
		mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

		req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(`{"interaction_definitions":null}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for null interaction_definitions, got %d", http.StatusBadRequest, rec.Code)
		}
		if summaryer.calls != 0 {
			t.Fatalf("expected dry-run summary not to call live exporter, got %d calls", summaryer.calls)
		}
	})

	t.Run("validate", func(t *testing.T) {
		mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))

		req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", strings.NewReader(`{"interaction_definitions":null}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for null interaction_definitions, got %d", http.StatusBadRequest, rec.Code)
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

func TestLocalContentBundleEndpointRejectsUnsupportedInteractionKindBeforeImport(t *testing.T) {
	importer := &stubContentBundleImporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), nil, importer.ImportContentBundle)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle", strings.NewReader(`{"static_actors":[{"name":"QuestBoard","map_index":42,"x":1700,"y":2800,"race_num":20300,"interaction_kind":"quest","interaction_ref":"quest:first_steps"}],"interaction_definitions":[{"kind":"info","ref":"quest:first_steps","text":"Quest text should not make an unsupported actor kind importable."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for unsupported interaction kind, got %d", http.StatusBadRequest, rec.Code)
	}
	if importer.calls != 0 {
		t.Fatalf("expected unsupported interaction kind bundle to be rejected before importer call, got %d calls", importer.calls)
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
	if len(got.StaticActors) != 7 || len(got.SpawnGroups) != 1 || len(got.ItemTemplates) != 2 || len(got.QuestState) != 1 || len(got.InteractionDefinitions) != 7 {
		t.Fatalf("unexpected canonical example validation response: %+v", got)
	}
	wantSpawn := contentbundle.SpawnGroup{
		Ref:              "practice.qa_reward_mob",
		Name:             "QARewardMob",
		MapIndex:         1,
		X:                469800,
		Y:                964200,
		RaceNum:          20350,
		CombatProfile:    "practice_mob",
		RewardExperience: 75,
		RewardGold:       60,
		RewardDropVnums:  []uint32{27001},
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		RequireQuestRef:  "quest:first_steps",
		RequireQuestFlag: "met_guide",
		RequireQuestFrom: 1,
	}
	if !reflect.DeepEqual(got.SpawnGroups[0], wantSpawn) {
		t.Fatalf("unexpected canonical example spawn group:\n got: %#v\nwant: %#v", got.SpawnGroups[0], wantSpawn)
	}
	wantQuestState := []queststate.Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}
	if !reflect.DeepEqual(got.QuestState, wantQuestState) {
		t.Fatalf("unexpected canonical example quest-state rows:\n got: %#v\nwant: %#v", got.QuestState, wantQuestState)
	}
	if !reflect.DeepEqual(rec.Body.Bytes(), raw) {
		t.Fatalf("expected example validation response to be byte-for-byte canonical\n--- got ---\n%s\n--- want ---\n%s", rec.Body.String(), string(raw))
	}
}

func TestLocalContentBundleValidateEndpointExpandsDropTableAuthoringExample(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate ops contentbundle test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "examples", "bootstrap-drop-table-authoring-bundle.json"))
	if err != nil {
		t.Fatalf("read drop-table authoring example bundle: %v", err)
	}
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", bytes.NewReader(raw))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for drop-table authoring example validation, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"drop_tables"`)) || bytes.Contains(rec.Body.Bytes(), []byte(`"reward_drop_table_ref"`)) {
		t.Fatalf("expected validation response to strip authoring-only drop-table fields, got %s", rec.Body.String())
	}
	var got contentbundle.Bundle
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode drop-table validation response: %v", err)
	}
	wantDrops := []uint32{27001, 27002}
	if len(got.SpawnGroups) != 1 ||
		got.SpawnGroups[0].RewardExperience != 75 ||
		got.SpawnGroups[0].RewardGold != 60 ||
		!reflect.DeepEqual(got.SpawnGroups[0].RewardDropVnums, wantDrops) ||
		got.SpawnGroups[0].RewardQuestRef != "quest:first_steps" ||
		got.SpawnGroups[0].RewardQuestFlag != "killed_qa_mob" ||
		got.SpawnGroups[0].RewardQuestTo != 1 ||
		got.SpawnGroups[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." ||
		got.SpawnGroups[0].RequireQuestRef != "quest:first_steps" ||
		got.SpawnGroups[0].RequireQuestFlag != "met_guide" ||
		got.SpawnGroups[0].RequireQuestFrom != 1 {
		t.Fatalf("expected validation response to expand fixed reward table plus kill-quest require gate into spawn-group descriptor, got %+v", got.SpawnGroups)
	}
}

func TestLocalContentBundleValidateEndpointExpandsRegenAuthoringExample(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate ops contentbundle test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "examples", "bootstrap-regen-authoring-bundle.json"))
	if err != nil {
		t.Fatalf("read regen authoring example bundle: %v", err)
	}
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", bytes.NewReader(raw))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for regen authoring example validation, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"regen_spawns"`)) || bytes.Contains(rec.Body.Bytes(), []byte(`"drop_tables"`)) || bytes.Contains(rec.Body.Bytes(), []byte(`"reward_drop_table_ref"`)) {
		t.Fatalf("expected validation response to strip authoring-only regen/drop-table fields, got %s", rec.Body.String())
	}
	var got contentbundle.Bundle
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode regen validation response: %v", err)
	}
	wantDrops := []uint32{27001, 27002}
	if len(got.SpawnGroups) != 1 ||
		got.SpawnGroups[0].Ref != "practice.qa_regen_mob" ||
		got.SpawnGroups[0].RewardExperience != 90 ||
		got.SpawnGroups[0].RewardGold != 45 ||
		!reflect.DeepEqual(got.SpawnGroups[0].RewardDropVnums, wantDrops) ||
		got.SpawnGroups[0].RewardQuestRef != "quest:first_steps" ||
		got.SpawnGroups[0].RewardQuestFlag != "killed_qa_mob" ||
		got.SpawnGroups[0].RewardQuestTo != 1 ||
		got.SpawnGroups[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." ||
		got.SpawnGroups[0].RequireQuestRef != "quest:first_steps" ||
		got.SpawnGroups[0].RequireQuestFlag != "met_guide" ||
		got.SpawnGroups[0].RequireQuestFrom != 1 {
		t.Fatalf("expected validation response to expand regen authoring plus kill-quest require gate into spawn-group descriptor, got %+v", got.SpawnGroups)
	}
}

func TestLocalContentBundleValidateEndpointExpandsCombatProfileFormulaExample(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate ops contentbundle test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "examples", "bootstrap-combat-profile-formula-bundle.json"))
	if err != nil {
		t.Fatalf("read combat-profile formula example bundle: %v", err)
	}
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", bytes.NewReader(raw))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for combat-profile formula example validation, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var got contentbundle.Bundle
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode combat-profile formula validation response: %v", err)
	}
	if len(got.CombatProfiles) != 1 || got.CombatProfiles[0].Profile != "qa_formula_practice_mob" || got.CombatProfiles[0].DamagePerNormalAttack != 5 || got.CombatProfiles[0].AttackValue != 9 || got.CombatProfiles[0].DefenseValue != 4 || got.CombatProfiles[0].Level != 1 || got.CombatProfiles[0].MaxHP != 20 {
		t.Fatalf("expected validation response to derive formula damage/default level, got %+v", got.CombatProfiles)
	}
	wantDrops := []uint32{27001}
	if len(got.SpawnGroups) != 1 || got.SpawnGroups[0].Ref != "practice.qa_formula_mob" || got.SpawnGroups[0].CombatProfile != "qa_formula_practice_mob" || got.SpawnGroups[0].RewardExperience != 40 || got.SpawnGroups[0].RewardGold != 25 || !reflect.DeepEqual(got.SpawnGroups[0].RewardDropVnums, wantDrops) {
		t.Fatalf("expected validation response to copy profile-default rewards onto the formula spawn group, got %+v", got.SpawnGroups)
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

func TestLocalContentBundleValidateEndpointRejectsCombatProfileDuplicateRewardDrops(t *testing.T) {
	mux := RegisterLocalContentBundleValidateEndpoint(NewPprofMux("gamed"))

	body := `{"spawn_groups":[{"ref":"practice.ops_duplicate_reward_wolf","name":"Ops Duplicate Reward Wolf","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_ops_duplicate_reward_wolf"}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200},{"vnum":27002,"name":"Small Blue Potion","stackable":true,"max_count":200}],"combat_profiles":[{"profile":"practice_ops_duplicate_reward_wolf","max_hp":24,"attack_value":8,"defense_value":3,"respawn_delay_ms":1500,"death_reward":{"drop_vnums":[27002,27001,27002]}}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/validate", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for duplicate combat-profile reward drops, got %d", http.StatusBadRequest, rec.Code)
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

func TestLocalContentBundleSummaryEndpointReturnsQuestFlagTriggerAndRouteJSONForLoopbackPost(t *testing.T) {
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), func() (any, int) {
		t.Fatal("dry-run quest-flag summary should not call live exporter")
		return nil, http.StatusInternalServerError
	})

	body := `{"static_actors":[{"name":"QuestGuide","map_index":1,"x":469350,"y":964200,"race_num":20302,"interaction_kind":"quest_flag","interaction_ref":"quest:first_steps"}],"interaction_definitions":[{"kind":"quest_flag","ref":"quest:first_steps","text":"Quest updated: first_steps.met_guide = 1.","quest_ref":"quest:first_steps","quest_flag":"met_guide","quest_to":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest-flag summary response: %v", err)
	}
	wantTrigger := []contentbundle.QuestFlagTriggerSummary{{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated: first_steps.met_guide = 1.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}}
	if got.QuestFlagTriggerCount != 1 || !reflect.DeepEqual(got.QuestFlagTriggers, wantTrigger) {
		t.Fatalf("unexpected quest-flag trigger summary:\n got count=%d rows=%#v\nwant count=1 rows=%#v", got.QuestFlagTriggerCount, got.QuestFlagTriggers, wantTrigger)
	}
	wantRoute := []contentbundle.QuestFlagRouteSummary{{ActorName: "QuestGuide", SourceMapIndex: 1, SourceX: 469350, SourceY: 964200, Ref: "quest:first_steps", Text: "Quest updated: first_steps.met_guide = 1.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}}
	if got.QuestFlagRouteCount != 1 || !reflect.DeepEqual(got.QuestFlagRoutes, wantRoute) {
		t.Fatalf("unexpected quest-flag route summary:\n got count=%d rows=%#v\nwant count=1 rows=%#v", got.QuestFlagRouteCount, got.QuestFlagRoutes, wantRoute)
	}
}

func TestLocalContentBundleQuestFlagTriggerEndpointReturnsExactTriggerForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{
		QuestFlagTriggers: []contentbundle.QuestFlagTriggerSummary{
			{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1},
			{Kind: interactionstore.KindQuestFlag, Ref: "quest:daily_check", Text: "Daily updated.", QuestRef: "quest:daily_check", QuestFlag: "talked_to_guide", QuestTo: 1},
		},
	}}
	mux := RegisterLocalContentBundleQuestFlagTriggerEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-flag-triggers/quest_flag/quest:first_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected summary exporter to be called once, got %d", summaryer.calls)
	}
	var got contentbundle.QuestFlagTriggerSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact quest-flag trigger response: %v", err)
	}
	want := contentbundle.QuestFlagTriggerSummary{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact quest-flag trigger response:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestFlagTriggerEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestFlagTriggerEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-flag-triggers/quest_flag/quest:first_steps", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback quest-flag trigger lookup, got %d", http.StatusForbidden, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call summary exporter, got %d", summaryer.calls)
	}
}

func TestLocalContentBundleSummaryEndpointExposesSelectedCharacterGuardMetadataForLoopbackPost(t *testing.T) {
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), func() (any, int) {
		t.Fatal("dry-run summary should not call live exporter")
		return nil, http.StatusInternalServerError
	})

	body := `{"spawn_groups":[{"ref":"practice.restricted_reward","name":"Restricted Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}],"item_templates":[{"vnum":27001,"name":"Restricted Potion","stackable":true,"max_count":200,"shop_buy_price":5,"shop_sell_price":2,"anti_warrior":true,"anti_empire_a":true,"min_level":25,"buy_reject_message":"The merchant will not sell this restricted potion to you.","sell_reject_message":"The merchant refuses this restricted potion."}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:restricted_merchant","title":"Restricted Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":2}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode selected-character guard summary response: %v", err)
	}
	if len(got.ItemTemplates) != 1 || !got.ItemTemplates[0].AntiWarrior || !got.ItemTemplates[0].AntiEmpireA || got.ItemTemplates[0].MinLevel != 25 {
		t.Fatalf("expected selected-character guards on item template summary, got %+v", got.ItemTemplates)
	}
	if len(got.ShopCatalogs) != 1 || len(got.ShopCatalogs[0].Entries) != 1 || !got.ShopCatalogs[0].Entries[0].AntiWarrior || !got.ShopCatalogs[0].Entries[0].AntiEmpireA || got.ShopCatalogs[0].Entries[0].MinLevel != 25 {
		t.Fatalf("expected selected-character guards on shop catalog summary, got %+v", got.ShopCatalogs)
	}
	if len(got.SpawnGroups) != 1 || len(got.SpawnGroups[0].RewardDropItems) != 1 || !got.SpawnGroups[0].RewardDropItems[0].AntiWarrior || !got.SpawnGroups[0].RewardDropItems[0].AntiEmpireA || got.SpawnGroups[0].RewardDropItems[0].MinLevel != 25 {
		t.Fatalf("expected selected-character guards on reward item summary, got %+v", got.SpawnGroups)
	}
	if len(got.RewardDrops) != 1 || !got.RewardDrops[0].AntiWarrior || !got.RewardDrops[0].AntiEmpireA || got.RewardDrops[0].MinLevel != 25 {
		t.Fatalf("expected selected-character guards on aggregate reward drop summary, got %+v", got.RewardDrops)
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

func TestLocalContentBundleImportPreviewEndpointReturnsQuestFlagTriggerAndRouteDeltaJSONForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{Name: "QuestGuide", MapIndex: 1, X: 469350, Y: 964200, RaceNum: 20302, InteractionKind: interactionstore.KindQuestFlag, InteractionRef: "quest:first_steps"}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:      interactionstore.KindQuestFlag,
			Ref:       "quest:first_steps",
			Text:      "Old quest acknowledgement.",
			QuestRef:  "quest:first_steps",
			QuestFlag: "met_guide",
			QuestTo:   1,
		}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	body := `{"static_actors":[{"name":"QuestGuide","map_index":1,"x":469350,"y":964200,"race_num":20302,"interaction_kind":"quest_flag","interaction_ref":"quest:first_steps"},{"name":"QuestResetGuide","map_index":1,"x":469375,"y":964200,"race_num":20302,"interaction_kind":"quest_flag","interaction_ref":"quest:first_steps_reset"}],"interaction_definitions":[{"kind":"quest_flag","ref":"quest:first_steps","text":"New quest acknowledgement.","quest_ref":"quest:first_steps","quest_flag":"met_guide","quest_to":1},{"kind":"quest_flag","ref":"quest:first_steps_reset","text":"Quest cleared.","quest_ref":"quest:first_steps","quest_flag":"met_guide","quest_from":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest-flag import preview response: %v", err)
	}
	if got.Deltas.QuestFlagTriggerCount != (contentbundle.SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1}) || got.Deltas.QuestFlagRouteCount != (contentbundle.SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1}) {
		t.Fatalf("unexpected quest-flag trigger/route count deltas: %+v", got.Deltas)
	}
	currentTrigger := contentbundle.QuestFlagTriggerSummary{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Old quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	candidateTrigger := contentbundle.QuestFlagTriggerSummary{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "New quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	candidateResetTrigger := contentbundle.QuestFlagTriggerSummary{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps_reset", Text: "Quest cleared.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1}
	wantTriggers := []contentbundle.QuestFlagTriggerDelta{
		{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Change: "changed", Current: &currentTrigger, Candidate: &candidateTrigger},
		{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps_reset", Change: "added", Candidate: &candidateResetTrigger},
	}
	if !reflect.DeepEqual(got.Deltas.QuestFlagTriggers, wantTriggers) {
		t.Fatalf("unexpected quest-flag trigger deltas:\n got: %#v\nwant: %#v", got.Deltas.QuestFlagTriggers, wantTriggers)
	}
	currentRoute := contentbundle.QuestFlagRouteSummary{ActorName: "QuestGuide", SourceMapIndex: 1, SourceX: 469350, SourceY: 964200, Ref: "quest:first_steps", Text: "Old quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	candidateRoute := contentbundle.QuestFlagRouteSummary{ActorName: "QuestGuide", SourceMapIndex: 1, SourceX: 469350, SourceY: 964200, Ref: "quest:first_steps", Text: "New quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	candidateResetRoute := contentbundle.QuestFlagRouteSummary{ActorName: "QuestResetGuide", SourceMapIndex: 1, SourceX: 469375, SourceY: 964200, Ref: "quest:first_steps_reset", Text: "Quest cleared.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1}
	wantRoutes := []contentbundle.QuestFlagRouteDelta{
		{ActorName: "QuestGuide", SourceMapIndex: 1, SourceX: 469350, SourceY: 964200, Ref: "quest:first_steps", Change: "changed", Current: &currentRoute, Candidate: &candidateRoute},
		{ActorName: "QuestResetGuide", SourceMapIndex: 1, SourceX: 469375, SourceY: 964200, Ref: "quest:first_steps_reset", Change: "added", Candidate: &candidateResetRoute},
	}
	if !reflect.DeepEqual(got.Deltas.QuestFlagRoutes, wantRoutes) {
		t.Fatalf("unexpected quest-flag route deltas:\n got: %#v\nwant: %#v", got.Deltas.QuestFlagRoutes, wantRoutes)
	}
	if len(got.Deltas.Maps) != 1 || !reflect.DeepEqual(got.Deltas.Maps[0].QuestFlagRoutes, wantRoutes) {
		t.Fatalf("unexpected map-local quest-flag route deltas: %+v", got.Deltas.Maps)
	}
}

func TestLocalContentBundleInteractionKindImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "Read the notice board."}},
	}}
	mux := RegisterLocalContentBundleInteractionKindImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interaction-kinds/info", strings.NewReader(`{"static_actors":[{"name":"NoticeBoard","map_index":1,"x":900,"y":1900,"race_num":20304,"interaction_kind":"info","interaction_ref":"lore:notice"}],"interaction_definitions":[{"kind":"info","ref":"lore:notice","text":"Read the notice board."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.InteractionKindDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact interaction-kind import-preview delta response: %v", err)
	}
	want := contentbundle.InteractionKindDelta{
		Kind:              interactionstore.KindInfo,
		Count:             contentbundle.SummaryCountDelta{Current: 1, Candidate: 1},
		ReferencedCount:   contentbundle.SummaryCountDelta{Current: 0, Candidate: 1, Delta: 1},
		UnreferencedCount: contentbundle.SummaryCountDelta{Current: 1, Candidate: 0, Delta: -1},
	}
	if got != want {
		t.Fatalf("unexpected exact interaction-kind import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleInteractionKindImportPreviewEndpointReturnsNotFoundWhenKindDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "NoticeBoard", MapIndex: 1, X: 900, Y: 1900, RaceNum: 20304, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:notice"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "Read the notice board."}},
	}}
	mux := RegisterLocalContentBundleInteractionKindImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interaction-kinds/info", strings.NewReader(`{"static_actors":[{"name":"NoticeBoard","map_index":1,"x":900,"y":1900,"race_num":20304,"interaction_kind":"info","interaction_ref":"lore:notice"}],"interaction_definitions":[{"kind":"info","ref":"lore:notice","text":"Read the notice board."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged interaction-kind delta, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected unchanged interaction-kind lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractionKindImportPreviewEndpointRejectsInvalidKindBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractionKindImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/interaction-kinds/",
		"/local/content-bundle/import-preview/interaction-kinds/quest",
		"/local/content-bundle/import-preview/interaction-kinds/talk/extra",
		"/local/content-bundle/import-preview/interaction-kinds/bad%2Fkind",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"interaction_definitions":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed interaction-kind import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed interaction kind not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractionKindImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractionKindImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interaction-kinds/info", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback interaction-kind import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractionKindImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractionKindImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/interaction-kinds/info", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method interaction-kind import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractionKindImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "Old notice."}}}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleInteractionKindImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interaction-kinds/info", strings.NewReader(`{"static_actors":[{"name":"NoticeBoard","map_index":1,"x":900,"y":1900,"race_num":20304,"interaction_kind":"info","interaction_ref":"lore:notice"}],"interaction_definitions":[{"kind":"info","ref":"lore:notice","text":"Old notice."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.InteractionKindDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused interaction-kind import-preview delta response from shared mux: %v", err)
	}
	if got.Kind != interactionstore.KindInfo || got.ReferencedCount.Delta != 1 || got.UnreferencedCount.Delta != -1 {
		t.Fatalf("unexpected focused interaction-kind import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleQuestStateImportPreviewEndpointReturnsCompactOverviewAndDeltasForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{QuestState: []queststate.Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "old_flag", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}}
	mux := RegisterLocalContentBundleQuestStateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state", strings.NewReader(`{"quest_state":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":2},{"character":"AnotherHero","quest_ref":"quest:first_steps","name":"met_guard","value":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.QuestStateImportPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode compact quest-state import-preview response: %v", err)
	}
	currentOldFlag := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "old_flag", Value: 1}
	currentStep := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "step", Value: 1}
	candidateMetGuard := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}
	candidateStep := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "step", Value: 2}
	want := contentbundle.QuestStateImportPreview{
		Current: contentbundle.QuestStateOverview{FlagCount: 2, CharacterCount: 1, QuestCount: 1, QuestRefs: []string{"quest:first_steps"}},
		Candidate: contentbundle.QuestStateOverview{
			FlagCount:      2,
			CharacterCount: 2,
			QuestCount:     1,
			QuestRefs:      []string{"quest:first_steps"},
			Characters: []contentbundle.QuestStateCharacterSummary{
				{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{candidateMetGuard}},
				{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{candidateStep}},
			},
			Quests: []contentbundle.QuestStateQuestSummary{{
				QuestRef:  "quest:first_steps",
				FlagCount: 2,
				Characters: []contentbundle.QuestStateCharacterSummary{
					{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{candidateMetGuard}},
					{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{candidateStep}},
				},
			}},
		},
		Deltas: contentbundle.QuestStateImportPreviewDeltas{
			FlagCount:      contentbundle.SummaryCountDelta{Current: 2, Candidate: 2, Delta: 0},
			CharacterCount: contentbundle.SummaryCountDelta{Current: 1, Candidate: 2, Delta: 1},
			QuestCount:     contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
			Flags: []contentbundle.QuestStateDelta{
				{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Change: "added", Candidate: &candidateMetGuard},
				{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "old_flag", Change: "removed", Current: &currentOldFlag},
				{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Change: "changed", Current: &currentStep, Candidate: &candidateStep},
			},
		},
	}
	want.Current.Characters = []contentbundle.QuestStateCharacterSummary{{Character: "QuestHero", FlagCount: 2, Flags: []queststate.FlagSnapshot{currentOldFlag, currentStep}}}
	want.Current.Quests = []contentbundle.QuestStateQuestSummary{{QuestRef: "quest:first_steps", FlagCount: 2, Characters: []contentbundle.QuestStateCharacterSummary{{Character: "QuestHero", FlagCount: 2, Flags: []queststate.FlagSnapshot{currentOldFlag, currentStep}}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected compact quest-state import-preview response:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestStateImportPreviewEndpointReturnsNotFoundWhenNoQuestStateDeltas(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{QuestState: []queststate.Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}}
	mux := RegisterLocalContentBundleQuestStateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state", strings.NewReader(`{"quest_state":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged quest-state import preview, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected unchanged compact quest-state preview to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestStateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state", strings.NewReader(`{"quest_state":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback compact quest-state import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback compact quest-state request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestStateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong-method compact quest-state import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method compact quest-state request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{QuestState: []queststate.Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "old_flag", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}}
	mux := RegisterLocalContentBundleQuestStateFlagImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	body := `{"quest_state":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":2},{"character":"AnotherHero","quest_ref":"quest:first_steps","name":"met_guard","value":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state/flags/QuestHero/quest:first_steps/step", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	wantCandidate := contentbundle.Bundle{QuestState: []queststate.Flag{
		{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2},
	}}
	if !reflect.DeepEqual(previewer.lastBundle, wantCandidate) {
		t.Fatalf("expected canonical candidate bundle passed to previewer:\n got: %#v\nwant: %#v", previewer.lastBundle, wantCandidate)
	}
	var got contentbundle.QuestStateDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact quest-state import-preview delta response: %v", err)
	}
	current := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "step", Value: 1}
	candidate := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "step", Value: 2}
	want := contentbundle.QuestStateDelta{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Change: "changed", Current: &current, Candidate: &candidate}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact quest-state import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestStateCharacterImportPreviewEndpointReturnsCharacterDeltasForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{QuestState: []queststate.Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "old_flag", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
		{Character: "OtherHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}}
	mux := RegisterLocalContentBundleQuestStateCharacterImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	body := `{"quest_state":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":2},{"character":"OtherHero","quest_ref":"quest:first_steps","name":"step","value":3}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state/characters/QuestHero", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got []contentbundle.QuestStateDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode character quest-state import-preview delta response: %v", err)
	}
	oldFlag := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "old_flag", Value: 1}
	currentStep := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "step", Value: 1}
	candidateStep := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "step", Value: 2}
	want := []contentbundle.QuestStateDelta{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "old_flag", Change: "removed", Current: &oldFlag},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Change: "changed", Current: &currentStep, Candidate: &candidateStep},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected character quest-state import-preview deltas:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestStateQuestImportPreviewEndpointReturnsQuestDeltasForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{QuestState: []queststate.Flag{
		{Character: "QuestHero", QuestRef: "quest:daily_check", Name: "talked_to_guide", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}}
	mux := RegisterLocalContentBundleQuestStateQuestImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	body := `{"quest_state":[{"character":"QuestHero","quest_ref":"quest:daily_check","name":"talked_to_guide","value":2},{"character":"AnotherHero","quest_ref":"quest:first_steps","name":"met_guard","value":1},{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":2}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state/quests/quest:first_steps", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got []contentbundle.QuestStateDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest quest-state import-preview delta response: %v", err)
	}
	metGuard := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}
	currentStep := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "step", Value: 1}
	candidateStep := queststate.FlagSnapshot{QuestRef: "quest:first_steps", Name: "step", Value: 2}
	want := []contentbundle.QuestStateDelta{
		{Character: "AnotherHero", QuestRef: "quest:first_steps", Name: "met_guard", Change: "added", Candidate: &metGuard},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Change: "changed", Current: &currentStep, Candidate: &candidateStep},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest quest-state import-preview deltas:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestStateCharacterImportPreviewEndpointReturnsNotFoundWhenCharacterHasNoDeltas(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{QuestState: []queststate.Flag{{Character: "OtherHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}}
	mux := RegisterLocalContentBundleQuestStateCharacterImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state/characters/QuestHero", strings.NewReader(`{"quest_state":[{"character":"OtherHero","quest_ref":"quest:first_steps","name":"step","value":2}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged character quest-state import preview, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing character delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateQuestImportPreviewEndpointReturnsNotFoundWhenQuestHasNoDeltas(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{QuestState: []queststate.Flag{{Character: "QuestHero", QuestRef: "quest:daily_check", Name: "talked_to_guide", Value: 1}}}}
	mux := RegisterLocalContentBundleQuestStateQuestImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state/quests/quest:first_steps", strings.NewReader(`{"quest_state":[{"character":"QuestHero","quest_ref":"quest:daily_check","name":"talked_to_guide","value":2}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged quest quest-state import preview, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing quest delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateCharacterImportPreviewEndpointRejectsMalformedIdentityBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestStateCharacterImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{"/local/content-bundle/import-preview/quest-state/characters/", "/local/content-bundle/import-preview/quest-state/characters/Bad%2FName", "/local/content-bundle/import-preview/quest-state/characters/Bad-Name", "/local/content-bundle/import-preview/quest-state/characters/QuestHero/extra"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"quest_state":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed character quest-state import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed character identities not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateQuestImportPreviewEndpointRejectsMalformedIdentityBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestStateQuestImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{"/local/content-bundle/import-preview/quest-state/quests/", "/local/content-bundle/import-preview/quest-state/quests/first_steps", "/local/content-bundle/import-preview/quest-state/quests/quest%2Ffirst_steps", "/local/content-bundle/import-preview/quest-state/quests/quest:first_steps/extra"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"quest_state":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed quest quest-state import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed quest identities not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateGroupedImportPreviewEndpointsRejectNonLoopbackRemoteAddr(t *testing.T) {
	tests := []struct {
		name     string
		register func(*http.ServeMux, func(contentbundle.Bundle) (any, int)) *http.ServeMux
		path     string
	}{
		{name: "character", register: RegisterLocalContentBundleQuestStateCharacterImportPreviewEndpoint, path: "/local/content-bundle/import-preview/quest-state/characters/QuestHero"},
		{name: "quest", register: RegisterLocalContentBundleQuestStateQuestImportPreviewEndpoint, path: "/local/content-bundle/import-preview/quest-state/quests/quest:first_steps"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			previewer := &stubContentBundleImportPreviewer{}
			mux := tc.register(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"quest_state":[]}`))
			req.RemoteAddr = "203.0.113.10:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected status %d for non-loopback %s quest-state import preview, got %d", http.StatusForbidden, tc.name, rec.Code)
			}
			if previewer.calls != 0 {
				t.Fatalf("expected non-loopback %s request not to call import previewer, got %d calls", tc.name, previewer.calls)
			}
		})
	}
}

func TestLocalContentBundleQuestStateGroupedImportPreviewEndpointsRejectWrongMethod(t *testing.T) {
	tests := []struct {
		name     string
		register func(*http.ServeMux, func(contentbundle.Bundle) (any, int)) *http.ServeMux
		path     string
	}{
		{name: "character", register: RegisterLocalContentBundleQuestStateCharacterImportPreviewEndpoint, path: "/local/content-bundle/import-preview/quest-state/characters/QuestHero"},
		{name: "quest", register: RegisterLocalContentBundleQuestStateQuestImportPreviewEndpoint, path: "/local/content-bundle/import-preview/quest-state/quests/quest:first_steps"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			previewer := &stubContentBundleImportPreviewer{}
			mux := tc.register(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status %d for wrong method %s quest-state import preview, got %d", http.StatusMethodNotAllowed, tc.name, rec.Code)
			}
			if previewer.calls != 0 {
				t.Fatalf("expected wrong method %s request not to call import previewer, got %d calls", tc.name, previewer.calls)
			}
		})
	}
}

func TestLocalContentBundleQuestStateFlagImportPreviewEndpointReturnsNotFoundWhenExactFlagDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{QuestState: []queststate.Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}}
	mux := RegisterLocalContentBundleQuestStateFlagImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state/flags/QuestHero/quest:first_steps/step", strings.NewReader(`{"quest_state":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged exact quest-state flag, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected unchanged flag lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagImportPreviewEndpointRejectsMalformedIdentityBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestStateFlagImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state/flags/QuestHero/quest:first_steps/bad-flag", strings.NewReader(`{"quest_state":[]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for malformed exact quest-state flag identity, got %d", http.StatusBadRequest, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed identity not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestStateFlagImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-state/flags/QuestHero/quest:first_steps/step", strings.NewReader(`{"quest_state":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback exact quest-state import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestStateFlagImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method exact quest-state import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong method not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractionDefinitionImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Old text."},
	}}}
	mux := RegisterLocalContentBundleInteractionDefinitionImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interaction-definitions/talk/npc:guide", strings.NewReader(`{"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"New text."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.InteractionDefinitionDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact interaction-definition import-preview delta response: %v", err)
	}
	want := contentbundle.InteractionDefinitionDelta{Kind: interactionstore.KindTalk, Ref: "npc:guide", Change: "changed", CurrentPreview: "Old text.", CandidatePreview: "New text."}
	if got != want {
		t.Fatalf("unexpected exact interaction-definition import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleInteractionDefinitionImportPreviewEndpointReturnsNotFoundWhenDefinitionDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Same text."},
	}}}
	mux := RegisterLocalContentBundleInteractionDefinitionImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interaction-definitions/talk/npc:guide", strings.NewReader(`{"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"Same text."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged interaction-definition delta, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestLocalContentBundleInteractionDefinitionImportPreviewEndpointRejectsInvalidIdentityBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractionDefinitionImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/interaction-definitions/quest/quest:first_steps",
		"/local/content-bundle/import-preview/interaction-definitions/talk/npc%2Fguide",
		"/local/content-bundle/import-preview/interaction-definitions/talk/npc:guide/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"interaction_definitions":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed interaction-definition import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed identity not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractionDefinitionImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractionDefinitionImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interaction-definitions/talk/npc:guide", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback interaction-definition import-preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractionDefinitionImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractionDefinitionImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/interaction-definitions/talk/npc:guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method interaction-definition import-preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractionDefinitionImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Old text."},
	}}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleInteractionDefinitionImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interaction-definitions/talk/npc:guide", strings.NewReader(`{"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"New text."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.InteractionDefinitionDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused interaction-definition import-preview delta response from shared mux: %v", err)
	}
	if got.Kind != interactionstore.KindTalk || got.Ref != "npc:guide" || got.Change != "changed" {
		t.Fatalf("unexpected focused interaction-definition import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleStaticActorImportPreviewEndpointReturnsNameDeltasForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
			{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "Notice."},
		},
	}}
	mux := RegisterLocalContentBundleStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/static-actors/Village%20Guide", strings.NewReader(`{"static_actors":[{"name":"Village Guide","map_index":2,"x":1100,"y":2100,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:guide"},{"name":"Remote Notice","map_index":7,"x":1300,"y":2300,"race_num":20304,"interaction_kind":"info","interaction_ref":"lore:notice"}],"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"Welcome."},{"kind":"info","ref":"lore:notice","text":"Notice."}]}`))
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
		StaticActors: []contentbundle.StaticActor{
			{Name: "Remote Notice", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20304, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:notice"},
			{Name: "Village Guide", MapIndex: 2, X: 1100, Y: 2100, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindInfo, Ref: "lore:notice", Text: "Notice."},
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."},
		},
	}
	if !reflect.DeepEqual(previewer.lastBundle, wantCandidate) {
		t.Fatalf("expected canonical candidate bundle passed to previewer:\n got: %#v\nwant: %#v", previewer.lastBundle, wantCandidate)
	}
	var got []contentbundle.StaticActorDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode static-actor import-preview delta response: %v", err)
	}
	current := contentbundle.StaticActor{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}
	candidate := contentbundle.StaticActor{Name: "Village Guide", MapIndex: 2, X: 1100, Y: 2100, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}
	want := []contentbundle.StaticActorDelta{
		{Change: "removed", Current: &current},
		{Change: "added", Candidate: &candidate},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected static-actor import-preview deltas:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleStaticActorImportPreviewEndpointReturnsNotFoundWhenNameDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
	}}
	mux := RegisterLocalContentBundleStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/static-actors/Village%20Guide", strings.NewReader(`{"static_actors":[{"name":"Village Guide","map_index":1,"x":1000,"y":2000,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:guide"}],"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"Welcome."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged static-actor name, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected unchanged static-actor lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleStaticActorImportPreviewEndpointRejectsInvalidNameBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/static-actors/",
		"/local/content-bundle/import-preview/static-actors/Bad%2FGuide",
		"/local/content-bundle/import-preview/static-actors/Village%20Guide/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"static_actors":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed static-actor import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed static-actor identity not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleStaticActorImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/static-actors/Village%20Guide", strings.NewReader(`{"static_actors":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback static-actor import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback static-actor import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleStaticActorImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/static-actors/Village%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method static-actor import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method static-actor import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleStaticActorImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{StaticActors: []contentbundle.StaticActor{{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302}}}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleStaticActorImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/static-actors/Village%20Guide", strings.NewReader(`{"static_actors":[{"name":"Village Guide","map_index":2,"x":1100,"y":2100,"race_num":20302}],"interaction_definitions":[]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused static-actor route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got []contentbundle.StaticActorDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused static-actor import-preview delta response from shared mux: %v", err)
	}
	if len(got) != 2 || got[0].Change != "removed" || got[1].Change != "added" {
		t.Fatalf("unexpected focused static-actor import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleInteractableStaticActorImportPreviewEndpointReturnsNameDeltasForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Old greeting."},
		},
	}}
	mux := RegisterLocalContentBundleInteractableStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interactable-static-actors/Village%20Guide", strings.NewReader(`{"static_actors":[{"name":"Village Guide","map_index":1,"x":1000,"y":2000,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:guide"},{"name":"Remote Merchant","map_index":7,"x":1300,"y":2300,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:merchant"}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200},{"vnum":11200,"name":"Wooden Sword","stackable":false,"max_count":1}],"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"New greeting."},{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1},{"slot":1,"item_vnum":11200,"price":500,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got []contentbundle.InteractableStaticActorDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode interactable static-actor import-preview delta response: %v", err)
	}
	current := contentbundle.InteractableStaticActorSummary{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide", Preview: "Village Guide:\nOld greeting."}
	candidate := contentbundle.InteractableStaticActorSummary{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide", Preview: "Village Guide:\nNew greeting."}
	want := []contentbundle.InteractableStaticActorDelta{{Change: "changed", Current: &current, Candidate: &candidate}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected interactable static-actor import-preview deltas:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleInteractableStaticActorImportPreviewEndpointReturnsNotFoundWhenNameDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
	}}
	mux := RegisterLocalContentBundleInteractableStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interactable-static-actors/Village%20Guide", strings.NewReader(`{"static_actors":[{"name":"Village Guide","map_index":1,"x":1000,"y":2000,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:guide"}],"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"Welcome."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged interactable static-actor name, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected unchanged interactable static-actor lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractableStaticActorImportPreviewEndpointRejectsInvalidNameBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractableStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/interactable-static-actors/",
		"/local/content-bundle/import-preview/interactable-static-actors/Bad%2FGuide",
		"/local/content-bundle/import-preview/interactable-static-actors/Village%20Guide/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"static_actors":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed interactable static-actor import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed interactable static-actor identity not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractableStaticActorImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractableStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interactable-static-actors/Village%20Guide", strings.NewReader(`{"static_actors":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback interactable static-actor import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback interactable static-actor import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractableStaticActorImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleInteractableStaticActorImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/interactable-static-actors/Village%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method interactable static-actor import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method interactable static-actor import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleInteractableStaticActorImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Old text."}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleInteractableStaticActorImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/interactable-static-actors/Village%20Guide", strings.NewReader(`{"static_actors":[{"name":"Village Guide","map_index":1,"x":1000,"y":2000,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:guide"}],"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"New text."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused interactable static-actor route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got []contentbundle.InteractableStaticActorDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused interactable static-actor import-preview delta response from shared mux: %v", err)
	}
	if len(got) != 1 || got[0].Change != "changed" || got[0].Current == nil || got[0].Candidate == nil {
		t.Fatalf("unexpected focused interactable static-actor import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleShopCatalogImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Old Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			},
		}},
	}}
	mux := RegisterLocalContentBundleShopCatalogImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/shop-catalogs/shop_preview/npc:merchant", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":75,"count":2}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.ShopCatalogDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact shop-catalog import-preview delta response: %v", err)
	}
	current := contentbundle.ShopCatalogSummary{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Old Merchant", EntryCount: 1, Entries: []contentbundle.ShopCatalogEntrySummary{{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200}}}
	candidate := contentbundle.ShopCatalogSummary{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 1, Entries: []contentbundle.ShopCatalogEntrySummary{{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 2, Price: 75, Stackable: true, MaxCount: 200}}}
	want := contentbundle.ShopCatalogDelta{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Change: "changed", Current: &current, Candidate: &candidate}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact shop-catalog import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleShopCatalogImportPreviewEndpointReturnsNotFoundWhenCatalogDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates:          []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}},
	}}
	mux := RegisterLocalContentBundleShopCatalogImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/shop-catalogs/shop_preview/npc:merchant", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged shop catalog delta, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestLocalContentBundleShopCatalogImportPreviewEndpointRejectsInvalidIdentityBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleShopCatalogImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/shop-catalogs/shop_preview",
		"/local/content-bundle/import-preview/shop-catalogs/quest/npc:first_steps",
		"/local/content-bundle/import-preview/shop-catalogs/shop_preview/npc%2Fmerchant",
		"/local/content-bundle/import-preview/shop-catalogs/shop_preview/npc:merchant/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"interaction_definitions":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed shop-catalog import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed identity not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleShopCatalogImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleShopCatalogImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/shop-catalogs/shop_preview/npc:merchant", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback shop-catalog import-preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleShopCatalogImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleShopCatalogImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/shop-catalogs/shop_preview/npc:merchant", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method shop-catalog import-preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleShopCatalogImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates:          []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Old Merchant", Catalog: []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleShopCatalogImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/shop-catalogs/shop_preview/npc:merchant", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":75,"count":2}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ShopCatalogDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused shop-catalog import-preview delta response from shared mux: %v", err)
	}
	if got.Kind != interactionstore.KindShopPreview || got.Ref != "npc:merchant" || got.Change != "changed" {
		t.Fatalf("unexpected focused shop-catalog import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleWarpDestinationImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{
		{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000},
	}}}
	mux := RegisterLocalContentBundleWarpDestinationImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/warp-destinations/warp/npc:gate", strings.NewReader(`{"interaction_definitions":[{"kind":"warp","ref":"npc:gate","text":"New gate.","map_index":42,"x":1700,"y":2800}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.WarpDestinationDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact warp-destination import-preview delta response: %v", err)
	}
	current := contentbundle.WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}
	candidate := contentbundle.WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "New gate.", MapIndex: 42, X: 1700, Y: 2800}
	want := contentbundle.WarpDestinationDelta{Kind: interactionstore.KindWarp, Ref: "npc:gate", Change: "changed", Current: &current, Candidate: &candidate}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact warp-destination import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleWarpDestinationImportPreviewEndpointReturnsNotFoundWhenDestinationDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{
		{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Same gate.", MapIndex: 42, X: 1700, Y: 2800},
	}}}
	mux := RegisterLocalContentBundleWarpDestinationImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/warp-destinations/warp/npc:gate", strings.NewReader(`{"interaction_definitions":[{"kind":"warp","ref":"npc:gate","text":"Same gate.","map_index":42,"x":1700,"y":2800}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged warp destination delta, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestLocalContentBundleWarpDestinationImportPreviewEndpointRejectsInvalidIdentityBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleWarpDestinationImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/warp-destinations/warp",
		"/local/content-bundle/import-preview/warp-destinations/quest/npc:first_steps",
		"/local/content-bundle/import-preview/warp-destinations/warp/npc%2Fgate",
		"/local/content-bundle/import-preview/warp-destinations/warp/npc:gate/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"interaction_definitions":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed warp-destination import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed identity not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleWarpDestinationImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleWarpDestinationImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/warp-destinations/warp/npc:gate", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback warp-destination import-preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleWarpDestinationImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleWarpDestinationImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/warp-destinations/warp/npc:gate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method warp-destination import-preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method import preview not to call previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleWarpDestinationImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{
		{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000},
	}}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleWarpDestinationImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/warp-destinations/warp/npc:gate", strings.NewReader(`{"interaction_definitions":[{"kind":"warp","ref":"npc:gate","text":"New gate.","map_index":42,"x":1700,"y":2800}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.WarpDestinationDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused warp-destination import-preview delta response from shared mux: %v", err)
	}
	if got.Kind != interactionstore.KindWarp || got.Ref != "npc:gate" || got.Change != "changed" {
		t.Fatalf("unexpected focused warp-destination import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleShopRouteImportPreviewEndpointReturnsActorDeltasForLoopbackPost(t *testing.T) {
	currentShop := interactionstore.Definition{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Old Merchant", Catalog: []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Merchant", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: currentShop.Ref}},
		ItemTemplates:          []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		InteractionDefinitions: []interactionstore.Definition{currentShop},
	}}
	mux := RegisterLocalContentBundleShopRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/shop-routes/Merchant", strings.NewReader(`{"static_actors":[{"name":"Merchant","map_index":1,"x":1000,"y":2000,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:merchant"},{"name":"RemoteMerchant","map_index":3,"x":3000,"y":4000,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:remote_merchant"}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5},{"vnum":11200,"name":"Wooden Sword","stackable":false,"max_count":1}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1},{"slot":1,"item_vnum":11200,"price":500,"count":1}]},{"kind":"shop_preview","ref":"npc:remote_merchant","title":"Remote Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":75,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got []contentbundle.ShopRouteDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode shop-route import-preview delta response: %v", err)
	}
	currentRoute := contentbundle.ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Title: "Old Merchant", EntryCount: 1}
	candidateRoute := contentbundle.ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 2}
	want := []contentbundle.ShopRouteDelta{{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Change: "changed", Current: &currentRoute, Candidate: &candidateRoute}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected shop-route import-preview deltas:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestFlagRouteImportPreviewEndpointReturnsActorDeltasForLoopbackPost(t *testing.T) {
	currentQuest := interactionstore.Definition{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Old quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "QuestGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindQuestFlag, InteractionRef: currentQuest.Ref}},
		InteractionDefinitions: []interactionstore.Definition{currentQuest},
	}}
	mux := RegisterLocalContentBundleQuestFlagRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-flag-routes/QuestGuide", strings.NewReader(`{"static_actors":[{"name":"QuestGuide","map_index":1,"x":1000,"y":2000,"race_num":20302,"interaction_kind":"quest_flag","interaction_ref":"quest:first_steps"},{"name":"RemoteGuide","map_index":3,"x":3000,"y":4000,"race_num":20302,"interaction_kind":"quest_flag","interaction_ref":"quest:remote_steps"}],"interaction_definitions":[{"kind":"quest_flag","ref":"quest:first_steps","text":"New quest acknowledgement.","quest_ref":"quest:first_steps","quest_flag":"met_guide","quest_to":1},{"kind":"quest_flag","ref":"quest:remote_steps","text":"Remote quest acknowledgement.","quest_ref":"quest:remote_steps","quest_flag":"met_remote","quest_to":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got []contentbundle.QuestFlagRouteDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest-flag-route import-preview delta response: %v", err)
	}
	currentRoute := contentbundle.QuestFlagRouteSummary{ActorName: "QuestGuide", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "quest:first_steps", Text: "Old quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	candidateRoute := contentbundle.QuestFlagRouteSummary{ActorName: "QuestGuide", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "quest:first_steps", Text: "New quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	want := []contentbundle.QuestFlagRouteDelta{{ActorName: "QuestGuide", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "quest:first_steps", Change: "changed", Current: &currentRoute, Candidate: &candidateRoute}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected quest-flag-route import-preview deltas:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestFlagTriggerImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	currentQuest := interactionstore.Definition{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Old quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{InteractionDefinitions: []interactionstore.Definition{currentQuest}}}
	mux := RegisterLocalContentBundleQuestFlagTriggerImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-flag-triggers/quest_flag/quest:first_steps", strings.NewReader(`{"interaction_definitions":[{"kind":"quest_flag","ref":"quest:first_steps","text":"New quest acknowledgement.","quest_ref":"quest:first_steps","quest_flag":"met_guide","quest_to":1},{"kind":"quest_flag","ref":"quest:remote_steps","text":"Remote quest acknowledgement.","quest_ref":"quest:remote_steps","quest_flag":"met_remote","quest_to":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.QuestFlagTriggerDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact quest-flag-trigger import-preview delta response: %v", err)
	}
	current := contentbundle.QuestFlagTriggerSummary{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Old quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	candidate := contentbundle.QuestFlagTriggerSummary{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "New quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}
	want := contentbundle.QuestFlagTriggerDelta{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Change: "changed", Current: &current, Candidate: &candidate}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact quest-flag-trigger import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestFlagTriggerImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestFlagTriggerImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-flag-triggers/quest_flag/quest:first_steps", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback quest-flag trigger import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestFlagRouteImportPreviewEndpointReturnsNotFoundWhenActorHasNoRouteDeltas(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "QuestGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindQuestFlag, InteractionRef: "quest:first_steps"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Same quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}},
	}}
	mux := RegisterLocalContentBundleQuestFlagRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-flag-routes/QuestGuide", strings.NewReader(`{"static_actors":[{"name":"QuestGuide","map_index":1,"x":1000,"y":2000,"race_num":20302,"interaction_kind":"quest_flag","interaction_ref":"quest:first_steps"}],"interaction_definitions":[{"kind":"quest_flag","ref":"quest:first_steps","text":"Same quest acknowledgement.","quest_ref":"quest:first_steps","quest_flag":"met_guide","quest_to":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged quest-flag route actor, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing quest-flag route delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestFlagRouteImportPreviewEndpointRejectsMalformedActorNameBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestFlagRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/quest-flag-routes/",
		"/local/content-bundle/import-preview/quest-flag-routes/Bad%2FGuide",
		"/local/content-bundle/import-preview/quest-flag-routes/QuestGuide/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"interaction_definitions":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed quest-flag-route import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed quest-flag route actor names not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestFlagRouteImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestFlagRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-flag-routes/QuestGuide", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback quest-flag-route import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestFlagRouteImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleQuestFlagRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/quest-flag-routes/QuestGuide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method quest-flag-route import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleQuestFlagRouteImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "QuestGuide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindQuestFlag, InteractionRef: "quest:first_steps"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps", Text: "Old quest acknowledgement.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleQuestFlagRouteImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/quest-flag-routes/QuestGuide", strings.NewReader(`{"static_actors":[{"name":"QuestGuide","map_index":1,"x":1000,"y":2000,"race_num":20302,"interaction_kind":"quest_flag","interaction_ref":"quest:first_steps"}],"interaction_definitions":[{"kind":"quest_flag","ref":"quest:first_steps","text":"New quest acknowledgement.","quest_ref":"quest:first_steps","quest_flag":"met_guide","quest_to":1}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got []contentbundle.QuestFlagRouteDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused quest-flag-route import-preview delta response from shared mux: %v", err)
	}
	if len(got) != 1 || got[0].ActorName != "QuestGuide" || got[0].Change != "changed" || got[0].Candidate == nil || got[0].Candidate.Text != "New quest acknowledgement." {
		t.Fatalf("unexpected focused quest-flag-route import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleShopRouteImportPreviewEndpointReturnsNotFoundWhenActorHasNoRouteDeltas(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Merchant", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		ItemTemplates:          []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}},
	}}
	mux := RegisterLocalContentBundleShopRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/shop-routes/Merchant", strings.NewReader(`{"static_actors":[{"name":"Merchant","map_index":1,"x":1000,"y":2000,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:merchant"}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged shop route actor, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing shop route delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleShopRouteImportPreviewEndpointRejectsMalformedActorNameBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleShopRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/shop-routes/",
		"/local/content-bundle/import-preview/shop-routes/Bad%2FMerchant",
		"/local/content-bundle/import-preview/shop-routes/Merchant/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"interaction_definitions":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed shop-route import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed shop route actor names not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleShopRouteImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleShopRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/shop-routes/Merchant", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback shop-route import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleShopRouteImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleShopRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/shop-routes/Merchant", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method shop-route import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleShopRouteImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Merchant", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"}},
		ItemTemplates:          []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Old Merchant", Catalog: []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleShopRouteImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/shop-routes/Merchant", strings.NewReader(`{"static_actors":[{"name":"Merchant","map_index":1,"x":1000,"y":2000,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:merchant"}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200},{"vnum":11200,"name":"Wooden Sword","stackable":false,"max_count":1}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1},{"slot":1,"item_vnum":11200,"price":500,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got []contentbundle.ShopRouteDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused shop-route import-preview delta response from shared mux: %v", err)
	}
	if len(got) != 1 || got[0].ActorName != "Merchant" || got[0].Change != "changed" || got[0].Candidate == nil || got[0].Candidate.EntryCount != 2 {
		t.Fatalf("unexpected focused shop-route import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleWarpRouteImportPreviewEndpointReturnsActorDeltasForLoopbackPost(t *testing.T) {
	currentGate := interactionstore.Definition{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Gate", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: currentGate.Ref}},
		InteractionDefinitions: []interactionstore.Definition{currentGate},
	}}
	mux := RegisterLocalContentBundleWarpRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/warp-routes/Gate", strings.NewReader(`{"static_actors":[{"name":"Gate","map_index":1,"x":1100,"y":2100,"race_num":20300,"interaction_kind":"warp","interaction_ref":"npc:gate"},{"name":"RemoteGate","map_index":3,"x":3000,"y":4000,"race_num":20300,"interaction_kind":"warp","interaction_ref":"npc:remote_gate"}],"interaction_definitions":[{"kind":"warp","ref":"npc:gate","text":"New gate.","map_index":3,"x":2100,"y":3100},{"kind":"warp","ref":"npc:remote_gate","text":"Remote route.","map_index":9,"x":9000,"y":9100}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got []contentbundle.WarpRouteDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode warp-route import-preview delta response: %v", err)
	}
	currentRoute := contentbundle.WarpRouteSummary{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Text: "Old gate.", TargetMapIndex: 2, TargetX: 2000, TargetY: 3000}
	candidateRoute := contentbundle.WarpRouteSummary{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Text: "New gate.", TargetMapIndex: 3, TargetX: 2100, TargetY: 3100}
	want := []contentbundle.WarpRouteDelta{{ActorName: "Gate", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Change: "changed", Current: &currentRoute, Candidate: &candidateRoute}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected warp-route import-preview deltas:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleWarpRouteImportPreviewEndpointReturnsNotFoundWhenActorHasNoRouteDeltas(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Gate", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:gate"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Same gate.", MapIndex: 2, X: 2000, Y: 3000}},
	}}
	mux := RegisterLocalContentBundleWarpRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/warp-routes/Gate", strings.NewReader(`{"static_actors":[{"name":"Gate","map_index":1,"x":1100,"y":2100,"race_num":20300,"interaction_kind":"warp","interaction_ref":"npc:gate"}],"interaction_definitions":[{"kind":"warp","ref":"npc:gate","text":"Same gate.","map_index":2,"x":2000,"y":3000}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged warp route actor, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing warp route delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleWarpRouteImportPreviewEndpointRejectsMalformedActorNameBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleWarpRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/warp-routes/",
		"/local/content-bundle/import-preview/warp-routes/Bad%2FGate",
		"/local/content-bundle/import-preview/warp-routes/Gate/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"interaction_definitions":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed warp-route import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed warp route actor names not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleWarpRouteImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleWarpRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/warp-routes/Gate", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback warp-route import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleWarpRouteImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleWarpRouteImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/warp-routes/Gate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method warp-route import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleWarpRouteImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "Gate", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:gate"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleWarpRouteImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/warp-routes/Gate", strings.NewReader(`{"static_actors":[{"name":"Gate","map_index":1,"x":1100,"y":2100,"race_num":20300,"interaction_kind":"warp","interaction_ref":"npc:gate"}],"interaction_definitions":[{"kind":"warp","ref":"npc:gate","text":"New gate.","map_index":3,"x":2100,"y":3100}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got []contentbundle.WarpRouteDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused warp-route import-preview delta response from shared mux: %v", err)
	}
	if len(got) != 1 || got[0].ActorName != "Gate" || got[0].Change != "changed" || got[0].Candidate == nil || got[0].Candidate.TargetMapIndex != 3 {
		t.Fatalf("unexpected focused warp-route import-preview response from shared mux: %#v", got)
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
			ShopRoutes: []contentbundle.ShopRouteDelta{{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1200, SourceY: 2200, Ref: "npc:merchant", Change: "added", Candidate: &contentbundle.ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 1, SourceX: 1200, SourceY: 2200, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 2}}},
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
			WarpRoutes:                   []contentbundle.WarpRouteDelta{{ActorName: "Teleporter", SourceMapIndex: 7, SourceX: 1300, SourceY: 2300, Ref: "npc:teleporter", Change: "added", Candidate: &contentbundle.WarpRouteSummary{ActorName: "Teleporter", SourceMapIndex: 7, SourceX: 1300, SourceY: 2300, Ref: "npc:teleporter", Text: "Step through the gate.", TargetMapIndex: 7, TargetX: 1300, TargetY: 2300}}},
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

func TestLocalContentBundleMapImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuide", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
	}}
	mux := RegisterLocalContentBundleMapImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/maps/42", strings.NewReader(`{"static_actors":[{"name":"VillageGuide","map_index":42,"x":1100,"y":2100,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:guide"},{"name":"RemoteGuide","map_index":7,"x":1300,"y":2300,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:remote_guide"}],"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"Welcome."},{"kind":"talk","ref":"npc:remote_guide","text":"Remote."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.MapContentDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact map import-preview delta response: %v", err)
	}
	currentActor := contentbundle.StaticActor{Name: "VillageGuide", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}
	candidateActor := contentbundle.StaticActor{Name: "VillageGuide", MapIndex: 42, X: 1100, Y: 2100, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}
	want := contentbundle.MapContentDelta{
		MapIndex:                     42,
		StaticActorCount:             contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
		InteractableStaticActorCount: contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
		TalkActorCount:               contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
		StaticActors: []contentbundle.StaticActorDelta{
			{Change: "removed", Current: &currentActor},
			{Change: "added", Candidate: &candidateActor},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact map import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapImportPreviewEndpointReturnsServiceRouteDeltasForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{
			{Name: "Merchant", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 20301, InteractionKind: interactionstore.KindShopPreview, InteractionRef: "npc:merchant"},
			{Name: "Gate", MapIndex: 42, X: 1100, Y: 2100, RaceNum: 20300, InteractionKind: interactionstore.KindWarp, InteractionRef: "npc:gate"},
		},
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		InteractionDefinitions: []interactionstore.Definition{
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Old Merchant", Catalog: []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}}},
			{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000},
		},
	}}
	mux := RegisterLocalContentBundleMapImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/maps/42", strings.NewReader(`{"static_actors":[{"name":"Merchant","map_index":42,"x":1000,"y":2000,"race_num":20301,"interaction_kind":"shop_preview","interaction_ref":"npc:merchant"},{"name":"Gate","map_index":42,"x":1100,"y":2100,"race_num":20300,"interaction_kind":"warp","interaction_ref":"npc:gate"}],"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"New Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1}]},{"kind":"warp","ref":"npc:gate","text":"New gate.","map_index":3,"x":2100,"y":3100}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.MapContentDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact map import-preview service-route delta response: %v", err)
	}
	currentShopRoute := contentbundle.ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 42, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Title: "Old Merchant", EntryCount: 1}
	candidateShopRoute := contentbundle.ShopRouteSummary{ActorName: "Merchant", SourceMapIndex: 42, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Title: "New Merchant", EntryCount: 1}
	currentWarpRoute := contentbundle.WarpRouteSummary{ActorName: "Gate", SourceMapIndex: 42, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Text: "Old gate.", TargetMapIndex: 2, TargetX: 2000, TargetY: 3000}
	candidateWarpRoute := contentbundle.WarpRouteSummary{ActorName: "Gate", SourceMapIndex: 42, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Text: "New gate.", TargetMapIndex: 3, TargetX: 2100, TargetY: 3100}
	want := contentbundle.MapContentDelta{
		MapIndex:                     42,
		StaticActorCount:             contentbundle.SummaryCountDelta{Current: 2, Candidate: 2, Delta: 0},
		InteractableStaticActorCount: contentbundle.SummaryCountDelta{Current: 2, Candidate: 2, Delta: 0},
		ShopPreviewActorCount:        contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
		ShopCatalogEntryCount:        contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
		WarpActorCount:               contentbundle.SummaryCountDelta{Current: 1, Candidate: 1, Delta: 0},
		ShopRoutes: []contentbundle.ShopRouteDelta{
			{ActorName: "Merchant", SourceMapIndex: 42, SourceX: 1000, SourceY: 2000, Ref: "npc:merchant", Change: "changed", Current: &currentShopRoute, Candidate: &candidateShopRoute},
		},
		WarpRoutes: []contentbundle.WarpRouteDelta{
			{ActorName: "Gate", SourceMapIndex: 42, SourceX: 1100, SourceY: 2100, Ref: "npc:gate", Change: "changed", Current: &currentWarpRoute, Candidate: &candidateWarpRoute},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact map service-route import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapImportPreviewEndpointReturnsNotFoundWhenMapDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		StaticActors:           []contentbundle.StaticActor{{Name: "VillageGuide", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:guide"}},
		InteractionDefinitions: []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Text: "Welcome."}},
	}}
	mux := RegisterLocalContentBundleMapImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/maps/42", strings.NewReader(`{"static_actors":[{"name":"VillageGuide","map_index":42,"x":1000,"y":2000,"race_num":20302,"interaction_kind":"talk","interaction_ref":"npc:guide"}],"interaction_definitions":[{"kind":"talk","ref":"npc:guide","text":"Welcome."}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged map import preview, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing map delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleMapImportPreviewEndpointRejectsInvalidMapIndexBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleMapImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/maps/0",
		"/local/content-bundle/import-preview/maps/not-a-map",
		"/local/content-bundle/import-preview/maps/42/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"interaction_definitions":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed map import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed map indexes not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleMapImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleMapImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/maps/42", strings.NewReader(`{"interaction_definitions":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback map import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleMapImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleMapImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/maps/42", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method map import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleItemTemplateImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:  interactionstore.KindShopPreview,
			Ref:   "npc:merchant",
			Title: "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{
				{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
				{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
			},
		}},
	}}
	mux := RegisterLocalContentBundleItemTemplateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/item-templates/27001", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":7},{"vnum":27002,"name":"Small Blue Potion","stackable":true,"max_count":200,"shop_buy_price":6}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1},{"slot":1,"item_vnum":27002,"price":80,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.ItemTemplateDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact item-template import-preview delta response: %v", err)
	}
	currentRed := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	candidateRed := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 7}
	want := contentbundle.ItemTemplateDelta{Vnum: 27001, Change: "changed", Current: &currentRed, Candidate: &candidateRed}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact item-template import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleItemTemplateImportPreviewEndpointReturnsNotFoundWhenTemplateDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:    interactionstore.KindShopPreview,
			Ref:     "npc:merchant",
			Title:   "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}},
		}},
	}}
	mux := RegisterLocalContentBundleItemTemplateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/item-templates/27001", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged item-template import preview, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing item-template delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleItemTemplateImportPreviewEndpointRejectsInvalidVnumBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleItemTemplateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/item-templates/0",
		"/local/content-bundle/import-preview/item-templates/not-a-vnum",
		"/local/content-bundle/import-preview/item-templates/27001/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"item_templates":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed item-template import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed item-template identities not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleItemTemplateImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleItemTemplateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/item-templates/27001", strings.NewReader(`{"item_templates":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback item-template import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleItemTemplateImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleItemTemplateImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/item-templates/27001", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method item-template import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleItemTemplateImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:    interactionstore.KindShopPreview,
			Ref:     "npc:merchant",
			Title:   "Village Merchant",
			Catalog: []interactionstore.MerchantCatalogEntry{{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1}},
		}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleItemTemplateImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/item-templates/27001", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":7}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:merchant","title":"Village Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":1}]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.ItemTemplateDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused item-template import-preview delta response from shared mux: %v", err)
	}
	if got.Vnum != 27001 || got.Change != "changed" || got.Candidate == nil || got.Candidate.ShopBuyPrice != 7 {
		t.Fatalf("unexpected focused item-template import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleRewardDropImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
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
	mux := RegisterLocalContentBundleRewardDropImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/reward-drops/27001", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5}],"spawn_groups":[{"ref":"practice.red","name":"Red Drop Mob","map_index":42,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]},{"ref":"practice.red_bonus","name":"Bonus Red Drop Mob","map_index":42,"x":1200,"y":2200,"race_num":103,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.RewardDropDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact reward-drop import-preview delta response: %v", err)
	}
	currentRed := contentbundle.RewardDropAggregateSummary{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	candidateRed := contentbundle.RewardDropAggregateSummary{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 2, Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	want := contentbundle.RewardDropDelta{ItemVnum: 27001, Change: "changed", Current: &currentRed, Candidate: &candidateRed}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact reward-drop import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleRewardDropImportPreviewEndpointReturnsNotFoundWhenRewardDropDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		SpawnGroups:   []contentbundle.SpawnGroup{{Ref: "practice.red", Name: "Red Drop Mob", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27001}}},
	}}
	mux := RegisterLocalContentBundleRewardDropImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/reward-drops/27001", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5}],"spawn_groups":[{"ref":"practice.red","name":"Red Drop Mob","map_index":42,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged reward-drop import preview, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing reward-drop delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleRewardDropImportPreviewEndpointRejectsInvalidVnumBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleRewardDropImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/reward-drops/0",
		"/local/content-bundle/import-preview/reward-drops/not-a-vnum",
		"/local/content-bundle/import-preview/reward-drops/27001/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"item_templates":[],"spawn_groups":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed reward-drop import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed reward-drop identities not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleRewardDropImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleRewardDropImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/reward-drops/27001", strings.NewReader(`{"item_templates":[],"spawn_groups":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback reward-drop import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleRewardDropImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleRewardDropImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/reward-drops/27001", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method reward-drop import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleRewardDropImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		SpawnGroups:   []contentbundle.SpawnGroup{{Ref: "practice.red", Name: "Red Drop Mob", MapIndex: 42, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardDropVnums: []uint32{27001}}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleRewardDropImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/reward-drops/27001", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5}],"spawn_groups":[{"ref":"practice.red","name":"Red Drop Mob","map_index":42,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]},{"ref":"practice.red_bonus","name":"Bonus Red Drop Mob","map_index":42,"x":1200,"y":2200,"race_num":103,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.RewardDropDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused reward-drop import-preview delta response from shared mux: %v", err)
	}
	if got.ItemVnum != 27001 || got.Change != "changed" || got.Candidate == nil || got.Candidate.SourceCount != 2 {
		t.Fatalf("unexpected focused reward-drop import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleSpawnGroupImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		ItemTemplates: []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}},
		SpawnGroups: []contentbundle.SpawnGroup{
			{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 10, RewardGold: 5, RewardDropVnums: []uint32{27001}},
			{Ref: "practice.remove", Name: "Removed Mob", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 102, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 3, RewardGold: 1},
		},
	}}
	mux := RegisterLocalContentBundleSpawnGroupImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/spawn-groups/practice.keep", strings.NewReader(`{"item_templates":[{"vnum":27001,"name":"Small Red Potion","stackable":true,"max_count":200,"shop_buy_price":5},{"vnum":27002,"name":"Small Blue Potion","stackable":true,"max_count":200,"shop_buy_price":7}],"spawn_groups":[{"ref":"practice.add","name":"Added Mob","map_index":2,"x":1300,"y":2300,"race_num":103,"combat_profile":"practice_mob","reward_experience":7,"reward_gold":2,"reward_drop_vnums":[27002]},{"ref":"practice.keep","name":"Keep Mob","map_index":1,"x":1200,"y":2200,"race_num":101,"combat_profile":"practice_mob","reward_experience":20,"reward_gold":8,"reward_drop_vnums":[27001]}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.SpawnGroupDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact spawn-group import-preview delta response: %v", err)
	}
	currentKeep := contentbundle.SpawnGroupReferenceSummary{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 10, RewardGold: 5, RewardDropVnums: []uint32{27001}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}
	candidateKeep := contentbundle.SpawnGroupReferenceSummary{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1200, Y: 2200, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, RewardExperience: 20, RewardGold: 8, RewardDropVnums: []uint32{27001}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}
	want := contentbundle.SpawnGroupDelta{Ref: "practice.keep", Change: "changed", Current: &currentKeep, Candidate: &candidateKeep}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact spawn-group import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleSpawnGroupImportPreviewEndpointReturnsNotFoundWhenSpawnGroupDoesNotChange(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob}},
	}}
	mux := RegisterLocalContentBundleSpawnGroupImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/spawn-groups/practice.keep", strings.NewReader(`{"spawn_groups":[{"ref":"practice.keep","name":"Keep Mob","map_index":1,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_mob"}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged spawn-group import preview, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing spawn-group delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleSpawnGroupImportPreviewEndpointRejectsInvalidRefBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleSpawnGroupImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/spawn-groups/practice",
		"/local/content-bundle/import-preview/spawn-groups/practice%2Fkeep",
		"/local/content-bundle/import-preview/spawn-groups/practice.keep/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"spawn_groups":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed spawn-group import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed spawn-group identities not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleSpawnGroupImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleSpawnGroupImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/spawn-groups/practice.keep", strings.NewReader(`{"spawn_groups":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback spawn-group import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleSpawnGroupImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleSpawnGroupImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/spawn-groups/practice.keep", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method spawn-group import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleSpawnGroupImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleSpawnGroupImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/spawn-groups/practice.keep", strings.NewReader(`{"spawn_groups":[{"ref":"practice.keep","name":"Keep Mob","map_index":1,"x":1200,"y":2200,"race_num":101,"combat_profile":"practice_mob"}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.SpawnGroupDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused spawn-group import-preview delta response from shared mux: %v", err)
	}
	if got.Ref != "practice.keep" || got.Change != "changed" || got.Candidate == nil || got.Candidate.X != 1200 {
		t.Fatalf("unexpected focused spawn-group import-preview response from shared mux: %#v", got)
	}
}

func TestLocalContentBundleCombatProfileImportPreviewEndpointReturnsExactDeltaForLoopbackPost(t *testing.T) {
	currentKeepProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_keep_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 2, Rank: 1, RespawnDelayMs: 1500}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{currentKeepProfile},
		SpawnGroups: []contentbundle.SpawnGroup{
			{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: currentKeepProfile.Profile},
		},
	}}
	mux := RegisterLocalContentBundleCombatProfileImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/combat-profiles/practice_keep_profile", strings.NewReader(`{"combat_profiles":[{"profile":"practice_keep_profile","max_hp":28,"damage_per_normal_attack":3,"attack_value":7,"defense_value":4,"level":2,"rank":1,"respawn_delay_ms":1500}],"spawn_groups":[{"ref":"practice.keep","name":"Keep Mob","map_index":1,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_keep_profile"}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected import previewer to be called once, got %d calls", previewer.calls)
	}
	var got contentbundle.CombatProfileDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode exact combat-profile import-preview delta response: %v", err)
	}
	candidateKeepProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_keep_profile", MaxHP: 28, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 2, Rank: 1, RespawnDelayMs: 1500}
	want := contentbundle.CombatProfileDelta{Profile: "practice_keep_profile", Change: "changed", Current: &currentKeepProfile, Candidate: &candidateKeepProfile}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exact combat-profile import-preview delta:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleCombatProfileImportPreviewEndpointReturnsNotFoundWhenProfileDoesNotChange(t *testing.T) {
	currentKeepProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_keep_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 2, Rank: 1, RespawnDelayMs: 1500}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{currentKeepProfile},
		SpawnGroups:    []contentbundle.SpawnGroup{{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: currentKeepProfile.Profile}},
	}}
	mux := RegisterLocalContentBundleCombatProfileImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/combat-profiles/practice_keep_profile", strings.NewReader(`{"combat_profiles":[{"profile":"practice_keep_profile","max_hp":24,"damage_per_normal_attack":3,"attack_value":7,"defense_value":4,"level":2,"rank":1,"respawn_delay_ms":1500}],"spawn_groups":[{"ref":"practice.keep","name":"Keep Mob","map_index":1,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_keep_profile"}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for unchanged combat-profile import preview, got %d", http.StatusNotFound, rec.Code)
	}
	if previewer.calls != 1 {
		t.Fatalf("expected missing combat-profile delta lookup to call import previewer once, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleCombatProfileImportPreviewEndpointRejectsInvalidProfileBeforeCallback(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleCombatProfileImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	for _, path := range []string{
		"/local/content-bundle/import-preview/combat-profiles/PracticeKeep",
		"/local/content-bundle/import-preview/combat-profiles/practice%2Fkeep",
		"/local/content-bundle/import-preview/combat-profiles/practice_keep_profile/extra",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"combat_profiles":[]}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for malformed combat-profile import-preview path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if previewer.calls != 0 {
		t.Fatalf("expected malformed combat-profile identities not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleCombatProfileImportPreviewEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleCombatProfileImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/combat-profiles/practice_keep_profile", strings.NewReader(`{"combat_profiles":[]}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback combat-profile import preview, got %d", http.StatusForbidden, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleCombatProfileImportPreviewEndpointRejectsWrongMethod(t *testing.T) {
	previewer := &stubContentBundleImportPreviewer{}
	mux := RegisterLocalContentBundleCombatProfileImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/import-preview/combat-profiles/practice_keep_profile", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method combat-profile import preview, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if previewer.calls != 0 {
		t.Fatalf("expected wrong-method request not to call import previewer, got %d calls", previewer.calls)
	}
}

func TestLocalContentBundleCombatProfileImportPreviewEndpointCoexistsWithBroadImportPreview(t *testing.T) {
	currentKeepProfile := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_keep_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 2, Rank: 1, RespawnDelayMs: 1500}
	previewer := &stubContentBundleImportPreviewer{current: contentbundle.Bundle{
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{currentKeepProfile},
		SpawnGroups:    []contentbundle.SpawnGroup{{Ref: "practice.keep", Name: "Keep Mob", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 101, CombatProfile: currentKeepProfile.Profile}},
	}}
	mux := RegisterLocalContentBundleImportPreviewEndpoint(NewPprofMux("gamed"), previewer.PreviewContentBundleImport)
	mux = RegisterLocalContentBundleCombatProfileImportPreviewEndpoint(mux, previewer.PreviewContentBundleImport)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/import-preview/combat-profiles/practice_keep_profile", strings.NewReader(`{"combat_profiles":[{"profile":"practice_keep_profile","max_hp":28,"damage_per_normal_attack":3,"attack_value":7,"defense_value":4,"level":2,"rank":1,"respawn_delay_ms":1500}],"spawn_groups":[{"ref":"practice.keep","name":"Keep Mob","map_index":1,"x":1000,"y":2000,"race_num":101,"combat_profile":"practice_keep_profile"}]}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d from focused route on shared mux, got %d", http.StatusOK, rec.Code)
	}
	var got contentbundle.CombatProfileDelta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode focused combat-profile import-preview delta response from shared mux: %v", err)
	}
	if got.Profile != "practice_keep_profile" || got.Change != "changed" || got.Candidate == nil || got.Candidate.MaxHP != 28 {
		t.Fatalf("unexpected focused combat-profile import-preview response from shared mux: %#v", got)
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

func TestLocalContentBundleSummaryEndpointReturnsQuestStateQuestCountForLoopbackPost(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"quest_state":[{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":1},{"character":"QuestHero","quest_ref":"quest:daily_check","name":"talked_to_guide","value":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/summary", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected dry-run quest-state summary not to call live exporter, got %d calls", summaryer.calls)
	}
	var got contentbundle.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode dry-run quest-state summary response body: %v", err)
	}
	if got.QuestStateFlagCount != 2 || got.QuestStateCharacterCount != 1 || got.QuestStateQuestCount != 2 {
		t.Fatalf("unexpected quest-state summary counts: %+v", got)
	}
	wantQuestRefs := []string{"quest:daily_check", "quest:first_steps"}
	if !reflect.DeepEqual(got.QuestStateQuestRefs, wantQuestRefs) {
		t.Fatalf("unexpected quest-state quest refs:\n got: %#v\nwant: %#v", got.QuestStateQuestRefs, wantQuestRefs)
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

func TestLocalContentBundleMapSummaryEndpointReturnsExactMapForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{
			{MapIndex: 7, StaticActorCount: 1, InteractableStaticActorCount: 1, InfoActorCount: 1},
			{MapIndex: 42, StaticActorCount: 2, InteractableStaticActorCount: 1, ShopPreviewActorCount: 1, ShopCatalogEntryCount: 3, SpawnGroupCount: 1, RewardDropItemCount: 2},
		}},
	}
	mux := RegisterLocalContentBundleMapSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/42", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.MapContentSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode map summary response body: %v", err)
	}
	want := contentbundle.MapContentSummary{MapIndex: 42, StaticActorCount: 2, InteractableStaticActorCount: 1, ShopPreviewActorCount: 1, ShopCatalogEntryCount: 3, SpawnGroupCount: 1, RewardDropItemCount: 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle map summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapSummaryEndpointCoexistsWithContentBundleCollectionRoutes(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{}}
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1}}}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)
	mux = RegisterLocalContentBundleSummaryEndpoint(mux, summaryer.ExportContentBundleSummary)
	mux = RegisterLocalContentBundleMapSummaryEndpoint(mux, summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/42", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for map summary alongside content-bundle routes, got %d", http.StatusOK, rec.Code)
	}
	if exporter.calls != 0 {
		t.Fatalf("expected collection exporter not to handle map summary route, got %d calls", exporter.calls)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleMapSummaryEndpointReturnsNotFoundForMissingMap(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 7, StaticActorCount: 1}}}}
	mux := RegisterLocalContentBundleMapSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/42", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing map summary, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleMapSummaryEndpointRejectsInvalidMapIndex(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1}}}}
	mux := RegisterLocalContentBundleMapSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/maps/0", "/local/content-bundle/maps/not-a-map", "/local/content-bundle/maps/42/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid map summary path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid map index, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleMapSummaryEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1}}}}
	mux := RegisterLocalContentBundleMapSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/42", nil)
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

func TestLocalContentBundleMapSummaryEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1}}}}
	mux := RegisterLocalContentBundleMapSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/maps/42", nil)
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

func TestLocalContentBundleMapStaticActorsEndpointReturnsMatchingActorsForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			Maps: []contentbundle.MapContentSummary{
				{MapIndex: 1, StaticActorCount: 2, InteractableStaticActorCount: 1},
				{MapIndex: 7, StaticActorCount: 1, InteractableStaticActorCount: 1},
			},
			StaticActors: []contentbundle.StaticActor{
				{Name: "Remote Guide", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:remote_guide"},
				{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide"},
				{Name: "Village Herald", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20303},
			},
		},
	}
	mux := RegisterLocalContentBundleMapStaticActorsEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/1/static-actors", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.StaticActor
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode map static-actor response body: %v", err)
	}
	want := []contentbundle.StaticActor{
		{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide"},
		{Name: "Village Herald", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20303},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle map static actors:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapInteractableStaticActorsEndpointReturnsMatchingActorsForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			Maps: []contentbundle.MapContentSummary{
				{MapIndex: 1, StaticActorCount: 3, InteractableStaticActorCount: 2, InfoActorCount: 1, TalkActorCount: 1},
				{MapIndex: 7, StaticActorCount: 1, InteractableStaticActorCount: 1, TalkActorCount: 1},
			},
			InteractableStaticActors: []contentbundle.InteractableStaticActorSummary{
				{Name: "Remote Guide", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:remote_guide", Preview: "Remote hello."},
				{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide", Preview: "Welcome."},
				{Name: "Village Lore", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20303, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:village", Preview: "The village is quiet."},
			},
		},
	}
	mux := RegisterLocalContentBundleMapInteractableStaticActorsEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/1/interactable-static-actors", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.InteractableStaticActorSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode map interactable-static-actor response body: %v", err)
	}
	want := []contentbundle.InteractableStaticActorSummary{
		{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide", Preview: "Welcome."},
		{Name: "Village Lore", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 20303, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:village", Preview: "The village is quiet."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle map interactable static actors:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapSpawnGroupsEndpointReturnsMatchingSpawnGroupsForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			Maps: []contentbundle.MapContentSummary{
				{MapIndex: 1, SpawnGroupCount: 2},
				{MapIndex: 7, SpawnGroupCount: 1},
			},
			SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{
				{Ref: "practice.remote_wolf", Name: "Remote Wolf", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 101, CombatProfile: "practice_mob", RewardExperience: 25, RewardGold: 10},
				{Ref: "practice.village_dummy", Name: "Village Dummy", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 102, CombatProfile: "training_dummy"},
				{Ref: "practice.village_wolf", Name: "Village Wolf", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 101, CombatProfile: "practice_mob", RewardExperience: 50, RewardGold: 20, RewardDropVnums: []uint32{27001}},
			},
		},
	}
	mux := RegisterLocalContentBundleMapSpawnGroupsEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/1/spawn-groups", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.SpawnGroupReferenceSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode map spawn-group response body: %v", err)
	}
	want := []contentbundle.SpawnGroupReferenceSummary{
		{Ref: "practice.village_dummy", Name: "Village Dummy", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 102, CombatProfile: "training_dummy"},
		{Ref: "practice.village_wolf", Name: "Village Wolf", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 101, CombatProfile: "practice_mob", RewardExperience: 50, RewardGold: 20, RewardDropVnums: []uint32{27001}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle map spawn groups:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapRewardDropsEndpointReturnsMapLocalAggregatesForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			Maps: []contentbundle.MapContentSummary{
				{MapIndex: 1, SpawnGroupCount: 2, RewardDropItemCount: 3},
				{MapIndex: 7, SpawnGroupCount: 1, RewardDropItemCount: 1},
			},
			SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{
				{Ref: "practice.remote_wolf", Name: "Remote Wolf", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 101, CombatProfile: "practice_mob", RewardDropVnums: []uint32{27001}},
				{Ref: "practice.village_dummy", Name: "Village Dummy", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 102, CombatProfile: "training_dummy", RewardDropVnums: []uint32{27002, 27001}},
				{Ref: "practice.village_wolf", Name: "Village Wolf", MapIndex: 1, X: 1100, Y: 2100, RaceNum: 101, CombatProfile: "practice_mob", RewardDropVnums: []uint32{27001}},
			},
			RewardDrops: []contentbundle.RewardDropAggregateSummary{
				{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 3, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
				{ItemVnum: 27002, ItemName: "Small Blue Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
			},
		},
	}
	mux := RegisterLocalContentBundleMapRewardDropsEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/1/reward-drops", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.RewardDropAggregateSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode map reward-drop response body: %v", err)
	}
	want := []contentbundle.RewardDropAggregateSummary{
		{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 2, Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{ItemVnum: 27002, ItemName: "Small Blue Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle map reward drops:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapShopRoutesEndpointReturnsMatchingRoutesForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			Maps: []contentbundle.MapContentSummary{
				{MapIndex: 1, StaticActorCount: 2, ShopPreviewActorCount: 2},
				{MapIndex: 7, StaticActorCount: 1, ShopPreviewActorCount: 1},
			},
			ShopRoutes: []contentbundle.ShopRouteSummary{
				{ActorName: "Remote Merchant", SourceMapIndex: 7, SourceX: 1300, SourceY: 2300, Ref: "npc:remote_merchant", Title: "Remote Merchant", EntryCount: 1},
				{ActorName: "Village Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_merchant", Title: "Village Merchant", EntryCount: 2},
				{ActorName: "Village Provisioner", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:village_provisioner", Title: "Village Provisioner", EntryCount: 1},
			},
		},
	}
	mux := RegisterLocalContentBundleMapShopRoutesEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/1/shop-routes", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.ShopRouteSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode map shop-route response body: %v", err)
	}
	want := []contentbundle.ShopRouteSummary{
		{ActorName: "Village Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_merchant", Title: "Village Merchant", EntryCount: 2},
		{ActorName: "Village Provisioner", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:village_provisioner", Title: "Village Provisioner", EntryCount: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle map shop routes:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapQuestFlagRoutesEndpointReturnsMatchingRoutesForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			Maps: []contentbundle.MapContentSummary{
				{MapIndex: 1, StaticActorCount: 2, QuestFlagActorCount: 2},
				{MapIndex: 7, StaticActorCount: 1, QuestFlagActorCount: 1},
			},
			QuestFlagRoutes: []contentbundle.QuestFlagRouteSummary{
				{ActorName: "Remote Guide", SourceMapIndex: 7, SourceX: 1300, SourceY: 2300, Ref: "quest:remote_steps", Text: "Remote quest acknowledgement.", QuestRef: "quest:remote_steps", QuestFlag: "met_remote", QuestTo: 1},
				{ActorName: "Quest Guide", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1},
				{ActorName: "Quest Reset", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "quest:first_steps_reset", Text: "Quest cleared.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1},
			},
		},
	}
	mux := RegisterLocalContentBundleMapQuestFlagRoutesEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/1/quest-flag-routes", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.QuestFlagRouteSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode map quest-flag-route response body: %v", err)
	}
	want := []contentbundle.QuestFlagRouteSummary{
		{ActorName: "Quest Guide", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1},
		{ActorName: "Quest Reset", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "quest:first_steps_reset", Text: "Quest cleared.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle map quest-flag routes:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapWarpRoutesEndpointReturnsMatchingRoutesForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			Maps: []contentbundle.MapContentSummary{
				{MapIndex: 1, StaticActorCount: 2, WarpActorCount: 2},
				{MapIndex: 7, StaticActorCount: 1, WarpActorCount: 1},
			},
			WarpRoutes: []contentbundle.WarpRouteSummary{
				{ActorName: "Remote Gate", SourceMapIndex: 7, SourceX: 1300, SourceY: 2300, Ref: "npc:remote_gate", Text: "Remote gate.", TargetMapIndex: 8, TargetX: 1400, TargetY: 2400},
				{ActorName: "Village Gate", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_gate", Text: "Step through the gate.", TargetMapIndex: 42, TargetX: 1700, TargetY: 2800},
				{ActorName: "Village Ferry", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:village_ferry", TargetMapIndex: 43, TargetX: 1800, TargetY: 2900},
			},
		},
	}
	mux := RegisterLocalContentBundleMapWarpRoutesEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/1/warp-routes", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.WarpRouteSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode map warp-route response body: %v", err)
	}
	want := []contentbundle.WarpRouteSummary{
		{ActorName: "Village Gate", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_gate", Text: "Step through the gate.", TargetMapIndex: 42, TargetX: 1700, TargetY: 2800},
		{ActorName: "Village Ferry", SourceMapIndex: 1, SourceX: 1100, SourceY: 2100, Ref: "npc:village_ferry", TargetMapIndex: 43, TargetX: 1800, TargetY: 2900},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle map warp routes:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleMapFocusedContentEndpointsReturnEmptyListForKnownMapWithoutMatches(t *testing.T) {
	tests := []struct {
		name     string
		register func(*http.ServeMux, func() (any, int)) *http.ServeMux
		path     string
	}{
		{name: "static actors", register: RegisterLocalContentBundleMapStaticActorsEndpoint, path: "/local/content-bundle/maps/42/static-actors"},
		{name: "interactable static actors", register: RegisterLocalContentBundleMapInteractableStaticActorsEndpoint, path: "/local/content-bundle/maps/42/interactable-static-actors"},
		{name: "quest flag routes", register: RegisterLocalContentBundleMapQuestFlagRoutesEndpoint, path: "/local/content-bundle/maps/42/quest-flag-routes"},
		{name: "shop routes", register: RegisterLocalContentBundleMapShopRoutesEndpoint, path: "/local/content-bundle/maps/42/shop-routes"},
		{name: "warp routes", register: RegisterLocalContentBundleMapWarpRoutesEndpoint, path: "/local/content-bundle/maps/42/warp-routes"},
		{name: "spawn groups", register: RegisterLocalContentBundleMapSpawnGroupsEndpoint, path: "/local/content-bundle/maps/42/spawn-groups"},
		{name: "reward drops", register: RegisterLocalContentBundleMapRewardDropsEndpoint, path: "/local/content-bundle/maps/42/reward-drops"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1}}}}
			mux := tc.register(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d for known map without routes, got %d", http.StatusOK, rec.Code)
			}
			if strings.TrimSpace(rec.Body.String()) != "[]" {
				t.Fatalf("expected empty JSON list for known map without routes, got %q", rec.Body.String())
			}
			if summaryer.calls != 1 {
				t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
			}
		})
	}
}

func TestLocalContentBundleMapFocusedContentEndpointsReturnNotFoundForMissingMap(t *testing.T) {
	tests := []struct {
		name     string
		register func(*http.ServeMux, func() (any, int)) *http.ServeMux
		path     string
	}{
		{name: "static actors", register: RegisterLocalContentBundleMapStaticActorsEndpoint, path: "/local/content-bundle/maps/42/static-actors"},
		{name: "interactable static actors", register: RegisterLocalContentBundleMapInteractableStaticActorsEndpoint, path: "/local/content-bundle/maps/42/interactable-static-actors"},
		{name: "quest flag routes", register: RegisterLocalContentBundleMapQuestFlagRoutesEndpoint, path: "/local/content-bundle/maps/42/quest-flag-routes"},
		{name: "shop routes", register: RegisterLocalContentBundleMapShopRoutesEndpoint, path: "/local/content-bundle/maps/42/shop-routes"},
		{name: "warp routes", register: RegisterLocalContentBundleMapWarpRoutesEndpoint, path: "/local/content-bundle/maps/42/warp-routes"},
		{name: "spawn groups", register: RegisterLocalContentBundleMapSpawnGroupsEndpoint, path: "/local/content-bundle/maps/42/spawn-groups"},
		{name: "reward drops", register: RegisterLocalContentBundleMapRewardDropsEndpoint, path: "/local/content-bundle/maps/42/reward-drops"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 7, StaticActorCount: 1}}}}
			mux := tc.register(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected status %d for missing map service route list, got %d", http.StatusNotFound, rec.Code)
			}
			if summaryer.calls != 1 {
				t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
			}
		})
	}
}

func TestLocalContentBundleMapFocusedContentEndpointsRejectInvalidMapIndex(t *testing.T) {
	tests := []struct {
		name     string
		register func(*http.ServeMux, func() (any, int)) *http.ServeMux
		paths    []string
	}{
		{name: "static actors", register: RegisterLocalContentBundleMapStaticActorsEndpoint, paths: []string{"/local/content-bundle/maps/0/static-actors", "/local/content-bundle/maps/not-a-map/static-actors", "/local/content-bundle/maps/42/static-actors/extra"}},
		{name: "interactable static actors", register: RegisterLocalContentBundleMapInteractableStaticActorsEndpoint, paths: []string{"/local/content-bundle/maps/0/interactable-static-actors", "/local/content-bundle/maps/not-a-map/interactable-static-actors", "/local/content-bundle/maps/42/interactable-static-actors/extra"}},
		{name: "quest flag routes", register: RegisterLocalContentBundleMapQuestFlagRoutesEndpoint, paths: []string{"/local/content-bundle/maps/0/quest-flag-routes", "/local/content-bundle/maps/not-a-map/quest-flag-routes", "/local/content-bundle/maps/42/quest-flag-routes/extra"}},
		{name: "shop routes", register: RegisterLocalContentBundleMapShopRoutesEndpoint, paths: []string{"/local/content-bundle/maps/0/shop-routes", "/local/content-bundle/maps/not-a-map/shop-routes", "/local/content-bundle/maps/42/shop-routes/extra"}},
		{name: "warp routes", register: RegisterLocalContentBundleMapWarpRoutesEndpoint, paths: []string{"/local/content-bundle/maps/0/warp-routes", "/local/content-bundle/maps/not-a-map/warp-routes", "/local/content-bundle/maps/42/warp-routes/extra"}},
		{name: "spawn groups", register: RegisterLocalContentBundleMapSpawnGroupsEndpoint, paths: []string{"/local/content-bundle/maps/0/spawn-groups", "/local/content-bundle/maps/not-a-map/spawn-groups", "/local/content-bundle/maps/42/spawn-groups/extra"}},
		{name: "reward drops", register: RegisterLocalContentBundleMapRewardDropsEndpoint, paths: []string{"/local/content-bundle/maps/0/reward-drops", "/local/content-bundle/maps/not-a-map/reward-drops", "/local/content-bundle/maps/42/reward-drops/extra"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1}}}}
			mux := tc.register(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)
			for _, path := range tc.paths {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.RemoteAddr = "127.0.0.1:12345"
				rec := httptest.NewRecorder()

				mux.ServeHTTP(rec, req)

				if rec.Code != http.StatusBadRequest {
					t.Fatalf("expected status %d for invalid map service route path %q, got %d", http.StatusBadRequest, path, rec.Code)
				}
			}
			if summaryer.calls != 0 {
				t.Fatalf("expected content bundle summary exporter not to be called for invalid map index, got %d calls", summaryer.calls)
			}
		})
	}
}

func TestLocalContentBundleMapFocusedContentEndpointsRejectNonLoopbackRemoteAddr(t *testing.T) {
	tests := []struct {
		name     string
		register func(*http.ServeMux, func() (any, int)) *http.ServeMux
		path     string
	}{
		{name: "static actors", register: RegisterLocalContentBundleMapStaticActorsEndpoint, path: "/local/content-bundle/maps/42/static-actors"},
		{name: "interactable static actors", register: RegisterLocalContentBundleMapInteractableStaticActorsEndpoint, path: "/local/content-bundle/maps/42/interactable-static-actors"},
		{name: "quest flag routes", register: RegisterLocalContentBundleMapQuestFlagRoutesEndpoint, path: "/local/content-bundle/maps/42/quest-flag-routes"},
		{name: "shop routes", register: RegisterLocalContentBundleMapShopRoutesEndpoint, path: "/local/content-bundle/maps/42/shop-routes"},
		{name: "warp routes", register: RegisterLocalContentBundleMapWarpRoutesEndpoint, path: "/local/content-bundle/maps/42/warp-routes"},
		{name: "spawn groups", register: RegisterLocalContentBundleMapSpawnGroupsEndpoint, path: "/local/content-bundle/maps/42/spawn-groups"},
		{name: "reward drops", register: RegisterLocalContentBundleMapRewardDropsEndpoint, path: "/local/content-bundle/maps/42/reward-drops"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1}}}}
			mux := tc.register(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = "203.0.113.10:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected status %d for non-loopback caller, got %d", http.StatusForbidden, rec.Code)
			}
			if summaryer.calls != 0 {
				t.Fatalf("expected content bundle summary exporter not to be called, got %d calls", summaryer.calls)
			}
		})
	}
}

func TestLocalContentBundleMapFocusedContentEndpointsRejectWrongMethod(t *testing.T) {
	tests := []struct {
		name     string
		register func(*http.ServeMux, func() (any, int)) *http.ServeMux
		path     string
	}{
		{name: "static actors", register: RegisterLocalContentBundleMapStaticActorsEndpoint, path: "/local/content-bundle/maps/42/static-actors"},
		{name: "interactable static actors", register: RegisterLocalContentBundleMapInteractableStaticActorsEndpoint, path: "/local/content-bundle/maps/42/interactable-static-actors"},
		{name: "quest flag routes", register: RegisterLocalContentBundleMapQuestFlagRoutesEndpoint, path: "/local/content-bundle/maps/42/quest-flag-routes"},
		{name: "shop routes", register: RegisterLocalContentBundleMapShopRoutesEndpoint, path: "/local/content-bundle/maps/42/shop-routes"},
		{name: "warp routes", register: RegisterLocalContentBundleMapWarpRoutesEndpoint, path: "/local/content-bundle/maps/42/warp-routes"},
		{name: "spawn groups", register: RegisterLocalContentBundleMapSpawnGroupsEndpoint, path: "/local/content-bundle/maps/42/spawn-groups"},
		{name: "reward drops", register: RegisterLocalContentBundleMapRewardDropsEndpoint, path: "/local/content-bundle/maps/42/reward-drops"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{Maps: []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1}}}}
			mux := tc.register(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status %d for wrong method, got %d", http.StatusMethodNotAllowed, rec.Code)
			}
			if summaryer.calls != 0 {
				t.Fatalf("expected content bundle summary exporter not to be called, got %d calls", summaryer.calls)
			}
		})
	}
}

func TestLocalContentBundleMapFocusedContentEndpointsCoexistWithMapSummaryRoute(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			Maps:        []contentbundle.MapContentSummary{{MapIndex: 42, StaticActorCount: 1, ShopPreviewActorCount: 1, SpawnGroupCount: 1, RewardDropItemCount: 1}},
			ShopRoutes:  []contentbundle.ShopRouteSummary{{ActorName: "Village Merchant", SourceMapIndex: 42, SourceX: 1700, SourceY: 2800, Ref: "npc:village_merchant", Title: "Village Merchant", EntryCount: 1}},
			SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{{Ref: "practice.village_wolf", Name: "Village Wolf", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: "practice_mob", RewardDropVnums: []uint32{27001}}},
			RewardDrops: []contentbundle.RewardDropAggregateSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 1, Stackable: true, MaxCount: 200}},
		},
	}
	mux := RegisterLocalContentBundleMapSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)
	mux = RegisterLocalContentBundleMapShopRoutesEndpoint(mux, summaryer.ExportContentBundleSummary)
	mux = RegisterLocalContentBundleMapWarpRoutesEndpoint(mux, summaryer.ExportContentBundleSummary)
	mux = RegisterLocalContentBundleMapSpawnGroupsEndpoint(mux, summaryer.ExportContentBundleSummary)
	mux = RegisterLocalContentBundleMapRewardDropsEndpoint(mux, summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/42/shop-routes", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for map-scoped shop routes alongside map summary route, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected map-scoped service route handler to call summary exporter once, got %d calls", summaryer.calls)
	}

	req = httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/42/spawn-groups", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec = httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for map-scoped spawn groups alongside map summary route, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 2 {
		t.Fatalf("expected map-scoped spawn-group handler to call summary exporter once, got total calls %d", summaryer.calls)
	}

	req = httptest.NewRequest(http.MethodGet, "/local/content-bundle/maps/42/reward-drops", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec = httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for map-scoped reward drops alongside map summary route, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 3 {
		t.Fatalf("expected map-scoped reward-drop handler to call summary exporter once, got total calls %d", summaryer.calls)
	}
}

func TestLocalContentBundleStaticActorEndpointReturnsMatchingActorsForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{StaticActors: []contentbundle.StaticActor{
			{Name: "Remote Guide", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:remote_guide"},
			{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide"},
			{Name: "Village Guide", MapIndex: 2, X: 1100, Y: 2100, RaceNum: 20303},
		}},
	}
	mux := RegisterLocalContentBundleStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/static-actors/Village%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.StaticActor
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode static actor response body: %v", err)
	}
	want := []contentbundle.StaticActor{
		{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide"},
		{Name: "Village Guide", MapIndex: 2, X: 1100, Y: 2100, RaceNum: 20303},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle static actor rows:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleStaticActorEndpointCoexistsWithContentBundleCollectionRoutes(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{}}
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{StaticActors: []contentbundle.StaticActor{{Name: "Village Guide", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 20302}}}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)
	mux = RegisterLocalContentBundleStaticActorEndpoint(mux, summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/static-actors/Village%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for static-actor summary alongside content-bundle routes, got %d", http.StatusOK, rec.Code)
	}
	if exporter.calls != 0 {
		t.Fatalf("expected collection exporter not to handle static-actor summary route, got %d calls", exporter.calls)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleStaticActorEndpointReturnsNotFoundForMissingActor(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{StaticActors: []contentbundle.StaticActor{{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302}}}}
	mux := RegisterLocalContentBundleStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/static-actors/Missing%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing static actor, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleStaticActorEndpointRejectsInvalidName(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{StaticActors: []contentbundle.StaticActor{{Name: "Village Guide"}}}}
	mux := RegisterLocalContentBundleStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/static-actors/", "/local/content-bundle/static-actors/Bad%2FGuide", "/local/content-bundle/static-actors/Village%20Guide/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid static-actor name path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid static-actor names, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleStaticActorEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{StaticActors: []contentbundle.StaticActor{{Name: "Village Guide"}}}}
	mux := RegisterLocalContentBundleStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/static-actors/Village%20Guide", nil)
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

func TestLocalContentBundleStaticActorEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{StaticActors: []contentbundle.StaticActor{{Name: "Village Guide"}}}}
	mux := RegisterLocalContentBundleStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/static-actors/Village%20Guide", nil)
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

func TestLocalContentBundleStaticActorEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/static-actors/Village%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSpawnGroupEndpointReturnsExactSpawnGroupForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{
			{Ref: "practice.alpha_mob", Name: "AlphaMob", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 101, CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob)},
			{Ref: "practice.reward_mob", Name: "RewardMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 102, CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy), RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}},
		}},
	}
	mux := RegisterLocalContentBundleSpawnGroupEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/spawn-groups/practice.reward_mob", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.SpawnGroupReferenceSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode spawn-group response body: %v", err)
	}
	want := contentbundle.SpawnGroupReferenceSummary{Ref: "practice.reward_mob", Name: "RewardMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 102, CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy), RewardExperience: 75, RewardGold: 60, RewardDropVnums: []uint32{27001}, RewardDropItems: []contentbundle.RewardDropItemSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle spawn-group summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleSpawnGroupEndpointCoexistsWithContentBundleCollectionRoutes(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{}}
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{{Ref: "practice.reward_mob", Name: "RewardMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 102, CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob)}}}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)
	mux = RegisterLocalContentBundleSpawnGroupEndpoint(mux, summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/spawn-groups/practice.reward_mob", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for spawn-group summary alongside content-bundle routes, got %d", http.StatusOK, rec.Code)
	}
	if exporter.calls != 0 {
		t.Fatalf("expected collection exporter not to handle spawn-group summary route, got %d calls", exporter.calls)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSpawnGroupEndpointReturnsNotFoundForMissingRef(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{{Ref: "practice.reward_mob", Name: "RewardMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 102, CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob)}}}}
	mux := RegisterLocalContentBundleSpawnGroupEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/spawn-groups/practice.missing_mob", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing spawn group, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSpawnGroupEndpointRejectsInvalidRef(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{{Ref: "practice.reward_mob", Name: "RewardMob"}}}}
	mux := RegisterLocalContentBundleSpawnGroupEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/spawn-groups/", "/local/content-bundle/spawn-groups/bad%20ref", "/local/content-bundle/spawn-groups/practice%2Freward_mob", "/local/content-bundle/spawn-groups/practice.reward_mob%20", "/local/content-bundle/spawn-groups/practice.reward_mob/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid spawn-group ref path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid spawn-group refs, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSpawnGroupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{{Ref: "practice.reward_mob", Name: "RewardMob"}}}}
	mux := RegisterLocalContentBundleSpawnGroupEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/spawn-groups/practice.reward_mob", nil)
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

func TestLocalContentBundleSpawnGroupEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{SpawnGroups: []contentbundle.SpawnGroupReferenceSummary{{Ref: "practice.reward_mob", Name: "RewardMob"}}}}
	mux := RegisterLocalContentBundleSpawnGroupEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/spawn-groups/practice.reward_mob", nil)
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

func TestLocalContentBundleSpawnGroupEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleSpawnGroupEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/spawn-groups/practice.reward_mob", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleCombatProfileEndpointReturnsExactProfileForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{
			{Profile: "practice_alpha_profile", MaxHP: 24, DamagePerNormalAttack: 3, AttackValue: 7, DefenseValue: 4, Level: 4, Rank: 1, RespawnDelayMs: 1500},
			{Profile: "practice_reward_profile", MaxHP: 30, DamagePerNormalAttack: 5, AttackValue: 9, DefenseValue: 4, Level: 6, Rank: 2, RespawnDelayMs: 2500, RetaliationPointDelta: -2, DeathReward: worldruntime.StaticActorDeathReward{Experience: 12, Gold: 7, DropVnums: []uint32{27001}}},
		}},
	}
	mux := RegisterLocalContentBundleCombatProfileEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/combat-profiles/practice_reward_profile", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got worldruntime.StaticActorCombatProfileSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode combat-profile response body: %v", err)
	}
	want := worldruntime.StaticActorCombatProfileSnapshot{Profile: "practice_reward_profile", MaxHP: 30, DamagePerNormalAttack: 5, AttackValue: 9, DefenseValue: 4, Level: 6, Rank: 2, RespawnDelayMs: 2500, RetaliationPointDelta: -2, DeathReward: worldruntime.StaticActorDeathReward{Experience: 12, Gold: 7, DropVnums: []uint32{27001}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle combat-profile summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleCombatProfileEndpointCoexistsWithContentBundleCollectionRoutes(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{}}
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{Profile: "practice_reward_profile", MaxHP: 30, DamagePerNormalAttack: 5, AttackValue: 9, DefenseValue: 4, RespawnDelayMs: 2500}}}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)
	mux = RegisterLocalContentBundleCombatProfileEndpoint(mux, summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/combat-profiles/practice_reward_profile", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for combat-profile summary alongside content-bundle routes, got %d", http.StatusOK, rec.Code)
	}
	if exporter.calls != 0 {
		t.Fatalf("expected collection exporter not to handle combat-profile summary route, got %d calls", exporter.calls)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleCombatProfileEndpointReturnsNotFoundForMissingProfile(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{Profile: "practice_reward_profile", MaxHP: 30, DamagePerNormalAttack: 5, AttackValue: 9, DefenseValue: 4, RespawnDelayMs: 2500}}}}
	mux := RegisterLocalContentBundleCombatProfileEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/combat-profiles/practice_missing_profile", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing combat profile, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleCombatProfileEndpointRejectsInvalidProfile(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{Profile: "practice_reward_profile"}}}}
	mux := RegisterLocalContentBundleCombatProfileEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/combat-profiles/", "/local/content-bundle/combat-profiles/bad%20profile", "/local/content-bundle/combat-profiles/practice%2Freward_profile", "/local/content-bundle/combat-profiles/practice.reward_profile", "/local/content-bundle/combat-profiles/practice_reward_profile%20", "/local/content-bundle/combat-profiles/practice_reward_profile/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid combat-profile path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid combat-profile names, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleCombatProfileEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{Profile: "practice_reward_profile"}}}}
	mux := RegisterLocalContentBundleCombatProfileEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/combat-profiles/practice_reward_profile", nil)
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

func TestLocalContentBundleCombatProfileEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{Profile: "practice_reward_profile"}}}}
	mux := RegisterLocalContentBundleCombatProfileEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/combat-profiles/practice_reward_profile", nil)
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

func TestLocalContentBundleCombatProfileEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleCombatProfileEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/combat-profiles/practice_reward_profile", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractableStaticActorEndpointReturnsMatchingActorsForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{InteractableStaticActors: []contentbundle.InteractableStaticActorSummary{
			{Name: "Remote Guide", MapIndex: 7, X: 1300, Y: 2300, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:remote_guide", Preview: "Remote Guide:\nRemote route."},
			{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide", Preview: "Village Guide:\nWelcome."},
			{Name: "Village Guide", MapIndex: 2, X: 1100, Y: 2100, RaceNum: 20303, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:village_guide", Preview: "Second placement."},
		}},
	}
	mux := RegisterLocalContentBundleInteractableStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interactable-static-actors/Village%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.InteractableStaticActorSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode interactable static actor response body: %v", err)
	}
	want := []contentbundle.InteractableStaticActorSummary{
		{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide", Preview: "Village Guide:\nWelcome."},
		{Name: "Village Guide", MapIndex: 2, X: 1100, Y: 2100, RaceNum: 20303, InteractionKind: interactionstore.KindInfo, InteractionRef: "lore:village_guide", Preview: "Second placement."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle interactable static actor rows:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleInteractableStaticActorEndpointReturnsNotFoundForMissingActor(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractableStaticActors: []contentbundle.InteractableStaticActorSummary{{Name: "Village Guide", MapIndex: 1, X: 1000, Y: 2000, RaceNum: 20302, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guide"}}}}
	mux := RegisterLocalContentBundleInteractableStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interactable-static-actors/Missing%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing interactable static actor, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractableStaticActorEndpointRejectsInvalidName(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractableStaticActors: []contentbundle.InteractableStaticActorSummary{{Name: "Village Guide"}}}}
	mux := RegisterLocalContentBundleInteractableStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/interactable-static-actors/", "/local/content-bundle/interactable-static-actors/Village%2FGuide", "/local/content-bundle/interactable-static-actors/Village%20Guide/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid interactable static actor path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid actor names, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractableStaticActorEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractableStaticActors: []contentbundle.InteractableStaticActorSummary{{Name: "Village Guide"}}}}
	mux := RegisterLocalContentBundleInteractableStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interactable-static-actors/Village%20Guide", nil)
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

func TestLocalContentBundleInteractableStaticActorEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractableStaticActors: []contentbundle.InteractableStaticActorSummary{{Name: "Village Guide"}}}}
	mux := RegisterLocalContentBundleInteractableStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/interactable-static-actors/Village%20Guide", nil)
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

func TestLocalContentBundleInteractableStaticActorEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleInteractableStaticActorEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interactable-static-actors/Village%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractionKindEndpointReturnsExactKindSummaryForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{InteractionKinds: []contentbundle.InteractionKindSummary{
			{Kind: interactionstore.KindInfo, Count: 1, ReferencedCount: 0, UnreferencedCount: 1},
			{Kind: interactionstore.KindTalk, Count: 2, ReferencedCount: 1, UnreferencedCount: 1},
		}},
	}
	mux := RegisterLocalContentBundleInteractionKindEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-kinds/talk", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.InteractionKindSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode interaction kind response body: %v", err)
	}
	want := contentbundle.InteractionKindSummary{Kind: interactionstore.KindTalk, Count: 2, ReferencedCount: 1, UnreferencedCount: 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle interaction kind summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleInteractionKindEndpointReturnsNotFoundForMissingKind(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractionKinds: []contentbundle.InteractionKindSummary{{Kind: interactionstore.KindTalk, Count: 1, ReferencedCount: 1}}}}
	mux := RegisterLocalContentBundleInteractionKindEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-kinds/info", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing interaction kind, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractionKindEndpointRejectsInvalidKind(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractionKinds: []contentbundle.InteractionKindSummary{{Kind: interactionstore.KindTalk, Count: 1}}}}
	mux := RegisterLocalContentBundleInteractionKindEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/interaction-kinds/", "/local/content-bundle/interaction-kinds/quest", "/local/content-bundle/interaction-kinds/talk/extra", "/local/content-bundle/interaction-kinds/bad%2Fkind"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid interaction kind path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid interaction kind identities, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractionKindEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractionKinds: []contentbundle.InteractionKindSummary{{Kind: interactionstore.KindTalk, Count: 1}}}}
	mux := RegisterLocalContentBundleInteractionKindEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-kinds/talk", nil)
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

func TestLocalContentBundleInteractionKindEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractionKinds: []contentbundle.InteractionKindSummary{{Kind: interactionstore.KindTalk, Count: 1}}}}
	mux := RegisterLocalContentBundleInteractionKindEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/interaction-kinds/talk", nil)
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

func TestLocalContentBundleInteractionKindEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleInteractionKindEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-kinds/talk", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractionDefinitionEndpointReturnsExactReferencedDefinitionForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			InteractionDefinitionPreviews: []contentbundle.InteractionDefinitionPreviewSummary{
				{Kind: interactionstore.KindInfo, Ref: "lore:village", Preview: "The village is quiet."},
				{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Welcome."},
			},
			ReferencedInteractionDefinitions: []contentbundle.InteractionDefinitionReferenceSummary{{Kind: interactionstore.KindTalk, Ref: "npc:guide"}},
		},
	}
	mux := RegisterLocalContentBundleInteractionDefinitionEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-definitions/talk/npc:guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got localContentBundleInteractionDefinitionSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode interaction definition response body: %v", err)
	}
	want := localContentBundleInteractionDefinitionSummary{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Welcome.", Referenced: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle interaction definition summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleInteractionDefinitionEndpointReturnsUnreferencedDefinitionForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			InteractionDefinitionPreviews:      []contentbundle.InteractionDefinitionPreviewSummary{{Kind: interactionstore.KindInfo, Ref: "lore:village", Preview: "The village is quiet."}},
			UnreferencedInteractionDefinitions: []contentbundle.InteractionDefinitionReferenceSummary{{Kind: interactionstore.KindInfo, Ref: "lore:village"}},
		},
	}
	mux := RegisterLocalContentBundleInteractionDefinitionEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-definitions/info/lore:village", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got localContentBundleInteractionDefinitionSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode unreferenced interaction definition response body: %v", err)
	}
	want := localContentBundleInteractionDefinitionSummary{Kind: interactionstore.KindInfo, Ref: "lore:village", Preview: "The village is quiet.", Referenced: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected unreferenced content-bundle interaction definition summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleInteractionDefinitionEndpointReturnsNotFoundForMissingDefinition(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractionDefinitionPreviews: []contentbundle.InteractionDefinitionPreviewSummary{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Welcome."}}}}
	mux := RegisterLocalContentBundleInteractionDefinitionEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-definitions/talk/npc:missing", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing interaction definition, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractionDefinitionEndpointRejectsInvalidIdentity(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractionDefinitionPreviews: []contentbundle.InteractionDefinitionPreviewSummary{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Welcome."}}}}
	mux := RegisterLocalContentBundleInteractionDefinitionEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/interaction-definitions/talk", "/local/content-bundle/interaction-definitions/quest/npc:first_steps", "/local/content-bundle/interaction-definitions/talk/npc%2Fguide", "/local/content-bundle/interaction-definitions/talk/npc:guide/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid interaction definition path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid interaction definition identities, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleInteractionDefinitionEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractionDefinitionPreviews: []contentbundle.InteractionDefinitionPreviewSummary{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Welcome."}}}}
	mux := RegisterLocalContentBundleInteractionDefinitionEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-definitions/talk/npc:guide", nil)
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

func TestLocalContentBundleInteractionDefinitionEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{InteractionDefinitionPreviews: []contentbundle.InteractionDefinitionPreviewSummary{{Kind: interactionstore.KindTalk, Ref: "npc:guide", Preview: "Welcome."}}}}
	mux := RegisterLocalContentBundleInteractionDefinitionEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/interaction-definitions/talk/npc:guide", nil)
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

func TestLocalContentBundleInteractionDefinitionEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleInteractionDefinitionEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/interaction-definitions/talk/npc:guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleShopCatalogEndpointReturnsExactCatalogForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{ShopCatalogs: []contentbundle.ShopCatalogSummary{
			{Kind: interactionstore.KindShopPreview, Ref: "npc:alchemist", Title: "Alchemist", EntryCount: 1, Entries: []contentbundle.ShopCatalogEntrySummary{{Slot: 0, ItemVnum: 27002, ItemName: "Small Blue Potion", Count: 1, Price: 75, Stackable: true, MaxCount: 200}}},
			{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 2, Entries: []contentbundle.ShopCatalogEntrySummary{{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200}, {Slot: 1, ItemVnum: 11200, ItemName: "Wooden Sword", Count: 1, Price: 500, Stackable: false, MaxCount: 1}}},
		}},
	}
	mux := RegisterLocalContentBundleShopCatalogEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/shop-catalogs/shop_preview/npc:merchant", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.ShopCatalogSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode shop catalog response body: %v", err)
	}
	want := contentbundle.ShopCatalogSummary{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 2, Entries: []contentbundle.ShopCatalogEntrySummary{{Slot: 0, ItemVnum: 27001, ItemName: "Small Red Potion", Count: 1, Price: 50, Stackable: true, MaxCount: 200}, {Slot: 1, ItemVnum: 11200, ItemName: "Wooden Sword", Count: 1, Price: 500, Stackable: false, MaxCount: 1}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle shop catalog:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleShopCatalogEndpointReturnsNotFoundForMissingCatalog(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ShopCatalogs: []contentbundle.ShopCatalogSummary{{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 1}}}}
	mux := RegisterLocalContentBundleShopCatalogEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/shop-catalogs/shop_preview/npc:missing", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing shop catalog, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleShopCatalogEndpointRejectsInvalidIdentity(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ShopCatalogs: []contentbundle.ShopCatalogSummary{{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 1}}}}
	mux := RegisterLocalContentBundleShopCatalogEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/shop-catalogs/shop_preview", "/local/content-bundle/shop-catalogs/quest/npc:first_steps", "/local/content-bundle/shop-catalogs/shop_preview/npc%2Fmerchant", "/local/content-bundle/shop-catalogs/shop_preview/npc:merchant/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid shop catalog path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid shop catalog identity, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleShopCatalogEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ShopCatalogs: []contentbundle.ShopCatalogSummary{{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 1}}}}
	mux := RegisterLocalContentBundleShopCatalogEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/shop-catalogs/shop_preview/npc:merchant", nil)
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

func TestLocalContentBundleShopCatalogEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ShopCatalogs: []contentbundle.ShopCatalogSummary{{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", EntryCount: 1}}}}
	mux := RegisterLocalContentBundleShopCatalogEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/shop-catalogs/shop_preview/npc:merchant", nil)
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

func TestLocalContentBundleShopRouteEndpointReturnsMatchingRoutesForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{ShopRoutes: []contentbundle.ShopRouteSummary{
			{ActorName: "Remote Merchant", SourceMapIndex: 7, SourceX: 1300, SourceY: 2300, Ref: "npc:remote_merchant", Title: "Remote Merchant", EntryCount: 1},
			{ActorName: "Village Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_merchant", Title: "Village Merchant", EntryCount: 2},
			{ActorName: "Village Merchant", SourceMapIndex: 2, SourceX: 1100, SourceY: 2100, Ref: "npc:village_merchant_branch", Title: "Branch Merchant", EntryCount: 3},
		}},
	}
	mux := RegisterLocalContentBundleShopRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/shop-routes/Village%20Merchant", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.ShopRouteSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode shop route response body: %v", err)
	}
	want := []contentbundle.ShopRouteSummary{
		{ActorName: "Village Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_merchant", Title: "Village Merchant", EntryCount: 2},
		{ActorName: "Village Merchant", SourceMapIndex: 2, SourceX: 1100, SourceY: 2100, Ref: "npc:village_merchant_branch", Title: "Branch Merchant", EntryCount: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle shop route rows:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleShopRouteEndpointReturnsNotFoundForMissingRoute(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ShopRoutes: []contentbundle.ShopRouteSummary{{ActorName: "Village Merchant", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_merchant", Title: "Village Merchant", EntryCount: 2}}}}
	mux := RegisterLocalContentBundleShopRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/shop-routes/Missing%20Merchant", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing shop route, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleShopRouteEndpointRejectsInvalidActorName(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ShopRoutes: []contentbundle.ShopRouteSummary{{ActorName: "Village Merchant"}}}}
	mux := RegisterLocalContentBundleShopRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/shop-routes/", "/local/content-bundle/shop-routes/Village%2FMerchant", "/local/content-bundle/shop-routes/Village%20Merchant/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid shop route path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid shop route actor names, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleShopRouteEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ShopRoutes: []contentbundle.ShopRouteSummary{{ActorName: "Village Merchant"}}}}
	mux := RegisterLocalContentBundleShopRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/shop-routes/Village%20Merchant", nil)
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

func TestLocalContentBundleShopRouteEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ShopRoutes: []contentbundle.ShopRouteSummary{{ActorName: "Village Merchant"}}}}
	mux := RegisterLocalContentBundleShopRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/shop-routes/Village%20Merchant", nil)
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

func TestLocalContentBundleQuestFlagRouteEndpointReturnsMatchingRoutesForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{QuestFlagRoutes: []contentbundle.QuestFlagRouteSummary{
			{ActorName: "Remote Guide", SourceMapIndex: 7, SourceX: 1300, SourceY: 2300, Ref: "quest:remote_steps", Text: "Remote quest acknowledgement.", QuestRef: "quest:remote_steps", QuestFlag: "met_remote", QuestTo: 1},
			{ActorName: "Quest Guide", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1},
			{ActorName: "Quest Guide", SourceMapIndex: 2, SourceX: 1100, SourceY: 2100, Ref: "quest:first_steps_reset", Text: "Quest cleared.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1},
		}},
	}
	mux := RegisterLocalContentBundleQuestFlagRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-flag-routes/Quest%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.QuestFlagRouteSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest-flag route response body: %v", err)
	}
	want := []contentbundle.QuestFlagRouteSummary{
		{ActorName: "Quest Guide", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1},
		{ActorName: "Quest Guide", SourceMapIndex: 2, SourceX: 1100, SourceY: 2100, Ref: "quest:first_steps_reset", Text: "Quest cleared.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestFrom: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle quest-flag route rows:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestFlagRouteEndpointReturnsNotFoundForMissingRoute(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{QuestFlagRoutes: []contentbundle.QuestFlagRouteSummary{{ActorName: "Quest Guide", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "quest:first_steps", Text: "Quest updated.", QuestRef: "quest:first_steps", QuestFlag: "met_guide", QuestTo: 1}}}}
	mux := RegisterLocalContentBundleQuestFlagRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-flag-routes/Missing%20Guide", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing quest-flag route, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestFlagRouteEndpointRejectsInvalidActorName(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{QuestFlagRoutes: []contentbundle.QuestFlagRouteSummary{{ActorName: "Quest Guide"}}}}
	mux := RegisterLocalContentBundleQuestFlagRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/quest-flag-routes/", "/local/content-bundle/quest-flag-routes/Quest%2FGuide", "/local/content-bundle/quest-flag-routes/Quest%20Guide/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid quest-flag route path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid quest-flag route actor names, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestFlagRouteEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{QuestFlagRoutes: []contentbundle.QuestFlagRouteSummary{{ActorName: "Quest Guide"}}}}
	mux := RegisterLocalContentBundleQuestFlagRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-flag-routes/Quest%20Guide", nil)
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

func TestLocalContentBundleQuestFlagRouteEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{QuestFlagRoutes: []contentbundle.QuestFlagRouteSummary{{ActorName: "Quest Guide"}}}}
	mux := RegisterLocalContentBundleQuestFlagRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/quest-flag-routes/Quest%20Guide", nil)
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

func TestLocalContentBundleWarpDestinationEndpointReturnsExactDestinationForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{WarpDestinations: []contentbundle.WarpDestinationSummary{
			{Kind: interactionstore.KindWarp, Ref: "npc:gate", Text: "Old gate.", MapIndex: 2, X: 2000, Y: 3000},
			{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800},
		}},
	}
	mux := RegisterLocalContentBundleWarpDestinationEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/warp-destinations/warp/npc:teleporter", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.WarpDestinationSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode warp destination response body: %v", err)
	}
	want := contentbundle.WarpDestinationSummary{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", Text: "Step through the gate.", MapIndex: 42, X: 1700, Y: 2800}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle warp destination:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleWarpDestinationEndpointReturnsNotFoundForMissingDestination(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{WarpDestinations: []contentbundle.WarpDestinationSummary{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800}}}}
	mux := RegisterLocalContentBundleWarpDestinationEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/warp-destinations/warp/npc:missing", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing warp destination, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleWarpDestinationEndpointRejectsInvalidIdentity(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{WarpDestinations: []contentbundle.WarpDestinationSummary{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800}}}}
	mux := RegisterLocalContentBundleWarpDestinationEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/warp-destinations/warp", "/local/content-bundle/warp-destinations/quest/npc:first_steps", "/local/content-bundle/warp-destinations/warp/npc%2Fteleporter", "/local/content-bundle/warp-destinations/warp/npc:teleporter/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid warp destination path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid warp destination identity, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleWarpDestinationEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{WarpDestinations: []contentbundle.WarpDestinationSummary{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800}}}}
	mux := RegisterLocalContentBundleWarpDestinationEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/warp-destinations/warp/npc:teleporter", nil)
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

func TestLocalContentBundleWarpDestinationEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{WarpDestinations: []contentbundle.WarpDestinationSummary{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800}}}}
	mux := RegisterLocalContentBundleWarpDestinationEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/warp-destinations/warp/npc:teleporter", nil)
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

func TestLocalContentBundleWarpRouteEndpointReturnsMatchingRoutesForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{WarpRoutes: []contentbundle.WarpRouteSummary{
			{ActorName: "Remote Gate", SourceMapIndex: 7, SourceX: 1300, SourceY: 2300, Ref: "npc:remote_gate", Text: "Remote gate.", TargetMapIndex: 8, TargetX: 1400, TargetY: 2400},
			{ActorName: "Village Gate", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_gate", Text: "Step through the gate.", TargetMapIndex: 42, TargetX: 1700, TargetY: 2800},
			{ActorName: "Village Gate", SourceMapIndex: 2, SourceX: 1100, SourceY: 2100, Ref: "npc:village_gate_branch", TargetMapIndex: 43, TargetX: 1800, TargetY: 2900},
		}},
	}
	mux := RegisterLocalContentBundleWarpRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/warp-routes/Village%20Gate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got []contentbundle.WarpRouteSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode warp route response body: %v", err)
	}
	want := []contentbundle.WarpRouteSummary{
		{ActorName: "Village Gate", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_gate", Text: "Step through the gate.", TargetMapIndex: 42, TargetX: 1700, TargetY: 2800},
		{ActorName: "Village Gate", SourceMapIndex: 2, SourceX: 1100, SourceY: 2100, Ref: "npc:village_gate_branch", TargetMapIndex: 43, TargetX: 1800, TargetY: 2900},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle warp route rows:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleWarpRouteEndpointReturnsNotFoundForMissingRoute(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{WarpRoutes: []contentbundle.WarpRouteSummary{{ActorName: "Village Gate", SourceMapIndex: 1, SourceX: 1000, SourceY: 2000, Ref: "npc:village_gate", TargetMapIndex: 42, TargetX: 1700, TargetY: 2800}}}}
	mux := RegisterLocalContentBundleWarpRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/warp-routes/Missing%20Gate", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing warp route, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleWarpRouteEndpointRejectsInvalidActorName(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{WarpRoutes: []contentbundle.WarpRouteSummary{{ActorName: "Village Gate"}}}}
	mux := RegisterLocalContentBundleWarpRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/warp-routes/", "/local/content-bundle/warp-routes/Village%2FGate", "/local/content-bundle/warp-routes/Village%20Gate/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid warp route path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid warp route actor names, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleWarpRouteEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{WarpRoutes: []contentbundle.WarpRouteSummary{{ActorName: "Village Gate"}}}}
	mux := RegisterLocalContentBundleWarpRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/warp-routes/Village%20Gate", nil)
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

func TestLocalContentBundleWarpRouteEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{WarpRoutes: []contentbundle.WarpRouteSummary{{ActorName: "Village Gate"}}}}
	mux := RegisterLocalContentBundleWarpRouteEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/warp-routes/Village%20Gate", nil)
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

func TestLocalContentBundleItemTemplateEndpointReturnsExactTemplateForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{ItemTemplates: []contentbundle.ItemTemplateReferenceSummary{
			{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5, ShopSellPrice: 2, AntiGet: true, UseRejectMessage: "This quest-sealed potion cannot be used yet.", PickupRange: 750},
		}},
	}
	mux := RegisterLocalContentBundleItemTemplateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/item-templates/27001", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.ItemTemplateReferenceSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode item-template response body: %v", err)
	}
	want := contentbundle.ItemTemplateReferenceSummary{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5, ShopSellPrice: 2, AntiGet: true, UseRejectMessage: "This quest-sealed potion cannot be used yet.", PickupRange: 750}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle item-template summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleItemTemplateEndpointCoexistsWithContentBundleCollectionRoutes(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{}}
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ItemTemplates: []contentbundle.ItemTemplateReferenceSummary{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)
	mux = RegisterLocalContentBundleItemTemplateEndpoint(mux, summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/item-templates/27001", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for item-template summary alongside content-bundle routes, got %d", http.StatusOK, rec.Code)
	}
	if exporter.calls != 0 {
		t.Fatalf("expected collection exporter not to handle item-template summary route, got %d calls", exporter.calls)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleItemTemplateEndpointReturnsNotFoundForMissingVnum(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ItemTemplates: []contentbundle.ItemTemplateReferenceSummary{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}}}}
	mux := RegisterLocalContentBundleItemTemplateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/item-templates/11200", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing item template, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleItemTemplateEndpointRejectsInvalidVnum(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ItemTemplates: []contentbundle.ItemTemplateReferenceSummary{{Vnum: 27001, Name: "Small Red Potion"}}}}
	mux := RegisterLocalContentBundleItemTemplateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/item-templates/", "/local/content-bundle/item-templates/0", "/local/content-bundle/item-templates/not-a-vnum", "/local/content-bundle/item-templates/27001/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid item-template path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid item-template vnums, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleItemTemplateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ItemTemplates: []contentbundle.ItemTemplateReferenceSummary{{Vnum: 27001, Name: "Small Red Potion"}}}}
	mux := RegisterLocalContentBundleItemTemplateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/item-templates/27001", nil)
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

func TestLocalContentBundleItemTemplateEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{ItemTemplates: []contentbundle.ItemTemplateReferenceSummary{{Vnum: 27001, Name: "Small Red Potion"}}}}
	mux := RegisterLocalContentBundleItemTemplateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/item-templates/27001", nil)
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

func TestLocalContentBundleItemTemplateEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleItemTemplateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/item-templates/27001", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleRewardDropEndpointReturnsExactRewardDropForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{RewardDrops: []contentbundle.RewardDropAggregateSummary{
			{ItemVnum: 27002, ItemName: "Small Blue Potion", SourceCount: 1, Stackable: true, MaxCount: 200, ShopBuyPrice: 7},
			{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 2, Stackable: true, MaxCount: 200, ShopBuyPrice: 5, ShopSellPrice: 2, AntiDrop: true, PickupRange: 750},
		}},
	}
	mux := RegisterLocalContentBundleRewardDropEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/reward-drops/27001", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.RewardDropAggregateSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode reward-drop response body: %v", err)
	}
	want := contentbundle.RewardDropAggregateSummary{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 2, Stackable: true, MaxCount: 200, ShopBuyPrice: 5, ShopSellPrice: 2, AntiDrop: true, PickupRange: 750}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle reward-drop summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleRewardDropEndpointCoexistsWithContentBundleCollectionRoutes(t *testing.T) {
	exporter := &stubContentBundleExporter{status: http.StatusOK, bundle: contentbundle.Bundle{}}
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{RewardDrops: []contentbundle.RewardDropAggregateSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 1, Stackable: true, MaxCount: 200}}}}
	mux := RegisterLocalContentBundleEndpoint(NewPprofMux("gamed"), exporter.ExportContentBundle, nil)
	mux = RegisterLocalContentBundleRewardDropEndpoint(mux, summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/reward-drops/27001", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for reward-drop summary alongside content-bundle routes, got %d", http.StatusOK, rec.Code)
	}
	if exporter.calls != 0 {
		t.Fatalf("expected collection exporter not to handle reward-drop summary route, got %d calls", exporter.calls)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleRewardDropEndpointReturnsNotFoundForMissingVnum(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{RewardDrops: []contentbundle.RewardDropAggregateSummary{{ItemVnum: 27001, ItemName: "Small Red Potion", SourceCount: 1, Stackable: true, MaxCount: 200}}}}
	mux := RegisterLocalContentBundleRewardDropEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/reward-drops/11200", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing reward drop, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleRewardDropEndpointRejectsInvalidVnum(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{RewardDrops: []contentbundle.RewardDropAggregateSummary{{ItemVnum: 27001, ItemName: "Small Red Potion"}}}}
	mux := RegisterLocalContentBundleRewardDropEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/reward-drops/", "/local/content-bundle/reward-drops/0", "/local/content-bundle/reward-drops/not-a-vnum", "/local/content-bundle/reward-drops/27001/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid reward-drop path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected content bundle summary exporter not to be called for invalid reward-drop vnums, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleRewardDropEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{RewardDrops: []contentbundle.RewardDropAggregateSummary{{ItemVnum: 27001, ItemName: "Small Red Potion"}}}}
	mux := RegisterLocalContentBundleRewardDropEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/reward-drops/27001", nil)
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

func TestLocalContentBundleRewardDropEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK, summary: contentbundle.Summary{RewardDrops: []contentbundle.RewardDropAggregateSummary{{ItemVnum: 27001, ItemName: "Small Red Potion"}}}}
	mux := RegisterLocalContentBundleRewardDropEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/reward-drops/27001", nil)
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

func TestLocalContentBundleRewardDropEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleRewardDropEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/reward-drops/27001", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateEndpointReturnsOverviewForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{
			QuestStateFlagCount:      2,
			QuestStateCharacterCount: 2,
			QuestStateQuestCount:     1,
			QuestStateQuestRefs:      []string{"quest:first_steps"},
			QuestStateCharacters: []contentbundle.QuestStateCharacterSummary{
				{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
				{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
			},
			QuestStateQuests: []contentbundle.QuestStateQuestSummary{
				{QuestRef: "quest:first_steps", FlagCount: 2, Characters: []contentbundle.QuestStateCharacterSummary{
					{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
					{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
				}},
			},
		},
	}
	mux := RegisterLocalContentBundleQuestStateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.QuestStateOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode content-bundle quest-state overview response body: %v", err)
	}
	want := contentbundle.QuestStateOverview{
		FlagCount:      2,
		CharacterCount: 2,
		QuestCount:     1,
		QuestRefs:      []string{"quest:first_steps"},
		Characters: []contentbundle.QuestStateCharacterSummary{
			{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
			{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
		},
		Quests: []contentbundle.QuestStateQuestSummary{
			{QuestRef: "quest:first_steps", FlagCount: 2, Characters: []contentbundle.QuestStateCharacterSummary{
				{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
				{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
			}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle quest-state overview:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestStateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback caller, got %d", http.StatusForbidden, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected wrong method not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleQuestStateEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateCharacterEndpointReturnsMatchingCharacterForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{QuestStateCharacters: []contentbundle.QuestStateCharacterSummary{
			{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
			{Character: "QuestHero", FlagCount: 2, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}, {QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
		}},
	}
	mux := RegisterLocalContentBundleQuestStateCharacterEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/characters/QuestHero", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.QuestStateCharacterSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest-state character summary response body: %v", err)
	}
	want := contentbundle.QuestStateCharacterSummary{Character: "QuestHero", FlagCount: 2, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}, {QuestRef: "quest:first_steps", Name: "step", Value: 2}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle quest-state character summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestStateCharacterEndpointReturnsNotFoundForMissingCharacter(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status:  http.StatusOK,
		summary: contentbundle.Summary{QuestStateCharacters: []contentbundle.QuestStateCharacterSummary{{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}}}},
	}
	mux := RegisterLocalContentBundleQuestStateCharacterEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/characters/MissingHero", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing quest-state character, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateCharacterEndpointRejectsAmbiguousCharacterName(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateCharacterEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/characters/Bad%2FName", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for ambiguous character path, got %d", http.StatusBadRequest, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected malformed path not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateCharacterEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateCharacterEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/characters/QuestHero", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback caller, got %d", http.StatusForbidden, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateCharacterEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateCharacterEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/quest-state/characters/QuestHero", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected wrong method not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateCharacterEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleQuestStateCharacterEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/characters/QuestHero", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagEndpointReturnsExactFlagForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{QuestStateCharacters: []contentbundle.QuestStateCharacterSummary{
			{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
			{Character: "QuestHero", FlagCount: 2, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:daily_check", Name: "talked_to_guide", Value: 1}, {QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
		}},
	}
	mux := RegisterLocalContentBundleQuestStateFlagEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got queststate.Flag
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest-state flag summary response body: %v", err)
	}
	want := queststate.Flag{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle quest-state flag summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestStateFlagEndpointReturnsNotFoundForMissingFlag(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status:  http.StatusOK,
		summary: contentbundle.Summary{QuestStateCharacters: []contentbundle.QuestStateCharacterSummary{{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}}}},
	}
	mux := RegisterLocalContentBundleQuestStateFlagEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps/missing_flag", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing quest-state flag, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagEndpointRejectsInvalidIdentity(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateFlagEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{
		"/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps",
		"/local/content-bundle/quest-state/flags/Bad%2FName/quest:first_steps/step",
		"/local/content-bundle/quest-state/flags/QuestHero/first_steps/step",
		"/local/content-bundle/quest-state/flags/QuestHero/quest%2Ffirst_steps/step",
		"/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps/BadFlag",
		"/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps/step/extra",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid quest-state flag path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected malformed quest-state flag identities not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateFlagEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback caller, got %d", http.StatusForbidden, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateFlagEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected wrong method not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateFlagEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleQuestStateFlagEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps/step", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateQuestEndpointReturnsMatchingQuestForLoopbackGet(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status: http.StatusOK,
		summary: contentbundle.Summary{QuestStateQuests: []contentbundle.QuestStateQuestSummary{
			{QuestRef: "quest:daily_check", FlagCount: 1, Characters: []contentbundle.QuestStateCharacterSummary{{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:daily_check", Name: "talked_to_guide", Value: 1}}}}},
			{QuestRef: "quest:first_steps", FlagCount: 2, Characters: []contentbundle.QuestStateCharacterSummary{
				{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
				{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
			}},
		}},
	}
	mux := RegisterLocalContentBundleQuestStateQuestEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/quests/quest:first_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
	var got contentbundle.QuestStateQuestSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode quest-state quest summary response body: %v", err)
	}
	want := contentbundle.QuestStateQuestSummary{
		QuestRef:  "quest:first_steps",
		FlagCount: 2,
		Characters: []contentbundle.QuestStateCharacterSummary{
			{Character: "AnotherHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "met_guard", Value: 1}}},
			{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected content-bundle quest-state quest summary:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLocalContentBundleQuestStateQuestEndpointReturnsNotFoundForMissingQuest(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{
		status:  http.StatusOK,
		summary: contentbundle.Summary{QuestStateQuests: []contentbundle.QuestStateQuestSummary{{QuestRef: "quest:first_steps", FlagCount: 1, Characters: []contentbundle.QuestStateCharacterSummary{{Character: "QuestHero", FlagCount: 1, Flags: []queststate.FlagSnapshot{{QuestRef: "quest:first_steps", Name: "step", Value: 2}}}}}}},
	}
	mux := RegisterLocalContentBundleQuestStateQuestEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/quests/quest:missing_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for missing quest-state quest, got %d", http.StatusNotFound, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateQuestEndpointRejectsInvalidQuestRef(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateQuestEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	for _, path := range []string{"/local/content-bundle/quest-state/quests/", "/local/content-bundle/quest-state/quests/first_steps", "/local/content-bundle/quest-state/quests/quest%2Ffirst_steps", "/local/content-bundle/quest-state/quests/quest:first_steps/extra"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for invalid quest ref path %q, got %d", http.StatusBadRequest, path, rec.Code)
		}
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected malformed quest refs not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateQuestEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateQuestEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/quests/quest:first_steps", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for non-loopback caller, got %d", http.StatusForbidden, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected non-loopback request not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateQuestEndpointRejectsWrongMethod(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleQuestStateQuestEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodPost, "/local/content-bundle/quest-state/quests/quest:first_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for wrong method, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if summaryer.calls != 0 {
		t.Fatalf("expected wrong method not to call content bundle summary exporter, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleQuestStateQuestEndpointForwardsSummaryExporterErrors(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusConflict, result: map[string]string{"error": "content summary unavailable"}}
	mux := RegisterLocalContentBundleQuestStateQuestEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	req := httptest.NewRequest(http.MethodGet, "/local/content-bundle/quest-state/quests/quest:first_steps", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d for exporter failure, got %d", http.StatusConflict, rec.Code)
	}
	if summaryer.calls != 1 {
		t.Fatalf("expected content bundle summary exporter to be called once, got %d calls", summaryer.calls)
	}
}

func TestLocalContentBundleSummaryEndpointReturnsPickupRangeMetadataForLoopbackPost(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"spawn_groups":[{"ref":"practice.long_reach_reward","name":"Long Reach Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}],"item_templates":[{"vnum":27001,"name":"Long Reach Potion","stackable":true,"max_count":200,"shop_buy_price":5,"pickup_range":750}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:long_reach_merchant","title":"Long Reach Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":2}]}]}`
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
		t.Fatalf("decode pickup-range summary response body: %v", err)
	}
	if got.ItemTemplates[0].PickupRange != 750 {
		t.Fatalf("expected item-template pickup_range 750, got %+v", got.ItemTemplates)
	}
	if got.ShopCatalogs[0].Entries[0].PickupRange != 750 {
		t.Fatalf("expected shop-catalog pickup_range 750, got %+v", got.ShopCatalogs)
	}
	if got.SpawnGroups[0].RewardDropItems[0].PickupRange != 750 {
		t.Fatalf("expected spawn reward-drop pickup_range 750, got %+v", got.SpawnGroups)
	}
	if got.RewardDrops[0].PickupRange != 750 {
		t.Fatalf("expected aggregate reward-drop pickup_range 750, got %+v", got.RewardDrops)
	}
}

func TestLocalContentBundleSummaryEndpointReturnsShopSellPriceMetadataForLoopbackPost(t *testing.T) {
	const shopSellPrice uint64 = 13
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"spawn_groups":[{"ref":"practice.sell_price_reward","name":"Sell Price Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}],"item_templates":[{"vnum":27001,"name":"Sell Price Potion","stackable":true,"max_count":200,"shop_buy_price":5,"shop_sell_price":13}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:sell_price_merchant","title":"Sell Price Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":2}]}]}`
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
		t.Fatalf("decode shop-sell-price summary response body: %v", err)
	}
	if got.ItemTemplates[0].ShopSellPrice != shopSellPrice {
		t.Fatalf("expected item-template shop_sell_price %d, got %+v", shopSellPrice, got.ItemTemplates)
	}
	if got.ShopCatalogs[0].Entries[0].ShopSellPrice != shopSellPrice {
		t.Fatalf("expected shop-catalog shop_sell_price %d, got %+v", shopSellPrice, got.ShopCatalogs)
	}
	if got.SpawnGroups[0].RewardDropItems[0].ShopSellPrice != shopSellPrice {
		t.Fatalf("expected spawn reward-drop shop_sell_price %d, got %+v", shopSellPrice, got.SpawnGroups)
	}
	if got.RewardDrops[0].ShopSellPrice != shopSellPrice {
		t.Fatalf("expected aggregate reward-drop shop_sell_price %d, got %+v", shopSellPrice, got.RewardDrops)
	}
}

func TestLocalContentBundleSummaryEndpointReturnsMerchantAntiFlagMetadataForLoopbackPost(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"spawn_groups":[{"ref":"practice.guarded_reward","name":"Guarded Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}],"item_templates":[{"vnum":27001,"name":"Guarded Potion","stackable":true,"max_count":200,"shop_buy_price":5,"shop_sell_price":2,"anti_get":true,"anti_sell":true,"buy_reject_message":"The merchant will not sell this guarded potion to you.","sell_reject_message":"The merchant refuses this guarded potion."}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:guarded_merchant","title":"Guarded Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":2}]}]}`
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
		t.Fatalf("decode merchant anti-flag summary response body: %v", err)
	}
	if !got.ItemTemplates[0].AntiGet || !got.ItemTemplates[0].AntiSell {
		t.Fatalf("expected item-template anti_get/anti_sell flags, got %+v", got.ItemTemplates)
	}
	if !got.ShopCatalogs[0].Entries[0].AntiGet || !got.ShopCatalogs[0].Entries[0].AntiSell {
		t.Fatalf("expected shop-catalog anti_get/anti_sell flags, got %+v", got.ShopCatalogs)
	}
	if !got.SpawnGroups[0].RewardDropItems[0].AntiGet || !got.SpawnGroups[0].RewardDropItems[0].AntiSell {
		t.Fatalf("expected spawn reward-drop anti_get/anti_sell flags, got %+v", got.SpawnGroups)
	}
	if !got.RewardDrops[0].AntiGet || !got.RewardDrops[0].AntiSell {
		t.Fatalf("expected aggregate reward-drop anti_get/anti_sell flags, got %+v", got.RewardDrops)
	}
}

func TestLocalContentBundleSummaryEndpointReturnsTransferGuardMetadataForLoopbackPost(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"spawn_groups":[{"ref":"practice.bound_reward","name":"Bound Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}],"item_templates":[{"vnum":27001,"name":"Bound Potion","stackable":true,"max_count":200,"shop_buy_price":5,"anti_drop":true,"anti_give":true,"give_reject_message":"You cannot give this bound potion.","anti_stack":true,"drop_reject_message":"You cannot drop this bound potion.","pickup_reject_message":"You cannot pick up this bound potion."}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:bound_merchant","title":"Bound Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":2}]}]}`
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
		t.Fatalf("decode transfer-guard summary response body: %v", err)
	}
	for label, template := range map[string]struct {
		AntiDrop            bool
		AntiGive            bool
		AntiStack           bool
		DropRejectMessage   string
		GiveRejectMessage   string
		PickupRejectMessage string
	}{
		"item template": {
			AntiDrop:            got.ItemTemplates[0].AntiDrop,
			AntiGive:            got.ItemTemplates[0].AntiGive,
			AntiStack:           got.ItemTemplates[0].AntiStack,
			DropRejectMessage:   got.ItemTemplates[0].DropRejectMessage,
			GiveRejectMessage:   got.ItemTemplates[0].GiveRejectMessage,
			PickupRejectMessage: got.ItemTemplates[0].PickupRejectMessage,
		},
		"shop catalog": {
			AntiDrop:            got.ShopCatalogs[0].Entries[0].AntiDrop,
			AntiGive:            got.ShopCatalogs[0].Entries[0].AntiGive,
			AntiStack:           got.ShopCatalogs[0].Entries[0].AntiStack,
			DropRejectMessage:   got.ShopCatalogs[0].Entries[0].DropRejectMessage,
			GiveRejectMessage:   got.ShopCatalogs[0].Entries[0].GiveRejectMessage,
			PickupRejectMessage: got.ShopCatalogs[0].Entries[0].PickupRejectMessage,
		},
		"spawn reward drop": {
			AntiDrop:            got.SpawnGroups[0].RewardDropItems[0].AntiDrop,
			AntiGive:            got.SpawnGroups[0].RewardDropItems[0].AntiGive,
			AntiStack:           got.SpawnGroups[0].RewardDropItems[0].AntiStack,
			DropRejectMessage:   got.SpawnGroups[0].RewardDropItems[0].DropRejectMessage,
			GiveRejectMessage:   got.SpawnGroups[0].RewardDropItems[0].GiveRejectMessage,
			PickupRejectMessage: got.SpawnGroups[0].RewardDropItems[0].PickupRejectMessage,
		},
		"aggregate reward drop": {
			AntiDrop:            got.RewardDrops[0].AntiDrop,
			AntiGive:            got.RewardDrops[0].AntiGive,
			AntiStack:           got.RewardDrops[0].AntiStack,
			DropRejectMessage:   got.RewardDrops[0].DropRejectMessage,
			GiveRejectMessage:   got.RewardDrops[0].GiveRejectMessage,
			PickupRejectMessage: got.RewardDrops[0].PickupRejectMessage,
		},
	} {
		if !template.AntiDrop || !template.AntiGive || !template.AntiStack {
			t.Fatalf("expected %s transfer guard flags, got %+v", label, template)
		}
		if template.DropRejectMessage != "You cannot drop this bound potion." || template.GiveRejectMessage != "You cannot give this bound potion." || template.PickupRejectMessage != "You cannot pick up this bound potion." {
			t.Fatalf("unexpected %s transfer guard messages: %+v", label, template)
		}
	}
}

func TestLocalContentBundleSummaryEndpointReturnsEquipmentGuardMetadataForLoopbackPost(t *testing.T) {
	const (
		equipRejectMessage   = "This armor rejects your path."
		unequipRejectMessage = "This armor cannot be removed here."
	)
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"spawn_groups":[{"ref":"practice.equipment_reward","name":"Equipment Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[11200]}],"item_templates":[{"vnum":11200,"name":"Guarded Practice Armor","stackable":false,"max_count":1,"equip_slot":"body","appearance_vnum":11299,"irremovable":true,"anti_warrior":true,"equip_reject_message":"This armor rejects your path.","unequip_reject_message":"This armor cannot be removed here."}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:equipment_merchant","title":"Equipment Merchant","catalog":[{"slot":0,"item_vnum":11200,"price":50,"count":1}]}]}`
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
		t.Fatalf("decode equipment guard summary response body: %v", err)
	}
	for label, template := range map[string]struct {
		EquipSlot            string
		AppearanceVnum       uint32
		Irremovable          bool
		AntiWarrior          bool
		EquipRejectMessage   string
		UnequipRejectMessage string
	}{
		"item template": {
			EquipSlot:            got.ItemTemplates[0].EquipSlot,
			AppearanceVnum:       got.ItemTemplates[0].AppearanceVnum,
			Irremovable:          got.ItemTemplates[0].Irremovable,
			AntiWarrior:          got.ItemTemplates[0].AntiWarrior,
			EquipRejectMessage:   got.ItemTemplates[0].EquipRejectMessage,
			UnequipRejectMessage: got.ItemTemplates[0].UnequipRejectMessage,
		},
		"shop catalog": {
			EquipSlot:            got.ShopCatalogs[0].Entries[0].EquipSlot,
			AppearanceVnum:       got.ShopCatalogs[0].Entries[0].AppearanceVnum,
			Irremovable:          got.ShopCatalogs[0].Entries[0].Irremovable,
			AntiWarrior:          got.ShopCatalogs[0].Entries[0].AntiWarrior,
			EquipRejectMessage:   got.ShopCatalogs[0].Entries[0].EquipRejectMessage,
			UnequipRejectMessage: got.ShopCatalogs[0].Entries[0].UnequipRejectMessage,
		},
		"spawn reward drop": {
			EquipSlot:            got.SpawnGroups[0].RewardDropItems[0].EquipSlot,
			AppearanceVnum:       got.SpawnGroups[0].RewardDropItems[0].AppearanceVnum,
			Irremovable:          got.SpawnGroups[0].RewardDropItems[0].Irremovable,
			AntiWarrior:          got.SpawnGroups[0].RewardDropItems[0].AntiWarrior,
			EquipRejectMessage:   got.SpawnGroups[0].RewardDropItems[0].EquipRejectMessage,
			UnequipRejectMessage: got.SpawnGroups[0].RewardDropItems[0].UnequipRejectMessage,
		},
		"aggregate reward drop": {
			EquipSlot:            got.RewardDrops[0].EquipSlot,
			AppearanceVnum:       got.RewardDrops[0].AppearanceVnum,
			Irremovable:          got.RewardDrops[0].Irremovable,
			AntiWarrior:          got.RewardDrops[0].AntiWarrior,
			EquipRejectMessage:   got.RewardDrops[0].EquipRejectMessage,
			UnequipRejectMessage: got.RewardDrops[0].UnequipRejectMessage,
		},
	} {
		if template.EquipSlot != inventory.EquipmentSlotBody.String() || template.AppearanceVnum != 11299 || !template.Irremovable || !template.AntiWarrior {
			t.Fatalf("expected %s equipment guard metadata, got %+v", label, template)
		}
		if template.EquipRejectMessage != equipRejectMessage || template.UnequipRejectMessage != unequipRejectMessage {
			t.Fatalf("unexpected %s equipment guard messages: %+v", label, template)
		}
	}
}

func TestLocalContentBundleSummaryEndpointReturnsRefineGuardMetadataForLoopbackPost(t *testing.T) {
	const refineRejectMessage = "This stone cannot be refined yet."
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"spawn_groups":[{"ref":"practice.refine_reward","name":"Refine Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001,11200]}],"item_templates":[{"vnum":27001,"name":"Sealed Upgrade Stone","stackable":true,"max_count":200,"refine_reject_message":"This stone cannot be refined yet."},{"vnum":11200,"name":"Refineable Practice Sword","stackable":false,"max_count":1,"refineable":true}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:refine_merchant","title":"Refine Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":2},{"slot":1,"item_vnum":11200,"price":500,"count":1}]}]}`
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
		t.Fatalf("decode refine guard summary response body: %v", err)
	}
	itemTemplateByVnum := make(map[uint32]contentbundle.ItemTemplateReferenceSummary, len(got.ItemTemplates))
	for _, template := range got.ItemTemplates {
		itemTemplateByVnum[template.Vnum] = template
	}
	if len(itemTemplateByVnum) != 2 || !itemTemplateByVnum[11200].Refineable || itemTemplateByVnum[27001].RefineRejectMessage != refineRejectMessage {
		t.Fatalf("expected refine metadata on item-template summaries, got %+v", got.ItemTemplates)
	}
	shopCatalogEntriesByVnum := make(map[uint32]contentbundle.ShopCatalogEntrySummary)
	if len(got.ShopCatalogs) == 1 {
		for _, entry := range got.ShopCatalogs[0].Entries {
			shopCatalogEntriesByVnum[entry.ItemVnum] = entry
		}
	}
	if len(shopCatalogEntriesByVnum) != 2 || shopCatalogEntriesByVnum[27001].RefineRejectMessage != refineRejectMessage || !shopCatalogEntriesByVnum[11200].Refineable {
		t.Fatalf("expected refine metadata on shop catalog summaries, got %+v", got.ShopCatalogs)
	}
	rewardDropItemsByVnum := make(map[uint32]contentbundle.RewardDropItemSummary)
	if len(got.SpawnGroups) == 1 {
		for _, item := range got.SpawnGroups[0].RewardDropItems {
			rewardDropItemsByVnum[item.ItemVnum] = item
		}
	}
	if len(rewardDropItemsByVnum) != 2 || rewardDropItemsByVnum[27001].RefineRejectMessage != refineRejectMessage || !rewardDropItemsByVnum[11200].Refineable {
		t.Fatalf("expected refine metadata on reward item summaries, got %+v", got.SpawnGroups)
	}
	aggregateDropsByVnum := make(map[uint32]contentbundle.RewardDropAggregateSummary, len(got.RewardDrops))
	for _, item := range got.RewardDrops {
		aggregateDropsByVnum[item.ItemVnum] = item
	}
	if len(aggregateDropsByVnum) != 2 || aggregateDropsByVnum[27001].RefineRejectMessage != refineRejectMessage || !aggregateDropsByVnum[11200].Refineable {
		t.Fatalf("expected refine metadata on aggregate reward drop summaries, got %+v", got.RewardDrops)
	}
}

func TestLocalContentBundleSummaryEndpointReturnsDirectUseGuardMetadataForLoopbackPost(t *testing.T) {
	const useRejectMessage = "This quest-sealed potion cannot be used yet."
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"spawn_groups":[{"ref":"practice.quest_sealed_reward","name":"Quest Sealed Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27001]}],"item_templates":[{"vnum":27001,"name":"Quest-Sealed Potion","stackable":true,"max_count":200,"shop_buy_price":5,"confirm_when_use":true,"quest_use":true,"quest_use_multiple":true,"applicable":true,"use_effect":{"point_type":1,"point_index":1,"point_delta":50,"message":"quest-sealed-use"},"use_reject_message":"This quest-sealed potion cannot be used yet."}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:quest_sealed_merchant","title":"Quest Sealed Merchant","catalog":[{"slot":0,"item_vnum":27001,"price":50,"count":2}]}]}`
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
		t.Fatalf("decode direct-use guard summary response body: %v", err)
	}
	for label, template := range map[string]struct {
		ConfirmWhenUse   bool
		QuestUse         bool
		QuestUseMultiple bool
		Applicable       bool
		UseRejectMessage string
	}{
		"item template": {
			ConfirmWhenUse:   got.ItemTemplates[0].ConfirmWhenUse,
			QuestUse:         got.ItemTemplates[0].QuestUse,
			QuestUseMultiple: got.ItemTemplates[0].QuestUseMultiple,
			Applicable:       got.ItemTemplates[0].Applicable,
			UseRejectMessage: got.ItemTemplates[0].UseRejectMessage,
		},
		"shop catalog": {
			ConfirmWhenUse:   got.ShopCatalogs[0].Entries[0].ConfirmWhenUse,
			QuestUse:         got.ShopCatalogs[0].Entries[0].QuestUse,
			QuestUseMultiple: got.ShopCatalogs[0].Entries[0].QuestUseMultiple,
			Applicable:       got.ShopCatalogs[0].Entries[0].Applicable,
			UseRejectMessage: got.ShopCatalogs[0].Entries[0].UseRejectMessage,
		},
		"spawn reward drop": {
			ConfirmWhenUse:   got.SpawnGroups[0].RewardDropItems[0].ConfirmWhenUse,
			QuestUse:         got.SpawnGroups[0].RewardDropItems[0].QuestUse,
			QuestUseMultiple: got.SpawnGroups[0].RewardDropItems[0].QuestUseMultiple,
			Applicable:       got.SpawnGroups[0].RewardDropItems[0].Applicable,
			UseRejectMessage: got.SpawnGroups[0].RewardDropItems[0].UseRejectMessage,
		},
		"aggregate reward drop": {
			ConfirmWhenUse:   got.RewardDrops[0].ConfirmWhenUse,
			QuestUse:         got.RewardDrops[0].QuestUse,
			QuestUseMultiple: got.RewardDrops[0].QuestUseMultiple,
			Applicable:       got.RewardDrops[0].Applicable,
			UseRejectMessage: got.RewardDrops[0].UseRejectMessage,
		},
	} {
		if !template.ConfirmWhenUse || !template.QuestUse || !template.QuestUseMultiple || !template.Applicable {
			t.Fatalf("expected %s direct-use guard flags, got %+v", label, template)
		}
		if template.UseRejectMessage != useRejectMessage {
			t.Fatalf("unexpected %s direct-use guard message: %+v", label, template)
		}
	}
}

func TestLocalContentBundleSummaryEndpointReturnsUseAndEquipEffectMetadataForLoopbackPost(t *testing.T) {
	summaryer := &stubContentBundleSummaryExporter{status: http.StatusOK}
	mux := RegisterLocalContentBundleSummaryEndpoint(NewPprofMux("gamed"), summaryer.ExportContentBundleSummary)

	body := `{"spawn_groups":[{"ref":"practice.effect_reward","name":"Effect Reward","map_index":42,"x":1800,"y":2900,"race_num":101,"combat_profile":"practice_mob","reward_drop_vnums":[27020,12220]}],"item_templates":[{"vnum":27020,"name":"Effect Potion","stackable":true,"max_count":200,"use_effect":{"point_type":1,"point_index":1,"point_delta":50,"consume_count":2,"message":"consume:27020:+50","info_message":"You feel restored.","special_effect_type":3}},{"vnum":12220,"name":"Penalty Blade","stackable":false,"max_count":1,"equip_slot":"weapon","equip_effect":{"point_type":1,"point_index":2,"point_delta":-10}}],"interaction_definitions":[{"kind":"shop_preview","ref":"npc:effect_merchant","title":"Effect Merchant","catalog":[{"slot":0,"item_vnum":27020,"price":50,"count":2},{"slot":1,"item_vnum":12220,"price":500,"count":1}]}]}`
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
		t.Fatalf("decode effect metadata summary response body: %v", err)
	}

	useEffect := &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, ConsumeCount: 2, Message: "consume:27020:+50", InfoMessage: "You feel restored.", SpecialEffectType: 3}
	equipEffect := &itemcatalog.PointEffect{PointType: 1, PointIndex: 2, PointDelta: -10}
	itemTemplatesByVnum := make(map[uint32]contentbundle.ItemTemplateReferenceSummary, len(got.ItemTemplates))
	for _, template := range got.ItemTemplates {
		itemTemplatesByVnum[template.Vnum] = template
	}
	if !reflect.DeepEqual(itemTemplatesByVnum[27020].UseEffect, useEffect) || !reflect.DeepEqual(itemTemplatesByVnum[12220].EquipEffect, equipEffect) {
		t.Fatalf("expected effect metadata on item-template summaries, got %+v", got.ItemTemplates)
	}
	shopEntriesByVnum := make(map[uint32]contentbundle.ShopCatalogEntrySummary)
	if len(got.ShopCatalogs) == 1 {
		for _, entry := range got.ShopCatalogs[0].Entries {
			shopEntriesByVnum[entry.ItemVnum] = entry
		}
	}
	if !reflect.DeepEqual(shopEntriesByVnum[27020].UseEffect, useEffect) || !reflect.DeepEqual(shopEntriesByVnum[12220].EquipEffect, equipEffect) {
		t.Fatalf("expected effect metadata on shop-catalog summaries, got %+v", got.ShopCatalogs)
	}
	rewardItemsByVnum := make(map[uint32]contentbundle.RewardDropItemSummary)
	if len(got.SpawnGroups) == 1 {
		for _, item := range got.SpawnGroups[0].RewardDropItems {
			rewardItemsByVnum[item.ItemVnum] = item
		}
	}
	if !reflect.DeepEqual(rewardItemsByVnum[27020].UseEffect, useEffect) || !reflect.DeepEqual(rewardItemsByVnum[12220].EquipEffect, equipEffect) {
		t.Fatalf("expected effect metadata on reward item summaries, got %+v", got.SpawnGroups)
	}
	aggregateDropsByVnum := make(map[uint32]contentbundle.RewardDropAggregateSummary, len(got.RewardDrops))
	for _, item := range got.RewardDrops {
		aggregateDropsByVnum[item.ItemVnum] = item
	}
	if !reflect.DeepEqual(aggregateDropsByVnum[27020].UseEffect, useEffect) || !reflect.DeepEqual(aggregateDropsByVnum[12220].EquipEffect, equipEffect) {
		t.Fatalf("expected effect metadata on aggregate reward drop summaries, got %+v", got.RewardDrops)
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
	result  any
	status  int
	calls   int
}

func (s *stubContentBundleSummaryExporter) ExportContentBundleSummary() (any, int) {
	s.calls++
	if s.result != nil {
		return s.result, s.status
	}
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
