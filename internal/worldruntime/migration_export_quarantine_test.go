package worldruntime

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateBootstrapGroundItemStateExportAcceptsCanonicalExport(t *testing.T) {
	export := sampleBootstrapGroundItemStateExport()

	summary, err := ValidateBootstrapGroundItemStateExport(export)
	if err != nil {
		t.Fatalf("validate bootstrap ground-item-state export: %v", err)
	}
	want := BootstrapGroundItemStateQuarantineSummary{
		GroundItemCount: 2,
		ItemShapedCount: 1,
		GoldShapedCount: 1,
		VIDs:            []uint32{0x0700002c, 0x0700002d},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateBootstrapGroundItemStateExportAcceptsEmptyCollection(t *testing.T) {
	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems:      []BootstrapGroundItemStateRow{},
	}

	summary, err := ValidateBootstrapGroundItemStateExport(export)
	if err != nil {
		t.Fatalf("validate empty bootstrap ground-item-state export: %v", err)
	}
	want := BootstrapGroundItemStateQuarantineSummary{
		GroundItemCount: 0,
		ItemShapedCount: 0,
		GoldShapedCount: 0,
		VIDs:            []uint32{},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateBootstrapGroundItemStateExportRejectsWrongMigrationBoundary(t *testing.T) {
	export := sampleBootstrapGroundItemStateExport()
	export.MigrationVersion = 9
	if _, err := ValidateBootstrapGroundItemStateExport(export); !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
		t.Fatalf("expected invalid export error, got %v", err)
	}

	export = sampleBootstrapGroundItemStateExport()
	export.MigrationName = "character_point_state"
	if _, err := ValidateBootstrapGroundItemStateExport(export); !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
		t.Fatalf("expected invalid export error, got %v", err)
	}
}

func TestValidateBootstrapGroundItemStateExportRejectsMalformedRows(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*BootstrapGroundItemStateExport)
	}{
		{
			name: "nil ground items",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems = nil
			},
		},
		{
			name: "zero vid",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[0].VID = 0
			},
		},
		{
			name: "duplicate vid",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems = append(export.GroundItems, export.GroundItems[0])
			},
		},
		{
			name: "both item and gold",
			mutate: func(export *BootstrapGroundItemStateExport) {
				count := uint16(2)
				gold := uint32(250)
				export.GroundItems[0].ItemCount = &count
				export.GroundItems[0].GoldAmount = &gold
			},
		},
		{
			name: "neither item nor gold",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[0].ItemCount = nil
				export.GroundItems[0].GoldAmount = nil
			},
		},
		{
			name: "gold with non-gold vnum",
			mutate: func(export *BootstrapGroundItemStateExport) {
				gold := uint32(250)
				export.GroundItems[0].ItemCount = nil
				export.GroundItems[0].GoldAmount = &gold
				export.GroundItems[0].Vnum = 3001
			},
		},
		{
			name: "empty owner login",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[0].OwnerLogin = ""
			},
		},
		{
			name: "zero map index",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[0].MapIndex = 0
			},
		},
		{
			name: "non-zero sockets without has_sockets",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[0].HasSockets = false
				export.GroundItems[0].Socket0 = 1
			},
		},
		{
			name: "gold-shaped with has_sockets",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[1].HasSockets = true
			},
		},
		{
			name: "gold-shaped with sockets",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[1].Socket0 = 1
			},
		},
		{
			name: "non-zero attributes without has_attributes",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[0].HasAttributes = false
				export.GroundItems[0].Attr0Type = 1
			},
		},
		{
			name: "gold-shaped with has_attributes",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[1].HasAttributes = true
			},
		},
		{
			name: "gold-shaped with attributes",
			mutate: func(export *BootstrapGroundItemStateExport) {
				export.GroundItems[1].Attr0Value = 1
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			export := sampleBootstrapGroundItemStateExport()
			tc.mutate(&export)
			if _, err := ValidateBootstrapGroundItemStateExport(export); !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
				t.Fatalf("expected %v, got %v", ErrInvalidBootstrapGroundItemStateExport, err)
			}
		})
	}
}

func TestQuarantineBootstrapGroundItemStateExportCanonicalizesRowOrder(t *testing.T) {
	export := sampleBootstrapGroundItemStateExport()
	export.GroundItems = []BootstrapGroundItemStateRow{
		export.GroundItems[1],
		export.GroundItems[0],
	}

	quarantined, summary, err := QuarantineBootstrapGroundItemStateExport(export)
	if err != nil {
		t.Fatalf("quarantine bootstrap ground-item-state export: %v", err)
	}
	wantExport := sampleBootstrapGroundItemStateExport()
	if !reflect.DeepEqual(quarantined, wantExport) {
		t.Fatalf("unexpected canonical quarantine export:\n got: %#v\nwant: %#v", quarantined, wantExport)
	}
	wantSummary := BootstrapGroundItemStateQuarantineSummary{
		GroundItemCount: 2,
		ItemShapedCount: 1,
		GoldShapedCount: 1,
		VIDs:            []uint32{0x0700002c, 0x0700002d},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}
}

func sampleBootstrapGroundItemStateExport() BootstrapGroundItemStateExport {
	count := uint16(2)
	gold := uint32(250)
	return BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		VIDs:             []uint32{0x0700002c, 0x0700002d},
		GroundItems: []BootstrapGroundItemStateRow{
			{VID: 0x0700002c, Vnum: 3001, ItemCount: &count, OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, OwnerName: "GroundItemOwner", MapIndex: 1, X: 1100, Y: 2100, Z: 2, PickupRange: 450, HasSockets: true, Socket0: 1, Socket1: 2, Socket2: 3, HasAttributes: true, Attr0Type: 1, Attr0Value: 10, Attr6Type: 7, Attr6Value: -3},
			{VID: 0x0700002d, Vnum: 1, GoldAmount: &gold, OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, OwnerName: "GroundGoldOwner", MapIndex: 42, X: 1200, Y: 2200, Z: 3, PickupRange: 750},
		},
	}
}
