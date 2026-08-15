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
	"time"
	"unicode/utf8"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2

	maxLedgerSnapshotBytes = 64 * 1024
)

// Run executes the small migration preflight CLI and returns a process-style exit
// code. The catalog, status, empty-ledger-snapshot, ledger-snapshot, plan, and
// plan-artifact commands are read-only. The apply command is an explicit CLI-only
// mutation surface: it requires an operator-supplied database driver, DSN, strict
// offline ledger snapshot, and target version, and it remains deliberately
// separate from daemon startup and local ops endpoints. Operators can optionally
// require a previously inspected plan checksum and request an exclusive
// metadata-only audit file for non-empty apply plans.
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
	case "empty-ledger-snapshot":
		return runEmptyLedgerSnapshot(args[1:], stdout, stderr)
	case "ledger-snapshot":
		return runLedgerSnapshot(args[1:], stdout, stderr)
	case "apply":
		return runApply(args[1:], stdin, stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return exitOK
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return exitUsage
	}
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

type migrationPlanArtifact struct {
	Format     string            `json:"format"`
	PlanSHA256 string            `json:"plan_sha256"`
	Plan       dbmigrations.Plan `json:"plan"`
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
	var planSHA256Text string
	var planArtifactPath string
	flags.StringVar(&driverName, "driver", "", "database/sql driver name for the migration target")
	flags.StringVar(&dsn, "dsn", "", "database/sql DSN for the migration target")
	flags.StringVar(&snapshotPath, "ledger-snapshot", "", "path to go-metin2 schema_migrations ledger snapshot JSON, or - for stdin")
	flags.StringVar(&targetVersionText, "target-version", "", "catalog target version to apply")
	flags.StringVar(&auditFilePath, "audit-file", "", "optional path for an exclusive metadata-only apply audit JSON file")
	flags.StringVar(&planSHA256Text, "plan-sha256", "", "optional SHA-256 of the metadata-only dry-run plan JSON that must match before applying")
	flags.StringVar(&planArtifactPath, "plan-artifact", "", "optional path to a metadata-only migration plan artifact that must match before applying")
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
	if trimmedPlanSHA256 != "" && trimmedPlanArtifactPath != "" {
		fmt.Fprintln(stderr, "--plan-sha256 and --plan-artifact cannot be used together")
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
	if confirmedPlanSHA256 != "" {
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
		if gotPlanSHA256 != confirmedPlanSHA256 {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", fmt.Errorf("%w: plan sha256 mismatch: got %s want %s", ErrMigrationApplyPlanConfirmation, gotPlanSHA256, confirmedPlanSHA256))
			return exitError
		}
	}

	var auditFile *migrationApplyAuditFile
	if strings.TrimSpace(auditFilePath) != "" {
		plan, err := dbmigrations.PlanToVersion(ledger, resolvedTarget)
		if err != nil {
			writeMigrationCommandError(stderr, dsn, "migration apply: %v", err)
			return exitError
		}
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

const migrationApplyAuditFormat = "go-metin2-migration-apply-audit-v1"

var ErrMigrationApplyAudit = errors.New("migration apply audit failed")

var ErrMigrationApplyPlanConfirmation = errors.New("migration apply plan confirmation failed")

type migrationApplyAudit struct {
	Format               string                   `json:"format"`
	AppliedAt            string                   `json:"applied_at"`
	Driver               string                   `json:"driver"`
	DSNConfigured        bool                     `json:"dsn_configured"`
	TargetVersion        int                      `json:"target_version"`
	TargetLatest         bool                     `json:"target_latest"`
	ConfirmedPlanSHA256  string                   `json:"confirmed_plan_sha256,omitempty"`
	LedgerSnapshotSHA256 string                   `json:"ledger_snapshot_sha256"`
	Result               dbmigrations.ApplyResult `json:"result"`
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

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
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
	fmt.Fprintln(w, "  plan                   print metadata-only dry-run plan from an offline ledger snapshot")
	fmt.Fprintln(w, "  plan-artifact          print dry-run plan plus checksum for apply confirmation")
	fmt.Fprintln(w, "  apply                  apply a target plan using a database/sql driver and offline ledger snapshot")
	fmt.Fprintln(w, "")
	printStatusUsage(w)
	fmt.Fprintln(w, "")
	printEmptyLedgerSnapshotUsage(w)
	fmt.Fprintln(w, "")
	printLedgerSnapshotUsage(w)
	fmt.Fprintln(w, "")
	printPlanUsage(w)
	fmt.Fprintln(w, "")
	printPlanArtifactUsage(w)
	fmt.Fprintln(w, "")
	printApplyUsage(w)
}

func printEmptyLedgerSnapshotUsage(w io.Writer) {
	fmt.Fprintln(w, "empty-ledger-snapshot usage:")
	fmt.Fprintln(w, "  metin2-migrate empty-ledger-snapshot")
}

func printLedgerSnapshotUsage(w io.Writer) {
	fmt.Fprintln(w, "ledger-snapshot usage:")
	fmt.Fprintln(w, "  metin2-migrate ledger-snapshot --driver <database/sql-driver> --dsn <dsn>")
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

func printApplyUsage(w io.Writer) {
	fmt.Fprintln(w, "apply usage:")
	fmt.Fprintln(w, "  metin2-migrate apply --driver <database/sql-driver> --dsn <dsn> --ledger-snapshot <path|-> --target-version <version|latest> [--plan-sha256 <hex>] [--audit-file <path>]")
}
