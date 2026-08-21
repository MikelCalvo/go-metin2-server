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

func TestGameSessionFlowStaticActorQuestFlagConsumeGoldDebitsCurrencyAndGrantsReward(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("QuestHero", 0x0103011a, 0x0204011a, 1100, 2100, 0, 101, 201)
	peer.Gold = 40
	peer.Inventory = []inventory.ItemInstance{{ID: 61, Vnum: 27001, Count: 1, Slot: 0}}
	issuePeerTicket(t, ticketStore, "qf-consume-gold", 0x1e1e1e1e, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-consume-gold", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag consume-gold account: %v", err)
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
		t.Fatalf("seed quest-flag consume-gold templates: %v", err)
	}
	questStore := queststate.NewMemoryStore()
	if err := questStore.Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-gold turn-in: %v", err)
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
		t.Fatalf("unexpected quest-flag consume-gold runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag consume-gold static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-consume-gold", 0x1e1e1e1e)
	if len(enterOut) < 8 {
		t.Fatalf("expected bootstrap frames with visible quest-flag consume-gold actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-gold interaction error: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("expected chat + consume-gold + reward-gold + consume + reward frames for quest-flag consume-gold interaction, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected quest-flag consume-gold chat delivery: %+v err=%v", delivery, err)
	}
	consumeGoldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
	if err != nil || consumeGoldChange.Amount != -25 || consumeGoldChange.Value != 15 {
		t.Fatalf("unexpected quest-flag consume-gold debit frame: %+v err=%v", consumeGoldChange, err)
	}
	rewardGoldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[2]))
	if err != nil || rewardGoldChange.Amount != 100 || rewardGoldChange.Value != 115 {
		t.Fatalf("unexpected quest-flag consume-gold reward frame: %+v err=%v", rewardGoldChange, err)
	}
	consumeDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[3]))
	if err != nil || consumeDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected quest-flag consume-gold delete frame: %+v err=%v", consumeDel, err)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[4]))
	if err != nil || itemSet.Position != itemproto.InventoryPosition(0) || itemSet.Vnum != 11200 || itemSet.Count != 1 {
		t.Fatalf("unexpected quest-flag consume-gold reward set frame: %+v err=%v", itemSet, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 115 {
		t.Fatalf("expected live gold 115 after quest-flag consume-gold interaction, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	account, err := accounts.Load("qf-consume-gold")
	if err != nil {
		t.Fatalf("load persisted quest-flag consume-gold account: %v", err)
	}
	if account.Characters[0].Gold != 115 {
		t.Fatalf("expected persisted gold 115 after quest-flag consume-gold interaction, got %d", account.Characters[0].Gold)
	}
	loaded, err := questStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after quest-flag consume-gold interaction: %v", err)
	}
	if want := (queststate.Snapshot{Flags: []queststate.Flag{}}); !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected persisted quest-state after quest-flag consume-gold interaction:\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestGameSessionFlowStaticActorQuestFlagConsumeGoldRejectsInsufficientGoldWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("QuestHero", 0x0103011b, 0x0204011b, 1100, 2100, 0, 101, 201)
	peer.Gold = 10
	peer.Inventory = []inventory.ItemInstance{{ID: 62, Vnum: 27001, Count: 1, Slot: 0}}
	issuePeerTicket(t, ticketStore, "qf-consume-gold-miss", 0x1f1f1f1f, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-consume-gold-miss", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag consume-gold mismatch account: %v", err)
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
		t.Fatalf("seed quest-flag consume-gold mismatch templates: %v", err)
	}
	questStore := queststate.NewMemoryStore()
	if err := questStore.Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-gold mismatch turn-in: %v", err)
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
		t.Fatalf("unexpected quest-flag consume-gold mismatch runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag consume-gold mismatch static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-consume-gold-miss", 0x1f1f1f1f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-gold mismatch interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one self-only mismatch frame for insufficient consume gold, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != questFlagInsufficientGoldInfoMessage {
		t.Fatalf("unexpected quest-flag consume-gold mismatch chat: %+v err=%v", delivery, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 10 {
		t.Fatalf("expected gold unchanged after consume-gold mismatch, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 27001 {
		t.Fatalf("expected inventory unchanged after consume-gold mismatch, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	loaded, err := questStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after consume-gold mismatch: %v", err)
	}
	wantFlags := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantFlags) {
		t.Fatalf("unexpected quest-state after consume-gold mismatch:\n got: %#v\nwant: %#v", loaded, wantFlags)
	}
}
