package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterInRadiusRelease(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AggroSuppressOwner", 0x01030196, 0x02040196, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "aggro-suppress-owner", 0x46464646, owner)
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	currentTime := time.Unix(1700002400, 0)

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
		t.Fatalf("new game runtime for proximity suppress leave/re-enter: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.aggro_suppress_reenter",
		Name:          "AggroSuppressMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import proximity suppress spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.aggro_suppress_reenter")
	if !ok {
		t.Fatal("expected proximity suppress spawn group to resolve by ref")
	}

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "aggro-suppress-owner", 0x46464646)
	defer closeSessionFlow(t, ownerFlow)
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("AggroSuppressOwner")
	if !ok {
		t.Fatal("expected proximity suppress owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected pending-frame proximity acquisition to engage owner for entity %d", group.EntityID)
	}

	clearOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: 0})))
	if err != nil {
		t.Fatalf("unexpected owner TARGET(0) clear while still inside aggro radius: %v", err)
	}
	if len(clearOut) != 0 {
		t.Fatalf("expected proximity-only TARGET(0) clear to emit no frames, got %d", len(clearOut))
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected in-radius TARGET(0) clear to release engaged_by for entity %d", group.EntityID)
	}

	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected suppress to keep acquisition silent while owner remains inside aggro radius, got %d queued frames", len(queued))
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected still-inside owner to stay suppressed after in-radius engagement release for entity %d", group.EntityID)
	}
	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation to stay cancelled while suppress blocks reacquire, got %d queued frames", len(queued))
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected delayed flush not to re-lock suppressed still-inside owner for entity %d", group.EntityID)
	}

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1950,
		Y:    2800,
		Time: 0x51525355,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while leaving aggro radius after suppress: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after leaving aggro radius, got %d frames", len(moveOut))
	}
	_ = flushServerFrames(t, ownerFlow)

	moveIn, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1850,
		Y:    2800,
		Time: 0x51525356,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while re-entering aggro radius after suppress: %v", err)
	}
	if len(moveIn) != 1 {
		t.Fatalf("expected 1 immediate self move ack after re-entering aggro radius, got %d frames", len(moveIn))
	}
	_ = flushServerFrames(t, ownerFlow)
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected leave/re-enter to clear suppress and reacquire engagement for entity %d", group.EntityID)
	}
}

func TestGameRuntimeProximityAggroDeathAndRespawnSeedSuppressesNearbyReacquireUntilLeaveAndReenter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AggroDeathSuppressOwner", 0x01030197, 0x02040197, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "aggro-death-suppress-owner", 0x47474747, owner)
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	currentTime := time.Unix(1700002500, 0)

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
		t.Fatalf("new game runtime for proximity death/respawn suppress: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.aggro_death_suppress",
		Name:          "AggroDeathSuppressMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import proximity death/respawn suppress spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.aggro_death_suppress")
	if !ok {
		t.Fatal("expected proximity death/respawn suppress spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "aggro-death-suppress-owner", 0x47474747)
	defer closeSessionFlow(t, ownerFlow)
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("AggroDeathSuppressOwner")
	if !ok {
		t.Fatal("expected proximity death/respawn suppress owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected pending-frame proximity acquisition before kill for entity %d", group.EntityID)
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected owner target selection before proximity death suppress kill: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 owner target-selection frame before kill, got %d", len(selectOut))
	}

	for hit := 0; hit < int(worldruntime.PracticeMobBootstrapMaxHP); hit++ {
		attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected owner attack %d before proximity death suppress: %v", hit+1, err)
		}
		if len(attackOut) == 0 {
			t.Fatalf("expected accepted attack %d before proximity death suppress to emit frames", hit+1)
		}
		currentTime = currentTime.Add(250 * time.Millisecond)
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected killing hit to release engagement and seed proximity suppress for entity %d", group.EntityID)
	}

	_ = flushServerFrames(t, ownerFlow)
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected still-dead nearby owner not to reacquire engagement for entity %d", group.EntityID)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) == 0 {
		t.Fatal("expected respawn rebuild frames for nearby owner after practice-mob respawn delay")
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected respawn seed suppress to block instant nearby reacquire for entity %d", group.EntityID)
	}
	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		for _, raw := range queued {
			if _, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, raw)); err == nil {
				t.Fatalf("expected suppressed post-respawn owner not to receive delayed retaliation POINT_CHANGE, got %#v", queued)
			}
		}
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected delayed post-respawn flush to keep nearby owner suppressed for entity %d", group.EntityID)
	}

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1950,
		Y:    2800,
		Time: 0x51525357,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while leaving aggro radius after respawn suppress: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after leaving aggro radius post-respawn, got %d frames", len(moveOut))
	}
	_ = flushServerFrames(t, ownerFlow)

	moveIn, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1850,
		Y:    2800,
		Time: 0x51525358,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while re-entering aggro radius after respawn suppress: %v", err)
	}
	if len(moveIn) != 1 {
		t.Fatalf("expected 1 immediate self move ack after re-entering aggro radius post-respawn, got %d frames", len(moveIn))
	}
	_ = flushServerFrames(t, ownerFlow)
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected leave/re-enter after respawn suppress to reacquire engagement for entity %d", group.EntityID)
	}
}

func TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorRestartHere(t *testing.T) {
	// Owner death floor is an engagement-release boundary. The still-inside owner must
	// stay proximity-suppressed through /restart_here until an explicit leave/re-enter of
	// DefaultSpawnAggroRadius, matching TARGET(0) and death/respawn suppress seeding.
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AggroFloorSuppressOwner", 0x0103019a, 0x0204019a, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "aggro-floor-suppress-owner", 0x4a4a4a4a, owner)
	if err := accounts.Save(accountstore.Account{Login: "aggro-floor-suppress-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed proximity death-floor suppress owner account: %v", err)
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	currentTime := time.Unix(1700002600, 0)

	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     400,
			VisibilitySectorSize: 200,
		},
		store,
		accounts,
		staticActorStore,
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
	)
	if err != nil {
		t.Fatalf("new game runtime for proximity death-floor suppress restart_here: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.aggro_floor_suppress_restart",
		Name:          "AggroFloorSuppressMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import proximity death-floor suppress spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.aggro_floor_suppress_restart")
	if !ok {
		t.Fatal("expected proximity death-floor suppress spawn group to resolve by ref")
	}

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "aggro-floor-suppress-owner", 0x4a4a4a4a)
	defer closeSessionFlow(t, ownerFlow)
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("AggroFloorSuppressOwner")
	if !ok {
		t.Fatal("expected proximity death-floor suppress owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected pending-frame proximity acquisition to engage owner for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("AggroFloorSuppressOwner"); ok {
		t.Fatalf("expected proximity-armed death-floor suppress path not to invent selected combat target ownership, got %+v", snapshot)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	floorQueued := flushServerFrames(t, ownerFlow)
	if len(floorQueued) != 3 {
		t.Fatalf("expected proximity-armed owner-floor retaliation to emit point-change, player dead, and target clear, got %d frames", len(floorQueued))
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected proximity-armed death floor to release engagement for entity %d", group.EntityID)
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after proximity-armed death floor: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatal("expected /restart_here recovery frames after proximity-armed death floor")
	}
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok = runtime.sharedWorld.playerEntityByName("AggroFloorSuppressOwner")
	if !ok {
		t.Fatal("expected proximity death-floor suppress owner entity to remain registered after /restart_here")
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected still-inside owner to stay suppressed after death-floor /restart_here for entity %d", group.EntityID)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		for _, raw := range queued {
			if _, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, raw)); err == nil {
				t.Fatalf("expected suppressed post-restart_here owner not to receive delayed retaliation POINT_CHANGE, got %#v", queued)
			}
		}
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected delayed flush not to re-lock suppressed still-inside owner after death-floor /restart_here for entity %d", group.EntityID)
	}

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1950,
		Y:    2800,
		Time: 0x51525359,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while leaving aggro radius after death-floor /restart_here suppress: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after leaving aggro radius post-restart_here, got %d frames", len(moveOut))
	}
	_ = flushServerFrames(t, ownerFlow)

	moveIn, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1850,
		Y:    2800,
		Time: 0x5152535a,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while re-entering aggro radius after death-floor /restart_here suppress: %v", err)
	}
	if len(moveIn) != 1 {
		t.Fatalf("expected 1 immediate self move ack after re-entering aggro radius post-restart_here, got %d frames", len(moveIn))
	}
	_ = flushServerFrames(t, ownerFlow)
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected leave/re-enter after death-floor /restart_here suppress to reacquire engagement for entity %d", group.EntityID)
	}
}

func TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorPhaseSelectRestartHere(t *testing.T) {
	// Owner death floor seeds proximity suppress by subject entity ID. /phase_select
	// Leave + fresh Join allocates a new entity ID, so the same still-inside owner must
	// keep suppress across that identity change through /restart_here until leave/re-enter.
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AggroFloorPhaseOwner", 0x0103019b, 0x0204019b, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "aggro-floor-phase", 0x4b4b4b4b, owner)
	if err := accounts.Save(accountstore.Account{Login: "aggro-floor-phase", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed proximity death-floor phase_select suppress owner account: %v", err)
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	currentTime := time.Unix(1700002700, 0)

	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     400,
			VisibilitySectorSize: 200,
		},
		store,
		accounts,
		staticActorStore,
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
	)
	if err != nil {
		t.Fatalf("new game runtime for proximity death-floor suppress phase_select restart_here: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.aggro_floor_suppress_phase_select",
		Name:          "AggroFloorPhaseSuppressMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import proximity death-floor phase_select suppress spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.aggro_floor_suppress_phase_select")
	if !ok {
		t.Fatal("expected proximity death-floor phase_select suppress spawn group to resolve by ref")
	}

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "aggro-floor-phase", 0x4b4b4b4b)
	defer closeSessionFlow(t, ownerFlow)
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("AggroFloorPhaseOwner")
	if !ok {
		t.Fatal("expected proximity death-floor phase_select suppress owner entity to remain registered")
	}
	prePhaseSelectEntityID := ownerEntity.Entity.ID
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, prePhaseSelectEntityID) {
		t.Fatalf("expected pending-frame proximity acquisition to engage owner for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("AggroFloorPhaseOwner"); ok {
		t.Fatalf("expected proximity-armed death-floor phase_select suppress path not to invent selected combat target ownership, got %+v", snapshot)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	floorQueued := flushServerFrames(t, ownerFlow)
	if len(floorQueued) != 3 {
		t.Fatalf("expected proximity-armed owner-floor retaliation to emit point-change, player dead, and target clear, got %d frames", len(floorQueued))
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, prePhaseSelectEntityID) {
		t.Fatalf("expected proximity-armed death floor to release engagement for entity %d", group.EntityID)
	}

	phaseSelectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/phase_select",
	})))
	if err != nil {
		t.Fatalf("unexpected /phase_select after proximity-armed death floor: %v", err)
	}
	if len(phaseSelectOut) == 0 {
		t.Fatal("expected /phase_select frames after proximity-armed death floor")
	}

	selectPhaseOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0})))
	if err != nil {
		t.Fatalf("unexpected character select after proximity-armed death-floor /phase_select: %v", err)
	}
	if len(selectPhaseOut) != 3 {
		t.Fatalf("expected 3 character-select frames after proximity-armed death-floor /phase_select, got %d", len(selectPhaseOut))
	}

	reenterOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected enter-game after proximity-armed death-floor /phase_select: %v", err)
	}
	if len(reenterOut) == 0 {
		t.Fatal("expected enter-game bootstrap frames after proximity-armed death-floor /phase_select")
	}
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok = runtime.sharedWorld.playerEntityByName("AggroFloorPhaseOwner")
	if !ok {
		t.Fatal("expected proximity death-floor phase_select suppress owner entity after re-entry")
	}
	postPhaseSelectEntityID := ownerEntity.Entity.ID
	if postPhaseSelectEntityID == 0 {
		t.Fatal("expected non-zero owner entity ID after /phase_select re-entry")
	}
	if postPhaseSelectEntityID == prePhaseSelectEntityID {
		t.Fatalf("expected /phase_select Leave/Join to allocate a new owner entity ID; still %d", postPhaseSelectEntityID)
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, postPhaseSelectEntityID) {
		t.Fatalf("expected zero-HP /phase_select re-entry not to reacquire proximity engagement for entity %d", group.EntityID)
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after proximity-armed death-floor /phase_select re-entry: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatal("expected /restart_here recovery frames after proximity-armed death-floor /phase_select re-entry")
	}
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok = runtime.sharedWorld.playerEntityByName("AggroFloorPhaseOwner")
	if !ok {
		t.Fatal("expected proximity death-floor phase_select suppress owner entity after /restart_here")
	}
	if ownerEntity.Entity.ID != postPhaseSelectEntityID {
		t.Fatalf("expected /restart_here to keep the post-/phase_select entity ID %d, got %d", postPhaseSelectEntityID, ownerEntity.Entity.ID)
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected still-inside owner to stay suppressed after death-floor /phase_select /restart_here for entity %d", group.EntityID)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		for _, raw := range queued {
			if _, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, raw)); err == nil {
				t.Fatalf("expected suppressed post-/phase_select /restart_here owner not to receive delayed retaliation POINT_CHANGE, got %#v", queued)
			}
		}
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected delayed flush not to re-lock suppressed still-inside owner after death-floor /phase_select /restart_here for entity %d", group.EntityID)
	}

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1950,
		Y:    2800,
		Time: 0x5152535b,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while leaving aggro radius after death-floor /phase_select /restart_here suppress: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after leaving aggro radius post-/phase_select /restart_here, got %d frames", len(moveOut))
	}
	_ = flushServerFrames(t, ownerFlow)

	moveIn, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1850,
		Y:    2800,
		Time: 0x5152535c,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while re-entering aggro radius after death-floor /phase_select /restart_here suppress: %v", err)
	}
	if len(moveIn) != 1 {
		t.Fatalf("expected 1 immediate self move ack after re-entering aggro radius post-/phase_select /restart_here, got %d frames", len(moveIn))
	}
	_ = flushServerFrames(t, ownerFlow)
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected leave/re-enter after death-floor /phase_select /restart_here suppress to reacquire engagement for entity %d", group.EntityID)
	}
}

func TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorReconnectRestartHere(t *testing.T) {
	// Owner death floor seeds proximity suppress by subject entity ID. Abrupt
	// disconnect Leave + fresh Join allocates a new entity ID, so the same still-inside
	// owner must keep suppress across that identity change through /restart_here until leave/re-enter.
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AggroFloorReconnectOwner", 0x0103019c, 0x0204019c, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "aggro-floor-reconnect", 0x4c4c4c4c, owner)
	if err := accounts.Save(accountstore.Account{Login: "aggro-floor-reconnect", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed proximity death-floor reconnect suppress owner account: %v", err)
	}
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	currentTime := time.Unix(1700002800, 0)

	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     400,
			VisibilitySectorSize: 200,
		},
		store,
		accounts,
		staticActorStore,
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
	)
	if err != nil {
		t.Fatalf("new game runtime for proximity death-floor suppress reconnect restart_here: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.aggro_floor_suppress_reconnect",
		Name:          "AggroFloorReconnectSuppressMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import proximity death-floor reconnect suppress spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.aggro_floor_suppress_reconnect")
	if !ok {
		t.Fatal("expected proximity death-floor reconnect suppress spawn group to resolve by ref")
	}

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "aggro-floor-reconnect", 0x4c4c4c4c)
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("AggroFloorReconnectOwner")
	if !ok {
		t.Fatal("expected proximity death-floor reconnect suppress owner entity to remain registered")
	}
	preReconnectEntityID := ownerEntity.Entity.ID
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, preReconnectEntityID) {
		t.Fatalf("expected pending-frame proximity acquisition to engage owner for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("AggroFloorReconnectOwner"); ok {
		t.Fatalf("expected proximity-armed death-floor reconnect suppress path not to invent selected combat target ownership, got %+v", snapshot)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	floorQueued := flushServerFrames(t, ownerFlow)
	if len(floorQueued) != 3 {
		t.Fatalf("expected proximity-armed owner-floor retaliation to emit point-change, player dead, and target clear, got %d frames", len(floorQueued))
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, preReconnectEntityID) {
		t.Fatalf("expected proximity-armed death floor to release engagement for entity %d", group.EntityID)
	}

	persisted, err := accounts.Load("aggro-floor-reconnect")
	if err != nil {
		t.Fatalf("load persisted account after proximity-armed death floor: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after proximity-armed death floor, got %+v", persisted)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected proximity-armed death floor to persist points[%d]=0 before reconnect, got %d", bootstrapPlayerPointValueIndex, persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	closeSessionFlow(t, ownerFlow)
	issuePeerTicket(t, store, "aggro-floor-reconnect", 0x4d4d4d4d, persisted.Characters[0])

	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "aggro-floor-reconnect", 0x4d4d4d4d)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) == 0 {
		t.Fatal("expected enter-game bootstrap frames after proximity-armed death-floor reconnect")
	}
	_ = flushServerFrames(t, reconnectFlow)

	ownerEntity, ok = runtime.sharedWorld.playerEntityByName("AggroFloorReconnectOwner")
	if !ok {
		t.Fatal("expected proximity death-floor reconnect suppress owner entity after re-entry")
	}
	postReconnectEntityID := ownerEntity.Entity.ID
	if postReconnectEntityID == 0 {
		t.Fatal("expected non-zero owner entity ID after reconnect re-entry")
	}
	if postReconnectEntityID == preReconnectEntityID {
		t.Fatalf("expected reconnect Leave/Join to allocate a new owner entity ID; still %d", postReconnectEntityID)
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, postReconnectEntityID) {
		t.Fatalf("expected zero-HP reconnect re-entry not to reacquire proximity engagement for entity %d", group.EntityID)
	}

	restartOut, err := reconnectFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after proximity-armed death-floor reconnect re-entry: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatal("expected /restart_here recovery frames after proximity-armed death-floor reconnect re-entry")
	}
	_ = flushServerFrames(t, reconnectFlow)

	ownerEntity, ok = runtime.sharedWorld.playerEntityByName("AggroFloorReconnectOwner")
	if !ok {
		t.Fatal("expected proximity death-floor reconnect suppress owner entity after /restart_here")
	}
	if ownerEntity.Entity.ID != postReconnectEntityID {
		t.Fatalf("expected /restart_here to keep the post-reconnect entity ID %d, got %d", postReconnectEntityID, ownerEntity.Entity.ID)
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected still-inside owner to stay suppressed after death-floor reconnect /restart_here for entity %d", group.EntityID)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, reconnectFlow); len(queued) != 0 {
		for _, raw := range queued {
			if _, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, raw)); err == nil {
				t.Fatalf("expected suppressed post-reconnect /restart_here owner not to receive delayed retaliation POINT_CHANGE, got %#v", queued)
			}
		}
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected delayed flush not to re-lock suppressed still-inside owner after death-floor reconnect /restart_here for entity %d", group.EntityID)
	}

	moveOut, err := reconnectFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1950,
		Y:    2800,
		Time: 0x5152535d,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while leaving aggro radius after death-floor reconnect /restart_here suppress: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after leaving aggro radius post-reconnect /restart_here, got %d frames", len(moveOut))
	}
	_ = flushServerFrames(t, reconnectFlow)

	moveIn, err := reconnectFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1850,
		Y:    2800,
		Time: 0x5152535e,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while re-entering aggro radius after death-floor reconnect /restart_here suppress: %v", err)
	}
	if len(moveIn) != 1 {
		t.Fatalf("expected 1 immediate self move ack after re-entering aggro radius post-reconnect /restart_here, got %d frames", len(moveIn))
	}
	_ = flushServerFrames(t, reconnectFlow)
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected leave/re-enter after death-floor reconnect /restart_here suppress to reacquire engagement for entity %d", group.EntityID)
	}
}

