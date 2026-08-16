package worldruntime

import (
	"errors"
	"reflect"
	"testing"
)

func TestExportBootstrapGroundItemStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	count := uint16(2)
	gold := uint32(250)
	snapshots := []GroundItemSnapshot{
		{VID: 0x0700002d, Vnum: 1, GoldAmount: gold, OwnerName: "GroundGoldOwner", OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, PickupRange: 750, MapIndex: 42, X: 1200, Y: 2200, Z: 3},
		{VID: 0x0700002c, Vnum: 3001, Count: count, OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, PickupRange: 450, MapIndex: 1, X: 1100, Y: 2100, Z: 2},
	}

	export, err := ExportBootstrapGroundItemState(snapshots)
	if err != nil {
		t.Fatalf("export bootstrap ground-item state: %v", err)
	}
	if export.MigrationVersion != BootstrapGroundItemStateMigrationVersion || export.MigrationName != BootstrapGroundItemStateMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	want := []BootstrapGroundItemStateRow{
		{VID: 0x0700002c, Vnum: 3001, ItemCount: &count, OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, OwnerName: "GroundItemOwner", MapIndex: 1, X: 1100, Y: 2100, Z: 2, PickupRange: 450},
		{VID: 0x0700002d, Vnum: 1, GoldAmount: &gold, OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, OwnerName: "GroundGoldOwner", MapIndex: 42, X: 1200, Y: 2200, Z: 3, PickupRange: 750},
	}
	if !reflect.DeepEqual(export.GroundItems, want) {
		t.Fatalf("unexpected ground-item rows:\n got: %#v\nwant: %#v", export.GroundItems, want)
	}

	exportAgain, err := ExportBootstrapGroundItemState(snapshots)
	if err != nil {
		t.Fatalf("export bootstrap ground-item state again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic bootstrap ground-item export:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportBootstrapGroundItemStateRejectsRowsThatCannotTargetMigrationSchema(t *testing.T) {
	valid := GroundItemSnapshot{VID: 0x0700002c, Vnum: 3001, Count: 2, OwnerName: "GroundOwner", OwnerLogin: "ground-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, PickupRange: 300, MapIndex: 1, X: 1100, Y: 2100}
	cases := []struct {
		name     string
		snapshot GroundItemSnapshot
	}{
		{name: "zero vid", snapshot: withGroundVID(valid, 0)},
		{name: "zero vnum", snapshot: withGroundVnum(valid, 0)},
		{name: "duplicate vid", snapshot: valid},
		{name: "empty owner login", snapshot: withGroundOwnerLogin(valid, "")},
		{name: "blank owner login", snapshot: withGroundOwnerLogin(valid, " ground-owner")},
		{name: "invalid utf8 owner login", snapshot: withGroundOwnerLogin(valid, string([]byte{0xff, 0xfe}))},
		{name: "empty owner name", snapshot: withGroundOwnerName(valid, "")},
		{name: "owner name longer than fixed field", snapshot: withGroundOwnerName(valid, "ABCDEFGHIJKLMNOPQRSTUVWXY1")},
		{name: "zero owner character id", snapshot: withGroundOwnerCharacterID(valid, 0)},
		{name: "zero owner vid", snapshot: withGroundOwnerVID(valid, 0)},
		{name: "zero map index", snapshot: withGroundMapIndex(valid, 0)},
		{name: "zero pickup range", snapshot: withGroundPickupRange(valid, 0)},
		{name: "neither item nor gold", snapshot: withGroundCount(valid, 0)},
		{name: "item count exceeds visible carrier", snapshot: withGroundCount(valid, uint16(^uint8(0))+1)},
		{name: "both item and gold", snapshot: withGroundGold(valid, 250)},
		{name: "gold with non-gold vnum", snapshot: withGroundVnum(withGroundCount(withGroundGold(valid, 250), 0), 3001)},
		{name: "gold exceeds signed carrier", snapshot: withGroundGold(withGroundVnum(withGroundCount(valid, 0), 1), uint32(1<<31))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshots := []GroundItemSnapshot{tc.snapshot}
			if tc.name == "duplicate vid" {
				other := valid
				other.Vnum = 3002
				snapshots = []GroundItemSnapshot{valid, other}
			}
			_, err := ExportBootstrapGroundItemState(snapshots)
			if !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
				t.Fatalf("expected ErrInvalidBootstrapGroundItemStateExport, got %v", err)
			}
		})
	}
}

func withGroundVID(snapshot GroundItemSnapshot, value uint32) GroundItemSnapshot {
	snapshot.VID = value
	return snapshot
}

func withGroundVnum(snapshot GroundItemSnapshot, value uint32) GroundItemSnapshot {
	snapshot.Vnum = value
	return snapshot
}

func withGroundCount(snapshot GroundItemSnapshot, value uint16) GroundItemSnapshot {
	snapshot.Count = value
	return snapshot
}

func withGroundGold(snapshot GroundItemSnapshot, value uint32) GroundItemSnapshot {
	snapshot.GoldAmount = value
	return snapshot
}

func withGroundOwnerLogin(snapshot GroundItemSnapshot, value string) GroundItemSnapshot {
	snapshot.OwnerLogin = value
	return snapshot
}

func withGroundOwnerName(snapshot GroundItemSnapshot, value string) GroundItemSnapshot {
	snapshot.OwnerName = value
	return snapshot
}

func withGroundOwnerCharacterID(snapshot GroundItemSnapshot, value uint32) GroundItemSnapshot {
	snapshot.OwnerCharacterID = value
	return snapshot
}

func withGroundOwnerVID(snapshot GroundItemSnapshot, value uint32) GroundItemSnapshot {
	snapshot.OwnerVID = value
	return snapshot
}

func withGroundMapIndex(snapshot GroundItemSnapshot, value uint32) GroundItemSnapshot {
	snapshot.MapIndex = value
	return snapshot
}

func withGroundPickupRange(snapshot GroundItemSnapshot, value int64) GroundItemSnapshot {
	snapshot.PickupRange = value
	return snapshot
}
