package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestGameSessionFlowQuickslotAddRetargetsDuplicateItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("QuickslotRetarget", 0x0103058c, 0x0204058c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 401, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "quickslot-retarget", 0x5050508c, owner)
	if err := accounts.Save(accountstore.Account{Login: "quickslot-retarget", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed quickslot retarget account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected quickslot retarget runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quickslot-retarget", 0x5050508c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}})))
	if err != nil {
		t.Fatalf("unexpected quickslot retarget packet error: %v", err)
	}
	wantFrames := [][]byte{
		quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: 2}),
		quickslotproto.EncodeAdd(quickslotproto.AddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}}),
	}
	if !reflect.DeepEqual(out, wantFrames) {
		t.Fatalf("unexpected quickslot retarget frames:\n got %#v\nwant %#v", out, wantFrames)
	}
	persisted, err := accounts.Load("quickslot-retarget")
	if err != nil {
		t.Fatalf("load quickslot retarget account: %v", err)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 5},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after retarget: got %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameSessionFlowQuickslotSwapRejectsSamePositionWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("QuickslotSameSwap", 0x0103058e, 0x0204058e, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 421, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "quickslot-same-swap", 0x5050508e, owner)
	if err := accounts.Save(accountstore.Account{Login: "quickslot-same-swap", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed quickslot same-swap account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected quickslot same-swap runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quickslot-same-swap", 0x5050508e)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientSwap(quickslotproto.ClientSwapPacket{Position: 2, TargetPosition: 2})))
	if err != nil {
		t.Fatalf("unexpected quickslot same-swap packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected same-position quickslot swap to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("quickslot-same-swap")
	if err != nil {
		t.Fatalf("load quickslot same-swap account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("same-position quickslot swap mutated persisted quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("same-position quickslot swap mutated persisted inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
}

func TestGameSessionFlowQuickslotSwapRejectsBothEmptyPositionsWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("QuickslotEmptySwap", 0x01030590, 0x02040590, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 441, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "quickslot-empty-swap", 0x50505090, owner)
	if err := accounts.Save(accountstore.Account{Login: "quickslot-empty-swap", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed quickslot empty-swap account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected quickslot empty-swap runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quickslot-empty-swap", 0x50505090)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientSwap(quickslotproto.ClientSwapPacket{Position: 4, TargetPosition: 5})))
	if err != nil {
		t.Fatalf("unexpected quickslot empty-swap packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty-position quickslot swap to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("quickslot-empty-swap")
	if err != nil {
		t.Fatalf("load quickslot empty-swap account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("empty-position quickslot swap mutated persisted quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("empty-position quickslot swap mutated persisted inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
}

func TestGameSessionFlowQuickslotDelRejectsEmptyPositionWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("QuickslotEmptyDel", 0x0103058f, 0x0204058f, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 431, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}}
	issuePeerTicket(t, ticketStore, "quickslot-empty-del", 0x5050508f, owner)
	if err := accounts.Save(accountstore.Account{Login: "quickslot-empty-del", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed quickslot empty-delete account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected quickslot empty-delete runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quickslot-empty-del", 0x5050508f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientDel(quickslotproto.ClientDelPacket{Position: 4})))
	if err != nil {
		t.Fatalf("unexpected quickslot empty-delete packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty-position quickslot delete to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("quickslot-empty-del")
	if err != nil {
		t.Fatalf("load quickslot empty-delete account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("empty-position quickslot delete mutated persisted quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("empty-position quickslot delete mutated persisted inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
}

func TestGameSessionFlowQuickslotAddRejectsDuplicateItemSlotOccupancyWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("QuickslotDuplicateSlot", 0x0103058d, 0x0204058d, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 411, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 412, Vnum: 27002, Count: 1, Slot: 5},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}}
	issuePeerTicket(t, ticketStore, "quickslot-duplicate-slot", 0x5050508d, owner)
	if err := accounts.Save(accountstore.Account{Login: "quickslot-duplicate-slot", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed quickslot duplicate-slot account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected quickslot duplicate-slot runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quickslot-duplicate-slot", 0x5050508d)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}})))
	if err != nil {
		t.Fatalf("unexpected quickslot duplicate-slot packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected duplicate-slot quickslot add to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("quickslot-duplicate-slot")
	if err != nil {
		t.Fatalf("load quickslot duplicate-slot account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("duplicate-slot quickslot add mutated persisted quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("duplicate-slot quickslot add mutated persisted inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
}

func TestGameSessionFlowQuickslotAddStaleAfterReclaimIsSelfLocalOnly(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("QuickslotStale", 0x0103059a, 0x0204059a, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 451, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "quickslot-stale", 0x5050509a, owner)
	if err := accounts.Save(accountstore.Account{Login: "quickslot-stale", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed stale quickslot account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected stale quickslot runtime error: %v", err)
	}
	staleFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quickslot-stale", 0x5050509a)
	closeSessionFlow(t, staleFlow)

	issuePeerTicket(t, ticketStore, "quickslot-stale", 0x5050509b, owner)
	replacementFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quickslot-stale", 0x5050509b)
	defer closeSessionFlow(t, replacementFlow)

	out, err := staleFlow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}})))
	if err != nil {
		t.Fatalf("unexpected stale quickslot add packet error: %v", err)
	}
	wantFrames := [][]byte{
		quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: 2}),
		quickslotproto.EncodeAdd(quickslotproto.AddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}}),
	}
	if !reflect.DeepEqual(out, wantFrames) {
		t.Fatalf("unexpected stale quickslot self-local frames:\n got %#v\nwant %#v", out, wantFrames)
	}

	persisted, err := accounts.Load("quickslot-stale")
	if err != nil {
		t.Fatalf("load stale quickslot account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("stale quickslot add mutated authoritative quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("stale quickslot add mutated authoritative inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	live, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatalf("expected replacement live inventory snapshot")
	}
	if !reflect.DeepEqual(live.Inventory, []InventoryItemSnapshot{{ID: 451, Vnum: 27001, Count: 2, Slot: 5}}) {
		t.Fatalf("stale quickslot add replaced live replacement inventory: %+v", live.Inventory)
	}

	replacementOut, err := replacementFlow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}})))
	if err != nil {
		t.Fatalf("unexpected replacement quickslot add packet error: %v", err)
	}
	if !reflect.DeepEqual(replacementOut, wantFrames) {
		t.Fatalf("replacement quickslot add did not see authoritative original quickslot:\n got %#v\nwant %#v", replacementOut, wantFrames)
	}
}
