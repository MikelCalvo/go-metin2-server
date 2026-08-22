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
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// TestGameRuntimeQuestFlagRewardStateRematerializesAcrossDaemonRestart proves the
// Track E.4 crash/restart rematerialization contract for PvE character gold,
// inventory, and quest flags: after a successful quest_flag turn-in, a fresh
// gameRuntime rebuilt from the same FileStore paths rematerializes committed
// account/quest state on EnterGame even when the post-restart login ticket still
// carries the pre-reward character snapshot.
//
// This deliberately does not cover pending ground-item / ground-gold restart
// durability (covered separately by TestGameRuntimePendingGroundItemAndGoldRematerializeAcrossDaemonRestart).
func TestGameRuntimeQuestFlagRewardStateRematerializesAcrossDaemonRestart(t *testing.T) {
	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	itemTemplatePath := filepath.Join(t.TempDir(), "item-templates.json")

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)

	peer := peerVisibilityCharacter("QuestHero", 0x01030121, 0x02040121, 1100, 2100, 0, 101, 201)
	peer.Gold = 25
	const (
		login    = "quest-hero-restart-reward"
		loginKey = uint32(0x19191919)
	)
	issuePeerTicket(t, ticketStore, login, loginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag restart-reward account: %v", err)
	}

	interactionPath := filepath.Join(t.TempDir(), "interaction-definitions.json")
	interactionStore := interactionstore.NewFileStore(interactionPath)
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{{
		Kind:            interactionstore.KindQuestFlag,
		Ref:             "quest:first_steps_kill_turnin",
		Text:            "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:        "quest:first_steps",
		QuestFlag:       "killed_qa_mob",
		QuestFrom:       1,
		QuestTo:         0,
		RewardGold:      100,
		RewardItemVnum:  27001,
		RewardItemCount: 1,
	}}}); err != nil {
		t.Fatalf("seed quest-flag restart-reward interactions: %v", err)
	}
	itemStore := itemcatalog.NewFileStore(itemTemplatePath)
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5,
	}}}); err != nil {
		t.Fatalf("seed quest-flag restart-reward templates: %v", err)
	}
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for restart-reward turn-in: %v", err)
	}

	cfg := config.Service{
		LegacyAddr:          ":13000",
		PublicAddr:          "127.0.0.1",
		QuestStateStorePath: questStatePath,
	}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected quest-flag restart-reward runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag restart-reward static actor registration to succeed")
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible quest-flag restart-reward actor, got %d", len(enterOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag restart-reward interaction error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected chat + gold + item frames for quest-flag restart-reward interaction, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected quest-flag restart-reward chat delivery: %+v err=%v", delivery, err)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
	if err != nil || pointChange.Amount != 100 || pointChange.Value != 125 {
		t.Fatalf("unexpected quest-flag restart-reward gold frame: %+v err=%v", pointChange, err)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[2]))
	if err != nil || itemSet.Position != itemproto.InventoryPosition(0) || itemSet.Vnum != 27001 || itemSet.Count != 1 {
		t.Fatalf("unexpected quest-flag restart-reward set frame: %+v err=%v", itemSet, err)
	}
	closeSessionFlow(t, flow)

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted quest-flag restart-reward account before daemon restart: %v", err)
	}
	if account.Characters[0].Gold != 125 {
		t.Fatalf("expected persisted gold 125 before daemon restart, got %d", account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 27001 || account.Characters[0].Inventory[0].Count != 1 {
		t.Fatalf("expected persisted inventory grant before daemon restart, got %+v", account.Characters[0].Inventory)
	}
	loadedQuest, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state before daemon restart: %v", err)
	}
	if want := (queststate.Snapshot{Flags: []queststate.Flag{}}); !reflect.DeepEqual(loadedQuest, want) {
		t.Fatalf("unexpected persisted quest-state before daemon restart:\n got: %#v\nwant: %#v", loadedQuest, want)
	}

	// Simulate process restart: rebuild runtime from the same FileStore paths.
	// Issue a fresh ticket that still carries the pre-reward snapshot so the
	// account-store rematerialization path is exercised instead of ticket state.
	staleTicketStore := loginticket.NewFileStore(ticketDir)
	const postRestartLoginKey = uint32(0x1a1a1a1a)
	issuePeerTicket(t, staleTicketStore, login, postRestartLoginKey, peer)
	staleTicket, err := staleTicketStore.Load(login, postRestartLoginKey)
	if err != nil {
		t.Fatalf("load stale post-restart ticket: %v", err)
	}
	if len(staleTicket.Characters) != 1 || staleTicket.Characters[0].Gold != 25 || len(staleTicket.Characters[0].Inventory) != 0 {
		t.Fatalf("expected stale post-restart ticket to keep pre-reward gold/inventory, got %+v", staleTicket.Characters)
	}

	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloadedInteractions := interactionstore.NewFileStore(interactionPath)
	reloadedItems := itemcatalog.NewFileStore(itemTemplatePath)
	reloaded, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg, staleTicketStore, reloadedAccounts, reloadedInteractions, reloadedItems)
	if err != nil {
		t.Fatalf("reload runtime after quest-flag reward daemon restart: %v", err)
	}
	if _, ok := reloaded.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin"); !ok {
		t.Fatal("expected quest-flag restart-reward static actor registration after daemon restart")
	}

	restartFlow, restartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	if len(restartEnter) < 5 {
		t.Fatalf("expected rematerialized EnterGame bootstrap after daemon restart, got %d frames", len(restartEnter))
	}

	currencySnapshot, ok := reloaded.CurrencySnapshot(peer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after daemon restart rematerialization")
	}
	if currencySnapshot.Gold != 125 {
		t.Fatalf("expected rematerialized gold 125 after daemon restart, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := reloaded.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[0].Count != 1 {
		t.Fatalf("expected rematerialized inventory grant after daemon restart, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}

	reloadedQuest, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state after daemon restart: %v", err)
	}
	if want := (queststate.Snapshot{Flags: []queststate.Flag{}}); !reflect.DeepEqual(reloadedQuest, want) {
		t.Fatalf("unexpected quest-state after daemon restart:\n got: %#v\nwant: %#v", reloadedQuest, want)
	}

	// Turn-in must stay idempotent: cleared quest flag should not grant again.
	actors := reloaded.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one static actor after daemon restart, got %+v", actors)
	}
	repeatOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actors[0].EntityID)})))
	if err != nil {
		t.Fatalf("unexpected post-restart quest-flag interaction error: %v", err)
	}
	if len(repeatOut) != 1 {
		t.Fatalf("expected one requirements-not-met info frame after daemon restart, got %d", len(repeatOut))
	}
	repeatDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, repeatOut[0]))
	if err != nil || repeatDelivery.Type != chatproto.ChatTypeInfo || repeatDelivery.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected post-restart quest-flag rejection delivery: %+v err=%v", repeatDelivery, err)
	}
	if currency, ok := reloaded.CurrencySnapshot(peer.Name); !ok || currency.Gold != 125 {
		t.Fatalf("expected gold to remain 125 after rejected post-restart turn-in, ok=%v snapshot=%+v", ok, currency)
	}
	inventoryAfterReject, ok := reloaded.InventorySnapshot(peer.Name)
	if !ok || len(inventoryAfterReject.Inventory) != 1 || inventoryAfterReject.Inventory[0].Vnum != 27001 || inventoryAfterReject.Inventory[0].Count != 1 {
		t.Fatalf("expected inventory to remain rematerialized after rejected post-restart turn-in, ok=%v snapshot=%+v", ok, inventoryAfterReject)
	}
}