func TestSharedWorldRegistrySubjectReleaseSeedsProximitySuppressWhenOwnerAlreadyAtHPFloor(t *testing.T) {
	// Death-floor subject release must seed leave/re-enter suppress for the releasing
	// owner even when that owner's shared-world snapshot is already at the bootstrap
	// 0-HP floor. seedProximity alone skips floor candidates, so subject clear must
	// still mark the releasing subject explicitly before /restart_here recovers HP.
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)

	owner := peerVisibilityCharacter("AggroFloorSeedOwner", 0x0103019d, 0x0204019d, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	ownerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	if ownerID == 0 {
		t.Fatal("expected owner to join shared world before floor-seed suppress proof")
	}
	ownerPending.flush()

	actor, ok := registry.registerStaticActor(0, "AggroFloorSeedMob", 42, 1700, 2800, 20350, "", "", worldruntime.StaticActorCombatProfilePracticeMob, "practice.aggro_floor_seed_suppress", worldruntime.StaticActorDeathReward{})
	if !ok {
		t.Fatal("expected spawn-backed practice mob registration to succeed before floor-seed suppress proof")
	}
	ownerPending.flush()

	registry.mu.Lock()
	registry.setStaticActorCombatEngagementLocked(actor.EntityID, ownerID)
	registry.mu.Unlock()
	if !registry.StaticActorCombatEngagedBySubject(actor.EntityID, ownerID) {
		t.Fatalf("expected engagement to be recorded before floor-seed suppress proof for entity %d", actor.EntityID)
	}

	floored := owner
	floored.Points[bootstrapPlayerPointValueIndex] = 0
	registry.UpdateCharacter(ownerID, floored)

	registry.ClearStaticActorCombatEngagementsBySubject(ownerID)
	if registry.StaticActorCombatEngagedBySubject(actor.EntityID, ownerID) {
		t.Fatalf("expected subject release to clear engagement for entity %d", actor.EntityID)
	}

	recovered := floored
	recovered.Points[bootstrapPlayerPointValueIndex] = 800
	registry.UpdateCharacter(ownerID, recovered)

	if acquired := registry.AcquireProximitySpawnGroupAggro(); len(acquired) != 0 {
		t.Fatalf("expected still-inside recovered owner to stay proximity-suppressed after floor-HP subject release, got acquired=%v", acquired)
	}
	if registry.StaticActorCombatEngagedBySubject(actor.EntityID, ownerID) {
		t.Fatalf("expected suppressed recovered owner not to reacquire engagement for entity %d", actor.EntityID)
	}

	outside := recovered
	outside.X = 1950
	registry.UpdateCharacter(ownerID, outside)
	_ = registry.AcquireProximitySpawnGroupAggro()

	inside := outside
	inside.X = 1850
	registry.UpdateCharacter(ownerID, inside)
	acquired := registry.AcquireProximitySpawnGroupAggro()
	if len(acquired) != 1 || acquired[0] != actor.EntityID {
		t.Fatalf("expected leave/re-enter after floor-HP subject-release suppress to reacquire entity %d, got %v", actor.EntityID, acquired)
	}
	if !registry.StaticActorCombatEngagedBySubject(actor.EntityID, ownerID) {
		t.Fatalf("expected leave/re-enter to restore engagement for entity %d", actor.EntityID)
	}
}

