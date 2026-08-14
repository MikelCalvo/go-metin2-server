package itemstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestExportItemTemplateStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	snapshot := Snapshot{Templates: []Template{
		{
			Vnum:              27001,
			Name:              "Small Red Potion",
			Stackable:         true,
			MaxCount:          200,
			ShopBuyPrice:      50,
			ShopSellPrice:     13,
			Highlight:         true,
			Unique:            true,
			AntiSell:          true,
			AntiGet:           true,
			AntiSafebox:       true,
			PickupRange:       750,
			Sockets:           SocketValues{1, 2, 3},
			Attributes:        AttributeValues{{Type: 1, Value: 10}},
			UseEffect:         &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, ConsumeCount: 2, Message: "Recovered HP", InfoMessage: "You feel better.", SpecialEffectType: 3},
			UseRejectText:     "You cannot use this yet.",
			BuyRejectText:     "The merchant will not sell this.",
			DropRejectText:    "You cannot drop this.",
			PickupRejectText:  "You cannot pick this up.",
			SellRejectText:    "The merchant refuses this.",
			SafeboxRejectText: "You cannot store this.",
		},
		{
			Vnum:              11200,
			Name:              "Wooden Sword",
			Stackable:         false,
			MaxCount:          1,
			Refineable:        true,
			RefineInfo:        &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75, Materials: []RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}}},
			Save:              true,
			Irremovable:       true,
			AntiMale:          true,
			AppearanceVnum:    11201,
			EquipSlot:         "weapon",
			EquipEffect:       &PointEffect{PointType: 1, PointIndex: 0, PointDelta: 4},
			EquipRejectText:   "You cannot wield this.",
			UnequipRejectText: "You cannot remove this.",
		},
	}}

	export, err := ExportItemTemplateState(snapshot)
	if err != nil {
		t.Fatalf("export item template state: %v", err)
	}

	if export.MigrationVersion != ItemTemplateStateMigrationVersion || export.MigrationName != ItemTemplateStateMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	if export.MigrationVersion != 7 || export.MigrationName != "item_template_refine_information" {
		t.Fatalf("expected refine-information export boundary, got version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	wantTemplates := []ItemTemplateRow{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, Refineable: true, Save: true, Irremovable: true, AppearanceVnum: 11201, AntiMale: true, EquipSlot: "weapon", EquipRejectText: "You cannot wield this.", UnequipRejectText: "You cannot remove this."},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50, ShopSellPrice: 13, Highlight: true, Unique: true, AntiSell: true, AntiGet: true, AntiSafebox: true, PickupRange: 750, UseRejectText: "You cannot use this yet.", BuyRejectText: "The merchant will not sell this.", DropRejectText: "You cannot drop this.", PickupRejectText: "You cannot pick this up.", SellRejectText: "The merchant refuses this.", SafeboxRejectText: "You cannot store this."},
	}
	if !reflect.DeepEqual(export.Templates, wantTemplates) {
		t.Fatalf("unexpected item-template rows:\n got: %#v\nwant: %#v", export.Templates, wantTemplates)
	}
	wantRefineInfos := []ItemTemplateRefineInfoRow{{Vnum: 11200, ResultVnum: 11201, Cost: 2500, Probability: 75}}
	if !reflect.DeepEqual(export.RefineInfos, wantRefineInfos) {
		t.Fatalf("unexpected item-template refine-info rows:\n got: %#v\nwant: %#v", export.RefineInfos, wantRefineInfos)
	}
	wantRefineMaterials := []ItemTemplateRefineMaterialRow{{Vnum: 11200, Position: 0, MaterialVnum: 27001, Count: 2}, {Vnum: 11200, Position: 1, MaterialVnum: 27002, Count: 3}}
	if !reflect.DeepEqual(export.RefineMaterials, wantRefineMaterials) {
		t.Fatalf("unexpected item-template refine-material rows:\n got: %#v\nwant: %#v", export.RefineMaterials, wantRefineMaterials)
	}
	wantSockets := []ItemTemplateSocketRow{
		{Vnum: 27001, Position: 0, Value: 1},
		{Vnum: 27001, Position: 1, Value: 2},
		{Vnum: 27001, Position: 2, Value: 3},
	}
	if !reflect.DeepEqual(export.Sockets, wantSockets) {
		t.Fatalf("unexpected item-template socket rows:\n got: %#v\nwant: %#v", export.Sockets, wantSockets)
	}
	wantAttributes := []ItemTemplateAttributeRow{{Vnum: 27001, Position: 0, Type: 1, Value: 10}}
	if !reflect.DeepEqual(export.Attributes, wantAttributes) {
		t.Fatalf("unexpected item-template attribute rows:\n got: %#v\nwant: %#v", export.Attributes, wantAttributes)
	}
	wantUseEffects := []ItemTemplateUseEffectRow{{Vnum: 27001, PointType: 7, PointIndex: 1, PointDelta: 25, ConsumeCount: 2, Message: "Recovered HP", InfoMessage: "You feel better.", SpecialEffectType: 3}}
	if !reflect.DeepEqual(export.UseEffects, wantUseEffects) {
		t.Fatalf("unexpected item-template use-effect rows:\n got: %#v\nwant: %#v", export.UseEffects, wantUseEffects)
	}
	wantEquipEffects := []ItemTemplateEquipEffectRow{{Vnum: 11200, PointType: 1, PointIndex: 0, PointDelta: 4}}
	if !reflect.DeepEqual(export.EquipEffects, wantEquipEffects) {
		t.Fatalf("unexpected item-template equip-effect rows:\n got: %#v\nwant: %#v", export.EquipEffects, wantEquipEffects)
	}

	rawJSON, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	if !bytes.Contains(rawJSON, []byte(`"unique_item":true`)) || bytes.Contains(rawJSON, []byte(`"unique":true`)) {
		t.Fatalf("expected exported JSON to use migration column name unique_item, got %s", rawJSON)
	}

	exportAgain, err := ExportItemTemplateState(snapshot)
	if err != nil {
		t.Fatalf("export item template state again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic item-template export:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportItemTemplateStateRejectsRowsThatCannotTargetMigrationSchema(t *testing.T) {
	oversizedBuyPrice := validTemplateStateTemplate(27001, "Too Expensive")
	oversizedBuyPrice.ShopBuyPrice = 1 << 32

	oversizedSellPrice := validTemplateStateTemplate(27002, "Too Valuable")
	oversizedSellPrice.ShopSellPrice = 1 << 31

	stackableEquipment := validTemplateStateTemplate(11200, "Bad Sword")
	stackableEquipment.Stackable = true
	stackableEquipment.MaxCount = 200
	stackableEquipment.EquipSlot = "weapon"

	badAttributePosition := validTemplateStateTemplate(27004, "Bad Attribute")
	badAttributePosition.Attributes = AttributeValues{{Type: 0, Value: 5}}

	cases := []struct {
		name     string
		snapshot Snapshot
	}{
		{name: "duplicate vnum", snapshot: Snapshot{Templates: []Template{validTemplateStateTemplate(27001, "Potion"), validTemplateStateTemplate(27001, "Potion Again")}}},
		{name: "oversized buy price", snapshot: Snapshot{Templates: []Template{oversizedBuyPrice}}},
		{name: "oversized sell price", snapshot: Snapshot{Templates: []Template{oversizedSellPrice}}},
		{name: "stackable equipment", snapshot: Snapshot{Templates: []Template{stackableEquipment}}},
		{name: "invalid attribute placeholder", snapshot: Snapshot{Templates: []Template{badAttributePosition}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExportItemTemplateState(tc.snapshot)
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot, got %v", err)
			}
		})
	}
}

func TestFileStoreExportItemTemplateStateReadsCommittedSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "item-templates.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Templates: []Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50},
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, EquipSlot: "weapon"},
	}}); err != nil {
		t.Fatalf("save item-template snapshot: %v", err)
	}

	export, err := store.ExportItemTemplateState()
	if err != nil {
		t.Fatalf("file-store item-template export: %v", err)
	}
	if export.MigrationVersion != ItemTemplateStateMigrationVersion || export.MigrationName != ItemTemplateStateMigrationName {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.Templates) != 2 || export.Templates[0].Vnum != 11200 || export.Templates[1].Vnum != 27001 {
		t.Fatalf("unexpected file-store template rows: %#v", export.Templates)
	}
}

