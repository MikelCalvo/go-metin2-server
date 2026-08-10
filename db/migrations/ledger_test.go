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
	errStubLedgerQuery = errors.New("stub ledger query failed")
	errStubLedgerScan  = errors.New("stub ledger scan failed")
	errStubLedgerRows  = errors.New("stub ledger rows failed")
	errStubLedgerClose = errors.New("stub ledger close failed")
)

var _ SQLLedgerQuerier = (*sql.DB)(nil)

func TestPlanCatalogUpToLatestFromSQLLedgerReadsSchemaMigrationsRows(t *testing.T) {
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
	scenario := &testLedgerSQLScenario{rows: [][]driver.Value{
		{int64(catalog[0].Version), catalog[0].Name, catalog[0].UpSHA256},
	}}
	db := openTestLedgerDB(t, scenario)

	plan, err := PlanCatalogUpToLatestFromSQLLedger(context.Background(), catalog, db)
	if err != nil {
		t.Fatalf("expected queried ledger plan to validate: %v", err)
	}

	query, args, closeCalls := scenario.snapshot()
	if query != SchemaMigrationsLedgerQuery {
		t.Fatalf("unexpected ledger query:\n%s", query)
	}
	if len(args) != 0 {
		t.Fatalf("expected ledger query to be argument-free, got %#v", args)
	}
	if closeCalls != 1 {
		t.Fatalf("expected SQL ledger rows to be closed once, got %d", closeCalls)
	}
	if plan.CurrentVersion != 1 || plan.LatestVersion != 2 || plan.UpToDate {
		t.Fatalf("unexpected queried ledger plan: %#v", plan)
	}
	if len(plan.Pending) != 1 {
		t.Fatalf("expected one pending migration, got %#v", plan.Pending)
	}
	pending := plan.Pending[0]
	if pending.Version != 2 || pending.Name != "accounts" || pending.Direction != DirectionUp || pending.Path != "0002_accounts.up.sql" || pending.SHA256 != catalog[1].UpSHA256 {
		t.Fatalf("unexpected pending migration from queried ledger: %#v", pending)
	}
}

