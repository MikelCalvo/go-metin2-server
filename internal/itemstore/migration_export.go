package itemstore

import (
	"errors"
	"fmt"
)

const (
	// ItemTemplateStateMigrationVersion / Name pin the tip-0009 export /
	// quarantine / import-result identity. Additive 0021 adds keep_on_fail onto
	// item_template_refine_infos without retipping that identity.
	ItemTemplateStateMigrationVersion = 9
	ItemTemplateStateMigrationName    = "item_template_refine_info"

	// ItemTemplateRefineKeepOnFailMigrationVersion / Name pin the additive
	// schema boundary that SQL import must require beside tip-0009 before
	// inserting keep_on_fail.
	ItemTemplateRefineKeepOnFailMigrationVersion = 21
	ItemTemplateRefineKeepOnFailMigrationName    = "item_template_refine_keep_on_fail"
)

type ItemTemplateStateExport struct {
	MigrationVersion int                             `json:"migration_version"`
	MigrationName    string                          `json:"migration_name"`
	Templates        []ItemTemplateRow               `json:"templates"`
	Sockets          []ItemTemplateSocketRow         `json:"sockets"`
	Attributes       []ItemTemplateAttributeRow      `json:"attributes"`
	UseEffects       []ItemTemplateUseEffectRow      `json:"use_effects"`
	EquipEffects     []ItemTemplateEquipEffectRow    `json:"equip_effects"`
	RefineInfos      []ItemTemplateRefineInfoRow     `json:"refine_infos"`
	RefineMaterials  []ItemTemplateRefineMaterialRow `json:"refine_materials"`
}

type ItemTemplateRow struct {
	Vnum              uint32 `json:"vnum"`
	Name              string `json:"name"`
	Stackable         bool   `json:"stackable"`
	MaxCount          uint16 `json:"max_count"`
	ShopBuyPrice      uint64 `json:"shop_buy_price,omitempty"`
	ShopSellPrice     uint64 `json:"shop_sell_price,omitempty"`
	Refineable        bool   `json:"refineable,omitempty"`
	RefineRejectText  string `json:"refine_reject_message,omitempty"`
	Save              bool   `json:"save,omitempty"`
	SellCountPerGold  bool   `json:"sell_count_per_gold,omitempty"`
	SlowQuery         bool   `json:"slow_query,omitempty"`
	Highlight         bool   `json:"highlight,omitempty"`
	Rare              bool   `json:"rare,omitempty"`
	Unique            bool   `json:"unique_item,omitempty"`
	MakeCount         bool   `json:"make_count,omitempty"`
	Irremovable       bool   `json:"irremovable,omitempty"`
	ConfirmWhenUse    bool   `json:"confirm_when_use,omitempty"`
	QuestUse          bool   `json:"quest_use,omitempty"`
	QuestUseMultiple  bool   `json:"quest_use_multiple,omitempty"`
	Log               bool   `json:"log,omitempty"`
	Applicable        bool   `json:"applicable,omitempty"`
	AppearanceVnum    uint32 `json:"appearance_vnum,omitempty"`
	AntiSell          bool   `json:"anti_sell,omitempty"`
	AntiDrop          bool   `json:"anti_drop,omitempty"`
	AntiGive          bool   `json:"anti_give,omitempty"`
	AntiStack         bool   `json:"anti_stack,omitempty"`
	AntiGet           bool   `json:"anti_get,omitempty"`
	AntiMale          bool   `json:"anti_male,omitempty"`
	AntiFemale        bool   `json:"anti_female,omitempty"`
	AntiWarrior       bool   `json:"anti_warrior,omitempty"`
	AntiAssassin      bool   `json:"anti_assassin,omitempty"`
	AntiSura          bool   `json:"anti_sura,omitempty"`
	AntiShaman        bool   `json:"anti_shaman,omitempty"`
	AntiEmpireA       bool   `json:"anti_empire_a,omitempty"`
	AntiEmpireB       bool   `json:"anti_empire_b,omitempty"`
	AntiEmpireC       bool   `json:"anti_empire_c,omitempty"`
	AntiSave          bool   `json:"anti_save,omitempty"`
	AntiPKDrop        bool   `json:"anti_pk_drop,omitempty"`
	AntiMyShop        bool   `json:"anti_myshop,omitempty"`
	MyShopRejectText  string `json:"myshop_reject_message,omitempty"`
	AntiSafebox       bool   `json:"anti_safebox,omitempty"`
	SafeboxRejectText string `json:"safebox_reject_message,omitempty"`
	MinLevel          uint8  `json:"min_level,omitempty"`
	EquipSlot         string `json:"equip_slot,omitempty"`
	UseRejectText     string `json:"use_reject_message,omitempty"`
	BuyRejectText     string `json:"buy_reject_message,omitempty"`
	DropRejectText    string `json:"drop_reject_message,omitempty"`
	GiveRejectText    string `json:"give_reject_message,omitempty"`
	PickupRejectText  string `json:"pickup_reject_message,omitempty"`
	SellRejectText    string `json:"sell_reject_message,omitempty"`
	EquipRejectText   string `json:"equip_reject_message,omitempty"`
	UnequipRejectText string `json:"unequip_reject_message,omitempty"`
	PickupRange       uint16 `json:"pickup_range,omitempty"`
}

