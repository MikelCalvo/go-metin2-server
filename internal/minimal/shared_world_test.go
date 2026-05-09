package minimal

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	worldentry "github.com/MikelCalvo/go-metin2-server/internal/worldentry"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const defaultMerchantPreview = "Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g; [2] Small Red Potion x2 @ 100g"

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

func TestNewGameSessionFactoryProjectsPeerEquipmentAppearanceDuringBootstrap(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerOne.Equipment = []inventory.ItemInstance{
		{ID: 81, Vnum: 11500, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody},
		{ID: 82, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
		{ID: 83, Vnum: 50053, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotHead},
	}
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	_, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for first player with equipped items, got %d", len(firstEnter))
	}

	_, secondEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(secondEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for second player with equipped peer snapshot, got %d", len(secondEnter))
	}

	peerInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, secondEnter[6]))
	if err != nil {
		t.Fatalf("decode peer character additional info: %v", err)
	}
	if peerInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 11200, 50053, 201} {
		t.Fatalf("unexpected projected peer additional-info parts: %+v", peerInfo.Parts)
	}

	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, secondEnter[7]))
	if err != nil {
		t.Fatalf("decode peer character update: %v", err)
	}
	if peerUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 11200, 50053, 201} {
		t.Fatalf("unexpected projected peer update parts: %+v", peerUpdate.Parts)
	}
}

func TestGameRuntimeEquipQueuesPeerAppearanceUpdateForVisibleWatcher(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 11500, Count: 1, Slot: 8}}
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwner, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after owner join, got %d", len(queued))
	}

	equipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip error: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected 3 self equip frames, got %d", len(equipOut))
	}

	peerFrames := flushServerFrames(t, flowWatcher)
	if len(peerFrames) != 1 {
		t.Fatalf("expected 1 queued peer appearance update after equip, got %d", len(peerFrames))
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, peerFrames[0]))
	if err != nil {
		t.Fatalf("decode queued peer equip update: %v", err)
	}
	if update.VID != owner.VID || update.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 201} {
		t.Fatalf("unexpected queued peer equip update: %+v", update)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwner)
}

func TestGameRuntimeEquipSkipsPeerAppearanceUpdateForNonProjectedSlot(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1002, Vnum: 13000, Count: 1, Slot: 8}}
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwner, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after owner join, got %d", len(queued))
	}

	equipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 shield"})))
	if err != nil {
		t.Fatalf("unexpected shield equip error: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected 3 self equip frames for shield equip, got %d", len(equipOut))
	}
	if peerFrames := flushServerFrames(t, flowWatcher); len(peerFrames) != 0 {
		t.Fatalf("expected no queued peer appearance update for non-projected shield equip, got %d", len(peerFrames))
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwner)
}

func TestGameRuntimeUnequipQueuesPeerAppearanceUpdateForVisibleWatcher(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Equipment = []inventory.ItemInstance{{ID: 2002, Vnum: 11200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwner, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after owner join, got %d", len(queued))
	}

	unequipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected 3 self unequip frames, got %d", len(unequipOut))
	}

	peerFrames := flushServerFrames(t, flowWatcher)
	if len(peerFrames) != 1 {
		t.Fatalf("expected 1 queued peer appearance update after unequip, got %d", len(peerFrames))
	}
	update, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, peerFrames[0]))
	if err != nil {
		t.Fatalf("decode queued peer unequip update: %v", err)
	}
	if update.VID != owner.VID || update.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected queued peer unequip update: %+v", update)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwner)
}

func TestGameRuntimeLateJoinSeesPeerAppearanceAfterRuntimeEquip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 3001, Vnum: 11500, Count: 1, Slot: 8}}
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	for _, account := range []accountstore.Account{
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOwner, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	equipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip error: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected 3 self equip frames before late join, got %d", len(equipOut))
	}

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for late-joining watcher, got %d", len(watcherEnter))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, watcherEnter[5]))
	if err != nil {
		t.Fatalf("decode late-join peer add after equip: %v", err)
	}
	if peerAdd.VID != owner.VID {
		t.Fatalf("unexpected late-join peer add after equip: %+v", peerAdd)
	}
	peerInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, watcherEnter[6]))
	if err != nil {
		t.Fatalf("decode late-join peer additional info after equip: %v", err)
	}
	if peerInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 201} {
		t.Fatalf("unexpected late-join peer additional info parts after equip: %+v", peerInfo.Parts)
	}
	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, watcherEnter[7]))
	if err != nil {
		t.Fatalf("decode late-join peer update after equip: %v", err)
	}
	if peerUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 201} {
		t.Fatalf("unexpected late-join peer update parts after equip: %+v", peerUpdate.Parts)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwner)
}

func TestGameRuntimeLateJoinSeesPeerAppearanceAfterRuntimeUnequip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Equipment = []inventory.ItemInstance{{ID: 3002, Vnum: 11200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	for _, account := range []accountstore.Account{
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOwner, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	unequipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected 3 self unequip frames before late join, got %d", len(unequipOut))
	}

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for late-joining watcher after unequip, got %d", len(watcherEnter))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, watcherEnter[5]))
	if err != nil {
		t.Fatalf("decode late-join peer add after unequip: %v", err)
	}
	if peerAdd.VID != owner.VID {
		t.Fatalf("unexpected late-join peer add after unequip: %+v", peerAdd)
	}
	peerInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, watcherEnter[6]))
	if err != nil {
		t.Fatalf("decode late-join peer additional info after unequip: %v", err)
	}
	if peerInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected late-join peer additional info parts after unequip: %+v", peerInfo.Parts)
	}
	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, watcherEnter[7]))
	if err != nil {
		t.Fatalf("decode late-join peer update after unequip: %v", err)
	}
	if peerUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected late-join peer update parts after unequip: %+v", peerUpdate.Parts)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwner)
}

func TestGameRuntimeRadiusAOIMoveIntoRangeSeesPeerAppearanceAfterRuntimeEquip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 4001, Vnum: 11500, Count: 1, Slot: 8}}
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1700, 2900, 0, 100, 200)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	for _, account := range []accountstore.Account{
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerEnter) != 6 {
		t.Fatalf("expected 6 bootstrap frames for owner outside radius AOI with one carried item, got %d", len(ownerEnter))
	}
	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher outside radius AOI, got %d", len(watcherEnter))
	}
	if queued := flushServerFrames(t, flowOwner); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames for owner outside radius AOI, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames for watcher outside radius AOI, got %d", len(queued))
	}

	equipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip error outside radius AOI: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected 3 self equip frames outside radius AOI, got %d", len(equipOut))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected no queued peer appearance frames while watcher remains outside radius AOI, got %d", len(queued))
	}

	moveOut, err := flowWatcher.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1300, Y: 2300, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move-into-range error after equip: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame for watcher entering range after equip, got %d", len(moveOut))
	}

	peerEntry := flushServerFrames(t, flowOwner)
	if len(peerEntry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for owner after watcher enters range, got %d", len(peerEntry))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerEntry[0]))
	if err != nil {
		t.Fatalf("decode watcher add after move into range: %v", err)
	}
	if peerAdd.VID != watcher.VID || peerAdd.X != 1300 || peerAdd.Y != 2300 {
		t.Fatalf("unexpected watcher add after move into range: %+v", peerAdd)
	}

	originEntry := flushServerFrames(t, flowWatcher)
	if len(originEntry) != 3 {
		t.Fatalf("expected 3 queued owner peer-entry frames for watcher after entering range, got %d", len(originEntry))
	}
	originAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, originEntry[0]))
	if err != nil {
		t.Fatalf("decode owner add after watcher enters range: %v", err)
	}
	if originAdd.VID != owner.VID || originAdd.X != owner.X || originAdd.Y != owner.Y {
		t.Fatalf("unexpected owner add after watcher enters range: %+v", originAdd)
	}
	originInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, originEntry[1]))
	if err != nil {
		t.Fatalf("decode owner additional info after watcher enters range: %v", err)
	}
	if originInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 201} {
		t.Fatalf("unexpected owner additional-info parts after runtime equip then move into range: %+v", originInfo.Parts)
	}
	originUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, originEntry[2]))
	if err != nil {
		t.Fatalf("decode owner update after watcher enters range: %v", err)
	}
	if originUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 201} {
		t.Fatalf("unexpected owner update parts after runtime equip then move into range: %+v", originUpdate.Parts)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwner)
}

func TestGameRuntimeRadiusAOIMoveIntoRangeSeesPeerAppearanceAfterRuntimeUnequip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Equipment = []inventory.ItemInstance{{ID: 4002, Vnum: 11200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1700, 2900, 0, 100, 200)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	for _, account := range []accountstore.Account{
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerEnter) != 6 {
		t.Fatalf("expected 6 bootstrap frames for owner outside radius AOI with one equipped item, got %d", len(ownerEnter))
	}
	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher outside radius AOI, got %d", len(watcherEnter))
	}
	if queued := flushServerFrames(t, flowOwner); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames for owner outside radius AOI, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames for watcher outside radius AOI, got %d", len(queued))
	}

	unequipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error outside radius AOI: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected 3 self unequip frames outside radius AOI, got %d", len(unequipOut))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected no queued peer appearance frames while watcher remains outside radius AOI after unequip, got %d", len(queued))
	}

	moveOut, err := flowWatcher.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1300, Y: 2300, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move-into-range error after unequip: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame for watcher entering range after unequip, got %d", len(moveOut))
	}

	peerEntry := flushServerFrames(t, flowOwner)
	if len(peerEntry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for owner after watcher enters range post-unequip, got %d", len(peerEntry))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerEntry[0]))
	if err != nil {
		t.Fatalf("decode watcher add after move into range post-unequip: %v", err)
	}
	if peerAdd.VID != watcher.VID || peerAdd.X != 1300 || peerAdd.Y != 2300 {
		t.Fatalf("unexpected watcher add after move into range post-unequip: %+v", peerAdd)
	}

	originEntry := flushServerFrames(t, flowWatcher)
	if len(originEntry) != 3 {
		t.Fatalf("expected 3 queued owner peer-entry frames for watcher after entering range post-unequip, got %d", len(originEntry))
	}
	originAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, originEntry[0]))
	if err != nil {
		t.Fatalf("decode owner add after watcher enters range post-unequip: %v", err)
	}
	if originAdd.VID != owner.VID || originAdd.X != owner.X || originAdd.Y != owner.Y {
		t.Fatalf("unexpected owner add after watcher enters range post-unequip: %+v", originAdd)
	}
	originInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, originEntry[1]))
	if err != nil {
		t.Fatalf("decode owner additional info after watcher enters range post-unequip: %v", err)
	}
	if originInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected owner additional-info parts after runtime unequip then move into range: %+v", originInfo.Parts)
	}
	originUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, originEntry[2]))
	if err != nil {
		t.Fatalf("decode owner update after watcher enters range post-unequip: %v", err)
	}
	if originUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected owner update parts after runtime unequip then move into range: %+v", originUpdate.Parts)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwner)
}

func TestNewGameSessionFactoryAppendsVisibleStaticActorFramesAfterPeerBootstrap(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	blacksmith, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1200, 2200, 20301)
	if !ok {
		t.Fatal("expected bootstrap static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOne, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for first player with visible static actor, got %d", len(firstEnter))
	}
	staticAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, firstEnter[5]))
	if err != nil {
		t.Fatalf("decode first bootstrap static actor add: %v", err)
	}
	if staticAdd.VID != uint32(blacksmith.EntityID) || staticAdd.Type != 1 || staticAdd.X != 1200 || staticAdd.Y != 2200 || staticAdd.RaceNum != 20301 {
		t.Fatalf("unexpected first bootstrap static actor add: %+v", staticAdd)
	}
	staticInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, firstEnter[6]))
	if err != nil {
		t.Fatalf("decode first bootstrap static actor additional info: %v", err)
	}
	if staticInfo.VID != uint32(blacksmith.EntityID) || staticInfo.Name != "Blacksmith" {
		t.Fatalf("unexpected first bootstrap static actor additional info: %+v", staticInfo)
	}
	staticUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, firstEnter[7]))
	if err != nil {
		t.Fatalf("decode first bootstrap static actor update: %v", err)
	}
	if staticUpdate.VID != uint32(blacksmith.EntityID) {
		t.Fatalf("unexpected first bootstrap static actor update: %+v", staticUpdate)
	}

	_, secondEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(secondEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for second player with peer and static actor snapshots, got %d", len(secondEnter))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, secondEnter[5]))
	if err != nil {
		t.Fatalf("decode peer add before static actor burst: %v", err)
	}
	if peerAdd.VID != peerOne.VID {
		t.Fatalf("expected peer bootstrap burst before static actor frames, got %+v", peerAdd)
	}
	staticAdd, err = worldproto.DecodeCharacterAdd(decodeSingleFrame(t, secondEnter[8]))
	if err != nil {
		t.Fatalf("decode second bootstrap static actor add: %v", err)
	}
	if staticAdd.VID != uint32(blacksmith.EntityID) || staticAdd.Type != 1 || staticAdd.X != 1200 || staticAdd.Y != 2200 || staticAdd.RaceNum != 20301 {
		t.Fatalf("unexpected second bootstrap static actor add: %+v", staticAdd)
	}
	staticInfo, err = worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, secondEnter[9]))
	if err != nil {
		t.Fatalf("decode second bootstrap static actor additional info: %v", err)
	}
	if staticInfo.VID != uint32(blacksmith.EntityID) || staticInfo.Name != "Blacksmith" {
		t.Fatalf("unexpected second bootstrap static actor additional info: %+v", staticInfo)
	}
	staticUpdate, err = worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, secondEnter[10]))
	if err != nil {
		t.Fatalf("decode second bootstrap static actor update: %v", err)
	}
	if staticUpdate.VID != uint32(blacksmith.EntityID) {
		t.Fatalf("unexpected second bootstrap static actor update: %+v", staticUpdate)
	}

	peerEntry := flushServerFrames(t, flowOne)
	if len(peerEntry) != 3 {
		t.Fatalf("expected queued peer-entry frames only for the new player, got %d", len(peerEntry))
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

func TestNewGameSessionFactoryRadiusAOIOnlyBootstrapsNearbyStaticActors(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	nearActor, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1200, 2200, 20301)
	if !ok {
		t.Fatal("expected near static actor registration to succeed")
	}
	if _, ok := runtime.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 2000, 3200, 20300); !ok {
		t.Fatal("expected far static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOne, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with only one nearby static actor, got %d", len(enterOut))
	}
	staticAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, enterOut[5]))
	if err != nil {
		t.Fatalf("decode nearby static actor add: %v", err)
	}
	if staticAdd.VID != uint32(nearActor.EntityID) || staticAdd.Type != 1 || staticAdd.X != 1200 || staticAdd.Y != 2200 || staticAdd.RaceNum != 20301 {
		t.Fatalf("unexpected nearby static actor add: %+v", staticAdd)
	}
	staticInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, enterOut[6]))
	if err != nil {
		t.Fatalf("decode nearby static actor additional info: %v", err)
	}
	if staticInfo.VID != uint32(nearActor.EntityID) || staticInfo.Name != "Blacksmith" {
		t.Fatalf("unexpected nearby static actor additional info: %+v", staticInfo)
	}
	staticUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, enterOut[7]))
	if err != nil {
		t.Fatalf("decode nearby static actor update: %v", err)
	}
	if staticUpdate.VID != uint32(nearActor.EntityID) {
		t.Fatalf("unexpected nearby static actor update: %+v", staticUpdate)
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no queued frames for a single entering player with nearby static actors, got %d", len(queued))
	}
}

func TestNewGameSessionFactoryRadiusAOIMoveIntoRangeBootstrapsStaticActorVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1700, 2900, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1300, 2300, 20301)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOne, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 5 {
		t.Fatalf("expected 5 bootstrap frames for player outside static-actor AOI, got %d", len(enterOut))
	}

	moveOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1300, Y: 2300, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame, got %d", len(moveOut))
	}

	originEntry := flushServerFrames(t, flowOne)
	if len(originEntry) != 3 {
		t.Fatalf("expected 3 queued static-actor entry frames after move into radius AOI, got %d", len(originEntry))
	}
	staticAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, originEntry[0]))
	if err != nil {
		t.Fatalf("decode static actor add after move into AOI: %v", err)
	}
	if staticAdd.VID != uint32(actor.EntityID) || staticAdd.Type != 1 || staticAdd.X != 1300 || staticAdd.Y != 2300 || staticAdd.RaceNum != 20301 {
		t.Fatalf("unexpected static actor add after move into AOI: %+v", staticAdd)
	}
	staticInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, originEntry[1]))
	if err != nil {
		t.Fatalf("decode static actor additional info after move into AOI: %v", err)
	}
	if staticInfo.VID != uint32(actor.EntityID) || staticInfo.Name != "Blacksmith" {
		t.Fatalf("unexpected static actor additional info after move into AOI: %+v", staticInfo)
	}
	staticUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, originEntry[2]))
	if err != nil {
		t.Fatalf("decode static actor update after move into AOI: %v", err)
	}
	if staticUpdate.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected static actor update after move into AOI: %+v", staticUpdate)
	}
}

func TestNewGameSessionFactoryRadiusAOIMoveOutOfRangeRemovesStaticActorVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1200, 2200, 20301)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOne, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for player inside static-actor AOI, got %d", len(enterOut))
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no queued frames after enter bootstrap with static actor, got %d", len(queued))
	}

	moveOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1900, Y: 3100, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame, got %d", len(moveOut))
	}

	originExit := flushServerFrames(t, flowOne)
	if len(originExit) != 1 {
		t.Fatalf("expected 1 queued static-actor delete after leaving radius AOI, got %d", len(originExit))
	}
	staticDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, originExit[0]))
	if err != nil {
		t.Fatalf("decode static actor delete after move out of AOI: %v", err)
	}
	if staticDelete.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected static actor delete after move out of AOI: %+v", staticDelete)
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

func TestNewGameSessionFactoryRadiusAOIMoveIntoRangeBootstrapsPeerVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1700, 2900, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for first player, got %d", len(firstEnter))
	}
	flowTwo, secondEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(secondEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for second player outside radius AOI, got %d", len(secondEnter))
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames outside radius AOI, got %d", len(queued))
	}

	moveOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1300, Y: 2300, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame, got %d", len(moveOut))
	}

	peerEntry := flushServerFrames(t, flowOne)
	if len(peerEntry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames after crossing into radius AOI, got %d", len(peerEntry))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerEntry[0]))
	if err != nil {
		t.Fatalf("decode peer add after move into AOI: %v", err)
	}
	if peerAdd.VID != peerTwo.VID || peerAdd.X != 1300 || peerAdd.Y != 2300 {
		t.Fatalf("unexpected peer add after move into AOI: %+v", peerAdd)
	}

	originEntry := flushServerFrames(t, flowTwo)
	if len(originEntry) != 3 {
		t.Fatalf("expected 3 queued origin peer-entry frames after crossing into radius AOI, got %d", len(originEntry))
	}
	originAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, originEntry[0]))
	if err != nil {
		t.Fatalf("decode origin add after move into AOI: %v", err)
	}
	if originAdd.VID != peerOne.VID || originAdd.X != peerOne.X || originAdd.Y != peerOne.Y {
		t.Fatalf("unexpected origin add after move into AOI: %+v", originAdd)
	}
}

func TestNewGameSessionFactoryRadiusAOIMoveOutOfRangeRemovesPeerVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, secondEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(secondEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for second player inside radius AOI, got %d", len(secondEnter))
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 3 {
		t.Fatalf("expected initial queued peer-entry frames inside radius AOI, got %d", len(queued))
	}

	moveOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1900, Y: 3100, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame, got %d", len(moveOut))
	}

	peerExit := flushServerFrames(t, flowOne)
	if len(peerExit) != 1 {
		t.Fatalf("expected 1 queued peer delete after leaving radius AOI, got %d", len(peerExit))
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, peerExit[0]))
	if err != nil {
		t.Fatalf("decode peer delete after move out of AOI: %v", err)
	}
	if peerDelete.VID != peerTwo.VID {
		t.Fatalf("unexpected peer delete after move out of AOI: %+v", peerDelete)
	}

	originExit := flushServerFrames(t, flowTwo)
	if len(originExit) != 1 {
		t.Fatalf("expected 1 queued origin delete after leaving radius AOI, got %d", len(originExit))
	}
	originDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, originExit[0]))
	if err != nil {
		t.Fatalf("decode origin delete after move out of AOI: %v", err)
	}
	if originDelete.VID != peerOne.VID {
		t.Fatalf("unexpected origin delete after move out of AOI: %+v", originDelete)
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

func TestNewGameSessionFactoryRadiusAOISyncPositionIntoRangeBootstrapsPeerVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1700, 2900, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, firstEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(firstEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for first player, got %d", len(firstEnter))
	}
	flowTwo, secondEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(secondEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for second player outside radius AOI, got %d", len(secondEnter))
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames outside radius AOI, got %d", len(queued))
	}

	syncOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: peerTwo.VID, X: 1300, Y: 2300}},
	})))
	if err != nil {
		t.Fatalf("unexpected sync_position error: %v", err)
	}
	if len(syncOut) != 1 {
		t.Fatalf("expected 1 self sync_position ack frame, got %d", len(syncOut))
	}

	peerEntry := flushServerFrames(t, flowOne)
	if len(peerEntry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames after sync_position crosses into radius AOI, got %d", len(peerEntry))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerEntry[0]))
	if err != nil {
		t.Fatalf("decode peer add after sync_position into AOI: %v", err)
	}
	if peerAdd.VID != peerTwo.VID || peerAdd.X != 1300 || peerAdd.Y != 2300 {
		t.Fatalf("unexpected peer add after sync_position into AOI: %+v", peerAdd)
	}

	originEntry := flushServerFrames(t, flowTwo)
	if len(originEntry) != 3 {
		t.Fatalf("expected 3 queued origin peer-entry frames after sync_position crosses into radius AOI, got %d", len(originEntry))
	}
	originAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, originEntry[0]))
	if err != nil {
		t.Fatalf("decode origin add after sync_position into AOI: %v", err)
	}
	if originAdd.VID != peerOne.VID || originAdd.X != peerOne.X || originAdd.Y != peerOne.Y {
		t.Fatalf("unexpected origin add after sync_position into AOI: %+v", originAdd)
	}
}

func TestNewGameSessionFactoryRadiusAOISyncPositionIntoRangeBootstrapsStaticActorVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1700, 2900, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1300, 2300, 20301)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOne, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 5 {
		t.Fatalf("expected 5 bootstrap frames for player outside static-actor AOI, got %d", len(enterOut))
	}

	syncOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: peerOne.VID, X: 1300, Y: 2300}},
	})))
	if err != nil {
		t.Fatalf("unexpected sync_position error: %v", err)
	}
	if len(syncOut) != 1 {
		t.Fatalf("expected 1 self sync_position ack frame, got %d", len(syncOut))
	}

	originEntry := flushServerFrames(t, flowOne)
	if len(originEntry) != 3 {
		t.Fatalf("expected 3 queued static-actor entry frames after sync_position into radius AOI, got %d", len(originEntry))
	}
	staticAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, originEntry[0]))
	if err != nil {
		t.Fatalf("decode static actor add after sync_position into AOI: %v", err)
	}
	if staticAdd.VID != uint32(actor.EntityID) || staticAdd.Type != 1 || staticAdd.X != 1300 || staticAdd.Y != 2300 || staticAdd.RaceNum != 20301 {
		t.Fatalf("unexpected static actor add after sync_position into AOI: %+v", staticAdd)
	}
	staticInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, originEntry[1]))
	if err != nil {
		t.Fatalf("decode static actor additional info after sync_position into AOI: %v", err)
	}
	if staticInfo.VID != uint32(actor.EntityID) || staticInfo.Name != "Blacksmith" {
		t.Fatalf("unexpected static actor additional info after sync_position into AOI: %+v", staticInfo)
	}
	staticUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, originEntry[2]))
	if err != nil {
		t.Fatalf("decode static actor update after sync_position into AOI: %v", err)
	}
	if staticUpdate.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected static actor update after sync_position into AOI: %+v", staticUpdate)
	}
}

func TestNewGameSessionFactoryRadiusAOISyncPositionOutOfRangeRemovesPeerVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	factory, err := newGameSessionFactory(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store)
	if err != nil {
		t.Fatalf("unexpected game session factory error: %v", err)
	}

	flowOne, _ := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	flowTwo, secondEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(secondEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for second player inside radius AOI, got %d", len(secondEnter))
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 3 {
		t.Fatalf("expected initial queued peer-entry frames inside radius AOI, got %d", len(queued))
	}

	syncOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: peerTwo.VID, X: 1900, Y: 3100}},
	})))
	if err != nil {
		t.Fatalf("unexpected sync_position error: %v", err)
	}
	if len(syncOut) != 1 {
		t.Fatalf("expected 1 self sync_position ack frame, got %d", len(syncOut))
	}

	peerExit := flushServerFrames(t, flowOne)
	if len(peerExit) != 1 {
		t.Fatalf("expected 1 queued peer delete after sync_position leaves radius AOI, got %d", len(peerExit))
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, peerExit[0]))
	if err != nil {
		t.Fatalf("decode peer delete after sync_position out of AOI: %v", err)
	}
	if peerDelete.VID != peerTwo.VID {
		t.Fatalf("unexpected peer delete after sync_position out of AOI: %+v", peerDelete)
	}

	originExit := flushServerFrames(t, flowTwo)
	if len(originExit) != 1 {
		t.Fatalf("expected 1 queued origin delete after sync_position leaves radius AOI, got %d", len(originExit))
	}
	originDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, originExit[0]))
	if err != nil {
		t.Fatalf("decode origin delete after sync_position out of AOI: %v", err)
	}
	if originDelete.VID != peerOne.VID {
		t.Fatalf("unexpected origin delete after sync_position out of AOI: %+v", originDelete)
	}
}

func TestNewGameSessionFactoryRadiusAOISyncPositionOutOfRangeRemovesStaticActorVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1200, 2200, 20301)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOne, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for player inside static-actor AOI, got %d", len(enterOut))
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no queued frames after enter bootstrap with static actor, got %d", len(queued))
	}

	syncOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: peerOne.VID, X: 1900, Y: 3100}},
	})))
	if err != nil {
		t.Fatalf("unexpected sync_position error: %v", err)
	}
	if len(syncOut) != 1 {
		t.Fatalf("expected 1 self sync_position ack frame, got %d", len(syncOut))
	}

	originExit := flushServerFrames(t, flowOne)
	if len(originExit) != 1 {
		t.Fatalf("expected 1 queued static-actor delete after sync_position leaves radius AOI, got %d", len(originExit))
	}
	staticDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, originExit[0]))
	if err != nil {
		t.Fatalf("decode static actor delete after sync_position out of AOI: %v", err)
	}
	if staticDelete.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected static actor delete after sync_position out of AOI: %+v", staticDelete)
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

func TestNewGameSessionFactoryAppliesExactPositionTransferTriggerOnMoveWithStaticActorVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

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
	sourceActor, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1200, 2200, 20301)
	if !ok {
		t.Fatal("expected source static actor registration to succeed")
	}
	targetActor, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected target static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowTwo, enterOut := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 enter-game frames with one visible source static actor, got %d", len(enterOut))
	}
	_ = flushServerFrames(t, flowTwo)

	moveOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected move error: %v", err)
	}
	if len(moveOut) != 8 {
		t.Fatalf("expected 8 self transfer-rebootstrap frames with static actor visibility deltas, got %d", len(moveOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode self transfer add with static actors: %v", err)
	}
	if selfAdd.VID != peerTwo.VID || selfAdd.X != 1700 || selfAdd.Y != 2800 {
		t.Fatalf("unexpected self transfer add with static actors: %+v", selfAdd)
	}
	removedActor, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, moveOut[4]))
	if err != nil {
		t.Fatalf("decode source static actor delete during transfer: %v", err)
	}
	if removedActor.VID != uint32(sourceActor.EntityID) {
		t.Fatalf("expected source static actor delete for entity %d, got %+v", sourceActor.EntityID, removedActor)
	}
	addedActor, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moveOut[5]))
	if err != nil {
		t.Fatalf("decode target static actor add during transfer: %v", err)
	}
	if addedActor.VID != uint32(targetActor.EntityID) || addedActor.Type != 1 || addedActor.X != 1700 || addedActor.Y != 2800 || addedActor.RaceNum != 20300 {
		t.Fatalf("unexpected target static actor add during transfer: %+v", addedActor)
	}
	actorInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, moveOut[6]))
	if err != nil {
		t.Fatalf("decode target static actor additional info during transfer: %v", err)
	}
	if actorInfo.VID != uint32(targetActor.EntityID) || actorInfo.Name != "VillageGuard" {
		t.Fatalf("unexpected target static actor additional info during transfer: %+v", actorInfo)
	}
	actorUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, moveOut[7]))
	if err != nil {
		t.Fatalf("decode target static actor update during transfer: %v", err)
	}
	if actorUpdate.VID != uint32(targetActor.EntityID) {
		t.Fatalf("unexpected target static actor update during transfer: %+v", actorUpdate)
	}
	if queued := flushServerFrames(t, flowTwo); len(queued) != 0 {
		t.Fatalf("expected no queued mover frames after immediate transfer rebootstrap with static actors, got %d", len(queued))
	}
}

func TestNewGameSessionFactoryAppliesExactPositionTransferTriggerOnSyncPositionWithStaticActorVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

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
	sourceActor, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1200, 2200, 20301)
	if !ok {
		t.Fatal("expected source static actor registration to succeed")
	}
	targetActor, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected target static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowTwo, enterOut := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 enter-game frames with one visible source static actor, got %d", len(enterOut))
	}
	_ = flushServerFrames(t, flowTwo)

	syncOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{Elements: []movep.SyncPositionElement{{VID: peerTwo.VID, X: 1500, Y: 2600}}})))
	if err != nil {
		t.Fatalf("unexpected sync_position error: %v", err)
	}
	if len(syncOut) != 8 {
		t.Fatalf("expected 8 self transfer-rebootstrap frames with static actor visibility deltas from sync_position, got %d", len(syncOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, syncOut[0]))
	if err != nil {
		t.Fatalf("decode self transfer add with static actors from sync_position: %v", err)
	}
	if selfAdd.VID != peerTwo.VID || selfAdd.X != 1700 || selfAdd.Y != 2800 {
		t.Fatalf("unexpected self transfer add with static actors from sync_position: %+v", selfAdd)
	}
	removedActor, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, syncOut[4]))
	if err != nil {
		t.Fatalf("decode source static actor delete during sync_position transfer: %v", err)
	}
	if removedActor.VID != uint32(sourceActor.EntityID) {
		t.Fatalf("expected source static actor delete for entity %d, got %+v", sourceActor.EntityID, removedActor)
	}
	addedActor, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, syncOut[5]))
	if err != nil {
		t.Fatalf("decode target static actor add during sync_position transfer: %v", err)
	}
	if addedActor.VID != uint32(targetActor.EntityID) || addedActor.Type != 1 || addedActor.X != 1700 || addedActor.Y != 2800 || addedActor.RaceNum != 20300 {
		t.Fatalf("unexpected target static actor add during sync_position transfer: %+v", addedActor)
	}
	actorInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, syncOut[6]))
	if err != nil {
		t.Fatalf("decode target static actor additional info during sync_position transfer: %v", err)
	}
	if actorInfo.VID != uint32(targetActor.EntityID) || actorInfo.Name != "VillageGuard" {
		t.Fatalf("unexpected target static actor additional info during sync_position transfer: %+v", actorInfo)
	}
	actorUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, syncOut[7]))
	if err != nil {
		t.Fatalf("decode target static actor update during sync_position transfer: %v", err)
	}
	if actorUpdate.VID != uint32(targetActor.EntityID) {
		t.Fatalf("unexpected target static actor update during sync_position transfer: %+v", actorUpdate)
	}
	if queued := flushServerFrames(t, flowTwo); len(queued) != 0 {
		t.Fatalf("expected no queued mover frames after immediate sync_position transfer rebootstrap with static actors, got %d", len(queued))
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

func TestGameRuntimeTransferCharacterQueuesPeerAppearanceAfterRuntimeEquip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	owner.Inventory = []inventory.ItemInstance{{ID: 5001, Vnum: 11500, Count: 1, Slot: 8}}
	watcher := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	watcher.MapIndex = 42
	issuePeerTicket(t, store, "peer-two", 0x22222222, owner)
	issuePeerTicket(t, store, "peer-three", 0x33333333, watcher)
	for _, account := range []accountstore.Account{
		{Login: "peer-two", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
		{Login: "peer-three", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(ownerEnter) != 6 {
		t.Fatalf("expected 6 bootstrap frames for owner with one carried item before transfer, got %d", len(ownerEnter))
	}
	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher on another map before transfer, got %d", len(watcherEnter))
	}
	if queued := flushServerFrames(t, flowOwner); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames across maps before transfer, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames for watcher across maps before transfer, got %d", len(queued))
	}

	equipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip error before transfer: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected 3 self equip frames before transfer, got %d", len(equipOut))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected no queued watcher frames while owner remains on another map after equip, got %d", len(queued))
	}

	result, ok := runtime.TransferCharacter("PeerTwo", 42, 1700, 2800)
	if !ok {
		t.Fatal("expected transfer to succeed after runtime equip")
	}
	if !result.Applied || result.Target.MapIndex != 42 || result.Target.X != 1700 || result.Target.Y != 2800 {
		t.Fatalf("unexpected transfer result after runtime equip: %+v", result)
	}

	originEntry := flushServerFrames(t, flowOwner)
	if len(originEntry) != 3 {
		t.Fatalf("expected 3 queued origin peer-entry frames for owner after transfer, got %d", len(originEntry))
	}
	originAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, originEntry[0]))
	if err != nil {
		t.Fatalf("decode watcher add for owner after transfer: %v", err)
	}
	if originAdd.VID != watcher.VID {
		t.Fatalf("unexpected watcher add for owner after transfer: %+v", originAdd)
	}

	peerEntry := flushServerFrames(t, flowWatcher)
	if len(peerEntry) != 3 {
		t.Fatalf("expected 3 queued owner peer-entry frames for watcher after transfer, got %d", len(peerEntry))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerEntry[0]))
	if err != nil {
		t.Fatalf("decode owner add for watcher after transfer: %v", err)
	}
	if peerAdd.VID != owner.VID || peerAdd.X != 1700 || peerAdd.Y != 2800 {
		t.Fatalf("unexpected owner add for watcher after transfer: %+v", peerAdd)
	}
	peerInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, peerEntry[1]))
	if err != nil {
		t.Fatalf("decode owner additional info for watcher after transfer: %v", err)
	}
	if peerInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 202} {
		t.Fatalf("unexpected owner additional-info parts after runtime equip then transfer: %+v", peerInfo.Parts)
	}
	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, peerEntry[2]))
	if err != nil {
		t.Fatalf("decode owner update for watcher after transfer: %v", err)
	}
	if peerUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 202} {
		t.Fatalf("unexpected owner update parts after runtime equip then transfer: %+v", peerUpdate.Parts)
	}
}

func TestGameRuntimeTransferCharacterQueuesPeerAppearanceAfterRuntimeUnequip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	owner.Equipment = []inventory.ItemInstance{{ID: 5002, Vnum: 11200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	watcher := peerVisibilityCharacter("PeerThree", 0x01030103, 0x02040103, 1500, 2500, 1, 103, 203)
	watcher.MapIndex = 42
	issuePeerTicket(t, store, "peer-two", 0x22222222, owner)
	issuePeerTicket(t, store, "peer-three", 0x33333333, watcher)
	for _, account := range []accountstore.Account{
		{Login: "peer-two", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
		{Login: "peer-three", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(ownerEnter) != 6 {
		t.Fatalf("expected 6 bootstrap frames for owner with one equipped item before transfer, got %d", len(ownerEnter))
	}
	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "peer-three", 0x33333333)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher on another map before transfer, got %d", len(watcherEnter))
	}
	if queued := flushServerFrames(t, flowOwner); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames across maps before transfer, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected no initial queued peer-entry frames for watcher across maps before transfer, got %d", len(queued))
	}

	unequipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error before transfer: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected 3 self unequip frames before transfer, got %d", len(unequipOut))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected no queued watcher frames while owner remains on another map after unequip, got %d", len(queued))
	}

	result, ok := runtime.TransferCharacter("PeerTwo", 42, 1700, 2800)
	if !ok {
		t.Fatal("expected transfer to succeed after runtime unequip")
	}
	if !result.Applied || result.Target.MapIndex != 42 || result.Target.X != 1700 || result.Target.Y != 2800 {
		t.Fatalf("unexpected transfer result after runtime unequip: %+v", result)
	}

	originEntry := flushServerFrames(t, flowOwner)
	if len(originEntry) != 3 {
		t.Fatalf("expected 3 queued origin peer-entry frames for owner after transfer post-unequip, got %d", len(originEntry))
	}
	originAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, originEntry[0]))
	if err != nil {
		t.Fatalf("decode watcher add for owner after transfer post-unequip: %v", err)
	}
	if originAdd.VID != watcher.VID {
		t.Fatalf("unexpected watcher add for owner after transfer post-unequip: %+v", originAdd)
	}

	peerEntry := flushServerFrames(t, flowWatcher)
	if len(peerEntry) != 3 {
		t.Fatalf("expected 3 queued owner peer-entry frames for watcher after transfer post-unequip, got %d", len(peerEntry))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerEntry[0]))
	if err != nil {
		t.Fatalf("decode owner add for watcher after transfer post-unequip: %v", err)
	}
	if peerAdd.VID != owner.VID || peerAdd.X != 1700 || peerAdd.Y != 2800 {
		t.Fatalf("unexpected owner add for watcher after transfer post-unequip: %+v", peerAdd)
	}
	peerInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, peerEntry[1]))
	if err != nil {
		t.Fatalf("decode owner additional info for watcher after transfer post-unequip: %v", err)
	}
	if peerInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{102, 0, 0, 202} {
		t.Fatalf("unexpected owner additional-info parts after runtime unequip then transfer: %+v", peerInfo.Parts)
	}
	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, peerEntry[2]))
	if err != nil {
		t.Fatalf("decode owner update for watcher after transfer post-unequip: %v", err)
	}
	if peerUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{102, 0, 0, 202} {
		t.Fatalf("unexpected owner update parts after runtime unequip then transfer: %+v", peerUpdate.Parts)
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
	if len(result.CurrentVisibleStaticActors) != 1 || result.CurrentVisibleStaticActors[0].EntityID != blacksmith.EntityID || result.CurrentVisibleStaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected current visible static actors: %+v", result.CurrentVisibleStaticActors)
	}
	if len(result.TargetVisibleStaticActors) != 1 || result.TargetVisibleStaticActors[0].EntityID != guard.EntityID || result.TargetVisibleStaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected target visible static actors: %+v", result.TargetVisibleStaticActors)
	}
	if len(result.RemovedVisibleStaticActors) != 1 || result.RemovedVisibleStaticActors[0].EntityID != blacksmith.EntityID || result.RemovedVisibleStaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected removed visible static actors: %+v", result.RemovedVisibleStaticActors)
	}
	if len(result.AddedVisibleStaticActors) != 1 || result.AddedVisibleStaticActors[0].EntityID != guard.EntityID || result.AddedVisibleStaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected added visible static actors: %+v", result.AddedVisibleStaticActors)
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
	if len(preview.CurrentVisibleStaticActors) != 1 || preview.CurrentVisibleStaticActors[0].EntityID != blacksmith.EntityID || preview.CurrentVisibleStaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected current visible static actors: %+v", preview.CurrentVisibleStaticActors)
	}
	if len(preview.TargetVisibleStaticActors) != 1 || preview.TargetVisibleStaticActors[0].EntityID != guard.EntityID || preview.TargetVisibleStaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected target visible static actors: %+v", preview.TargetVisibleStaticActors)
	}
	if len(preview.RemovedVisibleStaticActors) != 1 || preview.RemovedVisibleStaticActors[0].EntityID != blacksmith.EntityID || preview.RemovedVisibleStaticActors[0].Name != "Blacksmith" {
		t.Fatalf("unexpected removed visible static actors: %+v", preview.RemovedVisibleStaticActors)
	}
	if len(preview.AddedVisibleStaticActors) != 1 || preview.AddedVisibleStaticActors[0].EntityID != guard.EntityID || preview.AddedVisibleStaticActors[0].Name != "VillageGuard" {
		t.Fatalf("unexpected added visible static actors: %+v", preview.AddedVisibleStaticActors)
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

func TestGameRuntimeReconnectQueuesPeerAppearanceAfterRuntimeEquip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	owner.Inventory = []inventory.ItemInstance{{ID: 6001, Vnum: 11500, Count: 1, Slot: 8}}
	issuePeerTicket(t, store, "peer-one", 0x11111111, watcher)
	issuePeerTicket(t, store, "peer-two", 0x22222222, owner)
	for _, account := range []accountstore.Account{
		{Login: "peer-one", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-two", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher before reconnect slice, got %d", len(watcherEnter))
	}
	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(ownerEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for owner with one carried item and one visible peer, got %d", len(ownerEnter))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 initial peer-entry frames for watcher, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwner); len(queued) != 0 {
		t.Fatalf("expected no initial queued frames for owner, got %d", len(queued))
	}

	equipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip error before reconnect: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected 3 self equip frames before reconnect, got %d", len(equipOut))
	}
	liveRefresh := flushServerFrames(t, flowWatcher)
	if len(liveRefresh) != 1 {
		t.Fatalf("expected 1 queued live appearance refresh for watcher before reconnect, got %d", len(liveRefresh))
	}
	liveUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, liveRefresh[0]))
	if err != nil {
		t.Fatalf("decode live refresh before reconnect: %v", err)
	}
	if liveUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 202} {
		t.Fatalf("unexpected live refresh parts before reconnect after equip: %+v", liveUpdate.Parts)
	}

	closeSessionFlow(t, flowOwner)
	peerExit := flushServerFrames(t, flowWatcher)
	if len(peerExit) != 1 {
		t.Fatalf("expected 1 queued peer-exit frame before reconnect, got %d", len(peerExit))
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, peerExit[0]))
	if err != nil {
		t.Fatalf("decode peer delete before reconnect: %v", err)
	}
	if peerDelete.VID != owner.VID {
		t.Fatalf("expected peer delete for owner VID %#08x, got %#08x", owner.VID, peerDelete.VID)
	}

	account, ok := loadOrCreateAccount(accounts, "peer-two")
	if !ok {
		t.Fatal("expected persisted account snapshot for reconnect after equip")
	}
	reconnectKey, ok := issueLoginTicket(store, account.Login, account.Empire, account.Characters, func() (uint32, error) {
		return 0x44444444, nil
	})
	if !ok {
		t.Fatal("expected fresh login ticket for reconnect after equip")
	}

	flowOwnerReconnect, reenterFrames := enterGameWithLoginTicket(t, factory, "peer-two", reconnectKey)
	if len(reenterFrames) != 9 {
		t.Fatalf("expected 9 bootstrap frames for reconnected owner after equip with existing visible peer, got %d", len(reenterFrames))
	}
	peerReentry := flushServerFrames(t, flowWatcher)
	if len(peerReentry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames after reconnect post-equip, got %d", len(peerReentry))
	}
	queuedAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerReentry[0]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry add after equip reconnect: %v", err)
	}
	if queuedAdd.VID != owner.VID {
		t.Fatalf("expected queued peer re-entry add for owner VID %#08x, got %#08x", owner.VID, queuedAdd.VID)
	}
	queuedInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, peerReentry[1]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry additional info after equip reconnect: %v", err)
	}
	if queuedInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 202} {
		t.Fatalf("unexpected queued peer re-entry additional-info parts after equip reconnect: %+v", queuedInfo.Parts)
	}
	queuedUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, peerReentry[2]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry update after equip reconnect: %v", err)
	}
	if queuedUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 202} {
		t.Fatalf("unexpected queued peer re-entry update parts after equip reconnect: %+v", queuedUpdate.Parts)
	}

	closeSessionFlow(t, flowOwnerReconnect)
	closeSessionFlow(t, flowWatcher)
}

func TestGameRuntimeReconnectQueuesPeerAppearanceAfterRuntimeUnequip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	owner.Equipment = []inventory.ItemInstance{{ID: 6002, Vnum: 11200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	issuePeerTicket(t, store, "peer-one", 0x11111111, watcher)
	issuePeerTicket(t, store, "peer-two", 0x22222222, owner)
	for _, account := range []accountstore.Account{
		{Login: "peer-one", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-two", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher before reconnect slice, got %d", len(watcherEnter))
	}
	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-two", 0x22222222)
	if len(ownerEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for owner with one equipped item and one visible peer, got %d", len(ownerEnter))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 initial peer-entry frames for watcher, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwner); len(queued) != 0 {
		t.Fatalf("expected no initial queued frames for owner, got %d", len(queued))
	}

	unequipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error before reconnect: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected 3 self unequip frames before reconnect, got %d", len(unequipOut))
	}
	liveRefresh := flushServerFrames(t, flowWatcher)
	if len(liveRefresh) != 1 {
		t.Fatalf("expected 1 queued live appearance refresh for watcher before reconnect, got %d", len(liveRefresh))
	}
	liveUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, liveRefresh[0]))
	if err != nil {
		t.Fatalf("decode live refresh before reconnect after unequip: %v", err)
	}
	if liveUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{102, 0, 0, 202} {
		t.Fatalf("unexpected live refresh parts before reconnect after unequip: %+v", liveUpdate.Parts)
	}

	closeSessionFlow(t, flowOwner)
	peerExit := flushServerFrames(t, flowWatcher)
	if len(peerExit) != 1 {
		t.Fatalf("expected 1 queued peer-exit frame before reconnect, got %d", len(peerExit))
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, peerExit[0]))
	if err != nil {
		t.Fatalf("decode peer delete before reconnect after unequip: %v", err)
	}
	if peerDelete.VID != owner.VID {
		t.Fatalf("expected peer delete for owner VID %#08x after unequip, got %#08x", owner.VID, peerDelete.VID)
	}

	account, ok := loadOrCreateAccount(accounts, "peer-two")
	if !ok {
		t.Fatal("expected persisted account snapshot for reconnect after unequip")
	}
	reconnectKey, ok := issueLoginTicket(store, account.Login, account.Empire, account.Characters, func() (uint32, error) {
		return 0x55555555, nil
	})
	if !ok {
		t.Fatal("expected fresh login ticket for reconnect after unequip")
	}

	flowOwnerReconnect, reenterFrames := enterGameWithLoginTicket(t, factory, "peer-two", reconnectKey)
	if len(reenterFrames) != 9 {
		t.Fatalf("expected 9 bootstrap frames for reconnected owner after unequip with existing visible peer, got %d", len(reenterFrames))
	}
	peerReentry := flushServerFrames(t, flowWatcher)
	if len(peerReentry) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames after reconnect post-unequip, got %d", len(peerReentry))
	}
	queuedAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerReentry[0]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry add after unequip reconnect: %v", err)
	}
	if queuedAdd.VID != owner.VID {
		t.Fatalf("expected queued peer re-entry add for owner VID %#08x after unequip, got %#08x", owner.VID, queuedAdd.VID)
	}
	queuedInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, peerReentry[1]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry additional info after unequip reconnect: %v", err)
	}
	if queuedInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{102, 0, 0, 202} {
		t.Fatalf("unexpected queued peer re-entry additional-info parts after unequip reconnect: %+v", queuedInfo.Parts)
	}
	queuedUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, peerReentry[2]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry update after unequip reconnect: %v", err)
	}
	if queuedUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{102, 0, 0, 202} {
		t.Fatalf("unexpected queued peer re-entry update parts after unequip reconnect: %+v", queuedUpdate.Parts)
	}

	closeSessionFlow(t, flowOwnerReconnect)
	closeSessionFlow(t, flowWatcher)
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

func TestGameRuntimeRetryEnterGameQueuesPeerAppearanceAfterRuntimeEquip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 7001, Vnum: 11500, Count: 1, Slot: 8}}
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for owner with one carried item and one visible peer, got %d", len(ownerEnter))
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

	equipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 body"})))
	if err != nil {
		t.Fatalf("unexpected equip error before retry join: %v", err)
	}
	if len(equipOut) != 3 {
		t.Fatalf("expected 3 self equip frames before retry join, got %d", len(equipOut))
	}
	liveRefresh := flushServerFrames(t, flowWatcher)
	if len(liveRefresh) != 1 {
		t.Fatalf("expected 1 queued live appearance refresh for watcher before retry join, got %d", len(liveRefresh))
	}
	liveUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, liveRefresh[0]))
	if err != nil {
		t.Fatalf("decode live refresh before retry join: %v", err)
	}
	if liveUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 201} {
		t.Fatalf("unexpected live refresh parts before retry join after equip: %+v", liveUpdate.Parts)
	}

	closeSessionFlow(t, flowOwner)
	watcherExit := flushServerFrames(t, flowWatcher)
	if len(watcherExit) != 1 {
		t.Fatalf("expected 1 queued peer-exit frame for watcher after owner close, got %d", len(watcherExit))
	}
	removedOwner, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherExit[0]))
	if err != nil {
		t.Fatalf("decode queued owner delete before retry join: %v", err)
	}
	if removedOwner.VID != owner.VID {
		t.Fatalf("expected queued owner delete for VID %#08x, got %#08x", owner.VID, removedOwner.VID)
	}

	retryEnter, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame retry error after owner close: %v", err)
	}
	if len(retryEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames on retry after owner equip+close, got %d", len(retryEnter))
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
		t.Fatalf("decode queued peer re-entry add after retry equip: %v", err)
	}
	if queuedAdd.VID != owner.VID {
		t.Fatalf("expected queued peer re-entry add for VID %#08x, got %#08x", owner.VID, queuedAdd.VID)
	}
	queuedInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, watcherReentry[1]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry additional info after retry equip: %v", err)
	}
	if queuedInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 201} {
		t.Fatalf("unexpected queued peer re-entry additional-info parts after retry equip: %+v", queuedInfo.Parts)
	}
	queuedUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, watcherReentry[2]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry update after retry equip: %v", err)
	}
	if queuedUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{11500, 0, 0, 201} {
		t.Fatalf("unexpected queued peer re-entry update parts after retry equip: %+v", queuedUpdate.Parts)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowRetry)
}

func TestGameRuntimeRetryEnterGameQueuesPeerAppearanceAfterRuntimeUnequip(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Equipment = []inventory.ItemInstance{{ID: 7002, Vnum: 11200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwner, ownerEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for owner with one equipped item and one visible peer, got %d", len(ownerEnter))
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

	unequipOut, err := flowOwner.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected unequip error before retry join: %v", err)
	}
	if len(unequipOut) != 3 {
		t.Fatalf("expected 3 self unequip frames before retry join, got %d", len(unequipOut))
	}
	liveRefresh := flushServerFrames(t, flowWatcher)
	if len(liveRefresh) != 1 {
		t.Fatalf("expected 1 queued live appearance refresh for watcher before retry join, got %d", len(liveRefresh))
	}
	liveUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, liveRefresh[0]))
	if err != nil {
		t.Fatalf("decode live refresh before retry join after unequip: %v", err)
	}
	if liveUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected live refresh parts before retry join after unequip: %+v", liveUpdate.Parts)
	}

	closeSessionFlow(t, flowOwner)
	watcherExit := flushServerFrames(t, flowWatcher)
	if len(watcherExit) != 1 {
		t.Fatalf("expected 1 queued peer-exit frame for watcher after owner close, got %d", len(watcherExit))
	}
	removedOwner, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherExit[0]))
	if err != nil {
		t.Fatalf("decode queued owner delete before retry join after unequip: %v", err)
	}
	if removedOwner.VID != owner.VID {
		t.Fatalf("expected queued owner delete for VID %#08x after unequip, got %#08x", owner.VID, removedOwner.VID)
	}

	retryEnter, err := flowRetry.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame retry error after owner close: %v", err)
	}
	if len(retryEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames on retry after owner unequip+close, got %d", len(retryEnter))
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
		t.Fatalf("decode queued peer re-entry add after retry unequip: %v", err)
	}
	if queuedAdd.VID != owner.VID {
		t.Fatalf("expected queued peer re-entry add for VID %#08x after unequip, got %#08x", owner.VID, queuedAdd.VID)
	}
	queuedInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, watcherReentry[1]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry additional info after retry unequip: %v", err)
	}
	if queuedInfo.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected queued peer re-entry additional-info parts after retry unequip: %+v", queuedInfo.Parts)
	}
	queuedUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, watcherReentry[2]))
	if err != nil {
		t.Fatalf("decode queued peer re-entry update after retry unequip: %v", err)
	}
	if queuedUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected queued peer re-entry update parts after retry unequip: %+v", queuedUpdate.Parts)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowRetry)
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

func TestGameRuntimeEnterGameReclaimPreventsStaleSessionMoveFanout(t *testing.T) {
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
	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerOldEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for original owner with watcher already visible, got %d", len(ownerOldEnter))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after original owner join, got %d", len(queued))
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected live player entity for PeerOne before simulating stale ownership")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale session-directory entry removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerNewEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for replacement owner after reclaim, got %d", len(ownerNewEnter))
	}
	_ = flushServerFrames(t, flowWatcher)
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to start with no queued frames, got %d", len(queued))
	}

	beforeSnapshots := runtime.ConnectedCharacters()
	if len(beforeSnapshots) != 2 {
		t.Fatalf("expected watcher and replacement owner connected before stale move, got %+v", beforeSnapshots)
	}

	moveOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1400, Y: 2400, Time: 0x31323334})))
	if err != nil {
		t.Fatalf("unexpected stale owner move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected stale owner to receive exactly 1 self move ack frame, got %d", len(moveOut))
	}
	selfAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode stale owner self move ack: %v", err)
	}
	if selfAck.VID != peerOne.VID || selfAck.X != 1400 || selfAck.Y != 2400 || selfAck.Time != 0x31323334 {
		t.Fatalf("unexpected stale owner self move ack: %+v", selfAck)
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected watcher to receive no move replication from stale owner after reclaim, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale owner move, got %d", len(queued))
	}

	afterSnapshots := runtime.ConnectedCharacters()
	if len(afterSnapshots) != 2 {
		t.Fatalf("expected watcher and replacement owner to remain the only connected snapshots after stale move, got %+v", afterSnapshots)
	}
	for _, snapshot := range afterSnapshots {
		if snapshot.Name == "PeerOne" && (snapshot.X != 1100 || snapshot.Y != 2100) {
			t.Fatalf("expected replacement owner snapshot to stay unchanged after stale move, got %+v", snapshot)
		}
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func setupReclaimedOwnerRuntimeWithAccounts(t *testing.T) (accountstore.Store, service.SessionFactory, service.SessionFlow, service.SessionFlow, service.SessionFlow, loginticket.Character, loginticket.Character) {
	t.Helper()

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: peerOne.Empire, Characters: []loginticket.Character{peerOne}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerOldEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for original owner with watcher already visible, got %d", len(ownerOldEnter))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after original owner join, got %d", len(queued))
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected live player entity for PeerOne before simulating stale ownership")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale session-directory entry removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerNewEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for replacement owner after reclaim, got %d", len(ownerNewEnter))
	}
	_ = flushServerFrames(t, flowWatcher)
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to start with no queued frames, got %d", len(queued))
	}

	return accounts, factory, flowWatcher, flowOwnerOld, flowOwnerNew, watcher, peerOne
}

func TestGameRuntimeEnterGameReclaimPreventsStaleSessionMoveFromPersistingSnapshot(t *testing.T) {
	accounts, _, flowWatcher, flowOwnerOld, flowOwnerNew, _, peerOne := setupReclaimedOwnerRuntimeWithAccounts(t)

	moveOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1400, Y: 2400, Time: 0x31323334})))
	if err != nil {
		t.Fatalf("unexpected stale owner move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected stale owner to receive exactly 1 self move ack frame, got %d", len(moveOut))
	}
	selfAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode stale owner self move ack: %v", err)
	}
	if selfAck.VID != peerOne.VID || selfAck.X != 1400 || selfAck.Y != 2400 || selfAck.Time != 0x31323334 {
		t.Fatalf("unexpected stale owner self move ack: %+v", selfAck)
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected watcher to receive no move replication from stale owner after reclaim, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale owner move, got %d", len(queued))
	}
	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after stale move: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted character after stale move, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != peerOne.MapIndex || persisted.Characters[0].X != peerOne.X || persisted.Characters[0].Y != peerOne.Y {
		t.Fatalf("expected stale move to leave persisted snapshot unchanged, got %+v", persisted.Characters[0])
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeEnterGameReclaimPreventsStaleSessionSyncPositionFromPersistingSnapshot(t *testing.T) {
	accounts, _, flowWatcher, flowOwnerOld, flowOwnerNew, _, peerOne := setupReclaimedOwnerRuntimeWithAccounts(t)

	syncOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: peerOne.VID, X: 1600, Y: 2600}},
	})))
	if err != nil {
		t.Fatalf("unexpected stale owner sync_position error: %v", err)
	}
	if len(syncOut) != 1 {
		t.Fatalf("expected stale owner to receive exactly 1 self sync_position ack frame, got %d", len(syncOut))
	}
	selfAck, err := movep.DecodeSyncPositionAck(decodeSingleFrame(t, syncOut[0]))
	if err != nil {
		t.Fatalf("decode stale owner self sync_position ack: %v", err)
	}
	if len(selfAck.Elements) != 1 {
		t.Fatalf("expected 1 self sync_position ack element, got %d", len(selfAck.Elements))
	}
	if selfAck.Elements[0].VID != peerOne.VID || selfAck.Elements[0].X != 1600 || selfAck.Elements[0].Y != 2600 {
		t.Fatalf("unexpected stale owner self sync_position ack: %+v", selfAck)
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected watcher to receive no sync replication from stale owner after reclaim, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale owner sync_position, got %d", len(queued))
	}
	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after stale sync_position: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted character after stale sync_position, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != peerOne.MapIndex || persisted.Characters[0].X != peerOne.X || persisted.Characters[0].Y != peerOne.Y {
		t.Fatalf("expected stale sync_position to leave persisted snapshot unchanged, got %+v", persisted.Characters[0])
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeEnterGameReclaimKeepsStaleSessionWhisperSelfLocal(t *testing.T) {
	_, _, flowWatcher, flowOwnerOld, flowOwnerNew, watcher, _ := setupReclaimedOwnerRuntimeWithAccounts(t)

	whisperOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{Target: watcher.Name, Message: "hola fantasma"})))
	if err != nil {
		t.Fatalf("unexpected stale owner whisper error: %v", err)
	}
	if len(whisperOut) != 0 {
		t.Fatalf("expected stale owner whisper to stay self-local with no direct response, got %d frames", len(whisperOut))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected watcher to receive no whisper from stale owner after reclaim, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale owner whisper, got %d", len(queued))
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeEnterGameReclaimKeepsStaleEquipAppearanceMutationNonAuthoritative(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 700
	owner.Inventory = []inventory.ItemInstance{{ID: 8001, Vnum: 12200, Count: 1, Slot: 8}}
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerOldEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for original owner with carried item and watcher already visible, got %d", len(ownerOldEnter))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after original owner join, got %d", len(queued))
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected live player entity for PeerOne before simulating stale ownership")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale session-directory entry removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerNewEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for replacement owner after reclaim, got %d", len(ownerNewEnter))
	}
	_ = flushServerFrames(t, flowWatcher)
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to start with no queued frames, got %d", len(queued))
	}

	beforePersisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account before stale equip: %v", err)
	}
	beforeEquipment, ok := runtime.EquipmentSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner equipment snapshot before stale equip")
	}
	beforeInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner inventory snapshot before stale equip")
	}

	equipOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 weapon"})))
	if err != nil {
		t.Fatalf("unexpected stale owner equip error: %v", err)
	}
	if len(equipOut) != 4 {
		t.Fatalf("expected stale owner equip to remain self-local with 4 frames, got %d", len(equipOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, equipOut[2]))
	if err != nil {
		t.Fatalf("decode stale owner self equip point change: %v", err)
	}
	if pointPacket.VID != owner.VID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != 10 || pointPacket.Value != 710 {
		t.Fatalf("unexpected stale owner self equip point change: %+v", pointPacket)
	}
	selfUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, equipOut[3]))
	if err != nil {
		t.Fatalf("decode stale owner self equip update: %v", err)
	}
	if selfUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 12200, 0, 201} {
		t.Fatalf("unexpected stale owner self equip appearance: %+v", selfUpdate.Parts)
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected watcher to receive no appearance refresh from stale owner equip, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale owner equip, got %d", len(queued))
	}

	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after stale equip: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted character after stale equip, got %+v", persisted)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, beforePersisted.Characters[0].Inventory) || !reflect.DeepEqual(persisted.Characters[0].Equipment, beforePersisted.Characters[0].Equipment) {
		t.Fatalf("expected stale equip to leave persisted carried/equipped state unchanged, before inventory=%+v before equipment=%+v after inventory=%+v after equipment=%+v", beforePersisted.Characters[0].Inventory, beforePersisted.Characters[0].Equipment, persisted.Characters[0].Inventory, persisted.Characters[0].Equipment)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected stale equip to leave persisted points unchanged, before=%d after=%d", beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	afterEquipment, ok := runtime.EquipmentSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner equipment snapshot after stale equip")
	}
	afterInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner inventory snapshot after stale equip")
	}
	if !reflect.DeepEqual(afterEquipment, beforeEquipment) {
		t.Fatalf("expected stale equip to leave replacement owner equipment snapshot unchanged, before=%+v after=%+v", beforeEquipment, afterEquipment)
	}
	if !reflect.DeepEqual(afterInventory, beforeInventory) {
		t.Fatalf("expected stale equip to leave replacement owner inventory snapshot unchanged, before=%+v after=%+v", beforeInventory, afterInventory)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeEnterGameReclaimKeepsStaleUnequipAppearanceMutationNonAuthoritative(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 710
	owner.Equipment = []inventory.ItemInstance{{ID: 8002, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerOldEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for original owner with equipped item and watcher already visible, got %d", len(ownerOldEnter))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after original owner join, got %d", len(queued))
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected live player entity for PeerOne before simulating stale ownership")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale session-directory entry removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerNewEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for replacement owner after reclaim, got %d", len(ownerNewEnter))
	}
	_ = flushServerFrames(t, flowWatcher)
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to start with no queued frames, got %d", len(queued))
	}

	beforePersisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account before stale unequip: %v", err)
	}
	beforeEquipment, ok := runtime.EquipmentSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner equipment snapshot before stale unequip")
	}
	beforeInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner inventory snapshot before stale unequip")
	}

	unequipOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected stale owner unequip error: %v", err)
	}
	if len(unequipOut) != 4 {
		t.Fatalf("expected stale owner unequip to remain self-local with 4 frames, got %d", len(unequipOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, unequipOut[2]))
	if err != nil {
		t.Fatalf("decode stale owner self unequip point change: %v", err)
	}
	if pointPacket.VID != owner.VID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != -10 || pointPacket.Value != 700 {
		t.Fatalf("unexpected stale owner self unequip point change: %+v", pointPacket)
	}
	selfUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, unequipOut[3]))
	if err != nil {
		t.Fatalf("decode stale owner self unequip update: %v", err)
	}
	if selfUpdate.Parts != [worldproto.CharacterEquipmentPartCount]uint16{101, 0, 0, 201} {
		t.Fatalf("unexpected stale owner self unequip appearance: %+v", selfUpdate.Parts)
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected watcher to receive no appearance refresh from stale owner unequip, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale owner unequip, got %d", len(queued))
	}

	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after stale unequip: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted character after stale unequip, got %+v", persisted)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, beforePersisted.Characters[0].Inventory) || !reflect.DeepEqual(persisted.Characters[0].Equipment, beforePersisted.Characters[0].Equipment) {
		t.Fatalf("expected stale unequip to leave persisted carried/equipped state unchanged, before inventory=%+v before equipment=%+v after inventory=%+v after equipment=%+v", beforePersisted.Characters[0].Inventory, beforePersisted.Characters[0].Equipment, persisted.Characters[0].Inventory, persisted.Characters[0].Equipment)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected stale unequip to leave persisted points unchanged, before=%d after=%d", beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	afterEquipment, ok := runtime.EquipmentSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner equipment snapshot after stale unequip")
	}
	afterInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner inventory snapshot after stale unequip")
	}
	if !reflect.DeepEqual(afterEquipment, beforeEquipment) {
		t.Fatalf("expected stale unequip to leave replacement owner equipment snapshot unchanged, before=%+v after=%+v", beforeEquipment, afterEquipment)
	}
	if !reflect.DeepEqual(afterInventory, beforeInventory) {
		t.Fatalf("expected stale unequip to leave replacement owner inventory snapshot unchanged, before=%+v after=%+v", beforeInventory, afterInventory)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeEnterGameReclaimKeepsStaleItemUseMutationNonAuthoritative(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	watcher := peerVisibilityCharacter("Watcher", 0x01030100, 0x02040100, 1000, 2000, 0, 100, 200)
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 700
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	issuePeerTicket(t, store, "watcher", 0x10101010, watcher)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	for _, account := range []accountstore.Account{
		{Login: "watcher", Empire: watcher.Empire, Characters: []loginticket.Character{watcher}},
		{Login: "peer-one", Empire: owner.Empire, Characters: []loginticket.Character{owner}},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("save preloaded account %q: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowWatcher, watcherEnter := enterGameWithLoginTicket(t, factory, "watcher", 0x10101010)
	if len(watcherEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames for watcher, got %d", len(watcherEnter))
	}
	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerOldEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for original owner with consumable and watcher already visible, got %d", len(ownerOldEnter))
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-entry frames for watcher after original owner join, got %d", len(queued))
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName("PeerOne")
	if !ok {
		t.Fatal("expected live player entity for PeerOne before simulating stale ownership")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale session-directory entry removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerNewEnter) != 9 {
		t.Fatalf("expected 9 bootstrap frames for replacement owner after reclaim, got %d", len(ownerNewEnter))
	}
	_ = flushServerFrames(t, flowWatcher)
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to start with no queued frames, got %d", len(queued))
	}

	beforePersisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account before stale item use: %v", err)
	}
	beforeInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner inventory snapshot before stale item use")
	}

	useOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected stale owner item use error: %v", err)
	}
	if len(useOut) != 3 {
		t.Fatalf("expected stale owner item use to remain self-local with 3 frames, got %d", len(useOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, useOut[0]))
	if err != nil {
		t.Fatalf("decode stale owner item-use point change: %v", err)
	}
	if pointPacket.VID != owner.VID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != 50 || pointPacket.Value != 750 {
		t.Fatalf("unexpected stale owner item-use point change: %+v", pointPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, useOut[1]))
	if err != nil {
		t.Fatalf("decode stale owner item-use set: %v", err)
	}
	if setPacket.Position.WindowType != itemproto.WindowInventory || setPacket.Position.Cell != 5 || setPacket.Vnum != 27001 || setPacket.Count != 2 {
		t.Fatalf("unexpected stale owner item-use set packet: %+v", setPacket)
	}
	infoPacket, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, useOut[2]))
	if err != nil {
		t.Fatalf("decode stale owner item-use info: %v", err)
	}
	if infoPacket.Type != chatproto.ChatTypeInfo || infoPacket.VID != 0 || infoPacket.Message != "consume:27001:+50" {
		t.Fatalf("unexpected stale owner item-use info packet: %+v", infoPacket)
	}
	if queued := flushServerFrames(t, flowWatcher); len(queued) != 0 {
		t.Fatalf("expected watcher to receive no replication from stale owner item use, got %d", len(queued))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale owner item use, got %d", len(queued))
	}

	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after stale item use: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted character after stale item use, got %+v", persisted)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex] || !reflect.DeepEqual(persisted.Characters[0].Inventory, beforePersisted.Characters[0].Inventory) {
		t.Fatalf("expected stale item use to leave persisted points/inventory unchanged, before points=%d before inventory=%+v after points=%d after inventory=%+v", beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex], beforePersisted.Characters[0].Inventory, persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Inventory)
	}

	afterInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected replacement owner inventory snapshot after stale item use")
	}
	if !reflect.DeepEqual(afterInventory, beforeInventory) {
		t.Fatalf("expected stale item use to leave replacement owner inventory snapshot unchanged, before=%+v after=%+v", beforeInventory, afterInventory)
	}

	closeSessionFlow(t, flowWatcher)
	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeEnterGameReclaimKeepsStaleMerchantBuyMutationNonAuthoritative(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantBuyerStale", 0x01040112, 0x02050112, 125, nil)
	issuePeerTicket(t, store, "merchant-stale", 0x12121212, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-stale", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed stale merchant buyer account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected stale merchant runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}

	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-stale", 0x12121212)
	if len(ownerOldEnter) < 8 {
		t.Fatalf("expected stale merchant owner bootstrap to emit at least 8 frames, got %d", len(ownerOldEnter))
	}
	interactWithMerchantForBuy(t, flowOwnerOld, actor.EntityID)

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName(buyer.Name)
	if !ok {
		t.Fatal("expected live player entity for merchant buyer before reclaim")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale merchant session-directory entry removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-stale", 0x12121212)
	if len(ownerNewEnter) < 8 {
		t.Fatalf("expected replacement merchant owner bootstrap to emit at least 8 frames, got %d", len(ownerNewEnter))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement merchant owner to start with no queued frames, got %d", len(queued))
	}

	beforePersisted, err := accounts.Load("merchant-stale")
	if err != nil {
		t.Fatalf("load persisted account before stale merchant buy: %v", err)
	}
	beforeCurrency, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected replacement owner currency snapshot before stale merchant buy")
	}
	beforeInventory, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected replacement owner inventory snapshot before stale merchant buy")
	}

	buyOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected stale merchant packet buy error: %v", err)
	}
	if len(buyOut) != 2 {
		t.Fatalf("expected stale merchant packet buy to remain self-local with 2 frames, got %d", len(buyOut))
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode stale merchant buy item frame: %v", err)
	}
	if setPacket.Position != itemproto.InventoryPosition(0) || setPacket.Vnum != 27001 || setPacket.Count != 1 {
		t.Fatalf("unexpected stale merchant buy item frame: %+v", setPacket)
	}
	if err := shopproto.DecodeServerOK(decodeSingleFrame(t, buyOut[1])); err != nil {
		t.Fatalf("decode stale merchant buy ok frame: %v", err)
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement merchant owner to receive no queued frames from stale merchant buy, got %d", len(queued))
	}

	persisted, err := accounts.Load("merchant-stale")
	if err != nil {
		t.Fatalf("load persisted account after stale merchant buy: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted merchant buyer after stale buy, got %+v", persisted)
	}
	if persisted.Characters[0].Gold != beforePersisted.Characters[0].Gold || !reflect.DeepEqual(persisted.Characters[0].Inventory, beforePersisted.Characters[0].Inventory) {
		t.Fatalf("expected stale merchant buy to leave persisted gold/inventory unchanged, before gold=%d before inventory=%+v after gold=%d after inventory=%+v", beforePersisted.Characters[0].Gold, beforePersisted.Characters[0].Inventory, persisted.Characters[0].Gold, persisted.Characters[0].Inventory)
	}

	afterCurrency, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected replacement owner currency snapshot after stale merchant buy")
	}
	if afterCurrency != beforeCurrency {
		t.Fatalf("expected stale merchant buy to leave replacement owner currency snapshot unchanged, before=%+v after=%+v", beforeCurrency, afterCurrency)
	}
	afterInventory, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected replacement owner inventory snapshot after stale merchant buy")
	}
	if !reflect.DeepEqual(afterInventory, beforeInventory) {
		t.Fatalf("expected stale merchant buy to leave replacement owner inventory snapshot unchanged, before=%+v after=%+v", beforeInventory, afterInventory)
	}

	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeEnterGameReclaimKeepsStaleCombatAttackNonAuthoritative(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	if err := accounts.Save(accountstore.Account{Login: "peer-one", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed stale combat account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected stale combat runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20300, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected training dummy registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerOldEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for original owner with visible training dummy, got %d", len(ownerOldEnter))
	}
	targetOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected original owner target error before reclaim: %v", err)
	}
	if len(targetOut) != 1 {
		t.Fatalf("expected original owner target ack before reclaim, got %d frames", len(targetOut))
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName(owner.Name)
	if !ok {
		t.Fatal("expected live player entity before stale combat reclaim")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale combat session-directory removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerNewEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for replacement owner with visible training dummy, got %d", len(ownerNewEnter))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to start with no queued frames, got %d", len(queued))
	}

	staleAttackOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected stale combat attack error: %v", err)
	}
	if len(staleAttackOut) != 0 {
		t.Fatalf("expected stale combat attack to remain non-authoritative with no frames, got %d", len(staleAttackOut))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale combat attack, got %d", len(queued))
	}

	reselectOut, err := flowOwnerNew.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected replacement owner target error after stale combat attack: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected replacement owner target ack after stale combat attack, got %d frames", len(reselectOut))
	}
	targetPacket, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode replacement owner target ack after stale combat attack: %v", err)
	}
	if targetPacket.TargetVID != uint32(actor.EntityID) || targetPacket.HPPercent != 100 {
		t.Fatalf("unexpected replacement owner target ack after stale combat attack: %+v", targetPacket)
	}

	liveAttackOut, err := flowOwnerNew.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected replacement owner attack error after stale combat attack: %v", err)
	}
	if len(liveAttackOut) != 1 {
		t.Fatalf("expected replacement owner live attack to emit 1 frame, got %d", len(liveAttackOut))
	}
	attackPacket, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, liveAttackOut[0]))
	if err != nil {
		t.Fatalf("decode replacement owner attack refresh after stale combat attack: %v", err)
	}
	if attackPacket.TargetVID != uint32(actor.EntityID) || attackPacket.HPPercent != 90 {
		t.Fatalf("unexpected replacement owner attack refresh after stale combat attack: %+v", attackPacket)
	}

	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeEnterGameReclaimKeepsStaleCombatTargetSelectionNonAuthoritative(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	if err := accounts.Save(accountstore.Account{Login: "peer-one", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed stale combat account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected stale combat runtime error: %v", err)
	}
	firstDummy, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummyA", bootstrapMapIndex, 1200, 2200, 20300, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected first training dummy registration to succeed")
	}
	secondDummy, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummyB", bootstrapMapIndex, 1250, 2250, 20301, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected second training dummy registration to succeed")
	}
	factory := runtime.SessionFactory()

	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerOldEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for original owner with two visible training dummies, got %d", len(ownerOldEnter))
	}
	oldTargetOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(firstDummy.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected original owner target error before reclaim: %v", err)
	}
	if len(oldTargetOut) != 1 {
		t.Fatalf("expected original owner target ack before reclaim, got %d frames", len(oldTargetOut))
	}

	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName(owner.Name)
	if !ok {
		t.Fatal("expected live player entity before stale combat target reclaim")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale combat target session-directory removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(ownerNewEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for replacement owner with two visible training dummies, got %d", len(ownerNewEnter))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to start with no queued frames, got %d", len(queued))
	}

	liveTargetOut, err := flowOwnerNew.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(secondDummy.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected replacement owner target error before stale target attempt: %v", err)
	}
	if len(liveTargetOut) != 1 {
		t.Fatalf("expected replacement owner target ack before stale target attempt, got %d frames", len(liveTargetOut))
	}

	staleTargetOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(firstDummy.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected stale target error after reclaim: %v", err)
	}
	if len(staleTargetOut) != 0 {
		t.Fatalf("expected stale target attempt to remain non-authoritative with no frames, got %d", len(staleTargetOut))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale target attempt, got %d", len(queued))
	}

	liveAttackOut, err := flowOwnerNew.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(secondDummy.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected replacement owner attack error after stale target attempt: %v", err)
	}
	if len(liveAttackOut) != 1 {
		t.Fatalf("expected replacement owner attack after stale target attempt to emit 1 frame, got %d", len(liveAttackOut))
	}
	attackPacket, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, liveAttackOut[0]))
	if err != nil {
		t.Fatalf("decode replacement owner attack refresh after stale target attempt: %v", err)
	}
	if attackPacket.TargetVID != uint32(secondDummy.EntityID) || attackPacket.HPPercent != 90 {
		t.Fatalf("unexpected replacement owner attack refresh after stale target attempt: %+v", attackPacket)
	}

	closeSessionFlow(t, flowOwnerOld)
	closeSessionFlow(t, flowOwnerNew)
}

func TestGameRuntimeReconnectAfterStaleItemUseCloseRebuildsAuthoritativeState(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ReconnectAfterStaleUse", 0x01030121, 0x02040121, 1300, 2300, 2, 121, 221)
	owner.Points[bootstrapPlayerPointValueIndex] = 700
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	issuePeerTicket(t, store, "reconnect-stale-use", 0x55555555, owner)
	if err := accounts.Save(accountstore.Account{Login: "reconnect-stale-use", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed reconnect stale-use account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("unexpected reconnect stale-use runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	flowOwnerOld, ownerOldEnter := enterGameWithLoginTicket(t, factory, "reconnect-stale-use", 0x55555555)
	if len(ownerOldEnter) < 6 {
		t.Fatalf("expected original owner bootstrap to emit at least 6 frames, got %d", len(ownerOldEnter))
	}
	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName(owner.Name)
	if !ok {
		t.Fatal("expected live player entity before reclaim for reconnect stale-use test")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale-use session-directory entry removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, "reconnect-stale-use", 0x55555555)
	if len(ownerNewEnter) < 6 {
		t.Fatalf("expected replacement owner bootstrap to emit at least 6 frames, got %d", len(ownerNewEnter))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to start with no queued frames, got %d", len(queued))
	}

	useOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected stale item-use error before reconnect rebuild: %v", err)
	}
	if len(useOut) != 3 {
		t.Fatalf("expected stale item-use to remain self-local with 3 frames before reconnect rebuild, got %d", len(useOut))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement owner to receive no queued frames from stale item use before reconnect rebuild, got %d", len(queued))
	}

	closeSessionFlow(t, flowOwnerNew)
	closeSessionFlow(t, flowOwnerOld)

	persisted, err := accounts.Load("reconnect-stale-use")
	if err != nil {
		t.Fatalf("load persisted account after stale item-use closes: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted character after stale item-use closes, got %+v", persisted)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != 700 || !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected persisted authoritative state after stale item-use closes, got points=%d inventory=%+v", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Inventory)
	}

	account, ok := loadOrCreateAccount(accounts, "reconnect-stale-use")
	if !ok {
		t.Fatal("expected persisted account for reconnect after stale item use")
	}
	reconnectKey, ok := issueLoginTicket(store, account.Login, account.Empire, account.Characters, func() (uint32, error) {
		return 0x66666666, nil
	})
	if !ok {
		t.Fatal("expected fresh login ticket for reconnect after stale item use")
	}
	flowReconnect, reenterFrames := enterGameWithLoginTicket(t, factory, "reconnect-stale-use", reconnectKey)
	if len(reenterFrames) < 6 {
		t.Fatalf("expected reconnect bootstrap to emit at least 6 frames, got %d", len(reenterFrames))
	}

	var pointPacket *worldproto.PlayerPointChangePacket
	var itemPacket *itemproto.SetPacket
	for _, raw := range reenterFrames {
		decoded := decodeSingleFrame(t, raw)
		if pointPacket == nil {
			if pkt, err := worldproto.DecodePlayerPointChange(decoded); err == nil {
				copyPkt := pkt
				pointPacket = &copyPkt
			}
		}
		if itemPacket == nil {
			if pkt, err := itemproto.DecodeSet(decoded); err == nil && pkt.Position == itemproto.InventoryPosition(5) {
				copyPkt := pkt
				itemPacket = &copyPkt
			}
		}
	}
	if pointPacket == nil {
		t.Fatalf("expected reconnect bootstrap to include authoritative point snapshot, got %d frames", len(reenterFrames))
	}
	if pointPacket.VID != owner.VID || pointPacket.Type != bootstrapPlayerPointType || pointPacket.Amount != 700 || pointPacket.Value != 700 {
		t.Fatalf("unexpected reconnect point snapshot after stale item-use closes: %+v", *pointPacket)
	}
	if itemPacket == nil {
		t.Fatalf("expected reconnect bootstrap to include authoritative inventory slot 5 snapshot, got %d frames", len(reenterFrames))
	}
	if itemPacket.Vnum != 27001 || itemPacket.Count != 3 {
		t.Fatalf("unexpected reconnect inventory snapshot after stale item-use closes: %+v", *itemPacket)
	}

	closeSessionFlow(t, flowReconnect)
}

func TestGameRuntimeReconnectAfterStaleMerchantBuyCloseRebuildsAuthoritativeState(t *testing.T) {
	buyer := merchantBuyerCharacter("ReconnectAfterStaleMerchant", 0x01030122, 0x02040122, 125, nil)
	runtime, accounts, flowOwnerOld, actorID, login := setupMerchantBuySession(t, "reconnect-stale-merchant", 0x77777777, buyer)
	factory := runtime.SessionFactory()

	interactWithMerchantForBuy(t, flowOwnerOld, actorID)
	ownerEntity, ok := runtime.sharedWorld.entities.PlayerByName(buyer.Name)
	if !ok {
		t.Fatal("expected live merchant buyer entity before reclaim for reconnect stale-merchant test")
	}
	if _, ok := runtime.sharedWorld.sessionDirectory.Remove(ownerEntity.Entity.ID); !ok {
		t.Fatal("expected stale-merchant session-directory entry removal to succeed")
	}

	flowOwnerNew, ownerNewEnter := enterGameWithLoginTicket(t, factory, login, 0x77777777)
	if len(ownerNewEnter) < 8 {
		t.Fatalf("expected replacement merchant buyer bootstrap to emit at least 8 frames, got %d", len(ownerNewEnter))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement merchant buyer to start with no queued frames, got %d", len(queued))
	}

	buyOut, err := flowOwnerOld.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected stale merchant-buy error before reconnect rebuild: %v", err)
	}
	if len(buyOut) != 2 {
		t.Fatalf("expected stale merchant buy to remain self-local with 2 frames before reconnect rebuild, got %d", len(buyOut))
	}
	if queued := flushServerFrames(t, flowOwnerNew); len(queued) != 0 {
		t.Fatalf("expected replacement merchant buyer to receive no queued frames from stale merchant buy before reconnect rebuild, got %d", len(queued))
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected authoritative currency snapshot after stale merchant buy before reconnect rebuild")
	}
	if currencySnapshot.Gold != 125 {
		t.Fatalf("expected authoritative gold to remain 125 after stale merchant buy before reconnect rebuild, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected authoritative inventory snapshot after stale merchant buy before reconnect rebuild")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected authoritative inventory to stay empty after stale merchant buy before reconnect rebuild, got %+v", inventorySnapshot.Inventory)
	}

	closeSessionFlow(t, flowOwnerNew)
	closeSessionFlow(t, flowOwnerOld)

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted account after stale merchant-buy closes: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted character after stale merchant-buy closes, got %+v", persisted)
	}
	if persisted.Characters[0].Gold != 125 || len(persisted.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted authoritative merchant state after stale merchant-buy closes, got %+v", persisted.Characters[0])
	}

	flowReconnect, reenterFrames := enterGameWithLoginTicket(t, factory, login, 0x77777777)
	if len(reenterFrames) < 8 {
		t.Fatalf("expected reconnect merchant bootstrap to emit at least 8 frames, got %d", len(reenterFrames))
	}

	var foundItemSet bool
	for _, raw := range reenterFrames {
		decoded := decodeSingleFrame(t, raw)
		if _, err := itemproto.DecodeSet(decoded); err == nil {
			foundItemSet = true
		}
	}
	if foundItemSet {
		t.Fatalf("expected reconnect merchant bootstrap to include no carried item refresh after stale merchant-buy closes, got item frames in %d bootstrap frames", len(reenterFrames))
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected reconnect currency snapshot after stale merchant-buy closes")
	}
	if currencySnapshot.Gold != 125 {
		t.Fatalf("expected reconnect authoritative gold 125 after stale merchant-buy closes, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected reconnect inventory snapshot after stale merchant-buy closes")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected reconnect authoritative inventory to stay empty after stale merchant-buy closes, got %+v", inventorySnapshot.Inventory)
	}

	closeSessionFlow(t, flowReconnect)
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

func TestGameRuntimeSlashQuitReturnsClientCommandChat(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	character := peerVisibilityCharacter("QuitPlayer", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	if _, ok := issueLoginTicket(store, "quit-player", character.Empire, []loginticket.Character{character}, func() (uint32, error) {
		return 0x11111111, nil
	}); !ok {
		t.Fatal("expected login ticket issuance to succeed")
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quit-player", 0x11111111)
	if len(enterOut) != 5 {
		t.Fatalf("expected 5 enter-game frames before quit command, got %d", len(enterOut))
	}

	quitOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/quit"})))
	if err != nil {
		t.Fatalf("unexpected /quit error: %v", err)
	}
	if len(quitOut) != 1 {
		t.Fatalf("expected 1 outgoing quit command frame, got %d", len(quitOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, quitOut[0]))
	if err != nil {
		t.Fatalf("decode quit command chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeCommand || delivery.Message != "quit" {
		t.Fatalf("expected command chat 'quit', got %+v", delivery)
	}
	phaseAware, ok := flow.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected queued flow to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseGame {
		t.Fatalf("expected /quit to keep session in game until client disconnects, got %q", phaseAware.CurrentPhase())
	}
}

func TestGameRuntimeSlashLogoutTransitionsToClosePhase(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	character := peerVisibilityCharacter("LogoutPlayer", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	if _, ok := issueLoginTicket(store, "logout-player", character.Empire, []loginticket.Character{character}, func() (uint32, error) {
		return 0x11111111, nil
	}); !ok {
		t.Fatal("expected login ticket issuance to succeed")
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "logout-player", 0x11111111)
	if len(enterOut) != 5 {
		t.Fatalf("expected 5 enter-game frames before logout command, got %d", len(enterOut))
	}

	logoutOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/logout"})))
	if err != nil {
		t.Fatalf("unexpected /logout error: %v", err)
	}
	if len(logoutOut) != 1 {
		t.Fatalf("expected 1 outgoing close-phase frame, got %d", len(logoutOut))
	}
	phase, err := control.DecodePhase(decodeSingleFrame(t, logoutOut[0]))
	if err != nil {
		t.Fatalf("decode logout phase frame: %v", err)
	}
	if phase.Phase != session.PhaseClose {
		t.Fatalf("expected phase %q after /logout, got %q", session.PhaseClose, phase.Phase)
	}
	phaseAware, ok := flow.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected queued flow to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseClose {
		t.Fatalf("expected /logout to transition to close phase, got %q", phaseAware.CurrentPhase())
	}
	if snapshots := runtime.ConnectedCharacters(); len(snapshots) != 0 {
		t.Fatalf("expected /logout to leave shared world immediately, got %+v", snapshots)
	}
}

func TestGameRuntimeSlashPhaseSelectReturnsToSelectAndAllowsAnotherCharacterChoice(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	first := peerVisibilityCharacter("MkmkWar", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	second := peerVisibilityCharacter("MkmkSura", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	if _, ok := issueLoginTicket(store, "phase-select-player", first.Empire, []loginticket.Character{first, second}, func() (uint32, error) {
		return 0x11111111, nil
	}); !ok {
		t.Fatal("expected login ticket issuance to succeed")
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	flow := runtime.SessionFactory()()
	_ = mustCompleteSecureHandshake(t, flow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "phase-select-player", LoginKey: 0x11111111})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	loginOut, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if len(loginOut) != 3 {
		t.Fatalf("expected 3 login frames, got %d", len(loginOut))
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected initial character select error: %v", err)
	}
	enterOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected entergame error: %v", err)
	}
	if len(enterOut) != 5 {
		t.Fatalf("expected 5 enter-game frames before /phase_select, got %d", len(enterOut))
	}
	if snapshots := runtime.ConnectedCharacters(); len(snapshots) != 1 || snapshots[0].Name != first.Name {
		t.Fatalf("expected first character to own the shared world before /phase_select, got %+v", snapshots)
	}

	phaseSelectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/phase_select"})))
	if err != nil {
		t.Fatalf("unexpected /phase_select error: %v", err)
	}
	if len(phaseSelectOut) != 1 {
		t.Fatalf("expected 1 outgoing select-phase frame, got %d", len(phaseSelectOut))
	}
	phase, err := control.DecodePhase(decodeSingleFrame(t, phaseSelectOut[0]))
	if err != nil {
		t.Fatalf("decode phase-select frame: %v", err)
	}
	if phase.Phase != session.PhaseSelect {
		t.Fatalf("expected phase %q after /phase_select, got %q", session.PhaseSelect, phase.Phase)
	}
	phaseAware, ok := flow.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected queued flow to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseSelect {
		t.Fatalf("expected /phase_select to transition back to select phase, got %q", phaseAware.CurrentPhase())
	}
	if snapshots := runtime.ConnectedCharacters(); len(snapshots) != 0 {
		t.Fatalf("expected /phase_select to leave shared world before reselection, got %+v", snapshots)
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1})))
	if err != nil {
		t.Fatalf("unexpected second character select error after /phase_select: %v", err)
	}
	if len(selectOut) != 3 {
		t.Fatalf("expected 3 select frames after /phase_select, got %d", len(selectOut))
	}
	mainCharacter, err := worldproto.DecodeMainCharacter(decodeSingleFrame(t, selectOut[1]))
	if err != nil {
		t.Fatalf("decode second main character after /phase_select: %v", err)
	}
	if mainCharacter.Name != second.Name {
		t.Fatalf("expected second character %q after /phase_select, got %q", second.Name, mainCharacter.Name)
	}
}

func TestNewGameRuntimeBootLoadsPersistedStaticActorsBeforeEnterGame(t *testing.T) {
	loginStore := loginticket.NewFileStore(t.TempDir())
	player := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, loginStore, "peer-one", 0x11111111, player)
	staticPath := filepath.Join(t.TempDir(), "static-actors.json")
	staticActorStore := staticstore.NewFileStore(staticPath)
	if err := staticActorStore.Save(staticstore.Snapshot{StaticActors: []staticstore.StaticActor{{EntityID: 42, Name: "VillageGuard", MapIndex: bootstrapMapIndex, X: 1200, Y: 2200, RaceNum: 20300}}}); err != nil {
		t.Fatalf("save static actor snapshot: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStoreAndStaticStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginStore, nil, staticActorStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 loaded static actor snapshot, got %d", len(actors))
	}
	if actors[0].EntityID != 42 || actors[0].Name != "VillageGuard" || actors[0].MapIndex != bootstrapMapIndex || actors[0].X != 1200 || actors[0].Y != 2200 || actors[0].RaceNum != 20300 {
		t.Fatalf("unexpected loaded static actor snapshot: %+v", actors[0])
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with loaded persisted static actor, got %d", len(enterOut))
	}
	loadedAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, enterOut[5]))
	if err != nil {
		t.Fatalf("decode loaded static actor add: %v", err)
	}
	if loadedAdd.VID != 42 || loadedAdd.Type != 1 || loadedAdd.X != 1200 || loadedAdd.Y != 2200 || loadedAdd.RaceNum != 20300 {
		t.Fatalf("unexpected loaded static actor add: %+v", loadedAdd)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after enter bootstrap with loaded static actor, got %d", len(queued))
	}
}

func TestNewGameRuntimeRejectsMalformedPersistedStaticActorSnapshot(t *testing.T) {
	staticPath := filepath.Join(t.TempDir(), "static-actors.json")
	if err := os.WriteFile(staticPath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write malformed static actor snapshot: %v", err)
	}

	_, err := newGameRuntimeWithAccountStoreAndStaticStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticstore.NewFileStore(staticPath))
	if !errors.Is(err, staticstore.ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot on malformed persisted static actor snapshot, got %v", err)
	}
}

func TestNewGameRuntimeLoadsPersistedInteractionDefinitions(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	definition, ok := runtime.ResolveInteractionDefinition(interactionstore.KindInfo, "lore:alchemist")
	if !ok || definition.Kind != interactionstore.KindInfo || definition.Ref != "lore:alchemist" || definition.Text != "The alchemist studies forgotten herbs." {
		t.Fatalf("expected persisted interaction definition to resolve at runtime, got definition=%+v ok=%v", definition, ok)
	}
}

func TestNewGameRuntimeRejectsMalformedPersistedInteractionSnapshot(t *testing.T) {
	interactionPath := filepath.Join(t.TempDir(), "interaction-definitions.json")
	if err := os.WriteFile(interactionPath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write malformed interaction snapshot: %v", err)
	}

	_, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionstore.NewFileStore(interactionPath))
	if !errors.Is(err, interactionstore.ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot on malformed persisted interaction snapshot, got %v", err)
	}
}

func TestNewGameRuntimeRejectsPersistedStaticActorWithMissingInteractionDefinition(t *testing.T) {
	staticPath := filepath.Join(t.TempDir(), "static-actors.json")
	staticActorStore := staticstore.NewFileStore(staticPath)
	if err := staticActorStore.Save(staticstore.Snapshot{StaticActors: []staticstore.StaticActor{{EntityID: 42, Name: "VillageGuard", MapIndex: bootstrapMapIndex, X: 1200, Y: 2200, RaceNum: 20300, InteractionKind: "talk", InteractionRef: "npc:village_guard"}}}); err != nil {
		t.Fatalf("save persisted static actor snapshot: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, nil)

	_, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if !errors.Is(err, staticstore.ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for persisted static actor with missing interaction definition, got %v", err)
	}
}

func TestGameRuntimeRegisterStaticActorPersistsSnapshotOnSuccess(t *testing.T) {
	staticPath := filepath.Join(t.TempDir(), "static-actors.json")
	staticActorStore := staticstore.NewFileStore(staticPath)
	runtime, err := newGameRuntimeWithAccountStoreAndStaticStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	actor, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted static actor snapshot: %v", err)
	}
	want := staticstore.Snapshot{StaticActors: []staticstore.StaticActor{{EntityID: actor.EntityID, Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300}}}
	if !reflect.DeepEqual(persisted, want) {
		t.Fatalf("unexpected persisted static actor snapshot after register:\n got: %#v\nwant: %#v", persisted, want)
	}
}

func TestGameRuntimeRegisterStaticActorDoesNotMutateRuntimeWhenSnapshotPersistFails(t *testing.T) {
	runtime, err := newGameRuntimeWithAccountStoreAndStaticStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, &failingStaticActorStore{})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	if actor, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300); ok || actor != (StaticActorSnapshot{}) {
		t.Fatalf("expected static actor registration to fail closed on snapshot persist error, got actor=%+v ok=%v", actor, ok)
	}
	if actors := runtime.StaticActors(); len(actors) != 0 {
		t.Fatalf("expected no runtime static actors after failed snapshot persist, got %+v", actors)
	}
}

func TestGameRuntimeRegisterStaticActorWithInteractionUpdatesSnapshot(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	actor, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", 42, 1700, 2800, 20300, "talk", "npc:village_guard")
	if !ok {
		t.Fatal("expected static actor registration with interaction metadata to succeed")
	}
	if actor.InteractionKind != "talk" || actor.InteractionRef != "npc:village_guard" {
		t.Fatalf("expected interaction metadata in registered static actor snapshot, got %+v", actor)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 || actors[0].InteractionKind != "talk" || actors[0].InteractionRef != "npc:village_guard" {
		t.Fatalf("expected interaction metadata in runtime static actor snapshot, got %+v", actors)
	}
}

func TestGameRuntimeRegisterStaticActorWithInteractionPersistsSnapshotOnSuccess(t *testing.T) {
	staticPath := filepath.Join(t.TempDir(), "static-actors.json")
	staticActorStore := staticstore.NewFileStore(staticPath)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "VillageGuard : Keep your blade sharp."}})
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	actor, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", 42, 1700, 2800, 20300, "talk", "npc:village_guard")
	if !ok {
		t.Fatal("expected static actor registration with interaction metadata to succeed")
	}
	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted static actor snapshot: %v", err)
	}
	want := staticstore.Snapshot{StaticActors: []staticstore.StaticActor{{EntityID: actor.EntityID, Name: "VillageGuard", MapIndex: 42, X: 1700, Y: 2800, RaceNum: 20300, InteractionKind: "talk", InteractionRef: "npc:village_guard"}}}
	if !reflect.DeepEqual(persisted, want) {
		t.Fatalf("unexpected persisted static actor snapshot after interaction register:\n got: %#v\nwant: %#v", persisted, want)
	}
}

func TestGameRuntimeRegisterStaticActorWithInteractionRejectsUnknownInteractionDefinition(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}

	if actor, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", 42, 1700, 2800, 20300, "talk", "npc:village_guard"); ok || actor != (StaticActorSnapshot{}) {
		t.Fatalf("expected static actor interaction registration to fail for missing definition, got actor=%+v ok=%v", actor, ok)
	}
	if actors := runtime.StaticActors(); len(actors) != 0 {
		t.Fatalf("expected runtime static actors to remain empty after failed interaction registration, got %+v", actors)
	}
}

func TestGameRuntimeUpdateStaticActorWithInteractionRejectsUnknownInteractionDefinition(t *testing.T) {
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected base static actor registration to succeed")
	}

	if updated, ok := runtime.UpdateStaticActorWithInteraction(actor.EntityID, "VillageGuard", 42, 1700, 2800, 20300, "talk", "npc:village_guard"); ok || updated != (StaticActorSnapshot{}) {
		t.Fatalf("expected static actor interaction update to fail for missing definition, got actor=%+v ok=%v", updated, ok)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 || actors[0].InteractionKind != "" || actors[0].InteractionRef != "" {
		t.Fatalf("expected runtime static actor snapshot to remain without interaction metadata after failed update, got %+v", actors)
	}
}

func TestGameRuntimeUpdateStaticActorPersistsSnapshotOnSuccess(t *testing.T) {
	staticPath := filepath.Join(t.TempDir(), "static-actors.json")
	staticActorStore := staticstore.NewFileStore(staticPath)
	runtime, err := newGameRuntimeWithAccountStoreAndStaticStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("VillageGuard", 42, 1700, 2800, 20300)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}

	updated, ok := runtime.UpdateStaticActor(actor.EntityID, "Merchant", 43, 1800, 2900, 20301)
	if !ok {
		t.Fatal("expected static actor update to succeed")
	}
	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted static actor snapshot after update: %v", err)
	}
	want := staticstore.Snapshot{StaticActors: []staticstore.StaticActor{{EntityID: updated.EntityID, Name: "Merchant", MapIndex: 43, X: 1800, Y: 2900, RaceNum: 20301}}}
	if !reflect.DeepEqual(persisted, want) {
		t.Fatalf("unexpected persisted static actor snapshot after update:\n got: %#v\nwant: %#v", persisted, want)
	}
}

func TestGameRuntimeRemoveStaticActorPersistsSnapshotOnSuccess(t *testing.T) {
	staticPath := filepath.Join(t.TempDir(), "static-actors.json")
	staticActorStore := staticstore.NewFileStore(staticPath)
	runtime, err := newGameRuntimeWithAccountStoreAndStaticStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, loginticket.NewFileStore(t.TempDir()), nil, staticActorStore)
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
	persisted, err := staticActorStore.Load()
	if err != nil {
		t.Fatalf("load persisted static actor snapshot after remove: %v", err)
	}
	want := staticstore.Snapshot{StaticActors: []staticstore.StaticActor{{EntityID: blacksmith.EntityID, Name: "Blacksmith", MapIndex: 42, X: 1900, Y: 3000, RaceNum: 20301}}}
	if !reflect.DeepEqual(persisted, want) {
		t.Fatalf("unexpected persisted static actor snapshot after remove:\n got: %#v\nwant: %#v", persisted, want)
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

func TestGameRuntimeRegisterStaticActorQueuesVisibleBootstrapForOnlinePlayers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	nearPlayer := peerVisibilityCharacter("NearPlayer", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	farPlayer := peerVisibilityCharacter("FarPlayer", 0x01030102, 0x02040102, 2800, 3800, 2, 102, 202)
	issuePeerTicket(t, store, "near-player", 0x11111111, nearPlayer)
	issuePeerTicket(t, store, "far-player", 0x22222222, farPlayer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	factory := runtime.SessionFactory()

	nearFlow, _ := enterGameWithLoginTicket(t, factory, "near-player", 0x11111111)
	farFlow, _ := enterGameWithLoginTicket(t, factory, "far-player", 0x22222222)
	_ = flushServerFrames(t, nearFlow)
	_ = flushServerFrames(t, farFlow)

	actor, ok := runtime.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}

	nearQueued := flushServerFrames(t, nearFlow)
	if len(nearQueued) != 3 {
		t.Fatalf("expected 3 queued static actor bootstrap frames for nearby player, got %d", len(nearQueued))
	}
	actorAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, nearQueued[0]))
	if err != nil {
		t.Fatalf("decode queued static actor add: %v", err)
	}
	if actorAdd.VID != uint32(actor.EntityID) || actorAdd.Type != 1 || actorAdd.X != 1200 || actorAdd.Y != 2200 || actorAdd.RaceNum != 20300 {
		t.Fatalf("unexpected queued static actor add: %+v", actorAdd)
	}
	actorInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, nearQueued[1]))
	if err != nil {
		t.Fatalf("decode queued static actor additional info: %v", err)
	}
	if actorInfo.VID != uint32(actor.EntityID) || actorInfo.Name != "VillageGuard" {
		t.Fatalf("unexpected queued static actor additional info: %+v", actorInfo)
	}
	actorUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, nearQueued[2]))
	if err != nil {
		t.Fatalf("decode queued static actor update: %v", err)
	}
	if actorUpdate.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected queued static actor update: %+v", actorUpdate)
	}

	if farQueued := flushServerFrames(t, farFlow); len(farQueued) != 0 {
		t.Fatalf("expected no queued static actor bootstrap frames for far player, got %d", len(farQueued))
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

func TestGameRuntimeRemoveStaticActorQueuesDeleteForVisibleOnlinePlayers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	nearPlayer := peerVisibilityCharacter("NearPlayer", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	farPlayer := peerVisibilityCharacter("FarPlayer", 0x01030102, 0x02040102, 2800, 3800, 2, 102, 202)
	issuePeerTicket(t, store, "near-player", 0x11111111, nearPlayer)
	issuePeerTicket(t, store, "far-player", 0x22222222, farPlayer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	nearFlow, _ := enterGameWithLoginTicket(t, factory, "near-player", 0x11111111)
	farFlow, _ := enterGameWithLoginTicket(t, factory, "far-player", 0x22222222)
	_ = flushServerFrames(t, nearFlow)
	_ = flushServerFrames(t, farFlow)

	removed, ok := runtime.RemoveStaticActor(actor.EntityID)
	if !ok || removed.EntityID != actor.EntityID {
		t.Fatalf("expected static actor removal to return actor snapshot, got actor=%+v ok=%v", removed, ok)
	}

	nearQueued := flushServerFrames(t, nearFlow)
	if len(nearQueued) != 1 {
		t.Fatalf("expected 1 queued static actor delete for nearby player, got %d", len(nearQueued))
	}
	actorDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, nearQueued[0]))
	if err != nil {
		t.Fatalf("decode queued static actor delete: %v", err)
	}
	if actorDelete.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected queued static actor delete: %+v", actorDelete)
	}

	if farQueued := flushServerFrames(t, farFlow); len(farQueued) != 0 {
		t.Fatalf("expected no queued static actor delete for far player, got %d", len(farQueued))
	}
}

func TestGameRuntimeUpdateStaticActorRefreshesVisibleActorForOnlinePlayers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	nearPlayer := peerVisibilityCharacter("NearPlayer", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	farPlayer := peerVisibilityCharacter("FarPlayer", 0x01030102, 0x02040102, 2800, 3800, 2, 102, 202)
	issuePeerTicket(t, store, "near-player", 0x11111111, nearPlayer)
	issuePeerTicket(t, store, "far-player", 0x22222222, farPlayer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	nearFlow, _ := enterGameWithLoginTicket(t, factory, "near-player", 0x11111111)
	farFlow, _ := enterGameWithLoginTicket(t, factory, "far-player", 0x22222222)
	_ = flushServerFrames(t, nearFlow)
	_ = flushServerFrames(t, farFlow)

	updated, ok := runtime.UpdateStaticActor(actor.EntityID, "Merchant", bootstrapMapIndex, 1250, 2250, 20301)
	if !ok {
		t.Fatal("expected static actor update to succeed")
	}
	if updated.EntityID != actor.EntityID || updated.Name != "Merchant" || updated.MapIndex != bootstrapMapIndex || updated.X != 1250 || updated.Y != 2250 || updated.RaceNum != 20301 {
		t.Fatalf("unexpected updated actor snapshot: %+v", updated)
	}

	nearQueued := flushServerFrames(t, nearFlow)
	if len(nearQueued) != 4 {
		t.Fatalf("expected 4 queued static actor refresh frames for nearby player, got %d", len(nearQueued))
	}
	actorDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, nearQueued[0]))
	if err != nil {
		t.Fatalf("decode queued static actor delete during refresh: %v", err)
	}
	if actorDelete.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected queued static actor delete during refresh: %+v", actorDelete)
	}
	actorAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, nearQueued[1]))
	if err != nil {
		t.Fatalf("decode queued static actor add during refresh: %v", err)
	}
	if actorAdd.VID != uint32(actor.EntityID) || actorAdd.Type != 1 || actorAdd.X != 1250 || actorAdd.Y != 2250 || actorAdd.RaceNum != 20301 {
		t.Fatalf("unexpected queued static actor add during refresh: %+v", actorAdd)
	}
	actorInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, nearQueued[2]))
	if err != nil {
		t.Fatalf("decode queued static actor additional info during refresh: %v", err)
	}
	if actorInfo.VID != uint32(actor.EntityID) || actorInfo.Name != "Merchant" {
		t.Fatalf("unexpected queued static actor additional info during refresh: %+v", actorInfo)
	}
	actorUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, nearQueued[3]))
	if err != nil {
		t.Fatalf("decode queued static actor update during refresh: %v", err)
	}
	if actorUpdate.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected queued static actor update during refresh: %+v", actorUpdate)
	}

	if farQueued := flushServerFrames(t, farFlow); len(farQueued) != 0 {
		t.Fatalf("expected no queued static actor refresh frames for far player, got %d", len(farQueued))
	}
}

func TestGameRuntimeUpdateStaticActorRelocateAcrossAOIBoundaryQueuesVisibilityDeltasForOnlinePlayers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	nearPlayer := peerVisibilityCharacter("NearPlayer", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	farPlayer := peerVisibilityCharacter("FarPlayer", 0x01030102, 0x02040102, 1900, 3000, 2, 102, 202)
	issuePeerTicket(t, store, "near-player", 0x11111111, nearPlayer)
	issuePeerTicket(t, store, "far-player", 0x22222222, farPlayer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	nearFlow, _ := enterGameWithLoginTicket(t, factory, "near-player", 0x11111111)
	farFlow, _ := enterGameWithLoginTicket(t, factory, "far-player", 0x22222222)
	_ = flushServerFrames(t, nearFlow)
	_ = flushServerFrames(t, farFlow)

	updated, ok := runtime.UpdateStaticActor(actor.EntityID, "VillageGuard", bootstrapMapIndex, 1900, 3000, 20300)
	if !ok {
		t.Fatal("expected static actor update to succeed")
	}
	if updated.EntityID != actor.EntityID || updated.X != 1900 || updated.Y != 3000 {
		t.Fatalf("unexpected updated actor snapshot: %+v", updated)
	}

	nearQueued := flushServerFrames(t, nearFlow)
	if len(nearQueued) != 1 {
		t.Fatalf("expected 1 queued static actor delete for player leaving AOI visibility, got %d", len(nearQueued))
	}
	actorDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, nearQueued[0]))
	if err != nil {
		t.Fatalf("decode queued static actor delete across AOI update: %v", err)
	}
	if actorDelete.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected queued static actor delete across AOI update: %+v", actorDelete)
	}

	farQueued := flushServerFrames(t, farFlow)
	if len(farQueued) != 3 {
		t.Fatalf("expected 3 queued static actor bootstrap frames for player entering AOI visibility, got %d", len(farQueued))
	}
	actorAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, farQueued[0]))
	if err != nil {
		t.Fatalf("decode queued static actor add across AOI update: %v", err)
	}
	if actorAdd.VID != uint32(actor.EntityID) || actorAdd.X != 1900 || actorAdd.Y != 3000 || actorAdd.RaceNum != 20300 {
		t.Fatalf("unexpected queued static actor add across AOI update: %+v", actorAdd)
	}
	actorInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, farQueued[1]))
	if err != nil {
		t.Fatalf("decode queued static actor additional info across AOI update: %v", err)
	}
	if actorInfo.VID != uint32(actor.EntityID) || actorInfo.Name != "VillageGuard" {
		t.Fatalf("unexpected queued static actor additional info across AOI update: %+v", actorInfo)
	}
	actorUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, farQueued[2]))
	if err != nil {
		t.Fatalf("decode queued static actor update across AOI update: %v", err)
	}
	if actorUpdate.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected queued static actor update across AOI update: %+v", actorUpdate)
	}
}

func TestGameRuntimeUpdateStaticActorRelocateAcrossMapBoundaryQueuesVisibilityDeltasForOnlinePlayers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	originPlayer := peerVisibilityCharacter("OriginPlayer", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	destinationPlayer := peerVisibilityCharacter("DestinationPlayer", 0x01030102, 0x02040102, 1700, 2800, 2, 102, 202)
	destinationPlayer.MapIndex = 42
	issuePeerTicket(t, store, "origin-player", 0x11111111, originPlayer)
	issuePeerTicket(t, store, "destination-player", 0x22222222, destinationPlayer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300)
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	factory := runtime.SessionFactory()

	originFlow, _ := enterGameWithLoginTicket(t, factory, "origin-player", 0x11111111)
	destinationFlow, _ := enterGameWithLoginTicket(t, factory, "destination-player", 0x22222222)
	_ = flushServerFrames(t, originFlow)
	_ = flushServerFrames(t, destinationFlow)

	updated, ok := runtime.UpdateStaticActor(actor.EntityID, "Merchant", 42, 1700, 2800, 20301)
	if !ok {
		t.Fatal("expected static actor update to succeed")
	}
	if updated.EntityID != actor.EntityID || updated.MapIndex != 42 || updated.X != 1700 || updated.Y != 2800 || updated.Name != "Merchant" || updated.RaceNum != 20301 {
		t.Fatalf("unexpected updated actor snapshot: %+v", updated)
	}

	originQueued := flushServerFrames(t, originFlow)
	if len(originQueued) != 1 {
		t.Fatalf("expected 1 queued static actor delete for player leaving map visibility, got %d", len(originQueued))
	}
	actorDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, originQueued[0]))
	if err != nil {
		t.Fatalf("decode queued static actor delete across map update: %v", err)
	}
	if actorDelete.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected queued static actor delete across map update: %+v", actorDelete)
	}

	destinationQueued := flushServerFrames(t, destinationFlow)
	if len(destinationQueued) != 3 {
		t.Fatalf("expected 3 queued static actor bootstrap frames for player entering map visibility, got %d", len(destinationQueued))
	}
	actorAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, destinationQueued[0]))
	if err != nil {
		t.Fatalf("decode queued static actor add across map update: %v", err)
	}
	if actorAdd.VID != uint32(actor.EntityID) || actorAdd.X != 1700 || actorAdd.Y != 2800 || actorAdd.RaceNum != 20301 {
		t.Fatalf("unexpected queued static actor add across map update: %+v", actorAdd)
	}
	actorInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, destinationQueued[1]))
	if err != nil {
		t.Fatalf("decode queued static actor additional info across map update: %v", err)
	}
	if actorInfo.VID != uint32(actor.EntityID) || actorInfo.Name != "Merchant" {
		t.Fatalf("unexpected queued static actor additional info across map update: %+v", actorInfo)
	}
	actorUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, destinationQueued[2]))
	if err != nil {
		t.Fatalf("decode queued static actor update across map update: %v", err)
	}
	if actorUpdate.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected queued static actor update across map update: %+v", actorUpdate)
	}
}

func TestGameRuntimeUpdateStaticActorUpdatesSnapshot(t *testing.T) {
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

	updated, ok := runtime.UpdateStaticActor(guard.EntityID, "Merchant", 99, 900, 1200, 9001)
	if !ok {
		t.Fatal("expected static actor update to succeed")
	}
	if updated.EntityID != guard.EntityID || updated.Name != "Merchant" || updated.MapIndex != 99 || updated.X != 900 || updated.Y != 1200 || updated.RaceNum != 9001 {
		t.Fatalf("unexpected updated actor snapshot: %+v", updated)
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 static actors after update, got %d", len(actors))
	}
	if actors[0].EntityID != blacksmith.EntityID || actors[0].Name != "Blacksmith" {
		t.Fatalf("expected Blacksmith to remain first in sorted snapshot, got %+v", actors[0])
	}
	if actors[1].EntityID != guard.EntityID || actors[1].Name != "Merchant" || actors[1].MapIndex != 99 || actors[1].RaceNum != 9001 {
		t.Fatalf("expected Merchant update in runtime snapshot, got %+v", actors[1])
	}
	maps := runtime.MapOccupancy()
	if len(maps) != 2 {
		t.Fatalf("expected 2 map occupancy snapshots after static actor move, got %+v", maps)
	}
	if maps[0].MapIndex != 42 || len(maps[0].StaticActors) != 1 || maps[0].StaticActors[0].EntityID != blacksmith.EntityID {
		t.Fatalf("expected only Blacksmith on old map after update, got %+v", maps[0])
	}
	if maps[1].MapIndex != 99 || len(maps[1].StaticActors) != 1 || maps[1].StaticActors[0].EntityID != guard.EntityID || maps[1].StaticActors[0].Name != "Merchant" {
		t.Fatalf("expected Merchant on new map after update, got %+v", maps[1])
	}
}

func TestSharedWorldRegistryAttemptStaticActorInteractionResolvesVisibleActorWithMetadata(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActorWithInteraction(0, "VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, "talk", "npc:village_guard")
	if !ok {
		t.Fatal("expected visible interactable static actor registration to succeed")
	}

	attempt := registry.AttemptStaticActorInteraction(subjectID, uint32(actor.EntityID))
	if !attempt.Accepted {
		t.Fatalf("expected visible interactable static actor attempt to be accepted, got %+v", attempt)
	}
	if attempt.Failure != "" {
		t.Fatalf("expected accepted interaction attempt to have no failure reason, got %+v", attempt)
	}
	if attempt.TargetVID != uint32(actor.EntityID) {
		t.Fatalf("expected interaction attempt target VID %#08x, got %#08x", uint32(actor.EntityID), attempt.TargetVID)
	}
	if attempt.Actor.EntityID != actor.EntityID || attempt.Actor.Name != "VillageGuard" || attempt.Actor.InteractionKind != "talk" || attempt.Actor.InteractionRef != "npc:village_guard" {
		t.Fatalf("unexpected resolved static actor interaction attempt: %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorInteractionRejectsInvisibleTarget(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActorWithInteraction(0, "VillageGuard", bootstrapMapIndex, 2800, 3800, 20300, "talk", "npc:village_guard")
	if !ok {
		t.Fatal("expected far interactable static actor registration to succeed")
	}

	attempt := registry.AttemptStaticActorInteraction(subjectID, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected invisible static actor interaction attempt to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorInteractionFailureTargetNotVisible {
		t.Fatalf("expected invisible static actor failure %q, got %+v", StaticActorInteractionFailureTargetNotVisible, attempt)
	}
	if attempt.Actor != (StaticActorSnapshot{}) {
		t.Fatalf("expected invisible static actor interaction attempt to keep actor snapshot empty, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorInteractionRejectsVisibleActorOutsideInteractionRange(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActorWithInteraction(0, "VillageGuard", bootstrapMapIndex, 2600, 3600, 20300, "talk", "npc:village_guard")
	if !ok {
		t.Fatal("expected visible but far interactable static actor registration to succeed")
	}

	attempt := registry.AttemptStaticActorInteraction(subjectID, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected out-of-range static actor interaction attempt to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorInteractionFailureTargetOutOfRange {
		t.Fatalf("expected out-of-range static actor failure %q, got %+v", StaticActorInteractionFailureTargetOutOfRange, attempt)
	}
	if attempt.Actor.EntityID != actor.EntityID || attempt.Actor.Name != "VillageGuard" || attempt.Actor.InteractionKind != "talk" || attempt.Actor.InteractionRef != "npc:village_guard" {
		t.Fatalf("expected out-of-range interaction attempt to preserve the resolved actor snapshot, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorInteractionRejectsActorWithoutMetadata(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300)
	if !ok {
		t.Fatal("expected visible static actor registration to succeed")
	}

	attempt := registry.AttemptStaticActorInteraction(subjectID, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected non-interactable static actor interaction attempt to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorInteractionFailureTargetHasNoInteraction {
		t.Fatalf("expected missing-metadata static actor failure %q, got %+v", StaticActorInteractionFailureTargetHasNoInteraction, attempt)
	}
	if attempt.Actor.EntityID != actor.EntityID || attempt.Actor.Name != "VillageGuard" || attempt.Actor.InteractionKind != "" || attempt.Actor.InteractionRef != "" {
		t.Fatalf("expected missing-metadata static actor interaction attempt to return the resolved actor snapshot, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorInteractionRejectsUnknownSubject(t *testing.T) {
	registry := newSharedWorldRegistry()
	actor, ok := registry.RegisterStaticActorWithInteraction(0, "VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, "talk", "npc:village_guard")
	if !ok {
		t.Fatal("expected interactable static actor registration to succeed")
	}

	attempt := registry.AttemptStaticActorInteraction(999, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected unknown-subject interaction attempt to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorInteractionFailureSubjectNotFound {
		t.Fatalf("expected unknown-subject failure %q, got %+v", StaticActorInteractionFailureSubjectNotFound, attempt)
	}
	if attempt.Actor != (StaticActorSnapshot{}) {
		t.Fatalf("expected unknown-subject interaction attempt to keep actor snapshot empty, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorCombatTargetResolvesVisibleTrainingDummy(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}

	attempt := registry.AttemptStaticActorCombatTarget(subjectID, uint32(actor.EntityID))
	if !attempt.Accepted {
		t.Fatalf("expected visible training dummy combat-target attempt to be accepted, got %+v", attempt)
	}
	if attempt.Failure != "" {
		t.Fatalf("expected accepted combat-target attempt to have no failure reason, got %+v", attempt)
	}
	if attempt.TargetVID != uint32(actor.EntityID) {
		t.Fatalf("expected combat-target attempt target VID %#08x, got %#08x", uint32(actor.EntityID), attempt.TargetVID)
	}
	if attempt.Actor.EntityID != actor.EntityID || attempt.Actor.Name != "TrainingDummy" {
		t.Fatalf("unexpected resolved training-dummy combat-target attempt: %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptSelectedStaticActorAttackAcceptsMatchingVisibleTrainingDummy(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	targetAttempt := registry.AttemptStaticActorCombatTarget(subjectID, uint32(actor.EntityID))
	if !targetAttempt.Accepted {
		t.Fatalf("expected target selection before selected attack to succeed, got %+v", targetAttempt)
	}

	attempt := registry.AttemptSelectedStaticActorAttack(subjectID, targetAttempt.TargetVID, targetAttempt.SnapshotVersion, uint32(actor.EntityID))
	if !attempt.Accepted {
		t.Fatalf("expected matching selected training-dummy attack attempt to be accepted, got %+v", attempt)
	}
	if attempt.Failure != "" {
		t.Fatalf("expected accepted selected-attack attempt to have no failure reason, got %+v", attempt)
	}
	if attempt.ActiveTargetVID != uint32(actor.EntityID) || attempt.RequestedTargetVID != uint32(actor.EntityID) {
		t.Fatalf("unexpected selected-attack target ownership: %+v", attempt)
	}
	if attempt.Actor.EntityID != actor.EntityID || attempt.Actor.Name != "TrainingDummy" {
		t.Fatalf("unexpected resolved training-dummy selected-attack attempt: %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptSelectedStaticActorAttackRejectsWithoutActiveTarget(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}

	attempt := registry.AttemptSelectedStaticActorAttack(subjectID, 0, 0, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected selected attack without active target to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorCombatAttackFailureNoActiveTarget {
		t.Fatalf("expected no-active-target attack failure %q, got %+v", StaticActorCombatAttackFailureNoActiveTarget, attempt)
	}
	if attempt.Actor != (StaticActorSnapshot{}) {
		t.Fatalf("expected selected attack without active target to keep actor snapshot empty, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptSelectedStaticActorAttackRejectsMismatchedRequestedTarget(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	first, ok := registry.RegisterStaticActorWithCombatKind(0, "TrainingDummyOne", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected first visible training-dummy registration to succeed")
	}
	second, ok := registry.RegisterStaticActorWithCombatKind(0, "TrainingDummyTwo", bootstrapMapIndex, 1210, 2210, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected second visible training-dummy registration to succeed")
	}
	targetAttempt := registry.AttemptStaticActorCombatTarget(subjectID, uint32(first.EntityID))
	if !targetAttempt.Accepted {
		t.Fatalf("expected target selection before mismatched attack to succeed, got %+v", targetAttempt)
	}

	attempt := registry.AttemptSelectedStaticActorAttack(subjectID, targetAttempt.TargetVID, targetAttempt.SnapshotVersion, uint32(second.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected selected attack with mismatched request target to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorCombatAttackFailureTargetMismatch {
		t.Fatalf("expected target-mismatch attack failure %q, got %+v", StaticActorCombatAttackFailureTargetMismatch, attempt)
	}
	if attempt.Actor != (StaticActorSnapshot{}) {
		t.Fatalf("expected mismatched selected attack to keep actor snapshot empty, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorCombatTargetRejectsInvisibleTarget(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 2800, 3800, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected far training-dummy registration to succeed")
	}

	attempt := registry.AttemptStaticActorCombatTarget(subjectID, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected invisible training-dummy combat-target attempt to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorCombatTargetFailureTargetNotVisible {
		t.Fatalf("expected invisible training-dummy failure %q, got %+v", StaticActorCombatTargetFailureTargetNotVisible, attempt)
	}
	if attempt.Actor != (StaticActorSnapshot{}) {
		t.Fatalf("expected invisible training-dummy combat-target attempt to keep actor snapshot empty, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorCombatTargetRejectsVisibleActorOutsideCombatRange(t *testing.T) {
	registry := newSharedWorldRegistry()
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 2600, 3600, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible but far training-dummy registration to succeed")
	}

	attempt := registry.AttemptStaticActorCombatTarget(subjectID, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected out-of-range training-dummy combat-target attempt to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorCombatTargetFailureTargetOutOfRange {
		t.Fatalf("expected out-of-range training-dummy failure %q, got %+v", StaticActorCombatTargetFailureTargetOutOfRange, attempt)
	}
	if attempt.Actor.EntityID != actor.EntityID || attempt.Actor.Name != "TrainingDummy" {
		t.Fatalf("expected out-of-range training-dummy attempt to preserve the resolved actor snapshot, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorCombatTargetRejectsVisibleNonTargetableActor(t *testing.T) {
	topology := worldruntime.NewBootstrapTopology(1).WithRadiusVisibilityPolicy(400, 200)
	registry := newSharedWorldRegistryWithTopology(topology)
	subject := peerVisibilityCharacter("Subject", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	subjectID, _ := registry.Join(subject, newPendingServerFrames(), nil)
	if subjectID == 0 {
		t.Fatal("expected subject join to return a live shared-world entity ID")
	}
	actor, ok := registry.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300)
	if !ok {
		t.Fatal("expected visible non-targetable static actor registration to succeed")
	}

	attempt := registry.AttemptStaticActorCombatTarget(subjectID, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected visible non-targetable combat-target attempt to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorCombatTargetFailureTargetNotTargetable {
		t.Fatalf("expected non-targetable static actor failure %q, got %+v", StaticActorCombatTargetFailureTargetNotTargetable, attempt)
	}
	if attempt.Actor.EntityID != actor.EntityID || attempt.Actor.Name != "VillageGuard" {
		t.Fatalf("expected non-targetable combat-target attempt to preserve the resolved actor snapshot, got %+v", attempt)
	}
}

func TestSharedWorldRegistryAttemptStaticActorCombatTargetRejectsUnknownSubject(t *testing.T) {
	registry := newSharedWorldRegistry()
	actor, ok := registry.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected training-dummy registration to succeed")
	}

	attempt := registry.AttemptStaticActorCombatTarget(999, uint32(actor.EntityID))
	if attempt.Accepted {
		t.Fatalf("expected unknown-subject combat-target attempt to fail, got %+v", attempt)
	}
	if attempt.Failure != StaticActorCombatTargetFailureSubjectNotFound {
		t.Fatalf("expected unknown-subject failure %q, got %+v", StaticActorCombatTargetFailureSubjectNotFound, attempt)
	}
	if attempt.Actor != (StaticActorSnapshot{}) {
		t.Fatalf("expected unknown-subject combat-target attempt to keep actor snapshot empty, got %+v", attempt)
	}
}

func TestGameRuntimeResolveStaticActorInfoInteractionReturnsChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Alchemist", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindInfo, "lore:alchemist")
	if !ok {
		t.Fatal("expected info static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible interactable static actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	subject, ok := runtime.sharedWorld.entities.PlayerByName(peer.Name)
	if !ok {
		t.Fatalf("expected live shared-world entity for %q after enter", peer.Name)
	}
	resolution := runtime.resolveStaticActorInteraction(subject.Entity.ID, uint32(actor.EntityID))
	if !resolution.Accepted {
		t.Fatalf("expected info interaction resolution to be accepted, got %+v", resolution)
	}
	if resolution.Failure != "" {
		t.Fatalf("expected accepted info interaction to carry no failure, got %+v", resolution)
	}
	if resolution.Actor.EntityID != actor.EntityID || resolution.Definition.Kind != interactionstore.KindInfo || resolution.Definition.Ref != "lore:alchemist" {
		t.Fatalf("unexpected info interaction resolution payload: %+v", resolution)
	}
	if resolution.Delivery == nil {
		t.Fatalf("expected accepted info interaction to return a self chat delivery, got %+v", resolution)
	}
	if resolution.Delivery.Type != chatproto.ChatTypeInfo || resolution.Delivery.VID != 0 || resolution.Delivery.Empire != 0 || resolution.Delivery.Message != "The alchemist studies forgotten herbs." {
		t.Fatalf("unexpected info interaction delivery: %+v", resolution.Delivery)
	}
}

func TestGameRuntimeResolveStaticActorInfoInteractionRejectsMissingDefinition(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, nil)

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	if _, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "Alchemist", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindInfo, "lore:missing"); !ok {
		t.Fatal("expected direct shared-world registration with dangling ref to succeed for fail-closed runtime test")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	subject, ok := runtime.sharedWorld.entities.PlayerByName(peer.Name)
	if !ok {
		t.Fatalf("expected live shared-world entity for %q after enter", peer.Name)
	}
	resolution := runtime.resolveStaticActorInteraction(subject.Entity.ID, 1)
	if resolution.Accepted {
		t.Fatalf("expected dangling-definition info interaction to fail closed, got %+v", resolution)
	}
	if resolution.Failure != staticActorInteractionFailureDefinitionNotFound {
		t.Fatalf("expected dangling-definition failure %q, got %+v", staticActorInteractionFailureDefinitionNotFound, resolution)
	}
	if resolution.Delivery == nil {
		t.Fatalf("expected dangling-definition info interaction to return a self chat delivery, got %+v", resolution)
	}
	if resolution.Delivery.Type != chatproto.ChatTypeInfo || resolution.Delivery.VID != 0 || resolution.Delivery.Empire != 0 || resolution.Delivery.Message != "Interaction content is missing." {
		t.Fatalf("unexpected dangling-definition interaction delivery: %+v", resolution.Delivery)
	}
}

func TestGameSessionFlowStaticActorInfoInteractionReturnsSelfOnlyChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Alchemist", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindInfo, "lore:alchemist")
	if !ok {
		t.Fatal("expected info static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible interactable static actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected info interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only info interaction frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode info interaction chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "The alchemist studies forgotten herbs." {
		t.Fatalf("unexpected info interaction chat delivery: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for self-only info interaction, got %d", len(queued))
	}
}

func TestGameRuntimeResolveStaticActorTalkInteractionReturnsChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guard", Text: "Keep your blade sharp."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindTalk, "npc:guard")
	if !ok {
		t.Fatal("expected talk static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	subject, ok := runtime.sharedWorld.entities.PlayerByName(peer.Name)
	if !ok {
		t.Fatalf("expected live shared-world entity for %q after enter", peer.Name)
	}
	resolution := runtime.resolveStaticActorInteraction(subject.Entity.ID, uint32(actor.EntityID))
	if !resolution.Accepted {
		t.Fatalf("expected talk interaction resolution to be accepted, got %+v", resolution)
	}
	if resolution.Delivery == nil {
		t.Fatalf("expected accepted talk interaction to return a self chat delivery, got %+v", resolution)
	}
	if resolution.Delivery.Type != chatproto.ChatTypeInfo || resolution.Delivery.VID != 0 || resolution.Delivery.Empire != 0 || resolution.Delivery.Message != "VillageGuard:\nKeep your blade sharp." {
		t.Fatalf("unexpected talk interaction delivery: %+v", resolution.Delivery)
	}
}

func TestGameSessionFlowStaticActorTalkInteractionReturnsSelfOnlyChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guard", Text: "Keep your blade sharp."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindTalk, "npc:guard")
	if !ok {
		t.Fatal("expected talk static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible interactable static actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected talk interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only talk interaction frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode talk interaction chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "VillageGuard:\nKeep your blade sharp." {
		t.Fatalf("unexpected talk interaction chat delivery: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for self-only talk interaction, got %d", len(queued))
	}
}

func TestGameRuntimeResolveStaticActorShopPreviewInteractionReturnsChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected shop preview static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	subject, ok := runtime.sharedWorld.entities.PlayerByName(peer.Name)
	if !ok {
		t.Fatalf("expected live shared-world entity for %q after enter", peer.Name)
	}
	resolution := runtime.resolveStaticActorInteraction(subject.Entity.ID, uint32(actor.EntityID))
	if !resolution.Accepted {
		t.Fatalf("expected shop preview interaction resolution to be accepted, got %+v", resolution)
	}
	if resolution.Delivery == nil {
		t.Fatalf("expected accepted shop preview interaction to return a self chat delivery, got %+v", resolution)
	}
	if resolution.Delivery.Type != chatproto.ChatTypeInfo || resolution.Delivery.VID != 0 || resolution.Delivery.Empire != 0 || resolution.Delivery.Message != defaultMerchantPreview {
		t.Fatalf("unexpected shop preview interaction delivery: %+v", resolution.Delivery)
	}
}

func TestGameSessionFlowStaticActorShopPreviewInteractionOpensMerchantWindow(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected shop preview static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible interactable static actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected shop preview interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame, got %d", len(out))
	}
	start, err := shopproto.DecodeServerStart(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode merchant shop start: %v", err)
	}
	if start.OwnerVID != uint32(actor.EntityID) {
		t.Fatalf("expected merchant owner vid %d, got %d", actor.EntityID, start.OwnerVID)
	}
	if start.Items[0].Vnum != 27001 || start.Items[0].Price != 50 || start.Items[0].Count != 1 || start.Items[0].DisplayPos != 0 {
		t.Fatalf("unexpected merchant slot 0 entry: %+v", start.Items[0])
	}
	if start.Items[1].Vnum != 11200 || start.Items[1].Price != 500 || start.Items[1].Count != 1 || start.Items[1].DisplayPos != 1 {
		t.Fatalf("unexpected merchant slot 1 entry: %+v", start.Items[1])
	}
	if start.Items[2].Vnum != 27001 || start.Items[2].Price != 100 || start.Items[2].Count != 2 || start.Items[2].DisplayPos != 2 {
		t.Fatalf("unexpected merchant slot 2 entry: %+v", start.Items[2])
	}
	if start.Items[3] != (shopproto.ItemEntry{}) {
		t.Fatalf("expected trailing merchant shop entries to stay zeroed, got %+v", start.Items[3])
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for merchant shop open, got %d", len(queued))
	}
}

func TestGameSessionFlowShopEndClosesMerchantWindowContext(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerClose", 0x01040104, 0x02050104, 125, nil)
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "merchant-close", 0x44444444, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected shop end error: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected 1 merchant shop-end frame, got %d", len(closeOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, closeOut[0])); err != nil {
		t.Fatalf("decode merchant shop end: %v", err)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for merchant shop close, got %d", len(queued))
	}

	packetBuyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop-buy error after close: %v", err)
	}
	if len(packetBuyOut) != 0 {
		t.Fatalf("expected closed merchant context to reject packet buy frames, got %d", len(packetBuyOut))
	}

	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/shop_buy 0"})))
	if err != nil {
		t.Fatalf("unexpected slash shop-buy error after close: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected closed merchant context to reject slash buy frames, got %d", len(buyOut))
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after merchant close")
	}
	if currencySnapshot.Gold != 125 {
		t.Fatalf("expected merchant close to keep gold at 125, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after merchant close")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected merchant close to keep inventory unchanged, got %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted merchant close account: %v", err)
	}
	if account.Characters[0].Gold != 125 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted merchant-close account to stay unchanged, got %#v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyPacketDebitsCurrencyAndAddsItem(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacket", 0x01040105, 0x02050105, 125, nil)
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "merchant-buy-packet", 0x55555555, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop buy attempt error: %v", err)
	}
	if len(buyOut) != 2 {
		t.Fatalf("expected packet shop buy success path to emit 2 frames, got %d", len(buyOut))
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode packet shop-buy item frame: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(0) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected packet shop-buy item frame: %+v", set)
	}
	if err := shopproto.DecodeServerOK(decodeSingleFrame(t, buyOut[1])); err != nil {
		t.Fatalf("decode packet shop-buy ok frame: %v", err)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for packet shop buy, got %d", len(queued))
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after packet shop buy attempt")
	}
	if currencySnapshot.Gold != 75 {
		t.Fatalf("expected packet shop buy to debit gold to 75, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after packet shop buy attempt")
	}
	if len(inventorySnapshot.Inventory) != 1 {
		t.Fatalf("expected packet shop buy to add one inventory item, got %+v", inventorySnapshot.Inventory)
	}
	bought := inventorySnapshot.Inventory[0]
	if bought.ID == 0 || bought.Vnum != 27001 || bought.Count != 1 || bought.Slot != 0 {
		t.Fatalf("unexpected packet shop-buy inventory item: %+v", bought)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted packet merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 75 {
		t.Fatalf("expected persisted packet merchant buyer gold 75, got %d", account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 27001 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("unexpected persisted packet merchant buyer inventory: %#v", account.Characters[0].Inventory)
	}
}

func TestGameSessionFlowShopBuyPacketMergesIntoExistingCompatibleCarriedStack(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacketMerge", 0x01040106, 0x02050106, 125, []inventory.ItemInstance{{ID: 77, Vnum: 27001, Count: 3, Slot: 5}})
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "merchant-buy-packet-merge", 0x66666666, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected merchant packet buy merge error: %v", err)
	}
	if len(buyOut) != 2 {
		t.Fatalf("expected 2 frames for merged merchant packet buy, got %d", len(buyOut))
	}
	updatedItem, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode merged merchant packet buy item: %v", err)
	}
	if updatedItem.Position != itemproto.InventoryPosition(5) || updatedItem.Vnum != 27001 || updatedItem.Count != 4 {
		t.Fatalf("unexpected merged merchant packet buy item: %+v", updatedItem)
	}
	if err := shopproto.DecodeServerOK(decodeSingleFrame(t, buyOut[1])); err != nil {
		t.Fatalf("decode merged merchant packet buy ok frame: %v", err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after merged merchant packet buy")
	}
	if currencySnapshot.Gold != 75 {
		t.Fatalf("expected gold to drop to 75 after merged merchant packet buy, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after merged merchant packet buy")
	}
	if len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].ID != 77 || inventorySnapshot.Inventory[0].Slot != 5 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[0].Count != 4 {
		t.Fatalf("unexpected runtime merchant buyer merged inventory: %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted merged merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 75 || len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Slot != 5 || account.Characters[0].Inventory[0].Count != 4 {
		t.Fatalf("unexpected persisted merchant buyer merged inventory: %+v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyPacketPartiallyMergesIntoExistingCompatibleStackThenUsesFreshSlot(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacketPartialMerge", 0x01040109, 0x02050109, 125, []inventory.ItemInstance{{ID: 77, Vnum: 27001, Count: 199, Slot: 5}, {ID: 12, Vnum: 1120, Count: 1, Slot: 8}})
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "m-buy-p-partial", 0x99999999, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 2})))
	if err != nil {
		t.Fatalf("unexpected merchant packet buy partial-merge error: %v", err)
	}
	if len(buyOut) != 3 {
		t.Fatalf("expected 3 frames for partial-merge merchant packet buy, got %d", len(buyOut))
	}
	firstUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode partial-merge merchant packet buy first item: %v", err)
	}
	if firstUpdate.Position != itemproto.InventoryPosition(0) || firstUpdate.Vnum != 27001 || firstUpdate.Count != 1 {
		t.Fatalf("unexpected partial-merge merchant packet buy first item: %+v", firstUpdate)
	}
	secondUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[1]))
	if err != nil {
		t.Fatalf("decode partial-merge merchant packet buy second item: %v", err)
	}
	if secondUpdate.Position != itemproto.InventoryPosition(5) || secondUpdate.Vnum != 27001 || secondUpdate.Count != 200 {
		t.Fatalf("unexpected partial-merge merchant packet buy second item: %+v", secondUpdate)
	}
	if err := shopproto.DecodeServerOK(decodeSingleFrame(t, buyOut[2])); err != nil {
		t.Fatalf("decode partial-merge merchant packet buy ok frame: %v", err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after partial-merge merchant packet buy")
	}
	if currencySnapshot.Gold != 25 {
		t.Fatalf("expected gold to drop to 25 after partial-merge merchant packet buy, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after partial-merge merchant packet buy")
	}
	if len(inventorySnapshot.Inventory) != 3 || inventorySnapshot.Inventory[0].ID != 78 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 || inventorySnapshot.Inventory[1].ID != 77 || inventorySnapshot.Inventory[1].Vnum != 27001 || inventorySnapshot.Inventory[1].Count != 200 || inventorySnapshot.Inventory[1].Slot != 5 || inventorySnapshot.Inventory[2].ID != 12 || inventorySnapshot.Inventory[2].Vnum != 1120 || inventorySnapshot.Inventory[2].Count != 1 || inventorySnapshot.Inventory[2].Slot != 8 {
		t.Fatalf("unexpected runtime merchant buyer partial-merge inventory: %#v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted partial-merge merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 25 || !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{
		{ID: 78, Vnum: 27001, Count: 1, Slot: 0},
		{ID: 77, Vnum: 27001, Count: 200, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected persisted merchant buyer partial-merge inventory: %#v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyPacketFansOutAcrossSeveralExistingCompatibleStacksWithoutFreshSlot(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacketDistributedMerge", 0x01040110, 0x02050110, 1000, merchantBuyerFullInventoryWithDistributedPotionCapacity())
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "m-buy-p-dist", 0x10101010, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 2})))
	if err != nil {
		t.Fatalf("unexpected merchant packet buy distributed-merge error: %v", err)
	}
	if len(buyOut) != 3 {
		t.Fatalf("expected 3 frames for distributed-merge merchant packet buy, got %d", len(buyOut))
	}
	firstUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode distributed-merge merchant packet buy first item: %v", err)
	}
	if firstUpdate.Position != itemproto.InventoryPosition(5) || firstUpdate.Vnum != 27001 || firstUpdate.Count != 200 {
		t.Fatalf("unexpected distributed-merge merchant packet buy first item: %+v", firstUpdate)
	}
	secondUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[1]))
	if err != nil {
		t.Fatalf("decode distributed-merge merchant packet buy second item: %v", err)
	}
	if secondUpdate.Position != itemproto.InventoryPosition(7) || secondUpdate.Vnum != 27001 || secondUpdate.Count != 200 {
		t.Fatalf("unexpected distributed-merge merchant packet buy second item: %+v", secondUpdate)
	}
	if err := shopproto.DecodeServerOK(decodeSingleFrame(t, buyOut[2])); err != nil {
		t.Fatalf("decode distributed-merge merchant packet buy ok frame: %v", err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after distributed-merge merchant packet buy")
	}
	if currencySnapshot.Gold != 900 {
		t.Fatalf("expected gold to drop to 900 after distributed-merge merchant packet buy, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after distributed-merge merchant packet buy")
	}
	if len(inventorySnapshot.Inventory) != int(inventory.CarriedInventorySlotCount) || inventorySnapshot.Inventory[5].ID != 77 || inventorySnapshot.Inventory[5].Vnum != 27001 || inventorySnapshot.Inventory[5].Count != 200 || inventorySnapshot.Inventory[5].Slot != 5 || inventorySnapshot.Inventory[7].ID != 79 || inventorySnapshot.Inventory[7].Vnum != 27001 || inventorySnapshot.Inventory[7].Count != 200 || inventorySnapshot.Inventory[7].Slot != 7 {
		t.Fatalf("unexpected runtime merchant buyer distributed-merge inventory: %#v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted distributed-merge merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 900 || len(account.Characters[0].Inventory) != int(inventory.CarriedInventorySlotCount) || account.Characters[0].Inventory[5] != (inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 200, Slot: 5}) || account.Characters[0].Inventory[7] != (inventory.ItemInstance{ID: 79, Vnum: 27001, Count: 200, Slot: 7}) {
		t.Fatalf("unexpected persisted merchant buyer distributed-merge inventory: %#v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyPacketFansOutAcrossSeveralExistingCompatibleStacksThenUsesFreshSlot(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacketDistributedMergeFresh", 0x01040111, 0x02050111, 1000, []inventory.ItemInstance{{ID: 77, Vnum: 27001, Count: 199, Slot: 5}, {ID: 79, Vnum: 27001, Count: 198, Slot: 7}, {ID: 12, Vnum: 1120, Count: 1, Slot: 8}})
	runtime, accounts, flow, actorID, login := setupMerchantBuySessionWithCatalogDefinition(t, "m-buy-p-dist-slot", 0x11101010, buyer, merchantCatalogDefinitionWithPotionCount(4, 200))
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuyWithExpectedSlotTwo(t, flow, actorID, 4, 200)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 2})))
	if err != nil {
		t.Fatalf("unexpected merchant packet buy distributed-merge-plus-slot error: %v", err)
	}
	if len(buyOut) != 4 {
		t.Fatalf("expected 4 frames for distributed-merge-plus-slot merchant packet buy, got %d", len(buyOut))
	}
	firstUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode distributed-merge-plus-slot merchant packet buy first item: %v", err)
	}
	if firstUpdate.Position != itemproto.InventoryPosition(0) || firstUpdate.Vnum != 27001 || firstUpdate.Count != 1 {
		t.Fatalf("unexpected distributed-merge-plus-slot merchant packet buy first item: %+v", firstUpdate)
	}
	secondUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[1]))
	if err != nil {
		t.Fatalf("decode distributed-merge-plus-slot merchant packet buy second item: %v", err)
	}
	if secondUpdate.Position != itemproto.InventoryPosition(5) || secondUpdate.Vnum != 27001 || secondUpdate.Count != 200 {
		t.Fatalf("unexpected distributed-merge-plus-slot merchant packet buy second item: %+v", secondUpdate)
	}
	thirdUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[2]))
	if err != nil {
		t.Fatalf("decode distributed-merge-plus-slot merchant packet buy third item: %v", err)
	}
	if thirdUpdate.Position != itemproto.InventoryPosition(7) || thirdUpdate.Vnum != 27001 || thirdUpdate.Count != 200 {
		t.Fatalf("unexpected distributed-merge-plus-slot merchant packet buy third item: %+v", thirdUpdate)
	}
	if err := shopproto.DecodeServerOK(decodeSingleFrame(t, buyOut[3])); err != nil {
		t.Fatalf("decode distributed-merge-plus-slot merchant packet buy ok frame: %v", err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after distributed-merge-plus-slot merchant packet buy")
	}
	if currencySnapshot.Gold != 800 {
		t.Fatalf("expected gold to drop to 800 after distributed-merge-plus-slot merchant packet buy, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after distributed-merge-plus-slot merchant packet buy")
	}
	if len(inventorySnapshot.Inventory) != 4 || inventorySnapshot.Inventory[0].ID != 80 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 || inventorySnapshot.Inventory[1].ID != 77 || inventorySnapshot.Inventory[1].Vnum != 27001 || inventorySnapshot.Inventory[1].Count != 200 || inventorySnapshot.Inventory[1].Slot != 5 || inventorySnapshot.Inventory[2].ID != 79 || inventorySnapshot.Inventory[2].Vnum != 27001 || inventorySnapshot.Inventory[2].Count != 200 || inventorySnapshot.Inventory[2].Slot != 7 || inventorySnapshot.Inventory[3].ID != 12 || inventorySnapshot.Inventory[3].Vnum != 1120 || inventorySnapshot.Inventory[3].Count != 1 || inventorySnapshot.Inventory[3].Slot != 8 {
		t.Fatalf("unexpected runtime merchant buyer distributed-merge-plus-slot inventory: %#v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted distributed-merge-plus-slot merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 800 || !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{{ID: 80, Vnum: 27001, Count: 1, Slot: 0}, {ID: 77, Vnum: 27001, Count: 200, Slot: 5}, {ID: 79, Vnum: 27001, Count: 200, Slot: 7}, {ID: 12, Vnum: 1120, Count: 1, Slot: 8}}) {
		t.Fatalf("unexpected persisted merchant buyer distributed-merge-plus-slot inventory: %#v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyPacketReturnsMerchantNotEnoughMoneyOnInsufficientCurrency(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacketPoor", 0x01040107, 0x02050107, 25, nil)
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "merchant-buy-packet-poor", 0x77777777, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet insufficient-currency shop buy error: %v", err)
	}
	if len(buyOut) != 1 {
		t.Fatalf("expected insufficient-currency packet shop buy to emit 1 merchant error frame, got %d", len(buyOut))
	}
	if err := shopproto.DecodeServerNotEnoughMoney(decodeSingleFrame(t, buyOut[0])); err != nil {
		t.Fatalf("decode insufficient-currency packet merchant buy error frame: %v", err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after insufficient-currency packet buy attempt")
	}
	if currencySnapshot.Gold != 25 {
		t.Fatalf("expected gold to stay at 25 after insufficient-currency packet buy attempt, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after insufficient-currency packet buy attempt")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected inventory to stay empty after insufficient-currency packet buy attempt, got %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted insufficient-currency packet merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 25 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("unexpected persisted state after insufficient-currency packet buy attempt: %+v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyPacketReturnsMerchantInventoryFullOnNoValidPlacement(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacketPacked", 0x01040108, 0x02050108, 1000, merchantBuyerFullInventory())
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "merchant-buy-packet-packed", 0x88888888, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 1})))
	if err != nil {
		t.Fatalf("unexpected packet no-valid-placement shop buy error: %v", err)
	}
	if len(buyOut) != 1 {
		t.Fatalf("expected no-valid-placement packet shop buy to emit 1 merchant error frame, got %d", len(buyOut))
	}
	if err := shopproto.DecodeServerInventoryFull(decodeSingleFrame(t, buyOut[0])); err != nil {
		t.Fatalf("decode no-valid-placement packet merchant buy error frame: %v", err)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after no-valid-placement packet buy attempt")
	}
	if currencySnapshot.Gold != 1000 {
		t.Fatalf("expected gold to stay at 1000 after no-valid-placement packet buy attempt, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after no-valid-placement packet buy attempt")
	}
	if len(inventorySnapshot.Inventory) != int(inventory.CarriedInventorySlotCount) {
		t.Fatalf("expected full inventory to stay unchanged after no-valid-placement packet buy attempt, got %d items", len(inventorySnapshot.Inventory))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted no-valid-placement packet merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 1000 || len(account.Characters[0].Inventory) != int(inventory.CarriedInventorySlotCount) {
		t.Fatalf("unexpected persisted state after no-valid-placement packet buy attempt: %+v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyPacketFailsClosedWhenMerchantActorLosesInteractionContext(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacketLostActorContext", 0x01040112, 0x02050112, 125, nil)
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "m-buy-p-lctx", 0x12121212, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	if _, ok := runtime.UpdateStaticActor(actorID, "Merchant", bootstrapMapIndex, 1200, 2200, 20300); !ok {
		t.Fatal("expected merchant actor interaction-context removal to succeed")
	}
	_ = flushServerFrames(t, flow)

	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop buy error after merchant actor lost interaction context: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected packet shop buy to fail closed once merchant actor lost interaction context, got %d frames", len(buyOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after merchant actor lost interaction-context packet buy failure, got %d", len(queued))
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after merchant actor lost interaction-context packet buy attempt")
	}
	if currencySnapshot.Gold != 125 {
		t.Fatalf("expected gold to stay at 125 after merchant actor lost interaction-context packet buy attempt, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after merchant actor lost interaction-context packet buy attempt")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected inventory to stay empty after merchant actor lost interaction-context packet buy attempt, got %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted merchant actor lost interaction-context packet buyer account: %v", err)
	}
	if account.Characters[0].Gold != 125 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("unexpected persisted state after merchant actor lost interaction-context packet buy attempt: %+v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyPacketFailsClosedWhenBoundCatalogSnapshotBecomesStale(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacketStaleCatalog", 0x01040113, 0x02050113, 1000, nil)
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "m-buy-p-stale", 0x13131313, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	updated, err := runtime.UpsertInteractionDefinition(merchantCatalogDefinitionWithPotionCount(4, 200))
	if err != nil {
		t.Fatalf("update merchant interaction definition after window open: %v", err)
	}
	if updated.Catalog[2].Count != 4 || updated.Catalog[2].Price != 200 {
		t.Fatalf("unexpected updated merchant interaction definition after window open: %+v", updated)
	}

	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 2})))
	if err != nil {
		t.Fatalf("unexpected packet shop buy error after merchant catalog changed: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected packet shop buy to fail closed once bound merchant catalog snapshot became stale, got %d frames", len(buyOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after stale merchant catalog packet buy failure, got %d", len(queued))
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after stale merchant catalog packet buy attempt")
	}
	if currencySnapshot.Gold != 1000 {
		t.Fatalf("expected gold to stay at 1000 after stale merchant catalog packet buy attempt, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after stale merchant catalog packet buy attempt")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected inventory to stay empty after stale merchant catalog packet buy attempt, got %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted stale merchant catalog packet buyer account: %v", err)
	}
	if account.Characters[0].Gold != 1000 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("unexpected persisted state after stale merchant catalog packet buy attempt: %+v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyInteractionDebitsCurrencyAndAddsItem(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerSuccess", 0x01040101, 0x02050101, 125, nil)
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "merchant-buy-success", 0x11111111, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/shop_buy 0"})))
	if err != nil {
		t.Fatalf("unexpected shop buy attempt error: %v", err)
	}
	if len(buyOut) == 0 {
		t.Fatalf("expected merchant buy success path to emit at least one frame")
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after merchant buy attempt")
	}
	if currencySnapshot.Gold != 75 {
		t.Fatalf("expected merchant buy to debit gold to 75, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after merchant buy attempt")
	}
	if len(inventorySnapshot.Inventory) != 1 {
		t.Fatalf("expected merchant buy to add one inventory item, got %+v", inventorySnapshot.Inventory)
	}
	bought := inventorySnapshot.Inventory[0]
	if bought.ID == 0 || bought.Vnum != 27001 || bought.Count != 1 || bought.Slot != 0 {
		t.Fatalf("unexpected bought inventory item: %+v", bought)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 75 {
		t.Fatalf("expected persisted merchant buyer gold 75, got %d", account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 27001 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("unexpected persisted merchant buyer inventory: %#v", account.Characters[0].Inventory)
	}
}

func TestGameSessionFlowShopBuyInteractionFansOutAcrossSeveralExistingCompatibleStacksThenUsesFreshSlot(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerSlashDistributedMergeFresh", 0x01040112, 0x02050112, 1000, []inventory.ItemInstance{{ID: 77, Vnum: 27001, Count: 199, Slot: 5}, {ID: 79, Vnum: 27001, Count: 198, Slot: 7}, {ID: 12, Vnum: 1120, Count: 1, Slot: 8}})
	runtime, accounts, flow, actorID, login := setupMerchantBuySessionWithCatalogDefinition(t, "m-buy-slash-dist-slot", 0x12101010, buyer, merchantCatalogDefinitionWithPotionCount(4, 200))
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuyWithExpectedSlotTwo(t, flow, actorID, 4, 200)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/shop_buy 2"})))
	if err != nil {
		t.Fatalf("unexpected slash distributed-merge-plus-slot merchant buy error: %v", err)
	}
	if len(buyOut) != 4 {
		t.Fatalf("expected 4 frames for slash distributed-merge-plus-slot merchant buy, got %d", len(buyOut))
	}
	firstUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode slash distributed-merge-plus-slot merchant buy first item: %v", err)
	}
	if firstUpdate.Position != itemproto.InventoryPosition(0) || firstUpdate.Vnum != 27001 || firstUpdate.Count != 1 {
		t.Fatalf("unexpected slash distributed-merge-plus-slot merchant buy first item: %+v", firstUpdate)
	}
	secondUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[1]))
	if err != nil {
		t.Fatalf("decode slash distributed-merge-plus-slot merchant buy second item: %v", err)
	}
	if secondUpdate.Position != itemproto.InventoryPosition(5) || secondUpdate.Vnum != 27001 || secondUpdate.Count != 200 {
		t.Fatalf("unexpected slash distributed-merge-plus-slot merchant buy second item: %+v", secondUpdate)
	}
	thirdUpdate, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[2]))
	if err != nil {
		t.Fatalf("decode slash distributed-merge-plus-slot merchant buy third item: %v", err)
	}
	if thirdUpdate.Position != itemproto.InventoryPosition(7) || thirdUpdate.Vnum != 27001 || thirdUpdate.Count != 200 {
		t.Fatalf("unexpected slash distributed-merge-plus-slot merchant buy third item: %+v", thirdUpdate)
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, buyOut[3]))
	if err != nil {
		t.Fatalf("decode slash distributed-merge-plus-slot merchant buy delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.Message != "Merchant purchase complete." {
		t.Fatalf("unexpected slash distributed-merge-plus-slot merchant buy delivery: %+v", delivery)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after slash distributed-merge-plus-slot merchant buy")
	}
	if currencySnapshot.Gold != 800 {
		t.Fatalf("expected gold to drop to 800 after slash distributed-merge-plus-slot merchant buy, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after slash distributed-merge-plus-slot merchant buy")
	}
	if len(inventorySnapshot.Inventory) != 4 || inventorySnapshot.Inventory[0].ID != 80 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 || inventorySnapshot.Inventory[1].ID != 77 || inventorySnapshot.Inventory[1].Vnum != 27001 || inventorySnapshot.Inventory[1].Count != 200 || inventorySnapshot.Inventory[1].Slot != 5 || inventorySnapshot.Inventory[2].ID != 79 || inventorySnapshot.Inventory[2].Vnum != 27001 || inventorySnapshot.Inventory[2].Count != 200 || inventorySnapshot.Inventory[2].Slot != 7 || inventorySnapshot.Inventory[3].ID != 12 || inventorySnapshot.Inventory[3].Vnum != 1120 || inventorySnapshot.Inventory[3].Count != 1 || inventorySnapshot.Inventory[3].Slot != 8 {
		t.Fatalf("unexpected runtime slash distributed-merge-plus-slot inventory: %#v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted slash distributed-merge-plus-slot merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 800 || !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{{ID: 80, Vnum: 27001, Count: 1, Slot: 0}, {ID: 77, Vnum: 27001, Count: 200, Slot: 5}, {ID: 79, Vnum: 27001, Count: 200, Slot: 7}, {ID: 12, Vnum: 1120, Count: 1, Slot: 8}}) {
		t.Fatalf("unexpected persisted slash distributed-merge-plus-slot inventory: %#v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyInteractionReturnsInfoDeliveryOnInsufficientCurrency(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPoor", 0x01040102, 0x02050102, 25, nil)
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "merchant-buy-poor", 0x22222222, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/shop_buy 0"})))
	if err != nil {
		t.Fatalf("unexpected insufficient-currency shop buy attempt error: %v", err)
	}
	if len(buyOut) != 1 {
		t.Fatalf("expected insufficient-currency shop buy to emit 1 info frame, got %d", len(buyOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode insufficient-currency shop buy delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.Message != "Not enough gold." {
		t.Fatalf("unexpected insufficient-currency shop buy delivery: %+v", delivery)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after insufficient-currency buy attempt")
	}
	if currencySnapshot.Gold != 25 {
		t.Fatalf("expected gold to stay at 25 after insufficient-currency buy attempt, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after insufficient-currency buy attempt")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected inventory to stay empty after insufficient-currency buy attempt, got %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted insufficient-currency merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 25 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("unexpected persisted state after insufficient-currency buy attempt: %+v", account.Characters[0])
	}
}

func TestGameSessionFlowShopBuyInteractionReturnsInfoDeliveryOnNoValidPlacement(t *testing.T) {
	buyer := merchantBuyerCharacter("MerchantBuyerPacked", 0x01040103, 0x02050103, 1000, merchantBuyerFullInventory())
	runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, "merchant-buy-packed", 0x33333333, buyer)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, actorID)
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/shop_buy 1"})))
	if err != nil {
		t.Fatalf("unexpected no-free-slot shop buy attempt error: %v", err)
	}
	if len(buyOut) != 1 {
		t.Fatalf("expected no-valid-placement shop buy to emit 1 info frame, got %d", len(buyOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode no-valid-placement shop buy delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.Message != "Inventory full." {
		t.Fatalf("unexpected no-valid-placement shop buy delivery: %+v", delivery)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after no-free-slot buy attempt")
	}
	if currencySnapshot.Gold != 1000 {
		t.Fatalf("expected gold to stay at 1000 after no-free-slot buy attempt, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after no-free-slot buy attempt")
	}
	if len(inventorySnapshot.Inventory) != int(inventory.CarriedInventorySlotCount) {
		t.Fatalf("expected full inventory to stay unchanged after no-free-slot buy attempt, got %d items", len(inventorySnapshot.Inventory))
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted no-free-slot merchant buyer account: %v", err)
	}
	if account.Characters[0].Gold != 1000 || len(account.Characters[0].Inventory) != int(inventory.CarriedInventorySlotCount) {
		t.Fatalf("unexpected persisted state after no-free-slot buy attempt: %+v", account.Characters[0])
	}
}

func TestGameSessionFlowStaticActorInteractionCooldownSuppressesRepeatedFramesPerActorAndExpires(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		{Kind: interactionstore.KindTalk, Ref: "npc:guard", Text: "Keep your blade sharp."},
		{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."},
	})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000000, 0)
	runtime.now = func() time.Time { return currentTime }
	guard, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindTalk, "npc:guard")
	if !ok {
		t.Fatal("expected talk static actor registration to succeed")
	}
	alchemist, ok := runtime.RegisterStaticActorWithInteraction("Alchemist", bootstrapMapIndex, 1300, 2300, 20302, interactionstore.KindInfo, "lore:alchemist")
	if !ok {
		t.Fatal("expected info static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	first, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(guard.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected first interaction error: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 first interaction frame, got %d", len(first))
	}
	firstDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, first[0]))
	if err != nil {
		t.Fatalf("decode first interaction delivery: %v", err)
	}
	if firstDelivery.Message != "VillageGuard:\nKeep your blade sharp." {
		t.Fatalf("unexpected first interaction delivery: %+v", firstDelivery)
	}

	repeated, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(guard.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected repeated interaction error: %v", err)
	}
	if len(repeated) != 0 {
		t.Fatalf("expected cooldown to suppress repeated interaction frames, got %d", len(repeated))
	}

	otherActor, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(alchemist.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected second-actor interaction error: %v", err)
	}
	if len(otherActor) != 1 {
		t.Fatalf("expected different actor interaction to bypass the first actor cooldown, got %d frames", len(otherActor))
	}
	otherDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, otherActor[0]))
	if err != nil {
		t.Fatalf("decode second-actor interaction delivery: %v", err)
	}
	if otherDelivery.Message != "The alchemist studies forgotten herbs." {
		t.Fatalf("unexpected second-actor interaction delivery: %+v", otherDelivery)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	afterCooldown, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(guard.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected post-cooldown interaction error: %v", err)
	}
	if len(afterCooldown) != 1 {
		t.Fatalf("expected cooldown expiry to restore interaction delivery, got %d frames", len(afterCooldown))
	}
}

func TestGameSessionFlowStaticActorInteractionCooldownIsPerPlayerAndClearsAcrossReconnect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:guard", Text: "Keep your blade sharp."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000100, 0)
	runtime.now = func() time.Time { return currentTime }
	guard, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindTalk, "npc:guard")
	if !ok {
		t.Fatal("expected talk static actor registration to succeed")
	}

	flowOne, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	firstOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(guard.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected first player interaction error: %v", err)
	}
	if len(firstOut) != 1 {
		t.Fatalf("expected first player interaction to produce 1 frame, got %d", len(firstOut))
	}
	firstRepeat, err := flowOne.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(guard.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected first player repeated interaction error: %v", err)
	}
	if len(firstRepeat) != 0 {
		t.Fatalf("expected first player repeated interaction to be cooldown-suppressed, got %d frames", len(firstRepeat))
	}

	flowTwo, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	defer closeSessionFlow(t, flowTwo)
	secondPlayerOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(guard.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected second player interaction error: %v", err)
	}
	if len(secondPlayerOut) != 1 {
		t.Fatalf("expected second player interaction to bypass the first player's cooldown, got %d frames", len(secondPlayerOut))
	}

	closeSessionFlow(t, flowOne)
	issuePeerTicket(t, store, "peer-one", 0x33333333, peerOne)
	flowReconnect, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x33333333)
	defer closeSessionFlow(t, flowReconnect)
	reconnectOut, err := flowReconnect.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(guard.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected reconnect interaction error: %v", err)
	}
	if len(reconnectOut) != 1 {
		t.Fatalf("expected reconnect to clear the prior session cooldown, got %d frames", len(reconnectOut))
	}
}

func TestGameSessionFlowStaticActorWarpInteractionCooldownSuppressesRepeatedTransfers(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: bootstrapMapIndex, X: peer.X, Y: peer.Y, Text: "Step through the gate."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000200, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.RegisterStaticActorWithInteraction("Teleporter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindWarp, "npc:teleporter")
	if !ok {
		t.Fatal("expected warp static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	first, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected first warp interaction error: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected first warp interaction to produce frames")
	}
	firstDelivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, first[0]))
	if err != nil {
		t.Fatalf("decode first warp interaction delivery: %v", err)
	}
	if firstDelivery.Message != "Step through the gate." {
		t.Fatalf("unexpected first warp interaction delivery: %+v", firstDelivery)
	}

	repeated, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected repeated warp interaction error: %v", err)
	}
	if len(repeated) != 0 {
		t.Fatalf("expected cooldown to suppress repeated warp interaction frames, got %d", len(repeated))
	}
}

func TestGameRuntimeResolveStaticActorWarpInteractionReturnsAcceptedTransferDefinition(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Teleporter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindWarp, "npc:teleporter")
	if !ok {
		t.Fatal("expected warp static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	subject, ok := runtime.sharedWorld.entities.PlayerByName(peer.Name)
	if !ok {
		t.Fatalf("expected live shared-world entity for %q after enter", peer.Name)
	}
	resolution := runtime.resolveStaticActorInteraction(subject.Entity.ID, uint32(actor.EntityID))
	if !resolution.Accepted {
		t.Fatalf("expected warp interaction resolution to be accepted, got %+v", resolution)
	}
	if resolution.Failure != "" {
		t.Fatalf("expected accepted warp interaction to carry no failure, got %+v", resolution)
	}
	if resolution.Definition.Kind != interactionstore.KindWarp || resolution.Definition.Ref != "npc:teleporter" || resolution.Definition.MapIndex != 42 || resolution.Definition.X != 1700 || resolution.Definition.Y != 2800 {
		t.Fatalf("unexpected warp interaction definition: %+v", resolution.Definition)
	}
	if resolution.Delivery == nil || resolution.Delivery.Type != chatproto.ChatTypeInfo || resolution.Delivery.Message != "Step through the gate." {
		t.Fatalf("expected warp interaction to carry optional self chat delivery, got %+v", resolution.Delivery)
	}
}

func TestGameSessionFlowStaticActorWarpInteractionReturnsTransferRebootstrapFrames(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: ""}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Teleporter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindWarp, "npc:teleporter")
	if !ok {
		t.Fatal("expected warp static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible interactable static actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected warp interaction error: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("expected 5 warp interaction frames (4 self rebootstrap + 1 static actor delete), got %d", len(out))
	}
	connected := runtime.ConnectedCharacters()
	if len(connected) != 1 || connected[0].MapIndex != 42 || connected[0].X != 1700 || connected[0].Y != 2800 {
		t.Fatalf("expected runtime connected character snapshot to move to warp destination, got %+v", connected)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for self-only warp interaction, got %d", len(queued))
	}
}

func TestGameRuntimeResolveStaticActorWarpInteractionRejectsInvalidDestinationDefinition(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, nil)

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "BrokenTeleporter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindWarp, "npc:broken-teleporter")
	if !ok {
		t.Fatal("expected direct shared-world registration with warp metadata to succeed for invalid-destination runtime test")
	}
	runtime.interactionDefinitionMu.Lock()
	if runtime.interactionDefinitions == nil {
		runtime.interactionDefinitions = make(map[string]interactionstore.Definition)
	}
	runtime.interactionDefinitions[interactionDefinitionKey(interactionstore.KindWarp, "npc:broken-teleporter")] = InteractionDefinition{Kind: interactionstore.KindWarp, Ref: "npc:broken-teleporter", MapIndex: 0, X: 1700, Y: 2800}
	runtime.interactionDefinitionMu.Unlock()
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	subject, ok := runtime.sharedWorld.entities.PlayerByName(peer.Name)
	if !ok {
		t.Fatalf("expected live shared-world entity for %q after enter", peer.Name)
	}
	resolution := runtime.resolveStaticActorInteraction(subject.Entity.ID, uint32(actor.EntityID))
	if resolution.Accepted {
		t.Fatalf("expected invalid warp interaction definition to be rejected, got %+v", resolution)
	}
	if resolution.Failure != staticActorInteractionFailureWarpDestinationInvalid {
		t.Fatalf("expected invalid warp destination failure %q, got %+v", staticActorInteractionFailureWarpDestinationInvalid, resolution)
	}
	if resolution.Delivery == nil || resolution.Delivery.Type != chatproto.ChatTypeInfo || resolution.Delivery.Message != "Warp destination is invalid." {
		t.Fatalf("unexpected invalid warp destination delivery: %+v", resolution.Delivery)
	}
}

func TestGameSessionFlowStaticActorWarpInteractionReturnsSelfOnlyChatWhenTransferNotApplied(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	accounts := newPreloadedFailingAccountStore(accountstore.Account{Login: "peer-one", Empire: peer.Empire, Characters: []loginticket.Character{peer}})
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindWarp, Ref: "npc:teleporter", MapIndex: 42, X: 1700, Y: 2800, Text: "Step through the gate."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Teleporter", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindWarp, "npc:teleporter")
	if !ok {
		t.Fatal("expected warp static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected warp transfer failure interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only warp transfer failure frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode warp transfer failure chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "Warp unavailable right now." {
		t.Fatalf("unexpected warp transfer failure chat delivery: %+v", delivery)
	}
	connected := runtime.ConnectedCharacters()
	if len(connected) != 1 || connected[0].MapIndex != bootstrapMapIndex || connected[0].X != peer.X || connected[0].Y != peer.Y {
		t.Fatalf("expected failed warp interaction to keep the player at the original location, got %+v", connected)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for failed self-only warp interaction, got %d", len(queued))
	}
}

func TestGameRuntimeResolveStaticActorFailureInteractionReturnsSelfOnlyChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindInfo, Ref: "lore:alchemist", Text: "The alchemist studies forgotten herbs."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	blacksmith, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1200, 2200, 20301)
	if !ok {
		t.Fatal("expected non-interactable static actor registration to succeed")
	}
	farActor, ok := runtime.RegisterStaticActorWithInteraction("FarAlchemist", bootstrapMapIndex+1, 1200, 2200, 20300, interactionstore.KindInfo, "lore:alchemist")
	if !ok {
		t.Fatal("expected invisible interactable static actor registration to succeed")
	}
	brokenActor, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "BrokenAlchemist", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindInfo, "lore:missing")
	if !ok {
		t.Fatal("expected direct shared-world registration with dangling ref to succeed for fail-closed runtime test")
	}
	unsupportedActor, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "MysticPreview", bootstrapMapIndex, 1200, 2200, 20300, "quest_offer", "npc:mystic")
	if !ok {
		t.Fatal("expected direct shared-world registration with unsupported kind to succeed for fail-closed runtime test")
	}
	runtime.interactionDefinitionMu.Lock()
	runtime.interactionDefinitions[interactionDefinitionKey("quest_offer", "npc:mystic")] = InteractionDefinition{Kind: "quest_offer", Ref: "npc:mystic", Text: "preview"}
	runtime.interactionDefinitionMu.Unlock()

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	subject, ok := runtime.sharedWorld.entities.PlayerByName(peer.Name)
	if !ok {
		t.Fatalf("expected live shared-world entity for %q after enter", peer.Name)
	}

	tests := []struct {
		name        string
		subjectID   uint64
		targetVID   uint32
		wantFailure string
		wantMessage string
	}{
		{name: "subject not found", subjectID: 999, targetVID: uint32(blacksmith.EntityID), wantFailure: StaticActorInteractionFailureSubjectNotFound, wantMessage: "Interaction unavailable right now."},
		{name: "target not visible", subjectID: subject.Entity.ID, targetVID: uint32(farActor.EntityID), wantFailure: StaticActorInteractionFailureTargetNotVisible, wantMessage: "You cannot interact with that target right now."},
		{name: "target has no interaction", subjectID: subject.Entity.ID, targetVID: uint32(blacksmith.EntityID), wantFailure: StaticActorInteractionFailureTargetHasNoInteraction, wantMessage: "Nothing happens."},
		{name: "interaction definition not found", subjectID: subject.Entity.ID, targetVID: uint32(brokenActor.EntityID), wantFailure: staticActorInteractionFailureDefinitionNotFound, wantMessage: "Interaction content is missing."},
		{name: "unsupported interaction kind", subjectID: subject.Entity.ID, targetVID: uint32(unsupportedActor.EntityID), wantFailure: staticActorInteractionFailureUnsupportedKind, wantMessage: "Interaction not supported yet."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolution := runtime.resolveStaticActorInteraction(tc.subjectID, tc.targetVID)
			if resolution.Accepted {
				t.Fatalf("expected failure interaction resolution to remain rejected, got %+v", resolution)
			}
			if resolution.Failure != tc.wantFailure {
				t.Fatalf("expected failure %q, got %+v", tc.wantFailure, resolution)
			}
			if resolution.Delivery == nil {
				t.Fatalf("expected failure interaction resolution to return a self chat delivery, got %+v", resolution)
			}
			if resolution.Delivery.Type != chatproto.ChatTypeInfo || resolution.Delivery.VID != 0 || resolution.Delivery.Empire != 0 || resolution.Delivery.Message != tc.wantMessage {
				t.Fatalf("unexpected failure interaction delivery for %q: %+v", tc.name, resolution.Delivery)
			}
		})
	}
}

func TestGameSessionFlowStaticActorFailedInteractionReturnsSelfOnlyChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, nil)

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("Blacksmith", bootstrapMapIndex, 1200, 2200, 20301)
	if !ok {
		t.Fatal("expected non-interactable static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible non-interactable static actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected failed interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only failed interaction frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode failed interaction chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "Nothing happens." {
		t.Fatalf("unexpected failed interaction chat delivery: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for failed self-only interaction, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorMissingDefinitionFailureReturnsSelfOnlyChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, nil)

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "BrokenAlchemist", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindInfo, "lore:missing")
	if !ok {
		t.Fatal("expected direct shared-world registration with dangling ref to succeed for failed session-flow interaction test")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible interactable static actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected missing-definition interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only missing-definition interaction frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode missing-definition interaction chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "Interaction content is missing." {
		t.Fatalf("unexpected missing-definition interaction chat delivery: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for missing-definition self-only interaction, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorOutOfRangeInteractionReturnsSelfOnlyChatDelivery(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{Kind: interactionstore.KindTalk, Ref: "npc:village_guard", Text: "Keep your blade sharp."}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("VillageGuard", bootstrapMapIndex, 2600, 3600, 20300, interactionstore.KindTalk, "npc:village_guard")
	if !ok {
		t.Fatal("expected visible but far interactable static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible far interactable static actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected out-of-range interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only out-of-range interaction frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode out-of-range interaction chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "You are too far away to interact with that target." {
		t.Fatalf("unexpected out-of-range interaction chat delivery: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for out-of-range self-only interaction, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorCombatTargetReturnsSelfOnlyTargetPacket(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat target error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only combat target frame, got %d", len(out))
	}
	target, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode combat target frame: %v", err)
	}
	if target.TargetVID != uint32(actor.EntityID) || target.HPPercent != 100 {
		t.Fatalf("unexpected combat target packet: %+v", target)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for self-only combat targeting, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorCombatTargetRejectsVisibleNonTargetableActorWithoutFrames(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActor("VillageGuard", bootstrapMapIndex, 1200, 2200, 20300)
	if !ok {
		t.Fatal("expected visible non-targetable static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible non-targetable actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected non-targetable combat target error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no self frames for rejected non-targetable combat target, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for rejected combat target, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorAttackReturnsSelfOnlyTargetRefreshForSelectedDummy(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000200, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat target error: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame, got %d", len(selectOut))
	}
	selected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, selectOut[0]))
	if err != nil {
		t.Fatalf("decode selected training-dummy target frame: %v", err)
	}
	if selected.TargetVID != uint32(actor.EntityID) || selected.HPPercent != 100 {
		t.Fatalf("unexpected selected training-dummy target packet: %+v", selected)
	}

	firstAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(actor.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error: %v", err)
	}
	if len(firstAttack) != 1 {
		t.Fatalf("expected 1 self-only target-refresh frame for accepted dummy attack, got %d", len(firstAttack))
	}
	firstTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, firstAttack[0]))
	if err != nil {
		t.Fatalf("decode accepted dummy attack target-refresh frame: %v", err)
	}
	if firstTarget.TargetVID != uint32(actor.EntityID) || firstTarget.HPPercent != 90 {
		t.Fatalf("unexpected first accepted dummy attack target-refresh packet: %+v", firstTarget)
	}

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	secondAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(actor.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected second combat attack error: %v", err)
	}
	if len(secondAttack) != 1 {
		t.Fatalf("expected 1 self-only target-refresh frame for second accepted dummy attack, got %d", len(secondAttack))
	}
	secondTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, secondAttack[0]))
	if err != nil {
		t.Fatalf("decode second accepted dummy attack target-refresh frame: %v", err)
	}
	if secondTarget.TargetVID != uint32(actor.EntityID) || secondTarget.HPPercent != 80 {
		t.Fatalf("unexpected second accepted dummy attack target-refresh packet: %+v", secondTarget)
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat reselect error after dummy damage: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after dummy damage, got %d", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode reselected damaged training-dummy target frame: %v", err)
	}
	if reselected.TargetVID != uint32(actor.EntityID) || reselected.HPPercent != 80 {
		t.Fatalf("unexpected reselected damaged training-dummy target packet: %+v", reselected)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for accepted bootstrap dummy attack, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorAttackTransitionsSelectedDummyToDeadStateAndRejectsPostDeathRequests(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000250, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	targetVID := uint32(actor.EntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected combat target error before death transition: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame before death transition, got %d", len(selectOut))
	}

	for attackIndex := 0; attackIndex < 9; attackIndex++ {
		if attackIndex > 0 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected combat attack error on pre-death hit %d: %v", attackIndex+1, err)
		}
		if len(attackOut) != 1 {
			t.Fatalf("expected 1 self-only target-refresh frame on pre-death hit %d, got %d", attackIndex+1, len(attackOut))
		}
		refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
		if err != nil {
			t.Fatalf("decode pre-death target-refresh frame %d: %v", attackIndex+1, err)
		}
		wantHPPercent := uint8(90 - (attackIndex * 10))
		if refresh.TargetVID != targetVID || refresh.HPPercent != wantHPPercent {
			t.Fatalf("unexpected target-refresh packet on pre-death hit %d: %+v", attackIndex+1, refresh)
		}
	}

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	finalAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error on zero-HP death hit: %v", err)
	}
	if len(finalAttack) != 2 {
		t.Fatalf("expected 2 self-only death-transition frames on zero-HP hit, got %d", len(finalAttack))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, finalAttack[0]))
	if err != nil {
		t.Fatalf("decode zero-HP death frame: %v", err)
	}
	if dead.VID != targetVID {
		t.Fatalf("unexpected dead packet on zero-HP hit: %+v", dead)
	}
	cleared, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, finalAttack[1]))
	if err != nil {
		t.Fatalf("decode zero-HP clear-target frame: %v", err)
	}
	if cleared.TargetVID != 0 || cleared.HPPercent != 0 {
		t.Fatalf("expected zero-target clear after zero-HP death, got %+v", cleared)
	}

	postDeathAttackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-death combat attack error: %v", err)
	}
	if len(postDeathAttackOut) != 0 {
		t.Fatalf("expected zero self frames for post-death attack rejection, got %d", len(postDeathAttackOut))
	}

	postDeathReselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected post-death combat target error: %v", err)
	}
	if len(postDeathReselectOut) != 0 {
		t.Fatalf("expected zero self frames for post-death target rejection, got %d", len(postDeathReselectOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for zero-HP death with a single visible session, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorDummyDeathClearsOtherSelectedVisibleSessions(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000275, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flowOne, enterOne := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOne) != 8 {
		t.Fatalf("expected 8 bootstrap frames for first player with visible training dummy, got %d", len(enterOne))
	}
	defer closeSessionFlow(t, flowOne)
	flowTwo, enterTwo := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(enterTwo) != 11 {
		t.Fatalf("expected 11 bootstrap frames for second player with visible peer and training dummy, got %d", len(enterTwo))
	}
	defer closeSessionFlow(t, flowTwo)
	if queued := flushServerFrames(t, flowOne); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for the first player after second player joins, got %d", len(queued))
	}

	targetVID := uint32(actor.EntityID)
	for idx, flow := range []service.SessionFlow{flowOne, flowTwo} {
		selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected combat target error for selected visible session %d: %v", idx+1, err)
		}
		if len(selectOut) != 1 {
			t.Fatalf("expected 1 self-only combat target frame for selected visible session %d, got %d", idx+1, len(selectOut))
		}
	}

	for attackIndex := 0; attackIndex < 9; attackIndex++ {
		if attackIndex > 0 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		attackOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected combat attack error on visible-session pre-death hit %d: %v", attackIndex+1, err)
		}
		if len(attackOut) != 1 {
			t.Fatalf("expected 1 self-only target-refresh frame on visible-session pre-death hit %d, got %d", attackIndex+1, len(attackOut))
		}
	}

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	finalAttack, err := flowOne.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error on visible-session zero-HP death hit: %v", err)
	}
	if len(finalAttack) != 2 {
		t.Fatalf("expected 2 self-only death-transition frames for the killing hit, got %d", len(finalAttack))
	}
	peerFrames := flushServerFrames(t, flowTwo)
	if len(peerFrames) != 2 {
		t.Fatalf("expected 2 queued death-transition frames for the other selected visible session, got %d", len(peerFrames))
	}
	peerDead, err := worldproto.DecodeDead(decodeSingleFrame(t, peerFrames[0]))
	if err != nil {
		t.Fatalf("decode queued dead frame for other selected visible session: %v", err)
	}
	if peerDead.VID != targetVID {
		t.Fatalf("unexpected queued dead packet for other selected visible session: %+v", peerDead)
	}
	peerCleared, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, peerFrames[1]))
	if err != nil {
		t.Fatalf("decode queued clear-target frame for other selected visible session: %v", err)
	}
	if peerCleared.TargetVID != 0 || peerCleared.HPPercent != 0 {
		t.Fatalf("expected zero-target clear for other selected visible session after zero-HP death, got %+v", peerCleared)
	}
	if queued := flushServerFrames(t, flowOne); len(queued) != 0 {
		t.Fatalf("expected no queued self frames for the killing session after zero-HP death, got %d", len(queued))
	}

	peerPostDeathAttackOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-death combat attack error for cleared visible session: %v", err)
	}
	if len(peerPostDeathAttackOut) != 0 {
		t.Fatalf("expected zero self frames for cleared visible-session post-death attack rejection, got %d", len(peerPostDeathAttackOut))
	}

	peerPostDeathReselectOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected post-death combat target error for cleared visible session: %v", err)
	}
	if len(peerPostDeathReselectOut) != 0 {
		t.Fatalf("expected zero self frames for cleared visible-session post-death target rejection, got %d", len(peerPostDeathReselectOut))
	}
}

func TestGameSessionFlowStaticActorDummyRespawnsAfterServerDrivenDelayAndRequiresFreshReselect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000300, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	targetVID := uint32(actor.EntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected combat target error before respawn slice: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame before respawn slice, got %d", len(selectOut))
	}

	for attackIndex := 0; attackIndex < 9; attackIndex++ {
		if attackIndex > 0 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected combat attack error on respawn pre-death hit %d: %v", attackIndex+1, err)
		}
		if len(attackOut) != 1 {
			t.Fatalf("expected 1 self-only target-refresh frame on respawn pre-death hit %d, got %d", attackIndex+1, len(attackOut))
		}
	}

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	finalAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error on respawn death hit: %v", err)
	}
	if len(finalAttack) != 2 {
		t.Fatalf("expected 2 self-only death-transition frames before respawn, got %d", len(finalAttack))
	}

	currentTime = currentTime.Add((2 * time.Second) - time.Millisecond)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued respawn frames before the server-driven delay expires, got %d", len(queued))
	}

	currentTime = currentTime.Add(time.Millisecond)
	respawnFrames := flushServerFrames(t, flow)
	if len(respawnFrames) != 4 {
		t.Fatalf("expected delete + add/info/update respawn rebuild after server-driven delay, got %d frames", len(respawnFrames))
	}
	deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, respawnFrames[0]))
	if err != nil {
		t.Fatalf("decode respawn delete frame: %v", err)
	}
	if deleted.VID != targetVID {
		t.Fatalf("unexpected respawn delete frame: %+v", deleted)
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, respawnFrames[1]))
	if err != nil {
		t.Fatalf("decode respawn add frame: %v", err)
	}
	if added.VID != targetVID || added.X != 1200 || added.Y != 2200 || added.RaceNum != 20350 {
		t.Fatalf("unexpected respawn add frame: %+v", added)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, respawnFrames[2]))
	if err != nil {
		t.Fatalf("decode respawn additional info frame: %v", err)
	}
	if info.VID != targetVID || info.Name != "TrainingDummy" {
		t.Fatalf("unexpected respawn additional info frame: %+v", info)
	}
	updated, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, respawnFrames[3]))
	if err != nil {
		t.Fatalf("decode respawn update frame: %v", err)
	}
	if updated.VID != targetVID {
		t.Fatalf("unexpected respawn update frame: %+v", updated)
	}

	attackWithoutReselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-respawn attack-without-reselect error: %v", err)
	}
	if len(attackWithoutReselectOut) != 0 {
		t.Fatalf("expected post-respawn attack without fresh target selection to fail closed, got %d frames", len(attackWithoutReselectOut))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected post-respawn combat reselect error: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after respawn reselection, got %d", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode post-respawn target frame: %v", err)
	}
	if reselected.TargetVID != targetVID || reselected.HPPercent != 100 {
		t.Fatalf("unexpected post-respawn target packet: %+v", reselected)
	}

	postRespawnAttackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-respawn combat attack error: %v", err)
	}
	if len(postRespawnAttackOut) != 1 {
		t.Fatalf("expected 1 self-only target-refresh frame after respawn reselection, got %d", len(postRespawnAttackOut))
	}
	postRespawnTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, postRespawnAttackOut[0]))
	if err != nil {
		t.Fatalf("decode post-respawn target-refresh frame: %v", err)
	}
	if postRespawnTarget.TargetVID != targetVID || postRespawnTarget.HPPercent != 90 {
		t.Fatalf("unexpected post-respawn target-refresh packet: %+v", postRespawnTarget)
	}
}

func TestGameSessionFlowContentSpawnGroupPracticeMobRespawnsAfterServerDrivenDelayAndRequiresFreshReselect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000350, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 || actors[0].Name != "PracticeMobAlpha" || actors[0].SpawnGroupRef != "practice.mob_alpha" || actors[0].CombatProfile != string(worldruntime.StaticActorCombatProfileTrainingDummy) {
		t.Fatalf("unexpected runtime spawn-group actors after import: %#v", actors)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	targetVID := uint32(actors[0].EntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected combat target error before content respawn slice: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame before content respawn slice, got %d", len(selectOut))
	}

	for attackIndex := 0; attackIndex < 9; attackIndex++ {
		if attackIndex > 0 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected combat attack error on content practice-mob pre-death hit %d: %v", attackIndex+1, err)
		}
		if len(attackOut) != 2 {
			t.Fatalf("expected target-refresh plus self-only retaliation point-loss frames on content practice-mob pre-death hit %d, got %d", attackIndex+1, len(attackOut))
		}
	}

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	finalAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error on content practice-mob death hit: %v", err)
	}
	if len(finalAttack) != 2 {
		t.Fatalf("expected 2 self-only death-transition frames before content practice-mob respawn, got %d", len(finalAttack))
	}
	death, err := worldproto.DecodeDead(decodeSingleFrame(t, finalAttack[0]))
	if err != nil {
		t.Fatalf("decode content practice-mob death frame: %v", err)
	}
	if death.VID != targetVID {
		t.Fatalf("unexpected content practice-mob death packet: %+v", death)
	}
	cleared, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, finalAttack[1]))
	if err != nil {
		t.Fatalf("decode content practice-mob clear-target frame: %v", err)
	}
	if cleared.TargetVID != 0 || cleared.HPPercent != 0 {
		t.Fatalf("unexpected content practice-mob clear-target packet on death hit: %+v", cleared)
	}

	currentTime = currentTime.Add((2 * time.Second) - time.Millisecond)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued content practice-mob respawn frames before the server-driven delay expires, got %d", len(queued))
	}

	currentTime = currentTime.Add(time.Millisecond)
	respawnFrames := flushServerFrames(t, flow)
	if len(respawnFrames) != 4 {
		t.Fatalf("expected delete + add/info/update respawn rebuild for content practice mob after server-driven delay, got %d frames", len(respawnFrames))
	}
	deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, respawnFrames[0]))
	if err != nil {
		t.Fatalf("decode content practice-mob respawn delete frame: %v", err)
	}
	if deleted.VID != targetVID {
		t.Fatalf("unexpected content practice-mob respawn delete frame: %+v", deleted)
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, respawnFrames[1]))
	if err != nil {
		t.Fatalf("decode content practice-mob respawn add frame: %v", err)
	}
	if added.VID != targetVID || added.X != 1200 || added.Y != 2200 || added.RaceNum != 101 {
		t.Fatalf("unexpected content practice-mob respawn add frame: %+v", added)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, respawnFrames[2]))
	if err != nil {
		t.Fatalf("decode content practice-mob respawn additional info frame: %v", err)
	}
	if info.VID != targetVID || info.Name != "PracticeMobAlpha" {
		t.Fatalf("unexpected content practice-mob respawn additional info frame: %+v", info)
	}
	updated, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, respawnFrames[3]))
	if err != nil {
		t.Fatalf("decode content practice-mob respawn update frame: %v", err)
	}
	if updated.VID != targetVID {
		t.Fatalf("unexpected content practice-mob respawn update frame: %+v", updated)
	}

	attackWithoutReselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected post-respawn content practice-mob attack-without-reselect error: %v", err)
	}
	if len(attackWithoutReselectOut) != 0 {
		t.Fatalf("expected post-respawn content practice-mob attack without fresh target selection to fail closed, got %d frames", len(attackWithoutReselectOut))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected post-respawn content practice-mob reselect error: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after content practice-mob respawn reselection, got %d", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode post-respawn content practice-mob target frame: %v", err)
	}
	if reselected.TargetVID != targetVID || reselected.HPPercent != 100 {
		t.Fatalf("unexpected post-respawn content practice-mob target packet: %+v", reselected)
	}
}

func TestGameSessionFlowPracticeMobAggroLiteRejectsFreshThirdPartyTargetAfterFirstAcceptedHit(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flowOne, enterOne := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOne) != 8 {
		t.Fatalf("expected 8 bootstrap frames for first player with visible content practice mob, got %d", len(enterOne))
	}
	defer closeSessionFlow(t, flowOne)
	flowTwo, enterTwo := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(enterTwo) != 11 {
		t.Fatalf("expected 11 bootstrap frames for second player with visible peer and content practice mob, got %d", len(enterTwo))
	}
	defer closeSessionFlow(t, flowTwo)
	if queued := flushServerFrames(t, flowOne); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for first player after second player joins, got %d", len(queued))
	}

	selectOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before first aggro-lite hit: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before first aggro-lite hit, got %d", len(selectOut))
	}

	attackOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first accepted aggro-lite attack error: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected target-refresh plus self-only retaliation point-loss frames on first aggro-lite hit, got %d", len(attackOut))
	}

	thirdPartyTarget, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected third-party target-selection error after first aggro-lite hit: %v", err)
	}
	if len(thirdPartyTarget) != 0 {
		t.Fatalf("expected fresh third-party target selection to fail closed once the content practice mob is engaged, got %d frames", len(thirdPartyTarget))
	}
}

func TestGameSessionFlowPracticeMobFirstHostileRetaliationAppliesSelfOnlyPointLossOnAcceptedOwnerHit(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	if actors[0].SpawnGroupRef != "practice.mob_alpha" || actors[0].CombatProfile != worldruntime.StaticActorCombatProfileTrainingDummy {
		t.Fatalf("expected imported practice-mob actor metadata to preserve spawn_group_ref + combat_profile, got %+v", actors[0])
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before first hostile retaliation hit: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before first hostile retaliation hit, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first hostile retaliation attack error: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected target-refresh plus self-only point-loss retaliation on first hostile practice-mob hit, got %d frames", len(attackOut))
	}
	refreshed, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode first hostile retaliation target-refresh frame: %v", err)
	}
	if refreshed.TargetVID != targetVID || refreshed.HPPercent != 90 {
		t.Fatalf("unexpected first hostile retaliation target-refresh packet: %+v", refreshed)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode first hostile retaliation point-change frame: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != -1 || pointChange.Value != owner.Points[bootstrapPlayerPointValueIndex]-1 {
		t.Fatalf("unexpected first hostile retaliation point-change packet: %+v", pointChange)
	}
}

func TestGameSessionFlowPracticeMobQueuesDelayedServerOriginRetaliationBeatAfterFirstAcceptedOwnerHit(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before delayed retaliation beat: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before delayed retaliation beat, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before delayed retaliation beat: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation on first practice-mob hit, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued delayed retaliation beat before the owned delay expires, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected exactly 1 queued delayed retaliation beat after the owned delay, got %d frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode queued delayed retaliation beat: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != -1 || pointChange.Value != owner.Points[bootstrapPlayerPointValueIndex]-2 {
		t.Fatalf("unexpected queued delayed retaliation point-change packet: %+v", pointChange)
	}
}

func TestGameSessionFlowPracticeMobRetaliationStopsAtOwnerHPFloorAfterImmediateBeat(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before owner-HP-floor retaliation test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before owner-HP-floor retaliation test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before owner-HP-floor retaliation test: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate target-refresh, self-only point-loss retaliation, self dead, and clear-target on owner-HP-floor hit, got %d frames", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change at owner HP floor: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != -1 || pointChange.Value != 0 {
		t.Fatalf("unexpected immediate retaliation point-change packet at owner HP floor: %+v", pointChange)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop once the immediate beat reached owner HP floor, got %d queued frames", len(queued))
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationStopsAtOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before delayed owner-HP-floor retaliation test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before delayed owner-HP-floor retaliation test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before delayed owner-HP-floor retaliation test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before delayed owner-HP-floor beat, got %d frames", len(attackOut))
	}
	firstPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before delayed owner-HP-floor beat: %v", err)
	}
	if firstPointChange.VID != owner.VID || firstPointChange.Type != bootstrapPlayerPointType || firstPointChange.Amount != -1 || firstPointChange.Value != 1 {
		t.Fatalf("unexpected immediate retaliation point-change packet before delayed owner-HP-floor beat: %+v", firstPointChange)
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected exactly 1 queued delayed retaliation beat plus self dead and clear-target frames before the owner HP floor is reached, got %d frames", len(queued))
	}
	secondPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change at owner HP floor: %v", err)
	}
	if secondPointChange.VID != owner.VID || secondPointChange.Type != bootstrapPlayerPointType || secondPointChange.Amount != -1 || secondPointChange.Value != 0 {
		t.Fatalf("unexpected delayed retaliation point-change packet at owner HP floor: %+v", secondPointChange)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop once a queued beat reached owner HP floor, got %d queued frames", len(queued))
	}
}

func TestGameSessionFlowPracticeMobImmediateRetaliationPointLossStaysRuntimeOnly(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	if err := accounts.Save(accountstore.Account{Login: "peer-one", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed owner account before immediate retaliation runtime-only test: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before immediate retaliation runtime-only test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before immediate retaliation runtime-only test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before immediate retaliation runtime-only test: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate target-refresh, self-only point-loss retaliation, self dead, and clear-target frames before immediate retaliation runtime-only test, got %d", len(attackOut))
	}

	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after immediate retaliation floor: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after immediate retaliation floor, got %+v", persisted)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected immediate retaliation point-loss to stay runtime-only with persisted points[%d] still %d, got %d", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationPhaseSelectReentryRebuildsPersistedPointsAndKeepsLiveMobHP(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	if err := accounts.Save(accountstore.Account{Login: "peer-one", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed owner account before delayed retaliation /phase_select recovery test: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before delayed retaliation /phase_select recovery test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before delayed retaliation /phase_select recovery test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before delayed retaliation /phase_select recovery test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before delayed retaliation /phase_select recovery test, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation beat plus self dead and clear-target frames before /phase_select recovery test, got %d queued frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before /phase_select recovery test: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before /phase_select recovery test, got %+v", pointChange)
	}

	phaseSelectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/phase_select"})))
	if err != nil {
		t.Fatalf("unexpected /phase_select error after delayed retaliation floor: %v", err)
	}
	if len(phaseSelectOut) != 1 {
		t.Fatalf("expected 1 /phase_select frame after delayed retaliation floor, got %d", len(phaseSelectOut))
	}

	selectPhaseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0})))
	if err != nil {
		t.Fatalf("unexpected character select error after delayed retaliation /phase_select recovery: %v", err)
	}
	if len(selectPhaseOut) != 3 {
		t.Fatalf("expected 3 character-select frames after delayed retaliation /phase_select recovery, got %d", len(selectPhaseOut))
	}

	reenterOut, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected enter-game error after delayed retaliation /phase_select recovery: %v", err)
	}
	if len(reenterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames after delayed retaliation /phase_select recovery, got %d", len(reenterOut))
	}
	reenterPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reenterOut[4]))
	if err != nil {
		t.Fatalf("decode bootstrap point-change after delayed retaliation /phase_select recovery: %v", err)
	}
	if reenterPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected /phase_select re-entry bootstrap to rebuild persisted points[%d] value %d after delayed retaliation floor, got %+v", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], reenterPointChange)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames immediately after delayed retaliation /phase_select re-entry, got %d", len(queued))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error after delayed retaliation /phase_select recovery: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame after delayed retaliation /phase_select recovery, got %d", len(reselectOut))
	}
	reselectedTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode target ack after delayed retaliation /phase_select recovery: %v", err)
	}
	if reselectedTarget.TargetVID != targetVID || reselectedTarget.HPPercent != 90 {
		t.Fatalf("expected /phase_select recovery to keep the same live practice mob at runtime-owned HP 90, got %+v", reselectedTarget)
	}

	attackAfterRecovery, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error after delayed retaliation /phase_select recovery: %v", err)
	}
	if len(attackAfterRecovery) != 2 {
		t.Fatalf("expected target-refresh plus self-only retaliation point-loss after delayed retaliation /phase_select recovery, got %d frames", len(attackAfterRecovery))
	}
	recoveryTargetRefresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackAfterRecovery[0]))
	if err != nil {
		t.Fatalf("decode target-refresh after delayed retaliation /phase_select recovery: %v", err)
	}
	if recoveryTargetRefresh.TargetVID != targetVID || recoveryTargetRefresh.HPPercent != 80 {
		t.Fatalf("expected first accepted attack after delayed retaliation /phase_select recovery to keep stepping live mob HP to 80, got %+v", recoveryTargetRefresh)
	}
	recoveryPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackAfterRecovery[1]))
	if err != nil {
		t.Fatalf("decode retaliation point-change after delayed retaliation /phase_select recovery: %v", err)
	}
	if recoveryPointChange.VID != owner.VID || recoveryPointChange.Type != bootstrapPlayerPointType || recoveryPointChange.Amount != -1 || recoveryPointChange.Value != 1 {
		t.Fatalf("unexpected self-only retaliation point-loss after delayed retaliation /phase_select recovery: %+v", recoveryPointChange)
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationPointLossStaysRuntimeOnlyAcrossReconnect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	if err := accounts.Save(accountstore.Account{Login: "peer-one", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed owner account before delayed retaliation reconnect test: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	flow, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before delayed retaliation reconnect test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before delayed retaliation reconnect test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before delayed retaliation reconnect test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before delayed retaliation reconnect test, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation beat plus self dead and clear-target frames before reconnect test, got %d queued frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before reconnect test: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before reconnect test, got %+v", pointChange)
	}

	closeSessionFlow(t, flow)
	issuePeerTicket(t, store, "peer-one", 0x22222222, owner)

	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x22222222)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for reconnecting owner with visible content practice mob, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after delayed retaliation floor: %v", err)
	}
	if reconnectPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected reconnect bootstrap to rebuild persisted points[%d] value %d after delayed retaliation floor, got %+v", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], reconnectPointChange)
	}

	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after delayed retaliation reconnect test: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after delayed retaliation reconnect test, got %+v", persisted)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected delayed retaliation point-loss to stay runtime-only with persisted points[%d] still %d, got %d", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowPracticeMobRetaliationPointLossStaysRuntimeOnlyAcrossPersistedMove(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 3
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	if err := accounts.Save(accountstore.Account{Login: "peer-one", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed owner account before persisted-move retaliation test: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000460, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	flow, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before persisted-move retaliation test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before persisted-move retaliation test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before persisted-move retaliation test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before persisted-move retaliation test, got %d frames", len(attackOut))
	}
	immediatePointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before persisted-move retaliation test: %v", err)
	}
	if immediatePointChange.Value != 2 {
		t.Fatalf("expected immediate retaliation to drop live owner points to 2 before persisted move, got %+v", immediatePointChange)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1110, Y: 2110, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected move error after immediate retaliation: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move-ack frame after persisted move, got %d", len(moveOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep the live owner at runtime-only points after persisted move, got %d queued frames", len(queued))
	}
	delayedPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change after persisted move: %v", err)
	}
	if delayedPointChange.Value != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points at runtime-only value 1 after persisted move, got %+v", delayedPointChange)
	}

	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after persisted-move retaliation test: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after persisted-move retaliation test, got %+v", persisted)
	}
	if persisted.Characters[0].X != 1110 || persisted.Characters[0].Y != 2110 {
		t.Fatalf("expected persisted move to save updated coordinates (1110,2110), got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected persisted move to keep pre-retaliation points[%d] value %d, got %d", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	closeSessionFlow(t, flow)
	issuePeerTicket(t, store, "peer-one", 0x22222222, owner)
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x22222222)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames after persisted move reconnect, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after persisted move: %v", err)
	}
	if reconnectPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected reconnect bootstrap after persisted move to rebuild pre-retaliation points[%d] value %d, got %+v", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], reconnectPointChange)
	}
}

func TestGameSessionFlowPracticeMobImmediateRetaliationPointLossStaysRuntimeOnlyAcrossTransferRebootstrap(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	if err := accounts.Save(accountstore.Account{Login: "peer-one", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed owner account before transfer rebootstrap retaliation test: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, []bootstrapTransferTrigger{{
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
	currentTime := time.Unix(1700000470, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	flow, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before transfer rebootstrap retaliation test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before transfer rebootstrap retaliation test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before transfer rebootstrap retaliation test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before transfer rebootstrap retaliation test, got %d frames", len(attackOut))
	}
	immediatePointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before transfer rebootstrap retaliation test: %v", err)
	}
	if immediatePointChange.Value != 1 {
		t.Fatalf("expected immediate retaliation to drop live owner points to 1 before transfer rebootstrap, got %+v", immediatePointChange)
	}

	transferOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222326})))
	if err != nil {
		t.Fatalf("unexpected transfer move error after immediate retaliation: %v", err)
	}
	if len(transferOut) == 0 {
		t.Fatal("expected transfer rebootstrap frames after immediate retaliation")
	}
	foundTransferPointChange := false
	for _, raw := range transferOut {
		decoded, decodeErr := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, raw))
		if decodeErr != nil {
			continue
		}
		if decoded.VID != owner.VID {
			continue
		}
		foundTransferPointChange = true
		if decoded.Value != 1 {
			t.Fatalf("expected transfer rebootstrap self point-change to keep live owner points at runtime-only value 1, got %+v", decoded)
		}
		break
	}
	if !foundTransferPointChange {
		t.Fatal("expected transfer rebootstrap to include a self bootstrap point-change frame for the owner")
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected transfer rebootstrap to clear delayed retaliation cadence after immediate retaliation, got %d queued frames", len(queued))
	}

	persisted, err := accounts.Load("peer-one")
	if err != nil {
		t.Fatalf("load persisted account after transfer rebootstrap retaliation test: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after transfer rebootstrap retaliation test, got %+v", persisted)
	}
	if persisted.Characters[0].MapIndex != 42 || persisted.Characters[0].X != 1700 || persisted.Characters[0].Y != 2800 {
		t.Fatalf("expected transfer rebootstrap to persist destination coordinates, got %+v", persisted.Characters[0])
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected transfer rebootstrap to keep pre-retaliation points[%d] value %d, got %d", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	closeSessionFlow(t, flow)
	issuePeerTicket(t, store, "peer-one", 0x22222222, owner)
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x22222222)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) != 5 {
		t.Fatalf("expected 5 bootstrap frames after transfer rebootstrap reconnect on destination map, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after transfer rebootstrap: %v", err)
	}
	if reconnectPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected reconnect bootstrap after transfer rebootstrap to rebuild pre-retaliation points[%d] value %d, got %+v", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], reconnectPointChange)
	}
}

func TestGameSessionFlowPracticeMobAttackFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner attack denial after immediate retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before zero-HP owner attack denial after immediate retaliation, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before zero-HP owner attack denial after immediate retaliation: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate target-refresh, self-only point-loss retaliation, self dead, and clear-target frames before zero-HP owner attack denial after immediate retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop once the immediate beat reached owner HP floor before zero-HP owner attack denial, got %d queued frames", len(queued))
	}

	repeatedAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected repeated attack error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(repeatedAttack) != 0 {
		t.Fatalf("expected same-owner normal attack to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(repeatedAttack))
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner attack denial after immediate retaliation not to re-arm delayed retaliation cadence, got %d queued frames", len(queued))
	}
}

func TestGameSessionFlowPracticeMobAttackFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner attack denial after delayed retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before zero-HP owner attack denial after delayed retaliation, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before zero-HP owner attack denial after delayed retaliation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before zero-HP owner attack denial after delayed retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected exactly 1 queued delayed retaliation beat plus self dead and clear-target frames before zero-HP owner attack denial after delayed retaliation, got %d frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before zero-HP owner attack denial after delayed retaliation: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before zero-HP owner attack denial, got %+v", pointChange)
	}

	repeatedAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected repeated attack error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(repeatedAttack) != 0 {
		t.Fatalf("expected same-owner normal attack to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(repeatedAttack))
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner attack denial after delayed retaliation not to re-arm delayed retaliation cadence, got %d queued frames", len(queued))
	}
}

func TestGameSessionFlowPracticeMobTargetFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected initial target-selection error before zero-HP owner target denial after immediate retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 initial self-only target frame before zero-HP owner target denial after immediate retaliation, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before zero-HP owner target denial after immediate retaliation: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate target-refresh, self-only point-loss retaliation, self dead, and clear-target frames before zero-HP owner target denial after immediate retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop once the immediate beat reached owner HP floor before zero-HP owner target denial, got %d queued frames", len(queued))
	}

	repeatedTarget, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected repeated target-selection error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(repeatedTarget) != 0 {
		t.Fatalf("expected combat target attempt to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(repeatedTarget))
	}
}

func TestGameSessionFlowPracticeMobTargetFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected initial target-selection error before zero-HP owner target denial after delayed retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 initial self-only target frame before zero-HP owner target denial after delayed retaliation, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before zero-HP owner target denial after delayed retaliation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before zero-HP owner target denial after delayed retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected exactly 1 queued delayed retaliation beat plus self dead and clear-target frame before zero-HP owner target denial after delayed retaliation, got %d frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before zero-HP owner target denial after delayed retaliation: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before zero-HP owner target denial, got %+v", pointChange)
	}

	repeatedTarget, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected repeated target-selection error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(repeatedTarget) != 0 {
		t.Fatalf("expected combat target attempt to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(repeatedTarget))
	}
}

func TestGameSessionFlowPracticeMobThirdPartyCanRetargetAfterImmediateRetaliationKillsOwner(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected owner combat target error before immediate-retaliation aggro release: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 owner target-selection frame before immediate-retaliation aggro release, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected owner attack error before immediate-retaliation aggro release: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-loss retaliation, self dead, and clear-target frames before immediate-retaliation aggro release, got %d frames", len(attackOut))
	}

	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected immediate owner death to queue 1 visible-peer DEAD frame before aggro release, got %d", len(watcherQueued))
	}

	watcherTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher combat target error after immediate retaliation killed owner: %v", err)
	}
	if len(watcherTargetOut) != 1 {
		t.Fatalf("expected watcher fresh target to succeed after immediate retaliation killed the engaged owner, got %d frames", len(watcherTargetOut))
	}
	watcherTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, watcherTargetOut[0]))
	if err != nil {
		t.Fatalf("decode watcher target frame after immediate retaliation killed owner: %v", err)
	}
	if watcherTarget.TargetVID != targetVID || watcherTarget.HPPercent != 90 {
		t.Fatalf("unexpected watcher target packet after immediate retaliation killed owner: %+v", watcherTarget)
	}

	watcherAttackOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected watcher attack error after immediate retaliation killed owner: %v", err)
	}
	if len(watcherAttackOut) != 2 {
		t.Fatalf("expected watcher retargeted hit to refresh mob HP and append self-only retaliation after immediate owner death, got %d frames", len(watcherAttackOut))
	}
	watcherRefresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, watcherAttackOut[0]))
	if err != nil {
		t.Fatalf("decode watcher attack refresh after immediate retaliation killed owner: %v", err)
	}
	if watcherRefresh.TargetVID != targetVID || watcherRefresh.HPPercent != 80 {
		t.Fatalf("unexpected watcher attack refresh after immediate retaliation killed owner: %+v", watcherRefresh)
	}
}

func TestGameSessionFlowPracticeMobThirdPartyCanRetargetAfterDelayedRetaliationKillsOwner(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected owner combat target error before delayed-retaliation aggro release: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 owner target-selection frame before delayed-retaliation aggro release, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected owner attack error before delayed-retaliation aggro release: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected target refresh plus first point-loss retaliation before delayed-retaliation aggro release, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames before delayed-retaliation aggro release, got %d frames", len(queued))
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected delayed owner death to queue 1 visible-peer DEAD frame before aggro release, got %d", len(watcherQueued))
	}

	watcherTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher combat target error after delayed retaliation killed owner: %v", err)
	}
	if len(watcherTargetOut) != 1 {
		t.Fatalf("expected watcher fresh target to succeed after delayed retaliation killed the engaged owner, got %d frames", len(watcherTargetOut))
	}
	watcherTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, watcherTargetOut[0]))
	if err != nil {
		t.Fatalf("decode watcher target frame after delayed retaliation killed owner: %v", err)
	}
	if watcherTarget.TargetVID != targetVID || watcherTarget.HPPercent != 90 {
		t.Fatalf("unexpected watcher target packet after delayed retaliation killed owner: %+v", watcherTarget)
	}

	watcherAttackOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected watcher attack error after delayed retaliation killed owner: %v", err)
	}
	if len(watcherAttackOut) != 2 {
		t.Fatalf("expected watcher retargeted hit to refresh mob HP and append self-only retaliation after delayed owner death, got %d frames", len(watcherAttackOut))
	}
	watcherRefresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, watcherAttackOut[0]))
	if err != nil {
		t.Fatalf("decode watcher attack refresh after delayed retaliation killed owner: %v", err)
	}
	if watcherRefresh.TargetVID != targetVID || watcherRefresh.HPPercent != 80 {
		t.Fatalf("unexpected watcher attack refresh after delayed retaliation killed owner: %+v", watcherRefresh)
	}
}

func TestGameSessionFlowPracticeMobMoveFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        3100,
		TargetY:        4200,
	}})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner move denial after immediate retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner move denial after immediate retaliation, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner move denial after immediate retaliation: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-loss retaliation, self dead, and clear-target frames before zero-HP owner move denial after immediate retaliation, got %d frames", len(attackOut))
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected immediate retaliation reaching owner HP floor to queue 1 visible-peer DEAD frame before zero-HP owner move denial, got %d", len(watcherQueued))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode visible-peer dead frame before zero-HP owner move denial after immediate retaliation: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected visible-peer DEAD(owner_vid) before zero-HP owner move denial after immediate retaliation, got %+v", dead)
	}

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected owner MOVE to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(moveOut))
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner move denial after immediate retaliation not to queue peer relocation frames, got %d", len(queued))
	}

	connected := runtime.ConnectedCharacters()
	foundOwner := false
	for _, snapshot := range connected {
		if snapshot.Name != owner.Name {
			continue
		}
		foundOwner = true
		if snapshot.MapIndex != owner.MapIndex || snapshot.X != owner.X || snapshot.Y != owner.Y {
			t.Fatalf("expected zero-HP owner move denial after immediate retaliation to keep the live character at the pre-death coordinates, got %+v", snapshot)
		}
	}
	if !foundOwner {
		t.Fatalf("expected owner snapshot to remain connected after zero-HP owner move denial after immediate retaliation, got %#v", connected)
	}
}

func TestGameSessionFlowPracticeMobSyncPositionFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner sync-position denial after delayed retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner sync-position denial after delayed retaliation, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner sync-position denial after delayed retaliation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected target refresh plus first point-loss retaliation before zero-HP owner sync-position denial after delayed retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames before zero-HP owner sync-position denial after delayed retaliation, got %d frames", len(queued))
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected delayed retaliation reaching owner HP floor to queue 1 visible-peer DEAD frame before zero-HP owner sync-position denial, got %d", len(watcherQueued))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode visible-peer dead frame before zero-HP owner sync-position denial after delayed retaliation: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected visible-peer DEAD(owner_vid) before zero-HP owner sync-position denial after delayed retaliation, got %+v", dead)
	}

	syncOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: owner.VID, X: 1500, Y: 2600}},
	})))
	if err != nil {
		t.Fatalf("unexpected sync-position error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(syncOut) != 0 {
		t.Fatalf("expected owner SYNC_POSITION to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(syncOut))
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner sync-position denial after delayed retaliation not to queue peer relocation frames, got %d", len(queued))
	}

	connected := runtime.ConnectedCharacters()
	foundOwner := false
	for _, snapshot := range connected {
		if snapshot.Name != owner.Name {
			continue
		}
		foundOwner = true
		if snapshot.MapIndex != owner.MapIndex || snapshot.X != owner.X || snapshot.Y != owner.Y {
			t.Fatalf("expected zero-HP owner sync-position denial after delayed retaliation to keep the live character at the pre-death coordinates, got %+v", snapshot)
		}
	}
	if !foundOwner {
		t.Fatalf("expected owner snapshot to remain connected after zero-HP owner sync-position denial after delayed retaliation, got %#v", connected)
	}
}

func TestGameSessionFlowPracticeMobInteractionFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:     interactionstore.KindWarp,
		Ref:      "npc:teleporter",
		MapIndex: 42,
		X:        3100,
		Y:        4200,
		Text:     "Step through the gate.",
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	teleporter, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "Teleporter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindWarp, "npc:teleporter")
	if !ok {
		t.Fatal("expected warp static actor registration to succeed")
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 runtime static actors after practice-mob + teleporter setup, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)
	if targetVID == uint32(teleporter.EntityID) {
		targetVID = uint32(actors[1].EntityID)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 11 {
		t.Fatalf("expected 11 bootstrap frames for owner with visible teleporter and content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner interaction denial after immediate retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner interaction denial after immediate retaliation, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner interaction denial after immediate retaliation: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-loss retaliation, self dead, and clear-target frames before zero-HP owner interaction denial after immediate retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop once immediate retaliation reached owner HP floor before zero-HP owner interaction denial, got %d queued frames", len(queued))
	}

	interactOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(teleporter.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected interaction error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(interactOut) != 0 {
		t.Fatalf("expected owner INTERACT to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(interactOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner interaction denial after immediate retaliation not to queue transfer frames, got %d", len(queued))
	}

	connected := runtime.ConnectedCharacters()
	foundOwner := false
	for _, snapshot := range connected {
		if snapshot.Name != owner.Name {
			continue
		}
		foundOwner = true
		if snapshot.MapIndex != owner.MapIndex || snapshot.X != owner.X || snapshot.Y != owner.Y {
			t.Fatalf("expected zero-HP owner interaction denial after immediate retaliation to keep the live character at the pre-death coordinates, got %+v", snapshot)
		}
	}
	if !foundOwner {
		t.Fatalf("expected owner snapshot to remain connected after zero-HP owner interaction denial after immediate retaliation, got %#v", connected)
	}
}

func TestGameSessionFlowPracticeMobInteractionFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:     interactionstore.KindWarp,
		Ref:      "npc:teleporter",
		MapIndex: 42,
		X:        3100,
		Y:        4200,
		Text:     "Step through the gate.",
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	teleporter, ok := runtime.sharedWorld.RegisterStaticActorWithInteraction(0, "Teleporter", bootstrapMapIndex, 1250, 2250, 20301, interactionstore.KindWarp, "npc:teleporter")
	if !ok {
		t.Fatal("expected warp static actor registration to succeed")
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 runtime static actors after practice-mob + teleporter setup, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)
	if targetVID == uint32(teleporter.EntityID) {
		targetVID = uint32(actors[1].EntityID)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 11 {
		t.Fatalf("expected 11 bootstrap frames for owner with visible teleporter and content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner interaction denial after delayed retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner interaction denial after delayed retaliation, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner interaction denial after delayed retaliation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected target refresh plus first point-loss retaliation before zero-HP owner interaction denial after delayed retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames before zero-HP owner interaction denial after delayed retaliation, got %d frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before zero-HP owner interaction denial after delayed retaliation: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before zero-HP owner interaction denial after delayed retaliation, got %+v", pointChange)
	}

	interactOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(teleporter.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected interaction error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(interactOut) != 0 {
		t.Fatalf("expected owner INTERACT to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(interactOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner interaction denial after delayed retaliation not to queue transfer frames, got %d", len(queued))
	}

	connected := runtime.ConnectedCharacters()
	foundOwner := false
	for _, snapshot := range connected {
		if snapshot.Name != owner.Name {
			continue
		}
		foundOwner = true
		if snapshot.MapIndex != owner.MapIndex || snapshot.X != owner.X || snapshot.Y != owner.Y {
			t.Fatalf("expected zero-HP owner interaction denial after delayed retaliation to keep the live character at the pre-death coordinates, got %+v", snapshot)
		}
	}
	if !foundOwner {
		t.Fatalf("expected owner snapshot to remain connected after zero-HP owner interaction denial after delayed retaliation, got %#v", connected)
	}
}

func TestGameSessionFlowPracticeMobPacketShopBuyFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantOwnerImmediate", 0x01030110, 0x02040110, 125, nil)
	buyer.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "merchant-owner-immediate", 0x51515151, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-owner-immediate", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed immediate zero-HP merchant owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000440, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_alpha",
			Name:          "PracticeMobAlpha",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content merchant/practice-mob bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected merchant and practice-mob actors before zero-HP merchant packet-buy denial, got %d", len(actors))
	}
	merchantEntityID := uint64(0)
	practiceMobTargetVID := uint32(0)
	for _, actor := range actors {
		if actor.SpawnGroupRef == "practice.mob_alpha" {
			practiceMobTargetVID = uint32(actor.EntityID)
			continue
		}
		if actor.InteractionKind == interactionstore.KindShopPreview && actor.InteractionRef == "npc:merchant" {
			merchantEntityID = actor.EntityID
		}
	}
	if merchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected to find merchant and content-loaded practice mob before zero-HP merchant packet-buy denial, got %#v", actors)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-owner-immediate", 0x51515151)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 11 {
		t.Fatalf("expected merchant/practice-mob owner bootstrap to emit 11 frames, got %d", len(enterOut))
	}

	interactWithMerchantForBuy(t, flow, merchantEntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before zero-HP merchant packet-buy denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before zero-HP merchant packet-buy denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  practiceMobTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP merchant packet-buy denial: %v", err)
	}
	if len(attackOut) != 5 {
		t.Fatalf("expected immediate retaliation floor attack to emit 5 frames including merchant close before zero-HP merchant packet-buy denial, got %d", len(attackOut))
	}
	if !reflect.DeepEqual(attackOut[4], shopproto.EncodeServerEnd()) {
		t.Fatalf("expected immediate retaliation floor attack to append merchant close before zero-HP merchant packet-buy denial, got %#v", attackOut[4])
	}
	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate zero-HP merchant packet-buy denial setup, got %d", len(queued))
	}

	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop buy error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected packet SHOP BUY to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(buyOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected zero-HP merchant packet-buy denial not to queue frames, got %d", len(queued))
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after zero-HP merchant packet-buy denial")
	}
	if currencySnapshot.Gold != 125 {
		t.Fatalf("expected zero-HP merchant packet-buy denial to keep gold at 125, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after zero-HP merchant packet-buy denial")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected zero-HP merchant packet-buy denial to keep inventory unchanged, got %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load("merchant-owner-immediate")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP merchant owner account: %v", err)
	}
	if account.Characters[0].Gold != 125 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted immediate zero-HP merchant owner account to stay unchanged, got %#v", account.Characters[0])
	}
}

func TestGameSessionFlowPracticeMobSlashShopBuyFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantOwnerDelayed", 0x01030111, 0x02040111, 125, nil)
	buyer.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "merchant-owner-delayed", 0x52525252, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-owner-delayed", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed delayed zero-HP merchant owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_alpha",
			Name:          "PracticeMobAlpha",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content merchant/practice-mob bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected merchant and practice-mob actors before zero-HP merchant slash-buy denial, got %d", len(actors))
	}
	merchantEntityID := uint64(0)
	practiceMobTargetVID := uint32(0)
	for _, actor := range actors {
		if actor.SpawnGroupRef == "practice.mob_alpha" {
			practiceMobTargetVID = uint32(actor.EntityID)
			continue
		}
		if actor.InteractionKind == interactionstore.KindShopPreview && actor.InteractionRef == "npc:merchant" {
			merchantEntityID = actor.EntityID
		}
	}
	if merchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected to find merchant and content-loaded practice mob before zero-HP merchant slash-buy denial, got %#v", actors)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-owner-delayed", 0x52525252)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 11 {
		t.Fatalf("expected merchant/practice-mob owner bootstrap to emit 11 frames, got %d", len(enterOut))
	}

	interactWithMerchantForBuy(t, flow, merchantEntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before zero-HP merchant slash-buy denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before zero-HP merchant slash-buy denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  practiceMobTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP merchant slash-buy denial: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected delayed retaliation setup attack to emit 2 frames before zero-HP merchant slash-buy denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 4 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, clear-target, and merchant close frames before zero-HP merchant slash-buy denial, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before zero-HP merchant slash-buy denial: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before zero-HP merchant slash-buy denial, got %+v", pointChange)
	}
	if !reflect.DeepEqual(queued[3], shopproto.EncodeServerEnd()) {
		t.Fatalf("expected delayed retaliation floor to append merchant close before zero-HP merchant slash-buy denial, got %#v", queued[3])
	}

	slashBuyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/shop_buy 0"})))
	if err != nil {
		t.Fatalf("unexpected slash merchant buy error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(slashBuyOut) != 0 {
		t.Fatalf("expected /shop_buy to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(slashBuyOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected zero-HP merchant slash-buy denial not to queue frames, got %d", len(queued))
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after zero-HP merchant slash-buy denial")
	}
	if currencySnapshot.Gold != 125 {
		t.Fatalf("expected zero-HP merchant slash-buy denial to keep gold at 125, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after zero-HP merchant slash-buy denial")
	}
	if len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected zero-HP merchant slash-buy denial to keep inventory unchanged, got %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load("merchant-owner-delayed")
	if err != nil {
		t.Fatalf("load persisted delayed zero-HP merchant owner account: %v", err)
	}
	if account.Characters[0].Gold != 125 || len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted delayed zero-HP merchant owner account to stay unchanged, got %#v", account.Characters[0])
	}
}

func TestGameSessionFlowPracticeMobMerchantBuyKeepsRetaliationPointLossRuntimeOnlyWhilePersistingPurchase(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantOwnerRuntimeOnly", 0x01030119, 0x02040119, 125, nil)
	buyer.Points[bootstrapPlayerPointValueIndex] = 3
	issuePeerTicket(t, store, "merchant-owner-runtime-only", 0x53535353, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-owner-runtime-only", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed runtime-only merchant owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected merchant/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000515, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_alpha",
			Name:          "PracticeMobAlpha",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content merchant/practice-mob bundle for runtime-only merchant buy persistence: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected merchant and practice-mob actors before runtime-only merchant buy persistence, got %d", len(actors))
	}
	merchantEntityID := uint64(0)
	practiceMobTargetVID := uint32(0)
	for _, actor := range actors {
		if actor.SpawnGroupRef == "practice.mob_alpha" {
			practiceMobTargetVID = uint32(actor.EntityID)
			continue
		}
		if actor.InteractionKind == interactionstore.KindShopPreview && actor.InteractionRef == "npc:merchant" {
			merchantEntityID = actor.EntityID
		}
	}
	if merchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected to find merchant and content-loaded practice mob before runtime-only merchant buy persistence, got %#v", actors)
	}

	factory := runtime.SessionFactory()
	flow, enterOut := enterGameWithLoginTicket(t, factory, "merchant-owner-runtime-only", 0x53535353)
	if len(enterOut) != 11 {
		t.Fatalf("expected runtime-only merchant owner bootstrap to emit 11 frames, got %d", len(enterOut))
	}

	interactWithMerchantForBuy(t, flow, merchantEntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before runtime-only merchant buy persistence: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before runtime-only merchant buy persistence, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error before runtime-only merchant buy persistence: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate retaliation setup attack to emit 2 frames before runtime-only merchant buy persistence, got %d", len(attackOut))
	}
	immediatePointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before runtime-only merchant buy persistence: %v", err)
	}
	if immediatePointChange.Value != 2 {
		t.Fatalf("expected immediate retaliation to drop live owner points to 2 before runtime-only merchant buy persistence, got %+v", immediatePointChange)
	}

	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/shop_buy 0"})))
	if err != nil {
		t.Fatalf("unexpected slash merchant buy error before runtime-only merchant buy persistence: %v", err)
	}
	if len(buyOut) == 0 {
		t.Fatal("expected runtime-only merchant buy persistence to emit at least one success frame")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points runtime-only after merchant buy, got %d queued frames", len(queued))
	}
	delayedPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change after runtime-only merchant buy persistence: %v", err)
	}
	if delayedPointChange.Value != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points at runtime-only value 1 after merchant buy, got %+v", delayedPointChange)
	}

	currencySnapshot, ok := runtime.CurrencySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected currency snapshot after runtime-only merchant buy persistence")
	}
	if currencySnapshot.Gold != 75 {
		t.Fatalf("expected merchant buy to debit runtime gold to 75 while preserving runtime-only retaliation loss, got %+v", currencySnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(buyer.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after runtime-only merchant buy persistence")
	}
	if len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected merchant buy to add one runtime inventory item at slot 0, got %+v", inventorySnapshot.Inventory)
	}
	account, err := accounts.Load("merchant-owner-runtime-only")
	if err != nil {
		t.Fatalf("load persisted runtime-only merchant owner account: %v", err)
	}
	if account.Characters[0].Gold != 75 {
		t.Fatalf("expected persisted merchant buyer gold 75 after runtime-only merchant buy persistence, got %d", account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 27001 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("unexpected persisted merchant buyer inventory after runtime-only merchant buy persistence: %#v", account.Characters[0].Inventory)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != buyer.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected merchant buy persistence to keep pre-retaliation points[%d] value %d, got %d", bootstrapPlayerPointValueIndex, buyer.Points[bootstrapPlayerPointValueIndex], account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	closeSessionFlow(t, flow)
	issuePeerTicket(t, store, "merchant-owner-runtime-only", 0x63636363, buyer)
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "merchant-owner-runtime-only", 0x63636363)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) != 12 {
		t.Fatalf("expected runtime-only merchant buy reconnect bootstrap to emit 12 frames, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after runtime-only merchant buy persistence: %v", err)
	}
	if reconnectPointChange.Value != buyer.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected reconnect bootstrap after merchant buy to rebuild pre-retaliation points[%d] value %d, got %+v", bootstrapPlayerPointValueIndex, buyer.Points[bootstrapPlayerPointValueIndex], reconnectPointChange)
	}
}

func TestGameSessionFlowPracticeMobMerchantWindowClosesAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantOwnerCloseImmediate", 0x01030111, 0x02040111, 125, nil)
	buyer.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "merchant-owner-close-immediate", 0x5A5A5A5A, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-owner-close-immediate", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed immediate zero-HP merchant close owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000520, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_alpha",
			Name:          "PracticeMobAlpha",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content merchant/practice-mob bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected merchant and practice-mob actors before zero-HP merchant close, got %d", len(actors))
	}
	merchantEntityID := uint64(0)
	practiceMobTargetVID := uint32(0)
	for _, actor := range actors {
		if actor.SpawnGroupRef == "practice.mob_alpha" {
			practiceMobTargetVID = uint32(actor.EntityID)
			continue
		}
		if actor.InteractionKind == interactionstore.KindShopPreview && actor.InteractionRef == "npc:merchant" {
			merchantEntityID = actor.EntityID
		}
	}
	if merchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected to find merchant and content-loaded practice mob before zero-HP merchant close, got %#v", actors)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-owner-close-immediate", 0x5A5A5A5A)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 11 {
		t.Fatalf("expected merchant/practice-mob owner bootstrap to emit 11 frames, got %d", len(enterOut))
	}

	interactWithMerchantForBuy(t, flow, merchantEntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before immediate zero-HP merchant close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before immediate zero-HP merchant close, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  practiceMobTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate zero-HP merchant close: %v", err)
	}
	if len(attackOut) != 5 {
		t.Fatalf("expected immediate retaliation floor attack to emit 5 frames including merchant close, got %d", len(attackOut))
	}
	if !reflect.DeepEqual(attackOut[4], shopproto.EncodeServerEnd()) {
		t.Fatalf("expected immediate retaliation floor attack to append merchant close, got %#v", attackOut[4])
	}
	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate zero-HP merchant close, got %d", len(queued))
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected client SHOP END error after immediate zero-HP merchant close: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected client SHOP END to fail closed after immediate zero-HP merchant close already cleared context, got %d frames", len(closeOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected immediate zero-HP merchant close not to queue follow-up frames, got %d", len(queued))
	}
}

func TestGameSessionFlowPracticeMobMerchantWindowClosesAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantOwnerCloseDelayed", 0x01030111, 0x02040111, 125, nil)
	buyer.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "merchant-owner-close-delayed", 0x5B5B5B5B, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-owner-close-delayed", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed delayed zero-HP merchant close owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000530, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{
		StaticActors: []contentbundle.StaticActor{{
			Name:            "Merchant",
			MapIndex:        bootstrapMapIndex,
			X:               1250,
			Y:               2250,
			RaceNum:         20300,
			InteractionKind: interactionstore.KindShopPreview,
			InteractionRef:  "npc:merchant",
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.mob_alpha",
			Name:          "PracticeMobAlpha",
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
		}},
		InteractionDefinitions: []interactionstore.Definition{defaultMerchantCatalogDefinition()},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content merchant/practice-mob bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected merchant and practice-mob actors before delayed zero-HP merchant close, got %d", len(actors))
	}
	merchantEntityID := uint64(0)
	practiceMobTargetVID := uint32(0)
	for _, actor := range actors {
		if actor.SpawnGroupRef == "practice.mob_alpha" {
			practiceMobTargetVID = uint32(actor.EntityID)
			continue
		}
		if actor.InteractionKind == interactionstore.KindShopPreview && actor.InteractionRef == "npc:merchant" {
			merchantEntityID = actor.EntityID
		}
	}
	if merchantEntityID == 0 || practiceMobTargetVID == 0 {
		t.Fatalf("expected to find merchant and content-loaded practice mob before delayed zero-HP merchant close, got %#v", actors)
	}

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-owner-close-delayed", 0x5B5B5B5B)
	defer closeSessionFlow(t, flow)
	if len(enterOut) != 11 {
		t.Fatalf("expected merchant/practice-mob owner bootstrap to emit 11 frames, got %d", len(enterOut))
	}

	interactWithMerchantForBuy(t, flow, merchantEntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before delayed zero-HP merchant close: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before delayed zero-HP merchant close, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  practiceMobTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before delayed zero-HP merchant close: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected delayed retaliation setup attack to emit 2 frames before delayed zero-HP merchant close, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 4 {
		t.Fatalf("expected delayed retaliation floor to emit point-loss, self dead, clear-target, and merchant close frames, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before zero-HP merchant close: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before zero-HP merchant close, got %+v", pointChange)
	}
	if !reflect.DeepEqual(queued[3], shopproto.EncodeServerEnd()) {
		t.Fatalf("expected delayed retaliation floor to append merchant close, got %#v", queued[3])
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected client SHOP END error after delayed zero-HP merchant close: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected client SHOP END to fail closed after delayed zero-HP merchant close already cleared context, got %d frames", len(closeOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed zero-HP merchant close not to queue follow-up frames, got %d", len(queued))
	}
}

func TestGameSessionFlowPracticeMobUseItemFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ItemOwnerImmediate", 0x01030112, 0x02040112, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 3, Slot: 5}}
	owner.Equipment = []inventory.ItemInstance{}
	issuePeerTicket(t, store, "item-owner-immediate", 0x53535353, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-owner-immediate", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed immediate zero-HP item-use owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  7,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 25,
			Message:    "consume:27002:+25",
		},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000460, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for immediate zero-HP item-use denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before immediate zero-HP item-use denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-owner-immediate", 0x53535353)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected item-use/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before immediate zero-HP item-use denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before immediate zero-HP item-use denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate zero-HP item-use denial: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate retaliation floor attack to emit 4 frames before immediate zero-HP item-use denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate zero-HP item-use denial setup, got %d", len(queued))
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected slash item-use error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(useOut) != 0 {
		t.Fatalf("expected /use_item to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(useOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected immediate zero-HP item-use denial not to queue frames, got %d", len(queued))
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after immediate zero-HP item-use denial")
	}
	if len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].ID != 1001 || inventorySnapshot.Inventory[0].Vnum != 27002 || inventorySnapshot.Inventory[0].Count != 3 || inventorySnapshot.Inventory[0].Slot != 5 {
		t.Fatalf("expected immediate zero-HP item-use denial to keep runtime inventory unchanged, got %#v", inventorySnapshot.Inventory)
	}
	persisted, err := accounts.Load("item-owner-immediate")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP item-use owner account: %v", err)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected persisted immediate zero-HP item-use owner points to stay at pre-retaliation value %d, got %d", owner.Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 3, Slot: 5}}) {
		t.Fatalf("expected persisted immediate zero-HP item-use owner inventory to stay unchanged, got %#v", persisted.Characters[0].Inventory)
	}
}

func TestGameSessionFlowPracticeMobItemUsePacketFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ItemPacketOwnerImmediate", 0x01030192, 0x02040192, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 3, Slot: 5}}
	owner.Equipment = []inventory.ItemInstance{}
	issuePeerTicket(t, store, "item-packet-owner-immediate", 0x92929292, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-packet-owner-immediate", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed immediate zero-HP packet item-use owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  7,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 25,
			Message:    "consume:27002:+25",
		},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected packet item-use/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000560, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for immediate zero-HP packet item-use denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before immediate zero-HP packet item-use denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-packet-owner-immediate", 0x92929292)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected packet item-use/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before immediate zero-HP packet item-use denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before immediate zero-HP packet item-use denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate zero-HP packet item-use denial: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate retaliation floor attack to emit 4 frames before immediate zero-HP packet item-use denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate zero-HP packet item-use denial setup, got %d", len(queued))
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.Position{WindowType: itemproto.WindowInventory, Cell: 5}})))
	if err != nil {
		t.Fatalf("unexpected packet item-use error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(useOut) != 0 {
		t.Fatalf("expected ITEM_USE to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(useOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected immediate zero-HP packet item-use denial not to queue frames, got %d", len(queued))
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after immediate zero-HP packet item-use denial")
	}
	if len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].ID != 1001 || inventorySnapshot.Inventory[0].Vnum != 27002 || inventorySnapshot.Inventory[0].Count != 3 || inventorySnapshot.Inventory[0].Slot != 5 {
		t.Fatalf("expected immediate zero-HP packet item-use denial to keep runtime inventory unchanged, got %#v", inventorySnapshot.Inventory)
	}
	persisted, err := accounts.Load("item-packet-owner-immediate")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP packet item-use owner account: %v", err)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected persisted immediate zero-HP packet item-use owner points to stay at pre-retaliation value %d, got %d", owner.Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 3, Slot: 5}}) {
		t.Fatalf("expected persisted immediate zero-HP packet item-use owner inventory to stay unchanged, got %#v", persisted.Characters[0].Inventory)
	}
}

func TestGameSessionFlowPracticeMobUseItemFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ItemOwnerDelayed", 0x01030113, 0x02040113, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 3, Slot: 5}}
	owner.Equipment = []inventory.ItemInstance{}
	issuePeerTicket(t, store, "item-owner-delayed", 0x54545454, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-owner-delayed", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed delayed zero-HP item-use owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  7,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 25,
			Message:    "consume:27002:+25",
		},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000470, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for delayed zero-HP item-use denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before delayed zero-HP item-use denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-owner-delayed", 0x54545454)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected item-use/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before delayed zero-HP item-use denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before delayed zero-HP item-use denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before delayed zero-HP item-use denial: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected delayed retaliation setup attack to emit 2 frames before delayed zero-HP item-use denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames before delayed zero-HP item-use denial, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before delayed zero-HP item-use denial: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before delayed zero-HP item-use denial, got %+v", pointChange)
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected slash item-use error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(useOut) != 0 {
		t.Fatalf("expected /use_item to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(useOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed zero-HP item-use denial not to queue frames, got %d", len(queued))
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after delayed zero-HP item-use denial")
	}
	if len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].ID != 1001 || inventorySnapshot.Inventory[0].Vnum != 27002 || inventorySnapshot.Inventory[0].Count != 3 || inventorySnapshot.Inventory[0].Slot != 5 {
		t.Fatalf("expected delayed zero-HP item-use denial to keep runtime inventory unchanged, got %#v", inventorySnapshot.Inventory)
	}
	persisted, err := accounts.Load("item-owner-delayed")
	if err != nil {
		t.Fatalf("load persisted delayed zero-HP item-use owner account: %v", err)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected persisted delayed zero-HP item-use owner points to stay at pre-retaliation value %d, got %d", owner.Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 3, Slot: 5}}) {
		t.Fatalf("expected persisted delayed zero-HP item-use owner inventory to stay unchanged, got %#v", persisted.Characters[0].Inventory)
	}
}

func TestGameSessionFlowPracticeMobEquipItemFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("EquipOwnerImmediate", 0x01030114, 0x02040114, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 12200, Count: 1, Slot: 8}}
	owner.Equipment = []inventory.ItemInstance{}
	issuePeerTicket(t, store, "equip-owner-immediate", 0x55555555, owner)
	if err := accounts.Save(accountstore.Account{Login: "equip-owner-immediate", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed immediate zero-HP equip owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected equip/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000480, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for immediate zero-HP equip denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before immediate zero-HP equip denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "equip-owner-immediate", 0x55555555)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected equip/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before immediate zero-HP equip denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before immediate zero-HP equip denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate zero-HP equip denial: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate retaliation floor attack to emit 4 frames before immediate zero-HP equip denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate zero-HP equip denial setup, got %d", len(queued))
	}

	beforePersisted, err := accounts.Load("equip-owner-immediate")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP equip owner account before denial: %v", err)
	}
	beforeEquipment, ok := runtime.EquipmentSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected equipment snapshot before immediate zero-HP equip denial")
	}
	beforeInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot before immediate zero-HP equip denial")
	}

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 weapon"})))
	if err != nil {
		t.Fatalf("unexpected slash equip error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(equipOut) != 0 {
		t.Fatalf("expected /equip_item to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(equipOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected immediate zero-HP equip denial not to queue frames, got %d", len(queued))
	}

	afterEquipment, ok := runtime.EquipmentSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected equipment snapshot after immediate zero-HP equip denial")
	}
	afterInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after immediate zero-HP equip denial")
	}
	if !reflect.DeepEqual(afterEquipment, beforeEquipment) {
		t.Fatalf("expected immediate zero-HP equip denial to keep runtime equipment unchanged, before=%+v after=%+v", beforeEquipment, afterEquipment)
	}
	if !reflect.DeepEqual(afterInventory, beforeInventory) {
		t.Fatalf("expected immediate zero-HP equip denial to keep runtime inventory unchanged, before=%+v after=%+v", beforeInventory, afterInventory)
	}
	persisted, err := accounts.Load("equip-owner-immediate")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP equip owner account after denial: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, beforePersisted.Characters[0].Inventory) || !reflect.DeepEqual(persisted.Characters[0].Equipment, beforePersisted.Characters[0].Equipment) {
		t.Fatalf("expected immediate zero-HP equip denial to keep persisted carried/equipped state unchanged, before inventory=%+v before equipment=%+v after inventory=%+v after equipment=%+v", beforePersisted.Characters[0].Inventory, beforePersisted.Characters[0].Equipment, persisted.Characters[0].Inventory, persisted.Characters[0].Equipment)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected immediate zero-HP equip denial to keep persisted points unchanged, before=%d after=%d", beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowPracticeMobUnequipItemFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("EquipOwnerDelayed", 0x01030115, 0x02040115, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Inventory = []inventory.ItemInstance{}
	owner.Equipment = []inventory.ItemInstance{{ID: 2002, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	issuePeerTicket(t, store, "equip-owner-delayed", 0x56565656, owner)
	if err := accounts.Save(accountstore.Account{Login: "equip-owner-delayed", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed delayed zero-HP equip owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      12200,
		Name:      "Practice Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: "weapon",
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  bootstrapPlayerPointType,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 10,
		},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected equip/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000490, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for delayed zero-HP equip denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before delayed zero-HP equip denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "equip-owner-delayed", 0x56565656)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected equip/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before delayed zero-HP equip denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before delayed zero-HP equip denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before delayed zero-HP equip denial: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected delayed retaliation setup attack to emit 2 frames before delayed zero-HP equip denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames before delayed zero-HP equip denial, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before delayed zero-HP equip denial: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before delayed zero-HP equip denial, got %+v", pointChange)
	}

	beforePersisted, err := accounts.Load("equip-owner-delayed")
	if err != nil {
		t.Fatalf("load persisted delayed zero-HP equip owner account before denial: %v", err)
	}
	beforeEquipment, ok := runtime.EquipmentSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected equipment snapshot before delayed zero-HP equip denial")
	}
	beforeInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot before delayed zero-HP equip denial")
	}

	unequipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected slash unequip error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(unequipOut) != 0 {
		t.Fatalf("expected /unequip_item to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(unequipOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed zero-HP equip denial not to queue frames, got %d", len(queued))
	}

	afterEquipment, ok := runtime.EquipmentSnapshot(owner.Name)
	if !ok {
		t.Fatal("expected equipment snapshot after delayed zero-HP equip denial")
	}
	afterInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after delayed zero-HP equip denial")
	}
	if !reflect.DeepEqual(afterEquipment, beforeEquipment) {
		t.Fatalf("expected delayed zero-HP equip denial to keep runtime equipment unchanged, before=%+v after=%+v", beforeEquipment, afterEquipment)
	}
	if !reflect.DeepEqual(afterInventory, beforeInventory) {
		t.Fatalf("expected delayed zero-HP equip denial to keep runtime inventory unchanged, before=%+v after=%+v", beforeInventory, afterInventory)
	}
	persisted, err := accounts.Load("equip-owner-delayed")
	if err != nil {
		t.Fatalf("load persisted delayed zero-HP equip owner account after denial: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, beforePersisted.Characters[0].Inventory) || !reflect.DeepEqual(persisted.Characters[0].Equipment, beforePersisted.Characters[0].Equipment) {
		t.Fatalf("expected delayed zero-HP equip denial to keep persisted carried/equipped state unchanged, before inventory=%+v before equipment=%+v after inventory=%+v after equipment=%+v", beforePersisted.Characters[0].Inventory, beforePersisted.Characters[0].Equipment, persisted.Characters[0].Inventory, persisted.Characters[0].Equipment)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected delayed zero-HP equip denial to keep persisted points unchanged, before=%d after=%d", beforePersisted.Characters[0].Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowPracticeMobInventoryMoveFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("InventoryMoveImmediate", 0x01030116, 0x02040116, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 1, Slot: 5}, {ID: 1002, Vnum: 27003, Count: 1, Slot: 6}}
	issuePeerTicket(t, store, "inventory-move-immediate", 0x57575757, owner)
	if err := accounts.Save(accountstore.Account{Login: "inventory-move-immediate", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed immediate zero-HP inventory-move owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected inventory-move/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000500, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for immediate zero-HP inventory-move denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before immediate zero-HP inventory-move denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "inventory-move-immediate", 0x57575757)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected inventory-move/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before immediate zero-HP inventory-move denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before immediate zero-HP inventory-move denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate zero-HP inventory-move denial: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate retaliation floor attack to emit 4 frames before immediate zero-HP inventory-move denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate zero-HP inventory-move denial setup, got %d", len(queued))
	}

	beforePersisted, err := accounts.Load("inventory-move-immediate")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP inventory-move owner account before denial: %v", err)
	}
	beforeInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot before immediate zero-HP inventory-move denial")
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/inventory_move 5 9"})))
	if err != nil {
		t.Fatalf("unexpected inventory-move slash error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected /inventory_move to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(moveOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected immediate zero-HP inventory-move denial not to queue frames, got %d", len(queued))
	}

	afterInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after immediate zero-HP inventory-move denial")
	}
	if !reflect.DeepEqual(afterInventory, beforeInventory) {
		t.Fatalf("expected immediate zero-HP inventory-move denial to keep runtime inventory unchanged, before=%+v after=%+v", beforeInventory, afterInventory)
	}
	persisted, err := accounts.Load("inventory-move-immediate")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP inventory-move owner account after denial: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, beforePersisted.Characters[0].Inventory) {
		t.Fatalf("expected immediate zero-HP inventory-move denial to keep persisted inventory unchanged, before=%+v after=%+v", beforePersisted.Characters[0].Inventory, persisted.Characters[0].Inventory)
	}
}

func TestGameSessionFlowPracticeMobInventoryMoveFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("InventoryMoveDelayed", 0x01030117, 0x02040117, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 1, Slot: 5}, {ID: 1002, Vnum: 27003, Count: 1, Slot: 6}}
	issuePeerTicket(t, store, "inventory-move-delayed", 0x58585858, owner)
	if err := accounts.Save(accountstore.Account{Login: "inventory-move-delayed", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed delayed zero-HP inventory-move owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected inventory-move/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000510, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for delayed zero-HP inventory-move denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before delayed zero-HP inventory-move denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "inventory-move-delayed", 0x58585858)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected inventory-move/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before delayed zero-HP inventory-move denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before delayed zero-HP inventory-move denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error before delayed zero-HP inventory-move denial: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected delayed retaliation setup attack to emit 2 frames before delayed zero-HP inventory-move denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames before delayed zero-HP inventory-move denial, got %d", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before delayed zero-HP inventory-move denial: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before delayed zero-HP inventory-move denial, got %+v", pointChange)
	}

	beforePersisted, err := accounts.Load("inventory-move-delayed")
	if err != nil {
		t.Fatalf("load persisted delayed zero-HP inventory-move owner account before denial: %v", err)
	}
	beforeInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot before delayed zero-HP inventory-move denial")
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/inventory_move 5 9"})))
	if err != nil {
		t.Fatalf("unexpected inventory-move slash error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected /inventory_move to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(moveOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed zero-HP inventory-move denial not to queue frames, got %d", len(queued))
	}

	afterInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after delayed zero-HP inventory-move denial")
	}
	if !reflect.DeepEqual(afterInventory, beforeInventory) {
		t.Fatalf("expected delayed zero-HP inventory-move denial to keep runtime inventory unchanged, before=%+v after=%+v", beforeInventory, afterInventory)
	}
	persisted, err := accounts.Load("inventory-move-delayed")
	if err != nil {
		t.Fatalf("load persisted delayed zero-HP inventory-move owner account after denial: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, beforePersisted.Characters[0].Inventory) {
		t.Fatalf("expected delayed zero-HP inventory-move denial to keep persisted inventory unchanged, before=%+v after=%+v", beforePersisted.Characters[0].Inventory, persisted.Characters[0].Inventory)
	}
}

func TestGameSessionFlowPracticeMobInventoryMoveKeepsRetaliationPointLossRuntimeOnlyWhilePersistingInventory(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("InventoryMoveRuntimeOnly", 0x01030118, 0x02040118, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 3
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 1, Slot: 5}, {ID: 1002, Vnum: 27003, Count: 1, Slot: 6}}
	issuePeerTicket(t, store, "inventory-move-runtime-only", 0x59595959, owner)
	if err := accounts.Save(accountstore.Account{Login: "inventory-move-runtime-only", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed runtime-only inventory-move owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected inventory-move/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000502, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for runtime-only inventory-move persistence: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before runtime-only inventory-move persistence, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	flow, enterOut := enterGameWithLoginTicket(t, factory, "inventory-move-runtime-only", 0x59595959)
	if len(enterOut) < 8 {
		t.Fatalf("expected runtime-only inventory-move owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before runtime-only inventory-move persistence: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before runtime-only inventory-move persistence, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error before runtime-only inventory-move persistence: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate retaliation attack to emit 2 frames before runtime-only inventory-move persistence, got %d", len(attackOut))
	}
	immediatePointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before runtime-only inventory-move persistence: %v", err)
	}
	if immediatePointChange.Value != 2 {
		t.Fatalf("expected immediate retaliation to drop live owner points to 2 before inventory move persistence, got %+v", immediatePointChange)
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/inventory_move 5 9"})))
	if err != nil {
		t.Fatalf("unexpected inventory-move slash error before runtime-only inventory persistence: %v", err)
	}
	if len(moveOut) != 2 {
		t.Fatalf("expected runtime inventory move to emit 2 self-only item frames, got %d", len(moveOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points runtime-only after inventory move, got %d queued frames", len(queued))
	}
	delayedPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change after runtime-only inventory move persistence: %v", err)
	}
	if delayedPointChange.Value != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points at runtime-only value 1 after inventory move, got %+v", delayedPointChange)
	}

	runtimeInventory, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatal("expected inventory snapshot after runtime-only inventory move persistence")
	}
	if len(runtimeInventory.Inventory) != 2 || runtimeInventory.Inventory[0].Slot != 6 || runtimeInventory.Inventory[1].Slot != 9 {
		t.Fatalf("expected runtime inventory move to leave occupied slots at 6 and 9, got %+v", runtimeInventory.Inventory)
	}

	persisted, err := accounts.Load("inventory-move-runtime-only")
	if err != nil {
		t.Fatalf("load persisted runtime-only inventory-move owner account: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly one persisted owner after runtime-only inventory-move persistence, got %+v", persisted)
	}
	if len(persisted.Characters[0].Inventory) != 2 || persisted.Characters[0].Inventory[0].Slot != 6 || persisted.Characters[0].Inventory[1].Slot != 9 {
		t.Fatalf("expected persisted inventory move to save occupied slots at 6 and 9, got %+v", persisted.Characters[0].Inventory)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected inventory move persistence to keep pre-retaliation points[%d] value %d, got %d", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	closeSessionFlow(t, flow)
	issuePeerTicket(t, store, "inventory-move-runtime-only", 0x69696969, owner)
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "inventory-move-runtime-only", 0x69696969)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) < 8 {
		t.Fatalf("expected runtime-only inventory-move reconnect bootstrap to emit at least 8 frames, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after runtime-only inventory move persistence: %v", err)
	}
	if reconnectPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected reconnect bootstrap after inventory move to rebuild pre-retaliation points[%d] value %d, got %+v", bootstrapPlayerPointValueIndex, owner.Points[bootstrapPlayerPointValueIndex], reconnectPointChange)
	}
}

func TestGameSessionFlowPracticeMobPeerChatFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.GuildID = 77
	owner.GuildName = "Guild"
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	watcher.GuildID = owner.GuildID
	watcher.GuildName = owner.GuildName
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner peer-chat denial after immediate retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner peer-chat denial after immediate retaliation, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner peer-chat denial after immediate retaliation: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-loss retaliation, self dead, and clear-target frames before zero-HP owner peer-chat denial after immediate retaliation, got %d frames", len(attackOut))
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected immediate retaliation reaching owner HP floor to queue 1 visible-peer DEAD frame before zero-HP owner peer-chat denial, got %d", len(watcherQueued))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode visible-peer dead frame before zero-HP owner peer-chat denial after immediate retaliation: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected visible-peer DEAD(owner_vid) before zero-HP owner peer-chat denial after immediate retaliation, got %+v", dead)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop once immediate retaliation reached owner HP floor before zero-HP owner peer-chat denial, got %d queued frames", len(queued))
	}

	chatCases := []struct {
		name     string
		chatType uint8
		message  string
	}{
		{name: "talking", chatType: chatproto.ChatTypeTalking, message: "hola local"},
		{name: "party", chatType: chatproto.ChatTypeParty, message: "hola party"},
		{name: "guild", chatType: chatproto.ChatTypeGuild, message: "hola guild"},
		{name: "shout", chatType: chatproto.ChatTypeShout, message: "hola shout"},
	}
	for _, tc := range chatCases {
		t.Run(tc.name, func(t *testing.T) {
			chatOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: tc.chatType, Message: tc.message})))
			if err != nil {
				t.Fatalf("unexpected %s chat error after immediate retaliation reached owner HP floor: %v", tc.name, err)
			}
			if len(chatOut) != 0 {
				t.Fatalf("expected %s chat to fail closed once immediate retaliation reached owner HP floor, got %d frames", tc.name, len(chatOut))
			}
			if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
				t.Fatalf("expected zero-HP owner %s chat denial after immediate retaliation not to queue peer delivery, got %d", tc.name, len(queued))
			}
		})
	}
}

func TestGameSessionFlowPracticeMobInfoChatFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner info-chat denial after immediate retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner info-chat denial after immediate retaliation, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner info-chat denial after immediate retaliation: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-loss retaliation, self dead, and clear-target frames before zero-HP owner info-chat denial after immediate retaliation, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop once immediate retaliation reached owner HP floor before zero-HP owner info-chat denial, got %d queued frames", len(queued))
	}

	infoOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeInfo, Message: "mensaje info"})))
	if err != nil {
		t.Fatalf("unexpected info chat error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(infoOut) != 0 {
		t.Fatalf("expected info chat to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(infoOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner info-chat denial after immediate retaliation not to queue frames, got %d", len(queued))
	}
}

func TestGameSessionFlowPracticeMobInfoChatFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner info-chat denial after delayed retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner info-chat denial after delayed retaliation, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner info-chat denial after delayed retaliation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected target refresh plus first point-loss retaliation before zero-HP owner info-chat denial after delayed retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames before zero-HP owner info-chat denial after delayed retaliation, got %d frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before zero-HP owner info-chat denial after delayed retaliation: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before zero-HP owner info-chat denial after delayed retaliation, got %+v", pointChange)
	}

	infoOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeInfo, Message: "mensaje info"})))
	if err != nil {
		t.Fatalf("unexpected info chat error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(infoOut) != 0 {
		t.Fatalf("expected info chat to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(infoOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner info-chat denial after delayed retaliation not to queue frames, got %d", len(queued))
	}
}

func TestGameSessionFlowPracticeMobWhisperFailsClosedAfterDelayedRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner whisper denial after delayed retaliation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner whisper denial after delayed retaliation, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner whisper denial after delayed retaliation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected target refresh plus first point-loss retaliation before zero-HP owner whisper denial after delayed retaliation, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, ownerFlow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames before zero-HP owner whisper denial after delayed retaliation, got %d frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change before zero-HP owner whisper denial after delayed retaliation: %v", err)
	}
	if pointChange.Value != 0 {
		t.Fatalf("expected delayed retaliation beat to reach owner HP floor before zero-HP owner whisper denial after delayed retaliation, got %+v", pointChange)
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected delayed retaliation reaching owner HP floor to queue 1 visible-peer DEAD frame before zero-HP owner whisper denial, got %d", len(watcherQueued))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode visible-peer dead frame before zero-HP owner whisper denial after delayed retaliation: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected visible-peer DEAD(owner_vid) before zero-HP owner whisper denial after delayed retaliation, got %+v", dead)
	}

	existingTargetOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{Target: watcher.Name, Message: "hola privado"})))
	if err != nil {
		t.Fatalf("unexpected whisper-to-existing-target error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(existingTargetOut) != 0 {
		t.Fatalf("expected whisper to existing target to fail closed once delayed retaliation reached owner HP floor, got %d frames", len(existingTargetOut))
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected zero-HP owner whisper denial after delayed retaliation not to queue target delivery, got %d", len(queued))
	}

	missingTargetOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientWhisper(chatproto.ClientWhisperPacket{Target: "GhostPlayer", Message: "still there?"})))
	if err != nil {
		t.Fatalf("unexpected whisper-to-missing-target error after delayed retaliation reached owner HP floor: %v", err)
	}
	if len(missingTargetOut) != 0 {
		t.Fatalf("expected whisper to missing target to fail closed once delayed retaliation reached owner HP floor without a NOT_EXIST fallback, got %d frames", len(missingTargetOut))
	}
}

func TestGameSessionFlowPracticeMobQuitSlashCommandStillWorksAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before zero-HP owner /quit regression check: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before zero-HP owner /quit regression check, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before zero-HP owner /quit regression check: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-loss retaliation, self dead, and clear-target frames before zero-HP owner /quit regression check, got %d frames", len(attackOut))
	}

	quitOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/quit"})))
	if err != nil {
		t.Fatalf("unexpected /quit error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(quitOut) != 1 {
		t.Fatalf("expected /quit to keep its existing command delivery after immediate retaliation reached owner HP floor, got %d frames", len(quitOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, quitOut[0]))
	if err != nil {
		t.Fatalf("decode /quit command chat after immediate retaliation reached owner HP floor: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeCommand || delivery.Message != "quit" {
		t.Fatalf("expected /quit to keep returning the command chat after immediate retaliation reached owner HP floor, got %+v", delivery)
	}
	phaseAware, ok := flow.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected queued flow to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseGame {
		t.Fatalf("expected /quit to keep session in game after immediate retaliation reached owner HP floor, got %q", phaseAware.CurrentPhase())
	}
}

func TestGameSessionFlowPracticeMobImmediateRetaliationSendsSelfDeadBeforeTargetClearAtOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before immediate retaliation target clear: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before immediate retaliation target clear, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate retaliation target clear: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-loss retaliation, self dead, and clear-target frames when immediate retaliation reaches owner HP floor, got %d frames", len(attackOut))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode immediate retaliation self-dead frame: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected immediate retaliation owner HP floor to append self-only DEAD(owner_vid), got %+v", dead)
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode immediate retaliation target-clear frame: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected immediate retaliation owner HP floor to append self-only TARGET(0, 0) clear, got %+v", clearTarget)
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationSendsSelfDeadBeforeTargetClearAtOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before delayed retaliation target clear: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before delayed retaliation target clear, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before delayed retaliation target clear: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target refresh plus first point-loss retaliation before delayed retaliation target clear, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames when delayed retaliation reaches owner HP floor, got %d frames", len(queued))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode delayed retaliation self-dead frame: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected delayed retaliation owner HP floor to append self-only DEAD(owner_vid), got %+v", dead)
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queued[2]))
	if err != nil {
		t.Fatalf("decode delayed retaliation target-clear frame: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected delayed retaliation owner HP floor to append self-only TARGET(0, 0) clear, got %+v", clearTarget)
	}
}

func TestGameSessionFlowPracticeMobImmediateRetaliationQueuesVisiblePeerDeadAtOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before immediate peer-dead retaliation check: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before immediate peer-dead retaliation check, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate peer-dead retaliation check: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-loss retaliation, self dead, and clear-target frames when immediate retaliation reaches owner HP floor, got %d frames", len(attackOut))
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected 1 queued visible-peer DEAD frame when immediate retaliation reaches owner HP floor, got %d", len(watcherQueued))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode visible-peer immediate retaliation dead frame: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected visible-peer DEAD(owner_vid) when immediate retaliation reaches owner HP floor, got %+v", dead)
	}
}

func TestGameSessionFlowPracticeMobDelayedRetaliationQueuesVisiblePeerDeadAtOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 2
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before delayed peer-dead retaliation check: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before delayed peer-dead retaliation check, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before delayed peer-dead retaliation check: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target refresh plus first point-loss retaliation before delayed peer-dead retaliation check, got %d frames", len(attackOut))
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no visible-peer death frame before delayed retaliation actually reaches owner HP floor, got %d", len(queued))
	}

	currentTime = currentTime.Add(time.Second)
	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) != 3 {
		t.Fatalf("expected delayed retaliation point-loss, self dead, and clear-target frames when delayed retaliation reaches owner HP floor, got %d frames", len(ownerQueued))
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected 1 queued visible-peer DEAD frame when delayed retaliation reaches owner HP floor, got %d", len(watcherQueued))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode visible-peer delayed retaliation dead frame: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected visible-peer DEAD(owner_vid) when delayed retaliation reaches owner HP floor, got %+v", dead)
	}
}

func TestGameSessionFlowPracticeMobCadenceWindowDeniedRepeatDoesNotAppendRetaliationOrResetDelayedBeat(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before cadence-window retaliation test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before cadence-window retaliation test, got %d", len(selectOut))
	}

	firstAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before cadence-window retaliation test: %v", err)
	}
	if len(firstAttack) != 2 {
		t.Fatalf("expected target-refresh plus self-only point-loss retaliation on first practice-mob hit before cadence-window denial, got %d frames", len(firstAttack))
	}

	currentTime = currentTime.Add(100 * time.Millisecond)
	repeatedAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected repeated attack error inside cadence window before delayed retaliation beat: %v", err)
	}
	if len(repeatedAttack) != 0 {
		t.Fatalf("expected repeated practice-mob attack inside owned 250ms cadence window to fail closed, got %d frames", len(repeatedAttack))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames before delayed retaliation beat after cadence-window denial, got %d", len(queued))
	}

	currentTime = time.Unix(1700000450, 0).Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected exactly 1 queued delayed retaliation beat after cadence-window denial, got %d frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode queued delayed retaliation beat after cadence-window denial: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != -1 || pointChange.Value != owner.Points[bootstrapPlayerPointValueIndex]-2 {
		t.Fatalf("unexpected delayed retaliation point-change packet after cadence-window denial: %+v", pointChange)
	}
}

func TestGameSessionFlowPracticeMobDelayedServerOriginRetaliationContinuesAutonomouslyWhileEngaged(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before autonomous delayed retaliation cadence: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before autonomous delayed retaliation cadence, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before autonomous delayed retaliation cadence: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation on first practice-mob hit, got %d frames", len(attackOut))
	}

	currentTime = currentTime.Add(time.Second)
	firstQueued := flushServerFrames(t, flow)
	if len(firstQueued) != 1 {
		t.Fatalf("expected exactly 1 first queued delayed retaliation beat, got %d frames", len(firstQueued))
	}
	firstPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, firstQueued[0]))
	if err != nil {
		t.Fatalf("decode first queued autonomous delayed retaliation beat: %v", err)
	}
	if firstPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex]-2 {
		t.Fatalf("unexpected first queued autonomous delayed retaliation value: %+v", firstPointChange)
	}

	currentTime = currentTime.Add(time.Second)
	secondQueued := flushServerFrames(t, flow)
	if len(secondQueued) != 1 {
		t.Fatalf("expected exactly 1 second autonomous delayed retaliation beat without another accepted hit, got %d frames", len(secondQueued))
	}
	secondPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, secondQueued[0]))
	if err != nil {
		t.Fatalf("decode second queued autonomous delayed retaliation beat: %v", err)
	}
	if secondPointChange.VID != owner.VID || secondPointChange.Type != bootstrapPlayerPointType || secondPointChange.Amount != -1 || secondPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex]-3 {
		t.Fatalf("unexpected second queued autonomous delayed retaliation point-change packet: %+v", secondPointChange)
	}
}

func TestGameSessionFlowPracticeMobDelayedServerOriginRetaliationDoesNotResetPendingCadenceWhenLaterAcceptedHitLands(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before repeatable delayed retaliation beat: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before repeatable delayed retaliation beat, got %d", len(selectOut))
	}

	firstAttackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before repeatable delayed retaliation beat: %v", err)
	}
	if len(firstAttackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation on first practice-mob hit, got %d frames", len(firstAttackOut))
	}

	currentTime = currentTime.Add(time.Second)
	firstQueued := flushServerFrames(t, flow)
	if len(firstQueued) != 1 {
		t.Fatalf("expected exactly 1 first queued delayed retaliation beat, got %d frames", len(firstQueued))
	}
	firstPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, firstQueued[0]))
	if err != nil {
		t.Fatalf("decode first queued delayed retaliation beat: %v", err)
	}
	if firstPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex]-2 {
		t.Fatalf("unexpected first queued delayed retaliation value: %+v", firstPointChange)
	}

	currentTime = currentTime.Add(500 * time.Millisecond)
	secondAttackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected second attack error while autonomous delayed retaliation cadence is already pending: %v", err)
	}
	if len(secondAttackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation on second practice-mob hit during autonomous cadence, got %d frames", len(secondAttackOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no extra delayed retaliation beat immediately after the later accepted hit during autonomous cadence, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(500 * time.Millisecond)
	secondQueued := flushServerFrames(t, flow)
	if len(secondQueued) != 1 {
		t.Fatalf("expected exactly 1 pending autonomous delayed retaliation beat on the original cadence timer after the later accepted hit, got %d frames", len(secondQueued))
	}
	secondPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, secondQueued[0]))
	if err != nil {
		t.Fatalf("decode autonomous delayed retaliation beat after later accepted hit: %v", err)
	}
	if secondPointChange.VID != owner.VID || secondPointChange.Type != bootstrapPlayerPointType || secondPointChange.Amount != -1 || secondPointChange.Value != owner.Points[bootstrapPlayerPointValueIndex]-4 {
		t.Fatalf("unexpected autonomous delayed retaliation point-change packet after later accepted hit: %+v", secondPointChange)
	}

	currentTime = currentTime.Add(500 * time.Millisecond)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no duplicate delayed retaliation beat half a second after the original cadence beat already fired, got %d frames", len(queued))
	}
}

func TestGameSessionFlowPracticeMobDelayedServerOriginRetaliationDoesNotStackWhileBeatPending(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before non-stacking delayed retaliation beat: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before non-stacking delayed retaliation beat, got %d", len(selectOut))
	}

	for hitIndex := 0; hitIndex < 2; hitIndex++ {
		if hitIndex > 0 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected attack error on rapid practice-mob hit %d before delayed retaliation beat: %v", hitIndex+1, err)
		}
		if len(attackOut) != 2 {
			t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation on rapid practice-mob hit %d, got %d frames", hitIndex+1, len(attackOut))
		}
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected exactly 1 queued delayed retaliation beat while a beat was already pending, got %d frames", len(queued))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode non-stacking delayed retaliation beat: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != -1 || pointChange.Value != owner.Points[bootstrapPlayerPointValueIndex]-3 {
		t.Fatalf("unexpected non-stacking delayed retaliation point-change packet: %+v", pointChange)
	}
}

func TestGameSessionFlowPracticeMobDelayedServerOriginRetaliationStopsAfterTargetReplacement(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}, {
		Ref:           "practice.mob_beta",
		Name:          "PracticeMobBeta",
		MapIndex:      bootstrapMapIndex,
		X:             1250,
		Y:             2250,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 runtime practice-mob actors after import, got %#v", actors)
	}
	firstTargetVID := uint32(actors[0].EntityID)
	secondTargetVID := uint32(actors[1].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 11 {
		t.Fatalf("expected 11 bootstrap frames for owner with two visible content practice mobs, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: firstTargetVID})))
	if err != nil {
		t.Fatalf("unexpected first target-selection error before delayed retaliation stop test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before delayed retaliation stop test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  firstTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first attack error before delayed retaliation stop test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before target replacement, got %d frames", len(attackOut))
	}

	replaceOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: secondTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target-replacement error before delayed retaliation stop test: %v", err)
	}
	if len(replaceOut) != 1 {
		t.Fatalf("expected 1 self-only target frame for replacement target before delayed retaliation stop test, got %d", len(replaceOut))
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop after target replacement, got %d queued frames", len(queued))
	}
}

func TestGameSessionFlowPracticeMobDelayedServerOriginRetaliationStopsAfterMovementClearsTarget(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before movement-clear retaliation stop test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before movement-clear retaliation stop test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before movement-clear retaliation stop test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before movement-clear target teardown, got %d frames", len(attackOut))
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1900, Y: 3100, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected move error before movement-clear retaliation stop test: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack frame after moving out of target range, got %d frames", len(moveOut))
	}
	queuedClear := flushServerFrames(t, flow)
	if len(queuedClear) != 1 {
		t.Fatalf("expected 1 queued self target-clear frame after moving out of target range, got %d frames", len(queuedClear))
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queuedClear[0]))
	if err != nil {
		t.Fatalf("decode movement-clear target frame: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected movement-driven target clear packet, got %+v", clearTarget)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop after movement cleared target, got %d queued frames", len(queued))
	}
}

func TestGameSessionFlowPracticeMobDelayedServerOriginRetaliationStopsAfterSyncPositionClearsTarget(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before sync-position-clear retaliation stop test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before sync-position-clear retaliation stop test, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before sync-position-clear retaliation stop test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before sync-position-clear target teardown, got %d frames", len(attackOut))
	}

	syncOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{Elements: []movep.SyncPositionElement{{VID: owner.VID, X: 1900, Y: 3100}}})))
	if err != nil {
		t.Fatalf("unexpected sync-position error before sync-position-clear retaliation stop test: %v", err)
	}
	if len(syncOut) != 1 {
		t.Fatalf("expected 1 immediate self sync-position ack frame after moving out of target range, got %d frames", len(syncOut))
	}
	queuedClear := flushServerFrames(t, flow)
	if len(queuedClear) != 1 {
		t.Fatalf("expected 1 queued self target-clear frame after sync-position moved out of target range, got %d frames", len(queuedClear))
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queuedClear[0]))
	if err != nil {
		t.Fatalf("decode sync-position-clear target frame: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected sync-position-driven target clear packet, got %+v", clearTarget)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected delayed retaliation cadence to stop after sync-position cleared target, got %d queued frames", len(queued))
	}
}

func TestGameSessionFlowPracticeMobMovementClearReleasesAggro(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before movement-clear aggro release test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before movement-clear aggro release test, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before movement-clear aggro release test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before movement-clear aggro release test, got %d frames", len(attackOut))
	}

	blockedTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target-selection error while owner still holds practice-mob aggro-lite gate: %v", err)
	}
	if len(blockedTargetOut) != 0 {
		t.Fatalf("expected watcher target-selection to fail closed while owner still holds practice-mob aggro-lite gate, got %d frames", len(blockedTargetOut))
	}

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1900, Y: 3100, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected move error before movement-clear aggro release test: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack frame after moving out of target range, got %d frames", len(moveOut))
	}
	queuedClear := flushServerFrames(t, ownerFlow)
	if len(queuedClear) != 1 {
		t.Fatalf("expected 1 queued self target-clear frame after moving out of target range, got %d frames", len(queuedClear))
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queuedClear[0]))
	if err != nil {
		t.Fatalf("decode movement-clear target frame before aggro release: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected movement-driven target clear packet before aggro release, got %+v", clearTarget)
	}

	releasedTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target-selection error after movement cleared owner target intent: %v", err)
	}
	if len(releasedTargetOut) != 1 {
		t.Fatalf("expected watcher target-selection to succeed after movement cleared owner target intent, got %d frames", len(releasedTargetOut))
	}
	releasedTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, releasedTargetOut[0]))
	if err != nil {
		t.Fatalf("decode watcher target-selection frame after movement-clear aggro release: %v", err)
	}
	if releasedTarget.TargetVID != targetVID || releasedTarget.HPPercent != 90 {
		t.Fatalf("expected watcher to reacquire same live practice mob at its current runtime-owned HP after movement cleared owner target intent, got %+v", releasedTarget)
	}
}

func TestGameSessionFlowPracticeMobRetargetReleasesPreviousAggro(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}, {
		Ref:           "practice.mob_beta",
		Name:          "PracticeMobBeta",
		MapIndex:      bootstrapMapIndex,
		X:             1250,
		Y:             2250,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 runtime practice-mob actors after import, got %#v", actors)
	}
	firstTargetVID := uint32(actors[0].EntityID)
	secondTargetVID := uint32(actors[1].EntityID)
	if firstTargetVID == secondTargetVID {
		t.Fatalf("expected distinct runtime practice-mob VIDs for retarget release test, got %d", firstTargetVID)
	}

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for owner with two visible content practice mobs, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 14 {
		t.Fatalf("expected 14 bootstrap frames for watcher with visible owner and two content practice mobs, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	firstSelectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: firstTargetVID})))
	if err != nil {
		t.Fatalf("unexpected first target-selection error before retarget aggro release test: %v", err)
	}
	if len(firstSelectOut) != 1 {
		t.Fatalf("expected 1 self-only first target frame before retarget aggro release test, got %d", len(firstSelectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  firstTargetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before retarget aggro release test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before retarget aggro release test, got %d frames", len(attackOut))
	}

	blockedTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: firstTargetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target-selection error while owner still holds first practice-mob aggro-lite gate: %v", err)
	}
	if len(blockedTargetOut) != 0 {
		t.Fatalf("expected watcher target-selection to fail closed while owner still holds first practice-mob aggro-lite gate, got %d frames", len(blockedTargetOut))
	}

	secondSelectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: secondTargetVID})))
	if err != nil {
		t.Fatalf("unexpected second target-selection error when owner replaces target intent: %v", err)
	}
	if len(secondSelectOut) != 1 {
		t.Fatalf("expected 1 self-only second target frame when owner replaces target intent, got %d", len(secondSelectOut))
	}

	releasedTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: firstTargetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target-selection error after owner replaced first practice-mob target intent: %v", err)
	}
	if len(releasedTargetOut) != 1 {
		t.Fatalf("expected watcher target-selection to succeed after owner replaced first practice-mob target intent, got %d frames", len(releasedTargetOut))
	}
	releasedTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, releasedTargetOut[0]))
	if err != nil {
		t.Fatalf("decode watcher target-selection frame after owner retarget release: %v", err)
	}
	if releasedTarget.TargetVID != firstTargetVID || releasedTarget.HPPercent != 90 {
		t.Fatalf("expected watcher to reacquire first live practice mob at its current runtime-owned HP after owner retarget released aggro-lite gate, got %+v", releasedTarget)
	}
}

func TestGameSessionFlowPracticeMobSlashLogoutStopsPendingRetaliationAndReleasesAggro(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before slash-logout retaliation stop test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before slash-logout retaliation stop test, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before slash-logout retaliation stop test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before slash logout, got %d frames", len(attackOut))
	}

	blockedTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target-selection error while owner still holds practice-mob aggro-lite gate: %v", err)
	}
	if len(blockedTargetOut) != 0 {
		t.Fatalf("expected watcher target-selection to fail closed while owner still holds practice-mob aggro-lite gate, got %d frames", len(blockedTargetOut))
	}

	logoutOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/logout"})))
	if err != nil {
		t.Fatalf("unexpected /logout error after delayed retaliation was armed: %v", err)
	}
	if len(logoutOut) != 1 {
		t.Fatalf("expected 1 close-phase frame after /logout with pending retaliation, got %d", len(logoutOut))
	}
	phase, err := control.DecodePhase(decodeSingleFrame(t, logoutOut[0]))
	if err != nil {
		t.Fatalf("decode /logout close-phase frame after pending retaliation: %v", err)
	}
	if phase.Phase != session.PhaseClose {
		t.Fatalf("expected /logout to transition to close phase after pending retaliation, got %q", phase.Phase)
	}
	phaseAware, ok := ownerFlow.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected owner flow to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseClose {
		t.Fatalf("expected /logout to keep owner flow in close phase after pending retaliation, got %q", phaseAware.CurrentPhase())
	}
	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 1 || snapshots[0].Name != watcher.Name {
		t.Fatalf("expected /logout to remove owner from live connected snapshots immediately while watcher stayed connected, got %+v", snapshots)
	}

	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected watcher to receive exactly 1 queued peer-delete frame when owner logs out mid-engagement, got %d", len(watcherQueued))
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode watcher peer-delete frame after owner logout: %v", err)
	}
	if peerDelete.VID != owner.VID {
		t.Fatalf("expected watcher peer-delete for owner VID after owner logout, got %+v", peerDelete)
	}

	releasedTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target-selection error after owner /logout should release practice-mob aggro-lite gate: %v", err)
	}
	if len(releasedTargetOut) != 1 {
		t.Fatalf("expected watcher target-selection to succeed after owner /logout released practice-mob aggro-lite gate, got %d frames", len(releasedTargetOut))
	}
	releasedTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, releasedTargetOut[0]))
	if err != nil {
		t.Fatalf("decode watcher target-selection frame after owner logout: %v", err)
	}
	if releasedTarget.TargetVID != targetVID || releasedTarget.HPPercent != 90 {
		t.Fatalf("expected watcher to reacquire same live practice mob at its current runtime-owned HP after owner logout released aggro-lite gate, got %+v", releasedTarget)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected pending delayed retaliation cadence to stop after owner /logout, got %d queued frames", len(queued))
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no extra watcher frames after owner /logout cancelled pending retaliation, got %d", len(queued))
	}
}

func TestGameSessionFlowPracticeMobSlashQuitStopsPendingRetaliationAndReleasesAggro(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	watcher := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, owner)
	issuePeerTicket(t, store, "peer-two", 0x22222222, watcher)

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000450, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before slash-quit retaliation stop test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame before slash-quit retaliation stop test, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before slash-quit retaliation stop test: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate target-refresh plus self-only point-loss retaliation before slash quit, got %d frames", len(attackOut))
	}

	blockedTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target-selection error while owner still holds practice-mob aggro-lite gate: %v", err)
	}
	if len(blockedTargetOut) != 0 {
		t.Fatalf("expected watcher target-selection to fail closed while owner still holds practice-mob aggro-lite gate, got %d frames", len(blockedTargetOut))
	}

	quitOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/quit"})))
	if err != nil {
		t.Fatalf("unexpected /quit error after delayed retaliation was armed: %v", err)
	}
	if len(quitOut) != 1 {
		t.Fatalf("expected 1 command frame after /quit with pending retaliation, got %d", len(quitOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, quitOut[0]))
	if err != nil {
		t.Fatalf("decode /quit command frame after pending retaliation: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeCommand || delivery.Message != "quit" {
		t.Fatalf("expected /quit to keep returning command chat after pending retaliation, got %+v", delivery)
	}
	phaseAware, ok := ownerFlow.(interface{ CurrentPhase() session.Phase })
	if !ok {
		t.Fatal("expected owner flow to expose current phase")
	}
	if phaseAware.CurrentPhase() != session.PhaseGame {
		t.Fatalf("expected /quit to keep owner flow in game phase after pending retaliation, got %q", phaseAware.CurrentPhase())
	}
	snapshots := runtime.ConnectedCharacters()
	if len(snapshots) != 1 || snapshots[0].Name != watcher.Name {
		t.Fatalf("expected /quit to remove owner from live connected snapshots immediately while watcher stayed connected, got %+v", snapshots)
	}

	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected watcher to receive exactly 1 queued peer-delete frame when owner quits mid-engagement, got %d", len(watcherQueued))
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode watcher peer-delete frame after owner quit: %v", err)
	}
	if peerDelete.VID != owner.VID {
		t.Fatalf("expected watcher peer-delete for owner VID after owner quit, got %+v", peerDelete)
	}

	releasedTargetOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target-selection error after owner /quit should release practice-mob aggro-lite gate: %v", err)
	}
	if len(releasedTargetOut) != 1 {
		t.Fatalf("expected watcher target-selection to succeed after owner /quit released practice-mob aggro-lite gate, got %d frames", len(releasedTargetOut))
	}
	releasedTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, releasedTargetOut[0]))
	if err != nil {
		t.Fatalf("decode watcher target-selection frame after owner quit: %v", err)
	}
	if releasedTarget.TargetVID != targetVID || releasedTarget.HPPercent != 90 {
		t.Fatalf("expected watcher to reacquire same live practice mob at its current runtime-owned HP after owner quit released aggro-lite gate, got %+v", releasedTarget)
	}

	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected pending delayed retaliation cadence to stop after owner /quit, got %d queued frames", len(queued))
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no extra watcher frames after owner /quit cancelled pending retaliation, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorDummyRespawnRebuildsForOtherVisibleSessionsAndRequiresFreshReselect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peerOne := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	peerTwo := peerVisibilityCharacter("PeerTwo", 0x01030102, 0x02040102, 1300, 2300, 2, 102, 202)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peerOne)
	issuePeerTicket(t, store, "peer-two", 0x22222222, peerTwo)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000400, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flowOne, enterOne := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOne) != 8 {
		t.Fatalf("expected 8 bootstrap frames for first player with visible training dummy, got %d", len(enterOne))
	}
	defer closeSessionFlow(t, flowOne)
	flowTwo, enterTwo := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-two", 0x22222222)
	if len(enterTwo) != 11 {
		t.Fatalf("expected 11 bootstrap frames for second player with visible peer and training dummy, got %d", len(enterTwo))
	}
	defer closeSessionFlow(t, flowTwo)
	if queued := flushServerFrames(t, flowOne); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for first player after second player joins, got %d", len(queued))
	}

	targetVID := uint32(actor.EntityID)
	for idx, flow := range []service.SessionFlow{flowOne, flowTwo} {
		selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected combat target error for respawn-visible session %d: %v", idx+1, err)
		}
		if len(selectOut) != 1 {
			t.Fatalf("expected 1 self-only combat target frame for respawn-visible session %d, got %d", idx+1, len(selectOut))
		}
	}

	for attackIndex := 0; attackIndex < 9; attackIndex++ {
		if attackIndex > 0 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		attackOut, err := flowOne.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected combat attack error on respawn-visible pre-death hit %d: %v", attackIndex+1, err)
		}
		if len(attackOut) != 1 {
			t.Fatalf("expected 1 self-only target-refresh frame on respawn-visible pre-death hit %d, got %d", attackIndex+1, len(attackOut))
		}
	}

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	finalAttack, err := flowOne.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error on visible-session respawn death hit: %v", err)
	}
	if len(finalAttack) != 2 {
		t.Fatalf("expected 2 self-only death-transition frames for respawn-visible killing hit, got %d", len(finalAttack))
	}
	peerDeathFrames := flushServerFrames(t, flowTwo)
	if len(peerDeathFrames) != 2 {
		t.Fatalf("expected 2 queued death-transition frames for other visible session before respawn, got %d", len(peerDeathFrames))
	}

	currentTime = currentTime.Add(2 * time.Second)
	selfRespawnFrames := flushServerFrames(t, flowOne)
	if len(selfRespawnFrames) != 4 {
		t.Fatalf("expected 4 self respawn rebuild frames after server-driven delay, got %d", len(selfRespawnFrames))
	}
	peerRespawnFrames := flushServerFrames(t, flowTwo)
	if len(peerRespawnFrames) != 4 {
		t.Fatalf("expected 4 queued respawn rebuild frames for the other visible session, got %d", len(peerRespawnFrames))
	}
	peerDeleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, peerRespawnFrames[0]))
	if err != nil {
		t.Fatalf("decode peer respawn delete frame: %v", err)
	}
	if peerDeleted.VID != targetVID {
		t.Fatalf("unexpected peer respawn delete frame: %+v", peerDeleted)
	}
	peerAdded, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, peerRespawnFrames[1]))
	if err != nil {
		t.Fatalf("decode peer respawn add frame: %v", err)
	}
	if peerAdded.VID != targetVID || peerAdded.X != 1200 || peerAdded.Y != 2200 {
		t.Fatalf("unexpected peer respawn add frame: %+v", peerAdded)
	}
	peerInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, peerRespawnFrames[2]))
	if err != nil {
		t.Fatalf("decode peer respawn additional info frame: %v", err)
	}
	if peerInfo.VID != targetVID || peerInfo.Name != "TrainingDummy" {
		t.Fatalf("unexpected peer respawn additional info frame: %+v", peerInfo)
	}
	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, peerRespawnFrames[3]))
	if err != nil {
		t.Fatalf("decode peer respawn update frame: %v", err)
	}
	if peerUpdate.VID != targetVID {
		t.Fatalf("unexpected peer respawn update frame: %+v", peerUpdate)
	}

	peerAttackWithoutReselectOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected other-session post-respawn attack-without-reselect error: %v", err)
	}
	if len(peerAttackWithoutReselectOut) != 0 {
		t.Fatalf("expected other visible session post-respawn attack without fresh target selection to fail closed, got %d frames", len(peerAttackWithoutReselectOut))
	}

	peerReselectOut, err := flowTwo.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected other-session post-respawn combat reselect error: %v", err)
	}
	if len(peerReselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame for other visible session after respawn reselection, got %d", len(peerReselectOut))
	}
	peerReselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, peerReselectOut[0]))
	if err != nil {
		t.Fatalf("decode other-session post-respawn target frame: %v", err)
	}
	if peerReselected.TargetVID != targetVID || peerReselected.HPPercent != 100 {
		t.Fatalf("unexpected other-session post-respawn target packet: %+v", peerReselected)
	}
}

func TestGameSessionFlowStaticActorAttackRejectsSelectedDummyAfterSnapshotReplacement(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat target error: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame before snapshot replacement, got %d", len(selectOut))
	}
	if _, ok := runtime.sharedWorld.UpdateStaticActorWithCombatKind(actor.EntityID, "TrainingDummy", bootstrapMapIndex, 1210, 2210, 20350, worldruntime.StaticActorCombatKindTrainingDummy); !ok {
		t.Fatal("expected training-dummy snapshot replacement to succeed")
	}
	if queued := flushServerFrames(t, flow); len(queued) == 0 {
		t.Fatal("expected actor replacement to enqueue refreshed static-actor frames")
	}

	staleAttackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(actor.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error after snapshot replacement: %v", err)
	}
	if len(staleAttackOut) != 0 {
		t.Fatalf("expected stale selected-target attack to fail closed after snapshot replacement, got %d frames", len(staleAttackOut))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat reselect error after snapshot replacement: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after snapshot replacement reselect, got %d", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode reselected training-dummy target frame after snapshot replacement: %v", err)
	}
	if reselected.TargetVID != uint32(actor.EntityID) || reselected.HPPercent != 100 {
		t.Fatalf("unexpected reselected training-dummy target packet after snapshot replacement: %+v", reselected)
	}

	freshAttackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(actor.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error after snapshot reselect: %v", err)
	}
	if len(freshAttackOut) != 1 {
		t.Fatalf("expected 1 self-only target-refresh frame after snapshot reselect, got %d", len(freshAttackOut))
	}
	freshTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, freshAttackOut[0]))
	if err != nil {
		t.Fatalf("decode fresh target-refresh frame after snapshot reselect: %v", err)
	}
	if freshTarget.TargetVID != uint32(actor.EntityID) || freshTarget.HPPercent != 90 {
		t.Fatalf("unexpected target-refresh packet after snapshot reselect: %+v", freshTarget)
	}
}

func TestGameSessionFlowStaticActorCombatTargetAndAttackRejectDeadTrainingDummy(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat target error before dead-state injection: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame before dead-state injection, got %d", len(selectOut))
	}

	runtime.sharedWorld.mu.Lock()
	runtime.sharedWorld.staticActorCombatHP[actor.EntityID] = 0
	runtime.sharedWorld.mu.Unlock()

	deadAttackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(actor.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error after dead-state injection: %v", err)
	}
	if len(deadAttackOut) != 0 {
		t.Fatalf("expected dead selected-target attack to fail closed, got %d frames", len(deadAttackOut))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat target error after dead-state injection: %v", err)
	}
	if len(reselectOut) != 0 {
		t.Fatalf("expected dead training-dummy reselect to fail closed, got %d frames", len(reselectOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames for dead training-dummy rejection, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorCombatTargetClearsWhenSelectedDummyLeavesCombatRange(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat target error: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after selection, got %d", len(selectOut))
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1700, Y: 2800, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move-out error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame while leaving combat range, got %d", len(moveOut))
	}
	moveOutAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveOut[0]))
	if err != nil {
		t.Fatalf("decode move-out ack: %v", err)
	}
	if moveOutAck.VID != peer.VID || moveOutAck.X != 1700 || moveOutAck.Y != 2800 {
		t.Fatalf("unexpected move-out ack: %+v", moveOutAck)
	}

	clearedFrames := flushServerFrames(t, flow)
	if len(clearedFrames) != 1 {
		t.Fatalf("expected 1 queued clear-target frame after leaving combat range, got %d", len(clearedFrames))
	}
	cleared, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, clearedFrames[0]))
	if err != nil {
		t.Fatalf("decode clear-target frame after leaving combat range: %v", err)
	}
	if cleared.TargetVID != 0 || cleared.HPPercent != 0 {
		t.Fatalf("expected zero-target clear packet after leaving combat range, got %+v", cleared)
	}

	moveBack, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 8, X: 1100, Y: 2100, Time: 0x11121315})))
	if err != nil {
		t.Fatalf("unexpected move-back error: %v", err)
	}
	if len(moveBack) != 1 {
		t.Fatalf("expected 1 self move ack frame while returning to dummy range, got %d", len(moveBack))
	}
	moveBackAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveBack[0]))
	if err != nil {
		t.Fatalf("decode move-back ack: %v", err)
	}
	if moveBackAck.VID != peer.VID || moveBackAck.X != 1100 || moveBackAck.Y != 2100 {
		t.Fatalf("unexpected move-back ack: %+v", moveBackAck)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued visibility frames when returning to in-range whole-map visibility, got %d", len(queued))
	}

	attackWithoutReselect, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(actor.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected attack-after-range-loss error: %v", err)
	}
	if len(attackWithoutReselect) != 0 {
		t.Fatalf("expected no self frames when attacking after range-loss clear without reselection, got %d", len(attackWithoutReselect))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat reselect error after range-loss clear: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after range-loss clear and reselection, got %d", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode reselected training-dummy target frame after range-loss clear: %v", err)
	}
	if reselected.TargetVID != uint32(actor.EntityID) || reselected.HPPercent != 100 {
		t.Fatalf("unexpected reselected target packet after range-loss clear: %+v", reselected)
	}
}

func TestGameSessionFlowStaticActorCombatTargetClearsWhenSelectedDummyLeavesVisibility(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy inside radius visibility, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat target error: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after selection, got %d", len(selectOut))
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1700, Y: 2800, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected move-out visibility error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 self move ack frame while leaving visibility, got %d", len(moveOut))
	}

	clearedFrames := flushServerFrames(t, flow)
	if len(clearedFrames) != 2 {
		t.Fatalf("expected static-actor delete plus clear-target frame after leaving visibility, got %d", len(clearedFrames))
	}
	deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, clearedFrames[0]))
	if err != nil {
		t.Fatalf("decode static-actor delete after visibility loss: %v", err)
	}
	if deleted.VID != uint32(actor.EntityID) {
		t.Fatalf("unexpected static-actor delete after visibility loss: %+v", deleted)
	}
	cleared, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, clearedFrames[1]))
	if err != nil {
		t.Fatalf("decode clear-target frame after leaving visibility: %v", err)
	}
	if cleared.TargetVID != 0 || cleared.HPPercent != 0 {
		t.Fatalf("expected zero-target clear packet after visibility loss, got %+v", cleared)
	}

	moveBack, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 8, X: 1100, Y: 2100, Time: 0x11121315})))
	if err != nil {
		t.Fatalf("unexpected move-back visibility error: %v", err)
	}
	if len(moveBack) != 1 {
		t.Fatalf("expected 1 self move ack frame while returning to visibility, got %d", len(moveBack))
	}

	restoreFrames := flushServerFrames(t, flow)
	if len(restoreFrames) != 3 {
		t.Fatalf("expected 3 queued static-actor visibility frames after returning into visibility, got %d", len(restoreFrames))
	}
	restoredAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restoreFrames[0]))
	if err != nil {
		t.Fatalf("decode static-actor add after visibility restore: %v", err)
	}
	if restoredAdd.VID != uint32(actor.EntityID) || restoredAdd.X != 1200 || restoredAdd.Y != 2200 {
		t.Fatalf("unexpected static-actor add after visibility restore: %+v", restoredAdd)
	}

	attackWithoutReselect, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(actor.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected attack-after-visibility-loss error: %v", err)
	}
	if len(attackWithoutReselect) != 0 {
		t.Fatalf("expected no self frames when attacking after visibility-loss clear without reselection, got %d", len(attackWithoutReselect))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat reselect error after visibility-loss clear: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after visibility-loss clear and reselection, got %d", len(reselectOut))
	}
}

func TestGameSessionFlowStaticActorCombatTargetClearsAcrossTransferRebootstrap(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}, {
		SourceMapIndex: 42,
		SourceX:        1700,
		SourceY:        2800,
		TargetMapIndex: bootstrapMapIndex,
		TargetX:        1100,
		TargetY:        2100,
	}})
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected training dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after enter, got %d", len(queued))
	}

	targetOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected target selection error before transfer: %v", err)
	}
	if len(targetOut) != 1 {
		t.Fatalf("expected 1 target ack frame before transfer, got %d", len(targetOut))
	}

	transferOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222324})))
	if err != nil {
		t.Fatalf("unexpected transfer move error: %v", err)
	}
	if len(transferOut) == 0 {
		t.Fatal("expected transfer rebootstrap frames after first move trigger")
	}
	returnOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1700, Y: 2800, Time: 0x21222325})))
	if err != nil {
		t.Fatalf("unexpected return transfer move error: %v", err)
	}
	if len(returnOut) == 0 {
		t.Fatal("expected transfer rebootstrap frames after return move trigger")
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after round-trip transfer rebootstrap, got %d", len(queued))
	}

	attackWithoutReselect, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected attack error after round-trip transfer rebootstrap: %v", err)
	}
	if len(attackWithoutReselect) != 0 {
		t.Fatalf("expected transfer rebootstrap to clear the active combat target, got %d frames", len(attackWithoutReselect))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected reselect error after transfer rebootstrap: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 target ack frame after transfer rebootstrap, got %d", len(reselectOut))
	}
	attackAfterReselect, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected attack error after transfer rebootstrap reselect: %v", err)
	}
	if len(attackAfterReselect) != 1 {
		t.Fatalf("expected 1 self-only attack refresh after transfer rebootstrap reselect, got %d", len(attackAfterReselect))
	}
}

func TestGameSessionFlowStaticActorCombatTargetClearsAcrossPhaseSelectReenter(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	first := peerVisibilityCharacter("MkmkWar", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	second := peerVisibilityCharacter("MkmkSura", 0x01030102, 0x02040102, 1120, 2120, 2, 102, 202)
	if _, ok := issueLoginTicket(store, "phase-select-combat", first.Empire, []loginticket.Character{first, second}, func() (uint32, error) {
		return 0x11111111, nil
	}); !ok {
		t.Fatal("expected login ticket issuance to succeed")
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected training dummy registration to succeed")
	}
	flow := runtime.SessionFactory()()
	_ = mustCompleteSecureHandshake(t, flow)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "phase-select-combat", LoginKey: 0x11111111})
	if err != nil {
		t.Fatalf("encode login2: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("unexpected first character select error: %v", err)
	}
	firstEnter, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected first enter-game error: %v", err)
	}
	if len(firstEnter) != 8 {
		t.Fatalf("expected 8 first bootstrap frames with visible training dummy, got %d", len(firstEnter))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after first enter, got %d", len(queued))
	}

	targetOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected first target selection error: %v", err)
	}
	if len(targetOut) != 1 {
		t.Fatalf("expected 1 target ack frame before /phase_select, got %d", len(targetOut))
	}

	phaseSelectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/phase_select"})))
	if err != nil {
		t.Fatalf("unexpected /phase_select error: %v", err)
	}
	if len(phaseSelectOut) != 1 {
		t.Fatalf("expected 1 /phase_select frame, got %d", len(phaseSelectOut))
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("unexpected second character select error after /phase_select: %v", err)
	}
	secondEnter, err := flow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected second enter-game error after /phase_select: %v", err)
	}
	if len(secondEnter) != 8 {
		t.Fatalf("expected 8 second bootstrap frames with visible training dummy, got %d", len(secondEnter))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after second enter, got %d", len(queued))
	}

	attackWithoutReselect, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected attack error after /phase_select re-enter: %v", err)
	}
	if len(attackWithoutReselect) != 0 {
		t.Fatalf("expected /phase_select re-enter to clear the active combat target, got %d frames", len(attackWithoutReselect))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected second target selection error after /phase_select: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 target ack frame after /phase_select re-enter, got %d", len(reselectOut))
	}
}

func TestGameSessionFlowStaticActorCombatTargetClearsAcrossReconnect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected training dummy registration to succeed")
	}
	factory := runtime.SessionFactory()
	flowOld, enterOut := enterGameWithLoginTicket(t, factory, "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	if queued := flushServerFrames(t, flowOld); len(queued) != 0 {
		t.Fatalf("expected no queued frames after first enter, got %d", len(queued))
	}
	targetOut, err := flowOld.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected target selection error before reconnect: %v", err)
	}
	if len(targetOut) != 1 {
		t.Fatalf("expected 1 target ack frame before reconnect, got %d", len(targetOut))
	}
	closeSessionFlow(t, flowOld)

	issuePeerTicket(t, store, "peer-one", 0x22222222, peer)
	flowReconnect, reconnectEnter := enterGameWithLoginTicket(t, factory, "peer-one", 0x22222222)
	if len(reconnectEnter) != 8 {
		t.Fatalf("expected 8 reconnect bootstrap frames with visible training dummy, got %d", len(reconnectEnter))
	}
	defer closeSessionFlow(t, flowReconnect)
	if queued := flushServerFrames(t, flowReconnect); len(queued) != 0 {
		t.Fatalf("expected no queued frames after reconnect enter, got %d", len(queued))
	}

	attackWithoutReselect, err := flowReconnect.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected attack error after reconnect: %v", err)
	}
	if len(attackWithoutReselect) != 0 {
		t.Fatalf("expected reconnect to clear the active combat target, got %d frames", len(attackWithoutReselect))
	}

	reselectOut, err := flowReconnect.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected target selection error after reconnect: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 target ack frame after reconnect, got %d", len(reselectOut))
	}
}

func TestGameSessionFlowStaticActorAttackRejectsWithoutActiveTargetOrMatchingSelection(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	first, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummyOne", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected first visible training-dummy registration to succeed")
	}
	second, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummyTwo", bootstrapMapIndex, 1210, 2210, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected second visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 11 {
		t.Fatalf("expected 11 bootstrap frames with two visible training dummies, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	withoutSelection, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(first.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected no-selection combat attack error: %v", err)
	}
	if len(withoutSelection) != 0 {
		t.Fatalf("expected no self frames for combat attack without active target selection, got %d", len(withoutSelection))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(first.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected combat target error: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame after selection, got %d", len(selectOut))
	}

	mismatched, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  uint32(second.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected mismatched-target combat attack error: %v", err)
	}
	if len(mismatched) != 0 {
		t.Fatalf("expected no self frames for mismatched selected-target attack, got %d", len(mismatched))
	}

	nonNormal, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal + 1,
		TargetVID:  uint32(first.EntityID),
	})))
	if err != nil {
		t.Fatalf("unexpected non-normal combat attack error: %v", err)
	}
	if len(nonNormal) != 0 {
		t.Fatalf("expected no self frames for non-normal bootstrap attack type, got %d", len(nonNormal))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for accepted bootstrap dummy attack, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorAttackSuppressesRepeatedSameTargetHitUntilCadenceWindowExpires(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000400, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	targetVID := uint32(actor.EntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected combat target error before cadence-window test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame before cadence-window test, got %d", len(selectOut))
	}

	firstAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first combat attack error before cadence-window test: %v", err)
	}
	if len(firstAttack) != 1 {
		t.Fatalf("expected 1 self-only target-refresh frame on first accepted dummy hit before cadence-window test, got %d", len(firstAttack))
	}

	repeatedAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected repeated combat attack error inside cadence window: %v", err)
	}
	if len(repeatedAttack) != 0 {
		t.Fatalf("expected repeated same-target attack inside owned 250ms cadence window to fail closed, got %d frames", len(repeatedAttack))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames for cadence-window denied repeated dummy attack, got %d", len(queued))
	}

	currentTime = currentTime.Add(250 * time.Millisecond)
	afterCooldown, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected combat attack error after cadence window expired: %v", err)
	}
	if len(afterCooldown) != 1 {
		t.Fatalf("expected same-target attack after cadence window expiry to be accepted, got %d frames", len(afterCooldown))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, afterCooldown[0]))
	if err != nil {
		t.Fatalf("decode accepted dummy target-refresh frame after cadence window expiry: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 80 {
		t.Fatalf("unexpected dummy target-refresh packet after cadence window expiry: %+v", refresh)
	}
}

func TestGameSessionFlowStaticActorAttackReselectingSameTargetDoesNotBypassCadenceWindow(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000400, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "TrainingDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected visible training-dummy registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) != 8 {
		t.Fatalf("expected 8 bootstrap frames with visible training dummy, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	targetVID := uint32(actor.EntityID)
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected combat target error before cadence-window reselect test: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 self-only combat target frame before cadence-window reselect test, got %d", len(selectOut))
	}

	firstAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected first combat attack error before cadence-window reselect test: %v", err)
	}
	if len(firstAttack) != 1 {
		t.Fatalf("expected 1 self-only target-refresh frame on first accepted dummy hit before cadence-window reselect test, got %d", len(firstAttack))
	}

	reselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected combat reselect error inside cadence window: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected 1 self-only target frame for same-target reselect inside cadence window, got %d", len(reselectOut))
	}

	repeatedAttack, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected repeated combat attack error after same-target reselect inside cadence window: %v", err)
	}
	if len(repeatedAttack) != 0 {
		t.Fatalf("expected same-target reselect not to bypass the owned 250ms cadence window, got %d frames", len(repeatedAttack))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames for cadence-window denied repeated dummy attack after same-target reselect, got %d", len(queued))
	}
}

type failingStaticActorStore struct{}

func (f *failingStaticActorStore) Load() (staticstore.Snapshot, error) {
	return staticstore.Snapshot{}, staticstore.ErrSnapshotNotFound
}

func (f *failingStaticActorStore) Save(staticstore.Snapshot) error {
	return errors.New("save failed")
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
	flow := newQueuedSessionFlow(inner, newPendingServerFrames(), nil, func() { closed = true })
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
	flow := newQueuedSessionFlow(inner, newPendingServerFrames(), nil, nil)
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

func TestGameSessionFlowPracticeMobEquipKeepsRetaliationPointLossRuntimeOnlyWhilePersistingEquipEffect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("EquipOwnerRuntimeOnly", 0x01030189, 0x02040189, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 3
	owner.Inventory = []inventory.ItemInstance{{ID: 1002, Vnum: 12200, Count: 1, Slot: 8}}
	owner.Equipment = []inventory.ItemInstance{}
	issuePeerTicket(t, store, "equip-owner-runtime-only", 0x89898989, owner)
	if err := accounts.Save(accountstore.Account{Login: "equip-owner-runtime-only", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed runtime-only equip owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, defaultBootstrapItemTemplateSnapshot().Templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected practice-mob equip runtime error: %v", err)
	}
	currentTime := time.Unix(1700000610, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for runtime-only equip persistence: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before runtime-only equip persistence, got %d", len(actors))
	}
	practiceMobTargetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	flow, enterOut := enterGameWithLoginTicket(t, factory, "equip-owner-runtime-only", 0x89898989)
	if len(enterOut) < 8 {
		t.Fatalf("expected runtime-only equip owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before runtime-only equip persistence: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before runtime-only equip persistence, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error before runtime-only equip persistence: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate retaliation setup attack to emit 2 frames before runtime-only equip persistence, got %d", len(attackOut))
	}
	immediatePointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before runtime-only equip persistence: %v", err)
	}
	if immediatePointChange.Value != 2 {
		t.Fatalf("expected immediate retaliation to drop live owner points to 2 before runtime-only equip persistence, got %+v", immediatePointChange)
	}

	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/equip_item 8 weapon"})))
	if err != nil {
		t.Fatalf("unexpected slash equip error before runtime-only equip persistence: %v", err)
	}
	if len(equipOut) == 0 {
		t.Fatalf("expected runtime-only equip to emit frames")
	}
	foundEquipPointChange := false
	for _, raw := range equipOut {
		pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, raw))
		if err != nil {
			continue
		}
		if pointChange.Value != 12 {
			t.Fatalf("expected slash equip to raise live owner points to 12 while keeping retaliation runtime-only, got %+v", pointChange)
		}
		foundEquipPointChange = true
		break
	}
	if !foundEquipPointChange {
		t.Fatalf("expected slash equip frames to include a point change after runtime-only retaliation")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points runtime-only after equip, got %d queued frames", len(queued))
	}
	delayedPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change after runtime-only equip persistence: %v", err)
	}
	if delayedPointChange.Value != 11 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points at runtime-only value 11 after equip, got %+v", delayedPointChange)
	}

	account, err := accounts.Load("equip-owner-runtime-only")
	if err != nil {
		t.Fatalf("load persisted runtime-only equip owner account: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != 13 {
		t.Fatalf("expected persisted equip owner points[%d] to keep pre-retaliation value plus equip delta 13, got %d", bootstrapPlayerPointValueIndex, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted inventory to be empty after runtime-only equip persistence, got %#v", account.Characters[0].Inventory)
	}
	if len(account.Characters[0].Equipment) != 1 || account.Characters[0].Equipment[0].Vnum != 12200 || !account.Characters[0].Equipment[0].Equipped || account.Characters[0].Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon {
		t.Fatalf("unexpected persisted equipment after runtime-only equip persistence: %#v", account.Characters[0].Equipment)
	}

	closeSessionFlow(t, flow)
	issuePeerTicket(t, store, "equip-owner-runtime-only", 0x99999999, owner)
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "equip-owner-runtime-only", 0x99999999)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) < 8 {
		t.Fatalf("expected runtime-only equip reconnect bootstrap to emit at least 8 frames, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after runtime-only equip persistence: %v", err)
	}
	if reconnectPointChange.Value != 13 {
		t.Fatalf("expected reconnect bootstrap after equip to rebuild persisted points[%d] value 13 without retaliation loss, got %+v", bootstrapPlayerPointValueIndex, reconnectPointChange)
	}
}

func TestGameSessionFlowPracticeMobUnequipKeepsRetaliationPointLossRuntimeOnlyWhilePersistingEquipRemoval(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UnequipOwnerRuntimeOnly", 0x0103018a, 0x0204018a, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 13
	owner.Inventory = []inventory.ItemInstance{}
	owner.Equipment = []inventory.ItemInstance{{ID: 2002, Vnum: 12200, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	issuePeerTicket(t, store, "unequip-owner-runtime-only", 0x8a8a8a8a, owner)
	if err := accounts.Save(accountstore.Account{Login: "unequip-owner-runtime-only", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed runtime-only unequip owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, defaultBootstrapItemTemplateSnapshot().Templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected practice-mob unequip runtime error: %v", err)
	}
	currentTime := time.Unix(1700000620, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for runtime-only unequip persistence: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before runtime-only unequip persistence, got %d", len(actors))
	}
	practiceMobTargetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	flow, enterOut := enterGameWithLoginTicket(t, factory, "unequip-owner-runtime-only", 0x8a8a8a8a)
	if len(enterOut) < 8 {
		t.Fatalf("expected runtime-only unequip owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before runtime-only unequip persistence: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before runtime-only unequip persistence, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error before runtime-only unequip persistence: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate retaliation setup attack to emit 2 frames before runtime-only unequip persistence, got %d", len(attackOut))
	}
	immediatePointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before runtime-only unequip persistence: %v", err)
	}
	if immediatePointChange.Value != 12 {
		t.Fatalf("expected immediate retaliation to drop live owner points to 12 before runtime-only unequip persistence, got %+v", immediatePointChange)
	}

	unequipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 4"})))
	if err != nil {
		t.Fatalf("unexpected slash unequip error before runtime-only unequip persistence: %v", err)
	}
	if len(unequipOut) == 0 {
		t.Fatalf("expected runtime-only unequip to emit frames")
	}
	foundUnequipPointChange := false
	for _, raw := range unequipOut {
		pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, raw))
		if err != nil {
			continue
		}
		if pointChange.Value != 2 {
			t.Fatalf("expected slash unequip to lower live owner points to 2 while keeping retaliation runtime-only, got %+v", pointChange)
		}
		foundUnequipPointChange = true
		break
	}
	if !foundUnequipPointChange {
		t.Fatalf("expected slash unequip frames to include a point change after runtime-only retaliation")
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points runtime-only after unequip, got %d queued frames", len(queued))
	}
	delayedPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change after runtime-only unequip persistence: %v", err)
	}
	if delayedPointChange.Value != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points at runtime-only value 1 after unequip, got %+v", delayedPointChange)
	}

	account, err := accounts.Load("unequip-owner-runtime-only")
	if err != nil {
		t.Fatalf("load persisted runtime-only unequip owner account: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != 3 {
		t.Fatalf("expected persisted unequip owner points[%d] to keep pre-retaliation value after equip removal, got %d", bootstrapPlayerPointValueIndex, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if len(account.Characters[0].Equipment) != 0 {
		t.Fatalf("expected persisted equipment to be empty after runtime-only unequip persistence, got %#v", account.Characters[0].Equipment)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 12200 || account.Characters[0].Inventory[0].Slot != 4 || account.Characters[0].Inventory[0].Equipped {
		t.Fatalf("unexpected persisted inventory after runtime-only unequip persistence: %#v", account.Characters[0].Inventory)
	}

	closeSessionFlow(t, flow)
	issuePeerTicket(t, store, "unequip-owner-runtime-only", 0x9a9a9a9a, owner)
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "unequip-owner-runtime-only", 0x9a9a9a9a)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) < 8 {
		t.Fatalf("expected runtime-only unequip reconnect bootstrap to emit at least 8 frames, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after runtime-only unequip persistence: %v", err)
	}
	if reconnectPointChange.Value != 3 {
		t.Fatalf("expected reconnect bootstrap after unequip to rebuild persisted points[%d] value 3 without retaliation loss, got %+v", bootstrapPlayerPointValueIndex, reconnectPointChange)
	}
}

func TestGameSessionFlowPracticeMobUseItemKeepsRetaliationPointLossRuntimeOnlyWhilePersistingUseEffect(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseOwnerRuntimeOnly", 0x01030188, 0x02040188, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 3
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Equipment = []inventory.ItemInstance{}
	issuePeerTicket(t, store, "use-owner-runtime-only", 0x88888888, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-owner-runtime-only", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed runtime-only use-item owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, defaultBootstrapItemTemplateSnapshot().Templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected practice-mob item-use runtime error: %v", err)
	}
	currentTime := time.Unix(1700000600, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for runtime-only item-use persistence: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before runtime-only item-use persistence, got %d", len(actors))
	}
	practiceMobTargetVID := uint32(actors[0].EntityID)

	factory := runtime.SessionFactory()
	flow, enterOut := enterGameWithLoginTicket(t, factory, "use-owner-runtime-only", 0x88888888)
	if len(enterOut) < 8 {
		t.Fatalf("expected runtime-only item-use owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before runtime-only item-use persistence: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before runtime-only item-use persistence, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: practiceMobTargetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error before runtime-only item-use persistence: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected immediate retaliation setup attack to emit 2 frames before runtime-only item-use persistence, got %d", len(attackOut))
	}
	immediatePointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode immediate retaliation point-change before runtime-only item-use persistence: %v", err)
	}
	if immediatePointChange.Value != 2 {
		t.Fatalf("expected immediate retaliation to drop live owner points to 2 before runtime-only item-use persistence, got %+v", immediatePointChange)
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected slash item-use error before runtime-only item-use persistence: %v", err)
	}
	if len(useOut) != 3 {
		t.Fatalf("expected runtime-only item use to emit 3 frames, got %d", len(useOut))
	}
	usePointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, useOut[0]))
	if err != nil {
		t.Fatalf("decode slash item-use point change during runtime-only item-use persistence: %v", err)
	}
	if usePointChange.Value != 52 {
		t.Fatalf("expected slash item use to raise live owner points to 52 while keeping retaliation runtime-only, got %+v", usePointChange)
	}
	useSetPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, useOut[1]))
	if err != nil {
		t.Fatalf("decode slash item-use set during runtime-only item-use persistence: %v", err)
	}
	if useSetPacket.Position.WindowType != itemproto.WindowInventory || useSetPacket.Position.Cell != 5 || useSetPacket.Vnum != 27001 || useSetPacket.Count != 2 {
		t.Fatalf("unexpected slash item-use set packet during runtime-only item-use persistence: %+v", useSetPacket)
	}

	currentTime = currentTime.Add(time.Second)
	queued := flushServerFrames(t, flow)
	if len(queued) != 1 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points runtime-only after item use, got %d queued frames", len(queued))
	}
	delayedPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode delayed retaliation point-change after runtime-only item-use persistence: %v", err)
	}
	if delayedPointChange.Value != 51 {
		t.Fatalf("expected delayed retaliation cadence to keep live owner points at runtime-only value 51 after item use, got %+v", delayedPointChange)
	}

	account, err := accounts.Load("use-owner-runtime-only")
	if err != nil {
		t.Fatalf("load persisted runtime-only item-use owner account: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != 53 {
		t.Fatalf("expected persisted item-use owner points[%d] to keep pre-retaliation value plus use-effect delta 53, got %d", bootstrapPlayerPointValueIndex, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected persisted inventory after runtime-only item-use persistence: %#v", account.Characters[0].Inventory)
	}

	closeSessionFlow(t, flow)
	issuePeerTicket(t, store, "use-owner-runtime-only", 0x98989898, owner)
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "use-owner-runtime-only", 0x98989898)
	defer closeSessionFlow(t, reconnectFlow)
	if len(reconnectEnter) < 8 {
		t.Fatalf("expected runtime-only item-use reconnect bootstrap to emit at least 8 frames, got %d", len(reconnectEnter))
	}
	reconnectPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, reconnectEnter[4]))
	if err != nil {
		t.Fatalf("decode reconnect bootstrap point-change after runtime-only item-use persistence: %v", err)
	}
	if reconnectPointChange.Value != 53 {
		t.Fatalf("expected reconnect bootstrap after item use to rebuild persisted points[%d] value 53 without retaliation loss, got %+v", bootstrapPlayerPointValueIndex, reconnectPointChange)
	}
}

func TestGameRuntimeItemUseResolvesPointEffectFromItemTemplateStore(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("Owner", 0x01030144, 0x02040144, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 700
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 3, Slot: 5}}
	owner.Equipment = []inventory.ItemInstance{}
	issuePeerTicket(t, store, "owner", 0x44444444, owner)
	if err := accounts.Save(accountstore.Account{Login: "owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use owner account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, nil)
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27002,
		Name:      "Practice Elixir",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  7,
			PointIndex: bootstrapPlayerPointValueIndex,
			PointDelta: 25,
			Message:    "consume:27002:+25",
		},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-use runtime error: %v", err)
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "owner", 0x44444444)
	if len(enterOut) < 5 {
		t.Fatalf("expected item-use owner bootstrap to emit at least 5 frames, got %d", len(enterOut))
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected template-store item use error: %v", err)
	}
	if len(useOut) != 3 {
		t.Fatalf("expected point-change + item-set + info frames for template-store item use, got %d", len(useOut))
	}
	pointPacket, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, useOut[0]))
	if err != nil {
		t.Fatalf("decode template-store item-use point change: %v", err)
	}
	if pointPacket.VID != owner.VID || pointPacket.Type != 7 || pointPacket.Amount != 25 || pointPacket.Value != 725 {
		t.Fatalf("unexpected template-store item-use point change: %+v", pointPacket)
	}
	setPacket, err := itemproto.DecodeSet(decodeSingleFrame(t, useOut[1]))
	if err != nil {
		t.Fatalf("decode template-store item-use set: %v", err)
	}
	if setPacket.Position.WindowType != itemproto.WindowInventory || setPacket.Position.Cell != 5 || setPacket.Vnum != 27002 || setPacket.Count != 2 {
		t.Fatalf("unexpected template-store item-use set packet: %+v", setPacket)
	}
	infoPacket, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, useOut[2]))
	if err != nil {
		t.Fatalf("decode template-store item-use info: %v", err)
	}
	if infoPacket.Type != chatproto.ChatTypeInfo || infoPacket.VID != 0 || infoPacket.Message != "consume:27002:+25" {
		t.Fatalf("unexpected template-store item-use info packet: %+v", infoPacket)
	}

	persisted, err := accounts.Load("owner")
	if err != nil {
		t.Fatalf("load persisted account after template-store item use: %v", err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected exactly 1 persisted owner after template-store item use, got %+v", persisted)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != 725 {
		t.Fatalf("expected persisted points[1] to follow template-store delta, got %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected persisted inventory after template-store item use: %#v", persisted.Characters[0].Inventory)
	}
}

func issuePeerTicket(t *testing.T, store loginticket.Store, login string, loginKey uint32, character loginticket.Character) {
	t.Helper()
	if err := store.Issue(loginticket.Ticket{Login: login, LoginKey: loginKey, Empire: character.Empire, Characters: []loginticket.Character{character}}); err != nil {
		t.Fatalf("issue peer ticket: %v", err)
	}
}

func newInteractionDefinitionStore(t *testing.T, definitions []interactionstore.Definition) interactionstore.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "interaction-definitions.json")
	store := interactionstore.NewFileStore(path)
	if err := store.Save(interactionstore.Snapshot{Definitions: definitions}); err != nil {
		t.Fatalf("save interaction definitions: %v", err)
	}
	return store
}

func newItemTemplateStore(t *testing.T, templates []itemcatalog.Template) itemcatalog.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "item-templates.json")
	store := itemcatalog.NewFileStore(path)
	if err := store.Save(itemcatalog.Snapshot{Templates: templates}); err != nil {
		t.Fatalf("save item templates: %v", err)
	}
	return store
}

func defaultMerchantItemTemplates() []itemcatalog.Template {
	return []itemcatalog.Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1},
		{
			Vnum:      27001,
			Name:      "Small Red Potion",
			Stackable: true,
			MaxCount:  200,
			UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
		},
	}
}

func defaultMerchantCatalogDefinition() interactionstore.Definition {
	return interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:merchant",
		Title: "Village Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27001, Price: 50, Count: 1},
			{Slot: 1, ItemVnum: 11200, Price: 500, Count: 1},
			{Slot: 2, ItemVnum: 27001, Price: 100, Count: 2},
		},
	}
}

func merchantCatalogDefinitionWithPotionCount(count uint16, price uint64) interactionstore.Definition {
	definition := defaultMerchantCatalogDefinition()
	definition.Catalog[2].Count = count
	definition.Catalog[2].Price = price
	return definition
}

func merchantBuyerCharacter(name string, id uint32, vid uint32, gold uint64, items []inventory.ItemInstance) loginticket.Character {
	character := peerVisibilityCharacter(name, id, vid, 1100, 2100, 0, 101, 201)
	character.Gold = gold
	character.Inventory = append([]inventory.ItemInstance(nil), items...)
	character.Equipment = []inventory.ItemInstance{}
	return character
}

func merchantBuyerFullInventory() []inventory.ItemInstance {
	items := make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		items = append(items, inventory.ItemInstance{ID: 1000 + uint64(slot), Vnum: 40000 + uint32(slot), Count: 1, Slot: slot})
	}
	return items
}

func merchantBuyerFullInventoryWithDistributedPotionCapacity() []inventory.ItemInstance {
	items := merchantBuyerFullInventory()
	items[5] = inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 199, Slot: 5}
	items[7] = inventory.ItemInstance{ID: 79, Vnum: 27001, Count: 199, Slot: 7}
	return items
}

func setupMerchantBuySession(t *testing.T, login string, loginKey uint32, buyer loginticket.Character) (*gameRuntime, accountstore.Store, service.SessionFlow, uint64, string) {
	return setupMerchantBuySessionWithCatalogDefinition(t, login, loginKey, buyer, defaultMerchantCatalogDefinition())
}

func setupMerchantBuySessionWithCatalogDefinition(t *testing.T, login string, loginKey uint32, buyer loginticket.Character, catalog interactionstore.Definition) (*gameRuntime, accountstore.Store, service.SessionFlow, uint64, string) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	issuePeerTicket(t, ticketStore, login, loginKey, buyer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed merchant buyer account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{catalog})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected merchant buy runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	if len(enterOut) < 8 {
		t.Fatalf("expected merchant buyer session bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}
	return runtime, accounts, flow, actor.EntityID, login
}

func interactWithMerchantForBuy(t *testing.T, flow service.SessionFlow, actorID uint64) {
	interactWithMerchantForBuyWithExpectedSlotTwo(t, flow, actorID, 2, 100)
}

func interactWithMerchantForBuyWithExpectedSlotTwo(t *testing.T, flow service.SessionFlow, actorID uint64, slotTwoCount uint8, slotTwoPrice uint32) {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actorID)})))
	if err != nil {
		t.Fatalf("unexpected merchant interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 merchant interaction frame before buy, got %d", len(out))
	}
	start, err := shopproto.DecodeServerStart(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode merchant interaction shop start: %v", err)
	}
	if start.OwnerVID != uint32(actorID) {
		t.Fatalf("expected merchant shop owner vid %d, got %d", actorID, start.OwnerVID)
	}
	if start.Items[0].Vnum != 27001 || start.Items[0].Price != 50 || start.Items[0].Count != 1 || start.Items[0].DisplayPos != 0 {
		t.Fatalf("unexpected merchant shop slot 0 before buy: %+v", start.Items[0])
	}
	if start.Items[1].Vnum != 11200 || start.Items[1].Price != 500 || start.Items[1].Count != 1 || start.Items[1].DisplayPos != 1 {
		t.Fatalf("unexpected merchant shop slot 1 before buy: %+v", start.Items[1])
	}
	if start.Items[2].Vnum != 27001 || start.Items[2].Price != slotTwoPrice || start.Items[2].Count != slotTwoCount || start.Items[2].DisplayPos != 2 {
		t.Fatalf("unexpected merchant shop slot 2 before buy: %+v", start.Items[2])
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
