package accountstore

import (
	"errors"
	"fmt"
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

// ErrInvalidCharacterItemStateExport reports that a retained item-state export
// failed the 0003 migration-shaped quarantine contract.
var ErrInvalidCharacterItemStateExport = errors.New("invalid character item-state export")

// CharacterItemStateQuarantineSummary is the metadata-only result of validating
// or quarantining a retained character item-state export. It never includes item
// payloads, SQL, DSNs, or account snapshot bytes.
type CharacterItemStateQuarantineSummary struct {
	CharacterCount     int      `json:"character_count"`
	InventoryItemCount int      `json:"inventory_item_count"`
	EquipmentItemCount int      `json:"equipment_item_count"`
	QuickslotCount     int      `json:"quickslot_count"`
	CharacterIDs       []uint32 `json:"character_ids"`
}

// CharacterItemStateQuarantineResult pairs the metadata-only quarantine summary
// with a canonicalized export ready for later offline review or backfill tools.
type CharacterItemStateQuarantineResult struct {
	Summary CharacterItemStateQuarantineSummary `json:"summary"`
	Export  CharacterItemStateExport            `json:"export"`
}

// ValidateCharacterItemStateExport fails closed when a retained export does not
// match the 0003_character_item_state shape. It does not open a database, write
// account snapshots, or mutate the supplied export.
func ValidateCharacterItemStateExport(export CharacterItemStateExport) (CharacterItemStateQuarantineSummary, error) {
	canonical, summary, err := canonicalizeCharacterItemStateExport(export)
	if err != nil {
		return CharacterItemStateQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineCharacterItemStateExport validates a retained export and returns a
// canonicalized copy ordered by character id and the migration-stable inventory,
// equipment, and quickslot sort keys. It never opens a database or mutates
// account snapshots.
func QuarantineCharacterItemStateExport(export CharacterItemStateExport) (CharacterItemStateExport, CharacterItemStateQuarantineSummary, error) {
	return canonicalizeCharacterItemStateExport(export)
}

func canonicalizeCharacterItemStateExport(export CharacterItemStateExport) (CharacterItemStateExport, CharacterItemStateQuarantineSummary, error) {
	if export.MigrationVersion != CharacterItemStateMigrationVersion {
		return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidCharacterItemStateExport, export.MigrationVersion)
	}
	if export.MigrationName != CharacterItemStateMigrationName {
		return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidCharacterItemStateExport, export.MigrationName)
	}
	if export.InventoryItems == nil {
		return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: inventory_items must be present", ErrInvalidCharacterItemStateExport)
	}
	if export.EquipmentItems == nil {
		return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: equipment_items must be present", ErrInvalidCharacterItemStateExport)
	}
	if export.Quickslots == nil {
		return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: quickslots must be present", ErrInvalidCharacterItemStateExport)
	}

	characterIDs := make(map[uint32]struct{})
	seenItemIDs := make(map[uint64]string, len(export.InventoryItems)+len(export.EquipmentItems))
	seenInventorySlots := make(map[string]struct{}, len(export.InventoryItems))
	seenEquipmentSlots := make(map[string]struct{}, len(export.EquipmentItems))
	seenQuickslotPositions := make(map[string]struct{}, len(export.Quickslots))

	inventoryItems := make([]CharacterInventoryItemRow, 0, len(export.InventoryItems))
	for _, row := range export.InventoryItems {
		if err := validateQuarantineInventoryRow(row); err != nil {
			return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, err
		}
		slotKey := fmt.Sprintf("%d:%d", row.CharacterID, row.Slot)
		if _, exists := seenInventorySlots[slotKey]; exists {
			return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: duplicate character_id=%d inventory slot %d", ErrInvalidCharacterItemStateExport, row.CharacterID, row.Slot)
		}
		seenInventorySlots[slotKey] = struct{}{}
		if previous, ok := seenItemIDs[row.ID]; ok {
			return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: item id %d appears in %s and inventory character %d slot %d", ErrInvalidCharacterItemStateExport, row.ID, previous, row.CharacterID, row.Slot)
		}
		seenItemIDs[row.ID] = fmt.Sprintf("inventory character %d slot %d", row.CharacterID, row.Slot)
		characterIDs[row.CharacterID] = struct{}{}
		inventoryItems = append(inventoryItems, row)
	}

	equipmentItems := make([]CharacterEquipmentItemRow, 0, len(export.EquipmentItems))
	for _, row := range export.EquipmentItems {
		if err := validateQuarantineEquipmentRow(row); err != nil {
			return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, err
		}
		slotKey := fmt.Sprintf("%d:%s", row.CharacterID, row.EquipSlot)
		if _, exists := seenEquipmentSlots[slotKey]; exists {
			return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: duplicate character_id=%d equip_slot %q", ErrInvalidCharacterItemStateExport, row.CharacterID, row.EquipSlot)
		}
		seenEquipmentSlots[slotKey] = struct{}{}
		if previous, ok := seenItemIDs[row.ID]; ok {
			return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: item id %d appears in %s and equipment character %d slot %s", ErrInvalidCharacterItemStateExport, row.ID, previous, row.CharacterID, row.EquipSlot)
		}
		seenItemIDs[row.ID] = fmt.Sprintf("equipment character %d slot %s", row.CharacterID, row.EquipSlot)
		characterIDs[row.CharacterID] = struct{}{}
		equipmentItems = append(equipmentItems, row)
	}

	quickslots := make([]CharacterQuickslotRow, 0, len(export.Quickslots))
	for _, row := range export.Quickslots {
		if err := validateQuarantineQuickslotRow(row); err != nil {
			return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, err
		}
		positionKey := fmt.Sprintf("%d:%d", row.CharacterID, row.Position)
		if _, exists := seenQuickslotPositions[positionKey]; exists {
			return CharacterItemStateExport{}, CharacterItemStateQuarantineSummary{}, fmt.Errorf("%w: duplicate character_id=%d quickslot position %d", ErrInvalidCharacterItemStateExport, row.CharacterID, row.Position)
		}
		seenQuickslotPositions[positionKey] = struct{}{}
		characterIDs[row.CharacterID] = struct{}{}
		quickslots = append(quickslots, row)
	}

	sort.SliceStable(inventoryItems, func(i, j int) bool {
		if inventoryItems[i].CharacterID != inventoryItems[j].CharacterID {
			return inventoryItems[i].CharacterID < inventoryItems[j].CharacterID
		}
		if inventoryItems[i].Slot != inventoryItems[j].Slot {
			return inventoryItems[i].Slot < inventoryItems[j].Slot
		}
		return inventoryItems[i].ID < inventoryItems[j].ID
	})
	sort.SliceStable(equipmentItems, func(i, j int) bool {
		if equipmentItems[i].CharacterID != equipmentItems[j].CharacterID {
			return equipmentItems[i].CharacterID < equipmentItems[j].CharacterID
		}
		leftRank := equipmentSlotNameExportRank(equipmentItems[i].EquipSlot)
		rightRank := equipmentSlotNameExportRank(equipmentItems[j].EquipSlot)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if equipmentItems[i].EquipSlot != equipmentItems[j].EquipSlot {
			return equipmentItems[i].EquipSlot < equipmentItems[j].EquipSlot
		}
		return equipmentItems[i].ID < equipmentItems[j].ID
	})
	sort.SliceStable(quickslots, func(i, j int) bool {
		if quickslots[i].CharacterID != quickslots[j].CharacterID {
			return quickslots[i].CharacterID < quickslots[j].CharacterID
		}
		if quickslots[i].Position != quickslots[j].Position {
			return quickslots[i].Position < quickslots[j].Position
		}
		if quickslots[i].Type != quickslots[j].Type {
			return quickslots[i].Type < quickslots[j].Type
		}
		return quickslots[i].Slot < quickslots[j].Slot
	})

	sortedCharacterIDs := make([]uint32, 0, len(characterIDs))
	for characterID := range characterIDs {
		sortedCharacterIDs = append(sortedCharacterIDs, characterID)
	}
	sort.Slice(sortedCharacterIDs, func(i, j int) bool { return sortedCharacterIDs[i] < sortedCharacterIDs[j] })

	canonical := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems:   inventoryItems,
		EquipmentItems:   equipmentItems,
		Quickslots:       quickslots,
	}
	summary := CharacterItemStateQuarantineSummary{
		CharacterCount:     len(sortedCharacterIDs),
		InventoryItemCount: len(inventoryItems),
		EquipmentItemCount: len(equipmentItems),
		QuickslotCount:     len(quickslots),
		CharacterIDs:       sortedCharacterIDs,
	}
	if summary.CharacterIDs == nil {
		summary.CharacterIDs = []uint32{}
	}
	return canonical, summary, nil
}

