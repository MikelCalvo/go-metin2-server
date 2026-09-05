package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Live same-map spawn-backed operator/runtime position updates reuse retained-viewer
// MOVE instead of delete/readd. Presentation/name/race refreshes stay on the already-
// owned delete/readd path.
func TestGameRuntimeUpdateStaticActorSameMapSpawnGroupPositionUsesRetainedViewerMove(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	viewer := peerVisibilityCharacter("OperatorPositionMoveViewer", 0x010301fa, 0x020401fa, 1200, 2200, 0, 110, 210)
	issuePeerTicket(t, store, "op-position-move-viewer", 0xfafafafa, viewer)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
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
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.operator_position_move",
		Name:          "OperatorPositionMoveMob",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.operator_position_move")
	if !ok {
		t.Fatal("expected spawn group to resolve by ref")
	}

	flow, enter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "op-position-move-viewer", 0xfafafafa)
	defer closeSessionFlow(t, flow)
	if len(enter) != 8 {
		t.Fatalf("expected bootstrap with visible spawn-backed mob, got %d frames", len(enter))
	}
	flushServerFrames(t, flow)

	updated, ok := runtime.UpdateStaticActor(group.EntityID, "OperatorPositionMoveMob", bootstrapMapIndex, 1250, 2200, 101)
	if !ok {
		t.Fatal("expected same-map live spawn-backed position update to succeed")
	}
	if updated.EntityID != group.EntityID || updated.X != 1250 || updated.Y != 2200 || updated.Name != "OperatorPositionMoveMob" || updated.RaceNum != 101 {
		t.Fatalf("unexpected updated spawn-group snapshot: %+v", updated)
	}

	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		if len(queued) == 4 {
			if _, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, queued[0])); err == nil {
				t.Fatalf("expected retained-viewer MOVE for same-map live spawn-backed position update, got delete/readd (%d frames)", len(queued))
			}
		}
		t.Fatalf("expected 1 retained-viewer MOVE frame, got %d", len(queued))
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("expected retained-viewer MOVE replication instead of delete/readd: %v", err)
	}
	if moveAck.VID != uint32(group.EntityID) || moveAck.X != 1250 || moveAck.Y != 2200 {
		t.Fatalf("unexpected retained-viewer MOVE payload: %+v", moveAck)
	}
	if moveAck.Duration == 0 {
		t.Fatalf("expected retained-viewer MOVE to carry a non-zero bootstrap duration, got %+v", moveAck)
	}
}

func TestGameRuntimeUpdateStaticActorSameMapSpawnGroupPresentationKeepsDeleteReadd(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	viewer := peerVisibilityCharacter("OperatorPresentationViewer", 0x010301fb, 0x020401fb, 1200, 2200, 0, 111, 211)
	issuePeerTicket(t, store, "op-presentation-viewer", 0xfbfbfbfb, viewer)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
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
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.operator_presentation_refresh",
		Name:          "OperatorPresentationMob",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.operator_presentation_refresh")
	if !ok {
		t.Fatal("expected spawn group to resolve by ref")
	}

	flow, enter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "op-presentation-viewer", 0xfbfbfbfb)
	defer closeSessionFlow(t, flow)
	if len(enter) != 8 {
		t.Fatalf("expected bootstrap with visible spawn-backed mob, got %d frames", len(enter))
	}
	flushServerFrames(t, flow)

	updated, ok := runtime.UpdateStaticActor(group.EntityID, "OperatorPresentationMobRenamed", bootstrapMapIndex, 1200, 2200, 102)
	if !ok {
		t.Fatal("expected same-map live spawn-backed presentation update to succeed")
	}
	if updated.EntityID != group.EntityID || updated.X != 1200 || updated.Y != 2200 || updated.Name != "OperatorPresentationMobRenamed" || updated.RaceNum != 102 {
		t.Fatalf("unexpected updated spawn-group snapshot: %+v", updated)
	}

	queued := flushServerFrames(t, flow)
	if len(queued) != 4 {
		t.Fatalf("expected retained-viewer delete/readd for same-map presentation refresh, got %d frames", len(queued))
	}
	if _, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, queued[0])); err != nil {
		t.Fatalf("expected CHARACTER_DEL first for presentation refresh, got: %v", err)
	}
	add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode presentation refresh CHARACTER_ADD: %v", err)
	}
	if add.VID != uint32(group.EntityID) || add.Type != worldproto.CharacterTypeMonster || add.X != 1200 || add.Y != 2200 || add.RaceNum != 102 {
		t.Fatalf("unexpected presentation refresh CHARACTER_ADD: %+v", add)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, queued[2]))
	if err != nil {
		t.Fatalf("decode presentation refresh CHAR_ADDITIONAL_INFO: %v", err)
	}
	if info.VID != uint32(group.EntityID) || info.Name != "OperatorPresentationMobRenamed" {
		t.Fatalf("unexpected presentation refresh CHAR_ADDITIONAL_INFO: %+v", info)
	}
	if _, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, queued[3])); err != nil {
		t.Fatalf("expected CHARACTER_UPDATE last for presentation refresh: %v", err)
	}
}

