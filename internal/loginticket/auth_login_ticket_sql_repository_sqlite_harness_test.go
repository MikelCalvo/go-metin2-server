//go:build sqlite_harness

package loginticket

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestSQLiteHarnessSQLRepositoryIssueLoadExportRoundTrip(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	alpha := sampleAuthLoginTicket()
	zeta := Ticket{
		Login:    "zeta",
		LoginKey: 0x02000000,
		Empire:   3,
		IssuedAt: time.Date(2026, 8, 12, 10, 45, 0, 0, time.UTC),
	}
	repo := NewSQLRepository(db)
	if err := repo.Issue(zeta); err != nil {
		t.Fatalf("Issue(zeta): %v", err)
	}
	if err := repo.Issue(alpha); err != nil {
		t.Fatalf("Issue(alpha): %v", err)
	}
	if err := repo.Issue(alpha); !errors.Is(err, ErrTicketExists) {
		t.Fatalf("duplicate Issue error = %v, want %v", err, ErrTicketExists)
	}

	loaded, err := repo.Load("Alpha", alpha.LoginKey)
	if err != nil {
		t.Fatalf("Load(Alpha): %v", err)
	}
	if !reflect.DeepEqual(loaded, normalizedSampleTicket(alpha)) {
		t.Fatalf("loaded Alpha:\n got %#v\nwant %#v", loaded, normalizedSampleTicket(alpha))
	}
	loaded.Characters[0].Inventory[0].Count = 99
	reloaded, err := repo.Load("Alpha", alpha.LoginKey)
	if err != nil {
		t.Fatalf("reload Alpha: %v", err)
	}
	if reloaded.Characters[0].Inventory[0].Count != 2 {
		t.Fatalf("SQL repository leaked caller mutation: %#v", reloaded.Characters[0].Inventory[0])
	}
	if _, err := repo.Load("Bravo", alpha.LoginKey); !errors.Is(err, ErrTicketLoginMismatch) {
		t.Fatalf("Load(mismatched login) error = %v, want %v", err, ErrTicketLoginMismatch)
	}
	if _, err := repo.Load("missing", 0x00abcdef); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("Load(missing) error = %v, want %v", err, ErrTicketNotFound)
	}

	export, err := repo.ExportAuthLoginTicketHandoff()
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff: %v", err)
	}
	wantExport, err := ExportAuthLoginTicketHandoff([]Ticket{alpha, zeta})
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff(memory): %v", err)
	}
	if !reflect.DeepEqual(export, wantExport) {
		t.Fatalf("SQL export:\n got %#v\nwant %#v", export, wantExport)
	}
	if export.MigrationVersion != AuthLoginTicketHandoffMigrationVersion || export.MigrationName != AuthLoginTicketHandoffMigrationName {
		t.Fatalf("export identity = %d %q, want tip-0007", export.MigrationVersion, export.MigrationName)
	}
}

func TestSQLiteHarnessSQLRepositoryConsumeLeavesRowInPlace(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	alpha := sampleAuthLoginTicket()
	repo := NewSQLRepository(db)
	if err := repo.Issue(alpha); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	consumed, err := repo.Consume("Alpha", alpha.LoginKey)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if !reflect.DeepEqual(consumed, normalizedSampleTicket(alpha)) {
		t.Fatalf("consumed ticket:\n got %#v\nwant %#v", consumed, normalizedSampleTicket(alpha))
	}

	var (
		ticketRows  int
		activeRows  int
		consumedSet int
	)
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets`).Scan(&ticketRows); err != nil {
		t.Fatalf("count tickets after consume: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets WHERE consumed_at IS NULL`).Scan(&activeRows); err != nil {
		t.Fatalf("count active tickets after consume: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_login_tickets WHERE consumed_at IS NOT NULL`).Scan(&consumedSet); err != nil {
		t.Fatalf("count consumed tickets after consume: %v", err)
	}
	if ticketRows != 1 || activeRows != 1 || consumedSet != 0 {
		t.Fatalf("after SQL Consume rows=%d active=%d consumed=%d, want 1/1/0 (no SQL consume update)", ticketRows, activeRows, consumedSet)
	}
	if _, err := repo.Load("Alpha", alpha.LoginKey); err != nil {
		t.Fatalf("Load after non-destructive SQL Consume: %v", err)
	}
}

func TestSQLiteHarnessSQLRepositoryExportOmitsConsumedHistory(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AuthLoginTicketHandoffMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AuthLoginTicketHandoffMigrationVersion, err)
	}

	alpha := sampleAuthLoginTicket()
	repo := NewSQLRepository(db)
	if err := repo.Issue(alpha); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	consumedAt := alpha.IssuedAt.Add(time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `UPDATE auth_login_tickets SET consumed_at = ? WHERE login_key = ?`, consumedAt, int64(alpha.LoginKey)); err != nil {
		t.Fatalf("mark historical consumed row: %v", err)
	}

	if _, err := repo.Load("Alpha", alpha.LoginKey); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("Load(consumed history) error = %v, want %v", err, ErrTicketNotFound)
	}
	export, err := repo.ExportAuthLoginTicketHandoff()
	if err != nil {
		t.Fatalf("Export consumed-only table: %v", err)
	}
	if export.MigrationVersion != AuthLoginTicketHandoffMigrationVersion || len(export.Tickets) != 0 {
		t.Fatalf("consumed history leaked into pending export: %#v", export)
	}
}

func TestSQLiteHarnessSQLRepositoryRejectsMissingSchema(t *testing.T) {
	db := openSQLiteAuthLoginTicketHandoffImportDB(t)
	defer db.Close()

	repo := NewSQLRepository(db)
	alpha := sampleAuthLoginTicket()
	if err := repo.Issue(alpha); !errors.Is(err, ErrAuthLoginTicketHandoffImportSchemaRequired) {
		t.Fatalf("Issue empty-ledger error = %v, want %v", err, ErrAuthLoginTicketHandoffImportSchemaRequired)
	}
	if _, err := repo.Load("Alpha", alpha.LoginKey); !errors.Is(err, ErrAuthLoginTicketHandoffImportSchemaRequired) {
		t.Fatalf("Load empty-ledger error = %v, want %v", err, ErrAuthLoginTicketHandoffImportSchemaRequired)
	}
	if _, err := repo.ExportAuthLoginTicketHandoff(); !errors.Is(err, ErrAuthLoginTicketHandoffImportSchemaRequired) {
		t.Fatalf("Export empty-ledger error = %v, want %v", err, ErrAuthLoginTicketHandoffImportSchemaRequired)
	}
}

func normalizedSampleTicket(ticket Ticket) Ticket {
	cloned := cloneTicket(ticket)
	normalizeCharactersItemState(cloned.Characters)
	cloned.IssuedAt = cloned.IssuedAt.UTC()
	return cloned
}
