package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestContentBundleRegenFacingOverlayStaysAuthoringOnly(t *testing.T) {
	authored := contentbundle.Bundle{RegenSpawns: []contentbundle.RegenSpawn{{
		Ref:      "practice.regen_facing",
		Name:     "RegenFacing",
		MapIndex: 42,
		X:        1700,
		Y:        2800,
		RaceNum:  20350,
		Count:    1,
		Angle:    90.5,
	}}}
	overlay := contentbundle.RegenFacingAngleBySpawnGroupRef(authored)
	if overlay["practice.regen_facing"] != 90.5 || len(overlay) != 1 {
		t.Fatalf("expected authored overlay on practice.regen_facing, got %#v", overlay)
	}
	canonical, err := contentbundle.Canonicalize(authored)
	if err != nil {
		t.Fatalf("canonicalize opt-in one-count regen facing: %v", err)
	}
	if len(canonical.RegenSpawns) != 0 {
		t.Fatalf("expected regen_spawns to stay authoring-only after canonicalize, got %+v", canonical.RegenSpawns)
	}
	if got := contentbundle.RegenFacingAngleBySpawnGroupRef(canonical); len(got) != 0 {
		t.Fatalf("expected canonical bundle to drop regen facing overlay, got %#v", got)
	}
}

// Opt-in regen_spawns direction / facing / angle copies CHARACTER_ADD angle
// for that expanded spawn. One-count keeps the authored ref. Default rows and
// spawn_groups without the overlay stay at angle=0.
func TestGameRuntimeOptInRegenFacingCopiesCharacterAddAngleForOneCount(t *testing.T) {
	runtime, currentTime := newRegenFacingRuntime(t)
	worldruntime.UnregisterStaticActorCombatProfileForTest("qa_regen_facing")
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest("qa_regen_facing") })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           "practice.regen_facing",
			Name:          "RegenFacing",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_regen_facing",
			Count:         1,
			Angle:         90.5,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.regen_facing_other",
			Name:          "RegenFacingOther",
			MapIndex:      42,
			X:             1600,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_regen_facing",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{regenFacingOneHitProfile("qa_regen_facing")},
	}); err != nil {
		t.Fatalf("import opt-in one-count regen facing plus independent spawn group: %v", err)
	}

	actor, ok := runtime.SpawnGroupByRef("practice.regen_facing")
	if !ok || actor.Dead || actor.X != 1700 || actor.Y != 2800 {
		t.Fatalf("expected live one-count overlay spawn at authored origin, ok=%v snapshot=%+v", ok, actor)
	}
	other, ok := runtime.SpawnGroupByRef("practice.regen_facing_other")
	if !ok || other.Dead || other.X != 1600 || other.Y != 2800 {
		t.Fatalf("expected live independent spawn group, ok=%v snapshot=%+v", ok, other)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "regen-facing-owner", 0xf4f4f4f4)
	defer closeSessionFlow(t, flow)
	assertCharacterAddAngle(t, enterOut, uint32(actor.EntityID), 90.5)
	assertCharacterAddAngle(t, enterOut, uint32(other.EntityID), 0)
	flushServerFrames(t, flow)

	killVisibleSpawnGroup(t, flow, actor)
	deathAt := *currentTime
	overlayRespawn, ok := runtime.StaticActorRespawn(actor.EntityID)
	if !ok {
		t.Fatal("expected pending respawn for opted-in one-count regen facing")
	}
	wantReadyAt := deathAt.Add(2 * time.Second)
	if !overlayRespawn.ReadyAt.Equal(wantReadyAt) {
		t.Fatalf("expected profile ReadyAt %s, got %s", wantReadyAt, overlayRespawn.ReadyAt)
	}

	*currentTime = wantReadyAt
	respawnFrames := flushServerFrames(t, flow)
	assertSpawnDead(t, runtime, actor.SpawnGroupRef, false)
	assertCharacterAddAngle(t, respawnFrames, uint32(actor.EntityID), 90.5)
}

