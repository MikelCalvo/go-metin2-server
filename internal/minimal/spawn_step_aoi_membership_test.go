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

// Same-map due chase-step MOVE must honor AOI membership through the shared
// RelocateStaticActorTargetDiff path already proven for operator/runtime
// position MOVE: old-position-only viewers get CHARACTER_DEL, newly-visible
// viewers get the ordinary add/info/update burst, and retained viewers still
// get MOVE only while engagement / selected-target stay preserved.
func TestGameRuntimeFlushServerFramesAppliesDueSpawnGroupChaseStepQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Geometry with VisibilityRadius=800, combat target max distance=300,
	// DefaultSpawnAggroRadius=200, and chase max_step=100 toward owner:
	//   mob 1200,2200 -> 1300,2200
	//   owner stays within combat range east of the mob so chase steps toward +X
	//     and outside aggro so proximity cannot steal engagement before the hit
	//   old-only viewer sits at the western visibility edge and loses after +100
	//   retained viewer sits outside aggro but inside AOI for origin and stepped
	//   new-only viewer sits at the eastern destination edge and gains after +100
	owner := peerVisibilityCharacter("ChaseAOIOwner", 0x01030301, 0x02040301, 1450, 2200, 0, 130, 230)
	owner.MapIndex = bootstrapMapIndex
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	oldViewer := peerVisibilityCharacter("ChaseAOIOld", 0x01030302, 0x02040302, 400, 2200, 0, 131, 231)
	oldViewer.MapIndex = bootstrapMapIndex
	retainedViewer := peerVisibilityCharacter("ChaseAOIRetained", 0x01030303, 0x02040303, 1250, 2800, 0, 132, 232)
	retainedViewer.MapIndex = bootstrapMapIndex
	newViewer := peerVisibilityCharacter("ChaseAOINew", 0x01030304, 0x02040304, 2100, 2200, 0, 133, 233)
	newViewer.MapIndex = bootstrapMapIndex
	issuePeerTicket(t, store, "chase-aoi-owner", 0xb0b0b0b1, owner)
	issuePeerTicket(t, store, "chase-aoi-old", 0xb0b0b0b2, oldViewer)
	issuePeerTicket(t, store, "chase-aoi-retained", 0xb0b0b0b3, retainedViewer)
	issuePeerTicket(t, store, "chase-aoi-new", 0xb0b0b0b4, newViewer)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700003100, 0)
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
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.chase_step_aoi_membership",
		Name:          "ChaseStepAOIMob",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.chase_step_aoi_membership")
	if !ok {
		t.Fatal("expected spawn group to resolve by ref")
	}
	mobVID := uint32(group.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "chase-aoi-owner", 0xb0b0b0b1)
	defer closeSessionFlow(t, ownerFlow)
	if !enterBurstContainsStaticActorVID(t, ownerEnter, mobVID) {
		t.Fatal("expected chase owner enter burst to include spawn-backed mob")
	}
	oldFlow, oldEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "chase-aoi-old", 0xb0b0b0b2)
	defer closeSessionFlow(t, oldFlow)
	if !enterBurstContainsStaticActorVID(t, oldEnter, mobVID) {
		t.Fatal("expected old-position viewer enter burst to include spawn-backed mob")
	}
	retainedFlow, retainedEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "chase-aoi-retained", 0xb0b0b0b3)
	defer closeSessionFlow(t, retainedFlow)
	if !enterBurstContainsStaticActorVID(t, retainedEnter, mobVID) {
		t.Fatal("expected retained viewer enter burst to include spawn-backed mob")
	}
	newFlow, newEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "chase-aoi-new", 0xb0b0b0b4)
	defer closeSessionFlow(t, newFlow)
	if enterBurstContainsStaticActorVID(t, newEnter, mobVID) {
		t.Fatal("expected newly-visible destination viewer enter burst to omit origin-only spawn-backed mob")
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, oldFlow)
	flushServerFrames(t, retainedFlow)
	flushServerFrames(t, newFlow)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: mobVID})))
	if err != nil {
		t.Fatalf("unexpected owner target error before chase-step AOI arm: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select chase-step AOI practice mob, got %d frames", len(selectOut))
	}
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  mobVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before chase-step AOI arm: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, immediate retaliation, and damage-info on first chase-arming hit, got %d frames", len(attackOut))
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm a pending chase-step row before AOI membership proof, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected the owned delayed retaliation beat to fire before the later chase MOVE step, got %d frames", len(queued))
	}
	_ = flushServerFrames(t, oldFlow)
	_ = flushServerFrames(t, retainedFlow)
	_ = flushServerFrames(t, newFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) == 0 {
		t.Fatal("expected due chase-step MOVE fanout to queue at least one retained-owner frame")
	}
	ownerMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, ownerQueued[0]))
	if err != nil {
		t.Fatalf("expected retained chase owner to receive MOVE replication, first frame decode err=%v", err)
	}
	if ownerMove.VID != mobVID || ownerMove.X != 1300 || ownerMove.Y != 2200 {
		t.Fatalf("expected chase-step MOVE replication at planned +100 toward owner, got %+v", ownerMove)
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
	if moveAck.VID != mobVID || moveAck.X != 1300 || moveAck.Y != 2200 || moveAck.Duration == 0 {
		t.Fatalf("unexpected retained-viewer MOVE payload: %+v", moveAck)
	}
	if queuedFramesContainCharacterDeleteForVID(t, retainedQueued, mobVID) {
		t.Fatal("expected retained viewer not to receive CHARACTER_DEL across same-map chase MOVE")
	}
	if queuedFramesContainCharacterAddForVID(t, retainedQueued, mobVID) {
		t.Fatal("expected retained viewer not to receive CHARACTER_ADD across same-map chase MOVE")
	}

	newQueued := flushServerFrames(t, newFlow)
	if len(newQueued) < 3 {
		t.Fatalf("expected newly-visible viewer to receive add/info/update burst, got %d frames", len(newQueued))
	}
	add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, newQueued[0]))
	if err != nil {
		t.Fatalf("expected newly-visible CHARACTER_ADD first, got: %v", err)
	}
	if add.VID != mobVID || add.X != 1300 || add.Y != 2200 || add.RaceNum != 20350 {
		t.Fatalf("unexpected newly-visible CHARACTER_ADD: %+v", add)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, newQueued[1]))
	if err != nil {
		t.Fatalf("decode newly-visible CHAR_ADDITIONAL_INFO: %v", err)
	}
	if info.VID != mobVID || info.Name != "ChaseStepAOIMob" {
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

	stepped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || stepped.X != 1300 || stepped.Y != 2200 || stepped.Dead {
		t.Fatalf("expected runtime actor to move one chase step while remaining live, ok=%v snapshot=%+v", ok, stepped)
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("ChaseAOIOwner")
	if !ok {
		t.Fatal("expected chase AOI owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected chase-step AOI membership flush to preserve engagement ownership for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("ChaseAOIOwner"); !ok || snapshot.TargetVID != mobVID {
		t.Fatalf("expected chase-step AOI membership flush to preserve selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}
}

// Same-map due return-step MOVE must honor AOI membership through the shared
// RelocateStaticActorTargetDiff path: old-position-only CHARACTER_DEL, retained
// MOVE, newly-visible add/info/update.
func TestGameRuntimeFlushServerFramesAppliesDueSpawnGroupReturnStepQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Geometry with VisibilityRadius=800 and return max_step=100 toward home:
	//   authored home 2100,2200; displace to 1200,2200 (return_required)
	//   due return step 1200,2200 -> 1300,2200
	//   old-only viewer loses after +100; retained keeps; new-only gains
	oldViewer := peerVisibilityCharacter("ReturnAOIOld", 0x01030401, 0x02040401, 400, 2200, 0, 141, 241)
	oldViewer.MapIndex = bootstrapMapIndex
	retainedViewer := peerVisibilityCharacter("ReturnAOIRetained", 0x01030402, 0x02040402, 1250, 2800, 0, 142, 242)
	retainedViewer.MapIndex = bootstrapMapIndex
	newViewer := peerVisibilityCharacter("ReturnAOINew", 0x01030403, 0x02040403, 2100, 2200, 0, 143, 243)
	newViewer.MapIndex = bootstrapMapIndex
	issuePeerTicket(t, store, "return-aoi-old", 0xc0c0c0c1, oldViewer)
	issuePeerTicket(t, store, "return-aoi-retained", 0xc0c0c0c2, retainedViewer)
	issuePeerTicket(t, store, "return-aoi-new", 0xc0c0c0c3, newViewer)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700003200, 0)
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
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.return_step_aoi_membership",
		Name:          "ReturnStepAOIMob",
		MapIndex:      bootstrapMapIndex,
		X:             2100,
		Y:             2200,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.return_step_aoi_membership")
	if !ok {
		t.Fatal("expected spawn group to resolve by ref")
	}
	mobVID := uint32(group.EntityID)

	oldFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "return-aoi-old", 0xc0c0c0c1)
	defer closeSessionFlow(t, oldFlow)
	retainedFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "return-aoi-retained", 0xc0c0c0c2)
	defer closeSessionFlow(t, retainedFlow)
	newFlow, newEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "return-aoi-new", 0xc0c0c0c3)
	defer closeSessionFlow(t, newFlow)
	if !enterBurstContainsStaticActorVID(t, newEnter, mobVID) {
		t.Fatal("expected destination-home viewer enter burst to include home-position spawn-backed mob")
	}
	flushServerFrames(t, oldFlow)
	flushServerFrames(t, retainedFlow)
	flushServerFrames(t, newFlow)

	if _, ok := runtime.UpdateStaticActor(group.EntityID, "ReturnStepAOIMob", bootstrapMapIndex, 1200, 2200, 20350); !ok {
		t.Fatal("expected same-map displace outside leash to succeed")
	}
	// Drain displace membership / engagement cleanup before the due return step.
	oldDisplaceQueued := flushServerFrames(t, oldFlow)
	if !queuedFramesContainCharacterAddForVID(t, oldDisplaceQueued, mobVID) {
		t.Fatal("expected old-position viewer to gain visibility on return_required displace")
	}
	retainedDisplaceQueued := flushServerFrames(t, retainedFlow)
	if !queuedFramesContainMoveForVID(t, retainedDisplaceQueued, mobVID) && !queuedFramesContainCharacterAddForVID(t, retainedDisplaceQueued, mobVID) {
		t.Fatal("expected retained viewer to observe return_required displace via MOVE or add")
	}
	newDisplaceQueued := flushServerFrames(t, newFlow)
	if !queuedFramesContainCharacterDeleteForVID(t, newDisplaceQueued, mobVID) {
		t.Fatal("expected home-position viewer to lose visibility on return_required displace away from home")
	}
	if pending, ok := runtime.SpawnGroupReturnStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected return_required displace to arm pending return-step, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupReturnStepDelay)
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
	if moveAck.VID != mobVID || moveAck.X != 1300 || moveAck.Y != 2200 || moveAck.Duration == 0 {
		t.Fatalf("unexpected retained-viewer MOVE payload: %+v", moveAck)
	}
	if queuedFramesContainCharacterDeleteForVID(t, retainedQueued, mobVID) {
		t.Fatal("expected retained viewer not to receive CHARACTER_DEL across same-map return MOVE")
	}
	if queuedFramesContainCharacterAddForVID(t, retainedQueued, mobVID) {
		t.Fatal("expected retained viewer not to receive CHARACTER_ADD across same-map return MOVE")
	}

	newQueued := flushServerFrames(t, newFlow)
	if len(newQueued) < 3 {
		t.Fatalf("expected newly-visible viewer to receive add/info/update burst, got %d frames", len(newQueued))
	}
	add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, newQueued[0]))
	if err != nil {
		t.Fatalf("expected newly-visible CHARACTER_ADD first, got: %v", err)
	}
	if add.VID != mobVID || add.X != 1300 || add.Y != 2200 || add.RaceNum != 20350 {
		t.Fatalf("unexpected newly-visible CHARACTER_ADD: %+v", add)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, newQueued[1]))
	if err != nil {
		t.Fatalf("decode newly-visible CHAR_ADDITIONAL_INFO: %v", err)
	}
	if info.VID != mobVID || info.Name != "ReturnStepAOIMob" {
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

	stepped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || stepped.X != 1300 || stepped.Y != 2200 || stepped.Dead || stepped.SpawnLeash == nil || stepped.SpawnLeash.Status != worldruntime.SpawnLeashStatusReturnRequired {
		t.Fatalf("expected runtime actor to move one return step and remain return_required, ok=%v snapshot=%+v", ok, stepped)
	}
}

