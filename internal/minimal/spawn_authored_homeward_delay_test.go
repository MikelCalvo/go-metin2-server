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

// Live homeward-step arming / post-step re-arm must consume the profile-authored
// homeward_delay_ms seam: 2000ms arms and re-arms instead of hard-coding bootstrap 1s.
func TestGameRuntimeAuthoredHomewardDelayArmsAndRearmsAtTwoSeconds(t *testing.T) {
	const profile = "practice_live_homeward_delay_wolf"
	const authoredHomewardDelay = 2 * time.Second

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AuthoredHomewardDelayOwner", 0x01030361, 0x02040361, 1850, 2800, 0, 121, 221)
	owner.MapIndex = 42
	issuePeerTicket(t, store, "authored-homeward-delay-owner", 0xe3e3e3e3, owner)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
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
			Ref:           "practice.live_homeward_delay_wolf",
			Name:          "LiveHomewardDelayMob",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: profile,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:         profile,
			MaxHP:           24,
			AttackValue:     8,
			DefenseValue:    2,
			RespawnDelayMs:  1500,
			HomewardDelayMs: authoredHomewardDelay.Milliseconds(),
		}},
	}); err != nil {
		t.Fatalf("import authored homeward-delay spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.live_homeward_delay_wolf")
	if !ok {
		t.Fatal("expected authored homeward-delay spawn group to resolve by ref")
	}
	if got := worldruntime.EffectiveStaticActorSpawnHomewardDelay(profile); got != authoredHomewardDelay {
		t.Fatalf("expected imported profile effective homeward delay %v, got %v", authoredHomewardDelay, got)
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "authored-homeward-delay-owner", 0xe3e3e3e3)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	// Displace +200 from home so one max_step=100 homeward beat leaves the actor
	// still within_radius and forces a post-step re-arm.
	if _, ok := runtime.UpdateStaticActor(group.EntityID, "LiveHomewardDelayMob", 42, 1900, 2800, 20350); !ok {
		t.Fatal("expected within_radius displace to succeed before authored homeward arm")
	}
	flushServerFrames(t, flow)

	runtime.spawnHomewardMu.Lock()
	dueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected within_radius displace to arm pending homeward for entity %d", group.EntityID)
	}
	expectedDueAt := currentTime.Add(authoredHomewardDelay)
	if !dueAt.Equal(expectedDueAt) {
		t.Fatalf("expected authored homeward deadline at %s, got %s", expectedDueAt, dueAt)
	}
	if dueAt.Equal(currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)) {
		t.Fatal("expected authored homeward delay not to keep hard-coding bootstrap 1s")
	}

	currentTime = currentTime.Add(authoredHomewardDelay)
	firstQueued := flushServerFrames(t, flow)
	if len(firstQueued) == 0 {
		t.Fatal("expected first due authored homeward-step to queue retained owner MOVE replication")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, firstQueued[0]))
	if err != nil {
		t.Fatalf("expected retained authored homeward-step viewer to receive MOVE replication, first frame decode err=%v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 1800 || moveAck.Y != 2800 {
		t.Fatalf("expected authored homeward-step MOVE replication at planned -100 toward home, got %+v", moveAck)
	}
	for _, raw := range firstQueued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected authored homeward-step MOVE fanout not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
	}

	runtime.spawnHomewardMu.Lock()
	rearmDueAt, rearmScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !rearmScheduled {
		t.Fatalf("expected post-step homeward re-arm for entity %d", group.EntityID)
	}
	expectedRearmDueAt := currentTime.Add(authoredHomewardDelay)
	if !rearmDueAt.Equal(expectedRearmDueAt) {
		t.Fatalf("expected authored homeward re-arm deadline at %s, got %s", expectedRearmDueAt, rearmDueAt)
	}
}