// TestGameRuntimeQuestFlagConsumeExperienceStateRematerializesAcrossDaemonRestart
// proves the Track E.4 crash/restart rematerialization contract for the durable
// PvE experience delta owned by a successful quest_flag turn-in that both debits
// consume_experience and grants reward_experience: after the turn-in, a fresh
// gameRuntime rebuilt from the same FileStore paths rematerializes the net
// experience / gold / inventory state on EnterGame even when the post-restart
// login ticket still carries the pre-turn-in character snapshot.
//
// This deliberately does not cover pending ground-item / ground-gold restart
// durability (still deferred for migration 0010).
func TestGameRuntimeQuestFlagConsumeExperienceStateRematerializesAcrossDaemonRestart(t *testing.T) {
	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	itemTemplatePath := filepath.Join(t.TempDir(), "item-templates.json")

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)

	peer := peerVisibilityCharacter("QuestHero", 0x0103012c, 0x0204012c, 1100, 2100, 0, 101, 201)
	peer.Gold = 40
	peer.Points[bootstrapExperiencePointType] = 40
	peer.Inventory = []inventory.ItemInstance{{ID: 73, Vnum: 27001, Count: 1, Slot: 0}}
	const (
		login    = "quest-hero-restart-consume-exp"
		loginKey = uint32(0x1b1b1b1b)
	)
	issuePeerTicket(t, ticketStore, login, loginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag restart-consume-experience account: %v", err)
	}

	interactionPath := filepath.Join(t.TempDir(), "interaction-definitions.json")
	interactionStore := interactionstore.NewFileStore(interactionPath)
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{{
		Kind:              interactionstore.KindQuestFlag,
		Ref:               "quest:first_steps_kill_turnin",
		Text:              "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:          "quest:first_steps",
		QuestFlag:         "killed_qa_mob",
		QuestFrom:         1,
		QuestTo:           0,
		RewardGold:        100,
		RewardExperience:  50,
		RewardItems:       []interactionstore.RewardItemEntry{{ItemVnum: 11200, Count: 1}},
		ConsumeItems:      []interactionstore.RewardItemEntry{{ItemVnum: 27001, Count: 1}},
		ConsumeGold:       25,
		ConsumeExperience: 10,
	}}}); err != nil {
		t.Fatalf("seed quest-flag restart-consume-experience interactions: %v", err)
	}
	itemStore := itemcatalog.NewFileStore(itemTemplatePath)
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag restart-consume-experience templates: %v", err)
	}
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for restart-consume-experience turn-in: %v", err)
	}

	cfg := config.Service{
		LegacyAddr:          ":13000",
		PublicAddr:          "127.0.0.1",
		QuestStateStorePath: questStatePath,
	}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected quest-flag restart-consume-experience runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag restart-consume-experience static actor registration to succeed")
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) < 8 {
		t.Fatalf("expected bootstrap frames with visible quest-flag restart-consume-experience actor, got %d", len(enterOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag restart-consume-experience interaction error: %v", err)
	}
	if len(out) != 7 {
		t.Fatalf("expected chat + consume-gold + consume-experience + reward-gold + reward-experience + consume + reward frames for quest-flag restart-consume-experience interaction, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected quest-flag restart-consume-experience chat delivery: %+v err=%v", delivery, err)
	}
	consumeGoldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
	if err != nil || consumeGoldChange.Amount != -25 || consumeGoldChange.Value != 15 {
		t.Fatalf("unexpected quest-flag restart-consume-experience gold debit frame: %+v err=%v", consumeGoldChange, err)
	}
	consumeExperienceChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[2]))
	if err != nil || consumeExperienceChange.Type != bootstrapExperiencePointType || consumeExperienceChange.Amount != -10 || consumeExperienceChange.Value != 30 {
		t.Fatalf("unexpected quest-flag restart-consume-experience debit frame: %+v err=%v", consumeExperienceChange, err)
	}
	rewardGoldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[3]))
	if err != nil || rewardGoldChange.Amount != 100 || rewardGoldChange.Value != 115 {
		t.Fatalf("unexpected quest-flag restart-consume-experience reward-gold frame: %+v err=%v", rewardGoldChange, err)
	}
	rewardExperienceChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[4]))
	if err != nil || rewardExperienceChange.Type != bootstrapExperiencePointType || rewardExperienceChange.Amount != 50 || rewardExperienceChange.Value != 80 {
		t.Fatalf("unexpected quest-flag restart-consume-experience reward-experience frame: %+v err=%v", rewardExperienceChange, err)
	}
	consumeDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[5]))
	if err != nil || consumeDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected quest-flag restart-consume-experience delete frame: %+v err=%v", consumeDel, err)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[6]))
	if err != nil || itemSet.Position != itemproto.InventoryPosition(0) || itemSet.Vnum != 11200 || itemSet.Count != 1 {
		t.Fatalf("unexpected quest-flag restart-consume-experience reward set frame: %+v err=%v", itemSet, err)
	}
	closeSessionFlow(t, flow)

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted quest-flag restart-consume-experience account before daemon restart: %v", err)
	}
	if account.Characters[0].Gold != 115 {
		t.Fatalf("expected persisted gold 115 before daemon restart, got %d", account.Characters[0].Gold)
	}
	if account.Characters[0].Points[bootstrapExperiencePointType] != 80 {
		t.Fatalf("expected persisted experience 80 before daemon restart, got %d", account.Characters[0].Points[bootstrapExperiencePointType])
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 11200 || account.Characters[0].Inventory[0].Count != 1 {
		t.Fatalf("expected persisted inventory reward before daemon restart, got %+v", account.Characters[0].Inventory)
	}
	loadedQuest, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state before daemon restart: %v", err)
	}
	if want := (queststate.Snapshot{Flags: []queststate.Flag{}}); !reflect.DeepEqual(loadedQuest, want) {
		t.Fatalf("unexpected persisted quest-state before daemon restart:\n got: %#v\nwant: %#v", loadedQuest, want)
	}

	// Simulate process restart: rebuild runtime from the same FileStore paths.
	// Issue a fresh ticket that still carries the pre-turn-in snapshot so the
	// account-store rematerialization path is exercised instead of ticket state.
	staleTicketStore := loginticket.NewFileStore(ticketDir)
	const postRestartLoginKey = uint32(0x1c1c1c1c)
	issuePeerTicket(t, staleTicketStore, login, postRestartLoginKey, peer)
	staleTicket, err := staleTicketStore.Load(login, postRestartLoginKey)
	if err != nil {
		t.Fatalf("load stale post-restart ticket: %v", err)
	}
	if len(staleTicket.Characters) != 1 ||
		staleTicket.Characters[0].Gold != 40 ||
		staleTicket.Characters[0].Points[bootstrapExperiencePointType] != 40 ||
		len(staleTicket.Characters[0].Inventory) != 1 ||
		staleTicket.Characters[0].Inventory[0].Vnum != 27001 {
		t.Fatalf("expected stale post-restart ticket to keep pre-turn-in gold/experience/inventory, got %+v", staleTicket.Characters)
	}

	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloadedInteractions := interactionstore.NewFileStore(interactionPath)
	reloadedItems := itemcatalog.NewFileStore(itemTemplatePath)
	reloaded, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg, staleTicketStore, reloadedAccounts, reloadedInteractions, reloadedItems)
	if err != nil {
		t.Fatalf("reload runtime after quest-flag consume-experience daemon restart: %v", err)
	}
	if _, ok := reloaded.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin"); !ok {
		t.Fatal("expected quest-flag restart-consume-experience static actor registration after daemon restart")
	}

	restartFlow, restartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	if len(restartEnter) < 5 {
		t.Fatalf("expected rematerialized EnterGame bootstrap after daemon restart, got %d frames", len(restartEnter))
	}

	currencySnapshot, ok := reloaded.CurrencySnapshot(peer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after daemon restart rematerialization")
	}
	if currencySnapshot.Gold != 115 {
		t.Fatalf("expected rematerialized gold 115 after daemon restart, got %+v", currencySnapshot)
	}
	pointsSnapshot, ok := reloaded.PointsSnapshot(peer.Name)
	if !ok || pointsSnapshot.Points[bootstrapExperiencePointType] != 80 {
		t.Fatalf("expected rematerialized experience 80 after daemon restart, ok=%v snapshot=%+v", ok, pointsSnapshot)
	}
	inventorySnapshot, ok := reloaded.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Count != 1 {
		t.Fatalf("expected rematerialized inventory reward after daemon restart, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}

	reloadedQuest, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state after daemon restart: %v", err)
	}
	if want := (queststate.Snapshot{Flags: []queststate.Flag{}}); !reflect.DeepEqual(reloadedQuest, want) {
		t.Fatalf("unexpected quest-state after daemon restart:\n got: %#v\nwant: %#v", reloadedQuest, want)
	}

	// Turn-in must stay idempotent: cleared quest flag should not debit/grant again.
	actors := reloaded.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one static actor after daemon restart, got %+v", actors)
	}
	repeatOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actors[0].EntityID)})))
	if err != nil {
		t.Fatalf("unexpected post-restart quest-flag interaction error: %v", err)
	}
	if len(repeatOut) != 1 {
		t.Fatalf("expected one requirements-not-met info frame after daemon restart, got %d", len(repeatOut))
	}
	repeatDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, repeatOut[0]))
	// After the successful turn-in consumed the material fee, the owned insufficient-
	// materials preflight rejects before CAS mismatch chat, so the idempotent retry
	// surfaces the fee-specific string rather than the generic requirements message.
	if err != nil || repeatDelivery.Type != chatproto.ChatTypeInfo || repeatDelivery.Message != questFlagInsufficientMaterialsInfoMessage {
		t.Fatalf("unexpected post-restart quest-flag rejection delivery: %+v err=%v", repeatDelivery, err)
	}
	if currency, ok := reloaded.CurrencySnapshot(peer.Name); !ok || currency.Gold != 115 {
		t.Fatalf("expected gold to remain 115 after rejected post-restart turn-in, ok=%v snapshot=%+v", ok, currency)
	}
	if points, ok := reloaded.PointsSnapshot(peer.Name); !ok || points.Points[bootstrapExperiencePointType] != 80 {
		t.Fatalf("expected experience to remain 80 after rejected post-restart turn-in, ok=%v snapshot=%+v", ok, points)
	}
	inventoryAfterReject, ok := reloaded.InventorySnapshot(peer.Name)
	if !ok || len(inventoryAfterReject.Inventory) != 1 || inventoryAfterReject.Inventory[0].Vnum != 11200 || inventoryAfterReject.Inventory[0].Count != 1 {
		t.Fatalf("expected inventory to remain rematerialized after rejected post-restart turn-in, ok=%v snapshot=%+v", ok, inventoryAfterReject)
	}
}

