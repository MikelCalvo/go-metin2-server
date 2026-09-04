package migratecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const exportTreeStatusFormat = "go-metin2-export-tree-status-v1"

// ErrExportTreeStatus reports a fail-closed export-tree inspection failure.
var ErrExportTreeStatus = errors.New("export-tree status failed")

type exportTreeArtifactStatus struct {
	Present    bool   `json:"present"`
	Path       string `json:"path,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	ScopeKey   string `json:"scope_key,omitempty"`
	ScopeCount int    `json:"scope_count,omitempty"`
}

type exportTreeImportResultOutcome struct {
	Replaced bool `json:"replaced"`
	RowCount int  `json:"row_count"`
}

type exportTreeKindStatus struct {
	Kind                   string                         `json:"kind"`
	WipeKind               bool                           `json:"wipe_kind"`
	Quarantine             exportTreeArtifactStatus       `json:"quarantine"`
	WipeQuarantine         *exportTreeArtifactStatus      `json:"wipe_quarantine,omitempty"`
	WipeQuarantineStatus   *exportTreeArtifactStatus      `json:"wipe_quarantine_status,omitempty"`
	ImportResult           exportTreeArtifactStatus       `json:"import_result"`
	ImportResultStatus     exportTreeArtifactStatus       `json:"import_result_status"`
	ImportResultOutcome    *exportTreeImportResultOutcome `json:"import_result_outcome,omitempty"`
	WipeImportResult       *exportTreeArtifactStatus      `json:"wipe_import_result,omitempty"`
	WipeImportResultStatus *exportTreeArtifactStatus      `json:"wipe_import_result_status,omitempty"`
}

type exportTreeStatus struct {
	Format                             string                 `json:"format"`
	Present                            bool                   `json:"present"`
	ExportTree                         string                 `json:"export_tree,omitempty"`
	KindCount                          int                    `json:"kind_count,omitempty"`
	QuarantinePresentCount             int                    `json:"quarantine_present_count,omitempty"`
	QuarantineComplete                 bool                   `json:"quarantine_complete,omitempty"`
	WipeQuarantinePresentCount         int                    `json:"wipe_quarantine_present_count,omitempty"`
	TwoPhaseWipeArtifactsComplete      bool                   `json:"two_phase_wipe_artifacts_complete,omitempty"`
	ImportResultPresentCount           int                    `json:"import_result_present_count,omitempty"`
	ImportResultStatusPresentCount     int                    `json:"import_result_status_present_count,omitempty"`
	ImportResultArtifactsComplete      bool                   `json:"import_result_artifacts_complete,omitempty"`
	ImportResultReplacedCount          int                    `json:"import_result_replaced_count,omitempty"`
	ImportResultRowCountTotal          int                    `json:"import_result_row_count_total,omitempty"`
	ImportResultOutcomesComplete       bool                   `json:"import_result_outcomes_complete,omitempty"`
	ImportResultAllReplaced            bool                   `json:"import_result_all_replaced,omitempty"`
	WipeImportResultPresentCount       int                    `json:"wipe_import_result_present_count,omitempty"`
	WipeImportResultStatusPresentCount int                    `json:"wipe_import_result_status_present_count,omitempty"`
	WipeImportArtifactsComplete        bool                   `json:"wipe_import_artifacts_complete,omitempty"`
	Kinds                              []exportTreeKindStatus `json:"kinds,omitempty"`
}

func runExportTreeStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("export-tree-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var exportTree string
	var requireQuarantineComplete bool
	var requireTwoPhaseWipeArtifactsComplete bool
	var requireImportResultArtifactsComplete bool
	var requireWipeImportArtifactsComplete bool
	flags.StringVar(&exportTree, "export-tree", "", "absolute path to a retained export/quarantine tree")
	flags.BoolVar(&requireQuarantineComplete, "require-quarantine-complete", false, "fail closed unless quarantine_complete is true")
	flags.BoolVar(&requireTwoPhaseWipeArtifactsComplete, "require-two-phase-wipe-artifacts-complete", false, "fail closed unless two_phase_wipe_artifacts_complete is true")
	flags.BoolVar(&requireImportResultArtifactsComplete, "require-import-result-artifacts-complete", false, "fail closed unless import_result_artifacts_complete is true")
	flags.BoolVar(&requireWipeImportArtifactsComplete, "require-wipe-import-artifacts-complete", false, "fail closed unless wipe_import_artifacts_complete is true")
	flags.Usage = func() { printExportTreeStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected export-tree-status argument %q\n", flags.Arg(0))
		printExportTreeStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(exportTree) == "" {
		fmt.Fprintln(stderr, "--export-tree is required for export-tree-status")
		printExportTreeStatusUsage(stderr)
		return exitUsage
	}

	status, err := inspectExportTreeStatus(exportTree)
	if err != nil {
		fmt.Fprintf(stderr, "export-tree status: %v\n", err)
		return exitError
	}
	if err := enforceExportTreeStatusRequireGates(status, exportTreeStatusRequireGates{
		QuarantineComplete:            requireQuarantineComplete,
		TwoPhaseWipeArtifactsComplete: requireTwoPhaseWipeArtifactsComplete,
		ImportResultArtifactsComplete: requireImportResultArtifactsComplete,
		WipeImportArtifactsComplete:   requireWipeImportArtifactsComplete,
	}); err != nil {
		fmt.Fprintf(stderr, "export-tree status: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, status)
}

type exportTreeStatusRequireGates struct {
	QuarantineComplete            bool
	TwoPhaseWipeArtifactsComplete bool
	ImportResultArtifactsComplete bool
	WipeImportArtifactsComplete   bool
}

func enforceExportTreeStatusRequireGates(status exportTreeStatus, gates exportTreeStatusRequireGates) error {
	if !gates.QuarantineComplete && !gates.TwoPhaseWipeArtifactsComplete && !gates.ImportResultArtifactsComplete && !gates.WipeImportArtifactsComplete {
		return nil
	}
	if !status.Present {
		switch {
		case gates.QuarantineComplete:
			return fmt.Errorf("%w: --require-quarantine-complete failed: export-tree is absent", ErrExportTreeStatus)
		case gates.TwoPhaseWipeArtifactsComplete:
			return fmt.Errorf("%w: --require-two-phase-wipe-artifacts-complete failed: export-tree is absent", ErrExportTreeStatus)
		case gates.ImportResultArtifactsComplete:
			return fmt.Errorf("%w: --require-import-result-artifacts-complete failed: export-tree is absent", ErrExportTreeStatus)
		default:
			return fmt.Errorf("%w: --require-wipe-import-artifacts-complete failed: export-tree is absent", ErrExportTreeStatus)
		}
	}
	if gates.QuarantineComplete && !status.QuarantineComplete {
		return fmt.Errorf("%w: --require-quarantine-complete failed: quarantine_complete=false", ErrExportTreeStatus)
	}
	if gates.TwoPhaseWipeArtifactsComplete && !status.TwoPhaseWipeArtifactsComplete {
		return fmt.Errorf("%w: --require-two-phase-wipe-artifacts-complete failed: two_phase_wipe_artifacts_complete=false", ErrExportTreeStatus)
	}
	if gates.ImportResultArtifactsComplete && !status.ImportResultArtifactsComplete {
		return fmt.Errorf("%w: --require-import-result-artifacts-complete failed: import_result_artifacts_complete=false", ErrExportTreeStatus)
	}
	if gates.WipeImportArtifactsComplete && !status.WipeImportArtifactsComplete {
		return fmt.Errorf("%w: --require-wipe-import-artifacts-complete failed: wipe_import_artifacts_complete=false", ErrExportTreeStatus)
	}
	return nil
}

func inspectExportTreeStatus(exportTree string) (exportTreeStatus, error) {
	normalizedTree, err := normalizeImportExportDrillAbsolutePath(exportTree, "export-tree")
	if err != nil {
		return exportTreeStatus{}, fmt.Errorf("%w: %v", ErrExportTreeStatus, err)
	}

	info, err := os.Lstat(normalizedTree)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return exportTreeStatus{
				Format:  exportTreeStatusFormat,
				Present: false,
			}, nil
		}
		return exportTreeStatus{}, fmt.Errorf("%w: stat export-tree: %v", ErrExportTreeStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return exportTreeStatus{}, fmt.Errorf("%w: export-tree must not be a symlink: %s", ErrExportTreeStatus, normalizedTree)
	}
	if !info.IsDir() {
		return exportTreeStatus{}, fmt.Errorf("%w: export-tree must be a directory: %s", ErrExportTreeStatus, normalizedTree)
	}

	status := exportTreeStatus{
		Format:     exportTreeStatusFormat,
		Present:    true,
		ExportTree: normalizedTree,
		KindCount:  len(exportQuarantineKinds),
		Kinds:      make([]exportTreeKindStatus, 0, len(exportQuarantineKinds)),
	}

	wipeKindSet := make(map[string]struct{}, len(importExportDrillWipeKinds))
	for _, kind := range importExportDrillWipeKinds {
		wipeKindSet[kind] = struct{}{}
	}

	quarantinePresent := 0
	wipeQuarantinePresent := 0
	wipeStatusPresent := 0
	importResultPresent := 0
	importResultStatusPresent := 0
	wipeImportResultPresent := 0
	wipeImportResultStatusPresent := 0

	for _, kind := range exportQuarantineKinds {
		_, isWipeKind := wipeKindSet[kind]
		kindStatus := exportTreeKindStatus{
			Kind:     kind,
			WipeKind: isWipeKind,
		}

		quarantineRel := filepath.ToSlash(filepath.Join(kind, "quarantine.json"))
		quarantineAbs := filepath.Join(normalizedTree, kind, "quarantine.json")
		quarantineArtifact, err := inspectExportTreeQuarantineArtifact(kind, quarantineAbs, quarantineRel)
		if err != nil {
			return exportTreeStatus{}, err
		}
		kindStatus.Quarantine = quarantineArtifact
		if quarantineArtifact.Present {
			quarantinePresent++
		}

		importResultRel := filepath.ToSlash(filepath.Join(kind, "import-result.json"))
		importResultAbs := filepath.Join(normalizedTree, kind, "import-result.json")
		importResultArtifact, importResultSHA, importResultOutcome, err := inspectExportTreeImportResultArtifact(kind, importResultAbs, importResultRel)
		if err != nil {
			return exportTreeStatus{}, err
		}
		kindStatus.ImportResult = importResultArtifact
		if importResultArtifact.Present {
			importResultPresent++
			kindStatus.ImportResultOutcome = importResultOutcome
		}

		importResultStatusRel := filepath.ToSlash(filepath.Join(kind, "import-result-status.json"))
		importResultStatusAbs := filepath.Join(normalizedTree, kind, "import-result-status.json")
		importResultStatusArtifact, err := inspectExportTreeImportResultStatusArtifact(kind, importResultStatusAbs, importResultStatusRel, importResultSHA)
		if err != nil {
			return exportTreeStatus{}, err
		}
		kindStatus.ImportResultStatus = importResultStatusArtifact
		if importResultStatusArtifact.Present {
			importResultStatusPresent++
		}

		if isWipeKind {
			wipeQuarantineRel := filepath.ToSlash(filepath.Join(kind, "wipe-quarantine.json"))
			wipeQuarantineAbs := filepath.Join(normalizedTree, kind, "wipe-quarantine.json")
			wipeQuarantineArtifact, wipeSHA, wipeScopeKey, wipeScopeCount, err := inspectExportTreeWipeQuarantineArtifact(kind, wipeQuarantineAbs, wipeQuarantineRel)
			if err != nil {
				return exportTreeStatus{}, err
			}
			kindStatus.WipeQuarantine = &wipeQuarantineArtifact
			if wipeQuarantineArtifact.Present {
				wipeQuarantinePresent++
			}

			wipeStatusRel := filepath.ToSlash(filepath.Join(kind, "wipe-quarantine-status.json"))
			wipeStatusAbs := filepath.Join(normalizedTree, kind, "wipe-quarantine-status.json")
			wipeStatusArtifact, err := inspectExportTreeWipeQuarantineStatusArtifact(kind, wipeStatusAbs, wipeStatusRel, wipeSHA)
			if err != nil {
				return exportTreeStatus{}, err
			}
			kindStatus.WipeQuarantineStatus = &wipeStatusArtifact
			if wipeStatusArtifact.Present {
				wipeStatusPresent++
			}
			if wipeQuarantineArtifact.Present {
				kindStatus.WipeQuarantine.ScopeKey = wipeScopeKey
				kindStatus.WipeQuarantine.ScopeCount = wipeScopeCount
			}

			wipeImportRel := filepath.ToSlash(filepath.Join(kind, "wipe-import-result.json"))
			wipeImportAbs := filepath.Join(normalizedTree, kind, "wipe-import-result.json")
			wipeImportArtifact, wipeImportSHA, _, err := inspectExportTreeImportResultArtifact(kind, wipeImportAbs, wipeImportRel)
			if err != nil {
				return exportTreeStatus{}, err
			}
			kindStatus.WipeImportResult = &wipeImportArtifact
			if wipeImportArtifact.Present {
				wipeImportResultPresent++
			}

			wipeImportStatusRel := filepath.ToSlash(filepath.Join(kind, "wipe-import-result-status.json"))
			wipeImportStatusAbs := filepath.Join(normalizedTree, kind, "wipe-import-result-status.json")
			wipeImportStatusArtifact, err := inspectExportTreeImportResultStatusArtifact(kind, wipeImportStatusAbs, wipeImportStatusRel, wipeImportSHA)
			if err != nil {
				return exportTreeStatus{}, err
			}
			kindStatus.WipeImportResultStatus = &wipeImportStatusArtifact
			if wipeImportStatusArtifact.Present {
				wipeImportResultStatusPresent++
			}
		}

		status.Kinds = append(status.Kinds, kindStatus)
	}

	status.QuarantinePresentCount = quarantinePresent
	status.QuarantineComplete = quarantinePresent == len(exportQuarantineKinds)
	status.WipeQuarantinePresentCount = wipeQuarantinePresent
	status.TwoPhaseWipeArtifactsComplete = wipeQuarantinePresent == len(importExportDrillWipeKinds) &&
		wipeStatusPresent == len(importExportDrillWipeKinds)
	status.ImportResultPresentCount = importResultPresent
	status.ImportResultStatusPresentCount = importResultStatusPresent
	status.ImportResultArtifactsComplete = importResultPresent == len(exportQuarantineKinds) &&
		importResultStatusPresent == len(exportQuarantineKinds)
	replacedCount := 0
	rowCountTotal := 0
	outcomeCount := 0
	for _, entry := range status.Kinds {
		if entry.ImportResultOutcome == nil {
			continue
		}
		outcomeCount++
		rowCountTotal += entry.ImportResultOutcome.RowCount
		if entry.ImportResultOutcome.Replaced {
			replacedCount++
		}
	}
	status.ImportResultReplacedCount = replacedCount
	status.ImportResultRowCountTotal = rowCountTotal
	status.ImportResultOutcomesComplete = outcomeCount == len(exportQuarantineKinds)
	status.ImportResultAllReplaced = status.ImportResultOutcomesComplete && replacedCount == len(exportQuarantineKinds)
	status.WipeImportResultPresentCount = wipeImportResultPresent
	status.WipeImportResultStatusPresentCount = wipeImportResultStatusPresent
	status.WipeImportArtifactsComplete = wipeImportResultPresent == len(importExportDrillWipeKinds) &&
		wipeImportResultStatusPresent == len(importExportDrillWipeKinds)
	return status, nil
}

func inspectExportTreeQuarantineArtifact(kind, absPath, relPath string) (exportTreeArtifactStatus, error) {
	raw, present, err := readOptionalExportTreeRegularFile(absPath, maxExportQuarantineBytes, "quarantine")
	if err != nil {
		return exportTreeArtifactStatus{}, err
	}
	if !present {
		return exportTreeArtifactStatus{Present: false}, nil
	}
	exportRaw, err := normalizeImportExportJSON(raw)
	if err != nil {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: %v", ErrExportTreeStatus, relPath, err)
	}
	if _, err := quarantineExportJSON(kind, exportRaw); err != nil {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: %v", ErrExportTreeStatus, relPath, err)
	}
	return exportTreeArtifactStatus{
		Present: true,
		Path:    relPath,
		SHA256:  sha256Hex(raw),
	}, nil
}

func inspectExportTreeWipeQuarantineArtifact(kind, absPath, relPath string) (exportTreeArtifactStatus, string, string, int, error) {
	export, present, raw, scopeKey, scopeIDs, err := readSynthesizeWipeExportStatusFile(kind, absPath)
	if err != nil {
		return exportTreeArtifactStatus{}, "", "", 0, fmt.Errorf("%w: %s: %v", ErrExportTreeStatus, relPath, err)
	}
	if !present {
		return exportTreeArtifactStatus{Present: false}, "", "", 0, nil
	}
	_ = export
	return exportTreeArtifactStatus{
		Present:    true,
		Path:       relPath,
		SHA256:     sha256Hex(raw),
		ScopeKey:   scopeKey,
		ScopeCount: len(scopeIDs),
	}, sha256Hex(raw), scopeKey, len(scopeIDs), nil
}

func inspectExportTreeWipeQuarantineStatusArtifact(kind, absPath, relPath, wipeSHA string) (exportTreeArtifactStatus, error) {
	raw, present, err := readOptionalExportTreeRegularFile(absPath, maxSynthesizeWipeExportBytes, "wipe-quarantine-status")
	if err != nil {
		return exportTreeArtifactStatus{}, err
	}
	if !present {
		return exportTreeArtifactStatus{Present: false}, nil
	}
	var status synthesizeWipeExportStatus
	if err := decodeStrictExportTreeStatusJSON(raw, &status, "wipe-quarantine-status"); err != nil {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: %v", ErrExportTreeStatus, relPath, err)
	}
	if status.Format != synthesizeWipeExportStatusFormat {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: unexpected format %q", ErrExportTreeStatus, relPath, status.Format)
	}
	if !status.Present {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: present must be true for retained wipe-quarantine-status", ErrExportTreeStatus, relPath)
	}
	if status.Kind != kind {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: kind mismatch got %q want %q", ErrExportTreeStatus, relPath, status.Kind, kind)
	}
	if wipeSHA == "" {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: wipe-quarantine.json must be present beside wipe-quarantine-status.json", ErrExportTreeStatus, relPath)
	}
	if status.WipeExportSHA256 != wipeSHA {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: wipe_export_sha256 mismatch", ErrExportTreeStatus, relPath)
	}
	return exportTreeArtifactStatus{
		Present: true,
		Path:    relPath,
		SHA256:  sha256Hex(raw),
	}, nil
}

func inspectExportTreeImportResultArtifact(kind, absPath, relPath string) (exportTreeArtifactStatus, string, *exportTreeImportResultOutcome, error) {
	result, present, raw, err := readImportExportStatusFile(kind, absPath)
	if err != nil {
		return exportTreeArtifactStatus{}, "", nil, fmt.Errorf("%w: %s: %v", ErrExportTreeStatus, relPath, err)
	}
	if !present {
		return exportTreeArtifactStatus{Present: false}, "", nil, nil
	}
	outcome, err := importResultOutcomeFromDecoded(kind, result)
	if err != nil {
		return exportTreeArtifactStatus{}, "", nil, fmt.Errorf("%w: %s: %v", ErrExportTreeStatus, relPath, err)
	}
	sum := sha256Hex(raw)
	return exportTreeArtifactStatus{
		Present: true,
		Path:    relPath,
		SHA256:  sum,
	}, sum, &outcome, nil
}

func importResultOutcomeFromDecoded(kind string, result any) (exportTreeImportResultOutcome, error) {
	switch typed := result.(type) {
	case accountstore.AccountCharacterRosterImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.CharacterCount}, nil
	case accountstore.CharacterItemStateImportResult:
		return exportTreeImportResultOutcome{
			Replaced: typed.Replaced,
			RowCount: typed.InventoryItemCount + typed.EquipmentItemCount + typed.QuickslotCount,
		}, nil
	case accountstore.CharacterPointStateImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.PointRowCount}, nil
	case accountstore.CharacterMyShopUnitPricesImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.PriceRowCount}, nil
	case queststate.CharacterQuestStateImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.FlagCount}, nil
	case safeboxstore.CharacterSafeboxStateImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.ItemCount}, nil
	case loginticket.AuthLoginTicketHandoffImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.TicketCount}, nil
	case itemstore.ItemTemplateStateImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.TemplateCount}, nil
	case staticstore.StaticActorContentStateImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.StaticActorCount}, nil
	case worldruntime.BootstrapGroundItemStateImportResult:
		return exportTreeImportResultOutcome{Replaced: typed.Replaced, RowCount: typed.GroundItemCount}, nil
	default:
		return exportTreeImportResultOutcome{}, fmt.Errorf("unsupported import-result outcome kind %q (%T)", kind, result)
	}
}

func inspectExportTreeImportResultStatusArtifact(kind, absPath, relPath, importResultSHA string) (exportTreeArtifactStatus, error) {
	raw, present, err := readOptionalExportTreeRegularFile(absPath, maxImportExportResultBytes, "import-result-status")
	if err != nil {
		return exportTreeArtifactStatus{}, err
	}
	if !present {
		return exportTreeArtifactStatus{Present: false}, nil
	}
	var status importExportStatus
	if err := decodeStrictExportTreeStatusJSON(raw, &status, "import-result-status"); err != nil {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: %v", ErrExportTreeStatus, relPath, err)
	}
	if status.Format != importExportStatusFormat {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: unexpected format %q", ErrExportTreeStatus, relPath, status.Format)
	}
	if !status.Present {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: present must be true for retained import-result-status", ErrExportTreeStatus, relPath)
	}
	if status.Kind != kind {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: kind mismatch got %q want %q", ErrExportTreeStatus, relPath, status.Kind, kind)
	}
	if importResultSHA == "" {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: import-result.json must be present beside import-result-status.json", ErrExportTreeStatus, relPath)
	}
	if status.ImportResultSHA256 != importResultSHA {
		return exportTreeArtifactStatus{}, fmt.Errorf("%w: %s: import_result_sha256 mismatch", ErrExportTreeStatus, relPath)
	}
	return exportTreeArtifactStatus{
		Present: true,
		Path:    relPath,
		SHA256:  sha256Hex(raw),
	}, nil
}

func readOptionalExportTreeRegularFile(path string, maxBytes int, label string) ([]byte, bool, error) {
	trimmed := strings.TrimSpace(path)
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%w: stat %s: %v", ErrExportTreeStatus, label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("%w: %s must not be a symlink: %s", ErrExportTreeStatus, label, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: %s must be a regular file: %s", ErrExportTreeStatus, label, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, false, fmt.Errorf("%w: open %s: %v", ErrExportTreeStatus, label, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("%w: stat opened %s: %v", ErrExportTreeStatus, label, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: opened %s must be a regular file: %s", ErrExportTreeStatus, label, trimmed)
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, false, fmt.Errorf("%w: read %s: %v", ErrExportTreeStatus, label, err)
	}
	if len(raw) > maxBytes {
		return nil, false, fmt.Errorf("%w: %s exceeds %d bytes", ErrExportTreeStatus, label, maxBytes)
	}
	if !utf8.Valid(raw) {
		return nil, false, fmt.Errorf("%w: %s is not valid UTF-8", ErrExportTreeStatus, label)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, fmt.Errorf("%w: %s is empty", ErrExportTreeStatus, label)
	}
	return raw, true, nil
}

func decodeStrictExportTreeStatusJSON(raw []byte, dest any, label string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("decode %s: %v", label, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s has trailing JSON", label)
	}
	return nil
}

func printExportTreeStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "export-tree-status usage:")
	fmt.Fprintln(w, "  metin2-migrate export-tree-status --export-tree <absolute-path> [--require-quarantine-complete] [--require-two-phase-wipe-artifacts-complete] [--require-import-result-artifacts-complete] [--require-wipe-import-artifacts-complete]")
}
