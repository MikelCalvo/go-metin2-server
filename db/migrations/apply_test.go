package migrations

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var (
	errStubMigrationBegin    = errors.New("stub migration begin failed")
	errStubMigrationExec     = errors.New("stub migration exec failed")
	errStubMigrationQuery    = errors.New("stub migration query failed")
	errStubMigrationCommit   = errors.New("stub migration commit failed")
	errStubMigrationRollback = errors.New("stub migration rollback failed")
	errStubMigrationRows     = errors.New("stub migration rows affected failed")
)

func TestApplyCatalogUpToVersionExecutesPendingMigrationsAndRecordsLedgerInOneTransaction(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)
	scenario := &testMigrationApplySQLScenario{
		currentLedger: nil,
	}

	result, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 2)
	if err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if result.PreviousVersion != 0 || result.CurrentVersion != 2 || result.LatestVersion != 2 {
		t.Fatalf("unexpected apply result versions: %#v", result)
	}
	if len(result.Applied) != 2 || result.Applied[0].Version != 1 || result.Applied[1].Version != 2 {
		t.Fatalf("unexpected applied steps: %#v", result.Applied)
	}

	want := append([]string{"begin"}, execEventsForMigrationSQL(t, catalog[0].UpSQL)...)
	want = append(want,
		"exec:"+schemaMigrationLedgerInsertSQL(catalog[0]),
	)
	want = append(want, execEventsForMigrationSQL(t, catalog[1].UpSQL)...)
	want = append(want,
		"exec:"+schemaMigrationLedgerInsertSQL(catalog[1]),
		"query:"+SchemaMigrationsLedgerQuery,
		"commit",
	)
	if got := scenario.eventsSnapshot(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected execution order:\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestApplyCatalogUpToVersionExecutesEachMigrationStatementBeforeLedgerWrite(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL: "-- go-metin2 migration: 0002 accounts up\n" +
				"CREATE TABLE accounts (login TEXT PRIMARY KEY CHECK (login <> 'semi;colon'));\n" +
				"CREATE INDEX accounts_login_index ON accounts (login);\n",
			downSQL: "-- go-metin2 migration: 0002 accounts down\n" +
				"DROP INDEX accounts_login_index;\n" +
				"DROP TABLE accounts;\n",
		},
	)
	scenario := &testMigrationApplySQLScenario{}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 2)
	if err != nil {
		t.Fatalf("apply multi-statement migration: %v", err)
	}

	got := scenario.eventsSnapshot()
	upStatements := execEventsForMigrationSQL(t, catalog[1].UpSQL)
	ledgerEvent := "exec:" + schemaMigrationLedgerInsertSQL(catalog[1])
	for _, want := range upStatements {
		if !containsEvent(got, want) {
			t.Fatalf("expected statement event %q, got %#v", want, got)
		}
	}
	ledgerIndex := eventIndex(got, ledgerEvent)
	if ledgerIndex < 0 {
		t.Fatalf("expected ledger insert event %q, got %#v", ledgerEvent, got)
	}
	for _, statement := range upStatements {
		statementIndex := eventIndex(got, statement)
		if statementIndex < 0 || statementIndex > ledgerIndex {
			t.Fatalf("expected statement %q before ledger insert; got events %#v", statement, got)
		}
	}
	combinedBodyEvent := "exec:" + catalog[1].UpSQL
	if containsEvent(got, combinedBodyEvent) {
		t.Fatalf("expected migration body to be split into individual statements, got combined exec event %#v", got)
	}
}

