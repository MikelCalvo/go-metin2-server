package migratecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2

	maxLedgerSnapshotBytes          = 64 * 1024
	maxMigrationPlanArtifactBytes   = 128 * 1024
	maxMigrationApplyPreflightBytes = 128 * 1024
	maxMigrationApplyLockBytes      = 16 * 1024
	maxMigrationApplyAuditBytes     = 128 * 1024
)

// Run executes the small migration preflight CLI and returns a process-style exit
// code. The catalog, status, empty-ledger-snapshot, ledger-snapshot,
// ledger-snapshot-status, plan, plan-artifact, plan-artifact-status,
// apply-preflight, apply-preflight-status, apply-lock-status, apply-audit-status,
// import-export-status, quarantine-export, export-quarantine-drill,
// backup-restore-drill, migration-run-retention, and artifact-retention-gc
// commands are read-only/print-only.
// artifact-gc-aside-purge is a confirmation-gated print-only companion that emits
// a shell script for deleting aged .gc-aside-* trees; the CLI still never executes
// that purge itself and never opens a database target.
// import-export-drill is a confirmation-gated print-only companion that emits a
// shell script walking a retained export/quarantine tree into import-export; the
// CLI still never executes that import itself, never embeds a DSN value, and never
// opens a database target.
// The apply command is an explicit CLI-only mutation surface: it requires an
// operator-supplied database driver, DSN, strict offline ledger snapshot, and
// target version, and it remains deliberately separate from daemon startup and
// local ops endpoints.
// import-export is a second CLI-only mutation surface: it requires an
// operator-supplied database driver, DSN, retained migration-shaped export, and
// --i-confirm-sql-import, then dispatches to the landed programmatic Import*
// seams without inventing upsert policy or registering a stock production driver.
// apply-lock-aside is a separate confirmation-gated local filesystem mutation: it
// only aside-renames a leftover apply lock after recomputing the lab stale-lock
// gate and never opens a database target.
// Rollback/down plans must be explicitly confirmed with --allow-rollback plus
// --plan-sha256, --plan-artifact, or --apply-preflight. Operators can
// optionally require a previously inspected plan checksum, plan artifact, or
// preflight artifact, reserve an exclusive local lock file before opening the
// database, and request an exclusive metadata-only audit file for non-empty apply
// plans.
func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if stdin == nil {
		stdin = strings.NewReader("")
	}

	if len(args) == 0 {
		fmt.Fprintln(stderr, "missing command")
		printUsage(stderr)
		return exitUsage
	}

	switch args[0] {
	case "catalog":
		return runCatalog(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "plan":
		return runPlan(args[1:], stdin, stdout, stderr)
	case "plan-artifact":
		return runPlanArtifact(args[1:], stdin, stdout, stderr)
	case "plan-artifact-status":
		return runPlanArtifactStatus(args[1:], stdout, stderr)
	case "apply-preflight":
		return runApplyPreflight(args[1:], stdin, stdout, stderr)
	case "apply-preflight-status":
		return runApplyPreflightStatus(args[1:], stdout, stderr)
	case "apply-lock-status":
		return runApplyLockStatus(args[1:], stdout, stderr)
	case "apply-lock-aside":
		return runApplyLockAside(args[1:], stdout, stderr)
	case "apply-audit-status":
		return runApplyAuditStatus(args[1:], stdout, stderr)
	case "empty-ledger-snapshot":
		return runEmptyLedgerSnapshot(args[1:], stdout, stderr)
	case "ledger-snapshot":
		return runLedgerSnapshot(args[1:], stdout, stderr)
	case "ledger-snapshot-status":
		return runLedgerSnapshotStatus(args[1:], stdout, stderr)
	case "apply":
		return runApply(args[1:], stdin, stdout, stderr)
	case "quarantine-export":
		return runQuarantineExport(args[1:], stdin, stdout, stderr)
	case "synthesize-wipe-export":
		return runSynthesizeWipeExport(args[1:], stdin, stdout, stderr)
	case "import-export":
		return runImportExport(args[1:], stdin, stdout, stderr)
	case "import-export-status":
		return runImportExportStatus(args[1:], stdout, stderr)
	case "import-export-drill":
		return runImportExportDrill(args[1:], stdout, stderr)
	case "export-quarantine-drill":
		return runExportQuarantineDrill(args[1:], stdin, stdout, stderr)
	case "backup-restore-drill":
		return runBackupRestoreDrill(args[1:], stdin, stdout, stderr)
	case "migration-run-retention":
		return runMigrationRunRetention(args[1:], stdin, stdout, stderr)
	case "artifact-retention-gc":
		return runArtifactRetentionGC(args[1:], stdout, stderr)
	case "artifact-gc-aside-purge":
		return runArtifactGCAsidePurge(args[1:], stdout, stderr)
	case "version", "--version":
		return runVersion(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return exitOK
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return exitUsage
	}
}

func runVersion(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "version does not accept arguments")
		printVersionUsage(stderr)
		return exitUsage
	}
	return writeJSON(stdout, stderr, buildinfo.Current())
}

func runCatalog(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "catalog does not accept arguments")
		printUsage(stderr)
		return exitUsage
	}
	summary, err := dbmigrations.BuiltInCatalogSummary()
	if err != nil {
		fmt.Fprintf(stderr, "migration catalog: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, summary)
}

func runEmptyLedgerSnapshot(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "empty-ledger-snapshot does not accept arguments")
		printEmptyLedgerSnapshotUsage(stderr)
		return exitUsage
	}
	raw, err := dbmigrations.MarshalJSONLedgerSnapshot(nil)
	if err != nil {
		fmt.Fprintf(stderr, "migration empty-ledger-snapshot: %v\n", err)
		return exitError
	}
	if _, err := stdout.Write(raw); err != nil {
		fmt.Fprintf(stderr, "write JSON: %v\n", err)
		return exitError
	}
	return exitOK
}

func runLedgerSnapshot(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("ledger-snapshot", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var driverName string
	var dsn string
	flags.StringVar(&driverName, "driver", "", "database/sql driver name for the migration target")
	flags.StringVar(&dsn, "dsn", "", "database/sql DSN for the migration target")
	flags.Usage = func() { printLedgerSnapshotUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected ledger-snapshot argument %q\n", flags.Arg(0))
		printLedgerSnapshotUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(driverName) == "" || strings.TrimSpace(dsn) == "" {
		fmt.Fprintln(stderr, "--driver and --dsn are required for ledger-snapshot")
		printLedgerSnapshotUsage(stderr)
		return exitUsage
	}

	db, err := sql.Open(strings.TrimSpace(driverName), strings.TrimSpace(dsn))
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration ledger-snapshot: open database driver %q: %v", strings.TrimSpace(driverName), err)
		return exitError
	}
	defer db.Close()

	snapshot, err := dbmigrations.LedgerSnapshotFromSQLLedger(context.Background(), db)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration ledger-snapshot: %v", err)
		return exitError
	}
	return writeJSON(stdout, stderr, snapshot)
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var driverName string
	var dsn string
	var targetVersionText string
	flags.StringVar(&driverName, "driver", "", "database/sql driver name for the migration target")
	flags.StringVar(&dsn, "dsn", "", "database/sql DSN for the migration target")
	flags.StringVar(&targetVersionText, "target-version", "latest", "catalog target version for the read-only status plan")
	flags.Usage = func() { printStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected status argument %q\n", flags.Arg(0))
		printStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(driverName) == "" || strings.TrimSpace(dsn) == "" {
		fmt.Fprintln(stderr, "--driver and --dsn are required for status")
		printStatusUsage(stderr)
		return exitUsage
	}
	targetVersion, targetLatest, err := parseTargetVersion(targetVersionText)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --target-version %q: %v\n", targetVersionText, err)
		return exitUsage
	}

	db, err := sql.Open(strings.TrimSpace(driverName), strings.TrimSpace(dsn))
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration status: open database driver %q: %v", strings.TrimSpace(driverName), err)
		return exitError
	}
	defer db.Close()

	snapshot, err := dbmigrations.LedgerSnapshotFromSQLLedger(context.Background(), db)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration status: %v", err)
		return exitError
	}
	plan, err := planDecodedLedgerSnapshot(snapshot, targetVersion, targetLatest)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration status: %v", err)
		return exitError
	}
	return writeJSON(stdout, stderr, plan)
}

func runPlan(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	plan, code := readMigrationPlanFromArgs("plan", args, stdin, stderr, printPlanUsage)
	if code != exitOK {
		return code
	}
	return writeJSON(stdout, stderr, plan)
}

const migrationPlanArtifactFormat = "go-metin2-migration-plan-artifact-v1"

const migrationPlanArtifactStatusFormat = "go-metin2-migration-plan-artifact-status-v1"

const migrationLedgerSnapshotStatusFormat = "go-metin2-schema-migrations-ledger-snapshot-status-v1"

const migrationApplyPreflightFormat = "go-metin2-migration-apply-preflight-v1"

const migrationApplyPreflightStatusFormat = "go-metin2-migration-apply-preflight-status-v1"

var ErrMigrationLedgerSnapshot = errors.New("migration ledger snapshot failed")

type migrationPlanArtifact struct {
	Format     string            `json:"format"`
	PlanSHA256 string            `json:"plan_sha256"`
	Plan       dbmigrations.Plan `json:"plan"`
}

type migrationPlanArtifactStatus struct {
	Format   string                 `json:"format"`
	Present  bool                   `json:"present"`
	Artifact *migrationPlanArtifact `json:"artifact,omitempty"`
}

type migrationLedgerSnapshotStatus struct {
	Format               string                       `json:"format"`
	Present              bool                         `json:"present"`
	LedgerSnapshotSHA256 string                       `json:"ledger_snapshot_sha256,omitempty"`
	CurrentVersion       int                          `json:"current_version,omitempty"`
	LatestVersion        int                          `json:"latest_version,omitempty"`
	UpToDate             bool                         `json:"up_to_date,omitempty"`
	Snapshot             *dbmigrations.LedgerSnapshot `json:"snapshot,omitempty"`
}

type migrationApplyPreflight struct {
	Format               string            `json:"format"`
	TargetVersion        int               `json:"target_version"`
	TargetLatest         bool              `json:"target_latest"`
	LedgerSnapshotSHA256 string            `json:"ledger_snapshot_sha256"`
	PlanSHA256           string            `json:"plan_sha256"`
	Plan                 dbmigrations.Plan `json:"plan"`
}

type migrationApplyPreflightStatus struct {
	Format    string                   `json:"format"`
	Present   bool                     `json:"present"`
	Preflight *migrationApplyPreflight `json:"preflight,omitempty"`
}

func runPlanArtifact(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	plan, code := readMigrationPlanFromArgs("plan-artifact", args, stdin, stderr, printPlanArtifactUsage)
	if code != exitOK {
		return code
	}
	planSHA256, err := planSHA256(plan)
	if err != nil {
		fmt.Fprintf(stderr, "migration plan-artifact: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, migrationPlanArtifact{
		Format:     migrationPlanArtifactFormat,
		PlanSHA256: planSHA256,
		Plan:       plan,
	})
}

func runPlanArtifactStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("plan-artifact-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var planArtifactPath string
	flags.StringVar(&planArtifactPath, "plan-artifact", "", "path to a metadata-only migration plan artifact")
	flags.Usage = func() { printPlanArtifactStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected plan-artifact-status argument %q\n", flags.Arg(0))
		printPlanArtifactStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(planArtifactPath) == "" {
		fmt.Fprintln(stderr, "--plan-artifact is required for plan-artifact-status")
		printPlanArtifactStatusUsage(stderr)
		return exitUsage
	}

	artifact, present, err := readMigrationPlanArtifactStatusFile(planArtifactPath)
	if err != nil {
		fmt.Fprintf(stderr, "migration plan artifact status: %v\n", err)
		return exitError
	}
	status := migrationPlanArtifactStatus{
		Format:  migrationPlanArtifactStatusFormat,
		Present: present,
	}
	if present {
		status.Artifact = &artifact
	}
	return writeJSON(stdout, stderr, status)
}

func runLedgerSnapshotStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("ledger-snapshot-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var snapshotPath string
	flags.StringVar(&snapshotPath, "ledger-snapshot", "", "path to a metadata-only schema_migrations ledger snapshot")
	flags.Usage = func() { printLedgerSnapshotStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected ledger-snapshot-status argument %q\n", flags.Arg(0))
		printLedgerSnapshotStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(snapshotPath) == "" {
		fmt.Fprintln(stderr, "--ledger-snapshot is required for ledger-snapshot-status")
		printLedgerSnapshotStatusUsage(stderr)
		return exitUsage
	}

	snapshot, present, raw, err := readMigrationLedgerSnapshotStatusFile(snapshotPath)
	if err != nil {
		fmt.Fprintf(stderr, "migration ledger snapshot status: %v\n", err)
		return exitError
	}
	status := migrationLedgerSnapshotStatus{
		Format:  migrationLedgerSnapshotStatusFormat,
		Present: present,
	}
	if present {
		plan, err := dbmigrations.PlanUpToLatest(snapshot.Entries)
		if err != nil {
			fmt.Fprintf(stderr, "migration ledger snapshot status: %v\n", fmt.Errorf("%w: %v", ErrMigrationLedgerSnapshot, err))
			return exitError
		}
		status.LedgerSnapshotSHA256 = sha256Hex(raw)
		status.CurrentVersion = plan.CurrentVersion
		status.LatestVersion = plan.LatestVersion
		status.UpToDate = plan.UpToDate
		status.Snapshot = &snapshot
	}
	return writeJSON(stdout, stderr, status)
}

func runApplyPreflight(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply-preflight", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var snapshotPath string
	var targetVersionText string
	var planSHA256Text string
	var planArtifactPath string
	var allowRollback bool
	flags.StringVar(&snapshotPath, "ledger-snapshot", "", "path to go-metin2 schema_migrations ledger snapshot JSON, or - for stdin")
	flags.StringVar(&targetVersionText, "target-version", "", "catalog target version to preflight for apply")
	flags.StringVar(&planSHA256Text, "plan-sha256", "", "optional SHA-256 of the metadata-only dry-run plan JSON that must match")
	flags.StringVar(&planArtifactPath, "plan-artifact", "", "optional path to a metadata-only migration plan artifact that must match")
	flags.BoolVar(&allowRollback, "allow-rollback", false, "allow preflight of down/rollback migration steps")
	flags.Usage = func() { printApplyPreflightUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected apply-preflight argument %q\n", flags.Arg(0))
		printApplyPreflightUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(snapshotPath) == "" {
		fmt.Fprintln(stderr, "--ledger-snapshot is required for apply-preflight")
		printApplyPreflightUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(targetVersionText) == "" {
		fmt.Fprintln(stderr, "missing --target-version")
		printApplyPreflightUsage(stderr)
		return exitUsage
	}
	targetVersion, targetLatest, err := parseTargetVersion(targetVersionText)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --target-version %q: %v\n", targetVersionText, err)
		return exitUsage
	}
	resolvedTarget, err := resolveTargetVersion(targetVersion, targetLatest)
	if err != nil {
		fmt.Fprintf(stderr, "migration apply-preflight: %v\n", err)
		return exitError
	}
	confirmedPlanSHA256 := ""
	trimmedPlanSHA256 := strings.TrimSpace(planSHA256Text)
	trimmedPlanArtifactPath := strings.TrimSpace(planArtifactPath)
	if trimmedPlanSHA256 != "" && trimmedPlanArtifactPath != "" {
		fmt.Fprintln(stderr, "--plan-sha256 and --plan-artifact cannot be used together")
		printApplyPreflightUsage(stderr)
		return exitUsage
	}
	if trimmedPlanSHA256 != "" {
		confirmedPlanSHA256, err = parsePlanSHA256(trimmedPlanSHA256)
		if err != nil {
			fmt.Fprintf(stderr, "invalid --plan-sha256 %q: %v\n", planSHA256Text, err)
			return exitUsage
		}
	}

	reader, closeReader, err := openLedgerSnapshotReader(snapshotPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "open ledger snapshot: %v\n", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	rawLedger, err := readBoundedLedgerSnapshot(reader)
	if err != nil {
		fmt.Fprintf(stderr, "migration apply-preflight: %v\n", err)
		return exitError
	}
	ledger, err := dbmigrations.ReadJSONLedgerSnapshot(bytes.NewReader(rawLedger))
	if err != nil {
		fmt.Fprintf(stderr, "migration apply-preflight: %v\n", err)
		return exitError
	}
	plan, err := dbmigrations.PlanToVersion(ledger, resolvedTarget)
	if err != nil {
		fmt.Fprintf(stderr, "migration apply-preflight: %v\n", err)
		return exitError
	}
	gotPlanSHA256, err := planSHA256(plan)
	if err != nil {
		fmt.Fprintf(stderr, "migration apply-preflight: %v\n", err)
		return exitError
	}
	if planContainsRollbackStep(plan) {
		if !allowRollback {
			fmt.Fprintf(stderr, "migration apply-preflight: %v\n", fmt.Errorf("%w: rollback/down migration plan requires --allow-rollback", ErrMigrationApplyRollbackConfirmation))
			return exitError
		}
		if confirmedPlanSHA256 == "" && trimmedPlanArtifactPath == "" {
			fmt.Fprintf(stderr, "migration apply-preflight: %v\n", fmt.Errorf("%w: rollback/down migration plan requires --plan-sha256 or --plan-artifact", ErrMigrationApplyPlanConfirmation))
			return exitError
		}
	}
	if confirmedPlanSHA256 != "" && gotPlanSHA256 != confirmedPlanSHA256 {
		fmt.Fprintf(stderr, "migration apply-preflight: %v\n", fmt.Errorf("%w: plan sha256 mismatch: got %s want %s", ErrMigrationApplyPlanConfirmation, gotPlanSHA256, confirmedPlanSHA256))
		return exitError
	}
	if trimmedPlanArtifactPath != "" {
		artifact, err := readMigrationPlanArtifactFile(trimmedPlanArtifactPath)
		if err != nil {
			fmt.Fprintf(stderr, "migration apply-preflight: %v\n", err)
			return exitError
		}
		if artifact.PlanSHA256 != gotPlanSHA256 || !reflect.DeepEqual(artifact.Plan, plan) {
			fmt.Fprintf(stderr, "migration apply-preflight: %v\n", fmt.Errorf("%w: plan artifact does not match requested ledger snapshot and target", ErrMigrationApplyPlanConfirmation))
			return exitError
		}
	}

	return writeJSON(stdout, stderr, migrationApplyPreflight{
		Format:               migrationApplyPreflightFormat,
		TargetVersion:        resolvedTarget,
		TargetLatest:         targetLatest,
		LedgerSnapshotSHA256: sha256Hex(rawLedger),
		PlanSHA256:           gotPlanSHA256,
		Plan:                 plan,
	})
}

func runApplyPreflightStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply-preflight-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var applyPreflightPath string
	flags.StringVar(&applyPreflightPath, "apply-preflight", "", "path to a metadata-only migration apply preflight JSON file")
	flags.Usage = func() { printApplyPreflightStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected apply-preflight-status argument %q\n", flags.Arg(0))
		printApplyPreflightStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(applyPreflightPath) == "" {
		fmt.Fprintln(stderr, "--apply-preflight is required for apply-preflight-status")
		printApplyPreflightStatusUsage(stderr)
		return exitUsage
	}

	preflight, present, err := readMigrationApplyPreflightStatusFile(applyPreflightPath)
	if err != nil {
		fmt.Fprintf(stderr, "migration apply preflight status: %v\n", err)
		return exitError
	}
	status := migrationApplyPreflightStatus{
		Format:  migrationApplyPreflightStatusFormat,
		Present: present,
	}
	if present {
		status.Preflight = &preflight
	}
	return writeJSON(stdout, stderr, status)
}

func readMigrationPlanFromArgs(command string, args []string, stdin io.Reader, stderr io.Writer, printCommandUsage func(io.Writer)) (dbmigrations.Plan, int) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	var snapshotPath string
	var targetVersionText string
	flags.StringVar(&snapshotPath, "ledger-snapshot", "", "path to go-metin2 schema_migrations ledger snapshot JSON, or - for stdin")
	flags.StringVar(&targetVersionText, "target-version", "", "catalog target version for the dry-run plan")
	flags.Usage = func() { printCommandUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return dbmigrations.Plan{}, exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected %s argument %q\n", command, flags.Arg(0))
		printCommandUsage(stderr)
		return dbmigrations.Plan{}, exitUsage
	}
	if strings.TrimSpace(snapshotPath) == "" {
		fmt.Fprintln(stderr, "missing --ledger-snapshot")
		printCommandUsage(stderr)
		return dbmigrations.Plan{}, exitUsage
	}
	if strings.TrimSpace(targetVersionText) == "" {
		fmt.Fprintln(stderr, "missing --target-version")
		printCommandUsage(stderr)
		return dbmigrations.Plan{}, exitUsage
	}
	targetVersion, targetLatest, err := parseTargetVersion(targetVersionText)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --target-version %q: %v\n", targetVersionText, err)
		return dbmigrations.Plan{}, exitUsage
	}

	reader, closeReader, err := openLedgerSnapshotReader(snapshotPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "open ledger snapshot: %v\n", err)
		return dbmigrations.Plan{}, exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	ledger, err := readBoundedLedgerSnapshot(reader)
	if err != nil {
		fmt.Fprintf(stderr, "migration %s: %v\n", command, err)
		return dbmigrations.Plan{}, exitError
	}
	plan, err := planLedgerSnapshot(ledger, targetVersion, targetLatest)
	if err != nil {
		fmt.Fprintf(stderr, "migration %s: %v\n", command, err)
		return dbmigrations.Plan{}, exitError
	}
	return plan, exitOK
}

func runApply(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var driverName string
	var dsn string
	var snapshotPath string
	var targetVersionText string
	var auditFilePath string
	var lockFilePath string
	var planSHA256Text string
	var planArtifactPath string
	var applyPreflightPath string
	var allowRollback bool
	flags.StringVar(&driverName, "driver", "", "database/sql driver name for the migration target")
	flags.StringVar(&dsn, "dsn", "", "database/sql DSN for the migration target")
	flags.StringVar(&snapshotPath, "ledger-snapshot", "", "path to go-metin2 schema_migrations ledger snapshot JSON, or - for stdin")
	flags.StringVar(&targetVersionText, "target-version", "", "catalog target version to apply")
	flags.StringVar(&auditFilePath, "audit-file", "", "optional path for an exclusive metadata-only apply audit JSON file")
	flags.StringVar(&lockFilePath, "lock-file", "", "optional path for an exclusive local migration apply lock file")
	flags.StringVar(&planSHA256Text, "plan-sha256", "", "optional SHA-256 of the metadata-only dry-run plan JSON that must match before applying")
	flags.StringVar(&planArtifactPath, "plan-artifact", "", "optional path to a metadata-only migration plan artifact that must match before applying")
	flags.StringVar(&applyPreflightPath, "apply-preflight", "", "optional path to a metadata-only migration apply preflight artifact that must match before applying")
	flags.BoolVar(&allowRollback, "allow-rollback", false, "allow apply to execute down/rollback migration steps")
	flags.Usage = func() { printApplyUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected apply argument %q\n", flags.Arg(0))
		printApplyUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(driverName) == "" || strings.TrimSpace(dsn) == "" {
		fmt.Fprintln(stderr, "--driver and --dsn are required for apply")
		printApplyUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(snapshotPath) == "" {
		fmt.Fprintln(stderr, "--ledger-snapshot is required for apply")
		printApplyUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(targetVersionText) == "" {
		fmt.Fprintln(stderr, "missing --target-version")
		printApplyUsage(stderr)
		return exitUsage
	}
	targetVersion, targetLatest, err := parseTargetVersion(targetVersionText)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --target-version %q: %v\n", targetVersionText, err)
		return exitUsage
	}
	resolvedTarget, err := resolveTargetVersion(targetVersion, targetLatest)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}
	confirmedPlanSHA256 := ""
	trimmedPlanSHA256 := strings.TrimSpace(planSHA256Text)
	trimmedPlanArtifactPath := strings.TrimSpace(planArtifactPath)
	trimmedApplyPreflightPath := strings.TrimSpace(applyPreflightPath)
	if trimmedPlanSHA256 != "" && trimmedPlanArtifactPath != "" {
		fmt.Fprintln(stderr, "--plan-sha256 and --plan-artifact cannot be used together")
		printApplyUsage(stderr)
		return exitUsage
	}
	if trimmedApplyPreflightPath != "" && (trimmedPlanSHA256 != "" || trimmedPlanArtifactPath != "") {
		fmt.Fprintln(stderr, "--apply-preflight cannot be used together with --plan-sha256 or --plan-artifact")
		printApplyUsage(stderr)
		return exitUsage
	}
	if trimmedPlanSHA256 != "" {
		confirmedPlanSHA256, err = parsePlanSHA256(trimmedPlanSHA256)
		if err != nil {
			fmt.Fprintf(stderr, "invalid --plan-sha256 %q: %v\n", planSHA256Text, err)
			return exitUsage
		}
	}

	reader, closeReader, err := openLedgerSnapshotReader(snapshotPath, stdin)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "open ledger snapshot: %v", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	rawLedger, err := readBoundedLedgerSnapshot(reader)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}
	ledger, err := dbmigrations.ReadJSONLedgerSnapshot(bytes.NewReader(rawLedger))
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}
	plan, err := dbmigrations.PlanToVersion(ledger, resolvedTarget)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}
	gotPlanSHA256, err := planSHA256(plan)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}
	containsRollback := planContainsRollbackStep(plan)
	if containsRollback {
		if !allowRollback {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", fmt.Errorf("%w: rollback/down migration plan requires --allow-rollback", ErrMigrationApplyRollbackConfirmation))
			return exitError
		}
	}
	if confirmedPlanSHA256 != "" && gotPlanSHA256 != confirmedPlanSHA256 {
		writeMigrationCommandError(stderr, dsn, "migration apply: %v", fmt.Errorf("%w: plan sha256 mismatch: got %s want %s", ErrMigrationApplyPlanConfirmation, gotPlanSHA256, confirmedPlanSHA256))
		return exitError
	}
	if trimmedPlanArtifactPath != "" {
		artifact, err := readMigrationPlanArtifactFile(trimmedPlanArtifactPath)
		if err != nil {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
			return exitError
		}
		if artifact.PlanSHA256 != gotPlanSHA256 || !reflect.DeepEqual(artifact.Plan, plan) {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", fmt.Errorf("%w: plan artifact does not match requested ledger snapshot and target", ErrMigrationApplyPlanConfirmation))
			return exitError
		}
		confirmedPlanSHA256 = artifact.PlanSHA256
	}
	if trimmedApplyPreflightPath != "" {
		preflight, _, err := readMigrationApplyPreflightPath(trimmedApplyPreflightPath, false)
		if err != nil {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
			return exitError
		}
		if err := validateMigrationApplyPreflightForApply(preflight, rawLedger, plan, resolvedTarget, targetLatest); err != nil {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
			return exitError
		}
		confirmedPlanSHA256 = preflight.PlanSHA256
	}
	if containsRollback && confirmedPlanSHA256 == "" {
		writeMigrationCommandError(stderr, dsn, "migration apply: %v", fmt.Errorf("%w: rollback/down migration plan requires --plan-sha256, --plan-artifact, or --apply-preflight", ErrMigrationApplyPlanConfirmation))
		return exitError
	}

	var lockFile *migrationApplyLockFile
	if strings.TrimSpace(lockFilePath) != "" {
		hostname, hostErr := os.Hostname()
		if hostErr != nil {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", fmt.Errorf("%w: resolve local hostname: %v", ErrMigrationApplyLock, hostErr))
			return exitError
		}
		hostname = strings.TrimSpace(hostname)
		if hostname == "" {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", fmt.Errorf("%w: local hostname is empty", ErrMigrationApplyLock))
			return exitError
		}
		identity := buildinfo.Current()
		lockFile, err = createMigrationApplyLockFile(lockFilePath, migrationApplyLock{
			Format:               migrationApplyLockFormat,
			CreatedAt:            time.Now().UTC().Format(time.RFC3339Nano),
			PID:                  os.Getpid(),
			Hostname:             hostname,
			BuildVersion:         identity.Version,
			BuildCommit:          identity.Commit,
			BuildDate:            identity.BuildDate,
			Driver:               strings.TrimSpace(driverName),
			DSNConfigured:        strings.TrimSpace(dsn) != "",
			TargetVersion:        resolvedTarget,
			TargetLatest:         targetLatest,
			PlanSHA256:           gotPlanSHA256,
			ConfirmedPlanSHA256:  confirmedPlanSHA256,
			LedgerSnapshotSHA256: sha256Hex(rawLedger),
		})
		if err != nil {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
			return exitError
		}
		defer lockFile.Release()
	}

	var auditFile *migrationApplyAuditFile
	if strings.TrimSpace(auditFilePath) != "" {
		if len(plan.Pending) == 0 {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", fmt.Errorf("%w: audit file requires at least one applied migration", ErrMigrationApplyAudit))
			return exitError
		}
		auditFile, err = createMigrationApplyAuditFile(auditFilePath)
		if err != nil {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
			return exitError
		}
		defer auditFile.Discard()
	}

	db, err := sql.Open(strings.TrimSpace(driverName), strings.TrimSpace(dsn))
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration apply: open database driver %q: %v", strings.TrimSpace(driverName), err)
		return exitError
	}
	defer db.Close()

	result, err := dbmigrations.ApplyToVersion(context.Background(), db, ledger, resolvedTarget)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}
	if auditFile != nil {
		if err := auditFile.Commit(migrationApplyAudit{
			Format:               migrationApplyAuditFormat,
			AppliedAt:            time.Now().UTC().Format(time.RFC3339Nano),
			Driver:               strings.TrimSpace(driverName),
			DSNConfigured:        strings.TrimSpace(dsn) != "",
			TargetVersion:        resolvedTarget,
			TargetLatest:         targetLatest,
			PlanSHA256:           gotPlanSHA256,
			ConfirmedPlanSHA256:  confirmedPlanSHA256,
			LedgerSnapshotSHA256: sha256Hex(rawLedger),
			Result:               result,
		}); err != nil {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
			return exitError
		}
	}
	return writeJSON(stdout, stderr, result)
}

func runApplyLockStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply-lock-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var lockFilePath string
	flags.StringVar(&lockFilePath, "lock-file", "", "path to the local migration apply lock file")
	flags.Usage = func() { printApplyLockStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected apply-lock-status argument %q\n", flags.Arg(0))
		printApplyLockStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(lockFilePath) == "" {
		fmt.Fprintln(stderr, "--lock-file is required for apply-lock-status")
		printApplyLockStatusUsage(stderr)
		return exitUsage
	}

	status, err := inspectMigrationApplyLockStatus(lockFilePath)
	if err != nil {
		fmt.Fprintf(stderr, "migration apply lock status: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, status)
}

func runApplyLockAside(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply-lock-aside", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var lockFilePath string
	var confirmAsideRename bool
	flags.StringVar(&lockFilePath, "lock-file", "", "path to the local migration apply lock file")
	flags.BoolVar(&confirmAsideRename, "i-confirm-lab-aside-rename", false, "confirm lab stale-lock aside-rename after recomputing the candidate gate")
	flags.Usage = func() { printApplyLockAsideUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected apply-lock-aside argument %q\n", flags.Arg(0))
		printApplyLockAsideUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(lockFilePath) == "" {
		fmt.Fprintln(stderr, "--lock-file is required for apply-lock-aside")
		printApplyLockAsideUsage(stderr)
		return exitUsage
	}
	if !confirmAsideRename {
		fmt.Fprintln(stderr, "--i-confirm-lab-aside-rename is required for apply-lock-aside")
		printApplyLockAsideUsage(stderr)
		return exitUsage
	}

	status, err := inspectMigrationApplyLockStatus(lockFilePath)
	if err != nil {
		fmt.Fprintf(stderr, "migration apply lock aside: %v\n", err)
		return exitError
	}
	if !status.Present || status.Lock == nil {
		fmt.Fprintf(stderr, "migration apply lock aside: %v\n", fmt.Errorf("%w: lock file is not present: %s", ErrMigrationApplyLockAside, strings.TrimSpace(lockFilePath)))
		return exitError
	}
	if status.ManualClearCandidate == nil || !*status.ManualClearCandidate {
		fmt.Fprintf(stderr, "migration apply lock aside: %v\n", fmt.Errorf("%w: lock is not a lab manual-clear candidate", ErrMigrationApplyLockAside))
		return exitError
	}

	now := applyLockStatusNow().UTC()
	asidePath := lockAsidePath(lockFilePath, now)
	if _, err := os.Lstat(asidePath); err == nil {
		fmt.Fprintf(stderr, "migration apply lock aside: %v\n", fmt.Errorf("%w: aside path already exists: %s", ErrMigrationApplyLockAside, asidePath))
		return exitError
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "migration apply lock aside: %v\n", fmt.Errorf("%w: stat aside path: %v", ErrMigrationApplyLockAside, err))
		return exitError
	}

	if err := os.Rename(strings.TrimSpace(lockFilePath), asidePath); err != nil {
		fmt.Fprintf(stderr, "migration apply lock aside: %v\n", fmt.Errorf("%w: rename lock file: %v", ErrMigrationApplyLockAside, err))
		return exitError
	}

	result := migrationApplyLockAside{
		Format:               migrationApplyLockAsideFormat,
		LockFile:             strings.TrimSpace(lockFilePath),
		AsidePath:            asidePath,
		RenamedAt:            now.Format(time.RFC3339Nano),
		Lock:                 status.Lock,
		HolderPIDAlive:       status.HolderPIDAlive,
		HolderPIDCheck:       status.HolderPIDCheck,
		HolderHostnameLocal:  status.HolderHostnameLocal,
		HolderHostnameCheck:  status.HolderHostnameCheck,
		HolderBuildMatches:   status.HolderBuildMatches,
		HolderBuildCheck:     status.HolderBuildCheck,
		LockAgeSeconds:       status.LockAgeSeconds,
		LockAgeCheck:         status.LockAgeCheck,
		ManualClearCandidate: status.ManualClearCandidate,
		ManualClearCheck:     status.ManualClearCheck,
	}
	return writeJSON(stdout, stderr, result)
}

func runApplyAuditStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply-audit-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var auditFilePath string
	flags.StringVar(&auditFilePath, "audit-file", "", "path to a migration apply audit file")
	flags.Usage = func() { printApplyAuditStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected apply-audit-status argument %q\n", flags.Arg(0))
		printApplyAuditStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(auditFilePath) == "" {
		fmt.Fprintln(stderr, "--audit-file is required for apply-audit-status")
		printApplyAuditStatusUsage(stderr)
		return exitUsage
	}

	audit, present, err := readMigrationApplyAuditFile(auditFilePath)
	if err != nil {
		fmt.Fprintf(stderr, "migration apply audit status: %v\n", err)
		return exitError
	}
	status := migrationApplyAuditStatus{
		Format:  migrationApplyAuditStatusFormat,
		Present: present,
	}
	if present {
		status.Audit = &audit
	}
	return writeJSON(stdout, stderr, status)
}

const migrationApplyAuditFormat = "go-metin2-migration-apply-audit-v1"

const migrationApplyLockFormat = "go-metin2-migration-apply-lock-v1"

const migrationApplyLockStatusFormat = "go-metin2-migration-apply-lock-status-v1"

const migrationApplyLockAsideFormat = "go-metin2-migration-apply-lock-aside-v1"

const migrationApplyLockHolderPIDCheck = "local_signal_0"

const migrationApplyLockHolderHostnameCheck = "local_os_hostname"

const migrationApplyLockHolderBuildCheck = "local_buildinfo_current"

const migrationApplyLockAgeCheck = "local_wall_clock"

// migrationApplyLockManualClearCheck is the fixed probe name for the lab
// stale-lock recovery policy. A true candidate is triage evidence only and
// never authorizes automatic lock deletion.
const migrationApplyLockManualClearCheck = "lab_stale_lock_policy_v1"

// labManualClearMinAgeSeconds is the minimum advisory wall-clock age required
// by the lab stale-lock recovery policy before a leftover lock may be
// considered for operator aside-rename. It is not an auto-expiry threshold.
const labManualClearMinAgeSeconds int64 = 3600

const migrationApplyAuditStatusFormat = "go-metin2-migration-apply-audit-status-v1"

// applyLockStatusNow is the wall clock used by apply-lock-status age triage and
// apply-lock-aside destination timestamps. Tests may override it; production
// keeps time.Now.
var applyLockStatusNow = time.Now

var ErrMigrationApplyAudit = errors.New("migration apply audit failed")

var ErrMigrationApplyLock = errors.New("migration apply lock failed")

var ErrMigrationApplyLockAside = errors.New("migration apply lock aside failed")

var ErrMigrationApplyPlanConfirmation = errors.New("migration apply plan confirmation failed")

var ErrMigrationApplyPreflight = errors.New("migration apply preflight failed")

var ErrMigrationApplyRollbackConfirmation = errors.New("migration apply rollback confirmation failed")

type migrationApplyAudit struct {
	Format               string                   `json:"format"`
	AppliedAt            string                   `json:"applied_at"`
	Driver               string                   `json:"driver"`
	DSNConfigured        bool                     `json:"dsn_configured"`
	TargetVersion        int                      `json:"target_version"`
	TargetLatest         bool                     `json:"target_latest"`
	PlanSHA256           string                   `json:"plan_sha256"`
	ConfirmedPlanSHA256  string                   `json:"confirmed_plan_sha256,omitempty"`
	LedgerSnapshotSHA256 string                   `json:"ledger_snapshot_sha256"`
	Result               dbmigrations.ApplyResult `json:"result"`
}

type migrationApplyLock struct {
	Format               string `json:"format"`
	CreatedAt            string `json:"created_at"`
	PID                  int    `json:"pid"`
	Hostname             string `json:"hostname"`
	BuildVersion         string `json:"build_version"`
	BuildCommit          string `json:"build_commit"`
	BuildDate            string `json:"build_date"`
	Driver               string `json:"driver"`
	DSNConfigured        bool   `json:"dsn_configured"`
	TargetVersion        int    `json:"target_version"`
	TargetLatest         bool   `json:"target_latest"`
	PlanSHA256           string `json:"plan_sha256"`
	ConfirmedPlanSHA256  string `json:"confirmed_plan_sha256,omitempty"`
	LedgerSnapshotSHA256 string `json:"ledger_snapshot_sha256"`
}

type migrationApplyLockStatus struct {
	Format               string              `json:"format"`
	Present              bool                `json:"present"`
	Lock                 *migrationApplyLock `json:"lock,omitempty"`
	HolderPIDAlive       *bool               `json:"holder_pid_alive,omitempty"`
	HolderPIDCheck       string              `json:"holder_pid_check,omitempty"`
	HolderHostnameLocal  *bool               `json:"holder_hostname_local,omitempty"`
	HolderHostnameCheck  string              `json:"holder_hostname_check,omitempty"`
	HolderBuildMatches   *bool               `json:"holder_build_matches,omitempty"`
	HolderBuildCheck     string              `json:"holder_build_check,omitempty"`
	LockAgeSeconds       *int64              `json:"lock_age_seconds,omitempty"`
	LockAgeCheck         string              `json:"lock_age_check,omitempty"`
	ManualClearCandidate *bool               `json:"manual_clear_candidate,omitempty"`
	ManualClearCheck     string              `json:"manual_clear_check,omitempty"`
}

type migrationApplyLockAside struct {
	Format               string              `json:"format"`
	LockFile             string              `json:"lock_file"`
	AsidePath            string              `json:"aside_path"`
	RenamedAt            string              `json:"renamed_at"`
	Lock                 *migrationApplyLock `json:"lock,omitempty"`
	HolderPIDAlive       *bool               `json:"holder_pid_alive,omitempty"`
	HolderPIDCheck       string              `json:"holder_pid_check,omitempty"`
	HolderHostnameLocal  *bool               `json:"holder_hostname_local,omitempty"`
	HolderHostnameCheck  string              `json:"holder_hostname_check,omitempty"`
	HolderBuildMatches   *bool               `json:"holder_build_matches,omitempty"`
	HolderBuildCheck     string              `json:"holder_build_check,omitempty"`
	LockAgeSeconds       *int64              `json:"lock_age_seconds,omitempty"`
	LockAgeCheck         string              `json:"lock_age_check,omitempty"`
	ManualClearCandidate *bool               `json:"manual_clear_candidate,omitempty"`
	ManualClearCheck     string              `json:"manual_clear_check,omitempty"`
}

// inspectMigrationApplyLockStatus validates and triages a local apply lock without
// mutating it. Missing paths return present=false; malformed/symlink/oversized
// locks fail closed.
func inspectMigrationApplyLockStatus(lockFilePath string) (migrationApplyLockStatus, error) {
	lock, present, err := readMigrationApplyLockFile(lockFilePath)
	if err != nil {
		return migrationApplyLockStatus{}, err
	}
	status := migrationApplyLockStatus{
		Format:  migrationApplyLockStatusFormat,
		Present: present,
	}
	if !present {
		return status, nil
	}
	status.Lock = &lock
	alive, err := localProcessExists(lock.PID)
	if err != nil {
		return migrationApplyLockStatus{}, err
	}
	status.HolderPIDAlive = &alive
	status.HolderPIDCheck = migrationApplyLockHolderPIDCheck
	hostnameLocal, err := localHostnameMatches(lock.Hostname)
	if err != nil {
		return migrationApplyLockStatus{}, err
	}
	status.HolderHostnameLocal = &hostnameLocal
	status.HolderHostnameCheck = migrationApplyLockHolderHostnameCheck
	buildMatches, err := localBuildIdentityMatches(lock.BuildVersion, lock.BuildCommit, lock.BuildDate)
	if err != nil {
		return migrationApplyLockStatus{}, err
	}
	status.HolderBuildMatches = &buildMatches
	status.HolderBuildCheck = migrationApplyLockHolderBuildCheck
	ageSeconds, err := lockAgeSeconds(lock.CreatedAt, applyLockStatusNow())
	if err != nil {
		return migrationApplyLockStatus{}, err
	}
	status.LockAgeSeconds = &ageSeconds
	status.LockAgeCheck = migrationApplyLockAgeCheck
	candidate := labManualClearCandidate(alive, hostnameLocal, buildMatches, ageSeconds)
	status.ManualClearCandidate = &candidate
	status.ManualClearCheck = migrationApplyLockManualClearCheck
	return status, nil
}

func lockAsidePath(lockFilePath string, now time.Time) string {
	return strings.TrimSpace(lockFilePath) + ".stale-" + now.UTC().Format("20060102T150405Z")
}

// localProcessExists reports whether pid appears in the local process table.
// Signal 0 is used as a non-mutating existence probe. ESRCH means absent;
// success or EPERM means the pid still exists. Other probe errors fail closed.
func localProcessExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("%w: lock pid must be positive", ErrMigrationApplyLock)
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if errors.Is(err, syscall.EPERM) {
		return true, nil
	}
	return false, fmt.Errorf("%w: probe lock holder pid %d: %v", ErrMigrationApplyLock, pid, err)
}

// localHostnameMatches reports whether lockHostname equals the current host's
// os.Hostname() value after trimming. Lookup failures fail closed so operators
// never treat an inconclusive probe as authorization to delete a lock.
func localHostnameMatches(lockHostname string) (bool, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return false, fmt.Errorf("%w: resolve local hostname: %v", ErrMigrationApplyLock, err)
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return false, fmt.Errorf("%w: local hostname is empty", ErrMigrationApplyLock)
	}
	return hostname == strings.TrimSpace(lockHostname), nil
}

