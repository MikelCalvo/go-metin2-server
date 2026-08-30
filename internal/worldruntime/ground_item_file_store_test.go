package worldruntime

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGroundItemFileStoreRoundTripPersistsTimersAndItemID(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()

	path := filepath.Join(t.TempDir(), "ground-items.json")
	store := NewGroundItemFileStore(path)

	itemCount := uint16(2)
	goldAmount := uint32(75)
	ownershipExpires := time.Date(2026, 8, 22, 12, 0, 30, 0, time.UTC)
	despawnItem := time.Date(2026, 8, 22, 12, 5, 0, 0, time.UTC)
	despawnGold := time.Date(2026, 8, 22, 12, 6, 0, 0, time.UTC)

	want := DurableGroundItemSnapshot{GroundItems: []DurableGroundItemRecord{
		{
			VID:                0x07000002,
			Vnum:               1,
			GoldAmount:         &goldAmount,
			OwnerLogin:         "gold-owner",
			OwnerCharacterID:   22,
			OwnerVID:           0x02000022,
			OwnerName:          "GoldHero",
			MapIndex:           1,
			X:                  1200,
			Y:                  2200,
			Z:                  0,
			PickupRange:        300,
			OwnershipExclusive: false,
			DespawnAt:          despawnGold,
		},
		{
			VID:                0x07000001,
			Vnum:               27001,
			ItemCount:          &itemCount,
			ItemID:             0x30010001,
			OwnerLogin:         "item-owner",
			OwnerCharacterID:   11,
			OwnerVID:           0x02000011,
			OwnerName:          "ItemHero",
			MapIndex:           1,
			X:                  1100,
			Y:                  2100,
			Z:                  0,
			PickupRange:        450,
			OwnershipExclusive: true,
			OwnershipExpiresAt: &ownershipExpires,
			DespawnAt:          despawnItem,
		},
	}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save durable ground items: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load durable ground items: %v", err)
	}
	if len(got.GroundItems) != 2 {
		t.Fatalf("expected 2 sorted ground items, got %#v", got.GroundItems)
	}
	if got.GroundItems[0].VID != 0x07000001 || got.GroundItems[0].ItemID != 0x30010001 {
		t.Fatalf("expected item-shaped row first with stable item id, got %#v", got.GroundItems[0])
	}
	if got.GroundItems[1].VID != 0x07000002 || got.GroundItems[1].GoldAmount == nil || *got.GroundItems[1].GoldAmount != 75 {
		t.Fatalf("expected gold-shaped row second, got %#v", got.GroundItems[1])
	}
	if got.GroundItems[0].OwnershipExpiresAt == nil || !got.GroundItems[0].OwnershipExpiresAt.Equal(ownershipExpires) {
		t.Fatalf("expected absolute ownership expiry, got %#v", got.GroundItems[0].OwnershipExpiresAt)
	}

	export, err := store.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("export 0010 projection: %v", err)
	}
	if export.MigrationVersion != BootstrapGroundItemStateMigrationVersion || len(export.GroundItems) != 2 {
		t.Fatalf("unexpected 0010 export: %#v", export)
	}
	raw, err := json.Marshal(export.GroundItems[0])
	if err != nil {
		t.Fatalf("marshal export row: %v", err)
	}
	if strings.Contains(string(raw), "item_id") || strings.Contains(string(raw), "ownership_exclusive") || strings.Contains(string(raw), "despawn_at") {
		t.Fatalf("0010 export must omit durable extras, got %s", raw)
	}
}