// Same-map live spawn-backed position MOVE must also honor AOI membership:
// old-position-only viewers get CHARACTER_DEL, newly-visible viewers get the
// ordinary add/info/update burst, and retained viewers still get MOVE only.
func TestGameRuntimeUpdateStaticActorSameMapSpawnGroupPositionQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Geometry with VisibilityRadius=800:
	//   mob 1200,2200 -> 2100,2200 (distance 900)
	//   old-only viewer stays near origin and loses visibility
	//   retained viewer sits midway and keeps visibility
	//   new-only viewer sits at destination and gains visibility
	oldViewer := peerVisibilityCharacter("OperatorAOIOld", 0x01030201, 0x02040201, 1200, 2200, 0, 120, 220)
	retainedViewer := peerVisibilityCharacter("OperatorAOIRetained", 0x01030202, 0x02040202, 1650, 2200, 0, 121, 221)
	newViewer := peerVisibilityCharacter("OperatorAOINew", 0x01030203, 0x02040203, 2100, 2200, 0, 122, 222)
	issuePeerTicket(t, store, "op-aoi-old", 0xa0a0a0a1, oldViewer)
	issuePeerTicket(t, store, "op-aoi-retained", 0xa0a0a0a2, retainedViewer)
	issuePeerTicket(t, store, "op-aoi-new", 0xa0a0a0a3, newViewer)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
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
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.operator_position_aoi_membership",
		Name:          "OperatorPositionAOIMob",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.operator_position_aoi_membership")
	if !ok {
		t.Fatal("expected spawn group to resolve by ref")
	}
	mobVID := uint32(group.EntityID)

	oldFlow, oldEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "op-aoi-old", 0xa0a0a0a1)
	defer closeSessionFlow(t, oldFlow)
	if !enterBurstContainsStaticActorVID(t, oldEnter, mobVID) {
		t.Fatal("expected old-position viewer enter burst to include spawn-backed mob")
	}
	retainedFlow, retainedEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "op-aoi-retained", 0xa0a0a0a2)
	defer closeSessionFlow(t, retainedFlow)
	if !enterBurstContainsStaticActorVID(t, retainedEnter, mobVID) {
		t.Fatal("expected retained viewer enter burst to include spawn-backed mob")
	}
	newFlow, newEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "op-aoi-new", 0xa0a0a0a3)
	defer closeSessionFlow(t, newFlow)
	if enterBurstContainsStaticActorVID(t, newEnter, mobVID) {
		t.Fatal("expected newly-visible destination viewer enter burst to omit origin-only spawn-backed mob")
	}
	flushServerFrames(t, oldFlow)
	flushServerFrames(t, retainedFlow)
	flushServerFrames(t, newFlow)

	updated, ok := runtime.UpdateStaticActor(group.EntityID, "OperatorPositionAOIMob", bootstrapMapIndex, 2100, 2200, 101)
	if !ok {
		t.Fatal("expected same-map live spawn-backed position update to succeed")
	}
	if updated.EntityID != group.EntityID || updated.X != 2100 || updated.Y != 2200 {
		t.Fatalf("unexpected updated spawn-group snapshot: %+v", updated)
	}

	oldQueued := flushServerFrames(t, oldFlow)
	if !queuedFramesContainCharacterDeleteForVID(t, oldQueued, mobVID) {
		t.Fatalf("expected old-position-only viewer to receive CHARACTER_DEL for mob VID %d, got %d frames", mobVID, len(oldQueued))
	}
	if queuedFramesContainMoveForVID(t, oldQueued, mobVID) {
		t.Fatal("expected old-position-only viewer not to receive retained MOVE for lost visibility")
	}
	if queuedFramesContainCharacterAddForVID(t, oldQueued, mobVID) {
		t.Fatal("expected old-position-only viewer not to receive CHARACTER_ADD after losing visibility")
	}

	retainedQueued := flushServerFrames(t, retainedFlow)
	if len(retainedQueued) == 0 {
		t.Fatal("expected retained viewer to receive MOVE replication")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, retainedQueued[0]))
	if err != nil {
		t.Fatalf("expected retained-viewer MOVE replication instead of delete/readd: %v", err)
	}
	if moveAck.VID != mobVID || moveAck.X != 2100 || moveAck.Y != 2200 || moveAck.Duration == 0 {
		t.Fatalf("unexpected retained-viewer MOVE payload: %+v", moveAck)
	}
	if queuedFramesContainCharacterDeleteForVID(t, retainedQueued, mobVID) {
		t.Fatal("expected retained viewer not to receive CHARACTER_DEL across same-map position MOVE")
	}
	if queuedFramesContainCharacterAddForVID(t, retainedQueued, mobVID) {
		t.Fatal("expected retained viewer not to receive CHARACTER_ADD across same-map position MOVE")
	}

	newQueued := flushServerFrames(t, newFlow)
	if len(newQueued) < 3 {
		t.Fatalf("expected newly-visible viewer to receive add/info/update burst, got %d frames", len(newQueued))
	}
	add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, newQueued[0]))
	if err != nil {
		t.Fatalf("expected newly-visible CHARACTER_ADD first, got: %v", err)
	}
	if add.VID != mobVID || add.X != 2100 || add.Y != 2200 || add.RaceNum != 101 {
		t.Fatalf("unexpected newly-visible CHARACTER_ADD: %+v", add)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, newQueued[1]))
	if err != nil {
		t.Fatalf("decode newly-visible CHAR_ADDITIONAL_INFO: %v", err)
	}
	if info.VID != mobVID || info.Name != "OperatorPositionAOIMob" {
		t.Fatalf("unexpected newly-visible CHAR_ADDITIONAL_INFO: %+v", info)
	}
	if _, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, newQueued[2])); err != nil {
		t.Fatalf("expected newly-visible CHARACTER_UPDATE third: %v", err)
	}
	if queuedFramesContainMoveForVID(t, newQueued, mobVID) {
		t.Fatal("expected newly-visible viewer not to receive retained MOVE")
	}
	if queuedFramesContainCharacterDeleteForVID(t, newQueued, mobVID) {
		t.Fatal("expected newly-visible viewer not to receive CHARACTER_DEL")
	}
}

