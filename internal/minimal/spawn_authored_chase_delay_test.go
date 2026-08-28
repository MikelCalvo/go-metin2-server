package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Live chase arming / post-step re-arm must consume the profile-authored
// chase_delay_ms seam: 2000ms arms and re-arms after the owned 1s retaliation
// beat instead of hard-coding bootstrap 5s.
func TestGameRuntimeAuthoredChaseDelayArmsAndRearmsAtTwoSeconds(t *testing.T) {
	const profile = "practice_live_chase_delay_wolf"
	const authoredChaseDelay = 2 * time.Second

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AuthoredChaseDelayOwner", 0x01030341, 0x02040341, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "authored-chase-delay-owner", 0xe1e1e1e1, owner)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700004400, 0)
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
			Ref:           "practice.live_chase_delay_wolf",
			Name:          "LiveChaseDelayMob",
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
			ChaseDelayMs:   authoredChaseDelay.Milliseconds(),
		}},
	}); err != nil {
		t.Fatalf("import authored chase-delay spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.live_chase_delay_wolf")
	if !ok {
		t.Fatal("expected authored chase-delay spawn group to resolve by ref")
	}
	if got := worldruntime.EffectiveStaticActorSpawnChaseDelay(profile); got != authoredChaseDelay {
		t.Fatalf("expected imported profile effective chase delay %v, got %v", authoredChaseDelay, got)
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "authored-chase-delay-owner", 0xe1e1e1e1)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected owner target error before authored chase arm: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select authored chase-delay practice mob, got %d frames", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before authored chase arm: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, immediate retaliation, and damage-info on first chase-arming hit, got %d frames", len(attackOut))
	}

	runtime.spawnChaseMu.Lock()
	dueAt, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !chaseScheduled {
		t.Fatalf("expected accepted engagement hit to arm pending chase for entity %d", group.EntityID)
	}
	expectedDueAt := currentTime.Add(authoredChaseDelay)
	if !dueAt.Equal(expectedDueAt) {
		t.Fatalf("expected authored chase deadline at %s, got %s", expectedDueAt, dueAt)
	}
	if dueAt.Equal(currentTime.Add(bootstrapSpawnGroupChaseStepDelay)) {
		t.Fatal("expected authored chase delay not to keep hard-coding bootstrap 5s")
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 2 {
		t.Fatalf("expected the owned delayed retaliation beat to fire before the first authored chase step, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(authoredChaseDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	firstQueued := flushServerFrames(t, flow)
	if len(firstQueued) == 0 {
		t.Fatal("expected first due authored chase-step to queue retained owner MOVE replication")
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, firstQueued[0]))
	if err != nil {
		t.Fatalf("expected retained authored chase-step viewer to receive MOVE replication, first frame decode err=%v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 1800 || moveAck.Y != 2800 {
		t.Fatalf("expected authored chase-step MOVE replication at planned +100 toward owner, got %+v", moveAck)
	}
	for _, raw := range firstQueued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected authored chase-step MOVE fanout not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
	}

	runtime.spawnChaseMu.Lock()
	rearmDueAt, rearmScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !rearmScheduled {
		t.Fatalf("expected post-step chase re-arm for entity %d", group.EntityID)
	}
	expectedRearmDueAt := currentTime.Add(authoredChaseDelay)
	if !rearmDueAt.Equal(expectedRearmDueAt) {
		t.Fatalf("expected authored chase re-arm deadline at %s, got %s", expectedRearmDueAt, rearmDueAt)
	}
}