func TestGroundItemFileStoreRoundTripPersistsInstanceSocketsIncludingExplicitZero(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()

	path := filepath.Join(t.TempDir(), "ground-items-sockets.json")
	store := NewGroundItemFileStore(path)

	itemCount := uint16(1)
	zeroCount := uint16(1)
	ownershipExpires := time.Date(2026, 8, 29, 12, 0, 30, 0, time.UTC)
	despawnItem := time.Date(2026, 8, 29, 12, 5, 0, 0, time.UTC)
	despawnZero := time.Date(2026, 8, 29, 12, 6, 0, 0, time.UTC)

	want := DurableGroundItemSnapshot{GroundItems: []DurableGroundItemRecord{
		{
			VID:                0x07000011,
			Vnum:               72723,
			ItemCount:          &itemCount,
			ItemID:             0x30010011,
			HasSockets:         true,
			Socket0:            1,
			Socket1:            2,
			Socket2:            3,
			OwnerLogin:         "socket-owner",
			OwnerCharacterID:   11,
			OwnerVID:           0x02000011,
			OwnerName:          "SocketHero",
			MapIndex:           1,
			X:                  1100,
			Y:                  2100,
			PickupRange:        450,
			OwnershipExclusive: true,
			OwnershipExpiresAt: &ownershipExpires,
			DespawnAt:          despawnItem,
		},
		{
			VID:                0x07000012,
			Vnum:               72727,
			ItemCount:          &zeroCount,
			ItemID:             0x30010012,
			HasSockets:         true,
			OwnerLogin:         "socket-owner",
			OwnerCharacterID:   11,
			OwnerVID:           0x02000011,
			OwnerName:          "SocketHero",
			MapIndex:           1,
			X:                  1110,
			Y:                  2110,
			PickupRange:        450,
			OwnershipExclusive: true,
			OwnershipExpiresAt: &ownershipExpires,
			DespawnAt:          despawnZero,
		},
	}}

	if err := store.Save(want); err != nil {
		t.Fatalf("save durable ground items with sockets: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load durable ground items with sockets: %v", err)
	}
	if len(got.GroundItems) != 2 {
		t.Fatalf("expected 2 socketed ground items, got %#v", got.GroundItems)
	}
	active := got.GroundItems[0]
	if !active.HasSockets || active.Socket0 != 1 || active.Socket1 != 2 || active.Socket2 != 3 {
		t.Fatalf("expected active sockets rematerialized, got %#v", active)
	}
	zero := got.GroundItems[1]
	if !zero.HasSockets || zero.Socket0 != 0 || zero.Socket1 != 0 || zero.Socket2 != 0 {
		t.Fatalf("expected explicit-zero sockets rematerialized, got %#v", zero)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted ground items: %v", err)
	}
	if !strings.Contains(string(raw), `"has_sockets": true`) {
		t.Fatalf("expected has_sockets in durable JSON, got %s", raw)
	}
	if !strings.Contains(string(raw), `"socket0": 1`) {
		t.Fatalf("expected non-zero socket0 in durable JSON, got %s", raw)
	}

	export, err := store.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("export 0010 projection with sockets: %v", err)
	}
	if len(export.GroundItems) != 2 {
		t.Fatalf("expected 2 projected ground items, got %#v", export.GroundItems)
	}
	if !export.GroundItems[0].HasSockets || export.GroundItems[0].Socket0 != 1 || export.GroundItems[0].Socket1 != 2 || export.GroundItems[0].Socket2 != 3 {
		t.Fatalf("expected active sockets in 0010 projection, got %#v", export.GroundItems[0])
	}
	if !export.GroundItems[1].HasSockets || export.GroundItems[1].Socket0 != 0 || export.GroundItems[1].Socket1 != 0 || export.GroundItems[1].Socket2 != 0 {
		t.Fatalf("expected explicit-zero sockets in 0010 projection, got %#v", export.GroundItems[1])
	}

	projected := DurableGroundItemRecordsToSnapshots(got.GroundItems)
	if len(projected) != 2 || !projected[0].HasSockets || projected[0].Socket0 != 1 || projected[0].Socket1 != 2 || projected[0].Socket2 != 3 {
		t.Fatalf("DurableGroundItemRecordsToSnapshots lost active sockets: %#v", projected)
	}
	if !projected[1].HasSockets || projected[1].Socket0 != 0 || projected[1].Socket1 != 0 || projected[1].Socket2 != 0 {
		t.Fatalf("DurableGroundItemRecordsToSnapshots lost explicit-zero sockets: %#v", projected)
	}
}

func TestGroundItemFileStoreRejectsNonZeroSocketsWithoutHasSocketsAndGoldSockets(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()

	path := filepath.Join(t.TempDir(), "ground-items-bad-sockets.json")
	store := NewGroundItemFileStore(path)

	itemCount := uint16(1)
	goldAmount := uint32(10)
	despawn := time.Now().UTC().Add(time.Minute)

	badItem := DurableGroundItemSnapshot{GroundItems: []DurableGroundItemRecord{{
		VID: 1, Vnum: 27001, ItemCount: &itemCount, ItemID: 11,
		Socket0:    1,
		OwnerLogin: "x", OwnerCharacterID: 1, OwnerVID: 1, OwnerName: "X",
		MapIndex: 1, PickupRange: 300, DespawnAt: despawn,
	}}}
	if err := store.Save(badItem); !errors.Is(err, ErrInvalidGroundItemSnapshot) {
		t.Fatalf("expected invalid snapshot for non-zero sockets without has_sockets, got %v", err)
	}

	badGold := DurableGroundItemSnapshot{GroundItems: []DurableGroundItemRecord{{
		VID: 2, Vnum: 1, GoldAmount: &goldAmount, HasSockets: true,
		OwnerLogin: "y", OwnerCharacterID: 2, OwnerVID: 2, OwnerName: "Y",
		MapIndex: 1, PickupRange: 300, DespawnAt: despawn,
	}}}
	if err := store.Save(badGold); !errors.Is(err, ErrInvalidGroundItemSnapshot) {
		t.Fatalf("expected invalid snapshot for gold sockets, got %v", err)
	}
}

