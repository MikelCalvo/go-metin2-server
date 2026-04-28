package inventory

import (
	"fmt"
	"strings"
)

type SlotIndex uint16

type EquipmentSlot uint8

const (
	EquipmentSlotNone EquipmentSlot = iota
	EquipmentSlotBody
	EquipmentSlotWeapon
	EquipmentSlotHead
	EquipmentSlotHair
	EquipmentSlotShield
	EquipmentSlotWrist
	EquipmentSlotShoes
	EquipmentSlotNeck
	EquipmentSlotEar
	EquipmentSlotUnique1
	EquipmentSlotUnique2
	EquipmentSlotArrow
)

var equipmentSlotOrder = []EquipmentSlot{
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

func AllEquipmentSlots() []EquipmentSlot {
	out := make([]EquipmentSlot, len(equipmentSlotOrder))
	copy(out, equipmentSlotOrder)
	return out
}

func (s EquipmentSlot) Valid() bool {
	switch s {
	case EquipmentSlotBody,
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
		EquipmentSlotArrow:
		return true
	default:
		return false
	}
}

func (s EquipmentSlot) String() string {
	switch s {
	case EquipmentSlotNone:
		return "none"
	case EquipmentSlotBody:
		return "body"
	case EquipmentSlotWeapon:
		return "weapon"
	case EquipmentSlotHead:
		return "head"
	case EquipmentSlotHair:
		return "hair"
	case EquipmentSlotShield:
		return "shield"
	case EquipmentSlotWrist:
		return "wrist"
	case EquipmentSlotShoes:
		return "shoes"
	case EquipmentSlotNeck:
		return "neck"
	case EquipmentSlotEar:
		return "ear"
	case EquipmentSlotUnique1:
		return "unique1"
	case EquipmentSlotUnique2:
		return "unique2"
	case EquipmentSlotArrow:
		return "arrow"
	default:
		return fmt.Sprintf("EquipmentSlot(%d)", s)
	}
}

func ParseEquipmentSlot(name string) (EquipmentSlot, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "none":
		return EquipmentSlotNone, true
	case "body":
		return EquipmentSlotBody, true
	case "weapon":
		return EquipmentSlotWeapon, true
	case "head":
		return EquipmentSlotHead, true
	case "hair":
		return EquipmentSlotHair, true
	case "shield":
		return EquipmentSlotShield, true
	case "wrist":
		return EquipmentSlotWrist, true
	case "shoes":
		return EquipmentSlotShoes, true
	case "neck":
		return EquipmentSlotNeck, true
	case "ear":
		return EquipmentSlotEar, true
	case "unique1":
		return EquipmentSlotUnique1, true
	case "unique2":
		return EquipmentSlotUnique2, true
	case "arrow":
		return EquipmentSlotArrow, true
	default:
		return EquipmentSlotNone, false
	}
}