// localBuildIdentityMatches reports whether the lock's stamped build identity
// equals buildinfo.Current() on the inspecting binary after trimming each
// field. Empty inspecting-binary identity fails closed so operators never
// treat an inconclusive probe as authorization to delete a lock.
func localBuildIdentityMatches(lockVersion, lockCommit, lockBuildDate string) (bool, error) {
	identity := buildinfo.Current()
	version := strings.TrimSpace(identity.Version)
	commit := strings.TrimSpace(identity.Commit)
	buildDate := strings.TrimSpace(identity.BuildDate)
	if version == "" || commit == "" || buildDate == "" {
		return false, fmt.Errorf("%w: local build identity is incomplete", ErrMigrationApplyLock)
	}
	return version == strings.TrimSpace(lockVersion) &&
		commit == strings.TrimSpace(lockCommit) &&
		buildDate == strings.TrimSpace(lockBuildDate), nil
}

// lockAgeSeconds reports the non-negative whole-second floor of the wall-clock
// age between lock created_at and now. Future-dated locks clamp to 0 so operators
// never see a negative age from clock skew; parse failures fail closed.
func lockAgeSeconds(createdAt string, now time.Time) (int64, error) {
	created, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(createdAt))
	if err != nil {
		return 0, fmt.Errorf("%w: invalid lock created_at: %v", ErrMigrationApplyLock, err)
	}
	age := now.UTC().Sub(created.UTC())
	if age < 0 {
		return 0, nil
	}
	return int64(age / time.Second), nil
}

