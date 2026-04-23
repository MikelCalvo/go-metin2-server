package minimal

import (
	"io"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	worldentry "github.com/MikelCalvo/go-metin2-server/internal/worldentry"
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

func TestNewGameSessionFactoryDoesNotBootstrapPeerVisibilityAcrossMaps(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerTwo.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for first player, got %d", len(firstEnter))
	}

	_, secondEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(secondEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for second player on another map, got %d", len(secondEnter))
	}

	peerEntry := flushServerFrames(t, flowOne)
	if len(peerEntry) != 0 {
		t.Fatalf("expected no queued peer-entry frames across maps, got %d", len(peerEntry))
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

func TestNewGameSessionFactoryDoesNotQueuePeerMoveAcrossMaps(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerTwo.MapIndex = 42
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

	peerMove := flushServerFrames(t, flowOne)
	if len(peerMove) != 0 {
		t.Fatalf("expected no queued peer move frames across maps, got %d", len(peerMove))
	}
}

func TestNewGameSessionFactoryDoesNotQueuePeerSyncPositionAcrossMaps(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerTwo.MapIndex = 42
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

	peerSync := flushServerFrames(t, flowOne)
	if len(peerSync) != 0 {
		t.Fatalf("expected no queued peer sync_position frames across maps, got %d", len(peerSync))
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

func TestNewGameSessionFactoryAppliesExactPositionTransferTriggerOnMove(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)

	moveOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 8 {
		t.Fatalf("expected 8 self transfer-rebootstrap frames, got %d", len(moveOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode self transfer add: %v", err)
	}
	if selfAdd.VID != peerTwo.VID || selfAdd.X != 1700 || selfAdd.Y != 2800 {
		t.Fatalf("unexpected self transfer add: %+v", selfAdd)
	}
	selfInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode self transfer additional info: %v", err)
	}
	if selfInfo.VID != peerTwo.VID || selfInfo.Name != peerTwo.Name {
		t.Fatalf("unexpected self transfer additional info: %+v", selfInfo)
	}
	selfUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, moveOut[2]))
	if err != nil {
		t.Fatalf("decode self transfer update: %v", err)
	}
	if selfUpdate.VID != peerTwo.VID {
		t.Fatalf("unexpected self transfer update: %+v", selfUpdate)
	}
	selfPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, moveOut[3]))
	if err != nil {
		t.Fatalf("decode self transfer point change: %v", err)
	}
	if selfPointChange.VID != peerTwo.VID {
		t.Fatalf("unexpected self transfer point change: %+v", selfPointChange)
	}
	removedPeer, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, moveOut[4]))
	if err != nil {
		t.Fatalf("decode mover delete notice: %v", err)
	}
	if removedPeer.VID != peerOne.VID {
		t.Fatalf("expected mover delete notice for old-map peer %#08x, got %#08x", peerOne.VID, removedPeer.VID)
	}
	addedPeer, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moveOut[5]))
	if err != nil {
		t.Fatalf("decode mover peer add: %v", err)
	}
	if addedPeer.VID != peerThree.VID || addedPeer.X != peerThree.X || addedPeer.Y != peerThree.Y {
		t.Fatalf("unexpected mover peer add: %+v", addedPeer)
	}

	moverFrames := flushServerFrames(t, flowTwo)
	if len(moverFrames) != 0 {
		t.Fatalf("expected no queued mover frames after immediate transfer rebootstrap, got %d", len(moverFrames))
	}

	oldMapFrames := flushServerFrames(t, flowOne)
	if len(oldMapFrames) != 1 {
		t.Fatalf("expected 1 old-map frame after triggered transfer, got %d", len(oldMapFrames))
	}
	removedMover, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, oldMapFrames[0]))
	if err != nil {
		t.Fatalf("decode old-map delete notice: %v", err)
	}
	if removedMover.VID != peerTwo.VID {
		t.Fatalf("expected old-map delete notice for moved peer %#08x, got %#08x", peerTwo.VID, removedMover.VID)
	}

	newMapFrames := flushServerFrames(t, flowThree)
	if len(newMapFrames) != 3 {
		t.Fatalf("expected 3 destination-map frames after triggered transfer, got %d", len(newMapFrames))
	}
	newPeerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, newMapFrames[0]))
	if err != nil {
		t.Fatalf("decode destination-map peer add: %v", err)
	}
	if newPeerAdd.VID != peerTwo.VID || newPeerAdd.X != 1700 || newPeerAdd.Y != 2800 {
		t.Fatalf("unexpected destination-map peer add: %+v", newPeerAdd)
	}

	if snapshots := runtime.ConnectedCharacters(); len(snapshots) != 3 || snapshots[2].Name != "PeerTwo" || snapshots[2].MapIndex != 42 || snapshots[2].X != 1700 || snapshots[2].Y != 2800 {
		t.Fatalf("expected connected character snapshot to reflect triggered transfer, got %+v", snapshots)
	}
}

func TestNewGameSessionFactoryAppliesExactPositionTransferTriggerOnSyncPosition(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)

	syncOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{Elements: []movep.SyncPositionElement{{VID: peerTwo.VID, X: 1500, Y: 2600}}})))
	if err != nil {
		t.Fatalf("unexpected sync_position error: %v", err)
	}
	if len(syncOut) != 8 {
		t.Fatalf("expected 8 self transfer-rebootstrap frames from sync_position trigger, got %d", len(syncOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, syncOut[0]))
	if err != nil {
		t.Fatalf("decode self transfer add from sync_position: %v", err)
	}
	if selfAdd.VID != peerTwo.VID || selfAdd.X != 1700 || selfAdd.Y != 2800 {
		t.Fatalf("unexpected self transfer add from sync_position: %+v", selfAdd)
	}
	selfPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, syncOut[3]))
	if err != nil {
		t.Fatalf("decode self transfer point change from sync_position: %v", err)
	}
	if selfPointChange.VID != peerTwo.VID {
		t.Fatalf("unexpected self transfer point change from sync_position: %+v", selfPointChange)
	}

	if queued := flushServerFrames(t, flowOne); len(queued) != 1 {
		t.Fatalf("expected 1 old-map frame after triggered sync_position transfer, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowTwo); len(queued) != 0 {
		t.Fatalf("expected no queued mover frames after sync_position transfer rebootstrap, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowThree); len(queued) != 3 {
		t.Fatalf("expected 3 destination-map frames after triggered sync_position transfer, got %d", len(queued))
	}
	if snapshots := runtime.ConnectedCharacters(); len(snapshots) != 3 || snapshots[2].Name != "PeerTwo" || snapshots[2].MapIndex != 42 || snapshots[2].X != 1700 || snapshots[2].Y != 2800 {
		t.Fatalf("expected connected character snapshot to reflect triggered sync_position transfer, got %+v", snapshots)
	}
}

func TestNewGameSessionFactoryDoesNotMutateWorldWhenTransferTriggerSaveFails(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)
	accounts := newPreloadedFailingAccountStore(
		accountstore.Account{Login: "peer-one", Empire: peerOne.Empire, Characters: []loginticket.Character{peerOne}},
		accountstore.Account{Login: "peer-two", Empire: peerTwo.Empire, Characters: []loginticket.Character{peerTwo}},
		accountstore.Account{Login: "peer-three", Empire: peerThree.Empire, Characters: []loginticket.Character{peerThree}},
	)

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)
	beforeCharacters := runtime.ConnectedCharacters()

	moveOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected no self frames on failed trigger transfer, got %d", len(moveOut))
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no old-map frames on failed triggered transfer, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowTwo); len(queued) != 0 {
		t.Fatalf("expected no mover frames on failed triggered transfer, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowThree); len(queued) != 0 {
		t.Fatalf("expected no destination-map frames on failed triggered transfer, got %d", len(queued))
	}
	if afterCharacters := runtime.ConnectedCharacters(); !reflect.DeepEqual(afterCharacters, beforeCharacters) {
		t.Fatalf("expected connected characters to remain unchanged on failed triggered transfer, before=%+v after=%+v", beforeCharacters, afterCharacters)
	}
}

