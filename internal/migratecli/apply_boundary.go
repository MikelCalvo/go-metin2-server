package migratecli

import (
	"flag"
	"fmt"
	"io"
)

const applyBoundaryFormat = "go-metin2-migration-apply-boundary-v1"

const (
	applyBoundaryCLIApply         = "apply"
	applyBoundaryCLIRollbackFlag  = "--allow-rollback"
	applyBoundaryMutatingApply    = "/local/db/migrations/apply"
	applyBoundaryMutatingRollback = "/local/db/migrations/rollback"
)

// applyBoundaryReadOnlyPaths is the closed loopback migration ops set. Apply and
// rollback stay off this list so daemon mutation cannot drift in by renaming a
// read-only route.
var applyBoundaryReadOnlyPaths = []string{
	"/local/db/migrations/catalog",
	"/local/db/migrations/status",
	"/local/db/migrations/plan",
	"/local/db/migrations/ledger-snapshot",
	"/local/db/migrations/plan-from-ledger-snapshot",
}

var applyBoundaryMutatingPaths = []string{
	applyBoundaryMutatingApply,
	applyBoundaryMutatingRollback,
}

type applyBoundary struct {
	Format                      string   `json:"format"`
	CLIApply                    string   `json:"cli_apply"`
	CLIRollbackFlag             string   `json:"cli_rollback_flag"`
	CLIApplyIsMutating          bool     `json:"cli_apply_is_mutating"`
	DaemonOpsMutatingPaths      []string `json:"daemon_ops_mutating_paths"`
	DaemonOpsMutatingRegistered bool     `json:"daemon_ops_mutating_registered"`
	DaemonOpsReadOnlyPaths      []string `json:"daemon_ops_read_only_paths"`
}

func runApplyBoundary(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply-boundary", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printApplyBoundaryUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected apply-boundary argument %q\n", flags.Arg(0))
		printApplyBoundaryUsage(stderr)
		return exitUsage
	}
	return writeJSON(stdout, stderr, applyBoundary{
		Format:                      applyBoundaryFormat,
		CLIApply:                    applyBoundaryCLIApply,
		CLIRollbackFlag:             applyBoundaryCLIRollbackFlag,
		CLIApplyIsMutating:          true,
		DaemonOpsMutatingPaths:      append([]string{}, applyBoundaryMutatingPaths...),
		DaemonOpsMutatingRegistered: false,
		DaemonOpsReadOnlyPaths:      append([]string{}, applyBoundaryReadOnlyPaths...),
	})
}

func printApplyBoundaryUsage(w io.Writer) {
	fmt.Fprintln(w, "apply-boundary usage:")
	fmt.Fprintln(w, "  metin2-migrate apply-boundary")
}
