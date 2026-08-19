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
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

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