func TestNewGameSessionFactoryRoutesPostTransferChatAndMoveToDestinationMapPeers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)

	transferOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(transferOut) != 8 {
		t.Fatalf("expected 8 self transfer-rebootstrap frames before post-transfer gameplay, got %d", len(transferOut))
	}
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)

	chatOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "hola despues del transfer"})))
	if err != nil {
		t.Fatalf("unexpected local chat error: %v", err)
	}
	if len(chatOut) != 1 {
		t.Fatalf("expected 1 sender local chat frame after transfer, got %d", len(chatOut))
	}
	selfChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, chatOut[0]))
	if err != nil {
		t.Fatalf("decode sender local chat after transfer: %v", err)
	}
	if selfChat.Type != chatproto.ChatTypeTalking || selfChat.VID != peerTwo.VID || selfChat.Message != "PeerTwo : hola despues del transfer" {
		t.Fatalf("unexpected sender local chat after transfer: %+v", selfChat)
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no old-map local chat frames after transfer, got %d", len(queued))
	}
	peerChat := flushServerFrames(t, flowThree)
	if len(peerChat) != 1 {
		t.Fatalf("expected 1 destination-map local chat frame after transfer, got %d", len(peerChat))
	}
	peerDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerChat[0]))
	if err != nil {
		t.Fatalf("decode destination-map local chat after transfer: %v", err)
	}
	if peerDelivery.Type != chatproto.ChatTypeTalking || peerDelivery.VID != peerTwo.VID || peerDelivery.Message != "PeerTwo : hola despues del transfer" {
		t.Fatalf("unexpected destination-map local chat after transfer: %+v", peerDelivery)
	}

	moveOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 20, X: 1750, Y: 2850, Time: 0x31323334})))
	if err != nil {
		t.Fatalf("unexpected post-transfer move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack after transfer, got %d", len(moveOut))
	}
	selfMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode self move ack after transfer: %v", err)
	}
	if selfMove.VID != peerTwo.VID || selfMove.X != 1750 || selfMove.Y != 2850 || selfMove.Time != 0x31323334 {
		t.Fatalf("unexpected self move ack after transfer: %+v", selfMove)
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no old-map move replication after transfer, got %d", len(queued))
	}
	peerMove := flushServerFrames(t, flowThree)
	if len(peerMove) != 1 {
		t.Fatalf("expected 1 destination-map move replication after transfer, got %d", len(peerMove))
	}
	peerAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, peerMove[0]))
	if err != nil {
		t.Fatalf("decode destination-map move replication after transfer: %v", err)
	}
	if peerAck.VID != peerTwo.VID || peerAck.X != 1750 || peerAck.Y != 2850 || peerAck.Time != 0x31323334 {
		t.Fatalf("unexpected destination-map move replication after transfer: %+v", peerAck)
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

func TestNewGameSessionFactoryDoesNotQueueLocalChatAcrossEmpires(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerTwo.Empire = 3
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

	peerChat := flushServerFrames(t, flowOne)
	if len(peerChat) != 0 {
		t.Fatalf("expected no queued peer chat frames across empires, got %d", len(peerChat))
	}
}

func TestNewGameSessionFactoryDoesNotQueueLocalChatAcrossMaps(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerTwo.MapIndex = 42
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

	peerChat := flushServerFrames(t, flowOne)
	if len(peerChat) != 0 {
		t.Fatalf("expected no queued peer chat frames across maps, got %d", len(peerChat))
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

func TestNewGameSessionFactoryRoutesWhisperToRelocatedNamedPeerAndKeepsConnectedSnapshotsUpdated(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if !runtime.RelocateCharacter("PeerOne", 42, 1700, 2800) {
		t.Fatal("expected relocate to succeed")
	}
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)

	whisperOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{Target: "PeerOne", Message: "hola movido"})))
	if err != nil {
		t.Fatalf("unexpected whisper error: %v", err)
	}
	if len(whisperOut) != 0 {
		t.Fatalf("expected no direct sender whisper frames on success, got %d", len(whisperOut))
	}

	recipientWhisper := flushServerFrames(t, flowOne)
	if len(recipientWhisper) != 1 {
		t.Fatalf("expected 1 queued whisper frame for relocated target, got %d", len(recipientWhisper))
	}
	delivery, err := chatproto.DecodeServerWhisper(decodeSingleFrame(t, recipientWhisper[0]))
	if err != nil {
		t.Fatalf("decode recipient whisper: %v", err)
	}
	if delivery.Type != chatproto.WhisperTypeChat || delivery.FromName != "PeerTwo" || delivery.Message != "hola movido" {
		t.Fatalf("unexpected recipient whisper: %+v", delivery)
	}

	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 connected character snapshots, got %d", len(snapshots))
	}
	var relocated *ConnectedCharacterSnapshot
	for i := range snapshots {
		if snapshots[i].Name == "PeerOne" {
			relocated = &snapshots[i]
			break
		}
	}
	if relocated == nil {
		t.Fatalf("expected connected character snapshots to include PeerOne, got %+v", snapshots)
	}
	if relocated.MapIndex != 42 || relocated.X != 1700 || relocated.Y != 2800 {
		t.Fatalf("expected relocated connected character snapshot for PeerOne, got %+v", *relocated)
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

func TestNewGameSessionFactoryQueuesPartyChatAcrossMapsForConnectedPeers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerTwo.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)

	partyOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeParty, Message: "hola intermap"})))
	if err != nil {
		t.Fatalf("unexpected cross-map party chat error: %v", err)
	}
	if len(partyOut) != 1 {
		t.Fatalf("expected 1 sender party chat frame across maps, got %d", len(partyOut))
	}

	peerParty := flushServerFrames(t, flowOne)
	if len(peerParty) != 1 {
		t.Fatalf("expected 1 queued cross-map party chat frame, got %d", len(peerParty))
	}
	peerDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerParty[0]))
	if err != nil {
		t.Fatalf("decode cross-map peer party chat: %v", err)
	}
	if peerDelivery.Type != chatproto.ChatTypeParty || peerDelivery.VID != peerTwo.VID || peerDelivery.Message != "PeerTwo : hola intermap" {
		t.Fatalf("unexpected cross-map peer party chat delivery: %+v", peerDelivery)
	}
}

func TestNewGameSessionFactoryQueuesGuildChatForConnectedPeers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerOne.GuildID = 10
	peerOne.GuildName = "Alpha"
	peerTwo.GuildID = 10
	peerTwo.GuildName = "Alpha"
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

func TestNewGameSessionFactoryDoesNotQueueGuildChatAcrossDifferentGuilds(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerOne.GuildID = 10
	peerOne.GuildName = "Alpha"
	peerTwo.GuildID = 20
	peerTwo.GuildName = "Beta"
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

	peerGuild := flushServerFrames(t, flowOne)
	if len(peerGuild) != 0 {
		t.Fatalf("expected no queued guild chat frames across different guilds, got %d", len(peerGuild))
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

func TestNewGameSessionFactoryDoesNotQueueShoutAcrossEmpires(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerTwo.Empire = 3
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

	peerShout := flushServerFrames(t, flowOne)
	if len(peerShout) != 0 {
		t.Fatalf("expected no queued shout chat frames across empires, got %d", len(peerShout))
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

func TestNewGameSessionFactoryRejectsClientOriginatedNoticeChat(t *testing.T) {
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
	if len(noticeOut) != 0 {
		t.Fatalf("expected no sender notice chat frames, got %d", len(noticeOut))
	}

	peerNotice := flushServerFrames(t, flowOne)
	if len(peerNotice) != 0 {
		t.Fatalf("expected no queued notice chat frames, got %d", len(peerNotice))
	}
}

func TestGameRuntimeBroadcastNoticeQueuesSystemMessageToConnectedSessions(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)

	delivered := runtime.BroadcastNotice("server maintenance")
	if delivered != 2 {
		t.Fatalf("expected notice to be queued for 2 connected sessions, got %d", delivered)
	}

	noticeOne := flushServerFrames(t, flowOne)
	if len(noticeOne) != 1 {
		t.Fatalf("expected 1 queued server notice for first player, got %d", len(noticeOne))
	}
	decodedOne, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, noticeOne[0]))
	if err != nil {
		t.Fatalf("decode first server notice: %v", err)
	}
	if decodedOne.Type != chatproto.ChatTypeNotice || decodedOne.VID != 0 || decodedOne.Message != "server maintenance" {
		t.Fatalf("unexpected first server notice: %+v", decodedOne)
	}

	noticeTwo := flushServerFrames(t, flowTwo)
	if len(noticeTwo) != 1 {
		t.Fatalf("expected 1 queued server notice for second player, got %d", len(noticeTwo))
	}
	decodedTwo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, noticeTwo[0]))
	if err != nil {
		t.Fatalf("decode second server notice: %v", err)
	}
	if decodedTwo.Type != chatproto.ChatTypeNotice || decodedTwo.VID != 0 || decodedTwo.Message != "server maintenance" {
		t.Fatalf("unexpected second server notice: %+v", decodedTwo)
	}
}

func TestGameRuntimeBroadcastNoticeQueuesSystemMessageAcrossMaps(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerTwo.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)

	delivered := runtime.BroadcastNotice("cross-map notice")
	if delivered != 2 {
		t.Fatalf("expected cross-map notice to be queued for 2 connected sessions, got %d", delivered)
	}

	noticeOne := flushServerFrames(t, flowOne)
	noticeTwo := flushServerFrames(t, flowTwo)
	if len(noticeOne) != 1 || len(noticeTwo) != 1 {
		t.Fatalf("expected one queued notice per session across maps, got first=%d second=%d", len(noticeOne), len(noticeTwo))
	}
}

func TestGameRuntimeBroadcastNoticeRejectsEmptyMessage(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if delivered := runtime.BroadcastNotice(""); delivered != 0 {
		t.Fatalf("expected empty notice to queue for 0 sessions, got %d", delivered)
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

func TestSharedWorldRegistryRelocateRebuildsVisibilityAcrossMaps(t *testing.T) {
	registry := newSharedWorldRegistry()
	pendingOne := newPendingServerFrames()
	pendingTwo := newPendingServerFrames()
	pendingThree := newPendingServerFrames()

	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42

	_, _ = registry.Join(peerOne, pendingOne, nil)
	idTwo, _ := registry.Join(peerTwo, pendingTwo, nil)
	_, _ = registry.Join(peerThree, pendingThree, nil)
	_ = pendingOne.flush()
	_ = pendingTwo.flush()
	_ = pendingThree.flush()

	peerTwo.MapIndex = 42
	peerTwo.X = 1700
	peerTwo.Y = 2800
	if !registry.Relocate(idTwo, peerTwo) {
		t.Fatal("expected relocate to succeed")
	}

	oldPeerFrames := pendingOne.flush()
	if len(oldPeerFrames) != 1 {
		t.Fatalf("expected 1 queued old-peer visibility frame, got %d", len(oldPeerFrames))
	}
	oldPeerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, oldPeerFrames[0]))
	if err != nil {
		t.Fatalf("decode old-peer delete: %v", err)
	}
	if oldPeerDelete.VID != peerTwo.VID {
		t.Fatalf("expected old peer delete for VID %#08x, got %#08x", peerTwo.VID, oldPeerDelete.VID)
	}

	moverFrames := pendingTwo.flush()
	if len(moverFrames) != 4 {
		t.Fatalf("expected 4 queued mover visibility-rebuild frames, got %d", len(moverFrames))
	}
	moverDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, moverFrames[0]))
	if err != nil {
		t.Fatalf("decode mover delete: %v", err)
	}
	if moverDelete.VID != peerOne.VID {
		t.Fatalf("expected mover delete for old peer VID %#08x, got %#08x", peerOne.VID, moverDelete.VID)
	}
	moverAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moverFrames[1]))
	if err != nil {
		t.Fatalf("decode mover peer add: %v", err)
	}
	if moverAdd.VID != peerThree.VID || moverAdd.X != peerThree.X || moverAdd.Y != peerThree.Y {
		t.Fatalf("unexpected mover peer add packet: %+v", moverAdd)
	}
	moverInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, moverFrames[2]))
	if err != nil {
		t.Fatalf("decode mover peer additional info: %v", err)
	}
	if moverInfo.VID != peerThree.VID || moverInfo.Name != peerThree.Name {
		t.Fatalf("unexpected mover peer additional info packet: %+v", moverInfo)
	}
	moverUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, moverFrames[3]))
	if err != nil {
		t.Fatalf("decode mover peer update: %v", err)
	}
	if moverUpdate.VID != peerThree.VID {
		t.Fatalf("expected mover peer update for VID %#08x, got %#08x", peerThree.VID, moverUpdate.VID)
	}

	newPeerFrames := pendingThree.flush()
	if len(newPeerFrames) != 3 {
		t.Fatalf("expected 3 queued new-peer visibility frames, got %d", len(newPeerFrames))
	}
	newPeerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, newPeerFrames[0]))
	if err != nil {
		t.Fatalf("decode new-peer add: %v", err)
	}
	if newPeerAdd.VID != peerTwo.VID || newPeerAdd.X != peerTwo.X || newPeerAdd.Y != peerTwo.Y {
		t.Fatalf("unexpected new-peer add packet: %+v", newPeerAdd)
	}
	newPeerInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, newPeerFrames[1]))
	if err != nil {
		t.Fatalf("decode new-peer additional info: %v", err)
	}
	if newPeerInfo.VID != peerTwo.VID || newPeerInfo.Name != peerTwo.Name {
		t.Fatalf("unexpected new-peer additional info packet: %+v", newPeerInfo)
	}
	newPeerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, newPeerFrames[2]))
	if err != nil {
		t.Fatalf("decode new-peer update: %v", err)
	}
	if newPeerUpdate.VID != peerTwo.VID {
		t.Fatalf("expected new-peer update for VID %#08x, got %#08x", peerTwo.VID, newPeerUpdate.VID)
	}
}

