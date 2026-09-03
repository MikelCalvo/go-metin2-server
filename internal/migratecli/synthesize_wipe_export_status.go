package migratecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const (
	synthesizeWipeExportStatusFormat = "go-metin2-synthesize-wipe-export-status-v1"
	maxSynthesizeWipeExportBytes     = maxExportQuarantineBytes
)

// ErrSynthesizeWipeExportStatus reports a fail-closed wipe-export inspection failure.
var ErrSynthesizeWipeExportStatus = errors.New("synthesize-wipe-export status failed")

type synthesizeWipeExportStatus struct {
	Format           string   `json:"format"`
	Present          bool     `json:"present"`
	Kind             string   `json:"kind,omitempty"`
	WipeExportSHA256 string   `json:"wipe_export_sha256,omitempty"`
	ScopeKey         string   `json:"scope_key,omitempty"`
	ScopeCount       int      `json:"scope_count,omitempty"`
	ScopeIDs         []uint32 `json:"scope_ids,omitempty"`
	Export           any      `json:"export,omitempty"`
}

func runSynthesizeWipeExportStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("synthesize-wipe-export-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var kind string
	var wipeExportPath string
	flags.StringVar(&kind, "kind", "", "character-FK wipe tip kind to inspect")
	flags.StringVar(&wipeExportPath, "wipe-export", "", "path to a retained synthesize-wipe-export JSON file")
	flags.Usage = func() { printSynthesizeWipeExportStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected synthesize-wipe-export-status argument %q\n", flags.Arg(0))
		printSynthesizeWipeExportStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(kind) == "" || strings.TrimSpace(wipeExportPath) == "" {
		fmt.Fprintln(stderr, "--kind and --wipe-export are required for synthesize-wipe-export-status")
		printSynthesizeWipeExportStatusUsage(stderr)
		return exitUsage
	}
	if !isSupportedSynthesizeWipeExportKind(kind) {
		fmt.Fprintf(stderr, "unsupported synthesize-wipe-export-status kind %q\n", kind)
		printSynthesizeWipeExportStatusUsage(stderr)
		return exitUsage
	}

	export, present, raw, scopeKey, scopeIDs, err := readSynthesizeWipeExportStatusFile(kind, wipeExportPath)
	if err != nil {
		fmt.Fprintf(stderr, "synthesize-wipe-export status: %v\n", err)
		return exitError
	}
	status := synthesizeWipeExportStatus{
		Format:  synthesizeWipeExportStatusFormat,
		Present: present,
	}
	if present {
		status.Kind = kind
		status.WipeExportSHA256 = sha256Hex(raw)
		status.ScopeKey = scopeKey
		status.ScopeCount = len(scopeIDs)
		status.ScopeIDs = emptyUint32Slice(scopeIDs)
		status.Export = export
	}
	return writeJSON(stdout, stderr, status)
}

func readSynthesizeWipeExportStatusFile(kind, path string) (any, bool, []byte, string, []uint32, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, false, nil, "", nil, fmt.Errorf("%w: wipe-export path is required", ErrSynthesizeWipeExportStatus)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil, "", nil, nil
		}
		return nil, false, nil, "", nil, fmt.Errorf("%w: stat wipe-export: %v", ErrSynthesizeWipeExportStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, nil, "", nil, fmt.Errorf("%w: wipe-export must not be a symlink: %s", ErrSynthesizeWipeExportStatus, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, false, nil, "", nil, fmt.Errorf("%w: wipe-export must be a regular file: %s", ErrSynthesizeWipeExportStatus, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, false, nil, "", nil, fmt.Errorf("%w: open wipe-export: %v", ErrSynthesizeWipeExportStatus, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, nil, "", nil, fmt.Errorf("%w: stat opened wipe-export: %v", ErrSynthesizeWipeExportStatus, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, false, nil, "", nil, fmt.Errorf("%w: opened wipe-export must be a regular file: %s", ErrSynthesizeWipeExportStatus, trimmed)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxSynthesizeWipeExportBytes+1))
	if err != nil {
		return nil, false, nil, "", nil, fmt.Errorf("%w: read wipe-export: %v", ErrSynthesizeWipeExportStatus, err)
	}
	if len(raw) > maxSynthesizeWipeExportBytes {
		return nil, false, nil, "", nil, fmt.Errorf("%w: wipe-export exceeds %d bytes", ErrSynthesizeWipeExportStatus, maxSynthesizeWipeExportBytes)
	}
	export, scopeKey, scopeIDs, err := decodeSynthesizeWipeExportStatus(kind, raw)
	if err != nil {
		return nil, false, nil, "", nil, err
	}
	return export, true, raw, scopeKey, scopeIDs, nil
}

func decodeSynthesizeWipeExportStatus(kind string, raw []byte) (any, string, []uint32, error) {
	if !utf8.Valid(raw) {
		return nil, "", nil, fmt.Errorf("%w: wipe-export is not valid UTF-8", ErrSynthesizeWipeExportStatus)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, "", nil, fmt.Errorf("%w: wipe-export is empty", ErrSynthesizeWipeExportStatus)
	}
	if err := rejectWrappedQuarantineWipeExport(raw); err != nil {
		return nil, "", nil, err
	}

	switch kind {
	case "character-item-state":
		var export accountstore.CharacterItemStateExport
		if err := decodeStrictSynthesizeWipeExportStatusJSON(raw, &export); err != nil {
			return nil, "", nil, err
		}
		canonical, summary, err := accountstore.QuarantineCharacterItemStateExport(export)
		if err != nil {
			return nil, "", nil, fmt.Errorf("%w: %v", ErrSynthesizeWipeExportStatus, err)
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe scope character_ids is empty", ErrSynthesizeWipeExportStatus)
		}
		if len(canonical.InventoryItems) != 0 || len(canonical.EquipmentItems) != 0 || len(canonical.Quickslots) != 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe export row arrays must be empty", ErrSynthesizeWipeExportStatus)
		}
		canonical.InventoryItems = emptyCharacterInventoryItemRows(canonical.InventoryItems)
		canonical.EquipmentItems = emptyCharacterEquipmentItemRows(canonical.EquipmentItems)
		canonical.Quickslots = emptyCharacterQuickslotRows(canonical.Quickslots)
		return canonical, "character_ids", append([]uint32(nil), summary.CharacterIDs...), nil
	case "character-point-state":
		var export accountstore.CharacterPointStateExport
		if err := decodeStrictSynthesizeWipeExportStatusJSON(raw, &export); err != nil {
			return nil, "", nil, err
		}
		canonical, summary, err := accountstore.QuarantineCharacterPointStateExport(export)
		if err != nil {
			return nil, "", nil, fmt.Errorf("%w: %v", ErrSynthesizeWipeExportStatus, err)
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe scope character_ids is empty", ErrSynthesizeWipeExportStatus)
		}
		if len(canonical.Points) != 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe export row arrays must be empty", ErrSynthesizeWipeExportStatus)
		}
		canonical.Points = emptyCharacterPointRows(canonical.Points)
		return canonical, "character_ids", append([]uint32(nil), summary.CharacterIDs...), nil
	case "character-myshop-unit-prices":
		var export accountstore.CharacterMyShopUnitPricesExport
		if err := decodeStrictSynthesizeWipeExportStatusJSON(raw, &export); err != nil {
			return nil, "", nil, err
		}
		canonical, summary, err := accountstore.QuarantineCharacterMyShopUnitPricesExport(export)
		if err != nil {
			return nil, "", nil, fmt.Errorf("%w: %v", ErrSynthesizeWipeExportStatus, err)
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe scope character_ids is empty", ErrSynthesizeWipeExportStatus)
		}
		if len(canonical.UnitPrices) != 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe export row arrays must be empty", ErrSynthesizeWipeExportStatus)
		}
		canonical.UnitPrices = emptyCharacterMyShopUnitPriceRows(canonical.UnitPrices)
		return canonical, "character_ids", append([]uint32(nil), summary.CharacterIDs...), nil
	case "character-quest-state":
		var export queststate.CharacterQuestStateExport
		if err := decodeStrictSynthesizeWipeExportStatusJSON(raw, &export); err != nil {
			return nil, "", nil, err
		}
		canonical, summary, err := queststate.QuarantineCharacterQuestStateExport(export)
		if err != nil {
			return nil, "", nil, fmt.Errorf("%w: %v", ErrSynthesizeWipeExportStatus, err)
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe scope character_ids is empty", ErrSynthesizeWipeExportStatus)
		}
		if len(canonical.Flags) != 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe export row arrays must be empty", ErrSynthesizeWipeExportStatus)
		}
		canonical.Flags = emptyCharacterQuestFlagRows(canonical.Flags)
		return canonical, "character_ids", append([]uint32(nil), summary.CharacterIDs...), nil
	case "character-safebox-state":
		var export safeboxstore.CharacterSafeboxStateExport
		if err := decodeStrictSynthesizeWipeExportStatusJSON(raw, &export); err != nil {
			return nil, "", nil, err
		}
		canonical, summary, err := safeboxstore.QuarantineCharacterSafeboxStateExport(export)
		if err != nil {
			return nil, "", nil, fmt.Errorf("%w: %v", ErrSynthesizeWipeExportStatus, err)
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe scope character_ids is empty", ErrSynthesizeWipeExportStatus)
		}
		if len(canonical.Items) != 0 || len(canonical.Passwords) != 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe export row arrays must be empty", ErrSynthesizeWipeExportStatus)
		}
		canonical.Items = emptyCharacterSafeboxItemRows(canonical.Items)
		canonical.Passwords = emptyCharacterSafeboxPasswordRows(canonical.Passwords)
		return canonical, "character_ids", append([]uint32(nil), summary.CharacterIDs...), nil
	case "bootstrap-ground-item-state":
		var export worldruntime.BootstrapGroundItemStateExport
		if err := decodeStrictSynthesizeWipeExportStatusJSON(raw, &export); err != nil {
			return nil, "", nil, err
		}
		canonical, summary, err := worldruntime.QuarantineBootstrapGroundItemStateExport(export)
		if err != nil {
			return nil, "", nil, fmt.Errorf("%w: %v", ErrSynthesizeWipeExportStatus, err)
		}
		if len(summary.VIDs) == 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe scope vids is empty", ErrSynthesizeWipeExportStatus)
		}
		if len(canonical.GroundItems) != 0 {
			return nil, "", nil, fmt.Errorf("%w: wipe export row arrays must be empty", ErrSynthesizeWipeExportStatus)
		}
		canonical.GroundItems = emptyBootstrapGroundItemRows(canonical.GroundItems)
		return canonical, "vids", append([]uint32(nil), summary.VIDs...), nil
	default:
		return nil, "", nil, fmt.Errorf("%w: unsupported kind %q", ErrSynthesizeWipeExportStatus, kind)
	}
}

