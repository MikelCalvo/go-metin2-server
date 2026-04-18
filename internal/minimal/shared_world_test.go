package minimal

import (
	"io"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
)

func TestNewGameSessionFactoryIncludesExistingPeerInSecondPlayerBootstrap(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	_, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for first player, got %d", len(firstEnter))
	}

	_, secondEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(secondEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for second player with peer snapshot, got %d", len(secondEnter))
	}

	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, secondEnter[5]))
	if err != nil {
		t.Fatalf("decode peer character add: %v", err)
	}
	if peerAdd.VID != peerOne.VID || peerAdd.X != peerOne.X || peerAdd.Y != peerOne.Y || peerAdd.RaceNum != peerOne.RaceNum {
		t.Fatalf("unexpected peer add packet: %+v", peerAdd)
	}

	peerInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, secondEnter[6]))
	if err != nil {
		t.Fatalf("decode peer character additional info: %v", err)
	}
	if peerInfo.VID != peerOne.VID || peerInfo.Name != peerOne.Name || peerInfo.Parts[0] != peerOne.MainPart || peerInfo.Parts[3] != peerOne.HairPart {
		t.Fatalf("unexpected peer additional info packet: %+v", peerInfo)
	}

	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, secondEnter[7]))
	if err != nil {
		t.Fatalf("decode peer character update: %v", err)
	}
	if peerUpdate.VID != peerOne.VID || peerUpdate.Parts[0] != peerOne.MainPart || peerUpdate.Parts[3] != peerOne.HairPart {
		t.Fatalf("unexpected peer update packet: %+v", peerUpdate)
	}
}

func TestNewGameSessionFactoryQueuesPeerEntryAndExitForExistingPlayer(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)

	peerEntry := flushServerFrames(t, flowOne)
	if len(peerEntry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames, got %d", len(peerEntry))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerEntry[0]))
	if err != nil {
		t.Fatalf("decode queued peer add: %v", err)
	}
	if peerAdd.VID != peerTwo.VID {
		t.Fatalf("expected queued peer add for VID %#08x, got %#08x", peerTwo.VID, peerAdd.VID)
	}

	closeSessionFlow(t, flowTwo)

	peerExit := flushServerFrames(t, flowOne)
	if len(peerExit) != 1 {
		t.Fatalf("expected 1 queued peer-exit frame, got %d", len(peerExit))
	}
	removed, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, peerExit[0]))
	if err != nil {
		t.Fatalf("decode queued peer delete: %v", err)
	}
	if removed.VID != peerTwo.VID {
		t.Fatalf("expected queued peer delete for VID %#08x, got %#08x", peerTwo.VID, removed.VID)
	}
}

func TestNewGameSessionFactoryQueuesPeerMoveForVisiblePlayers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	moveOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame, got %d", len(moveOut))
	}
	selfAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode self move ack: %v", err)
	}
	if selfAck.VID != peerTwo.VID || selfAck.X != 1500 || selfAck.Y != 2600 {
		t.Fatalf("unexpected self move ack: %+v", selfAck)
	}

	peerMove := flushServerFrames(t, flowOne)
	if len(peerMove) != 1 {
		t.Fatalf("expected 1 queued peer move frame, got %d", len(peerMove))
	}
	peerAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, peerMove[0]))
	if err != nil {
		t.Fatalf("decode peer move ack: %v", err)
	}
	if peerAck.VID != peerTwo.VID || peerAck.X != 1500 || peerAck.Y != 2600 || peerAck.Time != 0x11121314 {
		t.Fatalf("unexpected peer move ack: %+v", peerAck)
	}
}

func TestNewGameSessionFactoryQueuesPeerSyncPositionForVisiblePlayers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	syncOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: peerTwo.VID, X: 1700, Y: 2800}},
	})))
	if err != nil {
		t.Fatalf("unexpected sync_position error: %v", err)
	}
	if len(syncOut) != 1 {
		t.Fatalf("expected 1 self sync_position ack frame, got %d", len(syncOut))
	}
	selfAck, err := movep.DecodeSyncPositionAck(decodeSingleFrame(t, syncOut[0]))
	if err != nil {
		t.Fatalf("decode self sync_position ack: %v", err)
	}
	if len(selfAck.Elements) != 1 {
		t.Fatalf("expected 1 self sync_position ack element, got %d", len(selfAck.Elements))
	}
	if selfAck.Elements[0].VID != peerTwo.VID || selfAck.Elements[0].X != 1700 || selfAck.Elements[0].Y != 2800 {
		t.Fatalf("unexpected self sync_position ack: %+v", selfAck)
	}

	peerSync := flushServerFrames(t, flowOne)
	if len(peerSync) != 1 {
		t.Fatalf("expected 1 queued peer sync_position frame, got %d", len(peerSync))
	}
	peerAck, err := movep.DecodeSyncPositionAck(decodeSingleFrame(t, peerSync[0]))
	if err != nil {
		t.Fatalf("decode peer sync_position ack: %v", err)
	}
	if len(peerAck.Elements) != 1 {
		t.Fatalf("expected 1 queued peer sync_position element, got %d", len(peerAck.Elements))
	}
	if peerAck.Elements[0].VID != peerTwo.VID || peerAck.Elements[0].X != 1700 || peerAck.Elements[0].Y != 2800 {
		t.Fatalf("unexpected peer sync_position ack: %+v", peerAck)
	}
}

