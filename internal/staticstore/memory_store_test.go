package staticstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestMemoryStoreSaveLoadWithoutFilesystem(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore()

	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound before save, got %v", err)
	}

	const customProfile = "practice_static_memory_formula_wolf"
	worldruntime.UnregisterStaticActorCombatProfileForTest(customProfile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(customProfile) })

	seed := Snapshot{
		StaticActors: []StaticActor{
			{
				EntityID:        9,
				Name:            "VillageGuard",
				MapIndex:        1,
				X:               469300,
				Y:               964200,
				RaceNum:         20355,
				InteractionKind: interactionstore.KindTalk,
				InteractionRef:  "npc:village_guard",
			},
			{
				EntityID:         7,
				Name:             "PracticeMob",
				MapIndex:         42,
				X:                1800,
				Y:                2900,
				RaceNum:          101,
				CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
				SpawnGroupRef:    "practice.reward_mob",
				SpawnHome:        &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800},
				RewardExperience: 25,
				RewardGold:       12,
				RewardDropVnums:  []uint32{27002, 27001},
			},
			{
				EntityID:      23,
				Name:          "FormulaWolf",
				MapIndex:      42,
				X:             1900,
				Y:             3000,
				RaceNum:       101,
				CombatProfile: customProfile,
				SpawnGroupRef: "practice.formula_wolf",
			},
		},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        customProfile,
			MaxHP:          24,
			AttackValue:    9,
			DefenseValue:   4,
			RespawnDelayMs: 1500,
			DeathReward:    worldruntime.StaticActorDeathReward{Experience: 8, Gold: 3, DropVnums: []uint32{27002, 27001}},
		}},
	}
	if err := store.Save(seed); err != nil {
		t.Fatalf("save memory static actors: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load memory static actors: %v", err)
	}
	if len(loaded.StaticActors) != 3 || loaded.StaticActors[0].Name != "FormulaWolf" || loaded.StaticActors[1].Name != "PracticeMob" || loaded.StaticActors[2].Name != "VillageGuard" {
		t.Fatalf("unexpected loaded actors: %#v", loaded.StaticActors)
	}
	if loaded.StaticActors[1].SpawnHome == nil || loaded.StaticActors[1].SpawnHome.X != 1700 {
		t.Fatalf("expected spawn home to round-trip, got %#v", loaded.StaticActors[1].SpawnHome)
	}
	if len(loaded.StaticActors[1].RewardDropVnums) != 2 || loaded.StaticActors[1].RewardDropVnums[0] != 27001 {
		t.Fatalf("expected sorted reward drops to round-trip, got %#v", loaded.StaticActors[1].RewardDropVnums)
	}
	if len(loaded.CombatProfiles) != 1 || loaded.CombatProfiles[0].Profile != customProfile || loaded.CombatProfiles[0].DamagePerNormalAttack != 5 {
		t.Fatalf("expected custom combat profile to round-trip with formula damage, got %#v", loaded.CombatProfiles)
	}

	loaded.StaticActors[1].Name = "mutated"
	loaded.StaticActors[1].RewardDropVnums[0] = 99999
	loaded.StaticActors[1].SpawnHome.X = 1
	loaded.CombatProfiles[0].MaxHP = 99
	loaded.CombatProfiles[0].DeathReward.DropVnums[0] = 99999

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload memory static actors: %v", err)
	}
	if reloaded.StaticActors[1].Name != "PracticeMob" {
		t.Fatalf("memory store leaked caller mutation into name: %#v", reloaded.StaticActors[1])
	}
	if reloaded.StaticActors[1].RewardDropVnums[0] != 27001 {
		t.Fatalf("memory store leaked caller mutation into reward drops: %#v", reloaded.StaticActors[1].RewardDropVnums)
	}
	if reloaded.StaticActors[1].SpawnHome == nil || reloaded.StaticActors[1].SpawnHome.X != 1700 {
		t.Fatalf("memory store leaked caller mutation into spawn home: %#v", reloaded.StaticActors[1].SpawnHome)
	}
	if len(reloaded.CombatProfiles) != 1 || reloaded.CombatProfiles[0].MaxHP != 24 {
		t.Fatalf("memory store leaked caller mutation into combat profiles: %#v", reloaded.CombatProfiles)
	}
	if len(reloaded.CombatProfiles[0].DeathReward.DropVnums) != 2 || reloaded.CombatProfiles[0].DeathReward.DropVnums[0] != 27001 {
		t.Fatalf("memory store leaked caller mutation into profile drops: %#v", reloaded.CombatProfiles[0].DeathReward.DropVnums)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("memory store wrote filesystem entries: %#v", entries)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".static-actors-*.json")); err != nil || len(matches) != 0 {
		t.Fatalf("memory store created crash-temp shaped files: matches=%v err=%v", matches, err)
	}
}

