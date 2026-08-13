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

func TestApplyCatalogUpToVersionRejectsRollbackTargetWithoutExecuting(t *testing.T) {
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

	_, err := ApplyCatalogUpToVersion(context.Background(), openTestMigrationApplyDB(t, scenario), catalog, []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
	}, 1)
	if !errors.Is(err, ErrMigrationApplyUnsupportedDirection) {
		t.Fatalf("expected ErrMigrationApplyUnsupportedDirection, got %v", err)
	}
	if got := scenario.eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected rollback target to fail before transaction, got events %#v", got)
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
	mu              sync.Mutex
	events          []string
	beginErr        error
	execErrContains string
	commitErr       error
	rollbackErr     error
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
	return driver.RowsAffected(1), nil
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
