package minimal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
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

func loadBootstrapNpcServiceKillQuestCreditBundle(t *testing.T, name string) contentbundle.Bundle {
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

func TestGameRuntimeImportsKillQuestCreditExample(t *testing.T) {
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
		t.Fatalf("new kill-quest-credit example runtime: %v", err)
	}

	authored := loadBootstrapNpcServiceKillQuestCreditBundle(t, "bootstrap-kill-quest-credit-bundle.json")
	if len(authored.DropTables) != 0 || len(authored.RegenSpawns) != 0 {
		t.Fatalf("expected kill-quest-credit example to be runtime-shaped without authoring-only collections, got regen=%+v drop_tables=%+v", authored.RegenSpawns, authored.DropTables)
	}
	if len(authored.SpawnGroups) != 1 || len(authored.ItemTemplates) != 1 || len(authored.InteractionDefinitions) != 0 {
		t.Fatalf("expected kill-quest-credit example to keep one spawn group + one item template and empty interactions, got spawn_groups=%+v item_templates=%+v interactions=%+v", authored.SpawnGroups, authored.ItemTemplates, authored.InteractionDefinitions)
	}
	if authored.SpawnGroups[0].RequireQuestFlag != "" || authored.SpawnGroups[0].RequireQuestRef != "" {
		t.Fatalf("expected kill-quest-credit example to stay ungated, got %+v", authored.SpawnGroups[0])
	}

	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import kill-quest-credit example bundle: %v", err)
	}
	if len(imported.DropTables) != 0 || len(imported.RegenSpawns) != 0 {
		t.Fatalf("expected import to keep runtime-shaped collections empty of authoring-only fields, got regen=%+v drop_tables=%+v", imported.RegenSpawns, imported.DropTables)
	}

	wantGroups := []contentbundle.SpawnGroup{{
		Ref:              "practice.qa_kill_quest_mob",
		Name:             "QAKillQuestMob",
		MapIndex:         1,
		X:                469800,
		Y:                964200,
		RaceNum:          20350,
		CombatProfile:    worldruntime.StaticActorCombatProfileTrainingDummy,
		RewardExperience: 25,
		RewardGold:       10,
		RewardDropVnums:  []uint32{27001},
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
	}}
	if !reflect.DeepEqual(imported.SpawnGroups, wantGroups) {
		t.Fatalf("unexpected imported kill-quest-credit spawn groups:\n got: %#v\nwant: %#v", imported.SpawnGroups, wantGroups)
	}
	if len(imported.InteractionDefinitions) != 0 {
		t.Fatalf("expected imported kill-quest-credit interactions to stay empty, got %#v", imported.InteractionDefinitions)
	}
	if len(imported.ItemTemplates) != 1 || imported.ItemTemplates[0].Vnum != 27001 || imported.ItemTemplates[0].Name != "Small Red Potion" {
		t.Fatalf("unexpected imported kill-quest-credit item templates: %#v", imported.ItemTemplates)
	}

	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one imported kill-quest-credit actor, got %#v", actors)
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
		got.RequireQuestRef != "" ||
		got.RequireQuestFlag != "" ||
		got.RequireQuestFrom != 0 {
		t.Fatalf("unexpected live kill-quest-credit actor:\n got: %+v\nwant spawn-group: %+v", got, want)
	}
}