func TestMemoryStoreRejectsInvalidSave(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(Snapshot{StaticActors: []StaticActor{{EntityID: 0, Name: "Broken", MapIndex: 1, RaceNum: 1}}}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero entity id, got %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("invalid save must leave store uncommitted, got %v", err)
	}
}

func TestMemoryStoreExportsMatchFileStoreAndPassQuarantine(t *testing.T) {
	staticSnapshot := Snapshot{StaticActors: []StaticActor{
		{EntityID: 9, Name: "VillageGuard", MapIndex: 1, X: 469300, Y: 964200, RaceNum: 20355, InteractionKind: interactionstore.KindTalk, InteractionRef: "npc:village_guard"},
		{EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101, CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob", SpawnHome: &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800}, RewardExperience: 25, RewardGold: 12, RewardDropVnums: []uint32{27002, 27001}},
	}}
	interactionSnapshot := interactionstore.Snapshot{Definitions: []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."},
		{Kind: interactionstore.KindShopPreview, Ref: "npc:merchant", Title: "Village Merchant", Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 2},
		}},
	}}

	fileStatic := NewFileStore(filepath.Join(t.TempDir(), "static-actors.json"))
	fileInteractions := interactionstore.NewFileStore(filepath.Join(t.TempDir(), "interaction-definitions.json"))
	memoryStatic := NewMemoryStore()
	memoryInteractions := interactionstore.NewMemoryStore()
	if err := fileStatic.Save(staticSnapshot); err != nil {
		t.Fatalf("save file static actors: %v", err)
	}
	if err := fileInteractions.Save(interactionSnapshot); err != nil {
		t.Fatalf("save file interactions: %v", err)
	}
	if err := memoryStatic.Save(staticSnapshot); err != nil {
		t.Fatalf("save memory static actors: %v", err)
	}
	if err := memoryInteractions.Save(interactionSnapshot); err != nil {
		t.Fatalf("save memory interactions: %v", err)
	}

	fileExport, err := fileStatic.ExportStaticActorContentState(fileInteractions)
	if err != nil {
		t.Fatalf("file static-actor content-state export: %v", err)
	}
	memoryExport, err := memoryStatic.ExportStaticActorContentState(memoryInteractions)
	if err != nil {
		t.Fatalf("memory static-actor content-state export: %v", err)
	}
	if !reflect.DeepEqual(fileExport, memoryExport) {
		t.Fatalf("static-actor content-state export mismatch:\n file: %#v\nmemory: %#v", fileExport, memoryExport)
	}
	if _, err := ValidateStaticActorContentStateExport(memoryExport); err != nil {
		t.Fatalf("quarantine memory static-actor content-state export: %v", err)
	}
}

func TestMemoryStoreExportTreatsEmptyStoreAsEmpty(t *testing.T) {
	store := NewMemoryStore()
	export, err := store.ExportStaticActorContentState(interactionstore.NewMemoryStore())
	if err != nil {
		t.Fatalf("export empty memory store: %v", err)
	}
	if export.MigrationVersion != StaticActorContentStateMigrationVersion || export.MigrationName != StaticActorContentStateMigrationName {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.InteractionDefinitions) != 0 || len(export.MerchantCatalogEntries) != 0 || len(export.QuestFlagRewardItems) != 0 || len(export.QuestFlagConsumeItems) != 0 || len(export.StaticActors) != 0 || len(export.RewardDrops) != 0 {
		t.Fatalf("expected empty static-actor content-state export, got %#v", export)
	}
}

func TestMemoryStoreSatisfiesStaticActorContentStateExporter(t *testing.T) {
	var exporter StaticActorContentStateExporter = NewMemoryStore()
	if _, err := exporter.ExportStaticActorContentState(nil); err != nil {
		t.Fatalf("empty static-actor content-state export: %v", err)
	}
}
