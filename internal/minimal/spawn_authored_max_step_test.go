package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Live chase/return/homeward planners must consume profile-authored max_step:
// authored 50 steps -50 toward home instead of hard-coding bootstrap 100.
func TestGameRuntimeAuthoredMaxStepPlansHomewardAtFifty(t *testing.T) {
	const profile = "practice_live_max_step_wolf"
	const authoredMaxStep int32 = 50

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AuthoredMaxStepOwner", 0x01030362, 0x02040362, 1850, 2800, 0, 121, 221)
	owner.MapIndex = 42
	issuePeerTicket(t, store, "authored-max-step-owner", 0xe4e4e4e4, owner)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
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
		staticActorStore,
		interactionStore,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.live_max_step_wolf",
			Name:          "LiveMaxStepMob",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: profile,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    8,
			DefenseValue:   2,
			RespawnDelayMs: 1500,
			MaxStep:        authoredMaxStep,
		}},
	}); err != nil {
		t.Fatalf("import authored max-step spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.live_max_step_wolf")
	if !ok {
		t.Fatal("expected authored max-step spawn group to resolve by ref")
	}
	if got := worldruntime.EffectiveStaticActorSpawnMaxStep(profile); got != authoredMaxStep {
		t.Fatalf("expected imported profile effective max step %d, got %d", authoredMaxStep, got)
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "authored-max-step-owner", 0xe4e4e4e4)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	// Displace +200 from home so one authored max_step=50 homeward beat leaves
	// the actor still within_radius and proves the planner did not keep 100.
	if _, ok := runtime.UpdateStaticActor(group.EntityID, "LiveMaxStepMob", 42, 1900, 2800, 20350); !ok {
		t.Fatal("expected within_radius displace to succeed before authored max-step homeward")
	}
	flushServerFrames(t, flow)

	runtime.spawnHomewardMu.Lock()
	dueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected within_radius displace to arm pending homeward for entity %d", group.EntityID)
	}
	expectedDueAt := currentTime.Add(worldruntime.DefaultSpawnHomewardDelay)
	if !dueAt.Equal(expectedDueAt) {
		t.Fatalf("expected bootstrap homeward deadline at %s, got %s", expectedDueAt, dueAt)
	}

	pending, ok := runtime.SpawnGroupHomewardStep(group.EntityID)
	if !ok {
		t.Fatal("expected pending homeward inspection snapshot for authored max-step actor")
	}
	if pending.Step.Next.X != 1850 || pending.Step.Next.Y != 2800 {
		t.Fatalf("expected pending homeward plan next at 1850 (step 50), got %+v", pending.Step.Next)
	}

	currentTime = currentTime.Add(worldruntime.DefaultSpawnHomewardDelay)
	firstQueued := flushServerFrames(t, flow)
	if len(firstQueued) == 0 {
		t.Fatal("expected first due authored max-step homeward to queue retained owner MOVE replication")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, firstQueued[0]))
	if err != nil {
		t.Fatalf("expected retained authored max-step homeward viewer to receive MOVE replication, first frame decode err=%v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 1850 || moveAck.Y != 2800 {
		t.Fatalf("expected authored max-step homeward MOVE at planned -50 toward home, got %+v", moveAck)
	}
	if moveAck.X == 1800 {
		t.Fatal("expected authored max_step=50 not to keep hard-coding bootstrap step 100")
	}
	for _, raw := range firstQueued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected authored max-step homeward MOVE fanout not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
	}

	runtime.spawnHomewardMu.Lock()
	_, rearmScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !rearmScheduled {
		t.Fatalf("expected post-step homeward re-arm for entity %d while still within_radius", group.EntityID)
	}
}