func TestSharedWorldRegistryRelocateUsesPreviousVIDForOldPeerDelete(t *testing.T) {
	registry := newSharedWorldRegistry()
	pendingOne := newPendingServerFrames()
	pendingTwo := newPendingServerFrames()
	pendingThree := newPendingServerFrames()

	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42

	_, _ = registry.Join(peerOne, pendingOne, nil)
	idTwo, _ := registry.Join(peerTwo, pendingTwo, nil)
	_, _ = registry.Join(peerThree, pendingThree, nil)
	_ = pendingOne.flush()
	_ = pendingTwo.flush()
	_ = pendingThree.flush()

	relocated := peerTwo
	relocated.VID = 0x0badf00d
	relocated.MapIndex = 42
	if !registry.Relocate(idTwo, relocated) {
		t.Fatal("expected relocate to succeed")
	}

	oldPeerFrames := pendingOne.flush()
	if len(oldPeerFrames) != 1 {
		t.Fatalf("expected 1 queued old-peer delete frame, got %d", len(oldPeerFrames))
	}
	oldPeerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, oldPeerFrames[0]))
	if err != nil {
		t.Fatalf("decode old-peer delete: %v", err)
	}
	if oldPeerDelete.VID != peerTwo.VID {
		t.Fatalf("expected old peer delete for previous VID %#08x, got %#08x", peerTwo.VID, oldPeerDelete.VID)
	}
}

func TestSharedWorldRegistryRelocateUpdatesSnapshotWithoutVisibilityRebuildOnSameMap(t *testing.T) {
	registry := newSharedWorldRegistry()
	pendingOne := newPendingServerFrames()
	pendingTwo := newPendingServerFrames()

	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)

	_, _ = registry.Join(peerOne, pendingOne, nil)
	idTwo, _ := registry.Join(peerTwo, pendingTwo, nil)
	_ = pendingOne.flush()
	_ = pendingTwo.flush()

	peerTwo.X = 1700
	peerTwo.Y = 2800
	if !registry.Relocate(idTwo, peerTwo) {
		t.Fatal("expected relocate to succeed")
	}
	if queued := pendingOne.flush(); len(queued) != 0 {
		t.Fatalf("expected no old-peer rebuild frames on same-map relocate, got %d", len(queued))
	}
	if queued := pendingTwo.flush(); len(queued) != 0 {
		t.Fatalf("expected no mover rebuild frames on same-map relocate, got %d", len(queued))
	}

	pendingThree := newPendingServerFrames()
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	_, existing := registry.Join(peerThree, pendingThree, nil)
	if len(existing) != 2 {
		t.Fatalf("expected 2 existing peers for later join, got %d", len(existing))
	}

	foundRelocatedPeer := false
	for _, existingCharacter := range existing {
		if existingCharacter.VID != peerTwo.VID {
			continue
		}
		foundRelocatedPeer = true
		if existingCharacter.X != peerTwo.X || existingCharacter.Y != peerTwo.Y {
			t.Fatalf("expected relocated peer snapshot at (%d, %d), got (%d, %d)", peerTwo.X, peerTwo.Y, existingCharacter.X, existingCharacter.Y)
		}
	}
	if !foundRelocatedPeer {
		t.Fatal("expected later join to see relocated peer snapshot")
	}
}

func TestGameRuntimeRelocateCharacterMovesConnectedSessionAcrossMaps(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)

	if !runtime.RelocateCharacter("PeerTwo", 42, 1700, 2800) {
		t.Fatal("expected relocate to succeed")
	}

	oldPeerFrames := flushServerFrames(t, flowOne)
	if len(oldPeerFrames) != 1 {
		t.Fatalf("expected 1 queued old-peer delete frame, got %d", len(oldPeerFrames))
	}
	oldPeerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, oldPeerFrames[0]))
	if err != nil {
		t.Fatalf("decode old-peer delete: %v", err)
	}
	if oldPeerDelete.VID != peerTwo.VID {
		t.Fatalf("expected old peer delete for VID %#08x, got %#08x", peerTwo.VID, oldPeerDelete.VID)
	}

	moverFrames := flushServerFrames(t, flowTwo)
	if len(moverFrames) != 4 {
		t.Fatalf("expected 4 queued mover rebuild frames, got %d", len(moverFrames))
	}
	moverDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, moverFrames[0]))
	if err != nil {
		t.Fatalf("decode mover delete: %v", err)
	}
	if moverDelete.VID != peerOne.VID {
		t.Fatalf("expected mover delete for old peer VID %#08x, got %#08x", peerOne.VID, moverDelete.VID)
	}
	moverAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moverFrames[1]))
	if err != nil {
		t.Fatalf("decode mover add: %v", err)
	}
	if moverAdd.VID != peerThree.VID || moverAdd.X != peerThree.X || moverAdd.Y != peerThree.Y {
		t.Fatalf("unexpected mover add packet: %+v", moverAdd)
	}

	newPeerFrames := flushServerFrames(t, flowThree)
	if len(newPeerFrames) != 3 {
		t.Fatalf("expected 3 queued new-peer visibility frames, got %d", len(newPeerFrames))
	}
	newPeerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, newPeerFrames[0]))
	if err != nil {
		t.Fatalf("decode new-peer add: %v", err)
	}
	if newPeerAdd.VID != peerTwo.VID || newPeerAdd.X != 1700 || newPeerAdd.Y != 2800 {
		t.Fatalf("unexpected new-peer add packet: %+v", newPeerAdd)
	}

	moveOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1750, Y: 2850, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected move after relocate error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame after relocate, got %d", len(moveOut))
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no old-map queued move frames after relocate, got %d", len(queued))
	}
	peerMove := flushServerFrames(t, flowThree)
	if len(peerMove) != 1 {
		t.Fatalf("expected 1 destination-map queued move frame after relocate, got %d", len(peerMove))
	}
	peerAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, peerMove[0]))
	if err != nil {
		t.Fatalf("decode destination-map peer move: %v", err)
	}
	if peerAck.VID != peerTwo.VID || peerAck.X != 1750 || peerAck.Y != 2850 || peerAck.Time != 0x21222324 {
		t.Fatalf("unexpected destination-map peer move ack: %+v", peerAck)
	}
}

func TestGameRuntimeTransferCharacterReturnsStructuredResult(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	blacksmith, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1050, 2050, 20301)
	if !ok {
		t.Fatal("expected bootstrap static actor registration to succeed")
	}
	guard, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected destination static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)

	result, ok := runtime.TransferCharacter("PeerTwo", 42, 1700, 2800)
	if !ok {
		t.Fatal("expected transfer to succeed")
	}
	if !result.Applied {
		t.Fatal("expected transfer result to be marked applied")
	}
	if result.Character.Name != "PeerTwo" || result.Character.MapIndex != bootstrapMapIndex || result.Character.X != 1300 || result.Character.Y != 2300 {
		t.Fatalf("unexpected source transfer snapshot: %+v", result.Character)
	}
	if result.Target.Name != "PeerTwo" || result.Target.MapIndex != 42 || result.Target.X != 1700 || result.Target.Y != 2800 {
		t.Fatalf("unexpected target transfer snapshot: %+v", result.Target)
	}
	if len(result.CurrentVisiblePeers) != 1 || result.CurrentVisiblePeers[0].Name != "PeerOne" {
		t.Fatalf("unexpected current visible peers: %+v", result.CurrentVisiblePeers)
	}
	if len(result.TargetVisiblePeers) != 1 || result.TargetVisiblePeers[0].Name != "PeerThree" {
		t.Fatalf("unexpected target visible peers: %+v", result.TargetVisiblePeers)
	}
	if len(result.RemovedVisiblePeers) != 1 || result.RemovedVisiblePeers[0].Name != "PeerOne" {
		t.Fatalf("unexpected removed visible peers: %+v", result.RemovedVisiblePeers)
	}
	if len(result.AddedVisiblePeers) != 1 || result.AddedVisiblePeers[0].Name != "PeerThree" {
		t.Fatalf("unexpected added visible peers: %+v", result.AddedVisiblePeers)
	}
	if len(result.MapOccupancyChanges) != 2 || result.MapOccupancyChanges[0].MapIndex != bootstrapMapIndex || result.MapOccupancyChanges[0].BeforeCount != 2 || result.MapOccupancyChanges[0].AfterCount != 1 || result.MapOccupancyChanges[1].MapIndex != 42 || result.MapOccupancyChanges[1].BeforeCount != 1 || result.MapOccupancyChanges[1].AfterCount != 2 {
		t.Fatalf("unexpected map occupancy changes: %+v", result.MapOccupancyChanges)
	}
	if len(result.BeforeMapOccupancy) != 2 {
		t.Fatalf("expected 2 before-map occupancy snapshots, got %d", len(result.BeforeMapOccupancy))
	}
	if result.BeforeMapOccupancy[0].MapIndex != bootstrapMapIndex || result.BeforeMapOccupancy[0].CharacterCount != 2 || len(result.BeforeMapOccupancy[0].Characters) != 2 || result.BeforeMapOccupancy[0].StaticActorCount != 1 || len(result.BeforeMapOccupancy[0].StaticActors) != 1 || result.BeforeMapOccupancy[0].StaticActors[0].EntityID != blacksmith.EntityID || result.BeforeMapOccupancy[0].StaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected source before-map occupancy snapshot: %+v", result.BeforeMapOccupancy[0])
	}
	if result.BeforeMapOccupancy[1].MapIndex != 42 || result.BeforeMapOccupancy[1].CharacterCount != 1 || len(result.BeforeMapOccupancy[1].Characters) != 1 || result.BeforeMapOccupancy[1].StaticActorCount != 1 || len(result.BeforeMapOccupancy[1].StaticActors) != 1 || result.BeforeMapOccupancy[1].StaticActors[0].EntityID != guard.EntityID || result.BeforeMapOccupancy[1].StaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected destination before-map occupancy snapshot: %+v", result.BeforeMapOccupancy[1])
	}
	if len(result.AfterMapOccupancy) != 2 {
		t.Fatalf("expected 2 after-map occupancy snapshots, got %d", len(result.AfterMapOccupancy))
	}
	if result.AfterMapOccupancy[0].MapIndex != bootstrapMapIndex || result.AfterMapOccupancy[0].CharacterCount != 1 || len(result.AfterMapOccupancy[0].Characters) != 1 || result.AfterMapOccupancy[0].StaticActorCount != 1 || len(result.AfterMapOccupancy[0].StaticActors) != 1 || result.AfterMapOccupancy[0].StaticActors[0].EntityID != blacksmith.EntityID || result.AfterMapOccupancy[0].StaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected source after-map occupancy snapshot: %+v", result.AfterMapOccupancy[0])
	}
	if result.AfterMapOccupancy[1].MapIndex != 42 || result.AfterMapOccupancy[1].CharacterCount != 2 || len(result.AfterMapOccupancy[1].Characters) != 2 || result.AfterMapOccupancy[1].StaticActorCount != 1 || len(result.AfterMapOccupancy[1].StaticActors) != 1 || result.AfterMapOccupancy[1].StaticActors[0].EntityID != guard.EntityID || result.AfterMapOccupancy[1].StaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected destination after-map occupancy snapshot: %+v", result.AfterMapOccupancy[1])
	}

	if snapshots := runtime.ConnectedCharacters(); len(snapshots) != 3 || snapshots[2].Name != "PeerTwo" || snapshots[2].MapIndex != 42 || snapshots[2].X != 1700 || snapshots[2].Y != 2800 {
		t.Fatalf("expected connected character snapshot to reflect transfer, got %+v", snapshots)
	}
}

