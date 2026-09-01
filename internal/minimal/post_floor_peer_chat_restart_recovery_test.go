package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorPeerFacingChatFailsClosed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadChatOwner", 0x01030c10, 0x02040c10, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.GuildID = 77
	owner.GuildName = "Guild"
	peer := peerVisibilityCharacter("DeadChatPeer", 0x01030c11, 0x02040c11, 1120, 2120, 0, 101, 201)
	peer.GuildID = owner.GuildID
	peer.GuildName = owner.GuildName
	login := "post-floor-peer-chat-owner"
	loginKey := uint32(0x19191c10)
	peerLogin := "post-floor-peer-chat-peer"
	peerLoginKey := uint32(0x19191c11)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerLoginKey, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor peer-chat owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed post-floor peer-chat peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected post-floor peer-chat runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_peer_chat",
		Name:          "PracticeMobPostFloorPeerChat",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor peer-chat practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor peer-chat practice mob, got %#v", actors)
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
		t.Fatalf("expected owner to receive peer-entry frames before post-floor peer chat, got %d", len(queued))
	}
	_ = flushServerFrames(t, peerFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 1 {
		t.Fatalf("expected peer DEAD fanout after owner floor before peer chat, got %d", len(queued))
	}

	assertPostFloorPeerFacingChatDenied(t, ownerFlow, peerFlow, peer.Name, "post-floor")
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor peer chat")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_here",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_here after post-floor peer chat: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_here recovery frames after post-floor peer chat, got %d", len(restartOut))
	}
	_ = flushServerFrames(t, peerFlow)

	assertPostFloorPeerFacingChatRecovered(t, ownerFlow, peerFlow, owner, peer, "post-restart")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart peer chat: %v", err)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_here to persist recovered owner HP %d after peer-chat floor, got %+v", wantHP, account.Characters[0])
	}
}

func TestGameSessionFlowPostFloorPeerFacingChatFailsClosedBeforeRestartTown(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadChatTownOwner", 0x01030c12, 0x02040c12, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.GuildID = 77
	owner.GuildName = "Guild"
	sourcePeer := peerVisibilityCharacter("DeadChatTownSource", 0x01030c13, 0x02040c13, 1120, 2120, 0, 101, 201)
	sourcePeer.GuildID = owner.GuildID
	sourcePeer.GuildName = owner.GuildName
	townPeer := peerVisibilityCharacter("DeadChatTownPeer", 0x01030c14, 0x02040c14, 52070, 166600, 4, 103, 203)
	townPeer.MapIndex = 21
	townPeer.GuildID = owner.GuildID
	townPeer.GuildName = owner.GuildName
	login := "pf-peer-chat-town"
	loginKey := uint32(0x19191c12)
	sourceLogin := "pf-peer-chat-town-s"
	sourceLoginKey := uint32(0x19191c13)
	townLogin := "pf-peer-chat-town-t"
	townLoginKey := uint32(0x19191c14)
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	issuePeerTicket(t, ticketStore, sourceLogin, sourceLoginKey, sourcePeer)
	issuePeerTicket(t, ticketStore, townLogin, townLoginKey, townPeer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor town peer-chat owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: sourceLogin, Empire: sourcePeer.Empire, Characters: cloneCharacters([]loginticket.Character{sourcePeer})}); err != nil {
		t.Fatalf("seed post-floor town peer-chat source peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: townLogin, Empire: townPeer.Empire, Characters: cloneCharacters([]loginticket.Character{townPeer})}); err != nil {
		t.Fatalf("seed post-floor town peer-chat town peer account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected post-floor town peer-chat runtime error: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_peer_chat_town",
		Name:          "PracticeMobPostFloorPeerChatTown",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import post-floor town peer-chat practice mob: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor town peer-chat practice mob, got %#v", actors)
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
		t.Fatalf("expected owner to receive source peer-entry frames before post-floor town peer chat, got %d", len(queued))
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, ownerFlow, owner, targetVID)
	if queued := flushServerFrames(t, sourceFlow); len(queued) != 1 {
		t.Fatalf("expected source peer DEAD fanout after owner floor before town peer chat, got %d", len(queued))
	}

	assertPostFloorPeerFacingChatDenied(t, ownerFlow, sourceFlow, sourcePeer.Name, "post-floor town")
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor town peer chat")

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/restart_town",
	})))
	if err != nil {
		t.Fatalf("unexpected /restart_town after post-floor peer chat: %v", err)
	}
	if len(restartOut) == 0 {
		t.Fatalf("expected /restart_town recovery frames after post-floor peer chat, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after post-floor peer chat /restart_town: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 52070 || selfAdd.Y != 166600 {
		t.Fatalf("expected /restart_town self bootstrap at empire town position after peer-chat floor, got %+v", selfAdd)
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
		t.Fatal("expected /restart_town recovery to include self PLAYER_POINT_CHANGE after peer-chat floor")
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	if selfPoints.Value != wantHP {
		t.Fatalf("expected /restart_town to rebuild recovered owner HP %d after peer-chat floor, got %+v", wantHP, selfPoints)
	}
	_ = flushServerFrames(t, sourceFlow)
	_ = flushServerFrames(t, townFlow)

	assertPostFloorPeerFacingChatRecovered(t, ownerFlow, townFlow, owner, townPeer, "post-restart_town")

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load account after post-restart_town peer chat: %v", err)
	}
	if account.Characters[0].MapIndex != 21 || account.Characters[0].X != 52070 || account.Characters[0].Y != 166600 {
		t.Fatalf("expected /restart_town to persist empire town position after peer-chat floor, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected /restart_town to persist recovered owner HP %d after peer-chat floor, got %+v", wantHP, account.Characters[0])
	}
}

