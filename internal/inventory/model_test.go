package inventory

import (
	"errors"
	"testing"
)

func TestItemInstanceValidateCarriedItem(t *testing.T) {
	item := ItemInstance{ID: 1, Vnum: 19, Count: 3, Slot: 7}
	if err := item.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestItemInstanceValidateEquippedItem(t *testing.T) {
	item := ItemInstance{
		ID:        42,
		Vnum:      1120,
		Count:     1,
		Slot:      3,
		Equipped:  true,
		EquipSlot: EquipmentSlotWeapon,
	}
	if err := item.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestItemInstanceValidateRejectsMissingCoreFields(t *testing.T) {
	tests := []struct {
		name string
		item ItemInstance
		want error
	}{
		{
			name: "missing id",
			item: ItemInstance{Vnum: 19, Count: 1, Slot: 7},
			want: ErrItemInstanceIDRequired,
		},
		{
			name: "missing vnum",
			item: ItemInstance{ID: 1, Count: 1, Slot: 7},
			want: ErrItemVnumRequired,
		},
		{
			name: "missing count",
			item: ItemInstance{ID: 1, Vnum: 19, Slot: 7},
			want: ErrItemCountRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.Validate()
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestItemInstanceValidateRejectsInconsistentEquipmentState(t *testing.T) {
	tests := []struct {
		name string
		item ItemInstance
		want error
	}{
		{
			name: "equipped without equip slot",
			item: ItemInstance{ID: 1, Vnum: 19, Count: 1, Slot: 7, Equipped: true},
			want: ErrEquippedItemSlotRequired,
		},
		{
			name: "equipped with invalid equip slot",
			item: ItemInstance{ID: 1, Vnum: 19, Count: 1, Slot: 7, Equipped: true, EquipSlot: EquipmentSlot(255)},
			want: ErrEquippedItemSlotRequired,
		},
		{
			name: "unequipped with equip slot",
			item: ItemInstance{ID: 1, Vnum: 19, Count: 1, Slot: 7, EquipSlot: EquipmentSlotWeapon},
			want: ErrUnequippedItemSlotMustBeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.Validate()
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestItemInstanceWithInventorySlotClearsEquipmentStateButKeepsLock(t *testing.T) {
	item := ItemInstance{ID: 42, Vnum: 1120, Count: 1, Slot: 3, Equipped: true, EquipSlot: EquipmentSlotWeapon, Locked: true}

	moved, err := item.WithInventorySlot(8)
	if err != nil {
		t.Fatalf("WithInventorySlot() unexpected error: %v", err)
	}
	if moved.Slot != 8 || moved.Equipped || moved.EquipSlot != EquipmentSlotNone || !moved.Locked {
		t.Fatalf("unexpected carried item after WithInventorySlot(): %+v", moved)
	}
	if item.Slot != 3 || !item.Equipped || item.EquipSlot != EquipmentSlotWeapon || !item.Locked {
		t.Fatalf("expected original item to stay unchanged, got %+v", item)
	}
}

func TestItemInstanceWithInventorySlotRejectsOutOfRangeSlot(t *testing.T) {
	item := ItemInstance{ID: 42, Vnum: 1120, Count: 1, Slot: 3}

	_, err := item.WithInventorySlot(90)
	if !errors.Is(err, ErrInventorySlotOutOfRange) {
		t.Fatalf("expected ErrInventorySlotOutOfRange, got %v", err)
	}
}

func TestItemInstanceEffectiveSocketsPreferInstancePresenceIncludingZero(t *testing.T) {
	fallback := SocketValues{9, 8, 7}
	item := ItemInstance{ID: 1, Vnum: 72723, Count: 1, Slot: 5}
	if got := item.EffectiveSockets(fallback); got != fallback {
		t.Fatalf("expected omitted sockets to fall back, got %+v", got)
	}
	if item.HasSockets() {
		t.Fatal("expected omitted sockets to report HasSockets=false")
	}

	zero := SocketValues{}
	item.Sockets = &zero
	if !item.HasSockets() {
		t.Fatal("expected explicit zero sockets to report HasSockets=true")
	}
	if got := item.EffectiveSockets(fallback); got != zero {
		t.Fatalf("expected explicit zero sockets to win over template fallback, got %+v", got)
	}

	active := SocketValues{1, 2, 3}
	item.Sockets = &active
	if got := item.EffectiveSockets(fallback); got != active {
		t.Fatalf("expected instance sockets %+v, got %+v", active, got)
	}

	cloned := item.CloneSockets()
	if cloned == nil || *cloned != active || cloned == item.Sockets {
		t.Fatalf("CloneSockets() = %v want independent copy of %+v", cloned, active)
	}
	cloned[0] = 0
	if item.Sockets[0] != 1 {
		t.Fatalf("expected CloneSockets to leave original unchanged, got %+v", *item.Sockets)
	}

	moved, err := item.WithInventorySlot(8)
	if err != nil {
		t.Fatalf("WithInventorySlot() unexpected error: %v", err)
	}
	if !moved.HasSockets() || moved.Sockets == item.Sockets || *moved.Sockets != active {
		t.Fatalf("expected WithInventorySlot to clone sockets, got %+v", moved)
	}
}
