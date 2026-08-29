package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Live delayed server-origin retaliation arming / re-arm must consume the
// profile-authored reaction_delay_ms seam: 2000ms arms and re-arms instead of
// hard-coding bootstrap 1s.
func TestGameRuntimeAuthoredReactionDelayArmsAndRearmsAtTwoSeconds(t *testing.T) {
	const profile = "practice_live_reaction_delay_wolf"
	const authoredReactionDelay = 2 * time.Second

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AuthoredReactionDelayOwner", 0x010303e3, 0x020403e3, 2200, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "authored-reaction-delay-owner", 0xe3e3e3e3, owner)

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
			Ref:           "practice.live_reaction_delay_wolf",
			Name:          "LiveReactionDelayMob",
			MapIndex:      42,
			X:             1950,
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
			ReactionDelayMs: authoredReactionDelay.Milliseconds(),
		}},
	}); err != nil {
		t.Fatalf("import authored reaction-delay spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.live_reaction_delay_wolf")
	if !ok {
		t.Fatal("expected authored reaction-delay spawn group to resolve by ref")
	}
	if got := worldruntime.EffectiveStaticActorSpawnReactionDelay(profile); got != authoredReactionDelay {
		t.Fatalf("expected imported profile effective reaction delay %v, got %v", authoredReactionDelay, got)
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "authored-reaction-delay-owner", 0xe3e3e3e3)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	if selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(selectOut) != 1 {
		t.Fatalf("expected owner to select authored reaction-delay practice mob, got frames=%d err=%v", len(selectOut), err)
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before authored reaction arm: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, immediate retaliation, and damage-info on first reaction-arming hit, got %d frames", len(attackOut))
	}

	wantReadyAt := currentTime.Add(authoredReactionDelay)
	snapshot, ok := runtime.CombatTargetSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected runtime exact-name combat target snapshot after authored reaction arm")
	}
	if !snapshot.RetaliationPending || snapshot.RetaliationReadyAt == nil || !snapshot.RetaliationReadyAt.Equal(wantReadyAt) || snapshot.RetaliationRemainingMs == nil || *snapshot.RetaliationRemainingMs != authoredReactionDelay.Milliseconds() {
		t.Fatalf("expected authored pending retaliation timing ready_at=%s remaining_ms=%d, got %+v", wantReadyAt, authoredReactionDelay.Milliseconds(), snapshot)
	}
	if snapshot.RetaliationReadyAt.Equal(currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)) {
		t.Fatal("expected authored reaction delay not to keep hard-coding bootstrap 1s")
	}
	if snapshot.TargetVID != targetVID {
		t.Fatalf("expected delayed retaliation target %d, got %d", targetVID, snapshot.TargetVID)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected authored 2s reaction delay not to fire at bootstrap 1s, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(authoredReactionDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	queued := flushServerFrames(t, flow)
	if len(queued) != 2 {
		t.Fatalf("expected two delayed self-only retaliation frames after authored delay (point-change + owner damage-info), got %d frames", len(queued))
	}
	delayedRetaliation, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode authored delayed retaliation point change: %v", err)
	}
	if delayedRetaliation.VID != owner.VID || delayedRetaliation.Type != bootstrapPlayerPointType || delayedRetaliation.Amount != -1 || delayedRetaliation.Value != 48 {
		t.Fatalf("expected authored delayed retaliation to lower owner HP to 48, got %+v", delayedRetaliation)
	}
	ownerDamage, err := combatproto.DecodeServerDamageInfo(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode authored delayed retaliation owner damage-info: %v", err)
	}
	if ownerDamage.VID != owner.VID || ownerDamage.Flag != 0 || ownerDamage.Damage != 1 {
		t.Fatalf("unexpected authored delayed retaliation owner damage-info: %+v", ownerDamage)
	}

	nextReadyAt := currentTime.Add(authoredReactionDelay)
	rescheduled, ok := runtime.CombatTargetSnapshot(owner.Name)
	if !ok || !rescheduled.RetaliationPending || rescheduled.RetaliationReadyAt == nil || !rescheduled.RetaliationReadyAt.Equal(nextReadyAt) || rescheduled.RetaliationRemainingMs == nil || *rescheduled.RetaliationRemainingMs != authoredReactionDelay.Milliseconds() {
		t.Fatalf("expected authored reaction re-arm ready_at=%s remaining_ms=%d, ok=%v snapshot=%+v", nextReadyAt, authoredReactionDelay.Milliseconds(), ok, rescheduled)
	}
	if rescheduled.RetaliationReadyAt.Equal(currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)) {
		t.Fatal("expected authored reaction re-arm not to keep hard-coding bootstrap 1s")
	}
}
