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

func TestGameSessionFlowPracticeMobReconnectRestartHereFreshTargetResumesNormalAttack(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ReconnectAttackOwner", 0x010301e1, 0x020401e1, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	watcher := peerVisibilityCharacter("ReconnectAttackWatcher", 0x010301e2, 0x020401e2, 1450, 2200, 2, 102, 202)
	issuePeerTicket(t, store, "reconnect-attack-owner", 0xe1e1e1e1, owner)
	issuePeerTicket(t, store, "reconnect-attack-watcher", 0xe2e2e2e2, watcher)
	if err := accounts.Save(accountstore.Account{Login: "reconnect-attack-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed reconnect attack-resume owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "reconnect-attack-watcher", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed reconnect attack-resume watcher account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error for reconnect attack-resume: %v", err)
	}
	currentTime := time.Unix(1700000997, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.reconnect_restart_here_attack",
		Name:          "ReconnectRestartHereAttackMob",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle for reconnect attack-resume: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import for reconnect attack-resume, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "reconnect-attack-owner", 0xe1e1e1e1)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob before reconnect attack-resume, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, "reconnect-attack-watcher", 0xe2e2e2e2)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob before reconnect attack-resume, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins before reconnect attack-resume, got %d", len(queued))
	}

	advance := func(duration time.Duration) {
		currentTime = currentTime.Add(duration)
	}
	drivePracticeMobOwnerToZeroHPAfterDelayedRetaliation(t, ownerFlow, watcherFlow, targetVID, owner.VID, advance)

	persistedBeforeReconnect, err := accounts.Load("reconnect-attack-owner")
	if err != nil {
		t.Fatalf("load persisted owner account after delayed retaliation floor before reconnect attack-resume: %v", err)
	}
	if len(persistedBeforeReconnect.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after delayed retaliation floor before reconnect attack-resume, got %+v", persistedBeforeReconnect)
	}
	if persistedBeforeReconnect.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected delayed retaliation floor to persist points[%d]=0 before reconnect attack-resume, got %d", bootstrapPlayerPointValueIndex, persistedBeforeReconnect.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	closeSessionFlow(t, ownerFlow)
	leaveQueued := flushServerFrames(t, watcherFlow)
	if len(leaveQueued) != 1 {
		t.Fatalf("expected watcher to receive 1 queued owner delete after abrupt disconnect before reconnect attack-resume, got %d", len(leaveQueued))
	}
	ownerLeaveDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, leaveQueued[0]))
	if err != nil {
		t.Fatalf("decode watcher owner-delete after abrupt disconnect before reconnect attack-resume: %v", err)
	}
	if ownerLeaveDelete.VID != owner.VID {
		t.Fatalf("expected watcher owner-delete for vid %d after abrupt disconnect before reconnect attack-resume, got %+v", owner.VID, ownerLeaveDelete)
	}

	issuePeerTicket(t, store, "reconnect-attack-owner", 0xe3e3e3e3, persistedBeforeReconnect.Characters[0])
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "reconnect-attack-owner", 0xe3e3e3e3)
	defer closeSessionFlow(t, reconnectFlow)
	// Still-dead reconnect with a living visible peer and practice mob rebuilds:
	// 4 self bootstrap frames + floor POINT_CHANGE + self DEAD + 3 peer-entry frames.
	if len(reconnectEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames including self DEAD and living peer entry for reconnecting owner persisted at HP floor before reconnect attack-resume, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after persisted delayed retaliation floor before attack resume: %v", err)
	}
	if reconnectPointChange.Value != 0 || reconnectPointChange.Amount != 0 {
		t.Fatalf("expected reconnect bootstrap to rebuild persisted points[%d] at floor 0 before attack resume, got %+v", bootstrapPlayerPointValueIndex, reconnectPointChange)
	}
	reconnectDead, err := worldproto.DecodeDead(decodeSingleFrame(t, reconnectEnter[5]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap dead replay after persisted delayed retaliation floor before attack resume: %v", err)
	}
	if reconnectDead.VID != owner.VID {
		t.Fatalf("expected reconnect bootstrap dead replay for owner vid %d before attack resume, got %+v", owner.VID, reconnectDead)
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, reconnectEnter[6]))
	if err != nil {
		t.Fatalf("decode living peer add during still-dead reconnect bootstrap before attack resume: %v", err)
	}
	if peerAdd.VID != watcher.VID {
		t.Fatalf("expected still-dead reconnect bootstrap to include living watcher peer vid %d, got %+v", watcher.VID, peerAdd)
	}
	reentryQueued := flushServerFrames(t, watcherFlow)
	if len(reentryQueued) != 4 {
		t.Fatalf("expected watcher to receive 3 queued still-dead owner re-entry frames plus trailing DEAD before reconnect attack-resume, got %d", len(reentryQueued))
	}
	reentryDead, err := worldproto.DecodeDead(decodeSingleFrame(t, reentryQueued[3]))
	if err != nil {
		t.Fatalf("decode watcher trailing dead replay for still-dead reconnect re-entry before attack resume: %v", err)
	}
	if reentryDead.VID != owner.VID {
		t.Fatalf("expected watcher trailing DEAD(owner_vid) for still-dead reconnect re-entry before attack resume, got %+v", reentryDead)
	}

	restartOut, err := reconnectFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_here"})))
	if err != nil {
		t.Fatalf("unexpected /restart_here error after still-dead reconnect before attack resume: %v", err)
	}
	if len(restartOut) != 8 {
		t.Fatalf("expected 4 self bootstrap frames plus 4 visible practice-mob catch-up frames from /restart_here after reconnect before attack resume, got %d", len(restartOut))
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 4 {
		t.Fatalf("expected /restart_here recovery after reconnect to queue 4 peer refresh frames before attack resume, got %d", len(queued))
	}

	staleAttackOut, err := reconnectFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale attack error after reconnect /restart_here before fresh-target attack resume: %v", err)
	}
	if len(staleAttackOut) != 0 {
		t.Fatalf("expected stale attack without fresh target to fail closed after reconnect /restart_here before attack resume, got %d frames", len(staleAttackOut))
	}

	retargetOut, err := reconnectFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected fresh target-selection error after reconnect /restart_here before attack resume: %v", err)
	}
	if len(retargetOut) != 1 {
		t.Fatalf("expected fresh target-selection to succeed after reconnect /restart_here before attack resume, got %d frames", len(retargetOut))
	}
	retarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, retargetOut[0]))
	if err != nil {
		t.Fatalf("decode fresh target-selection after reconnect /restart_here before attack resume: %v", err)
	}
	if retarget.TargetVID != targetVID || retarget.HPPercent != 90 {
		t.Fatalf("expected fresh target-selection after reconnect /restart_here to preserve the still-live practice mob at 90%% HP before attack resume, got %+v", retarget)
	}

	attackOut, err := reconnectFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected resumed normal attack error after reconnect /restart_here fresh target: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected resumed post-reconnect /restart_here attack to return target refresh, retaliation, and damage-info, got %d frames", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode resumed post-reconnect /restart_here attack target refresh: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 80 {
		t.Fatalf("expected resumed post-reconnect /restart_here attack to move the still-live practice mob from 90%% to 80%% HP, got %+v", refresh)
	}
	retaliation, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode resumed post-reconnect /restart_here attack retaliation point-change: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP + bootstrapPracticeMobRetaliationPointDelta
	if retaliation.VID != owner.VID || retaliation.Type != bootstrapPlayerPointType || retaliation.Amount != bootstrapPracticeMobRetaliationPointDelta || retaliation.Value != wantHP {
		t.Fatalf("expected resumed post-reconnect /restart_here attack retaliation to apply delta %d to recovered owner HP, got %+v want value %d", bootstrapPracticeMobRetaliationPointDelta, retaliation, wantHP)
	}
	assertDamageInfoFrame(t, attackOut[2], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "resumed post-reconnect /restart_here mob hit")
	assertDamageInfoFrame(t, attackOut[3], owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "resumed post-reconnect /restart_here owner retaliation")
	flushSpawnBackedAttackPeerDamageInfoFrames(t, watcherFlow, targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "resumed post-reconnect /restart_here attack")
}
