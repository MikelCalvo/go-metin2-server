package itemstore

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidItemTemplateStateExport reports that a retained item-template-state
// export failed the 0009 migration-shaped quarantine contract.
var ErrInvalidItemTemplateStateExport = errors.New("invalid item template state export")

// ItemTemplateStateQuarantineSummary is the metadata-only result of validating
// or quarantining a retained item-template-state export. It never includes
// template payloads, SQL, DSNs, or item-template snapshot bytes.
type ItemTemplateStateQuarantineSummary struct {
	TemplateCount       int      `json:"template_count"`
	SocketCount         int      `json:"socket_count"`
	AttributeCount      int      `json:"attribute_count"`
	UseEffectCount      int      `json:"use_effect_count"`
	EquipEffectCount    int      `json:"equip_effect_count"`
	RefineInfoCount     int      `json:"refine_info_count"`
	RefineMaterialCount int      `json:"refine_material_count"`
	Vnums               []uint32 `json:"vnums"`
}

// ItemTemplateStateQuarantineResult pairs the metadata-only quarantine summary
// with a canonicalized export ready for later offline review or backfill tools.
type ItemTemplateStateQuarantineResult struct {
	Summary ItemTemplateStateQuarantineSummary `json:"summary"`
	Export  ItemTemplateStateExport            `json:"export"`
}