// labManualClearCandidate reports whether the lab stale-lock recovery policy
// considers a leftover lock an advisory candidate for operator aside-rename.
// All four gates must hold: absent local PID, local hostname, matching build
// identity, and age at least one hour. A true result never deletes the lock.
func labManualClearCandidate(pidAlive, hostnameLocal, buildMatches bool, ageSeconds int64) bool {
	return !pidAlive && hostnameLocal && buildMatches && ageSeconds >= labManualClearMinAgeSeconds
}

type migrationApplyAuditStatus struct {
	Format  string               `json:"format"`
	Present bool                 `json:"present"`
	Audit   *migrationApplyAudit `json:"audit,omitempty"`
}

type migrationApplyLockFile struct {
	path string
	file *os.File
}

func createMigrationApplyLockFile(path string, lock migrationApplyLock) (*migrationApplyLockFile, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, nil
	}
	file, err := os.OpenFile(trimmed, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w: lock file already exists: %s", ErrMigrationApplyLock, trimmed)
		}
		return nil, fmt.Errorf("%w: create lock file: %v", ErrMigrationApplyLock, err)
	}
	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		_ = file.Close()
		_ = os.Remove(trimmed)
		return nil, fmt.Errorf("%w: marshal lock file: %v", ErrMigrationApplyLock, err)
	}
	raw = append(raw, '\n')
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		_ = os.Remove(trimmed)
		return nil, fmt.Errorf("%w: write lock file: %v", ErrMigrationApplyLock, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(trimmed)
		return nil, fmt.Errorf("%w: sync lock file: %v", ErrMigrationApplyLock, err)
	}
	return &migrationApplyLockFile{path: trimmed, file: file}, nil
}

func (f *migrationApplyLockFile) Release() {
	if f == nil {
		return
	}
	if f.file != nil {
		_ = f.file.Close()
		f.file = nil
	}
	if f.path != "" {
		_ = os.Remove(f.path)
		f.path = ""
	}
}

func readMigrationApplyLockFile(path string) (migrationApplyLock, bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return migrationApplyLock{}, false, fmt.Errorf("%w: lock file path is required", ErrMigrationApplyLock)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return migrationApplyLock{}, false, nil
		}
		return migrationApplyLock{}, false, fmt.Errorf("%w: stat lock file: %v", ErrMigrationApplyLock, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return migrationApplyLock{}, false, fmt.Errorf("%w: lock file must not be a symlink: %s", ErrMigrationApplyLock, trimmed)
	}
	if !info.Mode().IsRegular() {
		return migrationApplyLock{}, false, fmt.Errorf("%w: lock file must be a regular file: %s", ErrMigrationApplyLock, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return migrationApplyLock{}, false, fmt.Errorf("%w: read lock file: %v", ErrMigrationApplyLock, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return migrationApplyLock{}, false, fmt.Errorf("%w: stat opened lock file: %v", ErrMigrationApplyLock, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return migrationApplyLock{}, false, fmt.Errorf("%w: opened lock file must be a regular file: %s", ErrMigrationApplyLock, trimmed)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxMigrationApplyLockBytes+1))
	if err != nil {
		return migrationApplyLock{}, false, fmt.Errorf("%w: read lock file: %v", ErrMigrationApplyLock, err)
	}
	if len(raw) > maxMigrationApplyLockBytes {
		return migrationApplyLock{}, false, fmt.Errorf("%w: lock file exceeds %d bytes", ErrMigrationApplyLock, maxMigrationApplyLockBytes)
	}
	if !utf8.Valid(raw) {
		return migrationApplyLock{}, false, fmt.Errorf("%w: lock file is not valid UTF-8", ErrMigrationApplyLock)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return migrationApplyLock{}, false, fmt.Errorf("%w: lock file is empty", ErrMigrationApplyLock)
	}

	var lock migrationApplyLock
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil {
		return migrationApplyLock{}, false, fmt.Errorf("%w: decode lock file: %v", ErrMigrationApplyLock, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return migrationApplyLock{}, false, fmt.Errorf("%w: lock file has trailing JSON", ErrMigrationApplyLock)
	}
	normalized, err := normalizeMigrationApplyLock(lock)
	if err != nil {
		return migrationApplyLock{}, false, err
	}
	return normalized, true, nil
}

func normalizeMigrationApplyLock(lock migrationApplyLock) (migrationApplyLock, error) {
	if lock.Format != migrationApplyLockFormat {
		return migrationApplyLock{}, fmt.Errorf("%w: unsupported lock format %q", ErrMigrationApplyLock, lock.Format)
	}
	createdAt := strings.TrimSpace(lock.CreatedAt)
	if createdAt == "" {
		return migrationApplyLock{}, fmt.Errorf("%w: lock created_at is required", ErrMigrationApplyLock)
	}
	if _, err := time.Parse(time.RFC3339Nano, createdAt); err != nil {
		return migrationApplyLock{}, fmt.Errorf("%w: invalid lock created_at: %v", ErrMigrationApplyLock, err)
	}
	lock.CreatedAt = createdAt
	if lock.PID <= 0 {
		return migrationApplyLock{}, fmt.Errorf("%w: lock pid must be positive", ErrMigrationApplyLock)
	}
	hostname := strings.TrimSpace(lock.Hostname)
	if hostname == "" {
		return migrationApplyLock{}, fmt.Errorf("%w: lock hostname is required", ErrMigrationApplyLock)
	}
	lock.Hostname = hostname
	buildVersion := strings.TrimSpace(lock.BuildVersion)
	if buildVersion == "" {
		return migrationApplyLock{}, fmt.Errorf("%w: lock build_version is required", ErrMigrationApplyLock)
	}
	lock.BuildVersion = buildVersion
	buildCommit := strings.TrimSpace(lock.BuildCommit)
	if buildCommit == "" {
		return migrationApplyLock{}, fmt.Errorf("%w: lock build_commit is required", ErrMigrationApplyLock)
	}
	lock.BuildCommit = buildCommit
	buildDate := strings.TrimSpace(lock.BuildDate)
	if buildDate == "" {
		return migrationApplyLock{}, fmt.Errorf("%w: lock build_date is required", ErrMigrationApplyLock)
	}
	lock.BuildDate = buildDate
	lock.Driver = strings.TrimSpace(lock.Driver)
	if lock.Driver == "" {
		return migrationApplyLock{}, fmt.Errorf("%w: lock driver is required", ErrMigrationApplyLock)
	}
	if !lock.DSNConfigured {
		return migrationApplyLock{}, fmt.Errorf("%w: lock dsn_configured must be true for apply locks", ErrMigrationApplyLock)
	}
	if lock.TargetVersion < 0 {
		return migrationApplyLock{}, fmt.Errorf("%w: lock target_version must be non-negative", ErrMigrationApplyLock)
	}
	if lock.TargetLatest && lock.TargetVersion == 0 {
		return migrationApplyLock{}, fmt.Errorf("%w: latest target lock must name a positive resolved target_version", ErrMigrationApplyLock)
	}
	planSHA256, err := parsePlanSHA256(lock.PlanSHA256)
	if err != nil {
		return migrationApplyLock{}, fmt.Errorf("%w: invalid lock plan_sha256: %v", ErrMigrationApplyLock, err)
	}
	lock.PlanSHA256 = planSHA256
	ledgerSnapshotSHA256, err := parsePlanSHA256(lock.LedgerSnapshotSHA256)
	if err != nil {
		return migrationApplyLock{}, fmt.Errorf("%w: invalid lock ledger_snapshot_sha256: %v", ErrMigrationApplyLock, err)
	}
	lock.LedgerSnapshotSHA256 = ledgerSnapshotSHA256
	if strings.TrimSpace(lock.ConfirmedPlanSHA256) != "" {
		confirmedPlanSHA256, err := parsePlanSHA256(lock.ConfirmedPlanSHA256)
		if err != nil {
			return migrationApplyLock{}, fmt.Errorf("%w: invalid lock confirmed_plan_sha256: %v", ErrMigrationApplyLock, err)
		}
		lock.ConfirmedPlanSHA256 = confirmedPlanSHA256
	}
	return lock, nil
}

type migrationApplyAuditFile struct {
	path string
	file *os.File
}

func createMigrationApplyAuditFile(path string) (*migrationApplyAuditFile, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, nil
	}
	file, err := os.OpenFile(trimmed, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w: audit file already exists: %s", ErrMigrationApplyAudit, trimmed)
		}
		return nil, fmt.Errorf("%w: create audit file: %v", ErrMigrationApplyAudit, err)
	}
	return &migrationApplyAuditFile{path: trimmed, file: file}, nil
}

func readMigrationApplyAuditFile(path string) (migrationApplyAudit, bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: audit file path is required", ErrMigrationApplyAudit)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return migrationApplyAudit{}, false, nil
		}
		return migrationApplyAudit{}, false, fmt.Errorf("%w: stat audit file: %v", ErrMigrationApplyAudit, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: audit file must not be a symlink: %s", ErrMigrationApplyAudit, trimmed)
	}
	if !info.Mode().IsRegular() {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: audit file must be a regular file: %s", ErrMigrationApplyAudit, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: read audit file: %v", ErrMigrationApplyAudit, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: stat opened audit file: %v", ErrMigrationApplyAudit, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: opened audit file must be a regular file: %s", ErrMigrationApplyAudit, trimmed)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxMigrationApplyAuditBytes+1))
	if err != nil {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: read audit file: %v", ErrMigrationApplyAudit, err)
	}
	if len(raw) > maxMigrationApplyAuditBytes {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: audit file exceeds %d bytes", ErrMigrationApplyAudit, maxMigrationApplyAuditBytes)
	}
	if !utf8.Valid(raw) {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: audit file is not valid UTF-8", ErrMigrationApplyAudit)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: audit file is empty", ErrMigrationApplyAudit)
	}

	var audit migrationApplyAudit
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&audit); err != nil {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: decode audit file: %v", ErrMigrationApplyAudit, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return migrationApplyAudit{}, false, fmt.Errorf("%w: audit file has trailing JSON", ErrMigrationApplyAudit)
	}
	normalized, err := normalizeMigrationApplyAudit(audit)
	if err != nil {
		return migrationApplyAudit{}, false, err
	}
	return normalized, true, nil
}

