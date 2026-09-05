package migratecli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

const (
	exportTreeStatusStatusFormat = "go-metin2-export-tree-status-status-v1"
	maxExportTreeStatusBytes     = 128 * 1024
)

type exportTreeStatusStatus struct {
	Format                 string            `json:"format"`
	Present                bool              `json:"present"`
	ExportTreeStatusSHA256 string            `json:"export_tree_status_sha256,omitempty"`
	Status                 *exportTreeStatus `json:"status,omitempty"`
}

func runExportTreeStatusStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("export-tree-status-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var exportTreeStatusPath string
	var requireQuarantineComplete bool
	var requireTwoPhaseWipeArtifactsComplete bool
	var requireImportResultArtifactsComplete bool
	var requireWipeImportArtifactsComplete bool
	var requireImportResultOutcomesComplete bool
	var requireImportResultAllReplaced bool
	var requireWipeImportResultOutcomesComplete bool
	var requireWipeImportResultAllReplaced bool
	flags.StringVar(&exportTreeStatusPath, "export-tree-status", "", "path to a retained export-tree-status JSON snapshot")
	flags.BoolVar(&requireQuarantineComplete, "require-quarantine-complete", false, "fail closed unless inner quarantine_complete is true")
	flags.BoolVar(&requireTwoPhaseWipeArtifactsComplete, "require-two-phase-wipe-artifacts-complete", false, "fail closed unless inner two_phase_wipe_artifacts_complete is true")
	flags.BoolVar(&requireImportResultArtifactsComplete, "require-import-result-artifacts-complete", false, "fail closed unless inner import_result_artifacts_complete is true")
	flags.BoolVar(&requireWipeImportArtifactsComplete, "require-wipe-import-artifacts-complete", false, "fail closed unless inner wipe_import_artifacts_complete is true")
	flags.BoolVar(&requireImportResultOutcomesComplete, "require-import-result-outcomes-complete", false, "fail closed unless inner import_result_outcomes_complete is true")
	flags.BoolVar(&requireImportResultAllReplaced, "require-import-result-all-replaced", false, "fail closed unless inner import_result_all_replaced is true")
	flags.BoolVar(&requireWipeImportResultOutcomesComplete, "require-wipe-import-result-outcomes-complete", false, "fail closed unless inner wipe_import_result_outcomes_complete is true")
	flags.BoolVar(&requireWipeImportResultAllReplaced, "require-wipe-import-result-all-replaced", false, "fail closed unless inner wipe_import_result_all_replaced is true")
	flags.Usage = func() { printExportTreeStatusStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected export-tree-status-status argument %q\n", flags.Arg(0))
		printExportTreeStatusStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(exportTreeStatusPath) == "" {
		fmt.Fprintln(stderr, "--export-tree-status is required for export-tree-status-status")
		printExportTreeStatusStatusUsage(stderr)
		return exitUsage
	}

	gates := exportTreeStatusRequireGates{
		QuarantineComplete:               requireQuarantineComplete,
		TwoPhaseWipeArtifactsComplete:    requireTwoPhaseWipeArtifactsComplete,
		ImportResultArtifactsComplete:    requireImportResultArtifactsComplete,
		WipeImportArtifactsComplete:      requireWipeImportArtifactsComplete,
		ImportResultOutcomesComplete:     requireImportResultOutcomesComplete,
		ImportResultAllReplaced:          requireImportResultAllReplaced,
		WipeImportResultOutcomesComplete: requireWipeImportResultOutcomesComplete,
		WipeImportResultAllReplaced:      requireWipeImportResultAllReplaced,
	}

	raw, present, err := readOptionalExportTreeRegularFile(exportTreeStatusPath, maxExportTreeStatusBytes, "export-tree-status")
	if err != nil {
		fmt.Fprintf(stderr, "export-tree-status-status: %v\n", err)
		return exitError
	}
	if !present {
		if err := enforceExportTreeStatusRequireGates(exportTreeStatus{Present: false}, gates); err != nil {
			fmt.Fprintf(stderr, "export-tree-status-status: %v\n", err)
			return exitError
		}
		return writeJSON(stdout, stderr, exportTreeStatusStatus{
			Format:  exportTreeStatusStatusFormat,
			Present: false,
		})
	}

	var inner exportTreeStatus
	if err := decodeStrictExportTreeStatusJSON(raw, &inner, "export-tree-status"); err != nil {
		fmt.Fprintf(stderr, "export-tree-status-status: %v\n", err)
		return exitError
	}
	if err := validateRetainedExportTreeStatus(inner); err != nil {
		fmt.Fprintf(stderr, "export-tree-status-status: %v\n", err)
		return exitError
	}
	if err := enforceExportTreeStatusRequireGates(inner, gates); err != nil {
		fmt.Fprintf(stderr, "export-tree-status-status: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, exportTreeStatusStatus{
		Format:                 exportTreeStatusStatusFormat,
		Present:                true,
		ExportTreeStatusSHA256: sha256Hex(raw),
		Status:                 &inner,
	})
}

func validateRetainedExportTreeStatus(status exportTreeStatus) error {
	if status.Format != exportTreeStatusFormat {
		return fmt.Errorf("%w: unexpected format %q", ErrExportTreeStatus, status.Format)
	}
	if !status.Present {
		if status.ExportTree != "" ||
			status.KindCount != 0 ||
			status.QuarantinePresentCount != 0 ||
			status.QuarantineComplete ||
			status.WipeQuarantinePresentCount != 0 ||
			status.TwoPhaseWipeArtifactsComplete ||
			status.ImportResultPresentCount != 0 ||
			status.ImportResultStatusPresentCount != 0 ||
			status.ImportResultArtifactsComplete ||
			status.ImportResultReplacedCount != 0 ||
			status.ImportResultRowCountTotal != 0 ||
			status.ImportResultOutcomesComplete ||
			status.ImportResultAllReplaced ||
			status.WipeImportResultPresentCount != 0 ||
			status.WipeImportResultStatusPresentCount != 0 ||
			status.WipeImportArtifactsComplete ||
			status.WipeImportResultReplacedCount != 0 ||
			status.WipeImportResultRowCountTotal != 0 ||
			status.WipeImportResultOutcomesComplete ||
			status.WipeImportResultAllReplaced ||
			len(status.Kinds) != 0 {
			return fmt.Errorf("%w: absent export-tree status has extra fields", ErrExportTreeStatus)
		}
		return nil
	}

	normalizedTree, err := normalizeImportExportDrillAbsolutePath(status.ExportTree, "export-tree")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrExportTreeStatus, err)
	}
	if normalizedTree != status.ExportTree {
		return fmt.Errorf("%w: export_tree must be an absolute cleaned path", ErrExportTreeStatus)
	}
	if status.KindCount != len(exportQuarantineKinds) || len(status.Kinds) != len(exportQuarantineKinds) {
		return fmt.Errorf("%w: kind_count mismatch", ErrExportTreeStatus)
	}

	wipeKindSet := exportTreeWipeKindSet()
	for i, wantKind := range exportQuarantineKinds {
		entry := status.Kinds[i]
		if entry.Kind != wantKind {
			return fmt.Errorf("%w: kind mismatch at index %d: got %q want %q", ErrExportTreeStatus, i, entry.Kind, wantKind)
		}
		_, wantWipe := wipeKindSet[wantKind]
		if entry.WipeKind != wantWipe {
			return fmt.Errorf("%w: wipe_kind mismatch for %s", ErrExportTreeStatus, wantKind)
		}
		if wantWipe {
			if entry.WipeQuarantine == nil {
				return fmt.Errorf("%w: wipe_quarantine missing for %s", ErrExportTreeStatus, wantKind)
			}
			if entry.WipeQuarantineStatus == nil {
				return fmt.Errorf("%w: wipe_quarantine_status missing for %s", ErrExportTreeStatus, wantKind)
			}
			if entry.WipeImportResult == nil {
				return fmt.Errorf("%w: wipe_import_result missing for %s", ErrExportTreeStatus, wantKind)
			}
			if entry.WipeImportResultStatus == nil {
				return fmt.Errorf("%w: wipe_import_result_status missing for %s", ErrExportTreeStatus, wantKind)
			}
		} else if entry.WipeQuarantine != nil ||
			entry.WipeQuarantineStatus != nil ||
			entry.WipeImportResult != nil ||
			entry.WipeImportResultStatus != nil ||
			entry.WipeImportResultOutcome != nil {
			return fmt.Errorf("%w: non-wipe kind %s must omit wipe-only pointers", ErrExportTreeStatus, wantKind)
		}
		if (entry.ImportResultOutcome != nil) != entry.ImportResult.Present {
			return fmt.Errorf("%w: import_result_outcome must be present iff import_result.present for %s", ErrExportTreeStatus, wantKind)
		}
		if wantWipe && (entry.WipeImportResultOutcome != nil) != entry.WipeImportResult.Present {
			return fmt.Errorf("%w: wipe_import_result_outcome must be present iff wipe_import_result.present for %s", ErrExportTreeStatus, wantKind)
		}
	}

	reported := status
	applyExportTreeStatusAggregates(&status)
	switch {
	case status.QuarantinePresentCount != reported.QuarantinePresentCount:
		return fmt.Errorf("%w: quarantine_present_count mismatch", ErrExportTreeStatus)
	case status.QuarantineComplete != reported.QuarantineComplete:
		return fmt.Errorf("%w: quarantine_complete mismatch", ErrExportTreeStatus)
	case status.WipeQuarantinePresentCount != reported.WipeQuarantinePresentCount:
		return fmt.Errorf("%w: wipe_quarantine_present_count mismatch", ErrExportTreeStatus)
	case status.TwoPhaseWipeArtifactsComplete != reported.TwoPhaseWipeArtifactsComplete:
		return fmt.Errorf("%w: two_phase_wipe_artifacts_complete mismatch", ErrExportTreeStatus)
	case status.ImportResultPresentCount != reported.ImportResultPresentCount:
		return fmt.Errorf("%w: import_result_present_count mismatch", ErrExportTreeStatus)
	case status.ImportResultStatusPresentCount != reported.ImportResultStatusPresentCount:
		return fmt.Errorf("%w: import_result_status_present_count mismatch", ErrExportTreeStatus)
	case status.ImportResultArtifactsComplete != reported.ImportResultArtifactsComplete:
		return fmt.Errorf("%w: import_result_artifacts_complete mismatch", ErrExportTreeStatus)
	case status.ImportResultReplacedCount != reported.ImportResultReplacedCount:
		return fmt.Errorf("%w: import_result_replaced_count mismatch", ErrExportTreeStatus)
	case status.ImportResultRowCountTotal != reported.ImportResultRowCountTotal:
		return fmt.Errorf("%w: import_result_row_count_total mismatch", ErrExportTreeStatus)
	case status.ImportResultOutcomesComplete != reported.ImportResultOutcomesComplete:
		return fmt.Errorf("%w: import_result_outcomes_complete mismatch", ErrExportTreeStatus)
	case status.ImportResultAllReplaced != reported.ImportResultAllReplaced:
		return fmt.Errorf("%w: import_result_all_replaced mismatch", ErrExportTreeStatus)
	case status.WipeImportResultPresentCount != reported.WipeImportResultPresentCount:
		return fmt.Errorf("%w: wipe_import_result_present_count mismatch", ErrExportTreeStatus)
	case status.WipeImportResultStatusPresentCount != reported.WipeImportResultStatusPresentCount:
		return fmt.Errorf("%w: wipe_import_result_status_present_count mismatch", ErrExportTreeStatus)
	case status.WipeImportArtifactsComplete != reported.WipeImportArtifactsComplete:
		return fmt.Errorf("%w: wipe_import_artifacts_complete mismatch", ErrExportTreeStatus)
	case status.WipeImportResultReplacedCount != reported.WipeImportResultReplacedCount:
		return fmt.Errorf("%w: wipe_import_result_replaced_count mismatch", ErrExportTreeStatus)
	case status.WipeImportResultRowCountTotal != reported.WipeImportResultRowCountTotal:
		return fmt.Errorf("%w: wipe_import_result_row_count_total mismatch", ErrExportTreeStatus)
	case status.WipeImportResultOutcomesComplete != reported.WipeImportResultOutcomesComplete:
		return fmt.Errorf("%w: wipe_import_result_outcomes_complete mismatch", ErrExportTreeStatus)
	case status.WipeImportResultAllReplaced != reported.WipeImportResultAllReplaced:
		return fmt.Errorf("%w: wipe_import_result_all_replaced mismatch", ErrExportTreeStatus)
	default:
		return nil
	}
}

func printExportTreeStatusStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "export-tree-status-status usage:")
	fmt.Fprintln(w, "  metin2-migrate export-tree-status-status --export-tree-status <path> [--require-quarantine-complete] [--require-two-phase-wipe-artifacts-complete] [--require-import-result-artifacts-complete] [--require-wipe-import-artifacts-complete] [--require-import-result-outcomes-complete] [--require-import-result-all-replaced] [--require-wipe-import-result-outcomes-complete] [--require-wipe-import-result-all-replaced]")
}
