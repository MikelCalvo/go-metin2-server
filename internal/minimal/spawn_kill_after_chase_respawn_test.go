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
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// A live killing hit after chase has already displaced a practice mob
// within_radius must not piggyback homeward recovery. The corpse stays at the
// death coords through the still-dead interval (no chase/homeward MOVE, late
// EnterGame add/info/update + trailing DEAD at those coords), and due respawn
// rebuilds at authored home with fresh reselect. Contrast with the synthetic
// timer cleanup in
// TestGameRuntimeRespawnClearsStaleSpawnGroupChaseAndHomewardStepSchedules and
// the at-home session flow in
// TestGameSessionFlowContentSpawnGroupPracticeMobRespawnsAfterServerDrivenDelayAndRequiresFreshReselect.
func TestGameRuntimeKillingHitAfterChaseDisplaceKeepsStillDeadAtDeathCoordsAndRespawnsAtAuthoredHome(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Owner at +200 so one due chase beat lands the mob at 1800 (within_radius)
	// while remaining inside combat-target range / visibility for the remaining
	// killing hits.
	owner := peerVisibilityCharacter("KillAfterChaseOwner", 0x01030a01, 0x02040a01, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	// Watcher stays outside default aggro of displaced 1800 and home 1700 so
	// proximity cannot re-engage during the dead interval or after authored-home
	// respawn, while remaining a retained visibility viewer for both positions.
	watcher := peerVisibilityCharacter("KillAfterChaseWatcher", 0x01030a02, 0x02040a02, 1450, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	lateViewer := peerVisibilityCharacter("KillAfterChaseLateViewer", 0x01030a03, 0x02040a03, 2100, 2800, 0, 102, 202)
	lateViewer.MapIndex = 42
	lateViewer.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "kill-after-chase-owner", 0xa1a1a1a1, owner)
	issuePeerTicket(t, store, "kill-after-chase-watcher", 0xa1a1a1a2, watcher)
	issuePeerTicket(t, store, "kill-after-chase-late-viewer", 0xa1a1a1a3, lateViewer)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005700, 0)
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
		t.Fatalf("unexpected game runtime error for kill-after-chase respawn: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	const spawnRef = "practice.kill_after_chase_respawn_home"
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           spawnRef,
		Name:          "KillAfterChaseMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import kill-after-chase respawn spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok {
		t.Fatal("expected kill-after-chase respawn spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-after-chase-owner", 0xa1a1a1a1)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-after-chase-watcher", 0xa1a1a1a2)
	defer closeSessionFlow(t, watcherFlow)
	flushServerFrames(t, watcherFlow)
	flushServerFrames(t, ownerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before kill-after-chase displace: %v", err)
	}
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before kill-after-chase displace: %v", err)
	}
	if len(attackOut) == 0 {
		t.Fatal("expected accepted chase-arming hit before kill-after-chase displace")
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm chase before kill-after-chase displace, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) == 0 {
		t.Fatal("expected delayed retaliation before kill-after-chase displace")
	}
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to displace actor toward owner before killing hit")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before killing hit: %v", err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward owner before killing hit, got %+v", chaseMove)
	}
	_ = flushServerFrames(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)

	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.Dead || displaced.X != 1800 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected live chase-displaced within_radius actor before killing hit, ok=%v snapshot=%+v", ok, displaced)
	}

	remainingHits := int(worldruntime.PracticeMobBootstrapMaxHP) - 1
	if remainingHits <= 0 {
		t.Fatal("expected practice-mob bootstrap HP to leave remaining hits after the chase-arming strike")
	}
	var killingAttack [][]byte
	for hit := 0; hit < remainingHits; hit++ {
		if hit > 0 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected post-chase owner attack %d: %v", hit+1, err)
		}
		if hit == remainingHits-1 {
			killingAttack = attackOut
			break
		}
		if len(attackOut) == 0 {
			t.Fatalf("expected accepted non-killing post-chase hit %d to emit frames", hit+1)
		}
		_ = flushServerFrames(t, watcherFlow)
	}
	if len(killingAttack) != 3 {
		t.Fatalf("expected killing hit after chase displace to emit DEAD, TARGET(0), and damage-info, got %d frames", len(killingAttack))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, killingAttack[0]))
	if err != nil {
		t.Fatalf("decode mob-death frame after chase displace: %v", err)
	}
	if dead.VID != targetVID {
		t.Fatalf("expected killing hit after chase displace to kill target vid %d, got %+v", targetVID, dead)
	}
	cleared, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, killingAttack[1]))
	if err != nil {
		t.Fatalf("decode TARGET clear on killing hit after chase displace: %v", err)
	}
	if cleared.TargetVID != 0 || cleared.HPPercent != 0 {
		t.Fatalf("expected zero-target clear on killing hit after chase displace, got %+v", cleared)
	}
	assertDamageInfoFrame(t, killingAttack[2], targetVID, int32(worldruntime.PracticeMobBootstrapDamagePerNormalAttack), "kill-after-chase killing hit")
	_ = flushServerFrames(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)

	corpse, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || !corpse.Dead || corpse.X != 1800 || corpse.Y != 2800 {
		t.Fatalf("expected still-dead corpse to stay at chase-displaced death coords, ok=%v snapshot=%+v", ok, corpse)
	}
	respawns := runtime.StaticActorRespawns()
	if len(respawns) != 1 || respawns[0].EntityID != group.EntityID || respawns[0].Actor.X != 1800 || respawns[0].Actor.Y != 2800 {
		t.Fatalf("expected one pending respawn at death coords after chase-displace kill, got %+v", respawns)
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("KillAfterChaseOwner")
	if !ok {
		t.Fatal("expected kill-after-chase owner entity to remain registered")
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected killing hit to release engagement for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("KillAfterChaseOwner"); ok {
		t.Fatalf("expected killing hit to clear selected combat target, got %+v", snapshot)
	}
	runtime.spawnChaseMu.Lock()
	_, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduled {
		t.Fatalf("expected killing hit after chase displace to clear pending chase deadline for entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase-step inspection to omit still-dead actor, ok=%v snapshot=%+v", ok, pending)
	}
	runtime.spawnHomewardMu.Lock()
	_, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if homewardScheduled {
		t.Fatalf("expected killing hit after chase displace not to arm homeward for dead entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupHomewardStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected homeward-step inspection to omit still-dead actor, ok=%v snapshot=%+v", ok, pending)
	}

	assertNoActorMoveFrames(t, flushServerFrames(t, watcherFlow), targetVID, "immediately after chase-displace kill")

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	assertNoActorMoveFrames(t, flushServerFrames(t, watcherFlow), targetVID, "after homeward delay during still-dead interval")
	assertNoActorMoveFrames(t, flushServerFrames(t, ownerFlow), targetVID, "owner after homeward delay during still-dead interval")
	stillDead, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || !stillDead.Dead || stillDead.X != 1800 || stillDead.Y != 2800 {
		t.Fatalf("expected still-dead actor to stay at death coords through homeward delay, ok=%v snapshot=%+v", ok, stillDead)
	}

	lateFlow, lateEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "kill-after-chase-late-viewer", 0xa1a1a1a3)
	defer closeSessionFlow(t, lateFlow)
	assertStillDeadBootstrapAt(t, lateEnter, targetVID, 1800, 2800, "KillAfterChaseMob")
	deniedTarget, err := lateFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected still-dead displaced target error: %v", err)
	}
	if len(deniedTarget) != 0 {
		t.Fatalf("expected still-dead displaced spawn group to stay non-targetable, got %d frames", len(deniedTarget))
	}
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay - bootstrapSpawnGroupHomewardStepDelay)
	respawnFrames := flushServerFrames(t, ownerFlow)
	if len(respawnFrames) != 4 {
		t.Fatalf("expected delete + add/info/update respawn rebuild at authored home after chase-displace kill, got %d frames", len(respawnFrames))
	}
	deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, respawnFrames[0]))
	if err != nil {
		t.Fatalf("decode chase-displace kill respawn delete frame: %v", err)
	}
	if deleted.VID != targetVID {
		t.Fatalf("unexpected chase-displace kill respawn delete frame: %+v", deleted)
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, respawnFrames[1]))
	if err != nil {
		t.Fatalf("decode chase-displace kill respawn add frame: %v", err)
	}
	if added.VID != targetVID || added.X != 1700 || added.Y != 2800 || added.RaceNum != 20350 {
		t.Fatalf("expected authored-home respawn add after chase-displace kill, got %+v", added)
	}
	_ = flushServerFrames(t, watcherFlow)
	_ = flushServerFrames(t, lateFlow)

	respawned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || respawned.Dead || respawned.X != 1700 || respawned.Y != 2800 || respawned.CombatHPPercent != 100 || respawned.SpawnLeash == nil || respawned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected due respawn after chase-displace kill to restore live authored home, ok=%v snapshot=%+v", ok, respawned)
	}
	if respawns := runtime.StaticActorRespawns(); len(respawns) != 0 {
		t.Fatalf("expected no pending respawn after authored-home rebuild, got %+v", respawns)
	}
	runtime.spawnChaseMu.Lock()
	_, chaseAfterRespawn := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseAfterRespawn {
		t.Fatalf("expected authored-home respawn to leave chase unarmed for entity %d", group.EntityID)
	}
	runtime.spawnHomewardMu.Lock()
	_, homewardAfterRespawn := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if homewardAfterRespawn {
		t.Fatalf("expected authored-home respawn to leave homeward unarmed for entity %d", group.EntityID)
	}

	attackWithoutReselect, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-respawn attack-without-reselect error after chase-displace kill: %v", err)
	}
	if len(attackWithoutReselect) != 0 {
		t.Fatalf("expected post-respawn attack without fresh target selection to fail closed, got %d frames", len(attackWithoutReselect))
	}
	reselectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected post-respawn reselect error after chase-displace kill: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after authored-home respawn reselection, got %d frames", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode post-respawn target frame after chase-displace kill: %v", err)
	}
	if reselected.TargetVID != targetVID || reselected.HPPercent != 100 {
		t.Fatalf("unexpected post-respawn target packet after chase-displace kill: %+v", reselected)
	}
}

