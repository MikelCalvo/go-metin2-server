package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRuntimePickupGroundItemCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	sourceActiveSockets := inventory.SocketValues{1, 2, 3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destZeroSockets := inventory.SocketValues{}
	destZeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name              string
		destinationSocks  *inventory.SocketValues
		destinationAttrs  *inventory.AttributeValues
		sourceSocks       *inventory.SocketValues
		sourceAttrs       *inventory.AttributeValues
		wantHasSockets    bool
		wantHasAttributes bool
		wantSockets       inventory.SocketValues
		wantAttributes    inventory.AttributeValues
	}{
		{
			name:              "active destination wins over different source",
			destinationSocks:  &destActiveSockets,
			destinationAttrs:  &destActiveAttributes,
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    true,
			wantHasAttributes: true,
			wantSockets:       destActiveSockets,
			wantAttributes:    destActiveAttributes,
		},
		{
			name:              "explicit-zero destination wins over active source",
			destinationSocks:  &destZeroSockets,
			destinationAttrs:  &destZeroAttributes,
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    true,
			wantHasAttributes: true,
			wantSockets:       destZeroSockets,
			wantAttributes:    destZeroAttributes,
		},
		{
			name:              "omitted destination stays omitted",
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    false,
			wantHasAttributes: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:   0x01030901,
				VID:  0x02040901,
				Name: "MergeDestWinsPickup",
				Inventory: []inventory.ItemInstance{{
					ID:         11,
					Vnum:       27001,
					Count:      4,
					Slot:       0,
					Sockets:    tc.destinationSocks,
					Attributes: tc.destinationAttrs,
				}},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "merge-dest-wins-pickup", CharacterIndex: 1})
			ground := inventory.ItemInstance{
				ID:         13,
				Vnum:       27001,
				Count:      3,
				Slot:       6,
				Sockets:    tc.sourceSocks,
				Attributes: tc.sourceAttrs,
			}
			result, ok := runtime.PickupGroundItem(ground, 6, 200)
			if !ok {
				t.Fatal("expected compatible pickup merge to succeed")
			}
			if !result.Merged || result.Split || result.Placed.ID != 0 {
				t.Fatalf("expected pure merge pickup result, got %+v", result)
			}
			if result.Updated.ID != 11 || result.Updated.Slot != 0 || result.Updated.Count != 7 {
				t.Fatalf("unexpected merge updated stack: %+v", result.Updated)
			}
			assertDestinationPresenceWins(t, result.Updated, ground, tc.wantHasSockets, tc.wantHasAttributes, tc.wantSockets, tc.wantAttributes)

			live := runtime.LiveInventory()
			if len(live) != 1 || live[0].ID != 11 || live[0].Count != 7 || live[0].Slot != 0 {
				t.Fatalf("unexpected live inventory after merge: %#v", live)
			}
			assertDestinationPresenceWins(t, live[0], ground, tc.wantHasSockets, tc.wantHasAttributes, tc.wantSockets, tc.wantAttributes)
		})
	}
}

func TestRuntimePickupGroundItemFreeCellPreservesSourceInstancePresence(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
	}{
		{name: "active sockets and attributes", sockets: &activeSockets, attributes: &activeAttributes},
		{name: "explicit zero sockets and attributes", sockets: &zeroSockets, attributes: &zeroAttributes},
		{name: "omitted presence"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(loginticket.Character{
				ID:   0x01030902,
				VID:  0x02040902,
				Name: "MergeFreeCellPreservePickup",
			}, SessionLink{Login: "merge-free-cell-pickup", CharacterIndex: 1})

			ground := inventory.ItemInstance{
				ID:         21,
				Vnum:       27001,
				Count:      3,
				Slot:       6,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}
			result, ok := runtime.PickupGroundItem(ground, 6, 200)
			if !ok {
				t.Fatal("expected free-cell pickup placement to succeed")
			}
			if result.Merged || result.Split || result.Placed.ID != 21 || result.Placed.Slot != 6 || result.Placed.Count != 3 {
				t.Fatalf("unexpected free-cell pickup result: %+v", result)
			}
			assertIndependentPickupPlacementPresence(t, ground, result.Placed, "pickup result placement")
			if len(runtime.liveInventory) != 1 {
				t.Fatalf("expected one live inventory item after free-cell pickup, got %#v", runtime.liveInventory)
			}
			assertIndependentPickupPlacementPresence(t, ground, runtime.liveInventory[0], "live placement")
		})
	}
}

