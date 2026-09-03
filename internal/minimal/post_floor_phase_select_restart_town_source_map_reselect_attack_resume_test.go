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

func TestGameSessionFlowPracticeMobPhaseSelectRestartTownSourceMapReselectResumesNormalAttack(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PsRTSourceOwner", 0x01030221, 0x02040221, 1100, 2100, 0, 101, 201)
	owner.Empire = 2
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	sourceWatcher := peerVisibilityCharacter("PsRTSourcePeer", 0x01030223, 0x02040223, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "ps-rt-source-owner", 0x21212121, owner)
	issuePeerTicket(t, store, "ps-rt-source-peer", 0x23232323, sourceWatcher)
	if err := accounts.Save(accountstore.Account{Login: "ps-rt-source-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed /phase_select /restart_town source-map reselect owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "ps-rt-source-peer", Empire: sourceWatcher.Empire, Characters: cloneCharacters([]loginticket.Character{sourceWatcher})}); err != nil {
		t.Fatalf("seed /phase_select /restart_town source-map reselect source watcher account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error for /phase_select /restart_town source-map reselect: %v", err)
	}
	currentTime := time.Unix(1700002005, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.phase_select_restart_town_source_reselect",
		Name:          "PhaseSelectRestartTownSourceReselectMob",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle for /phase_select /restart_town source-map reselect: %v", err)
	}
	sourceGroup, ok := runtime.SpawnGroupByRef("practice.phase_select_restart_town_source_reselect")
	if !ok {
		t.Fatal("expected source practice mob to resolve by ref before /phase_select /restart_town source-map reselect")
	}
	sourceVID := uint32(sourceGroup.EntityID)

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "ps-rt-source-owner", 0x21212121)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap with visible source practice mob before /phase_select /restart_town source-map reselect, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, factory, "ps-rt-source-peer", 0x23232323)
	if len(sourceEnter) != 11 {
		t.Fatalf("expected source watcher bootstrap with visible owner and source practice mob before /phase_select /restart_town source-map reselect, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after source watcher joins before /phase_select /restart_town source-map reselect, got %d", len(queued))
	}

	advance := func(duration time.Duration) {
		currentTime = currentTime.Add(duration)
	}
	drivePracticeMobOwnerToZeroHPAfterDelayedRetaliation(t, ownerFlow, sourceFlow, sourceVID, owner.VID, advance)

	persistedBeforePhaseSelect, err := accounts.Load("ps-rt-source-owner")
	if err != nil {
		t.Fatalf("load persisted owner account after delayed retaliation floor before /phase_select /restart_town source-map reselect: %v", err)
	}
	if len(persistedBeforePhaseSelect.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after delayed retaliation floor before /phase_select /restart_town source-map reselect, got %+v", persistedBeforePhaseSelect)
	}
	if persistedBeforePhaseSelect.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected delayed retaliation floor to persist points[%d]=0 before /phase_select /restart_town source-map reselect, got %d", bootstrapPlayerPointValueIndex, persistedBeforePhaseSelect.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	phaseSelectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/phase_select"})))
	if err != nil {
		t.Fatalf("unexpected /phase_select error after delayed retaliation floor before source-map reselect: %v", err)
	}
	if len(phaseSelectOut) == 0 {
		t.Fatal("expected /phase_select frames after delayed retaliation floor before source-map reselect")
	}
	leaveQueued := flushServerFrames(t, sourceFlow)
	if len(leaveQueued) != 1 {
		t.Fatalf("expected source watcher to receive 1 queued owner delete after /phase_select before source-map reselect, got %d", len(leaveQueued))
	}
	ownerLeaveDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, leaveQueued[0]))
	if err != nil {
		t.Fatalf("decode source watcher owner-delete after /phase_select before source-map reselect: %v", err)
	}
	if ownerLeaveDelete.VID != owner.VID {
		t.Fatalf("expected source watcher owner-delete for vid %d after /phase_select before source-map reselect, got %+v", owner.VID, ownerLeaveDelete)
	}

	selectPhaseOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0})))
	if err != nil {
		t.Fatalf("unexpected character select after delayed retaliation /phase_select before source-map reselect: %v", err)
	}
	if len(selectPhaseOut) != 3 {
		t.Fatalf("expected 3 character-select frames after delayed retaliation /phase_select before source-map reselect, got %d", len(selectPhaseOut))
	}

	reenterOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected enter-game after delayed retaliation /phase_select before source-map reselect: %v", err)
	}
	// Still-dead same-socket re-entry with a living visible source peer and practice mob rebuilds:
	// 4 self bootstrap frames + floor POINT_CHANGE + self DEAD + 3 peer-entry frames.
	if len(reenterOut) != 9 {
		t.Fatalf("expected 9 bootstrap frames including self DEAD and living source peer entry for /phase_select re-entry persisted at HP floor before source-map reselect, got %d", len(reenterOut))
	}
	reenterPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reenterOut[4]))
	if err != nil {
		t.Fatalf("decode /phase_select re-entry bootstrap point-change after persisted delayed retaliation floor before source-map reselect: %v", err)
	}
	if reenterPointChange.Value != 0 || reenterPointChange.Amount != 0 {
		t.Fatalf("expected /phase_select re-entry bootstrap to rebuild persisted points[%d] at floor 0 before source-map reselect, got %+v", bootstrapPlayerPointValueIndex, reenterPointChange)
	}
	reenterDead, err := worldproto.DecodeDead(decodeSingleFrame(t, reenterOut[5]))
	if err != nil {
		t.Fatalf("decode /phase_select re-entry bootstrap dead replay after persisted delayed retaliation floor before source-map reselect: %v", err)
	}
	if reenterDead.VID != owner.VID {
		t.Fatalf("expected /phase_select re-entry bootstrap dead replay for owner vid %d before source-map reselect, got %+v", owner.VID, reenterDead)
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, reenterOut[6]))
	if err != nil {
		t.Fatalf("decode living source peer add during still-dead /phase_select re-entry bootstrap before source-map reselect: %v", err)
	}
	if peerAdd.VID != sourceWatcher.VID {
		t.Fatalf("expected still-dead reconnect bootstrap to include living source watcher peer vid %d, got %+v", sourceWatcher.VID, peerAdd)
	}
	reentryQueued := flushServerFrames(t, sourceFlow)
	if len(reentryQueued) != 4 {
		t.Fatalf("expected source watcher to receive 3 queued still-dead owner re-entry frames plus trailing DEAD before /phase_select /restart_town source-map reselect, got %d", len(reentryQueued))
	}
	reentryDead, err := worldproto.DecodeDead(decodeSingleFrame(t, reentryQueued[3]))
	if err != nil {
		t.Fatalf("decode source watcher trailing dead replay for still-dead /phase_select re-entry before source-map reselect: %v", err)
	}
	if reentryDead.VID != owner.VID {
		t.Fatalf("expected source watcher trailing DEAD(owner_vid) for still-dead /phase_select re-entry before source-map reselect, got %+v", reentryDead)
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_town"})))
	if err != nil {
		t.Fatalf("unexpected /restart_town error after still-dead /phase_select re-entry before source-map reselect: %v", err)
	}
	if len(restartOut) < 5 {
		t.Fatalf("expected /restart_town after reconnect to return self bootstrap plus source teardown frames before source-map reselect, got %d frames", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after /phase_select /restart_town before source-map reselect: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /phase_select /restart_town self bootstrap at empire town position before source-map reselect, got %+v", selfAdd)
	}
	foundSourceDelete := false
	for _, raw := range restartOut {
		if del, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && del.VID == sourceVID {
			foundSourceDelete = true
			break
		}
	}
	if !foundSourceDelete {
		t.Fatalf("expected /phase_select /restart_town source practice-mob delete for vid %d before source-map reselect", sourceVID)
	}
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source watcher to receive 1 queued owner delete after /phase_select /restart_town before source-map reselect, got %d", len(queued))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no extra owner queued frames before source-map reselect relocate, got %d", len(queued))
	}

	staleAttackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  sourceVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale source-map attack error after /phase_select /restart_town before source-map reselect: %v", err)
	}
	if len(staleAttackOut) != 0 {
		t.Fatalf("expected stale source-map attack without fresh target to fail closed after /phase_select /restart_town before source-map reselect, got %d frames", len(staleAttackOut))
	}

	townRetargetOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: sourceVID})))
	if err != nil {
		t.Fatalf("unexpected town-side source target-selection error after /phase_select /restart_town before source-map reselect: %v", err)
	}
	if len(townRetargetOut) != 0 {
		t.Fatalf("expected town-side source target-selection after reconnect /restart_town to fail closed before source-map reselect, got %d frames", len(townRetargetOut))
	}

	if !runtime.RelocateCharacter(owner.Name, bootstrapMapIndex, owner.X, owner.Y) {
		t.Fatal("expected relocate back to source map to succeed before /phase_select /restart_town source-map reselect")
	}
	ownerRelocateFrames := flushServerFrames(t, ownerFlow)
	if len(ownerRelocateFrames) == 0 {
		t.Fatal("expected owner relocate-back frames to restore source-map visibility before /phase_select /restart_town source-map reselect")
	}
	foundSourceMob := false
	foundSourcePeer := false
	for _, raw := range ownerRelocateFrames {
		add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw))
		if err != nil {
			continue
		}
		switch add.VID {
		case sourceVID:
			if add.X != 1200 || add.Y != 2200 || add.RaceNum != 101 {
				t.Fatalf("expected relocate-back to restore live source practice mob at authored home before /phase_select /restart_town source-map reselect, got %+v", add)
			}
			foundSourceMob = true
		case sourceWatcher.VID:
			foundSourcePeer = true
		}
	}
	if !foundSourceMob {
		t.Fatalf("expected relocate-back source practice-mob add for vid %d before /phase_select /restart_town source-map reselect", sourceVID)
	}
	if !foundSourcePeer {
		t.Fatalf("expected relocate-back source peer add for vid %d before /phase_select /restart_town source-map reselect", sourceWatcher.VID)
	}
	sourceQueued := flushServerFrames(t, sourceFlow)
	if len(sourceQueued) != 3 {
		t.Fatalf("expected source watcher to receive 3 queued owner re-entry frames after relocate-back before /phase_select /restart_town source-map reselect, got %d", len(sourceQueued))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no extra owner queued frames after relocate-back before source-map reselect, got %d", len(queued))
	}

	retargetOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: sourceVID})))
	if err != nil {
		t.Fatalf("unexpected source-map fresh target-selection error after /phase_select /restart_town relocate-back: %v", err)
	}
	if len(retargetOut) != 1 {
		t.Fatalf("expected source-map fresh target-selection to succeed after /phase_select /restart_town relocate-back, got %d frames", len(retargetOut))
	}
	retarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, retargetOut[0]))
	if err != nil {
		t.Fatalf("decode source-map fresh target-selection after /phase_select /restart_town relocate-back: %v", err)
	}
	if retarget.TargetVID != sourceVID || retarget.HPPercent != 90 {
		t.Fatalf("expected source-map fresh target-selection after /phase_select /restart_town relocate-back to preserve the still-live practice mob at 90%% HP, got %+v", retarget)
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  sourceVID,
	})))
	if err != nil {
		t.Fatalf("unexpected resumed normal attack error after /phase_select /restart_town source-map fresh target: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected resumed post-/phase_select /restart_town source-map attack to return target refresh, retaliation, and damage-info, got %d frames", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode resumed post-/phase_select /restart_town source-map attack target refresh: %v", err)
	}
	if refresh.TargetVID != sourceVID || refresh.HPPercent != 80 {
		t.Fatalf("expected resumed post-/phase_select /restart_town source-map attack to move the still-live practice mob from 90%% to 80%% HP, got %+v", refresh)
	}
	retaliation, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode resumed post-/phase_select /restart_town source-map attack retaliation point-change: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP + bootstrapPracticeMobRetaliationPointDelta
	if retaliation.VID != owner.VID || retaliation.Type != bootstrapPlayerPointType || retaliation.Amount != bootstrapPracticeMobRetaliationPointDelta || retaliation.Value != wantHP {
		t.Fatalf("expected resumed post-/phase_select /restart_town source-map attack retaliation to apply delta %d to recovered owner HP, got %+v want value %d", bootstrapPracticeMobRetaliationPointDelta, retaliation, wantHP)
	}
	assertDamageInfoFrame(t, attackOut[2], sourceVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "resumed post-/phase_select /restart_town source-map mob hit")
	assertDamageInfoFrame(t, attackOut[3], owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "resumed post-/phase_select /restart_town source-map owner retaliation")
	flushSpawnBackedAttackPeerDamageInfoFrames(t, sourceFlow, sourceVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "resumed post-/phase_select /restart_town source-map attack")
}
