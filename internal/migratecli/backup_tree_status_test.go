package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestRunBackupTreeStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-backup-tree")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"backup-tree-status", "--backup-tree", missing}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing backup-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing backup-tree-status not to write stderr, got %q", stderr.String())
	}
	var got backupTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing backup-tree-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != backupTreeStatusFormat || got.Present || got.BackupTree != "" || len(got.Stores) != 0 || got.StoreCount != 0 || got.StoresComplete != nil {
		t.Fatalf("unexpected missing backup-tree-status: %#v", got)
	}
	if strings.Contains(stdout.String(), `"backup_tree"`) || strings.Contains(stdout.String(), `"stores"`) || strings.Contains(stdout.String(), `"stores_complete"`) {
		t.Fatalf("missing backup-tree-status must omit inner fields, got %s", stdout.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("backup-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunBackupTreeStatusReportsCompleteEmptyAndSeededStores(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T120000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected complete backup-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on complete backup-tree-status, got %q", stderr.String())
	}

	var got backupTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode complete backup-tree-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != backupTreeStatusFormat || !got.Present || got.BackupTree != tree {
		t.Fatalf("unexpected complete envelope: %#v", got)
	}
	if got.StoreCount != 8 || got.StorePresentCount != 8 || got.StoresComplete == nil || !*got.StoresComplete || len(got.Stores) != 8 {
		t.Fatalf("unexpected complete aggregates: %#v", got)
	}
	wantKinds := []string{"accounts", "login-tickets", "item-templates", "interaction-store", "static-actors", "quest-state", "ground-items", "safebox"}
	for i, kind := range wantKinds {
		entry := got.Stores[i]
		if entry.Kind != kind || !entry.Present || !entry.Valid || entry.Path != kind || entry.ManifestFilename == "" || entry.Format == "" || entry.ManifestSHA256 == "" {
			t.Fatalf("unexpected store[%d]=%#v", i, entry)
		}
		raw, err := os.ReadFile(filepath.Join(tree, kind, entry.ManifestFilename))
		if err != nil {
			t.Fatalf("read %s manifest: %v", kind, err)
		}
		if entry.ManifestSHA256 != sha256Hex(raw) {
			t.Fatalf("%s manifest_sha256=%s want %s", kind, entry.ManifestSHA256, sha256Hex(raw))
		}
	}
	if got.Stores[0].AccountCount != 1 || got.Stores[0].CharacterCount != 1 {
		t.Fatalf("expected seeded accounts counts, got %#v", got.Stores[0])
	}

	body := stdout.String()
	for _, forbidden := range []string{
		"mkmk", "MkmkWar", "CREATE TABLE", "DROP TABLE", "INSERT ", "SELECT ",
		"memory://", "postgres://", "mysql://", "password=", "DSN=",
		`"logins"`, `"login_keys"`, `"vnums"`, `"vids"`, `"actor_ids"`, `"actor_names"`,
		`"definition_keys"`, `"flag_keys"`, `"character_keys"`, `"crash_temp_files"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("backup-tree-status must not expose %q, got %s", forbidden, body)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("backup-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunBackupTreeStatusReportsIncompleteStoresUngated(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T130000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, false)
	if err := os.RemoveAll(filepath.Join(tree, "safebox")); err != nil {
		t.Fatalf("remove safebox store: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected incomplete backup-tree-status to succeed ungated, exit=%d stderr=%q", code, stderr.String())
	}
	var got backupTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode incomplete backup-tree-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || got.StoreCount != 8 || got.StorePresentCount != 7 || got.StoresComplete == nil || *got.StoresComplete {
		t.Fatalf("unexpected incomplete aggregates: %#v", got)
	}
	last := got.Stores[len(got.Stores)-1]
	if last.Kind != "safebox" || last.Present || last.Valid || last.Path != "" {
		t.Fatalf("expected missing safebox entry, got %#v", last)
	}
	if !strings.Contains(stdout.String(), `"stores_complete": false`) {
		t.Fatalf("expected incomplete JSON to emit stores_complete=false, got %s", stdout.String())
	}
}

func TestRunBackupTreeStatusRequireStoresCompleteRejectsMissingAndIncomplete(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	missing := filepath.Join(t.TempDir(), "missing-backup-tree")
	var missingOut bytes.Buffer
	var missingErr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", missing, "--require-stores-complete"}, nil, &missingOut, &missingErr)
	if code != exitError {
		t.Fatalf("expected require-stores-complete missing tree to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, missingOut.String(), missingErr.String())
	}
	if missingOut.Len() != 0 {
		t.Fatalf("expected no stdout on require-stores-complete miss, got %q", missingOut.String())
	}
	if !strings.Contains(missingErr.String(), "require-stores-complete") || !strings.Contains(missingErr.String(), "absent") {
		t.Fatalf("expected require-stores-complete/absent guidance, got %q", missingErr.String())
	}

	tree := filepath.Join(t.TempDir(), "backups", "20260906T140000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, false)
	if err := os.RemoveAll(filepath.Join(tree, "quest-state")); err != nil {
		t.Fatalf("remove quest-state store: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"backup-tree-status", "--backup-tree", tree, "--require-stores-complete"}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected require-stores-complete incomplete tree to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-stores-complete incomplete, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "require-stores-complete") || !strings.Contains(stderr.String(), "stores_complete") {
		t.Fatalf("expected require-stores-complete guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("backup-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunBackupTreeStatusAcceptsHiddenCrashTemps(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T150000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, true)
	if err := os.WriteFile(filepath.Join(tree, "accounts", ".account-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write account crash temp: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected crash-temp backup-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got backupTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode crash-temp backup-tree-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.StoresComplete == nil || !*got.StoresComplete || got.Stores[0].CrashTempCount != 1 {
		t.Fatalf("expected accounts crash_temp_count=1, got %#v", got.Stores[0])
	}
	if strings.Contains(stdout.String(), ".account-crashed.json") {
		t.Fatalf("backup-tree-status must not expose crash-temp filenames, got %s", stdout.String())
	}
}

func TestRunBackupTreeStatusRequireNoCrashTempsRejectsHiddenCrashTemps(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T151000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, true)
	if err := os.WriteFile(filepath.Join(tree, "accounts", ".account-crashed.json"), []byte(`{"not":"committed"}`), 0o644); err != nil {
		t.Fatalf("write account crash temp: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", tree, "--require-no-crash-temps"}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected require-no-crash-temps crash-temp tree to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-no-crash-temps crash-temp tree, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "require-no-crash-temps") || !strings.Contains(stderr.String(), "crash_temp_count") || !strings.Contains(stderr.String(), "accounts") {
		t.Fatalf("expected require-no-crash-temps/crash_temp_count/accounts guidance, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), ".account-crashed.json") {
		t.Fatalf("require-no-crash-temps must not expose crash-temp filenames, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("backup-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunBackupTreeStatusRequireNoCrashTempsRejectsMissingTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-backup-tree")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", missing, "--require-no-crash-temps"}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected require-no-crash-temps missing tree to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-no-crash-temps miss, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "require-no-crash-temps") || !strings.Contains(stderr.String(), "absent") {
		t.Fatalf("expected require-no-crash-temps/absent guidance, got %q", stderr.String())
	}
}

func TestRunBackupTreeStatusRequireNoCrashTempsSucceedsOnCompleteCleanTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T152000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", tree, "--require-no-crash-temps"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected require-no-crash-temps clean tree to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on require-no-crash-temps clean tree, got %q", stderr.String())
	}
	var got backupTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode require-no-crash-temps clean JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.StoresComplete == nil || !*got.StoresComplete || got.StorePresentCount != 8 {
		t.Fatalf("unexpected require-no-crash-temps clean aggregates: %#v", got)
	}
	for _, entry := range got.Stores {
		if entry.CrashTempCount != 0 {
			t.Fatalf("expected omitted/zero crash_temp_count, got %#v", entry)
		}
	}
}

func TestRunBackupTreeStatusRequireNoCrashTempsAllowsIncompleteCleanTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T153000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, false)
	if err := os.RemoveAll(filepath.Join(tree, "safebox")); err != nil {
		t.Fatalf("remove safebox store: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", tree, "--require-no-crash-temps"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected require-no-crash-temps incomplete clean tree to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got backupTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode incomplete clean JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || got.StorePresentCount != 7 || got.StoresComplete == nil || *got.StoresComplete {
		t.Fatalf("unexpected incomplete clean aggregates: %#v", got)
	}
}

func TestRunBackupTreeStatusBothRequireFlagsFailStoresCompleteFirst(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T154000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, false)
	if err := os.RemoveAll(filepath.Join(tree, "quest-state")); err != nil {
		t.Fatalf("remove quest-state store: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"backup-tree-status",
		"--backup-tree", tree,
		"--require-stores-complete",
		"--require-no-crash-temps",
	}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected combined require flags on incomplete tree to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on combined require flags, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "require-stores-complete") || !strings.Contains(stderr.String(), "stores_complete") {
		t.Fatalf("expected stores-complete to fail first, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "require-no-crash-temps") {
		t.Fatalf("expected stores-complete reason without crash-temp gate, got %q", stderr.String())
	}
}

func TestRunBackupTreeStatusRejectsInvalidPresentStores(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)

	t.Run("missing-manifest", func(t *testing.T) {
		tree := filepath.Join(t.TempDir(), "backups", "20260906T160000Z-abcdef012345")
		mustMaterializeCompleteBackupTree(t, tree, false)
		if err := os.Remove(filepath.Join(tree, "accounts", accountstore.BackupManifestFilename)); err != nil {
			t.Fatalf("remove accounts manifest: %v", err)
		}
		assertBackupTreeStatusRejects(t, tree, "accounts")
	})

	t.Run("checksum-mismatch", func(t *testing.T) {
		tree := filepath.Join(t.TempDir(), "backups", "20260906T161000Z-abcdef012345")
		mustMaterializeCompleteBackupTree(t, tree, true)
		entries, err := os.ReadDir(filepath.Join(tree, "accounts"))
		if err != nil {
			t.Fatalf("read accounts backup: %v", err)
		}
		corrupted := false
		for _, entry := range entries {
			if entry.Name() == accountstore.BackupManifestFilename || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			path := filepath.Join(tree, "accounts", entry.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read account snapshot: %v", err)
			}
			if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
				t.Fatalf("corrupt account snapshot: %v", err)
			}
			corrupted = true
			break
		}
		if !corrupted {
			t.Fatal("expected a committed account snapshot to corrupt")
		}
		assertBackupTreeStatusRejects(t, tree, "accounts")
	})

	t.Run("symlink-store", func(t *testing.T) {
		tree := filepath.Join(t.TempDir(), "backups", "20260906T162000Z-abcdef012345")
		mustMaterializeCompleteBackupTree(t, tree, false)
		realSafebox := filepath.Join(tree, "safebox")
		aside := filepath.Join(t.TempDir(), "safebox-real")
		if err := os.Rename(realSafebox, aside); err != nil {
			t.Fatalf("rename safebox store: %v", err)
		}
		if err := os.Symlink(aside, realSafebox); err != nil {
			t.Fatalf("symlink safebox store: %v", err)
		}
		assertBackupTreeStatusRejects(t, tree, "safebox")
	})

	t.Run("snapshot-basename-drift", func(t *testing.T) {
		tree := filepath.Join(t.TempDir(), "backups", "20260906T163000Z-abcdef012345")
		mustMaterializeCompleteBackupTree(t, tree, false)
		if err := os.RemoveAll(filepath.Join(tree, "item-templates")); err != nil {
			t.Fatalf("remove item-templates: %v", err)
		}
		live := filepath.Join(t.TempDir(), "wrong-name.json")
		if err := itemstore.NewFileStore(live).Save(itemstore.Snapshot{Templates: []itemstore.Template{{
			Vnum:      27001,
			Name:      "Small Red Potion",
			Stackable: true,
			MaxCount:  200,
		}}}); err != nil {
			t.Fatalf("save drifted item templates: %v", err)
		}
		if err := itemstore.NewFileStore(live).BackupTo(filepath.Join(tree, "item-templates")); err != nil {
			t.Fatalf("backup drifted item templates: %v", err)
		}
		assertBackupTreeStatusRejects(t, tree, "item-templates")
	})
}

func TestRunBackupTreeStatusRejectsRelativeSymlinkAndFileTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	fileTree := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(fileTree, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write file tree: %v", err)
	}
	realTree := filepath.Join(dir, "real-tree")
	if err := os.MkdirAll(realTree, 0o755); err != nil {
		t.Fatalf("mkdir real-tree: %v", err)
	}
	linkTree := filepath.Join(dir, "link-tree")
	if err := os.Symlink(realTree, linkTree); err != nil {
		t.Fatalf("symlink backup-tree: %v", err)
	}

	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{
			name: "relative",
			args: []string{"backup-tree-status", "--backup-tree", "relative/backups/tree"},
			code: exitError,
			want: "absolute path",
		},
		{
			name: "file",
			args: []string{"backup-tree-status", "--backup-tree", fileTree},
			code: exitError,
			want: "directory",
		},
		{
			name: "symlink",
			args: []string{"backup-tree-status", "--backup-tree", linkTree},
			code: exitError,
			want: "symlink",
		},
		{
			name: "usage",
			args: []string{"backup-tree-status"},
			code: exitUsage,
			want: "--backup-tree is required",
		},
		{
			name: "extra-arg",
			args: []string{"backup-tree-status", "--backup-tree", realTree, "extra"},
			code: exitUsage,
			want: "unexpected backup-tree-status argument",
		},
		{
			name: "stdin-dash",
			args: []string{"backup-tree-status", "--backup-tree", "-"},
			code: exitError,
			want: "absolute path",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != tc.code {
				t.Fatalf("exit=%d want %d stderr=%q", code, tc.code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.want, stderr.String())
			}
		})
	}
}

func TestRunBackupTreeStatusHelpListsCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	body := stdout.String()
	if !strings.Contains(body, "backup-tree-status") {
		t.Fatalf("expected help to list backup-tree-status, got %q", body)
	}
	if !strings.Contains(body, "backup-tree-status usage:") {
		t.Fatalf("expected backup-tree-status usage block, got %q", body)
	}
	if !strings.Contains(body, "--backup-tree") || !strings.Contains(body, "--require-stores-complete") {
		t.Fatalf("expected --backup-tree and --require-stores-complete in usage, got %q", body)
	}
}

func TestRunRejectsUnknownCommandMentionsBackupTreeStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "backup-tree-status") {
		t.Fatalf("expected usage to mention backup-tree-status, got %q", stderr.String())
	}
}

func TestRunBackupTreeStatusUsageListsRequireFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--require-stores-complete") {
		t.Fatalf("expected usage to list --require-stores-complete, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--require-no-crash-temps") {
		t.Fatalf("expected usage to list --require-no-crash-temps, got %q", stderr.String())
	}
}

func assertBackupTreeStatusRejects(t *testing.T, tree string, wantKind string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", tree}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected invalid %s backup-tree-status to exit %d, got exit=%d stdout=%q stderr=%q", wantKind, exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on invalid %s tree, got %q", wantKind, stdout.String())
	}
	if !strings.Contains(stderr.String(), wantKind) {
		t.Fatalf("expected stderr to name store kind %q, got %q", wantKind, stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("backup-tree-status must not open a database target, got events %#v", events)
	}
}

func disableBackupTreeStatusDurableSync(t *testing.T) {
	t.Helper()
	restores := []func(){
		accountstore.DisableDurableSyncForTest(),
		loginticket.DisableDurableSyncForTest(),
		itemstore.DisableDurableSyncForTest(),
		interactionstore.DisableDurableSyncForTest(),
		staticstore.DisableDurableSyncForTest(),
		queststate.DisableDurableSyncForTest(),
		worldruntime.DisableDurableGroundItemSyncForTest(),
		safeboxstore.DisableDurableSyncForTest(),
	}
	t.Cleanup(func() {
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
	})
}

func mustMaterializeCompleteBackupTree(t *testing.T, tree string, seedAccount bool) {
	t.Helper()
	live := t.TempDir()
	accountDir := filepath.Join(live, "accounts")
	if seedAccount {
		if err := accountstore.NewFileStore(accountDir).Save(accountstore.Account{
			Login:  "mkmk",
			Empire: 2,
			Characters: []loginticket.Character{{
				ID:   1,
				Name: "MkmkWar",
			}},
		}); err != nil {
			t.Fatalf("seed account store: %v", err)
		}
	}
	if err := accountstore.NewFileStore(accountDir).BackupTo(filepath.Join(tree, "accounts")); err != nil {
		t.Fatalf("backup accounts: %v", err)
	}
	if err := loginticket.NewFileStore(filepath.Join(live, "login-tickets")).BackupTo(filepath.Join(tree, "login-tickets")); err != nil {
		t.Fatalf("backup login tickets: %v", err)
	}
	if err := itemstore.NewFileStore(filepath.Join(live, "item-templates", "item-templates.json")).BackupTo(filepath.Join(tree, "item-templates")); err != nil {
		t.Fatalf("backup item templates: %v", err)
	}
	if err := interactionstore.NewFileStore(filepath.Join(live, "interaction-store", "interaction-definitions.json")).BackupTo(filepath.Join(tree, "interaction-store")); err != nil {
		t.Fatalf("backup interactions: %v", err)
	}
	if err := staticstore.NewFileStore(filepath.Join(live, "static-actors", "static-actors.json")).BackupTo(filepath.Join(tree, "static-actors")); err != nil {
		t.Fatalf("backup static actors: %v", err)
	}
	if err := queststate.NewFileStore(filepath.Join(live, "quest-state", "quest-state.json")).BackupTo(filepath.Join(tree, "quest-state")); err != nil {
		t.Fatalf("backup quest state: %v", err)
	}
	if err := worldruntime.NewGroundItemFileStore(filepath.Join(live, "ground-items", "ground-items.json")).BackupTo(filepath.Join(tree, "ground-items")); err != nil {
		t.Fatalf("backup ground items: %v", err)
	}
	if err := safeboxstore.NewFileStore(filepath.Join(live, "safebox", "safebox.json")).BackupTo(filepath.Join(tree, "safebox")); err != nil {
		t.Fatalf("backup safebox: %v", err)
	}
}
