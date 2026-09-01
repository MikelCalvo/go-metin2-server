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
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorInteractFailsClosed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadInteractOwner", 0x01030e10, 0x02040e10, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	peer := peerVisibilityCharacter("DeadInteractPeer", 0x01030e11, 0x02040e11, 1120, 2120, 0, 101, 201)
	login := "post-floor-interact-owner"
	loginKey := uint32(0x19191e10)
	peerLogin := "post-floor-interact-peer"
	peerLoginKey := uint32(0x19191e11)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor interact owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed post-floor interact peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindTalk,
		Ref:  "npc:guard",
		Text: "Keep your blade sharp.",
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected post-floor interact runtime error: %v", err)
	}
	currentTime := time.Unix(1700000470, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "VillageGuard",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindTalk,
			InteractionRef:  "npc:guard",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_post_floor_interact",
			Name:          "PracticeMobPostFloorInteract",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind: interactionstore.KindTalk,
			Ref:  "npc:guard",
			Text: "Keep your blade sharp.",
		}},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor interact bundle: %v", err)
	}
	var (
		guardEntityID uint64
		targetVID     uint32
	)
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "PracticeMobPostFloorInteract":
			targetVID = uint32(actor.EntityID)
		case "VillageGuard":
			guardEntityID = actor.EntityID
		}
	}
	if guardEntityID == 0 || targetVID == 0 {
		t.Fatalf("expected talk guard and practice mob before post-floor interact recovery, got %#v", runtime.StaticActors())
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 11 {
		t.Fatalf("expected owner bootstrap with visible practice mob and guard, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, peerLoginKey)
	if len(peerEnter) < 11 {
		t.Fatalf("expected peer bootstrap with visible owner and actors, got %d frames", len(peerEnter))
	}
	defer closeSessionFlow(t, peerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive peer-entry frames before post-floor interact, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 1 {
		t.Fatalf("expected peer DEAD fanout after owner floor before interact denial, got %d", len(queued))
	}

	assertPostFloorInteractDenied(t, ownerFlow, guardEntityID, "post-floor")
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor INTERACT")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor INTERACT: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor INTERACT, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	currentTime = currentTime.Add(staticActorInteractionCooldown + time.Nanosecond)
	assertPostFloorInteractRecovered(t, ownerFlow, guardEntityID, "VillageGuard:\nKeep your blade sharp.", "post-restart")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart INTERACT: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after interact floor, got %+v", wantHP, account.Characters[0])
	}
}

func TestGameSessionFlowPostFloorInteractFailsClosedBeforeRestartTown(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadInteractTownOwner", 0x01030e12, 0x02040e12, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	sourcePeer := peerVisibilityCharacter("DeadInteractTownSource", 0x01030e13, 0x02040e13, 1120, 2120, 0, 101, 201)
	townPeer := peerVisibilityCharacter("DeadInteractTownPeer", 0x01030e14, 0x02040e14, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	login := "pf-interact-town"
	loginKey := uint32(0x19191e12)
	sourceLogin := "pf-interact-town-s"
	sourceLoginKey := uint32(0x19191e13)
	townLogin := "pf-interact-town-t"
	townLoginKey := uint32(0x19191e14)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor town interact owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed post-floor town interact source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed post-floor town interact town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindTalk,
		Ref:  "npc:guard",
		Text: "Keep your blade sharp.",
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected post-floor town interact runtime error: %v", err)
	}
	currentTime := time.Unix(1700000471, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "VillageGuard",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindTalk,
			InteractionRef:  "npc:guard",
		}, {
			Name:            "TownGuard",
			MapIndex:        21,
			X:               52100,
			Y:               166650,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindTalk,
			InteractionRef:  "npc:guard",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_post_floor_interact_town",
			Name:          "PracticeMobPostFloorInteractTown",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		InteractionDefinitions: []interactionstore.Definition{{
			Kind: interactionstore.KindTalk,
			Ref:  "npc:guard",
			Text: "Keep your blade sharp.",
		}},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor town interact bundle: %v", err)
	}
	var (
		sourceGuardEntityID uint64
		townGuardEntityID   uint64
		targetVID           uint32
	)
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "PracticeMobPostFloorInteractTown":
			targetVID = uint32(actor.EntityID)
		case "VillageGuard":
			sourceGuardEntityID = actor.EntityID
		case "TownGuard":
			townGuardEntityID = actor.EntityID
		}
	}
	if sourceGuardEntityID == 0 || townGuardEntityID == 0 || targetVID == 0 {
		t.Fatalf("expected source guard, town guard, and practice mob before post-floor town interact recovery, got %#v", runtime.StaticActors())
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(ownerEnter) < 11 {
		t.Fatalf("expected owner bootstrap with visible practice mob and guard, got %d frames", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	sourceFlow, sourceEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), sourceLogin, sourceLoginKey)
	if len(sourceEnter) < 11 {
		t.Fatalf("expected source peer bootstrap with visible owner and actors, got %d frames", len(sourceEnter))
	}
	defer closeSessionFlow(t, sourceFlow)
	townFlow, townEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), townLogin, townLoginKey)
	if len(townEnter) == 0 {
		t.Fatalf("expected town peer bootstrap frames, got %d", len(townEnter))
	}
	defer closeSessionFlow(t, townFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive source peer-entry frames before post-floor town interact, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source peer DEAD fanout after owner floor before town interact denial, got %d", len(queued))
	}

	assertPostFloorInteractDenied(t, ownerFlow, sourceGuardEntityID, "post-floor town")
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town INTERACT")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor INTERACT: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor INTERACT, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor INTERACT /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after interact floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after interact floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after interact floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, townFlow)

	currentTime = currentTime.Add(staticActorInteractionCooldown + time.Nanosecond)
	assertPostFloorInteractRecovered(t, ownerFlow, townGuardEntityID, "TownGuard:\nKeep your blade sharp.", "post-restart_town")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town INTERACT: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after interact floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to leave recovered HP %d unchanged after interact recovery, got %+v", wantHP, account.Characters[0])
	}
}

func assertPostFloorInteractDenied(t *testing.T, ownerFlow service.SessionFlow, targetEntityID uint64, context string) {
	t.Helper()
	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(targetEntityID)})))
	if err != nil {
		t.Fatalf("unexpected %s INTERACT dispatch error: %v", context, err)
	}
	if len(out) != 0 {
		t.Fatalf("expected %s INTERACT to fail closed with no frames, got %d", context, len(out))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected %s INTERACT to queue no frames, got %d", context, len(queued))
	}
}

func assertPostFloorInteractRecovered(t *testing.T, ownerFlow service.SessionFlow, targetEntityID uint64, wantMessage, context string) {
	t.Helper()
	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(targetEntityID)})))
	if err != nil {
		t.Fatalf("unexpected %s INTERACT: %v", context, err)
	}
	if len(out) != 1 {
		t.Fatalf("expected %s INTERACT to emit one self-only talk frame, got %d", context, len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s talk interaction chat delivery: %v", context, err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != wantMessage {
		t.Fatalf("unexpected %s talk interaction chat delivery: %+v", context, delivery)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected %s INTERACT to queue no peer frames, got %d", context, len(queued))
	}
}
