package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Proximity-armed chase that already displaced a practice mob within_radius must
// clear the pending chase deadline and arm homeward when the owner walks outside
// aggro without a selected combat target. Spec already lists proximity leave-
// radius walk-away among homeward arm sources; the at-home proximity walk-away
// twin only asserts engagement/chase clear, so a future regression on the
// movement release helper could silently leave chase-displaced mobs sitting
// forever off-home after walk-away.
func TestGameRuntimeProximityWalkAwayClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Owner at +200 so one due chase beat lands the mob at 1800 (within_radius)
	// while still inside DefaultSpawnAggroRadius of the displaced actor.
	owner := peerVisibilityCharacter("ProxWalkHomeOwner", 0x01030501, 0x02040501, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	// Watcher stays outside default aggro of both displaced 1800 and home 1700
	// so proximity cannot re-engage after walk-away, while still remaining a
	// retained visibility viewer for homeward MOVE.
	watcher := peerVisibilityCharacter("ProxWalkHomeWatcher", 0x01030502, 0x02040502, 1450, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "prox-walk-home-owner", 0x60606061, owner)
	issuePeerTicket(t, store, "prox-walk-home-watcher", 0x60606062, watcher)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005200, 0)
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
		t.Fatalf("unexpected game runtime error for proximity walk-away chase/homeward: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	ref := "practice.proximity_walk_homeward_after_chase"
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           ref,
		Name:          "ProxWalkHomeMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import proximity walk-away chase/homeward spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef(ref)
	if !ok {
		t.Fatal("expected proximity walk-away chase/homeward spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "prox-walk-home-owner", 0x60606061)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("ProxWalkHomeOwner")
	if !ok {
		t.Fatal("expected proximity walk-away homeward owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected pending-frame proximity acquisition to engage owner for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("ProxWalkHomeOwner"); ok {
		t.Fatalf("expected proximity walk-away homeward fixture not to invent selected combat target ownership, got %+v", snapshot)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected proximity engagement to arm chase before displace, ok=%v snapshot=%+v", ok, pending)
	}

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "prox-walk-home-watcher", 0x60606062)
	defer closeSessionFlow(t, watcherFlow)
	flushServerFrames(t, watcherFlow)
	flushServerFrames(t, ownerFlow)

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before proximity chase displace, got %d frames", len(queued))
	}
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to displace actor toward owner before proximity walk-away")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before proximity walk-away: %v", err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward owner before proximity walk-away, got %+v", chaseMove)
	}
	_ = flushServerFrames(t, watcherFlow)

	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.X != 1800 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected chase-displaced within_radius actor before proximity walk-away, ok=%v snapshot=%+v", ok, displaced)
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected proximity engagement to survive chase displace for entity %d", group.EntityID)
	}

	// Leave DefaultSpawnAggroRadius of displaced 1800 (walk to 2050 => dist 250)
	// while staying inside visibility/leash so release is proximity-only.
	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    2050,
		Y:    2800,
		Time: 0x61626370,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while walking out of aggro after chase displace: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after proximity walk-away, got %d frames", len(moveOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected proximity walk-away release to stay silent without inventing TARGET(0,0), got %d queued frames", len(queued))
	}
	_ = flushServerFrames(t, watcherFlow)

	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected proximity walk-away to release engaged_by for entity %d", group.EntityID)
	}
	runtime.spawnChaseMu.Lock()
	_, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduled {
		t.Fatalf("expected proximity walk-away to clear pending chase deadline for entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase-step inspection to omit actor after proximity walk-away, ok=%v snapshot=%+v", ok, pending)
	}

	runtime.spawnHomewardMu.Lock()
	homewardDueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected proximity walk-away engagement release on within_radius displace to arm homeward deadline for entity %d", group.EntityID)
	}
	expectedHomewardDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !homewardDueAt.Equal(expectedHomewardDueAt) {
		t.Fatalf("expected proximity walk-away homeward deadline at %s, got %s", expectedHomewardDueAt, homewardDueAt)
	}

	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no homeward MOVE on watcher before the 1s deadline after proximity walk-away, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	homewardQueued := flushServerFrames(t, watcherFlow)
	if len(homewardQueued) == 0 {
		t.Fatal("expected due homeward-step after proximity walk-away to queue retained watcher MOVE toward home")
	}
	homewardMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, homewardQueued[0]))
	if err != nil {
		t.Fatalf("expected retained watcher after proximity walk-away to receive MOVE replication, first frame decode err=%v", err)
	}
	if homewardMove.VID != targetVID || homewardMove.X != 1700 || homewardMove.Y != 2800 {
		t.Fatalf("expected homeward MOVE to authored home after proximity walk-away, got %+v", homewardMove)
	}
	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected homeward-step after proximity walk-away to restore at_home leash state, ok=%v snapshot=%+v", ok, returned)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillHomeward := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillHomeward {
		t.Fatalf("expected completed at-home homeward-step after proximity walk-away to clear pending deadline for entity %d", group.EntityID)
	}
}