func TestApplyCatalogUpToVersionNoopsWhenTargetAlreadyApplied(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{}

	result, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{ledgerEntryFor(t, catalog, 1)}, 1)
	if err != nil {
		t.Fatalf("apply no-op target: %v", err)
	}
	if result.PreviousVersion != 1 || result.CurrentVersion != 1 || result.LatestVersion != 1 {
		t.Fatalf("unexpected no-op result: %#v", result)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("expected no applied steps for no-op target, got %#v", result.Applied)
	}
	if got := scenario.eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected no transaction for no-op target, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRejectsUnterminatedStatementBeforeTransaction(t *testing.T) {
	upSQL := "-- go-metin2 migration: 0001 bootstrap_schema_migrations up\nCREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)\n"
	downSQL := "-- go-metin2 migration: 0001 bootstrap_schema_migrations down\nDROP TABLE schema_migrations;\n"
	catalog := []Migration{{
		Version:    1,
		Name:       "bootstrap_schema_migrations",
		UpPath:     "0001_bootstrap_schema_migrations.up.sql",
		DownPath:   "0001_bootstrap_schema_migrations.down.sql",
		UpSQL:      upSQL,
		DownSQL:    downSQL,
		UpSHA256:   testSHA256Hex(upSQL),
		DownSHA256: testSHA256Hex(downSQL),
	}}
	scenario := &testMigrationApplySQLScenario{}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 1)
	if !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("expected ErrInvalidCatalog for unterminated statement, got %v", err)
	}
	if got := scenario.eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected invalid catalog to fail before transaction begin, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRejectsStaleInputLedgerInsideTransaction(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{currentLedger: nil}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{ledgerEntryFor(t, catalog, 1)}, 0)
	if !errors.Is(err, ErrMigrationApplyLedgerVerification) {
		t.Fatalf("expected stale caller ledger to fail closed, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected stale ledger validation to roll back, got events %#v", got)
	}
	if containsAnyExecEventForMigrationSQL(t, got, catalog[0].UpSQL) || containsEvent(got, "exec:"+schemaMigrationLedgerInsertSQL(catalog[0])) {
		t.Fatalf("expected stale ledger validation to reject before executing migration SQL, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected stale ledger validation not to commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRejectsLedgerDriftBeforeCommit(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{
		currentLedger:      nil,
		skipLedgerMutation: true,
	}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 1)
	if !errors.Is(err, ErrMigrationApplyLedgerVerification) {
		t.Fatalf("expected post-apply ledger verification failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsAnyExecEventForMigrationSQL(t, got, catalog[0].UpSQL) || !containsEvent(got, "exec:"+schemaMigrationLedgerInsertSQL(catalog[0])) {
		t.Fatalf("expected migration and ledger insert before verification, got events %#v", got)
	}
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected post-apply verification failure to roll back, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected post-apply verification failure not to commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRollsBackWhenTransactionalLedgerReadFails(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{currentLedger: []LedgerEntry{ledgerEntryFor(t, catalog, 1)}, queryErr: errStubMigrationQuery}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{ledgerEntryFor(t, catalog, 1)}, 0)
	if !errors.Is(err, errStubMigrationQuery) {
		t.Fatalf("expected transactional ledger read failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected query failure to roll back, got events %#v", got)
	}
	if containsAnyExecEventForMigrationSQL(t, got, catalog[0].UpSQL) || containsEvent(got, "commit") {
		t.Fatalf("expected query failure before migration execution/commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionExecutesRollbackMigrationsAndDeletesLedgerRowsInOneTransaction(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
		testMigration{
			version:  3,
			name:     "characters",
			upPath:   "0003_characters.up.sql",
			downPath: "0003_characters.down.sql",
			upSQL:    "-- go-metin2 migration: 0003 characters up\nCREATE TABLE characters (id INTEGER PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0003 characters down\nDROP TABLE characters;\n",
		},
	)
	ledger := []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
		ledgerEntryFor(t, catalog, 3),
	}
	scenario := &testMigrationApplySQLScenario{currentLedger: append([]LedgerEntry(nil), ledger...)}

	result, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, ledger, 1)
	if err != nil {
		t.Fatalf("rollback migrations: %v", err)
	}
	if result.PreviousVersion != 3 || result.CurrentVersion != 1 || result.LatestVersion != 3 {
		t.Fatalf("unexpected rollback result versions: %#v", result)
	}
	if len(result.Applied) != 2 || result.Applied[0].Version != 3 || result.Applied[0].Direction != DirectionDown || result.Applied[1].Version != 2 || result.Applied[1].Direction != DirectionDown {
		t.Fatalf("unexpected rollback applied steps: %#v", result.Applied)
	}

	want := []string{
		"begin",
		"query:" + SchemaMigrationsLedgerQuery,
		"exec:" + schemaMigrationLedgerDeleteSQL(catalog[2]),
	}
	want = append(want, execEventsForMigrationSQL(t, catalog[2].DownSQL)...)
	want = append(want,
		"exec:"+schemaMigrationLedgerDeleteSQL(catalog[1]),
	)
	want = append(want, execEventsForMigrationSQL(t, catalog[1].DownSQL)...)
	want = append(want,
		"query:"+SchemaMigrationsLedgerQuery,
		"commit",
	)
	if got := scenario.eventsSnapshot(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected rollback execution order:\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestApplyCatalogUpToVersionRollsBackToZeroByDeletingLedgerBeforeDroppingSchemaMigrationsTable(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	ledger := []LedgerEntry{ledgerEntryFor(t, catalog, 1)}
	scenario := &testMigrationApplySQLScenario{currentLedger: append([]LedgerEntry(nil), ledger...)}

	result, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, ledger, 0)
	if err != nil {
		t.Fatalf("rollback schema_migrations to zero: %v", err)
	}
	if result.PreviousVersion != 1 || result.CurrentVersion != 0 || result.LatestVersion != 1 {
		t.Fatalf("unexpected rollback-to-zero result versions: %#v", result)
	}
	want := []string{
		"begin",
		"query:" + SchemaMigrationsLedgerQuery,
		"exec:" + schemaMigrationLedgerDeleteSQL(catalog[0]),
	}
	want = append(want, execEventsForMigrationSQL(t, catalog[0].DownSQL)...)
	want = append(want, "commit")
	if got := scenario.eventsSnapshot(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected rollback-to-zero order:\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestApplyCatalogUpToVersionRollsBackWhenMigrationSQLFailsBeforeLedgerWrite(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)
	scenario := &testMigrationApplySQLScenario{execErrContains: "CREATE TABLE accounts"}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 2)
	if !errors.Is(err, errStubMigrationExec) {
		t.Fatalf("expected migration exec failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected failed migration SQL to roll back, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected failed migration SQL not to commit, got events %#v", got)
	}
	if containsEvent(got, "exec:"+schemaMigrationLedgerInsertSQL(catalog[1])) {
		t.Fatalf("expected failed migration SQL not to write its ledger row, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRollsBackWhenLedgerInsertFails(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{execErrContains: "INSERT INTO schema_migrations"}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 1)
	if !errors.Is(err, errStubMigrationExec) {
		t.Fatalf("expected ledger insert failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected failed ledger insert to roll back, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected failed ledger insert not to commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRollsBackWhenLedgerInsertAffectsNoRows(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{execRowsAffectedContains: "INSERT INTO schema_migrations", execRowsAffected: 0}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 1)
	if !errors.Is(err, ErrMigrationApplyLedgerRowCount) {
		t.Fatalf("expected ledger row count failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected zero-row ledger insert to roll back, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected zero-row ledger insert not to commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRollsBackWhenLedgerInsertRowsAffectedFails(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{execRowsAffectedContains: "INSERT INTO schema_migrations", execRowsAffectedErr: errStubMigrationRows}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 1)
	if !errors.Is(err, ErrMigrationApplyLedgerRowCount) || !errors.Is(err, errStubMigrationRows) {
		t.Fatalf("expected ledger row count and rows-affected failures, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected rows-affected failure to roll back, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected rows-affected failure not to commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRollsBackWhenLedgerDeleteFailsBeforeDownMigrationSQL(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)
	ledger := []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
	}
	scenario := &testMigrationApplySQLScenario{currentLedger: append([]LedgerEntry(nil), ledger...), execErrContains: "DELETE FROM schema_migrations"}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, ledger, 1)
	if !errors.Is(err, errStubMigrationExec) {
		t.Fatalf("expected ledger delete failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected failed ledger delete to roll back, got events %#v", got)
	}
	if containsAnyExecEventForMigrationSQL(t, got, catalog[1].DownSQL) {
		t.Fatalf("expected failed ledger delete not to execute down SQL, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected failed ledger delete not to commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRollsBackWhenLedgerDeleteAffectsNoRowsBeforeDownMigrationSQL(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)
	ledger := []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
	}
	scenario := &testMigrationApplySQLScenario{currentLedger: append([]LedgerEntry(nil), ledger...), execRowsAffectedContains: "DELETE FROM schema_migrations", execRowsAffected: 0}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, ledger, 1)
	if !errors.Is(err, ErrMigrationApplyLedgerRowCount) {
		t.Fatalf("expected ledger row count failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected zero-row ledger delete to roll back, got events %#v", got)
	}
	if containsAnyExecEventForMigrationSQL(t, got, catalog[1].DownSQL) {
		t.Fatalf("expected zero-row ledger delete not to execute down SQL, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected zero-row ledger delete not to commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRollsBackWhenDownMigrationSQLFailsAfterLedgerDelete(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)
	ledger := []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
	}
	scenario := &testMigrationApplySQLScenario{currentLedger: append([]LedgerEntry(nil), ledger...), execErrContains: "DROP TABLE accounts"}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, ledger, 1)
	if !errors.Is(err, errStubMigrationExec) {
		t.Fatalf("expected down migration SQL failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "exec:"+schemaMigrationLedgerDeleteSQL(catalog[1])) {
		t.Fatalf("expected down migration to delete ledger row before SQL statement execution, got events %#v", got)
	}
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected failed down migration SQL to roll back, got events %#v", got)
	}
	if containsEvent(got, "commit") {
		t.Fatalf("expected failed down migration SQL not to commit, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionReportsRollbackFailure(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{execErrContains: "INSERT INTO schema_migrations", rollbackErr: errStubMigrationRollback}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 1)
	if !errors.Is(err, errStubMigrationExec) || !errors.Is(err, errStubMigrationRollback) {
		t.Fatalf("expected exec and rollback errors, got %v", err)
	}
}

func TestApplyCatalogUpToVersionReportsCommitFailure(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{commitErr: errStubMigrationCommit}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, nil, 1)
	if !errors.Is(err, errStubMigrationCommit) {
		t.Fatalf("expected commit failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "commit") {
		t.Fatalf("expected commit to be attempted, got events %#v", got)
	}
}

func TestApplyCatalogUpToVersionRejectsNilExecutor(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())

	t.Run("nil interface", func(t *testing.T) {
		_, err := ApplyCatalogUpToVersion(context.Background(), nil, catalog, nil, 1)
		if !errors.Is(err, ErrMigrationApplyExecutorRequired) {
			t.Fatalf("expected ErrMigrationApplyExecutorRequired, got %v", err)
		}
	})

	t.Run("typed nil db", func(t *testing.T) {
		var db *sql.DB
		_, err := ApplyCatalogUpToVersion(context.Background(), db, catalog, nil, 1)
		if !errors.Is(err, ErrMigrationApplyExecutorRequired) {
			t.Fatalf("expected ErrMigrationApplyExecutorRequired, got %v", err)
		}
	})
}

func execEventsForMigrationSQL(t *testing.T, sql string) []string {
	t.Helper()
	statements, err := splitMigrationSQLStatements(sql)
	if err != nil {
		t.Fatalf("split test migration SQL: %v", err)
	}
	events := make([]string, 0, len(statements))
	for _, statement := range statements {
		events = append(events, "exec:"+statement)
	}
	return events
}

func containsAnyExecEventForMigrationSQL(t *testing.T, events []string, sql string) bool {
	t.Helper()
	for _, want := range execEventsForMigrationSQL(t, sql) {
		if containsEvent(events, want) {
			return true
		}
	}
	return false
}

func eventIndex(events []string, want string) int {
	for i, event := range events {
		if event == want {
			return i
		}
	}
	return -1
}

func containsEvent(events []string, want string) bool {
	return eventIndex(events, want) >= 0
}

const testMigrationApplySQLDriverName = "go_metin2_migrations_apply_test"

var (
	testMigrationApplySQLDriverOnce      sync.Once
	testMigrationApplySQLDriverMu        sync.Mutex
	testMigrationApplySQLDriverNext      int
	testMigrationApplySQLDriverScenarios = make(map[string]*testMigrationApplySQLScenario)
)

type testMigrationApplySQLScenario struct {
	mu                       sync.Mutex
	events                   []string
	currentLedger            []LedgerEntry
	skipLedgerMutation       bool
	queryErr                 error
	beginErr                 error
	execErrContains          string
	execRowsAffectedContains string
	execRowsAffected         int64
	execRowsAffectedErr      error
	commitErr                error
	rollbackErr              error
}

func (s *testMigrationApplySQLScenario) eventsSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.events...)
}

func (s *testMigrationApplySQLScenario) record(event string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *testMigrationApplySQLScenario) execErrorFor(query string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.execErrContains != "" && strings.Contains(query, s.execErrContains) {
		return errStubMigrationExec
	}
	return nil
}

func (s *testMigrationApplySQLScenario) execResultFor(query string) driver.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	rowsAffected := int64(1)
	rowsAffectedErr := error(nil)
	if s.execRowsAffectedContains != "" && strings.Contains(query, s.execRowsAffectedContains) {
		rowsAffected = s.execRowsAffected
		rowsAffectedErr = s.execRowsAffectedErr
	}
	if rowsAffected == 1 && rowsAffectedErr == nil && !s.skipLedgerMutation {
		if migration, ok := migrationFromLedgerSQL(query); ok {
			s.applyLedgerMutation(query, migration)
		}
	}
	return testMigrationApplySQLResult{rowsAffected: rowsAffected, rowsAffectedErr: rowsAffectedErr}
}

func (s *testMigrationApplySQLScenario) applyLedgerMutation(query string, entry LedgerEntry) {
	if strings.HasPrefix(query, "INSERT INTO schema_migrations") {
		s.currentLedger = append(s.currentLedger, entry)
		return
	}
	if strings.HasPrefix(query, "DELETE FROM schema_migrations") {
		for i, current := range s.currentLedger {
			if current == entry {
				s.currentLedger = append(s.currentLedger[:i], s.currentLedger[i+1:]...)
				return
			}
		}
	}
}

func migrationFromLedgerSQL(query string) (LedgerEntry, bool) {
	if strings.HasPrefix(query, "INSERT INTO schema_migrations") {
		start := strings.Index(query, "VALUES (")
		if start < 0 {
			return LedgerEntry{}, false
		}
		body := strings.TrimSuffix(strings.TrimSpace(query[start+len("VALUES ("):]), ");")
		parts := strings.Split(body, ", ")
		if len(parts) != 3 {
			return LedgerEntry{}, false
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return LedgerEntry{}, false
		}
		return LedgerEntry{Version: version, Name: strings.Trim(parts[1], "'"), UpSHA256: strings.Trim(parts[2], "'")}, true
	}
	if strings.HasPrefix(query, "DELETE FROM schema_migrations") {
		version, ok := sqlIntAfter(query, "version = ")
		if !ok {
			return LedgerEntry{}, false
		}
		name, ok := sqlTextAfter(query, "name = ")
		if !ok {
			return LedgerEntry{}, false
		}
		upSHA256, ok := sqlTextAfter(query, "up_sha256 = ")
		if !ok {
			return LedgerEntry{}, false
		}
		return LedgerEntry{Version: version, Name: name, UpSHA256: upSHA256}, true
	}
	return LedgerEntry{}, false
}

func sqlIntAfter(query, marker string) (int, bool) {
	start := strings.Index(query, marker)
	if start < 0 {
		return 0, false
	}
	rest := query[start+len(marker):]
	end := strings.IndexAny(rest, " ;\n")
	if end >= 0 {
		rest = rest[:end]
	}
	value, err := strconv.Atoi(rest)
	return value, err == nil
}

func sqlTextAfter(query, marker string) (string, bool) {
	start := strings.Index(query, marker)
	if start < 0 {
		return "", false
	}
	rest := strings.TrimSpace(query[start+len(marker):])
	if !strings.HasPrefix(rest, "'") {
		return "", false
	}
	rest = rest[1:]
	end := strings.IndexByte(rest, '\'')
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func (s *testMigrationApplySQLScenario) queryError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queryErr
}

func (s *testMigrationApplySQLScenario) ledgerForQuery() []LedgerEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]LedgerEntry(nil), s.currentLedger...)
}

func openTestMigrationApplyDB(t *testing.T, scenario *testMigrationApplySQLScenario) *sql.DB {
	t.Helper()
	testMigrationApplySQLDriverOnce.Do(func() {
		sql.Register(testMigrationApplySQLDriverName, testMigrationApplySQLDriver{})
	})

	testMigrationApplySQLDriverMu.Lock()
	testMigrationApplySQLDriverNext++
	name := fmt.Sprintf("scenario-%d", testMigrationApplySQLDriverNext)
	testMigrationApplySQLDriverScenarios[name] = scenario
	testMigrationApplySQLDriverMu.Unlock()

	db, err := sql.Open(testMigrationApplySQLDriverName, name)
	if err != nil {
		t.Fatalf("open test migration apply db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		testMigrationApplySQLDriverMu.Lock()
		delete(testMigrationApplySQLDriverScenarios, name)
		testMigrationApplySQLDriverMu.Unlock()
	})
	return db
}

type testMigrationApplySQLDriver struct{}

func (testMigrationApplySQLDriver) Open(name string) (driver.Conn, error) {
	return &testMigrationApplySQLConn{name: name}, nil
}

type testMigrationApplySQLConn struct {
	name string
}

func (c *testMigrationApplySQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported by test migration apply driver")
}

func (c *testMigrationApplySQLConn) Close() error {
	return nil
}

func (c *testMigrationApplySQLConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *testMigrationApplySQLConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	scenario, err := c.scenario()
	if err != nil {
		return nil, err
	}
	scenario.record("begin")
	if scenario.beginErr != nil {
		return nil, scenario.beginErr
	}
	return &testMigrationApplySQLTx{scenario: scenario}, nil
}

func (c *testMigrationApplySQLConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if len(args) != 0 {
		return nil, errors.New("test migration apply driver expected no bound args")
	}
	scenario, err := c.scenario()
	if err != nil {
		return nil, err
	}
	scenario.record("exec:" + query)
	if err := scenario.execErrorFor(query); err != nil {
		return nil, err
	}
	return scenario.execResultFor(query), nil
}

type testMigrationApplySQLResult struct {
	rowsAffected    int64
	rowsAffectedErr error
}

func (r testMigrationApplySQLResult) LastInsertId() (int64, error) {
	return 0, errors.New("last insert id unsupported by test migration apply driver")
}

func (r testMigrationApplySQLResult) RowsAffected() (int64, error) {
	if r.rowsAffectedErr != nil {
		return 0, r.rowsAffectedErr
	}
	return r.rowsAffected, nil
}

func (c *testMigrationApplySQLConn) scenario() (*testMigrationApplySQLScenario, error) {
	testMigrationApplySQLDriverMu.Lock()
	scenario := testMigrationApplySQLDriverScenarios[c.name]
	testMigrationApplySQLDriverMu.Unlock()
	if scenario == nil {
		return nil, errors.New("missing test migration apply SQL scenario")
	}
	return scenario, nil
}

type testMigrationApplySQLTx struct {
	scenario *testMigrationApplySQLScenario
}

func (tx *testMigrationApplySQLTx) Commit() error {
	tx.scenario.record("commit")
	return tx.scenario.commitErr
}

func (tx *testMigrationApplySQLTx) Rollback() error {
	tx.scenario.record("rollback")
	if tx.scenario.rollbackErr != nil {
		return tx.scenario.rollbackErr
	}
	return nil
}

func (c *testMigrationApplySQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 0 {
		return nil, errors.New("test migration apply driver expected no bound query args")
	}
	scenario, err := c.scenario()
	if err != nil {
		return nil, err
	}
	scenario.record("query:" + query)
	if err := scenario.queryError(); err != nil {
		return nil, err
	}
	return &testMigrationApplySQLRows{entries: scenario.ledgerForQuery()}, nil
}

type testMigrationApplySQLRows struct {
	entries []LedgerEntry
	index   int
}

func (testMigrationApplySQLRows) Columns() []string { return []string{"version", "name", "up_sha256"} }
func (testMigrationApplySQLRows) Close() error      { return nil }
func (r *testMigrationApplySQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.entries) {
		return io.EOF
	}
	entry := r.entries[r.index]
	r.index++
	dest[0] = int64(entry.Version)
	dest[1] = entry.Name
	dest[2] = entry.UpSHA256
	return nil
}
