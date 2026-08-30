package accountstore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

const (
	CharacterItemStateMigrationVersion = 3
	CharacterItemStateMigrationName    = "character_item_state"

	CharacterItemInstanceSocketsMigrationVersion = 24
	CharacterItemInstanceSocketsMigrationName    = "character_item_instance_sockets"

	CharacterItemInstanceAttributesMigrationVersion = 27
	CharacterItemInstanceAttributesMigrationName    = "character_item_instance_attributes"
)

// CharacterItemStateExport is a deterministic, schema-shaped projection of the
// item-bearing parts of bootstrap JSON account snapshots onto the
// 0003_character_item_state migration boundary. It is intentionally a
// data-model/export contract only: it does not open a database, emit SQL, apply
// migrations, or mutate the file store.
type CharacterItemStateExport struct {
	MigrationVersion int                         `json:"migration_version"`
	MigrationName    string                      `json:"migration_name"`
	InventoryItems   []CharacterInventoryItemRow `json:"inventory_items"`
	EquipmentItems   []CharacterEquipmentItemRow `json:"equipment_items"`
	Quickslots       []CharacterQuickslotRow     `json:"quickslots"`
}

// CharacterInventoryItemRow mirrors carried-inventory item state frozen by the
// 0003_character_item_state migration, including optional additive 0024 instance
// sockets and additive 0027 instance attributes. HasSockets=false / omitted
// means nil instance sockets (template fallback); HasSockets=true including
// all-zero is authoritative. HasAttributes=false / omitted means nil instance
// attributes (template fallback); HasAttributes=true including all-zero /
// type-zero is authoritative.
type CharacterInventoryItemRow struct {
	ID            uint64              `json:"id"`
	CharacterID   uint32              `json:"character_id"`
	Slot          inventory.SlotIndex `json:"slot"`
	Vnum          uint32              `json:"vnum"`
	Count         uint16              `json:"count"`
	Locked        bool                `json:"locked,omitempty"`
	HasSockets    bool                `json:"has_sockets,omitempty"`
	Socket0       int32               `json:"socket0,omitempty"`
	Socket1       int32               `json:"socket1,omitempty"`
	Socket2       int32               `json:"socket2,omitempty"`
	HasAttributes bool                `json:"has_attributes,omitempty"`
	Attr0Type     uint8               `json:"attr0_type,omitempty"`
	Attr0Value    int16               `json:"attr0_value,omitempty"`
	Attr1Type     uint8               `json:"attr1_type,omitempty"`
	Attr1Value    int16               `json:"attr1_value,omitempty"`
	Attr2Type     uint8               `json:"attr2_type,omitempty"`
	Attr2Value    int16               `json:"attr2_value,omitempty"`
	Attr3Type     uint8               `json:"attr3_type,omitempty"`
	Attr3Value    int16               `json:"attr3_value,omitempty"`
	Attr4Type     uint8               `json:"attr4_type,omitempty"`
	Attr4Value    int16               `json:"attr4_value,omitempty"`
	Attr5Type     uint8               `json:"attr5_type,omitempty"`
	Attr5Value    int16               `json:"attr5_value,omitempty"`
	Attr6Type     uint8               `json:"attr6_type,omitempty"`
	Attr6Value    int16               `json:"attr6_value,omitempty"`
}

// CharacterEquipmentItemRow mirrors equipped item state frozen by the
// 0003_character_item_state migration, including optional additive 0024 instance
// sockets and additive 0027 instance attributes. HasSockets=false / omitted
// means nil instance sockets (template fallback); HasSockets=true including
// all-zero is authoritative. HasAttributes=false / omitted means nil instance
// attributes (template fallback); HasAttributes=true including all-zero /
// type-zero is authoritative.
type CharacterEquipmentItemRow struct {
	ID            uint64 `json:"id"`
	CharacterID   uint32 `json:"character_id"`
	EquipSlot     string `json:"equip_slot"`
	Vnum          uint32 `json:"vnum"`
	Count         uint16 `json:"count"`
	Locked        bool   `json:"locked,omitempty"`
	HasSockets    bool   `json:"has_sockets,omitempty"`
	Socket0       int32  `json:"socket0,omitempty"`
	Socket1       int32  `json:"socket1,omitempty"`
	Socket2       int32  `json:"socket2,omitempty"`
	HasAttributes bool   `json:"has_attributes,omitempty"`
	Attr0Type     uint8  `json:"attr0_type,omitempty"`
	Attr0Value    int16  `json:"attr0_value,omitempty"`
	Attr1Type     uint8  `json:"attr1_type,omitempty"`
	Attr1Value    int16  `json:"attr1_value,omitempty"`
	Attr2Type     uint8  `json:"attr2_type,omitempty"`
	Attr2Value    int16  `json:"attr2_value,omitempty"`
	Attr3Type     uint8  `json:"attr3_type,omitempty"`
	Attr3Value    int16  `json:"attr3_value,omitempty"`
	Attr4Type     uint8  `json:"attr4_type,omitempty"`
	Attr4Value    int16  `json:"attr4_value,omitempty"`
	Attr5Type     uint8  `json:"attr5_type,omitempty"`
	Attr5Value    int16  `json:"attr5_value,omitempty"`
	Attr6Type     uint8  `json:"attr6_type,omitempty"`
	Attr6Value    int16  `json:"attr6_value,omitempty"`
}

