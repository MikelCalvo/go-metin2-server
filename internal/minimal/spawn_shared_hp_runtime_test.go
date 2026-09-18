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

func TestContentBundleSharedHPPackPrefixesStayAuthoringOnly(t *testing.T) {
	authored := contentbundle.Bundle{RegenSpawns: []contentbundle.RegenSpawn{{
		Ref:         "practice.shared_hp_pack",
		Name:        "SharedHPPack",
		MapIndex:    42,
		X:           1700,
		Y:           2800,
		RaceNum:     20350,
		Count:       2,
		PackSpacing: 100,
		SharedHP:    true,
	}}}
	prefixes := contentbundle.SharedHPPackPrefixes(authored)
	if _, ok := prefixes["practice.shared_hp_pack"]; !ok || len(prefixes) != 1 {
		t.Fatalf("expected authored shared_hp overlay for practice.shared_hp_pack, got %#v", prefixes)
	}
	canonical, err := contentbundle.Canonicalize(authored)
	if err != nil {
		t.Fatalf("canonicalize opt-in shared_hp pack: %v", err)
	}
	if len(canonical.RegenSpawns) != 0 {
		t.Fatalf("expected regen_spawns to stay authoring-only after canonicalize, got %+v", canonical.RegenSpawns)
	}
	if got := contentbundle.SharedHPPackPrefixes(canonical); len(got) != 0 {
		t.Fatalf("expected canonical bundle to drop shared_hp overlay, got %#v", got)
	}
	if len(canonical.SpawnGroups) != 2 ||
		canonical.SpawnGroups[0].Ref != "practice.shared_hp_pack.m01" ||
		canonical.SpawnGroups[1].Ref != "practice.shared_hp_pack.m02" {
		t.Fatalf("expected independent {ref}.mNN members after canonicalize, got %+v", canonical.SpawnGroups)
	}
}

func TestContentBundleRejectsOneCountRegenSharedHP(t *testing.T) {
	_, err := contentbundle.Canonicalize(contentbundle.Bundle{RegenSpawns: []contentbundle.RegenSpawn{{
		Ref:      "practice.one_count_shared_hp",
		Name:     "OneCountSharedHP",
		MapIndex: 42,
		X:        1700,
		Y:        2800,
		RaceNum:  20350,
		Count:    1,
		SharedHP: true,
	}}})
	if !errors.Is(err, contentbundle.ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for one-count regen shared_hp, got %v", err)
	}
}

// Opt-in shared_hp copies remaining HP from an accepted live hit onto other live
// same-prefix siblings. Independent one-count refs stay unlinked.
func TestGameRuntimeOptInPackSharedHPCopiesRemainingHPOntoLiveSibling(t *testing.T) {
	runtime := newSharedHPRuntime(t)
	worldruntime.UnregisterStaticActorCombatProfileForTest("qa_shared_hp_live")
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest("qa_shared_hp_live") })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           "practice.shared_hp_live",
			Name:          "SharedHPPack",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_shared_hp_live",
			Count:         2,
			PackSpacing:   400,
			SharedHP:      true,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.shared_hp_other",
			Name:          "SharedHPOther",
			MapIndex:      42,
			X:             1600,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: "qa_shared_hp_live",
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{sharedHPTenHitProfile("qa_shared_hp_live")},
	}); err != nil {
		t.Fatalf("import opt-in shared_hp pack plus independent one-count: %v", err)
	}

	first, second, other := sharedHPPackMembers(t, runtime, "practice.shared_hp_live")
	flow := enterSharedHPOwner(t, runtime)
	defer closeSessionFlow(t, flow)

	hitVisibleSpawnGroup(t, flow, first)
	wantPercent := uint8(90)
	assertSpawnHPPercent(t, runtime, first.SpawnGroupRef, wantPercent, false)
	assertSpawnHPPercent(t, runtime, second.SpawnGroupRef, wantPercent, false)
	liveOther, ok := runtime.SpawnGroupByRef(other.SpawnGroupRef)
	if !ok || liveOther.Dead || liveOther.CombatHPPercent == wantPercent || liveOther.X != 1600 || liveOther.Y != 2800 {
		t.Fatalf("expected independent one-count ref to stay unmoved without shared HP, ok=%v snapshot=%+v", ok, liveOther)
	}

	liveSibling, ok := runtime.SpawnGroupByRef(second.SpawnGroupRef)
	if !ok || liveSibling.Dead || liveSibling.X != 2100 || liveSibling.Y != 2800 {
		t.Fatalf("expected live sibling to stay at authored placement after shared-HP copy, ok=%v snapshot=%+v", ok, liveSibling)
	}
}

