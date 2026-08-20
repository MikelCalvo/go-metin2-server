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

func TestGameSessionFlowStaticActorQuestFlagConsumeExperienceDebitsPointAndGrantsReward(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("QuestHero", 0x0103011c, 0x0204011c, 1100, 2100, 0, 101, 201)
	peer.Gold = 40
	peer.Points[bootstrapExperiencePointType] = 40
	peer.Inventory = []inventory.ItemInstance{{ID: 63, Vnum: 27001, Count: 1, Slot: 0}}
	issuePeerTicket(t, ticketStore, "qf-consume-exp", 0x20202020, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-consume-exp", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag consume-experience account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
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
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag consume-experience templates: %v", err)
	}
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-experience turn-in: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-experience runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag consume-experience static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-consume-exp", 0x20202020)
	if len(enterOut) < 8 {
		t.Fatalf("expected bootstrap frames with visible quest-flag consume-experience actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-experience interaction error: %v", err)
	}
	if len(out) != 7 {
		t.Fatalf("expected chat + consume-gold + consume-experience + reward-gold + reward-experience + consume + reward frames for quest-flag consume-experience interaction, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected quest-flag consume-experience chat delivery: %+v err=%v", delivery, err)
	}
	consumeGoldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
	if err != nil || consumeGoldChange.Amount != -25 || consumeGoldChange.Value != 15 {
		t.Fatalf("unexpected quest-flag consume-experience gold debit frame: %+v err=%v", consumeGoldChange, err)
	}
	consumeExperienceChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[2]))
	if err != nil || consumeExperienceChange.Type != bootstrapExperiencePointType || consumeExperienceChange.Amount != -10 || consumeExperienceChange.Value != 30 {
		t.Fatalf("unexpected quest-flag consume-experience debit frame: %+v err=%v", consumeExperienceChange, err)
	}
	rewardGoldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[3]))
	if err != nil || rewardGoldChange.Amount != 100 || rewardGoldChange.Value != 115 {
		t.Fatalf("unexpected quest-flag consume-experience reward-gold frame: %+v err=%v", rewardGoldChange, err)
	}
	rewardExperienceChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[4]))
	if err != nil || rewardExperienceChange.Type != bootstrapExperiencePointType || rewardExperienceChange.Amount != 50 || rewardExperienceChange.Value != 80 {
		t.Fatalf("unexpected quest-flag consume-experience reward-experience frame: %+v err=%v", rewardExperienceChange, err)
	}
	consumeDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[5]))
	if err != nil || consumeDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected quest-flag consume-experience delete frame: %+v err=%v", consumeDel, err)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[6]))
	if err != nil || itemSet.Position != itemproto.InventoryPosition(0) || itemSet.Vnum != 11200 || itemSet.Count != 1 {
		t.Fatalf("unexpected quest-flag consume-experience reward set frame: %+v err=%v", itemSet, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 115 {
		t.Fatalf("expected live gold 115 after quest-flag consume-experience interaction, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	pointsSnapshot, ok := runtime.PointsSnapshot(peer.Name)
	if !ok || pointsSnapshot.Points[bootstrapExperiencePointType] != 80 {
		t.Fatalf("expected live experience 80 after quest-flag consume-experience interaction, got ok=%v snapshot=%+v", ok, pointsSnapshot)
	}
	account, err := accounts.Load("qf-consume-exp")
	if err != nil {
		t.Fatalf("load persisted quest-flag consume-experience account: %v", err)
	}
	if account.Characters[0].Gold != 115 {
		t.Fatalf("expected persisted gold 115 after quest-flag consume-experience interaction, got %d", account.Characters[0].Gold)
	}
	if account.Characters[0].Points[bootstrapExperiencePointType] != 80 {
		t.Fatalf("expected persisted experience 80 after quest-flag consume-experience interaction, got %d", account.Characters[0].Points[bootstrapExperiencePointType])
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state after quest-flag consume-experience interaction: %v", err)
	}
	if want := (queststate.Snapshot{Flags: []queststate.Flag{}}); !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected persisted quest-state after quest-flag consume-experience interaction:\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestGameSessionFlowStaticActorQuestFlagConsumeExperienceRejectsInsufficientExperienceWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("QuestHero", 0x0103011d, 0x0204011d, 1100, 2100, 0, 101, 201)
	peer.Gold = 40
	peer.Points[bootstrapExperiencePointType] = 5
	peer.Inventory = []inventory.ItemInstance{{ID: 64, Vnum: 27001, Count: 1, Slot: 0}}
	issuePeerTicket(t, ticketStore, "qf-consume-exp-miss", 0x21212121, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "qf-consume-exp-miss", Empire: peer.Empire, Characters: []loginticket.Character{peer}}); err != nil {
		t.Fatalf("seed quest-flag consume-experience mismatch account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
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
	}})
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopBuyPrice: 50},
	}}); err != nil {
		t.Fatalf("seed quest-flag consume-experience mismatch templates: %v", err)
	}
	if err := queststate.NewFileStore(questStatePath).Save(queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}); err != nil {
		t.Fatalf("seed quest-state for consume-experience mismatch turn-in: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-experience mismatch runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("QuestHunter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindQuestFlag, "quest:first_steps_kill_turnin")
	if !ok {
		t.Fatal("expected quest-flag consume-experience mismatch static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "qf-consume-exp-miss", 0x21212121)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected quest-flag consume-experience mismatch interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one self-only mismatch frame for insufficient consume experience, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil || delivery.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected quest-flag consume-experience mismatch chat: %+v err=%v", delivery, err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(peer.Name)
	if !ok || currencySnapshot.Gold != 40 {
		t.Fatalf("expected gold unchanged after consume-experience mismatch, got ok=%v snapshot=%+v", ok, currencySnapshot)
	}
	pointsSnapshot, ok := runtime.PointsSnapshot(peer.Name)
	if !ok || pointsSnapshot.Points[bootstrapExperiencePointType] != 5 {
		t.Fatalf("expected experience unchanged after consume-experience mismatch, got ok=%v snapshot=%+v", ok, pointsSnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(peer.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 27001 {
		t.Fatalf("expected inventory unchanged after consume-experience mismatch, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	loaded, err := queststate.NewFileStore(questStatePath).Load()
	if err != nil {
		t.Fatalf("load quest-state after consume-experience mismatch: %v", err)
	}
	wantFlags := queststate.Snapshot{Flags: []queststate.Flag{{
		Character: "QuestHero",
		QuestRef:  "quest:first_steps",
		Name:      "killed_qa_mob",
		Value:     1,
	}}}
	if !reflect.DeepEqual(loaded, wantFlags) {
		t.Fatalf("unexpected quest-state after consume-experience mismatch:\n got: %#v\nwant: %#v", loaded, wantFlags)
	}
}