// Same-map due homeward-step MOVE must honor AOI membership through the shared
// RelocateStaticActorTargetDiff path after an unengaged within_radius displace.
func TestGameRuntimeFlushServerFramesAppliesDueSpawnGroupHomewardStepQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Geometry with VisibilityRadius=800 and homeward max_step=100 toward home:
	//   authored home 1200,2200; UpdateStaticActor to 1500,2200 (within_radius)
	//   due homeward step 1500,2200 -> 1400,2200
	//   old-only viewer at eastern edge loses; retained keeps; new-only gains west
	oldViewer := peerVisibilityCharacter("HomewardAOIOld", 0x01030501, 0x02040501, 2300, 2200, 0, 151, 251)
	oldViewer.MapIndex = bootstrapMapIndex
	retainedViewer := peerVisibilityCharacter("HomewardAOIRetained", 0x01030502, 0x02040502, 1450, 2800, 0, 152, 252)
	retainedViewer.MapIndex = bootstrapMapIndex
	newViewer := peerVisibilityCharacter("HomewardAOINew", 0x01030503, 0x02040503, 600, 2200, 0, 153, 253)
	newViewer.MapIndex = bootstrapMapIndex
	issuePeerTicket(t, store, "homeward-aoi-old", 0xd0d0d0d1, oldViewer)
	issuePeerTicket(t, store, "homeward-aoi-retained", 0xd0d0d0d2, retainedViewer)
	issuePeerTicket(t, store, "homeward-aoi-new", 0xd0d0d0d3, newViewer)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700003300, 0)
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
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.homeward_step_aoi_membership",
		Name:          "HomewardStepAOIMob",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.homeward_step_aoi_membership")
	if !ok {
		t.Fatal("expected spawn group to resolve by ref")
	}
	mobVID := uint32(group.EntityID)

	oldFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "homeward-aoi-old", 0xd0d0d0d1)
	defer closeSessionFlow(t, oldFlow)
	retainedFlow, retainedEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "homeward-aoi-retained", 0xd0d0d0d2)
	defer closeSessionFlow(t, retainedFlow)
	if !enterBurstContainsStaticActorVID(t, retainedEnter, mobVID) {
		t.Fatal("expected retained viewer enter burst to include home-position spawn-backed mob")
	}
	newFlow, newEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "homeward-aoi-new", 0xd0d0d0d3)
	defer closeSessionFlow(t, newFlow)
	if !enterBurstContainsStaticActorVID(t, newEnter, mobVID) {
		t.Fatal("expected western home viewer enter burst to include home-position spawn-backed mob")
	}
	flushServerFrames(t, oldFlow)
	flushServerFrames(t, retainedFlow)
	flushServerFrames(t, newFlow)

	if _, ok := runtime.UpdateStaticActor(group.EntityID, "HomewardStepAOIMob", bootstrapMapIndex, 1500, 2200, 20350); !ok {
		t.Fatal("expected same-map within_radius displace to succeed")
	}
	oldDisplaceQueued := flushServerFrames(t, oldFlow)
	if !queuedFramesContainCharacterAddForVID(t, oldDisplaceQueued, mobVID) {
		t.Fatal("expected eastern old-position viewer to gain visibility on within_radius displace")
	}
	_ = flushServerFrames(t, retainedFlow)
	newDisplaceQueued := flushServerFrames(t, newFlow)
	if !queuedFramesContainCharacterDeleteForVID(t, newDisplaceQueued, mobVID) {
		t.Fatal("expected western home viewer to lose visibility on within_radius displace east")
	}
	if pending, ok := runtime.SpawnGroupHomewardStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected within_radius displace to arm pending homeward-step, ok=%v snapshot=%+v", ok, pending)
	}
	if pending, ok := runtime.SpawnGroupReturnStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected within_radius displace not to arm return-step, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
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
	if moveAck.VID != mobVID || moveAck.X != 1400 || moveAck.Y != 2200 || moveAck.Duration == 0 {
		t.Fatalf("unexpected retained-viewer MOVE payload: %+v", moveAck)
	}
	if queuedFramesContainCharacterDeleteForVID(t, retainedQueued, mobVID) {
		t.Fatal("expected retained viewer not to receive CHARACTER_DEL across same-map homeward MOVE")
	}
	if queuedFramesContainCharacterAddForVID(t, retainedQueued, mobVID) {
		t.Fatal("expected retained viewer not to receive CHARACTER_ADD across same-map homeward MOVE")
	}

	newQueued := flushServerFrames(t, newFlow)
	if len(newQueued) < 3 {
		t.Fatalf("expected newly-visible viewer to receive add/info/update burst, got %d frames", len(newQueued))
	}
	add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, newQueued[0]))
	if err != nil {
		t.Fatalf("expected newly-visible CHARACTER_ADD first, got: %v", err)
	}
	if add.VID != mobVID || add.X != 1400 || add.Y != 2200 || add.RaceNum != 20350 {
		t.Fatalf("unexpected newly-visible CHARACTER_ADD: %+v", add)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, newQueued[1]))
	if err != nil {
		t.Fatalf("decode newly-visible CHAR_ADDITIONAL_INFO: %v", err)
	}
	if info.VID != mobVID || info.Name != "HomewardStepAOIMob" {
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

	stepped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || stepped.X != 1400 || stepped.Y != 2200 || stepped.Dead || stepped.SpawnLeash == nil || stepped.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected runtime actor to move one homeward step and remain within_radius, ok=%v snapshot=%+v", ok, stepped)
	}
}
