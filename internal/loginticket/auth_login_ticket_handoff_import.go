package loginticket

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"time"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrAuthLoginTicketHandoffImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrAuthLoginTicketHandoffImportExecutorRequired = errors.New("auth login-ticket handoff import executor is required")

// ErrAuthLoginTicketHandoffImportSchemaRequired reports that the target database
// has not applied the 0007_auth_login_ticket_handoff migration boundary yet.
var ErrAuthLoginTicketHandoffImportSchemaRequired = errors.New("auth login-ticket handoff schema is not applied")

// ErrAuthLoginTicketHandoffImportRowCount reports that an INSERT affected an
// unexpected number of rows during login-ticket handoff backfill.
var ErrAuthLoginTicketHandoffImportRowCount = errors.New("auth login-ticket handoff import row count mismatch")

// AuthLoginTicketHandoffImportResult is the metadata-only outcome of importing a
// quarantined 0007 login-ticket handoff export. It never includes ticket
// payloads, SQL text, DSNs, or login-ticket snapshot bytes.
type AuthLoginTicketHandoffImportResult struct {
	MigrationVersion  int      `json:"migration_version"`
	MigrationName     string   `json:"migration_name"`
	TicketCount       int      `json:"ticket_count"`
	ActiveTicketCount int      `json:"active_ticket_count"`
	LoginKeys         []uint32 `json:"login_keys"`
	// Replaced is true when ImportAuthLoginTicketHandoff ran with the opt-in
	// scoped replace policy (delete then insert for listed login keys). Omitted
	// from JSON when false so legacy insert-only import-result files stay valid.
	Replaced bool `json:"replaced,omitempty"`
}

// ImportAuthLoginTicketHandoffOptions controls opt-in mutation policy for
// ImportAuthLoginTicketHandoff. The zero value keeps today's insert-only
// behavior.
type ImportAuthLoginTicketHandoffOptions struct {
	// Replace, when true, deletes existing tip-0007 auth_login_tickets rows for
	// every login key in the quarantined export summary before inserting the
	// canonicalized export rows, all inside one transaction. Login keys not
	// listed in the export remain untouched.
	Replace bool
}

// ImportAuthLoginTicketHandoff validates a retained 0007 login-ticket handoff
// export through the existing quarantine contract and inserts the canonicalized
// rows into auth_login_tickets inside one transaction.
//
// The caller still owns driver selection and DSN loading. This primitive does
// not mutate bootstrap file stores, does not rewrite schema_migrations, and
// without options (or with Replace=false) does not invent upsert / merge
// policy: duplicate primary keys or active login-key unique-index collisions
// fail closed and roll the transaction back. Pass
// ImportAuthLoginTicketHandoffOptions{Replace: true} for the opt-in scoped
// replace path frozen by the tip-0007 replace contract.
func ImportAuthLoginTicketHandoff(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export AuthLoginTicketHandoffExport, opts ...ImportAuthLoginTicketHandoffOptions) (AuthLoginTicketHandoffImportResult, error) {
	if authLoginTicketHandoffImportExecutorIsNil(executor) {
		return AuthLoginTicketHandoffImportResult{}, ErrAuthLoginTicketHandoffImportExecutorRequired
	}
	if len(opts) > 1 {
		return AuthLoginTicketHandoffImportResult{}, fmt.Errorf("ImportAuthLoginTicketHandoff accepts at most one options value")
	}
	replace := false
	if len(opts) == 1 {
		replace = opts[0].Replace
	}

	canonical, summary, err := QuarantineAuthLoginTicketHandoffExport(export)
	if err != nil {
		return AuthLoginTicketHandoffImportResult{}, err
	}

	result := AuthLoginTicketHandoffImportResult{
		MigrationVersion:  AuthLoginTicketHandoffMigrationVersion,
		MigrationName:     AuthLoginTicketHandoffMigrationName,
		TicketCount:       summary.TicketCount,
		ActiveTicketCount: summary.ActiveTicketCount,
		LoginKeys:         append([]uint32(nil), summary.LoginKeys...),
		Replaced:          replace,
	}
	if result.LoginKeys == nil {
		result.LoginKeys = []uint32{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return AuthLoginTicketHandoffImportResult{}, fmt.Errorf("begin auth login-ticket handoff import transaction: %w", err)
	}

	if err := requireAuthLoginTicketHandoffSchema(ctx, tx); err != nil {
		return AuthLoginTicketHandoffImportResult{}, rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}

	if replace {
		for _, loginKey := range summary.LoginKeys {
			if err := deleteAuthLoginTicketsForLoginKey(ctx, tx, loginKey); err != nil {
				return AuthLoginTicketHandoffImportResult{}, rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
			}
		}
	}

	for _, row := range canonical.Tickets {
		if err := insertAuthLoginTicketHandoff(ctx, tx, row); err != nil {
			return AuthLoginTicketHandoffImportResult{}, rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return AuthLoginTicketHandoffImportResult{}, fmt.Errorf("commit auth login-ticket handoff import transaction: %w", err)
	}
	return result, nil
}

func requireAuthLoginTicketHandoffSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrAuthLoginTicketHandoffImportSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == AuthLoginTicketHandoffMigrationVersion && entry.Name == AuthLoginTicketHandoffMigrationName {
			return nil
		}
	}
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrAuthLoginTicketHandoffImportSchemaRequired, latest, AuthLoginTicketHandoffMigrationVersion, AuthLoginTicketHandoffMigrationName)
}

func deleteAuthLoginTicketsForLoginKey(ctx context.Context, tx *sql.Tx, loginKey uint32) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_login_tickets WHERE login_key = ?`, int64(loginKey)); err != nil {
		return fmt.Errorf("delete auth login tickets login_key=%08x: %w", loginKey, err)
	}
	return nil
}

func insertAuthLoginTicketHandoff(ctx context.Context, tx *sql.Tx, row AuthLoginTicketHandoffRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO auth_login_tickets (
    login_key, issued_at, login, login_normalized, empire, consumed_at, characters_snapshot_json
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		int64(row.LoginKey),
		row.IssuedAt.UTC().Format(time.RFC3339Nano),
		row.Login,
		row.LoginNormalized,
		int64(row.Empire),
		nullableTimeSQL(row.ConsumedAt),
		row.CharactersSnapshotJSON,
	)
	if err != nil {
		return fmt.Errorf("insert auth login ticket login_key=%08x issued_at=%s: %w", row.LoginKey, row.IssuedAt.UTC().Format(time.RFC3339Nano), err)
	}
	return requireExactAuthLoginTicketHandoffImportRows(result, "insert auth login ticket", int64(row.LoginKey))
}

func nullableTimeSQL(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func requireExactAuthLoginTicketHandoffImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrAuthLoginTicketHandoffImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrAuthLoginTicketHandoffImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrAuthLoginTicketHandoffImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterAuthLoginTicketHandoffImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback auth login-ticket handoff import transaction: %w", rollbackErr))
	}
	return importErr
}

func authLoginTicketHandoffImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
	if executor == nil {
		return true
	}
	value := reflect.ValueOf(executor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
