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
