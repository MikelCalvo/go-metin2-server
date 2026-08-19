package itemstore

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateItemTemplateStateExportAcceptsCanonicalExport(t *testing.T) {
	export, err := ExportItemTemplateState(Snapshot{Templates: []Template{
		{
			Vnum:              27001,
			Name:              "Small Red Potion",
			Stackable:         true,
			MaxCount:          200,
			ShopBuyPrice:      50,
			Highlight:         true,
			AntiSafebox:       true,
			Sockets:           SocketValues{1, 0, 3},
			Attributes:        AttributeValues{{Type: 1, Value: 10}},
			UseEffect:         &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, ConsumeCount: 2, Message: "Recovered HP"},
			SafeboxRejectText: "You cannot store this.",
		},
		{
			Vnum:           11200,
			Name:           "Wooden Sword",
			Stackable:      false,
			MaxCount:       1,
			Refineable:     true,
			RefineInfo:     &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75, Materials: []RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}}},
			AppearanceVnum: 11201,
			EquipSlot:      "weapon",
			EquipEffect:    &PointEffect{PointType: 1, PointIndex: 0, PointDelta: 4},
		},
	}})
	if err != nil {
		t.Fatalf("export item template state: %v", err)
	}

	summary, err := ValidateItemTemplateStateExport(export)
	if err != nil {
		t.Fatalf("validate item template state export: %v", err)
	}
	want := ItemTemplateStateQuarantineSummary{
		TemplateCount:       2,
		SocketCount:         2,
		AttributeCount:      1,
		UseEffectCount:      1,
		EquipEffectCount:    1,
		RefineInfoCount:     1,
		RefineMaterialCount: 2,
		Vnums:               []uint32{11200, 27001},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateItemTemplateStateExportAcceptsEmptyCollections(t *testing.T) {
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

	summary, err := ValidateItemTemplateStateExport(export)
	if err != nil {
		t.Fatalf("validate empty item template state export: %v", err)
	}
	want := ItemTemplateStateQuarantineSummary{
		TemplateCount:       0,
		SocketCount:         0,
		AttributeCount:      0,
		UseEffectCount:      0,
		EquipEffectCount:    0,
		RefineInfoCount:     0,
		RefineMaterialCount: 0,
		Vnums:               []uint32{},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateItemTemplateStateExportRejectsWrongMigrationBoundary(t *testing.T) {
	export := ItemTemplateStateExport{
		MigrationVersion: 5,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates:        []ItemTemplateRow{},
		Sockets:          []ItemTemplateSocketRow{},
		Attributes:       []ItemTemplateAttributeRow{},
		UseEffects:       []ItemTemplateUseEffectRow{},
		EquipEffects:     []ItemTemplateEquipEffectRow{},
		RefineInfos:      []ItemTemplateRefineInfoRow{},
		RefineMaterials:  []ItemTemplateRefineMaterialRow{},
	}
	if _, err := ValidateItemTemplateStateExport(export); !errors.Is(err, ErrInvalidItemTemplateStateExport) {
		t.Fatalf("expected ErrInvalidItemTemplateStateExport, got %v", err)
	}
}

func TestValidateItemTemplateStateExportRejectsMalformedRows(t *testing.T) {
	base := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates: []ItemTemplateRow{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		},
		Sockets:         []ItemTemplateSocketRow{},
		Attributes:      []ItemTemplateAttributeRow{},
		UseEffects:      []ItemTemplateUseEffectRow{},
		EquipEffects:    []ItemTemplateEquipEffectRow{},
		RefineInfos:     []ItemTemplateRefineInfoRow{},
		RefineMaterials: []ItemTemplateRefineMaterialRow{},
	}

	cases := []struct {
		name   string
		mutate func(ItemTemplateStateExport) ItemTemplateStateExport
	}{
		{
			name: "nil templates",
			mutate: func(export ItemTemplateStateExport) ItemTemplateStateExport {
				export.Templates = nil
				return export
			},
		},
		{
			name: "duplicate template vnum",
			mutate: func(export ItemTemplateStateExport) ItemTemplateStateExport {
				export.Templates = append(export.Templates, ItemTemplateRow{Vnum: 27001, Name: "Duplicate", Stackable: true, MaxCount: 200})
				return export
			},
		},
		{
			name: "orphan socket",
			mutate: func(export ItemTemplateStateExport) ItemTemplateStateExport {
				export.Sockets = []ItemTemplateSocketRow{{Vnum: 99999, Position: 0, Value: 1}}
				return export
			},
		},
		{
			name: "zero socket value",
			mutate: func(export ItemTemplateStateExport) ItemTemplateStateExport {
				export.Sockets = []ItemTemplateSocketRow{{Vnum: 27001, Position: 0, Value: 0}}
				return export
			},
		},
		{
			name: "refine materials without refine info",
			mutate: func(export ItemTemplateStateExport) ItemTemplateStateExport {
				export.Templates[0].Refineable = true
				export.RefineMaterials = []ItemTemplateRefineMaterialRow{{Vnum: 27001, Position: 0, ItemVnum: 27002, Count: 1}}
				return export
			},
		},
		{
			name: "safebox reject without anti_safebox",
			mutate: func(export ItemTemplateStateExport) ItemTemplateStateExport {
				export.Templates[0].SafeboxRejectText = "blocked"
				return export
			},
		},
		{
			name: "non contiguous refine materials",
			mutate: func(export ItemTemplateStateExport) ItemTemplateStateExport {
				export.Templates[0].Refineable = true
				export.RefineInfos = []ItemTemplateRefineInfoRow{{Vnum: 27001, ResultVnum: 27002, Cost: 10, Probability: 50}}
				export.RefineMaterials = []ItemTemplateRefineMaterialRow{{Vnum: 27001, Position: 1, ItemVnum: 27002, Count: 1}}
				return export
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateItemTemplateStateExport(tc.mutate(base)); !errors.Is(err, ErrInvalidItemTemplateStateExport) {
				t.Fatalf("expected ErrInvalidItemTemplateStateExport, got %v", err)
			}
		})
	}
}

func TestQuarantineItemTemplateStateExportCanonicalizesRowOrder(t *testing.T) {
	export := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates: []ItemTemplateRow{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, AntiSafebox: true, SafeboxRejectText: "blocked"},
			{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, Refineable: true, EquipSlot: "weapon"},
		},
		Sockets: []ItemTemplateSocketRow{
			{Vnum: 27001, Position: 2, Value: 3},
			{Vnum: 27001, Position: 0, Value: 1},
		},
		Attributes: []ItemTemplateAttributeRow{
			{Vnum: 27001, Position: 1, Type: 2, Value: 5},
			{Vnum: 27001, Position: 0, Type: 1, Value: 10},
		},
		UseEffects: []ItemTemplateUseEffectRow{
			{Vnum: 27001, PointType: 7, PointIndex: 1, PointDelta: 25, ConsumeCount: 1, Message: "Recovered HP"},
		},
		EquipEffects: []ItemTemplateEquipEffectRow{
			{Vnum: 11200, PointType: 1, PointIndex: 0, PointDelta: 4},
		},
		RefineInfos: []ItemTemplateRefineInfoRow{
			{Vnum: 11200, ResultVnum: 11201, Cost: 2500, Probability: 75},
		},
		RefineMaterials: []ItemTemplateRefineMaterialRow{
			{Vnum: 11200, Position: 1, ItemVnum: 27002, Count: 3},
			{Vnum: 11200, Position: 0, ItemVnum: 27001, Count: 2},
		},
	}

	quarantined, summary, err := QuarantineItemTemplateStateExport(export)
	if err != nil {
		t.Fatalf("quarantine item template state export: %v", err)
	}
	wantSummary := ItemTemplateStateQuarantineSummary{
		TemplateCount:       2,
		SocketCount:         2,
		AttributeCount:      2,
		UseEffectCount:      1,
		EquipEffectCount:    1,
		RefineInfoCount:     1,
		RefineMaterialCount: 2,
		Vnums:               []uint32{11200, 27001},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}
	if quarantined.Templates[0].Vnum != 11200 || quarantined.Templates[1].Vnum != 27001 {
		t.Fatalf("expected templates sorted by vnum, got %#v", quarantined.Templates)
	}
	if quarantined.Sockets[0].Position != 0 || quarantined.Sockets[1].Position != 2 {
		t.Fatalf("expected sockets sorted by position, got %#v", quarantined.Sockets)
	}
	if quarantined.Attributes[0].Position != 0 || quarantined.Attributes[1].Position != 1 {
		t.Fatalf("expected attributes sorted by position, got %#v", quarantined.Attributes)
	}
	if quarantined.RefineMaterials[0].Position != 0 || quarantined.RefineMaterials[1].Position != 1 {
		t.Fatalf("expected refine materials sorted by position, got %#v", quarantined.RefineMaterials)
	}
}
