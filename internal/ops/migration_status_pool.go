package ops

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

var ErrInvalidMigrationStatusPoolConfig = errors.New("invalid migration status pool config")

// MigrationStatusPoolConfig keeps connection-pool policy at the explicit
// read-only migration-status boundary. An empty DSN disables the pool.
type MigrationStatusPoolConfig struct {
	Driver                string
	DSN                   string
	MaxOpenConnections    int
	MaxIdleConnections    int
	ConnectionMaxIdleTime time.Duration
	ConnectionMaxLifetime time.Duration
}

// MigrationStatusPool owns one reusable database/sql pool for metadata-only
// schema_migrations status reads. It never registers a driver or applies SQL.
type MigrationStatusPool struct {
	db *sql.DB
}

func NewMigrationStatusPool(config MigrationStatusPoolConfig) (*MigrationStatusPool, error) {
	dsn := strings.TrimSpace(config.DSN)
	if dsn == "" {
		return &MigrationStatusPool{}, nil
	}

	driverName := strings.TrimSpace(config.Driver)
	if driverName == "" {
		return nil, fmt.Errorf("%w: driver is required when DSN is configured", ErrInvalidMigrationStatusPoolConfig)
	}
	if config.MaxOpenConnections <= 0 {
		return nil, fmt.Errorf("%w: max open connections must be positive", ErrInvalidMigrationStatusPoolConfig)
	}
	if config.MaxIdleConnections <= 0 || config.MaxIdleConnections > config.MaxOpenConnections {
		return nil, fmt.Errorf("%w: max idle connections must be positive and not exceed max open connections", ErrInvalidMigrationStatusPoolConfig)
	}
	if config.ConnectionMaxIdleTime <= 0 {
		return nil, fmt.Errorf("%w: connection max idle time must be positive", ErrInvalidMigrationStatusPoolConfig)
	}
	if config.ConnectionMaxLifetime <= 0 {
		return nil, fmt.Errorf("%w: connection max lifetime must be positive", ErrInvalidMigrationStatusPoolConfig)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open migration status pool driver %q: %w", driverName, err)
	}
	db.SetMaxOpenConns(config.MaxOpenConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxIdleTime(config.ConnectionMaxIdleTime)
	db.SetConnMaxLifetime(config.ConnectionMaxLifetime)
	return &MigrationStatusPool{db: db}, nil
}

func (p *MigrationStatusPool) Plan() (dbmigrations.Plan, error) {
	if p == nil || p.db == nil {
		return dbmigrations.PlanUpToLatest(nil)
	}
	return dbmigrations.PlanUpToLatestFromSQLLedger(context.Background(), p.db)
}

func (p *MigrationStatusPool) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *MigrationStatusPool) Enabled() bool {
	return p != nil && p.db != nil
}

func (p *MigrationStatusPool) stats() sql.DBStats {
	if p == nil || p.db == nil {
		return sql.DBStats{}
	}
	return p.db.Stats()
}
