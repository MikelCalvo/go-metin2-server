package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobRestartTownDestinationFreshTargetResumesNormalAttack(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RTDestAttackOwner", 0x010301d1, 0x020401d1, 1100, 2100, 0, 101, 201)
	owner.Empire = 2
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	sourceWatcher := peerVisibilityCharacter("RTDestAttackSource", 0x010301d3, 0x020401d3, 1300, 2300, 2, 102, 202)
	// Keep the town peer outside default spawn aggro of the destination practice
	// mob so source-map death-timer advances do not arm destination proximity
	// retaliation against that peer before /restart_town. Whole-map visibility
	// still keeps that peer visible for destination combat fanout.
	townPeer := peerVisibilityCharacter("RTDestAttackTown", 0x010301d2, 0x020401d2, 52300, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	issuePeerTicket(t, store, "rt-dest-attack-owner", 0xd1d1d1d1, owner)
	issuePeerTicket(t, store, "rt-dest-attack-source", 0xd3d3d3d3, sourceWatcher)
	issuePeerTicket(t, store, "rt-dest-attack-town", 0xd2d2d2d2, townPeer)
	if err := accounts.Save(accountstore.Account{Login: "rt-dest-attack-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed /restart_town destination attack-resume owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "rt-dest-attack-source", Empire: sourceWatcher.Empire, Characters: cloneCharacters([]loginticket.Character{sourceWatcher})}); err != nil {
		t.Fatalf("seed /restart_town destination attack-resume source watcher account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "rt-dest-attack-town", Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed /restart_town destination attack-resume town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error for /restart_town destination attack-resume: %v", err)
	}
	currentTime := time.Unix(1700000995, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{
		{
			Ref:           "practice.restart_town_dest_attack_source",
			Name:          "RestartTownDestAttackSourceMob",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		},
		{
			Ref:           "practice.restart_town_dest_attack_destination",
			Name:          "RestartTownDestAttackDestinationMob",
			MapIndex:      21,
			X:             52070,
			Y:             166600,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		},
	}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle for /restart_town destination attack-resume: %v", err)
	}
	sourceGroup, ok := runtime.SpawnGroupByRef("practice.restart_town_dest_attack_source")
	if !ok {
		t.Fatal("expected source practice mob to resolve by ref before /restart_town destination attack-resume")
	}
	destinationGroup, ok := runtime.SpawnGroupByRef("practice.restart_town_dest_attack_destination")
	if !ok {
		t.Fatal("expected destination practice mob to resolve by ref before /restart_town destination attack-resume")
	}
	sourceVID := uint32(sourceGroup.EntityID)
	destinationVID := uint32(destinationGroup.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "rt-dest-attack-owner", 0xd1d1d1d1)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap with visible source practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "rt-dest-attack-source", 0xd3d3d3d3)
	if len(sourceEnter) != 11 {
		t.Fatalf("expected source watcher bootstrap with visible owner and source practice mob, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after source watcher joins, got %d", len(queued))
	}
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "rt-dest-attack-town", 0xd2d2d2d2)
	if len(townEnter) != 8 {
		t.Fatalf("expected destination town peer bootstrap with visible destination practice mob, got %d frames", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued owner frames before floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 0 {
		t.Fatalf("expected destination-map town peer join to avoid queued source frames before floor, got %d", len(queued))
	}
	if queued := flushServerFrames(t, townFlow); len(queued) != 0 {
		t.Fatalf("expected no extra town peer frames before owner floor, got %d", len(queued))
	}

	advance := func(duration time.Duration) {
		currentTime = currentTime.Add(duration)
	}
	drivePracticeMobOwnerToZeroHPAfterDelayedRetaliation(t, ownerFlow, sourceFlow, sourceVID, owner.VID, advance)
	if queued := flushServerFrames(t, townFlow); len(queued) != 0 {
		t.Fatalf("expected destination town peer to stay out of source-map death fanout, got %d queued frames", len(queued))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_town"})))
	if err != nil {
		t.Fatalf("unexpected /restart_town error before destination attack resume: %v", err)
	}
	if len(restartOut) < 8 {
		t.Fatalf("expected /restart_town to return at least self bootstrap plus destination visibility frames before attack resume, got %d frames", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after /restart_town before destination attack resume: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position before destination attack resume, got %+v", selfAdd)
	}
	var (
		foundSourceDelete    bool
		foundDestinationMob  bool
		foundDestinationPeer bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if del, err := worldproto.DecodeCharacterDeleteNotice(fr); err == nil && del.VID == sourceVID {
			foundSourceDelete = true
			continue
		}
		if add, err := worldproto.DecodeCharacterAdd(fr); err == nil {
			switch add.VID {
			case destinationVID:
				if add.X != 52070 || add.Y != 166600 || add.RaceNum != 101 {
					t.Fatalf("expected /restart_town to show live destination mob at authored home before attack resume, got %+v", add)
				}
				foundDestinationMob = true
			case townPeer.VID:
				foundDestinationPeer = true
			}
		}
	}
	if !foundSourceDelete {
		t.Fatalf("expected /restart_town source practice-mob delete for vid %d before destination attack resume", sourceVID)
	}
	if !foundDestinationMob {
		t.Fatalf("expected /restart_town destination practice-mob add for vid %d before destination attack resume", destinationVID)
	}
	if !foundDestinationPeer {
		t.Fatalf("expected /restart_town destination peer add for vid %d before destination attack resume", townPeer.VID)
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source watcher to receive 1 queued owner delete after /restart_town before destination attack resume, got %d", len(queued))
	}
	townQueued := flushServerFrames(t, townFlow)
	if len(townQueued) != 3 {
		t.Fatalf("expected destination town peer to receive 3 queued owner re-entry frames after /restart_town before attack resume, got %d", len(townQueued))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no extra owner queued frames before destination attack resume, got %d", len(queued))
	}

	staleAttackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  sourceVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale source-map attack error after /restart_town before destination attack resume: %v", err)
	}
	if len(staleAttackOut) != 0 {
		t.Fatalf("expected stale source-map attack without fresh target to fail closed after /restart_town before destination attack resume, got %d frames", len(staleAttackOut))
	}

	retargetOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: destinationVID})))
	if err != nil {
		t.Fatalf("unexpected destination fresh target-selection error after /restart_town before attack resume: %v", err)
	}
	if len(retargetOut) != 1 {
		t.Fatalf("expected destination fresh target-selection to succeed after /restart_town before attack resume, got %d frames", len(retargetOut))
	}
	retarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, retargetOut[0]))
	if err != nil {
		t.Fatalf("decode destination fresh target-selection after /restart_town before attack resume: %v", err)
	}
	if retarget.TargetVID != destinationVID || retarget.HPPercent != 100 {
		t.Fatalf("expected destination fresh target-selection after /restart_town to start at full HP before attack resume, got %+v", retarget)
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  destinationVID,
	})))
	if err != nil {
		t.Fatalf("unexpected resumed normal attack error after /restart_town destination fresh target: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected resumed post-/restart_town destination attack to return target refresh, retaliation, and damage-info, got %d frames", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode resumed post-/restart_town destination attack target refresh: %v", err)
	}
	if refresh.TargetVID != destinationVID || refresh.HPPercent != 90 {
		t.Fatalf("expected resumed post-/restart_town destination attack to move the town practice mob from 100%% to 90%% HP, got %+v", refresh)
	}
	retaliation, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode resumed post-/restart_town destination attack retaliation point-change: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP + bootstrapPracticeMobRetaliationPointDelta
	if retaliation.VID != owner.VID || retaliation.Type != bootstrapPlayerPointType || retaliation.Amount != bootstrapPracticeMobRetaliationPointDelta || retaliation.Value != wantHP {
		t.Fatalf("expected resumed post-/restart_town destination attack retaliation to apply delta %d to recovered owner HP, got %+v want value %d", bootstrapPracticeMobRetaliationPointDelta, retaliation, wantHP)
	}
	assertDamageInfoFrame(t, attackOut[2], destinationVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "resumed post-/restart_town destination mob hit")
	assertDamageInfoFrame(t, attackOut[3], owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "resumed post-/restart_town destination owner retaliation")
	flushSpawnBackedAttackPeerDamageInfoFrames(t, townFlow, destinationVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "resumed post-/restart_town destination attack")
}
