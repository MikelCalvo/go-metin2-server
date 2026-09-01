package minimal

import (
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorShopSellFailsClosed(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantSellerRestart", 0x0103019A, 0x0204019A, 125, []inventory.ItemInstance{{ID: 77, Vnum: 27001, Count: 3, Slot: 5}})
	buyer.Points[bootstrapPlayerPointValueIndex] = 1
	buyer.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, store, "merchant-seller-restart", 0x5a5a5a5a, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-seller-restart", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed merchant seller restart account: %v", err)
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant seller restart runtime error: %v", err)
	}
	currentTime := time.Unix(1700000465, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_alpha",
			Name:          "PracticeMobAlpha",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		ItemTemplates:          defaultMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content merchant seller restart bundle: %v", err)
	}
	var merchantEntityID uint64
	var practiceMobTargetVID uint32
	for _, actor := range runtime.StaticActors() {
		if actor.SpawnGroupRef == "practice.mob_alpha" {
			practiceMobTargetVID = uint32(actor.EntityID)
			continue
		}
		if actor.InteractionKind == interactionstore.KindShopPreview && actor.InteractionRef == "npc:merchant" {
			merchantEntityID = actor.EntityID
		}
	}
	if merchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected merchant and practice mob before post-floor shop sell recovery, got %#v", runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-seller-restart", 0x5a5a5a5a)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, merchantEntityID)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID}))); err != nil {
		t.Fatalf("unexpected target selection before post-floor shop sell: %v", err)
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  practiceMobTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before post-floor shop sell: %v", err)
	}
	if len(attackOut) < 5 || !reflect.DeepEqual(attackOut[len(attackOut)-1], shopproto.EncodeServerEnd()) {
		t.Fatalf("expected floor attack to append merchant close before post-floor shop sell, got %d frames", len(attackOut))
	}

	sellOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell(shopproto.ClientSellPacket{Slot: 5})))
	if err != nil {
		t.Fatalf("unexpected post-floor SHOP SELL dispatch error: %v", err)
	}
	if len(sellOut) != 0 {
		t.Fatalf("expected post-floor SHOP SELL to fail closed with no frames, got %d", len(sellOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor SHOP SELL to queue no frames, got %d", len(queued))
	}
	account, err := accounts.Load("merchant-seller-restart")
	if err != nil {
		t.Fatalf("load account after post-floor SHOP SELL: %v", err)
	}
	if account.Characters[0].Gold != 125 || !reflect.DeepEqual(account.Characters[0].Inventory, buyer.Inventory) || !reflect.DeepEqual(account.Characters[0].Quickslots, buyer.Quickslots) {
		t.Fatalf("expected post-floor SHOP SELL to leave persisted seller unchanged, got %#v", account.Characters[0])
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor SHOP SELL: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor SHOP SELL, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, flow)

	// Fresh merchant INTERACT after floor-edge open still needs the owned static-actor cooldown window.
	currentTime = currentTime.Add(staticActorInteractionCooldown + time.Nanosecond)
	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchantEntityID)})))
	if err != nil {
		t.Fatalf("unexpected post-restart merchant INTERACT: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected post-restart merchant INTERACT to reopen shop with one SHOP START, got %d frames", len(reopenOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart merchant SHOP START: %v", err)
	}

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell(shopproto.ClientSellPacket{Slot: 5})))
	if err != nil {
		t.Fatalf("unexpected post-restart SHOP SELL: %v", err)
	}
	assertPostFloorShopSellSuccessBurst(t, reuseOut, buyer.VID, 3, 128, 1, "post-restart SHOP SELL")
	account, err = accounts.Load("merchant-seller-restart")
	if err != nil {
		t.Fatalf("load account after post-restart SHOP SELL: %v", err)
	}
	wantHP := initialStatsForRace(buyer.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after SHOP SELL floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != 128 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected post-restart SHOP SELL to persist gold=128 and empty inventory, got %+v", account.Characters[0])
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5}}) {
		t.Fatalf("expected post-restart SHOP SELL to keep only skill quickslot, got %#v", account.Characters[0].Quickslots)
	}
}

