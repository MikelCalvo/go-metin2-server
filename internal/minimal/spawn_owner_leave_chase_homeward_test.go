package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Owner disappearance after a chase-displaced within_radius practice mob must
// clear the pending chase deadline and arm homeward, matching TARGET(0) /
// death-floor / walk-away release. Slash quit/logout/phase_select clear combat
// ownership before Leave so chase prune + within_radius homeward re-arm still
// see the engagements that subject owned (matching abrupt close).
func TestGameRuntimeSlashQuitClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	assertOwnerLeaveClearsChaseAndArmsHomewardAfterChaseDisplace(t, "/quit")
}

func TestGameRuntimeSlashLogoutClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	assertOwnerLeaveClearsChaseAndArmsHomewardAfterChaseDisplace(t, "/logout")
}

func TestGameRuntimePhaseSelectClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	assertOwnerLeaveClearsChaseAndArmsHomewardAfterChaseDisplace(t, "/phase_select")
}

func assertOwnerLeaveClearsChaseAndArmsHomewardAfterChaseDisplace(t *testing.T, leaveCommand string) {
	t.Helper()

	store := loginticket.NewFileStore(t.TempDir())
	// Owner at +200 so one due chase beat lands the mob at 1800 (within_radius).
	owner := peerVisibilityCharacter("LeaveHomewardOwner", 0x01030401, 0x02040401, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	// Watcher stays outside default aggro (200) of both displaced 1800 and home
	// 1700 so proximity cannot re-engage after owner disappearance, while still
	// remaining a retained visibility viewer for homeward MOVE.
	watcher := peerVisibilityCharacter("LeaveHomewardWatcher", 0x01030402, 0x02040402, 1450, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "leave-homeward-owner", 0x50505051, owner)
	issuePeerTicket(t, store, "leave-homeward-watcher", 0x50505052, watcher)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005100, 0)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
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
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error for %s chase/homeward leave: %v", leaveCommand, err)
	}
	runtime.now = func() time.Time { return currentTime }

	ref := "practice.leave_homeward_after_chase_" + leaveCommand[1:]
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           ref,
		Name:          "LeaveHomewardMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import %s chase/homeward leave spawn-group bundle: %v", leaveCommand, err)
	}
	group, ok := runtime.SpawnGroupByRef(ref)
	if !ok {
		t.Fatalf("expected %s chase/homeward leave spawn group to resolve by ref", leaveCommand)
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "leave-homeward-owner", 0x50505051)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "leave-homeward-watcher", 0x50505052)
	defer closeSessionFlow(t, watcherFlow)
	flushServerFrames(t, watcherFlow)
	flushServerFrames(t, ownerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before %s chase displace: %v", leaveCommand, err)
	}
	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before %s chase displace: %v", leaveCommand, err)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm chase before %s, ok=%v snapshot=%+v", leaveCommand, ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before %s chase displace, got %d frames", leaveCommand, len(queued))
	}
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatalf("expected due chase-step to displace actor toward owner before %s", leaveCommand)
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before %s: %v", leaveCommand, err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward owner before %s, got %+v", leaveCommand, chaseMove)
	}
	_ = flushServerFrames(t, watcherFlow)

	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.X != 1800 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected chase-displaced within_radius actor before %s, ok=%v snapshot=%+v", leaveCommand, ok, displaced)
	}

	leaveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: leaveCommand,
	})))
	if err != nil {
		t.Fatalf("unexpected %s error after chase displace: %v", leaveCommand, err)
	}
	if len(leaveOut) == 0 {
		t.Fatalf("expected %s to emit at least one close/select frame after chase displace", leaveCommand)
	}
	_ = flushServerFrames(t, watcherFlow)

	runtime.spawnChaseMu.Lock()
	_, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduled {
		t.Fatalf("expected %s to clear pending chase deadline for entity %d", leaveCommand, group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase-step inspection to omit actor after %s, ok=%v snapshot=%+v", leaveCommand, ok, pending)
	}

	runtime.spawnHomewardMu.Lock()
	homewardDueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected %s engagement release on within_radius displace to arm homeward deadline for entity %d", leaveCommand, group.EntityID)
	}
	expectedHomewardDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !homewardDueAt.Equal(expectedHomewardDueAt) {
		t.Fatalf("expected %s homeward deadline at %s, got %s", leaveCommand, expectedHomewardDueAt, homewardDueAt)
	}

	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no homeward MOVE on watcher before the 1s deadline after %s, got %d frames", leaveCommand, len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	homewardQueued := flushServerFrames(t, watcherFlow)
	if len(homewardQueued) == 0 {
		t.Fatalf("expected due homeward-step after %s to queue retained watcher MOVE toward home", leaveCommand)
	}
	homewardMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, homewardQueued[0]))
	if err != nil {
		t.Fatalf("expected retained watcher after %s to receive MOVE replication, first frame decode err=%v", leaveCommand, err)
	}
	if homewardMove.VID != targetVID || homewardMove.X != 1700 || homewardMove.Y != 2800 {
		t.Fatalf("expected homeward MOVE to authored home after %s, got %+v", leaveCommand, homewardMove)
	}
	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected homeward-step after %s to restore at_home leash state, ok=%v snapshot=%+v", leaveCommand, ok, returned)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillHomeward := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillHomeward {
		t.Fatalf("expected completed at-home homeward-step after %s to clear pending deadline for entity %d", leaveCommand, group.EntityID)
	}
}
