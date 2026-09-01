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

func TestGameSessionFlowPostFloorShopSell2FailsClosed(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantSeller2Restart", 0x0103019C, 0x0204019C, 125, []inventory.ItemInstance{{ID: 79, Vnum: 27001, Count: 3, Slot: 5}})
	buyer.Points[bootstrapPlayerPointValueIndex] = 1
	buyer.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, store, "merchant-seller2-restart", 0x5a5a5a5c, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-seller2-restart", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed merchant seller2 restart account: %v", err)
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant seller2 restart runtime error: %v", err)
	}
	currentTime := time.Unix(1700000467, 0)
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
			Ref:           "practice.mob_shop_sell2",
			Name:          "PracticeMobShopSell2",
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
		t.Fatalf("import content merchant seller2 restart bundle: %v", err)
	}
	var merchantEntityID uint64
	var practiceMobTargetVID uint32
	for _, actor := range runtime.StaticActors() {
		if actor.SpawnGroupRef == "practice.mob_shop_sell2" {
			practiceMobTargetVID = uint32(actor.EntityID)
			continue
		}
		if actor.InteractionKind == interactionstore.KindShopPreview && actor.InteractionRef == "npc:merchant" {
			merchantEntityID = actor.EntityID
		}
	}
	if merchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected merchant and practice mob before post-floor shop sell2 recovery, got %#v", runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-seller2-restart", 0x5a5a5a5c)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, merchantEntityID)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID}))); err != nil {
		t.Fatalf("unexpected target selection before post-floor shop sell2: %v", err)
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  practiceMobTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before post-floor shop sell2: %v", err)
	}
	if len(attackOut) < 5 || !reflect.DeepEqual(attackOut[len(attackOut)-1], shopproto.EncodeServerEnd()) {
		t.Fatalf("expected floor attack to append merchant close before post-floor shop sell2, got %d frames", len(attackOut))
	}

	sellOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 5, Count: 2})))
	if err != nil {
		t.Fatalf("unexpected post-floor SHOP SELL2 dispatch error: %v", err)
	}
	if len(sellOut) != 0 {
		t.Fatalf("expected post-floor SHOP SELL2 to fail closed with no frames, got %d", len(sellOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor SHOP SELL2 to queue no frames, got %d", len(queued))
	}
	account, err := accounts.Load("merchant-seller2-restart")
	if err != nil {
		t.Fatalf("load account after post-floor SHOP SELL2: %v", err)
	}
	if account.Characters[0].Gold != 125 || !reflect.DeepEqual(account.Characters[0].Inventory, buyer.Inventory) || !reflect.DeepEqual(account.Characters[0].Quickslots, buyer.Quickslots) {
		t.Fatalf("expected post-floor SHOP SELL2 to leave persisted seller unchanged, got %#v", account.Characters[0])
	}

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor SHOP SELL2: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor SHOP SELL2, got %d", len(restartOut))
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

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 5, Count: 2})))
	if err != nil {
		t.Fatalf("unexpected post-restart SHOP SELL2: %v", err)
	}
	assertPostFloorShopSell2SuccessBurst(t, reuseOut, buyer.VID, 2, 127, "post-restart SHOP SELL2")
	account, err = accounts.Load("merchant-seller2-restart")
	if err != nil {
		t.Fatalf("load account after post-restart SHOP SELL2: %v", err)
	}
	wantHP := initialStatsForRace(buyer.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after SHOP SELL2 floor, got %+v", wantHP, account.Characters[0])
	}
	if account.Characters[0].Gold != 127 || len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 5 || account.Characters[0].Inventory[0].Vnum != 27001 {
		t.Fatalf("expected post-restart SHOP SELL2 to persist gold=127 and remaining count=1 inventory, got %+v", account.Characters[0])
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, buyer.Quickslots) {
		t.Fatalf("expected post-restart SHOP SELL2 to keep item+skill quickslots on the still-occupied cell, got %#v", account.Characters[0].Quickslots)
	}
}