// TestGameRuntimeEquipmentAndQuickslotsRematerializeAcrossDaemonRestart proves the
// Track E.4 crash/restart rematerialization contract for durable PvE equipment and
// quickslots: after a live equip + quickslot bind, a fresh gameRuntime rebuilt from
// the same FileStore paths rematerializes committed account item-state on EnterGame
// even when the post-restart login ticket still carries the pre-mutation snapshot.
//
// This deliberately does not cover pending ground-item / ground-gold restart
// durability, map/x/y rematerialization, or point-state rematerialization.
func TestGameRuntimeEquipmentAndQuickslotsRematerializeAcrossDaemonRestart(t *testing.T) {
	ticketDir := t.TempDir()
	accountDir := t.TempDir()

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)

	peer := peerVisibilityCharacter("EquipHero", 0x01030131, 0x02040131, 1100, 2100, 0, 101, 201)
	peer.Gold = 50
	peer.Inventory = []inventory.ItemInstance{
		{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8},
		{ID: 1002, Vnum: 27001, Count: 3, Slot: 7},
	}
	peer.Equipment = []inventory.ItemInstance{}
	peer.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1},
	}
	const (
		login    = "equip-hero-restart"
		loginKey = uint32(0x29292929)
	)
	issuePeerTicket(t, ticketStore, login, loginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed equipment/quickslot restart account: %v", err)
	}

	cfg := config.Service{
		LegacyAddr: ":13000",
		PublicAddr: "127.0.0.1",
	}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg, ticketStore, accounts, nil, nil)
	if err != nil {
		t.Fatalf("unexpected equipment/quickslot restart runtime error: %v", err)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) < 5 {
		t.Fatalf("expected EnterGame bootstrap before equipment mutation, got %d frames", len(enterOut))
	}

	bodyPosition, err := itemproto.EquipmentPosition(0)
	if err != nil {
		t.Fatalf("resolve body equipment position: %v", err)
	}
	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(8),
		Destination: bodyPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected equipment restart equip error: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected delete+set+update frames for restart equip, got %d", len(equipOut))
	}

	quickslotOut, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{
		Position: 5,
		Slot:     quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 7},
	})))
	if err != nil {
		t.Fatalf("unexpected equipment restart quickslot add error: %v", err)
	}
	if len(quickslotOut) != 1 {
		t.Fatalf("expected one quickslot add frame before daemon restart, got %d", len(quickslotOut))
	}
	closeSessionFlow(t, flow)

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted equipment/quickslot account before daemon restart: %v", err)
	}
	if account.Characters[0].Gold != 50 {
		t.Fatalf("expected persisted gold 50 before daemon restart, got %d", account.Characters[0].Gold)
	}
	if !reflect.DeepEqual(account.Characters[0].Equipment, []inventory.ItemInstance{
		{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotBody},
	}) {
		t.Fatalf("unexpected persisted equipment before daemon restart: %#v", account.Characters[0].Equipment)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{
		{ID: 1002, Vnum: 27001, Count: 3, Slot: 7},
	}) {
		t.Fatalf("unexpected persisted inventory before daemon restart: %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1},
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 7},
	}) {
		t.Fatalf("unexpected persisted quickslots before daemon restart: %#v", account.Characters[0].Quickslots)
	}

	// Simulate process restart: rebuild runtime from the same FileStore paths.
	// Issue a fresh ticket that still carries the pre-mutation snapshot so the
	// account-store rematerialization path is exercised instead of ticket state.
	staleTicketStore := loginticket.NewFileStore(ticketDir)
	const postRestartLoginKey = uint32(0x2a2a2a2a)
	issuePeerTicket(t, staleTicketStore, login, postRestartLoginKey, peer)
	staleTicket, err := staleTicketStore.Load(login, postRestartLoginKey)
	if err != nil {
		t.Fatalf("load stale post-restart ticket: %v", err)
	}
	if len(staleTicket.Characters) != 1 ||
		len(staleTicket.Characters[0].Equipment) != 0 ||
		len(staleTicket.Characters[0].Inventory) != 2 ||
		len(staleTicket.Characters[0].Quickslots) != 1 {
		t.Fatalf("expected stale post-restart ticket to keep pre-mutation item state, got %+v", staleTicket.Characters)
	}

	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloaded, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(cfg, staleTicketStore, reloadedAccounts, nil, nil)
	if err != nil {
		t.Fatalf("reload runtime after equipment/quickslot daemon restart: %v", err)
	}

	restartFlow, restartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	if len(restartEnter) < 5 {
		t.Fatalf("expected rematerialized EnterGame bootstrap after daemon restart, got %d frames", len(restartEnter))
	}

	currencySnapshot, ok := reloaded.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 50 {
		t.Fatalf("expected rematerialized gold 50 after daemon restart, ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	inventorySnapshot, ok := reloaded.InventorySnapshot(peer.Name)
	if !ok || !reflect.DeepEqual(inventorySnapshot.Inventory, []InventoryItemSnapshot{
		{ID: 1002, Vnum: 27001, Count: 3, Slot: 7},
	}) {
		t.Fatalf("expected rematerialized inventory after daemon restart, ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	equipmentSnapshot, ok := reloaded.EquipmentSnapshot(peer.Name)
	if !ok || !reflect.DeepEqual(equipmentSnapshot.Equipment, []EquipmentItemSnapshot{
		{ID: 1001, Vnum: 0x11223344, Count: 1, EquipSlot: inventory.EquipmentSlotBody.String()},
	}) {
		t.Fatalf("expected rematerialized equipment after daemon restart, ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}
	quickslotsSnapshot, ok := reloaded.QuickslotsSnapshot(peer.Name)
	if !ok || !reflect.DeepEqual(quickslotsSnapshot.Quickslots, []QuickslotSnapshot{
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 1},
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 7},
	}) {
		t.Fatalf("expected rematerialized quickslots after daemon restart, ok=%v snapshot=%+v", ok, quickslotsSnapshot)
	}

	// Post-restart mutation must persist against rematerialized state.
	deleteOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientDel(quickslotproto.ClientDelPacket{Position: 2})))
	if err != nil {
		t.Fatalf("unexpected post-restart quickslot delete error: %v", err)
	}
	if len(deleteOut) != 1 {
		t.Fatalf("expected one quickslot delete frame after daemon restart, got %d", len(deleteOut))
	}
	accountAfterDelete, err := reloadedAccounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart quickslot delete: %v", err)
	}
	if !reflect.DeepEqual(accountAfterDelete.Characters[0].Quickslots, []loginticket.Quickslot{
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 7},
	}) {
		t.Fatalf("unexpected persisted quickslots after post-restart delete: %#v", accountAfterDelete.Characters[0].Quickslots)
	}
	if !reflect.DeepEqual(accountAfterDelete.Characters[0].Equipment, []inventory.ItemInstance{
		{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotBody},
	}) {
		t.Fatalf("expected equipment to remain rematerialized after post-restart quickslot delete: %#v", accountAfterDelete.Characters[0].Equipment)
	}
	liveQuickslots, ok := reloaded.QuickslotsSnapshot(peer.Name)
	if !ok || !reflect.DeepEqual(liveQuickslots.Quickslots, []QuickslotSnapshot{
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 7},
	}) {
		t.Fatalf("expected live quickslots to match post-restart delete, ok=%v snapshot=%+v", ok, liveQuickslots)
	}
}

