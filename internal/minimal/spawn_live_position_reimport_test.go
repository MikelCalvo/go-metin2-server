package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Content reimport of the same authored spawn coords must rematerialize a live
// spawn-backed actor at its FileStore current position, not snap it back to
// authored home. Authored home stays the leash origin. Dead / return_required
// corpses stay on the already-owned death-coord restore.
func TestGameRuntimeContentReimportRematerializesLiveSpawnPositionDistinctFromAuthoredHome(t *testing.T) {
	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewMemoryStore()
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticActorStore,
		interactionStore,
	)
	if err != nil {
		t.Fatalf("new game runtime for live-position reimport: %v", err)
	}
	currentTime := time.Unix(1700004700, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{
		{
			Ref:           "practice.live_position_within",
			Name:          "LivePositionWithinMob",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
		},
		{
			Ref:           "practice.live_position_dead",
			Name:          "LivePositionDeadMob",
			MapIndex:      42,
			X:             1700,
			Y:             3000,
			RaceNum:       20350,
			CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
		},
	}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import live-position spawn-group bundle: %v", err)
	}
	within, ok := runtime.SpawnGroupByRef("practice.live_position_within")
	if !ok {
		t.Fatal("expected within_radius live-position spawn group to resolve by ref")
	}
	dead, ok := runtime.SpawnGroupByRef("practice.live_position_dead")
	if !ok {
		t.Fatal("expected still-dead live-position spawn group to resolve by ref")
	}
	updatedWithin, ok := runtime.UpdateStaticActor(within.EntityID, "LivePositionWithinMob", 42, 1800, 2800, 20350)
	if !ok || updatedWithin.X != 1800 || updatedWithin.Y != 2800 || updatedWithin.SpawnHome == nil || updatedWithin.SpawnHome.X != 1700 || updatedWithin.SpawnHome.Y != 2800 {
		t.Fatalf("expected within_radius displace to keep authored home, ok=%v snapshot=%+v", ok, updatedWithin)
	}
	if updatedWithin.SpawnLeash == nil || updatedWithin.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius || updatedWithin.SpawnLeash.ReturnRequired {
		t.Fatalf("expected displaced live actor to classify within_radius before reimport, got %+v", updatedWithin.SpawnLeash)
	}
	updatedDead, ok := runtime.UpdateStaticActor(dead.EntityID, "LivePositionDeadMob", 42, 2301, 3000, 20350)
	if !ok || updatedDead.SpawnLeash == nil || !updatedDead.SpawnLeash.ReturnRequired || updatedDead.SpawnHome == nil || updatedDead.SpawnHome.X != 1700 {
		t.Fatalf("expected death-coord displace to classify return_required with authored home, ok=%v snapshot=%+v", ok, updatedDead)
	}
	if !runtime.sharedWorld.restoreStillDeadSpawnGroupCombatState(dead.EntityID, currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)) {
		t.Fatal("expected still-dead overlay at the displaced death coords")
	}
	if !runtime.persistSpawnGroupCombatState(dead.EntityID) {
		t.Fatal("expected still-dead death coords to persist before content reimport")
	}
	stillDead, ok := runtime.SpawnGroup(dead.EntityID)
	if !ok || !stillDead.Dead || stillDead.X != 2301 || stillDead.Y != 3000 {
		t.Fatalf("expected still-dead corpse at death coords before reimport, ok=%v snapshot=%+v", ok, stillDead)
	}

	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("reimport identical authored spawn coords: %v", err)
	}

	restoredWithin, ok := runtime.SpawnGroupByRef("practice.live_position_within")
	if !ok {
		t.Fatal("expected within_radius spawn group to remain resolvable after content reimport")
	}
	if restoredWithin.X != 1800 || restoredWithin.Y != 2800 || restoredWithin.Dead {
		t.Fatalf("expected live rematerialize at FileStore current position, got %+v", restoredWithin)
	}
	if restoredWithin.SpawnHome == nil || restoredWithin.SpawnHome.MapIndex != 42 || restoredWithin.SpawnHome.X != 1700 || restoredWithin.SpawnHome.Y != 2800 {
		t.Fatalf("expected authored home to stay the leash origin, got %+v", restoredWithin.SpawnHome)
	}
	if restoredWithin.SpawnLeash == nil || restoredWithin.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius || restoredWithin.SpawnLeash.ReturnRequired || restoredWithin.SpawnLeash.Home.X != 1700 || restoredWithin.SpawnLeash.Current.X != 1800 {
		t.Fatalf("expected rematerialized live actor to stay within_radius of authored home, got %+v", restoredWithin.SpawnLeash)
	}
	if pending, ok := runtime.SpawnGroupHomewardStep(restoredWithin.EntityID); !ok || pending.EntityID != restoredWithin.EntityID {
		t.Fatalf("expected unengaged within_radius rematerialize to arm homeward, ok=%v snapshot=%+v", ok, pending)
	}
	if pending, ok := runtime.SpawnGroupReturnStep(restoredWithin.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected within_radius rematerialize not to arm return-step, ok=%v snapshot=%+v", ok, pending)
	}

	restoredDead, ok := runtime.SpawnGroupByRef("practice.live_position_dead")
	if !ok {
		t.Fatal("expected still-dead spawn group to remain resolvable after content reimport")
	}
	if !restoredDead.Dead || restoredDead.X != 2301 || restoredDead.Y != 3000 {
		t.Fatalf("expected still-dead corpse to stay on death coords, got %+v", restoredDead)
	}
	if restoredDead.SpawnHome == nil || restoredDead.SpawnHome.X != 1700 || restoredDead.SpawnHome.Y != 3000 {
		t.Fatalf("expected still-dead authored home to stay the leash origin, got %+v", restoredDead.SpawnHome)
	}
	if pending, ok := runtime.SpawnGroupReturnStep(restoredDead.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected still-dead return_required rematerialize not to arm return-step, ok=%v snapshot=%+v", ok, pending)
	}
	if pending, ok := runtime.SpawnGroupHomewardStep(restoredDead.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected still-dead rematerialize not to arm homeward, ok=%v snapshot=%+v", ok, pending)
	}

	exported, err := runtime.ExportContentBundle()
	if err != nil {
		t.Fatalf("export content bundle after live-position reimport: %v", err)
	}
	if len(exported.SpawnGroups) != 2 {
		t.Fatalf("expected content export to keep authored spawn coords only, got %#v", exported.SpawnGroups)
	}
	for _, group := range exported.SpawnGroups {
		if group.X != 1700 || (group.Ref == "practice.live_position_within" && group.Y != 2800) || (group.Ref == "practice.live_position_dead" && group.Y != 3000) {
			t.Fatalf("expected content export to keep authored home, not the live position, got %#v", group)
		}
	}

	replaced := exported
	replaced.SpawnGroups = append([]contentbundle.SpawnGroup(nil), exported.SpawnGroups...)
	for i := range replaced.SpawnGroups {
		if replaced.SpawnGroups[i].Ref == "practice.live_position_within" {
			replaced.SpawnGroups[i].Name = "LivePositionWithinMobRenamed"
		}
	}
	if _, err := runtime.ImportContentBundle(replaced); err != nil {
		t.Fatalf("reimport non-identical authored spawn coords: %v", err)
	}
	renamedWithin, ok := runtime.SpawnGroupByRef("practice.live_position_within")
	if !ok || renamedWithin.Name != "LivePositionWithinMobRenamed" {
		t.Fatalf("expected non-identical reimport to apply the authored name, ok=%v snapshot=%+v", ok, renamedWithin)
	}
	if renamedWithin.X != 1800 || renamedWithin.Y != 2800 || renamedWithin.Dead {
		t.Fatalf("expected non-identical reimport to keep the live position, got %+v", renamedWithin)
	}
	if renamedWithin.SpawnHome == nil || renamedWithin.SpawnHome.X != 1700 || renamedWithin.SpawnHome.Y != 2800 {
		t.Fatalf("expected non-identical reimport to keep authored home as the leash origin, got %+v", renamedWithin.SpawnHome)
	}
	renamedDead, ok := runtime.SpawnGroupByRef("practice.live_position_dead")
	if !ok || !renamedDead.Dead || renamedDead.X != 2301 || renamedDead.Y != 3000 {
		t.Fatalf("expected non-identical reimport to keep the still-dead corpse on death coords, got %+v", renamedDead)
	}

	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load FileStore after live-position reimport: %v", err)
	}
	if len(persisted.StaticActors) != 2 {
		t.Fatalf("expected two persisted spawn-backed actors, got %+v", persisted.StaticActors)
	}
	for _, actor := range persisted.StaticActors {
		switch actor.SpawnGroupRef {
		case "practice.live_position_within":
			if actor.X != 1800 || actor.Y != 2800 || actor.SpawnHome == nil || actor.SpawnHome.X != 1700 || actor.SpawnHome.Y != 2800 {
				t.Fatalf("expected FileStore live position beside authored home, got %+v", actor)
			}
		case "practice.live_position_dead":
			if actor.X != 2301 || actor.Y != 3000 || actor.SpawnHome == nil || actor.SpawnHome.X != 1700 || actor.CombatCurrentHP == nil || *actor.CombatCurrentHP != 0 {
				t.Fatalf("expected FileStore death coords beside authored home, got %+v", actor)
			}
		default:
			t.Fatalf("unexpected persisted spawn group %q", actor.SpawnGroupRef)
		}
	}
}
