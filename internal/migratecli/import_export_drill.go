package migratecli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const defaultImportExportDrillDSNEnv = "METIN2_IMPORT_DSN"

var errInvalidImportExportDrillInput = errors.New("invalid import-export-drill input")

// importExportDrillScopedReplaceKinds is the FK-safe print order used when
// --i-confirm-print-scoped-replace is set. Character-scoped child tip domains
// must run before tip-0002 account-character-roster because roster scoped
// replace deletes characters/accounts without cascading child tip rows.
var importExportDrillScopedReplaceKinds = []string{
	"character-item-state",
	"character-point-state",
	"character-myshop-unit-prices",
	"character-quest-state",
	"character-safebox-state",
	"bootstrap-ground-item-state",
	"auth-login-ticket-handoff",
	"item-template-state",
	"static-actor-content-state",
	"account-character-roster",
}

type importExportDrillPlan struct {
	ExportTree         string
	Driver             string
	DSNEnv             string
	Kinds              []string
	PrintScopedReplace bool
}

func runImportExportDrill(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("import-export-drill", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var exportTree string
	var driverName string
	var dsnEnv string
	var confirmPrint bool
	var confirmPrintScopedReplace bool
	flags.StringVar(&exportTree, "export-tree", "", "absolute retained export/quarantine tree (YYYYMMDDTHHMMSSZ-<commit12>)")
	flags.StringVar(&driverName, "driver", "", "database/sql driver name literal printed into the drill script")
	flags.StringVar(&dsnEnv, "dsn-env", defaultImportExportDrillDSNEnv, "environment variable name the printed script reads for the import target DSN")
	flags.BoolVar(&confirmPrint, "i-confirm-print-sql-import-drill", false, "confirm emission of a lab SQL import-export drill script (CLI still does not execute it)")
	flags.BoolVar(&confirmPrintScopedReplace, "i-confirm-print-scoped-replace", false, "opt-in: print --i-confirm-scoped-replace on every tip-kind import-export line (requires --i-confirm-print-sql-import-drill; default remains insert-only)")
	flags.Usage = func() { printImportExportDrillUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected import-export-drill argument %q\n", flags.Arg(0))
		printImportExportDrillUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(exportTree) == "" || strings.TrimSpace(driverName) == "" {
		fmt.Fprintln(stderr, "--export-tree and --driver are required for import-export-drill")
		printImportExportDrillUsage(stderr)
		return exitUsage
	}
	if !confirmPrint {
		fmt.Fprintln(stderr, "--i-confirm-print-sql-import-drill is required for import-export-drill")
		return exitError
	}

	plan, err := buildImportExportDrillPlan(exportTree, driverName, dsnEnv, confirmPrintScopedReplace)
	if err != nil {
		fmt.Fprintf(stderr, "import-export-drill: %v\n", err)
		return exitError
	}
	if _, err := io.WriteString(stdout, renderImportExportDrillScript(plan)); err != nil {
		fmt.Fprintf(stderr, "import-export-drill: write script: %v\n", err)
		return exitError
	}
	return exitOK
}

func buildImportExportDrillPlan(exportTree, driverName, dsnEnv string, printScopedReplace bool) (importExportDrillPlan, error) {
	normalizedTree, err := normalizeImportExportDrillAbsolutePath(exportTree, "export-tree")
	if err != nil {
		return importExportDrillPlan{}, err
	}
	normalizedDriver := strings.TrimSpace(driverName)
	if normalizedDriver == "" {
		return importExportDrillPlan{}, fmt.Errorf("%w: driver is required", errInvalidImportExportDrillInput)
	}
	normalizedEnv, err := normalizeImportExportDrillDSNEnv(dsnEnv)
	if err != nil {
		return importExportDrillPlan{}, err
	}
	kinds := append([]string(nil), exportQuarantineKinds...)
	if printScopedReplace {
		kinds = append([]string(nil), importExportDrillScopedReplaceKinds...)
	}
	return importExportDrillPlan{
		ExportTree:         normalizedTree,
		Driver:             normalizedDriver,
		DSNEnv:             normalizedEnv,
		Kinds:              kinds,
		PrintScopedReplace: printScopedReplace,
	}, nil
}

func normalizeImportExportDrillAbsolutePath(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s is required", errInvalidImportExportDrillInput, label)
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("%w: %s must be an absolute path", errInvalidImportExportDrillInput, label)
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return "", fmt.Errorf("%w: %s is invalid", errInvalidImportExportDrillInput, label)
	}
	return cleaned, nil
}

func normalizeImportExportDrillDSNEnv(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: dsn-env is required", errInvalidImportExportDrillInput)
	}
	for i, r := range trimmed {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return "", fmt.Errorf("%w: dsn-env must be a shell-safe environment variable name", errInvalidImportExportDrillInput)
		}
	}
	return trimmed, nil
}

