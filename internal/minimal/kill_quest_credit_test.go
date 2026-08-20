package minimal

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// metGuideQuestFlagWriterDefinition is a minimal in-bundle writer for fixtures that
// gate on quest:first_steps.met_guide. Bundle canonicalization requires every
// service / kill-quest require gate to have a matching quest_flag or kill-quest
// credit writer in the same bundle.
func metGuideQuestFlagWriterDefinition() interactionstore.Definition {
	return interactionstore.Definition{
		Kind:      interactionstore.KindQuestFlag,
		Ref:       "quest:first_steps",
		Text:      "Quest updated: first_steps.met_guide = 1.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestTo:   1,
	}
}

func TestGameRuntimeImportsContentBundleKillQuestCredit(t *testing.T) {
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: filepath.Join(t.TempDir(), "quest-state.json")},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new kill quest runtime: %v", err)
	}
	imported, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:             "practice.kill_quest_mob",
			Name:            "KillQuestMob",
			MapIndex:        bootstrapMapIndex,
			X:               1200,
			Y:               2200,
			RaceNum:         20350,
			CombatProfile:   worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
	})
	if err != nil {
		t.Fatalf("import kill quest credit bundle: %v", err)
	}
	if len(imported.SpawnGroups) != 1 || imported.SpawnGroups[0].RewardQuestRef != "quest:first_steps" || imported.SpawnGroups[0].RewardQuestFlag != "killed_qa_mob" || imported.SpawnGroups[0].RewardQuestTo != 1 || imported.SpawnGroups[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected imported kill quest credit spawn group: %+v", imported.SpawnGroups)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 || actors[0].RewardQuestRef != "quest:first_steps" || actors[0].RewardQuestFlag != "killed_qa_mob" || actors[0].RewardQuestTo != 1 || actors[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("expected live actor to carry kill quest credit, got %+v", actors)
	}
	persisted, err := runtime.staticStore.Load()
	if err != nil {
		t.Fatalf("load persisted kill quest actors: %v", err)
	}
	if len(persisted.StaticActors) != 1 || persisted.StaticActors[0].RewardQuestRef != "quest:first_steps" || persisted.StaticActors[0].RewardQuestFlag != "killed_qa_mob" || persisted.StaticActors[0].RewardQuestTo != 1 || persisted.StaticActors[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("expected persisted actor to carry kill quest credit, got %+v", persisted.StaticActors)
	}
	reexported, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export kill quest credit bundle: %v", err)
	}
	if len(reexported.SpawnGroups) != 1 || !reflect.DeepEqual(reexported.SpawnGroups[0].RewardQuestRef, "quest:first_steps") || reexported.SpawnGroups[0].RewardQuestFlag != "killed_qa_mob" || reexported.SpawnGroups[0].RewardQuestTo != 1 || reexported.SpawnGroups[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("expected reexported spawn group to keep kill quest credit, got %+v", reexported.SpawnGroups)
	}
}

func TestGameRuntimeImportsDropTableKillQuestCredit(t *testing.T) {
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: filepath.Join(t.TempDir(), "quest-state.json")},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new drop-table kill quest runtime: %v", err)
	}
	imported, err := runtime.ImportContentBundle(contentbundle.Bundle{
		DropTables: []contentbundle.DropTable{{
			Ref:              "loot.qa_kill_quest_reward",
			RewardExperience: 75,
			RewardGold:       60,
			DropVnums:        []uint32{27001},
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
		ItemTemplates: rewardDropItemTemplates(27001),
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:                "practice.drop_table_kill_quest_mob",
			Name:               "DropTableKillQuestMob",
			MapIndex:           bootstrapMapIndex,
			X:                  1200,
			Y:                  2200,
			RaceNum:            20350,
			CombatProfile:      worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardDropTableRef: "loot.qa_kill_quest_reward",
		}},
	})
	if err != nil {
		t.Fatalf("import drop-table kill quest credit bundle: %v", err)
	}
	if len(imported.DropTables) != 0 {
		t.Fatalf("expected imported bundle to strip authoring-only drop tables, got %+v", imported.DropTables)
	}
	if len(imported.SpawnGroups) != 1 || imported.SpawnGroups[0].RewardDropTableRef != "" || imported.SpawnGroups[0].RewardExperience != 75 || imported.SpawnGroups[0].RewardGold != 60 || !reflect.DeepEqual(imported.SpawnGroups[0].RewardDropVnums, []uint32{27001}) || imported.SpawnGroups[0].RewardQuestRef != "quest:first_steps" || imported.SpawnGroups[0].RewardQuestFlag != "killed_qa_mob" || imported.SpawnGroups[0].RewardQuestTo != 1 || imported.SpawnGroups[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected imported drop-table kill quest spawn group: %+v", imported.SpawnGroups)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 || actors[0].RewardExperience != 75 || actors[0].RewardGold != 60 || !reflect.DeepEqual(actors[0].RewardDropVnums, []uint32{27001}) || actors[0].RewardQuestRef != "quest:first_steps" || actors[0].RewardQuestFlag != "killed_qa_mob" || actors[0].RewardQuestTo != 1 || actors[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("expected live actor to carry expanded drop-table kill quest credit, got %+v", actors)
	}
}