// Multi-count copies the overlay onto every {ref}.mNN member CHARACTER_ADD.
// Direct spawn_groups stay at angle=0.
func TestGameRuntimeOptInRegenFacingCopiesOntoEveryPackMember(t *testing.T) {
	runtime, _ := newRegenFacingRuntime(t)
	worldruntime.UnregisterStaticActorCombatProfileForTest("qa_regen_facing_pack")
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest("qa_regen_facing_pack") })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           "practice.regen_facing_pack",
			Name:          "RegenFacingPack",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_regen_facing_pack",
			Count:         2,
			PackSpacing:   400,
			Direction:     180,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.regen_facing_pack_other",
			Name:          "RegenFacingPackOther",
			MapIndex:      42,
			X:             1600,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_regen_facing_pack",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{regenFacingOneHitProfile("qa_regen_facing_pack")},
	}); err != nil {
		t.Fatalf("import opt-in multi-count regen facing: %v", err)
	}

	first, ok := runtime.SpawnGroupByRef("practice.regen_facing_pack.m01")
	if !ok || first.Dead || first.X != 1700 || first.Y != 2800 {
		t.Fatalf("expected live pack member .m01 at authored origin, ok=%v snapshot=%+v", ok, first)
	}
	second, ok := runtime.SpawnGroupByRef("practice.regen_facing_pack.m02")
	if !ok || second.Dead || second.X != 2100 || second.Y != 2800 {
		t.Fatalf("expected live pack member .m02 at +pack_spacing X, ok=%v snapshot=%+v", ok, second)
	}
	other, ok := runtime.SpawnGroupByRef("practice.regen_facing_pack_other")
	if !ok || other.Dead {
		t.Fatalf("expected live independent spawn group, ok=%v snapshot=%+v", ok, other)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "regen-facing-owner", 0xf4f4f4f4)
	defer closeSessionFlow(t, flow)
	assertCharacterAddAngle(t, enterOut, uint32(first.EntityID), 180)
	assertCharacterAddAngle(t, enterOut, uint32(second.EntityID), 180)
	assertCharacterAddAngle(t, enterOut, uint32(other.EntityID), 0)
}

func TestGameRuntimeDefaultRegenAndSpawnGroupsStayAtZeroAngle(t *testing.T) {
	runtime, _ := newRegenFacingRuntime(t)
	worldruntime.UnregisterStaticActorCombatProfileForTest("qa_regen_facing_default")
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest("qa_regen_facing_default") })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           "practice.default_regen_facing",
			Name:          "DefaultRegenFacing",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_regen_facing_default",
			Count:         1,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.default_regen_facing_other",
			Name:          "DefaultRegenFacingOther",
			MapIndex:      42,
			X:             1600,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_regen_facing_default",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{regenFacingOneHitProfile("qa_regen_facing_default")},
	}); err != nil {
		t.Fatalf("import default regen plus spawn group: %v", err)
	}

	actor, ok := runtime.SpawnGroupByRef("practice.default_regen_facing")
	if !ok || actor.Dead {
		t.Fatalf("expected live default regen spawn, ok=%v snapshot=%+v", ok, actor)
	}
	other, ok := runtime.SpawnGroupByRef("practice.default_regen_facing_other")
	if !ok || other.Dead {
		t.Fatalf("expected live default spawn group, ok=%v snapshot=%+v", ok, other)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "regen-facing-owner", 0xf4f4f4f4)
	defer closeSessionFlow(t, flow)
	assertCharacterAddAngle(t, enterOut, uint32(actor.EntityID), 0)
	assertCharacterAddAngle(t, enterOut, uint32(other.EntityID), 0)
}

func newRegenFacingRuntime(t *testing.T) (*gameRuntime, *time.Time) {
	t.Helper()
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RegenFacingOwner", 0x01030551, 0x02040551, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "regen-facing-owner", 0xf4f4f4f4, owner)

	currentTime := time.Unix(1700004800, 0)
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

func regenFacingOneHitProfile(name string) worldruntime.StaticActorCombatProfileSnapshot {
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

func assertCharacterAddAngle(t *testing.T, frames [][]byte, vid uint32, want float32) {
	t.Helper()
	for _, raw := range frames {
		add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw))
		if err != nil || add.VID != vid {
			continue
		}
		if add.Angle != want {
			t.Fatalf("expected CHARACTER_ADD angle %v for vid %d, got %+v", want, vid, add)
		}
		return
	}
	t.Fatalf("expected CHARACTER_ADD for vid %d among %d frames", vid, len(frames))
}
