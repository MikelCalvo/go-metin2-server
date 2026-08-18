package loginticket

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestFileStoreBackupToCopiesCommittedTickets(t *testing.T) {
	store := NewFileStore(t.TempDir())
	tickets := []Ticket{
		{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC), Characters: []Character{{ID: 1, Name: "MkmkWar"}}},
		{Login: "Beta", LoginKey: 0x0a0b0c0d, Empire: 1, IssuedAt: time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC), Characters: []Character{{ID: 2, Name: "BetaNinja"}}},
	}
	for _, ticket := range tickets {
		if err := store.Issue(ticket); err != nil {
			t.Fatalf("issue ticket %s: %v", ticket.Login, err)
		}
	}
	if err := os.WriteFile(filepath.Join(store.dir, ".ticket-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write crash temp file: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup tickets: %v", err)
	}

	backup := NewFileStore(backupDir)
	got, err := backup.List()
	if err != nil {
		t.Fatalf("list backup: %v", err)
	}
	gotLogins := make([]string, 0, len(got))
	for _, ticket := range got {
		gotLogins = append(gotLogins, ticket.Login)
	}
	if want := []string{"Beta", "mkmk"}; !reflect.DeepEqual(gotLogins, want) {
		t.Fatalf("unexpected backup logins: got %#v want %#v", gotLogins, want)
	}
	if _, err := os.Stat(filepath.Join(backupDir, ".ticket-crashed.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected crash temp file to be omitted from backup, stat err=%v", err)
	}
}

func TestFileStoreBackupToWritesDeterministicManifest(t *testing.T) {
	store := NewFileStore(t.TempDir())
	tickets := []Ticket{
		{Login: "zeta", LoginKey: 0x22222222, Empire: 3, IssuedAt: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC), Characters: []Character{{ID: 3, Name: "ZetaWar"}}},
		{Login: "alpha", LoginKey: 0x11111111, Empire: 1, IssuedAt: time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC), Characters: []Character{{ID: 1, Name: "AlphaWar"}, {}, {ID: 2, Name: "AlphaNinja"}}},
	}
	for _, ticket := range tickets {
		if err := store.Issue(ticket); err != nil {
			t.Fatalf("issue ticket %s: %v", ticket.Login, err)
		}
	}

	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup tickets: %v", err)
	}

	manifestRaw, err := os.ReadFile(filepath.Join(backupDir, BackupManifestFilename))
	if err != nil {
		t.Fatalf("read backup manifest: %v", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode backup manifest: %v", err)
	}
	if manifest.Format != BackupManifestFormat {
		t.Fatalf("unexpected manifest format: got %q want %q", manifest.Format, BackupManifestFormat)
	}
	wantSummary := SnapshotSummary{
		TicketCount:             2,
		CharacterCount:          4,
		EmptyCharacterSlotCount: 1,
		Logins:                  []string{"alpha", "zeta"},
		LoginKeys:               []uint32{0x11111111, 0x22222222},
		OldestIssuedAt:          timePtr(time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)),
		NewestIssuedAt:          timePtr(time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)),
	}
	if !reflect.DeepEqual(manifest.Summary, wantSummary) {
		t.Fatalf("unexpected manifest summary: got %#v want %#v", manifest.Summary, wantSummary)
	}
	wantLogins := []string{"alpha", "zeta"}
	gotLogins := make([]string, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		gotLogins = append(gotLogins, file.Login)
		raw, err := os.ReadFile(filepath.Join(backupDir, file.Filename))
		if err != nil {
			t.Fatalf("read manifest ticket file %s: %v", file.Filename, err)
		}
		checksum := sha256.Sum256(raw)
		if gotChecksum := hex.EncodeToString(checksum[:]); gotChecksum != file.SHA256 {
			t.Fatalf("unexpected checksum for %s: got %s want %s", file.Login, file.SHA256, gotChecksum)
		}
		if int64(len(raw)) != file.SizeBytes {
			t.Fatalf("unexpected size for %s: got %d want %d", file.Login, file.SizeBytes, len(raw))
		}
	}
	if !reflect.DeepEqual(gotLogins, wantLogins) {
		t.Fatalf("unexpected manifest file order: got %#v want %#v", gotLogins, wantLogins)
	}
}