func renderImportExportDrillScript(plan importExportDrillPlan) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# confirmation-gated printer: does not execute import-export or open a database\n")
	b.WriteString("# Generated for docs/workflow/migration-apply-runbook.md and docs/plans/2026-08-27-cli-import-export.md\n")
	b.WriteString("# after a retained export/quarantine tree from export-quarantine-drill.\n")
	b.WriteString("set -eu\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "EXPORT_TREE=%s\n", shellSingleQuote(plan.ExportTree))
	fmt.Fprintf(&b, "DRIVER=%s\n", shellSingleQuote(plan.Driver))
	fmt.Fprintf(&b, "DSN_ENV=%s\n", shellSingleQuote(plan.DSNEnv))
	b.WriteString("\n")
	fmt.Fprintf(&b, "DSN=\"${%s:?%s must be set to the import target DSN}\"\n", plan.DSNEnv, plan.DSNEnv)
	b.WriteString("\n")
	b.WriteString("echo '== confirmation-gated SQL import from retained quarantine.json artifacts =='\n")
	b.WriteString("# Reads DSN only from the named environment variable. Never paste DSNs into notes.\n")
	if plan.PrintScopedReplace {
		b.WriteString("# Each import-export invocation still requires --i-confirm-sql-import plus opt-in scoped replace.\n")
		b.WriteString("# Kind order is FK-safe for empty/wipe trees: character-scoped child tips before tip-0002 roster.\n")
		b.WriteString("# Seeded full-tree single-pass including tip-0002 still fails closed while child tip rows remain;\n")
		b.WriteString("# omit or wipe child domains before roster replace, or run tip-0002 alone after children are absent.\n")
	} else {
		b.WriteString("# Each import-export invocation still requires --i-confirm-sql-import and remains insert-only.\n")
	}
	replaceSuffix := ""
	if plan.PrintScopedReplace {
		replaceSuffix = " --i-confirm-scoped-replace"
	}
	for _, kind := range plan.Kinds {
		fmt.Fprintf(&b, "test -f \"$EXPORT_TREE/%s/quarantine.json\"\n", kind)
		fmt.Fprintf(&b, "metin2-migrate import-export --kind %s --export \"$EXPORT_TREE/%s/quarantine.json\" --driver \"$DRIVER\" --dsn \"$DSN\" --i-confirm-sql-import%s > \"$EXPORT_TREE/%s/import-result.json\"\n", kind, kind, replaceSuffix, kind)
		fmt.Fprintf(&b, "metin2-migrate import-export-status --kind %s --import-result \"$EXPORT_TREE/%s/import-result.json\" > \"$EXPORT_TREE/%s/import-result-status.json\"\n", kind, kind, kind)
	}
	return b.String()
}

func printImportExportDrillUsage(w io.Writer) {
	fmt.Fprintln(w, "import-export-drill usage:")
	fmt.Fprintln(w, "  metin2-migrate import-export-drill --export-tree <absolute-retained-tree> --driver <database/sql-driver> [--dsn-env METIN2_IMPORT_DSN] --i-confirm-print-sql-import-drill [--i-confirm-print-scoped-replace]")
}
