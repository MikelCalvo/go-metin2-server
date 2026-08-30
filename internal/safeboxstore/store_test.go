package safeboxstore

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestFileStoreRoundTripPersistsCellsAndItemIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	input := Snapshot{Characters: []CharacterRow{{
		Login:       "owner-one",
		CharacterID: 42,
		Cells: []Cell{
			{Cell: 2, ID: 9002, Vnum: 27001, Count: 3},
			{Cell: 0, ID: 9001, Vnum: 27002, Count: 1, Locked: true},
		},
	}}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save safebox: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load safebox: %v", err)
	}
	want := Snapshot{Characters: []CharacterRow{{
		Login:       "owner-one",
		CharacterID: 42,
		Cells: []Cell{
			{Cell: 0, ID: 9001, Vnum: 27002, Count: 1, Locked: true},
			{Cell: 2, ID: 9002, Vnum: 27001, Count: 3},
		},
	}}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected loaded safebox snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}
	cells := CharacterCells(loaded, "owner-one", 42)
	if cells[0].ID != 9001 || cells[2].ID != 9002 || len(cells) != 2 {
		t.Fatalf("unexpected CharacterCells projection: %#v", cells)
	}
}

func TestFileStoreRoundTripPersistsInstanceSocketsIncludingExplicitZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox-sockets.json")
	store := NewFileStore(path)
	input := Snapshot{Characters: []CharacterRow{{
		Login:       "socket-owner",
		CharacterID: 11,
		Cells: []Cell{
			{Cell: 1, ID: 9001, Vnum: 72723, Count: 1, HasSockets: true, Socket0: 1, Socket1: 2, Socket2: 3},
			{Cell: 3, ID: 9002, Vnum: 72727, Count: 1, HasSockets: true},
		},
	}}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save safebox sockets: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load safebox sockets: %v", err)
	}
	if len(loaded.Characters) != 1 || len(loaded.Characters[0].Cells) != 2 {
		t.Fatalf("unexpected loaded safebox sockets snapshot: %#v", loaded)
	}
	active := loaded.Characters[0].Cells[0]
	if !active.HasSockets || active.Socket0 != 1 || active.Socket1 != 2 || active.Socket2 != 3 {
		t.Fatalf("expected active sockets rematerialized, got %#v", active)
	}
	zero := loaded.Characters[0].Cells[1]
	if !zero.HasSockets || zero.Socket0 != 0 || zero.Socket1 != 0 || zero.Socket2 != 0 {
		t.Fatalf("expected explicit-zero sockets rematerialized, got %#v", zero)
	}
	cells := CharacterCells(loaded, "socket-owner", 11)
	activeItem := cells[1]
	if !activeItem.HasSockets() || *activeItem.Sockets != (inventory.SocketValues{1, 2, 3}) {
		t.Fatalf("expected CharacterCells active sockets, got %#v", activeItem)
	}
	zeroItem := cells[3]
	if !zeroItem.HasSockets() || *zeroItem.Sockets != (inventory.SocketValues{}) {
		t.Fatalf("expected CharacterCells explicit-zero sockets, got %#v", zeroItem)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read safebox sockets snapshot: %v", err)
	}
	if !strings.Contains(string(raw), `"has_sockets": true`) {
		t.Fatalf("expected has_sockets in durable JSON, got %s", raw)
	}
	if !strings.Contains(string(raw), `"socket0": 1`) {
		t.Fatalf("expected non-zero socket0 in durable JSON, got %s", raw)
	}
}

