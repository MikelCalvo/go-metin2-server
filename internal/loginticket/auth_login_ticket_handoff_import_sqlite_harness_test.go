//go:build sqlite_harness

package loginticket

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestSQLiteHarnessAuthLoginTicketHandoffImportInsertsTickets(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	issuedAlpha := time.Date(2026, 8, 12, 9, 30, 0, 123456789, time.UTC)
	issuedZeta := time.Date(2026, 8, 12, 10, 45, 0, 0, time.UTC)
	consumedZeta := issuedZeta.Add(2 * time.Minute)

	export, err := ExportAuthLoginTicketHandoff([]Ticket{
		{
			Login:    "Alpha",
			LoginKey: 0x01020304,
			Empire:   1,
			IssuedAt: issuedAlpha,
			Characters: []Character{{
				ID:       7,
				Name:     "AlphaWar",
				Level:    1,
				MapIndex: 1,
				Inventory: []inventory.ItemInstance{
					{ID: 1001, Vnum: 27001, Count: 2, Slot: 8},
				},
			}},
		},
		{
			Login:    "zeta",
			LoginKey: 0x02000000,
			Empire:   3,
			IssuedAt: issuedZeta,
		},
	})
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff: %v", err)
	}

	// Quarantine accepts optional consumed_at rows even though the live JSON
	// store removes tickets on consume; prove the importer binds NULL vs TEXT.
	export.Tickets[1].ConsumedAt = &consumedZeta

	result, err := ImportAuthLoginTicketHandoff(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportAuthLoginTicketHandoff: %v", err)
	}
	if result.MigrationVersion != AuthLoginTicketHandoffMigrationVersion || result.MigrationName != AuthLoginTicketHandoffMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.TicketCount != 2 || result.ActiveTicketCount != 1 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.LoginKeys) != 2 || result.LoginKeys[0] != 0x01020304 || result.LoginKeys[1] != 0x02000000 {
		t.Fatalf("unexpected login keys: %+v", result.LoginKeys)
	}

	var ticketRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets`).Scan(&ticketRows); err != nil {
		t.Fatalf("count auth login tickets: %v", err)
	}
	if ticketRows != 2 {
		t.Fatalf("auth_login_tickets rows = %d, want 2", ticketRows)
	}

	assertAuthLoginTicketRow(t, db, export.Tickets[0])
	assertAuthLoginTicketRow(t, db, export.Tickets[1])
}

func TestSQLiteHarnessAuthLoginTicketHandoffImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	export, err := ExportAuthLoginTicketHandoff([]Ticket{
		{
			Login:    "Alpha",
			LoginKey: 0x01020304,
			Empire:   1,
			IssuedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC),
			Characters: []Character{{
				ID:       7,
				Name:     "AlphaWar",
				Level:    1,
				MapIndex: 1,
			}},
		},
	})
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff: %v", err)
	}
	if _, err := ImportAuthLoginTicketHandoff(ctx, db, export); err != nil {
		t.Fatalf("first ImportAuthLoginTicketHandoff: %v", err)
	}

	_, err = ImportAuthLoginTicketHandoff(ctx, db, export)
	if err == nil {
		t.Fatal("second ImportAuthLoginTicketHandoff succeeded, want unique conflict")
	}

	var ticketRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets`).Scan(&ticketRows); err != nil {
		t.Fatalf("count tickets after failed reimport: %v", err)
	}
	if ticketRows != 1 {
		t.Fatalf("ticket rows after failed reimport = %d, want 1 (no partial second import)", ticketRows)
	}
}

func TestSQLiteHarnessAuthLoginTicketHandoffImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          []AuthLoginTicketHandoffRow{},
	}

	_, err := ImportAuthLoginTicketHandoff(ctx, db, export)
	if !errors.Is(err, ErrAuthLoginTicketHandoffImportSchemaRequired) {
		t.Fatalf("ImportAuthLoginTicketHandoff on empty DB error = %v, want %v", err, ErrAuthLoginTicketHandoffImportSchemaRequired)
	}
}