func assertPostFloorPeerFacingChatDenied(t *testing.T, ownerFlow, peerFlow service.SessionFlow, peerName, context string) {
	t.Helper()

	chatCases := []struct {
		name     string
		chatType uint8
		message  string
	}{
		{name: "talking", chatType: chatproto.ChatTypeTalking, message: "hola local"},
		{name: "party", chatType: chatproto.ChatTypeParty, message: "hola party"},
		{name: "guild", chatType: chatproto.ChatTypeGuild, message: "hola guild"},
		{name: "shout", chatType: chatproto.ChatTypeShout, message: "hola shout"},
		{name: "info", chatType: chatproto.ChatTypeInfo, message: "mensaje info"},
	}
	for _, tc := range chatCases {
		t.Run(context+" "+tc.name, func(t *testing.T) {
			out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    tc.chatType,
				Message: tc.message,
			})))
			if err != nil {
				t.Fatalf("unexpected %s %s chat dispatch error: %v", context, tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s %s chat to fail closed with no frames, got %d", context, tc.name, len(out))
			}
			if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
				t.Fatalf("expected %s %s chat to queue no owner frames, got %d", context, tc.name, len(queued))
			}
			if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
				t.Fatalf("expected %s %s chat to queue no peer frames, got %d", context, tc.name, len(queued))
			}
		})
	}

	whisperOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{
		Target:  peerName,
		Message: "hola privado",
	})))
	if err != nil {
		t.Fatalf("unexpected %s whisper dispatch error: %v", context, err)
	}
	if len(whisperOut) != 0 {
		t.Fatalf("expected %s whisper to fail closed with no frames, got %d", context, len(whisperOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected %s whisper to queue no owner frames, got %d", context, len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected %s whisper to queue no peer frames, got %d", context, len(queued))
	}

	missingOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{
		Target:  "GhostPlayer",
		Message: "still there?",
	})))
	if err != nil {
		t.Fatalf("unexpected %s missing-target whisper dispatch error: %v", context, err)
	}
	if len(missingOut) != 0 {
		t.Fatalf("expected %s missing-target whisper to fail closed without NOT_EXIST fallback, got %d", context, len(missingOut))
	}
}

func assertPostFloorPeerFacingChatRecovered(t *testing.T, ownerFlow, peerFlow service.SessionFlow, owner, peer loginticket.Character, context string) {
	t.Helper()

	talkOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "hola despues",
	})))
	if err != nil {
		t.Fatalf("unexpected %s talking chat: %v", context, err)
	}
	if len(talkOut) != 1 {
		t.Fatalf("expected %s talking chat to emit one self frame, got %d", context, len(talkOut))
	}
	selfChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, talkOut[0]))
	if err != nil {
		t.Fatalf("decode %s talking self chat: %v", context, err)
	}
	wantTalk := owner.Name + " : hola despues"
	if selfChat.Type != chatproto.ChatTypeTalking || selfChat.VID != owner.VID || selfChat.Message != wantTalk {
		t.Fatalf("unexpected %s talking self chat: %+v", context, selfChat)
	}
	peerChat := flushServerFrames(t, peerFlow)
	if len(peerChat) != 1 {
		t.Fatalf("expected %s talking chat to queue one peer frame, got %d", context, len(peerChat))
	}
	peerDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerChat[0]))
	if err != nil {
		t.Fatalf("decode %s talking peer chat: %v", context, err)
	}
	if peerDelivery.Type != chatproto.ChatTypeTalking || peerDelivery.VID != owner.VID || peerDelivery.Message != wantTalk {
		t.Fatalf("unexpected %s talking peer chat: %+v", context, peerDelivery)
	}

	infoOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeInfo,
		Message: "mensaje info despues",
	})))
	if err != nil {
		t.Fatalf("unexpected %s info chat: %v", context, err)
	}
	if len(infoOut) != 1 {
		t.Fatalf("expected %s info chat to emit one self frame, got %d", context, len(infoOut))
	}
	infoDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, infoOut[0]))
	if err != nil {
		t.Fatalf("decode %s info chat: %v", context, err)
	}
	if infoDelivery.Type != chatproto.ChatTypeInfo || infoDelivery.VID != 0 || infoDelivery.Message != "mensaje info despues" {
		t.Fatalf("unexpected %s info chat: %+v", context, infoDelivery)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected %s info chat to queue no peer frames, got %d", context, len(queued))
	}

	whisperOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{
		Target:  peer.Name,
		Message: "hola privado despues",
	})))
	if err != nil {
		t.Fatalf("unexpected %s whisper: %v", context, err)
	}
	if len(whisperOut) != 0 {
		t.Fatalf("expected %s whisper success to emit no direct sender frames, got %d", context, len(whisperOut))
	}
	recipientWhisper := flushServerFrames(t, peerFlow)
	if len(recipientWhisper) != 1 {
		t.Fatalf("expected %s whisper to queue one recipient frame, got %d", context, len(recipientWhisper))
	}
	whisperDelivery, err := chatproto.DecodeServerWhisper(decodeSingleFrame(t, recipientWhisper[0]))
	if err != nil {
		t.Fatalf("decode %s recipient whisper: %v", context, err)
	}
	if whisperDelivery.Type != chatproto.WhisperTypeChat || whisperDelivery.FromName != owner.Name || whisperDelivery.Message != "hola privado despues" {
		t.Fatalf("unexpected %s recipient whisper: %+v", context, whisperDelivery)
	}
}