func TestGameRuntimeImportsNpcServiceExample(t *testing.T) {
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
		t.Fatalf("new NPC service example runtime: %v", err)
	}

	authored := loadBootstrapNpcServiceKillQuestCreditBundle(t, "bootstrap-npc-service-bundle.json")
	if len(authored.DropTables) != 0 || len(authored.RegenSpawns) != 0 {
		t.Fatalf("expected NPC service example to be runtime-shaped without authoring-only collections, got regen=%+v drop_tables=%+v", authored.RegenSpawns, authored.DropTables)
	}
	if len(authored.StaticActors) != 9 || len(authored.SpawnGroups) != 1 || len(authored.InteractionDefinitions) != 9 || len(authored.ItemTemplates) != 2 || len(authored.QuestState) != 1 {
		t.Fatalf("unexpected authored NPC service example shape: static=%d spawn=%d interactions=%d items=%d quest_state=%d", len(authored.StaticActors), len(authored.SpawnGroups), len(authored.InteractionDefinitions), len(authored.ItemTemplates), len(authored.QuestState))
	}

	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import NPC service example bundle: %v", err)
	}
	if len(imported.DropTables) != 0 || len(imported.RegenSpawns) != 0 {
		t.Fatalf("expected import to keep runtime-shaped collections empty of authoring-only fields, got regen=%+v drop_tables=%+v", imported.RegenSpawns, imported.DropTables)
	}

	wantGroups := []contentbundle.SpawnGroup{{
		Ref:              "practice.qa_reward_mob",
		Name:             "QARewardMob",
		MapIndex:         1,
		X:                469800,
		Y:                964200,
		RaceNum:          20350,
		CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
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
	}}
	if !reflect.DeepEqual(imported.SpawnGroups, wantGroups) {
		t.Fatalf("unexpected imported NPC service spawn groups:\n got: %#v\nwant: %#v", imported.SpawnGroups, wantGroups)
	}
	if len(imported.InteractionDefinitions) != 9 {
		t.Fatalf("expected nine imported NPC service interaction definitions, got %#v", imported.InteractionDefinitions)
	}
	wantKinds := map[string]string{
		"lore:qa_square":                interactionstore.KindInfo,
		"npc:qa_cube":                   interactionstore.KindOpenCube,
		"npc:qa_warehouse":              interactionstore.KindOpenSafebox,
		"quest:first_steps":             interactionstore.KindQuestFlag,
		"quest:first_steps_kill_turnin": interactionstore.KindQuestFlag,
		"quest:first_steps_reset":       interactionstore.KindQuestFlag,
		"npc:qa_merchant":               interactionstore.KindShopPreview,
		"npc:qa_guide":                  interactionstore.KindTalk,
		"npc:qa_teleporter":             interactionstore.KindWarp,
	}
	gotKinds := make(map[string]string, len(imported.InteractionDefinitions))
	for _, definition := range imported.InteractionDefinitions {
		gotKinds[definition.Ref] = definition.Kind
	}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("unexpected imported NPC service interaction kinds:\n got: %#v\nwant: %#v", gotKinds, wantKinds)
	}
	if !reflect.DeepEqual(imported.QuestState, []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "step",
		Value:     1,
	}}) {
		t.Fatalf("unexpected imported NPC service quest state: %#v", imported.QuestState)
	}
	wantItemVnums := []uint32{11200, 27001}
	gotItemVnums := make([]uint32, 0, len(imported.ItemTemplates))
	for _, template := range imported.ItemTemplates {
		gotItemVnums = append(gotItemVnums, template.Vnum)
	}
	sort.Slice(gotItemVnums, func(i, j int) bool { return gotItemVnums[i] < gotItemVnums[j] })
	if !reflect.DeepEqual(gotItemVnums, wantItemVnums) {
		t.Fatalf("unexpected imported NPC service item templates: %#v", imported.ItemTemplates)
	}

	actors := runtime.StaticActors()
	if len(actors) != 10 {
		t.Fatalf("expected nine static NPCs plus one spawn-backed reward mob, got %#v", actors)
	}
	byName := make(map[string]worldruntime.StaticActorSnapshot, len(actors))
	for _, actor := range actors {
		byName[actor.Name] = actor
	}
	wantStatic := map[string]struct {
		kind string
		ref  string
	}{
		"CubeMaster":      {kind: interactionstore.KindOpenCube, ref: "npc:qa_cube"},
		"Merchant":        {kind: interactionstore.KindShopPreview, ref: "npc:qa_merchant"},
		"QuestGuide":      {kind: interactionstore.KindQuestFlag, ref: "quest:first_steps"},
		"QuestHunter":     {kind: interactionstore.KindQuestFlag, ref: "quest:first_steps_kill_turnin"},
		"QuestResetGuide": {kind: interactionstore.KindQuestFlag, ref: "quest:first_steps_reset"},
		"Teleporter":      {kind: interactionstore.KindWarp, ref: "npc:qa_teleporter"},
		"VillageGuide":    {kind: interactionstore.KindTalk, ref: "npc:qa_guide"},
		"VillageSignpost": {kind: interactionstore.KindInfo, ref: "lore:qa_square"},
		"Warehouse":       {kind: interactionstore.KindOpenSafebox, ref: "npc:qa_warehouse"},
	}
	for name, want := range wantStatic {
		got, ok := byName[name]
		if !ok {
			t.Fatalf("missing imported NPC service static actor %q in %#v", name, actors)
		}
		if got.InteractionKind != want.kind || got.InteractionRef != want.ref || got.SpawnGroupRef != "" {
			t.Fatalf("unexpected imported static actor %q: %+v", name, got)
		}
	}
	mob, ok := byName["QARewardMob"]
	if !ok {
		t.Fatalf("missing imported QARewardMob in %#v", actors)
	}
	want := wantGroups[0]
	if mob.SpawnGroupRef != want.Ref ||
		mob.CombatProfile != want.CombatProfile ||
		mob.RewardExperience != want.RewardExperience ||
		mob.RewardGold != want.RewardGold ||
		!reflect.DeepEqual(mob.RewardDropVnums, want.RewardDropVnums) ||
		mob.RewardQuestRef != want.RewardQuestRef ||
		mob.RewardQuestFlag != want.RewardQuestFlag ||
		mob.RewardQuestTo != want.RewardQuestTo ||
		mob.RewardQuestText != want.RewardQuestText ||
		mob.RequireQuestRef != want.RequireQuestRef ||
		mob.RequireQuestFlag != want.RequireQuestFlag ||
		mob.RequireQuestFrom != want.RequireQuestFrom {
		t.Fatalf("unexpected live NPC service reward mob:\n got: %+v\nwant spawn-group: %+v", mob, want)
	}

	overview, err := runtime.QuestStateOverview()
	if err != nil {
		t.Fatalf("quest-state overview after NPC service import: %v", err)
	}
	if overview.FlagCount != 1 || len(overview.Characters) != 1 || overview.Characters[0].Character != "QuestHero" {
		t.Fatalf("unexpected live quest-state overview after NPC service import: %#v", overview)
	}
	flag, ok, err := runtime.QuestStateFlag("QuestHero", "quest:first_steps", "step")
	if err != nil || !ok || flag.Value != 1 {
		t.Fatalf("expected seeded QuestHero quest:first_steps.step=1 after NPC service import, got ok=%v flag=%+v err=%v", ok, flag, err)
	}
}