func TestGameRuntimeTransferCharacterRejectsUnknownTarget(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.TransferCharacter("MissingPeer", 42, 1700, 2800); ok {
		t.Fatal("expected transfer to reject unknown target")
	}
}

func TestGameRuntimeTransferCharacterDoesNotMutateWorldOnSaveFailure(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)
	accounts := newPreloadedFailingAccountStore(
		accountstore.Account{Login: "peer-one", Empire: peerOne.Empire, Characters: []loginticket.Character{peerOne}},
		accountstore.Account{Login: "peer-two", Empire: peerTwo.Empire, Characters: []loginticket.Character{peerTwo}},
		accountstore.Account{Login: "peer-three", Empire: peerThree.Empire, Characters: []loginticket.Character{peerThree}},
	)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)
	beforeCharacters := runtime.ConnectedCharacters()

	if _, ok := runtime.TransferCharacter("PeerTwo", 42, 1700, 2800); ok {
		t.Fatal("expected transfer to fail when account snapshot save fails")
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no old-map frames on failed transfer, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowTwo); len(queued) != 0 {
		t.Fatalf("expected no mover frames on failed transfer, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowThree); len(queued) != 0 {
		t.Fatalf("expected no destination-map frames on failed transfer, got %d", len(queued))
	}
	if afterCharacters := runtime.ConnectedCharacters(); !reflect.DeepEqual(afterCharacters, beforeCharacters) {
		t.Fatalf("expected connected characters to remain unchanged on failed transfer, before=%+v after=%+v", beforeCharacters, afterCharacters)
	}
}

func TestGameRuntimeRelocateCharacterRejectsUnknownTarget(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if runtime.RelocateCharacter("MissingPeer", 42, 1700, 2800) {
		t.Fatal("expected relocate to reject unknown target")
	}
}

func TestGameRuntimeRelocateCharacterDoesNotMutateWorldOnSaveFailure(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)
	accounts := newPreloadedFailingAccountStore(
		accountstore.Account{Login: "peer-one", Empire: peerOne.Empire, Characters: []loginticket.Character{peerOne}},
		accountstore.Account{Login: "peer-two", Empire: peerTwo.Empire, Characters: []loginticket.Character{peerTwo}},
		accountstore.Account{Login: "peer-three", Empire: peerThree.Empire, Characters: []loginticket.Character{peerThree}},
	)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)

	if runtime.RelocateCharacter("PeerTwo", 42, 1700, 2800) {
		t.Fatal("expected relocate to fail when account snapshot save fails")
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no old-map frames on failed relocate, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowTwo); len(queued) != 0 {
		t.Fatalf("expected no mover frames on failed relocate, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowThree); len(queued) != 0 {
		t.Fatalf("expected no destination-map frames on failed relocate, got %d", len(queued))
	}
}

func TestGameRuntimeConnectedCharactersReturnsSortedSnapshots(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerZulu := peerVisibilityCharacter("Zulu", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerAlpha := peerVisibilityCharacter("Alpha", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerAlpha.MapIndex = 42
	issuePeerTicket(t, store, "peer-zulu", 0x11111111, peerZulu)
	issuePeerTicket(t, store, "peer-alpha", 0x22222222, peerAlpha)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	_, _ = enterGameWithLoginTicket(t, factory, "peer-zulu", 0x11111111)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-alpha", 0x22222222)

	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 connected character snapshots, got %d", len(snapshots))
	}
	if snapshots[0].Name != "Alpha" || snapshots[0].MapIndex != 42 || snapshots[0].X != 1300 || snapshots[0].Y != 2300 || snapshots[0].Empire != 2 || snapshots[0].GuildID != 0 {
		t.Fatalf("unexpected first connected character snapshot: %+v", snapshots[0])
	}
	if snapshots[1].Name != "Zulu" || snapshots[1].MapIndex != bootstrapMapIndex || snapshots[1].X != 1100 || snapshots[1].Y != 2100 || snapshots[1].Empire != 2 || snapshots[1].GuildID != 0 {
		t.Fatalf("unexpected second connected character snapshot: %+v", snapshots[1])
	}
}

func TestGameRuntimeConnectedCharactersReflectsRelocatedLocation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()
	_, _ = enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)

	if !runtime.RelocateCharacter("PeerTwo", 42, 1700, 2800) {
		t.Fatal("expected relocate to succeed")
	}

	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 connected character snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Name != "PeerTwo" || snapshots[0].MapIndex != 42 || snapshots[0].X != 1700 || snapshots[0].Y != 2800 {
		t.Fatalf("unexpected relocated connected character snapshot: %+v", snapshots[0])
	}
}

func TestGameRuntimeCharacterVisibilityReturnsSortedVisiblePeers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerZulu := peerVisibilityCharacter("Zulu", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerAlpha := peerVisibilityCharacter("Alpha", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerAlpha.MapIndex = 42
	issuePeerTicket(t, store, "peer-zulu", 0x11111111, peerZulu)
	issuePeerTicket(t, store, "peer-alpha", 0x22222222, peerAlpha)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	_, _ = enterGameWithLoginTicket(t, factory, "peer-zulu", 0x11111111)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-alpha", 0x22222222)

	snapshots := runtime.CharacterVisibility()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 character visibility snapshots, got %d", len(snapshots))
	}
	if snapshots[0].Name != "Alpha" || len(snapshots[0].VisiblePeers) != 0 {
		t.Fatalf("unexpected first character visibility snapshot: %+v", snapshots[0])
	}
	if snapshots[1].Name != "Zulu" || len(snapshots[1].VisiblePeers) != 0 {
		t.Fatalf("unexpected second character visibility snapshot: %+v", snapshots[1])
	}
}

func TestGameRuntimeCharacterVisibilityReflectsRelocatedPeers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	_, _ = enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)

	if !runtime.RelocateCharacter("PeerTwo", 42, 1700, 2800) {
		t.Fatal("expected relocate to succeed")
	}

	snapshots := runtime.CharacterVisibility()
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 character visibility snapshots, got %d", len(snapshots))
	}
	if snapshots[0].Name != "PeerOne" || len(snapshots[0].VisiblePeers) != 0 {
		t.Fatalf("unexpected old-map character visibility snapshot: %+v", snapshots[0])
	}
	if snapshots[1].Name != "PeerThree" || len(snapshots[1].VisiblePeers) != 1 || snapshots[1].VisiblePeers[0].Name != "PeerTwo" || snapshots[1].VisiblePeers[0].MapIndex != 42 || snapshots[1].VisiblePeers[0].X != 1700 || snapshots[1].VisiblePeers[0].Y != 2800 {
		t.Fatalf("unexpected destination peer visibility snapshot: %+v", snapshots[1])
	}
	if snapshots[2].Name != "PeerTwo" || len(snapshots[2].VisiblePeers) != 1 || snapshots[2].VisiblePeers[0].Name != "PeerThree" || snapshots[2].VisiblePeers[0].MapIndex != 42 {
		t.Fatalf("unexpected relocated character visibility snapshot: %+v", snapshots[2])
	}
}

func TestGameRuntimeMapOccupancyGroupsCharactersByEffectiveMap(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerZulu := peerVisibilityCharacter("Zulu", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerBravo := peerVisibilityCharacter("Bravo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerAlpha := peerVisibilityCharacter("Alpha", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerBravo.MapIndex = 42
	peerAlpha.MapIndex = 42
	issuePeerTicket(t, store, "peer-zulu", 0x11111111, peerZulu)
	issuePeerTicket(t, store, "peer-bravo", 0x22222222, peerBravo)
	issuePeerTicket(t, store, "peer-alpha", 0x33333333, peerAlpha)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	_, _ = enterGameWithLoginTicket(t, factory, "peer-zulu", 0x11111111)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-bravo", 0x22222222)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-alpha", 0x33333333)

	snapshots := runtime.MapOccupancy()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 map occupancy snapshots, got %d", len(snapshots))
	}
	if snapshots[0].MapIndex != bootstrapMapIndex || snapshots[0].CharacterCount != 1 || len(snapshots[0].Characters) != 1 || snapshots[0].Characters[0].Name != "Zulu" {
		t.Fatalf("unexpected bootstrap map occupancy snapshot: %+v", snapshots[0])
	}
	if snapshots[1].MapIndex != 42 || snapshots[1].CharacterCount != 2 || len(snapshots[1].Characters) != 2 || snapshots[1].Characters[0].Name != "Alpha" || snapshots[1].Characters[1].Name != "Bravo" {
		t.Fatalf("unexpected destination map occupancy snapshot: %+v", snapshots[1])
	}
}

func TestGameRuntimeMapOccupancyReflectsRelocatedCharacter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	_, _ = enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)

	if !runtime.RelocateCharacter("PeerTwo", 42, 1700, 2800) {
		t.Fatal("expected relocate to succeed")
	}

	snapshots := runtime.MapOccupancy()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 map occupancy snapshots, got %d", len(snapshots))
	}
	if snapshots[0].MapIndex != bootstrapMapIndex || snapshots[0].CharacterCount != 1 || len(snapshots[0].Characters) != 1 || snapshots[0].Characters[0].Name != "PeerOne" {
		t.Fatalf("unexpected source map occupancy snapshot after relocate: %+v", snapshots[0])
	}
	if snapshots[1].MapIndex != 42 || snapshots[1].CharacterCount != 2 || len(snapshots[1].Characters) != 2 || snapshots[1].Characters[0].Name != "PeerThree" || snapshots[1].Characters[1].Name != "PeerTwo" {
		t.Fatalf("unexpected destination map occupancy snapshot after relocate: %+v", snapshots[1])
	}
}

