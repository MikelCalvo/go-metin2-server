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

const (
	CarriedInventorySlotCount SlotIndex = 90
	ItemSocketCount                     = 3
	ItemAttributeCount                  = 7
)

// SocketValues is the fixed wire-width per-instance socket array.
type SocketValues [ItemSocketCount]int32

// Attribute is one opaque wire-width attribute cell.
type Attribute struct {
	Type  uint8 `json:"type"`
	Value int16 `json:"value"`
}

// AttributeValues is the fixed wire-width per-instance attribute array.
type AttributeValues [ItemAttributeCount]Attribute

type ItemInstance struct {
	ID        uint64
	Vnum      uint32
	Count     uint16
	Slot      SlotIndex
	Equipped  bool
	EquipSlot EquipmentSlot
	Locked    bool
	// Sockets is optional per-instance socket state. nil means "fall back to
	// template sockets"; a non-nil pointer (including all-zero) is authoritative.
	Sockets *SocketValues `json:"sockets,omitempty"`
	// Attributes is optional per-instance attribute state. nil means "fall back
	// to template attributes"; a non-nil pointer (including all-zero / type-zero)
	// is authoritative.
	Attributes *AttributeValues `json:"attributes,omitempty"`
}

// HasSockets reports whether this instance carries authoritative socket state.
func (i ItemInstance) HasSockets() bool {
	return i.Sockets != nil
}

// CloneSockets returns an independent copy of the instance sockets pointer.
func (i ItemInstance) CloneSockets() *SocketValues {
	if i.Sockets == nil {
		return nil
	}
	copied := *i.Sockets
	return &copied
}

// EffectiveSockets prefers instance sockets when present; otherwise fallback.
func (i ItemInstance) EffectiveSockets(fallback SocketValues) SocketValues {
	if i.Sockets != nil {
		return *i.Sockets
	}
	return fallback
}

// HasAttributes reports whether this instance carries authoritative attribute state.
func (i ItemInstance) HasAttributes() bool {
	return i.Attributes != nil
}

// CloneAttributes returns an independent copy of the instance attributes pointer.
func (i ItemInstance) CloneAttributes() *AttributeValues {
	if i.Attributes == nil {
		return nil
	}
	copied := *i.Attributes
	return &copied
}

// EffectiveAttributes prefers instance attributes when present; otherwise fallback.
func (i ItemInstance) EffectiveAttributes(fallback AttributeValues) AttributeValues {
	if i.Attributes != nil {
		return *i.Attributes
	}
	return fallback
}

type MoveResult struct {
	Changed        bool
	From           SlotIndex
	To             SlotIndex
	FromOccupied   bool
	FromItem       ItemInstance
	ToOccupied     bool
	ToItem         ItemInstance
	CountOnly      bool
	CompatibleSwap bool
	ForcedSwap     bool
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
	updated.Sockets = i.CloneSockets()
	updated.Attributes = i.CloneAttributes()
	if err := updated.Validate(); err != nil {
		return ItemInstance{}, err
	}
	return updated, nil
}