// CharacterQuickslotRow mirrors persisted quickslot state frozen by the
// 0003_character_item_state migration.
type CharacterQuickslotRow struct {
	CharacterID uint32 `json:"character_id"`
	Position    uint8  `json:"position"`
	Type        uint8  `json:"type"`
	Slot        uint8  `json:"slot"`
}

// ExportCharacterItemState validates bootstrap account snapshots and returns
// rows ordered exactly as a future backfill/import tool should process them:
// accounts by normalized login, characters by select-screen slot, carried items
// by inventory slot, equipped items by stable equipment-slot order, and
// quickslots by position. All validation fails closed against the 0002 roster
// and 0003 item-state migration constraints so malformed bootstrap JSON cannot
// be silently coerced into a future database import.
func ExportCharacterItemState(accounts []Account) (CharacterItemStateExport, error) {
	if _, err := ExportAccountCharacterRoster(accounts); err != nil {
		return CharacterItemStateExport{}, err
	}

	ordered := append([]Account(nil), accounts...)
	sort.Slice(ordered, func(i, j int) bool {
		left := strings.ToLower(ordered[i].Login)
		right := strings.ToLower(ordered[j].Login)
		if left != right {
			return left < right
		}
		return ordered[i].Login < ordered[j].Login
	})

	export := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}
	seenItemIDs := make(map[uint64]string)

	for _, account := range ordered {
		for slot, character := range account.Characters {
			if character.IsEmptySlot() {
				continue
			}
			if slot < 0 || slot >= accountCharacterRosterPlayerSlots {
				return CharacterItemStateExport{}, fmt.Errorf("%w: account %q character slot %d outside item-state roster", ErrInvalidAccount, account.Login, slot)
			}

			inventoryItems := append([]inventory.ItemInstance(nil), character.Inventory...)
			sort.Slice(inventoryItems, func(i, j int) bool {
				if inventoryItems[i].Slot != inventoryItems[j].Slot {
					return inventoryItems[i].Slot < inventoryItems[j].Slot
				}
				return inventoryItems[i].ID < inventoryItems[j].ID
			})
			for _, item := range inventoryItems {
				if err := validateCharacterItemStateItem(character, item, seenItemIDs, "inventory"); err != nil {
					return CharacterItemStateExport{}, err
				}
				seenItemIDs[item.ID] = fmt.Sprintf("character %q inventory slot %d", character.Name, item.Slot)
				hasSockets, socket0, socket1, socket2 := instanceSocketsExportFields(item)
				hasAttributes, attrs := instanceAttributesExportFields(item)
				export.InventoryItems = append(export.InventoryItems, CharacterInventoryItemRow{
					ID:            item.ID,
					CharacterID:   character.ID,
					Slot:          item.Slot,
					Vnum:          item.Vnum,
					Count:         item.Count,
					Locked:        item.Locked,
					HasSockets:    hasSockets,
					Socket0:       socket0,
					Socket1:       socket1,
					Socket2:       socket2,
					HasAttributes: hasAttributes,
					Attr0Type:     attrs[0].Type,
					Attr0Value:    attrs[0].Value,
					Attr1Type:     attrs[1].Type,
					Attr1Value:    attrs[1].Value,
					Attr2Type:     attrs[2].Type,
					Attr2Value:    attrs[2].Value,
					Attr3Type:     attrs[3].Type,
					Attr3Value:    attrs[3].Value,
					Attr4Type:     attrs[4].Type,
					Attr4Value:    attrs[4].Value,
					Attr5Type:     attrs[5].Type,
					Attr5Value:    attrs[5].Value,
					Attr6Type:     attrs[6].Type,
					Attr6Value:    attrs[6].Value,
				})
			}

			equipmentItems := append([]inventory.ItemInstance(nil), character.Equipment...)
			sort.Slice(equipmentItems, func(i, j int) bool {
				leftRank := equipmentSlotExportRank(equipmentItems[i].EquipSlot)
				rightRank := equipmentSlotExportRank(equipmentItems[j].EquipSlot)
				if leftRank != rightRank {
					return leftRank < rightRank
				}
				if equipmentItems[i].EquipSlot != equipmentItems[j].EquipSlot {
					return equipmentItems[i].EquipSlot < equipmentItems[j].EquipSlot
				}
				return equipmentItems[i].ID < equipmentItems[j].ID
			})
			for _, item := range equipmentItems {
				if err := validateCharacterItemStateItem(character, item, seenItemIDs, "equipment"); err != nil {
					return CharacterItemStateExport{}, err
				}
				seenItemIDs[item.ID] = fmt.Sprintf("character %q equipment slot %s", character.Name, item.EquipSlot.String())
				hasSockets, socket0, socket1, socket2 := instanceSocketsExportFields(item)
				hasAttributes, attrs := instanceAttributesExportFields(item)
				export.EquipmentItems = append(export.EquipmentItems, CharacterEquipmentItemRow{
					ID:            item.ID,
					CharacterID:   character.ID,
					EquipSlot:     item.EquipSlot.String(),
					Vnum:          item.Vnum,
					Count:         item.Count,
					Locked:        item.Locked,
					HasSockets:    hasSockets,
					Socket0:       socket0,
					Socket1:       socket1,
					Socket2:       socket2,
					HasAttributes: hasAttributes,
					Attr0Type:     attrs[0].Type,
					Attr0Value:    attrs[0].Value,
					Attr1Type:     attrs[1].Type,
					Attr1Value:    attrs[1].Value,
					Attr2Type:     attrs[2].Type,
					Attr2Value:    attrs[2].Value,
					Attr3Type:     attrs[3].Type,
					Attr3Value:    attrs[3].Value,
					Attr4Type:     attrs[4].Type,
					Attr4Value:    attrs[4].Value,
					Attr5Type:     attrs[5].Type,
					Attr5Value:    attrs[5].Value,
					Attr6Type:     attrs[6].Type,
					Attr6Value:    attrs[6].Value,
				})
			}

			quickslots := append([]loginticket.Quickslot(nil), character.Quickslots...)
			sort.Slice(quickslots, func(i, j int) bool {
				if quickslots[i].Position != quickslots[j].Position {
					return quickslots[i].Position < quickslots[j].Position
				}
				if quickslots[i].Type != quickslots[j].Type {
					return quickslots[i].Type < quickslots[j].Type
				}
				return quickslots[i].Slot < quickslots[j].Slot
			})
			for _, quickslot := range quickslots {
				if !validQuickslotTuple(quickslot) {
					return CharacterItemStateExport{}, fmt.Errorf("%w: character %q quickslot position %d has invalid type %d slot %d", ErrInvalidAccount, character.Name, quickslot.Position, quickslot.Type, quickslot.Slot)
				}
				export.Quickslots = append(export.Quickslots, CharacterQuickslotRow{
					CharacterID: character.ID,
					Position:    quickslot.Position,
					Type:        quickslot.Type,
					Slot:        quickslot.Slot,
				})
			}
		}
	}

	return export, nil
}

