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
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMerchantBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("MerchantTownImmediateOwner", 0x01040f21, 0x02050f21, 125, nil)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	sourcePeer := peerVisibilityCharacter("MerchantTownImmediateSource", 0x01040f22, 0x02050f22, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("MerchantTownImmediateTown", 0x01040f23, 0x02050f23, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "merch-town-imm"
	loginKey := uint32(0x2e2e0f21)
	sourceLogin := "merch-town-imm-s"
	sourceLoginKey := uint32(0x2e2e0f22)
	townLogin := "merch-town-imm-t"
	townLoginKey := uint32(0x2e2e0f23)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed merchant town immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed merchant town immediate floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed merchant town immediate floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()}}); err != nil {
		t.Fatalf("seed merchant town immediate floor-close interaction store: %v", err)
	}
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant town immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000921, 0)
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
			Ref:           "practice.mob_merchant_town_immediate_floor_close",
			Name:          "PracticeMobMerchantTownImmediateFloorClose",
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
		t.Fatalf("import content bundle for merchant town immediate floor-close test: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 11 {
		t.Fatalf("expected merchant town immediate floor-close owner bootstrap to emit at least 11 frames, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 14 {
		t.Fatalf("expected merchant town immediate floor-close source peer bootstrap to emit at least 14 frames, got %d", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before merchant town immediate floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for town peer on destination map, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued owner frames before floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued source frames before floor, got %d", len(queued))
	}

	actors := runtime.StaticActors()
	var targetVID uint32
	var merchantVID uint32
	for _, actor := range actors {
		switch actor.Name {
		case "PracticeMobMerchantTownImmediateFloorClose":
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
		t.Fatalf("unexpected target-selection error before merchant town immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before merchant town immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before merchant town immediate floor-close: %v", err)
	}
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, and merchant close frames, got %d", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode merchant town immediate floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected immediate retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode merchant town immediate floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected merchant town immediate floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode merchant town immediate floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected merchant town immediate floor-close clear target, got %+v", clear)
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, attackOut[4])); err != nil {
		t.Fatalf("decode merchant town immediate floor-close shop end: %v", err)
	}

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 1 {
		t.Fatalf("expected source peer DEAD after merchant town immediate floor, got %d", len(sourceQueued))
	}
	sourceDead, err := worldproto.DecodeDead(decodeSingleFrame(t, sourceQueued[0]))
	if err != nil {
		t.Fatalf("decode source peer DEAD after merchant town immediate floor-close: %v", err)
	}
	if sourceDead.VID != owner.VID {
		t.Fatalf("expected source peer DEAD for owner VID %d, got %+v", owner.VID, sourceDead)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate merchant town floor close, got %d", len(queued))
	}
	packetBuyOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop-buy error after merchant town immediate floor close: %v", err)
	}
	if len(packetBuyOut) != 0 {
		t.Fatalf("expected merchant town immediate floor close to clear packet-buy context, got %d frames", len(packetBuyOut))
	}
	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected shop-end error after merchant town immediate floor close: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected merchant town immediate floor close to consume active shop context before later SHOP END, got %d frames", len(closeOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after merchant town immediate floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after merchant town immediate floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after merchant town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after merchant immediate floor, got %+v", selfAdd)
	}
	var (
		selfPoints    worldproto.PlayerPointChangePacket
		foundPoints   bool
		foundTownPeer bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
				continue
			}
		}
		if add, err := worldproto.DecodeCharacterAdd(fr); err == nil && add.VID == townPeer.VID {
			foundTownPeer = true
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after merchant immediate floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownPeer {
		t.Fatalf("expected /restart_town destination visibility delta to add town peer vid %d", townPeer.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after /restart_town, got %d", len(townQueued))
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      townPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town exchange start after merchant immediate floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after merchant immediate floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after merchant immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], townPeer.VID, "post-restart_town exchange start after merchant immediate floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after merchant immediate floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after merchant immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "merchant town immediate floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after merchant town immediate /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after merchant town immediate /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after merchant immediate floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after merchant immediate floor, got %+v", wantHP, persisted.Characters[0])
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMerchantBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("MerchantTownDelayedOwner", 0x01040f24, 0x02050f24, 125, nil)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	sourcePeer := peerVisibilityCharacter("MerchantTownDelayedSource", 0x01040f25, 0x02050f25, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("MerchantTownDelayedTown", 0x01040f26, 0x02050f26, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "merch-town-del"
	loginKey := uint32(0x2e2e0f24)
	sourceLogin := "merch-town-del-s"
	sourceLoginKey := uint32(0x2e2e0f25)
	townLogin := "merch-town-del-t"
	townLoginKey := uint32(0x2e2e0f26)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed merchant town delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed merchant town delayed floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed merchant town delayed floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	if err := interactionStore.Save(interactionstore.Snapshot{Definitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()}}); err != nil {
		t.Fatalf("seed merchant town delayed floor-close interaction store: %v", err)
	}
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant town delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000924, 0)
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
			Ref:           "practice.mob_merchant_town_delayed_floor_close",
			Name:          "PracticeMobMerchantTownDelayedFloorClose",
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
		t.Fatalf("import content bundle for merchant town delayed floor-close test: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 11 {
		t.Fatalf("expected merchant town delayed floor-close owner bootstrap to emit at least 11 frames, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 14 {
		t.Fatalf("expected merchant town delayed floor-close source peer bootstrap to emit at least 14 frames, got %d", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before merchant town delayed floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for town peer on destination map, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued owner frames before delayed floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued source frames before delayed floor, got %d", len(queued))
	}

	actors := runtime.StaticActors()
	var targetVID uint32
	var merchantVID uint32
	for _, actor := range actors {
		switch actor.Name {
		case "PracticeMobMerchantTownDelayedFloorClose":
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
		t.Fatalf("unexpected target-selection error before merchant town delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before merchant town delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before merchant town delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before merchant town delayed floor-close, got %d frames", len(attackOut))
	}
	_ = flushServerFrames(t, sourceFlow)

	currentTime = currentTime.Add(time.Second)
	delayedOut := flushServerFrames(t, ownerFlow)
	if len(delayedOut) != 4 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, and merchant close frames, got %d", len(delayedOut))
	}
	delayedPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, delayedOut[0]))
	if err != nil {
		t.Fatalf("decode merchant town delayed floor-close point-change: %v", err)
	}
	if delayedPoint.Value != 0 {
		t.Fatalf("expected delayed retaliation floor to drop owner HP to 0, got %+v", delayedPoint)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, delayedOut[1]))
	if err != nil {
		t.Fatalf("decode merchant town delayed floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected merchant town delayed floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, delayedOut[2]))
	if err != nil {
		t.Fatalf("decode merchant town delayed floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected merchant town delayed floor-close clear target, got %+v", clear)
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, delayedOut[3])); err != nil {
		t.Fatalf("decode merchant town delayed floor-close shop end: %v", err)
	}

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 1 {
		t.Fatalf("expected source peer DEAD after merchant town delayed floor, got %d", len(sourceQueued))
	}
	sourceDead, err := worldproto.DecodeDead(decodeSingleFrame(t, sourceQueued[0]))
	if err != nil {
		t.Fatalf("decode source peer DEAD after merchant town delayed floor-close: %v", err)
	}
	if sourceDead.VID != owner.VID {
		t.Fatalf("expected source peer DEAD for owner VID %d, got %+v", owner.VID, sourceDead)
	}

	packetBuyOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop-buy error after merchant town delayed floor close: %v", err)
	}
	if len(packetBuyOut) != 0 {
		t.Fatalf("expected merchant town delayed floor close to clear packet-buy context, got %d frames", len(packetBuyOut))
	}
	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected shop-end error after merchant town delayed floor close: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected merchant town delayed floor close to consume active shop context before later SHOP END, got %d frames", len(closeOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after merchant town delayed floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after merchant town delayed floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after merchant town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after merchant delayed floor, got %+v", selfAdd)
	}
	var (
		selfPoints    worldproto.PlayerPointChangePacket
		foundPoints   bool
		foundTownPeer bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
				continue
			}
		}
		if add, err := worldproto.DecodeCharacterAdd(fr); err == nil && add.VID == townPeer.VID {
			foundTownPeer = true
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after merchant delayed floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownPeer {
		t.Fatalf("expected /restart_town destination visibility delta to add town peer vid %d", townPeer.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after /restart_town, got %d", len(townQueued))
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      townPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town exchange start after merchant delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after merchant delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after merchant delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], townPeer.VID, "post-restart_town exchange start after merchant delayed floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after merchant delayed floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after merchant delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "merchant town delayed floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after merchant town delayed /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after merchant town delayed /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after merchant delayed floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after merchant delayed floor, got %+v", wantHP, persisted.Characters[0])
	}
}
