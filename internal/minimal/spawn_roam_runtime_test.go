package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// One opt-in idle roam step around authored home for a live unengaged at_home
// spawn-backed actor. Omitted roam delay stays stationary. Chase, homeward,
// and return stay on their own seams; this proof does not engage the actor.
func TestGameRuntimeOptInRoamDelayStepsAtHomeOnce(t *testing.T) {
	const profile = "practice_live_roam_delay_wolf"
	const stationaryProfile = "practice_live_roam_omitted_wolf"
	const authoredRoamDelay = 2 * time.Second

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OptInRoamOwner", 0x01030363, 0x02040363, 2050, 2800, 0, 121, 221)
	owner.MapIndex = 42
	issuePeerTicket(t, store, "opt-in-roam-owner", 0xe5e5e5e5, owner)

	currentTime := time.Unix(1700004800, 0)
	runtime, err := newGameRuntimeWithAccountStore(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     400,
			VisibilitySectorSize: 200,
		},
		store,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	t.Cleanup(func() {
		worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
		worldruntime.UnregisterStaticActorCombatProfileForTest(stationaryProfile)
	})

	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:       24,
		AttackValue: 8,
		DefenseValue: 2,
		RespawnDelay: time.Second + 500*time.Millisecond,
		RoamDelay:    authoredRoamDelay,
		MaxStep:      50,
		LeashRadius:  400,
	}) {
		t.Fatal("expected opt-in roam profile to register")
	}
	if !worldruntime.RegisterStaticActorCombatProfile(stationaryProfile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: time.Second + 500*time.Millisecond,
	}) {
		t.Fatal("expected omitted-roam profile to register")
	}
	if got := worldruntime.EffectiveStaticActorSpawnRoamDelay(profile); got != authoredRoamDelay {
		t.Fatalf("expected authored roam delay %v, got %v", authoredRoamDelay, got)
	}
	if got := worldruntime.EffectiveStaticActorSpawnRoamDelay(stationaryProfile); got != 0 {
		t.Fatalf("expected omitted roam delay to stay stationary, got %v", got)
	}

	home := &worldruntime.PositionSnapshot{MapIndex: 42, X: 1700, Y: 2800}
	roamer, ok := runtime.registerStaticActorWithInteractionCombatProfileSpawnGroupRefHomeAndReward(
		"LiveRoamMob", 42, 1700, 2800, 20350, "", "", profile, "practice.live_roam_wolf", home, worldruntime.StaticActorDeathReward{},
	)
	if !ok {
		t.Fatal("expected opt-in roam spawn actor to register")
	}
	stationaryHome := &worldruntime.PositionSnapshot{MapIndex: 42, X: 1900, Y: 2800}
	stationary, ok := runtime.registerStaticActorWithInteractionCombatProfileSpawnGroupRefHomeAndReward(
		"LiveStillMob", 42, 1900, 2800, 20350, "", "", stationaryProfile, "practice.live_still_wolf", stationaryHome, worldruntime.StaticActorDeathReward{},
	)
	if !ok {
		t.Fatal("expected omitted-roam spawn actor to register")
	}

	runtime.spawnRoamMu.Lock()
	roamDue, roamArmed := runtime.spawnRoamStepDueAt[roamer.EntityID]
	_, stationaryArmed := runtime.spawnRoamStepDueAt[stationary.EntityID]
	runtime.spawnRoamMu.Unlock()
	if !roamArmed {
		t.Fatalf("expected at_home opt-in actor %d to arm one roam deadline", roamer.EntityID)
	}
	if stationaryArmed {
		t.Fatalf("expected omitted roam delay to keep entity %d stationary", stationary.EntityID)
	}
	expectedDue := currentTime.Add(authoredRoamDelay)
	if !roamDue.Equal(expectedDue) {
		t.Fatalf("expected roam deadline at %s, got %s", expectedDue, roamDue)
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "opt-in-roam-owner", 0xe5e5e5e5)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	currentTime = currentTime.Add(authoredRoamDelay)
	queued := flushServerFrames(t, flow)
	if len(queued) == 0 {
		t.Fatal("expected due opt-in roam to queue one retained-viewer MOVE")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("expected retained roam viewer to receive MOVE, decode err=%v", err)
	}
	if moveAck.VID != uint32(roamer.EntityID) || moveAck.X != 1750 || moveAck.Y != 2800 {
		t.Fatalf("expected one max_step=50 roam MOVE to +50 x around home, got %+v", moveAck)
	}
	for _, raw := range queued {
		frame := decodeSingleFrame(t, raw)
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(frame); err == nil && deleted.VID == uint32(roamer.EntityID) {
			t.Fatalf("expected roam MOVE fanout not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
		if speed, err := worldproto.DecodeChangeSpeed(frame); err == nil && speed.VID == uint32(roamer.EntityID) {
			t.Fatalf("expected idle roam not to emit chase CHANGE_SPEED, got %+v", speed)
		}
		if moved, err := movep.DecodeMoveAck(frame); err == nil && moved.VID == uint32(stationary.EntityID) {
			t.Fatalf("expected omitted-roam actor to stay still, got %+v", moved)
		}
	}

	stepped, ok := runtime.SpawnGroup(roamer.EntityID)
	if !ok || stepped.X != 1750 || stepped.Y != 2800 || stepped.SpawnLeash == nil || stepped.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected one roam step to leave the actor within_radius at +50, got ok=%v actor=%+v", ok, stepped)
	}
	still, ok := runtime.SpawnGroup(stationary.EntityID)
	if !ok || still.X != 1900 || still.Y != 2800 || still.SpawnLeash == nil || still.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected omitted-roam actor to stay at_home, got ok=%v actor=%+v", ok, still)
	}

	runtime.spawnRoamMu.Lock()
	_, roamerRearmed := runtime.spawnRoamStepDueAt[roamer.EntityID]
	_, stationaryRearmed := runtime.spawnRoamStepDueAt[stationary.EntityID]
	runtime.spawnRoamMu.Unlock()
	if roamerRearmed || stationaryRearmed {
		t.Fatal("expected one roam step not to keep wandering once the actor leaves at_home, and omitted delay to stay unarmed")
	}
	runtime.spawnHomewardMu.Lock()
	_, homewardArmed := runtime.spawnHomewardStepDueAt[roamer.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardArmed {
		t.Fatal("expected the post-roam within_radius actor to arm homeward so return-home still wins over another wander")
	}
}
