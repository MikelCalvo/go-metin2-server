package migrations

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

var (
	errStubMigrationBegin    = errors.New("stub migration begin failed")
	errStubMigrationExec     = errors.New("stub migration exec failed")
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
	scenario := &testMigrationApplySQLScenario{}

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

	want := []string{
		"begin",
		"exec:" + catalog[0].UpSQL,
		"exec:" + schemaMigrationLedgerInsertSQL(catalog[0]),
		"exec:" + catalog[1].UpSQL,
		"exec:" + schemaMigrationLedgerInsertSQL(catalog[1]),
		"commit",
	}
	if got := scenario.eventsSnapshot(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected execution order:\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
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
	scenario := &testMigrationApplySQLScenario{}

	result, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
		ledgerEntryFor(t, catalog, 3),
	}, 1)
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
		"exec:" + schemaMigrationLedgerDeleteSQL(catalog[2]),
		"exec:" + catalog[2].DownSQL,
		"exec:" + schemaMigrationLedgerDeleteSQL(catalog[1]),
		"exec:" + catalog[1].DownSQL,
		"commit",
	}
	if got := scenario.eventsSnapshot(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected rollback execution order:\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestApplyCatalogUpToVersionRollsBackToZeroByDeletingLedgerBeforeDroppingSchemaMigrationsTable(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	scenario := &testMigrationApplySQLScenario{}

	result, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{ledgerEntryFor(t, catalog, 1)}, 0)
	if err != nil {
		t.Fatalf("rollback schema_migrations to zero: %v", err)
	}
	if result.PreviousVersion != 1 || result.CurrentVersion != 0 || result.LatestVersion != 1 {
		t.Fatalf("unexpected rollback-to-zero result versions: %#v", result)
	}
	want := []string{
		"begin",
		"exec:" + schemaMigrationLedgerDeleteSQL(catalog[0]),
		"exec:" + catalog[0].DownSQL,
		"commit",
	}
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
	scenario := &testMigrationApplySQLScenario{execErrContains: "DELETE FROM schema_migrations"}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
	}, 1)
	if !errors.Is(err, errStubMigrationExec) {
		t.Fatalf("expected ledger delete failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected failed ledger delete to roll back, got events %#v", got)
	}
	if containsEvent(got, "exec:"+catalog[1].DownSQL) {
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
	scenario := &testMigrationApplySQLScenario{execRowsAffectedContains: "DELETE FROM schema_migrations", execRowsAffected: 0}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
	}, 1)
	if !errors.Is(err, ErrMigrationApplyLedgerRowCount) {
		t.Fatalf("expected ledger row count failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "rollback") {
		t.Fatalf("expected zero-row ledger delete to roll back, got events %#v", got)
	}
	if containsEvent(got, "exec:"+catalog[1].DownSQL) {
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
	scenario := &testMigrationApplySQLScenario{execErrContains: "DROP TABLE accounts"}

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
	}, 1)
	if !errors.Is(err, errStubMigrationExec) {
		t.Fatalf("expected down migration SQL failure, got %v", err)
	}
	got := scenario.eventsSnapshot()
	if !containsEvent(got, "exec:"+schemaMigrationLedgerDeleteSQL(catalog[1])) {
		t.Fatalf("expected down migration to delete ledger row before SQL body, got events %#v", got)
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

func containsEvent(events []string, want string) bool {
	for _, event := range events {
		if event == want {
			return true
		}
	}
	return false
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
	if s.execRowsAffectedContains != "" && strings.Contains(query, s.execRowsAffectedContains) {
		return testMigrationApplySQLResult{rowsAffected: s.execRowsAffected, rowsAffectedErr: s.execRowsAffectedErr}
	}
	return testMigrationApplySQLResult{rowsAffected: 1}
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

func (c *testMigrationApplySQLConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return testMigrationApplySQLRows{}, nil
}

type testMigrationApplySQLRows struct{}

func (testMigrationApplySQLRows) Columns() []string { return nil }
func (testMigrationApplySQLRows) Close() error      { return nil }
func (testMigrationApplySQLRows) Next([]driver.Value) error {
	return io.EOF
}