func TestGameRuntimeMapOccupancyReflectsDisconnectedCharacter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	flowThree, _ := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)
	_ = flushServerFrames(t, flowThree)

	closeSessionFlow(t, flowTwo)

	snapshots := runtime.MapOccupancy()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 map occupancy snapshots after disconnect, got %d", len(snapshots))
	}
	if snapshots[0].MapIndex != bootstrapMapIndex || snapshots[0].CharacterCount != 1 || len(snapshots[0].Characters) != 1 || snapshots[0].Characters[0].Name != "PeerOne" {
		t.Fatalf("unexpected bootstrap map occupancy snapshot after disconnect: %+v", snapshots[0])
	}
	if snapshots[1].MapIndex != 42 || snapshots[1].CharacterCount != 1 || len(snapshots[1].Characters) != 1 || snapshots[1].Characters[0].Name != "PeerThree" {
		t.Fatalf("unexpected destination map occupancy snapshot after disconnect: %+v", snapshots[1])
	}
}

func TestGameRuntimeMapOccupancyIncludesStaticActorsOnStaticOnlyMaps(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	guard, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	blacksmith, ok := runtime.RegisterStaticActor("Blacksmith", 1, 1100, 2100, 20301)
	if !ok {
		t.Fatal("expected blacksmith registration to succeed")
	}

	snapshots := runtime.MapOccupancy()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 map occupancy snapshots for static-only maps, got %d", len(snapshots))
	}
	if snapshots[0].MapIndex != 1 || snapshots[0].CharacterCount != 0 || len(snapshots[0].Characters) != 0 {
		t.Fatalf("unexpected bootstrap static-only map snapshot: %+v", snapshots[0])
	}
	if snapshots[0].StaticActorCount != 1 || len(snapshots[0].StaticActors) != 1 || snapshots[0].StaticActors[0].EntityID != blacksmith.EntityID || snapshots[0].StaticActors[0].Name != "Blacksmith" {
		t.Fatalf("expected Blacksmith in bootstrap static actor snapshot, got %+v", snapshots[0])
	}
	if snapshots[1].MapIndex != 42 || snapshots[1].CharacterCount != 0 || len(snapshots[1].Characters) != 0 {
		t.Fatalf("unexpected destination static-only map snapshot: %+v", snapshots[1])
	}
	if snapshots[1].StaticActorCount != 1 || len(snapshots[1].StaticActors) != 1 || snapshots[1].StaticActors[0].EntityID != guard.EntityID || snapshots[1].StaticActors[0].Name != "VillageGuard" {
		t.Fatalf("expected VillageGuard in destination static actor snapshot, got %+v", snapshots[1])
	}
}

func TestGameRuntimePreviewRelocationReturnsVisibilityAndMapChanges(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	blacksmith, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1050, 2050, 20301)
	if !ok {
		t.Fatal("expected bootstrap static actor registration to succeed")
	}
	guard, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected destination static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	_, _ = enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)

	preview, ok := runtime.PreviewRelocation("PeerTwo", 42, 1700, 2800)
	if !ok {
		t.Fatal("expected relocate preview to succeed")
	}
	if preview.Character.Name != "PeerTwo" || preview.Character.MapIndex != bootstrapMapIndex || preview.Character.X != 1300 || preview.Character.Y != 2300 {
		t.Fatalf("unexpected current preview character snapshot: %+v", preview.Character)
	}
	if preview.Target.Name != "PeerTwo" || preview.Target.MapIndex != 42 || preview.Target.X != 1700 || preview.Target.Y != 2800 {
		t.Fatalf("unexpected target preview character snapshot: %+v", preview.Target)
	}
	if len(preview.CurrentVisiblePeers) != 1 || preview.CurrentVisiblePeers[0].Name != "PeerOne" {
		t.Fatalf("unexpected current visible peers: %+v", preview.CurrentVisiblePeers)
	}
	if len(preview.TargetVisiblePeers) != 1 || preview.TargetVisiblePeers[0].Name != "PeerThree" {
		t.Fatalf("unexpected target visible peers: %+v", preview.TargetVisiblePeers)
	}
	if len(preview.RemovedVisiblePeers) != 1 || preview.RemovedVisiblePeers[0].Name != "PeerOne" {
		t.Fatalf("unexpected removed visible peers: %+v", preview.RemovedVisiblePeers)
	}
	if len(preview.AddedVisiblePeers) != 1 || preview.AddedVisiblePeers[0].Name != "PeerThree" {
		t.Fatalf("unexpected added visible peers: %+v", preview.AddedVisiblePeers)
	}
	if len(preview.MapOccupancyChanges) != 2 {
		t.Fatalf("expected 2 map occupancy changes, got %d", len(preview.MapOccupancyChanges))
	}
	if preview.MapOccupancyChanges[0].MapIndex != bootstrapMapIndex || preview.MapOccupancyChanges[0].BeforeCount != 2 || preview.MapOccupancyChanges[0].AfterCount != 1 {
		t.Fatalf("unexpected source map occupancy change: %+v", preview.MapOccupancyChanges[0])
	}
	if preview.MapOccupancyChanges[1].MapIndex != 42 || preview.MapOccupancyChanges[1].BeforeCount != 1 || preview.MapOccupancyChanges[1].AfterCount != 2 {
		t.Fatalf("unexpected destination map occupancy change: %+v", preview.MapOccupancyChanges[1])
	}
	if len(preview.BeforeMapOccupancy) != 2 {
		t.Fatalf("expected 2 before-map occupancy snapshots, got %d", len(preview.BeforeMapOccupancy))
	}
	if preview.BeforeMapOccupancy[0].MapIndex != bootstrapMapIndex || preview.BeforeMapOccupancy[0].CharacterCount != 2 || len(preview.BeforeMapOccupancy[0].Characters) != 2 || preview.BeforeMapOccupancy[0].StaticActorCount != 1 || len(preview.BeforeMapOccupancy[0].StaticActors) != 1 || preview.BeforeMapOccupancy[0].StaticActors[0].EntityID != blacksmith.EntityID || preview.BeforeMapOccupancy[0].StaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected source before-map occupancy snapshot: %+v", preview.BeforeMapOccupancy[0])
	}
	if preview.BeforeMapOccupancy[1].MapIndex != 42 || preview.BeforeMapOccupancy[1].CharacterCount != 1 || len(preview.BeforeMapOccupancy[1].Characters) != 1 || preview.BeforeMapOccupancy[1].StaticActorCount != 1 || len(preview.BeforeMapOccupancy[1].StaticActors) != 1 || preview.BeforeMapOccupancy[1].StaticActors[0].EntityID != guard.EntityID || preview.BeforeMapOccupancy[1].StaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected destination before-map occupancy snapshot: %+v", preview.BeforeMapOccupancy[1])
	}
	if len(preview.AfterMapOccupancy) != 2 {
		t.Fatalf("expected 2 after-map occupancy snapshots, got %d", len(preview.AfterMapOccupancy))
	}
	if preview.AfterMapOccupancy[0].MapIndex != bootstrapMapIndex || preview.AfterMapOccupancy[0].CharacterCount != 1 || len(preview.AfterMapOccupancy[0].Characters) != 1 || preview.AfterMapOccupancy[0].StaticActorCount != 1 || len(preview.AfterMapOccupancy[0].StaticActors) != 1 || preview.AfterMapOccupancy[0].StaticActors[0].EntityID != blacksmith.EntityID || preview.AfterMapOccupancy[0].StaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected source after-map occupancy snapshot: %+v", preview.AfterMapOccupancy[0])
	}
	if preview.AfterMapOccupancy[1].MapIndex != 42 || preview.AfterMapOccupancy[1].CharacterCount != 2 || len(preview.AfterMapOccupancy[1].Characters) != 2 || preview.AfterMapOccupancy[1].StaticActorCount != 1 || len(preview.AfterMapOccupancy[1].StaticActors) != 1 || preview.AfterMapOccupancy[1].StaticActors[0].EntityID != guard.EntityID || preview.AfterMapOccupancy[1].StaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected destination after-map occupancy snapshot: %+v", preview.AfterMapOccupancy[1])
	}
}

func TestGameRuntimePreviewRelocationDoesNotMutateWorld(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerThree := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	peerThree.MapIndex = 42
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	issuePeerTicket(t, store, "peer-three", 0x33333333, peerThree)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	_, _ = enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_, _ = enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)

	beforeCharacters := runtime.ConnectedCharacters()
	beforeVisibility := runtime.CharacterVisibility()
	beforeMaps := runtime.MapOccupancy()

	preview, ok := runtime.PreviewRelocation("PeerTwo", 42, 1700, 2800)
	if !ok {
		t.Fatal("expected relocate preview to succeed")
	}
	if preview.Target.MapIndex != 42 {
		t.Fatalf("unexpected preview target: %+v", preview.Target)
	}

	afterCharacters := runtime.ConnectedCharacters()
	afterVisibility := runtime.CharacterVisibility()
	afterMaps := runtime.MapOccupancy()

	if !reflect.DeepEqual(afterCharacters, beforeCharacters) {
		t.Fatalf("expected connected characters to remain unchanged, before=%+v after=%+v", beforeCharacters, afterCharacters)
	}
	if !reflect.DeepEqual(afterVisibility, beforeVisibility) {
		t.Fatalf("expected character visibility to remain unchanged, before=%+v after=%+v", beforeVisibility, afterVisibility)
	}
	if !reflect.DeepEqual(afterMaps, beforeMaps) {
		t.Fatalf("expected map occupancy to remain unchanged, before=%+v after=%+v", beforeMaps, afterMaps)
	}
}

func TestGameRuntimePreviewRelocationRejectsUnknownTarget(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.PreviewRelocation("MissingPeer", 42, 1700, 2800); ok {
		t.Fatal("expected relocate preview to reject unknown target")
	}
}

func TestSharedWorldRegistryJoinUsesSessionDirectoryFrameSink(t *testing.T) {
	registry := newSharedWorldRegistry()
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	originalPending := newPendingServerFrames()

	peerOneID, visiblePeers := registry.Join(peerOne, originalPending, nil)
	if peerOneID == 0 {
		t.Fatal("expected first join to register a shared-world entity")
	}
	if len(visiblePeers) != 0 {
		t.Fatalf("expected first join to have no visible peers, got %+v", visiblePeers)
	}

	replacementPending := newPendingServerFrames()
	if !registry.sessionDirectory.Replace(peerOneID, newSharedWorldSessionEntry(replacementPending, nil)) {
		t.Fatal("expected session directory replace to succeed for existing peer")
	}

	peerTwoID, visiblePeers := registry.Join(peerTwo, newPendingServerFrames(), nil)
	if peerTwoID == 0 {
		t.Fatal("expected second join to register a shared-world entity")
	}
	if len(visiblePeers) != 1 || visiblePeers[0].VID != peerOne.VID {
		t.Fatalf("expected second join to see peer one, got %+v", visiblePeers)
	}
	if frames := originalPending.flush(); len(frames) != 0 {
		t.Fatalf("expected replaced frame sink to receive join replication instead of original sink, got %d frames", len(frames))
	}

	queued := replacementPending.flush()
	if len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames via replacement sink, got %d", len(queued))
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode queued peer add: %v", err)
	}
	if added.VID != peerTwo.VID {
		t.Fatalf("expected queued peer add for VID %#08x, got %#08x", peerTwo.VID, added.VID)
	}
}

