package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	ErrMigrationApplyExecutorRequired     = errors.New("migration apply executor is required")
	ErrMigrationApplyUnsupportedDirection = errors.New("migration apply direction is unsupported")
)

// SQLMigrationExecutor is the narrow database/sql-compatible transaction
// boundary required by the first migration apply primitive. *sql.DB and
// *sql.Conn satisfy it without letting the migration package own driver
// selection, DSN loading, connection pools, or daemon startup policy.
type SQLMigrationExecutor interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// ApplyResult reports the metadata-only effect of an apply attempt. It exposes
// the applied plan steps and version boundary, but deliberately omits executable
// SQL text, DSNs, connection details, and row data.
type ApplyResult struct {
	PreviousVersion int        `json:"previous_version"`
	CurrentVersion  int        `json:"current_version"`
	LatestVersion   int        `json:"latest_version"`
	Applied         []PlanStep `json:"applied"`
}

// ApplyUpToLatest validates the embedded catalog against the supplied applied
// ledger and executes only pending up migrations to the catalog's latest version
// in a single database transaction.
//
// This is intentionally a small programmatic primitive, not a daemon endpoint or
// production CLI. Callers still own DB driver selection, DSN loading, concurrency
// policy, and how the current ledger was obtained.
func ApplyUpToLatest(ctx context.Context, executor SQLMigrationExecutor, ledger []LedgerEntry) (ApplyResult, error) {
	catalog, err := Catalog()
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyCatalogUpToLatest(ctx, executor, catalog, ledger)
}

// ApplyCatalogUpToLatest is the catalog-injected variant of ApplyUpToLatest.
func ApplyCatalogUpToLatest(ctx context.Context, executor SQLMigrationExecutor, catalog []Migration, ledger []LedgerEntry) (ApplyResult, error) {
	return ApplyCatalogUpToVersion(ctx, executor, catalog, ledger, len(catalog))
}

// ApplyToVersion validates the embedded catalog against the supplied ledger and
// executes pending up migrations to targetVersion. Rollback/down execution is
// deliberately not implemented in this first apply slice: targets below the
// current ledger version fail closed before a transaction is opened.
func ApplyToVersion(ctx context.Context, executor SQLMigrationExecutor, ledger []LedgerEntry, targetVersion int) (ApplyResult, error) {
	catalog, err := Catalog()
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyCatalogUpToVersion(ctx, executor, catalog, ledger, targetVersion)
}

// ApplyCatalogUpToVersion validates a catalog/ledger pair, rejects rollback
// plans, and executes pending up migrations plus their schema_migrations ledger
// rows inside one transaction. Each migration SQL body is executed before its
// corresponding ledger insert; any migration or ledger failure rolls the entire
// batch back.
func ApplyCatalogUpToVersion(ctx context.Context, executor SQLMigrationExecutor, catalog []Migration, ledger []LedgerEntry, targetVersion int) (ApplyResult, error) {
	if migrationExecutorIsNil(executor) {
		return ApplyResult{}, ErrMigrationApplyExecutorRequired
	}

	plan, err := PlanCatalogToVersion(catalog, ledger, targetVersion)
	if err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{
		PreviousVersion: plan.CurrentVersion,
		CurrentVersion:  plan.CurrentVersion,
		LatestVersion:   plan.LatestVersion,
		Applied:         []PlanStep{},
	}
	if plan.UpToDate || len(plan.Pending) == 0 {
		return result, nil
	}
	for _, step := range plan.Pending {
		if step.Direction != DirectionUp {
			return ApplyResult{}, fmt.Errorf("%w: cannot execute %s step for migration %04d", ErrMigrationApplyUnsupportedDirection, step.Direction, step.Version)
		}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("begin migration apply transaction: %w", err)
	}

	for _, step := range plan.Pending {
		migration := catalog[step.Version-1]
		if _, err := tx.ExecContext(ctx, migration.UpSQL); err != nil {
			return ApplyResult{}, rollbackAfterApplyFailure(tx, fmt.Errorf("execute migration %04d %s: %w", migration.Version, migration.Name, err))
		}
		if _, err := tx.ExecContext(ctx, schemaMigrationLedgerInsertSQL(migration)); err != nil {
			return ApplyResult{}, rollbackAfterApplyFailure(tx, fmt.Errorf("record schema_migrations ledger row for migration %04d %s: %w", migration.Version, migration.Name, err))
		}
		result.Applied = append(result.Applied, step)
		result.CurrentVersion = step.Version
	}

	if err := tx.Commit(); err != nil {
		return ApplyResult{}, fmt.Errorf("commit migration apply transaction: %w", err)
	}
	return result, nil
}

func rollbackAfterApplyFailure(tx *sql.Tx, applyErr error) error {
	if tx == nil {
		return applyErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(applyErr, fmt.Errorf("rollback migration apply transaction: %w", rollbackErr))
	}
	return applyErr
}

func migrationExecutorIsNil(executor SQLMigrationExecutor) bool {
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

func schemaMigrationLedgerInsertSQL(migration Migration) string {
	return fmt.Sprintf("INSERT INTO schema_migrations (version, name, up_sha256)\nVALUES (%d, %s, %s);", migration.Version, quoteSQLText(migration.Name), quoteSQLText(migration.UpSHA256))
}

func quoteSQLText(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