// ValidateItemTemplateStateExport fails closed when a retained export does not
// match the 0009_item_template_refine_info shape. It does not open a database,
// write item-template snapshots, or mutate the supplied export.
func ValidateItemTemplateStateExport(export ItemTemplateStateExport) (ItemTemplateStateQuarantineSummary, error) {
	canonical, summary, err := canonicalizeItemTemplateStateExport(export)
	if err != nil {
		return ItemTemplateStateQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineItemTemplateStateExport validates a retained export and returns a
// canonicalized copy ordered by ascending vnum and stable child-row keys. It
// never opens a database or mutates item-template snapshots.
func QuarantineItemTemplateStateExport(export ItemTemplateStateExport) (ItemTemplateStateExport, ItemTemplateStateQuarantineSummary, error) {
	return canonicalizeItemTemplateStateExport(export)
}

func canonicalizeItemTemplateStateExport(export ItemTemplateStateExport) (ItemTemplateStateExport, ItemTemplateStateQuarantineSummary, error) {
	if export.MigrationVersion != ItemTemplateStateMigrationVersion {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidItemTemplateStateExport, export.MigrationVersion)
	}
	if export.MigrationName != ItemTemplateStateMigrationName {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidItemTemplateStateExport, export.MigrationName)
	}
	if export.Templates == nil {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: templates must be present", ErrInvalidItemTemplateStateExport)
	}
	if export.Sockets == nil {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: sockets must be present", ErrInvalidItemTemplateStateExport)
	}
	if export.Attributes == nil {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: attributes must be present", ErrInvalidItemTemplateStateExport)
	}
	if export.UseEffects == nil {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: use_effects must be present", ErrInvalidItemTemplateStateExport)
	}
	if export.EquipEffects == nil {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: equip_effects must be present", ErrInvalidItemTemplateStateExport)
	}
	if export.RefineInfos == nil {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: refine_infos must be present", ErrInvalidItemTemplateStateExport)
	}
	if export.RefineMaterials == nil {
		return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: refine_materials must be present", ErrInvalidItemTemplateStateExport)
	}

	templatesByVnum := make(map[uint32]Template, len(export.Templates))
	canonicalTemplates := make([]ItemTemplateRow, 0, len(export.Templates))
	vnums := make([]uint32, 0, len(export.Templates))

	for _, row := range export.Templates {
		template, err := templateFromExportRow(row)
		if err != nil {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, err
		}
		if _, exists := templatesByVnum[template.Vnum]; exists {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: duplicate template vnum %d", ErrInvalidItemTemplateStateExport, template.Vnum)
		}
		templatesByVnum[template.Vnum] = template
		canonicalTemplates = append(canonicalTemplates, itemTemplateRowForExport(template))
		vnums = append(vnums, template.Vnum)
	}

	socketsByVnum := make(map[uint32]map[uint8]int32)
	seenSocketKeys := make(map[string]struct{}, len(export.Sockets))
	for _, row := range export.Sockets {
		if _, ok := templatesByVnum[row.Vnum]; !ok {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: socket vnum %d is not present in templates", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		if int(row.Position) >= ItemSocketCount {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: socket position %d out of range for vnum %d", ErrInvalidItemTemplateStateExport, row.Position, row.Vnum)
		}
		if row.Value == 0 {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: socket value must be non-zero for vnum %d position %d", ErrInvalidItemTemplateStateExport, row.Vnum, row.Position)
		}
		key := fmt.Sprintf("%d:%d", row.Vnum, row.Position)
		if _, exists := seenSocketKeys[key]; exists {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: duplicate socket vnum=%d position=%d", ErrInvalidItemTemplateStateExport, row.Vnum, row.Position)
		}
		seenSocketKeys[key] = struct{}{}
		positions, ok := socketsByVnum[row.Vnum]
		if !ok {
			positions = make(map[uint8]int32, ItemSocketCount)
			socketsByVnum[row.Vnum] = positions
		}
		positions[row.Position] = row.Value
	}

	attributesByVnum := make(map[uint32]map[uint8]Attribute)
	seenAttributeKeys := make(map[string]struct{}, len(export.Attributes))
	for _, row := range export.Attributes {
		if _, ok := templatesByVnum[row.Vnum]; !ok {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: attribute vnum %d is not present in templates", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		if int(row.Position) >= ItemAttributeCount {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: attribute position %d out of range for vnum %d", ErrInvalidItemTemplateStateExport, row.Position, row.Vnum)
		}
		if row.Type == 0 || row.Value == 0 {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: attribute type/value must be non-zero for vnum %d position %d", ErrInvalidItemTemplateStateExport, row.Vnum, row.Position)
		}
		key := fmt.Sprintf("%d:%d", row.Vnum, row.Position)
		if _, exists := seenAttributeKeys[key]; exists {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: duplicate attribute vnum=%d position=%d", ErrInvalidItemTemplateStateExport, row.Vnum, row.Position)
		}
		seenAttributeKeys[key] = struct{}{}
		positions, ok := attributesByVnum[row.Vnum]
		if !ok {
			positions = make(map[uint8]Attribute, ItemAttributeCount)
			attributesByVnum[row.Vnum] = positions
		}
		positions[row.Position] = Attribute{Type: row.Type, Value: row.Value}
	}

	useEffectsByVnum := make(map[uint32]UseEffect, len(export.UseEffects))
	for _, row := range export.UseEffects {
		template, ok := templatesByVnum[row.Vnum]
		if !ok {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: use_effect vnum %d is not present in templates", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		if _, exists := useEffectsByVnum[row.Vnum]; exists {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: duplicate use_effect for vnum %d", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		effect := UseEffect{
			PointType:         row.PointType,
			PointIndex:        row.PointIndex,
			PointDelta:        row.PointDelta,
			ConsumeCount:      row.ConsumeCount,
			Message:           strings.TrimSpace(row.Message),
			InfoMessage:       strings.TrimSpace(row.InfoMessage),
			SpecialEffectType: row.SpecialEffectType,
		}
		if !validUseEffect(&effect, template) {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: invalid use_effect for vnum %d", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		useEffectsByVnum[row.Vnum] = effect
	}

	equipEffectsByVnum := make(map[uint32]PointEffect, len(export.EquipEffects))
	for _, row := range export.EquipEffects {
		if _, ok := templatesByVnum[row.Vnum]; !ok {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: equip_effect vnum %d is not present in templates", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		if _, exists := equipEffectsByVnum[row.Vnum]; exists {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: duplicate equip_effect for vnum %d", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		effect := PointEffect{
			PointType:  row.PointType,
			PointIndex: row.PointIndex,
			PointDelta: row.PointDelta,
		}
		if !validPointEffect(&effect) {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: invalid equip_effect for vnum %d", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		equipEffectsByVnum[row.Vnum] = effect
	}

	refineInfosByVnum := make(map[uint32]ItemTemplateRefineInfoRow, len(export.RefineInfos))
	for _, row := range export.RefineInfos {
		template, ok := templatesByVnum[row.Vnum]
		if !ok {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: refine_info vnum %d is not present in templates", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		if !template.Refineable {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: refine_info requires refineable template vnum %d", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		if _, exists := refineInfosByVnum[row.Vnum]; exists {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: duplicate refine_info for vnum %d", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		if row.ResultVnum == 0 || row.Cost < 0 || row.Probability < 0 || row.Probability > 100 {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: invalid refine_info bounds for vnum %d", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		refineInfosByVnum[row.Vnum] = row
	}

	refineMaterialsByVnum := make(map[uint32]map[uint8]ItemTemplateRefineMaterialRow)
	seenRefineMaterialKeys := make(map[string]struct{}, len(export.RefineMaterials))
	for _, row := range export.RefineMaterials {
		if _, ok := refineInfosByVnum[row.Vnum]; !ok {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: refine_material vnum %d is not present in refine_infos", ErrInvalidItemTemplateStateExport, row.Vnum)
		}
		if int(row.Position) >= MaxRefineMaterialCount {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: refine_material position %d out of range for vnum %d", ErrInvalidItemTemplateStateExport, row.Position, row.Vnum)
		}
		if row.ItemVnum == 0 || row.Count <= 0 {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: invalid refine_material for vnum %d position %d", ErrInvalidItemTemplateStateExport, row.Vnum, row.Position)
		}
		key := fmt.Sprintf("%d:%d", row.Vnum, row.Position)
		if _, exists := seenRefineMaterialKeys[key]; exists {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: duplicate refine_material vnum=%d position=%d", ErrInvalidItemTemplateStateExport, row.Vnum, row.Position)
		}
		seenRefineMaterialKeys[key] = struct{}{}
		positions, ok := refineMaterialsByVnum[row.Vnum]
		if !ok {
			positions = make(map[uint8]ItemTemplateRefineMaterialRow, MaxRefineMaterialCount)
			refineMaterialsByVnum[row.Vnum] = positions
		}
		positions[row.Position] = row
	}
	for vnum, positions := range refineMaterialsByVnum {
		for index := 0; index < len(positions); index++ {
			if _, ok := positions[uint8(index)]; !ok {
				return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: refine_material positions for vnum %d must be contiguous from 0", ErrInvalidItemTemplateStateExport, vnum)
			}
		}
	}

	for vnum, template := range templatesByVnum {
		if sockets := socketsByVnum[vnum]; sockets != nil {
			for position, value := range sockets {
				template.Sockets[position] = value
			}
		}
		if attributes := attributesByVnum[vnum]; attributes != nil {
			for position, attribute := range attributes {
				template.Attributes[position] = attribute
			}
		}
		if effect, ok := useEffectsByVnum[vnum]; ok {
			copied := effect
			template.UseEffect = &copied
		}
		if effect, ok := equipEffectsByVnum[vnum]; ok {
			copied := effect
			template.EquipEffect = &copied
		}
		if info, ok := refineInfosByVnum[vnum]; ok {
			materials := make([]RefineMaterial, 0, len(refineMaterialsByVnum[vnum]))
			for index := 0; index < len(refineMaterialsByVnum[vnum]); index++ {
				material := refineMaterialsByVnum[vnum][uint8(index)]
				materials = append(materials, RefineMaterial{Vnum: material.ItemVnum, Count: material.Count})
			}
			template.RefineInfo = &RefineInfo{
				ResultVnum:     info.ResultVnum,
				Cost:           info.Cost,
				Probability:    info.Probability,
				KeepOnFail:     info.KeepOnFail,
				FailResultVnum: info.FailResultVnum,
				Materials:      materials,
			}
		} else if len(refineMaterialsByVnum[vnum]) > 0 {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: refine_materials without refine_info for vnum %d", ErrInvalidItemTemplateStateExport, vnum)
		}
		if !validTemplate(template) {
			return ItemTemplateStateExport{}, ItemTemplateStateQuarantineSummary{}, fmt.Errorf("%w: reconstructed template vnum %d is invalid", ErrInvalidItemTemplateStateExport, vnum)
		}
		templatesByVnum[vnum] = template
	}

	sort.Slice(vnums, func(i, j int) bool { return vnums[i] < vnums[j] })
	sort.Slice(canonicalTemplates, func(i, j int) bool { return canonicalTemplates[i].Vnum < canonicalTemplates[j].Vnum })

	canonicalSockets := make([]ItemTemplateSocketRow, 0, len(export.Sockets))
	canonicalAttributes := make([]ItemTemplateAttributeRow, 0, len(export.Attributes))
	canonicalUseEffects := make([]ItemTemplateUseEffectRow, 0, len(export.UseEffects))
	canonicalEquipEffects := make([]ItemTemplateEquipEffectRow, 0, len(export.EquipEffects))
	canonicalRefineInfos := make([]ItemTemplateRefineInfoRow, 0, len(export.RefineInfos))
	canonicalRefineMaterials := make([]ItemTemplateRefineMaterialRow, 0, len(export.RefineMaterials))

	for _, vnum := range vnums {
		template := templatesByVnum[vnum]
		for i, value := range template.Sockets {
			if value == 0 {
				continue
			}
			canonicalSockets = append(canonicalSockets, ItemTemplateSocketRow{Vnum: vnum, Position: uint8(i), Value: value})
		}
		for i, attribute := range template.Attributes {
			if attribute == (Attribute{}) {
				continue
			}
			canonicalAttributes = append(canonicalAttributes, ItemTemplateAttributeRow{Vnum: vnum, Position: uint8(i), Type: attribute.Type, Value: attribute.Value})
		}
		if template.UseEffect != nil {
			consumeCount := template.UseEffect.ConsumeCount
			if consumeCount == 0 {
				consumeCount = 1
			}
			canonicalUseEffects = append(canonicalUseEffects, ItemTemplateUseEffectRow{
				Vnum:              vnum,
				PointType:         template.UseEffect.PointType,
				PointIndex:        template.UseEffect.PointIndex,
				PointDelta:        template.UseEffect.PointDelta,
				ConsumeCount:      consumeCount,
				Message:           template.UseEffect.Message,
				InfoMessage:       template.UseEffect.InfoMessage,
				SpecialEffectType: template.UseEffect.SpecialEffectType,
			})
		}
		if template.EquipEffect != nil {
			canonicalEquipEffects = append(canonicalEquipEffects, ItemTemplateEquipEffectRow{
				Vnum:       vnum,
				PointType:  template.EquipEffect.PointType,
				PointIndex: template.EquipEffect.PointIndex,
				PointDelta: template.EquipEffect.PointDelta,
			})
		}
		if template.RefineInfo != nil {
			canonicalRefineInfos = append(canonicalRefineInfos, ItemTemplateRefineInfoRow{
				Vnum:           vnum,
				ResultVnum:     template.RefineInfo.ResultVnum,
				Cost:           template.RefineInfo.Cost,
				Probability:    template.RefineInfo.Probability,
				KeepOnFail:     template.RefineInfo.KeepOnFail,
				FailResultVnum: template.RefineInfo.FailResultVnum,
			})
			for i, material := range template.RefineInfo.Materials {
				canonicalRefineMaterials = append(canonicalRefineMaterials, ItemTemplateRefineMaterialRow{
					Vnum:     vnum,
					Position: uint8(i),
					ItemVnum: material.Vnum,
					Count:    material.Count,
				})
			}
		}
	}

	canonical := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates:        canonicalTemplates,
		Sockets:          canonicalSockets,
		Attributes:       canonicalAttributes,
		UseEffects:       canonicalUseEffects,
		EquipEffects:     canonicalEquipEffects,
		RefineInfos:      canonicalRefineInfos,
		RefineMaterials:  canonicalRefineMaterials,
	}
	summary := ItemTemplateStateQuarantineSummary{
		TemplateCount:       len(canonical.Templates),
		SocketCount:         len(canonical.Sockets),
		AttributeCount:      len(canonical.Attributes),
		UseEffectCount:      len(canonical.UseEffects),
		EquipEffectCount:    len(canonical.EquipEffects),
		RefineInfoCount:     len(canonical.RefineInfos),
		RefineMaterialCount: len(canonical.RefineMaterials),
		Vnums:               vnums,
	}
	if summary.Vnums == nil {
		summary.Vnums = []uint32{}
	}
	return canonical, summary, nil
}

func templateFromExportRow(row ItemTemplateRow) (Template, error) {
	name := strings.TrimSpace(row.Name)
	if row.Vnum == 0 {
		return Template{}, fmt.Errorf("%w: template vnum must be > 0", ErrInvalidItemTemplateStateExport)
	}
	if name == "" {
		return Template{}, fmt.Errorf("%w: template name is required for vnum %d", ErrInvalidItemTemplateStateExport, row.Vnum)
	}
	if row.Name != name {
		return Template{}, fmt.Errorf("%w: template name %q has leading or trailing whitespace", ErrInvalidItemTemplateStateExport, row.Name)
	}
	template := Template{
		Vnum:              row.Vnum,
		Name:              name,
		Stackable:         row.Stackable,
		MaxCount:          row.MaxCount,
		ShopBuyPrice:      row.ShopBuyPrice,
		ShopSellPrice:     row.ShopSellPrice,
		Refineable:        row.Refineable,
		RefineRejectText:  strings.TrimSpace(row.RefineRejectText),
		Save:              row.Save,
		SellCountPerGold:  row.SellCountPerGold,
		SlowQuery:         row.SlowQuery,
		Highlight:         row.Highlight,
		Rare:              row.Rare,
		Unique:            row.Unique,
		MakeCount:         row.MakeCount,
		Irremovable:       row.Irremovable,
		ConfirmWhenUse:    row.ConfirmWhenUse,
		QuestUse:          row.QuestUse,
		QuestUseMultiple:  row.QuestUseMultiple,
		Log:               row.Log,
		Applicable:        row.Applicable,
		AppearanceVnum:    row.AppearanceVnum,
		AntiSell:          row.AntiSell,
		AntiDrop:          row.AntiDrop,
		AntiGive:          row.AntiGive,
		AntiStack:         row.AntiStack,
		AntiGet:           row.AntiGet,
		AntiMale:          row.AntiMale,
		AntiFemale:        row.AntiFemale,
		AntiWarrior:       row.AntiWarrior,
		AntiAssassin:      row.AntiAssassin,
		AntiSura:          row.AntiSura,
		AntiShaman:        row.AntiShaman,
		AntiEmpireA:       row.AntiEmpireA,
		AntiEmpireB:       row.AntiEmpireB,
		AntiEmpireC:       row.AntiEmpireC,
		AntiSave:          row.AntiSave,
		AntiPKDrop:        row.AntiPKDrop,
		AntiMyShop:        row.AntiMyShop,
		MyShopRejectText:  strings.TrimSpace(row.MyShopRejectText),
		AntiSafebox:       row.AntiSafebox,
		SafeboxRejectText: strings.TrimSpace(row.SafeboxRejectText),
		MinLevel:          row.MinLevel,
		EquipSlot:         normalizeEquipSlot(row.EquipSlot),
		UseRejectText:     strings.TrimSpace(row.UseRejectText),
		BuyRejectText:     strings.TrimSpace(row.BuyRejectText),
		DropRejectText:    strings.TrimSpace(row.DropRejectText),
		GiveRejectText:    strings.TrimSpace(row.GiveRejectText),
		PickupRejectText:  strings.TrimSpace(row.PickupRejectText),
		SellRejectText:    strings.TrimSpace(row.SellRejectText),
		EquipRejectText:   strings.TrimSpace(row.EquipRejectText),
		UnequipRejectText: strings.TrimSpace(row.UnequipRejectText),
		PickupRange:       row.PickupRange,
	}
	return template, nil
}
