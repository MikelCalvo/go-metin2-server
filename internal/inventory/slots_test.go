package inventory

import (
	"reflect"
	"testing"
)

func TestAllEquipmentSlotsReturnsDeterministicOrder(t *testing.T) {
	want := []EquipmentSlot{
		EquipmentSlotBody,
		EquipmentSlotWeapon,
		EquipmentSlotHead,
		EquipmentSlotHair,
		EquipmentSlotShield,
		EquipmentSlotWrist,
		EquipmentSlotShoes,
		EquipmentSlotNeck,
		EquipmentSlotEar,
		EquipmentSlotUnique1,
		EquipmentSlotUnique2,
		EquipmentSlotArrow,
	}
	got := AllEquipmentSlots()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllEquipmentSlots() = %v, want %v", got, want)
	}

	got[0] = EquipmentSlotNone
	if again := AllEquipmentSlots(); !reflect.DeepEqual(again, want) {
		t.Fatalf("AllEquipmentSlots() should return an independent copy, got %v, want %v", again, want)
	}
}

func TestEquipmentSlotStringAndParse(t *testing.T) {
	tests := []struct {
		name  string
		slot  EquipmentSlot
		input string
	}{
		{name: "body", slot: EquipmentSlotBody, input: "body"},
		{name: "weapon", slot: EquipmentSlotWeapon, input: " Weapon "},
		{name: "head", slot: EquipmentSlotHead, input: "HEAD"},
		{name: "hair", slot: EquipmentSlotHair, input: "hair"},
		{name: "shield", slot: EquipmentSlotShield, input: "shield"},
		{name: "wrist", slot: EquipmentSlotWrist, input: "wrist"},
		{name: "shoes", slot: EquipmentSlotShoes, input: "shoes"},
		{name: "neck", slot: EquipmentSlotNeck, input: "neck"},
		{name: "ear", slot: EquipmentSlotEar, input: "ear"},
		{name: "unique1", slot: EquipmentSlotUnique1, input: "unique1"},
		{name: "unique2", slot: EquipmentSlotUnique2, input: "unique2"},
		{name: "arrow", slot: EquipmentSlotArrow, input: "arrow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.slot.String(); got != tt.name {
				t.Fatalf("%v.String() = %q, want %q", tt.slot, got, tt.name)
			}
			parsed, ok := ParseEquipmentSlot(tt.input)
			if !ok {
				t.Fatalf("ParseEquipmentSlot(%q) reported !ok", tt.input)
			}
			if parsed != tt.slot {
				t.Fatalf("ParseEquipmentSlot(%q) = %v, want %v", tt.input, parsed, tt.slot)
			}
		})
	}

	parsedNone, ok := ParseEquipmentSlot("none")
	if !ok {
		t.Fatalf("ParseEquipmentSlot(none) reported !ok")
	}
	if parsedNone != EquipmentSlotNone {
		t.Fatalf("ParseEquipmentSlot(none) = %v, want %v", parsedNone, EquipmentSlotNone)
	}

	if _, ok := ParseEquipmentSlot("unknown"); ok {
		t.Fatalf("ParseEquipmentSlot(unknown) reported ok")
	}
}

func TestEquipmentSlotValid(t *testing.T) {
	if EquipmentSlotNone.Valid() {
		t.Fatalf("EquipmentSlotNone should not be a valid equipped slot")
	}
	for _, slot := range AllEquipmentSlots() {
		if !slot.Valid() {
			t.Fatalf("slot %v should be valid", slot)
		}
	}
	if EquipmentSlot(255).Valid() {
		t.Fatalf("unexpected slot should not be valid")
	}
}
