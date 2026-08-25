package minimal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func loadBootstrapDropTableRegenAuthoringBundle(t *testing.T, name string) contentbundle.Bundle {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve minimal test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "examples", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var bundle contentbundle.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return bundle
}

func TestGameRuntimeImportsDropTableAuthoringExample(t *testing.T) {
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
		itemcatalog.NewMemoryStore(),
		queststate.NewMemoryStore(),
		nil,
	)
	if err != nil {
		t.Fatalf("new drop-table authoring runtime: %v", err)
	}

	authored := loadBootstrapDropTableRegenAuthoringBundle(t, "bootstrap-drop-table-authoring-bundle.json")
	if len(authored.DropTables) != 1 || len(authored.SpawnGroups) != 1 || len(authored.RegenSpawns) != 0 {
		t.Fatalf("expected authored drop-table example to keep drop_tables + spawn_groups, got regen=%+v drop_tables=%+v spawn_groups=%+v", authored.RegenSpawns, authored.DropTables, authored.SpawnGroups)
	}
	if authored.SpawnGroups[0].RewardDropTableRef != "loot.qa_reward" {
		t.Fatalf("expected authored drop-table example to keep reward_drop_table_ref, got %+v", authored.SpawnGroups[0])
	}
	if len(authored.ItemTemplates) != 2 {
		t.Fatalf("expected authored drop-table example to carry two item templates, got %+v", authored.ItemTemplates)
	}

	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import drop-table authoring example bundle: %v", err)
	}
	if len(imported.DropTables) != 0 || len(imported.RegenSpawns) != 0 {
		t.Fatalf("expected import to strip authoring-only drop/regen collections, got regen=%+v drop_tables=%+v", imported.RegenSpawns, imported.DropTables)
	}

	wantDrops := []uint32{27001, 27002}
	wantGroups := []contentbundle.SpawnGroup{{
		Ref:              "practice.qa_reward_table_mob",
		Name:             "QATableRewardMob",
		MapIndex:         1,
		X:                469850,
		Y:                964200,
		RaceNum:          20350,
		CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
		RewardExperience: 75,
		RewardGold:       60,
		RewardDropVnums:  wantDrops,
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		RequireQuestRef:  "quest:first_steps",
		RequireQuestFlag: "met_guide",
		RequireQuestFrom: 1,
	}}
	if !reflect.DeepEqual(imported.SpawnGroups, wantGroups) {
		t.Fatalf("unexpected imported drop-table spawn groups:\n got: %#v\nwant: %#v", imported.SpawnGroups, wantGroups)
	}
	wantWriter := []interactionstore.Definition{metGuideQuestFlagWriterDefinition()}
	if !reflect.DeepEqual(imported.InteractionDefinitions, wantWriter) {
		t.Fatalf("unexpected imported drop-table interaction definitions:\n got: %#v\nwant: %#v", imported.InteractionDefinitions, wantWriter)
	}

	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one imported drop-table actor, got %#v", actors)
	}
	got := actors[0]
	want := wantGroups[0]
	if got.SpawnGroupRef != want.Ref ||
		got.Name != want.Name ||
		got.MapIndex != want.MapIndex ||
		got.X != want.X ||
		got.Y != want.Y ||
		got.RaceNum != want.RaceNum ||
		got.CombatProfile != want.CombatProfile ||
		got.RewardExperience != want.RewardExperience ||
		got.RewardGold != want.RewardGold ||
		!reflect.DeepEqual(got.RewardDropVnums, want.RewardDropVnums) ||
		got.RewardQuestRef != want.RewardQuestRef ||
		got.RewardQuestFlag != want.RewardQuestFlag ||
		got.RewardQuestTo != want.RewardQuestTo ||
		got.RewardQuestText != want.RewardQuestText ||
		got.RequireQuestRef != want.RequireQuestRef ||
		got.RequireQuestFlag != want.RequireQuestFlag ||
		got.RequireQuestFrom != want.RequireQuestFrom {
		t.Fatalf("unexpected live drop-table actor:\n got: %+v\nwant spawn-group: %+v", got, want)
	}
}

