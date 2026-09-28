package migratecli

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
)

const advisoryLockProbeFormat = "go-metin2-migration-advisory-lock-probe-v1"

// advisoryLockProbeName names the only engine catalog this command reads.
// It is a Postgres pg_locks observation, not a user table and not a lock we take.
const advisoryLockProbeName = "postgres_pg_locks_v1"

// advisoryLockProbeEngine is the only database/sql driver name whose session
// advisory-lock catalog this probe knows how to read. Stock binaries register
// no production driver; the tagged SQLite harness name "sqlite" is unsupported.
const advisoryLockProbeEngine = "postgres"

// advisoryLockProbeClassID and advisoryLockProbeObjID are the two int4 halves
// of one fixed Postgres advisory-lock key (classid, objid). The probe only
// asks pg_locks whether some other session already holds that key.
const advisoryLockProbeClassID int32 = 0x6d657469
const advisoryLockProbeObjID int32 = 0x6e320001

const advisoryLockProbeTimeout = 2 * time.Second

var ErrAdvisoryLockProbe = errors.New("migration advisory lock probe failed")

var ErrAdvisoryLockProbeUnsupportedEngine = errors.New("advisory-lock probe engine unsupported")

type advisoryLockProbe struct {
	Format     string `json:"format"`
	Configured bool   `json:"configured"`
	Held       bool   `json:"held"`
	Probe      string `json:"probe"`
	Engine     string `json:"engine"`
}

func runAdvisoryLockProbe(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("advisory-lock-probe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var driverName string
	var dsn string
	var confirm bool
	flags.StringVar(&driverName, "driver", "", "database/sql driver name; only postgres answers this read-only pg_locks probe")
	flags.StringVar(&dsn, "dsn", "", "database/sql DSN for the read-only advisory-lock probe")
	flags.BoolVar(&confirm, "i-confirm-read-only-advisory-probe", false, "confirm the disabled-by-default read-only Postgres pg_locks probe")
	flags.Usage = func() { printAdvisoryLockProbeUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected advisory-lock-probe argument %q\n", flags.Arg(0))
		printAdvisoryLockProbeUsage(stderr)
		return exitUsage
	}
	if !confirm {
		fmt.Fprintln(stderr, "--i-confirm-read-only-advisory-probe is required for advisory-lock-probe")
		printAdvisoryLockProbeUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(driverName) == "" || strings.TrimSpace(dsn) == "" {
		fmt.Fprintln(stderr, "--driver and --dsn are required for advisory-lock-probe")
		printAdvisoryLockProbeUsage(stderr)
		return exitUsage
	}
	driverName = strings.TrimSpace(driverName)
	dsn = strings.TrimSpace(dsn)
	if err := config.RequireRegisteredDatabaseDriver(driverName); err != nil {
		writeMigrationCommandError(stderr, dsn, "migration advisory-lock-probe: %v", err)
		return exitError
	}
	if !advisoryLockProbeEngineSupported(driverName) {
		writeMigrationCommandError(stderr, dsn, "migration advisory-lock-probe: %v", unsupportedAdvisoryLockProbeEngine(driverName))
		return exitError
	}

	held, err := probePostgresSessionAdvisoryLock(driverName, dsn)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration advisory-lock-probe: %v", err)
		return exitError
	}
	return writeJSON(stdout, stderr, advisoryLockProbe{
		Format:     advisoryLockProbeFormat,
		Configured: true,
		Held:       held,
		Probe:      advisoryLockProbeName,
		Engine:     advisoryLockProbeEngine,
	})
}

func advisoryLockProbeEngineSupported(driverName string) bool {
	return driverName == advisoryLockProbeEngine
}

func unsupportedAdvisoryLockProbeEngine(driverName string) error {
	return fmt.Errorf("%w: driver %q cannot answer postgres pg_locks (probe %s); sqlite and other engines stay unsupported and this command does not create a lock table", ErrAdvisoryLockProbeUnsupportedEngine, driverName, advisoryLockProbeName)
}

// probePostgresSessionAdvisoryLock reports whether another session already
// holds the fixed Postgres advisory lock. It runs one read-only pg_locks
// query on one session and never takes, unlocks, deletes, or rewrites a
// filesystem apply lock.
func probePostgresSessionAdvisoryLock(driverName, dsn string) (bool, error) {
	if !advisoryLockProbeEngineSupported(driverName) {
		return false, unsupportedAdvisoryLockProbeEngine(driverName)
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return false, fmt.Errorf("%w: open database driver %q: %v", ErrAdvisoryLockProbe, driverName, err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), advisoryLockProbeTimeout)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: open session: %v", ErrAdvisoryLockProbe, err)
	}
	defer conn.Close()

	var held bool
	err = conn.QueryRowContext(ctx, advisoryLockProbeQuery(), advisoryLockProbeClassID, advisoryLockProbeObjID).Scan(&held)
	if err != nil {
		return false, fmt.Errorf("%w: read postgres pg_locks advisory lock for <redacted-dsn>: %v", ErrAdvisoryLockProbe, err)
	}
	return held, nil
}

// advisoryLockProbeQuery reads pg_locks only. It does not call
// pg_advisory_lock, pg_try_advisory_lock, or pg_advisory_unlock, so a free
// key stays free and a held key stays held on the sessions that own it.
func advisoryLockProbeQuery() string {
	return "SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND classid = $1 AND objid = $2 AND granted = true)"
}

func printAdvisoryLockProbeUsage(w io.Writer) {
	fmt.Fprintln(w, "advisory-lock-probe usage:")
	fmt.Fprintln(w, "  metin2-migrate advisory-lock-probe --driver postgres --dsn <dsn> --i-confirm-read-only-advisory-probe")
	fmt.Fprintln(w, "  reads pg_locks for one fixed key; other engines fail closed; never unlocks")
}