func TestSQLiteHarnessAuthLoginTicketHandoffImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          []AuthLoginTicketHandoffRow{},
	}
	result, err := ImportAuthLoginTicketHandoff(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportAuthLoginTicketHandoff(empty): %v", err)
	}
	if result.TicketCount != 0 || result.ActiveTicketCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
	if len(result.LoginKeys) != 0 {
		t.Fatalf("empty import login keys = %+v, want empty", result.LoginKeys)
	}
}

func TestSQLiteHarnessAuthLoginTicketHandoffImportReplaceReimportsSameExport(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	issuedAlpha := time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)
	issuedConsumed := issuedAlpha.Add(-time.Hour)
	consumedAt := issuedAlpha.Add(-30 * time.Minute)
	export, err := ExportAuthLoginTicketHandoff([]Ticket{
		{
			Login:    "Alpha",
			LoginKey: 0x01020304,
			Empire:   1,
			IssuedAt: issuedAlpha,
			Characters: []Character{{
				ID:       7,
				Name:     "AlphaWar",
				Level:    1,
				MapIndex: 1,
			}},
		},
	})
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff: %v", err)
	}
	export.Tickets = append(export.Tickets, AuthLoginTicketHandoffRow{
		LoginKey:               0x01020304,
		IssuedAt:               issuedConsumed,
		Login:                  "Alpha",
		LoginNormalized:        "alpha",
		Empire:                 1,
		ConsumedAt:             &consumedAt,
		CharactersSnapshotJSON: `[]`,
	})

	if _, err := ImportAuthLoginTicketHandoff(ctx, db, export); err != nil {
		t.Fatalf("first insert-only ImportAuthLoginTicketHandoff: %v", err)
	}
	if _, err := ImportAuthLoginTicketHandoff(ctx, db, export); err == nil {
		t.Fatal("second insert-only ImportAuthLoginTicketHandoff succeeded, want unique conflict")
	}

	replacedExport := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets: []AuthLoginTicketHandoffRow{
			{
				LoginKey:               0x01020304,
				IssuedAt:               issuedAlpha.Add(time.Minute),
				Login:                  "Alpha",
				LoginNormalized:        "alpha",
				Empire:                 2,
				CharactersSnapshotJSON: `[{"id":7,"name":"AlphaWar","level":2,"map_index":1,"inventory":[],"equipment":[],"quickslots":[]}]`,
			},
		},
	}
	result, err := ImportAuthLoginTicketHandoff(ctx, db, replacedExport, ImportAuthLoginTicketHandoffOptions{Replace: true})
	if err != nil {
		t.Fatalf("replace ImportAuthLoginTicketHandoff: %v", err)
	}
	if !result.Replaced {
		t.Fatalf("replace result.Replaced = false, want true")
	}
	if result.TicketCount != 1 || result.ActiveTicketCount != 1 {
		t.Fatalf("unexpected replace counts: %+v", result)
	}

	var ticketRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets WHERE login_key = ?`, int64(0x01020304)).Scan(&ticketRows); err != nil {
		t.Fatalf("count tickets after replace: %v", err)
	}
	if ticketRows != 1 {
		t.Fatalf("ticket rows after replace = %d, want 1 (history wiped)", ticketRows)
	}
	assertAuthLoginTicketRow(t, db, replacedExport.Tickets[0])
}

func TestSQLiteHarnessAuthLoginTicketHandoffImportReplaceLeavesUnlistedLoginKeysUntouched(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	issuedAlpha := time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)
	issuedBravo := issuedAlpha.Add(time.Hour)
	fullExport, err := ExportAuthLoginTicketHandoff([]Ticket{
		{
			Login:    "Alpha",
			LoginKey: 0x01020304,
			Empire:   1,
			IssuedAt: issuedAlpha,
			Characters: []Character{{
				ID:       7,
				Name:     "AlphaWar",
				Level:    1,
				MapIndex: 1,
			}},
		},
		{
			Login:    "Bravo",
			LoginKey: 0x02000000,
			Empire:   2,
			IssuedAt: issuedBravo,
		},
	})
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff: %v", err)
	}
	if _, err := ImportAuthLoginTicketHandoff(ctx, db, fullExport); err != nil {
		t.Fatalf("seed ImportAuthLoginTicketHandoff: %v", err)
	}

	alphaOnly := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		LoginKeys:        []uint32{0x01020304},
		Tickets: []AuthLoginTicketHandoffRow{
			{
				LoginKey:               0x01020304,
				IssuedAt:               issuedAlpha.Add(2 * time.Minute),
				Login:                  "Alpha",
				LoginNormalized:        "alpha",
				Empire:                 3,
				CharactersSnapshotJSON: `[]`,
			},
		},
	}
	result, err := ImportAuthLoginTicketHandoff(ctx, db, alphaOnly, ImportAuthLoginTicketHandoffOptions{Replace: true})
	if err != nil {
		t.Fatalf("scoped replace ImportAuthLoginTicketHandoff: %v", err)
	}
	if !result.Replaced || result.TicketCount != 1 || len(result.LoginKeys) != 1 || result.LoginKeys[0] != 0x01020304 {
		t.Fatalf("unexpected scoped replace result: %+v", result)
	}

	var alphaRows, bravoRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets WHERE login_key = ?`, int64(0x01020304)).Scan(&alphaRows); err != nil {
		t.Fatalf("count alpha tickets: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets WHERE login_key = ?`, int64(0x02000000)).Scan(&bravoRows); err != nil {
		t.Fatalf("count bravo tickets: %v", err)
	}
	if alphaRows != 1 || bravoRows != 1 {
		t.Fatalf("scoped replace left unexpected counts alpha=%d bravo=%d", alphaRows, bravoRows)
	}
	assertAuthLoginTicketRow(t, db, alphaOnly.Tickets[0])
	assertAuthLoginTicketRow(t, db, fullExport.Tickets[1])
}

