package migratecli

import (
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
)

// Run executes the small migration preflight CLI and returns a process-style exit
// code. The command is intentionally read-only: it can print the embedded catalog
// summary and produce dry-run plans from strict offline ledger snapshots, but it
// never opens a database, applies SQL, rolls migrations back, or mutates daemon
// runtime state.
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
	targetVersion, err := strconv.Atoi(targetVersionText)
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

	plan, err := dbmigrations.PlanToVersionFromJSONLedgerSnapshot(reader, targetVersion)
	if err != nil {
		fmt.Fprintf(stderr, "migration plan: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, plan)
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
	fmt.Fprintln(w, "")
	printPlanUsage(w)
}

func printPlanUsage(w io.Writer) {
	fmt.Fprintln(w, "plan usage:")
	fmt.Fprintln(w, "  metin2-migrate plan --ledger-snapshot <path|-> --target-version <version>")
}
