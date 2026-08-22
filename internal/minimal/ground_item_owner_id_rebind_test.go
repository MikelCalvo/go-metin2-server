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

func TestSharedWorldRegistryRebindsExclusiveGroundOwnerIDOnMatchingJoin(t *testing.T) {
	registry := newSharedWorldRegistry()
	owner := peerVisibilityCharacter("RebindOwner", 0x01030181, 0x02040181, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("RebindPeer", 0x01030182, 0x02040182, 1110, 2110, 0, 101, 201)

	now := time.Now().UTC().Truncate(time.Second)
	ownershipExpires := now.Add(30 * time.Second)
	despawnAt := now.Add(5 * time.Minute)
	itemCount := uint16(2)
	const itemVID uint32 = 0x07000091
	if err := registry.RestorePersistedGroundItems([]worldruntime.DurableGroundItemRecord{{
		VID: itemVID, Vnum: 27001, ItemCount: &itemCount, ItemID: 0x30010091,
		OwnerLogin: "rebind-owner", OwnerCharacterID: owner.ID, OwnerVID: owner.VID, OwnerName: owner.Name,
		MapIndex: bootstrapMapIndex, X: owner.X, Y: owner.Y, Z: owner.Z, PickupRange: 450,
		OwnershipExclusive: true, OwnershipExpiresAt: &ownershipExpires, DespawnAt: despawnAt,
	}}); err != nil {
		t.Fatalf("restore exclusive ground item: %v", err)
	}

	ownerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	if ownerID == 0 {
		t.Fatal("expected owner join to allocate a shared-world entity id")
	}
	peerPending := newPendingServerFrames()
	peerID, _ := registry.Join(peer, peerPending, nil)
	if peerID == 0 {
		t.Fatal("expected peer join to allocate a shared-world entity id")
	}

	if _, ok := registry.GroundItemPickupFor(peerID, peer, itemVID); ok {
		t.Fatal("expected exclusive rematerialized handle to keep blocking peer after owner rebind")
	}
	pickup, ok := registry.GroundItemPickupFor(ownerID, owner, itemVID)
	if !ok {
		t.Fatal("expected exclusive rematerialized handle to allow matching owner pickup after rebind")
	}
	if pickup.OwnerID != ownerID {
		t.Fatalf("expected rematerialized exclusive OwnerID to rebind to join entity %d, got %d", ownerID, pickup.OwnerID)
	}
	if pickup.Item.ID != 0x30010091 || pickup.Item.Count != 2 {
		t.Fatalf("unexpected rebound pickup item: %+v", pickup.Item)
	}
}

func TestSharedWorldRegistryDoesNotRebindExclusiveGroundOwnerIDForNonMatchingJoin(t *testing.T) {
	registry := newSharedWorldRegistry()
	owner := peerVisibilityCharacter("KeepOwner", 0x01030183, 0x02040183, 1100, 2100, 0, 101, 201)
	stranger := peerVisibilityCharacter("KeepStranger", 0x01030184, 0x02040184, 1110, 2110, 0, 101, 201)

	now := time.Now().UTC().Truncate(time.Second)
	ownershipExpires := now.Add(30 * time.Second)
	despawnAt := now.Add(5 * time.Minute)
	itemCount := uint16(1)
	const itemVID uint32 = 0x07000092
	if err := registry.RestorePersistedGroundItems([]worldruntime.DurableGroundItemRecord{{
		VID: itemVID, Vnum: 27001, ItemCount: &itemCount, ItemID: 0x30010092,
		OwnerLogin: "keep-owner", OwnerCharacterID: owner.ID, OwnerVID: owner.VID, OwnerName: owner.Name,
		MapIndex: bootstrapMapIndex, X: owner.X, Y: owner.Y, Z: owner.Z, PickupRange: 450,
		OwnershipExclusive: true, OwnershipExpiresAt: &ownershipExpires, DespawnAt: despawnAt,
	}}); err != nil {
		t.Fatalf("restore exclusive ground item: %v", err)
	}

	strangerID, _ := registry.Join(stranger, newPendingServerFrames(), nil)
	if strangerID == 0 {
		t.Fatal("expected stranger join to allocate a shared-world entity id")
	}
	if pickup, ok := registry.GroundItemPickupFor(strangerID, stranger, itemVID); ok {
		t.Fatalf("expected stranger join not to claim exclusive rematerialized handle, got pickup=%+v", pickup)
	}

	ownerID, _ := registry.Join(owner, newPendingServerFrames(), nil)
	if ownerID == 0 {
		t.Fatal("expected owner join to allocate a shared-world entity id")
	}
	pickup, ok := registry.GroundItemPickupFor(ownerID, owner, itemVID)
	if !ok || pickup.OwnerID != ownerID {
		t.Fatalf("expected later matching owner join to rebind OwnerID=%d, got ok=%v pickup=%+v", ownerID, ok, pickup)
	}
}

func TestSharedWorldRegistryDoesNotRebindAlreadyBoundOrPublicGroundOwnerID(t *testing.T) {
	registry := newSharedWorldRegistry()
	owner := peerVisibilityCharacter("BoundOwner", 0x01030185, 0x02040185, 1100, 2100, 0, 101, 201)
	other := peerVisibilityCharacter("BoundOther", 0x01030186, 0x02040186, 1110, 2110, 0, 101, 201)

	ownerID, _ := registry.Join(owner, newPendingServerFrames(), nil)
	if ownerID == 0 {
		t.Fatal("expected owner join")
	}
	const liveVID uint32 = 0x07000093
	if !registry.RegisterGroundItem(ownerID, "bound-owner", owner, liveVID, inventory.ItemInstance{ID: 0x30010093, Vnum: 27001, Count: 1}) {
		t.Fatal("expected live exclusive registration")
	}
	livePickup, ok := registry.GroundItemPickupFor(ownerID, owner, liveVID)
	if !ok || livePickup.OwnerID != ownerID {
		t.Fatalf("expected live registration OwnerID=%d, got ok=%v pickup=%+v", ownerID, ok, livePickup)
	}

	now := time.Now().UTC().Truncate(time.Second)
	despawnAt := now.Add(5 * time.Minute)
	goldAmount := uint32(40)
	const publicVID uint32 = 0x07000094
	if err := registry.RestorePersistedGroundItems([]worldruntime.DurableGroundItemRecord{{
		VID: publicVID, Vnum: 1, GoldAmount: &goldAmount,
		OwnerLogin: "bound-owner", OwnerCharacterID: owner.ID, OwnerVID: owner.VID, OwnerName: owner.Name,
		MapIndex: bootstrapMapIndex, X: owner.X, Y: owner.Y, Z: owner.Z, PickupRange: 300,
		OwnershipExclusive: false, DespawnAt: despawnAt,
	}}); err != nil {
		t.Fatalf("restore public gold: %v", err)
	}

	// Rejoin after removing the live session entry without Leave cleanup so the
	// already-bound exclusive handle remains while a fresh entity id is allocated.
	if _, ok := registry.sessionDirectory.Remove(ownerID); !ok {
		t.Fatal("expected owner session entry removable for reclaim setup")
	}
	freshOwnerID, _ := registry.Join(owner, newPendingServerFrames(), nil)
	if freshOwnerID == 0 || freshOwnerID == ownerID {
		t.Fatalf("expected reclaim join with fresh id, old=%d fresh=%d", ownerID, freshOwnerID)
	}
	// Stale reclaim removes OwnerID-matched live handles; public rematerialized
	// gold must remain OwnerID=0 and still pickable by the fresh owner.
	if registry.GroundItemExists(liveVID) {
		t.Fatal("expected stale reclaim to remove already-bound live exclusive handle")
	}
	publicPickup, ok := registry.GroundItemPickupFor(freshOwnerID, owner, publicVID)
	if !ok {
		t.Fatal("expected public rematerialized gold to stay pickable")
	}
	if publicPickup.OwnerID != 0 {
		t.Fatalf("expected public rematerialized gold to stay OwnerID=0, got %d", publicPickup.OwnerID)
	}

	otherID, _ := registry.Join(other, newPendingServerFrames(), nil)
	if otherID == 0 {
		t.Fatal("expected other join")
	}
	if _, ok := registry.GroundItemPickupFor(otherID, other, publicVID); !ok {
		t.Fatal("expected public rematerialized gold to stay pickable by peers")
	}
}

func TestPendingGroundItemExclusiveOwnerIDRebindsOnOwnerRejoin(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	groundItemPath := filepath.Join(t.TempDir(), "ground-items.json")

	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)

	owner := peerVisibilityCharacter("OwnerIDRebindHero", 0x01030187, 0x02040187, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("OwnerIDRebindPeer", 0x01030188, 0x02040188, 1110, 2110, 0, 101, 201)
	const (
		ownerLogin = "owner-id-rebind-hero"
		peerLogin  = "owner-id-rebind-peer"
		ownerKey   = uint32(0x81818181)
		peerKey    = uint32(0x82828282)
		itemVID    = uint32(0x07000095)
	)
	issuePeerTicket(t, ticketStore, ownerLogin, ownerKey, owner)
	issuePeerTicket(t, ticketStore, peerLogin, peerKey, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed peer account: %v", err)
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
	if !runtime.sharedWorld.RegisterGroundItemWithPickupRange(ownerID, ownerLogin, owner, itemVID, inventory.ItemInstance{ID: 0x30010095, Vnum: 27001, Count: 2}, 450) {
		t.Fatal("expected ground-item registration before daemon restart")
	}
	closeSessionFlow(t, ownerFlow)

	reloaded, err := NewGameRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected post-restart runtime error: %v", err)
	}
	reloaded.now = func() time.Time { return currentTime.Add(5 * time.Second) }
	reloaded.sharedWorld.now = reloaded.now

	ownerRestartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), ownerLogin, ownerKey)
	peerRestartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), peerLogin, peerKey)
	ownerEntity, ok = reloaded.sharedWorld.playerEntityByName(owner.Name)
	peerEntity, peerOK := reloaded.sharedWorld.playerEntityByName(peer.Name)
	if !ok || !peerOK || ownerEntity.Entity.ID == 0 || peerEntity.Entity.ID == 0 {
		t.Fatalf("expected rematerialized owner/peer entity ids, ownerOK=%v peerOK=%v", ok, peerOK)
	}
	ownerID = ownerEntity.Entity.ID
	peerID := peerEntity.Entity.ID

	if _, ok := reloaded.sharedWorld.GroundItemPickupFor(peerID, peer, itemVID); ok {
		t.Fatal("expected rematerialized exclusive ownership to block peer mid-window")
	}
	pickup, ok := reloaded.sharedWorld.GroundItemPickupFor(ownerID, owner, itemVID)
	if !ok || pickup.Item.ID != 0x30010095 || pickup.Item.Count != 2 {
		t.Fatalf("expected rematerialized exclusive ownership to allow owner pickup, ok=%v pickup=%+v", ok, pickup)
	}
	if pickup.OwnerID != ownerID {
		t.Fatalf("expected rematerialized exclusive OwnerID to rebind on owner rejoin to %d, got %d", ownerID, pickup.OwnerID)
	}

	// Rebind is process-local; durable snapshot must still omit OwnerID and keep timers.
	durable := reloaded.sharedWorld.DurableGroundItemSnapshot()
	if len(durable.GroundItems) != 1 || durable.GroundItems[0].VID != itemVID || !durable.GroundItems[0].OwnershipExclusive {
		t.Fatalf("unexpected durable snapshot after OwnerID rebind: %#v", durable.GroundItems)
	}

	closeSessionFlow(t, ownerRestartFlow)
	closeSessionFlow(t, peerRestartFlow)
}
