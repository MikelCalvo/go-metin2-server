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
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, "cube-immediate-floor-peer", 0x70708aa6, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube immediate floor-close owner account: %v", err)
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
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "cube-immediate-floor-peer", 0x70708aa6)
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
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, "cube-delayed-floor-peer", 0x70708aa8, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube delayed floor-close owner account: %v", err)
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
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "cube-delayed-floor-peer", 0x70708aa8)
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
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube delayed floor close")
}