func TestRuntimePickupGroundItemSplitRemainderPreservesSourceInstancePresence(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
	}{
		{name: "active sockets and attributes", sockets: &activeSockets, attributes: &activeAttributes},
		{name: "explicit zero sockets and attributes", sockets: &zeroSockets, attributes: &zeroAttributes},
		{name: "omitted presence"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(loginticket.Character{
				ID:   0x01030912,
				VID:  0x02040912,
				Name: "SplitRemainderPreservePickup",
				Inventory: []inventory.ItemInstance{
					{ID: 11, Vnum: 27001, Count: 198, Slot: 0},
					{ID: 12, Vnum: 27001, Count: 199, Slot: 2},
				},
			}, SessionLink{Login: "split-remainder-pickup", CharacterIndex: 1})

			ground := inventory.ItemInstance{
				ID:         13,
				Vnum:       27001,
				Count:      5,
				Slot:       6,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}
			result, ok := runtime.PickupGroundItem(ground, 6, 200)
			if !ok {
				t.Fatal("expected split pickup to fill compatible stacks and place the remainder")
			}
			if !result.Split || result.Merged || result.Placed.ID != 13 || result.Placed.Slot != 6 || result.Placed.Count != 2 {
				t.Fatalf("unexpected split pickup result: %+v", result)
			}
			if len(result.UpdatedItems) != 2 || result.UpdatedItems[0].ID != 11 || result.UpdatedItems[0].Count != 200 || result.UpdatedItems[1].ID != 12 || result.UpdatedItems[1].Count != 200 {
				t.Fatalf("unexpected split pickup updated stacks: %#v", result.UpdatedItems)
			}
			if result.UpdatedItems[0].HasSockets() || result.UpdatedItems[0].HasAttributes() || result.UpdatedItems[1].HasSockets() || result.UpdatedItems[1].HasAttributes() {
				t.Fatalf("expected destination-wins omitted presence on filled stacks, got %#v", result.UpdatedItems)
			}
			assertIndependentPickupPlacementPresence(t, ground, result.Placed, "split remainder placement")
			if len(runtime.liveInventory) != 3 {
				t.Fatalf("expected three live inventory items after split pickup, got %#v", runtime.liveInventory)
			}
			assertIndependentPickupPlacementPresence(t, ground, runtime.liveInventory[2], "live split remainder")
			if runtime.liveInventory[0].HasSockets() || runtime.liveInventory[0].HasAttributes() || runtime.liveInventory[1].HasSockets() || runtime.liveInventory[1].HasAttributes() {
				t.Fatalf("expected live filled stacks to stay omitted, got %#v", runtime.liveInventory)
			}
		})
	}
}