func normalizeMigrationApplyAudit(audit migrationApplyAudit) (migrationApplyAudit, error) {
	if audit.Format != migrationApplyAuditFormat {
		return migrationApplyAudit{}, fmt.Errorf("%w: unsupported audit format %q", ErrMigrationApplyAudit, audit.Format)
	}
	appliedAt := strings.TrimSpace(audit.AppliedAt)
	if appliedAt == "" {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit applied_at is required", ErrMigrationApplyAudit)
	}
	if _, err := time.Parse(time.RFC3339Nano, appliedAt); err != nil {
		return migrationApplyAudit{}, fmt.Errorf("%w: invalid audit applied_at: %v", ErrMigrationApplyAudit, err)
	}
	audit.AppliedAt = appliedAt
	audit.Driver = strings.TrimSpace(audit.Driver)
	if audit.Driver == "" {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit driver is required", ErrMigrationApplyAudit)
	}
	if !audit.DSNConfigured {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit dsn_configured must be true for apply audit files", ErrMigrationApplyAudit)
	}
	if audit.TargetVersion < 0 {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit target_version must be non-negative", ErrMigrationApplyAudit)
	}
	if audit.TargetLatest && audit.TargetVersion == 0 {
		return migrationApplyAudit{}, fmt.Errorf("%w: latest target audit must name a positive resolved target_version", ErrMigrationApplyAudit)
	}
	parsedPlanSHA256, err := parsePlanSHA256(audit.PlanSHA256)
	if err != nil {
		return migrationApplyAudit{}, fmt.Errorf("%w: invalid audit plan_sha256: %v", ErrMigrationApplyAudit, err)
	}
	audit.PlanSHA256 = parsedPlanSHA256
	ledgerSnapshotSHA256, err := parsePlanSHA256(audit.LedgerSnapshotSHA256)
	if err != nil {
		return migrationApplyAudit{}, fmt.Errorf("%w: invalid audit ledger_snapshot_sha256: %v", ErrMigrationApplyAudit, err)
	}
	audit.LedgerSnapshotSHA256 = ledgerSnapshotSHA256
	if strings.TrimSpace(audit.ConfirmedPlanSHA256) != "" {
		confirmedPlanSHA256, err := parsePlanSHA256(audit.ConfirmedPlanSHA256)
		if err != nil {
			return migrationApplyAudit{}, fmt.Errorf("%w: invalid audit confirmed_plan_sha256: %v", ErrMigrationApplyAudit, err)
		}
		audit.ConfirmedPlanSHA256 = confirmedPlanSHA256
	}
	if audit.Result.LatestVersion <= 0 {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit result latest_version must be positive", ErrMigrationApplyAudit)
	}
	if audit.Result.PreviousVersion < 0 || audit.Result.CurrentVersion < 0 {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit result versions must be non-negative", ErrMigrationApplyAudit)
	}
	if audit.Result.PreviousVersion > audit.Result.LatestVersion || audit.Result.CurrentVersion > audit.Result.LatestVersion {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit result versions must not exceed latest_version", ErrMigrationApplyAudit)
	}
	if audit.TargetVersion != audit.Result.CurrentVersion {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit target_version %d does not match result current_version %d", ErrMigrationApplyAudit, audit.TargetVersion, audit.Result.CurrentVersion)
	}
	if audit.TargetLatest && audit.TargetVersion != audit.Result.LatestVersion {
		return migrationApplyAudit{}, fmt.Errorf("%w: latest audit target_version %d does not match result latest_version %d", ErrMigrationApplyAudit, audit.TargetVersion, audit.Result.LatestVersion)
	}
	if len(audit.Result.Applied) == 0 {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit result requires at least one applied migration", ErrMigrationApplyAudit)
	}
	for i, step := range audit.Result.Applied {
		if step.Version <= 0 || step.Version > audit.Result.LatestVersion {
			return migrationApplyAudit{}, fmt.Errorf("%w: audit result applied step %d has invalid version %d", ErrMigrationApplyAudit, i+1, step.Version)
		}
		if step.Direction != dbmigrations.DirectionUp && step.Direction != dbmigrations.DirectionDown {
			return migrationApplyAudit{}, fmt.Errorf("%w: audit result applied step %d has invalid direction %q", ErrMigrationApplyAudit, i+1, step.Direction)
		}
		step.Name = strings.TrimSpace(step.Name)
		if step.Name == "" {
			return migrationApplyAudit{}, fmt.Errorf("%w: audit result applied step %d name is required", ErrMigrationApplyAudit, i+1)
		}
		step.Path = strings.TrimSpace(step.Path)
		if step.Path == "" {
			return migrationApplyAudit{}, fmt.Errorf("%w: audit result applied step %d path is required", ErrMigrationApplyAudit, i+1)
		}
		stepSHA256, err := parsePlanSHA256(step.SHA256)
		if err != nil {
			return migrationApplyAudit{}, fmt.Errorf("%w: invalid audit result applied step %d sha256: %v", ErrMigrationApplyAudit, i+1, err)
		}
		step.SHA256 = stepSHA256
		audit.Result.Applied[i] = step
	}
	currentVersion, err := replayMigrationApplyAuditResult(audit.Result)
	if err != nil {
		return migrationApplyAudit{}, err
	}
	if currentVersion != audit.Result.CurrentVersion {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit result current_version %d does not match applied steps ending at %d", ErrMigrationApplyAudit, audit.Result.CurrentVersion, currentVersion)
	}
	planFromAudit := dbmigrations.Plan{
		CurrentVersion: audit.Result.PreviousVersion,
		LatestVersion:  audit.Result.LatestVersion,
		UpToDate:       false,
		Pending:        audit.Result.Applied,
	}
	computedPlanSHA256, err := planSHA256(planFromAudit)
	if err != nil {
		return migrationApplyAudit{}, fmt.Errorf("%w: validate audit plan checksum: %v", ErrMigrationApplyAudit, err)
	}
	if audit.PlanSHA256 != computedPlanSHA256 {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit plan_sha256 mismatch: got %s want %s", ErrMigrationApplyAudit, computedPlanSHA256, audit.PlanSHA256)
	}
	if audit.ConfirmedPlanSHA256 != "" && audit.ConfirmedPlanSHA256 != audit.PlanSHA256 {
		return migrationApplyAudit{}, fmt.Errorf("%w: audit confirmed_plan_sha256 %s does not match plan_sha256 %s", ErrMigrationApplyAudit, audit.ConfirmedPlanSHA256, audit.PlanSHA256)
	}
	return audit, nil
}

func replayMigrationApplyAuditResult(result dbmigrations.ApplyResult) (int, error) {
	currentVersion := result.PreviousVersion
	for i, step := range result.Applied {
		switch step.Direction {
		case dbmigrations.DirectionUp:
			if step.Version != currentVersion+1 {
				return 0, fmt.Errorf("%w: audit result applied up step %d version %d does not continue from %d", ErrMigrationApplyAudit, i+1, step.Version, currentVersion)
			}
			currentVersion = step.Version
		case dbmigrations.DirectionDown:
			if step.Version != currentVersion {
				return 0, fmt.Errorf("%w: audit result applied down step %d version %d does not continue from %d", ErrMigrationApplyAudit, i+1, step.Version, currentVersion)
			}
			currentVersion = step.Version - 1
		default:
			return 0, fmt.Errorf("%w: audit result applied step %d has invalid direction %q", ErrMigrationApplyAudit, i+1, step.Direction)
		}
	}
	return currentVersion, nil
}

func (f *migrationApplyAuditFile) Commit(audit migrationApplyAudit) error {
	if f == nil || f.file == nil {
		return nil
	}
	if len(audit.Result.Applied) == 0 {
		return fmt.Errorf("%w: audit file requires at least one applied migration", ErrMigrationApplyAudit)
	}
	raw, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: marshal audit file: %v", ErrMigrationApplyAudit, err)
	}
	raw = append(raw, '\n')
	if _, err := f.file.Write(raw); err != nil {
		return fmt.Errorf("%w: write audit file: %v", ErrMigrationApplyAudit, err)
	}
	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("%w: sync audit file: %v", ErrMigrationApplyAudit, err)
	}
	if err := f.file.Close(); err != nil {
		return fmt.Errorf("%w: close audit file: %v", ErrMigrationApplyAudit, err)
	}
	f.file = nil
	return nil
}

func (f *migrationApplyAuditFile) Discard() {
	if f == nil {
		return
	}
	if f.file != nil {
		_ = f.file.Close()
		f.file = nil
		_ = os.Remove(f.path)
	}
}

func writeMigrationCommandError(stderr io.Writer, dsn string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	trimmedDSN := strings.TrimSpace(dsn)
	if trimmedDSN != "" {
		message = strings.ReplaceAll(message, trimmedDSN, "<redacted-dsn>")
	}
	fmt.Fprintln(stderr, message)
}

func openLedgerSnapshotReader(path string, stdin io.Reader) (io.Reader, func(), error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "-" {
		return stdin, nil, nil
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, nil, err
	}
	return file, func() { _ = file.Close() }, nil
}

func parseTargetVersion(value string) (int, bool, error) {
	trimmed := strings.TrimSpace(value)
	if strings.EqualFold(trimmed, "latest") {
		return 0, true, nil
	}
	version, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false, err
	}
	return version, false, nil
}

func readBoundedLedgerSnapshot(reader io.Reader) ([]byte, error) {
	if reader == nil {
		reader = strings.NewReader("")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxLedgerSnapshotBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read ledger snapshot: %w", err)
	}
	if len(raw) > maxLedgerSnapshotBytes {
		return nil, fmt.Errorf("ledger snapshot exceeds %d bytes", maxLedgerSnapshotBytes)
	}
	return raw, nil
}

func readMigrationLedgerSnapshotStatusFile(path string) (dbmigrations.LedgerSnapshot, bool, []byte, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: ledger snapshot path is required", ErrMigrationLedgerSnapshot)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return dbmigrations.LedgerSnapshot{}, false, nil, nil
		}
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: stat ledger snapshot: %v", ErrMigrationLedgerSnapshot, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: ledger snapshot must not be a symlink: %s", ErrMigrationLedgerSnapshot, trimmed)
	}
	if !info.Mode().IsRegular() {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: ledger snapshot must be a regular file: %s", ErrMigrationLedgerSnapshot, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: read ledger snapshot: %v", ErrMigrationLedgerSnapshot, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: stat opened ledger snapshot: %v", ErrMigrationLedgerSnapshot, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: opened ledger snapshot must be a regular file: %s", ErrMigrationLedgerSnapshot, trimmed)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxLedgerSnapshotBytes+1))
	if err != nil {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: read ledger snapshot: %v", ErrMigrationLedgerSnapshot, err)
	}
	if len(raw) > maxLedgerSnapshotBytes {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: ledger snapshot exceeds %d bytes", ErrMigrationLedgerSnapshot, maxLedgerSnapshotBytes)
	}
	entries, err := dbmigrations.ReadJSONLedgerSnapshot(bytes.NewReader(raw))
	if err != nil {
		return dbmigrations.LedgerSnapshot{}, false, nil, fmt.Errorf("%w: %v", ErrMigrationLedgerSnapshot, err)
	}
	return dbmigrations.LedgerSnapshot{Format: dbmigrations.LedgerSnapshotFormat, Entries: entries}, true, raw, nil
}

func readMigrationPlanArtifactFile(path string) (migrationPlanArtifact, error) {
	artifact, _, err := readMigrationPlanArtifactPath(path, false)
	return artifact, err
}

func readMigrationPlanArtifactStatusFile(path string) (migrationPlanArtifact, bool, error) {
	return readMigrationPlanArtifactPath(path, true)
}

func readMigrationPlanArtifactPath(path string, reportMissing bool) (migrationPlanArtifact, bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: plan artifact path is required", ErrMigrationApplyPlanConfirmation)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if reportMissing && errors.Is(err, os.ErrNotExist) {
			return migrationPlanArtifact{}, false, nil
		}
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: stat plan artifact: %v", ErrMigrationApplyPlanConfirmation, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: plan artifact must not be a symlink: %s", ErrMigrationApplyPlanConfirmation, trimmed)
	}
	if !info.Mode().IsRegular() {
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: plan artifact must be a regular file: %s", ErrMigrationApplyPlanConfirmation, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: read plan artifact: %v", ErrMigrationApplyPlanConfirmation, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: stat opened plan artifact: %v", ErrMigrationApplyPlanConfirmation, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: opened plan artifact must be a regular file: %s", ErrMigrationApplyPlanConfirmation, trimmed)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxMigrationPlanArtifactBytes+1))
	if err != nil {
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: read plan artifact: %v", ErrMigrationApplyPlanConfirmation, err)
	}
	if len(raw) > maxMigrationPlanArtifactBytes {
		return migrationPlanArtifact{}, false, fmt.Errorf("%w: plan artifact exceeds %d bytes", ErrMigrationApplyPlanConfirmation, maxMigrationPlanArtifactBytes)
	}
	artifact, err := decodeMigrationPlanArtifact(raw)
	if err != nil {
		return migrationPlanArtifact{}, false, err
	}
	return artifact, true, nil
}