func validateQuarantineInventoryRow(row CharacterInventoryItemRow) error {
	if row.CharacterID == 0 {
		return fmt.Errorf("%w: character_id must be > 0", ErrInvalidCharacterItemStateExport)
	}
	if row.ID == 0 || row.ID > maxSignedBigInt {
		return fmt.Errorf("%w: inventory item id %d out of range", ErrInvalidCharacterItemStateExport, row.ID)
	}
	if row.Vnum == 0 {
		return fmt.Errorf("%w: inventory item %d requires vnum > 0", ErrInvalidCharacterItemStateExport, row.ID)
	}
	if row.Count == 0 {
		return fmt.Errorf("%w: inventory item %d requires count > 0", ErrInvalidCharacterItemStateExport, row.ID)
	}
	if row.Slot >= inventory.CarriedInventorySlotCount {
		return fmt.Errorf("%w: inventory item %d slot %d outside migration inventory", ErrInvalidCharacterItemStateExport, row.ID, row.Slot)
	}
	if err := validateQuarantineInstanceSockets(row.ID, "inventory", row.HasSockets, row.Socket0, row.Socket1, row.Socket2); err != nil {
		return err
	}
	if err := validateQuarantineInstanceAttributes(
		row.ID,
		"inventory",
		row.HasAttributes,
		row.Attr0Type, row.Attr0Value,
		row.Attr1Type, row.Attr1Value,
		row.Attr2Type, row.Attr2Value,
		row.Attr3Type, row.Attr3Value,
		row.Attr4Type, row.Attr4Value,
		row.Attr5Type, row.Attr5Value,
		row.Attr6Type, row.Attr6Value,
	); err != nil {
		return err
	}
	return nil
}

