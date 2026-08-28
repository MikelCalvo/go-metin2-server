package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPostFloorItemGiveFailsClosedBeforeAntiGiveFeedback(t *testing.T) {
	login := "post-floor-item-give-owner"
	loginKey := uint32(0x19191a40)
	owner := peerVisibilityCharacter("DeadGiveOwner", 0x01030a40, 0x02040a40, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27042, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{
		Vnum:           27042,
		Name:           "Dead Guard Gift Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot give this item.",
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientGive(itemproto.ClientGivePacket{
		TargetVID: targetVID,
		Position:  itemproto.InventoryPosition(5),
		Count:     1,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor ITEM_GIVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor ITEM_GIVE to fail closed before anti-give feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor ITEM_GIVE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor ITEM_GIVE")
}

func TestGameSessionFlowPostFloorItemRefineFailsClosedBeforeRejectFeedback(t *testing.T) {
	login := "post-floor-item-refine-owner"
	loginKey := uint32(0x19191a50)
	owner := peerVisibilityCharacter("DeadRefineOwner", 0x01030a50, 0x02040a50, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 802, Vnum: 11201, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{
		Vnum:             11201,
		Name:             "Dead Guard Practice Blade",
		Stackable:        false,
		MaxCount:         1,
		RefineRejectText: "This item cannot be refined yet.",
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 2})))
	if err != nil {
		t.Fatalf("unexpected post-floor REFINE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor REFINE to fail closed before template reject feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor REFINE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor REFINE")
}

func TestGameSessionFlowPostFloorStoragePacketsFailClosedWithoutMutation(t *testing.T) {
	login := "post-floor-store"
	loginKey := uint32(0x19191a60)
	owner := peerVisibilityCharacter("DeadStorageOwner", 0x01030a60, 0x02040a60, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 803, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, nil)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	requests := []struct {
		name string
		raw  []byte
	}{
		{name: "safebox checkin", raw: itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})},
		{name: "safebox checkout", raw: itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{SafeSlot: 8, Position: itemproto.InventoryPosition(6)})},
		{name: "safebox item move", raw: itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{Source: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 7}, Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 8}, Count: 1})},
		{name: "mall checkout", raw: itemproto.EncodeClientMallCheckout(itemproto.ClientMallCheckoutPacket{MallSlot: 4, Position: itemproto.InventoryPosition(9)})},
	}
	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, request.raw))
			if err != nil {
				t.Fatalf("unexpected post-floor %s dispatch error: %v", request.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected post-floor %s to fail closed, got %d frames", request.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected post-floor %s to queue no frames, got %d", request.name, len(queued))
			}
			assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor "+request.name)
		})
	}
}

func TestGameSessionFlowPostFloorSafeboxCheckinFailsClosedBeforeAntiSafeboxFeedback(t *testing.T) {
	login := "post-floor-safe"
	loginKey := uint32(0x19191a70)
	owner := peerVisibilityCharacter("DeadSafeboxOwner", 0x01030a70, 0x02040a70, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 804, Vnum: 71127, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{
		Vnum:              71127,
		Name:              "Dead Guard Storage Charm",
		Stackable:         false,
		MaxCount:          1,
		AntiSafebox:       true,
		SafeboxRejectText: "This item cannot be placed in storage.",
	}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected post-floor SAFEBOX_CHECKIN dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to fail closed before anti-safebox feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor SAFEBOX_CHECKIN to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor SAFEBOX_CHECKIN")
}

func newPostFloorItemGuardRuntime(t *testing.T, login string, loginKey uint32, owner loginticket.Character, templates []itemcatalog.Template) (*gameRuntime, accountstore.Store, uint32) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed post-floor item-guard account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, staticActorStore, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected post-floor item-guard runtime error: %v", err)
	}

	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_post_floor_item_guard",
		Name:          "PracticeMobPostFloorItemGuard",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import post-floor item-guard practice mob: %v", err)
	}
	// ImportContentBundle replaces item templates from the bundle; re-seed the
	// authored templates needed by post-floor reopen recovery proofs.
	if len(templates) > 0 {
		if err := runtime.replaceItemTemplates(itemcatalog.Snapshot{Templates: templates}); err != nil {
			t.Fatalf("restore post-floor item-guard templates after content import: %v", err)
		}
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one post-floor item-guard practice mob, got %#v", actors)
	}
	return runtime, accounts, uint32(actors[0].EntityID)
}

func drivePracticeMobOwnerToBootstrapHPFloor(t *testing.T, flow service.SessionFlow, owner loginticket.Character, targetVID uint32) {
	t.Helper()
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection before post-floor item guard: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected one target-selection frame before post-floor item guard, got %d", len(selectOut))
	}

	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected practice-mob attack before post-floor item guard: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, point-change, self dead, and clear-target frames at HP floor, got %d frames", len(attackOut))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, attackOut[1]))
	if err != nil {
		t.Fatalf("decode post-floor item guard point-change: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != bootstrapPracticeMobRetaliationPointDelta || pointChange.Value != 0 {
		t.Fatalf("unexpected post-floor item guard point-change: %+v", pointChange)
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, attackOut[2]))
	if err != nil {
		t.Fatalf("decode post-floor item guard self dead: %v", err)
	}
	if dead.VID != owner.VID {
		t.Fatalf("expected self DEAD for owner %#08x, got %#08x", owner.VID, dead.VID)
	}
	clearTarget, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[3]))
	if err != nil {
		t.Fatalf("decode post-floor item guard target clear: %v", err)
	}
	if clearTarget.TargetVID != 0 || clearTarget.HPPercent != 0 {
		t.Fatalf("expected post-floor item guard to clear active target, got %+v", clearTarget)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-floor item guard owner death without peers, got %d", len(queued))
	}
}

func assertPostFloorItemGuardAccountUnchanged(t *testing.T, accounts accountstore.Store, login string, want loginticket.Character, context string) {
	t.Helper()
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted %s account: %v", context, err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected one persisted %s character, got %d", context, len(persisted.Characters))
	}
	got := persisted.Characters[0]
	if !samePostFloorItemGuardInventory(got.Inventory, want.Inventory) {
		t.Fatalf("%s mutated inventory: got %+v want %+v", context, got.Inventory, want.Inventory)
	}
	if !samePostFloorItemGuardQuickslots(got.Quickslots, want.Quickslots) {
		t.Fatalf("%s mutated quickslots: got %+v want %+v", context, got.Quickslots, want.Quickslots)
	}
	if !samePostFloorItemGuardInventory(got.Equipment, want.Equipment) {
		t.Fatalf("%s mutated equipment: got %+v want %+v", context, got.Equipment, want.Equipment)
	}
	if got.Gold != want.Gold {
		t.Fatalf("%s mutated gold: got %d want %d", context, got.Gold, want.Gold)
	}
	if got.Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("%s expected persisted death-floor HP 0, got %d", context, got.Points[bootstrapPlayerPointValueIndex])
	}
}

func samePostFloorItemGuardInventory(got []inventory.ItemInstance, want []inventory.ItemInstance) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}

func samePostFloorItemGuardQuickslots(got []loginticket.Quickslot, want []loginticket.Quickslot) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}