func decodeMigrationPlanArtifact(raw []byte) (migrationPlanArtifact, error) {
	if !utf8.Valid(raw) {
		return migrationPlanArtifact{}, fmt.Errorf("%w: plan artifact is not valid UTF-8", ErrMigrationApplyPlanConfirmation)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return migrationPlanArtifact{}, fmt.Errorf("%w: plan artifact is empty", ErrMigrationApplyPlanConfirmation)
	}
	var artifact migrationPlanArtifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return migrationPlanArtifact{}, fmt.Errorf("%w: decode plan artifact: %v", ErrMigrationApplyPlanConfirmation, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return migrationPlanArtifact{}, fmt.Errorf("%w: plan artifact has trailing JSON", ErrMigrationApplyPlanConfirmation)
	}
	return normalizeMigrationPlanArtifact(artifact)
}

func normalizeMigrationPlanArtifact(artifact migrationPlanArtifact) (migrationPlanArtifact, error) {
	if artifact.Format != migrationPlanArtifactFormat {
		return migrationPlanArtifact{}, fmt.Errorf("%w: unsupported plan artifact format %q", ErrMigrationApplyPlanConfirmation, artifact.Format)
	}
	parsedPlanSHA256, err := parsePlanSHA256(artifact.PlanSHA256)
	if err != nil {
		return migrationPlanArtifact{}, fmt.Errorf("%w: invalid plan artifact checksum: %v", ErrMigrationApplyPlanConfirmation, err)
	}
	artifact.PlanSHA256 = parsedPlanSHA256
	computedSHA256, err := planSHA256(artifact.Plan)
	if err != nil {
		return migrationPlanArtifact{}, fmt.Errorf("%w: validate plan artifact checksum: %v", ErrMigrationApplyPlanConfirmation, err)
	}
	if artifact.PlanSHA256 != computedSHA256 {
		return migrationPlanArtifact{}, fmt.Errorf("%w: plan artifact checksum mismatch: got %s want %s", ErrMigrationApplyPlanConfirmation, computedSHA256, artifact.PlanSHA256)
	}
	if err := validateMigrationPlanArtifactPlan(artifact.Plan); err != nil {
		return migrationPlanArtifact{}, err
	}
	return artifact, nil
}

func readMigrationApplyPreflightStatusFile(path string) (migrationApplyPreflight, bool, error) {
	return readMigrationApplyPreflightPath(path, true)
}

func readMigrationApplyPreflightPath(path string, reportMissing bool) (migrationApplyPreflight, bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: preflight file path is required", ErrMigrationApplyPreflight)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if reportMissing && errors.Is(err, os.ErrNotExist) {
			return migrationApplyPreflight{}, false, nil
		}
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: stat preflight file: %v", ErrMigrationApplyPreflight, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: preflight file must not be a symlink: %s", ErrMigrationApplyPreflight, trimmed)
	}
	if !info.Mode().IsRegular() {
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: preflight file must be a regular file: %s", ErrMigrationApplyPreflight, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: read preflight file: %v", ErrMigrationApplyPreflight, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: stat opened preflight file: %v", ErrMigrationApplyPreflight, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: opened preflight file must be a regular file: %s", ErrMigrationApplyPreflight, trimmed)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxMigrationApplyPreflightBytes+1))
	if err != nil {
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: read preflight file: %v", ErrMigrationApplyPreflight, err)
	}
	if len(raw) > maxMigrationApplyPreflightBytes {
		return migrationApplyPreflight{}, false, fmt.Errorf("%w: preflight file exceeds %d bytes", ErrMigrationApplyPreflight, maxMigrationApplyPreflightBytes)
	}
	preflight, err := decodeMigrationApplyPreflight(raw)
	if err != nil {
		return migrationApplyPreflight{}, false, err
	}
	return preflight, true, nil
}

func decodeMigrationApplyPreflight(raw []byte) (migrationApplyPreflight, error) {
	if !utf8.Valid(raw) {
		return migrationApplyPreflight{}, fmt.Errorf("%w: preflight file is not valid UTF-8", ErrMigrationApplyPreflight)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return migrationApplyPreflight{}, fmt.Errorf("%w: preflight file is empty", ErrMigrationApplyPreflight)
	}
	var preflight migrationApplyPreflight
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&preflight); err != nil {
		return migrationApplyPreflight{}, fmt.Errorf("%w: decode preflight file: %v", ErrMigrationApplyPreflight, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return migrationApplyPreflight{}, fmt.Errorf("%w: preflight file has trailing JSON", ErrMigrationApplyPreflight)
	}
	return normalizeMigrationApplyPreflight(preflight)
}

func normalizeMigrationApplyPreflight(preflight migrationApplyPreflight) (migrationApplyPreflight, error) {
	if preflight.Format != migrationApplyPreflightFormat {
		return migrationApplyPreflight{}, fmt.Errorf("%w: unsupported preflight format %q", ErrMigrationApplyPreflight, preflight.Format)
	}
	if preflight.TargetVersion < 0 {
		return migrationApplyPreflight{}, fmt.Errorf("%w: preflight target_version must be non-negative", ErrMigrationApplyPreflight)
	}
	if preflight.TargetLatest && preflight.TargetVersion == 0 {
		return migrationApplyPreflight{}, fmt.Errorf("%w: latest target preflight must name a positive resolved target_version", ErrMigrationApplyPreflight)
	}
	ledgerSnapshotSHA256, err := parsePlanSHA256(preflight.LedgerSnapshotSHA256)
	if err != nil {
		return migrationApplyPreflight{}, fmt.Errorf("%w: invalid preflight ledger_snapshot_sha256: %v", ErrMigrationApplyPreflight, err)
	}
	preflight.LedgerSnapshotSHA256 = ledgerSnapshotSHA256
	parsedPlanSHA256, err := parsePlanSHA256(preflight.PlanSHA256)
	if err != nil {
		return migrationApplyPreflight{}, fmt.Errorf("%w: invalid preflight plan_sha256: %v", ErrMigrationApplyPreflight, err)
	}
	preflight.PlanSHA256 = parsedPlanSHA256
	computedPlanSHA256, err := planSHA256(preflight.Plan)
	if err != nil {
		return migrationApplyPreflight{}, fmt.Errorf("%w: validate preflight plan checksum: %v", ErrMigrationApplyPreflight, err)
	}
	if preflight.PlanSHA256 != computedPlanSHA256 {
		return migrationApplyPreflight{}, fmt.Errorf("%w: preflight plan_sha256 mismatch: got %s want %s", ErrMigrationApplyPreflight, computedPlanSHA256, preflight.PlanSHA256)
	}
	endVersion, err := validateMigrationPlanShape(preflight.Plan, ErrMigrationApplyPreflight, "preflight")
	if err != nil {
		return migrationApplyPreflight{}, err
	}
	if preflight.TargetVersion != endVersion {
		return migrationApplyPreflight{}, fmt.Errorf("%w: preflight target_version %d does not match plan ending at %d", ErrMigrationApplyPreflight, preflight.TargetVersion, endVersion)
	}
	if preflight.TargetLatest && preflight.TargetVersion != preflight.Plan.LatestVersion {
		return migrationApplyPreflight{}, fmt.Errorf("%w: latest preflight target_version %d does not match plan latest_version %d", ErrMigrationApplyPreflight, preflight.TargetVersion, preflight.Plan.LatestVersion)
	}
	return preflight, nil
}

func validateMigrationApplyPreflightForApply(preflight migrationApplyPreflight, rawLedger []byte, plan dbmigrations.Plan, resolvedTarget int, targetLatest bool) error {
	if preflight.TargetVersion != resolvedTarget {
		return fmt.Errorf("%w: preflight target_version %d does not match requested target_version %d", ErrMigrationApplyPreflight, preflight.TargetVersion, resolvedTarget)
	}
	if preflight.TargetLatest != targetLatest {
		return fmt.Errorf("%w: preflight target_latest %t does not match requested target_latest %t", ErrMigrationApplyPreflight, preflight.TargetLatest, targetLatest)
	}
	ledgerSHA256 := sha256Hex(rawLedger)
	if preflight.LedgerSnapshotSHA256 != ledgerSHA256 {
		return fmt.Errorf("%w: preflight ledger_snapshot_sha256 mismatch: got %s want %s", ErrMigrationApplyPreflight, preflight.LedgerSnapshotSHA256, ledgerSHA256)
	}
	planSHA256, err := planSHA256(plan)
	if err != nil {
		return fmt.Errorf("%w: validate requested plan checksum: %v", ErrMigrationApplyPreflight, err)
	}
	if preflight.PlanSHA256 != planSHA256 || !reflect.DeepEqual(preflight.Plan, plan) {
		return fmt.Errorf("%w: apply preflight does not match requested ledger snapshot and target", ErrMigrationApplyPreflight)
	}
	return nil
}

func validateMigrationPlanArtifactPlan(plan dbmigrations.Plan) error {
	_, err := validateMigrationPlanShape(plan, ErrMigrationApplyPlanConfirmation, "plan artifact")
	return err
}

func validateMigrationPlanShape(plan dbmigrations.Plan, errRoot error, label string) (int, error) {
	if plan.LatestVersion <= 0 {
		return 0, fmt.Errorf("%w: %s latest_version must be positive", errRoot, label)
	}
	if plan.CurrentVersion < 0 || plan.CurrentVersion > plan.LatestVersion {
		return 0, fmt.Errorf("%w: %s current_version must be inside catalog range", errRoot, label)
	}
	if plan.UpToDate != (len(plan.Pending) == 0) {
		return 0, fmt.Errorf("%w: %s up_to_date does not match pending steps", errRoot, label)
	}
	for i, step := range plan.Pending {
		if step.Version <= 0 || step.Version > plan.LatestVersion {
			return 0, fmt.Errorf("%w: %s pending step %d has invalid version %d", errRoot, label, i+1, step.Version)
		}
		if step.Direction != dbmigrations.DirectionUp && step.Direction != dbmigrations.DirectionDown {
			return 0, fmt.Errorf("%w: %s pending step %d has invalid direction %q", errRoot, label, i+1, step.Direction)
		}
		if i > 0 && step.Direction != plan.Pending[0].Direction {
			return 0, fmt.Errorf("%w: %s pending step %d mixes direction %q after %q", errRoot, label, i+1, step.Direction, plan.Pending[0].Direction)
		}
		if strings.TrimSpace(step.Name) == "" {
			return 0, fmt.Errorf("%w: %s pending step %d name is required", errRoot, label, i+1)
		}
		if strings.TrimSpace(step.Path) == "" {
			return 0, fmt.Errorf("%w: %s pending step %d path is required", errRoot, label, i+1)
		}
		if _, err := parsePlanSHA256(step.SHA256); err != nil {
			return 0, fmt.Errorf("%w: invalid %s pending step %d sha256: %v", errRoot, label, i+1, err)
		}
	}
	return replayMigrationPlanSteps(plan, errRoot, label)
}

