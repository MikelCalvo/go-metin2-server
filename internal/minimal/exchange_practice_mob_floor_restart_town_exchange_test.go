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
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenExchangeShellBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeTownImmediateOwner", 0x01030f90, 0x02040f90, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 125
	sourcePeer := peerVisibilityCharacter("ExchangeTownImmediateSource", 0x01030f91, 0x02040f91, 1120, 2120, 0, 101, 201)
	sourcePeer.Gold = 22222
	townPeer := peerVisibilityCharacter("ExchangeTownImmediateTown", 0x01030f92, 0x02040f92, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "exch-town-imm"
	loginKey := uint32(0x19190f90)
	sourceLogin := "exch-town-imm-s"
	sourceLoginKey := uint32(0x19190f91)
	townLogin := "exch-town-imm-t"
	townLoginKey := uint32(0x19190f92)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange town immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed exchange town immediate floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed exchange town immediate floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange town immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000931, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_exchange_town_immediate_floor_close",
		Name:          "PracticeMobExchangeTownImmediateFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import exchange town immediate floor-close content bundle: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 11 {
		t.Fatalf("expected source peer bootstrap with visible owner and mob, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before exchange town immediate floor-close, got %d", len(queued))
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
	for _, actor := range actors {
		if actor.Name == "PracticeMobExchangeTownImmediateFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      sourcePeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected exchange start before town immediate floor-close: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected one owner exchange start frame before town immediate floor-close, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], sourcePeer.VID, "owner exchange start before town immediate floor-close")
	sourceStart := flushServerFrames(t, sourceFlow)
	if len(sourceStart) != 1 {
		t.Fatalf("expected source peer exchange start before town immediate floor-close, got %d", len(sourceStart))
	}
	assertExchangeStartFrame(t, sourceStart[0], owner.VID, "source peer exchange start before town immediate floor-close")

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before exchange town immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before exchange town immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before exchange town immediate floor-close: %v", err)
	}
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, and exchange END, got %d", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode exchange town immediate floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected immediate retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode exchange town immediate floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected exchange town immediate floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode exchange town immediate floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected exchange town immediate floor-close clear target, got %+v", clear)
	}
	assertExchangeEndFrame(t, attackOut[4], "owner exchange END after town immediate death")

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 2 {
		t.Fatalf("expected source peer DEAD plus exchange END after exchange town immediate floor, got %d", len(sourceQueued))
	}
	sourceDead, err := worldproto.DecodeDead(decodeSingleFrame(t, sourceQueued[0]))
	if err != nil {
		t.Fatalf("decode source peer DEAD after exchange town immediate floor-close: %v", err)
	}
	if sourceDead.VID != owner.VID {
		t.Fatalf("expected source peer DEAD for owner VID %d, got %+v", owner.VID, sourceDead)
	}
	assertExchangeEndFrame(t, sourceQueued[1], "source peer exchange END after town immediate death")

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate exchange town floor close, got %d", len(queued))
	}
	postFloorCancel, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderCancel,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor exchange cancel after town immediate floor: %v", err)
	}
	if len(postFloorCancel) != 0 {
		t.Fatalf("expected post-floor exchange cancel after town immediate floor to fail closed, got %d frames", len(postFloorCancel))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected no stale source exchange frames after post-floor cancel, got %d", len(queued))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after exchange town immediate floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after exchange town immediate floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after exchange town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after exchange immediate floor, got %+v", selfAdd)
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
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after exchange immediate floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownPeer {
		t.Fatalf("expected /restart_town destination visibility delta to add town peer vid %d", townPeer.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after /restart_town, got %d", len(townQueued))
	}

	freshStart, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      townPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town exchange start after exchange immediate floor: %v", err)
	}
	if len(freshStart) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after exchange immediate floor clear, got %d frames", len(freshStart))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, freshStart[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after exchange immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, freshStart[0], townPeer.VID, "post-restart_town exchange start after exchange immediate floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after exchange immediate floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after exchange immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "exchange town immediate floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after exchange town immediate /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after exchange town immediate /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after exchange immediate floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after exchange immediate floor, got %+v", wantHP, persisted.Characters[0])
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenExchangeShellBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeTownDelayedOwner", 0x01030f93, 0x02040f93, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 125
	sourcePeer := peerVisibilityCharacter("ExchangeTownDelayedSource", 0x01030f94, 0x02040f94, 1120, 2120, 0, 101, 201)
	sourcePeer.Gold = 22222
	townPeer := peerVisibilityCharacter("ExchangeTownDelayedTown", 0x01030f95, 0x02040f95, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "exch-town-del"
	loginKey := uint32(0x19190f93)
	sourceLogin := "exch-town-del-s"
	sourceLoginKey := uint32(0x19190f94)
	townLogin := "exch-town-del-t"
	townLoginKey := uint32(0x19190f95)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange town delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed exchange town delayed floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed exchange town delayed floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange town delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000932, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_exchange_town_delayed_floor_close",
		Name:          "PracticeMobExchangeTownDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import exchange town delayed floor-close content bundle: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected delayed owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 11 {
		t.Fatalf("expected delayed source peer bootstrap with visible owner and mob, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected delayed owner to receive source peer-entry frames before exchange town delayed floor-close, got %d", len(queued))
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
	for _, actor := range actors {
		if actor.Name == "PracticeMobExchangeTownDelayedFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after delayed import, got %+v", actors)
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      sourcePeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected delayed exchange start before town floor-close: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected one delayed owner exchange start frame before town floor-close, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], sourcePeer.VID, "delayed owner exchange start before town floor-close")
	sourceStart := flushServerFrames(t, sourceFlow)
	if len(sourceStart) != 1 {
		t.Fatalf("expected delayed source peer exchange start before town floor-close, got %d", len(sourceStart))
	}
	assertExchangeStartFrame(t, sourceStart[0], owner.VID, "delayed source peer exchange start before town floor-close")

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before exchange town delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before exchange town delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected delayed exchange first attack before town floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-change, and self damage-info before delayed exchange town death, got %d frames", len(attackOut))
	}
	firstPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode delayed exchange town first point-change: %v", err)
	}
	if firstPoint.VID != owner.VID || firstPoint.Type != bootstrapPlayerPointType || firstPoint.Amount != -1 || firstPoint.Value != 1 {
		t.Fatalf("unexpected delayed exchange town first point-change: %+v", firstPoint)
	}
	firstSourceQueued := flushServerFrames(t, sourceFlow)
	if len(firstSourceQueued) != 2 {
		t.Fatalf("expected delayed exchange source peer mob + owner retaliation damage-info after first hit, got %d", len(firstSourceQueued))
	}
	assertDamageInfoFrame(t, firstSourceQueued[0], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "delayed exchange town source first hit mob")
	assertDamageInfoFrame(t, firstSourceQueued[1], owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "delayed exchange town source first hit owner retaliation")

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	floorQueued := flushServerFrames(t, ownerFlow)
	if len(floorQueued) != 4 {
		t.Fatalf("expected delayed point-change, self dead, clear-target, and exchange END on owner death, got %d frames", len(floorQueued))
	}
	floorPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, floorQueued[0]))
	if err != nil {
		t.Fatalf("decode delayed exchange town floor point-change: %v", err)
	}
	if floorPoint.VID != owner.VID || floorPoint.Type != bootstrapPlayerPointType || floorPoint.Amount != -1 || floorPoint.Value != 0 {
		t.Fatalf("unexpected delayed exchange town floor point-change: %+v", floorPoint)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, floorQueued[1]))
	if err != nil {
		t.Fatalf("decode delayed exchange town self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected delayed exchange town self dead for owner %#08x, got %#08x", owner.VID, dead.VID)
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, floorQueued[2]))
	if err != nil {
		t.Fatalf("decode delayed exchange town target clear: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected delayed exchange town death to clear active combat target, got %+v", clearTarget)
	}
	assertExchangeEndFrame(t, floorQueued[3], "owner exchange END after delayed town death")

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 2 {
		t.Fatalf("expected source peer DEAD plus exchange END after delayed exchange town floor, got %d frames", len(sourceQueued))
	}
	sourceDead, err := worldproto.DecodeDead(decodeSingleFrame(t, sourceQueued[0]))
	if err != nil {
		t.Fatalf("decode source peer DEAD after delayed exchange town floor-close: %v", err)
	}
	if sourceDead.VID != owner.VID {
		t.Fatalf("expected source peer DEAD for owner VID %d, got %+v", owner.VID, sourceDead)
	}
	assertExchangeEndFrame(t, sourceQueued[1], "source peer exchange END after delayed town death")

	postFloorCancel, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderCancel,
	})))
	if err != nil {
		t.Fatalf("unexpected delayed post-floor exchange cancel after town floor: %v", err)
	}
	if len(postFloorCancel) != 0 {
		t.Fatalf("expected delayed post-floor exchange cancel after town floor to fail closed, got %d frames", len(postFloorCancel))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected no stale delayed source exchange frames after post-floor cancel, got %d", len(queued))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after exchange town delayed floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after exchange town delayed floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after delayed exchange town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after exchange delayed floor, got %+v", selfAdd)
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
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after exchange delayed floor, got %+v", wantHP, selfPoints)
	}
	if !foundTownPeer {
		t.Fatalf("expected /restart_town destination visibility delta to add town peer vid %d", townPeer.VID)
	}
	_ = flushServerFrames(t, sourceFlow)
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after delayed /restart_town, got %d", len(townQueued))
	}

	freshStart, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      townPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_town exchange start after exchange delayed floor: %v", err)
	}
	if len(freshStart) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after exchange delayed floor clear, got %d frames", len(freshStart))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, freshStart[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after exchange delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, freshStart[0], townPeer.VID, "post-restart_town exchange start after exchange delayed floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after exchange delayed floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after exchange delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "exchange town delayed floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after exchange town delayed /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after exchange town delayed /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after exchange delayed floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after exchange delayed floor, got %+v", wantHP, persisted.Characters[0])
	}
}