func TestFileStoreExportItemTemplateStateTreatsMissingSnapshotAsEmptyExport(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "item-templates.json"))

	export, err := store.ExportItemTemplateState()
	if err != nil {
		t.Fatalf("export missing item-template snapshot: %v", err)
	}
	if export.MigrationVersion != ItemTemplateStateMigrationVersion || export.MigrationName != ItemTemplateStateMigrationName {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.Templates) != 0 || len(export.RefineInfos) != 0 || len(export.RefineMaterials) != 0 || len(export.Sockets) != 0 || len(export.Attributes) != 0 || len(export.UseEffects) != 0 || len(export.EquipEffects) != 0 {
		t.Fatalf("expected empty item-template export for missing snapshot, got %#v", export)
	}
}

func validTemplateStateTemplate(vnum uint32, name string) Template {
	return Template{Vnum: vnum, Name: name, Stackable: true, MaxCount: 200}
}

func TestExportItemTemplateStateIncludesAllEquipmentSlotsAcceptedByMigration(t *testing.T) {
	templates := make([]Template, 0, len(inventory.AllEquipmentSlots()))
	for i, slot := range inventory.AllEquipmentSlots() {
		templates = append(templates, Template{Vnum: uint32(11200 + i), Name: "EquipTemplate", Stackable: false, MaxCount: 1, EquipSlot: slot.String()})
	}

	export, err := ExportItemTemplateState(Snapshot{Templates: templates})
	if err != nil {
		t.Fatalf("export equipment-slot templates: %v", err)
	}
	if len(export.Templates) != len(inventory.AllEquipmentSlots()) {
		t.Fatalf("expected one row per equipment slot, got %#v", export.Templates)
	}
	for i, slot := range inventory.AllEquipmentSlots() {
		if export.Templates[i].EquipSlot != slot.String() {
			t.Fatalf("expected export row %d equip slot %q, got %#v", i, slot.String(), export.Templates[i])
		}
	}
}