func TestNewGameSessionFactoryQueuesPeerChatForVisiblePlayers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	chatOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "hola"})))
	if err != nil {
		t.Fatalf("unexpected chat error: %v", err)
	}
	if len(chatOut) != 1 {
		t.Fatalf("expected 1 self chat frame, got %d", len(chatOut))
	}
	selfChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, chatOut[0]))
	if err != nil {
		t.Fatalf("decode self chat delivery: %v", err)
	}
	if selfChat.Type != chatproto.ChatTypeTalking || selfChat.VID != peerTwo.VID || selfChat.Message != "PeerTwo : hola" {
		t.Fatalf("unexpected self chat delivery: %+v", selfChat)
	}

	peerChat := flushServerFrames(t, flowOne)
	if len(peerChat) != 1 {
		t.Fatalf("expected 1 queued peer chat frame, got %d", len(peerChat))
	}
	peerDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerChat[0]))
	if err != nil {
		t.Fatalf("decode peer chat delivery: %v", err)
	}
	if peerDelivery.Type != chatproto.ChatTypeTalking || peerDelivery.VID != peerTwo.VID || peerDelivery.Message != "PeerTwo : hola" {
		t.Fatalf("unexpected peer chat delivery: %+v", peerDelivery)
	}
}

func TestNewGameSessionFactoryRoutesWhisperToNamedPeer(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	whisperOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{Target: "PeerOne", Message: "hola privado"})))
	if err != nil {
		t.Fatalf("unexpected whisper error: %v", err)
	}
	if len(whisperOut) != 0 {
		t.Fatalf("expected no direct sender whisper frames on success, got %d", len(whisperOut))
	}

	recipientWhisper := flushServerFrames(t, flowOne)
	if len(recipientWhisper) != 1 {
		t.Fatalf("expected 1 queued whisper frame for target, got %d", len(recipientWhisper))
	}
	delivery, err := chatproto.DecodeServerWhisper(decodeSingleFrame(t, recipientWhisper[0]))
	if err != nil {
		t.Fatalf("decode recipient whisper: %v", err)
	}
	if delivery.Type != chatproto.WhisperTypeChat || delivery.FromName != "PeerTwo" || delivery.Message != "hola privado" {
		t.Fatalf("unexpected recipient whisper: %+v", delivery)
	}
}

func TestNewGameSessionFactoryQueuesPartyChatForConnectedPeers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	partyOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeParty, Message: "hola party"})))
	if err != nil {
		t.Fatalf("unexpected party chat error: %v", err)
	}
	if len(partyOut) != 1 {
		t.Fatalf("expected 1 sender party chat frame, got %d", len(partyOut))
	}
	selfParty, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, partyOut[0]))
	if err != nil {
		t.Fatalf("decode sender party chat: %v", err)
	}
	if selfParty.Type != chatproto.ChatTypeParty || selfParty.VID != peerTwo.VID || selfParty.Message != "PeerTwo : hola party" {
		t.Fatalf("unexpected sender party chat: %+v", selfParty)
	}

	peerParty := flushServerFrames(t, flowOne)
	if len(peerParty) != 1 {
		t.Fatalf("expected 1 queued party chat frame, got %d", len(peerParty))
	}
	peerDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerParty[0]))
	if err != nil {
		t.Fatalf("decode peer party chat: %v", err)
	}
	if peerDelivery.Type != chatproto.ChatTypeParty || peerDelivery.VID != peerTwo.VID || peerDelivery.Message != "PeerTwo : hola party" {
		t.Fatalf("unexpected peer party chat delivery: %+v", peerDelivery)
	}
}