func rejectWrappedQuarantineWipeExport(raw []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return fmt.Errorf("%w: decode wipe-export: %v", ErrSynthesizeWipeExportStatus, err)
	}
	_, hasExport := top["export"]
	_, hasSummary := top["summary"]
	if hasExport && hasSummary {
		return fmt.Errorf("%w: wipe-export must be bare migration-shaped JSON (wrapped quarantine rejected)", ErrSynthesizeWipeExportStatus)
	}
	return nil
}

func decodeStrictSynthesizeWipeExportStatusJSON(raw []byte, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("%w: decode wipe-export: %v", ErrSynthesizeWipeExportStatus, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: wipe-export has trailing JSON", ErrSynthesizeWipeExportStatus)
	}
	return nil
}

func emptyCharacterInventoryItemRows(values []accountstore.CharacterInventoryItemRow) []accountstore.CharacterInventoryItemRow {
	if values == nil {
		return []accountstore.CharacterInventoryItemRow{}
	}
	return values
}

func emptyCharacterEquipmentItemRows(values []accountstore.CharacterEquipmentItemRow) []accountstore.CharacterEquipmentItemRow {
	if values == nil {
		return []accountstore.CharacterEquipmentItemRow{}
	}
	return values
}

func emptyCharacterQuickslotRows(values []accountstore.CharacterQuickslotRow) []accountstore.CharacterQuickslotRow {
	if values == nil {
		return []accountstore.CharacterQuickslotRow{}
	}
	return values
}