func replayMigrationPlanArtifactSteps(plan dbmigrations.Plan) (int, error) {
	return replayMigrationPlanSteps(plan, ErrMigrationApplyPlanConfirmation, "plan artifact")
}

func replayMigrationPlanSteps(plan dbmigrations.Plan, errRoot error, label string) (int, error) {
	currentVersion := plan.CurrentVersion
	for i, step := range plan.Pending {
		switch step.Direction {
		case dbmigrations.DirectionUp:
			if step.Version != currentVersion+1 {
				return 0, fmt.Errorf("%w: %s pending up step %d version %d does not continue from %d", errRoot, label, i+1, step.Version, currentVersion)
			}
			currentVersion = step.Version
		case dbmigrations.DirectionDown:
			if step.Version != currentVersion {
				return 0, fmt.Errorf("%w: %s pending down step %d version %d does not continue from %d", errRoot, label, i+1, step.Version, currentVersion)
			}
			currentVersion = step.Version - 1
		default:
			return 0, fmt.Errorf("%w: %s pending step %d has invalid direction %q", errRoot, label, i+1, step.Direction)
		}
	}
	return currentVersion, nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func planContainsRollbackStep(plan dbmigrations.Plan) bool {
	for _, step := range plan.Pending {
		if step.Direction == dbmigrations.DirectionDown {
			return true
		}
	}
	return false
}

func parsePlanSHA256(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != sha256.Size*2 {
		return "", fmt.Errorf("must be %d hex characters", sha256.Size*2)
	}
	if _, err := hex.DecodeString(trimmed); err != nil {
		return "", err
	}
	return strings.ToLower(trimmed), nil
}

func planSHA256(plan dbmigrations.Plan) (string, error) {
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", fmt.Errorf("plan sha256: marshal plan: %w", err)
	}
	raw = append(raw, '\n')
	return sha256Hex(raw), nil
}

func planLedgerSnapshot(raw []byte, targetVersion int, targetLatest bool) (dbmigrations.Plan, error) {
	if targetLatest {
		catalog, err := dbmigrations.Catalog()
		if err != nil {
			return dbmigrations.Plan{}, err
		}
		return dbmigrations.PlanCatalogUpToLatestFromJSONLedgerSnapshot(catalog, bytes.NewReader(raw))
	}
	return dbmigrations.PlanToVersionFromJSONLedgerSnapshot(bytes.NewReader(raw), targetVersion)
}

func planDecodedLedgerSnapshot(snapshot dbmigrations.LedgerSnapshot, targetVersion int, targetLatest bool) (dbmigrations.Plan, error) {
	if targetLatest {
		catalog, err := dbmigrations.Catalog()
		if err != nil {
			return dbmigrations.Plan{}, err
		}
		return dbmigrations.PlanCatalogToVersionFromLedgerSnapshot(catalog, snapshot, catalog[len(catalog)-1].Version)
	}
	return dbmigrations.PlanToVersionFromLedgerSnapshot(snapshot, targetVersion)
}

func resolveTargetVersion(targetVersion int, targetLatest bool) (int, error) {
	if !targetLatest {
		return targetVersion, nil
	}
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		return 0, err
	}
	return catalog[len(catalog)-1].Version, nil
}

func writeJSON(stdout io.Writer, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(stderr, "write JSON: %v\n", err)
		return exitError
	}
	return exitOK
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: metin2-migrate <command> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  catalog                print metadata-only embedded migration catalog summary")
	fmt.Fprintln(w, "  status                 read database schema_migrations metadata and print a dry-run plan")
	fmt.Fprintln(w, "  empty-ledger-snapshot  print an explicit empty schema_migrations ledger snapshot")
	fmt.Fprintln(w, "  ledger-snapshot        export metadata-only schema_migrations ledger snapshot from a database/sql target")
	fmt.Fprintln(w, "  ledger-snapshot-status inspect a retained schema_migrations ledger snapshot without mutating it")
	fmt.Fprintln(w, "  plan                   print metadata-only dry-run plan from an offline ledger snapshot")
	fmt.Fprintln(w, "  plan-artifact          print dry-run plan plus checksum for apply confirmation")
	fmt.Fprintln(w, "  plan-artifact-status   inspect a migration plan artifact without mutating it")
	fmt.Fprintln(w, "  apply-preflight        validate apply inputs and plan confirmation without opening a database")
	fmt.Fprintln(w, "  apply-preflight-status inspect a migration apply preflight file without mutating it")
	fmt.Fprintln(w, "  apply-lock-status      inspect a local migration apply lock file without mutating it")
	fmt.Fprintln(w, "  apply-lock-aside       confirmation-gated lab aside-rename for a stale apply lock")
	fmt.Fprintln(w, "  apply-audit-status     inspect a migration apply audit file without mutating it")
	fmt.Fprintln(w, "  apply                  apply a target plan using a database/sql driver and offline ledger snapshot")
	fmt.Fprintln(w, "  quarantine-export      validate and canonicalize a retained migration-shaped export offline")
	fmt.Fprintln(w, "  synthesize-wipe-export synthesize a wipe-scope export for character-FK tip kinds from retained quarantine/export JSON")
	fmt.Fprintln(w, "  import-export          import a retained migration-shaped export through the programmatic SQL import seams")
	fmt.Fprintln(w, "  import-export-status   inspect a retained import-export result without mutating it")
	fmt.Fprintln(w, "  import-export-drill    print confirmation-gated lab SQL import-export commands from a retained export/quarantine tree")
	fmt.Fprintln(w, "  export-quarantine-drill print path-aware lab export retention + offline quarantine-export commands from build-info")
	fmt.Fprintln(w, "  backup-restore-drill   print path-aware lab backup retention + file-store drill commands from runtime-config and build-info")
	fmt.Fprintln(w, "  migration-run-retention print path-aware migration-runs retention + correlation checklist commands from build-info")
	fmt.Fprintln(w, "  artifact-retention-gc  print path-aware lab retention aside-rename triage for aged YYYYMMDDTHHMMSSZ-<commit12> trees")
	fmt.Fprintln(w, "  artifact-gc-aside-purge print confirmation-gated lab purge script for aged .gc-aside-* retention trees")
	fmt.Fprintln(w, "  version                print metadata-only binary build identity")
	fmt.Fprintln(w, "")
	printVersionUsage(w)
	fmt.Fprintln(w, "")
	printStatusUsage(w)
	fmt.Fprintln(w, "")
	printEmptyLedgerSnapshotUsage(w)
	fmt.Fprintln(w, "")
	printLedgerSnapshotUsage(w)
	fmt.Fprintln(w, "")
	printLedgerSnapshotStatusUsage(w)
	fmt.Fprintln(w, "")
	printPlanUsage(w)
	fmt.Fprintln(w, "")
	printPlanArtifactUsage(w)
	fmt.Fprintln(w, "")
	printPlanArtifactStatusUsage(w)
	fmt.Fprintln(w, "")
	printApplyPreflightUsage(w)
	fmt.Fprintln(w, "")
	printApplyPreflightStatusUsage(w)
	fmt.Fprintln(w, "")
	printApplyLockStatusUsage(w)
	fmt.Fprintln(w, "")
	printApplyLockAsideUsage(w)
	fmt.Fprintln(w, "")
	printApplyAuditStatusUsage(w)
	fmt.Fprintln(w, "")
	printApplyUsage(w)
	fmt.Fprintln(w, "")
	printQuarantineExportUsage(w)
	fmt.Fprintln(w, "")
	printImportExportUsage(w)
	fmt.Fprintln(w, "")
	printImportExportStatusUsage(w)
	fmt.Fprintln(w, "")
	printImportExportDrillUsage(w)
	fmt.Fprintln(w, "")
	printExportQuarantineDrillUsage(w)
	fmt.Fprintln(w, "")
	printBackupRestoreDrillUsage(w)
	fmt.Fprintln(w, "")
	printMigrationRunRetentionUsage(w)
	fmt.Fprintln(w, "")
	printArtifactRetentionGCUsage(w)
}

func printVersionUsage(w io.Writer) {
	fmt.Fprintln(w, "version usage:")
	fmt.Fprintln(w, "  metin2-migrate version")
	fmt.Fprintln(w, "  metin2-migrate --version")
}

func printEmptyLedgerSnapshotUsage(w io.Writer) {
	fmt.Fprintln(w, "empty-ledger-snapshot usage:")
	fmt.Fprintln(w, "  metin2-migrate empty-ledger-snapshot")
}

func printLedgerSnapshotUsage(w io.Writer) {
	fmt.Fprintln(w, "ledger-snapshot usage:")
	fmt.Fprintln(w, "  metin2-migrate ledger-snapshot --driver <database/sql-driver> --dsn <dsn>")
}

func printLedgerSnapshotStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "ledger-snapshot-status usage:")
	fmt.Fprintln(w, "  metin2-migrate ledger-snapshot-status --ledger-snapshot <path>")
}

func printStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "status usage:")
	fmt.Fprintln(w, "  metin2-migrate status --driver <database/sql-driver> --dsn <dsn> [--target-version <version|latest>]")
}

func printPlanUsage(w io.Writer) {
	fmt.Fprintln(w, "plan usage:")
	fmt.Fprintln(w, "  metin2-migrate plan --ledger-snapshot <path|-> --target-version <version|latest>")
}

func printPlanArtifactUsage(w io.Writer) {
	fmt.Fprintln(w, "plan-artifact usage:")
	fmt.Fprintln(w, "  metin2-migrate plan-artifact --ledger-snapshot <path|-> --target-version <version|latest>")
}

func printPlanArtifactStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "plan-artifact-status usage:")
	fmt.Fprintln(w, "  metin2-migrate plan-artifact-status --plan-artifact <path>")
}

func printApplyPreflightUsage(w io.Writer) {
	fmt.Fprintln(w, "apply-preflight usage:")
	fmt.Fprintln(w, "  metin2-migrate apply-preflight --ledger-snapshot <path|-> --target-version <version|latest> [--plan-sha256 <hex> | --plan-artifact <path>] [--allow-rollback]")
}

func printApplyPreflightStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "apply-preflight-status usage:")
	fmt.Fprintln(w, "  metin2-migrate apply-preflight-status --apply-preflight <path>")
}

func printApplyLockStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "apply-lock-status usage:")
	fmt.Fprintln(w, "  metin2-migrate apply-lock-status --lock-file <path>")
}

func printApplyLockAsideUsage(w io.Writer) {
	fmt.Fprintln(w, "apply-lock-aside usage:")
	fmt.Fprintln(w, "  metin2-migrate apply-lock-aside --lock-file <path> --i-confirm-lab-aside-rename")
}

func printApplyAuditStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "apply-audit-status usage:")
	fmt.Fprintln(w, "  metin2-migrate apply-audit-status --audit-file <path>")
}

func printApplyUsage(w io.Writer) {
	fmt.Fprintln(w, "apply usage:")
	fmt.Fprintln(w, "  metin2-migrate apply --driver <database/sql-driver> --dsn <dsn> --ledger-snapshot <path|-> --target-version <version|latest> [--plan-sha256 <hex> | --plan-artifact <path> | --apply-preflight <path>] [--lock-file <path>] [--audit-file <path>] [--allow-rollback]")
}