func assertNoActorMoveFrames(t *testing.T, queued [][]byte, targetVID uint32, context string) {
	t.Helper()
	for _, raw := range queued {
		if moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, raw)); err == nil && moveAck.VID == targetVID {
			t.Fatalf("expected no chase/homeward MOVE %s, got %+v among %d frames", context, moveAck, len(queued))
		}
	}
}

func assertStillDeadBootstrapAt(t *testing.T, frames [][]byte, targetVID uint32, x int32, y int32, name string) {
	t.Helper()
	var (
		seenAdd  bool
		seenInfo bool
		seenDead bool
		add      worldproto.CharacterAddPacket
	)
	for _, raw := range frames {
		decoded := decodeSingleFrame(t, raw)
		if packet, err := worldproto.DecodeCharacterAdd(decoded); err == nil && packet.VID == targetVID {
			seenAdd = true
			add = packet
			continue
		}
		if packet, err := worldproto.DecodeCharacterAdditionalInfo(decoded); err == nil && packet.VID == targetVID {
			seenInfo = true
			if packet.Name != name {
				t.Fatalf("unexpected still-dead bootstrap additional info name %q, want %q", packet.Name, name)
			}
			continue
		}
		if packet, err := worldproto.DecodeDead(decoded); err == nil && packet.VID == targetVID {
			seenDead = true
		}
	}
	if !seenAdd || add.X != x || add.Y != y {
		t.Fatalf("expected still-dead bootstrap add at (%d,%d) for vid %d, seenAdd=%v add=%+v among %d frames", x, y, targetVID, seenAdd, add, len(frames))
	}
	if !seenInfo {
		t.Fatalf("expected still-dead bootstrap additional info for vid %d", targetVID)
	}
	if !seenDead {
		t.Fatalf("expected trailing GC DEAD replay for still-dead vid %d at death coords (%d,%d)", targetVID, x, y)
	}
}