func assertIndependentPickupPlacementPresence(t *testing.T, ground, placed inventory.ItemInstance, label string) {
	t.Helper()
	if placed.HasSockets() != ground.HasSockets() {
		t.Fatalf("%s HasSockets=%v want %v", label, placed.HasSockets(), ground.HasSockets())
	}
	if placed.HasAttributes() != ground.HasAttributes() {
		t.Fatalf("%s HasAttributes=%v want %v", label, placed.HasAttributes(), ground.HasAttributes())
	}
	if ground.HasSockets() {
		if placed.Sockets == ground.Sockets {
			t.Fatalf("%s expected pickup placement to clone sockets independently from the ground snapshot pointer", label)
		}
		want := *ground.Sockets
		if *placed.Sockets != want {
			t.Fatalf("%s expected placement sockets %+v, got %+v", label, want, *placed.Sockets)
		}
		(*placed.Sockets)[0] = 99
		if (*ground.Sockets)[0] == 99 {
			t.Fatalf("%s mutating placement sockets aliased the ground snapshot pointer", label)
		}
		*placed.Sockets = want
	} else if placed.Sockets != nil {
		t.Fatalf("%s expected omitted placement sockets, got %#v", label, placed.Sockets)
	}
	if ground.HasAttributes() {
		if placed.Attributes == ground.Attributes {
			t.Fatalf("%s expected pickup placement to clone attributes independently from the ground snapshot pointer", label)
		}
		want := *ground.Attributes
		if *placed.Attributes != want {
			t.Fatalf("%s expected placement attributes %+v, got %+v", label, want, *placed.Attributes)
		}
		(*placed.Attributes)[0].Value = 99
		if (*ground.Attributes)[0].Value == 99 {
			t.Fatalf("%s mutating placement attributes aliased the ground snapshot pointer", label)
		}
		*placed.Attributes = want
	} else if placed.Attributes != nil {
		t.Fatalf("%s expected omitted placement attributes, got %#v", label, placed.Attributes)
	}
}

func TestRuntimePickupGroundItemCompatibleMergeNegativesStayNonMutatingOrSourcePreserving(t *testing.T) {
	t.Run("locked compatible skipped then free-cell preserve", func(t *testing.T) {
		destSockets := inventory.SocketValues{7, 0, 9}
		sourceSockets := inventory.SocketValues{1, 2, 3}
		runtime := NewRuntime(loginticket.Character{
			ID:   0x01030903,
			VID:  0x02040903,
			Name: "MergeLockedSkipPickup",
			Inventory: []inventory.ItemInstance{{
				ID: 31, Vnum: 27001, Count: 4, Slot: 0, Locked: true,
				Sockets: &destSockets,
			}},
		}, SessionLink{Login: "merge-locked-skip-pickup", CharacterIndex: 1})
		ground := inventory.ItemInstance{ID: 33, Vnum: 27001, Count: 3, Slot: 6, Sockets: &sourceSockets}
		result, ok := runtime.PickupGroundItem(ground, 6, 200)
		if !ok {
			t.Fatal("expected locked-compatible pickup to fall through to free-cell placement")
		}
		if result.Merged || result.Placed.ID != 33 || result.Placed.Slot != 6 {
			t.Fatalf("expected free-cell placement after locked skip, got %+v", result)
		}
		if !result.Placed.HasSockets() || *result.Placed.Sockets != sourceSockets {
			t.Fatalf("expected source-preserving free-cell after locked skip, got %#v", result.Placed.Sockets)
		}
		live := runtime.LiveInventory()
		if len(live) != 2 || !live[0].Locked || live[0].Count != 4 || !live[0].HasSockets() || *live[0].Sockets != destSockets {
			t.Fatalf("locked destination mutated: %#v", live)
		}
	})

	t.Run("over max rejects without mutation", func(t *testing.T) {
		destSockets := inventory.SocketValues{7, 0, 9}
		sourceSockets := inventory.SocketValues{1, 2, 3}
		persisted := loginticket.Character{
			ID:   0x01030904,
			VID:  0x02040904,
			Name: "MergeOverMaxPickup",
			Inventory: []inventory.ItemInstance{{
				ID: 41, Vnum: 27001, Count: 199, Slot: 0,
				Sockets: &destSockets,
			}},
		}
		for slot := inventory.SlotIndex(1); slot < inventory.CarriedInventorySlotCount; slot++ {
			persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{
				ID: uint64(1000 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot,
			})
		}
		runtime := NewRuntime(persisted, SessionLink{Login: "merge-over-max-pickup", CharacterIndex: 1})
		before := runtime.LiveCharacter()
		if result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 43, Vnum: 27001, Count: 2, Slot: 6, Sockets: &sourceSockets}, 6, 200); ok {
			t.Fatalf("expected over-max pickup merge without free capacity to fail closed, got %+v", result)
		}
		if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
			t.Fatalf("over-max pickup mutated live state:\ngot:  %+v\nwant: %+v", got, before)
		}
	})
}

func TestRuntimeSafeboxCheckoutItemCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	sourceActiveSockets := inventory.SocketValues{1, 2, 3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destZeroSockets := inventory.SocketValues{}
	destZeroAttributes := inventory.AttributeValues{}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	cases := []struct {
		name              string
		destinationSocks  *inventory.SocketValues
		destinationAttrs  *inventory.AttributeValues
		sourceSocks       *inventory.SocketValues
		sourceAttrs       *inventory.AttributeValues
		wantHasSockets    bool
		wantHasAttributes bool
		wantSockets       inventory.SocketValues
		wantAttributes    inventory.AttributeValues
	}{
		{
			name:              "active destination wins over different source",
			destinationSocks:  &destActiveSockets,
			destinationAttrs:  &destActiveAttributes,
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    true,
			wantHasAttributes: true,
			wantSockets:       destActiveSockets,
			wantAttributes:    destActiveAttributes,
		},
		{
			name:              "explicit-zero destination wins over active source",
			destinationSocks:  &destZeroSockets,
			destinationAttrs:  &destZeroAttributes,
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    true,
			wantHasAttributes: true,
			wantSockets:       destZeroSockets,
			wantAttributes:    destZeroAttributes,
		},
		{
			name:              "omitted destination stays omitted",
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    false,
			wantHasAttributes: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:    0x01030911,
				VID:   0x02040911,
				Name:  "MergeDestWinsSafebox",
				Level: 1,
				Inventory: []inventory.ItemInstance{{
					ID:         121,
					Vnum:       27001,
					Count:      4,
					Slot:       5,
					Sockets:    tc.destinationSocks,
					Attributes: tc.destinationAttrs,
				}},
				Gold: 55,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "merge-dest-wins-safebox", CharacterIndex: 1})
			item := inventory.ItemInstance{
				ID:         123,
				Vnum:       27001,
				Count:      2,
				Slot:       0,
				Sockets:    tc.sourceSocks,
				Attributes: tc.sourceAttrs,
			}
			result, ok := runtime.SafeboxCheckoutItem(5, item, template)
			if !ok {
				t.Fatal("expected safebox check-out merge to succeed")
			}
			if !result.Merged || result.Destination != 5 || result.Item.ID != 121 || result.Item.Count != 6 || result.Item.Slot != 5 {
				t.Fatalf("unexpected safebox check-out merge result: %+v", result)
			}
			assertDestinationPresenceWins(t, result.Item, item, tc.wantHasSockets, tc.wantHasAttributes, tc.wantSockets, tc.wantAttributes)

			live := runtime.LiveCharacter()
			if len(live.Inventory) != 1 || live.Inventory[0].ID != 121 || live.Inventory[0].Count != 6 {
				t.Fatalf("unexpected live inventory after safebox merge: %#v", live.Inventory)
			}
			assertDestinationPresenceWins(t, live.Inventory[0], item, tc.wantHasSockets, tc.wantHasAttributes, tc.wantSockets, tc.wantAttributes)
			if got := runtime.PersistedSnapshot(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) {
				t.Fatalf("safebox check-out merge mutated persisted inventory early: got %#v want %#v", got.Inventory, persisted.Inventory)
			}
		})
	}
}