func enterBurstContainsStaticActorVID(t *testing.T, frames [][]byte, vid uint32) bool {
	t.Helper()
	for _, raw := range frames {
		add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw))
		if err == nil && add.VID == vid {
			return true
		}
	}
	return false
}

func queuedFramesContainCharacterDeleteForVID(t *testing.T, frames [][]byte, vid uint32) bool {
	t.Helper()
	for _, raw := range frames {
		deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw))
		if err == nil && deleted.VID == vid {
			return true
		}
	}
	return false
}

func queuedFramesContainCharacterAddForVID(t *testing.T, frames [][]byte, vid uint32) bool {
	t.Helper()
	for _, raw := range frames {
		add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw))
		if err == nil && add.VID == vid {
			return true
		}
	}
	return false
}

func queuedFramesContainMoveForVID(t *testing.T, frames [][]byte, vid uint32) bool {
	t.Helper()
	for _, raw := range frames {
		moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, raw))
		if err == nil && moveAck.VID == vid {
			return true
		}
	}
	return false
}

func TestSharedWorldRegistryUpdateStaticActorSameMapSpawnGroupPositionClearsEngagement(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1)
	registry := newSharedWorldRegistryWithTopology(topology)
	owner := peerVisibilityCharacter("Owner", 0x010301fc, 0x020401fc, 1100, 2100, 0, 101, 201)
	watcher := peerVisibilityCharacter("Watcher", 0x010301fd, 0x020401fd, 1200, 2200, 0, 102, 202)
	ownerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	if ownerID == 0 {
		t.Fatal("expected owner join to return a live shared-world entity ID")
	}
	watcherPending := newPendingServerFrames()
	watcherID, _ := registry.Join(watcher, watcherPending, nil)
	if watcherID == 0 {
		t.Fatal("expected watcher join to return a live shared-world entity ID")
	}
	ownerPending.flush()
	watcherPending.flush()

	actor, ok := registry.registerStaticActor(0, "PracticeMobAlpha", bootstrapMapIndex, 1200, 2200, 20350, "", "", worldruntime.StaticActorCombatKindTrainingDummy, "practice.operator_position_engagement", worldruntime.StaticActorDeathReward{})
	if !ok {
		t.Fatal("expected visible practice-mob registration to succeed")
	}
	targetVID := uint32(actor.EntityID)
	ownerTarget := registry.AttemptStaticActorCombatTarget(ownerID, targetVID)
	if !ownerTarget.Accepted {
		t.Fatalf("expected owner target-selection to accept visible practice mob before position MOVE, got %+v", ownerTarget)
	}
	if !registry.SetSessionCombatTarget(ownerID, ownerTarget.TargetVID) {
		t.Fatal("expected owner selected combat target ownership to be recorded before position MOVE")
	}
	ownerPending.flush()

	ownerAttack := registry.AttemptSelectedStaticActorAttack(ownerID, ownerTarget.TargetVID, ownerTarget.SnapshotVersion, targetVID)
	if !ownerAttack.Accepted {
		t.Fatalf("expected owner attack to accept visible practice mob before position MOVE, got %+v", ownerAttack)
	}
	if thirdTarget := registry.AttemptStaticActorCombatTarget(watcherID, targetVID); thirdTarget.Accepted {
		t.Fatalf("expected fresh third-party target attempt to stay aggro-gated before position MOVE, got %+v", thirdTarget)
	}

	updated, ok := registry.UpdateStaticActor(actor.EntityID, "PracticeMobAlpha", bootstrapMapIndex, 1250, 2200, 20350)
	if !ok || updated.EntityID != actor.EntityID || updated.X != 1250 || updated.Y != 2200 {
		t.Fatalf("expected position-only spawn-backed update to return moved actor snapshot, got actor=%+v ok=%v", updated, ok)
	}

	queued := ownerPending.flush()
	if len(queued) != 2 {
		t.Fatalf("expected retained-viewer MOVE plus selected-target clear for position-only update, got %d frames", len(queued))
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("expected retained-viewer MOVE for position-only update: %v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 1250 || moveAck.Y != 2200 || moveAck.Duration == 0 {
		t.Fatalf("unexpected retained-viewer MOVE payload for position-only update: %+v", moveAck)
	}

	afterUpdate := registry.AttemptStaticActorCombatTarget(watcherID, targetVID)
	if !afterUpdate.Accepted {
		t.Fatalf("expected position-only MOVE update to release old aggro-lite ownership for fresh target attempts, got %+v", afterUpdate)
	}
	staleAttack := registry.AttemptSelectedStaticActorAttack(ownerID, ownerTarget.TargetVID, ownerTarget.SnapshotVersion, targetVID)
	if staleAttack.Accepted || staleAttack.Failure != StaticActorCombatAttackFailureNoActiveTarget {
		t.Fatalf("expected position-only MOVE update to clear selected-target ownership, got %+v", staleAttack)
	}
}
