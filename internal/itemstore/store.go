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
	Vnum         uint32       `json:"vnum"`
	Name         string       `json:"name"`
	Stackable    bool         `json:"stackable"`
	MaxCount     uint16       `json:"max_count"`
	ShopBuyPrice uint64       `json:"shop_buy_price,omitempty"`
	EquipSlot    string       `json:"equip_slot,omitempty"`
	UseEffect    *UseEffect   `json:"use_effect,omitempty"`
	EquipEffect  *PointEffect `json:"equip_effect,omitempty"`
}

type PointEffect struct {
	PointType  uint8 `json:"point_type"`
	PointIndex uint8 `json:"point_index"`
	PointDelta int32 `json:"point_delta"`
}

type UseEffect struct {
	PointType  uint8  `json:"point_type"`
	PointIndex uint8  `json:"point_index"`
	PointDelta int32  `json:"point_delta"`
	Message    string `json:"message"`
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
	if template.UseEffect != nil {
		effect := *template.UseEffect
		effect.Message = strings.TrimSpace(effect.Message)
		template.UseEffect = &effect
	}
	if template.EquipEffect != nil {
		effect := *template.EquipEffect
		template.EquipEffect = &effect
	}
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
		return template.EquipEffect == nil && validUseEffect(template.UseEffect)
	}
	_, ok := inventory.ParseEquipmentSlot(template.EquipSlot)
	return ok && validUseEffect(template.UseEffect) && validPointEffect(template.EquipEffect)
}

func validUseEffect(effect *UseEffect) bool {
	if effect == nil {
		return true
	}
	if !validPointFields(effect.PointType, effect.PointIndex, effect.PointDelta) {
		return false
	}
	return strings.TrimSpace(effect.Message) != ""
}

func validPointEffect(effect *PointEffect) bool {
	if effect == nil {
		return true
	}
	return validPointFields(effect.PointType, effect.PointIndex, effect.PointDelta)
}

func validPointFields(pointType uint8, pointIndex uint8, pointDelta int32) bool {
	if pointType == 0 {
		return false
	}
	if pointIndex >= 255 {
		return false
	}
	return pointDelta > 0
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
	for i := range templates {
		cloned[i] = templates[i]
		if templates[i].UseEffect != nil {
			effect := *templates[i].UseEffect
			cloned[i].UseEffect = &effect
		}
		if templates[i].EquipEffect != nil {
			effect := *templates[i].EquipEffect
			cloned[i].EquipEffect = &effect
		}
	}
	return cloned
}
