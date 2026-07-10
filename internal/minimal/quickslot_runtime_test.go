package minimal

import (
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	contentbundle "github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowQuickslotAddTypeNoneDeletesExistingQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("QuickslotNoneDel", 0x0103058b, 0x0204058b, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 391, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "quickslot-none-del", 0x5050508b, owner)
	if err := accounts.Save(accountstore.Account{Login: "quickslot-none-del", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed quickslot type-none delete account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected quickslot type-none delete runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "quickslot-none-del", 0x5050508b)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 2, Slot: quickslotproto.Slot{Type: quickslotproto.TypeNone, Position: 0}})))
	if err != nil {
		t.Fatalf("unexpected quickslot type-none delete packet error: %v", err)
	}
	wantFrames := [][]byte{quickslotproto.EncodeDel(quickslotproto.DelPacket{Position: 2})}
	if !reflect.DeepEqual(out, wantFrames) {
		t.Fatalf("unexpected quickslot type-none delete frames:\n got %#v\nwant %#v", out, wantFrames)
	}
	persisted, err := accounts.Load("quickslot-none-del")
	if err != nil {
		t.Fatalf("load quickslot type-none delete account: %v", err)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after type-none delete: got %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("type-none quickslot delete mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
}

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

func TestGameSessionFlowQuickslotEditsRejectAtBootstrapHPFloorWithoutMutation(t *testing.T) {
	cases := []struct {
		name   string
		login  string
		packet []byte
	}{
		{
			name:   "add",
			login:  "quickslot-floor-add",
			packet: quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}}),
		},
		{
			name:   "delete",
			login:  "quickslot-floor-del",
			packet: quickslotproto.EncodeClientDel(quickslotproto.ClientDelPacket{Position: 2}),
		},
		{
			name:   "swap",
			login:  "quickslot-floor-swap",
			packet: quickslotproto.EncodeClientSwap(quickslotproto.ClientSwapPacket{Position: 2, TargetPosition: 3}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("QuickslotFloor", 0x01030591, 0x02040591, 1100, 2100, 0, 101, 201)
			owner.Points[bootstrapPlayerPointValueIndex] = 0
			owner.Inventory = []inventory.ItemInstance{{ID: 451, Vnum: 27001, Count: 2, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{
				{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
				{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
			}
			issuePeerTicket(t, ticketStore, tc.login, 0x50505091, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed quickslot hp-floor %s account: %v", tc.name, err)
			}
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("unexpected quickslot hp-floor %s runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, 0x50505091)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, tc.packet))
			if err != nil {
				t.Fatalf("unexpected quickslot hp-floor %s packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected hp-floor quickslot %s to emit no frames, got %d", tc.name, len(out))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load quickslot hp-floor %s account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("hp-floor quickslot %s mutated persisted quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("hp-floor quickslot %s mutated persisted inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
		})
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

func TestGameSessionFlowPracticeMobQuickslotEditsFailClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	tests := []struct {
		name       string
		login      string
		key        uint32
		quickslots []loginticket.Quickslot
		request    func(t *testing.T, flow service.SessionFlow) [][]byte
	}{
		{
			name:  "add",
			login: "quickslot-floor-add",
			key:   0x50505101,
			quickslots: []loginticket.Quickslot{
				{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
			},
			request: func(t *testing.T, flow service.SessionFlow) [][]byte {
				t.Helper()
				out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{Position: 4, Slot: quickslotproto.Slot{Type: quickslotproto.TypeItem, Position: 5}})))
				if err != nil {
					t.Fatalf("unexpected post-floor quickslot add packet error: %v", err)
				}
				return out
			},
		},
		{
			name:  "delete",
			login: "quickslot-floor-del",
			key:   0x50505102,
			quickslots: []loginticket.Quickslot{
				{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
			},
			request: func(t *testing.T, flow service.SessionFlow) [][]byte {
				t.Helper()
				out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientDel(quickslotproto.ClientDelPacket{Position: 2})))
				if err != nil {
					t.Fatalf("unexpected post-floor quickslot delete packet error: %v", err)
				}
				return out
			},
		},
		{
			name:  "swap",
			login: "quickslot-floor-swap",
			key:   0x50505103,
			quickslots: []loginticket.Quickslot{
				{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
				{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
			},
			request: func(t *testing.T, flow service.SessionFlow) [][]byte {
				t.Helper()
				out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientSwap(quickslotproto.ClientSwapPacket{Position: 2, TargetPosition: 3})))
				if err != nil {
					t.Fatalf("unexpected post-floor quickslot swap packet error: %v", err)
				}
				return out
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("QuickslotFloor"+tt.name, 0x010305a0, 0x020405a0, 1100, 2100, 0, 101, 201)
			owner.Points[bootstrapPlayerPointValueIndex] = 1
			owner.Inventory = []inventory.ItemInstance{{ID: 501, Vnum: 27001, Count: 2, Slot: 5}}
			owner.Quickslots = append([]loginticket.Quickslot(nil), tt.quickslots...)
			issuePeerTicket(t, ticketStore, tt.login, tt.key, owner)
			if err := accounts.Save(accountstore.Account{Login: tt.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed post-floor quickslot account: %v", err)
			}

			staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
			interactionStore := newInteractionDefinitionStore(t, nil)
			runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore)
			if err != nil {
				t.Fatalf("unexpected post-floor quickslot runtime error: %v", err)
			}
			currentTime := time.Unix(1700000600, 0)
			runtime.now = func() time.Time { return currentTime }
			bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
				Ref:           "practice.quickslot_floor_" + tt.name,
				Name:          "QuickslotFloorMob" + tt.name,
				MapIndex:      bootstrapMapIndex,
				X:             1200,
				Y:             2200,
				RaceNum:       101,
				CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
			}}}
			if _, err := runtime.ImportContentBundle(bundle); err != nil {
				t.Fatalf("import content practice mob for post-floor quickslot denial: %v", err)
			}
			actors := runtime.StaticActors()
			if len(actors) != 1 {
				t.Fatalf("expected one content-loaded practice mob before post-floor quickslot denial, got %#v", actors)
			}
			targetVID := uint32(actors[0].EntityID)

			flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), tt.login, tt.key)
			defer closeSessionFlow(t, flow)
			if len(enterOut) < 8 {
				t.Fatalf("expected post-floor quickslot owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
			}
			selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
			if err != nil {
				t.Fatalf("unexpected target selection error before post-floor quickslot denial: %v", err)
			}
			if len(selectOut) != 1 {
				t.Fatalf("expected target selection to emit 1 frame before post-floor quickslot denial, got %d", len(selectOut))
			}
			attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
			if err != nil {
				t.Fatalf("unexpected attack error before post-floor quickslot denial: %v", err)
			}
			if len(attackOut) != 4 {
				t.Fatalf("expected immediate retaliation floor attack to emit 4 frames before post-floor quickslot denial, got %d", len(attackOut))
			}
			currentTime = currentTime.Add(time.Second)
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no delayed retaliation frames after post-floor quickslot denial setup, got %d", len(queued))
			}

			before, err := accounts.Load(tt.login)
			if err != nil {
				t.Fatalf("load persisted post-floor quickslot account before denial: %v", err)
			}
			out := tt.request(t, flow)
			if len(out) != 0 {
				t.Fatalf("expected post-floor quickslot %s to fail closed, got %d frames", tt.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected post-floor quickslot %s denial not to queue frames, got %d", tt.name, len(queued))
			}
			after, err := accounts.Load(tt.login)
			if err != nil {
				t.Fatalf("load persisted post-floor quickslot account after denial: %v", err)
			}
			if !reflect.DeepEqual(after.Characters[0].Quickslots, before.Characters[0].Quickslots) {
				t.Fatalf("expected post-floor quickslot %s denial to keep persisted quickslots unchanged, before=%+v after=%+v", tt.name, before.Characters[0].Quickslots, after.Characters[0].Quickslots)
			}
			if !reflect.DeepEqual(after.Characters[0].Inventory, before.Characters[0].Inventory) {
				t.Fatalf("expected post-floor quickslot %s denial to keep persisted inventory unchanged, before=%+v after=%+v", tt.name, before.Characters[0].Inventory, after.Characters[0].Inventory)
			}
		})
	}
}
