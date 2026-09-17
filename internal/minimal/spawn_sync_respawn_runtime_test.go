package minimal

import (
	"errors"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestContentBundleSyncRespawnPackPrefixesStayAuthoringOnly(t *testing.T) {
	authored := contentbundle.Bundle{RegenSpawns: []contentbundle.RegenSpawn{{
		Ref:         "practice.sync_respawn_pack",
		Name:        "SyncRespawnPack",
		MapIndex:    42,
		X:           1700,
		Y:           2800,
		RaceNum:     20350,
		Count:       2,
		PackSpacing: 100,
		SyncRespawn: true,
	}}}
	prefixes := contentbundle.SyncRespawnPackPrefixes(authored)
	if _, ok := prefixes["practice.sync_respawn_pack"]; !ok || len(prefixes) != 1 {
		t.Fatalf("expected authored sync_respawn overlay for practice.sync_respawn_pack, got %#v", prefixes)
	}
	canonical, err := contentbundle.Canonicalize(authored)
	if err != nil {
		t.Fatalf("canonicalize opt-in sync_respawn pack: %v", err)
	}
	if len(canonical.RegenSpawns) != 0 {
		t.Fatalf("expected regen_spawns to stay authoring-only after canonicalize, got %+v", canonical.RegenSpawns)
	}
	if got := contentbundle.SyncRespawnPackPrefixes(canonical); len(got) != 0 {
		t.Fatalf("expected canonical bundle to drop sync_respawn overlay, got %#v", got)
	}
	if len(canonical.SpawnGroups) != 2 ||
		canonical.SpawnGroups[0].Ref != "practice.sync_respawn_pack.m01" ||
		canonical.SpawnGroups[1].Ref != "practice.sync_respawn_pack.m02" {
		t.Fatalf("expected independent {ref}.mNN members after canonicalize, got %+v", canonical.SpawnGroups)
	}
}

func TestContentBundleRejectsOneCountRegenSyncRespawn(t *testing.T) {
	_, err := contentbundle.Canonicalize(contentbundle.Bundle{RegenSpawns: []contentbundle.RegenSpawn{{
		Ref:         "practice.one_count_sync",
		Name:        "OneCountSync",
		MapIndex:    42,
		X:           1700,
		Y:           2800,
		RaceNum:     20350,
		Count:       1,
		SyncRespawn: true,
	}}})
	if !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for one-count regen sync_respawn, got %v", err)
	}
}

// Opt-in sync_respawn keeps a live same-prefix sibling on its own clock. Only
// already-dead members share one respawn instant.
func TestGameRuntimeOptInPackSyncRespawnKeepsLiveSiblingIndependent(t *testing.T) {
	runtime, currentTime := newSyncRespawnRuntime(t)
	importSyncRespawnPack(t, runtime, "practice.sync_respawn_live", "qa_sync_respawn_live", true)

	first, second, _ := syncRespawnPackMembers(t, runtime, "practice.sync_respawn_live")
	flow := enterSyncRespawnOwner(t, runtime)
	defer closeSessionFlow(t, flow)

	killVisibleSpawnGroup(t, flow, first)
	assertSpawnDead(t, runtime, first.SpawnGroupRef, true)
	assertSpawnDead(t, runtime, second.SpawnGroupRef, false)

	firstRespawn, ok := runtime.StaticActorRespawn(first.EntityID)
	if !ok || firstRespawn.ReadyAt.IsZero() {
		t.Fatalf("expected pending respawn for dead pack member .m01, ok=%v snapshot=%+v", ok, firstRespawn)
	}
	if _, waiting := runtime.StaticActorRespawn(second.EntityID); waiting {
		t.Fatal("expected live sibling .m02 not to wait on the dead member's respawn clock")
	}

	*currentTime = firstRespawn.ReadyAt
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, first.SpawnGroupRef, false)
	liveSibling, ok := runtime.SpawnGroupByRef(second.SpawnGroupRef)
	if !ok || liveSibling.Dead || liveSibling.X != 2100 || liveSibling.Y != 2800 {
		t.Fatalf("expected live sibling to stay at authored placement after independent member respawn, ok=%v snapshot=%+v", ok, liveSibling)
	}
	if respawns := runtime.StaticActorRespawns(); len(respawns) != 0 {
		t.Fatalf("expected no pending respawn after independent member rebuild, got %+v", respawns)
	}
}