func TestFileStoreBackupToRejectsNonEmptyDestination(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if err := store.Issue(Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	backupDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(backupDir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed non-empty backup dir: %v", err)
	}
	if err := store.BackupTo(backupDir); !errors.Is(err, ErrBackupDirNotEmpty) {
		t.Fatalf("expected ErrBackupDirNotEmpty, got %v", err)
	}
}

func TestFileStoreBackupToRejectsWhitespaceDestination(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if err := store.BackupTo("   "); !errors.Is(err, ErrBackupDirRequired) {
		t.Fatalf("expected ErrBackupDirRequired, got %v", err)
	}
}

func TestFileStoreBackupToRejectsDestinationInsideSourceStore(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if err := store.Issue(Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	nested := filepath.Join(store.dir, "nested-backup")
	if err := store.BackupTo(nested); !errors.Is(err, ErrBackupDirInsideStore) {
		t.Fatalf("expected ErrBackupDirInsideStore, got %v", err)
	}
}

func TestFileStoreBackupToTreatsMissingSourceAsEmptyBackup(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing-source"))
	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatalf("backup empty missing source: %v", err)
	}
	backup := NewFileStore(backupDir)
	summary, err := backup.Validate()
	if err != nil {
		t.Fatalf("validate empty backup: %v", err)
	}
	want := SnapshotSummary{Logins: []string{}, LoginKeys: []uint32{}}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty backup summary: got %#v want %#v", summary, want)
	}
	if _, err := os.Stat(filepath.Join(backupDir, BackupManifestFilename)); err != nil {
		t.Fatalf("expected empty backup manifest, stat err=%v", err)
	}
}

func TestFileStoreValidateBackupFromValidatesManifestWithoutRestoring(t *testing.T) {
	source := NewFileStore(t.TempDir())
	ticket := Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}
	if err := source.Issue(ticket); err != nil {
		t.Fatalf("issue source ticket: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	active := NewFileStore(filepath.Join(t.TempDir(), "active"))
	summary, err := active.ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate backup: %v", err)
	}
	want := SnapshotSummary{
		TicketCount:    1,
		CharacterCount: 1,
		Logins:         []string{"mkmk"},
		LoginKeys:      []uint32{0x01020304},
		OldestIssuedAt: timePtr(ticket.IssuedAt),
		NewestIssuedAt: timePtr(ticket.IssuedAt),
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected validate backup summary: got %#v want %#v", summary, want)
	}
	if got, err := active.List(); err != nil || len(got) != 0 {
		t.Fatalf("expected validate backup not to mutate active store, got %#v err=%v", got, err)
	}
}

func TestFileStoreValidateBackupFromReportsIgnoredCrashTempFiles(t *testing.T) {
	source := NewFileStore(t.TempDir())
	ticket := Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}
	if err := source.Issue(ticket); err != nil {
		t.Fatalf("issue source ticket: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".ticket-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write backup crash temp: %v", err)
	}
	summary, err := NewFileStore(t.TempDir()).ValidateBackupFrom(backupDir)
	if err != nil {
		t.Fatalf("validate backup with crash temp: %v", err)
	}
	if summary.CrashTempCount != 1 || !reflect.DeepEqual(summary.CrashTempFiles, []string{".ticket-crashed.json"}) {
		t.Fatalf("expected crash temp residue in dry-run summary, got %#v", summary)
	}
}

func TestFileStoreValidateBackupFromRejectsMissingBackupManifest(t *testing.T) {
	source := NewFileStore(t.TempDir())
	if err := source.Issue(Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}); err != nil {
		t.Fatalf("issue source ticket: %v", err)
	}
	_, err := NewFileStore(t.TempDir()).ValidateBackupFrom(source.dir)
	if !errors.Is(err, ErrBackupManifestRequired) {
		t.Fatalf("expected ErrBackupManifestRequired, got %v", err)
	}
}

func TestFileStoreValidateBackupFromRejectsManifestChecksumMismatch(t *testing.T) {
	source := NewFileStore(t.TempDir())
	ticket := Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}
	if err := source.Issue(ticket); err != nil {
		t.Fatalf("issue source ticket: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "01020304.json"), []byte(`{"login":"mkmk","login_key":16909060,"empire":2,"issued_at":"2026-08-18T10:00:00Z","characters":[{"id":1,"name":"Tampered"}]}`), 0o644); err != nil {
		t.Fatalf("tamper backup ticket: %v", err)
	}
	_, err := NewFileStore(t.TempDir()).ValidateBackupFrom(backupDir)
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for checksum mismatch, got %v", err)
	}
}

