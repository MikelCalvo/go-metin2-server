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
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenExchangeShellBeforeRestartExchange(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeRestartImmOwner", 0x01030f80, 0x02040f80, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 125
	partner := peerVisibilityCharacter("ExchangeRestartImmPartner", 0x01030f81, 0x02040f81, 1120, 2120, 0, 101, 201)
	partner.Gold = 22222
	login := "exch-rst-imm"
	loginKey := uint32(0x19190f80)
	partnerLogin := "exch-rst-imm-p"
	partnerLoginKey := uint32(0x19190f81)
	issuePeerTicket(t, store, login, loginKey, owner)
	issuePeerTicket(t, store, partnerLogin, partnerLoginKey, partner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange restart immediate floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: partnerLogin, Empire: partner.Empire, Characters: cloneCharacters([]loginticket.Character{partner})}); err != nil {
		t.Fatalf("seed exchange restart immediate floor-close partner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected exchange restart immediate floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000921, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_exchange_restart_immediate_floor_close",
		Name:          "PracticeMobExchangeRestartImmediateFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import exchange restart immediate floor-close content bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one exchange restart immediate practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 owner bootstrap frames with visible practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	partnerFlow, partnerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), partnerLogin, partnerLoginKey)
	if len(partnerEnter) != 11 {
		t.Fatalf("expected 11 partner bootstrap frames with visible owner and mob, got %d", len(partnerEnter))
	}
	defer closeSessionFlow(t, partnerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive partner peer-entry frames before exchange restart immediate floor-close, got %d", len(queued))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 0 {
		t.Fatalf("expected no initial partner queued frames before exchange restart immediate floor-close, got %d", len(queued))
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: partner.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange start before owner death: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected one owner exchange start frame before death, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], partner.VID, "owner exchange start before death")
	partnerStart := flushServerFrames(t, partnerFlow)
	if len(partnerStart) != 1 {
		t.Fatalf("expected partner exchange start before owner death, got %d", len(partnerStart))
	}
	assertExchangeStartFrame(t, partnerStart[0], owner.VID, "partner exchange start before owner death")

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before exchange restart immediate floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target selection frame before exchange restart immediate floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected exchange restart immediate floor-close attack: %v", err)
	}
	if len(attackOut) != 6 {
		t.Fatalf("expected target refresh, point-change, self dead, clear-target, owner damage-info, and exchange END on owner death, got %d frames", len(attackOut))
	}
	next := assertOwnerFloorDeathSequence(t, attackOut, 1, owner.VID, -1, "exchange_practice_mob_floor_restart_exchange owner-floor")
	assertExchangeEndFrame(t, attackOut[next], "owner exchange END after immediate death")

	partnerQueued := flushServerFrames(t, partnerFlow)
	if len(partnerQueued) != 3 {
		t.Fatalf("expected partner visible DEAD, owner damage-info, plus exchange END after owner death, got %d frames", len(partnerQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, partnerQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "exchange_practice_mob_floor_restart_exchange owner-floor peer")
	if len(remaining) != 1 {
		t.Fatalf("expected exchange END after owner-floor peer DEAD + damage-info, got %d leftover frames", len(remaining))
	}
	assertExchangeEndFrame(t, remaining[0], "partner exchange END after owner death")

	postFloorCancel, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-floor exchange cancel dispatch: %v", err)
	}
	if len(postFloorCancel) != 0 {
		t.Fatalf("expected post-floor exchange cancel to fail closed after death close, got %d frames", len(postFloorCancel))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 0 {
		t.Fatalf("expected no stale partner exchange frames after post-floor cancel, got %d", len(queued))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after exchange restart immediate floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after exchange restart immediate floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	freshStart, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      partner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_here exchange start after exchange immediate floor: %v", err)
	}
	if len(freshStart) != 1 {
		t.Fatalf("expected post-restart_here exchange start to succeed after exchange immediate floor clear, got %d frames", len(freshStart))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, freshStart[0])); err == nil {
		t.Fatalf("expected post-restart_here exchange start after exchange immediate floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, freshStart[0], partner.VID, "post-restart_here exchange start after exchange immediate floor clear")
	partnerFreshStart := flushServerFrames(t, partnerFlow)
	if len(partnerFreshStart) != 1 {
		t.Fatalf("expected partner exchange start after exchange immediate floor clear, got %d", len(partnerFreshStart))
	}
	assertExchangeStartFrame(t, partnerFreshStart[0], owner.VID, "partner exchange start after exchange immediate floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "exchange restart immediate floor close inventory/gold")
}

func TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenExchangeShellBeforeRestartExchange(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeRestartDelOwner", 0x01030f82, 0x02040f82, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Gold = 125
	partner := peerVisibilityCharacter("ExchangeRestartDelPartner", 0x01030f83, 0x02040f83, 1120, 2120, 0, 101, 201)
	partner.Gold = 22222
	login := "exch-rst-del"
	loginKey := uint32(0x19190f82)
	partnerLogin := "exch-rst-del-p"
	partnerLoginKey := uint32(0x19190f83)
	issuePeerTicket(t, store, login, loginKey, owner)
	issuePeerTicket(t, store, partnerLogin, partnerLoginKey, partner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange restart delayed floor-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: partnerLogin, Empire: partner.Empire, Characters: cloneCharacters([]loginticket.Character{partner})}); err != nil {
		t.Fatalf("seed exchange restart delayed floor-close partner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected exchange restart delayed floor-close runtime error: %v", err)
	}
	currentTime := time.Unix(1700000922, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_exchange_restart_delayed_floor_close",
		Name:          "PracticeMobExchangeRestartDelayedFloorClose",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import exchange restart delayed floor-close content bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one exchange restart delayed practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 delayed owner bootstrap frames with visible practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	partnerFlow, partnerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), partnerLogin, partnerLoginKey)
	if len(partnerEnter) != 11 {
		t.Fatalf("expected 11 delayed partner bootstrap frames with visible owner and mob, got %d", len(partnerEnter))
	}
	defer closeSessionFlow(t, partnerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected delayed owner to receive partner peer-entry frames before exchange restart delayed floor-close, got %d", len(queued))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 0 {
		t.Fatalf("expected no initial delayed partner queued frames before exchange restart delayed floor-close, got %d", len(queued))
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: partner.VID})))
	if err != nil {
		t.Fatalf("unexpected delayed exchange start before owner death: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected one delayed owner exchange start frame before death, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], partner.VID, "delayed owner exchange start before death")
	partnerStart := flushServerFrames(t, partnerFlow)
	if len(partnerStart) != 1 {
		t.Fatalf("expected delayed partner exchange start before owner death, got %d", len(partnerStart))
	}
	assertExchangeStartFrame(t, partnerStart[0], owner.VID, "delayed partner exchange start before owner death")

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before delayed exchange restart floor-close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target selection frame before delayed exchange restart floor-close, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected delayed exchange first attack: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-change, and self damage-info before delayed exchange death, got %d frames", len(attackOut))
	}
	firstPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode delayed exchange first point-change: %v", err)
	}
	if firstPoint.VID != owner.VID || firstPoint.Type != bootstrapPlayerPointType || firstPoint.Amount != -1 || firstPoint.Value != 1 {
		t.Fatalf("unexpected delayed exchange first point-change: %+v", firstPoint)
	}
	firstPeerQueued := flushServerFrames(t, partnerFlow)
	if len(firstPeerQueued) != 2 {
		t.Fatalf("expected delayed exchange partner mob + owner retaliation damage-info after first hit, got %d", len(firstPeerQueued))
	}
	assertDamageInfoFrame(t, firstPeerQueued[0], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "delayed exchange partner first hit mob")
	assertDamageInfoFrame(t, firstPeerQueued[1], owner.VID, -bootstrapPracticeMobRetaliationPointDelta, "delayed exchange partner first hit owner retaliation")

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	floorQueued := flushServerFrames(t, ownerFlow)
	if len(floorQueued) != 5 {
		t.Fatalf("expected delayed point-change, self dead, clear-target, owner damage-info, and exchange END on owner death, got %d frames", len(floorQueued))
	}
	next := assertOwnerFloorDeathSequence(t, floorQueued, 0, owner.VID, -1, "exchange_practice_mob_floor_restart_exchange owner-floor")
	assertExchangeEndFrame(t, floorQueued[next], "owner exchange END after delayed death")

	partnerQueued := flushServerFrames(t, partnerFlow)
	if len(partnerQueued) != 3 {
		t.Fatalf("expected partner visible DEAD, owner damage-info, plus exchange END after delayed owner death, got %d frames", len(partnerQueued))
	}
	remaining := assertOwnerFloorPeerDeadFanout(t, partnerQueued, owner.VID, int32(-bootstrapPracticeMobRetaliationPointDelta), "exchange_practice_mob_floor_restart_exchange owner-floor peer")
	if len(remaining) != 1 {
		t.Fatalf("expected exchange END after owner-floor peer DEAD + damage-info, got %d leftover frames", len(remaining))
	}
	assertExchangeEndFrame(t, remaining[0], "partner exchange END after delayed owner death")

	postFloorCancel, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected delayed post-floor exchange cancel dispatch: %v", err)
	}
	if len(postFloorCancel) != 0 {
		t.Fatalf("expected delayed post-floor exchange cancel to fail closed after death close, got %d frames", len(postFloorCancel))
	}
	if queued := flushServerFrames(t, partnerFlow); len(queued) != 0 {
		t.Fatalf("expected no stale delayed partner exchange frames after post-floor cancel, got %d", len(queued))
	}

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after exchange restart delayed floor: %v", err)
	}
	if len(restartOut) < 4 {
		t.Fatalf("expected /restart_here recovery frames after exchange restart delayed floor, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, partnerFlow)

	freshStart, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      partner.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-restart_here exchange start after exchange delayed floor: %v", err)
	}
	if len(freshStart) != 1 {
		t.Fatalf("expected post-restart_here exchange start to succeed after exchange delayed floor clear, got %d frames", len(freshStart))
	}
	if infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, freshStart[0])); err == nil {
		t.Fatalf("expected post-restart_here exchange start after exchange delayed floor clear, got busy info chat %+v", infoChat)
	}
	assertExchangeStartFrame(t, freshStart[0], partner.VID, "post-restart_here exchange start after exchange delayed floor clear")
	partnerFreshStart := flushServerFrames(t, partnerFlow)
	if len(partnerFreshStart) != 1 {
		t.Fatalf("expected partner exchange start after exchange delayed floor clear, got %d", len(partnerFreshStart))
	}
	assertExchangeStartFrame(t, partnerFreshStart[0], owner.VID, "partner exchange start after exchange delayed floor clear")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "exchange restart delayed floor close inventory/gold")
}