func TestSQLiteHarnessAuthLoginTicketHandoffImportReplaceWipesListedLoginKeyWithEmptyTickets(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	export, err := ExportAuthLoginTicketHandoff([]Ticket{
		{
			Login:    "Alpha",
			LoginKey: 0x01020304,
			Empire:   1,
			IssuedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC),
			Characters: []Character{{
				ID:       7,
				Name:     "AlphaWar",
				Level:    1,
				MapIndex: 1,
			}},
		},
	})
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff: %v", err)
	}
	if _, err := ImportAuthLoginTicketHandoff(ctx, db, export); err != nil {
		t.Fatalf("seed ImportAuthLoginTicketHandoff: %v", err)
	}

	emptyWipe := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		LoginKeys:        []uint32{0x01020304},
		Tickets:          []AuthLoginTicketHandoffRow{},
	}
	result, err := ImportAuthLoginTicketHandoff(ctx, db, emptyWipe, ImportAuthLoginTicketHandoffOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty wipe replace: %v", err)
	}
	if !result.Replaced || result.TicketCount != 0 || len(result.LoginKeys) != 1 || result.LoginKeys[0] != 0x01020304 {
		t.Fatalf("unexpected empty wipe result: %+v", result)
	}

	var ticketRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets`).Scan(&ticketRows); err != nil {
		t.Fatalf("count tickets after wipe: %v", err)
	}
	if ticketRows != 0 {
		t.Fatalf("ticket rows after empty wipe = %d, want 0", ticketRows)
	}
}

func TestSQLiteHarnessAuthLoginTicketHandoffImportReplaceNoOpForEmptyLoginKeys(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	seedExport, err := ExportAuthLoginTicketHandoff([]Ticket{
		{
			Login:    "Alpha",
			LoginKey: 0x01020304,
			Empire:   1,
			IssuedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC),
			Characters: []Character{{
				ID:       7,
				Name:     "AlphaWar",
				Level:    1,
				MapIndex: 1,
			}},
		},
	})
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff: %v", err)
	}
	if _, err := ImportAuthLoginTicketHandoff(ctx, db, seedExport); err != nil {
		t.Fatalf("seed ImportAuthLoginTicketHandoff: %v", err)
	}

	emptyKeys := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		LoginKeys:        []uint32{},
		Tickets:          []AuthLoginTicketHandoffRow{},
	}
	result, err := ImportAuthLoginTicketHandoff(ctx, db, emptyKeys, ImportAuthLoginTicketHandoffOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty login_keys replace: %v", err)
	}
	if !result.Replaced || result.TicketCount != 0 || len(result.LoginKeys) != 0 {
		t.Fatalf("unexpected empty login_keys result: %+v", result)
	}

	var ticketRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets`).Scan(&ticketRows); err != nil {
		t.Fatalf("count tickets after no-op replace: %v", err)
	}
	if ticketRows != 1 {
		t.Fatalf("ticket rows after no-op replace = %d, want 1", ticketRows)
	}
}