func TestGameRuntimeImportsDropTableKillQuestRequireGate(t *testing.T) {
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: filepath.Join(t.TempDir(), "quest-state.json")},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new drop-table gated kill quest runtime: %v", err)
	}
	imported, err := runtime.ImportContentBundle(contentbundle.Bundle{
		DropTables: []contentbundle.DropTable{{
			Ref:              "loot.qa_gated_kill_quest_reward",
			RewardExperience: 75,
			RewardGold:       60,
			DropVnums:        []uint32{27001},
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
		ItemTemplates: rewardDropItemTemplates(27001),
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:                "practice.drop_table_gated_kill_quest_mob",
			Name:               "DropTableGatedKillQuestMob",
			MapIndex:           bootstrapMapIndex,
			X:                  1200,
			Y:                  2200,
			RaceNum:            20350,
			CombatProfile:      worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardDropTableRef: "loot.qa_gated_kill_quest_reward",
		}},
		InteractionDefinitions: []interactionstore.Definition{metGuideQuestFlagWriterDefinition()},
	})
	if err != nil {
		t.Fatalf("import drop-table gated kill quest credit bundle: %v", err)
	}
	if len(imported.DropTables) != 0 {
		t.Fatalf("expected imported bundle to strip authoring-only drop tables, got %+v", imported.DropTables)
	}
	if len(imported.SpawnGroups) != 1 ||
		imported.SpawnGroups[0].RewardDropTableRef != "" ||
		imported.SpawnGroups[0].RewardExperience != 75 ||
		imported.SpawnGroups[0].RewardGold != 60 ||
		!reflect.DeepEqual(imported.SpawnGroups[0].RewardDropVnums, []uint32{27001}) ||
		imported.SpawnGroups[0].RewardQuestRef != "quest:first_steps" ||
		imported.SpawnGroups[0].RewardQuestFlag != "killed_qa_mob" ||
		imported.SpawnGroups[0].RewardQuestTo != 1 ||
		imported.SpawnGroups[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." ||
		imported.SpawnGroups[0].RequireQuestRef != "quest:first_steps" ||
		imported.SpawnGroups[0].RequireQuestFlag != "met_guide" ||
		imported.SpawnGroups[0].RequireQuestFrom != 1 {
		t.Fatalf("unexpected imported drop-table gated kill quest spawn group: %+v", imported.SpawnGroups)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 ||
		actors[0].RewardExperience != 75 ||
		actors[0].RewardGold != 60 ||
		!reflect.DeepEqual(actors[0].RewardDropVnums, []uint32{27001}) ||
		actors[0].RewardQuestRef != "quest:first_steps" ||
		actors[0].RewardQuestFlag != "killed_qa_mob" ||
		actors[0].RewardQuestTo != 1 ||
		actors[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." ||
		actors[0].RequireQuestRef != "quest:first_steps" ||
		actors[0].RequireQuestFlag != "met_guide" ||
		actors[0].RequireQuestFrom != 1 {
		t.Fatalf("expected live actor to carry expanded drop-table kill quest require gate, got %+v", actors)
	}
}

func TestGameRuntimeImportsRegenSpawnKillQuestRequireGate(t *testing.T) {
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: filepath.Join(t.TempDir(), "quest-state.json")},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new regen gated kill quest runtime: %v", err)
	}
	imported, err := runtime.ImportContentBundle(contentbundle.Bundle{
		DropTables: []contentbundle.DropTable{{
			Ref:              "loot.qa_regen_gated_kill_quest_reward",
			RewardExperience: 90,
			RewardGold:       45,
			DropVnums:        []uint32{27002, 27001},
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
		ItemTemplates: rewardDropItemTemplates(27001, 27002),
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:                "practice.regen_gated_kill_quest_mob",
			Name:               "RegenGatedKillQuestMob",
			MapIndex:           bootstrapMapIndex,
			X:                  469900,
			Y:                  964200,
			RaceNum:            20350,
			Count:              1,
			RewardDropTableRef: "loot.qa_regen_gated_kill_quest_reward",
		}},
		InteractionDefinitions: []interactionstore.Definition{metGuideQuestFlagWriterDefinition()},
	})
	if err != nil {
		t.Fatalf("import regen gated kill quest credit bundle: %v", err)
	}
	if len(imported.DropTables) != 0 || len(imported.RegenSpawns) != 0 {
		t.Fatalf("expected imported bundle to strip authoring-only regen/drop-table fields, got drop_tables=%+v regen_spawns=%+v", imported.DropTables, imported.RegenSpawns)
	}
	if len(imported.SpawnGroups) != 1 ||
		imported.SpawnGroups[0].RewardDropTableRef != "" ||
		imported.SpawnGroups[0].RewardExperience != 90 ||
		imported.SpawnGroups[0].RewardGold != 45 ||
		!reflect.DeepEqual(imported.SpawnGroups[0].RewardDropVnums, []uint32{27001, 27002}) ||
		imported.SpawnGroups[0].RewardQuestRef != "quest:first_steps" ||
		imported.SpawnGroups[0].RewardQuestFlag != "killed_qa_mob" ||
		imported.SpawnGroups[0].RewardQuestTo != 1 ||
		imported.SpawnGroups[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." ||
		imported.SpawnGroups[0].RequireQuestRef != "quest:first_steps" ||
		imported.SpawnGroups[0].RequireQuestFlag != "met_guide" ||
		imported.SpawnGroups[0].RequireQuestFrom != 1 {
		t.Fatalf("unexpected imported regen gated kill quest spawn group: %+v", imported.SpawnGroups)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 ||
		actors[0].RewardExperience != 90 ||
		actors[0].RewardGold != 45 ||
		!reflect.DeepEqual(actors[0].RewardDropVnums, []uint32{27001, 27002}) ||
		actors[0].RewardQuestRef != "quest:first_steps" ||
		actors[0].RewardQuestFlag != "killed_qa_mob" ||
		actors[0].RewardQuestTo != 1 ||
		actors[0].RewardQuestText != "Quest updated: first_steps.killed_qa_mob = 1." ||
		actors[0].RequireQuestRef != "quest:first_steps" ||
		actors[0].RequireQuestFlag != "met_guide" ||
		actors[0].RequireQuestFrom != 1 {
		t.Fatalf("expected live actor to carry expanded regen kill quest require gate, got %+v", actors)
	}
}

func TestHandleAttackKillAppliesQuestFlagCreditAfterDeathReward(t *testing.T) {
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	ticketStore := loginticket.NewFileStore(t.TempDir())
	killer := peerVisibilityCharacter("KillQuestKiller", 0x0103014F, 0x0204014F, 1100, 2100, 0, 101, 201)
	killer.Points[bootstrapExperiencePointType] = 25
	killer.Gold = 40
	issuePeerTicket(t, ticketStore, "kill-quest-killer", 0x4F4F4F4F, killer)

	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "kill-quest-killer", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed kill quest killer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath},
		ticketStore,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new kill quest credit runtime: %v", err)
	}
	currentTime := time.Unix(1_700_000_500, 0)
	runtime.now = func() time.Time { return currentTime }
	imported, err := runtime.ImportContentBundle(contentbundle.Bundle{
		ItemTemplates: rewardDropItemTemplates(27001),
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:              "practice.kill_quest_reward_mob",
			Name:             "KillQuestRewardMob",
			MapIndex:         bootstrapMapIndex,
			X:                1200,
			Y:                2200,
			RaceNum:          20350,
			CombatProfile:    worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardExperience: 75,
			RewardGold:       60,
			RewardDropVnums:  []uint32{27001},
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
	})
	if err != nil {
		t.Fatalf("import kill quest reward bundle: %v", err)
	}
	if len(imported.SpawnGroups) != 1 {
		t.Fatalf("expected one imported spawn group, got %+v", imported.SpawnGroups)
	}
	targetVID := uint32(runtime.StaticActors()[0].EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-quest-killer", 0x4F4F4F4F)
	defer closeSessionFlow(t, flow)
	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected kill quest target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}

	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected kill quest attack error on hit %d: %v", hit, err)
		}
	}
	if len(killOut) < 7 {
		t.Fatalf("expected killing hit to include death/reward frames plus quest chat, got %d", len(killOut))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, killOut[0]))
	if err != nil || dead.VID != targetVID {
		t.Fatalf("unexpected kill quest dead frame: dead=%+v err=%v", dead, err)
	}
	chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, killOut[len(killOut)-1]))
	if err != nil || chat.Type != chatproto.ChatTypeInfo || chat.VID != 0 || chat.Empire != 0 || chat.Message != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected kill quest chat delivery: %+v err=%v", chat, err)
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after kill credit: %v", err)
	}
	want := queststate.Snapshot{Flags: []queststate.Flag{{Character: killer.Name, QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1}}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected quest-state after kill credit:\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestHandleAttackKillQuestCreditSilentOnCurrentValueMismatch(t *testing.T) {
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	ticketStore := loginticket.NewFileStore(t.TempDir())
	killer := peerVisibilityCharacter("KillQuestMismatch", 0x01030150, 0x02040150, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "kill-quest-mismatch", 0x50505050, killer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "kill-quest-mismatch", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed mismatch killer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath},
		ticketStore,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new mismatch kill quest runtime: %v", err)
	}
	currentTime := time.Unix(1_700_000_600, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:             "practice.kill_quest_mismatch_mob",
			Name:            "KillQuestMismatchMob",
			MapIndex:        bootstrapMapIndex,
			X:               1200,
			Y:               2200,
			RaceNum:         20350,
			CombatProfile:   worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestFrom: 0,
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
		QuestState: []queststate.Flag{{Character: killer.Name, QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1}},
	}); err != nil {
		t.Fatalf("import mismatch kill quest bundle: %v", err)
	}
	targetVID := uint32(runtime.StaticActors()[0].EntityID)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-quest-mismatch", 0x50505050)
	defer closeSessionFlow(t, flow)
	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected mismatch target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected mismatch kill attack error on hit %d: %v", hit, err)
		}
	}
	if len(killOut) < 2 {
		t.Fatalf("expected mismatch kill to keep death/clear frames, got %d", len(killOut))
	}
	for idx, raw := range killOut[2:] {
		if _, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected mismatch kill to omit quest chat, found chat frame at offset %d", idx+2)
		}
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after mismatch kill: %v", err)
	}
	want := queststate.Snapshot{Flags: []queststate.Flag{{Character: killer.Name, QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1}}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("mismatch kill mutated quest-state:\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestHandleAttackKillQuestCreditAppliesWhenDeathRewardEmpty(t *testing.T) {
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	ticketStore := loginticket.NewFileStore(t.TempDir())
	killer := peerVisibilityCharacter("KillQuestEmptyReward", 0x01030151, 0x02040151, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "kill-quest-empty", 0x51515151, killer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "kill-quest-empty", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed empty-reward killer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath},
		ticketStore,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new empty-reward kill quest runtime: %v", err)
	}
	currentTime := time.Unix(1_700_000_700, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:             "practice.kill_quest_empty_reward_mob",
			Name:            "KillQuestEmptyRewardMob",
			MapIndex:        bootstrapMapIndex,
			X:               1200,
			Y:               2200,
			RaceNum:         20350,
			CombatProfile:   worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
	}); err != nil {
		t.Fatalf("import empty-reward kill quest bundle: %v", err)
	}
	targetVID := uint32(runtime.StaticActors()[0].EntityID)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-quest-empty", 0x51515151)
	defer closeSessionFlow(t, flow)
	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected empty-reward target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected empty-reward kill attack error on hit %d: %v", hit, err)
		}
	}
	if len(killOut) != 3 {
		t.Fatalf("expected empty-reward kill to return dead, clear, and quest chat, got %d", len(killOut))
	}
	chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, killOut[2]))
	if err != nil || chat.Message != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected empty-reward kill quest chat: %+v err=%v", chat, err)
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after empty-reward kill: %v", err)
	}
	want := queststate.Snapshot{Flags: []queststate.Flag{{Character: killer.Name, QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1}}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected quest-state after empty-reward kill:\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestKillQuestCreditThenTurnInClearsKilledQAMob(t *testing.T) {
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	ticketStore := loginticket.NewFileStore(t.TempDir())
	killer := peerVisibilityCharacter("KillQuestTurnIn", 0x01030152, 0x02040152, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "kill-quest-turnin", 0x52525252, killer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "kill-quest-turnin", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed turn-in killer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath},
		ticketStore,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new kill quest turn-in runtime: %v", err)
	}
	currentTime := time.Unix(1_700_000_800, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:             "practice.kill_quest_turnin_mob",
			Name:            "KillQuestTurnInMob",
			MapIndex:        bootstrapMapIndex,
			X:               1200,
			Y:               2200,
			RaceNum:         20350,
			CombatProfile:   worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
		StaticActors: []contentbundle.StaticActor{{
			Name:            "QuestHunter",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2200,
			RaceNum:         20302,
			InteractionKind: interactionstore.KindQuestFlag,
			InteractionRef:  "quest:first_steps_kill_turnin",
		}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:      interactionstore.KindQuestFlag,
			Ref:       "quest:first_steps_kill_turnin",
			Text:      "Quest updated: first_steps.killed_qa_mob = 0.",
			QuestRef:  "quest:first_steps",
			QuestFlag: "killed_qa_mob",
			QuestFrom: 1,
			QuestTo:   0,
		}},
	}); err != nil {
		t.Fatalf("import kill quest turn-in bundle: %v", err)
	}

	var mobVID, hunterVID uint32
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "KillQuestTurnInMob":
			mobVID = uint32(actor.EntityID)
		case "QuestHunter":
			hunterVID = uint32(actor.EntityID)
		}
	}
	if mobVID == 0 || hunterVID == 0 {
		t.Fatalf("expected imported mob and hunter actors, got %+v", runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-quest-turnin", 0x52525252)
	defer closeSessionFlow(t, flow)
	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: mobVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected turn-in target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: mobVID})))
		if err != nil {
			t.Fatalf("unexpected turn-in kill attack error on hit %d: %v", hit, err)
		}
	}
	chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, killOut[len(killOut)-1]))
	if err != nil || chat.Message != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected kill credit chat before turn-in: %+v err=%v", chat, err)
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after kill credit before turn-in: %v", err)
	}
	wantAfterKill := queststate.Snapshot{Flags: []queststate.Flag{{Character: killer.Name, QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1}}}
	if !reflect.DeepEqual(loaded, wantAfterKill) {
		t.Fatalf("unexpected quest-state after kill credit before turn-in:\n got: %#v\nwant: %#v", loaded, wantAfterKill)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	turnInOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: hunterVID})))
	if err != nil {
		t.Fatalf("unexpected kill quest turn-in interaction error: %v", err)
	}
	if len(turnInOut) != 1 {
		t.Fatalf("expected 1 self-only kill quest turn-in frame, got %d", len(turnInOut))
	}
	turnInChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, turnInOut[0]))
	if err != nil || turnInChat.Type != chatproto.ChatTypeInfo || turnInChat.VID != 0 || turnInChat.Empire != 0 || turnInChat.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected kill quest turn-in chat delivery: %+v err=%v", turnInChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after kill quest turn-in: %v", err)
	}
	wantAfterTurnIn := queststate.Snapshot{Flags: []queststate.Flag{}}
	if !reflect.DeepEqual(loaded, wantAfterTurnIn) {
		t.Fatalf("unexpected quest-state after kill quest turn-in:\n got: %#v\nwant: %#v", loaded, wantAfterTurnIn)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for self-only kill quest turn-in, got %d", len(queued))
	}
}

