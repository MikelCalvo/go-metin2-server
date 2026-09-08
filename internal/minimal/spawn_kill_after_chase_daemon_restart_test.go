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

// Daemon restart after a chase-displace killing hit must rematerialize the
// still-dead corpse at the persisted death coords (combat_current_hp=0 +
// current X/Y + absolute respawn_ready_at), not snap to authored home or arm
// homeward/return/chase. Due respawn still rebuilds at authored home.
//
// Ordinary GREEN twin beside
// TestGameRuntimeContentSpawnGroupStillDeadPersistsAcrossDaemonRestart (at-home
// death) and
// TestGameRuntimeKillingHitAfterChaseDisplaceKeepsStillDeadAtDeathCoordsAndRespawnsAtAuthoredHome
// (same-process kill-after-chase).
func TestGameRuntimeKillingHitAfterChaseDisplaceStillDeadPersistsAcrossDaemonRestart(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Owner at +200 so one due chase beat lands the mob at 1800 (within_radius)
	// while remaining inside combat-target range / visibility for the remaining
	// killing hits.
	owner := peerVisibilityCharacter("StillDeadRestartChaseOwner", 0x01030b01, 0x02040b01, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	lateViewer := peerVisibilityCharacter("StillDeadRestartChaseLate", 0x01030b02, 0x02040b02, 2100, 2800, 0, 102, 202)
	lateViewer.MapIndex = 42
	lateViewer.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "still-dead-restart-chase-owner", 0xb1b1b1b1, owner)
	issuePeerTicket(t, store, "still-dead-restart-chase-late", 0xb1b1b1b2, lateViewer)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005800, 0)
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
		t.Fatalf("unexpected game runtime error before chase-displace still-dead restart: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	const spawnRef = "practice.kill_after_chase_still_dead_restart"
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           spawnRef,
		Name:          "StillDeadRestartChaseMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import chase-displace still-dead restart spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok {
		t.Fatal("expected chase-displace still-dead restart spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)
	originalEntityID := group.EntityID

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "still-dead-restart-chase-owner", 0xb1b1b1b1)
	flushServerFrames(t, ownerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before chase-displace still-dead restart: %v", err)
	}
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before chase-displace still-dead restart: %v", err)
	}
	if len(attackOut) == 0 {
		t.Fatal("expected accepted chase-arming hit before chase-displace still-dead restart")
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm chase before still-dead restart, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) == 0 {
		t.Fatal("expected delayed retaliation before chase-displace still-dead restart")
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to displace actor toward owner before killing hit")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before still-dead restart: %v", err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward owner before still-dead restart, got %+v", chaseMove)
	}
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
	closeSessionFlow(t, ownerFlow)

	beforeRestart, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok || !beforeRestart.Dead || beforeRestart.X != 1800 || beforeRestart.Y != 2800 {
		t.Fatalf("expected still-dead corpse at chase-displaced death coords before daemon restart, ok=%v snapshot=%+v", ok, beforeRestart)
	}
	respawnsBefore := runtime.StaticActorRespawns()
	if len(respawnsBefore) != 1 || respawnsBefore[0].EntityID != originalEntityID || respawnsBefore[0].Actor.X != 1800 || respawnsBefore[0].Actor.Y != 2800 {
		t.Fatalf("expected one pending respawn at death coords before daemon restart, got %+v", respawnsBefore)
	}
	wantReadyAt := respawnsBefore[0].ReadyAt.UTC()

	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted chase-displace still-dead snapshot: %v", err)
	}
	if len(persisted.StaticActors) != 1 {
		t.Fatalf("expected one persisted spawn-group actor after chase-displace death, got %+v", persisted.StaticActors)
	}
	if persisted.StaticActors[0].X != 1800 || persisted.StaticActors[0].Y != 2800 {
		t.Fatalf("expected persisted still-dead snapshot to keep death coords, got %+v", persisted.StaticActors[0])
	}
	if persisted.StaticActors[0].SpawnHome == nil || persisted.StaticActors[0].SpawnHome.X != 1700 || persisted.StaticActors[0].SpawnHome.Y != 2800 {
		t.Fatalf("expected persisted spawn home to stay authored, got %+v", persisted.StaticActors[0])
	}
	if persisted.StaticActors[0].CombatCurrentHP == nil || *persisted.StaticActors[0].CombatCurrentHP != 0 {
		t.Fatalf("expected persisted still-dead combat_current_hp=0, got %+v", persisted.StaticActors[0])
	}
	if persisted.StaticActors[0].RespawnReadyAt == nil || !persisted.StaticActors[0].RespawnReadyAt.Equal(wantReadyAt) {
		t.Fatalf("expected persisted respawn_ready_at=%s, got %+v", wantReadyAt, persisted.StaticActors[0])
	}

	reloaded, err := newGameRuntimeWithAccountStoreAndContentStores(
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
		t.Fatalf("reload runtime with persisted chase-displace still-dead spawn group: %v", err)
	}
	reloaded.now = func() time.Time { return currentTime }

	afterRestart, ok := reloaded.SpawnGroupByRef(spawnRef)
	if !ok {
		t.Fatal("expected still-dead spawn group to remain resolvable by authored ref after daemon restart")
	}
	if !afterRestart.Dead || afterRestart.X != 1800 || afterRestart.Y != 2800 {
		t.Fatalf("expected daemon restart to keep still-dead corpse at death coords, got snapshot=%+v", afterRestart)
	}
	respawnsAfter := reloaded.StaticActorRespawns()
	if len(respawnsAfter) != 1 || respawnsAfter[0].EntityID != afterRestart.EntityID || !respawnsAfter[0].ReadyAt.Equal(wantReadyAt) || respawnsAfter[0].Actor.X != 1800 || respawnsAfter[0].Actor.Y != 2800 {
		t.Fatalf("expected pending respawn deadline and death coords to survive daemon restart, got %+v want ready_at=%s", respawnsAfter, wantReadyAt)
	}
	if pending, ok := reloaded.SpawnGroupChaseStep(afterRestart.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase to stay unarmed across still-dead daemon restart, ok=%v snapshot=%+v", ok, pending)
	}
	if pending, ok := reloaded.SpawnGroupHomewardStep(afterRestart.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected still-dead restore not to arm homeward, ok=%v snapshot=%+v", ok, pending)
	}
	if pending, ok := reloaded.SpawnGroupReturnStep(afterRestart.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected still-dead restore not to arm return-step, ok=%v snapshot=%+v", ok, pending)
	}
	reloaded.spawnHomewardMu.Lock()
	_, homewardScheduled := reloaded.spawnHomewardStepDueAt[afterRestart.EntityID]
	reloaded.spawnHomewardMu.Unlock()
	if homewardScheduled {
		t.Fatalf("expected still-dead restore not to schedule homeward for entity %d", afterRestart.EntityID)
	}
	reloaded.spawnReturnMu.Lock()
	_, returnScheduled := reloaded.spawnReturnStepDueAt[afterRestart.EntityID]
	reloaded.spawnReturnMu.Unlock()
	if returnScheduled {
		t.Fatalf("expected still-dead restore not to schedule return-step for entity %d", afterRestart.EntityID)
	}
	if targets := reloaded.CombatTargetSnapshots(); len(targets) != 0 {
		t.Fatalf("expected engagement/selected-target ownership to stay fail-closed across daemon restart, got %+v", targets)
	}

	lateFlow, lateEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), "still-dead-restart-chase-late", 0xb1b1b1b2)
	defer closeSessionFlow(t, lateFlow)
	assertStillDeadBootstrapAt(t, lateEnter, uint32(afterRestart.EntityID), 1800, 2800, "StillDeadRestartChaseMob")
	deniedTarget, err := lateFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(afterRestart.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected still-dead restart target error: %v", err)
	}
	if len(deniedTarget) != 0 {
		t.Fatalf("expected still-dead restarted spawn group to stay non-targetable, got %d frames", len(deniedTarget))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	assertNoActorMoveFrames(t, flushServerFrames(t, lateFlow), uint32(afterRestart.EntityID), "after homeward delay during rematerialized still-dead interval")
	stillDead, ok := reloaded.SpawnGroupByRef(spawnRef)
	if !ok || !stillDead.Dead || stillDead.X != 1800 || stillDead.Y != 2800 {
		t.Fatalf("expected rematerialized still-dead actor to stay at death coords through homeward delay, ok=%v snapshot=%+v", ok, stillDead)
	}

	currentTime = wantReadyAt.Add(25 * time.Millisecond)
	respawnFrames := flushServerFrames(t, lateFlow)
	if len(respawnFrames) != 4 {
		t.Fatalf("expected delete + add/info/update respawn rebuild at authored home after still-dead restart, got %d frames", len(respawnFrames))
	}
	deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, respawnFrames[0]))
	if err != nil {
		t.Fatalf("decode still-dead restart respawn delete frame: %v", err)
	}
	if deleted.VID != uint32(afterRestart.EntityID) {
		t.Fatalf("unexpected still-dead restart respawn delete frame: %+v", deleted)
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, respawnFrames[1]))
	if err != nil {
		t.Fatalf("decode still-dead restart respawn add frame: %v", err)
	}
	if added.VID != uint32(afterRestart.EntityID) || added.X != 1700 || added.Y != 2800 || added.RaceNum != 20350 {
		t.Fatalf("expected authored-home respawn add after still-dead restart, got %+v", added)
	}

	respawned, ok := reloaded.SpawnGroupByRef(spawnRef)
	if !ok || respawned.Dead || respawned.X != 1700 || respawned.Y != 2800 || respawned.CombatHPPercent != 100 || respawned.SpawnLeash == nil || respawned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected due respawn after still-dead restart to restore live authored home, ok=%v snapshot=%+v", ok, respawned)
	}
	if respawns := reloaded.StaticActorRespawns(); len(respawns) != 0 {
		t.Fatalf("expected no pending respawn after authored-home rebuild, got %+v", respawns)
	}
	cleared, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted snapshot after due respawn: %v", err)
	}
	if len(cleared.StaticActors) != 1 || cleared.StaticActors[0].X != 1700 || cleared.StaticActors[0].Y != 2800 || cleared.StaticActors[0].CombatCurrentHP != nil || cleared.StaticActors[0].RespawnReadyAt != nil {
		t.Fatalf("expected due respawn to restore authored home and clear still-dead persistence fields, got %+v", cleared.StaticActors)
	}

	// Late viewer sits at 2100, 400 from authored home, outside combat-target
	// range (300). Owner at 1900 can freshly reselect the rebuilt live actor.
	ownerRestartFlow, ownerRestartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), "still-dead-restart-chase-owner", 0xb1b1b1b1)
	defer closeSessionFlow(t, ownerRestartFlow)
	var seenLiveAdd bool
	for _, raw := range ownerRestartEnter {
		add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw))
		if err != nil || add.VID != uint32(respawned.EntityID) {
			continue
		}
		if add.X != 1700 || add.Y != 2800 {
			t.Fatalf("expected post-respawn owner bootstrap add at authored home, got %+v", add)
		}
		seenLiveAdd = true
		break
	}
	if !seenLiveAdd {
		t.Fatalf("expected post-respawn owner bootstrap to show live authored-home add among %d frames", len(ownerRestartEnter))
	}
	reselectOut, err := ownerRestartFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(respawned.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected post-respawn reselect error after still-dead restart: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after authored-home respawn reselection, got %d frames", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode post-respawn target frame after still-dead restart: %v", err)
	}
	if reselected.TargetVID != uint32(respawned.EntityID) || reselected.HPPercent != 100 {
		t.Fatalf("unexpected post-respawn target packet after still-dead restart: %+v", reselected)
	}
}
