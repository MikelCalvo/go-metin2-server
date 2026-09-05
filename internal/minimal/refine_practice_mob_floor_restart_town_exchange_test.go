package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenRefineBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineTownImmediateOwner", 0x01030d11, 0x02040d11, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 911, Vnum: 11240, Count: 1, Slot: 5}}
	sourcePeer := peerVisibilityCharacter("RefineTownImmediateSource", 0x01030d12, 0x02040d12, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("RefineTownImmediateTown", 0x01030d13, 0x02040d13, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "refine-town-immediate-floor"
	loginKey := uint32(0x7070dd11)
	sourceLogin := "refine-town-immediate-source"
	sourceLoginKey := uint32(0x7070dd12)
	townLogin := "refine-town-immediate-town"
	townLoginKey := uint32(0x7070dd13)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed refine town immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed refine town immediate floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed refine town immediate floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	sourceTemplate := itemcatalog.Template{
		Vnum:       11240,
		Name:       "Refine Town Immediate Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11241,
			Cost:        1000,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11241, Name: "Refine Town Immediate Result Blade", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Refine Town Immediate Material", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected refine town immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000721, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.registerStaticActorWithInteractionCombatProfileAndSpawnGroupRef(
		"PracticeMobRefineTownImmediateFloorClose",
		bootstrapMapIndex,
		1200,
		2200,
		101,
		"",
		"",
		string(worldruntime.StaticActorCombatProfileTrainingDummy),
		"practice.mob_refine_town_immediate_floor_close",
	)
	if !ok {
		t.Fatal("expected refine town immediate floor-close practice mob registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) != 9 {
		t.Fatalf("expected 9 owner bootstrap frames with inventory item plus visible practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) != 11 {
		t.Fatalf("expected 11 source peer bootstrap frames with visible owner and mob, got %d", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before refine town immediate floor-close, got %d", len(queued))
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

	previewOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected refine preview before town immediate floor-close: %v", err)
	}
	if len(previewOut) != 1 {
		t.Fatalf("expected refine preview before town immediate floor-close to emit one frame, got %d", len(previewOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode refine preview before town immediate floor-close: %v", err)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before refine town immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before refine town immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before refine town immediate floor-close: %v", err)
	}
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, and owner damage-info with no extra refine frames, got %d", len(attackOut))
	}
	_ = assertOwnerFloorDeathSequence(t, attackOut, 1, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "refine_practice_mob_floor_restart_town_exchange owner-floor")

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 2 {
		t.Fatalf("expected source peer DEAD plus owner damage-info after refine town immediate floor, got %d", len(sourceQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, sourceQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "refine_practice_mob_floor_restart_town_exchange owner-floor peer")
	if len(remaining) != 0 {
		t.Fatalf("expected no extra owner-floor peer frames after DEAD + damage-info, got %d", len(remaining))
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate refine town floor close, got %d", len(queued))
	}
	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 255})))
	if err != nil {
		t.Fatalf("unexpected already-closed refine cancel after town immediate floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed refine cancel after town immediate floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after refine town floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after refine town floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after refine town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after refine floor, got %+v", selfAdd)
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
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after refine floor, got %+v", wantHP, selfPoints)
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
		t.Fatalf("unexpected post-restart_town exchange start after refine immediate floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after refine immediate floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after refine immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], townPeer.VID, "post-restart_town exchange start after refine immediate floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after refine immediate floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after refine immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "refine town immediate floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after refine town immediate /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after refine town immediate /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after refine floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after refine floor, got %+v", wantHP, persisted.Characters[0])
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenRefineBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RefineTownDelayedOwner", 0x01030d14, 0x02040d14, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 912, Vnum: 11240, Count: 1, Slot: 5}}
	sourcePeer := peerVisibilityCharacter("RefineTownDelayedSource", 0x01030d15, 0x02040d15, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("RefineTownDelayedTown", 0x01030d16, 0x02040d16, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "refine-town-delayed-floor"
	loginKey := uint32(0x7070dd14)
	sourceLogin := "refine-town-delayed-source"
	sourceLoginKey := uint32(0x7070dd15)
	townLogin := "refine-town-delayed-town"
	townLoginKey := uint32(0x7070dd16)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed refine town delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed refine town delayed floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed refine town delayed floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	sourceTemplate := itemcatalog.Template{
		Vnum:       11240,
		Name:       "Refine Town Delayed Practice Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11241,
			Cost:        1000,
			Probability: 75,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	resultTemplate := itemcatalog.Template{Vnum: 11241, Name: "Refine Town Delayed Result Blade", Stackable: false, MaxCount: 1}
	materialTemplate := itemcatalog.Template{Vnum: 27001, Name: "Refine Town Delayed Material", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{sourceTemplate, resultTemplate, materialTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected refine town delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000724, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.registerStaticActorWithInteractionCombatProfileAndSpawnGroupRef(
		"PracticeMobRefineTownDelayedFloorClose",
		bootstrapMapIndex,
		1200,
		2200,
		101,
		"",
		"",
		string(worldruntime.StaticActorCombatProfileTrainingDummy),
		"practice.mob_refine_town_delayed_floor_close",
	)
	if !ok {
		t.Fatal("expected refine town delayed floor-close practice mob registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) != 9 {
		t.Fatalf("expected 9 owner bootstrap frames with inventory item plus visible practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) != 11 {
		t.Fatalf("expected 11 source peer bootstrap frames with visible owner and mob, got %d", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before refine town delayed floor-close, got %d", len(queued))
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

	previewOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected refine preview before town delayed floor-close: %v", err)
	}
	if len(previewOut) != 1 {
		t.Fatalf("expected refine preview before town delayed floor-close to emit one frame, got %d", len(previewOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode refine preview before town delayed floor-close: %v", err)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before refine town delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before refine town delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before refine town delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before delayed floor, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) == 0 {
		t.Fatal("expected source peer to receive live-hit retaliation fanout before delayed floor")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 4 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, and owner damage-info with no extra refine frames, got %d", len(queued))
	}
	_ = assertOwnerFloorDeathSequence(t, queued, 0, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "refine_practice_mob_floor_restart_town_exchange owner-floor")

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 2 {
		t.Fatalf("expected source peer DEAD plus owner damage-info after refine town delayed floor, got %d", len(sourceQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, sourceQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "refine_practice_mob_floor_restart_town_exchange owner-floor peer")
	if len(remaining) != 0 {
		t.Fatalf("expected no extra owner-floor peer frames after DEAD + damage-info, got %d", len(remaining))
	}

	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 255})))
	if err != nil {
		t.Fatalf("unexpected already-closed refine cancel after town delayed floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed refine cancel after town delayed floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after refine town floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after refine town floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after refine town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after refine delayed floor, got %+v", selfAdd)
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
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after refine delayed floor, got %+v", wantHP, selfPoints)
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
		t.Fatalf("unexpected post-restart_town exchange start after refine delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after refine delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after refine delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], townPeer.VID, "post-restart_town exchange start after refine delayed floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after refine delayed floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after refine delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "refine town delayed floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after refine town delayed /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after refine town delayed /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after refine delayed floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after refine delayed floor, got %+v", wantHP, persisted.Characters[0])
	}
}