func assertAuthLoginTicketRow(t *testing.T, db *sql.DB, want AuthLoginTicketHandoffRow) {
	t.Helper()

	var (
		gotLoginKey               int64
		gotIssuedAt               string
		gotLogin                  string
		gotLoginNormalized        string
		gotEmpire                 int64
		gotConsumedAt             sql.NullString
		gotCharactersSnapshotJSON string
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT login_key, issued_at, login, login_normalized, empire, consumed_at, characters_snapshot_json
FROM auth_login_tickets WHERE login_key = ? AND issued_at = ?`,
		int64(want.LoginKey), want.IssuedAt.UTC().Format(time.RFC3339Nano)).Scan(
		&gotLoginKey, &gotIssuedAt, &gotLogin, &gotLoginNormalized, &gotEmpire, &gotConsumedAt, &gotCharactersSnapshotJSON,
	); err != nil {
		t.Fatalf("select auth login ticket login_key=%08x: %v", want.LoginKey, err)
	}

	if gotLoginKey != int64(want.LoginKey) {
		t.Fatalf("login_key = %d, want %d", gotLoginKey, want.LoginKey)
	}
	if gotIssuedAt != want.IssuedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("issued_at = %q, want %q", gotIssuedAt, want.IssuedAt.UTC().Format(time.RFC3339Nano))
	}
	if gotLogin != want.Login || gotLoginNormalized != want.LoginNormalized || gotEmpire != int64(want.Empire) {
		t.Fatalf("ticket identity mismatch: login=%q norm=%q empire=%d want (%q,%q,%d)",
			gotLogin, gotLoginNormalized, gotEmpire, want.Login, want.LoginNormalized, want.Empire)
	}
	if want.ConsumedAt == nil {
		if gotConsumedAt.Valid {
			t.Fatalf("consumed_at = %q, want NULL", gotConsumedAt.String)
		}
	} else {
		wantConsumed := want.ConsumedAt.UTC().Format(time.RFC3339Nano)
		if !gotConsumedAt.Valid || gotConsumedAt.String != wantConsumed {
			t.Fatalf("consumed_at = %#v, want %q", gotConsumedAt, wantConsumed)
		}
	}
	if gotCharactersSnapshotJSON != want.CharactersSnapshotJSON {
		t.Fatalf("characters_snapshot_json mismatch:\n got: %s\nwant: %s", gotCharactersSnapshotJSON, want.CharactersSnapshotJSON)
	}
}

func openSQLiteAuthLoginTicketHandoffImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "auth-login-ticket-handoff-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite auth login-ticket handoff import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
