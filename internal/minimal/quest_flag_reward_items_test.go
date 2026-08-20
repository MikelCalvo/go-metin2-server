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
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
)

func merchantBuyerInventoryLeavingOneFreeSlot() []inventory.ItemInstance {
	items := merchantBuyerFullInventory()
	return items[:len(items)-1]
}

func TestGameSessionFlowStaticActorQuestFlagRewardItemsGrantsInventoryAndPersistsAccount(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("QuestHero", 0x01030116, 0x02040116, 1100, 2100, 0, 101, 201)
	peer.Gold = 25
	issuePeerTicket(t, ticketStore, "qf-reward-items", 0x1a1a1a1a, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-reward-items", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag reward-items account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:       interactionstore.KindQuestFlag,
		Ref:        "quest:first_steps_kill_turnin",
		Text:       "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:   "quest:first_steps",
		QuestFlag:  "killed_qa_mob",
		QuestFrom:  1,
		QuestTo:    0,
		RewardGold: 100,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
			{ItemVnum: 11200, Count: 1},
		},
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag reward-items templates: %v", err)
	}
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for reward-items turn-in: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected quest-flag reward-items runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag reward-items static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-reward-items", 0x1a1a1a1a)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible quest-flag reward-items actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag reward-items interaction error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected chat + gold + two item frames for quest-flag reward-items interaction, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected quest-flag reward-items chat delivery: %+v err=%v", delivery, err)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
	if err != nil || pointChange.Amount != 100 || pointChange.Value != 125 {
		t.Fatalf("unexpected quest-flag reward-items gold frame: %+v err=%v", pointChange, err)
	}
	itemSet0, err := itemproto.DecodeSet(decodeSingleFrame(t, out[2]))
	if err != nil || itemSet0.Position != itemproto.InventoryPosition(0) || itemSet0.Vnum != 27001 || itemSet0.Count != 1 {
		t.Fatalf("unexpected quest-flag reward-items first set frame: %+v err=%v", itemSet0, err)
	}
	itemSet1, err := itemproto.DecodeSet(decodeSingleFrame(t, out[3]))
	if err != nil || itemSet1.Position != itemproto.InventoryPosition(1) || itemSet1.Vnum != 11200 || itemSet1.Count != 1 {
		t.Fatalf("unexpected quest-flag reward-items second set frame: %+v err=%v", itemSet1, err)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 2 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[1].Vnum != 11200 {
		t.Fatalf("expected live inventory grant after quest-flag reward-items interaction, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err := accounts.Load("qf-reward-items")
	if err != nil {
		t.Fatalf("load persisted quest-flag reward-items account: %v", err)
	}
	if account.Characters[0].Gold != 125 {
		t.Fatalf("expected persisted gold 125 after quest-flag reward-items interaction, got %d", account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 2 || account.Characters[0].Inventory[0].Vnum != 27001 || account.Characters[0].Inventory[1].Vnum != 11200 {
		t.Fatalf("expected persisted inventory grant after quest-flag reward-items interaction, got %+v", account.Characters[0].Inventory)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state after quest-flag reward-items interaction: %v", err)
	}
	if want := (queststate.Snapshot{Flags: []queststate.Flag{}}); !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected persisted quest-state after quest-flag reward-items interaction:\n got: %#v\nwant: %#v", loaded, want)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for self-only quest-flag reward-items interaction, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorQuestFlagRewardItemsRejectsWhenSecondGrantWouldOverflowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("QuestHero", 0x01030117, 0x02040117, 1100, 2100, 0, 101, 201)
	peer.Gold = 25
	peer.Inventory = merchantBuyerInventoryLeavingOneFreeSlot()
	issuePeerTicket(t, ticketStore, "qf-reward-ovf", 0x1b1b1b1b, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-reward-ovf", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag reward-items overflow account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:       interactionstore.KindQuestFlag,
		Ref:        "quest:first_steps_kill_turnin",
		Text:       "Quest updated: first_steps.killed_qa_mob = 0.",
		QuestRef:   "quest:first_steps",
		QuestFlag:  "killed_qa_mob",
		QuestFrom:  1,
		QuestTo:    0,
		RewardGold: 100,
		RewardItems: []interactionstore.RewardItemEntry{
			{ItemVnum: 27001, Count: 1},
			{ItemVnum: 11200, Count: 1},
		},
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag reward-items overflow templates: %v", err)
	}
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for reward-items overflow turn-in: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected quest-flag reward-items overflow runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag reward-items overflow static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-reward-ovf", 0x1b1b1b1b)
	if len(enterOut) < 8 {
		t.Fatalf("expected bootstrap frames with visible quest-flag reward-items actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag reward-items overflow interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one inventory-full reject chat for quest-flag reward-items turn-in, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != itemPickupInventoryFullInfoMessage {
		t.Fatalf("unexpected quest-flag reward-items overflow chat delivery: %+v err=%v", delivery, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 25 {
		t.Fatalf("expected gold unchanged after reward-items overflow rejection, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != len(peer.Inventory) {
		t.Fatalf("expected inventory unchanged after reward-items overflow rejection, got ok=%v len=%d want=%d", ok, len(inventorySnapshot.Inventory), len(peer.Inventory))
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state after reward-items overflow rejection: %v", err)
	}
	wantFlags := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantFlags) {
		t.Fatalf("expected quest-state unchanged after reward-items overflow rejection:\n got: %#v\nwant: %#v", loaded, wantFlags)
	}
}

func TestGameSessionFlowStaticActorQuestFlagRewardItemRejectsAntiGetWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("QuestHero", 0x01030118, 0x02040118, 1100, 2100, 0, 101, 201)
	peer.Gold = 25
	issuePeerTicket(t, ticketStore, "qf-reward-antiget", 0x1c1c1c1c, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-reward-antiget", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag anti_get reward account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
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
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum: 27001, Name: "Bound Reward Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5, AntiGet: true,
		BuyRejectText: "This reward is sealed against you.",
	}}}); err != nil {
		t.Fatalf("seed quest-flag anti_get reward templates: %v", err)
	}
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for anti_get reward turn-in: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected quest-flag anti_get reward runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag anti_get reward static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-reward-antiget", 0x1c1c1c1c)
	if len(enterOut) < 8 {
		t.Fatalf("expected bootstrap frames with visible quest-flag anti_get reward actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag anti_get reward interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one restricted-reward reject chat for anti_get quest-flag turn-in, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "This reward is sealed against you." {
		t.Fatalf("unexpected quest-flag anti_get reward chat delivery: %+v err=%v", delivery, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 25 {
		t.Fatalf("expected gold unchanged after anti_get reward rejection, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected inventory unchanged after anti_get reward rejection, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err := accounts.Load("qf-reward-antiget")
	if err != nil {
		t.Fatalf("load persisted quest-flag anti_get reward account: %v", err)
	}
	if account.Characters[0].Gold != 25 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted account unchanged after anti_get reward rejection, got %+v", account.Characters[0])
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state after anti_get reward rejection: %v", err)
	}
	wantFlags := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantFlags) {
		t.Fatalf("expected quest-state unchanged after anti_get reward rejection:\n got: %#v\nwant: %#v", loaded, wantFlags)
	}
}

func TestGameSessionFlowStaticActorQuestFlagRewardItemRejectsSelectedCharacterRestrictionWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("QuestHero", 0x01030119, 0x02040119, 1100, 2100, 0, 101, 201)
	peer.Gold = 25
	peer.Level = 5
	issuePeerTicket(t, ticketStore, "qf-reward-minlevel", 0x1d1d1d1d, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-reward-minlevel", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag min_level reward account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
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
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum: 27001, Name: "High-Level Reward Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5, MinLevel: 10,
	}}}); err != nil {
		t.Fatalf("seed quest-flag min_level reward templates: %v", err)
	}
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for min_level reward turn-in: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected quest-flag min_level reward runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag min_level reward static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-reward-minlevel", 0x1d1d1d1d)
	if len(enterOut) < 8 {
		t.Fatalf("expected bootstrap frames with visible quest-flag min_level reward actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag min_level reward interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one restricted-reward reject chat for min_level quest-flag turn-in, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != questFlagRewardRestrictedInfoMessage {
		t.Fatalf("unexpected quest-flag min_level reward chat delivery: %+v err=%v", delivery, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 25 {
		t.Fatalf("expected gold unchanged after min_level reward rejection, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected inventory unchanged after min_level reward rejection, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err := accounts.Load("qf-reward-minlevel")
	if err != nil {
		t.Fatalf("load persisted quest-flag min_level reward account: %v", err)
	}
	if account.Characters[0].Gold != 25 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted account unchanged after min_level reward rejection, got %+v", account.Characters[0])
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state after min_level reward rejection: %v", err)
	}
	wantFlags := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantFlags) {
		t.Fatalf("expected quest-state unchanged after min_level reward rejection:\n got: %#v\nwant: %#v", loaded, wantFlags)
	}
}
