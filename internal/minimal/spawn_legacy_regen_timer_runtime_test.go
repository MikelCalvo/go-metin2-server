package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestContentBundleRegenTimerOverlayStaysAuthoringOnly(t *testing.T) {
	authored := contentbundle.Bundle{RegenSpawns: []contentbundle.RegenSpawn{{
		Ref:            "practice.legacy_regen_timer",
		Name:           "LegacyRegenTimer",
		MapIndex:       42,
		X:              1700,
		Y:              2800,
		RaceNum:        20350,
		Count:          1,
		RespawnDelayMs: 4000,
	}}}
	overlay := contentbundle.RegenRespawnDelayMsBySpawnGroupRef(authored)
	if overlay["practice.legacy_regen_timer"] != 4000 || len(overlay) != 1 {
		t.Fatalf("expected authored overlay on practice.legacy_regen_timer, got %#v", overlay)
	}
	canonical, err := contentbundle.Canonicalize(authored)
	if err != nil {
		t.Fatalf("canonicalize opt-in one-count regen timer: %v", err)
	}
	if len(canonical.RegenSpawns) != 0 {
		t.Fatalf("expected regen_spawns to stay authoring-only after canonicalize, got %+v", canonical.RegenSpawns)
	}
	if got := contentbundle.RegenRespawnDelayMsBySpawnGroupRef(canonical); len(got) != 0 {
		t.Fatalf("expected canonical bundle to drop regen timer overlay, got %#v", got)
	}
}

// Opt-in regen_spawns time / regen_time_ms / respawn_delay_ms overrides ReadyAt
// for that expanded spawn. One-count keeps the authored ref. Default rows and
// spawn_groups without the overlay stay on the combat-profile clock.
func TestGameRuntimeOptInRegenTimerOverridesReadyAtForOneCount(t *testing.T) {
	runtime, currentTime := newLegacyRegenTimerRuntime(t)
	worldruntime.UnregisterStaticActorCombatProfileForTest("qa_legacy_regen_timer")
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest("qa_legacy_regen_timer") })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:            "practice.legacy_regen_timer",
			Name:           "LegacyRegenTimer",
			MapIndex:       42,
			X:              1700,
			Y:              2800,
			RaceNum:        20350,
			CombatProfile:  "qa_legacy_regen_timer",
			Count:          1,
			RespawnDelayMs: 4000,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.legacy_regen_other",
			Name:          "LegacyRegenOther",
			MapIndex:      42,
			X:             1600,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_legacy_regen_timer",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{legacyRegenTimerOneHitProfile("qa_legacy_regen_timer")},
	}); err != nil {
		t.Fatalf("import opt-in one-count regen timer plus independent spawn group: %v", err)
	}

	actor, ok := runtime.SpawnGroupByRef("practice.legacy_regen_timer")
	if !ok || actor.Dead || actor.X != 1700 || actor.Y != 2800 {
		t.Fatalf("expected live one-count overlay spawn at authored origin, ok=%v snapshot=%+v", ok, actor)
	}
	other, ok := runtime.SpawnGroupByRef("practice.legacy_regen_other")
	if !ok || other.Dead || other.X != 1600 || other.Y != 2800 {
		t.Fatalf("expected live independent spawn group, ok=%v snapshot=%+v", ok, other)
	}

	flow := enterLegacyRegenTimerOwner(t, runtime)
	defer closeSessionFlow(t, flow)

	killVisibleSpawnGroup(t, flow, actor)
	deathAt := *currentTime
	overlayRespawn, ok := runtime.StaticActorRespawn(actor.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for opted-in one-count regen timer")
	}
	wantReadyAt := deathAt.Add(4 * time.Second)
	if !overlayRespawn.ReadyAt.Equal(wantReadyAt) {
		t.Fatalf("expected overlay ReadyAt %s, got %s", wantReadyAt, overlayRespawn.ReadyAt)
	}

	*currentTime = deathAt.Add(time.Second)
	killVisibleSpawnGroup(t, flow, other)
	otherRespawn, ok := runtime.StaticActorRespawn(other.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for spawn_groups without overlay")
	}
	wantOtherReadyAt := (*currentTime).Add(2 * time.Second)
	if !otherRespawn.ReadyAt.Equal(wantOtherReadyAt) {
		t.Fatalf("expected spawn_groups without overlay to keep profile ReadyAt %s, got %s", wantOtherReadyAt, otherRespawn.ReadyAt)
	}

	*currentTime = wantOtherReadyAt.Add(-time.Millisecond)
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, actor.SpawnGroupRef, true)
	assertSpawnDead(t, runtime, other.SpawnGroupRef, true)

	*currentTime = wantOtherReadyAt
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, actor.SpawnGroupRef, true)
	assertSpawnDead(t, runtime, other.SpawnGroupRef, false)

	*currentTime = wantReadyAt.Add(-time.Millisecond)
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, actor.SpawnGroupRef, true)

	*currentTime = wantReadyAt
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, actor.SpawnGroupRef, false)
	respawned, _ := runtime.SpawnGroupByRef(actor.SpawnGroupRef)
	if respawned.X != 1700 || respawned.Y != 2800 {
		t.Fatalf("expected overlay rebuild at authored home, got %+v", respawned)
	}
}

