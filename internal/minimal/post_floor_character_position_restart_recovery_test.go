package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorCharacterPositionFailsClosed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadStanceOwner", 0x01030d10, 0x02040d10, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	peer := peerVisibilityCharacter("DeadStancePeer", 0x01030d11, 0x02040d11, 1120, 2120, 0, 101, 201)
	login := "post-floor-stance-owner"
	loginKey := uint32(0x19191d10)
	peerLogin := "post-floor-stance-peer"
	peerLoginKey := uint32(0x19191d11)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor stance owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed post-floor stance peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected post-floor stance runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_stance",
		Name:          "PracticeMobPostFloorStance",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor stance practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor stance practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, peerLoginKey)
	if len(peerEnter) < 11 {
		t.Fatalf("expected peer bootstrap with visible owner and mob, got %d frames", len(peerEnter))
	}
	defer closeSessionFlow(t, peerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive peer-entry frames before post-floor stance, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 1 {
		t.Fatalf("expected peer DEAD fanout after owner floor before stance denial, got %d", len(queued))
	}

	assertPostFloorCharacterPositionDenied(t, ownerFlow, peerFlow, "post-floor")
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor CHARACTER_POSITION")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor CHARACTER_POSITION: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor CHARACTER_POSITION, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	assertPostFloorCharacterPositionRecovered(t, ownerFlow, peerFlow, owner, "post-restart")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart CHARACTER_POSITION: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after stance floor, got %+v", wantHP, account.Characters[0])
	}
}

func TestGameSessionFlowPostFloorCharacterPositionFailsClosedBeforeRestartTown(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadStanceTownOwner", 0x01030d12, 0x02040d12, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	sourcePeer := peerVisibilityCharacter("DeadStanceTownSource", 0x01030d13, 0x02040d13, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("DeadStanceTownPeer", 0x01030d14, 0x02040d14, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "pf-stance-town"
	loginKey := uint32(0x19191d12)
	sourceLogin := "pf-stance-town-s"
	sourceLoginKey := uint32(0x19191d13)
	townLogin := "pf-stance-town-t"
	townLoginKey := uint32(0x19191d14)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor town stance owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed post-floor town stance source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed post-floor town stance town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected post-floor town stance runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_stance_town",
		Name:          "PracticeMobPostFloorStanceTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor town stance practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor town stance practice mob, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 8 {
		t.Fatalf("expected owner bootstrap with visible practice mob, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 11 {
		t.Fatalf("expected source peer bootstrap with visible owner and mob, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) == 0 {
		t.Fatalf("expected town peer bootstrap frames, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before post-floor town stance, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source peer DEAD fanout after owner floor before town stance denial, got %d", len(queued))
	}

	assertPostFloorCharacterPositionDenied(t, ownerFlow, sourceFlow, "post-floor town")
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town CHARACTER_POSITION")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor CHARACTER_POSITION: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor CHARACTER_POSITION, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor CHARACTER_POSITION /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after stance floor, got %+v", selfAdd)
	}
	var (
		selfPoints  worldproto.PlayerPointChangePacket
		foundPoints bool
	)
	for _, raw := range restartOut {
		fr := decodeSingleFrame(t, raw)
		if !foundPoints {
			if points, err := worldproto.DecodePlayerPointChange(fr); err == nil {
				selfPoints = points
				foundPoints = true
			}
		}
	}
	if !foundPoints {
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after stance floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after stance floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	assertPostFloorCharacterPositionRecovered(t, ownerFlow, townFlow, owner, "post-restart_town")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town CHARACTER_POSITION: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after stance floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after stance floor, got %+v", wantHP, account.Characters[0])
	}
}

func assertPostFloorCharacterPositionDenied(t *testing.T, ownerFlow, peerFlow service.SessionFlow, context string) {
	t.Helper()

	for _, position := range []uint8{
		bootstrapCharacterPositionSittingGround,
		bootstrapCharacterPositionSittingChair,
		bootstrapCharacterPositionGeneral,
	} {
		out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{Position: position})))
		if err != nil {
			t.Fatalf("unexpected %s CHARACTER_POSITION(%d) dispatch error: %v", context, position, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %s CHARACTER_POSITION(%d) to fail closed with no frames, got %d", context, position, len(out))
		}
		if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
			t.Fatalf("expected %s CHARACTER_POSITION(%d) to queue no owner frames, got %d", context, position, len(queued))
		}
		if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
			t.Fatalf("expected %s CHARACTER_POSITION(%d) to queue no peer frames, got %d", context, position, len(queued))
		}
	}
}

func assertPostFloorCharacterPositionRecovered(t *testing.T, ownerFlow, peerFlow service.SessionFlow, owner loginticket.Character, context string) {
	t.Helper()

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{
		Position: bootstrapCharacterPositionSittingGround,
	})))
	if err != nil {
		t.Fatalf("unexpected %s CHARACTER_POSITION recovery: %v", context, err)
	}
	if len(out) != 1 {
		t.Fatalf("expected %s CHARACTER_POSITION to emit one self presentation frame, got %d", context, len(out))
	}
	selfPosition, err := worldproto.DecodeCharacterPosition(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s self CHARACTER_POSITION: %v", context, err)
	}
	if selfPosition.VID != owner.VID || selfPosition.Position != bootstrapCharacterPositionSittingGround {
		t.Fatalf("unexpected %s self CHARACTER_POSITION: %+v", context, selfPosition)
	}
	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 1 {
		t.Fatalf("expected %s CHARACTER_POSITION to queue one peer frame, got %d", context, len(peerQueued))
	}
	peerPosition, err := worldproto.DecodeCharacterPosition(decodeSingleFrame(t, peerQueued[0]))
	if err != nil {
		t.Fatalf("decode %s peer CHARACTER_POSITION: %v", context, err)
	}
	if peerPosition.VID != owner.VID || peerPosition.Position != bootstrapCharacterPositionSittingGround {
		t.Fatalf("unexpected %s peer CHARACTER_POSITION: %+v", context, peerPosition)
	}
}
