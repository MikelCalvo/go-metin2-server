package minimal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// loadPersistedStaticActors must re-read after the still-dead overlay restore.
// A rematerialized still-dead return_required corpse must not arm an automatic
// return-step deadline; inspection and the planner already fail closed on HP=0,
// and live operator updates already refuse to arm return-step while dead.
func TestGameRuntimeRestoreStillDeadReturnRequiredSpawnGroupDoesNotArmReturnStep(t *testing.T) {
	zeroHP := uint8(0)
	readyAt := time.Unix(1700006100, 0).UTC()
	staticActorStore := staticstore.NewFileStore(filepath.Join(t.TempDir(), "static-actors.json"))
	if err := staticActorStore.Save(staticstore.Snapshot{StaticActors: []staticstore.StaticActor{{
		EntityID:        0x361,
		Name:            "StillDeadReturnRestoreMob",
		MapIndex:        42,
		X:               2301,
		Y:               2800,
		RaceNum:         20350,
		SpawnHome:       &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800},
		CombatProfile:   string(worldruntime.StaticActorCombatProfilePracticeMob),
		SpawnGroupRef:   "practice.still_dead_return_restore",
		CombatCurrentHP: &zeroHP,
		RespawnReadyAt:  &readyAt,
	}}}); err != nil {
		t.Fatalf("seed persisted still-dead return-required spawn-group actor: %v", err)
	}

	currentTime := readyAt.Add(-2 * time.Second)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     400,
			VisibilitySectorSize: 200,
		},
		loginticket.NewFileStore(t.TempDir()),
		nil,
		staticActorStore,
		interactionstore.NewMemoryStore(),
	)
	if err != nil {
		t.Fatalf("new game runtime with restored still-dead return-required spawn group: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	group, ok := runtime.SpawnGroupByRef("practice.still_dead_return_restore")
	if !ok {
		t.Fatal("expected restored still-dead spawn group to resolve by ref")
	}
	if !group.Dead || group.X != 2301 || group.Y != 2800 || group.SpawnLeash == nil || !group.SpawnLeash.ReturnRequired {
		t.Fatalf("expected restored spawn group to stay still-dead return_required at death coords, got %+v", group)
	}
	respawns := runtime.StaticActorRespawns()
	if len(respawns) != 1 || respawns[0].EntityID != group.EntityID || !respawns[0].ReadyAt.Equal(readyAt) {
		t.Fatalf("expected pending respawn deadline to survive still-dead restore, got %+v", respawns)
	}

	runtime.spawnReturnMu.Lock()
	_, scheduled := runtime.spawnReturnStepDueAt[group.EntityID]
	runtime.spawnReturnMu.Unlock()
	if scheduled {
		t.Fatalf("expected still-dead return_required restore not to arm an automatic return-step schedule for entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupReturnStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected still-dead return_required restore to omit pending return-step inspection, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupReturnStepDelay)
	runtime.flushDueSpawnGroupReturnSteps()
	stillDead, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || !stillDead.Dead || stillDead.X != 2301 || stillDead.Y != 2800 {
		t.Fatalf("expected still-dead return_required restore not to MOVE the corpse toward home, ok=%v snapshot=%+v", ok, stillDead)
	}
}
