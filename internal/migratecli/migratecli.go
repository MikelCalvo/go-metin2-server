package migratecli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2

	maxLedgerSnapshotBytes = 64 * 1024
)

// Run executes the small migration preflight CLI and returns a process-style exit
// code. The default catalog/plan commands are read-only. The apply command is an
// explicit CLI-only mutation surface: it requires an operator-supplied database
// driver, DSN, strict offline ledger snapshot, and target version, and it remains
// deliberately separate from daemon startup and local ops endpoints.
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
	case "plan":
		return runPlan(args[1:], stdin, stdout, stderr)
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

func runPlan(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var snapshotPath string
	var targetVersionText string
	flags.StringVar(&snapshotPath, "ledger-snapshot", "", "path to go-metin2 schema_migrations ledger snapshot JSON, or - for stdin")
	flags.StringVar(&targetVersionText, "target-version", "", "catalog target version for the dry-run plan")
	flags.Usage = func() { printPlanUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected plan argument %q\n", flags.Arg(0))
		printPlanUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(snapshotPath) == "" {
		fmt.Fprintln(stderr, "missing --ledger-snapshot")
		printPlanUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(targetVersionText) == "" {
		fmt.Fprintln(stderr, "missing --target-version")
		printPlanUsage(stderr)
		return exitUsage
	}
	targetVersion, targetLatest, err := parseTargetVersion(targetVersionText)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --target-version %q: %v\n", targetVersionText, err)
		return exitUsage
	}

	reader, closeReader, err := openLedgerSnapshotReader(snapshotPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "open ledger snapshot: %v\n", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	ledger, err := readBoundedLedgerSnapshot(reader)
	if err != nil {
		fmt.Fprintf(stderr, "migration plan: %v\n", err)
		return exitError
	}
	plan, err := planLedgerSnapshot(ledger, targetVersion, targetLatest)
	if err != nil {
		fmt.Fprintf(stderr, "migration plan: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, plan)
}

func runApply(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var driverName string
	var dsn string
	var snapshotPath string
	var targetVersionText string
	flags.StringVar(&driverName, "driver", "", "database/sql driver name for the migration target")
	flags.StringVar(&dsn, "dsn", "", "database/sql DSN for the migration target")
	flags.StringVar(&snapshotPath, "ledger-snapshot", "", "path to go-metin2 schema_migrations ledger snapshot JSON, or - for stdin")
	flags.StringVar(&targetVersionText, "target-version", "", "catalog target version to apply")
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
		writeMigrationApplyError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}

	reader, closeReader, err := openLedgerSnapshotReader(snapshotPath, stdin)
	if err != nil {
		writeMigrationApplyError(stderr, dsn, "open ledger snapshot: %v", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	rawLedger, err := readBoundedLedgerSnapshot(reader)
	if err != nil {
		writeMigrationApplyError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}
	ledger, err := dbmigrations.ReadJSONLedgerSnapshot(bytes.NewReader(rawLedger))
	if err != nil {
		writeMigrationApplyError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}

	db, err := sql.Open(strings.TrimSpace(driverName), strings.TrimSpace(dsn))
	if err != nil {
		writeMigrationApplyError(stderr, dsn, "migration apply: open database driver %q: %v", strings.TrimSpace(driverName), err)
		return exitError
	}
	defer db.Close()

	result, err := dbmigrations.ApplyToVersion(context.Background(), db, ledger, resolvedTarget)
	if err != nil {
		writeMigrationApplyError(stderr, dsn, "migration apply: %v", err)
		return exitError
	}
	return writeJSON(stdout, stderr, result)
}

func writeMigrationApplyError(stderr io.Writer, dsn string, format string, args ...any) {
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
	fmt.Fprintln(w, "  catalog  print metadata-only embedded migration catalog summary")
	fmt.Fprintln(w, "  plan     print metadata-only dry-run plan from an offline ledger snapshot")
	fmt.Fprintln(w, "  apply    apply a target plan using a database/sql driver and offline ledger snapshot")
	fmt.Fprintln(w, "")
	printPlanUsage(w)
	fmt.Fprintln(w, "")
	printApplyUsage(w)
}

func printPlanUsage(w io.Writer) {
	fmt.Fprintln(w, "plan usage:")
	fmt.Fprintln(w, "  metin2-migrate plan --ledger-snapshot <path|-> --target-version <version|latest>")
}

func printApplyUsage(w io.Writer) {
	fmt.Fprintln(w, "apply usage:")
	fmt.Fprintln(w, "  metin2-migrate apply --driver <database/sql-driver> --dsn <dsn> --ledger-snapshot <path|-> --target-version <version|latest>")
}
