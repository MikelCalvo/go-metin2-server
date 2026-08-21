package minimal

import (
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
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func TestGameSessionFlowStaticActorQuestFlagConsumeItemsRemovesInventoryAndGrantsReward(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("QuestHero", 0x01030118, 0x02040118, 1100, 2100, 0, 101, 201)
	peer.Gold = 25
	peer.Inventory = []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 1, Slot: 0}}
	issuePeerTicket(t, ticketStore, "qf-consume-items", 0x1c1c1c1c, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-consume-items", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag consume-items account: %v", err)
	}
	interactionStore := interactionstore.NewMemoryStore()
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{{
		Kind:       interactionstore.KindQuestFlag,
		Ref:        "quest:first_steps_kill_turnin",
		Text:       "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:   "quest:first_steps",
		QuestFlag:  "killed_qa_mob",
		QuestFrom:  1,
		QuestTo:    0,
		RewardGold: 100,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 11200, Count: 1},
		},
		ConsumeItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
		},
		ConsumeGold: 25,
	}}}); err != nil {
		t.Fatalf("seed quest-flag interaction definitions: %v", err)
	}
	itemStore := itemcatalog.NewMemoryStore()
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag consume-items templates: %v", err)
	}
	questStore := queststate.NewMemoryStore()
	if err := questStore.Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-items turn-in: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		ticketStore,
		accounts,
		staticstore.NewMemoryStore(),
		interactionStore,
		itemStore,
		questStore,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-items runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag consume-items static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-consume-items", 0x1c1c1c1c)
	if len(enterOut) < 8 {
		t.Fatalf("expected bootstrap frames with visible quest-flag consume-items actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-items interaction error: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("expected chat + consume-gold + reward-gold + consume + reward frames for quest-flag consume-items interaction, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected quest-flag consume-items chat delivery: %+v err=%v", delivery, err)
	}
	consumeGoldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
	if err != nil || consumeGoldChange.Amount != -25 || consumeGoldChange.Value != 0 {
		t.Fatalf("unexpected quest-flag consume-items consume-gold frame: %+v err=%v", consumeGoldChange, err)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[2]))
	if err != nil || pointChange.Amount != 100 || pointChange.Value != 100 {
		t.Fatalf("unexpected quest-flag consume-items gold frame: %+v err=%v", pointChange, err)
	}
	consumeDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[3]))
	if err != nil || consumeDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected quest-flag consume-items delete frame: %+v err=%v", consumeDel, err)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[4]))
	if err != nil || itemSet.Position != itemproto.InventoryPosition(0) || itemSet.Vnum != 11200 || itemSet.Count != 1 {
		t.Fatalf("unexpected quest-flag consume-items reward set frame: %+v err=%v", itemSet, err)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 11200 {
		t.Fatalf("expected live inventory after quest-flag consume-items interaction, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err := accounts.Load("qf-consume-items")
	if err != nil {
		t.Fatalf("load persisted quest-flag consume-items account: %v", err)
	}
	if account.Characters[0].Gold != 100 {
		t.Fatalf("expected persisted gold 100 after quest-flag consume-items interaction, got %d", account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 11200 {
		t.Fatalf("expected persisted inventory after quest-flag consume-items interaction, got %+v", account.Characters[0].Inventory)
	}
	loaded, err := questStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after quest-flag consume-items interaction: %v", err)
	}
	if want := (queststate.Snapshot{Flags: []queststate.Flag{}}); !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected persisted quest-state after quest-flag consume-items interaction:\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestGameSessionFlowStaticActorQuestFlagConsumeItemsRejectsInsufficientMaterialsWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("QuestHero", 0x01030119, 0x02040119, 1100, 2100, 0, 101, 201)
	peer.Gold = 25
	issuePeerTicket(t, ticketStore, "qf-consume-miss", 0x1d1d1d1d, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-consume-miss", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag consume-items mismatch account: %v", err)
	}
	interactionStore := interactionstore.NewMemoryStore()
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{{
		Kind:       interactionstore.KindQuestFlag,
		Ref:        "quest:first_steps_kill_turnin",
		Text:       "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:   "quest:first_steps",
		QuestFlag:  "killed_qa_mob",
		QuestFrom:  1,
		QuestTo:    0,
		RewardGold: 100,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 11200, Count: 1},
		},
		ConsumeItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
		},
	}}}); err != nil {
		t.Fatalf("seed quest-flag interaction definitions: %v", err)
	}
	itemStore := itemcatalog.NewMemoryStore()
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag consume-items mismatch templates: %v", err)
	}
	questStore := queststate.NewMemoryStore()
	if err := questStore.Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-items mismatch turn-in: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		ticketStore,
		accounts,
		staticstore.NewMemoryStore(),
		interactionStore,
		itemStore,
		questStore,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-items mismatch runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag consume-items mismatch static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-consume-miss", 0x1d1d1d1d)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-items mismatch interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one self-only mismatch frame for missing consume items, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != questFlagInsufficientMaterialsInfoMessage {
		t.Fatalf("unexpected quest-flag consume-items mismatch chat: %+v err=%v", delivery, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 25 {
		t.Fatalf("expected gold unchanged after consume-items mismatch, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected inventory unchanged after consume-items mismatch, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	loaded, err := questStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after consume-items mismatch: %v", err)
	}
	wantFlags := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantFlags) {
		t.Fatalf("expected quest-state unchanged after consume-items mismatch:\n got: %#v\nwant: %#v", loaded, wantFlags)
	}
}
