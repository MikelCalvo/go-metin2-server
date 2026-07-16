package itemstore

import (
	"bytes"
	"encoding/json"
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

const (
	ItemSocketCount    = 3
	ItemAttributeCount = 7
)

type Template struct {
	Vnum             uint32          `json:"vnum"`
	Name             string          `json:"name"`
	Stackable        bool            `json:"stackable"`
	MaxCount         uint16          `json:"max_count"`
	ShopBuyPrice     uint64          `json:"shop_buy_price,omitempty"`
	Refineable       bool            `json:"refineable,omitempty"`
	Save             bool            `json:"save,omitempty"`
	SellCountPerGold bool            `json:"sell_count_per_gold,omitempty"`
	SlowQuery        bool            `json:"slow_query,omitempty"`
	Highlight        bool            `json:"highlight,omitempty"`
	Rare             bool            `json:"rare,omitempty"`
	Unique           bool            `json:"unique,omitempty"`
	MakeCount        bool            `json:"make_count,omitempty"`
	Irremovable      bool            `json:"irremovable,omitempty"`
	ConfirmWhenUse   bool            `json:"confirm_when_use,omitempty"`
	QuestUse         bool            `json:"quest_use,omitempty"`
	QuestUseMultiple bool            `json:"quest_use_multiple,omitempty"`
	Log              bool            `json:"log,omitempty"`
	Applicable       bool            `json:"applicable,omitempty"`
	AntiSell         bool            `json:"anti_sell,omitempty"`
	AntiDrop         bool            `json:"anti_drop,omitempty"`
	AntiGive         bool            `json:"anti_give,omitempty"`
	AntiStack        bool            `json:"anti_stack,omitempty"`
	AntiGet          bool            `json:"anti_get,omitempty"`
	AntiMale         bool            `json:"anti_male,omitempty"`
	AntiFemale       bool            `json:"anti_female,omitempty"`
	AntiWarrior      bool            `json:"anti_warrior,omitempty"`
	AntiAssassin     bool            `json:"anti_assassin,omitempty"`
	AntiSura         bool            `json:"anti_sura,omitempty"`
	AntiShaman       bool            `json:"anti_shaman,omitempty"`
	AntiEmpireA      bool            `json:"anti_empire_a,omitempty"`
	AntiEmpireB      bool            `json:"anti_empire_b,omitempty"`
	AntiEmpireC      bool            `json:"anti_empire_c,omitempty"`
	AntiSave         bool            `json:"anti_save,omitempty"`
	AntiPKDrop       bool            `json:"anti_pk_drop,omitempty"`
	AntiMyShop       bool            `json:"anti_myshop,omitempty"`
	AntiSafebox      bool            `json:"anti_safebox,omitempty"`
	MinLevel         uint8           `json:"min_level,omitempty"`
	EquipSlot        string          `json:"equip_slot,omitempty"`
	Sockets          SocketValues    `json:"sockets,omitempty"`
	Attributes       AttributeValues `json:"attributes,omitempty"`
	UseEffect        *UseEffect      `json:"use_effect,omitempty"`
	EquipEffect      *PointEffect    `json:"equip_effect,omitempty"`
}

type SocketValues [ItemSocketCount]int32

type AttributeValues [ItemAttributeCount]Attribute

type Attribute struct {
	Type  uint8 `json:"type"`
	Value int16 `json:"value"`
}

type templateJSON struct {
	Vnum             uint32           `json:"vnum"`
	Name             string           `json:"name"`
	Stackable        bool             `json:"stackable"`
	MaxCount         uint16           `json:"max_count"`
	ShopBuyPrice     uint64           `json:"shop_buy_price,omitempty"`
	Refineable       bool             `json:"refineable,omitempty"`
	Save             bool             `json:"save,omitempty"`
	SellCountPerGold bool             `json:"sell_count_per_gold,omitempty"`
	SlowQuery        bool             `json:"slow_query,omitempty"`
	Highlight        bool             `json:"highlight,omitempty"`
	Rare             bool             `json:"rare,omitempty"`
	Unique           bool             `json:"unique,omitempty"`
	MakeCount        bool             `json:"make_count,omitempty"`
	Irremovable      bool             `json:"irremovable,omitempty"`
	ConfirmWhenUse   bool             `json:"confirm_when_use,omitempty"`
	QuestUse         bool             `json:"quest_use,omitempty"`
	QuestUseMultiple bool             `json:"quest_use_multiple,omitempty"`
	Log              bool             `json:"log,omitempty"`
	Applicable       bool             `json:"applicable,omitempty"`
	AntiSell         bool             `json:"anti_sell,omitempty"`
	AntiDrop         bool             `json:"anti_drop,omitempty"`
	AntiGive         bool             `json:"anti_give,omitempty"`
	AntiStack        bool             `json:"anti_stack,omitempty"`
	AntiGet          bool             `json:"anti_get,omitempty"`
	AntiMale         bool             `json:"anti_male,omitempty"`
	AntiFemale       bool             `json:"anti_female,omitempty"`
	AntiWarrior      bool             `json:"anti_warrior,omitempty"`
	AntiAssassin     bool             `json:"anti_assassin,omitempty"`
	AntiSura         bool             `json:"anti_sura,omitempty"`
	AntiShaman       bool             `json:"anti_shaman,omitempty"`
	AntiEmpireA      bool             `json:"anti_empire_a,omitempty"`
	AntiEmpireB      bool             `json:"anti_empire_b,omitempty"`
	AntiEmpireC      bool             `json:"anti_empire_c,omitempty"`
	AntiSave         bool             `json:"anti_save,omitempty"`
	AntiPKDrop       bool             `json:"anti_pk_drop,omitempty"`
	AntiMyShop       bool             `json:"anti_myshop,omitempty"`
	AntiSafebox      bool             `json:"anti_safebox,omitempty"`
	MinLevel         uint8            `json:"min_level,omitempty"`
	EquipSlot        string           `json:"equip_slot,omitempty"`
	Sockets          *SocketValues    `json:"sockets,omitempty"`
	Attributes       *AttributeValues `json:"attributes,omitempty"`
	UseEffect        *UseEffect       `json:"use_effect,omitempty"`
	EquipEffect      *PointEffect     `json:"equip_effect,omitempty"`
}

func (template Template) MarshalJSON() ([]byte, error) {
	jsonTemplate := templateJSON{
		Vnum:             template.Vnum,
		Name:             template.Name,
		Stackable:        template.Stackable,
		MaxCount:         template.MaxCount,
		ShopBuyPrice:     template.ShopBuyPrice,
		Refineable:       template.Refineable,
		Save:             template.Save,
		SellCountPerGold: template.SellCountPerGold,
		SlowQuery:        template.SlowQuery,
		Highlight:        template.Highlight,
		Rare:             template.Rare,
		Unique:           template.Unique,
		MakeCount:        template.MakeCount,
		Irremovable:      template.Irremovable,
		ConfirmWhenUse:   template.ConfirmWhenUse,
		QuestUse:         template.QuestUse,
		QuestUseMultiple: template.QuestUseMultiple,
		Log:              template.Log,
		Applicable:       template.Applicable,
		AntiSell:         template.AntiSell,
		AntiDrop:         template.AntiDrop,
		AntiGive:         template.AntiGive,
		AntiStack:        template.AntiStack,
		AntiGet:          template.AntiGet,
		AntiMale:         template.AntiMale,
		AntiFemale:       template.AntiFemale,
		AntiWarrior:      template.AntiWarrior,
		AntiAssassin:     template.AntiAssassin,
		AntiSura:         template.AntiSura,
		AntiShaman:       template.AntiShaman,
		AntiEmpireA:      template.AntiEmpireA,
		AntiEmpireB:      template.AntiEmpireB,
		AntiEmpireC:      template.AntiEmpireC,
		AntiSave:         template.AntiSave,
		AntiPKDrop:       template.AntiPKDrop,
		AntiMyShop:       template.AntiMyShop,
		AntiSafebox:      template.AntiSafebox,
		MinLevel:         template.MinLevel,
		EquipSlot:        template.EquipSlot,
		UseEffect:        template.UseEffect,
		EquipEffect:      template.EquipEffect,
	}
	if template.Sockets != (SocketValues{}) {
		jsonTemplate.Sockets = &template.Sockets
	}
	if template.Attributes != (AttributeValues{}) {
		jsonTemplate.Attributes = &template.Attributes
	}
	return json.Marshal(jsonTemplate)
}

func (template *Template) UnmarshalJSON(raw []byte) error {
	var jsonTemplate templateJSON
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&jsonTemplate); err != nil {
		return err
	}
	*template = Template{
		Vnum:             jsonTemplate.Vnum,
		Name:             jsonTemplate.Name,
		Stackable:        jsonTemplate.Stackable,
		MaxCount:         jsonTemplate.MaxCount,
		ShopBuyPrice:     jsonTemplate.ShopBuyPrice,
		Refineable:       jsonTemplate.Refineable,
		Save:             jsonTemplate.Save,
		SellCountPerGold: jsonTemplate.SellCountPerGold,
		SlowQuery:        jsonTemplate.SlowQuery,
		Highlight:        jsonTemplate.Highlight,
		Rare:             jsonTemplate.Rare,
		Unique:           jsonTemplate.Unique,
		MakeCount:        jsonTemplate.MakeCount,
		Irremovable:      jsonTemplate.Irremovable,
		ConfirmWhenUse:   jsonTemplate.ConfirmWhenUse,
		QuestUse:         jsonTemplate.QuestUse,
		QuestUseMultiple: jsonTemplate.QuestUseMultiple,
		Log:              jsonTemplate.Log,
		Applicable:       jsonTemplate.Applicable,
		AntiSell:         jsonTemplate.AntiSell,
		AntiDrop:         jsonTemplate.AntiDrop,
		AntiGive:         jsonTemplate.AntiGive,
		AntiStack:        jsonTemplate.AntiStack,
		AntiGet:          jsonTemplate.AntiGet,
		AntiMale:         jsonTemplate.AntiMale,
		AntiFemale:       jsonTemplate.AntiFemale,
		AntiWarrior:      jsonTemplate.AntiWarrior,
		AntiAssassin:     jsonTemplate.AntiAssassin,
		AntiSura:         jsonTemplate.AntiSura,
		AntiShaman:       jsonTemplate.AntiShaman,
		AntiEmpireA:      jsonTemplate.AntiEmpireA,
		AntiEmpireB:      jsonTemplate.AntiEmpireB,
		AntiEmpireC:      jsonTemplate.AntiEmpireC,
		AntiSave:         jsonTemplate.AntiSave,
		AntiPKDrop:       jsonTemplate.AntiPKDrop,
		AntiMyShop:       jsonTemplate.AntiMyShop,
		AntiSafebox:      jsonTemplate.AntiSafebox,
		MinLevel:         jsonTemplate.MinLevel,
		EquipSlot:        jsonTemplate.EquipSlot,
		UseEffect:        jsonTemplate.UseEffect,
		EquipEffect:      jsonTemplate.EquipEffect,
	}
	if jsonTemplate.Sockets != nil {
		template.Sockets = *jsonTemplate.Sockets
	}
	if jsonTemplate.Attributes != nil {
		template.Attributes = *jsonTemplate.Attributes
	}
	return nil
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

func NormalizeTemplate(template Template) Template {
	return normalizeTemplate(template)
}

func NormalizeSnapshot(snapshot Snapshot) Snapshot {
	return normalizeSnapshot(snapshot)
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
	if template.MaxCount == 0 || template.MaxCount > 255 {
		return false
	}
	if !template.Stackable && template.MaxCount != 1 {
		return false
	}
	if !validDisplayAttributes(template.Attributes) {
		return false
	}
	if template.EquipSlot == "" {
		return template.EquipEffect == nil && validUseEffect(template.UseEffect)
	}
	_, ok := inventory.ParseEquipmentSlot(template.EquipSlot)
	return ok && template.UseEffect == nil && validPointEffect(template.EquipEffect)
}

func validDisplayAttributes(attributes AttributeValues) bool {
	for _, attribute := range attributes {
		if attribute.Type == 0 && attribute.Value != 0 {
			return false
		}
	}
	return true
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