func TestRuntimeSafeboxCheckoutItemFreeCellPreservesSourceInstancePresence(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
	}{
		{name: "active sockets and attributes", sockets: &activeSockets, attributes: &activeAttributes},
		{name: "explicit zero sockets and attributes", sockets: &zeroSockets, attributes: &zeroAttributes},
		{name: "omitted sockets and attributes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:    0x01030912,
				VID:   0x02040912,
				Name:  "CheckoutFreeCellPreserve",
				Level: 1,
				Inventory: []inventory.ItemInstance{
					{ID: 120, Vnum: 27002, Count: 1, Slot: 6},
				},
				Gold: 4242,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "checkout-free-cell-preserve", CharacterIndex: 1})
			item := inventory.ItemInstance{
				ID:         119,
				Vnum:       27001,
				Count:      3,
				Slot:       0,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}

			result, ok := runtime.SafeboxCheckoutItem(5, item, template)
			if !ok {
				t.Fatal("expected safebox check-out into empty destination to succeed")
			}
			if result.Merged || result.Destination != 5 || result.Item.ID != 119 || result.Item.Vnum != 27001 || result.Item.Count != 3 || result.Item.Slot != 5 {
				t.Fatalf("unexpected safebox check-out empty placement: %+v", result)
			}
			if (tc.sockets != nil) != result.Item.HasSockets() {
				t.Fatalf("HasSockets=%v want %v", result.Item.HasSockets(), tc.sockets != nil)
			}
			if (tc.attributes != nil) != result.Item.HasAttributes() {
				t.Fatalf("HasAttributes=%v want %v", result.Item.HasAttributes(), tc.attributes != nil)
			}
			if tc.sockets != nil {
				if result.Item.Sockets == nil || *result.Item.Sockets != *tc.sockets {
					t.Fatalf("expected preserved sockets %+v, got %#v", *tc.sockets, result.Item.Sockets)
				}
				if result.Item.Sockets == item.Sockets {
					t.Fatal("expected free-cell checkout to clone sockets independently from the seed pointer")
				}
			} else if result.Item.Sockets != nil {
				t.Fatalf("expected omitted sockets, got %#v", result.Item.Sockets)
			}
			if tc.attributes != nil {
				if result.Item.Attributes == nil || *result.Item.Attributes != *tc.attributes {
					t.Fatalf("expected preserved attributes %+v, got %#v", *tc.attributes, result.Item.Attributes)
				}
				if result.Item.Attributes == item.Attributes {
					t.Fatal("expected free-cell checkout to clone attributes independently from the seed pointer")
				}
			} else if result.Item.Attributes != nil {
				t.Fatalf("expected omitted attributes, got %#v", result.Item.Attributes)
			}

			live := runtime.LiveCharacter()
			if len(live.Inventory) != 2 || live.Inventory[0].ID != 119 || live.Inventory[0].Slot != 5 {
				t.Fatalf("unexpected live inventory after free-cell checkout: %#v", live.Inventory)
			}
			got := live.Inventory[0]
			if (tc.sockets != nil) != got.HasSockets() {
				t.Fatalf("live HasSockets=%v want %v", got.HasSockets(), tc.sockets != nil)
			}
			if (tc.attributes != nil) != got.HasAttributes() {
				t.Fatalf("live HasAttributes=%v want %v", got.HasAttributes(), tc.attributes != nil)
			}
			if tc.sockets != nil && (got.Sockets == nil || *got.Sockets != *tc.sockets) {
				t.Fatalf("live sockets drifted: %#v", got.Sockets)
			}
			if tc.attributes != nil && (got.Attributes == nil || *got.Attributes != *tc.attributes) {
				t.Fatalf("live attributes drifted: %#v", got.Attributes)
			}
			if got := runtime.PersistedSnapshot(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) {
				t.Fatalf("safebox free-cell check-out mutated persisted inventory early: got %#v want %#v", got.Inventory, persisted.Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	destSockets := inventory.SocketValues{7, 0, 9}
	destAttributes := inventory.AttributeValues{{Type: 1, Value: 25}}
	sourceSockets := inventory.SocketValues{1, 2, 3}
	sourceAttributes := inventory.AttributeValues{{Type: 4, Value: 55}}
	runtime := NewRuntime(loginticket.Character{
		ID:   0x01030921,
		VID:  0x02040921,
		Name: "MergeDestWinsUseToItem",
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 4, Slot: 5, Sockets: &sourceSockets, Attributes: &sourceAttributes},
			{ID: 12, Vnum: 27001, Count: 6, Slot: 6, Sockets: &destSockets, Attributes: &destAttributes},
		},
	}, SessionLink{Login: "merge-dest-wins-use-to-item", CharacterIndex: 1})

	result, ok := runtime.UseItemOnItem(5, 6, itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 10})
	if !ok {
		t.Fatal("expected full item-use-to-item merge to succeed")
	}
	if !result.Changed || result.FromOccupied || !result.ToOccupied || !result.CountOnly {
		t.Fatalf("unexpected full merge result flags: %+v", result)
	}
	if result.ToItem.ID != 12 || result.ToItem.Count != 10 || result.ToItem.Slot != 6 {
		t.Fatalf("unexpected full merge target item: %+v", result.ToItem)
	}
	if !result.ToItem.HasSockets() || result.ToItem.Sockets == nil || *result.ToItem.Sockets != destSockets {
		t.Fatalf("expected destination sockets to win, got %#v", result.ToItem.Sockets)
	}
	if !result.ToItem.HasAttributes() || result.ToItem.Attributes == nil || *result.ToItem.Attributes != destAttributes {
		t.Fatalf("expected destination attributes to win, got %#v", result.ToItem.Attributes)
	}

	live := runtime.LiveCharacter().Inventory
	if len(live) != 1 || live[0].ID != 12 || live[0].Count != 10 {
		t.Fatalf("unexpected live inventory after use-to-item merge: %#v", live)
	}
	if !live[0].HasSockets() || *live[0].Sockets != destSockets {
		t.Fatalf("live destination sockets drifted: %#v", live[0].Sockets)
	}
	if !live[0].HasAttributes() || *live[0].Attributes != destAttributes {
		t.Fatalf("live destination attributes drifted: %#v", live[0].Attributes)
	}
}

func TestRuntimeMoveInventoryItemCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	sourceActiveSockets := inventory.SocketValues{1, 2, 3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destZeroSockets := inventory.SocketValues{}
	destZeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name              string
		destinationSocks  *inventory.SocketValues
		destinationAttrs  *inventory.AttributeValues
		sourceSocks       *inventory.SocketValues
		sourceAttrs       *inventory.AttributeValues
		wantHasSockets    bool
		wantHasAttributes bool
		wantSockets       inventory.SocketValues
		wantAttributes    inventory.AttributeValues
		count             uint16
		wantSourceRemain  bool
		wantSourceCount   uint16
		wantDestCount     uint16
	}{
		{
			name:              "active destination wins on partial counted merge",
			destinationSocks:  &destActiveSockets,
			destinationAttrs:  &destActiveAttributes,
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    true,
			wantHasAttributes: true,
			wantSockets:       destActiveSockets,
			wantAttributes:    destActiveAttributes,
			count:             2,
			wantSourceRemain:  true,
			wantSourceCount:   2,
			wantDestCount:     8,
		},
		{
			name:              "explicit-zero destination wins on full counted merge",
			destinationSocks:  &destZeroSockets,
			destinationAttrs:  &destZeroAttributes,
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    true,
			wantHasAttributes: true,
			wantSockets:       destZeroSockets,
			wantAttributes:    destZeroAttributes,
			count:             4,
			wantSourceRemain:  false,
			wantDestCount:     10,
		},
		{
			name:              "omitted destination stays omitted on zero-count merge",
			sourceSocks:       &sourceActiveSockets,
			sourceAttrs:       &sourceActiveAttributes,
			wantHasSockets:    false,
			wantHasAttributes: false,
			count:             0,
			wantSourceRemain:  false,
			wantDestCount:     10,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:   0x01030931,
				VID:  0x02040931,
				Name: "MergeDestWinsItemMove",
				Inventory: []inventory.ItemInstance{
					{ID: 31, Vnum: 27001, Count: 4, Slot: 5, Sockets: tc.sourceSocks, Attributes: tc.sourceAttrs},
					{ID: 32, Vnum: 27001, Count: 6, Slot: 8, Sockets: tc.destinationSocks, Attributes: tc.destinationAttrs},
				},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "merge-dest-wins-item-move", CharacterIndex: 1})
			sourceBefore := persisted.Inventory[0]

			var (
				result inventory.MoveResult
				ok     bool
			)
			if tc.count == 0 {
				result, ok = runtime.MoveInventoryItemBounded(5, 8, 200)
			} else {
				result, ok = runtime.MoveInventoryItemCountBounded(5, 8, tc.count, 200)
			}
			if !ok {
				t.Fatal("expected compatible item-move merge to succeed")
			}
			if !result.Changed || !result.ToOccupied || !result.CountOnly || result.ToItem.ID != 32 || result.ToItem.Slot != 8 || result.ToItem.Count != tc.wantDestCount {
				t.Fatalf("unexpected item-move merge result: %+v", result)
			}
			if result.FromOccupied != tc.wantSourceRemain {
				t.Fatalf("unexpected source occupancy: %+v", result)
			}
			if tc.wantSourceRemain && (result.FromItem.ID != 31 || result.FromItem.Count != tc.wantSourceCount || result.FromItem.Slot != 5) {
				t.Fatalf("unexpected source remainder: %+v", result.FromItem)
			}
			assertDestinationPresenceWins(t, result.ToItem, sourceBefore, tc.wantHasSockets, tc.wantHasAttributes, tc.wantSockets, tc.wantAttributes)

			live := runtime.LiveInventory()
			var dest *inventory.ItemInstance
			for i := range live {
				if live[i].ID == 32 {
					dest = &live[i]
					break
				}
			}
			if dest == nil || dest.Count != tc.wantDestCount || dest.Slot != 8 {
				t.Fatalf("unexpected live destination after item-move merge: %#v", live)
			}
			assertDestinationPresenceWins(t, *dest, sourceBefore, tc.wantHasSockets, tc.wantHasAttributes, tc.wantSockets, tc.wantAttributes)
			if got := runtime.PersistedSnapshot(); !reflect.DeepEqual(got.Inventory, persisted.Inventory) {
				t.Fatalf("item-move merge mutated persisted inventory early: got %#v want %#v", got.Inventory, persisted.Inventory)
			}
		})
	}
}