// When two or more opted-in same-prefix members are already dead they share the
// latest pending ReadyAt and rebuild together. One-count refs stay unlinked.
func TestGameRuntimeOptInPackSyncRespawnSharesReadyAtForAlreadyDeadMembers(t *testing.T) {
	runtime, currentTime := newSyncRespawnRuntime(t)
	worldruntime.UnregisterStaticActorCombatProfileForTest("qa_sync_respawn_share")
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest("qa_sync_respawn_share") })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           "practice.sync_respawn_share",
			Name:          "SyncRespawnPack",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_sync_respawn_share",
			Count:         2,
			PackSpacing:   400,
			SyncRespawn:   true,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.sync_respawn_other",
			Name:          "SyncRespawnOther",
			MapIndex:      42,
			X:             1600,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_sync_respawn_share",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{syncRespawnOneHitProfile("qa_sync_respawn_share")},
	}); err != nil {
		t.Fatalf("import opt-in pack plus independent one-count: %v", err)
	}

	first, second, other := syncRespawnPackMembers(t, runtime, "practice.sync_respawn_share")
	flow := enterSyncRespawnOwner(t, runtime)
	defer closeSessionFlow(t, flow)

	killVisibleSpawnGroup(t, flow, first)
	firstDeathAt := *currentTime
	*currentTime = currentTime.Add(time.Second)
	killVisibleSpawnGroup(t, flow, second)
	secondDeathAt := *currentTime

	firstRespawn, ok := runtime.StaticActorRespawn(first.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for dead pack member .m01 after sibling death")
	}
	secondRespawn, ok := runtime.StaticActorRespawn(second.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for dead pack member .m02")
	}
	wantReadyAt := secondDeathAt.Add(2 * time.Second)
	if !firstRespawn.ReadyAt.Equal(secondRespawn.ReadyAt) || !firstRespawn.ReadyAt.Equal(wantReadyAt) {
		t.Fatalf("expected already-dead opted-in members to share latest ReadyAt %s, got .m01=%s .m02=%s firstDeath=%s", wantReadyAt, firstRespawn.ReadyAt, secondRespawn.ReadyAt, firstDeathAt)
	}

	*currentTime = currentTime.Add(time.Second)
	killVisibleSpawnGroup(t, flow, other)
	otherRespawn, ok := runtime.StaticActorRespawn(other.EntityID)
	if !ok || other.EntityID == 0 {
		t.Fatalf("expected independent one-count ref to keep its own respawn clock, ok=%v other=%+v", ok, other)
	}
	if otherRespawn.ReadyAt.Equal(wantReadyAt) {
		t.Fatalf("expected one-count ref not to share the pack ReadyAt %s, got %s", wantReadyAt, otherRespawn.ReadyAt)
	}

	*currentTime = wantReadyAt.Add(-time.Millisecond)
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, first.SpawnGroupRef, true)
	assertSpawnDead(t, runtime, second.SpawnGroupRef, true)
	assertSpawnDead(t, runtime, other.SpawnGroupRef, true)

	*currentTime = wantReadyAt
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, first.SpawnGroupRef, false)
	assertSpawnDead(t, runtime, second.SpawnGroupRef, false)
	assertSpawnDead(t, runtime, other.SpawnGroupRef, true)
	respawnedFirst, _ := runtime.SpawnGroupByRef(first.SpawnGroupRef)
	respawnedSecond, _ := runtime.SpawnGroupByRef(second.SpawnGroupRef)
	if respawnedFirst.X != 1700 || respawnedFirst.Y != 2800 || respawnedSecond.X != 2100 || respawnedSecond.Y != 2800 {
		t.Fatalf("expected synchronized rebuild at authored homes, first=%+v second=%+v", respawnedFirst, respawnedSecond)
	}
}

func TestGameRuntimeDefaultPackRespawnStaysIndependentPerMember(t *testing.T) {
	runtime, currentTime := newSyncRespawnRuntime(t)
	importSyncRespawnPack(t, runtime, "practice.default_pack_respawn", "qa_sync_respawn_default", false)

	first, second, _ := syncRespawnPackMembers(t, runtime, "practice.default_pack_respawn")
	flow := enterSyncRespawnOwner(t, runtime)
	defer closeSessionFlow(t, flow)

	killVisibleSpawnGroup(t, flow, first)
	firstDeathAt := *currentTime
	*currentTime = currentTime.Add(time.Second)
	killVisibleSpawnGroup(t, flow, second)

	firstRespawn, ok := runtime.StaticActorRespawn(first.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for default pack member .m01")
	}
	secondRespawn, ok := runtime.StaticActorRespawn(second.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for default pack member .m02")
	}
	if firstRespawn.ReadyAt.Equal(secondRespawn.ReadyAt) {
		t.Fatalf("expected default pack members to keep independent ReadyAt, both %s", firstRespawn.ReadyAt)
	}
	if !firstRespawn.ReadyAt.Equal(firstDeathAt.Add(2 * time.Second)) {
		t.Fatalf("expected .m01 ReadyAt %s, got %s", firstDeathAt.Add(2*time.Second), firstRespawn.ReadyAt)
	}

	*currentTime = firstRespawn.ReadyAt
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, first.SpawnGroupRef, false)
	assertSpawnDead(t, runtime, second.SpawnGroupRef, true)

	*currentTime = secondRespawn.ReadyAt
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, second.SpawnGroupRef, false)
}

