package minimal

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
)

// TestGameRuntimeQuestFlagRewardStateRematerializesAcrossDaemonRestart proves the
// Track E.4 crash/restart rematerialization contract for PvE character gold,
// inventory, and quest flags: after a successful quest_flag turn-in, a fresh
// gameRuntime rebuilt from the same FileStore paths rematerializes committed
// account/quest state on EnterGame even when the post-restart login ticket still
// carries the pre-reward character snapshot.
//
// This deliberately does not cover pending ground-item / ground-gold restart
// durability (still deferred for migration 0010).
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
