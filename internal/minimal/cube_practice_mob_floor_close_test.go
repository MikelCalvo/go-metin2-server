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

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenCube(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeImmediateFloorOwner", 0x010308a5, 0x020408a5, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	peer := peerVisibilityCharacter("CubeImmediateFloorPeer", 0x010308a6, 0x020408a6, 1120, 2120, 0, 101, 201)
	login := "cube-immediate-floor"
	loginKey := uint32(0x70708aa5)
	peerLogin := "cube-immediate-floor-peer"
	peerLoginKey := uint32(0x70708aa6)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed cube immediate floor-close peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected cube immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000495, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_cube_immediate_floor_close",
		Name:          "PracticeMobCubeImmediateFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import cube immediate floor-close content bundle: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, peerLoginKey)
	if len(peerEnter) < 11 {
		t.Fatalf("expected peer bootstrap with visible owner and mob, got %d frames", len(peerEnter))
	}
	defer closeSessionFlow(t, peerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive peer-entry frames before cube immediate floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobCubeImmediateFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before immediate floor-close: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before immediate floor-close to emit one cube open command chat, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "cube immediate floor-close open")

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before cube immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before cube immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before cube immediate floor-close: %v", err)
	}
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, and cube close, got %d", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode cube immediate floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected immediate retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode cube immediate floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected cube immediate floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode cube immediate floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected cube immediate floor-close clear target, got %+v", clear)
	}
	assertCubeCommandChatFrame(t, attackOut[4], "cube close", "cube immediate floor-close")

	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 1 {
		t.Fatalf("expected peer DEAD after cube immediate floor, got %d", len(peerQueued))
	}
	peerDead, err := worldproto.DecodeDead(decodeSingleFrame(t, peerQueued[0]))
	if err != nil {
		t.Fatalf("decode peer DEAD after cube immediate floor-close: %v", err)
	}
	if peerDead.VID != owner.VID {
		t.Fatalf("expected peer DEAD for owner VID %d, got %+v", owner.VID, peerDead)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate cube floor close, got %d", len(queued))
	}
	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_cube after immediate floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_cube after immediate floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after cube immediate floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after cube immediate floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart exchange start after cube immediate floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart exchange start to succeed after cube immediate floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart exchange start after cube immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "post-restart exchange start after cube immediate floor clear")
	peerStart := flushServerFrames(t, peerFlow)
	if len(peerStart) != 1 {
		t.Fatalf("expected peer exchange start after cube immediate floor clear, got %d", len(peerStart))
	}
	assertExchangeStartFrame(t, peerStart[0], owner.VID, "peer exchange start after cube immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube immediate floor close")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenCube(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeDelayedFloorOwner", 0x010308a7, 0x020408a7, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 5000
	peer := peerVisibilityCharacter("CubeDelayedFloorPeer", 0x010308a8, 0x020408a8, 1120, 2120, 0, 101, 201)
	login := "cube-delayed-floor"
	loginKey := uint32(0x70708aa7)
	peerLogin := "cube-delayed-floor-peer"
	peerLoginKey := uint32(0x70708aa8)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed cube delayed floor-close peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected cube delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000497, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_cube_delayed_floor_close",
		Name:          "PracticeMobCubeDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import cube delayed floor-close content bundle: %v", err)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, peerLoginKey)
	if len(peerEnter) < 11 {
		t.Fatalf("expected peer bootstrap with visible owner and mob, got %d frames", len(peerEnter))
	}
	defer closeSessionFlow(t, peerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive peer-entry frames before cube delayed floor-close, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	actors := runtime.StaticActors()
	var targetVID uint32
	for _, actor := range actors {
		if actor.Name == "PracticeMobCubeDelayedFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before delayed floor-close: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before delayed floor-close to emit one cube open command chat, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "cube delayed floor-close open")

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before cube delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before cube delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before cube delayed floor-close: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, first point-loss retaliation, and self damage-info before delayed floor, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) == 0 {
		t.Fatal("expected peer to receive live-hit retaliation fanout before delayed floor")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 4 {
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, and cube close, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode cube delayed floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode cube delayed floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected cube delayed floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queued[2]))
	if err != nil {
		t.Fatalf("decode cube delayed floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected cube delayed floor-close clear target, got %+v", clear)
	}
	assertCubeCommandChatFrame(t, queued[3], "cube close", "cube delayed floor-close")

	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 1 {
		t.Fatalf("expected peer DEAD after cube delayed floor, got %d", len(peerQueued))
	}
	peerDead, err := worldproto.DecodeDead(decodeSingleFrame(t, peerQueued[0]))
	if err != nil {
		t.Fatalf("decode peer DEAD after cube delayed floor-close: %v", err)
	}
	if peerDead.VID != owner.VID {
		t.Fatalf("expected peer DEAD for owner VID %d, got %+v", owner.VID, peerDead)
	}

	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_cube after delayed floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_cube after delayed floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after cube delayed floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after cube delayed floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart exchange start after cube delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart exchange start to succeed after cube delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart exchange start after cube delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "post-restart exchange start after cube delayed floor clear")
	peerStart := flushServerFrames(t, peerFlow)
	if len(peerStart) != 1 {
		t.Fatalf("expected peer exchange start after cube delayed floor clear, got %d", len(peerStart))
	}
	assertExchangeStartFrame(t, peerStart[0], owner.VID, "peer exchange start after cube delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube delayed floor close")
}

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenCubeBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeTownImmediateOwner", 0x010308b1, 0x020408b1, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5000
	sourcePeer := peerVisibilityCharacter("CubeTownImmediateSource", 0x010308b2, 0x020408b2, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("CubeTownImmediateTown", 0x010308b3, 0x020408b3, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "cube-town-immediate-floor"
	loginKey := uint32(0x70708bb1)
	sourceLogin := "cube-town-immediate-source"
	sourceLoginKey := uint32(0x70708bb2)
	townLogin := "cube-town-immediate-town"
	townLoginKey := uint32(0x70708bb3)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube town immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed cube town immediate floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed cube town immediate floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, nil)
	if err != nil {
		t.Fatalf("unexpected cube town immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000691, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_cube_town_immediate_floor_close",
		Name:          "PracticeMobCubeTownImmediateFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import cube town immediate floor-close content bundle: %v", err)
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
		t.Fatalf("expected owner to receive source peer-entry frames before cube town immediate floor-close, got %d", len(queued))
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
		if actor.Name == "PracticeMobCubeTownImmediateFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before town immediate floor-close: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before town immediate floor-close to emit one cube open command chat, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "cube town immediate floor-close open")

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before cube town immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before cube town immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before cube town immediate floor-close: %v", err)
	}
	if len(attackOut) != 5 {
		t.Fatalf("expected target refresh, point-loss, self dead, clear-target, and cube close, got %d", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode cube town immediate floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected immediate retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode cube town immediate floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected cube town immediate floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode cube town immediate floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected cube town immediate floor-close clear target, got %+v", clear)
	}
	assertCubeCommandChatFrame(t, attackOut[4], "cube close", "cube town immediate floor-close")

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 1 {
		t.Fatalf("expected source peer DEAD after cube town immediate floor, got %d", len(sourceQueued))
	}
	sourceDead, err := worldproto.DecodeDead(decodeSingleFrame(t, sourceQueued[0]))
	if err != nil {
		t.Fatalf("decode source peer DEAD after cube town immediate floor-close: %v", err)
	}
	if sourceDead.VID != owner.VID {
		t.Fatalf("expected source peer DEAD for owner VID %d, got %+v", owner.VID, sourceDead)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate cube town floor close, got %d", len(queued))
	}
	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_cube after town immediate floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_cube after town immediate floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after cube town floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after cube town floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after cube town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after cube floor, got %+v", selfAdd)
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
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after cube floor, got %+v", wantHP, selfPoints)
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
		t.Fatalf("unexpected post-restart_town exchange start after cube immediate floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after cube immediate floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after cube immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], townPeer.VID, "post-restart_town exchange start after cube immediate floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after cube immediate floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after cube immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube town immediate floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after cube town immediate /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after cube town immediate /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after cube floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after cube floor, got %+v", wantHP, persisted.Characters[0])
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenCubeBeforeRestartTownExchange(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CubeTownDelayedOwner", 0x010308b4, 0x020408b4, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 5000
	sourcePeer := peerVisibilityCharacter("CubeTownDelayedSource", 0x010308b5, 0x020408b5, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("CubeTownDelayedTown", 0x010308b6, 0x020408b6, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "cube-town-delayed-floor"
	loginKey := uint32(0x70708bb4)
	sourceLogin := "cube-town-delayed-source"
	sourceLoginKey := uint32(0x70708bb5)
	townLogin := "cube-town-delayed-town"
	townLoginKey := uint32(0x70708bb6)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube town delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed cube town delayed floor-close source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed cube town delayed floor-close town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, nil)
	if err != nil {
		t.Fatalf("unexpected cube town delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000694, 0)
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_cube_town_delayed_floor_close",
		Name:          "PracticeMobCubeTownDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import cube town delayed floor-close content bundle: %v", err)
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
		t.Fatalf("expected owner to receive source peer-entry frames before cube town delayed floor-close, got %d", len(queued))
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
		if actor.Name == "PracticeMobCubeTownDelayedFloorClose" {
			targetVID = uint32(actor.EntityID)
			break
		}
	}
	if targetVID == 0 {
		t.Fatalf("expected practice mob actor after import, got %+v", actors)
	}

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before town delayed floor-close: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_cube before town delayed floor-close to emit one cube open command chat, got %d", len(openOut))
	}
	assertCubeCommandChatFrame(t, openOut[0], "cube open 20022", "cube town delayed floor-close open")

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before cube town delayed floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before cube town delayed floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before cube town delayed floor-close: %v", err)
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
		t.Fatalf("expected delayed retaliation floor to queue point-loss, self dead, clear-target, and cube close, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode cube town delayed floor-close point-change: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation floor to drop owner HP to 0, got %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode cube town delayed floor-close self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected cube town delayed floor-close DEAD for owner VID %d, got %+v", owner.VID, dead)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queued[2]))
	if err != nil {
		t.Fatalf("decode cube town delayed floor-close clear target: %v", err)
	}
	if clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("expected cube town delayed floor-close clear target, got %+v", clear)
	}
	assertCubeCommandChatFrame(t, queued[3], "cube close", "cube town delayed floor-close")

	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 1 {
		t.Fatalf("expected source peer DEAD after cube town delayed floor, got %d", len(sourceQueued))
	}
	sourceDead, err := worldproto.DecodeDead(decodeSingleFrame(t, sourceQueued[0]))
	if err != nil {
		t.Fatalf("decode source peer DEAD after cube town delayed floor-close: %v", err)
	}
	if sourceDead.VID != owner.VID {
		t.Fatalf("expected source peer DEAD for owner VID %d, got %+v", owner.VID, sourceDead)
	}

	alreadyClosedOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_cube after town delayed floor: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_cube after town delayed floor to emit no frames, got %d", len(alreadyClosedOut))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after cube town floor: %v", err)
	}
	if len(restartOut) < 9 {
		t.Fatalf("expected at least 9 self frames from /restart_town recovery after cube town floor, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after cube town /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after cube delayed floor, got %+v", selfAdd)
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
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after cube delayed floor, got %+v", wantHP, selfPoints)
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
		t.Fatalf("unexpected post-restart_town exchange start after cube delayed floor: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected post-restart_town exchange start to succeed after cube delayed floor clear, got %d frames", len(startOut))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0])); err == nil {
		t.Fatalf("expected post-restart_town exchange start after cube delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, startOut[0], townPeer.VID, "post-restart_town exchange start after cube delayed floor clear")
	townStart := flushServerFrames(t, townFlow)
	if len(townStart) != 1 {
		t.Fatalf("expected town peer exchange start after cube delayed floor clear, got %d", len(townStart))
	}
	assertExchangeStartFrame(t, townStart[0], owner.VID, "town peer exchange start after cube delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube town delayed floor close inventory/gold")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted owner after cube town delayed /restart_town: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after cube town delayed /restart_town, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 21 || persisted.Characters[0].X != 52070 || persisted.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after cube delayed floor, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after cube delayed floor, got %+v", wantHP, persisted.Characters[0])
	}
}
