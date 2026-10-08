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

// Operator POST homeward-step companion: one within_radius step toward authored
// home, same-map retained MOVE, and a refreshed automatic deadline when the
// actor is still eligible. Dead, return_required, and engaged actors stay
// fail-closed. Automatic pending-frame homeward stays the already-owned path.
func TestGameRuntimeStepSpawnGroupHomewardMovesOnePlannedStepAndReschedules(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	viewer := peerVisibilityCharacter("HomewardOpViewer", 0x01030441, 0x02040441, 1900, 2800, 0, 101, 201)
	viewer.MapIndex = 42
	issuePeerTicket(t, store, "homeward-op-viewer", 0x41414141, viewer)
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	currentTime := time.Unix(1700005100, 0)

	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     400,
			VisibilitySectorSize: 200,
		},
		store,
		nil,
		staticActorStore,
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
	)
	if err != nil {
		t.Fatalf("new game runtime for operator homeward-step: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.homeward_operator_step",
		Name:          "HomewardOperatorStepMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import operator homeward-step spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.homeward_operator_step")
	if !ok {
		t.Fatal("expected operator homeward-step spawn group to resolve by ref")
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "homeward-op-viewer", 0x41414141)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)
	if _, ok := runtime.UpdateStaticActor(group.EntityID, "HomewardOperatorStepMob", 42, 1900, 2800, 20350); !ok {
		t.Fatal("expected within_radius displace before operator homeward-step")
	}
	flushServerFrames(t, flow)

	runtime.spawnHomewardMu.Lock()
	originalDueAt, scheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !scheduled {
		t.Fatalf("expected within_radius displace to arm automatic homeward for entity %d", group.EntityID)
	}

	currentTime = currentTime.Add(250 * time.Millisecond)
	stepped, ok := runtime.StepSpawnGroupHomeward(group.EntityID, 100)
	if !ok {
		t.Fatalf("expected operator homeward-step to accept entity %d", group.EntityID)
	}
	if stepped.Step.Current.X != 1900 || stepped.Step.Next.X != 1800 || stepped.Step.Complete {
		t.Fatalf("expected one 100-unit homeward step from 1900 to 1800, got %+v", stepped.Step)
	}
	if stepped.Actor.X != 1800 || stepped.Actor.Y != 2800 || stepped.Actor.SpawnGroupRef != "practice.homeward_operator_step" {
		t.Fatalf("expected stepped actor at planned next position, got %+v", stepped.Actor)
	}
	if stepped.Actor.SpawnLeash == nil || stepped.Actor.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius || stepped.Actor.SpawnLeash.ReturnRequired {
		t.Fatalf("expected operator homeward step to stay within_radius, got %+v", stepped.Actor.SpawnLeash)
	}
	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load static actor snapshot after operator homeward-step: %v", err)
	}
	if len(persisted.StaticActors) != 1 || persisted.StaticActors[0].X != 1800 || persisted.StaticActors[0].SpawnHome == nil || persisted.StaticActors[0].SpawnHome.X != 1700 {
		t.Fatalf("expected persisted spawn group to move one homeward step and preserve home, got %+v", persisted.StaticActors)
	}

	expectedDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	runtime.spawnHomewardMu.Lock()
	rescheduledDueAt, stillScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !stillScheduled || !rescheduledDueAt.Equal(expectedDueAt) {
		t.Fatalf("expected operator homeward-step to refresh automatic due time to %s, scheduled=%v due=%s original=%s", expectedDueAt, stillScheduled, rescheduledDueAt, originalDueAt)
	}
	if pending, ok := runtime.SpawnGroupReturnStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected operator homeward-step not to arm return-step, ok=%v snapshot=%+v", ok, pending)
	}

	queued := flushServerFrames(t, flow)
	if len(queued) == 0 {
		t.Fatal("expected retained viewer to receive operator homeward-step MOVE")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode operator homeward-step MOVE: %v", err)
	}
	if moveAck.VID != uint32(group.EntityID) || moveAck.X != 1800 || moveAck.Y != 2800 || moveAck.Duration == 0 {
		t.Fatalf("expected operator homeward MOVE at 1800,2800 with duration, got %+v", moveAck)
	}

	currentTime = originalDueAt.Add(time.Nanosecond)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected old pre-manual homeward deadline not to fire, got %d frames", len(queued))
	}
	if current, ok := runtime.SpawnGroup(group.EntityID); !ok || current.X != 1800 {
		t.Fatalf("expected actor to stay at operator step before refreshed deadline, ok=%v snapshot=%+v", ok, current)
	}
	landed, ok := runtime.StepSpawnGroupHomeward(group.EntityID, 100)
	if !ok || landed.Actor.X != 1700 || !landed.Step.Complete || landed.Actor.SpawnLeash == nil || landed.Actor.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected next operator step to land at authored home, ok=%v snapshot=%+v", ok, landed)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillScheduled = runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillScheduled {
		t.Fatal("expected arrival at home to clear automatic homeward deadline")
	}
	if stepped, ok := runtime.StepSpawnGroupHomeward(group.EntityID, 100); ok || stepped.Actor.EntityID != 0 {
		t.Fatalf("expected an actor already at home to reject a redundant operator step, ok=%v snapshot=%+v", ok, stepped)
	}
}