func newSyncRespawnRuntime(t *testing.T) (*gameRuntime, *time.Time) {
	t.Helper()
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SyncRespawnOwner", 0x01030521, 0x02040521, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "sync-respawn-owner", 0xf1f1f1f1, owner)

	currentTime := time.Unix(1700004500, 0)
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
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	return runtime, &currentTime
}

func syncRespawnOneHitProfile(name string) worldruntime.StaticActorCombatProfileSnapshot {
	return worldruntime.StaticActorCombatProfileSnapshot{
		Profile:               name,
		MaxHP:                 1,
		DamagePerNormalAttack: 1,
		AttackValue:           1,
		DefenseValue:          0,
		Level:                 worldruntime.TrainingDummyBootstrapLevel,
		Rank:                  worldruntime.TrainingDummyBootstrapRank,
		RespawnDelayMs:        worldruntime.PracticeMobBootstrapRespawnDelay.Milliseconds(),
		RetaliationPointDelta: worldruntime.PracticeMobBootstrapRetaliationPointDelta,
	}
}

func importSyncRespawnPack(t *testing.T, runtime *gameRuntime, packRef string, profile string, syncRespawn bool) {
	t.Helper()
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           packRef,
			Name:          "SyncRespawnPack",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: profile,
			Count:         2,
			PackSpacing:   400,
			SyncRespawn:   syncRespawn,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{syncRespawnOneHitProfile(profile)},
	}); err != nil {
		t.Fatalf("import sync-respawn regen bundle %s: %v", packRef, err)
	}
}

func syncRespawnPackMembers(t *testing.T, runtime *gameRuntime, packRef string) (StaticActorSnapshot, StaticActorSnapshot, StaticActorSnapshot) {
	t.Helper()
	first, ok := runtime.SpawnGroupByRef(packRef + ".m01")
	if !ok || first.Dead || first.X != 1700 || first.Y != 2800 {
		t.Fatalf("expected live pack member .m01 at authored origin, ok=%v snapshot=%+v", ok, first)
	}
	second, ok := runtime.SpawnGroupByRef(packRef + ".m02")
	if !ok || second.Dead || second.X != 2100 || second.Y != 2800 {
		t.Fatalf("expected live pack member .m02 at +pack_spacing X, ok=%v snapshot=%+v", ok, second)
	}
	other, ok := runtime.SpawnGroupByRef("practice.sync_respawn_other")
	if !ok {
		return first, second, StaticActorSnapshot{}
	}
	if other.Dead || other.X != 1600 || other.Y != 2800 {
		t.Fatalf("expected live independent one-count spawn group, ok=%v snapshot=%+v", ok, other)
	}
	return first, second, other
}

func enterSyncRespawnOwner(t *testing.T, runtime *gameRuntime) service.SessionFlow {
	t.Helper()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "sync-respawn-owner", 0xf1f1f1f1)
	flushServerFrames(t, flow)
	return flow
}

func killVisibleSpawnGroup(t *testing.T, flow service.SessionFlow, actor StaticActorSnapshot) {
	t.Helper()
	vid := uint32(actor.EntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: vid})))
	if err != nil {
		t.Fatalf("unexpected target error before kill %s: %v", actor.SpawnGroupRef, err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select %s, got %d frames", actor.SpawnGroupRef, len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  vid,
	})))
	if err != nil {
		t.Fatalf("unexpected killing hit on %s: %v", actor.SpawnGroupRef, err)
	}
	if len(attackOut) == 0 {
		t.Fatalf("expected killing hit on %s to emit death frames", actor.SpawnGroupRef)
	}
}

func assertSpawnDead(t *testing.T, runtime *gameRuntime, ref string, wantDead bool) {
	t.Helper()
	snapshot, ok := runtime.SpawnGroupByRef(ref)
	if !ok || snapshot.Dead != wantDead {
		t.Fatalf("expected %s dead=%v, ok=%v snapshot=%+v", ref, wantDead, ok, snapshot)
	}
}
