package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Daemon restart must keep the frozen re-arm-from-now posture: eligible
// return/homeward re-arm from now, chase stays unarmed until fresh post-restart
// engagement, and engagement / selected-target ownership stay fail-closed.
//
// Ordinary GREEN composite twin beside the already-owned restore-arm proofs
// (TestGameRuntimeRestoreReturnRequiredSpawnGroupSchedulesReturnStep and
// TestGameRuntimeLoadPersistedStaticActorsArmsHomewardForUnengagedWithinRadiusSpawn).
// Absolute mid-timer due-at rematerialize RED remains cancelled.
func TestGameRuntimeDaemonRestartRearmsReturnAndHomewardFromNowAndLeavesChaseUnarmed(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ChaseRestartOwner", 0x01030351, 0x02040351, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "chase-restart-schedule-owner", 0xa1a1a1a1, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700004600, 0)
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
		t.Fatalf("unexpected game runtime error before daemon-restart schedule twin: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{
		{
			Ref:           "practice.restart_chase_unarmed_within",
			Name:          "RestartChaseWithinMob",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
		},
		{
			Ref:           "practice.restart_return_required_restore",
			Name:          "RestartReturnRequiredMob",
			MapIndex:      42,
			X:             1700,
			Y:             3000,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
		},
	}}); err != nil {
		t.Fatalf("import daemon-restart schedule twin spawn-group bundle: %v", err)
	}
	withinGroup, ok := runtime.SpawnGroupByRef("practice.restart_chase_unarmed_within")
	if !ok {
		t.Fatal("expected within_radius chase-restart spawn group to resolve by ref")
	}
	returnGroup, ok := runtime.SpawnGroupByRef("practice.restart_return_required_restore")
	if !ok {
		t.Fatal("expected return_required restore spawn group to resolve by ref")
	}
	withinVID := uint32(withinGroup.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "chase-restart-schedule-owner", 0xa1a1a1a1)
	flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: withinVID}))); err != nil {
		t.Fatalf("unexpected owner target error before chase-armed restart setup: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  withinVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before chase-armed restart setup: %v", err)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(withinGroup.EntityID); !ok || pending.EntityID != withinGroup.EntityID {
		t.Fatalf("expected engaged hit to arm chase before daemon restart, ok=%v snapshot=%+v", ok, pending)
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("ChaseRestartOwner")
	if !ok {
		t.Fatal("expected chase-restart owner entity to remain registered before restart")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(withinGroup.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected engagement before daemon restart for entity %d", withinGroup.EntityID)
	}

	// One due chase beat leaves the actor within_radius while chase re-arms.
	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay)
	dueQueued := flushServerFrames(t, flow)
	if len(dueQueued) == 0 {
		t.Fatal("expected due chase-step to displace within_radius actor before daemon restart")
	}
	displaced, ok := runtime.SpawnGroup(withinGroup.EntityID)
	if !ok || displaced.X != 1800 || displaced.Y != 2800 || displaced.Dead || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius || displaced.SpawnLeash.ReturnRequired {
		t.Fatalf("expected due chase to leave unarmed-restart actor within_radius at 1800,2800, ok=%v snapshot=%+v", ok, displaced)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(withinGroup.EntityID); !ok || pending.EntityID != withinGroup.EntityID {
		t.Fatalf("expected incomplete chase displace to keep chase armed before restart, ok=%v snapshot=%+v", ok, pending)
	}
	if pending, ok := runtime.SpawnGroupHomewardStep(withinGroup.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected still-engaged within_radius chase actor not to arm homeward before restart, ok=%v snapshot=%+v", ok, pending)
	}
	closeSessionFlow(t, flow)

	// Displace the return actor only after the chase flush so the pending 1s
	// return-step deadline cannot fire during the 5s chase wait. Do not flush
	// again before reload: the snapshot must still show the displaced coords.
	updatedReturn, ok := runtime.UpdateStaticActor(returnGroup.EntityID, "RestartReturnRequiredMob", 42, 2301, 3000, 20350)
	if !ok {
		t.Fatal("expected UpdateStaticActor to displace return_required actor before restart")
	}
	if updatedReturn.SpawnLeash == nil || !updatedReturn.SpawnLeash.ReturnRequired {
		t.Fatalf("expected displaced return actor to classify return_required before restart, got %+v", updatedReturn)
	}
	if pending, ok := runtime.SpawnGroupReturnStep(returnGroup.EntityID); !ok || pending.EntityID != returnGroup.EntityID {
		t.Fatalf("expected return_required displace to arm return-step before restart, ok=%v snapshot=%+v", ok, pending)
	}

	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted schedule twin snapshot before restart: %v", err)
	}
	if len(persisted.StaticActors) != 2 {
		t.Fatalf("expected two persisted spawn-group actors before restart, got %+v", persisted.StaticActors)
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
		t.Fatalf("reload runtime for daemon-restart schedule twin: %v", err)
	}
	reloaded.now = func() time.Time { return currentTime }

	restoredWithin, ok := reloaded.SpawnGroupByRef("practice.restart_chase_unarmed_within")
	if !ok {
		t.Fatal("expected within_radius spawn group to remain resolvable after daemon restart")
	}
	if restoredWithin.X != 1800 || restoredWithin.Y != 2800 || restoredWithin.Dead || restoredWithin.SpawnLeash == nil || restoredWithin.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius || restoredWithin.SpawnLeash.ReturnRequired {
		t.Fatalf("expected rematerialized chase-displaced actor to stay live within_radius, got %+v", restoredWithin)
	}
	if pending, ok := reloaded.SpawnGroupHomewardStep(restoredWithin.EntityID); !ok || pending.EntityID != restoredWithin.EntityID {
		t.Fatalf("expected loadPersistedStaticActors to re-arm homeward from now for restored within_radius entity %d, ok=%v snapshot=%+v", restoredWithin.EntityID, ok, pending)
	}
	if pending, ok := reloaded.SpawnGroupChaseStep(restoredWithin.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase to stay unarmed across daemon restart for within_radius entity %d, ok=%v snapshot=%+v", restoredWithin.EntityID, ok, pending)
	}
	if pending, ok := reloaded.SpawnGroupReturnStep(restoredWithin.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected within_radius restore not to arm return-step, ok=%v snapshot=%+v", ok, pending)
	}

	restoredReturn, ok := reloaded.SpawnGroupByRef("practice.restart_return_required_restore")
	if !ok {
		t.Fatal("expected return_required spawn group to remain resolvable after daemon restart")
	}
	if restoredReturn.X != 2301 || restoredReturn.Y != 3000 || restoredReturn.SpawnLeash == nil || !restoredReturn.SpawnLeash.ReturnRequired {
		t.Fatalf("expected rematerialized return actor to stay return_required, got %+v", restoredReturn)
	}
	if pending, ok := reloaded.SpawnGroupReturnStep(restoredReturn.EntityID); !ok || pending.EntityID != restoredReturn.EntityID {
		t.Fatalf("expected loadPersistedStaticActors to re-arm return-step from now for restored return_required entity %d, ok=%v snapshot=%+v", restoredReturn.EntityID, ok, pending)
	}
	if pending, ok := reloaded.SpawnGroupChaseStep(restoredReturn.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase to stay unarmed across daemon restart for return_required entity %d, ok=%v snapshot=%+v", restoredReturn.EntityID, ok, pending)
	}
	if pending, ok := reloaded.SpawnGroupHomewardStep(restoredReturn.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected return_required restore not to arm homeward, ok=%v snapshot=%+v", ok, pending)
	}

	if pending := reloaded.SpawnGroupChaseSteps(); len(pending) != 0 {
		t.Fatalf("expected no pending chase rows after daemon restart, got %+v", pending)
	}
	if targets := reloaded.CombatTargetSnapshots(); len(targets) != 0 {
		t.Fatalf("expected engagement/selected-target ownership to stay fail-closed across daemon restart, got %+v", targets)
	}
}
