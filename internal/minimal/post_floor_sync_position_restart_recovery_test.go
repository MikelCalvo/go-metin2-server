package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorSyncPositionFailsClosed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadSyncOwner", 0x01030f10, 0x02040f10, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	peer := peerVisibilityCharacter("DeadSyncPeer", 0x01030f11, 0x02040f11, 1120, 2120, 0, 101, 201)
	login := "post-floor-sync-owner"
	loginKey := uint32(0x19191f10)
	peerLogin := "post-floor-sync-peer"
	peerLoginKey := uint32(0x19191f11)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor sync owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed post-floor sync peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        3100,
		TargetY:        4200,
	}})
	if err != nil {
		t.Fatalf("unexpected post-floor sync runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_sync",
		Name:          "PracticeMobPostFloorSync",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor sync practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor sync practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

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
		t.Fatalf("expected owner to receive peer-entry frames before post-floor sync, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 2 {
		t.Fatalf("expected peer DEAD plus owner damage-info fanout after owner floor before sync denial, got %d", len(queued))
	}

	assertPostFloorSyncPositionDenied(t, ownerFlow, peerFlow, runtime, owner, 1500, 2600, "post-floor")
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor SYNC_POSITION")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor SYNC_POSITION: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor SYNC_POSITION, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	assertPostFloorSyncPositionRecovered(t, ownerFlow, peerFlow, owner, 1150, 2150, "post-restart")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart SYNC_POSITION: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after sync floor, got %+v", wantHP, account.Characters[0])
	}
}

func TestGameSessionFlowPostFloorSyncPositionFailsClosedBeforeRestartTown(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadSyncTownOwner", 0x01030f12, 0x02040f12, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	sourcePeer := peerVisibilityCharacter("DeadSyncTownSource", 0x01030f13, 0x02040f13, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("DeadSyncTownPeer", 0x01030f14, 0x02040f14, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "pf-sync-town"
	loginKey := uint32(0x19191f12)
	sourceLogin := "pf-sync-town-s"
	sourceLoginKey := uint32(0x19191f13)
	townLogin := "pf-sync-town-t"
	townLoginKey := uint32(0x19191f14)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor town sync owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed post-floor town sync source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed post-floor town sync town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        3100,
		TargetY:        4200,
	}})
	if err != nil {
		t.Fatalf("unexpected post-floor town sync runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_sync_town",
		Name:          "PracticeMobPostFloorSyncTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor town sync practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor town sync practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

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
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) == 0 {
		t.Fatalf("expected town peer bootstrap frames, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before post-floor town sync, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 2 {
		t.Fatalf("expected source peer DEAD plus owner damage-info fanout after owner floor before town sync denial, got %d", len(queued))
	}

	assertPostFloorSyncPositionDenied(t, ownerFlow, sourceFlow, runtime, owner, 1500, 2600, "post-floor town")
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town SYNC_POSITION")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor SYNC_POSITION: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor SYNC_POSITION, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor SYNC_POSITION /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after sync floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after sync floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after sync floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	recoveredOwner := owner
	recoveredOwner.MapIndex = 21
	recoveredOwner.X = 52070
	recoveredOwner.Y = 166600
	assertPostFloorSyncPositionRecovered(t, ownerFlow, townFlow, recoveredOwner, 52100, 166650, "post-restart_town")

	foundOwner := false
	for _, snapshot := range runtime.ConnectedCharacters() {
		if snapshot.Name != owner.Name {
			continue
		}
		foundOwner = true
		if snapshot.MapIndex != 21 || snapshot.X != 52100 || snapshot.Y != 166650 {
			t.Fatalf("expected recovered post-restart_town SYNC_POSITION to leave live owner at empire town destination, got %+v", snapshot)
		}
	}
	if !foundOwner {
		t.Fatal("expected owner snapshot to remain connected after post-restart_town SYNC_POSITION recovery")
	}

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town SYNC_POSITION: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52100 || account.Characters[0].Y != 166650 {
		t.Fatalf("expected recovered post-restart_town SYNC_POSITION to persist empire town destination, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after sync floor, got %+v", wantHP, account.Characters[0])
	}
}

func assertPostFloorSyncPositionDenied(t *testing.T, ownerFlow, peerFlow service.SessionFlow, runtime *gameRuntime, owner loginticket.Character, x, y int32, context string) {
	t.Helper()

	syncOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: owner.VID, X: x, Y: y}},
	})))
	if err != nil {
		t.Fatalf("unexpected %s SYNC_POSITION dispatch error: %v", context, err)
	}
	if len(syncOut) != 0 {
		t.Fatalf("expected %s SYNC_POSITION to fail closed with no frames, got %d", context, len(syncOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected %s SYNC_POSITION to queue no owner frames, got %d", context, len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected %s SYNC_POSITION to queue no peer frames, got %d", context, len(queued))
	}

	connected := runtime.ConnectedCharacters()
	foundOwner := false
	for _, snapshot := range connected {
		if snapshot.Name != owner.Name {
			continue
		}
		foundOwner = true
		if snapshot.MapIndex != owner.MapIndex || snapshot.X != owner.X || snapshot.Y != owner.Y {
			t.Fatalf("expected %s SYNC_POSITION denial to keep live owner at pre-death coordinates, got %+v", context, snapshot)
		}
	}
	if !foundOwner {
		t.Fatalf("expected owner snapshot to remain connected after %s SYNC_POSITION denial, got %#v", context, connected)
	}
}

func assertPostFloorSyncPositionRecovered(t *testing.T, ownerFlow, peerFlow service.SessionFlow, owner loginticket.Character, x, y int32, context string) {
	t.Helper()

	syncOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: owner.VID, X: x, Y: y}},
	})))
	if err != nil {
		t.Fatalf("unexpected %s SYNC_POSITION recovery: %v", context, err)
	}
	if len(syncOut) == 0 {
		t.Fatalf("expected %s SYNC_POSITION to emit self SYNC_POSITION_ACK, got %d frames", context, len(syncOut))
	}
	selfAck, err := movep.DecodeSyncPositionAck(decodeSingleFrame(t, syncOut[0]))
	if err != nil {
		t.Fatalf("decode %s self SYNC_POSITION_ACK: %v", context, err)
	}
	if len(selfAck.Elements) != 1 || selfAck.Elements[0].VID != owner.VID || selfAck.Elements[0].X != x || selfAck.Elements[0].Y != y {
		t.Fatalf("unexpected %s self SYNC_POSITION_ACK: %+v", context, selfAck)
	}
	_ = flushServerFrames(t, peerFlow)
}
