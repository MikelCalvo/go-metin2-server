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

// Respawn restores authored home and must retire every earlier movement cadence.
// Return-step cleanup was already owned; chase and homeward have the same
// respawn boundary and must not survive to move the freshly rebuilt actor.
func TestGameRuntimeRespawnClearsStaleSpawnGroupChaseAndHomewardStepSchedules(t *testing.T) {
	currentTime := time.Unix(1700005640, 0)

	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
	)
	if err != nil {
		t.Fatalf("new game runtime for respawn chase/homeward schedule cleanup: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.respawn_clears_chase_homeward_steps",
		Name:          "RespawnClearsChaseHomewardStepsMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import respawn chase/homeward schedule cleanup spawn group: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.respawn_clears_chase_homeward_steps")
	if !ok {
		t.Fatal("expected respawn chase/homeward schedule cleanup spawn group to resolve by ref")
	}
	if _, ok := runtime.UpdateStaticActor(group.EntityID, "RespawnClearsChaseHomewardStepsMob", 42, 1800, 2800, 20350); !ok {
		t.Fatal("expected spawn-backed actor update to within_radius position to succeed")
	}

	// Keep the artificial stale deadlines beyond the natural respawn boundary;
	// reaching this time later proves the rebuilt actor cannot consume an old beat.
	futureDueAt := currentTime.Add(2 * worldruntime.PracticeMobBootstrapRespawnDelay)
	runtime.spawnChaseMu.Lock()
	runtime.spawnChaseStepDueAt[group.EntityID] = futureDueAt
	runtime.spawnChaseMu.Unlock()
	runtime.spawnHomewardMu.Lock()
	runtime.spawnHomewardStepDueAt[group.EntityID] = futureDueAt
	runtime.spawnHomewardMu.Unlock()
	if pending, ok := runtime.SpawnGroupHomewardStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected live within_radius actor to expose its pending homeward step before forced death, ok=%v snapshot=%+v", ok, pending)
	}

	runtime.sharedWorld.mu.Lock()
	actor, ok := runtime.sharedWorld.entities.StaticActor(group.EntityID)
	if !ok {
		runtime.sharedWorld.mu.Unlock()
		t.Fatalf("expected static actor %d before forced respawn cleanup death", group.EntityID)
	}
	runtime.sharedWorld.staticActorCombatHP[group.EntityID] = 0
	runtime.sharedWorld.scheduleStaticActorCombatRespawnLocked(actor)
	runtime.sharedWorld.mu.Unlock()

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)
	runtime.flushReadyStaticActorRespawns()

	respawned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || respawned.Dead || respawned.X != 1700 || respawned.Y != 2800 || respawned.SpawnLeash == nil || respawned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected respawn to restore authored home and live at_home leash state, ok=%v snapshot=%+v", ok, respawned)
	}
	runtime.spawnChaseMu.Lock()
	_, chaseStillScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseStillScheduled {
		t.Fatalf("expected respawn to clear stale pending chase-step deadline for actor %d", group.EntityID)
	}
	runtime.spawnHomewardMu.Lock()
	_, homewardStillScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if homewardStillScheduled {
		t.Fatalf("expected respawn to clear stale pending homeward-step deadline for actor %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected respawned home actor to have no pending chase-step snapshot, ok=%v snapshot=%+v", ok, pending)
	}
	if pending, ok := runtime.SpawnGroupHomewardStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected respawned home actor to have no pending homeward-step snapshot, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = futureDueAt
	runtime.flushDueSpawnGroupChaseSteps()
	runtime.flushDueSpawnGroupHomewardSteps()
	unchanged, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || unchanged.Dead || unchanged.X != 1700 || unchanged.Y != 2800 || unchanged.SpawnLeash == nil || unchanged.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected retired pre-respawn movement deadlines not to move rebuilt actor, ok=%v snapshot=%+v", ok, unchanged)
	}
}
