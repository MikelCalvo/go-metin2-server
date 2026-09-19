package safeboxstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestMallStorePathBesideSafeboxUsesSiblingDirectory(t *testing.T) {
	got := MallStorePathBesideSafebox(filepath.Join(string(filepath.Separator), "tmp", "safebox", "safebox.json"))
	want := filepath.Join(string(filepath.Separator), "tmp", "safebox-mall", MallSnapshotFilename)
	if got != want {
		t.Fatalf("unexpected sibling mall path:\n got: %q\nwant: %q", got, want)
	}
	got = MallStorePathBesideSafebox(filepath.Join(string(filepath.Separator), "tmp", "go-metin2-safebox-1-2", "safebox.json"))
	want = filepath.Join(string(filepath.Separator), "tmp", "go-metin2-safebox-1-2-mall", MallSnapshotFilename)
	if got != want {
		t.Fatalf("unexpected hermetic sibling mall path:\n got: %q\nwant: %q", got, want)
	}
	got = MallStorePathBesideSafebox(filepath.Join(string(filepath.Separator), "tmp", "safebox.json"))
	want = filepath.Join(string(filepath.Separator), "tmp-mall", MallSnapshotFilename)
	if got != want {
		t.Fatalf("unexpected file-sibling mall path:\n got: %q\nwant: %q", got, want)
	}
	if filepath.Dir(got) == filepath.Dir(filepath.Join(string(filepath.Separator), "tmp", "safebox.json")) {
		t.Fatal("mall FileStore must not share the safebox store directory")
	}
}

func TestMallFileStoreRoundTripPersistsCellsAndItemIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mall", "mall.json")
	store := NewMallFileStore(path)
	input := MallSnapshot{Characters: []MallCharacterRow{{
		Login:       "owner-one",
		CharacterID: 42,
		Cells: []Cell{
			{Cell: 2, ID: 9002, Vnum: 27001, Count: 3},
			{Cell: 0, ID: 9001, Vnum: 27002, Count: 1, Locked: true},
		},
	}}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save mall: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load mall: %v", err)
	}
	want := MallSnapshot{Characters: []MallCharacterRow{{
		Login:       "owner-one",
		CharacterID: 42,
		Cells: []Cell{
			{Cell: 0, ID: 9001, Vnum: 27002, Count: 1, Locked: true},
			{Cell: 2, ID: 9002, Vnum: 27001, Count: 3},
		},
	}}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected loaded mall snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}
	cells := MallCharacterCells(loaded, "owner-one", 42)
	if cells[0].ID != 9001 || cells[2].ID != 9002 || len(cells) != 2 {
		t.Fatalf("unexpected MallCharacterCells projection: %#v", cells)
	}
	if other := MallCharacterCells(loaded, "owner-one", 43); len(other) != 0 {
		t.Fatalf("expected empty mall cells for a second character id, got %#v", other)
	}
}

func TestMallFileStoreRoundTripPersistsInstanceSocketsIncludingExplicitZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mall", "mall-sockets.json")
	store := NewMallFileStore(path)
	input := MallSnapshot{Characters: []MallCharacterRow{{
		Login:       "socket-owner",
		CharacterID: 11,
		Cells: []Cell{
			{Cell: 1, ID: 9001, Vnum: 72723, Count: 1, HasSockets: true, Socket0: 1, Socket1: 2, Socket2: 3},
			{Cell: 3, ID: 9002, Vnum: 72727, Count: 1, HasSockets: true},
		},
	}}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save mall sockets: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load mall sockets: %v", err)
	}
	if len(loaded.Characters) != 1 || len(loaded.Characters[0].Cells) != 2 {
		t.Fatalf("unexpected loaded mall sockets snapshot: %#v", loaded)
	}
	active := loaded.Characters[0].Cells[0]
	if !active.HasSockets || active.Socket0 != 1 || active.Socket1 != 2 || active.Socket2 != 3 {
		t.Fatalf("expected active sockets rematerialized, got %#v", active)
	}
	zero := loaded.Characters[0].Cells[1]
	if !zero.HasSockets || zero.Socket0 != 0 || zero.Socket1 != 0 || zero.Socket2 != 0 {
		t.Fatalf("expected explicit-zero sockets rematerialized, got %#v", zero)
	}
	cells := MallCharacterCells(loaded, "socket-owner", 11)
	activeItem := cells[1]
	if !activeItem.HasSockets() || *activeItem.Sockets != (inventory.SocketValues{1, 2, 3}) {
		t.Fatalf("expected MallCharacterCells active sockets, got %#v", activeItem)
	}
	zeroItem := cells[3]
	if !zeroItem.HasSockets() || *zeroItem.Sockets != (inventory.SocketValues{}) {
		t.Fatalf("expected MallCharacterCells explicit-zero sockets, got %#v", zeroItem)
	}
}

func TestMallFileStoreSaveIsDeterministicJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mall", "mall.json")
	store := NewMallFileStore(path)
	input := MallSnapshot{Characters: []MallCharacterRow{
		{Login: "zeta", CharacterID: 2, Cells: []Cell{{Cell: 1, ID: 2, Vnum: 10, Count: 1}}},
		{Login: "alpha", CharacterID: 1, Cells: []Cell{{Cell: 0, ID: 1, Vnum: 20, Count: 2}}},
	}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save mall: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mall snapshot: %v", err)
	}
	wantRaw := "{\n  \"characters\": [\n    {\n      \"login\": \"alpha\",\n      \"character_id\": 1,\n      \"cells\": [\n        {\n          \"cell\": 0,\n          \"id\": 1,\n          \"vnum\": 20,\n          \"count\": 2\n        }\n      ]\n    },\n    {\n      \"login\": \"zeta\",\n      \"character_id\": 2,\n      \"cells\": [\n        {\n          \"cell\": 1,\n          \"id\": 2,\n          \"vnum\": 10,\n          \"count\": 1\n        }\n      ]\n    }\n  ]\n}\n"
	if string(raw) != wantRaw {
		t.Fatalf("unexpected deterministic mall JSON:\n got: %s\nwant: %s", string(raw), wantRaw)
	}
}

func TestMallFileStoreLoadReturnsNotFoundForMissingSnapshot(t *testing.T) {
	store := NewMallFileStore(filepath.Join(t.TempDir(), "missing", "mall.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
	loaded, err := LoadMallOrEmpty(store)
	if err != nil {
		t.Fatalf("LoadMallOrEmpty missing: %v", err)
	}
	if len(loaded.Characters) != 0 {
		t.Fatalf("expected empty mall snapshot, got %#v", loaded)
	}
}

func TestReplaceMallCharacterCellsUpsertsAndRemovesRows(t *testing.T) {
	snapshot := MallSnapshot{}
	next, err := ReplaceMallCharacterCells(snapshot, "owner", 1, map[uint8]inventory.ItemInstance{
		0: {ID: 10, Vnum: 27001, Count: 2, Slot: 0},
	})
	if err != nil {
		t.Fatalf("replace mall cells: %v", err)
	}
	if len(next.Characters) != 1 || next.Characters[0].Cells[0].ID != 10 {
		t.Fatalf("unexpected mall upsert: %#v", next)
	}
	cleared, err := ReplaceMallCharacterCells(next, "owner", 1, nil)
	if err != nil {
		t.Fatalf("clear mall cells: %v", err)
	}
	if len(cleared.Characters) != 0 {
		t.Fatalf("expected cleared mall character row, got %#v", cleared)
	}
}

func TestMallFileStoreRejectsNonZeroSocketsWithoutHasSockets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mall", "mall-bad-sockets.json")
	store := NewMallFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir mall dir: %v", err)
	}
	body := `{"characters":[{"login":"owner","character_id":1,"cells":[{"cell":0,"id":1,"vnum":10,"count":1,"socket0":1}]}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write bad mall sockets snapshot: %v", err)
	}
	_, err := store.Load()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-zero sockets without has_sockets, got %v", err)
	}
	if !strings.Contains(err.Error(), "mall") {
		t.Fatalf("expected mall snapshot error context, got %v", err)
	}
}