func emptyCharacterPointRows(values []accountstore.CharacterPointRow) []accountstore.CharacterPointRow {
	if values == nil {
		return []accountstore.CharacterPointRow{}
	}
	return values
}

func emptyCharacterMyShopUnitPriceRows(values []accountstore.CharacterMyShopUnitPriceRow) []accountstore.CharacterMyShopUnitPriceRow {
	if values == nil {
		return []accountstore.CharacterMyShopUnitPriceRow{}
	}
	return values
}

func emptyCharacterQuestFlagRows(values []queststate.CharacterQuestFlagRow) []queststate.CharacterQuestFlagRow {
	if values == nil {
		return []queststate.CharacterQuestFlagRow{}
	}
	return values
}

func emptyCharacterSafeboxItemRows(values []safeboxstore.CharacterSafeboxItemRow) []safeboxstore.CharacterSafeboxItemRow {
	if values == nil {
		return []safeboxstore.CharacterSafeboxItemRow{}
	}
	return values
}

func emptyCharacterSafeboxPasswordRows(values []safeboxstore.CharacterSafeboxPasswordRow) []safeboxstore.CharacterSafeboxPasswordRow {
	if values == nil {
		return []safeboxstore.CharacterSafeboxPasswordRow{}
	}
	return values
}

func emptyBootstrapGroundItemRows(values []worldruntime.BootstrapGroundItemStateRow) []worldruntime.BootstrapGroundItemStateRow {
	if values == nil {
		return []worldruntime.BootstrapGroundItemStateRow{}
	}
	return values
}

func printSynthesizeWipeExportStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "synthesize-wipe-export-status usage:")
	fmt.Fprintln(w, "  metin2-migrate synthesize-wipe-export-status --kind <kind> --wipe-export <path>")
	fmt.Fprintln(w, "kinds:")
	for _, kind := range importExportDrillWipeKinds {
		fmt.Fprintf(w, "  %s\n", kind)
	}
}
