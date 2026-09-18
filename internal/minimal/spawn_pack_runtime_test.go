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

func TestSpawnGroupPackMemberPrefix(t *testing.T) {
	cases := []struct {
		ref    string
		want   string
		wantOK bool
	}{
		{ref: "practice.pack_assist_mob.m01", want: "practice.pack_assist_mob", wantOK: true},
		{ref: "practice.pack_assist_mob.m08", want: "practice.pack_assist_mob", wantOK: true},
		{ref: "practice.pack_assist_mob.m00", wantOK: false},
		{ref: "practice.pack_assist_mob.m09", wantOK: false},
		{ref: "practice.pack_assist_mob.m1", wantOK: false},
		{ref: "practice.pack_assist_mob", wantOK: false},
		{ref: "practice.mob_alpha", wantOK: false},
		{ref: "", wantOK: false},
	}
	for _, tc := range cases {
		got, ok := spawnGroupPackMemberPrefix(tc.ref)
		if ok != tc.wantOK || got != tc.want {
			t.Fatalf("spawnGroupPackMemberPrefix(%q) = %q, %v; want %q, %v", tc.ref, got, ok, tc.want, tc.wantOK)
		}
	}
}

// First accepted live hit on one multi-count regen member copies engaged_by onto
// the other live same-prefix sibling without MOVE, chase arming, or a pack object.
func TestGameRuntimePackMemberAssistCopiesOwnerLockWithoutMove(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Owner sits 150 east of .m01 (inside DefaultSpawnAggroRadius 200 and combat
	// range 300) and 250 west of .m02 (outside aggro, still inside combat range).
	owner := peerVisibilityCharacter("PackAssistOwner", 0x01030421, 0x02040421, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "pack-assist-owner", 0xe1e1e1e1, owner)
	watcher := peerVisibilityCharacter("PackAssistWatcher", 0x01030422, 0x02040422, 1850, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "pack-assist-watcher", 0xe2e2e2e2, watcher)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700004400, 0)
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
		interactionStore,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           "practice.pack_assist_mob",
			Name:          "PackAssistMob",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
			Count:         2,
			PackSpacing:   400,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.pack_assist_other",
			Name:          "PackAssistOther",
			MapIndex:      42,
			X:             1600,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
		}},
	}); err != nil {
		t.Fatalf("import pack-assist regen bundle: %v", err)
	}

	hitMember, ok := runtime.SpawnGroupByRef("practice.pack_assist_mob.m01")
	if !ok || hitMember.X != 1700 || hitMember.Y != 2800 || hitMember.Dead {
		t.Fatalf("expected live pack member .m01 at authored origin, ok=%v snapshot=%+v", ok, hitMember)
	}
	sibling, ok := runtime.SpawnGroupByRef("practice.pack_assist_mob.m02")
	if !ok || sibling.X != 2100 || sibling.Y != 2800 || sibling.Dead {
		t.Fatalf("expected live pack member .m02 at +pack_spacing X, ok=%v snapshot=%+v", ok, sibling)
	}
	other, ok := runtime.SpawnGroupByRef("practice.pack_assist_other")
	if !ok || other.X != 1600 || other.Dead {
		t.Fatalf("expected independent one-count spawn group to stay at authored origin, ok=%v snapshot=%+v", ok, other)
	}

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pack-assist-owner", 0xe1e1e1e1)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	hitVID := uint32(hitMember.EntityID)
	siblingVID := uint32(sibling.EntityID)
	otherVID := uint32(other.EntityID)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: hitVID})))
	if err != nil {
		t.Fatalf("unexpected owner target error before pack-assist hit: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select pack member .m01, got %d frames", len(selectOut))
	}
	drainAcceptedTargetCreateNewIfQueued(t, ownerFlow)
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  hitVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted pack-assist hit: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, immediate retaliation, and damage-info on first pack-assist hit, got %d frames", len(attackOut))
	}

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("PackAssistOwner")
	if !ok {
		t.Fatal("expected pack-assist owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(hitMember.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected accepted hit to keep engaged_by on pack member .m01 entity %d", hitMember.EntityID)
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(sibling.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected pack assist to copy engaged_by onto live sibling .m02 entity %d", sibling.EntityID)
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(other.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected independent one-count spawn group entity %d to stay unengaged", other.EntityID)
	}

	runtime.spawnChaseMu.Lock()
	_, hitChaseScheduled := runtime.spawnChaseStepDueAt[hitMember.EntityID]
	_, siblingChaseScheduled := runtime.spawnChaseStepDueAt[sibling.EntityID]
	_, otherChaseScheduled := runtime.spawnChaseStepDueAt[other.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !hitChaseScheduled {
		t.Fatalf("expected accepted hit to arm chase on the selected pack member entity %d", hitMember.EntityID)
	}
	if siblingChaseScheduled {
		t.Fatalf("expected pack assist not to arm chase/MOVE on sibling entity %d", sibling.EntityID)
	}
	if otherChaseScheduled {
		t.Fatalf("expected pack assist not to arm chase on independent spawn group entity %d", other.EntityID)
	}

	liveSibling, ok := runtime.SpawnGroupByRef("practice.pack_assist_mob.m02")
	if !ok || liveSibling.Dead || liveSibling.X != 2100 || liveSibling.Y != 2800 {
		t.Fatalf("expected assisted sibling to stay at authored placement without MOVE, ok=%v snapshot=%+v", ok, liveSibling)
	}
	liveOther, ok := runtime.SpawnGroupByRef("practice.pack_assist_other")
	if !ok || liveOther.Dead || liveOther.X != 1600 || liveOther.Y != 2800 {
		t.Fatalf("expected independent spawn group to stay unmoved, ok=%v snapshot=%+v", ok, liveOther)
	}

	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		for _, raw := range queued {
			if move, err := movep.DecodeMoveAck(decodeSingleFrame(t, raw)); err == nil && move.VID == siblingVID {
				t.Fatalf("expected pack assist not to emit sibling MOVE, got %+v among %d queued frames", move, len(queued))
			}
		}
		t.Fatalf("expected pack-assist hit to stay silent before chase/retaliation due times, got %d queued frames", len(queued))
	}

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pack-assist-watcher", 0xe2e2e2e2)
	defer closeSessionFlow(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, watcherFlow)

	watcherBlocked, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: siblingVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target error against assisted sibling: %v", err)
	}
	if len(watcherBlocked) != 0 {
		t.Fatalf("expected third-party TARGET against assisted sibling to fail closed, got %d frames", len(watcherBlocked))
	}
	drainAcceptedTargetCreateNewIfQueued(t, watcherFlow)
	watcherOther, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: otherVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target error against independent spawn group: %v", err)
	}
	if len(watcherOther) != 1 {
		t.Fatalf("expected third-party TARGET against unengaged independent spawn group to succeed, got %d frames", len(watcherOther))
	}
	drainAcceptedTargetCreateNewIfQueued(t, watcherFlow)
}