func TestKillQuestTurnInMismatchWithoutKillCredit(t *testing.T) {
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	ticketStore := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("KillQuestTurnInMismatch", 0x01030153, 0x02040153, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "kill-quest-turnin-mismatch", 0x53535353, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "kill-quest-turnin-mismatch", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed turn-in mismatch account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath},
		ticketStore,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new kill quest turn-in mismatch runtime: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "QuestHunter",
			MapIndex:        bootstrapMapIndex,
			X:               1200,
			Y:               2200,
			RaceNum:         20302,
			InteractionKind: interactionstore.KindQuestFlag,
			InteractionRef:  "quest:first_steps_kill_turnin",
		}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:      interactionstore.KindQuestFlag,
			Ref:       "quest:first_steps_kill_turnin",
			Text:      "Quest updated: first_steps.killed_qa_mob = 0.",
			QuestRef:  "quest:first_steps",
			QuestFlag: "killed_qa_mob",
			QuestFrom: 1,
			QuestTo:   0,
		}},
	}); err != nil {
		t.Fatalf("import kill quest turn-in mismatch bundle: %v", err)
	}
	hunterVID := uint32(runtime.StaticActors()[0].EntityID)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-quest-turnin-mismatch", 0x53535353)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: hunterVID})))
	if err != nil {
		t.Fatalf("unexpected kill quest turn-in mismatch interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only mismatch frame for kill quest turn-in, got %d", len(out))
	}
	chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || chat.Type != chatproto.ChatTypeInfo || chat.VID != 0 || chat.Empire != 0 || chat.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected kill quest turn-in mismatch chat delivery: %+v err=%v", chat, err)
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after mismatch turn-in: %v", err)
	}
	if !reflect.DeepEqual(loaded, queststate.Snapshot{Flags: []queststate.Flag{}}) {
		t.Fatalf("mismatch turn-in mutated quest-state:\n got: %#v\nwant empty snapshot", loaded)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for mismatch kill quest turn-in, got %d", len(queued))
	}
}