type ItemTemplateSocketRow struct {
	Vnum     uint32 `json:"vnum"`
	Position uint8  `json:"position"`
	Value    int32  `json:"value"`
}

type ItemTemplateAttributeRow struct {
	Vnum     uint32 `json:"vnum"`
	Position uint8  `json:"position"`
	Type     uint8  `json:"type"`
	Value    int16  `json:"value"`
}

type ItemTemplateUseEffectRow struct {
	Vnum              uint32 `json:"vnum"`
	PointType         uint8  `json:"point_type"`
	PointIndex        uint8  `json:"point_index"`
	PointDelta        int32  `json:"point_delta"`
	ConsumeCount      uint16 `json:"consume_count"`
	Message           string `json:"message"`
	InfoMessage       string `json:"info_message,omitempty"`
	SpecialEffectType uint8  `json:"special_effect_type,omitempty"`
}

type ItemTemplateEquipEffectRow struct {
	Vnum       uint32 `json:"vnum"`
	PointType  uint8  `json:"point_type"`
	PointIndex uint8  `json:"point_index"`
	PointDelta int32  `json:"point_delta"`
}

type ItemTemplateRefineInfoRow struct {
	Vnum        uint32 `json:"vnum"`
	ResultVnum  uint32 `json:"result_vnum"`
	Cost        int32  `json:"cost"`
	Probability int32  `json:"probability"`
	KeepOnFail  bool   `json:"keep_on_fail,omitempty"`
}

type ItemTemplateRefineMaterialRow struct {
	Vnum     uint32 `json:"vnum"`
	Position uint8  `json:"position"`
	ItemVnum uint32 `json:"item_vnum"`
	Count    int32  `json:"count"`
}

func ExportItemTemplateState(snapshot Snapshot) (ItemTemplateStateExport, error) {
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("%w: validate item-template migration export", err)
	}
	export := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates:        []ItemTemplateRow{},
		Sockets:          []ItemTemplateSocketRow{},
		Attributes:       []ItemTemplateAttributeRow{},
		UseEffects:       []ItemTemplateUseEffectRow{},
		EquipEffects:     []ItemTemplateEquipEffectRow{},
		RefineInfos:      []ItemTemplateRefineInfoRow{},
		RefineMaterials:  []ItemTemplateRefineMaterialRow{},
	}
	seen := make(map[uint32]struct{}, len(normalized.Templates))
	for _, template := range normalized.Templates {
		if _, ok := seen[template.Vnum]; ok {
			return ItemTemplateStateExport{}, ErrInvalidSnapshot
		}
		seen[template.Vnum] = struct{}{}
		if template.ShopBuyPrice > uint64(^uint32(0)) || template.ShopSellPrice > uint64(1<<31-1) {
			return ItemTemplateStateExport{}, ErrInvalidSnapshot
		}
		export.Templates = append(export.Templates, itemTemplateRowForExport(template))
		for i, value := range template.Sockets {
			if value == 0 {
				continue
			}
			export.Sockets = append(export.Sockets, ItemTemplateSocketRow{Vnum: template.Vnum, Position: uint8(i), Value: value})
		}
		for i, attribute := range template.Attributes {
			if attribute == (Attribute{}) {
				continue
			}
			export.Attributes = append(export.Attributes, ItemTemplateAttributeRow{Vnum: template.Vnum, Position: uint8(i), Type: attribute.Type, Value: attribute.Value})
		}
		if template.UseEffect != nil {
			consumeCount := template.UseEffect.ConsumeCount
			if consumeCount == 0 {
				consumeCount = 1
			}
			export.UseEffects = append(export.UseEffects, ItemTemplateUseEffectRow{Vnum: template.Vnum, PointType: template.UseEffect.PointType, PointIndex: template.UseEffect.PointIndex, PointDelta: template.UseEffect.PointDelta, ConsumeCount: consumeCount, Message: template.UseEffect.Message, InfoMessage: template.UseEffect.InfoMessage, SpecialEffectType: template.UseEffect.SpecialEffectType})
		}
		if template.EquipEffect != nil {
			export.EquipEffects = append(export.EquipEffects, ItemTemplateEquipEffectRow{Vnum: template.Vnum, PointType: template.EquipEffect.PointType, PointIndex: template.EquipEffect.PointIndex, PointDelta: template.EquipEffect.PointDelta})
		}
		if template.RefineInfo != nil {
			export.RefineInfos = append(export.RefineInfos, ItemTemplateRefineInfoRow{
				Vnum:        template.Vnum,
				ResultVnum:  template.RefineInfo.ResultVnum,
				Cost:        template.RefineInfo.Cost,
				Probability: template.RefineInfo.Probability,
				KeepOnFail:  template.RefineInfo.KeepOnFail,
			})
			for i, material := range template.RefineInfo.Materials {
				export.RefineMaterials = append(export.RefineMaterials, ItemTemplateRefineMaterialRow{Vnum: template.Vnum, Position: uint8(i), ItemVnum: material.Vnum, Count: material.Count})
			}
		}
	}
	return export, nil
}