func TestGameRuntimeDefaultPackHPStaysIndependentPerMember(t *testing.T) {
	runtime := newSharedHPRuntime(t)
	importSharedHPPack(t, runtime, "practice.default_pack_hp", "qa_shared_hp_default", false)

	first, second, _ := sharedHPPackMembers(t, runtime, "practice.default_pack_hp")
	flow := enterSharedHPOwner(t, runtime)
	defer closeSessionFlow(t, flow)

	hitVisibleSpawnGroup(t, flow, first)
	assertSpawnHPPercent(t, runtime, first.SpawnGroupRef, 90, false)

	liveSibling, ok := runtime.SpawnGroupByRef(second.SpawnGroupRef)
	if !ok || liveSibling.Dead || liveSibling.CombatHPPercent == 90 || liveSibling.X != 2100 || liveSibling.Y != 2800 {
		t.Fatalf("expected default pack sibling to stay at authored placement with independent HP, ok=%v snapshot=%+v", ok, liveSibling)
	}
}

func newSharedHPRuntime(t *testing.T) *gameRuntime {
	t.Helper()
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SharedHPOwner", 0x01030531, 0x02040531, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "shared-hp-owner", 0xf2f2f2f2, owner)

	currentTime := time.Unix(1700004600, 0)
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
	return runtime
}

func sharedHPTenHitProfile(name string) worldruntime.StaticActorCombatProfileSnapshot {
	return worldruntime.StaticActorCombatProfileSnapshot{
		Profile:               name,
		MaxHP:                 worldruntime.PracticeMobBootstrapMaxHP,
		DamagePerNormalAttack: worldruntime.PracticeMobBootstrapDamagePerNormalAttack,
		AttackValue:           worldruntime.PracticeMobBootstrapAttackValue,
		DefenseValue:          worldruntime.PracticeMobBootstrapDefenseValue,
		Level:                 worldruntime.PracticeMobBootstrapLevel,
		Rank:                  worldruntime.PracticeMobBootstrapRank,
		RespawnDelayMs:        worldruntime.PracticeMobBootstrapRespawnDelay.Milliseconds(),
		RetaliationPointDelta: worldruntime.PracticeMobBootstrapRetaliationPointDelta,
	}
}

func importSharedHPPack(t *testing.T, runtime *gameRuntime, packRef string, profile string, sharedHP bool) {
	t.Helper()
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		RegenSpawns: []contentbundle.RegenSpawn{{
			Ref:           packRef,
			Name:          "SharedHPPack",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: profile,
			Count:         2,
			PackSpacing:   400,
			SharedHP:      sharedHP,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{sharedHPTenHitProfile(profile)},
	}); err != nil {
		t.Fatalf("import shared-hp regen bundle %s: %v", packRef, err)
	}
}

func sharedHPPackMembers(t *testing.T, runtime *gameRuntime, packRef string) (StaticActorSnapshot, StaticActorSnapshot, StaticActorSnapshot) {
	t.Helper()
	first, ok := runtime.SpawnGroupByRef(packRef + ".m01")
	if !ok || first.Dead || first.X != 1700 || first.Y != 2800 {
		t.Fatalf("expected live pack member .m01 at authored origin, ok=%v snapshot=%+v", ok, first)
	}
	second, ok := runtime.SpawnGroupByRef(packRef + ".m02")
	if !ok || second.Dead || second.X != 2100 || second.Y != 2800 {
		t.Fatalf("expected live pack member .m02 at +pack_spacing X, ok=%v snapshot=%+v", ok, second)
	}
	other, ok := runtime.SpawnGroupByRef("practice.shared_hp_other")
	if !ok {
		return first, second, StaticActorSnapshot{}
	}
	if other.Dead || other.X != 1600 || other.Y != 2800 {
		t.Fatalf("expected live independent one-count spawn group, ok=%v snapshot=%+v", ok, other)
	}
	return first, second, other
}

func enterSharedHPOwner(t *testing.T, runtime *gameRuntime) service.SessionFlow {
	t.Helper()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "shared-hp-owner", 0xf2f2f2f2)
	flushServerFrames(t, flow)
	return flow
}

func hitVisibleSpawnGroup(t *testing.T, flow service.SessionFlow, actor StaticActorSnapshot) {
	t.Helper()
	vid := uint32(actor.EntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: vid})))
	if err != nil {
		t.Fatalf("unexpected target error before hit %s: %v", actor.SpawnGroupRef, err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select %s, got %d frames", actor.SpawnGroupRef, len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  vid,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit on %s: %v", actor.SpawnGroupRef, err)
	}
	if len(attackOut) == 0 {
		t.Fatalf("expected accepted hit on %s to emit combat frames", actor.SpawnGroupRef)
	}
}

func assertSpawnHPPercent(t *testing.T, runtime *gameRuntime, ref string, wantPercent uint8, wantDead bool) {
	t.Helper()
	snapshot, ok := runtime.SpawnGroupByRef(ref)
	if !ok || snapshot.Dead != wantDead || snapshot.CombatHPPercent != wantPercent {
		t.Fatalf("expected %s dead=%v hp_percent=%d, ok=%v snapshot=%+v", ref, wantDead, wantPercent, ok, snapshot)
	}
}
