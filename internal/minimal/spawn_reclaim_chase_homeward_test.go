package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// EnterGame reclaim after a chase-displaced within_radius practice mob must
// clear the pending chase deadline and arm homeward, matching slash leave /
// transfer / TARGET(0). Join reclaim snapshots engagements owned by reclaimable
// stale subjects before removeStaleOwnership clears engaged_by, then re-syncs
// chase prune + within_radius homeward before encoding visibility.
func TestGameRuntimeEnterGameReclaimClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewMemoryStore()

	// Stale owner at +200 so one due chase beat lands the mob at 1800 (within_radius).
	staleOwner := peerVisibilityCharacter("ReclaimHomewardOwner", 0x01030601, 0x02040601, 1900, 2800, 0, 101, 201)
	staleOwner.MapIndex = 42
	staleOwner.Points[bootstrapPlayerPointValueIndex] = 50
	// Replacement joins outside default aggro of displaced 1800 and home 1700 so
	// proximity cannot re-engage after reclaim. Same Name/VID so Join reclaim
	// drops the stale ownership by identity. Stay outside the 800 visibility
	// radius of the displaced mob so retained homeward MOVE is asserted on the
	// dedicated watcher (mirrors transfer-test watcher placement).
	replacement := peerVisibilityCharacter("ReclaimHomewardOwner", 0x01030601, 0x02040601, 900, 2800, 0, 101, 201)
	replacement.MapIndex = 42
	replacement.Points[bootstrapPlayerPointValueIndex] = 50
	// Dedicated watcher stays outside aggro but inside visibility of displaced
	// 1800 / home 1700 for retained homeward MOVE (same coords as transfer twin).
	watcher := peerVisibilityCharacter("ReclaimHomewardWatcher", 0x01030602, 0x02040602, 1450, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50

	issuePeerTicket(t, store, "reclaim-homeward-a", 0x70707071, staleOwner)
	issuePeerTicket(t, store, "reclaim-homeward-b", 0x70707072, replacement)
	issuePeerTicket(t, store, "reclaim-homeward-watcher", 0x70707073, watcher)
	if err := accounts.Save(accountstore.Account{Login: "reclaim-homeward-a", Empire: staleOwner.Empire, Characters: []loginticket.Character{staleOwner}}); err != nil {
		t.Fatalf("seed stale reclaim-homeward owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "reclaim-homeward-b", Empire: replacement.Empire, Characters: []loginticket.Character{replacement}}); err != nil {
		t.Fatalf("seed replacement reclaim-homeward owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "reclaim-homeward-watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}}); err != nil {
		t.Fatalf("seed reclaim-homeward watcher account: %v", err)
	}

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005300, 0)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     800,
			VisibilitySectorSize: 200,
		},
		store,
		accounts,
		staticActorStore,
		interactionStore,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error for reclaim chase/homeward: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.reclaim_homeward_after_chase",
		Name:          "ReclaimHomewardMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import reclaim chase/homeward spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.reclaim_homeward_after_chase")
	if !ok {
		t.Fatal("expected reclaim chase/homeward spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)
	factory := runtime.SessionFactory()

	staleFlow, _ := enterGameWithLoginTicket(t, factory, "reclaim-homeward-a", 0x70707071)
	defer closeSessionFlow(t, staleFlow)
	flushServerFrames(t, staleFlow)

	watcherFlow, _ := enterGameWithLoginTicket(t, factory, "reclaim-homeward-watcher", 0x70707073)
	defer closeSessionFlow(t, watcherFlow)
	flushServerFrames(t, watcherFlow)
	flushServerFrames(t, staleFlow)

	if _, err := staleFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected stale owner target error before reclaim chase displace: %v", err)
	}
	if _, err := staleFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before reclaim chase displace: %v", err)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm chase before reclaim, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, staleFlow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before reclaim chase displace, got %d frames", len(queued))
	}
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, staleFlow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to displace actor toward stale owner before reclaim")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before reclaim: %v", err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward stale owner before reclaim, got %+v", chaseMove)
	}
	_ = flushServerFrames(t, watcherFlow)

	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.X != 1800 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected chase-displaced within_radius actor before reclaim, ok=%v snapshot=%+v", ok, displaced)
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName(staleOwner.Name)
	if !ok {
		t.Fatal("expected live stale chase owner entity before simulated reclaim")
	}
	staleOwnerID := ownerEntity.Entity.ID
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, staleOwnerID) {
		t.Fatalf("expected practice mob to stay engaged by stale owner %d before reclaim", staleOwnerID)
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(staleOwnerID); !ok {
		t.Fatal("expected simulated reclaim to remove stale owner session hook")
	}

	replacementFlow, _ := enterGameWithLoginTicket(t, factory, "reclaim-homeward-b", 0x70707072)
	defer closeSessionFlow(t, replacementFlow)
	flushServerFrames(t, replacementFlow)
	_ = flushServerFrames(t, watcherFlow)

	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, staleOwnerID) {
		t.Fatalf("expected EnterGame reclaim to release stale engagement ownership for entity %d", group.EntityID)
	}
	runtime.spawnChaseMu.Lock()
	_, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduled {
		t.Fatalf("expected EnterGame reclaim to clear pending chase deadline for entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase-step inspection to omit actor after reclaim, ok=%v snapshot=%+v", ok, pending)
	}

	runtime.spawnHomewardMu.Lock()
	homewardDueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected EnterGame reclaim engagement release on within_radius displace to arm homeward deadline for entity %d", group.EntityID)
	}
	expectedHomewardDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !homewardDueAt.Equal(expectedHomewardDueAt) {
		t.Fatalf("expected reclaim homeward deadline at %s, got %s", expectedHomewardDueAt, homewardDueAt)
	}

	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no homeward MOVE on watcher before the 1s deadline after reclaim, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	homewardQueued := flushServerFrames(t, watcherFlow)
	if len(homewardQueued) == 0 {
		t.Fatal("expected due homeward-step after reclaim to queue retained watcher MOVE toward home")
	}
	homewardMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, homewardQueued[0]))
	if err != nil {
		t.Fatalf("expected retained watcher after reclaim to receive MOVE replication, first frame decode err=%v", err)
	}
	if homewardMove.VID != targetVID || homewardMove.X != 1700 || homewardMove.Y != 2800 {
		t.Fatalf("expected homeward MOVE to authored home after reclaim, got %+v", homewardMove)
	}
	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected homeward-step after reclaim to restore at_home leash state, ok=%v snapshot=%+v", ok, returned)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillHomeward := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillHomeward {
		t.Fatalf("expected completed at-home homeward-step after reclaim to clear pending deadline for entity %d", group.EntityID)
	}
}