func TestGameRuntimeProximityAggroSuppressRemapsAcrossContentBundleReplacement(t *testing.T) {
	// After in-radius engagement release seeds proximity suppress, a non-identical
	// content-bundle replacement that keeps the same authored spawn_group_ref must
	// remap that suppress onto the newly registered actor. Still-inside owners must
	// not instantly reacquire until an explicit leave/re-enter of the effective
	// aggro radius. Engagement / selected-target ownership stay fail-closed.
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AggroSuppressReplaceOwner", 0x0103019c, 0x0204019c, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "aggro-suppress-replace-owner", 0x4c4c4c4c, owner)
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	currentTime := time.Unix(1700002800, 0)

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
		t.Fatalf("new game runtime for proximity suppress content-bundle remapping: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.aggro_suppress_replace",
		Name:          "AggroSuppressReplaceMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import initial proximity suppress replacement spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.aggro_suppress_replace")
	if !ok {
		t.Fatal("expected proximity suppress replacement spawn group to resolve by ref")
	}
	originalEntityID := group.EntityID

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "aggro-suppress-replace-owner", 0x4c4c4c4c)
	defer closeSessionFlow(t, ownerFlow)
	_ = flushServerFrames(t, ownerFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("AggroSuppressReplaceOwner")
	if !ok {
		t.Fatal("expected proximity suppress replacement owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(originalEntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected pending-frame proximity acquisition to engage owner for entity %d", originalEntityID)
	}

	clearOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: 0})))
	if err != nil {
		t.Fatalf("unexpected owner TARGET(0) clear before content-bundle replacement: %v", err)
	}
	if len(clearOut) != 0 {
		t.Fatalf("expected proximity-only TARGET(0) clear to emit no frames, got %d", len(clearOut))
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(originalEntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected in-radius TARGET(0) clear to release engaged_by for entity %d", originalEntityID)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected suppress to keep acquisition silent before replacement, got %d queued frames", len(queued))
	}

	replacementBundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.aggro_suppress_replace",
		Name:          "AggroSuppressReplaceMobRenamed",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}
	if _, err := runtime.ImportContentBundle(replacementBundle); err != nil {
		t.Fatalf("import non-identical proximity suppress replacement spawn-group bundle: %v", err)
	}
	afterReplace, ok := runtime.SpawnGroupByRef("practice.aggro_suppress_replace")
	if !ok {
		t.Fatal("expected proximity suppress spawn group to remain resolvable by authored ref after replacement")
	}
	if afterReplace.Name != "AggroSuppressReplaceMobRenamed" {
		t.Fatalf("expected replacement presentation name to apply, got %+v", afterReplace)
	}
	if afterReplace.EntityID == originalEntityID {
		t.Fatalf("expected content-bundle replacement to allocate a new entity id, got %d", afterReplace.EntityID)
	}
	if len(runtime.sharedWorld.CombatTargetSnapshots()) != 0 {
		t.Fatalf("expected engagement/selected-target ownership to stay fail-closed across replacement, got %+v", runtime.sharedWorld.CombatTargetSnapshots())
	}

	_ = flushServerFrames(t, ownerFlow)
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(afterReplace.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected remapped proximity suppress to block still-inside reacquire after content-bundle replacement for entity %d", afterReplace.EntityID)
	}
	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected delayed flush to keep remapped suppress silent after replacement, got %d queued frames", len(queued))
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(afterReplace.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected delayed flush not to re-lock remapped-suppressed still-inside owner for entity %d", afterReplace.EntityID)
	}

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1950,
		Y:    2800,
		Time: 0x5152535b,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while leaving aggro radius after replacement suppress: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after leaving aggro radius post-replacement, got %d frames", len(moveOut))
	}
	_ = flushServerFrames(t, ownerFlow)

	moveIn, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1850,
		Y:    2800,
		Time: 0x5152535c,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while re-entering aggro radius after replacement suppress: %v", err)
	}
	if len(moveIn) != 1 {
		t.Fatalf("expected 1 immediate self move ack after re-entering aggro radius post-replacement, got %d frames", len(moveIn))
	}
	_ = flushServerFrames(t, ownerFlow)
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(afterReplace.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected leave/re-enter after remapped replacement suppress to reacquire engagement for entity %d", afterReplace.EntityID)
	}
}
