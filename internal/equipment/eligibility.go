// Package equipment contains pure rules for deciding whether a character may wear
// an authored item template. It intentionally has no dependency on live runtime
// state, persistence, packet encoding, or session ownership.
package equipment

import (
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
)

// Subject is the character data that authored equipment restrictions inspect.
type Subject struct {
	Job     uint8
	RaceNum uint16
	Empire  uint8
	Level   uint8
}

// CanUse reports whether a valid template's character restrictions allow subject.
func CanUse(template itemcatalog.Template, subject Subject) bool {
	if !itemcatalog.ValidTemplate(template) {
		return false
	}
	if subject.Job == 0 && template.AntiWarrior {
		return false
	}
	if subject.Job == 1 && template.AntiAssassin {
		return false
	}
	if subject.Job == 2 && template.AntiSura {
		return false
	}
	if subject.Job == 3 && template.AntiShaman {
		return false
	}
	if subject.RaceNum%2 == 0 && template.AntiMale {
		return false
	}
	if subject.RaceNum%2 == 1 && template.AntiFemale {
		return false
	}
	if subject.Empire == 1 && template.AntiEmpireA {
		return false
	}
	if subject.Empire == 2 && template.AntiEmpireB {
		return false
	}
	if subject.Empire == 3 && template.AntiEmpireC {
		return false
	}
	return template.MinLevel == 0 || subject.Level >= template.MinLevel
}

// CanEquip reports whether subject may equip template into equipSlot. Equipment
// cannot carry transfer-guard flags because those operations would be invalid
// while the item is worn.
func CanEquip(template itemcatalog.Template, subject Subject, equipSlot inventory.EquipmentSlot) bool {
	if !CanUse(template, subject) || !equipSlot.Valid() || template.EquipSlot == "" {
		return false
	}
	templateSlot, ok := inventory.ParseEquipmentSlot(template.EquipSlot)
	if !ok || templateSlot != equipSlot {
		return false
	}
	return !template.AntiStack && !template.AntiGet && !template.AntiDrop && !template.AntiGive && !template.AntiSell
}
