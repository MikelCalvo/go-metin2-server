package itemstore

import (
	"errors"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

var (
	ErrStorePathRequired = errors.New("item template store path is required")
	ErrSnapshotNotFound  = errors.New("item template snapshot not found")
	ErrInvalidSnapshot   = errors.New("invalid item template snapshot")
)

type Template struct {
	Vnum      uint32 `json:"vnum"`
	Name      string `json:"name"`
	Stackable bool   `json:"stackable"`
	MaxCount  uint16 `json:"max_count"`
	EquipSlot string `json:"equip_slot,omitempty"`
}

type Snapshot struct {
	Templates []Template `json:"templates"`
}

type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := Snapshot{Templates: cloneTemplates(snapshot.Templates)}
	for i := range normalized.Templates {
		normalized.Templates[i] = normalizeTemplate(normalized.Templates[i])
	}
	sort.Slice(normalized.Templates, func(i int, j int) bool {
		return normalized.Templates[i].Vnum < normalized.Templates[j].Vnum
	})
	return normalized
}

func normalizeTemplate(template Template) Template {
	template.Name = strings.TrimSpace(template.Name)
	template.EquipSlot = normalizeEquipSlot(template.EquipSlot)
	return template
}

func validateSnapshot(snapshot Snapshot) error {
	seen := make(map[uint32]struct{}, len(snapshot.Templates))
	for _, template := range snapshot.Templates {
		if !validTemplate(template) {
			return ErrInvalidSnapshot
		}
		if _, ok := seen[template.Vnum]; ok {
			return ErrInvalidSnapshot
		}
		seen[template.Vnum] = struct{}{}
	}
	return nil
}

func validTemplate(template Template) bool {
	if template.Vnum == 0 {
		return false
	}
	if strings.TrimSpace(template.Name) == "" {
		return false
	}
	if template.MaxCount == 0 {
		return false
	}
	if !template.Stackable && template.MaxCount != 1 {
		return false
	}
	if template.EquipSlot == "" {
		return true
	}
	_, ok := inventory.ParseEquipmentSlot(template.EquipSlot)
	return ok
}

func ValidTemplate(template Template) bool {
	return validTemplate(normalizeTemplate(template))
}

func normalizeEquipSlot(name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return ""
	}
	slot, ok := inventory.ParseEquipmentSlot(trimmed)
	if !ok {
		return trimmed
	}
	if slot == inventory.EquipmentSlotNone {
		return ""
	}
	return slot.String()
}

func cloneTemplates(templates []Template) []Template {
	if len(templates) == 0 {
		return nil
	}
	cloned := make([]Template, len(templates))
	copy(cloned, templates)
	return cloned
}