func validateQuarantineEquipmentRow(row CharacterEquipmentItemRow) error {
	if row.CharacterID == 0 {
		return fmt.Errorf("%w: character_id must be > 0", ErrInvalidCharacterItemStateExport)
	}
	if row.ID == 0 || row.ID > maxSignedBigInt {
		return fmt.Errorf("%w: equipment item id %d out of range", ErrInvalidCharacterItemStateExport, row.ID)
	}
	if row.Vnum == 0 {
		return fmt.Errorf("%w: equipment item %d requires vnum > 0", ErrInvalidCharacterItemStateExport, row.ID)
	}
	if row.Count == 0 {
		return fmt.Errorf("%w: equipment item %d requires count > 0", ErrInvalidCharacterItemStateExport, row.ID)
	}
	slot, ok := inventory.ParseEquipmentSlot(row.EquipSlot)
	if !ok || !slot.Valid() {
		return fmt.Errorf("%w: equipment item %d has invalid equip_slot %q", ErrInvalidCharacterItemStateExport, row.ID, row.EquipSlot)
	}
	if err := validateQuarantineInstanceSockets(row.ID, "equipment", row.HasSockets, row.Socket0, row.Socket1, row.Socket2); err != nil {
		return err
	}
	if err := validateQuarantineInstanceAttributes(
		row.ID,
		"equipment",
		row.HasAttributes,
		row.Attr0Type, row.Attr0Value,
		row.Attr1Type, row.Attr1Value,
		row.Attr2Type, row.Attr2Value,
		row.Attr3Type, row.Attr3Value,
		row.Attr4Type, row.Attr4Value,
		row.Attr5Type, row.Attr5Value,
		row.Attr6Type, row.Attr6Value,
	); err != nil {
		return err
	}
	return nil
}

