//go:build sqlite_harness

package worldruntime

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
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
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemInstanceAttributesMigrationVersion, err)
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
		{VID: 0x0700002c, Vnum: 3001, Count: count, OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, PickupRange: 450, MapIndex: 1, X: 1100, Y: 2100, Z: 2, HasSockets: true, Socket0: 1, Socket1: 2, Socket2: 3, HasAttributes: true, Attr0Type: 1, Attr0Value: 10, Attr6Type: 7, Attr6Value: -3},
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

	assertGroundItemRow(t, db, 0x0700002c, 3001, sql.NullInt64{Int64: 2, Valid: true}, sql.NullInt64{}, "ground-item-owner", 0x0103019c, 0x0204019c, "GroundItemOwner", 1, 1100, 2100, 2, 450, true, 1, 2, 3, true, 1, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 7, -3)
	assertGroundItemRow(t, db, 0x0700002d, 1, sql.NullInt64{}, sql.NullInt64{Int64: 250, Valid: true}, "ground-gold-owner", 0x0103019d, 0x0204019d, "GroundGoldOwner", 42, 1200, 2200, 3, 750, false, 0, 0, 0, false, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
}

func TestSQLiteHarnessGroundItemStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemInstanceAttributesMigrationVersion, err)
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
		HasSockets: true, Socket0: 4, Socket1: 5, Socket2: 6,
		HasAttributes: true, Attr0Type: 2, Attr0Value: 5,
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
	assertGroundItemRow(t, db, 0x0700002c, 3001, sql.NullInt64{Int64: 1, Valid: true}, sql.NullInt64{}, "ground-item-owner", 0x0103019c, 0x0204019c, "GroundItemOwner", 1, 100, 200, 0, 300, true, 4, 5, 6, true, 2, 5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
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

func TestSQLiteHarnessGroundItemStateImportRejectsTipTenOnlyLedger(t *testing.T) {
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
	_, err := ImportBootstrapGroundItemState(ctx, db, export)
	if !errors.Is(err, ErrBootstrapGroundItemStateImportSchemaRequired) {
		t.Fatalf("ImportBootstrapGroundItemState tip-10-only error = %v, want %v", err, ErrBootstrapGroundItemStateImportSchemaRequired)
	}
	if err == nil || !strings.Contains(err.Error(), "26") || !strings.Contains(err.Error(), BootstrapGroundItemInstanceSocketsMigrationName) {
		t.Fatalf("expected tip-10-only error to name additive 26, got %v", err)
	}
}

func TestSQLiteHarnessGroundItemStateImportRejectsTipTwentySixOnlyLedger(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemInstanceSocketsMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemInstanceSocketsMigrationVersion, err)
	}

	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems:      []BootstrapGroundItemStateRow{},
	}
	_, err := ImportBootstrapGroundItemState(ctx, db, export)
	if !errors.Is(err, ErrBootstrapGroundItemStateImportSchemaRequired) {
		t.Fatalf("ImportBootstrapGroundItemState tip-26-only error = %v, want %v", err, ErrBootstrapGroundItemStateImportSchemaRequired)
	}
	if err == nil || !strings.Contains(err.Error(), "29") || !strings.Contains(err.Error(), BootstrapGroundItemInstanceAttributesMigrationName) {
		t.Fatalf("expected tip-26-only error to name additive 29, got %v", err)
	}
}