func TestHandleAttackKillQuestRequireGateSilentWhenUnmet(t *testing.T) {
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	ticketStore := loginticket.NewFileStore(t.TempDir())
	killer := peerVisibilityCharacter("KillQuestGateMiss", 0x01030154, 0x02040154, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "kill-quest-gate-miss", 0x54545454, killer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "kill-quest-gate-miss", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed gated miss killer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath},
		ticketStore,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new gated miss kill quest runtime: %v", err)
	}
	currentTime := time.Unix(1_700_000_600, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:              "practice.gated_kill_quest_miss_mob",
			Name:             "GatedKillQuestMissMob",
			MapIndex:         bootstrapMapIndex,
			X:                1200,
			Y:                2200,
			RaceNum:          20350,
			CombatProfile:    worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
		InteractionDefinitions: []interactionstore.Definition{metGuideQuestFlagWriterDefinition()},
	}); err != nil {
		t.Fatalf("import gated miss kill quest bundle: %v", err)
	}
	targetVID := uint32(runtime.StaticActors()[0].EntityID)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-quest-gate-miss", 0x54545454)
	defer closeSessionFlow(t, flow)
	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected gated miss target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected gated miss kill attack error on hit %d: %v", hit, err)
		}
	}
	for _, frame := range killOut {
		if chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame)); err == nil {
			t.Fatalf("expected no quest chat when require gate is unmet, got %+v", chat)
		}
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after gated miss kill: %v", err)
	}
	if !reflect.DeepEqual(loaded, queststate.Snapshot{Flags: []queststate.Flag{}}) {
		t.Fatalf("gated miss kill mutated quest-state:\n got: %#v\nwant empty snapshot", loaded)
	}
}

