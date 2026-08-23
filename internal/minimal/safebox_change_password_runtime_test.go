package minimal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
)

func TestGameSessionFlowSafeboxChangePasswordPersistsAndRequiresNewPasswordOnOpen(t *testing.T) {
	root := t.TempDir()
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	safeboxPath := filepath.Join(root, "state", "safebox.json")
	owner := peerVisibilityCharacter("ChangePasswordOwner", 0x01030911, 0x02040911, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 911, Vnum: 27001, Count: 1, Slot: 2}}
	login := "change-password-owner"
	const loginKey uint32 = 0x92929211
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed change-password account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Size: 1,
	}})
	cfg := config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", SafeboxStorePath: safeboxPath}
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(cfg, ticketStore, accounts, interactionStore)
	if err != nil {
		t.Fatalf("unexpected change-password runtime error: %v", err)
	}
	currentTime := time.Unix(1700000911, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	changeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password 000000 vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected safebox_change_password error: %v", err)
	}
	if len(changeOut) != 1 {
		t.Fatalf("expected one change-password success chat, got %d", len(changeOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, changeOut[0]))
	if err != nil {
		t.Fatalf("decode change-password success chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != safeboxPasswordChangedInfoMessage {
		t.Fatalf("unexpected change-password success chat: %+v", delivery)
	}

	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewFileStore(safeboxPath))
	if err != nil {
		t.Fatalf("load safebox after change-password: %v", err)
	}
	if got := safeboxstore.CharacterPassword(snapshot, login, owner.ID); got != "vault2" {
		t.Fatalf("durable password after change=%q want vault2", got)
	}

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse interact after change-password: %v", err)
	}
	oldOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password 000000",
	})))
	if err != nil {
		t.Fatalf("unexpected old-password open after change: %v", err)
	}
	if len(oldOut) != 1 {
		t.Fatalf("expected SAFEBOX_WRONG_PASSWORD for old password after change, got %d", len(oldOut))
	}
	if _, err := itemproto.DecodeSafeboxWrongPassword(decodeSingleFrame(t, oldOut[0])); err != nil {
		t.Fatalf("decode SAFEBOX_WRONG_PASSWORD after change: %v", err)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse re-prompt after old reject: %v", err)
	}
	newOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected new-password open after change: %v", err)
	}
	if len(newOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE after new password, got %d", len(newOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, newOut[0]))
	if err != nil {
		t.Fatalf("decode SAFEBOX_SIZE after new password: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 1}) {
		t.Fatalf("unexpected SAFEBOX_SIZE after new password: %+v", size)
	}
}

func TestGameSessionFlowSafeboxChangePasswordRejectsWrongOldWithInfoChat(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ChangePasswordWrongOld", 0x01030912, 0x02040912, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "change-password-wrong-old", 0x92929212, owner)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected wrong-old change-password runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "change-password-wrong-old", 0x92929212)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password badpwd vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected wrong-old change-password error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one wrong-old info chat, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode wrong-old change-password chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.Message != safeboxPasswordWrongInfoMessage {
		t.Fatalf("unexpected wrong-old change-password chat: %+v", delivery)
	}
}
