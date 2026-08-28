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

// Live return-step arming / post-step re-arm must consume the profile-authored
// return_delay_ms seam: 2000ms arms and re-arms instead of hard-coding bootstrap 1s.
func TestGameRuntimeAuthoredReturnDelayArmsAndRearmsAtTwoSeconds(t *testing.T) {
	const profile = "practice_live_return_delay_wolf"
	const authoredReturnDelay = 2 * time.Second

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AuthoredReturnDelayOwner", 0x01030351, 0x02040351, 2250, 2800, 0, 111, 211)
	owner.MapIndex = 42
	issuePeerTicket(t, store, "authored-return-delay-owner", 0xe2e2e2e2, owner)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
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
			Ref:           "practice.live_return_delay_wolf",
			Name:          "LiveReturnDelayMob",
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
			ReturnDelayMs:  authoredReturnDelay.Milliseconds(),
		}},
	}); err != nil {
		t.Fatalf("import authored return-delay spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.live_return_delay_wolf")
	if !ok {
		t.Fatal("expected authored return-delay spawn group to resolve by ref")
	}
	if got := worldruntime.EffectiveStaticActorSpawnReturnDelay(profile); got != authoredReturnDelay {
		t.Fatalf("expected imported profile effective return delay %v, got %v", authoredReturnDelay, got)
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "authored-return-delay-owner", 0xe2e2e2e2)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	if _, ok := runtime.UpdateStaticActor(group.EntityID, "LiveReturnDelayMob", 42, 2301, 2800, 20350); !ok {
		t.Fatal("expected return_required displace to succeed before authored return arm")
	}
	flushServerFrames(t, flow)

	runtime.spawnReturnMu.Lock()
	dueAt, returnScheduled := runtime.spawnReturnStepDueAt[group.EntityID]
	runtime.spawnReturnMu.Unlock()
	if !returnScheduled {
		t.Fatalf("expected return_required displace to arm pending return for entity %d", group.EntityID)
	}
	expectedDueAt := currentTime.Add(authoredReturnDelay)
	if !dueAt.Equal(expectedDueAt) {
		t.Fatalf("expected authored return deadline at %s, got %s", expectedDueAt, dueAt)
	}
	if dueAt.Equal(currentTime.Add(bootstrapSpawnGroupReturnStepDelay)) {
		t.Fatal("expected authored return delay not to keep hard-coding bootstrap 1s")
	}

	currentTime = currentTime.Add(authoredReturnDelay)
	firstQueued := flushServerFrames(t, flow)
	if len(firstQueued) == 0 {
		t.Fatal("expected first due authored return-step to queue retained owner MOVE replication")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, firstQueued[0]))
	if err != nil {
		t.Fatalf("expected retained authored return-step viewer to receive MOVE replication, first frame decode err=%v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 2201 || moveAck.Y != 2800 {
		t.Fatalf("expected authored return-step MOVE replication at planned -100 toward home, got %+v", moveAck)
	}
	for _, raw := range firstQueued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected authored return-step MOVE fanout not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
	}

	runtime.spawnReturnMu.Lock()
	rearmDueAt, rearmScheduled := runtime.spawnReturnStepDueAt[group.EntityID]
	runtime.spawnReturnMu.Unlock()
	if !rearmScheduled {
		t.Fatalf("expected post-step return re-arm for entity %d", group.EntityID)
	}
	expectedRearmDueAt := currentTime.Add(authoredReturnDelay)
	if !rearmDueAt.Equal(expectedRearmDueAt) {
		t.Fatalf("expected authored return re-arm deadline at %s, got %s", expectedRearmDueAt, rearmDueAt)
	}
}