func TestHandleAttackKillQuestRequireGateAppliesWhenMet(t *testing.T) {
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	ticketStore := loginticket.NewFileStore(t.TempDir())
	killer := peerVisibilityCharacter("KillQuestGateHit", 0x01030155, 0x02040155, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "kill-quest-gate-hit", 0x55555555, killer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "kill-quest-gate-hit", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed gated hit killer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath},
		ticketStore,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("new gated hit kill quest runtime: %v", err)
	}
	currentTime := time.Unix(1_700_000_700, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:              "practice.gated_kill_quest_hit_mob",
			Name:             "GatedKillQuestHitMob",
			MapIndex:         bootstrapMapIndex,
			X:                1200,
			Y:                2200,
			RaceNum:          20350,
			CombatProfile:    worldruntime.StaticActorCombatProfileTrainingDummy,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
		InteractionDefinitions: []interactionstore.Definition{metGuideQuestFlagWriterDefinition()},
	}); err != nil {
		t.Fatalf("import gated hit kill quest bundle: %v", err)
	}
	if _, err := runtime.ApplyQuestStateTransition(queststate.Transition{
		Character: killer.Name,
		QuestRef:  "quest:first_steps",
		Flag:      "met_guide",
		From:      0,
		To:        1,
	}); err != nil {
		t.Fatalf("seed met_guide prerequisite: %v", err)
	}
	targetVID := uint32(runtime.StaticActors()[0].EntityID)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-quest-gate-hit", 0x55555555)
	defer closeSessionFlow(t, flow)
	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected gated hit target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected gated hit kill attack error on hit %d: %v", hit, err)
		}
	}
	chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, killOut[len(killOut)-1]))
	if err != nil || chat.Type != chatproto.ChatTypeInfo || chat.VID != 0 || chat.Empire != 0 || chat.Message != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected gated hit kill quest chat delivery: %+v err=%v", chat, err)
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after gated hit kill: %v", err)
	}
	want := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: killer.Name, QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1},
		{Character: killer.Name, QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
	}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected quest-state after gated hit kill:\n got: %#v\nwant: %#v", loaded, want)
	}
}
