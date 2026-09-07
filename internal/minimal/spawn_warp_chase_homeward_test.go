package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Authored warp INTERACT after a chase-displaced within_radius practice mob
// must clear the pending chase deadline and arm homeward, matching operator
// RelocateCharacter / applySelectedCharacterTransfer. Warp rebootstrap uses
// the same engagement snapshot before sharedWorld.transfer clears engaged_by,
// then re-syncs homeward after clearActiveCombatTarget. Contrast with
// TestGameSessionFlowPracticeMobWarpRebootstrapQueuesTargetClearAndReleasesAggro,
// which only asserts TARGET(0, 0) plus aggro release on an at-home dummy.
func TestGameRuntimeWarpInteractClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("WarpHomewardOwner", 0x01030801, 0x02040801, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	// Watcher stays outside default aggro of displaced 1800 and home 1700 so
	// proximity cannot re-engage after warp, while remaining a retained
	// visibility viewer for homeward MOVE.
	watcher := peerVisibilityCharacter("WarpHomewardWatcher", 0x01030802, 0x02040802, 1450, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "warp-homeward-owner", 0x90909091, owner)
	issuePeerTicket(t, store, "warp-homeward-watcher", 0x90909092, watcher)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005500, 0)
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
		t.Fatalf("unexpected game runtime error for warp chase/homeward: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	const (
		spawnRef = "practice.warp_homeward_after_chase"
		warpRef  = "npc:practice_warp_homeward"
	)
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "WarpHomewardGate",
			MapIndex:        42,
			X:               2000,
			Y:               2800,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindWarp,
			InteractionRef:  warpRef,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           spawnRef,
			Name:          "WarpHomewardMob",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
		}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind:     interactionstore.KindWarp,
			Ref:      warpRef,
			MapIndex: 43,
			X:        5000,
			Y:        5000,
			Text:     "",
		}},
	}); err != nil {
		t.Fatalf("import warp chase/homeward spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok {
		t.Fatal("expected warp chase/homeward spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)
	var teleporterVID uint32
	for _, actor := range runtime.StaticActors() {
		if actor.InteractionKind == interactionstore.KindWarp && actor.InteractionRef == warpRef {
			teleporterVID = uint32(actor.EntityID)
			break
		}
	}
	if teleporterVID == 0 {
		t.Fatal("expected warp teleporter to resolve after import")
	}

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "warp-homeward-owner", 0x90909091)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "warp-homeward-watcher", 0x90909092)
	defer closeSessionFlow(t, watcherFlow)
	flushServerFrames(t, watcherFlow)
	flushServerFrames(t, ownerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before warp chase displace: %v", err)
	}
	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before warp chase displace: %v", err)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm chase before warp, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before warp chase displace, got %d frames", len(queued))
	}
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to displace actor toward owner before warp")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before warp: %v", err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward owner before warp, got %+v", chaseMove)
	}
	_ = flushServerFrames(t, watcherFlow)

	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.X != 1800 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected chase-displaced within_radius actor before warp, ok=%v snapshot=%+v", ok, displaced)
	}

	warpOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: teleporterVID})))
	if err != nil {
		t.Fatalf("unexpected warp interaction error after chase displace: %v", err)
	}
	if !containsServerTargetClear(t, warpOut) {
		t.Fatalf("expected warp rebootstrap response to carry selected-target clear, got %d frames", len(warpOut))
	}
	connected := runtime.ConnectedCharacters()
	if len(connected) != 2 {
		t.Fatalf("expected owner and watcher to remain connected after warp, got %+v", connected)
	}
	var ownerConnected bool
	for _, snapshot := range connected {
		if snapshot.Name != "WarpHomewardOwner" {
			continue
		}
		ownerConnected = true
		if snapshot.MapIndex != 43 || snapshot.X != 5000 || snapshot.Y != 5000 {
			t.Fatalf("expected owner warp destination 43,5000,5000, got %+v", snapshot)
		}
	}
	if !ownerConnected {
		t.Fatalf("expected owner connected snapshot after warp, got %+v", connected)
	}
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, watcherFlow)

	runtime.spawnChaseMu.Lock()
	_, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduled {
		t.Fatalf("expected owner warp to clear pending chase deadline for entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase-step inspection to omit actor after warp, ok=%v snapshot=%+v", ok, pending)
	}

	runtime.spawnHomewardMu.Lock()
	homewardDueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected warp engagement release on within_radius displace to arm homeward deadline for entity %d", group.EntityID)
	}
	expectedHomewardDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !homewardDueAt.Equal(expectedHomewardDueAt) {
		t.Fatalf("expected warp homeward deadline at %s, got %s", expectedHomewardDueAt, homewardDueAt)
	}

	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no homeward MOVE on watcher before the 1s deadline after warp, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	homewardQueued := flushServerFrames(t, watcherFlow)
	if len(homewardQueued) == 0 {
		t.Fatal("expected due homeward-step after warp to queue retained watcher MOVE toward home")
	}
	homewardMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, homewardQueued[0]))
	if err != nil {
		t.Fatalf("expected retained watcher after warp to receive MOVE replication, first frame decode err=%v", err)
	}
	if homewardMove.VID != targetVID || homewardMove.X != 1700 || homewardMove.Y != 2800 {
		t.Fatalf("expected homeward MOVE to authored home after warp, got %+v", homewardMove)
	}
	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected homeward-step after warp to restore at_home leash state, ok=%v snapshot=%+v", ok, returned)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillHomeward := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillHomeward {
		t.Fatalf("expected completed at-home homeward-step after warp to clear pending deadline for entity %d", group.EntityID)
	}
}