// TestGameRuntimePositionAndPointsRematerializeAcrossDaemonRestart proves the
// Track E.4 crash/restart rematerialization contract for durable PvE map/x/y and
// character point-state: after a live item-use point mutation and a transfer-backed
// location change, a fresh gameRuntime rebuilt from the same FileStore paths
// rematerializes committed account position and points on EnterGame even when the
// post-restart login ticket still carries the pre-mutation snapshot.
//
// This deliberately does not cover pending ground-item / ground-gold restart
// durability.
func TestGameRuntimePositionAndPointsRematerializeAcrossDaemonRestart(t *testing.T) {
	ticketDir := t.TempDir()
	accountDir := t.TempDir()

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)

	peer := peerVisibilityCharacter("PointHero", 0x01030141, 0x02040141, 1100, 2100, 0, 101, 201)
	peer.Gold = 40
	peer.Points[bootstrapPlayerPointValueIndex] = 700
	peer.Points[bootstrapExperiencePointType] = 25
	peer.Inventory = []inventory.ItemInstance{
		{ID: 2001, Vnum: 27001, Count: 2, Slot: 5},
	}
	const (
		login    = "point-hero-restart"
		loginKey = uint32(0x39393939)
	)
	issuePeerTicket(t, ticketStore, login, loginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed position/points restart account: %v", err)
	}

	const (
		transferSourceX   = int32(1500)
		transferSourceY   = int32(2600)
		transferTargetMap = uint32(42)
		transferTargetX   = int32(1700)
		transferTargetY   = int32(2800)
	)
	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{
		LegacyAddr: ":13000",
		PublicAddr: "127.0.0.1",
	}, ticketStore, accounts, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        transferSourceX,
		SourceY:        transferSourceY,
		TargetMapIndex: transferTargetMap,
		TargetX:        transferTargetX,
		TargetY:        transferTargetY,
	}})
	if err != nil {
		t.Fatalf("unexpected position/points restart runtime error: %v", err)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) < 5 {
		t.Fatalf("expected EnterGame bootstrap before position/points mutation, got %d frames", len(enterOut))
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected position/points restart item-use error: %v", err)
	}
	if len(useOut) < 3 {
		t.Fatalf("expected item-use frames before daemon restart, got %d", len(useOut))
	}

	transferOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    transferSourceX,
		Y:    transferSourceY,
		Time: 0x01020304,
	})))
	if err != nil {
		t.Fatalf("unexpected position/points restart transfer move error: %v", err)
	}
	if len(transferOut) == 0 {
		t.Fatal("expected transfer-backed move to emit location-change frames before daemon restart")
	}
	closeSessionFlow(t, flow)

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted position/points account before daemon restart: %v", err)
	}
	if account.Characters[0].MapIndex != transferTargetMap || account.Characters[0].X != transferTargetX || account.Characters[0].Y != transferTargetY {
		t.Fatalf("expected persisted location map=%d x=%d y=%d before daemon restart, got map=%d x=%d y=%d",
			transferTargetMap, transferTargetX, transferTargetY,
			account.Characters[0].MapIndex, account.Characters[0].X, account.Characters[0].Y)
	}
	if got := account.Characters[0].Points[bootstrapPlayerPointValueIndex]; got != 750 {
		t.Fatalf("expected persisted points[1]=750 before daemon restart, got %d", got)
	}
	if got := account.Characters[0].Points[bootstrapExperiencePointType]; got != 25 {
		t.Fatalf("expected persisted experience points to stay 25 before daemon restart, got %d", got)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{
		{ID: 2001, Vnum: 27001, Count: 1, Slot: 5},
	}) {
		t.Fatalf("unexpected persisted inventory before daemon restart: %#v", account.Characters[0].Inventory)
	}

	// Simulate process restart: rebuild runtime from the same FileStore paths.
	// Issue a fresh ticket that still carries the pre-mutation snapshot so the
	// account-store rematerialization path is exercised instead of ticket state.
	staleTicketStore := loginticket.NewFileStore(ticketDir)
	const postRestartLoginKey = uint32(0x3a3a3a3a)
	issuePeerTicket(t, staleTicketStore, login, postRestartLoginKey, peer)
	staleTicket, err := staleTicketStore.Load(login, postRestartLoginKey)
	if err != nil {
		t.Fatalf("load stale post-restart ticket: %v", err)
	}
	if len(staleTicket.Characters) != 1 ||
		staleTicket.Characters[0].MapIndex != bootstrapMapIndex ||
		staleTicket.Characters[0].X != 1100 ||
		staleTicket.Characters[0].Y != 2100 ||
		staleTicket.Characters[0].Points[bootstrapPlayerPointValueIndex] != 700 ||
		len(staleTicket.Characters[0].Inventory) != 1 ||
		staleTicket.Characters[0].Inventory[0].Count != 2 {
		t.Fatalf("expected stale post-restart ticket to keep pre-mutation location/points/inventory, got %+v", staleTicket.Characters)
	}

	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloaded, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{
		LegacyAddr: ":13000",
		PublicAddr: "127.0.0.1",
	}, staleTicketStore, reloadedAccounts, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        transferSourceX,
		SourceY:        transferSourceY,
		TargetMapIndex: transferTargetMap,
		TargetX:        transferTargetX,
		TargetY:        transferTargetY,
	}})
	if err != nil {
		t.Fatalf("reload runtime after position/points daemon restart: %v", err)
	}

	restartFlow, restartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	if len(restartEnter) < 5 {
		t.Fatalf("expected rematerialized EnterGame bootstrap after daemon restart, got %d frames", len(restartEnter))
	}

	connected, ok := reloaded.ConnectedCharacterSnapshot(peer.Name)
	if !ok {
		t.Fatal("expected connected character snapshot after daemon restart rematerialization")
	}
	if connected.MapIndex != transferTargetMap || connected.X != transferTargetX || connected.Y != transferTargetY {
		t.Fatalf("expected rematerialized location map=%d x=%d y=%d after daemon restart, got %+v",
			transferTargetMap, transferTargetX, transferTargetY, connected)
	}

	pointsSnapshot, ok := reloaded.PointsSnapshot(peer.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != 750 {
		t.Fatalf("expected rematerialized points[1]=750 after daemon restart, ok=%v snapshot=%+v", ok, pointsSnapshot)
	}
	if pointsSnapshot.Points[bootstrapExperiencePointType] != 25 {
		t.Fatalf("expected rematerialized experience points to stay 25 after daemon restart, got %d", pointsSnapshot.Points[bootstrapExperiencePointType])
	}

	inventorySnapshot, ok := reloaded.InventorySnapshot(peer.Name)
	if !ok || !reflect.DeepEqual(inventorySnapshot.Inventory, []InventoryItemSnapshot{
		{ID: 2001, Vnum: 27001, Count: 1, Slot: 5},
	}) {
		t.Fatalf("expected rematerialized inventory after daemon restart, ok=%v snapshot=%+v", ok, inventorySnapshot)
	}

	// Post-restart mutation must persist against rematerialized location/points.
	useAgainOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart item-use error: %v", err)
	}
	if len(useAgainOut) < 3 {
		t.Fatalf("expected post-restart item-use frames, got %d", len(useAgainOut))
	}
	accountAfterUse, err := reloadedAccounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart item use: %v", err)
	}
	if accountAfterUse.Characters[0].MapIndex != transferTargetMap || accountAfterUse.Characters[0].X != transferTargetX || accountAfterUse.Characters[0].Y != transferTargetY {
		t.Fatalf("expected rematerialized location to survive post-restart item use, got map=%d x=%d y=%d",
			accountAfterUse.Characters[0].MapIndex, accountAfterUse.Characters[0].X, accountAfterUse.Characters[0].Y)
	}
	if got := accountAfterUse.Characters[0].Points[bootstrapPlayerPointValueIndex]; got != 800 {
		t.Fatalf("expected persisted points[1]=800 after post-restart item use, got %d", got)
	}
	if len(accountAfterUse.Characters[0].Inventory) != 0 {
		t.Fatalf("expected inventory to empty after final post-restart item use, got %#v", accountAfterUse.Characters[0].Inventory)
	}
	livePoints, ok := reloaded.PointsSnapshot(peer.Name)
	if !ok || livePoints.Points[bootstrapPlayerPointValueIndex] != 800 {
		t.Fatalf("expected live points[1]=800 after post-restart item use, ok=%v snapshot=%+v", ok, livePoints)
	}
}

