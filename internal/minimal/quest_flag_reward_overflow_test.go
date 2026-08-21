package minimal

import (
	"math"
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
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func TestGameSessionFlowStaticActorQuestFlagRewardGoldRejectsOverflowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("QuestHero", 0x0103012e, 0x0204012e, 1100, 2100, 0, 101, 201)
	peer.Gold = uint64(math.MaxInt32)
	peer.Points[bootstrapExperiencePointType] = 40
	peer.Inventory = []inventory.ItemInstance{{ID: 81, Vnum: 27001, Count: 1, Slot: 0}}
	issuePeerTicket(t, ticketStore, "qf-reward-gold-overflow", 0x2a2a2a2a, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-reward-gold-overflow", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag reward-gold overflow account: %v", err)
	}
	interactionStore := interactionstore.NewMemoryStore()
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{{
		Kind:              interactionstore.KindQuestFlag,
		Ref:               "quest:first_steps_kill_turnin",
		Text:              "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:          "quest:first_steps",
		QuestFlag:         "killed_qa_mob",
		QuestFrom:         1,
		QuestTo:           0,
		RewardGold:        1,
		RewardExperience:  50,
		RewardItems:       []interactionstore.RewardItemEntry{{ItemVnum: 11200, Count: 1}},
		ConsumeItems:      []interactionstore.RewardItemEntry{{ItemVnum: 27001, Count: 1}},
		ConsumeGold:       0,
		ConsumeExperience: 0,
	}}}); err != nil {
		t.Fatalf("seed quest-flag interaction definitions: %v", err)
	}
	itemStore := itemcatalog.NewMemoryStore()
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag reward-gold overflow templates: %v", err)
	}
	questStore := queststate.NewMemoryStore()
	if err := questStore.Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for reward-gold overflow turn-in: %v", err)
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
		t.Fatalf("unexpected quest-flag reward-gold overflow runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag reward-gold overflow static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-reward-gold-overflow", 0x2a2a2a2a)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag reward-gold overflow interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one self-only overflow frame for reward gold, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != questFlagRewardGoldOverflowInfoMessage {
		t.Fatalf("unexpected quest-flag reward-gold overflow chat: %+v err=%v", delivery, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != uint64(math.MaxInt32) {
		t.Fatalf("expected gold unchanged after reward-gold overflow, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	pointsSnapshot, ok := runtime.PointsSnapshot(peer.Name)
	if !ok || pointsSnapshot.Points[bootstrapExperiencePointType] != 40 {
		t.Fatalf("expected experience unchanged after reward-gold overflow, got ok=%v snapshot=%+v", ok, pointsSnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 27001 {
		t.Fatalf("expected inventory unchanged after reward-gold overflow, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	loaded, err := questStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after reward-gold overflow: %v", err)
	}
	wantFlags := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantFlags) {
		t.Fatalf("unexpected quest-state after reward-gold overflow:\n got: %#v\nwant: %#v", loaded, wantFlags)
	}
}

func TestGameSessionFlowStaticActorQuestFlagRewardExperienceRejectsOverflowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("QuestHero", 0x0103012f, 0x0204012f, 1100, 2100, 0, 101, 201)
	peer.Gold = 40
	peer.Points[bootstrapExperiencePointType] = math.MaxInt32
	peer.Inventory = []inventory.ItemInstance{{ID: 82, Vnum: 27001, Count: 1, Slot: 0}}
	issuePeerTicket(t, ticketStore, "qf-reward-exp-overflow", 0x2b2b2b2b, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-reward-exp-overflow", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag reward-experience overflow account: %v", err)
	}
	interactionStore := interactionstore.NewMemoryStore()
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{{
		Kind:              interactionstore.KindQuestFlag,
		Ref:               "quest:first_steps_kill_turnin",
		Text:              "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:          "quest:first_steps",
		QuestFlag:         "killed_qa_mob",
		QuestFrom:         1,
		QuestTo:           0,
		RewardGold:        100,
		RewardExperience:  1,
		RewardItems:       []interactionstore.RewardItemEntry{{ItemVnum: 11200, Count: 1}},
		ConsumeItems:      []interactionstore.RewardItemEntry{{ItemVnum: 27001, Count: 1}},
		ConsumeGold:       0,
		ConsumeExperience: 0,
	}}}); err != nil {
		t.Fatalf("seed quest-flag interaction definitions: %v", err)
	}
	itemStore := itemcatalog.NewMemoryStore()
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag reward-experience overflow templates: %v", err)
	}
	questStore := queststate.NewMemoryStore()
	if err := questStore.Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for reward-experience overflow turn-in: %v", err)
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
		t.Fatalf("unexpected quest-flag reward-experience overflow runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag reward-experience overflow static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-reward-exp-overflow", 0x2b2b2b2b)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag reward-experience overflow interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one self-only overflow frame for reward experience, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != questFlagRewardExperienceOverflowInfoMessage {
		t.Fatalf("unexpected quest-flag reward-experience overflow chat: %+v err=%v", delivery, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 40 {
		t.Fatalf("expected gold unchanged after reward-experience overflow, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	pointsSnapshot, ok := runtime.PointsSnapshot(peer.Name)
	if !ok || pointsSnapshot.Points[bootstrapExperiencePointType] != math.MaxInt32 {
		t.Fatalf("expected experience unchanged after reward-experience overflow, got ok=%v snapshot=%+v", ok, pointsSnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 27001 {
		t.Fatalf("expected inventory unchanged after reward-experience overflow, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	loaded, err := questStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after reward-experience overflow: %v", err)
	}
	wantFlags := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantFlags) {
		t.Fatalf("unexpected quest-state after reward-experience overflow:\n got: %#v\nwant: %#v", loaded, wantFlags)
	}
}