func TestSQLiteHarnessGroundItemStateImportRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemInstanceAttributesMigrationVersion, err)
	}

	count := uint16(1)
	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems: []BootstrapGroundItemStateRow{{
			VID: 0x0700002c, Vnum: 3001, ItemCount: &count,
			OwnerLogin: "missing-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
			OwnerName: "MissingOwner", MapIndex: 1, X: 100, Y: 200, Z: 0, PickupRange: 300,
			HasSockets: true, Socket0: 9,
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
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemInstanceAttributesMigrationVersion, err)
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

func TestSQLiteHarnessGroundItemStateImportReplaceOverwritesCanonicalRows(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemInstanceAttributesMigrationVersion, err)
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

	firstExport, err := ExportBootstrapGroundItemState([]GroundItemSnapshot{{
		VID: 0x0700002c, Vnum: 3001, Count: 1,
		OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner",
		OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
		PickupRange: 300, MapIndex: 1, X: 100, Y: 200,
		HasSockets: true, Socket0: 4, Socket1: 5, Socket2: 6,
		HasAttributes: true, Attr0Type: 2, Attr0Value: 5,
	}})
	if err != nil {
		t.Fatalf("ExportBootstrapGroundItemState: %v", err)
	}
	if _, err := ImportBootstrapGroundItemState(ctx, db, firstExport); err != nil {
		t.Fatalf("first insert-only ImportBootstrapGroundItemState: %v", err)
	}
	if _, err := ImportBootstrapGroundItemState(ctx, db, firstExport); err == nil {
		t.Fatal("second insert-only ImportBootstrapGroundItemState succeeded, want unique conflict")
	}

	replacedExport, err := ExportBootstrapGroundItemState([]GroundItemSnapshot{{
		VID: 0x0700002c, Vnum: 3001, Count: 3,
		OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner",
		OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
		PickupRange: 450, MapIndex: 1, X: 111, Y: 222, Z: 1,
		HasSockets: true, Socket0: 7, Socket1: 8, Socket2: 9,
		HasAttributes: true, Attr0Type: 3, Attr0Value: 9,
	}})
	if err != nil {
		t.Fatalf("ExportBootstrapGroundItemState(replaced): %v", err)
	}
	result, err := ImportBootstrapGroundItemState(ctx, db, replacedExport, ImportBootstrapGroundItemStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("replace ImportBootstrapGroundItemState: %v", err)
	}
	if !result.Replaced {
		t.Fatalf("replace result.Replaced = false, want true")
	}
	if result.GroundItemCount != 1 || result.ItemShapedCount != 1 || result.GoldShapedCount != 0 {
		t.Fatalf("unexpected replace counts: %+v", result)
	}

	var rowCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bootstrap_ground_items`).Scan(&rowCount); err != nil {
		t.Fatalf("count ground items after replace: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("ground item rows after replace = %d, want 1", rowCount)
	}
	assertGroundItemRow(t, db, 0x0700002c, 3001, sql.NullInt64{Int64: 3, Valid: true}, sql.NullInt64{}, "ground-item-owner", 0x0103019c, 0x0204019c, "GroundItemOwner", 1, 111, 222, 1, 450, true, 7, 8, 9, true, 3, 9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
}

func TestSQLiteHarnessGroundItemStateImportReplaceLeavesUnlistedVIDsUntouched(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemInstanceAttributesMigrationVersion, err)
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
	fullExport, err := ExportBootstrapGroundItemState([]GroundItemSnapshot{
		{VID: 0x0700002c, Vnum: 3001, Count: count, OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, PickupRange: 450, MapIndex: 1, X: 1100, Y: 2100, Z: 2},
		{VID: 0x0700002d, Vnum: 1, GoldAmount: gold, OwnerName: "GroundGoldOwner", OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, PickupRange: 750, MapIndex: 42, X: 1200, Y: 2200, Z: 3},
	})
	if err != nil {
		t.Fatalf("ExportBootstrapGroundItemState: %v", err)
	}
	if _, err := ImportBootstrapGroundItemState(ctx, db, fullExport); err != nil {
		t.Fatalf("seed ImportBootstrapGroundItemState: %v", err)
	}

	newCount := uint16(9)
	itemOnly := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		VIDs:             []uint32{0x0700002c},
		GroundItems: []BootstrapGroundItemStateRow{{
			VID: 0x0700002c, Vnum: 3001, ItemCount: &newCount,
			OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
			OwnerName: "GroundItemOwner", MapIndex: 1, X: 1300, Y: 2300, Z: 4, PickupRange: 500,
		}},
	}
	result, err := ImportBootstrapGroundItemState(ctx, db, itemOnly, ImportBootstrapGroundItemStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("scoped replace ImportBootstrapGroundItemState: %v", err)
	}
	if !result.Replaced || result.GroundItemCount != 1 || result.ItemShapedCount != 1 {
		t.Fatalf("unexpected scoped replace result: %+v", result)
	}

	var itemRows, goldRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bootstrap_ground_items WHERE vid = 117440556`).Scan(&itemRows); err != nil {
		t.Fatalf("count item vid rows: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bootstrap_ground_items WHERE vid = 117440557`).Scan(&goldRows); err != nil {
		t.Fatalf("count gold vid rows: %v", err)
	}
	if itemRows != 1 || goldRows != 1 {
		t.Fatalf("scoped replace left unexpected counts item=%d gold=%d", itemRows, goldRows)
	}
	assertGroundItemRow(t, db, 0x0700002c, 3001, sql.NullInt64{Int64: 9, Valid: true}, sql.NullInt64{}, "ground-item-owner", 0x0103019c, 0x0204019c, "GroundItemOwner", 1, 1300, 2300, 4, 500, false, 0, 0, 0, false, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	assertGroundItemRow(t, db, 0x0700002d, 1, sql.NullInt64{}, sql.NullInt64{Int64: 250, Valid: true}, "ground-gold-owner", 0x0103019d, 0x0204019d, "GroundGoldOwner", 42, 1200, 2200, 3, 750, false, 0, 0, 0, false, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
}

func TestSQLiteHarnessGroundItemStateImportReplaceWipesListedVIDWithEmptyRows(t *testing.T) {
	db := openSQLiteGroundItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, BootstrapGroundItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", BootstrapGroundItemInstanceAttributesMigrationVersion, err)
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

	seedExport, err := ExportBootstrapGroundItemState([]GroundItemSnapshot{{
		VID: 0x0700002c, Vnum: 3001, Count: 1,
		OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner",
		OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
		PickupRange: 300, MapIndex: 1, X: 100, Y: 200,
	}})
	if err != nil {
		t.Fatalf("ExportBootstrapGroundItemState: %v", err)
	}
	if _, err := ImportBootstrapGroundItemState(ctx, db, seedExport); err != nil {
		t.Fatalf("seed ImportBootstrapGroundItemState: %v", err)
	}

	emptyWipe := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		VIDs:             []uint32{0x0700002c},
		GroundItems:      []BootstrapGroundItemStateRow{},
	}
	result, err := ImportBootstrapGroundItemState(ctx, db, emptyWipe, ImportBootstrapGroundItemStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty wipe replace: %v", err)
	}
	if !result.Replaced || result.GroundItemCount != 0 || len(result.VIDs) != 1 || result.VIDs[0] != 0x0700002c {
		t.Fatalf("unexpected empty wipe result: %+v", result)
	}

	var rowCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bootstrap_ground_items`).Scan(&rowCount); err != nil {
		t.Fatalf("count ground items after wipe: %v", err)
	}
	if rowCount != 0 {
		t.Fatalf("ground item rows after empty wipe = %d, want 0", rowCount)
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
	hasSockets bool,
	socket0, socket1, socket2 int32,
	hasAttributes bool,
	attr0Type uint8, attr0Value int16,
	attr1Type uint8, attr1Value int16,
	attr2Type uint8, attr2Value int16,
	attr3Type uint8, attr3Value int16,
	attr4Type uint8, attr4Value int16,
	attr5Type uint8, attr5Value int16,
	attr6Type uint8, attr6Value int16,
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
		gotHasSockets       int
		gotSocket0          int
		gotSocket1          int
		gotSocket2          int
		gotHasAttributes    int
		gotAttr0Type        int
		gotAttr0Value       int
		gotAttr1Type        int
		gotAttr1Value       int
		gotAttr2Type        int
		gotAttr2Value       int
		gotAttr3Type        int
		gotAttr3Value       int
		gotAttr4Type        int
		gotAttr4Value       int
		gotAttr5Type        int
		gotAttr5Value       int
		gotAttr6Type        int
		gotAttr6Value       int
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT vid, vnum, item_count, gold_amount, owner_login, owner_character_id, owner_vid, owner_name, map_index, x, y, z, pickup_range, has_sockets, socket0, socket1, socket2, has_attributes, attr0_type, attr0_value, attr1_type, attr1_value, attr2_type, attr2_value, attr3_type, attr3_value, attr4_type, attr4_value, attr5_type, attr5_value, attr6_type, attr6_value
FROM bootstrap_ground_items WHERE vid = ?`,
		vid).Scan(
		&gotVID, &gotVnum, &gotItemCount, &gotGoldAmount, &gotOwnerLogin, &gotOwnerCharacterID, &gotOwnerVID, &gotOwnerName,
		&gotMapIndex, &gotX, &gotY, &gotZ, &gotPickupRange, &gotHasSockets, &gotSocket0, &gotSocket1, &gotSocket2,
		&gotHasAttributes, &gotAttr0Type, &gotAttr0Value, &gotAttr1Type, &gotAttr1Value, &gotAttr2Type, &gotAttr2Value,
		&gotAttr3Type, &gotAttr3Value, &gotAttr4Type, &gotAttr4Value, &gotAttr5Type, &gotAttr5Value, &gotAttr6Type, &gotAttr6Value,
	); err != nil {
		t.Fatalf("select ground item vid %d: %v", vid, err)
	}
	wantHasSockets := 0
	if hasSockets {
		wantHasSockets = 1
	}
	wantHasAttributes := 0
	if hasAttributes {
		wantHasAttributes = 1
	}
	if gotVID != int64(vid) || gotVnum != int64(vnum) || gotOwnerLogin != ownerLogin || gotOwnerCharacterID != int64(ownerCharacterID) ||
		gotOwnerVID != int64(ownerVID) || gotOwnerName != ownerName || gotMapIndex != int64(mapIndex) ||
		gotX != int(x) || gotY != int(y) || gotZ != int(z) || gotPickupRange != pickupRange ||
		gotHasSockets != wantHasSockets || gotSocket0 != int(socket0) || gotSocket1 != int(socket1) || gotSocket2 != int(socket2) ||
		gotHasAttributes != wantHasAttributes ||
		gotAttr0Type != int(attr0Type) || gotAttr0Value != int(attr0Value) ||
		gotAttr1Type != int(attr1Type) || gotAttr1Value != int(attr1Value) ||
		gotAttr2Type != int(attr2Type) || gotAttr2Value != int(attr2Value) ||
		gotAttr3Type != int(attr3Type) || gotAttr3Value != int(attr3Value) ||
		gotAttr4Type != int(attr4Type) || gotAttr4Value != int(attr4Value) ||
		gotAttr5Type != int(attr5Type) || gotAttr5Value != int(attr5Value) ||
		gotAttr6Type != int(attr6Type) || gotAttr6Value != int(attr6Value) {
		t.Fatalf("ground row mismatch for vid %d: got vid=%d vnum=%d login=%q character=%d owner_vid=%d name=%q map=%d x=%d y=%d z=%d pickup=%d has_sockets=%d sockets=(%d,%d,%d) has_attributes=%d attrs=[(%d,%d) (%d,%d) (%d,%d) (%d,%d) (%d,%d) (%d,%d) (%d,%d)]",
			vid, gotVID, gotVnum, gotOwnerLogin, gotOwnerCharacterID, gotOwnerVID, gotOwnerName, gotMapIndex, gotX, gotY, gotZ, gotPickupRange, gotHasSockets, gotSocket0, gotSocket1, gotSocket2, gotHasAttributes,
			gotAttr0Type, gotAttr0Value, gotAttr1Type, gotAttr1Value, gotAttr2Type, gotAttr2Value, gotAttr3Type, gotAttr3Value, gotAttr4Type, gotAttr4Value, gotAttr5Type, gotAttr5Value, gotAttr6Type, gotAttr6Value)
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