func TestFileStoreRejectsNonZeroSocketsWithoutHasSockets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox-bad-sockets.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	body := `{"characters":[{"login":"owner","character_id":1,"cells":[{"cell":0,"id":1,"vnum":10,"count":1,"socket0":1}]}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write bad sockets snapshot: %v", err)
	}
	_, err := store.Load()
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for non-zero sockets without has_sockets, got %v", err)
	}
	next, err := ReplaceCharacterCells(Snapshot{}, "owner", 1, map[uint8]inventory.ItemInstance{
		0: {ID: 1, Vnum: 10, Count: 1},
	})
	if err != nil {
		t.Fatalf("seed replace cells: %v", err)
	}
	// Force invalid cell through Save validation path.
	bad := next
	bad.Characters[0].Cells[0].Socket0 = 9
	if err := store.Save(bad); err == nil || !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected Save to reject non-zero sockets without has_sockets, got %v", err)
	}
}

func TestFileStoreSaveIsDeterministicJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	input := Snapshot{Characters: []CharacterRow{
		{Login: "zeta", CharacterID: 2, Cells: []Cell{{Cell: 1, ID: 2, Vnum: 10, Count: 1}}},
		{Login: "alpha", CharacterID: 1, Cells: []Cell{{Cell: 0, ID: 1, Vnum: 20, Count: 2}}},
	}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save safebox: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read safebox snapshot: %v", err)
	}
	wantRaw := "{\n  \"characters\": [\n    {\n      \"login\": \"alpha\",\n      \"character_id\": 1,\n      \"cells\": [\n        {\n          \"cell\": 0,\n          \"id\": 1,\n          \"vnum\": 20,\n          \"count\": 2\n        }\n      ]\n    },\n    {\n      \"login\": \"zeta\",\n      \"character_id\": 2,\n      \"cells\": [\n        {\n          \"cell\": 1,\n          \"id\": 2,\n          \"vnum\": 10,\n          \"count\": 1\n        }\n      ]\n    }\n  ]\n}\n"
	if string(raw) != wantRaw {
		t.Fatalf("unexpected deterministic safebox JSON:\n got: %s\nwant: %s", string(raw), wantRaw)
	}
}

func TestFileStoreLoadReturnsNotFoundForMissingSnapshot(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "safebox.json"))
	_, err := store.Load()
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestFileStoreRejectsInvalidRowsAndMissingSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "null_root", body: `null`},
		{name: "null_characters", body: `{"characters":null}`},
		{name: "unknown_root_field", body: `{"characters":[],"unknown":true}`},
		{name: "unknown_cell_field", body: `{"characters":[{"login":"owner","character_id":1,"cells":[{"cell":0,"id":1,"vnum":10,"count":1,"unknown":true}]}]}`},
		{name: "blank_login", body: `{"characters":[{"login":" ","character_id":1,"cells":[{"cell":0,"id":1,"vnum":10,"count":1}]}]}`},
		{name: "zero_character_id", body: `{"characters":[{"login":"owner","character_id":0,"cells":[{"cell":0,"id":1,"vnum":10,"count":1}]}]}`},
		{name: "cell_out_of_range", body: `{"characters":[{"login":"owner","character_id":1,"cells":[{"cell":15,"id":1,"vnum":10,"count":1}]}]}`},
		{name: "duplicate_cell", body: `{"characters":[{"login":"owner","character_id":1,"cells":[{"cell":0,"id":1,"vnum":10,"count":1},{"cell":0,"id":2,"vnum":10,"count":1}]}]}`},
		{name: "duplicate_character", body: `{"characters":[{"login":"owner","character_id":1,"cells":[]},{"login":"owner","character_id":1,"cells":[]}]}`},
		{name: "zero_item_id", body: `{"characters":[{"login":"owner","character_id":1,"cells":[{"cell":0,"id":0,"vnum":10,"count":1}]}]}`},
		{name: "nonzero_sockets_without_has_sockets", body: `{"characters":[{"login":"owner","character_id":1,"cells":[{"cell":0,"id":1,"vnum":10,"count":1,"socket0":1}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write %s snapshot: %v", tc.name, err)
			}
			_, err := store.Load()
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.name, err)
			}
		})
	}
}