func TestSharedWorldRegistryLeaveUsesSessionDirectoryFrameSink(t *testing.T) {
	registry := newSharedWorldRegistry()
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	originalPending := newPendingServerFrames()

	peerOneID, _ := registry.Join(peerOne, originalPending, nil)
	peerTwoID, _ := registry.Join(peerTwo, newPendingServerFrames(), nil)
	_ = originalPending.flush()

	replacementPending := newPendingServerFrames()
	if !registry.sessionDirectory.Replace(peerOneID, newSharedWorldSessionEntry(replacementPending, nil)) {
		t.Fatal("expected session directory replace to succeed for existing peer")
	}

	registry.Leave(peerTwoID)
	if frames := originalPending.flush(); len(frames) != 0 {
		t.Fatalf("expected replaced frame sink to receive leave replication instead of original sink, got %d frames", len(frames))
	}

	queued := replacementPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected 1 queued peer-exit frame via replacement sink, got %d", len(queued))
	}
	removed, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode queued peer delete: %v", err)
	}
	if removed.VID != peerTwo.VID {
		t.Fatalf("expected queued peer delete for VID %#08x, got %#08x", peerTwo.VID, removed.VID)
	}
}

func TestSharedWorldRegistryJoinReclaimsStaleEntityWhenSessionDirectoryEntryIsMissing(t *testing.T) {
	registry := newSharedWorldRegistry()
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	stalePending := newPendingServerFrames()

	staleID, visiblePeers := registry.Join(peer, stalePending, nil)
	if staleID == 0 {
		t.Fatal("expected first join to register a shared-world entity")
	}
	if len(visiblePeers) != 0 {
		t.Fatalf("expected first join to have no visible peers, got %+v", visiblePeers)
	}
	if _, ok := registry.sessionDirectory.Remove(staleID); !ok {
		t.Fatal("expected stale session-directory entry removal to succeed")
	}

	replacementPending := newPendingServerFrames()
	replacementID, visiblePeers := registry.Join(peer, replacementPending, nil)
	if replacementID == 0 {
		t.Fatal("expected second join to reclaim stale ownership and register a shared-world entity")
	}
	if replacementID == staleID {
		t.Fatalf("expected reclaimed join to allocate a fresh entity ID, got stale=%d replacement=%d", staleID, replacementID)
	}
	if len(visiblePeers) != 0 {
		t.Fatalf("expected reclaimed join to have no visible peers, got %+v", visiblePeers)
	}
	if _, ok := registry.entities.Player(staleID); ok {
		t.Fatal("expected stale entity to be removed before reclaimed join succeeds")
	}
	if _, ok := registry.sessionDirectory.Lookup(staleID); ok {
		t.Fatal("expected stale session-directory entry to stay removed after reclaimed join")
	}
	if _, ok := registry.sessionDirectory.Lookup(replacementID); !ok {
		t.Fatal("expected replacement session-directory entry after reclaimed join")
	}
	snapshots := registry.ConnectedCharacters()
	if len(snapshots) != 1 || snapshots[0].Name != peer.Name {
		t.Fatalf("expected exactly one connected snapshot after reclaimed join, got %+v", snapshots)
	}
}

func TestSharedWorldRegistryJoinRejectsDuplicateWhenOnlySessionDirectoryEntrySurvives(t *testing.T) {
	registry := newSharedWorldRegistry()
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	originalPending := newPendingServerFrames()

	staleID, visiblePeers := registry.Join(peer, originalPending, nil)
	if staleID == 0 {
		t.Fatal("expected first join to register a shared-world entity")
	}
	if len(visiblePeers) != 0 {
		t.Fatalf("expected first join to have no visible peers, got %+v", visiblePeers)
	}
	if _, ok := registry.sessionDirectory.Lookup(staleID); !ok {
		t.Fatal("expected session-directory entry to exist before partial teardown simulation")
	}
	if _, ok := registry.entities.Remove(staleID); !ok {
		t.Fatal("expected entity removal to succeed before duplicate join attempt")
	}

	replacementPending := newPendingServerFrames()
	replacementID, visiblePeers := registry.Join(peer, replacementPending, nil)
	if replacementID != 0 {
		t.Fatalf("expected duplicate join to be rejected while stale session-directory entry still exists, got replacementID=%d visiblePeers=%+v", replacementID, visiblePeers)
	}
	if _, ok := registry.sessionDirectory.Lookup(staleID); !ok {
		t.Fatal("expected stale session-directory entry to remain the blocking live-conflict signal")
	}
	if snapshots := registry.ConnectedCharacters(); len(snapshots) != 0 {
		t.Fatalf("expected no connected snapshots after entity-only teardown with blocked duplicate join, got %+v", snapshots)
	}
	if frames := replacementPending.flush(); len(frames) != 0 {
		t.Fatalf("expected no replacement frames when duplicate join is rejected, got %d", len(frames))
	}
}

func TestSharedWorldRegistryJoinReclaimsStaleVisiblePeerWithDeleteThenReentry(t *testing.T) {
	registry := newSharedWorldRegistry()
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	peerOnePending := newPendingServerFrames()

	peerOneID, visiblePeers := registry.Join(peerOne, peerOnePending, nil)
	if peerOneID == 0 {
		t.Fatal("expected first join to register a shared-world entity")
	}
	if len(visiblePeers) != 0 {
		t.Fatalf("expected first join to have no visible peers, got %+v", visiblePeers)
	}
	staleID, visiblePeers := registry.Join(peerTwo, newPendingServerFrames(), nil)
	if staleID == 0 {
		t.Fatal("expected second join to register a stale shared-world entity")
	}
	if len(visiblePeers) != 1 || visiblePeers[0].VID != peerOne.VID {
		t.Fatalf("expected stale join to see peer one, got %+v", visiblePeers)
	}
	if frames := peerOnePending.flush(); len(frames) != 3 {
		t.Fatalf("expected initial stale peer entry frames, got %d", len(frames))
	}
	if _, ok := registry.sessionDirectory.Remove(staleID); !ok {
		t.Fatal("expected stale session-directory entry removal to succeed")
	}

	replacementPending := newPendingServerFrames()
	replacementID, visiblePeers := registry.Join(peerTwo, replacementPending, nil)
	if replacementID == 0 {
		t.Fatal("expected reclaimed join to register a replacement entity")
	}
	if len(visiblePeers) != 1 || visiblePeers[0].VID != peerOne.VID {
		t.Fatalf("expected reclaimed join to still see peer one, got %+v", visiblePeers)
	}
	queued := peerOnePending.flush()
	if len(queued) != 4 {
		t.Fatalf("expected stale delete plus fresh reentry frames for visible peer, got %d", len(queued))
	}
	removed, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode reclaimed stale delete: %v", err)
	}
	if removed.VID != peerTwo.VID {
		t.Fatalf("expected stale delete for VID %#08x, got %#08x", peerTwo.VID, removed.VID)
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode reclaimed peer add: %v", err)
	}
	if added.VID != peerTwo.VID {
		t.Fatalf("expected fresh reentry add for VID %#08x, got %#08x", peerTwo.VID, added.VID)
	}
	if _, ok := registry.sessionDirectory.Lookup(replacementID); !ok {
		t.Fatal("expected replacement session-directory entry after reclaimed reentry")
	}
	_ = replacementPending
}

func TestSharedWorldRegistryTransferUsesSessionDirectoryFrameSink(t *testing.T) {
	registry := newSharedWorldRegistry()
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	originalPending := newPendingServerFrames()

	peerOneID, _ := registry.Join(peerOne, originalPending, nil)
	peerTwoID, _ := registry.Join(peerTwo, newPendingServerFrames(), nil)
	_ = originalPending.flush()

	replacementPending := newPendingServerFrames()
	if !registry.sessionDirectory.Replace(peerOneID, newSharedWorldSessionEntry(replacementPending, nil)) {
		t.Fatal("expected session directory replace to succeed for existing peer")
	}

	movedPeer := peerTwo
	movedPeer.MapIndex = 42
	movedPeer.X = 1700
	movedPeer.Y = 2800
	preview, ok := registry.Transfer(peerTwoID, movedPeer)
	if !ok || !preview.Applied {
		t.Fatalf("expected transfer to succeed, got preview=%+v ok=%v", preview, ok)
	}
	if frames := originalPending.flush(); len(frames) != 0 {
		t.Fatalf("expected replaced frame sink to receive transfer replication instead of original sink, got %d frames", len(frames))
	}

	queued := replacementPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected 1 queued transfer frame via replacement sink, got %d", len(queued))
	}
	removed, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode queued transfer delete: %v", err)
	}
	if removed.VID != peerTwo.VID {
		t.Fatalf("expected queued transfer delete for VID %#08x, got %#08x", peerTwo.VID, removed.VID)
	}
}

func TestSharedWorldRegistryTransferCharacterUsesSessionDirectoryRelocator(t *testing.T) {
	registry := newSharedWorldRegistry()
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	originalCalls := 0
	replacementCalls := 0

	peerID, _ := registry.Join(peer, newPendingServerFrames(), func(mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
		originalCalls++
		return RelocationPreview{Applied: true, Target: ConnectedCharacterSnapshot{Name: peer.Name, MapIndex: mapIndex, X: x, Y: y}}, true
	})
	if peerID == 0 {
		t.Fatal("expected join to register a shared-world entity")
	}

	if !registry.sessionDirectory.Replace(peerID, newSharedWorldSessionEntry(nil, func(mapIndex uint32, x int32, y int32) (RelocationPreview, bool) {
		replacementCalls++
		return RelocationPreview{Applied: true, Target: ConnectedCharacterSnapshot{Name: peer.Name, MapIndex: mapIndex, X: x, Y: y}}, true
	})) {
		t.Fatal("expected session directory replace to succeed for relocator")
	}

	preview, ok := registry.TransferCharacter(peer.Name, 42, 1700, 2800)
	if !ok {
		t.Fatal("expected transfer character to succeed via replacement relocator")
	}
	if originalCalls != 0 {
		t.Fatalf("expected original relocator not to be used after replacement, got %d calls", originalCalls)
	}
	if replacementCalls != 1 {
		t.Fatalf("expected replacement relocator to be used exactly once, got %d calls", replacementCalls)
	}
	if preview.Target.MapIndex != 42 || preview.Target.X != 1700 || preview.Target.Y != 2800 {
		t.Fatalf("expected replacement relocator preview to be returned, got %+v", preview)
	}
}