func (s *FileStore) ExportItemTemplateState() (ItemTemplateStateExport, error) {
	if s == nil || s.path == "" {
		return ItemTemplateStateExport{}, ErrStorePathRequired
	}
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return ExportItemTemplateState(Snapshot{})
		}
		return ItemTemplateStateExport{}, err
	}
	return ExportItemTemplateState(snapshot)
}

func itemTemplateRowForExport(template Template) ItemTemplateRow {
	return ItemTemplateRow{
		Vnum:              template.Vnum,
		Name:              template.Name,
		Stackable:         template.Stackable,
		MaxCount:          template.MaxCount,
		ShopBuyPrice:      template.ShopBuyPrice,
		ShopSellPrice:     template.ShopSellPrice,
		Refineable:        template.Refineable,
		RefineRejectText:  template.RefineRejectText,
		Save:              template.Save,
		SellCountPerGold:  template.SellCountPerGold,
		SlowQuery:         template.SlowQuery,
		Highlight:         template.Highlight,
		Rare:              template.Rare,
		Unique:            template.Unique,
		MakeCount:         template.MakeCount,
		Irremovable:       template.Irremovable,
		ConfirmWhenUse:    template.ConfirmWhenUse,
		QuestUse:          template.QuestUse,
		QuestUseMultiple:  template.QuestUseMultiple,
		Log:               template.Log,
		Applicable:        template.Applicable,
		AppearanceVnum:    template.AppearanceVnum,
		AntiSell:          template.AntiSell,
		AntiDrop:          template.AntiDrop,
		AntiGive:          template.AntiGive,
		AntiStack:         template.AntiStack,
		AntiGet:           template.AntiGet,
		AntiMale:          template.AntiMale,
		AntiFemale:        template.AntiFemale,
		AntiWarrior:       template.AntiWarrior,
		AntiAssassin:      template.AntiAssassin,
		AntiSura:          template.AntiSura,
		AntiShaman:        template.AntiShaman,
		AntiEmpireA:       template.AntiEmpireA,
		AntiEmpireB:       template.AntiEmpireB,
		AntiEmpireC:       template.AntiEmpireC,
		AntiSave:          template.AntiSave,
		AntiPKDrop:        template.AntiPKDrop,
		AntiMyShop:        template.AntiMyShop,
		MyShopRejectText:  template.MyShopRejectText,
		AntiSafebox:       template.AntiSafebox,
		SafeboxRejectText: template.SafeboxRejectText,
		MinLevel:          template.MinLevel,
		EquipSlot:         template.EquipSlot,
		UseRejectText:     template.UseRejectText,
		BuyRejectText:     template.BuyRejectText,
		DropRejectText:    template.DropRejectText,
		GiveRejectText:    template.GiveRejectText,
		PickupRejectText:  template.PickupRejectText,
		SellRejectText:    template.SellRejectText,
		EquipRejectText:   template.EquipRejectText,
		UnequipRejectText: template.UnequipRejectText,
		PickupRange:       template.PickupRange,
	}
}
