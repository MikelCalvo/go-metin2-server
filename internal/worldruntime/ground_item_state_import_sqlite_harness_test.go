//go:build sqlite_harness

package worldruntime

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessGroundItemStateImportInsertsItemAndGoldRows(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemStateMigrationVersion, err)
	}

	accounts := []accountstore.Account{
		{
			Login:  "ground-item-owner",
			Empire: 1,
			Characters: []loginticket.Character{
				groundItemStateImportCharacter(0x0103019c, "GroundItemOwner"),
			},
		},
		{
			Login:  "ground-gold-owner",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				groundItemStateImportCharacter(0x0103019d, "GroundGoldOwner"),
			},
		},
	}

	rosterExport, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	count := uint16(2)
	gold := uint32(250)
	snapshots := []GroundItemSnapshot{
		{VID: 0x0700002d, Vnum: 1, GoldAmount: gold, OwnerName: "GroundGoldOwner", OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, PickupRange: 750, MapIndex: 42, X: 1200, Y: 2200, Z: 3},
		{VID: 0x0700002c, Vnum: 3001, Count: count, OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, PickupRange: 450, MapIndex: 1, X: 1100, Y: 2100, Z: 2},
	}
	groundExport, err := ExportBootstrapGroundItemState(snapshots)
	if err != nil {
		t.Fatalf("ExportBootstrapGroundItemState: %v", err)
	}

	result, err := ImportBootstrapGroundItemState(ctx, db, groundExport)
	if err != nil {
		t.Fatalf("ImportBootstrapGroundItemState: %v", err)
	}
	if result.MigrationVersion != BootstrapGroundItemStateMigrationVersion || result.MigrationName != BootstrapGroundItemStateMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.GroundItemCount != 2 || result.ItemShapedCount != 1 || result.GoldShapedCount != 1 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.VIDs) != 2 || result.VIDs[0] != 0x0700002c || result.VIDs[1] != 0x0700002d {
		t.Fatalf("unexpected vids: %+v", result.VIDs)
	}

	var rowCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bootstrap_ground_items`).Scan(&rowCount); err != nil {
		t.Fatalf("count ground items: %v", err)
	}
	if rowCount != 2 {
		t.Fatalf("ground item rows = %d, want 2", rowCount)
	}

	assertGroundItemRow(t, db, 0x0700002c, 3001, sql.NullInt64{Int64: 2, Valid: true}, sql.NullInt64{}, "ground-item-owner", 0x0103019c, 0x0204019c, "GroundItemOwner", 1, 1100, 2100, 2, 450)
	assertGroundItemRow(t, db, 0x0700002d, 1, sql.NullInt64{}, sql.NullInt64{Int64: 250, Valid: true}, "ground-gold-owner", 0x0103019d, 0x0204019d, "GroundGoldOwner", 42, 1200, 2200, 3, 750)
}

func TestSQLiteHarnessGroundItemStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemStateMigrationVersion, err)
	}

	accounts := []accountstore.Account{{
		Login:  "ground-item-owner",
		Empire: 1,
		Characters: []loginticket.Character{
			groundItemStateImportCharacter(0x0103019c, "GroundItemOwner"),
		},
	}}
	rosterExport, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	groundExport, err := ExportBootstrapGroundItemState([]GroundItemSnapshot{{
		VID: 0x0700002c, Vnum: 3001, Count: 1,
		OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner",
		OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
		PickupRange: 300, MapIndex: 1, X: 100, Y: 200,
	}})
	if err != nil {
		t.Fatalf("ExportBootstrapGroundItemState: %v", err)
	}
	if _, err := ImportBootstrapGroundItemState(ctx, db, groundExport); err != nil {
		t.Fatalf("first ImportBootstrapGroundItemState: %v", err)
	}

	_, err = ImportBootstrapGroundItemState(ctx, db, groundExport)
	if err == nil {
		t.Fatal("second ImportBootstrapGroundItemState succeeded, want unique conflict")
	}

	var rowCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bootstrap_ground_items`).Scan(&rowCount); err != nil {
		t.Fatalf("count ground items after failed reimport: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("ground item rows after failed reimport = %d, want 1 (no partial second import)", rowCount)
	}
}

func TestSQLiteHarnessGroundItemStateImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems:      []BootstrapGroundItemStateRow{},
	}

	_, err := ImportBootstrapGroundItemState(ctx, db, export)
	if !errors.Is(err, ErrBootstrapGroundItemStateImportSchemaRequired) {
		t.Fatalf("ImportBootstrapGroundItemState on empty DB error = %v, want %v", err, ErrBootstrapGroundItemStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessGroundItemStateImportRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemStateMigrationVersion, err)
	}

	count := uint16(1)
	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems: []BootstrapGroundItemStateRow{{
			VID: 0x0700002c, Vnum: 3001, ItemCount: &count,
			OwnerLogin: "missing-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
			OwnerName: "MissingOwner", MapIndex: 1, X: 100, Y: 200, Z: 0, PickupRange: 300,
		}},
	}

	_, err := ImportBootstrapGroundItemState(ctx, db, export)
	if err == nil {
		t.Fatal("ImportBootstrapGroundItemState without parent character succeeded, want FK failure")
	}

	var rowCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bootstrap_ground_items`).Scan(&rowCount); err != nil {
		t.Fatalf("count ground items after FK failure: %v", err)
	}
	if rowCount != 0 {
		t.Fatalf("ground item rows after FK failure = %d, want 0", rowCount)
	}
}

func TestSQLiteHarnessGroundItemStateImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemStateMigrationVersion, err)
	}

	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems:      []BootstrapGroundItemStateRow{},
	}
	result, err := ImportBootstrapGroundItemState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportBootstrapGroundItemState(empty): %v", err)
	}
	if result.GroundItemCount != 0 || result.ItemShapedCount != 0 || result.GoldShapedCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
	if len(result.VIDs) != 0 {
		t.Fatalf("empty import vids = %+v, want empty", result.VIDs)
	}
}

func assertGroundItemRow(
	t *testing.T,
	db *sql.DB,
	vid uint32,
	vnum uint32,
	itemCount sql.NullInt64,
	goldAmount sql.NullInt64,
	ownerLogin string,
	ownerCharacterID uint32,
	ownerVID uint32,
	ownerName string,
	mapIndex uint32,
	x, y, z int32,
	pickupRange int64,
) {
	t.Helper()

	var (
		gotVID              int64
		gotVnum             int64
		gotItemCount        sql.NullInt64
		gotGoldAmount       sql.NullInt64
		gotOwnerLogin       string
		gotOwnerCharacterID int64
		gotOwnerVID         int64
		gotOwnerName        string
		gotMapIndex         int64
		gotX                int
		gotY                int
		gotZ                int
		gotPickupRange      int64
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT vid, vnum, item_count, gold_amount, owner_login, owner_character_id, owner_vid, owner_name, map_index, x, y, z, pickup_range
FROM bootstrap_ground_items WHERE vid = ?`,
		vid).Scan(
		&gotVID, &gotVnum, &gotItemCount, &gotGoldAmount, &gotOwnerLogin, &gotOwnerCharacterID, &gotOwnerVID, &gotOwnerName,
		&gotMapIndex, &gotX, &gotY, &gotZ, &gotPickupRange,
	); err != nil {
		t.Fatalf("select ground item vid %d: %v", vid, err)
	}
	if gotVID != int64(vid) || gotVnum != int64(vnum) || gotOwnerLogin != ownerLogin || gotOwnerCharacterID != int64(ownerCharacterID) ||
		gotOwnerVID != int64(ownerVID) || gotOwnerName != ownerName || gotMapIndex != int64(mapIndex) ||
		gotX != int(x) || gotY != int(y) || gotZ != int(z) || gotPickupRange != pickupRange {
		t.Fatalf("ground row mismatch for vid %d: got vid=%d vnum=%d login=%q character=%d owner_vid=%d name=%q map=%d x=%d y=%d z=%d pickup=%d",
			vid, gotVID, gotVnum, gotOwnerLogin, gotOwnerCharacterID, gotOwnerVID, gotOwnerName, gotMapIndex, gotX, gotY, gotZ, gotPickupRange)
	}
	if gotItemCount.Valid != itemCount.Valid || (itemCount.Valid && gotItemCount.Int64 != itemCount.Int64) {
		t.Fatalf("ground vid %d item_count = %+v, want %+v", vid, gotItemCount, itemCount)
	}
	if gotGoldAmount.Valid != goldAmount.Valid || (goldAmount.Valid && gotGoldAmount.Int64 != goldAmount.Int64) {
		t.Fatalf("ground vid %d gold_amount = %+v, want %+v", vid, gotGoldAmount, goldAmount)
	}
}

func groundItemStateImportCharacter(id uint32, name string) loginticket.Character {
	return loginticket.Character{
		ID:       id,
		Name:     name,
		Job:      0,
		RaceNum:  0,
		Level:    1,
		X:        100,
		Y:        200,
		MapIndex: 1,
		Empire:   1,
		Gold:     0,
	}
}

func openSQLiteGroundItemStateImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ground-item-state-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite ground-item-state import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
