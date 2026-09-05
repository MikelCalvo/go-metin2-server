package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMerchantBeforeRestartExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("MerchantRestartImmediateOwner", 0x01040e21, 0x02050e21, 125, nil)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	partner := peerVisibilityCharacter("MerchantRestartImmediatePartner", 0x01040e22, 0x02050e22, 1120, 2120, 0, 101, 201)
	partner.Gold = 22222
	login := "merch-rst-imm"
	loginKey := uint32(0x2e2e0e21)
	partnerLogin := "merch-rst-imm-p"
	partnerLoginKey := uint32(0x2e2e0e22)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, partnerLogin, partnerLoginKey, partner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed merchant restart immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: partnerLogin, Empire: partner.Empire, Characters: cloneCharacters([]loginticket.Character{partner})}); err != nil {
		t.Fatalf("seed merchant restart immediate floor-close partner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()}}); err != nil {
		t.Fatalf("seed merchant restart immediate floor-close interaction store: %v", err)
	}
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant restart immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000821, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1200,
			Y:               2200,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_merchant_restart_immediate_floor_close",
			Name:          "PracticeMobMerchantRestartImmediateFloorClose",
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
		t.Fatalf("import content bundle for merchant restart immediate floor-close test: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 11 {
		t.Fatalf("expected merchant restart immediate floor-close owner bootstrap to emit at least 11 frames, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	partnerFlow, partnerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), partnerLogin, partnerLoginKey)
	if len(partnerEnter) < 14 {
		t.Fatalf("expected merchant restart immediate floor-close partner bootstrap to emit at least 14 frames, got %d", len(partnerEnter))
	}
	defer closeSessionFlow(t, partnerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive partner peer-entry frames before merchant restart immediate floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, partnerFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	var merchantVID uint32
	for _, actor := range actors {
		switch actor.Name {
		case "PracticeMobMerchantRestartImmediateFloorClose":
			targetVID = uint32(actor.EntityID)
		case "Merchant":
			merchantVID = uint32(actor.EntityID)
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}
	if merchantVID == 0 {
		t.Fatalf("expected merchant actor after import, got %+v", actors)
	}

	interactWithMerchantForBuy(t, ownerFlow, uint64(merchantVID))

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before merchant restart immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before merchant restart immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before merchant restart immediate floor-close: %v", err)
	}
	if len(attackOut) != 6 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, owner damage-info, and merchant close frames, got %d", len(attackOut))
	}
	next := assertOwnerFloorDeathSequence(t, attackOut, 1, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "merchant_practice_mob_floor_restart_exchange owner-floor")
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, attackOut[next])); err != nil {
		t.Fatalf("decode merchant restart immediate floor-close shop end: %v", err)
	}

	partnerQueued := flushServerFrames(t, partnerFlow)
	if len(partnerQueued) != 2 {
		t.Fatalf("expected partner DEAD plus owner damage-info after merchant restart immediate floor, got %d", len(partnerQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, partnerQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "merchant_practice_mob_floor_restart_exchange owner-floor peer")
	if len(remaining) != 0 {
		t.Fatalf("expected no extra owner-floor peer frames after DEAD + damage-info, got %d", len(remaining))
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate merchant restart floor close, got %d", len(queued))
	}
	packetBuyOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop-buy error after merchant restart immediate floor close: %v", err)
	}
	if len(packetBuyOut) != 0 {
		t.Fatalf("expected merchant restart immediate floor close to clear packet-buy context, got %d frames", len(packetBuyOut))
	}
	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected shop-end error after merchant restart immediate floor close: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected merchant restart immediate floor close to consume active shop context before later SHOP END, got %d frames", len(closeOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after merchant restart immediate floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after merchant restart immediate floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      partner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_here exchange start after merchant immediate floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_here exchange start to succeed after merchant immediate floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_here exchange start after merchant immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], partner.VID, "post-restart_here exchange start after merchant immediate floor clear")
	partnerStart := flushServerFrames(t, partnerFlow)
	if len(partnerStart) != 1 {
		t.Fatalf("expected partner exchange start after merchant immediate floor clear, got %d", len(partnerStart))
	}
	assertExchangeStartFrame(t, partnerStart[0], owner.VID, "partner exchange start after merchant immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "merchant restart immediate floor close inventory/gold")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMerchantBeforeRestartExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("MerchantRestartDelayedOwner", 0x01040e23, 0x02050e23, 125, nil)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	partner := peerVisibilityCharacter("MerchantRestartDelayedPartner", 0x01040e24, 0x02050e24, 1120, 2120, 0, 101, 201)
	partner.Gold = 22222
	login := "merch-rst-del"
	loginKey := uint32(0x2e2e0e23)
	partnerLogin := "merch-rst-del-p"
	partnerLoginKey := uint32(0x2e2e0e24)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, partnerLogin, partnerLoginKey, partner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed merchant restart delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: partnerLogin, Empire: partner.Empire, Characters: cloneCharacters([]loginticket.Character{partner})}); err != nil {
		t.Fatalf("seed merchant restart delayed floor-close partner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()}}); err != nil {
		t.Fatalf("seed merchant restart delayed floor-close interaction store: %v", err)
	}
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant restart delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000823, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1200,
			Y:               2200,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_merchant_restart_delayed_floor_close",
			Name:          "PracticeMobMerchantRestartDelayedFloorClose",
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
		t.Fatalf("import content bundle for merchant restart delayed floor-close test: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 11 {
		t.Fatalf("expected merchant restart delayed floor-close owner bootstrap to emit at least 11 frames, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	partnerFlow, partnerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), partnerLogin, partnerLoginKey)
	if len(partnerEnter) < 14 {
		t.Fatalf("expected merchant restart delayed floor-close partner bootstrap to emit at least 14 frames, got %d", len(partnerEnter))
	}
	defer closeSessionFlow(t, partnerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive partner peer-entry frames before merchant restart delayed floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, partnerFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	var merchantVID uint32
	for _, actor := range actors {
		switch actor.Name {
		case "PracticeMobMerchantRestartDelayedFloorClose":
			targetVID = uint32(actor.EntityID)
		case "Merchant":
			merchantVID = uint32(actor.EntityID)
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}
	if merchantVID == 0 {
		t.Fatalf("expected merchant actor after import, got %+v", actors)
	}

	interactWithMerchantForBuy(t, ownerFlow, uint64(merchantVID))

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before merchant restart delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before merchant restart delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before merchant restart delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before merchant restart delayed floor-close, got %d frames", len(attackOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	currentTime = currentTime.Add(time.Second)
	delayedOut := flushServerFrames(t, ownerFlow)
	if len(delayedOut) != 5 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, owner damage-info, and merchant close frames, got %d", len(delayedOut))
	}
	next := assertOwnerFloorDeathSequence(t, delayedOut, 0, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "merchant_practice_mob_floor_restart_exchange owner-floor")
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, delayedOut[next])); err != nil {
		t.Fatalf("decode merchant restart delayed floor-close shop end: %v", err)
	}

	partnerQueued := flushServerFrames(t, partnerFlow)
	if len(partnerQueued) != 2 {
		t.Fatalf("expected partner DEAD plus owner damage-info after merchant restart delayed floor, got %d", len(partnerQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, partnerQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "merchant_practice_mob_floor_restart_exchange owner-floor peer")
	if len(remaining) != 0 {
		t.Fatalf("expected no extra owner-floor peer frames after DEAD + damage-info, got %d", len(remaining))
	}

	packetBuyOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop-buy error after merchant restart delayed floor close: %v", err)
	}
	if len(packetBuyOut) != 0 {
		t.Fatalf("expected merchant restart delayed floor close to clear packet-buy context, got %d frames", len(packetBuyOut))
	}
	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected shop-end error after merchant restart delayed floor close: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected merchant restart delayed floor close to consume active shop context before later SHOP END, got %d frames", len(closeOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after merchant restart delayed floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after merchant restart delayed floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      partner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_here exchange start after merchant delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_here exchange start to succeed after merchant delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_here exchange start after merchant delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], partner.VID, "post-restart_here exchange start after merchant delayed floor clear")
	partnerStart := flushServerFrames(t, partnerFlow)
	if len(partnerStart) != 1 {
		t.Fatalf("expected partner exchange start after merchant delayed floor clear, got %d", len(partnerStart))
	}
	assertExchangeStartFrame(t, partnerStart[0], owner.VID, "partner exchange start after merchant delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "merchant restart delayed floor close inventory/gold")
}
