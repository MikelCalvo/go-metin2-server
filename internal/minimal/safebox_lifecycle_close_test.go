package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestGameSessionFlowTransferTriggerClosesOpenSafeboxWithCloseSafeboxCommand(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxTransferCloseOwner", 0x010307d1, 0x020407d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 771, Vnum: 27001, Count: 2, Slot: 5}}
	login := "safebox-transfer-close"
	issuePeerTicket(t, store, login, 0x62626262, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox transfer-close owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, nil, nil, nil, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected safebox transfer-close runtime error: %v", err)
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x62626262)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 5 {
		t.Fatalf("expected safebox transfer-close bootstrap to emit at least 5 frames, got %d", len(enterOut))
	}

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before transfer: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_safebox before transfer to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222328})))
	if err != nil {
		t.Fatalf("unexpected transfer-trigger move error with open safebox: %v", err)
	}
	if len(moveOut) < 2 {
		t.Fatalf("expected transfer-triggered safebox close to prepend CloseSafebox before transfer frames, got %d", len(moveOut))
	}
	assertCloseSafeboxCommandChatFrame(t, moveOut[0], "safebox transfer close")
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode self transfer add after safebox close: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 1700 || selfAdd.Y != 2800 {
		t.Fatalf("unexpected self transfer add after safebox close: %+v", selfAdd)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after transfer-triggered safebox close, got %d", len(queued))
	}

	alreadyClosedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected post-transfer /close_safebox error: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected transfer-triggered safebox close to clear open flag before later /close_safebox, got %d frames", len(alreadyClosedOut))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted safebox transfer-close account: %v", err)
	}
	if account.Characters[0].MapIndex != 42 || account.Characters[0].X != 1700 || account.Characters[0].Y != 2800 {
		t.Fatalf("expected persisted safebox transfer-close account to save the transfer destination, got %#v", account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold || len(account.Characters[0].Inventory) != 1 {
		t.Fatalf("expected persisted safebox transfer-close account to keep inventory/gold unchanged, got %#v", account.Characters[0])
	}
}

func TestGameSessionFlowPhaseSelectClosesOpenSafeboxWithCloseSafeboxCommand(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxPhaseSelectCloseOwner", 0x010307d2, 0x020407d2, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	login := "safebox-phase-select-close"
	issuePeerTicket(t, store, login, 0x63636363, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox phase-select-close owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected safebox phase-select-close runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x63636363)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before phase_select: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_safebox before phase_select to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}

	phaseSelectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/phase_select"})))
	if err != nil {
		t.Fatalf("unexpected /phase_select safebox close error: %v", err)
	}
	if len(phaseSelectOut) != 2 {
		t.Fatalf("expected safebox /phase_select to prepend 1 CloseSafebox before the select-phase frame, got %d", len(phaseSelectOut))
	}
	assertCloseSafeboxCommandChatFrame(t, phaseSelectOut[0], "safebox phase_select close")
	phase, err := control.DecodePhase(decodeSingleFrame(t, phaseSelectOut[1]))
	if err != nil {
		t.Fatalf("decode phase-select frame after safebox close: %v", err)
	}
	if phase.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q after safebox /phase_select, got %q", session.PhaseSelect, phase.Phase)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after safebox /phase_select close, got %d", len(queued))
	}
}