func TestFileStoreValidateReportsCrashTemps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Characters: []CharacterRow{{
		Login: "owner", CharacterID: 7, Cells: []Cell{{Cell: 0, ID: 1, Vnum: 10, Count: 1}},
	}}}); err != nil {
		t.Fatalf("save safebox: %v", err)
	}
	for _, name := range []string{".safebox-b.json", ".safebox-a.json", ".not-safebox.json"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), name), []byte(`{"characters":[]}`), 0o644); err != nil {
			t.Fatalf("write crash temp %s: %v", name, err)
		}
	}
	summary, err := store.Validate()
	if err != nil {
		t.Fatalf("validate safebox store: %v", err)
	}
	if summary.CharacterCount != 1 || summary.CellCount != 1 || summary.CrashTempCount != 2 {
		t.Fatalf("unexpected validate summary: %+v", summary)
	}
	if !reflect.DeepEqual(summary.CrashTempFiles, []string{".safebox-a.json", ".safebox-b.json"}) {
		t.Fatalf("unexpected crash temp files: %#v", summary.CrashTempFiles)
	}
	cleaned, err := store.CleanupCrashTempFiles()
	if err != nil {
		t.Fatalf("cleanup crash temps: %v", err)
	}
	if cleaned.CrashTempCount != 0 {
		t.Fatalf("expected cleaned crash temps, got %+v", cleaned)
	}
}

func TestFileStoreBackupToWritesCommittedSnapshotAndDeterministicManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	if err := store.Save(Snapshot{Characters: []CharacterRow{{
		Login: "owner", CharacterID: 3, Cells: []Cell{{Cell: 1, ID: 55, Vnum: 27001, Count: 4}},
	}}}); err != nil {
		t.Fatalf("save safebox: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "backup")
	if err := store.BackupTo(dst); err != nil {
		t.Fatalf("backup safebox: %v", err)
	}
	summary, err := store.ValidateBackupFrom(dst)
	if err != nil {
		t.Fatalf("validate backup: %v", err)
	}
	if summary.CharacterCount != 1 || summary.CellCount != 1 {
		t.Fatalf("unexpected backup summary: %+v", summary)
	}
}

func TestFileStoreBackupToTreatsMissingSnapshotAsEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	dst := filepath.Join(t.TempDir(), "backup")
	if err := store.BackupTo(dst); err != nil {
		t.Fatalf("backup empty safebox: %v", err)
	}
	summary, err := store.ValidateBackupFrom(dst)
	if err != nil {
		t.Fatalf("validate empty backup: %v", err)
	}
	if summary.CharacterCount != 0 || summary.CellCount != 0 {
		t.Fatalf("expected empty backup summary, got %+v", summary)
	}
}

func TestFileStoreRestoreFromRestoresManifestedBackupIntoEmptyStore(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "src", "safebox.json")
	src := NewFileStore(srcPath)
	if err := src.Save(Snapshot{Characters: []CharacterRow{{
		Login: "owner", CharacterID: 9, Cells: []Cell{{Cell: 0, ID: 77, Vnum: 27001, Count: 1}},
	}}}); err != nil {
		t.Fatalf("save source safebox: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "backup")
	if err := src.BackupTo(backupDir); err != nil {
		t.Fatalf("backup source safebox: %v", err)
	}
	dstPath := filepath.Join(t.TempDir(), "dst", "safebox.json")
	dst := NewFileStore(dstPath)
	if err := dst.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore safebox: %v", err)
	}
	loaded, err := dst.Load()
	if err != nil {
		t.Fatalf("load restored safebox: %v", err)
	}
	if len(loaded.Characters) != 1 || loaded.Characters[0].Cells[0].ID != 77 {
		t.Fatalf("unexpected restored snapshot: %#v", loaded)
	}
}

func TestMemorySafeboxStoreReplaceClearsWithoutFilesystem(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected missing memory snapshot, got %v", err)
	}
	if err := store.Save(Snapshot{Characters: []CharacterRow{{
		Login: "owner", CharacterID: 1, Cells: []Cell{{Cell: 0, ID: 1, Vnum: 10, Count: 1}},
	}}}); err != nil {
		t.Fatalf("save memory safebox: %v", err)
	}
	store.Clear()
	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected cleared memory snapshot, got %v", err)
	}
}

