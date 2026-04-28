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

func TestItemInstanceWithInventorySlotClearsEquipmentState(t *testing.T) {
	item := ItemInstance{ID: 42, Vnum: 1120, Count: 1, Slot: 3, Equipped: true, EquipSlot: EquipmentSlotWeapon}

	moved, err := item.WithInventorySlot(8)
	if err != nil {
		t.Fatalf("WithInventorySlot() unexpected error: %v", err)
	}
	if moved.Slot != 8 || moved.Equipped || moved.EquipSlot != EquipmentSlotNone {
		t.Fatalf("unexpected carried item after WithInventorySlot(): %+v", moved)
	}
	if item.Slot != 3 || !item.Equipped || item.EquipSlot != EquipmentSlotWeapon {
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