func TestReadSQLLedgerEntriesRejectsNilQuerierAndQueryFailures(t *testing.T) {
	cases := []struct {
		name    string
		querier SQLLedgerQuerier
		wantErr error
	}{
		{
			name:    "nil querier",
			querier: nil,
			wantErr: ErrMigrationLedgerReaderRequired,
		},
		{
			name:    "query error",
			querier: &stubSQLLedgerQuerier{err: errStubLedgerQuery},
			wantErr: errStubLedgerQuery,
		},
		{
			name:    "nil rows",
			querier: &stubSQLLedgerQuerier{},
			wantErr: ErrMigrationLedgerReaderRequired,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadSQLLedgerEntries(context.Background(), tc.querier)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestReadLedgerEntriesFromRowsFailsClosedOnRowErrors(t *testing.T) {
	cases := []struct {
		name    string
		rows    *stubLedgerRows
		wantErr error
	}{
		{
			name:    "scan error",
			rows:    &stubLedgerRows{rows: []stubLedgerRow{{scanErr: errStubLedgerScan}}},
			wantErr: errStubLedgerScan,
		},
		{
			name: "iterator error",
			rows: &stubLedgerRows{
				rows: []stubLedgerRow{{version: 1, name: "bootstrap_schema_migrations", upSHA256: strings.Repeat("a", 64)}},
				err:  errStubLedgerRows,
			},
			wantErr: errStubLedgerRows,
		},
		{
			name: "close error",
			rows: &stubLedgerRows{
				rows:     []stubLedgerRow{{version: 1, name: "bootstrap_schema_migrations", upSHA256: strings.Repeat("a", 64)}},
				closeErr: errStubLedgerClose,
			},
			wantErr: errStubLedgerClose,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readLedgerEntriesFromRows(tc.rows)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
			if !tc.rows.closed {
				t.Fatalf("expected ledger rows to be closed after %s failure", tc.name)
			}
		})
	}
}

func TestPlanUpToLatestFromSQLLedgerUsesBuiltInCatalog(t *testing.T) {
	catalog, err := Catalog()
	if err != nil {
		t.Fatalf("load built-in catalog: %v", err)
	}
	scenario := &testLedgerSQLScenario{rows: [][]driver.Value{
		{int64(catalog[0].Version), catalog[0].Name, catalog[0].UpSHA256},
	}}

	plan, err := PlanUpToLatestFromSQLLedger(context.Background(), openTestLedgerDB(t, scenario))
	if err != nil {
		t.Fatalf("expected built-in queried ledger plan to validate: %v", err)
	}
	if plan.CurrentVersion != 1 || plan.LatestVersion != len(catalog) {
		t.Fatalf("unexpected built-in queried ledger plan: %#v", plan)
	}
}

func TestPlanCatalogToVersionFromSQLLedgerReadsRowsAndReturnsRollbackPlan(t *testing.T) {
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
	scenario := &testLedgerSQLScenario{rows: [][]driver.Value{
		{int64(catalog[0].Version), catalog[0].Name, catalog[0].UpSHA256},
		{int64(catalog[1].Version), catalog[1].Name, catalog[1].UpSHA256},
	}}

	plan, err := PlanCatalogToVersionFromSQLLedger(context.Background(), catalog, openTestLedgerDB(t, scenario), 0)
	if err != nil {
		t.Fatalf("expected queried rollback plan to validate: %v", err)
	}

	query, args, closeCalls := scenario.snapshot()
	if query != SchemaMigrationsLedgerQuery {
		t.Fatalf("unexpected ledger query:\n%s", query)
	}
	if len(args) != 0 {
		t.Fatalf("expected ledger query to be argument-free, got %#v", args)
	}
	if closeCalls != 1 {
		t.Fatalf("expected SQL ledger rows to be closed once, got %d", closeCalls)
	}
	if plan.CurrentVersion != 2 || plan.LatestVersion != 2 || plan.UpToDate {
		t.Fatalf("unexpected queried rollback plan: %#v", plan)
	}
	if len(plan.Pending) != 2 {
		t.Fatalf("expected two down steps, got %#v", plan.Pending)
	}
	if plan.Pending[0].Version != 2 || plan.Pending[0].Direction != DirectionDown || plan.Pending[0].Path != "0002_accounts.down.sql" || plan.Pending[0].SHA256 != catalog[1].DownSHA256 {
		t.Fatalf("unexpected first queried rollback step: %#v", plan.Pending[0])
	}
	if plan.Pending[1].Version != 1 || plan.Pending[1].Direction != DirectionDown || plan.Pending[1].Path != "0001_bootstrap_schema_migrations.down.sql" || plan.Pending[1].SHA256 != catalog[0].DownSHA256 {
		t.Fatalf("unexpected second queried rollback step: %#v", plan.Pending[1])
	}
}

type stubSQLLedgerQuerier struct {
	err error
}

func (q *stubSQLLedgerQuerier) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	if q.err != nil {
		return nil, q.err
	}
	return nil, nil
}

type stubLedgerRow struct {
	version  int
	name     string
	upSHA256 string
	scanErr  error
}

type stubLedgerRows struct {
	rows     []stubLedgerRow
	idx      int
	err      error
	closeErr error
	closed   bool
}

func (r *stubLedgerRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *stubLedgerRows) Scan(dest ...any) error {
	row := r.rows[r.idx-1]
	if row.scanErr != nil {
		return row.scanErr
	}
	if len(dest) != 3 {
		return errors.New("unexpected ledger scan destination count")
	}
	version, ok := dest[0].(*int)
	if !ok {
		return errors.New("unexpected ledger version scan destination")
	}
	name, ok := dest[1].(*string)
	if !ok {
		return errors.New("unexpected ledger name scan destination")
	}
	upSHA256, ok := dest[2].(*string)
	if !ok {
		return errors.New("unexpected ledger checksum scan destination")
	}
	*version = row.version
	*name = row.name
	*upSHA256 = row.upSHA256
	return nil
}

func (r *stubLedgerRows) Err() error {
	return r.err
}

func (r *stubLedgerRows) Close() error {
	r.closed = true
	return r.closeErr
}

const testLedgerSQLDriverName = "go_metin2_migrations_ledger_test"

var (
	testLedgerSQLDriverOnce      sync.Once
	testLedgerSQLDriverMu        sync.Mutex
	testLedgerSQLDriverNext      int
	testLedgerSQLDriverScenarios = make(map[string]*testLedgerSQLScenario)
)

type testLedgerSQLScenario struct {
	mu         sync.Mutex
	rows       [][]driver.Value
	queryErr   error
	rowsErr    error
	closeErr   error
	query      string
	args       []driver.NamedValue
	closeCalls int
}

func (s *testLedgerSQLScenario) snapshot() (string, []driver.NamedValue, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.query, append([]driver.NamedValue(nil), s.args...), s.closeCalls
}

func openTestLedgerDB(t *testing.T, scenario *testLedgerSQLScenario) *sql.DB {
	t.Helper()
	testLedgerSQLDriverOnce.Do(func() {
		sql.Register(testLedgerSQLDriverName, testLedgerSQLDriver{})
	})

	testLedgerSQLDriverMu.Lock()
	testLedgerSQLDriverNext++
	name := fmt.Sprintf("scenario-%d", testLedgerSQLDriverNext)
	testLedgerSQLDriverScenarios[name] = scenario
	testLedgerSQLDriverMu.Unlock()

	db, err := sql.Open(testLedgerSQLDriverName, name)
	if err != nil {
		t.Fatalf("open test ledger db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		testLedgerSQLDriverMu.Lock()
		delete(testLedgerSQLDriverScenarios, name)
		testLedgerSQLDriverMu.Unlock()
	})
	return db
}

type testLedgerSQLDriver struct{}

func (testLedgerSQLDriver) Open(name string) (driver.Conn, error) {
	return &testLedgerSQLConn{name: name}, nil
}

type testLedgerSQLConn struct {
	name string
}

func (c *testLedgerSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported by test ledger driver")
}

func (c *testLedgerSQLConn) Close() error {
	return nil
}

func (c *testLedgerSQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions unsupported by test ledger driver")
}

func (c *testLedgerSQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	testLedgerSQLDriverMu.Lock()
	scenario := testLedgerSQLDriverScenarios[c.name]
	testLedgerSQLDriverMu.Unlock()
	if scenario == nil {
		return nil, errors.New("missing test ledger SQL scenario")
	}
	scenario.mu.Lock()
	scenario.query = query
	scenario.args = append([]driver.NamedValue(nil), args...)
	scenario.mu.Unlock()
	if scenario.queryErr != nil {
		return nil, scenario.queryErr
	}
	return &testLedgerSQLRows{scenario: scenario, rows: append([][]driver.Value(nil), scenario.rows...), err: scenario.rowsErr}, nil
}

type testLedgerSQLRows struct {
	scenario *testLedgerSQLScenario
	rows     [][]driver.Value
	idx      int
	err      error
}

func (r *testLedgerSQLRows) Columns() []string {
	return []string{"version", "name", "up_sha256"}
}

func (r *testLedgerSQLRows) Close() error {
	r.scenario.mu.Lock()
	r.scenario.closeCalls++
	closeErr := r.scenario.closeErr
	r.scenario.mu.Unlock()
	return closeErr
}

func (r *testLedgerSQLRows) Next(dest []driver.Value) error {
	if r.idx < len(r.rows) {
		copy(dest, r.rows[r.idx])
		r.idx++
		return nil
	}
	if r.err != nil {
		err := r.err
		r.err = nil
		return err
	}
	return io.EOF
}
