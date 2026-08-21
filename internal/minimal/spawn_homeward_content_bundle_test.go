package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameRuntimeFailedContentBundleImportRestoresSpawnGroupHomewardStepSchedule(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	viewer := peerVisibilityCharacter("SpawnHomewardRollback", 0x01035551, 4, 1850, 2800, 0, 101, 201)
	viewer.MapIndex = 42
	issuePeerTicket(t, store, "spawn-homeward-rollback", 0x55555151, viewer)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	currentTime := time.Unix(1700001060, 0)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", VisibilityMode: "radius", VisibilityRadius: 500, VisibilitySectorSize: 256},
		store,
		nil,
		staticActorStore,
		interactionStore,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "spawn-homeward-rollback", 0x55555151)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 5 {
		t.Fatalf("expected base enter-game burst before spawn import, got %d frames", len(enterOut))
	}
	flushServerFrames(t, flow)

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.homeward_step_rollback_original",
		Name:          "HomewardStepRollbackOriginalMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import original homeward-step rollback spawn bundle: %v", err)
	}
	flushServerFrames(t, flow)
	group, ok := runtime.SpawnGroupByRef("practice.homeward_step_rollback_original")
	if !ok {
		t.Fatal("expected original homeward-step rollback spawn group to resolve by ref")
	}
	if _, ok := runtime.UpdateStaticActor(group.EntityID, "HomewardStepRollbackOriginalMob", 42, 1800, 2800, 20350); !ok {
		t.Fatal("expected spawn-backed actor update to within_radius position to succeed")
	}
	flushServerFrames(t, flow)

	runtime.spawnHomewardMu.Lock()
	originalDueAt, scheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !scheduled {
		t.Fatalf("expected within_radius actor %d to have a pending automatic homeward-step schedule before failed import", group.EntityID)
	}

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{
		{
			Ref:           "practice.homeward_step_rollback_first",
			Name:          "HomewardStepRollbackFirstMob",
			MapIndex:      42,
			X:             1810,
			Y:             2910,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
		},
		{
			Ref:           "practice.homeward_step_rollback_conflict",
			Name:          "HomewardStepRollbackConflictMob",
			MapIndex:      42,
			X:             1820,
			Y:             2920,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
		},
	}})
	if err == nil {
		t.Fatal("expected replacement import to fail when the second spawn actor conflicts with a live player VID")
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected failed replacement import not to leak staged visibility frames, got %d", len(queued))
	}
	restored, ok := runtime.SpawnGroupByRef("practice.homeward_step_rollback_original")
	if !ok || restored.EntityID != group.EntityID || restored.X != 1800 || restored.SpawnLeash == nil || restored.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected failed import rollback to restore within_radius spawn actor, ok=%v snapshot=%+v", ok, restored)
	}
	runtime.spawnHomewardMu.Lock()
	restoredDueAt, restoredScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !restoredScheduled || !restoredDueAt.Equal(originalDueAt) {
		t.Fatalf("expected failed import rollback to restore pending homeward-step schedule at %s, got scheduled=%v due_at=%s", originalDueAt, restoredScheduled, restoredDueAt)
	}

	currentTime = originalDueAt.Add(time.Nanosecond)
	queued := flushServerFrames(t, flow)
	if len(queued) == 0 {
		t.Fatal("expected restored homeward-step schedule to fire retained viewer MOVE replication")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode restored homeward-step MOVE: %v", err)
	}
	if moveAck.VID != uint32(group.EntityID) || moveAck.X != 1700 || moveAck.Y != 2800 {
		t.Fatalf("expected restored homeward-step MOVE at authored home, got %+v", moveAck)
	}
	for _, raw := range queued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == uint32(group.EntityID) {
			t.Fatalf("expected restored homeward-step MOVE fanout not to emit retained-viewer CHARACTER_DEL, got %+v among %d queued frames", deleted, len(queued))
		}
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == uint32(group.EntityID) {
			t.Fatalf("expected restored homeward-step MOVE fanout not to emit retained-viewer CHARACTER_ADD, got %+v among %d queued frames", add, len(queued))
		}
	}
}