func TestGroundItemFileStoreRejectsInvalidRowsAndMissingSnapshot(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()

	path := filepath.Join(t.TempDir(), "ground-items.json")
	store := NewGroundItemFileStore(path)
	if _, err := store.Load(); !errors.Is(err, ErrGroundItemSnapshotNotFound) {
		t.Fatalf("expected missing snapshot, got %v", err)
	}

	itemCount := uint16(1)
	bad := DurableGroundItemSnapshot{GroundItems: []DurableGroundItemRecord{{
		VID: 1, Vnum: 27001, ItemCount: &itemCount, ItemID: 0, OwnerLogin: "x", OwnerCharacterID: 1, OwnerVID: 1, OwnerName: "X", MapIndex: 1, PickupRange: 300, DespawnAt: time.Now().UTC(),
	}}}
	if err := store.Save(bad); !errors.Is(err, ErrInvalidGroundItemSnapshot) {
		t.Fatalf("expected invalid snapshot without item_id, got %v", err)
	}
}

func TestFilterDurableGroundItemSnapshotForRestoreDropsDespawnedAndPublicizesExpired(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	itemCount := uint16(1)
	goldAmount := uint32(10)
	expired := now.Add(-time.Second)
	futureOwnership := now.Add(10 * time.Second)
	futureDespawn := now.Add(time.Minute)
	pastDespawn := now.Add(-time.Second)

	snapshot := DurableGroundItemSnapshot{GroundItems: []DurableGroundItemRecord{
		{VID: 1, Vnum: 27001, ItemCount: &itemCount, ItemID: 11, OwnerLogin: "a", OwnerCharacterID: 1, OwnerVID: 1, OwnerName: "A", MapIndex: 1, PickupRange: 300, OwnershipExclusive: true, OwnershipExpiresAt: &expired, DespawnAt: futureDespawn},
		{VID: 2, Vnum: 1, GoldAmount: &goldAmount, OwnerLogin: "b", OwnerCharacterID: 2, OwnerVID: 2, OwnerName: "B", MapIndex: 1, PickupRange: 300, OwnershipExclusive: true, OwnershipExpiresAt: &futureOwnership, DespawnAt: futureDespawn},
		{VID: 3, Vnum: 27002, ItemCount: &itemCount, ItemID: 33, OwnerLogin: "c", OwnerCharacterID: 3, OwnerVID: 3, OwnerName: "C", MapIndex: 1, PickupRange: 300, OwnershipExclusive: false, DespawnAt: pastDespawn},
	}}
	filtered := FilterDurableGroundItemSnapshotForRestore(snapshot, now)
	if len(filtered.GroundItems) != 2 {
		t.Fatalf("expected despawned row dropped, got %#v", filtered.GroundItems)
	}
	if filtered.GroundItems[0].VID != 1 || filtered.GroundItems[0].OwnershipExclusive || filtered.GroundItems[0].OwnershipExpiresAt != nil {
		t.Fatalf("expected expired exclusive row to become public, got %#v", filtered.GroundItems[0])
	}
	if filtered.GroundItems[1].VID != 2 || !filtered.GroundItems[1].OwnershipExclusive {
		t.Fatalf("expected still-exclusive gold row, got %#v", filtered.GroundItems[1])
	}
}

func TestGroundItemFileStoreValidateReportsCrashTemps(t *testing.T) {
	defer DisableDurableGroundItemSyncForTest()()

	dir := t.TempDir()
	path := filepath.Join(dir, "ground-items.json")
	store := NewGroundItemFileStore(path)
	if err := os.WriteFile(filepath.Join(dir, ".ground-items-crash.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("seed crash temp: %v", err)
	}
	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate empty store with crash temp: %v", err)
	}
	if summary.CrashTempCount != 1 || len(summary.CrashTempFiles) != 1 {
		t.Fatalf("expected crash temp summary, got %#v", summary)
	}
}