// TestGameRuntimePlayerDeathFloorRematerializesAcrossDaemonRestart proves the
// Track E.4 crash/restart rematerialization contract for the retaliation-owned
// player death floor: after immediate practice-mob retaliation persists
// points[1]=0, a fresh gameRuntime rebuilt from the same FileStore paths
// rematerializes that dead snapshot on EnterGame even when the post-restart
// login ticket still carries the pre-death live HP value.
//
// This deliberately does not cover pending ground-item / ground-gold restart
// durability (covered separately by TestGameRuntimePendingGroundItemAndGoldRematerializeAcrossDaemonRestart).
func TestGameRuntimePlayerDeathFloorRematerializesAcrossDaemonRestart(t *testing.T) {
	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	staticActorPath := filepath.Join(t.TempDir(), "static-actors.json")
	interactionPath := filepath.Join(t.TempDir(), "interaction-definitions.json")

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)
	staticActorStore := staticstore.NewFileStore(staticActorPath)
	interactionStore := interactionstore.NewFileStore(interactionPath)

	owner := peerVisibilityCharacter("DeathFloorHero", 0x01030151, 0x02040151, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	const (
		login    = "death-floor-hero-restart"
		loginKey = uint32(0x51515151)
	)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed death-floor restart account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{
		LegacyAddr: ":13000",
		PublicAddr: "127.0.0.1",
	}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected death-floor restart runtime error: %v", err)
	}
	currentTime := time.Unix(1700000485, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_death_floor_restart",
		Name:          "PracticeMobDeathFloorRestart",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import death-floor restart spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice mob after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible practice mob before death floor, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before death-floor daemon restart: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before death-floor daemon restart, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before death-floor daemon restart: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate target-refresh, point-loss, self dead, and clear-target frames before death-floor daemon restart, got %d", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before death-floor daemon restart: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected immediate retaliation to reach owner HP floor before daemon restart, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode immediate retaliation self dead before death-floor daemon restart: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected immediate retaliation self dead for owner vid %d, got %+v", owner.VID, dead)
	}

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted death-floor account before daemon restart: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner before daemon restart, got %+v", persisted)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected immediate retaliation floor to persist points[%d]=0 before daemon restart, got %d", bootstrapPlayerPointValueIndex, persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	closeSessionFlow(t, flow)

	// Simulate process restart: rebuild runtime from the same FileStore paths.
	// Issue a fresh ticket that still carries the pre-death live HP so the
	// account-store rematerialization path is exercised instead of ticket state.
	staleTicketStore := loginticket.NewFileStore(ticketDir)
	const postRestartLoginKey = uint32(0x52525252)
	issuePeerTicket(t, staleTicketStore, login, postRestartLoginKey, owner)
	staleTicket, err := staleTicketStore.Load(login, postRestartLoginKey)
	if err != nil {
		t.Fatalf("load stale post-restart ticket: %v", err)
	}
	if len(staleTicket.Characters) != 1 || staleTicket.Characters[0].Points[bootstrapPlayerPointValueIndex] != 1 {
		t.Fatalf("expected stale post-restart ticket to keep pre-death points[1]=1, got %+v", staleTicket.Characters)
	}

	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloadedStaticActors := staticstore.NewFileStore(staticActorPath)
	reloadedInteractions := interactionstore.NewFileStore(interactionPath)
	reloaded, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{
		LegacyAddr: ":13000",
		PublicAddr: "127.0.0.1",
	}, staleTicketStore, reloadedAccounts, reloadedStaticActors, reloadedInteractions)
	if err != nil {
		t.Fatalf("reload runtime after death-floor daemon restart: %v", err)
	}
	reloaded.now = func() time.Time { return currentTime }

	restartFlow, restartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	// Dead-owner EnterGame matches the already-owned reconnect contract: ordinary
	// selected-character bootstrap plus self DEAD, with non-player visibility
	// skipped for the still-dead recipient (6 frames total).
	if len(restartEnter) != 6 {
		t.Fatalf("expected 6 bootstrap frames including rematerialized self DEAD after daemon restart, got %d", len(restartEnter))
	}
	restartPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, restartEnter[4]))
	if err != nil {
		t.Fatalf("decode rematerialized bootstrap point-change after death-floor daemon restart: %v", err)
	}
	if restartPointChange.Value != 0 || restartPointChange.Amount != 0 {
		t.Fatalf("expected rematerialized points[%d] floor 0 after daemon restart, got %+v", bootstrapPlayerPointValueIndex, restartPointChange)
	}
	restartDead, err := worldproto.DecodeDead(decodeSingleFrame(t, restartEnter[5]))
	if err != nil {
		t.Fatalf("decode rematerialized bootstrap dead replay after death-floor daemon restart: %v", err)
	}
	if restartDead.VID != owner.VID {
		t.Fatalf("expected rematerialized dead replay for owner vid %d after daemon restart, got %+v", owner.VID, restartDead)
	}

	connected, ok := reloaded.ConnectedCharacterSnapshot(owner.Name)
	if !ok || !connected.Dead {
		t.Fatalf("expected connected character snapshot dead=true after daemon restart rematerialization, ok=%v snapshot=%+v", ok, connected)
	}
	pointsSnapshot, ok := reloaded.PointsSnapshot(owner.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected rematerialized points[1]=0 after daemon restart, ok=%v snapshot=%+v", ok, pointsSnapshot)
	}

	accountAfterRestart, err := reloadedAccounts.Load(login)
	if err != nil {
		t.Fatalf("load account after death-floor daemon restart rematerialization: %v", err)
	}
	if accountAfterRestart.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected persisted death floor to survive rematerialization, got %d", accountAfterRestart.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	actorsAfterRestart := reloaded.StaticActors()
	if len(actorsAfterRestart) != 1 {
		t.Fatalf("expected rematerialized practice mob after daemon restart, got %#v", actorsAfterRestart)
	}
	denyOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{
		TargetVID: uint32(actorsAfterRestart[0].EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart dead-owner target error: %v", err)
	}
	if len(denyOut) != 0 {
		t.Fatalf("expected rematerialized dead owner combat TARGET to fail closed, got %d frames", len(denyOut))
	}
}

