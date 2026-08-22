package minimal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestLeavePersistsOwnedGroundItemDeletion(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	groundItemPath := filepath.Join(t.TempDir(), "ground-items.json")

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)

	owner := peerVisibilityCharacter("LeavePersistHero", 0x01030191, 0x02040191, 1100, 2100, 0, 101, 201)
	const (
		ownerLogin = "leave-persist-hero"
		ownerKey   = uint32(0x91919191)
		itemVID    = uint32(0x070000a1)
	)
	issuePeerTicket(t, ticketStore, ownerLogin, ownerKey, owner)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed owner account: %v", err)
	}

	cfg := config.Service{
		PprofAddr:           "127.0.0.1:6060",
		LegacyAddr:          ":13000",
		PublicAddr:          "127.0.0.1",
		LoginTicketStoreDir: ticketDir,
		AccountStoreDir:     accountDir,
		GroundItemStorePath: groundItemPath,
	}
	runtime, err := NewGameRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected runtime error: %v", err)
	}
	currentTime := time.Now().UTC().Truncate(time.Second)
	runtime.now = func() time.Time { return currentTime }
	runtime.sharedWorld.now = runtime.now

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, ownerKey)
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName(owner.Name)
	if !ok || ownerEntity.Entity.ID == 0 {
		t.Fatal("expected owner shared-world entity id after enter-game")
	}
	ownerID := ownerEntity.Entity.ID
	if !runtime.sharedWorld.RegisterGroundItemWithPickupRange(ownerID, ownerLogin, owner, itemVID, inventory.ItemInstance{ID: 0x300100a1, Vnum: 27001, Count: 2}, 450) {
		t.Fatal("expected ground-item registration before leave")
	}
	before, err := worldruntime.NewGroundItemFileStore(groundItemPath).Load()
	if err != nil {
		t.Fatalf("load ground items before leave: %v", err)
	}
	if len(before.GroundItems) != 1 || before.GroundItems[0].VID != itemVID {
		t.Fatalf("expected one persisted handle before leave, got %#v", before.GroundItems)
	}

	closeSessionFlow(t, ownerFlow)

	after, err := worldruntime.NewGroundItemFileStore(groundItemPath).Load()
	if err != nil {
		t.Fatalf("load ground items after leave: %v", err)
	}
	if len(after.GroundItems) != 0 {
		t.Fatalf("expected graceful leave to persist owned-ground deletion, still have %#v", after.GroundItems)
	}
	if runtime.sharedWorld.GroundItemExists(itemVID) {
		t.Fatal("expected live registry to drop owned ground item on leave")
	}
}

func TestLeavePersistsOwnedGroundGoldDeletion(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	groundItemPath := filepath.Join(t.TempDir(), "ground-items.json")

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)

	owner := peerVisibilityCharacter("LeavePersistGoldHero", 0x01030192, 0x02040192, 1100, 2100, 0, 101, 201)
	const (
		ownerLogin = "leave-persist-gold-hero"
		ownerKey   = uint32(0x92929292)
		goldVID    = uint32(0x070000a2)
	)
	issuePeerTicket(t, ticketStore, ownerLogin, ownerKey, owner)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed owner account: %v", err)
	}

	cfg := config.Service{
		PprofAddr:           "127.0.0.1:6060",
		LegacyAddr:          ":13000",
		PublicAddr:          "127.0.0.1",
		LoginTicketStoreDir: ticketDir,
		AccountStoreDir:     accountDir,
		GroundItemStorePath: groundItemPath,
	}
	runtime, err := NewGameRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected runtime error: %v", err)
	}
	currentTime := time.Now().UTC().Truncate(time.Second)
	runtime.now = func() time.Time { return currentTime }
	runtime.sharedWorld.now = runtime.now

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, ownerKey)
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName(owner.Name)
	if !ok || ownerEntity.Entity.ID == 0 {
		t.Fatal("expected owner shared-world entity id after enter-game")
	}
	ownerID := ownerEntity.Entity.ID
	if !runtime.sharedWorld.RegisterGroundGoldWithPickupRange(ownerID, ownerLogin, owner, goldVID, 55, 300) {
		t.Fatal("expected ground-gold registration before leave")
	}
	before, err := worldruntime.NewGroundItemFileStore(groundItemPath).Load()
	if err != nil {
		t.Fatalf("load ground gold before leave: %v", err)
	}
	if len(before.GroundItems) != 1 || before.GroundItems[0].VID != goldVID {
		t.Fatalf("expected one persisted gold handle before leave, got %#v", before.GroundItems)
	}

	closeSessionFlow(t, ownerFlow)

	after, err := worldruntime.NewGroundItemFileStore(groundItemPath).Load()
	if err != nil {
		t.Fatalf("load ground gold after leave: %v", err)
	}
	if len(after.GroundItems) != 0 {
		t.Fatalf("expected graceful leave to persist owned-ground-gold deletion, still have %#v", after.GroundItems)
	}
}

func TestStaleReclaimPersistsOwnedGroundItemDeletion(t *testing.T) {
	registry := newSharedWorldRegistry()
	var persisted []worldruntime.DurableGroundItemRecord
	registry.SetGroundItemsChangedHook(func() {
		persisted = append([]worldruntime.DurableGroundItemRecord(nil), registry.DurableGroundItemSnapshot().GroundItems...)
	})

	owner := peerVisibilityCharacter("ReclaimPersistOwner", 0x01030193, 0x02040193, 1100, 2100, 0, 101, 201)
	ownerID, _ := registry.Join(owner, newPendingServerFrames(), nil)
	if ownerID == 0 {
		t.Fatal("expected owner join")
	}
	const itemVID uint32 = 0x070000a3
	if !registry.RegisterGroundItem(ownerID, "reclaim-persist-owner", owner, itemVID, inventory.ItemInstance{ID: 0x300100a3, Vnum: 27001, Count: 1}) {
		t.Fatal("expected ground item registration")
	}
	if len(persisted) != 1 || persisted[0].VID != itemVID {
		t.Fatalf("expected register to persist one handle, got %#v", persisted)
	}

	if _, ok := registry.sessionDirectory.Remove(ownerID); !ok {
		t.Fatal("expected owner session entry removable for stale reclaim setup")
	}
	freshOwnerID, _ := registry.Join(owner, newPendingServerFrames(), nil)
	if freshOwnerID == 0 || freshOwnerID == ownerID {
		t.Fatalf("expected reclaim join with fresh id, old=%d fresh=%d", ownerID, freshOwnerID)
	}
	if registry.GroundItemExists(itemVID) {
		t.Fatal("expected stale reclaim to remove owned ground item")
	}
	if len(persisted) != 0 {
		t.Fatalf("expected stale reclaim to persist owned-ground deletion, still have %#v", persisted)
	}
}