// ExportCharacterItemState validates and projects the committed file-store
// snapshots onto the 0003 character item-state migration shape. It reads the
// same committed snapshot set as List and applies no mutations.
func (s *FileStore) ExportCharacterItemState() (CharacterItemStateExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterItemStateExport{}, err
	}
	return ExportCharacterItemState(accounts)
}

func validateCharacterItemStateItem(character loginticket.Character, item inventory.ItemInstance, seenIDs map[uint64]string, context string) error {
	if item.ID > maxSignedBigInt {
		return fmt.Errorf("%w: character %q %s item id %d exceeds signed BIGINT", ErrInvalidAccount, character.Name, context, item.ID)
	}
	if previous, ok := seenIDs[item.ID]; ok {
		return fmt.Errorf("%w: item instance id %d appears in %s and character %q %s", ErrInvalidAccount, item.ID, previous, character.Name, context)
	}
	if err := item.Validate(); err != nil {
		return fmt.Errorf("%w: character %q %s item %d: %v", ErrInvalidAccount, character.Name, context, item.ID, err)
	}
	switch context {
	case "inventory":
		if item.Equipped || item.EquipSlot != inventory.EquipmentSlotNone {
			return fmt.Errorf("%w: character %q inventory item %d carries equipment state", ErrInvalidAccount, character.Name, item.ID)
		}
		if item.Slot >= inventory.CarriedInventorySlotCount {
			return fmt.Errorf("%w: character %q inventory item %d slot %d outside migration inventory", ErrInvalidAccount, character.Name, item.ID, item.Slot)
		}
	case "equipment":
		if !item.Equipped || !item.EquipSlot.Valid() {
			return fmt.Errorf("%w: character %q equipment item %d has invalid equipment state", ErrInvalidAccount, character.Name, item.ID)
		}
	default:
		return fmt.Errorf("%w: character %q item %d has unknown item-state context %q", ErrInvalidAccount, character.Name, item.ID, context)
	}
	return nil
}

func equipmentSlotExportRank(slot inventory.EquipmentSlot) int {
	for i, candidate := range inventory.AllEquipmentSlots() {
		if candidate == slot {
			return i
		}
	}
	return len(inventory.AllEquipmentSlots()) + int(slot)
}

func instanceSocketsExportFields(item inventory.ItemInstance) (hasSockets bool, socket0, socket1, socket2 int32) {
	if !item.HasSockets() {
		return false, 0, 0, 0
	}
	values := *item.Sockets
	return true, values[0], values[1], values[2]
}

func instanceAttributesExportFields(item inventory.ItemInstance) (hasAttributes bool, values inventory.AttributeValues) {
	if !item.HasAttributes() {
		return false, inventory.AttributeValues{}
	}
	return true, *item.Attributes
}