func TestGameRuntimeImportsRegenAuthoringExample(t *testing.T) {
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
		itemcatalog.NewMemoryStore(),
		queststate.NewMemoryStore(),
		nil,
	)
	if err != nil {
		t.Fatalf("new regen authoring runtime: %v", err)
	}

	authored := loadBootstrapDropTableRegenAuthoringBundle(t, "bootstrap-regen-authoring-bundle.json")
	if len(authored.RegenSpawns) != 1 || authored.RegenSpawns[0].Count != 1 || len(authored.DropTables) != 1 || len(authored.SpawnGroups) != 0 {
		t.Fatalf("expected authored regen example to keep one-count regen_spawns + drop_tables, got regen=%+v drop_tables=%+v spawn_groups=%+v", authored.RegenSpawns, authored.DropTables, authored.SpawnGroups)
	}
	if authored.RegenSpawns[0].RewardDropTableRef != "loot.qa_regen_reward" {
		t.Fatalf("expected authored regen example to keep reward_drop_table_ref, got %+v", authored.RegenSpawns[0])
	}
	if len(authored.ItemTemplates) != 2 {
		t.Fatalf("expected authored regen example to carry two item templates, got %+v", authored.ItemTemplates)
	}

	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import regen authoring example bundle: %v", err)
	}
	if len(imported.RegenSpawns) != 0 || len(imported.DropTables) != 0 {
		t.Fatalf("expected import to strip authoring-only regen/drop collections, got regen=%+v drop_tables=%+v", imported.RegenSpawns, imported.DropTables)
	}

	wantDrops := []uint32{27001, 27002}
	wantGroups := []contentbundle.SpawnGroup{{
		Ref:              "practice.qa_regen_mob",
		Name:             "QARegenMob",
		MapIndex:         1,
		X:                469900,
		Y:                964200,
		RaceNum:          20350,
		CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
		RewardExperience: 90,
		RewardGold:       45,
		RewardDropVnums:  wantDrops,
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		RequireQuestRef:  "quest:first_steps",
		RequireQuestFlag: "met_guide",
		RequireQuestFrom: 1,
	}}
	if !reflect.DeepEqual(imported.SpawnGroups, wantGroups) {
		t.Fatalf("unexpected imported regen spawn groups:\n got: %#v\nwant: %#v", imported.SpawnGroups, wantGroups)
	}
	wantWriter := []interactionstore.Definition{metGuideQuestFlagWriterDefinition()}
	if !reflect.DeepEqual(imported.InteractionDefinitions, wantWriter) {
		t.Fatalf("unexpected imported regen interaction definitions:\n got: %#v\nwant: %#v", imported.InteractionDefinitions, wantWriter)
	}

	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one imported regen actor, got %#v", actors)
	}
	got := actors[0]
	want := wantGroups[0]
	if got.SpawnGroupRef != want.Ref ||
		got.Name != want.Name ||
		got.MapIndex != want.MapIndex ||
		got.X != want.X ||
		got.Y != want.Y ||
		got.RaceNum != want.RaceNum ||
		got.CombatProfile != want.CombatProfile ||
		got.RewardExperience != want.RewardExperience ||
		got.RewardGold != want.RewardGold ||
		!reflect.DeepEqual(got.RewardDropVnums, want.RewardDropVnums) ||
		got.RewardQuestRef != want.RewardQuestRef ||
		got.RewardQuestFlag != want.RewardQuestFlag ||
		got.RewardQuestTo != want.RewardQuestTo ||
		got.RewardQuestText != want.RewardQuestText ||
		got.RequireQuestRef != want.RequireQuestRef ||
		got.RequireQuestFlag != want.RequireQuestFlag ||
		got.RequireQuestFrom != want.RequireQuestFrom {
		t.Fatalf("unexpected live regen actor:\n got: %+v\nwant spawn-group: %+v", got, want)
	}
}
