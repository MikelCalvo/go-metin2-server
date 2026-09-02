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

func TestGameSessionFlowPracticeMobPhaseSelectRestartHereFreshTargetResumesNormalAttack(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PhaseSelectAttackOwner", 0x010301f1, 0x020401f1, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	watcher := peerVisibilityCharacter("PhaseSelectAttackWatcher", 0x010301f2, 0x020401f2, 1450, 2200, 2, 102, 202)
	issuePeerTicket(t, store, "phase-select-attack-owner", 0xf1f1f1f1, owner)
	issuePeerTicket(t, store, "phase-select-attack-watcher", 0xf2f2f2f2, watcher)
	if err := accounts.Save(accountstore.Account{Login: "phase-select-attack-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed phase-select attack-resume owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "phase-select-attack-watcher", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed phase-select attack-resume watcher account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error for phase-select attack-resume: %v", err)
	}
	currentTime := time.Unix(1700001997, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.phase_select_restart_here_attack",
		Name:          "PhaseSelectRestartHereAttackMob",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle for phase-select attack-resume: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import for phase-select attack-resume, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "phase-select-attack-owner", 0xf1f1f1f1)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob before phase-select attack-resume, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, "phase-select-attack-watcher", 0xf2f2f2f2)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob before phase-select attack-resume, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	defer closeSessionFlow(t, ownerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins before phase-select attack-resume, got %d", len(queued))
	}

	advance := func(duration time.Duration) {
		currentTime = currentTime.Add(duration)
	}
	drivePracticeMobOwnerToZeroHPAfterDelayedRetaliation(t, ownerFlow, watcherFlow, targetVID, owner.VID, advance)

	persistedBeforePhaseSelect, err := accounts.Load("phase-select-attack-owner")
	if err != nil {
		t.Fatalf("load persisted owner account after delayed retaliation floor before phase-select attack-resume: %v", err)
	}
	if len(persistedBeforePhaseSelect.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after delayed retaliation floor before phase-select attack-resume, got %+v", persistedBeforePhaseSelect)
	}
	if persistedBeforePhaseSelect.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected delayed retaliation floor to persist points[%d]=0 before phase-select attack-resume, got %d", bootstrapPlayerPointValueIndex, persistedBeforePhaseSelect.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	phaseSelectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/phase_select"})))
	if err != nil {
		t.Fatalf("unexpected /phase_select error after delayed retaliation floor before attack resume: %v", err)
	}
	if len(phaseSelectOut) == 0 {
		t.Fatal("expected /phase_select frames after delayed retaliation floor before attack resume")
	}
	leaveQueued := flushServerFrames(t, watcherFlow)
	if len(leaveQueued) != 1 {
		t.Fatalf("expected watcher to receive 1 queued owner delete after /phase_select before attack resume, got %d", len(leaveQueued))
	}
	ownerLeaveDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, leaveQueued[0]))
	if err != nil {
		t.Fatalf("decode watcher owner-delete after /phase_select before attack resume: %v", err)
	}
	if ownerLeaveDelete.VID != owner.VID {
		t.Fatalf("expected watcher owner-delete for vid %d after /phase_select before attack resume, got %+v", owner.VID, ownerLeaveDelete)
	}

	selectPhaseOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0})))
	if err != nil {
		t.Fatalf("unexpected character select after delayed retaliation /phase_select before attack resume: %v", err)
	}
	if len(selectPhaseOut) != 3 {
		t.Fatalf("expected 3 character-select frames after delayed retaliation /phase_select before attack resume, got %d", len(selectPhaseOut))
	}

	reenterOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected enter-game after delayed retaliation /phase_select before attack resume: %v", err)
	}
	// Still-dead same-socket re-entry with a living visible peer and practice mob rebuilds:
	// 4 self bootstrap frames + floor POINT_CHANGE + self DEAD + 3 peer-entry frames.
	if len(reenterOut) != 9 {
		t.Fatalf("expected 9 bootstrap frames including self DEAD and living peer entry for /phase_select re-entry persisted at HP floor before attack resume, got %d", len(reenterOut))
	}
	reenterPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reenterOut[4]))
	if err != nil {
		t.Fatalf("decode /phase_select re-entry bootstrap point-change after persisted delayed retaliation floor before attack resume: %v", err)
	}
	if reenterPointChange.Value != 0 || reenterPointChange.Amount != 0 {
		t.Fatalf("expected /phase_select re-entry bootstrap to rebuild persisted points[%d] at floor 0 before attack resume, got %+v", bootstrapPlayerPointValueIndex, reenterPointChange)
	}
	reenterDead, err := worldproto.DecodeDead(decodeSingleFrame(t, reenterOut[5]))
	if err != nil {
		t.Fatalf("decode /phase_select re-entry bootstrap dead replay after persisted delayed retaliation floor before attack resume: %v", err)
	}
	if reenterDead.VID != owner.VID {
		t.Fatalf("expected /phase_select re-entry bootstrap dead replay for owner vid %d before attack resume, got %+v", owner.VID, reenterDead)
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, reenterOut[6]))
	if err != nil {
		t.Fatalf("decode living peer add during still-dead /phase_select re-entry bootstrap before attack resume: %v", err)
	}
	if peerAdd.VID != watcher.VID {
		t.Fatalf("expected still-dead /phase_select re-entry bootstrap to include living watcher peer vid %d, got %+v", watcher.VID, peerAdd)
	}
	reentryQueued := flushServerFrames(t, watcherFlow)
	if len(reentryQueued) != 4 {
		t.Fatalf("expected watcher to receive 3 queued still-dead owner re-entry frames plus trailing DEAD before phase-select attack-resume, got %d", len(reentryQueued))
	}
	reentryDead, err := worldproto.DecodeDead(decodeSingleFrame(t, reentryQueued[3]))
	if err != nil {
		t.Fatalf("decode watcher trailing dead replay for still-dead /phase_select re-entry before attack resume: %v", err)
	}
	if reentryDead.VID != owner.VID {
		t.Fatalf("expected watcher trailing DEAD(owner_vid) for still-dead /phase_select re-entry before attack resume, got %+v", reentryDead)
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_here"})))
	if err != nil {
		t.Fatalf("unexpected /restart_here error after still-dead /phase_select re-entry before attack resume: %v", err)
	}
	if len(restartOut) != 8 {
		t.Fatalf("expected 4 self bootstrap frames plus 4 visible practice-mob catch-up frames from /restart_here after /phase_select before attack resume, got %d", len(restartOut))
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 4 {
		t.Fatalf("expected /restart_here recovery after /phase_select to queue 4 peer refresh frames before attack resume, got %d", len(queued))
	}

	staleAttackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale attack error after /phase_select /restart_here before fresh-target attack resume: %v", err)
	}
	if len(staleAttackOut) != 0 {
		t.Fatalf("expected stale attack without fresh target to fail closed after /phase_select /restart_here before attack resume, got %d frames", len(staleAttackOut))
	}

	retargetOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected fresh target-selection error after /phase_select /restart_here before attack resume: %v", err)
	}
	if len(retargetOut) != 1 {
		t.Fatalf("expected fresh target-selection to succeed after /phase_select /restart_here before attack resume, got %d frames", len(retargetOut))
	}
	retarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, retargetOut[0]))
	if err != nil {
		t.Fatalf("decode fresh target-selection after /phase_select /restart_here before attack resume: %v", err)
	}
	if retarget.TargetVID != targetVID || retarget.HPPercent != 90 {
		t.Fatalf("expected fresh target-selection after /phase_select /restart_here to preserve the still-live practice mob at 90%% HP before attack resume, got %+v", retarget)
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected resumed normal attack error after /phase_select /restart_here fresh target: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected resumed post-/phase_select /restart_here attack to return target refresh, retaliation, and damage-info, got %d frames", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode resumed post-/phase_select /restart_here attack target refresh: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 80 {
		t.Fatalf("expected resumed post-/phase_select /restart_here attack to move the still-live practice mob from 90%% to 80%% HP, got %+v", refresh)
	}
	retaliation, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode resumed post-/phase_select /restart_here attack retaliation point-change: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP + bootstrapPracticeMobRetaliationPointDelta
	if retaliation.VID != owner.VID || retaliation.Type != bootstrapPlayerPointType || retaliation.Amount != bootstrapPracticeMobRetaliationPointDelta || retaliation.Value != wantHP {
		t.Fatalf("expected resumed post-/phase_select /restart_here attack retaliation to apply delta %d to recovered owner HP, got %+v want value %d", bootstrapPracticeMobRetaliationPointDelta, retaliation, wantHP)
	}
	assertDamageInfoFrame(t, attackOut[2], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "resumed post-/phase_select /restart_here mob hit")
	assertDamageInfoFrame(t, attackOut[3], owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "resumed post-/phase_select /restart_here owner retaliation")
	flushSpawnBackedAttackPeerDamageInfoFrames(t, watcherFlow, targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "resumed post-/phase_select /restart_here attack")
}
