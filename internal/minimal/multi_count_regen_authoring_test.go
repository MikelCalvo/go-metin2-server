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

func loadBootstrapMultiCountRegenAuthoringBundle(t *testing.T) contentbundle.Bundle {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve minimal test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "examples", "bootstrap-multi-count-regen-authoring-bundle.json"))
	if err != nil {
		t.Fatalf("read multi-count regen authoring example bundle: %v", err)
	}
	var bundle contentbundle.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode multi-count regen authoring example bundle: %v", err)
	}
	return bundle
}

func TestGameRuntimeImportsMultiCountRegenAuthoringExample(t *testing.T) {
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
		t.Fatalf("new multi-count regen authoring runtime: %v", err)
	}

	authored := loadBootstrapMultiCountRegenAuthoringBundle(t)
	if len(authored.RegenSpawns) != 1 || authored.RegenSpawns[0].Count != 2 || authored.RegenSpawns[0].PackSpacing != 100 {
		t.Fatalf("expected authored multi-count regen example to keep count=2 pack_spacing=100, got %+v", authored.RegenSpawns)
	}
	if len(authored.DropTables) == 0 || len(authored.SpawnGroups) != 0 {
		t.Fatalf("expected authored multi-count regen example to keep drop_tables and expand from regen_spawns, got regen=%+v drop_tables=%+v spawn_groups=%+v", authored.RegenSpawns, authored.DropTables, authored.SpawnGroups)
	}

	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import multi-count regen authoring example bundle: %v", err)
	}
	if len(imported.RegenSpawns) != 0 || len(imported.DropTables) != 0 {
		t.Fatalf("expected import to strip authoring-only regen/drop collections, got regen=%+v drop_tables=%+v", imported.RegenSpawns, imported.DropTables)
	}

	wantDrops := []uint32{27001, 27002}
	wantGroups := []contentbundle.SpawnGroup{
		{
			Ref:              "practice.qa_multi_regen_mob.m01",
			Name:             "QAMultiRegenMob 1",
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
		},
		{
			Ref:              "practice.qa_multi_regen_mob.m02",
			Name:             "QAMultiRegenMob 2",
			MapIndex:         1,
			X:                470000,
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
		},
	}
	if !reflect.DeepEqual(imported.SpawnGroups, wantGroups) {
		t.Fatalf("unexpected imported multi-count regen spawn groups:\n got: %#v\nwant: %#v", imported.SpawnGroups, wantGroups)
	}
	wantWriter := []interactionstore.Definition{metGuideQuestFlagWriterDefinition()}
	if !reflect.DeepEqual(imported.InteractionDefinitions, wantWriter) {
		t.Fatalf("unexpected imported multi-count regen interaction definitions:\n got: %#v\nwant: %#v", imported.InteractionDefinitions, wantWriter)
	}

	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected two imported multi-count pack members, got %#v", actors)
	}
	for i, want := range wantGroups {
		got := actors[i]
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
			t.Fatalf("unexpected live multi-count pack member[%d]:\n got: %+v\nwant spawn-group: %+v", i, got, want)
		}
	}
}