func assertDestinationPresenceWins(
	t *testing.T,
	got inventory.ItemInstance,
	source inventory.ItemInstance,
	wantHasSockets bool,
	wantHasAttributes bool,
	wantSockets inventory.SocketValues,
	wantAttributes inventory.AttributeValues,
) {
	t.Helper()
	if got.HasSockets() != wantHasSockets {
		t.Fatalf("HasSockets=%v want %v", got.HasSockets(), wantHasSockets)
	}
	if got.HasAttributes() != wantHasAttributes {
		t.Fatalf("HasAttributes=%v want %v", got.HasAttributes(), wantHasAttributes)
	}
	if wantHasSockets {
		if got.Sockets == nil || *got.Sockets != wantSockets {
			t.Fatalf("expected destination sockets %+v, got %#v", wantSockets, got.Sockets)
		}
		if got.Sockets == source.Sockets {
			t.Fatal("merged destination sockets pointer aliased the discarded source")
		}
	} else if got.Sockets != nil {
		t.Fatalf("expected omitted destination sockets, got %#v", got.Sockets)
	}
	if wantHasAttributes {
		if got.Attributes == nil || *got.Attributes != wantAttributes {
			t.Fatalf("expected destination attributes %+v, got %#v", wantAttributes, got.Attributes)
		}
		if got.Attributes == source.Attributes {
			t.Fatal("merged destination attributes pointer aliased the discarded source")
		}
	} else if got.Attributes != nil {
		t.Fatalf("expected omitted destination attributes, got %#v", got.Attributes)
	}
}

func TestRuntimeBuyMerchantItemCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	destZeroSockets := inventory.SocketValues{}
	destZeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name              string
		destinationSocks  *inventory.SocketValues
		destinationAttrs  *inventory.AttributeValues
		wantHasSockets    bool
		wantHasAttributes bool
		wantSockets       inventory.SocketValues
		wantAttributes    inventory.AttributeValues
	}{
		{
			name:              "active destination wins on merchant buy merge",
			destinationSocks:  &destActiveSockets,
			destinationAttrs:  &destActiveAttributes,
			wantHasSockets:    true,
			wantHasAttributes: true,
			wantSockets:       destActiveSockets,
			wantAttributes:    destActiveAttributes,
		},
		{
			name:              "explicit-zero destination wins on merchant buy merge",
			destinationSocks:  &destZeroSockets,
			destinationAttrs:  &destZeroAttributes,
			wantHasSockets:    true,
			wantHasAttributes: true,
			wantSockets:       destZeroSockets,
			wantAttributes:    destZeroAttributes,
		},
		{
			name:              "omitted destination stays omitted on merchant buy merge",
			wantHasSockets:    false,
			wantHasAttributes: false,
		},
	}

	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:   0x01030941,
				VID:  0x02040941,
				Name: "MergeDestWinsMerchantBuy",
				Gold: 500,
				Inventory: []inventory.ItemInstance{{
					ID:         41,
					Vnum:       27001,
					Count:      4,
					Slot:       0,
					Sockets:    tc.destinationSocks,
					Attributes: tc.destinationAttrs,
				}},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "merge-dest-wins-merchant-buy", CharacterIndex: 1})
			result, ok := runtime.BuyMerchantItem(template, 3, 50)
			if !ok {
				t.Fatal("expected merchant buy merge to succeed")
			}
			if result.Gold != 450 || len(result.Items) != 1 || len(result.ItemChanges) != 1 {
				t.Fatalf("unexpected merchant buy result: %+v", result)
			}
			if result.ItemChanges[0].Created || result.Items[0].ID != 41 || result.Items[0].Count != 7 || result.Items[0].Slot != 0 {
				t.Fatalf("expected count-only merge into destination id 41, got %+v", result)
			}
			assertDestinationPresenceWins(t, result.Items[0], inventory.ItemInstance{}, tc.wantHasSockets, tc.wantHasAttributes, tc.wantSockets, tc.wantAttributes)

			live := runtime.LiveInventory()
			if len(live) != 1 || live[0].ID != 41 || live[0].Count != 7 || live[0].Slot != 0 {
				t.Fatalf("unexpected live inventory after merchant buy merge: %#v", live)
			}
			assertDestinationPresenceWins(t, live[0], inventory.ItemInstance{}, tc.wantHasSockets, tc.wantHasAttributes, tc.wantSockets, tc.wantAttributes)
			if runtime.LiveGold() != 450 {
				t.Fatalf("expected live gold 450 after buy, got %d", runtime.LiveGold())
			}
		})
	}
}

func TestRuntimeGrantCarriedItemCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	destSockets := inventory.SocketValues{7, 0, 9}
	destAttributes := inventory.AttributeValues{{Type: 1, Value: 25}}
	runtime := NewRuntime(loginticket.Character{
		ID:   0x01030942,
		VID:  0x02040942,
		Name: "MergeDestWinsGrant",
		Inventory: []inventory.ItemInstance{{
			ID:         42,
			Vnum:       27001,
			Count:      6,
			Slot:       3,
			Sockets:    &destSockets,
			Attributes: &destAttributes,
		}},
	}, SessionLink{Login: "merge-dest-wins-grant", CharacterIndex: 1})

	result, ok := runtime.GrantCarriedItem(itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}, 2)
	if !ok {
		t.Fatal("expected grant merge to succeed")
	}
	if len(result.Items) != 1 || result.Items[0].ID != 42 || result.Items[0].Count != 8 || result.Items[0].Slot != 3 {
		t.Fatalf("unexpected grant merge result: %+v", result)
	}
	if !result.Items[0].HasSockets() || *result.Items[0].Sockets != destSockets {
		t.Fatalf("expected grant merge to keep destination sockets, got %#v", result.Items[0].Sockets)
	}
	if !result.Items[0].HasAttributes() || *result.Items[0].Attributes != destAttributes {
		t.Fatalf("expected grant merge to keep destination attributes, got %#v", result.Items[0].Attributes)
	}
}