func TestGameSessionFlowPostFloorShopSell2FailsClosedBeforeRestartTown(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantSeller2Town", 0x0103019D, 0x0204019D, 125, []inventory.ItemInstance{{ID: 80, Vnum: 27001, Count: 3, Slot: 5}})
	buyer.Points[bootstrapPlayerPointValueIndex] = 1
	buyer.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	login := "merchant-seller2-town"
	loginKey := uint32(0x5a5a5a5d)
	issuePeerTicket(t, store, login, loginKey, buyer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed merchant seller2 town account: %v", err)
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant seller2 town runtime error: %v", err)
	}
	currentTime := time.Unix(1700000468, 0)
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
			Ref:           "practice.mob_shop_sell2_town",
			Name:          "PracticeMobShopSell2Town",
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
		t.Fatalf("import content merchant seller2 town bundle: %v", err)
	}
	var (
		sourceMerchantEntityID uint64
		townMerchantEntityID   uint64
		practiceMobTargetVID   uint32
	)
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "PracticeMobShopSell2Town":
			practiceMobTargetVID = uint32(actor.EntityID)
		case "Merchant":
			sourceMerchantEntityID = actor.EntityID
		case "TownMerchant":
			townMerchantEntityID = actor.EntityID
		}
	}
	if sourceMerchantEntityID == 0 || townMerchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected source merchant, town merchant, and practice mob before post-floor shop sell2 town recovery, got %#v", runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, sourceMerchantEntityID)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID}))); err != nil {
		t.Fatalf("unexpected target selection before post-floor town shop sell2: %v", err)
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  practiceMobTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack before post-floor town shop sell2: %v", err)
	}
	if len(attackOut) < 5 || !reflect.DeepEqual(attackOut[len(attackOut)-1], shopproto.EncodeServerEnd()) {
		t.Fatalf("expected floor attack to append merchant close before post-floor town shop sell2, got %d frames", len(attackOut))
	}

	sellOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 5, Count: 2})))
	if err != nil {
		t.Fatalf("unexpected post-floor town SHOP SELL2 dispatch error: %v", err)
	}
	if len(sellOut) != 0 {
		t.Fatalf("expected post-floor town SHOP SELL2 to fail closed with no frames, got %d", len(sellOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor town SHOP SELL2 to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, buyer, "post-floor town SHOP SELL2")

	restartOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor SHOP SELL2: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor SHOP SELL2, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor SHOP SELL2 /restart_town: %v", err)
	}
	if selfAdd.VID != buyer.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after SHOP SELL2 floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after SHOP SELL2 floor")
	}
	wantHP := initialStatsForRace(buyer.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after SHOP SELL2 floor, got %+v", wantHP, selfPoints)
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

	reuseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 5, Count: 2})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town SHOP SELL2: %v", err)
	}
	assertPostFloorShopSell2SuccessBurst(t, reuseOut, buyer.VID, 2, 127, "post-restart_town SHOP SELL2")
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town SHOP SELL2: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after SHOP SELL2 floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town + SHOP SELL2 to leave recovered HP %d unchanged, got %+v", wantHP, account.Characters[0])
	}
	want := buyer
	want.Points[bootstrapPlayerPointValueIndex] = wantHP
	want.MapIndex = 21
	want.X = 52070
	want.Y = 166600
	want.Gold = 127
	want.Inventory = []inventory.ItemInstance{{ID: 80, Vnum: 27001, Count: 1, Slot: 5}}
	want.Quickslots = buyer.Quickslots
	assertExchangeAccountUnchanged(t, accounts, login, want, "post-restart_town SHOP SELL2 persists sell-back")
}

func assertPostFloorShopSell2SuccessBurst(t *testing.T, frames [][]byte, ownerVID uint32, goldAmount int32, goldValue int32, context string) {
	t.Helper()
	if len(frames) != 2 {
		t.Fatalf("expected %s to emit ITEM_UPDATE + gold POINT_CHANGE, got %d", context, len(frames))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s update: %v", context, err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 1 {
		t.Fatalf("unexpected %s item update: %+v", context, update)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s gold point-change: %v", context, err)
	}
	if pointChange.VID != ownerVID || pointChange.Type != bootstrapGoldPointType || pointChange.Amount != goldAmount || pointChange.Value != goldValue {
		t.Fatalf("unexpected %s gold point-change: %+v", context, pointChange)
	}
}
