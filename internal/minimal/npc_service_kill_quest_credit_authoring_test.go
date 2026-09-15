package minimal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/cubestore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
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
	if len(authored.StaticActors) != 9 || len(authored.SpawnGroups) != 1 || len(authored.InteractionDefinitions) != 9 || len(authored.ItemTemplates) != 3 || len(authored.CubeRecipes) != 1 || len(authored.QuestState) != 1 {
		t.Fatalf("unexpected authored NPC service example shape: static=%d spawn=%d interactions=%d items=%d cube_recipes=%d quest_state=%d", len(authored.StaticActors), len(authored.SpawnGroups), len(authored.InteractionDefinitions), len(authored.ItemTemplates), len(authored.CubeRecipes), len(authored.QuestState))
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
	wantItemVnums := []uint32{11200, 27001, 27002}
	gotItemVnums := make([]uint32, 0, len(imported.ItemTemplates))
	for _, template := range imported.ItemTemplates {
		gotItemVnums = append(gotItemVnums, template.Vnum)
	}
	sort.Slice(gotItemVnums, func(i, j int) bool { return gotItemVnums[i] < gotItemVnums[j] })
	if !reflect.DeepEqual(gotItemVnums, wantItemVnums) {
		t.Fatalf("unexpected imported NPC service item templates: %#v", imported.ItemTemplates)
	}
	byVnum := make(map[uint32]itemcatalog.Template, len(imported.ItemTemplates))
	for _, template := range imported.ItemTemplates {
		byVnum[template.Vnum] = template
	}
	wantSwordEffect := &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 10}
	if byVnum[11200].EquipSlot != "weapon" || byVnum[11200].AppearanceVnum != 11201 || byVnum[11200].UseEffect != nil || !reflect.DeepEqual(byVnum[11200].EquipEffect, wantSwordEffect) {
		t.Fatalf("expected imported NPC service 11200 to author weapon equip_slot + appearance_vnum + equip_effect without use_effect, got %+v", byVnum[11200])
	}
	wantPotionEffect := &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "consume:27001:+50", SpecialEffectType: 1}
	if byVnum[27001].EquipSlot != "" || !reflect.DeepEqual(byVnum[27001].UseEffect, wantPotionEffect) {
		t.Fatalf("expected imported NPC service 27001 to author use_effect, got %+v", byVnum[27001])
	}
	if byVnum[27002].Name != "Small Blue Potion" || !byVnum[27002].Stackable || byVnum[27002].MaxCount != 200 || byVnum[27002].ShopBuyPrice != 10 {
		t.Fatalf("expected imported NPC service 27002 to author cube material template with shop_buy_price 10, got %+v", byVnum[27002])
	}
	var merchantCatalog []interactionstore.MerchantCatalogEntry
	for _, definition := range imported.InteractionDefinitions {
		if definition.Kind == interactionstore.KindShopPreview && definition.Ref == "npc:qa_merchant" {
			merchantCatalog = definition.Catalog
			break
		}
	}
	wantMerchantCatalog := []interactionstore.MerchantCatalogEntry{
		{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
		{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
		{Slot: 2, ItemVnum: 27002, Price: 20, Count: 2},
	}
	if !reflect.DeepEqual(merchantCatalog, wantMerchantCatalog) {
		t.Fatalf("unexpected imported NPC service merchant catalog:\n got: %#v\nwant: %#v", merchantCatalog, wantMerchantCatalog)
	}
	if !reflect.DeepEqual(imported.CubeRecipes, cubestore.BootstrapSnapshot().NPCs) {
		t.Fatalf("unexpected imported NPC service cube recipes: %#v", imported.CubeRecipes)
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

func authoredTwoStepQuestFlagGraphBundle() contentbundle.Bundle {
	return contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{
			{Name: "QuestGuide", MapIndex: bootstrapMapIndex, X: 1200, Y: 2200, RaceNum: 20302, InteractionKind: interactionstore.KindQuestFlag, InteractionRef: "quest:first_steps"},
			{Name: "QuestBranch", MapIndex: bootstrapMapIndex, X: 1250, Y: 2200, RaceNum: 20302, InteractionKind: interactionstore.KindQuestFlag, InteractionRef: "quest:first_steps_branch"},
		},
		InteractionDefinitions: []interactionstore.Definition{
			{
				Kind:      interactionstore.KindQuestFlag,
				Ref:       "quest:first_steps",
				Text:      "Quest updated: first_steps.met_guide = 1.",
				QuestRef:  "quest:first_steps",
				QuestFlag: "met_guide",
				QuestTo:   1,
			},
			{
				Kind:      interactionstore.KindQuestFlag,
				Ref:       "quest:first_steps_branch",
				Text:      "Quest updated: first_steps.accepted_path = 1.",
				QuestRef:  "quest:first_steps",
				QuestFlag: "accepted_path",
				QuestTo:   1,
			},
		},
		QuestFlagGraphs: []contentbundle.QuestFlagGraph{{
			Ref: "quest:first_steps_graph",
			Steps: []contentbundle.QuestFlagGraphStep{
				{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps"},
				{Kind: interactionstore.KindQuestFlag, Ref: "quest:first_steps_branch"},
			},
		}},
	}
}

func TestGameRuntimeImportsTwoStepQuestFlagGraphAndExtraGatesLaterWriter(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	hero := peerVisibilityCharacter("QuestHero", 0x01030170, 0x02040170, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "quest-graph", 0x70707070, hero)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "quest-graph", Empire: hero.Empire, Characters: []loginticket.Character{hero}}); err != nil {
		t.Fatalf("seed two-step quest-flag graph account: %v", err)
	}

	sameFlagGraph := authoredTwoStepQuestFlagGraphBundle()
	sameFlagGraph.InteractionDefinitions[1].QuestFlag = "met_guide"
	if _, err := contentbundle.Canonicalize(sameFlagGraph); err != contentbundle.ErrInvalidBundle {
		t.Fatalf("expected same-flag quest_flag_graphs overlay to fail closed, got %v", err)
	}
	oneStepGraph := authoredTwoStepQuestFlagGraphBundle()
	oneStepGraph.QuestFlagGraphs[0].Steps = oneStepGraph.QuestFlagGraphs[0].Steps[:1]
	if _, err := contentbundle.Canonicalize(oneStepGraph); err != contentbundle.ErrInvalidBundle {
		t.Fatalf("expected one-step quest_flag_graphs overlay to fail closed, got %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		ticketStore,
		accounts,
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
		itemcatalog.NewMemoryStore(),
		queststate.NewMemoryStore(),
		nil,
	)
	if err != nil {
		t.Fatalf("new two-step quest-flag graph runtime: %v", err)
	}
	currentTime := time.Unix(1_700_002_000, 0)
	runtime.now = func() time.Time { return currentTime }

	imported, err := runtime.ImportContentBundle(authoredTwoStepQuestFlagGraphBundle())
	if err != nil {
		t.Fatalf("import two-step quest-flag graph bundle: %v", err)
	}
	if len(imported.QuestFlagGraphs) != 1 || imported.QuestFlagGraphs[0].Ref != "quest:first_steps_graph" || len(imported.QuestFlagGraphs[0].Steps) != 2 {
		t.Fatalf("expected imported two-step quest_flag_graphs overlay, got %#v", imported.QuestFlagGraphs)
	}
	exported, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export two-step quest-flag graph bundle: %v", err)
	}
	if !reflect.DeepEqual(exported.QuestFlagGraphs, imported.QuestFlagGraphs) {
		t.Fatalf("expected export to keep the authored quest_flag_graphs overlay:\\n got: %#v\\nwant: %#v", exported.QuestFlagGraphs, imported.QuestFlagGraphs)
	}

	var guideVID, branchVID uint32
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "QuestGuide":
			guideVID = uint32(actor.EntityID)
		case "QuestBranch":
			branchVID = uint32(actor.EntityID)
		}
	}
	if guideVID == 0 || branchVID == 0 {
		t.Fatalf("expected guide and branch quest_flag actors after graph import, got %+v", runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quest-graph", 0x70707070)
	defer closeSessionFlow(t, flow)

	mismatchOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: branchVID})))
	if err != nil {
		t.Fatalf("unexpected extra-gated branch interaction error: %v", err)
	}
	if len(mismatchOut) != 1 {
		t.Fatalf("expected 1 self-only extra-gated branch mismatch frame, got %d", len(mismatchOut))
	}
	mismatchChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, mismatchOut[0]))
	if err != nil || mismatchChat.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected extra-gated branch mismatch chat: %+v err=%v", mismatchChat, err)
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after extra-gated branch mismatch: %v", err)
	}
	if !reflect.DeepEqual(loaded, queststate.Snapshot{Flags: []queststate.Flag{}}) {
		t.Fatalf("expected no quest-state mutation before the first graph writer, got %#v", loaded)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	guideOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: guideVID})))
	if err != nil {
		t.Fatalf("unexpected QuestGuide graph writer error: %v", err)
	}
	if len(guideOut) != 1 {
		t.Fatalf("expected 1 self-only QuestGuide graph writer frame, got %d", len(guideOut))
	}
	guideChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, guideOut[0]))
	if err != nil || guideChat.Message != "Quest updated: first_steps.met_guide = 1." {
		t.Fatalf("unexpected QuestGuide graph writer chat: %+v err=%v", guideChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after QuestGuide graph writer: %v", err)
	}
	wantAfterGuide := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: hero.Name,
		QuestRef:  "quest:first_steps",
		Name:      "met_guide",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantAfterGuide) {
		t.Fatalf("unexpected quest-state after QuestGuide graph writer:\\n got: %#v\\nwant: %#v", loaded, wantAfterGuide)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	branchOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: branchVID})))
	if err != nil {
		t.Fatalf("unexpected unlocked branch graph writer error: %v", err)
	}
	if len(branchOut) != 1 {
		t.Fatalf("expected 1 self-only unlocked branch graph writer frame, got %d", len(branchOut))
	}
	branchChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, branchOut[0]))
	if err != nil || branchChat.Message != "Quest updated: first_steps.accepted_path = 1." {
		t.Fatalf("unexpected unlocked branch graph writer chat: %+v err=%v", branchChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after unlocked branch graph writer: %v", err)
	}
	wantAfterBranch := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "accepted_path", Value: 1},
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
	}}
	if !reflect.DeepEqual(loaded, wantAfterBranch) {
		t.Fatalf("unexpected quest-state after unlocked branch graph writer:\\n got: %#v\\nwant: %#v", loaded, wantAfterBranch)
	}
}