func TestNewGameSessionFactoryQueuesGuildChatForConnectedPeers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	guildOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeGuild, Message: "hola guild"})))
	if err != nil {
		t.Fatalf("unexpected guild chat error: %v", err)
	}
	if len(guildOut) != 1 {
		t.Fatalf("expected 1 sender guild chat frame, got %d", len(guildOut))
	}
	selfGuild, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, guildOut[0]))
	if err != nil {
		t.Fatalf("decode sender guild chat: %v", err)
	}
	if selfGuild.Type != chatproto.ChatTypeGuild || selfGuild.VID != peerTwo.VID || selfGuild.Message != "PeerTwo : hola guild" {
		t.Fatalf("unexpected sender guild chat: %+v", selfGuild)
	}

	peerGuild := flushServerFrames(t, flowOne)
	if len(peerGuild) != 1 {
		t.Fatalf("expected 1 queued guild chat frame, got %d", len(peerGuild))
	}
	peerDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerGuild[0]))
	if err != nil {
		t.Fatalf("decode peer guild chat: %v", err)
	}
	if peerDelivery.Type != chatproto.ChatTypeGuild || peerDelivery.VID != peerTwo.VID || peerDelivery.Message != "PeerTwo : hola guild" {
		t.Fatalf("unexpected peer guild chat delivery: %+v", peerDelivery)
	}
}

func TestNewGameSessionFactoryQueuesShoutChatForConnectedPeers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	shoutOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeShout, Message: "hola shout"})))
	if err != nil {
		t.Fatalf("unexpected shout chat error: %v", err)
	}
	if len(shoutOut) != 1 {
		t.Fatalf("expected 1 sender shout chat frame, got %d", len(shoutOut))
	}
	selfShout, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, shoutOut[0]))
	if err != nil {
		t.Fatalf("decode sender shout chat: %v", err)
	}
	if selfShout.Type != chatproto.ChatTypeShout || selfShout.VID != peerTwo.VID || selfShout.Message != "PeerTwo : hola shout" {
		t.Fatalf("unexpected sender shout chat: %+v", selfShout)
	}

	peerShout := flushServerFrames(t, flowOne)
	if len(peerShout) != 1 {
		t.Fatalf("expected 1 queued shout chat frame, got %d", len(peerShout))
	}
	peerDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerShout[0]))
	if err != nil {
		t.Fatalf("decode peer shout chat: %v", err)
	}
	if peerDelivery.Type != chatproto.ChatTypeShout || peerDelivery.VID != peerTwo.VID || peerDelivery.Message != "PeerTwo : hola shout" {
		t.Fatalf("unexpected peer shout chat delivery: %+v", peerDelivery)
	}
}

func TestNewGameSessionFactoryReturnsInfoChatOnlyToSenderAsSystemMessage(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	infoOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeInfo, Message: "mensaje info"})))
	if err != nil {
		t.Fatalf("unexpected info chat error: %v", err)
	}
	if len(infoOut) != 1 {
		t.Fatalf("expected 1 sender info chat frame, got %d", len(infoOut))
	}
	selfInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, infoOut[0]))
	if err != nil {
		t.Fatalf("decode sender info chat: %v", err)
	}
	if selfInfo.Type != chatproto.ChatTypeInfo || selfInfo.VID != 0 || selfInfo.Message != "mensaje info" {
		t.Fatalf("unexpected sender info chat: %+v", selfInfo)
	}

	peerInfo := flushServerFrames(t, flowOne)
	if len(peerInfo) != 0 {
		t.Fatalf("expected no queued info chat frames for peers, got %d", len(peerInfo))
	}
}

func TestNewGameSessionFactoryQueuesNoticeChatAsSystemBroadcast(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	noticeOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeNotice, Message: "mensaje notice"})))
	if err != nil {
		t.Fatalf("unexpected notice chat error: %v", err)
	}
	if len(noticeOut) != 1 {
		t.Fatalf("expected 1 sender notice chat frame, got %d", len(noticeOut))
	}
	selfNotice, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, noticeOut[0]))
	if err != nil {
		t.Fatalf("decode sender notice chat: %v", err)
	}
	if selfNotice.Type != chatproto.ChatTypeNotice || selfNotice.VID != 0 || selfNotice.Message != "mensaje notice" {
		t.Fatalf("unexpected sender notice chat: %+v", selfNotice)
	}

	peerNotice := flushServerFrames(t, flowOne)
	if len(peerNotice) != 1 {
		t.Fatalf("expected 1 queued notice chat frame, got %d", len(peerNotice))
	}
	peerDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerNotice[0]))
	if err != nil {
		t.Fatalf("decode peer notice chat: %v", err)
	}
	if peerDelivery.Type != chatproto.ChatTypeNotice || peerDelivery.VID != 0 || peerDelivery.Message != "mensaje notice" {
		t.Fatalf("unexpected peer notice chat delivery: %+v", peerDelivery)
	}
}

