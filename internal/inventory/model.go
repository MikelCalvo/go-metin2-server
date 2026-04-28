package inventory

import (
	"errors"
	"fmt"
)

var (
	ErrItemInstanceIDRequired       = errors.New("item instance id is required")
	ErrItemVnumRequired             = errors.New("item vnum is required")
	ErrItemCountRequired            = errors.New("item count is required")
	ErrEquippedItemSlotRequired     = errors.New("equipped item requires a valid equipment slot")
	ErrUnequippedItemSlotMustBeNone = errors.New("unequipped item must not carry an equipment slot")
	ErrInventorySlotOutOfRange      = errors.New("inventory slot is out of range")
)

const CarriedInventorySlotCount SlotIndex = 90

type ItemInstance struct {
	ID        uint64
	Vnum      uint32
	Count     uint16
	Slot      SlotIndex
	Equipped  bool
	EquipSlot EquipmentSlot
}

type MoveResult struct {
	Changed      bool
	From         SlotIndex
	To           SlotIndex
	FromOccupied bool
	FromItem     ItemInstance
	ToOccupied   bool
	ToItem       ItemInstance
}

func (i ItemInstance) Validate() error {
	if i.ID == 0 {
		return ErrItemInstanceIDRequired
	}
	if i.Vnum == 0 {
		return ErrItemVnumRequired
	}
	if i.Count == 0 {
		return ErrItemCountRequired
	}
	if i.Equipped {
		if !i.EquipSlot.Valid() {
			return fmt.Errorf("%w: %s", ErrEquippedItemSlotRequired, i.EquipSlot.String())
		}
		return nil
	}
	if i.EquipSlot != EquipmentSlotNone {
		return fmt.Errorf("%w: %s", ErrUnequippedItemSlotMustBeNone, i.EquipSlot.String())
	}
	return nil
}

func (i ItemInstance) WithInventorySlot(slot SlotIndex) (ItemInstance, error) {
	if slot >= CarriedInventorySlotCount {
		return ItemInstance{}, fmt.Errorf("%w: %d", ErrInventorySlotOutOfRange, slot)
	}
	updated := i
	updated.Slot = slot
	updated.Equipped = false
	updated.EquipSlot = EquipmentSlotNone
	if err := updated.Validate(); err != nil {
		return ItemInstance{}, err
	}
	return updated, nil
}
