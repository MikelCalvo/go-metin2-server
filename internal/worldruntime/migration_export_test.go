package worldruntime

import (
	"errors"
	"reflect"
	"testing"
)

func TestExportBootstrapGroundItemStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	count := uint16(2)
	explicitZeroCount := uint16(1)
	gold := uint32(250)
	snapshots := []GroundItemSnapshot{
		{VID: 0x0700002d, Vnum: 1, GoldAmount: gold, OwnerName: "GroundGoldOwner", OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, PickupRange: 750, MapIndex: 42, X: 1200, Y: 2200, Z: 3},
		{VID: 0x0700002c, Vnum: 3001, Count: count, OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, PickupRange: 450, MapIndex: 1, X: 1100, Y: 2100, Z: 2, HasSockets: true, Socket0: 1, Socket1: 2, Socket2: 3},
		{VID: 0x0700002e, Vnum: 3002, Count: explicitZeroCount, OwnerName: "GroundZeroOwner", OwnerLogin: "ground-zero-owner", OwnerCharacterID: 0x0103019e, OwnerVID: 0x0204019e, PickupRange: 300, MapIndex: 2, X: 1300, Y: 2300, Z: 1, HasSockets: true},
	}

	export, err := ExportBootstrapGroundItemState(snapshots)
	if err != nil {
		t.Fatalf("export bootstrap ground-item state: %v", err)
	}
	if export.MigrationVersion != BootstrapGroundItemStateMigrationVersion || export.MigrationName != BootstrapGroundItemStateMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	want := []BootstrapGroundItemStateRow{
		{VID: 0x0700002c, Vnum: 3001, ItemCount: &count, OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, OwnerName: "GroundItemOwner", MapIndex: 1, X: 1100, Y: 2100, Z: 2, PickupRange: 450, HasSockets: true, Socket0: 1, Socket1: 2, Socket2: 3},
		{VID: 0x0700002d, Vnum: 1, GoldAmount: &gold, OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, OwnerName: "GroundGoldOwner", MapIndex: 42, X: 1200, Y: 2200, Z: 3, PickupRange: 750},
		{VID: 0x0700002e, Vnum: 3002, ItemCount: &explicitZeroCount, OwnerLogin: "ground-zero-owner", OwnerCharacterID: 0x0103019e, OwnerVID: 0x0204019e, OwnerName: "GroundZeroOwner", MapIndex: 2, X: 1300, Y: 2300, Z: 1, PickupRange: 300, HasSockets: true},
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
		{name: "non-zero sockets without has_sockets", snapshot: withGroundSockets(valid, false, 1, 0, 0)},
		{name: "gold-shaped with has_sockets", snapshot: withGroundSockets(withGroundGold(withGroundVnum(withGroundCount(valid, 0), 1), 250), true, 0, 0, 0)},
		{name: "gold-shaped with sockets", snapshot: withGroundSockets(withGroundGold(withGroundVnum(withGroundCount(valid, 0), 1), 250), false, 1, 0, 0)},
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

func withGroundSockets(snapshot GroundItemSnapshot, hasSockets bool, socket0, socket1, socket2 int32) GroundItemSnapshot {
	snapshot.HasSockets = hasSockets
	snapshot.Socket0 = socket0
	snapshot.Socket1 = socket1
	snapshot.Socket2 = socket2
	return snapshot
}
