package minimal

import (
	"io"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
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
	if len(moveOut) != 0 {
		t.Fatalf("expected no self move reply while gameplay transfer uses bootstrap trigger contract, got %d frames", len(moveOut))
	}

	moverFrames := flushServerFrames(t, flowTwo)
	if len(moverFrames) != 4 {
		t.Fatalf("expected 4 mover frames after triggered transfer, got %d", len(moverFrames))
	}
	removedPeer, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, moverFrames[0]))
	if err != nil {
		t.Fatalf("decode mover delete notice: %v", err)
	}
	if removedPeer.VID != peerOne.VID {
		t.Fatalf("expected mover delete notice for old-map peer %#08x, got %#08x", peerOne.VID, removedPeer.VID)
	}
	addedPeer, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moverFrames[1]))
	if err != nil {
		t.Fatalf("decode mover peer add: %v", err)
	}
	if addedPeer.VID != peerThree.VID || addedPeer.X != peerThree.X || addedPeer.Y != peerThree.Y {
		t.Fatalf("unexpected mover peer add: %+v", addedPeer)
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
	if len(syncOut) != 0 {
		t.Fatalf("expected no self sync_position reply while gameplay transfer uses bootstrap trigger contract, got %d frames", len(syncOut))
	}

	if queued := flushServerFrames(t, flowOne); len(queued) != 1 {
		t.Fatalf("expected 1 old-map frame after triggered sync_position transfer, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowTwo); len(queued) != 4 {
		t.Fatalf("expected 4 mover frames after triggered sync_position transfer, got %d", len(queued))
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

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, &failingAccountStore{}, []bootstrapTransferTrigger{{
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
	if len(transferOut) != 0 {
		t.Fatalf("expected no self move reply while gameplay transfer uses bootstrap trigger contract, got %d frames", len(transferOut))
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

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, &failingAccountStore{})
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

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, &failingAccountStore{})
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