func TestReplaceCharacterCellsUpsertsAndRemovesRows(t *testing.T) {
	snapshot := Snapshot{}
	next, err := ReplaceCharacterCells(snapshot, "owner", 1, map[uint8]inventory.ItemInstance{
		0: {ID: 10, Vnum: 27001, Count: 2, Slot: 0},
	})
	if err != nil {
		t.Fatalf("replace cells: %v", err)
	}
	if len(next.Characters) != 1 || next.Characters[0].Cells[0].ID != 10 {
		t.Fatalf("unexpected upsert: %#v", next)
	}
	cleared, err := ReplaceCharacterCells(next, "owner", 1, nil)
	if err != nil {
		t.Fatalf("clear cells: %v", err)
	}
	if len(cleared.Characters) != 0 {
		t.Fatalf("expected cleared character row, got %#v", cleared)
	}
}

func TestFileStoreRoundTripPersistsPasswordAndDefaultsBlank(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	input := Snapshot{Characters: []CharacterRow{{
		Login:       "owner-one",
		CharacterID: 42,
		Password:    "secret",
		Cells:       []Cell{{Cell: 0, ID: 9001, Vnum: 27002, Count: 1}},
	}}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save safebox with password: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load safebox with password: %v", err)
	}
	if loaded.Characters[0].Password != "secret" {
		t.Fatalf("unexpected loaded password: %#v", loaded.Characters[0])
	}
	if got := CharacterPassword(loaded, "owner-one", 42); got != "secret" {
		t.Fatalf("CharacterPassword=%q want secret", got)
	}
	if got := CharacterPassword(Snapshot{}, "missing", 1); got != DefaultPassword {
		t.Fatalf("missing row CharacterPassword=%q want %q", got, DefaultPassword)
	}
	blank := Snapshot{Characters: []CharacterRow{{Login: "owner-two", CharacterID: 7}}}
	if got := CharacterPassword(blank, "owner-two", 7); got != DefaultPassword {
		t.Fatalf("blank password CharacterPassword=%q want %q", got, DefaultPassword)
	}
}

func TestFileStoreSaveOmitsBlankPasswordForDeterministicJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	input := Snapshot{Characters: []CharacterRow{{
		Login: "alpha", CharacterID: 1, Password: "", Cells: []Cell{{Cell: 0, ID: 1, Vnum: 20, Count: 2}},
	}}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save blank-password safebox: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read blank-password safebox: %v", err)
	}
	if strings.Contains(string(raw), `"password"`) {
		t.Fatalf("expected blank password omitted from JSON, got %s", string(raw))
	}
}

func TestFileStoreRejectsInvalidPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "too_long", body: `{"characters":[{"login":"owner","character_id":1,"password":"1234567","cells":[]}]}`},
		{name: "symbols", body: `{"characters":[{"login":"owner","character_id":1,"password":"ab cd","cells":[]}]}`},
		{name: "nul", body: "{\"characters\":[{\"login\":\"owner\",\"character_id\":1,\"password\":\"ab\\u0000c\",\"cells\":[]}]} "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(strings.TrimSpace(tc.body)), 0o644); err != nil {
				t.Fatalf("write %s: %v", tc.name, err)
			}
			_, err := store.Load()
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.name, err)
			}
		})
	}
}

func TestReplaceCharacterPasswordPreservesCellsAndSurvivesEmptyCells(t *testing.T) {
	snapshot := Snapshot{Characters: []CharacterRow{{
		Login: "owner", CharacterID: 1, Cells: []Cell{{Cell: 0, ID: 10, Vnum: 27001, Count: 2}},
	}}}
	withPassword, err := ReplaceCharacterPassword(snapshot, "owner", 1, "abc123")
	if err != nil {
		t.Fatalf("replace password: %v", err)
	}
	if withPassword.Characters[0].Password != "abc123" || withPassword.Characters[0].Cells[0].ID != 10 {
		t.Fatalf("unexpected password upsert: %#v", withPassword)
	}
	cleared, err := ReplaceCharacterCells(withPassword, "owner", 1, nil)
	if err != nil {
		t.Fatalf("clear cells with password: %v", err)
	}
	if len(cleared.Characters) != 1 || cleared.Characters[0].Password != "abc123" || len(cleared.Characters[0].Cells) != 0 {
		t.Fatalf("expected password-only row after clear, got %#v", cleared)
	}
	if got := CharacterPassword(cleared, "owner", 1); got != "abc123" {
		t.Fatalf("CharacterPassword after clear=%q want abc123", got)
	}
}

