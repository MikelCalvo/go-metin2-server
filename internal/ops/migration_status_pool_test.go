package ops

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

const migrationStatusPoolTestDriverName = "go_metin2_migration_status_pool_test"

var (
	registerMigrationStatusPoolTestDriver sync.Once
	migrationStatusPoolDriverState        migrationStatusPoolTestDriver
)

func TestMigrationStatusPoolDisabledWithoutDSNUsesEmptyLedgerPlan(t *testing.T) {
	pool, err := NewMigrationStatusPool(MigrationStatusPoolConfig{
		Driver: "not-registered-and-must-not-open",
	})
	if err != nil {
		t.Fatalf("NewMigrationStatusPool: %v", err)
	}
	defer pool.Close()

	if pool.Enabled() {
		t.Fatal("pool must stay disabled without a DSN")
	}
	plan, err := pool.Plan()
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.CurrentVersion != 0 || plan.LatestVersion == 0 || plan.UpToDate || len(plan.Pending) != plan.LatestVersion {
		t.Fatalf("expected embedded empty-ledger plan, got %+v", plan)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("Close disabled pool: %v", err)
	}
}

func TestMigrationStatusPoolReusesConnectionAcrossStatusRequests(t *testing.T) {
	state := registeredMigrationStatusPoolTestDriver(t)
	pool, err := NewMigrationStatusPool(MigrationStatusPoolConfig{
		Driver:                migrationStatusPoolTestDriverName,
		DSN:                   "opaque-test-target",
		MaxOpenConnections:    2,
		MaxIdleConnections:    1,
		ConnectionMaxIdleTime: time.Minute,
		ConnectionMaxLifetime: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewMigrationStatusPool: %v", err)
	}

	if !pool.Enabled() {
		t.Fatal("configured pool must be enabled")
	}
	if got := pool.stats().MaxOpenConnections; got != 2 {
		t.Fatalf("MaxOpenConnections = %d, want 2", got)
	}
	if got := state.opens.Load(); got != 0 {
		t.Fatalf("constructor opened %d physical connections; sql.Open must stay lazy", got)
	}

	mux := RegisterLocalMigrationStatusEndpoint(NewPprofMux("gamed"), pool.Plan)
	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		req := httptest.NewRequest(http.MethodGet, LocalDBMigrationsStatusPath, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d body=%s", requestNumber, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"current_version":0`) {
			t.Fatalf("request %d missing empty-ledger status: %s", requestNumber, rec.Body.String())
		}
	}

	if got := state.opens.Load(); got != 1 {
		t.Fatalf("physical connection opens = %d, want one reused connection", got)
	}
	if got := state.queries.Load(); got != 2 {
		t.Fatalf("ledger queries = %d, want 2", got)
	}
	if got := state.execs.Load(); got != 0 {
		t.Fatalf("mutation execs = %d, want 0", got)
	}
	if got := state.lastQuery(); got != dbmigrations.SchemaMigrationsLedgerQuery {
		t.Fatalf("query = %q, want metadata-only ledger query", got)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := state.closes.Load(); got != 1 {
		t.Fatalf("physical connection closes = %d, want 1", got)
	}
}

func TestMigrationStatusPoolRejectsUnsafeConfiguredLimits(t *testing.T) {
	testCases := []struct {
		name   string
		config MigrationStatusPoolConfig
	}{
		{name: "missing driver", config: MigrationStatusPoolConfig{DSN: "target", MaxOpenConnections: 1, MaxIdleConnections: 1, ConnectionMaxIdleTime: time.Minute, ConnectionMaxLifetime: time.Hour}},
		{name: "unbounded open", config: MigrationStatusPoolConfig{Driver: "driver", DSN: "target", MaxIdleConnections: 1, ConnectionMaxIdleTime: time.Minute, ConnectionMaxLifetime: time.Hour}},
		{name: "unbounded idle", config: MigrationStatusPoolConfig{Driver: "driver", DSN: "target", MaxOpenConnections: 1, ConnectionMaxIdleTime: time.Minute, ConnectionMaxLifetime: time.Hour}},
		{name: "idle exceeds open", config: MigrationStatusPoolConfig{Driver: "driver", DSN: "target", MaxOpenConnections: 1, MaxIdleConnections: 2, ConnectionMaxIdleTime: time.Minute, ConnectionMaxLifetime: time.Hour}},
		{name: "missing idle time", config: MigrationStatusPoolConfig{Driver: "driver", DSN: "target", MaxOpenConnections: 1, MaxIdleConnections: 1, ConnectionMaxLifetime: time.Hour}},
		{name: "missing lifetime", config: MigrationStatusPoolConfig{Driver: "driver", DSN: "target", MaxOpenConnections: 1, MaxIdleConnections: 1, ConnectionMaxIdleTime: time.Minute}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := NewMigrationStatusPool(tc.config)
			if pool != nil {
				_ = pool.Close()
				t.Fatal("expected nil pool for invalid configured limits")
			}
			if !errors.Is(err, ErrInvalidMigrationStatusPoolConfig) {
				t.Fatalf("error = %v, want ErrInvalidMigrationStatusPoolConfig", err)
			}
		})
	}
}

func registeredMigrationStatusPoolTestDriver(t *testing.T) *migrationStatusPoolTestDriver {
	t.Helper()
	registerMigrationStatusPoolTestDriver.Do(func() {
		sql.Register(migrationStatusPoolTestDriverName, &migrationStatusPoolDriverState)
	})
	migrationStatusPoolDriverState.reset()
	return &migrationStatusPoolDriverState
}

type migrationStatusPoolTestDriver struct {
	opens   atomic.Int64
	closes  atomic.Int64
	queries atomic.Int64
	execs   atomic.Int64
	mu      sync.Mutex
	query   string
}

func (d *migrationStatusPoolTestDriver) Open(string) (driver.Conn, error) {
	d.opens.Add(1)
	return &migrationStatusPoolTestConn{driver: d}, nil
}

func (d *migrationStatusPoolTestDriver) reset() {
	d.opens.Store(0)
	d.closes.Store(0)
	d.queries.Store(0)
	d.execs.Store(0)
	d.mu.Lock()
	d.query = ""
	d.mu.Unlock()
}

func (d *migrationStatusPoolTestDriver) lastQuery() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.query
}

type migrationStatusPoolTestConn struct {
	driver *migrationStatusPoolTestDriver
}

func (c *migrationStatusPoolTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not supported")
}

func (c *migrationStatusPoolTestConn) Close() error {
	c.driver.closes.Add(1)
	return nil
}

func (c *migrationStatusPoolTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *migrationStatusPoolTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.driver.queries.Add(1)
	c.driver.mu.Lock()
	c.driver.query = query
	c.driver.mu.Unlock()
	return migrationStatusPoolTestRows{}, nil
}

func (c *migrationStatusPoolTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	c.driver.execs.Add(1)
	return nil, errors.New("mutating execution is not supported")
}

type migrationStatusPoolTestRows struct{}

func (migrationStatusPoolTestRows) Columns() []string {
	return []string{"version", "name", "up_sha256"}
}

func (migrationStatusPoolTestRows) Close() error { return nil }

func (migrationStatusPoolTestRows) Next([]driver.Value) error { return io.EOF }