func TestGameRuntimeReconnectSameCharacterAfterDisconnectKeepsSingleRuntimeEntry(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, _ := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	_ = flushServerFrames(t, flowOne)
	_ = flushServerFrames(t, flowTwo)

	closeSessionFlow(t, flowTwo)
	peerExit := flushServerFrames(t, flowOne)
	if len(peerExit) != 1 {
		t.Fatalf("expected 1 queued peer-exit frame after disconnect, got %d", len(peerExit))
	}

	flowTwoReconnected, reenterFrames := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(reenterFrames) != 8 {
		t.Fatalf("expected 8 bootstrap frames for reconnected peer with existing visible peer, got %d", len(reenterFrames))
	}
	peerReentry := flushServerFrames(t, flowOne)
	if len(peerReentry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames after reconnect, got %d", len(peerReentry))
	}
	queuedAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerReentry[0]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry add: %v", err)
	}
	if queuedAdd.VID != peerTwo.VID {
		t.Fatalf("expected queued peer re-entry add for VID %#08x, got %#08x", peerTwo.VID, queuedAdd.VID)
	}

	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 2 {
		t.Fatalf("expected exactly 2 connected character snapshots after reconnect, got %d", len(snapshots))
	}
	if snapshots[0].Name != "PeerOne" || snapshots[1].Name != "PeerTwo" {
		t.Fatalf("unexpected connected snapshots after reconnect: %+v", snapshots)
	}
	closeSessionFlow(t, flowTwoReconnected)
}

func TestGameRuntimeRejectsDuplicateEnterGameWhenOnlySessionDirectoryEntrySurvives(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for first player, got %d", len(firstEnter))
	}
	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected live player entity for PeerOne before simulating entity-only teardown")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Lookup(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected session-directory entry for live owner before simulating entity-only teardown")
	}
	if _, ok := runtime.sharedWorld.entities.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected entity removal to succeed before duplicate enter-game attempt")
	}

	flowTwo := factory()
	_ = mustCompleteSecureHandshake(t, flowTwo)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "peer-one", LoginKey: 0x11111111})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	if _, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if _, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err == nil {
		t.Fatal("expected duplicate enter game to be rejected while stale session-directory entry still exists")
	} else if err != worldentry.ErrEnterGameRejected {
		t.Fatalf("expected ErrEnterGameRejected, got %v", err)
	}

	phaseAware, ok := flowTwo.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected queued flow to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseLoading {
		t.Fatalf("expected rejected second session to remain in loading, got %q", phaseAware.CurrentPhase())
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Lookup(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale session-directory entry to keep blocking duplicate enter-game")
	}
	if snapshots := runtime.ConnectedCharacters(); len(snapshots) != 0 {
		t.Fatalf("expected no connected snapshots while only stale session-directory hook remains, got %+v", snapshots)
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected original session to receive no extra queued frames after rejected duplicate enter game, got %d", len(queued))
	}

	closeSessionFlow(t, flowOne)
	closeSessionFlow(t, flowTwo)
}

func TestGameRuntimeRejectsConcurrentEnterGameForSameCharacter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for first player, got %d", len(firstEnter))
	}

	flowTwo := factory()
	_ = mustCompleteSecureHandshake(t, flowTwo)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "peer-one", LoginKey: 0x11111111})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	if _, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if _, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err == nil {
		t.Fatal("expected concurrent enter game for the same character to be rejected")
	} else if err != worldentry.ErrEnterGameRejected {
		t.Fatalf("expected ErrEnterGameRejected, got %v", err)
	}

	phaseAware, ok := flowTwo.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected queued flow to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseLoading {
		t.Fatalf("expected rejected second session to remain in loading, got %q", phaseAware.CurrentPhase())
	}

	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 1 || snapshots[0].Name != "PeerOne" {
		t.Fatalf("expected only the original live runtime entry after rejected duplicate enter game, got %+v", snapshots)
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected original session to receive no extra queued frames after rejected duplicate enter game, got %d", len(queued))
	}

	closeSessionFlow(t, flowOne)
	closeSessionFlow(t, flowTwo)
}

func TestGameRuntimeAllowsRetryEnterGameAfterRejectedDuplicateWhenLiveOwnerCloses(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with watcher already visible, got %d", len(ownerEnter))
	}
	ownerEntry := flushServerFrames(t, flowWatcher)
	if len(ownerEntry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after owner join, got %d", len(ownerEntry))
	}

	flowRetry := factory()
	_ = mustCompleteSecureHandshake(t, flowRetry)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "peer-one", LoginKey: 0x11111111})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	if _, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if _, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected character select error: %v", err)
	}
	if _, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err == nil {
		t.Fatal("expected first retry-session enter game to be rejected while original owner is still live")
	} else if err != worldentry.ErrEnterGameRejected {
		t.Fatalf("expected ErrEnterGameRejected, got %v", err)
	}
	phaseAware, ok := flowRetry.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected retry session to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseLoading {
		t.Fatalf("expected retry session to remain in loading after rejection, got %q", phaseAware.CurrentPhase())
	}

	closeSessionFlow(t, flowOwner)
	watcherExit := flushServerFrames(t, flowWatcher)
	if len(watcherExit) != 1 {
		t.Fatalf("expected 1 queued peer-exit frame for watcher after owner close, got %d", len(watcherExit))
	}
	removedOwner, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherExit[0]))
	if err != nil {
		t.Fatalf("decode queued owner delete: %v", err)
	}
	if removedOwner.VID != peerOne.VID {
		t.Fatalf("expected queued owner delete for VID %#08x, got %#08x", peerOne.VID, removedOwner.VID)
	}

	retryEnter, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame retry error after owner close: %v", err)
	}
	if len(retryEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames on retry after owner close, got %d", len(retryEnter))
	}
	if phaseAware.CurrentPhase() != session.PhaseGame {
		t.Fatalf("expected retry session to reach game after owner close, got %q", phaseAware.CurrentPhase())
	}
	watcherReentry := flushServerFrames(t, flowWatcher)
	if len(watcherReentry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after retry join, got %d", len(watcherReentry))
	}
	queuedAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, watcherReentry[0]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry add: %v", err)
	}
	if queuedAdd.VID != peerOne.VID {
		t.Fatalf("expected queued peer re-entry add for VID %#08x, got %#08x", peerOne.VID, queuedAdd.VID)
	}

	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 2 {
		t.Fatalf("expected exactly 2 connected snapshots after retry join, got %+v", snapshots)
	}
	foundWatcher := false
	foundPeerOne := false
	for _, snapshot := range snapshots {
		switch snapshot.Name {
		case "Watcher":
			foundWatcher = true
		case "PeerOne":
			foundPeerOne = true
		}
	}
	if !foundWatcher || !foundPeerOne {
		t.Fatalf("expected connected snapshots for watcher and retried owner, got %+v", snapshots)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowRetry)
}

func TestGameRuntimeRetryEnterGameAfterTransferThenCloseUsesPersistedDestinationSnapshot(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcherOld := peerVisibilityCharacter("WatcherOld", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	watcherNew := peerVisibilityCharacter("WatcherNew", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	watcherNew.MapIndex = 42
	issuePeerTicket(t, store, "watcher-old", 0x10101010, watcherOld)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "watcher-new", 0x33333333, watcherNew)
	for _, account := range []accountstore.Account{
		{Login: "watcher-old", Empire: watcherOld.Empire, Characters: []loginticket.Character{watcherOld}},
		{Login: "peer-one", Empire: peerOne.Empire, Characters: []loginticket.Character{peerOne}},
		{Login: "watcher-new", Empire: watcherNew.Empire, Characters: []loginticket.Character{watcherNew}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcherOld, oldEnter := enterGameWithLoginTicket(t, factory, "watcher-old", 0x10101010)
	if len(oldEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for old-map watcher, got %d", len(oldEnter))
	}
	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with old-map watcher visible, got %d", len(ownerEnter))
	}
	oldOwnerEntry := flushServerFrames(t, flowWatcherOld)
	if len(oldOwnerEntry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for old-map watcher after owner join, got %d", len(oldOwnerEntry))
	}
	flowWatcherNew, newEnter := enterGameWithLoginTicket(t, factory, "watcher-new", 0x33333333)
	if len(newEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for destination-map watcher before owner transfer, got %d", len(newEnter))
	}
	if queued := flushServerFrames(t, flowWatcherNew); len(queued) != 0 {
		t.Fatalf("expected no queued frames for destination-map watcher before owner transfer, got %d", len(queued))
	}

	flowRetry := factory()
	_ = mustCompleteSecureHandshake(t, flowRetry)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "peer-one", LoginKey: 0x11111111})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	if _, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected retry login error: %v", err)
	}
	if _, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected retry character select error: %v", err)
	}
	if _, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err == nil {
		t.Fatal("expected waiting retry session to be rejected while original owner is still live")
	} else if err != worldentry.ErrEnterGameRejected {
		t.Fatalf("expected ErrEnterGameRejected, got %v", err)
	}
	phaseAware, ok := flowRetry.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected retry session to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseLoading {
		t.Fatalf("expected retry session to remain in loading after duplicate rejection, got %q", phaseAware.CurrentPhase())
	}

	transferOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected owner transfer move error: %v", err)
	}
	if len(transferOut) != 8 {
		t.Fatalf("expected 8 self transfer-rebootstrap frames for owner, got %d", len(transferOut))
	}
	oldTransferExit := flushServerFrames(t, flowWatcherOld)
	if len(oldTransferExit) != 1 {
		t.Fatalf("expected 1 old-map delete frame after owner transfer, got %d", len(oldTransferExit))
	}
	newTransferEntry := flushServerFrames(t, flowWatcherNew)
	if len(newTransferEntry) != 3 {
		t.Fatalf("expected 3 destination-map entry frames after owner transfer, got %d", len(newTransferEntry))
	}

	closeSessionFlow(t, flowOwner)
	if queued := flushServerFrames(t, flowWatcherOld); len(queued) != 0 {
		t.Fatalf("expected no old-map frames after transferred owner closes, got %d", len(queued))
	}
	newCloseExit := flushServerFrames(t, flowWatcherNew)
	if len(newCloseExit) != 1 {
		t.Fatalf("expected 1 destination-map delete frame after transferred owner closes, got %d", len(newCloseExit))
	}

	retryEnter, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame retry error after transfer-then-close: %v", err)
	}
	if len(retryEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames on retry after transfer-then-close, got %d", len(retryEnter))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, retryEnter[1]))
	if err != nil {
		t.Fatalf("decode retry self add after transfer-then-close: %v", err)
	}
	if selfAdd.VID != peerOne.VID || selfAdd.X != 1700 || selfAdd.Y != 2800 {
		t.Fatalf("expected retry self add to use persisted destination snapshot, got %+v", selfAdd)
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, retryEnter[5]))
	if err != nil {
		t.Fatalf("decode retry trailing peer add after transfer-then-close: %v", err)
	}
	if peerAdd.VID != watcherNew.VID {
		t.Fatalf("expected retry trailing peer add for destination watcher VID %#08x, got %#08x", watcherNew.VID, peerAdd.VID)
	}
	if phaseAware.CurrentPhase() != session.PhaseGame {
		t.Fatalf("expected retry session to reach game after transfer-then-close, got %q", phaseAware.CurrentPhase())
	}
	if queued := flushServerFrames(t, flowWatcherOld); len(queued) != 0 {
		t.Fatalf("expected no old-map re-entry frames after retry from persisted destination, got %d", len(queued))
	}
	newRetryEntry := flushServerFrames(t, flowWatcherNew)
	if len(newRetryEntry) != 3 {
		t.Fatalf("expected 3 destination-map re-entry frames after retry, got %d", len(newRetryEntry))
	}
	queuedAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, newRetryEntry[0]))
	if err != nil {
		t.Fatalf("decode destination-map re-entry add after retry: %v", err)
	}
	if queuedAdd.VID != peerOne.VID || queuedAdd.X != 1700 || queuedAdd.Y != 2800 {
		t.Fatalf("expected destination-map re-entry add at persisted location, got %+v", queuedAdd)
	}

	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after transfer-then-close: %v", err)
	}
	if len(persisted.Characters) != 1 || persisted.Characters[0].MapIndex != 42 || persisted.Characters[0].X != 1700 || persisted.Characters[0].Y != 2800 {
		t.Fatalf("expected persisted account snapshot at destination after transfer-then-close, got %+v", persisted)
	}
	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 3 {
		t.Fatalf("expected exactly 3 connected snapshots after retry from persisted destination, got %+v", snapshots)
	}
	foundOwnerAtDestination := false
	for _, snapshot := range snapshots {
		if snapshot.Name == "PeerOne" {
			if snapshot.MapIndex != 42 || snapshot.X != 1700 || snapshot.Y != 2800 {
				t.Fatalf("expected retried owner snapshot at destination, got %+v", snapshot)
			}
			foundOwnerAtDestination = true
		}
	}
	if !foundOwnerAtDestination {
		t.Fatalf("expected connected snapshots to include retried owner at destination, got %+v", snapshots)
	}

	closeSessionFlow(t, flowWatcherOld)
	closeSessionFlow(t, flowWatcherNew)
	closeSessionFlow(t, flowRetry)
}