func validateQuarantineQuickslotRow(row CharacterQuickslotRow) error {
	if row.CharacterID == 0 {
		return fmt.Errorf("%w: character_id must be > 0", ErrInvalidCharacterItemStateExport)
	}
	if row.Position >= 36 {
		return fmt.Errorf("%w: character %d quickslot position %d out of range", ErrInvalidCharacterItemStateExport, row.CharacterID, row.Position)
	}
	switch row.Type {
	case quickslotproto.TypeNone:
		if row.Slot != 0 {
			return fmt.Errorf("%w: character %d quickslot position %d type none requires slot 0", ErrInvalidCharacterItemStateExport, row.CharacterID, row.Position)
		}
	case quickslotproto.TypeItem:
		if row.Slot >= uint8(inventory.CarriedInventorySlotCount) {
			return fmt.Errorf("%w: character %d quickslot position %d item slot %d out of range", ErrInvalidCharacterItemStateExport, row.CharacterID, row.Position, row.Slot)
		}
	case quickslotproto.TypeSkill:
		if row.Slot >= 200 {
			return fmt.Errorf("%w: character %d quickslot position %d skill slot %d out of range", ErrInvalidCharacterItemStateExport, row.CharacterID, row.Position, row.Slot)
		}
	case quickslotproto.TypeCommand:
		if row.Slot >= 60 {
			return fmt.Errorf("%w: character %d quickslot position %d command slot %d out of range", ErrInvalidCharacterItemStateExport, row.CharacterID, row.Position, row.Slot)
		}
	default:
		return fmt.Errorf("%w: character %d quickslot position %d has invalid type %d", ErrInvalidCharacterItemStateExport, row.CharacterID, row.Position, row.Type)
	}
	return nil
}

func equipmentSlotNameExportRank(name string) int {
	slot, ok := inventory.ParseEquipmentSlot(name)
	if !ok {
		return len(inventory.AllEquipmentSlots()) + 1
	}
	return equipmentSlotExportRank(slot)
}

func validateQuarantineInstanceSockets(itemID uint64, context string, hasSockets bool, socket0, socket1, socket2 int32) error {
	if hasSockets {
		return nil
	}
	if socket0 != 0 || socket1 != 0 || socket2 != 0 {
		return fmt.Errorf("%w: %s item %d has non-zero sockets without has_sockets", ErrInvalidCharacterItemStateExport, context, itemID)
	}
	return nil
}

func validateQuarantineInstanceAttributes(
	itemID uint64,
	context string,
	hasAttributes bool,
	attr0Type uint8, attr0Value int16,
	attr1Type uint8, attr1Value int16,
	attr2Type uint8, attr2Value int16,
	attr3Type uint8, attr3Value int16,
	attr4Type uint8, attr4Value int16,
	attr5Type uint8, attr5Value int16,
	attr6Type uint8, attr6Value int16,
) error {
	if hasAttributes {
		return nil
	}
	if attr0Type != 0 || attr0Value != 0 ||
		attr1Type != 0 || attr1Value != 0 ||
		attr2Type != 0 || attr2Value != 0 ||
		attr3Type != 0 || attr3Value != 0 ||
		attr4Type != 0 || attr4Value != 0 ||
		attr5Type != 0 || attr5Value != 0 ||
		attr6Type != 0 || attr6Value != 0 {
		return fmt.Errorf("%w: %s item %d has non-zero attributes without has_attributes", ErrInvalidCharacterItemStateExport, context, itemID)
	}
	return nil
}
