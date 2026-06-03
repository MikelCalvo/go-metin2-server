package minimal

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameRuntimeItemUseToItemRejectsLockedStacksWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name: "locked source",
			inventory: []inventory.ItemInstance{
				{ID: 1201, Vnum: 27001, Count: 2, Slot: 5, Locked: true},
				{ID: 1202, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
		{
			name: "locked target",
			inventory: []inventory.ItemInstance{
				{ID: 1201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1202, Vnum: 27001, Count: 3, Slot: 6, Locked: true},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
				Vnum:      27001,
				Name:      "Small Red Potion",
				Stackable: true,
				MaxCount:  200,
			}}}); err != nil {
				t.Fatalf("seed item template: %v", err)
			}
			owner := peerVisibilityCharacter("UseToItemLocked", 0x01030211, 0x02040211, 1300, 2300, 0, 101, 201)
			owner.Inventory = tc.inventory
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
			issuePeerTicket(t, ticketStore, "use-to-item-locked", 0x61616161, owner)
			if err := accounts.Save(accountstore.Account{Login: "use-to-item-locked", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed locked use-to-item owner account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected locked use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-locked", 0x61616161)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(6),
			})))
			if err != nil {
				t.Fatalf("unexpected locked use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected locked use-to-item to emit no frames, got %d", len(out))
			}
			account, err := accounts.Load("use-to-item-locked")
			if err != nil {
				t.Fatalf("load locked use-to-item owner account: %v", err)
			}
			if len(account.Characters) != 1 {
				t.Fatalf("expected one persisted locked use-to-item owner, got %+v", account)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("locked use-to-item mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("locked use-to-item mutated persisted quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsNonStackableTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Practice Relic",
		Stackable: false,
		MaxCount:  1,
	}}}); err != nil {
		t.Fatalf("seed non-stackable item template: %v", err)
	}
	owner := peerVisibilityCharacter("UseToItemNonStack", 0x01030212, 0x02040212, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1301, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 1302, Vnum: 27001, Count: 1, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	issuePeerTicket(t, ticketStore, "use-to-item-nonstack", 0x62626262, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-nonstack", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed non-stackable use-to-item owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected non-stackable use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-nonstack", 0x62626262)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected non-stackable use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected non-stackable use-to-item to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("use-to-item-nonstack")
	if err != nil {
		t.Fatalf("load non-stackable use-to-item owner account: %v", err)
	}
	if len(account.Characters) != 1 {
		t.Fatalf("expected one persisted non-stackable use-to-item owner, got %+v", account)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("non-stackable use-to-item mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("non-stackable use-to-item mutated persisted quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemMoveRejectsAntiStackTemplateCompatibleMergeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Bound Practice Potion",
		Stackable: true,
		MaxCount:  200,
		AntiStack: true,
	}}}); err != nil {
		t.Fatalf("seed anti-stack item template: %v", err)
	}
	owner := peerVisibilityCharacter("MoveAntiStack", 0x01030201, 0x02040201, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1101, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1102, Vnum: 27001, Count: 4, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "move-antistack-owner", 0x51515151, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-antistack-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed anti-stack move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected anti-stack item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-antistack-owner", 0x51515151)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected anti-stack item move error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected anti-stack compatible move to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("move-antistack-owner")
	if err != nil {
		t.Fatalf("load anti-stack move owner account: %v", err)
	}
	if len(account.Characters) != 1 {
		t.Fatalf("expected one persisted anti-stack move owner, got %+v", account)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("anti-stack compatible move mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
}

func TestGameSessionFlowPracticeMobItemMoveFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ItemMoveDeadOwner", 0x01030211, 0x02040211, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1101, Vnum: 27001, Count: 3, Slot: 5}}
	issuePeerTicket(t, store, "item-move-dead-owner", 0x52525252, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-move-dead-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed immediate zero-HP item-move owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected item-move/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000820, 0)
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
		t.Fatalf("import content practice-mob bundle for immediate zero-HP item-move denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before immediate zero-HP item-move denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-move-dead-owner", 0x52525252)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected item-move/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before immediate zero-HP item-move denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before immediate zero-HP item-move denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate zero-HP item-move denial: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate retaliation floor attack to emit 4 frames before immediate zero-HP item-move denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate zero-HP item-move denial setup, got %d", len(queued))
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected item move error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected item move to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(moveOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected immediate zero-HP item-move denial not to queue frames, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-move-dead-owner")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP item-move owner account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected immediate zero-HP item-move denial to keep persisted inventory unchanged, got %#v want %#v", persisted.Characters[0].Inventory, owner.Inventory)
	}
}