func TestGameSessionFlowPostFloorShopSellFailsClosedBeforeRestartTown(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantSellerTown", 0x0103019B, 0x0204019B, 125, []inventory.ItemInstance{{ID: 78, Vnum: 27001, Count: 3, Slot: 5}})
	buyer.Points[bootstrapPlayerPointValueIndex] = 1
	buyer.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	login := "merchant-seller-town"
	loginKey := uint32(0x5a5a5a5b)
	issuePeerTicket(t, store, login, loginKey, buyer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed merchant seller town account: %v", err)
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant seller town runtime error: %v", err)
	}
	currentTime := time.Unix(1700000466, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}, {
			Name:            "TownMerchant",
			MapIndex:        21,
			X:               52100,
			Y:               166650,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_shop_sell_town",
			Name:          "PracticeMobShopSellTown",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		ItemTemplates:          defaultMerchantItemTemplates(),
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content merchant seller town bundle: %v", err)
	}
	var (
		sourceMerchantEntityID uint64
		townMerchantEntityID   uint64
		practiceMobTargetVID   uint32
	)
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "PracticeMobShopSellTown":
			practiceMobTargetVID = uint32(actor.EntityID)
		case "Merchant":
			sourceMerchantEntityID = actor.EntityID
		case "TownMerchant":
			townMerchantEntityID = actor.EntityID
		}
	}
	if sourceMerchantEntityID == 0 || townMerchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected source merchant, town merchant, and practice mob before post-floor shop sell town recovery, got %#v", runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, sourceMerchantEntityID)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID}))); err != nil {
		t.Fatalf("unexpected target selection before post-floor town shop sell: %v", err)
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  practiceMobTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before post-floor town shop sell: %v", err)
	}
	if len(attackOut) < 5 || !reflect.DeepEqual(attackOut[len(attackOut)-1], shopproto.EncodeServerEnd()) {
		t.Fatalf("expected floor attack to append merchant close before post-floor town shop sell, got %d frames", len(attackOut))
	}

	sellOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell(shopproto.ClientSellPacket{Slot: 5})))
	if err != nil {
		t.Fatalf("unexpected post-floor town SHOP SELL dispatch error: %v", err)
	}
	if len(sellOut) != 0 {
		t.Fatalf("expected post-floor town SHOP SELL to fail closed with no frames, got %d", len(sellOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town SHOP SELL to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, buyer, "post-floor town SHOP SELL")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor SHOP SELL: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor SHOP SELL, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor SHOP SELL /restart_town: %v", err)
	}
	if selfAdd.VID != buyer.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after SHOP SELL floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after SHOP SELL floor")
	}
	wantHP := initialStatsForRace(buyer.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after SHOP SELL floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, flow)

	currentTime = currentTime.Add(staticActorInteractionCooldown + time.Nanosecond)
	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(townMerchantEntityID)})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town merchant INTERACT: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected post-restart_town merchant INTERACT to reopen shop with one SHOP START, got %d frames", len(reopenOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, reopenOut[0])); err != nil {
		t.Fatalf("decode post-restart_town merchant SHOP START: %v", err)
	}

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell(shopproto.ClientSellPacket{Slot: 5})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town SHOP SELL: %v", err)
	}
	assertPostFloorShopSellSuccessBurst(t, reuseOut, buyer.VID, 3, 128, 1, "post-restart_town SHOP SELL")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town SHOP SELL: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after SHOP SELL floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + SHOP SELL to leave recovered HP %d unchanged, got %+v", wantHP, account.Characters[0])
	}
	want := buyer
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Gold = 128
	want.Inventory = nil
	want.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town SHOP SELL persists sell-back")
}

func assertPostFloorShopSellSuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, goldAmount int32, goldValue int32, quickslotPosition uint8, context string) {
	t.Helper()
	if len(frames) != 3 {
		t.Fatalf("expected %s to emit ITEM_DEL + QUICKSLOT_DEL + gold POINT_CHANGE, got %d", context, len(frames))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s del: %v", context, err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected %s del: %+v", context, del)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s quickslot del: %v", context, err)
	}
	if quickslotDel.Position != quickslotPosition {
		t.Fatalf("expected %s to delete only item quickslot position %d, got %+v", context, quickslotPosition, quickslotDel)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode %s gold point-change: %v", context, err)
	}
	if pointChange.VID != ownerVID || pointChange.Type != bootstrapGoldPointType || pointChange.Amount != goldAmount || pointChange.Value != goldValue {
		t.Fatalf("unexpected %s gold point-change: %+v", context, pointChange)
	}
}