// TestGameRuntimePendingGroundItemAndGoldRematerializeAcrossDaemonRestart proves the
// Track E.4 crash/restart rematerialization contract for pending bootstrap ground
// item/gold handles: after register + FileStore persist, a fresh gameRuntime rebuilt
// from the same GroundItemStorePath rematerializes both shapes with absolute timers,
// keeps mid-window exclusive ownership identity-keyed for the owner, and blocks a peer.
func TestGameRuntimePendingGroundItemAndGoldRematerializeAcrossDaemonRestart(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	groundItemPath := filepath.Join(t.TempDir(), "ground-items.json")

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)

	owner := peerVisibilityCharacter("GroundRestartHero", 0x01030171, 0x02040171, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("GroundRestartPeer", 0x01030172, 0x02040172, 1110, 2110, 0, 101, 201)
	const (
		ownerLogin = "ground-restart-hero"
		peerLogin  = "ground-restart-peer"
		ownerKey   = uint32(0x71717171)
		peerKey    = uint32(0x72727272)
		itemVID    = uint32(0x07000071)
		goldVID    = uint32(0x07000072)
	)
	issuePeerTicket(t, ticketStore, ownerLogin, ownerKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerKey, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed ground-restart owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed ground-restart peer account: %v", err)
	}

	cfg := config.Service{
		PprofAddr:           "127.0.0.1:6060",
		LegacyAddr:          ":13000",
		PublicAddr:          "127.0.0.1",
		LoginTicketStoreDir: ticketDir,
		AccountStoreDir:     accountDir,
		GroundItemStorePath: groundItemPath,
	}
	runtime, err := NewGameRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected ground-restart runtime error: %v", err)
	}
	currentTime := time.Now().UTC().Truncate(time.Second)
	runtime.now = func() time.Time { return currentTime }
	runtime.sharedWorld.now = runtime.now

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, ownerKey)
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName(owner.Name)
	if !ok || ownerEntity.Entity.ID == 0 {
		t.Fatal("expected owner shared-world entity id after enter-game")
	}
	ownerID := ownerEntity.Entity.ID
	if !runtime.sharedWorld.RegisterGroundItemWithPickupRange(ownerID, ownerLogin, owner, itemVID, inventory.ItemInstance{ID: 0x30010071, Vnum: 27001, Count: 2}, 450) {
		t.Fatal("expected ground-item registration before daemon restart")
	}
	if !runtime.sharedWorld.RegisterGroundGoldWithPickupRange(ownerID, ownerLogin, owner, goldVID, 75, 300) {
		t.Fatal("expected ground-gold registration before daemon restart")
	}
	persisted, err := worldruntime.NewGroundItemFileStore(groundItemPath).Load()
	if err != nil {
		t.Fatalf("load persisted ground items before restart: %v", err)
	}
	if len(persisted.GroundItems) != 2 {
		t.Fatalf("expected 2 persisted pending ground handles before restart, got %#v", persisted.GroundItems)
	}
	closeSessionFlow(t, ownerFlow)

	reloaded, err := NewGameRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected post-restart ground rematerialize runtime error: %v", err)
	}
	reloaded.now = func() time.Time { return currentTime.Add(5 * time.Second) }
	reloaded.sharedWorld.now = reloaded.now

	snapshot := reloaded.sharedWorld.DurableGroundItemSnapshot()
	if len(snapshot.GroundItems) != 2 {
		t.Fatalf("expected durable rematerialized snapshot with 2 rows, got %#v", snapshot.GroundItems)
	}
	itemRow := snapshot.GroundItems[0]
	goldRow := snapshot.GroundItems[1]
	if itemRow.VID != itemVID || itemRow.ItemID != 0x30010071 || itemRow.ItemCount == nil || *itemRow.ItemCount != 2 || !itemRow.OwnershipExclusive {
		t.Fatalf("unexpected rematerialized item row: %#v", itemRow)
	}
	if goldRow.VID != goldVID || goldRow.GoldAmount == nil || *goldRow.GoldAmount != 75 || !goldRow.OwnershipExclusive {
		t.Fatalf("unexpected rematerialized gold row: %#v", goldRow)
	}

	ownerRestartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), ownerLogin, ownerKey)
	peerRestartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), peerLogin, peerKey)
	ownerEntity, ok = reloaded.sharedWorld.playerEntityByName(owner.Name)
	peerEntity, peerOK := reloaded.sharedWorld.playerEntityByName(peer.Name)
	if !ok || !peerOK || ownerEntity.Entity.ID == 0 || peerEntity.Entity.ID == 0 {
		t.Fatalf("expected rematerialized owner/peer entity ids, ownerOK=%v peerOK=%v", ok, peerOK)
	}
	ownerID = ownerEntity.Entity.ID
	peerID := peerEntity.Entity.ID

	items, mapOK := reloaded.GroundItemsForMap(bootstrapMapIndex)
	if !mapOK || len(items) != 2 {
		t.Fatalf("expected rematerialized pending ground handles visible on occupied map after rejoin, ok=%v items=%#v", mapOK, items)
	}

	if _, ok := reloaded.sharedWorld.GroundItemPickupFor(peerID, peer, itemVID); ok {
		t.Fatal("expected rematerialized exclusive ownership to block peer mid-window")
	}
	if pickup, ok := reloaded.sharedWorld.GroundItemPickupFor(ownerID, owner, itemVID); !ok || pickup.Item.ID != 0x30010071 || pickup.Item.Count != 2 {
		t.Fatalf("expected rematerialized exclusive ownership to allow owner pickup, ok=%v pickup=%+v", ok, pickup)
	}
	if !reloaded.sharedWorld.RemoveGroundItem(ownerID, owner, itemVID) {
		t.Fatal("expected owner to remove rematerialized ground item")
	}
	afterPickup, err := worldruntime.NewGroundItemFileStore(groundItemPath).Load()
	if err != nil {
		t.Fatalf("load ground items after rematerialized pickup: %v", err)
	}
	if len(afterPickup.GroundItems) != 1 || afterPickup.GroundItems[0].VID != goldVID {
		t.Fatalf("expected only gold handle to remain after item pickup, got %#v", afterPickup.GroundItems)
	}

	status := reloaded.PersistenceStatus()
	if !status.GroundItemStore.Valid || status.GroundItemStore.Path != groundItemPath {
		t.Fatalf("expected ground item persistence status path/valid, got %#v", status.GroundItemStore)
	}
	if status.GroundItemStore.Summary.GroundItemCount != 1 || status.GroundItemStore.Summary.GoldShapedCount != 1 {
		t.Fatalf("unexpected ground item persistence summary: %#v", status.GroundItemStore.Summary)
	}

	closeSessionFlow(t, ownerRestartFlow)
	closeSessionFlow(t, peerRestartFlow)
}