func TestNewGameSessionFactoryReturnsWhisperNotExistForUnknownTarget(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	whisperOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{Target: "MissingPeer", Message: "hola privado"})))
	if err != nil {
		t.Fatalf("unexpected whisper error: %v", err)
	}
	if len(whisperOut) != 1 {
		t.Fatalf("expected 1 sender whisper error frame, got %d", len(whisperOut))
	}
	errorPacket, err := chatproto.DecodeServerWhisper(decodeSingleFrame(t, whisperOut[0]))
	if err != nil {
		t.Fatalf("decode not-exist whisper: %v", err)
	}
	if errorPacket.Type != chatproto.WhisperTypeNotExist || errorPacket.FromName != "MissingPeer" || errorPacket.Message != "" {
		t.Fatalf("unexpected not-exist whisper packet: %+v", errorPacket)
	}
}

func enterGameWithLoginTicket(t *testing.T, factory service.SessionFactory, login string, loginKey uint32) (service.SessionFlow, [][]byte) {
	t.Helper()

	flow := factory()
	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{}))); err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: login, LoginKey: loginKey})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	enterOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	return flow, enterOut
}

func flushServerFrames(t *testing.T, flow service.SessionFlow) [][]byte {
	t.Helper()
	source, ok := flow.(service.ServerFrameSource)
	if !ok {
		t.Fatal("session flow does not implement service.ServerFrameSource")
	}
	out, err := source.FlushServerFrames()
	if err != nil {
		t.Fatalf("flush server frames: %v", err)
	}
	return out
}

func closeSessionFlow(t *testing.T, flow service.SessionFlow) {
	t.Helper()
	closer, ok := flow.(io.Closer)
	if !ok {
		t.Fatal("session flow does not implement io.Closer")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close session flow: %v", err)
	}
}

func TestQueuedSessionFlowCloseDelegatesToInnerCloser(t *testing.T) {
	inner := &stubClosableSessionFlow{}
	closed := false
	flow := newQueuedSessionFlow(inner, newPendingServerFrames(), func() { closed = true })
	if err := flow.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !closed {
		t.Fatal("expected onClose hook to run")
	}
	if inner.closeCalls != 1 {
		t.Fatalf("expected inner closer to be called once, got %d", inner.closeCalls)
	}
}

func TestQueuedSessionFlowCloseReturnsTheSameInnerErrorOnRepeatedCalls(t *testing.T) {
	inner := &stubClosableSessionFlow{err: io.EOF}
	flow := newQueuedSessionFlow(inner, newPendingServerFrames(), nil)
	if err := flow.Close(); err != io.EOF {
		t.Fatalf("expected first close error %v, got %v", io.EOF, err)
	}
	if err := flow.Close(); err != io.EOF {
		t.Fatalf("expected repeated close error %v, got %v", io.EOF, err)
	}
	if inner.closeCalls != 1 {
		t.Fatalf("expected inner closer to be called once, got %d", inner.closeCalls)
	}
}

func issuePeerTicket(t *testing.T, store loginticket.Store, login string, loginKey uint32, character loginticket.Character) {
	t.Helper()
	if err := store.Issue(loginticket.Ticket{Login: login, LoginKey: loginKey, Empire: character.Empire, Characters: []loginticket.Character{character}}); err != nil {
		t.Fatalf("issue peer ticket: %v", err)
	}
}

func peerVisibilityCharacter(name string, id uint32, vid uint32, x int32, y int32, race uint16, mainPart uint16, hairPart uint16) loginticket.Character {
	character := stubCharacters()[0]
	character.ID = id
	character.VID = vid
	character.Name = name
	character.Job = uint8(race)
	character.RaceNum = race
	character.Level = 20
	character.MainPart = mainPart
	character.HairPart = hairPart
	character.X = x
	character.Y = y
	character.Z = 0
	character.Empire = 2
	character.SkillGroup = 1
	character.GuildID = 0
	character.GuildName = ""
	character.Points[1] = 750
	return character
}

type stubClosableSessionFlow struct {
	closeCalls int
	err        error
}

func (f *stubClosableSessionFlow) Start() ([][]byte, error) {
	return nil, nil
}

func (f *stubClosableSessionFlow) HandleClientFrame(frame.Frame) ([][]byte, error) {
	return nil, nil
}

func (f *stubClosableSessionFlow) Close() error {
	f.closeCalls++
	return f.err
}