func TestGameRuntimeStepSpawnGroupHomewardFailsClosedForReturnRequiredDeadAndEngaged(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("HomewardOpFailOwner", 0x01030442, 0x02040442, 1700, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "homeward-op-fail-owner", 0x42424242, owner)
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")

	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		store,
		nil,
		staticActorStore,
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
	)
	if err != nil {
		t.Fatalf("new game runtime for operator homeward fail-closed: %v", err)
	}
	currentTime := time.Unix(1700005200, 0)
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.homeward_operator_fail",
		Name:          "HomewardOperatorFailMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import operator homeward fail-closed spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.homeward_operator_fail")
	if !ok {
		t.Fatal("expected operator homeward fail-closed spawn group to resolve by ref")
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "homeward-op-fail-owner", 0x42424242)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	if _, ok := runtime.UpdateStaticActor(group.EntityID, "HomewardOperatorFailMob", 42, 2301, 2800, 20350); !ok {
		t.Fatal("expected return_required displace before operator homeward fail-closed proof")
	}
	flushServerFrames(t, flow)
	if stepped, ok := runtime.StepSpawnGroupHomeward(group.EntityID, 100); ok || stepped.Actor.EntityID != 0 {
		t.Fatalf("expected return_required operator homeward-step to fail closed, ok=%v snapshot=%+v", ok, stepped)
	}
	if current, ok := runtime.SpawnGroup(group.EntityID); !ok || current.X != 2301 {
		t.Fatalf("expected return_required actor to stay put after rejected homeward-step, ok=%v snapshot=%+v", ok, current)
	}

	if _, ok := runtime.UpdateStaticActor(group.EntityID, "HomewardOperatorFailMob", 42, 1800, 2800, 20350); !ok {
		t.Fatal("expected within_radius displace before engaged homeward fail-closed proof")
	}
	flushServerFrames(t, flow)
	targetVID := uint32(group.EntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil || len(selectOut) != 1 {
		t.Fatalf("expected target selection before engaged homeward reject, err=%v frames=%d", err, len(selectOut))
	}
	drainAcceptedTargetCreateNewIfQueued(t, flow)
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil || len(attackOut) == 0 {
		t.Fatalf("expected accepted hit before engaged homeward reject, err=%v frames=%d", err, len(attackOut))
	}
	if stepped, ok := runtime.StepSpawnGroupHomeward(group.EntityID, 100); ok || stepped.Actor.EntityID != 0 {
		t.Fatalf("expected engaged operator homeward-step to fail closed, ok=%v snapshot=%+v", ok, stepped)
	}

	for hit := 2; hit <= int(worldruntime.PracticeMobBootstrapMaxHP); hit++ {
		currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		attackOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil || len(attackOut) == 0 {
			t.Fatalf("expected killing hit %d before dead homeward reject, err=%v frames=%d", hit, err, len(attackOut))
		}
	}
	if dead, ok := runtime.SpawnGroup(group.EntityID); !ok || !dead.Dead {
		t.Fatalf("expected killed spawn group before dead homeward reject, ok=%v snapshot=%+v", ok, dead)
	}
	if stepped, ok := runtime.StepSpawnGroupHomeward(group.EntityID, 100); ok || stepped.Actor.EntityID != 0 {
		t.Fatalf("expected dead operator homeward-step to fail closed, ok=%v snapshot=%+v", ok, stepped)
	}
}