// Multi-count copies the overlay onto every {ref}.mNN member while keeping
// independent ReadyAt clocks. Direct spawn_groups stay on the profile delay.
func TestGameRuntimeOptInRegenTimerCopiesOntoEveryPackMember(t *testing.T) {
	runtime, currentTime := newLegacyRegenTimerRuntime(t)
	worldruntime.UnregisterStaticActorCombatProfileForTest("qa_legacy_regen_timer_pack")
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest("qa_legacy_regen_timer_pack") })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           "practice.legacy_regen_timer_pack",
			Name:          "LegacyRegenTimerPack",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_legacy_regen_timer_pack",
			Count:         2,
			PackSpacing:   400,
			Time:          4000,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{legacyRegenTimerOneHitProfile("qa_legacy_regen_timer_pack")},
	}); err != nil {
		t.Fatalf("import opt-in multi-count regen timer: %v", err)
	}

	first, ok := runtime.SpawnGroupByRef("practice.legacy_regen_timer_pack.m01")
	if !ok || first.Dead || first.X != 1700 || first.Y != 2800 {
		t.Fatalf("expected live pack member .m01 at authored origin, ok=%v snapshot=%+v", ok, first)
	}
	second, ok := runtime.SpawnGroupByRef("practice.legacy_regen_timer_pack.m02")
	if !ok || second.Dead || second.X != 2100 || second.Y != 2800 {
		t.Fatalf("expected live pack member .m02 at +pack_spacing X, ok=%v snapshot=%+v", ok, second)
	}

	flow := enterLegacyRegenTimerOwner(t, runtime)
	defer closeSessionFlow(t, flow)

	killVisibleSpawnGroup(t, flow, first)
	firstDeathAt := *currentTime
	*currentTime = currentTime.Add(time.Second)
	killVisibleSpawnGroup(t, flow, second)

	firstRespawn, ok := runtime.StaticActorRespawn(first.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for overlay pack member .m01")
	}
	secondRespawn, ok := runtime.StaticActorRespawn(second.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for overlay pack member .m02")
	}
	if firstRespawn.ReadyAt.Equal(secondRespawn.ReadyAt) {
		t.Fatalf("expected independent overlay ReadyAt per member, both %s", firstRespawn.ReadyAt)
	}
	if !firstRespawn.ReadyAt.Equal(firstDeathAt.Add(4 * time.Second)) {
		t.Fatalf("expected .m01 overlay ReadyAt %s, got %s", firstDeathAt.Add(4*time.Second), firstRespawn.ReadyAt)
	}
	if !secondRespawn.ReadyAt.Equal(currentTime.Add(4 * time.Second)) {
		t.Fatalf("expected .m02 overlay ReadyAt %s, got %s", currentTime.Add(4*time.Second), secondRespawn.ReadyAt)
	}

	*currentTime = firstRespawn.ReadyAt
	_ = flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, first.SpawnGroupRef, false)
	assertSpawnDead(t, runtime, second.SpawnGroupRef, true)
	respawnedFirst, _ := runtime.SpawnGroupByRef(first.SpawnGroupRef)
	if respawnedFirst.X != 1700 || respawnedFirst.Y != 2800 {
		t.Fatalf("expected .m01 rebuild at authored home, got %+v", respawnedFirst)
	}
}

func TestGameRuntimeDefaultRegenAndSpawnGroupsStayOnProfileClock(t *testing.T) {
	runtime, currentTime := newLegacyRegenTimerRuntime(t)
	worldruntime.UnregisterStaticActorCombatProfileForTest("qa_legacy_regen_timer_default")
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest("qa_legacy_regen_timer_default") })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           "practice.default_regen_timer",
			Name:          "DefaultRegenTimer",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_legacy_regen_timer_default",
			Count:         1,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.default_regen_other",
			Name:          "DefaultRegenOther",
			MapIndex:      42,
			X:             1600,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_legacy_regen_timer_default",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{legacyRegenTimerOneHitProfile("qa_legacy_regen_timer_default")},
	}); err != nil {
		t.Fatalf("import default regen plus spawn group: %v", err)
	}

	actor, ok := runtime.SpawnGroupByRef("practice.default_regen_timer")
	if !ok || actor.Dead {
		t.Fatalf("expected live default regen spawn, ok=%v snapshot=%+v", ok, actor)
	}
	other, ok := runtime.SpawnGroupByRef("practice.default_regen_other")
	if !ok || other.Dead {
		t.Fatalf("expected live default spawn group, ok=%v snapshot=%+v", ok, other)
	}

	flow := enterLegacyRegenTimerOwner(t, runtime)
	defer closeSessionFlow(t, flow)

	killVisibleSpawnGroup(t, flow, actor)
	deathAt := *currentTime
	actorRespawn, ok := runtime.StaticActorRespawn(actor.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for default regen row")
	}
	if !actorRespawn.ReadyAt.Equal(deathAt.Add(2 * time.Second)) {
		t.Fatalf("expected default regen row to keep profile ReadyAt %s, got %s", deathAt.Add(2*time.Second), actorRespawn.ReadyAt)
	}

	*currentTime = deathAt.Add(time.Second)
	killVisibleSpawnGroup(t, flow, other)
	otherRespawn, ok := runtime.StaticActorRespawn(other.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for spawn_groups without overlay")
	}
	if !otherRespawn.ReadyAt.Equal(currentTime.Add(2 * time.Second)) {
		t.Fatalf("expected spawn_groups without overlay to keep profile ReadyAt %s, got %s", currentTime.Add(2*time.Second), otherRespawn.ReadyAt)
	}
}

func newLegacyRegenTimerRuntime(t *testing.T) (*gameRuntime, *time.Time) {
	t.Helper()
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("LegacyRegenOwner", 0x01030541, 0x02040541, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "legacy-regen-owner", 0xf3f3f3f3, owner)

	currentTime := time.Unix(1700004700, 0)
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

func legacyRegenTimerOneHitProfile(name string) worldruntime.StaticActorCombatProfileSnapshot {
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

func enterLegacyRegenTimerOwner(t *testing.T, runtime *gameRuntime) service.SessionFlow {
	t.Helper()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "legacy-regen-owner", 0xf3f3f3f3)
	flushServerFrames(t, flow)
	return flow
}