func TestGameRuntimeCloseAfterTransferEmitsPeerDeleteWhenEntityRegistryEntryAlreadyMissing(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	watcherOld := peerVisibilityCharacter("WatcherOld", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	watcherNew := peerVisibilityCharacter("WatcherNew", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	watcherNew.MapIndex = 42
	issuePeerTicket(t, store, "watcher-old", 0x10101010, watcherOld)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "watcher-new", 0x33333333, watcherNew)

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcherOld, oldEnter := enterGameWithLoginTicket(t, factory, "watcher-old", 0x10101010)
	if len(oldEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for old-map watcher, got %d", len(oldEnter))
	}
	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with old-map watcher visible, got %d", len(ownerEnter))
	}
	if queued := flushServerFrames(t, flowWatcherOld); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for old-map watcher after owner join, got %d", len(queued))
	}
	flowWatcherNew, newEnter := enterGameWithLoginTicket(t, factory, "watcher-new", 0x33333333)
	if len(newEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for destination-map watcher before owner transfer, got %d", len(newEnter))
	}
	if queued := flushServerFrames(t, flowWatcherNew); len(queued) != 0 {
		t.Fatalf("expected no queued frames for destination-map watcher before owner transfer, got %d", len(queued))
	}

	transferOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected owner transfer move error: %v", err)
	}
	if len(transferOut) != 8 {
		t.Fatalf("expected 8 self transfer-rebootstrap frames for owner, got %d", len(transferOut))
	}
	oldTransferExit := flushServerFrames(t, flowWatcherOld)
	if len(oldTransferExit) != 1 {
		t.Fatalf("expected 1 old-map delete frame after owner transfer, got %d", len(oldTransferExit))
	}
	newTransferEntry := flushServerFrames(t, flowWatcherNew)
	if len(newTransferEntry) != 3 {
		t.Fatalf("expected 3 destination-map entry frames after owner transfer, got %d", len(newTransferEntry))
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected live player entity for PeerOne before simulating partial teardown")
	}
	if _, ok := runtime.sharedWorld.entities.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected entity removal to succeed before close hardening test")
	}

	closeSessionFlow(t, flowOwner)

	newCloseExit := flushServerFrames(t, flowWatcherNew)
	if len(newCloseExit) != 1 {
		t.Fatalf("expected 1 destination-map delete frame after close with entity already missing, got %d", len(newCloseExit))
	}
	removedOwner, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, newCloseExit[0]))
	if err != nil {
		t.Fatalf("decode destination-map delete after partial teardown close: %v", err)
	}
	if removedOwner.VID != peerOne.VID {
		t.Fatalf("expected destination-map delete for VID %#08x, got %#08x", peerOne.VID, removedOwner.VID)
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Lookup(ownerEntity.Entity.ID); ok {
		t.Fatal("expected close to remove stale session-directory entry after partial teardown")
	}
	if snapshots := runtime.ConnectedCharacters(); len(snapshots) != 2 {
		t.Fatalf("expected only watchers to remain connected after partial teardown close, got %+v", snapshots)
	}

	closeSessionFlow(t, flowWatcherOld)
	closeSessionFlow(t, flowWatcherNew)
}

func TestGameRuntimeEnterGameReclaimsStaleOwnershipWhenSessionDirectoryEntryIsMissing(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOne, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for first player, got %d", len(firstEnter))
	}
	staleEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected live player entity for PeerOne before simulating stale ownership")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(staleEntity.Entity.ID); !ok {
		t.Fatal("expected stale session-directory entry removal to succeed")
	}

	flowTwo, secondEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(secondEnter) != 5 {
		t.Fatalf("expected reclaimed enter game to return 5 bootstrap frames, got %d", len(secondEnter))
	}
	replacementEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected replacement live player entity after reclaimed enter game")
	}
	if replacementEntity.Entity.ID == staleEntity.Entity.ID {
		t.Fatalf("expected reclaimed enter game to replace stale entity ID %d, got same ID", staleEntity.Entity.ID)
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Lookup(replacementEntity.Entity.ID); !ok {
		t.Fatal("expected replacement session-directory entry after reclaimed enter game")
	}
	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 1 || snapshots[0].Name != "PeerOne" {
		t.Fatalf("expected exactly one connected snapshot after reclaimed enter game, got %+v", snapshots)
	}

	closeSessionFlow(t, flowOne)
	closeSessionFlow(t, flowTwo)
}

func TestSharedWorldRegistryLeaveRemovesSessionDirectoryEntryWhenEntityAlreadyMissing(t *testing.T) {
	registry := newSharedWorldRegistry()
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerID, _ := registry.Join(peer, newPendingServerFrames(), nil)
	if peerID == 0 {
		t.Fatal("expected join to register a shared-world entity")
	}
	if _, ok := registry.sessionDirectory.Lookup(peerID); !ok {
		t.Fatal("expected session directory entry to exist before cleanup")
	}
	if _, ok := registry.entities.Remove(peerID); !ok {
		t.Fatal("expected entity removal to succeed before stale cleanup test")
	}

	registry.Leave(peerID)
	if _, ok := registry.sessionDirectory.Lookup(peerID); ok {
		t.Fatal("expected leave to remove stale session-directory entry even when entity registry entry is already missing")
	}
}

func TestSharedWorldRegistryLeaveIsIdempotentAfterCleanup(t *testing.T) {
	registry := newSharedWorldRegistry()
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerID, _ := registry.Join(peer, newPendingServerFrames(), nil)
	if peerID == 0 {
		t.Fatal("expected join to register a shared-world entity")
	}

	registry.Leave(peerID)
	registry.Leave(peerID)

	if _, ok := registry.sessionDirectory.Lookup(peerID); ok {
		t.Fatal("expected session directory entry to stay removed after repeated leave")
	}
	if _, ok := registry.entities.Player(peerID); ok {
		t.Fatal("expected entity registry entry to stay removed after repeated leave")
	}
	if snapshots := registry.MapOccupancy(); len(snapshots) != 0 {
		t.Fatalf("expected repeated leave to keep map occupancy empty, got %+v", snapshots)
	}
}

func TestGameRuntimeRegistersAndListsStaticActors(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	guard, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	if guard.EntityID == 0 || guard.Name != "VillageGuard" || guard.MapIndex != 42 || guard.X != 1700 || guard.Y != 2800 || guard.RaceNum != 20300 {
		t.Fatalf("unexpected guard snapshot: %+v", guard)
	}
	blacksmith, ok := runtime.RegisterStaticActor("Blacksmith", 42, 1900, 3000, 20301)
	if !ok {
		t.Fatal("expected blacksmith registration to succeed")
	}
	if blacksmith.EntityID == 0 || blacksmith.EntityID == guard.EntityID {
		t.Fatalf("expected distinct non-zero static actor IDs, guard=%+v blacksmith=%+v", guard, blacksmith)
	}

	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 static actors in runtime snapshot, got %d", len(actors))
	}
	if actors[0].EntityID != blacksmith.EntityID || actors[0].Name != "Blacksmith" || actors[0].RaceNum != 20301 {
		t.Fatalf("expected Blacksmith first in sorted runtime snapshot, got %+v", actors[0])
	}
	if actors[1].EntityID != guard.EntityID || actors[1].Name != "VillageGuard" || actors[1].RaceNum != 20300 {
		t.Fatalf("expected VillageGuard second in sorted runtime snapshot, got %+v", actors[1])
	}
}

func TestGameRuntimeRegisterStaticActorRejectsInvalidSeed(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	if _, ok := runtime.RegisterStaticActor("", 42, 1700, 2800, 20300); ok {
		t.Fatal("expected blank-name static actor registration to fail")
	}
	if _, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 0); ok {
		t.Fatal("expected zero-race static actor registration to fail")
	}
	if actors := runtime.StaticActors(); len(actors) != 0 {
		t.Fatalf("expected invalid static actor registration to keep runtime snapshot empty, got %+v", actors)
	}
}

func TestGameRuntimeRemoveStaticActorUpdatesSnapshot(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	guard, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	blacksmith, ok := runtime.RegisterStaticActor("Blacksmith", 42, 1900, 3000, 20301)
	if !ok {
		t.Fatal("expected blacksmith registration to succeed")
	}

	removed, ok := runtime.RemoveStaticActor(guard.EntityID)
	if !ok || removed.EntityID != guard.EntityID {
		t.Fatalf("expected static actor removal to return guard snapshot, got actor=%+v ok=%v", removed, ok)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 static actor after removal, got %d", len(actors))
	}
	if actors[0].EntityID != blacksmith.EntityID || actors[0].Name != "Blacksmith" {
		t.Fatalf("expected Blacksmith to remain after guard removal, got %+v", actors[0])
	}
}

func enterGameWithLoginTicket(t *testing.T, factory service.SessionFactory, login string, loginKey uint32) (service.SessionFlow, [][]byte) {
	t.Helper()

	flow := factory()
	_ = mustCompleteSecureHandshake(t, flow)
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
	character.MapIndex = bootstrapMapIndex
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