func TestFileStoreRestoreFromCopiesValidatedBackupIntoEmptyStore(t *testing.T) {
	source := NewFileStore(t.TempDir())
	ticket := Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}
	if err := source.Issue(ticket); err != nil {
		t.Fatalf("issue source ticket: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create backup: %v", err)
	}

	active := NewFileStore(filepath.Join(t.TempDir(), "active"))
	if err := active.RestoreFrom(backupDir); err != nil {
		t.Fatalf("restore tickets: %v", err)
	}
	got, err := active.Load("mkmk", 0x01020304)
	if err != nil {
		t.Fatalf("load restored ticket: %v", err)
	}
	ticket.Characters[0].NormalizeItemState()
	if !reflect.DeepEqual(got, ticket) {
		t.Fatalf("unexpected restored ticket:\n got: %#v\nwant: %#v", got, ticket)
	}
	if _, err := os.Stat(filepath.Join(active.dir, BackupManifestFilename)); err != nil {
		t.Fatalf("expected restored backup manifest, stat err=%v", err)
	}
}

func TestFileStoreRestoreFromRejectsNonEmptyDestination(t *testing.T) {
	source := NewFileStore(t.TempDir())
	if err := source.Issue(Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}); err != nil {
		t.Fatalf("issue source ticket: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	active := NewFileStore(t.TempDir())
	if err := active.Issue(Ticket{Login: "existing", LoginKey: 0x0fffffff, Empire: 1, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 9, Name: "Existing"}}}); err != nil {
		t.Fatalf("seed active ticket: %v", err)
	}
	if err := active.RestoreFrom(backupDir); !errors.Is(err, ErrRestoreDirNotEmpty) {
		t.Fatalf("expected ErrRestoreDirNotEmpty, got %v", err)
	}
}

func TestFileStoreRestoreFromRejectsDestinationInsideBackupSource(t *testing.T) {
	source := NewFileStore(t.TempDir())
	if err := source.Issue(Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}); err != nil {
		t.Fatalf("issue source ticket: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "login-ticket-backup")
	if err := source.BackupTo(backupDir); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	active := NewFileStore(filepath.Join(backupDir, "nested-active"))
	if err := active.RestoreFrom(backupDir); !errors.Is(err, ErrRestoreDirInsideSource) {
		t.Fatalf("expected ErrRestoreDirInsideSource, got %v", err)
	}
}

func TestFileStoreIssueRemovesStaleBackupManifestAfterMutation(t *testing.T) {
	store := NewFileStore(t.TempDir())
	first := Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}
	if err := store.Issue(first); err != nil {
		t.Fatalf("issue first ticket: %v", err)
	}
	if err := store.writeBackupManifest([]Ticket{first}); err != nil {
		t.Fatalf("write active backup manifest: %v", err)
	}
	second := Ticket{Login: "beta", LoginKey: 0x0a0b0c0d, Empire: 1, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 2, Name: "BetaNinja"}}}
	if err := store.Issue(second); err != nil {
		t.Fatalf("issue second ticket: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.dir, BackupManifestFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected issue to remove stale backup manifest, stat err=%v", err)
	}
}

func TestFileStoreValidateRejectsStaleBackupManifest(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ticket := Ticket{Login: "mkmk", LoginKey: 0x01020304, Empire: 2, IssuedAt: time.Now().UTC(), Characters: []Character{{ID: 1, Name: "MkmkWar"}}}
	if err := store.Issue(ticket); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	if err := store.writeBackupManifest([]Ticket{ticket}); err != nil {
		t.Fatalf("write backup manifest: %v", err)
	}
	if err := os.WriteFile(store.ticketPath(ticket.LoginKey), []byte(`{"login":"mkmk","login_key":16909060,"empire":2,"issued_at":"2026-08-18T10:00:00Z","characters":[{"id":1,"name":"TamperedWar"}]}`), 0o644); err != nil {
		t.Fatalf("tamper ticket after manifest write: %v", err)
	}
	_, err := store.Validate()
	if !errors.Is(err, ErrInvalidBackupManifest) {
		t.Fatalf("expected ErrInvalidBackupManifest for stale active manifest, got %v", err)
	}
}
