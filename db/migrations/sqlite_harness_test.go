//go:build sqlite_harness

package migrations_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestSQLiteHarnessAppliesCatalogAndReadsLedgerSnapshot(t *testing.T) {
	db := openSQLiteHarnessDB(t)
	defer db.Close()

	ctx := context.Background()
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("expected non-empty migration catalog")
	}

	applyResult, err := dbmigrations.ApplyUpToLatest(ctx, db, nil)
	if err != nil {
		t.Fatalf("ApplyUpToLatest on empty SQLite ledger: %v", err)
	}
	wantTip := catalog[len(catalog)-1].Version
	if applyResult.PreviousVersion != 0 {
		t.Fatalf("PreviousVersion = %d, want 0", applyResult.PreviousVersion)
	}
	if applyResult.CurrentVersion != wantTip {
		t.Fatalf("CurrentVersion = %d, want tip %d", applyResult.CurrentVersion, wantTip)
	}
	if applyResult.LatestVersion != wantTip {
		t.Fatalf("LatestVersion = %d, want tip %d", applyResult.LatestVersion, wantTip)
	}
	if len(applyResult.Applied) != wantTip {
		t.Fatalf("Applied steps = %d, want %d", len(applyResult.Applied), wantTip)
	}

	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, db)
	if err != nil {
		t.Fatalf("ReadSQLLedgerEntries: %v", err)
	}
	if len(ledger) != wantTip {
		t.Fatalf("ledger rows = %d, want %d", len(ledger), wantTip)
	}
	for i, migration := range catalog {
		got := ledger[i]
		if got.Version != migration.Version || got.Name != migration.Name || got.UpSHA256 != migration.UpSHA256 {
			t.Fatalf("ledger[%d] = %+v, want version=%d name=%q up_sha256=%q", i, got, migration.Version, migration.Name, migration.UpSHA256)
		}
	}

	snapshot, err := dbmigrations.LedgerSnapshotFromSQLLedger(ctx, db)
	if err != nil {
		t.Fatalf("LedgerSnapshotFromSQLLedger: %v", err)
	}
	if snapshot.Format != dbmigrations.LedgerSnapshotFormat {
		t.Fatalf("snapshot format = %q, want %q", snapshot.Format, dbmigrations.LedgerSnapshotFormat)
	}
	if len(snapshot.Entries) != wantTip {
		t.Fatalf("snapshot entries = %d, want %d", len(snapshot.Entries), wantTip)
	}
	for i := range snapshot.Entries {
		if snapshot.Entries[i] != ledger[i] {
			t.Fatalf("snapshot.Entries[%d] = %+v, want %+v", i, snapshot.Entries[i], ledger[i])
		}
	}

	plan, err := dbmigrations.PlanUpToLatestFromSQLLedger(ctx, db)
	if err != nil {
		t.Fatalf("PlanUpToLatestFromSQLLedger: %v", err)
	}
	if !plan.UpToDate {
		t.Fatalf("plan.UpToDate = false, want true at tip")
	}
	if plan.CurrentVersion != wantTip || plan.LatestVersion != wantTip {
		t.Fatalf("plan versions current=%d latest=%d, want both %d", plan.CurrentVersion, plan.LatestVersion, wantTip)
	}
	if len(plan.Pending) != 0 {
		t.Fatalf("pending ups = %#v, want empty", plan.Pending)
	}
}

func TestSQLiteHarnessRollbackToZeroThenReapplyBootstrap(t *testing.T) {
	db := openSQLiteHarnessDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyUpToLatest(ctx, db, nil); err != nil {
		t.Fatalf("initial ApplyUpToLatest: %v", err)
	}

	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	tipLedger := make([]dbmigrations.LedgerEntry, 0, len(catalog))
	for _, migration := range catalog {
		tipLedger = append(tipLedger, dbmigrations.LedgerEntry{
			Version:  migration.Version,
			Name:     migration.Name,
			UpSHA256: migration.UpSHA256,
		})
	}

	rollback, err := dbmigrations.ApplyToVersion(ctx, db, tipLedger, 0)
	if err != nil {
		t.Fatalf("ApplyToVersion(0): %v", err)
	}
	if rollback.CurrentVersion != 0 {
		t.Fatalf("rollback CurrentVersion = %d, want 0", rollback.CurrentVersion)
	}

	emptyLedger, err := dbmigrations.ReadSQLLedgerEntries(ctx, db)
	if err != nil {
		t.Fatalf("ReadSQLLedgerEntries after rollback-to-zero: %v", err)
	}
	if len(emptyLedger) != 0 {
		t.Fatalf("ledger after rollback-to-zero = %#v, want empty", emptyLedger)
	}
	emptySnapshot, err := dbmigrations.LedgerSnapshotFromSQLLedger(ctx, db)
	if err != nil {
		t.Fatalf("LedgerSnapshotFromSQLLedger after rollback-to-zero: %v", err)
	}
	if emptySnapshot.Format != dbmigrations.LedgerSnapshotFormat || len(emptySnapshot.Entries) != 0 {
		t.Fatalf("snapshot after rollback-to-zero = %+v, want empty %q", emptySnapshot, dbmigrations.LedgerSnapshotFormat)
	}
	emptyPlan, err := dbmigrations.PlanUpToLatestFromSQLLedger(ctx, db)
	if err != nil {
		t.Fatalf("PlanUpToLatestFromSQLLedger after rollback-to-zero: %v", err)
	}
	if emptyPlan.CurrentVersion != 0 || emptyPlan.UpToDate || len(emptyPlan.Pending) == 0 {
		t.Fatalf("unexpected empty-ledger plan after rollback-to-zero: %#v", emptyPlan)
	}

	reapply, err := dbmigrations.ApplyToVersion(ctx, db, nil, 1)
	if err != nil {
		t.Fatalf("ApplyToVersion(1) after rollback: %v", err)
	}
	if reapply.CurrentVersion != 1 {
		t.Fatalf("reapply CurrentVersion = %d, want 1", reapply.CurrentVersion)
	}

	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, db)
	if err != nil {
		t.Fatalf("ReadSQLLedgerEntries after reapply 0001: %v", err)
	}
	if len(ledger) != 1 {
		t.Fatalf("ledger rows after reapply = %d, want 1", len(ledger))
	}
	if ledger[0].Version != catalog[0].Version || ledger[0].Name != catalog[0].Name || ledger[0].UpSHA256 != catalog[0].UpSHA256 {
		t.Fatalf("bootstrap ledger row = %+v, want catalog[0] metadata", ledger[0])
	}

	snapshot, err := dbmigrations.LedgerSnapshotFromSQLLedger(ctx, db)
	if err != nil {
		t.Fatalf("LedgerSnapshotFromSQLLedger after reapply: %v", err)
	}
	if snapshot.Format != dbmigrations.LedgerSnapshotFormat || len(snapshot.Entries) != 1 {
		t.Fatalf("snapshot after reapply = %+v, want one entry in %q", snapshot, dbmigrations.LedgerSnapshotFormat)
	}
}

func openSQLiteHarnessDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "schema-migrations-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
