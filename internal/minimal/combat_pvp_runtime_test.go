package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"strconv"
)

func TestGameRuntimePVPSlashPaintsChallengeMarkForVisiblePeer(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PVPChallenger", 0x01030801, 0x02040801, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("PVPTarget", 0x01030802, 0x02040802, 1120, 2120, 0, 101, 201)
	watcher := peerVisibilityCharacter("PVPWatcher", 0x01030803, 0x02040803, 1140, 2140, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "pvp-challenger", 0x70708001, owner)
	issuePeerTicket(t, ticketStore, "pvp-target", 0x70708002, peer)
	issuePeerTicket(t, ticketStore, "pvp-watcher", 0x70708003, watcher)
	if err := accounts.Save(accountstore.Account{Login: "pvp-challenger", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pvp challenger account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "pvp-target", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed pvp target account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "pvp-watcher", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed pvp watcher account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected pvp presentation runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pvp-challenger", 0x70708001)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pvp-target", 0x70708002)
	defer closeSessionFlow(t, peerFlow)
	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pvp-watcher", 0x70708003)
	defer closeSessionFlow(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)
	_ = flushServerFrames(t, watcherFlow)

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/pvp " + formatUint(peer.VID),
	})))
	if err != nil {
		t.Fatalf("unexpected /pvp packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected challenger to receive one PVP frame, got %d", len(out))
	}
	assertPVPPresentationFrame(t, out[0], owner.VID, peer.VID, "challenger response")
	queued := flushServerFrames(t, peerFlow)
	if len(queued) != 1 {
		t.Fatalf("expected target to receive one queued PVP frame, got %d", len(queued))
	}
	assertPVPPresentationFrame(t, queued[0], owner.VID, peer.VID, "target queued response")
	if watcherQueued := flushServerFrames(t, watcherFlow); len(watcherQueued) != 0 {
		t.Fatalf("expected a third visible player to receive no PVP frame, got %d", len(watcherQueued))
	}
	if talk := flushServerFrames(t, ownerFlow); len(talk) != 0 {
		t.Fatalf("expected accepted /pvp to leave no extra challenger frames, got %d", len(talk))
	}
}

func TestGameRuntimePVPSlashFailsClosedWithoutPresentation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PVPRejectOwner", 0x01030811, 0x02040811, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("PVPRejectPeer", 0x01030812, 0x02040812, 1120, 2120, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "pvp-reject-owner", 0x70708011, owner)
	issuePeerTicket(t, ticketStore, "pvp-reject-peer", 0x70708012, peer)
	if err := accounts.Save(accountstore.Account{Login: "pvp-reject-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pvp reject owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "pvp-reject-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed pvp reject peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected pvp reject runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pvp-reject-owner", 0x70708011)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pvp-reject-peer", 0x70708012)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	rejects := []chatproto.ClientChatPacket{
		{Type: chatproto.ChatTypeTalking, Message: "/pvp"},
		{Type: chatproto.ChatTypeTalking, Message: "/pvp 0"},
		{Type: chatproto.ChatTypeTalking, Message: "/pvp abc"},
		{Type: chatproto.ChatTypeTalking, Message: "/pvp " + formatUint(peer.VID) + " extra"},
		{Type: chatproto.ChatTypeParty, Message: "/pvp " + formatUint(peer.VID)},
		{Type: chatproto.ChatTypeTalking, Message: "/pvp " + formatUint(owner.VID)},
		{Type: chatproto.ChatTypeTalking, Message: "/pvp 999999"},
	}
	for _, packet := range rejects {
		out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(packet)))
		if err != nil {
			t.Fatalf("unexpected reject packet error for %q: %v", packet.Message, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %q to emit no challenger frames, got %d", packet.Message, len(out))
		}
		if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
			t.Fatalf("expected %q to queue no target frames, got %d", packet.Message, len(queued))
		}
	}
}

func TestGameRuntimeOrdinaryCombatOmitsPVPAndDuelStart(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PVPSilentOwner", 0x01030821, 0x02040821, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "pvp-silent-owner", 0x70708021, owner)
	if err := accounts.Save(accountstore.Account{Login: "pvp-silent-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pvp silent owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected pvp silent runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("Training Dummy", bootstrapMapIndex, 1200, 2200, 101)
	if !ok {
		t.Fatal("expected training dummy registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pvp-silent-owner", 0x70708021)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	targetRaw := combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, targetRaw)); err != nil {
		t.Fatalf("unexpected target packet error: %v", err)
	}
	_ = flushServerFrames(t, flow)
	attackRaw := combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: 0, TargetVID: uint32(actor.EntityID)})
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, attackRaw))
	if err != nil {
		t.Fatalf("unexpected attack packet error: %v", err)
	}
	assertNoPVPOrDuelStart(t, out, "ordinary dummy attack")
}

func assertPVPPresentationFrame(t *testing.T, raw []byte, sourceVID uint32, destinationVID uint32, context string) {
	t.Helper()
	decoded, err := combatproto.DecodeServerPVP(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode PVP frame %s: %v", context, err)
	}
	if decoded.SourceVID != sourceVID || decoded.DestinationVID != destinationVID || decoded.Mode != combatproto.ServerPVPModeRevenge {
		t.Fatalf("unexpected PVP frame %s: %+v", context, decoded)
	}
}

func assertNoPVPOrDuelStart(t *testing.T, frames [][]byte, context string) {
	t.Helper()
	for _, raw := range frames {
		decoded := decodeSingleFrame(t, raw)
		if decoded.Header == combatproto.HeaderServerPVP || decoded.Header == combatproto.HeaderServerDuelStart {
			t.Fatalf("expected %s to omit PVP/DUEL_START, got header 0x%04X", context, decoded.Header)
		}
	}
}

func formatUint(value uint32) string {
	return strconv.FormatUint(uint64(value), 10)
}