func TestGameRuntimeNoOpContentBundleImportPrunesStaleSpawnGroupHomewardStepSchedule(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	viewer := peerVisibilityCharacter("SpawnHomewardNoOp", 0x01035553, 4, 1850, 2800, 0, 101, 201)
	viewer.MapIndex = 42
	issuePeerTicket(t, store, "spawn-homeward-noop", 0x55555353, viewer)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	currentTime := time.Unix(1700001080, 0)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", VisibilityMode: "radius", VisibilityRadius: 500, VisibilitySectorSize: 256},
		store,
		nil,
		staticActorStore,
		interactionStore,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "spawn-homeward-noop", 0x55555353)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 5 {
		t.Fatalf("expected base enter-game burst before spawn import, got %d frames", len(enterOut))
	}
	flushServerFrames(t, flow)

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.homeward_step_noop",
		Name:          "HomewardStepNoOpMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import no-op homeward-step spawn bundle: %v", err)
	}
	flushServerFrames(t, flow)
	group, ok := runtime.SpawnGroupByRef("practice.homeward_step_noop")
	if !ok {
		t.Fatal("expected no-op homeward-step spawn group to resolve by ref")
	}
	if _, ok := runtime.UpdateStaticActor(group.EntityID, "HomewardStepNoOpMob", 42, 1800, 2800, 20350); !ok {
		t.Fatal("expected no-op spawn-backed actor update to within_radius position to succeed")
	}
	flushServerFrames(t, flow)

	runtime.spawnHomewardMu.Lock()
	originalDueAt, scheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	staleEntityID := group.EntityID + 9999
	runtime.spawnHomewardStepDueAt[staleEntityID] = originalDueAt
	runtime.spawnHomewardMu.Unlock()
	if !scheduled {
		t.Fatalf("expected within_radius actor %d to have a pending automatic homeward-step schedule before no-op import", group.EntityID)
	}

	sameBundle, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export no-op homeward-step content bundle: %v", err)
	}
	if _, err := runtime.ImportContentBundle(sameBundle); err != nil {
		t.Fatalf("reimport same canonical homeward-step bundle: %v", err)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no-op content import not to queue visibility or target frames, got %d", len(queued))
	}

	runtime.spawnHomewardMu.Lock()
	preservedDueAt, validStillScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	_, staleStillScheduled := runtime.spawnHomewardStepDueAt[staleEntityID]
	scheduleCount := len(runtime.spawnHomewardStepDueAt)
	runtime.spawnHomewardMu.Unlock()
	if !validStillScheduled || !preservedDueAt.Equal(originalDueAt) || staleStillScheduled || scheduleCount != 1 {
		t.Fatalf("expected no-op import to preserve valid actor %d due at %s and prune stale %d schedule, valid_scheduled=%v due_at=%s stale_scheduled=%v schedule_count=%d", group.EntityID, originalDueAt, staleEntityID, validStillScheduled, preservedDueAt, staleStillScheduled, scheduleCount)
	}
	currentTime = originalDueAt.Add(time.Nanosecond)
	queued := flushServerFrames(t, flow)
	if len(queued) == 0 {
		t.Fatal("expected preserved homeward-step schedule to fire after no-op import")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode preserved no-op homeward-step MOVE: %v", err)
	}
	if moveAck.VID != uint32(group.EntityID) || moveAck.X != 1700 || moveAck.Y != 2800 {
		t.Fatalf("expected preserved no-op homeward-step MOVE at authored home, got %+v", moveAck)
	}
}

func TestGameRuntimeSuccessfulContentBundleReplacementClearsStaleSpawnGroupHomewardStepSchedule(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	viewer := peerVisibilityCharacter("SpawnHomewardReplace", 0x01035552, 4, 1850, 2800, 0, 101, 201)
	viewer.MapIndex = 42
	issuePeerTicket(t, store, "spawn-homeward-replace", 0x55555252, viewer)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	currentTime := time.Unix(1700001070, 0)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", VisibilityMode: "radius", VisibilityRadius: 500, VisibilitySectorSize: 256},
		store,
		nil,
		staticActorStore,
		interactionStore,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "spawn-homeward-replace", 0x55555252)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 5 {
		t.Fatalf("expected base enter-game burst before spawn import, got %d frames", len(enterOut))
	}
	flushServerFrames(t, flow)

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.homeward_step_replace_original",
		Name:          "HomewardStepReplaceOriginalMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import original homeward-step replacement spawn bundle: %v", err)
	}
	flushServerFrames(t, flow)
	original, ok := runtime.SpawnGroupByRef("practice.homeward_step_replace_original")
	if !ok {
		t.Fatal("expected original homeward-step replacement spawn group to resolve by ref")
	}
	if _, ok := runtime.UpdateStaticActor(original.EntityID, "HomewardStepReplaceOriginalMob", 42, 1800, 2800, 20350); !ok {
		t.Fatal("expected original spawn-backed actor update to within_radius position to succeed")
	}
	flushServerFrames(t, flow)

	runtime.spawnHomewardMu.Lock()
	originalDueAt, scheduled := runtime.spawnHomewardStepDueAt[original.EntityID]
	staleEntityID := original.EntityID + 9999
	runtime.spawnHomewardStepDueAt[staleEntityID] = originalDueAt
	runtime.spawnHomewardMu.Unlock()
	if !scheduled {
		t.Fatalf("expected within_radius actor %d to have a pending automatic homeward-step schedule before successful replacement", original.EntityID)
	}

	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.homeward_step_replace_new",
		Name:          "HomewardStepReplaceNewMob",
		MapIndex:      42,
		X:             1810,
		Y:             2910,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import replacement homeward-step spawn bundle: %v", err)
	}
	flushServerFrames(t, flow)

	runtime.spawnHomewardMu.Lock()
	_, removedActorStillScheduled := runtime.spawnHomewardStepDueAt[original.EntityID]
	_, unrelatedStaleStillScheduled := runtime.spawnHomewardStepDueAt[staleEntityID]
	scheduleCount := len(runtime.spawnHomewardStepDueAt)
	runtime.spawnHomewardMu.Unlock()
	if removedActorStillScheduled || unrelatedStaleStillScheduled || scheduleCount != 0 {
		t.Fatalf("expected successful replacement to prune removed actor %d and unrelated stale %d homeward-step schedules, removed_scheduled=%v unrelated_scheduled=%v schedule_count=%d", original.EntityID, staleEntityID, removedActorStillScheduled, unrelatedStaleStillScheduled, scheduleCount)
	}
	if _, ok := runtime.SpawnGroupByRef("practice.homeward_step_replace_original"); ok {
		t.Fatal("expected original spawn group to be absent after successful replacement")
	}
	replacement, ok := runtime.SpawnGroupByRef("practice.homeward_step_replace_new")
	if !ok || replacement.SpawnLeash == nil || replacement.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected replacement spawn group to be live at home without a pending homeward step, ok=%v snapshot=%+v", ok, replacement)
	}

	currentTime = originalDueAt.Add(time.Nanosecond)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected cleared stale homeward-step schedule not to fire after successful replacement, got %d frames", len(queued))
	}
}
