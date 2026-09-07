package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Exact-position MOVE and SYNC_POSITION transfer triggers both reuse
// applySelectedCharacterTransfer. After a chase-displaced within_radius
// practice mob, each client ingress must clear chase and arm homeward just as
// RelocateCharacter and authored warp INTERACT do.
func TestGameRuntimeMoveTransferTriggerClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	assertExactPositionTransferTriggerClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t, "MOVE")
}

func TestGameRuntimeSyncPositionTransferTriggerClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	assertExactPositionTransferTriggerClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t, "SYNC_POSITION")
}

func assertExactPositionTransferTriggerClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T, ingress string) {
	t.Helper()

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("TriggerHomewardOwner", 0x01030901, 0x02040901, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	// Watcher stays outside default aggro of displaced 1800 and home 1700 so
	// proximity cannot re-engage after the owner transfers, while remaining a
	// retained visibility viewer for the homeward MOVE.
	watcher := peerVisibilityCharacter("TriggerHomewardWatcher", 0x01030902, 0x02040902, 1450, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "trigger-homeward-owner", 0x91919191, owner)
	issuePeerTicket(t, store, "trigger-homeward-watcher", 0x91919192, watcher)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005600, 0)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     800,
			VisibilitySectorSize: 200,
		},
		store,
		nil,
		staticActorStore,
		interactionStore,
		[]bootstrapTransferTrigger{{
			SourceMapIndex: 42,
			SourceX:        2000,
			SourceY:        2800,
			TargetMapIndex: 43,
			TargetX:        5000,
			TargetY:        5000,
		}},
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error for %s transfer-trigger chase/homeward: %v", ingress, err)
	}
	runtime.now = func() time.Time { return currentTime }

	const spawnRef = "practice.transfer_trigger_homeward_after_chase"
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           spawnRef,
		Name:          "TransferTriggerHomewardMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import %s transfer-trigger chase/homeward spawn-group bundle: %v", ingress, err)
	}
	group, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok {
		t.Fatalf("expected %s transfer-trigger chase/homeward spawn group to resolve by ref", ingress)
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "trigger-homeward-owner", 0x91919191)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "trigger-homeward-watcher", 0x91919192)
	defer closeSessionFlow(t, watcherFlow)
	flushServerFrames(t, watcherFlow)
	flushServerFrames(t, ownerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before %s transfer-trigger chase displace: %v", ingress, err)
	}
	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before %s transfer-trigger chase displace: %v", ingress, err)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm chase before %s transfer trigger, ok=%v snapshot=%+v", ingress, ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before %s transfer-trigger chase displace, got %d frames", ingress, len(queued))
	}
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatalf("expected due chase-step to displace actor toward owner before %s transfer trigger", ingress)
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before %s transfer trigger: %v", ingress, err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward owner before %s transfer trigger, got %+v", ingress, chaseMove)
	}
	_ = flushServerFrames(t, watcherFlow)

	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.X != 1800 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected chase-displaced within_radius actor before %s transfer trigger, ok=%v snapshot=%+v", ingress, ok, displaced)
	}

	var transferOut [][]byte
	switch ingress {
	case "MOVE":
		transferOut, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
			Func: 1,
			Arg:  0,
			Rot:  12,
			X:    2000,
			Y:    2800,
			Time: 0x71727374,
		})))
	case "SYNC_POSITION":
		transferOut, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
			Elements: []movep.SyncPositionElement{{VID: owner.VID, X: 2000, Y: 2800}},
		})))
	default:
		t.Fatalf("unsupported transfer-trigger ingress %q", ingress)
	}
	if err != nil {
		t.Fatalf("unexpected %s transfer trigger error after chase displace: %v", ingress, err)
	}
	if !containsServerTargetClear(t, transferOut) {
		t.Fatalf("expected %s transfer rebootstrap response to carry selected-target clear, got %d frames", ingress, len(transferOut))
	}
	connected := runtime.ConnectedCharacters()
	if len(connected) != 2 {
		t.Fatalf("expected owner and watcher to remain connected after %s transfer trigger, got %+v", ingress, connected)
	}
	var ownerConnected bool
	for _, snapshot := range connected {
		if snapshot.Name != owner.Name {
			continue
		}
		ownerConnected = true
		if snapshot.MapIndex != 43 || snapshot.X != 5000 || snapshot.Y != 5000 {
			t.Fatalf("expected %s transfer destination 43,5000,5000, got %+v", ingress, snapshot)
		}
	}
	if !ownerConnected {
		t.Fatalf("expected owner connected snapshot after %s transfer trigger, got %+v", ingress, connected)
	}
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, watcherFlow)

	runtime.spawnChaseMu.Lock()
	_, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduled {
		t.Fatalf("expected owner %s transfer trigger to clear pending chase deadline for entity %d", ingress, group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase-step inspection to omit actor after %s transfer trigger, ok=%v snapshot=%+v", ingress, ok, pending)
	}

	runtime.spawnHomewardMu.Lock()
	homewardDueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected %s transfer-trigger engagement release on within_radius displace to arm homeward deadline for entity %d", ingress, group.EntityID)
	}
	expectedHomewardDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !homewardDueAt.Equal(expectedHomewardDueAt) {
		t.Fatalf("expected %s transfer-trigger homeward deadline at %s, got %s", ingress, expectedHomewardDueAt, homewardDueAt)
	}

	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no homeward MOVE on watcher before the 1s deadline after %s transfer trigger, got %d frames", ingress, len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	homewardQueued := flushServerFrames(t, watcherFlow)
	if len(homewardQueued) == 0 {
		t.Fatalf("expected due homeward-step after %s transfer trigger to queue retained watcher MOVE toward home", ingress)
	}
	homewardMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, homewardQueued[0]))
	if err != nil {
		t.Fatalf("expected retained watcher after %s transfer trigger to receive MOVE replication, first frame decode err=%v", ingress, err)
	}
	if homewardMove.VID != targetVID || homewardMove.X != 1700 || homewardMove.Y != 2800 {
		t.Fatalf("expected homeward MOVE to authored home after %s transfer trigger, got %+v", ingress, homewardMove)
	}
	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected homeward-step after %s transfer trigger to restore at_home leash state, ok=%v snapshot=%+v", ingress, ok, returned)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillHomeward := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillHomeward {
		t.Fatalf("expected completed at-home homeward-step after %s transfer trigger to clear pending deadline for entity %d", ingress, group.EntityID)
	}
}
