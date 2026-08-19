package minimal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func loadBootstrapPveVerticalAuthoringBundle(t *testing.T) contentbundle.Bundle {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve minimal test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "examples", "bootstrap-pve-vertical-authoring-bundle.json"))
	if err != nil {
		t.Fatalf("read PvE vertical authoring example bundle: %v", err)
	}
	var bundle contentbundle.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode PvE vertical authoring example bundle: %v", err)
	}
	return bundle
}

func TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn(t *testing.T) {
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	ticketStore := loginticket.NewFileStore(t.TempDir())
	hero := peerVisibilityCharacter("PveVerticalHero", 0x01030160, 0x02040160, 469500, 964200, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "pve-vertical", 0x60606060, hero)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "pve-vertical", Empire: hero.Empire, Characters: []loginticket.Character{hero}}); err != nil {
		t.Fatalf("seed PvE vertical account: %v", err)
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
		t.Fatalf("new PvE vertical runtime: %v", err)
	}
	currentTime := time.Unix(1_700_001_000, 0)
	runtime.now = func() time.Time { return currentTime }

	imported, err := runtime.ImportContentBundle(loadBootstrapPveVerticalAuthoringBundle(t))
	if err != nil {
		t.Fatalf("import PvE vertical authoring bundle: %v", err)
	}
	if len(imported.RegenSpawns) != 0 || len(imported.DropTables) != 0 {
		t.Fatalf("expected import to canonicalize away authoring-only regen/drop collections, got regen=%+v drop_tables=%+v", imported.RegenSpawns, imported.DropTables)
	}
	if len(imported.SpawnGroups) != 1 || imported.SpawnGroups[0].Ref != "practice.qa_pve_vertical_mob" {
		t.Fatalf("expected imported spawn group practice.qa_pve_vertical_mob, got %+v", imported.SpawnGroups)
	}

	var guideVID, hunterVID, merchantVID, mobVID uint32
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "QuestGuide":
			guideVID = uint32(actor.EntityID)
		case "QuestHunter":
			hunterVID = uint32(actor.EntityID)
		case "Merchant":
			merchantVID = uint32(actor.EntityID)
		case "QAPveVerticalMob":
			mobVID = uint32(actor.EntityID)
		}
	}
	if guideVID == 0 || hunterVID == 0 || merchantVID == 0 || mobVID == 0 {
		t.Fatalf("expected guide/hunter/merchant/mob actors after import, got %+v", runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pve-vertical", 0x60606060)

	mismatchOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected gated merchant mismatch interaction error: %v", err)
	}
	if len(mismatchOut) != 1 {
		t.Fatalf("expected 1 self-only merchant mismatch frame, got %d", len(mismatchOut))
	}
	mismatchChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, mismatchOut[0]))
	if err != nil || mismatchChat.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected gated merchant mismatch chat: %+v err=%v", mismatchChat, err)
	}

	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: mobVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected pre-guide target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var preGuideKillOut [][]byte
	for hit := 1; hit <= int(worldruntime.PracticeMobBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		preGuideKillOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: mobVID})))
		if err != nil {
			t.Fatalf("unexpected pre-guide kill attack error on hit %d: %v", hit, err)
		}
	}
	for _, frame := range preGuideKillOut {
		if chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame)); err == nil {
			t.Fatalf("expected no quest chat before guide unlock, got %+v", chat)
		}
	}
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after pre-guide kill: %v", err)
	}
	if !reflect.DeepEqual(loaded, queststate.Snapshot{Flags: []queststate.Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}) {
		t.Fatalf("unexpected quest-state after pre-guide kill:\n got: %#v", loaded)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)
	_ = flushServerFrames(t, flow)

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	guideOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: guideVID})))
	if err != nil {
		t.Fatalf("unexpected QuestGuide interaction error: %v", err)
	}
	if len(guideOut) != 1 {
		t.Fatalf("expected 1 self-only QuestGuide frame, got %d", len(guideOut))
	}
	guideChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, guideOut[0]))
	if err != nil || guideChat.Message != "Quest updated: first_steps.met_guide = 1." {
		t.Fatalf("unexpected QuestGuide chat delivery: %+v err=%v", guideChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after QuestGuide: %v", err)
	}
	wantAfterGuide := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}
	if !reflect.DeepEqual(loaded, wantAfterGuide) {
		t.Fatalf("unexpected quest-state after QuestGuide:\n got: %#v\nwant: %#v", loaded, wantAfterGuide)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	shopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected unlocked merchant interaction error: %v", err)
	}
	if len(shopOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after guide unlock, got %d", len(shopOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, shopOut[0])); err != nil {
		t.Fatalf("decode unlocked merchant shop start: %v", err)
	}

	closeSessionFlow(t, flow)
	flow, _ = enterGameWithLoginTicket(t, runtime.SessionFactory(), "pve-vertical", 0x60606060)
	defer closeSessionFlow(t, flow)

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	resumeShopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected reconnect merchant interaction error: %v", err)
	}
	if len(resumeShopOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after reconnect, got %d", len(resumeShopOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, resumeShopOut[0])); err != nil {
		t.Fatalf("decode reconnect merchant shop start: %v", err)
	}
	closeShopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected reconnect merchant close error: %v", err)
	}
	if len(closeShopOut) != 1 {
		t.Fatalf("expected 1 merchant close frame after reconnect shop open, got %d", len(closeShopOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, closeShopOut[0])); err != nil {
		t.Fatalf("decode reconnect merchant shop end: %v", err)
	}

	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: mobVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected post-guide target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var postGuideKillOut [][]byte
	for hit := 1; hit <= int(worldruntime.PracticeMobBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		postGuideKillOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: mobVID})))
		if err != nil {
			t.Fatalf("unexpected post-guide kill attack error on hit %d: %v", hit, err)
		}
	}
	killChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, postGuideKillOut[len(postGuideKillOut)-1]))
	if err != nil || killChat.Message != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected post-guide kill quest chat: %+v err=%v", killChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after gated kill credit: %v", err)
	}
	wantAfterKill := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1},
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}
	if !reflect.DeepEqual(loaded, wantAfterKill) {
		t.Fatalf("unexpected quest-state after gated kill credit:\n got: %#v\nwant: %#v", loaded, wantAfterKill)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	turnInOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: hunterVID})))
	if err != nil {
		t.Fatalf("unexpected QuestHunter turn-in interaction error: %v", err)
	}
	if len(turnInOut) != 1 {
		t.Fatalf("expected 1 self-only QuestHunter turn-in frame, got %d", len(turnInOut))
	}
	turnInChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, turnInOut[0]))
	if err != nil || turnInChat.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected QuestHunter turn-in chat: %+v err=%v", turnInChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after QuestHunter turn-in: %v", err)
	}
	wantAfterTurnIn := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}
	if !reflect.DeepEqual(loaded, wantAfterTurnIn) {
		t.Fatalf("unexpected quest-state after QuestHunter turn-in:\n got: %#v\nwant: %#v", loaded, wantAfterTurnIn)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	secondTurnInOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: hunterVID})))
	if err != nil {
		t.Fatalf("unexpected second QuestHunter interaction error: %v", err)
	}
	if len(secondTurnInOut) != 1 {
		t.Fatalf("expected 1 self-only second QuestHunter mismatch frame, got %d", len(secondTurnInOut))
	}
	secondTurnInChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, secondTurnInOut[0]))
	if err != nil || secondTurnInChat.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected second QuestHunter mismatch chat: %+v err=%v", secondTurnInChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after second QuestHunter mismatch: %v", err)
	}
	if !reflect.DeepEqual(loaded, wantAfterTurnIn) {
		t.Fatalf("second QuestHunter mismatch mutated quest-state:\n got: %#v\nwant: %#v", loaded, wantAfterTurnIn)
	}
}