func TestFileStoreRoundTripPersistsMoneyAndOmitsZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	input := Snapshot{Characters: []CharacterRow{{
		Login:       "owner-money",
		CharacterID: 42,
		Password:    "secret",
		Money:       1500,
		Cells:       []Cell{{Cell: 0, ID: 9001, Vnum: 27002, Count: 1}},
	}}}
	if err := store.Save(input); err != nil {
		t.Fatalf("save safebox with money: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load safebox with money: %v", err)
	}
	if loaded.Characters[0].Money != 1500 {
		t.Fatalf("unexpected loaded money: %#v", loaded.Characters[0])
	}
	if got := CharacterMoney(loaded, "owner-money", 42); got != 1500 {
		t.Fatalf("CharacterMoney=%d want 1500", got)
	}
	if got := CharacterMoney(Snapshot{}, "missing", 1); got != 0 {
		t.Fatalf("missing row CharacterMoney=%d want 0", got)
	}

	zero := Snapshot{Characters: []CharacterRow{{
		Login: "alpha", CharacterID: 1, Password: "vault1", Money: 0, Cells: []Cell{{Cell: 0, ID: 1, Vnum: 20, Count: 2}},
	}}}
	if err := store.Save(zero); err != nil {
		t.Fatalf("save zero-money safebox: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read zero-money safebox: %v", err)
	}
	if strings.Contains(string(raw), `"money"`) {
		t.Fatalf("expected zero money omitted from JSON, got %s", string(raw))
	}
}

func TestFileStoreRejectsInvalidMoney(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "safebox.json")
	store := NewFileStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "negative", body: `{"characters":[{"login":"owner","character_id":1,"money":-1,"cells":[]}]}`},
		{name: "over_max_int32", body: `{"characters":[{"login":"owner","character_id":1,"money":2147483648,"cells":[]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(strings.TrimSpace(tc.body)), 0o644); err != nil {
				t.Fatalf("write %s: %v", tc.name, err)
			}
			_, err := store.Load()
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("expected ErrInvalidSnapshot for %s, got %v", tc.name, err)
			}
		})
	}
}

func TestReplaceCharacterMoneyPreservesPasswordAndCells(t *testing.T) {
	snapshot := Snapshot{Characters: []CharacterRow{{
		Login: "owner", CharacterID: 1, Password: "abc123", Money: 10,
		Cells: []Cell{{Cell: 0, ID: 10, Vnum: 27001, Count: 2}},
	}}}
	withMoney, err := ReplaceCharacterMoney(snapshot, "owner", 1, 2500)
	if err != nil {
		t.Fatalf("replace money: %v", err)
	}
	if withMoney.Characters[0].Money != 2500 || withMoney.Characters[0].Password != "abc123" || withMoney.Characters[0].Cells[0].ID != 10 {
		t.Fatalf("unexpected money upsert: %#v", withMoney)
	}
	withPassword, err := ReplaceCharacterPassword(withMoney, "owner", 1, "vault2")
	if err != nil {
		t.Fatalf("replace password after money: %v", err)
	}
	if withPassword.Characters[0].Money != 2500 || withPassword.Characters[0].Password != "vault2" {
		t.Fatalf("password replace dropped money: %#v", withPassword)
	}
	cleared, err := ReplaceCharacterCells(withPassword, "owner", 1, nil)
	if err != nil {
		t.Fatalf("clear cells with money: %v", err)
	}
	if len(cleared.Characters) != 1 || cleared.Characters[0].Money != 2500 || cleared.Characters[0].Password != "vault2" || len(cleared.Characters[0].Cells) != 0 {
		t.Fatalf("expected password+money row after clear, got %#v", cleared)
	}
	if _, err := ReplaceCharacterMoney(snapshot, "owner", 1, -1); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected negative money reject, got %v", err)
	}
	if _, err := ReplaceCharacterMoney(snapshot, "owner", 1, int64(math.MaxInt32)+1); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected over-MaxInt32 money reject, got %v", err)
	}
}
