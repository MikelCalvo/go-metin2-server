package migratecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

var errInvalidSynthesizeWipeExportInput = errors.New("invalid synthesize-wipe-export input")

// importExportDrillWipeKinds are the character-FK tip kinds that must be wiped
// before tip-0002 roster scoped replace on a seeded/populated tree.
var importExportDrillWipeKinds = []string{
	"character-item-state",
	"character-point-state",
	"character-myshop-unit-prices",
	"character-quest-state",
	"character-safebox-state",
	"bootstrap-ground-item-state",
}

func runSynthesizeWipeExport(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("synthesize-wipe-export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var kind string
	var exportPath string
	flags.StringVar(&kind, "kind", "", "character-FK tip kind to synthesize a wipe-scope export for")
	flags.StringVar(&exportPath, "export", "", "path to retained quarantine/export JSON, or - for stdin")
	flags.Usage = func() { printSynthesizeWipeExportUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected synthesize-wipe-export argument %q\n", flags.Arg(0))
		printSynthesizeWipeExportUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(kind) == "" || strings.TrimSpace(exportPath) == "" {
		fmt.Fprintln(stderr, "--kind and --export are required for synthesize-wipe-export")
		printSynthesizeWipeExportUsage(stderr)
		return exitUsage
	}
	if !isSupportedSynthesizeWipeExportKind(kind) {
		fmt.Fprintf(stderr, "unsupported synthesize-wipe-export kind %q\n", kind)
		printSynthesizeWipeExportUsage(stderr)
		return exitUsage
	}

	reader, closeReader, err := openExportQuarantineReader(exportPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "synthesize-wipe-export: %v\n", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	raw, err := readBoundedExportQuarantine(reader)
	if err != nil {
		fmt.Fprintf(stderr, "synthesize-wipe-export: %v\n", err)
		return exitError
	}

	wipeExport, err := synthesizeWipeExportJSON(kind, raw)
	if err != nil {
		fmt.Fprintf(stderr, "synthesize-wipe-export: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, wipeExport)
}

func isSupportedSynthesizeWipeExportKind(kind string) bool {
	for _, supported := range importExportDrillWipeKinds {
		if kind == supported {
			return true
		}
	}
	return false
}

func synthesizeWipeExportJSON(kind string, raw []byte) (any, error) {
	exportRaw, err := normalizeImportExportJSON(raw)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "character-item-state":
		var export accountstore.CharacterItemStateExport
		if err := decodeStrictJSON(exportRaw, &export); err != nil {
			return nil, err
		}
		_, summary, err := accountstore.QuarantineCharacterItemStateExport(export)
		if err != nil {
			return nil, err
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, fmt.Errorf("%w: wipe scope character_ids is empty", errInvalidSynthesizeWipeExportInput)
		}
		return accountstore.CharacterItemStateExport{
			MigrationVersion: accountstore.CharacterItemStateMigrationVersion,
			MigrationName:    accountstore.CharacterItemStateMigrationName,
			CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
			InventoryItems:   []accountstore.CharacterInventoryItemRow{},
			EquipmentItems:   []accountstore.CharacterEquipmentItemRow{},
			Quickslots:       []accountstore.CharacterQuickslotRow{},
		}, nil
	case "character-point-state":
		var export accountstore.CharacterPointStateExport
		if err := decodeStrictJSON(exportRaw, &export); err != nil {
			return nil, err
		}
		_, summary, err := accountstore.QuarantineCharacterPointStateExport(export)
		if err != nil {
			return nil, err
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, fmt.Errorf("%w: wipe scope character_ids is empty", errInvalidSynthesizeWipeExportInput)
		}
		return accountstore.CharacterPointStateExport{
			MigrationVersion: accountstore.CharacterPointStateMigrationVersion,
			MigrationName:    accountstore.CharacterPointStateMigrationName,
			CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
			Points:           []accountstore.CharacterPointRow{},
		}, nil
	case "character-myshop-unit-prices":
		var export accountstore.CharacterMyShopUnitPricesExport
		if err := decodeStrictJSON(exportRaw, &export); err != nil {
			return nil, err
		}
		_, summary, err := accountstore.QuarantineCharacterMyShopUnitPricesExport(export)
		if err != nil {
			return nil, err
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, fmt.Errorf("%w: wipe scope character_ids is empty", errInvalidSynthesizeWipeExportInput)
		}
		return accountstore.CharacterMyShopUnitPricesExport{
			MigrationVersion: accountstore.CharacterMyShopUnitPricesMigrationVersion,
			MigrationName:    accountstore.CharacterMyShopUnitPricesMigrationName,
			CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
			UnitPrices:       []accountstore.CharacterMyShopUnitPriceRow{},
		}, nil
	case "character-quest-state":
		var export queststate.CharacterQuestStateExport
		if err := decodeStrictJSON(exportRaw, &export); err != nil {
			return nil, err
		}
		_, summary, err := queststate.QuarantineCharacterQuestStateExport(export)
		if err != nil {
			return nil, err
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, fmt.Errorf("%w: wipe scope character_ids is empty", errInvalidSynthesizeWipeExportInput)
		}
		return queststate.CharacterQuestStateExport{
			MigrationVersion: queststate.CharacterQuestStateMigrationVersion,
			MigrationName:    queststate.CharacterQuestStateMigrationName,
			CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
			Flags:            []queststate.CharacterQuestFlagRow{},
		}, nil
	case "character-safebox-state":
		var export safeboxstore.CharacterSafeboxStateExport
		if err := decodeStrictJSON(exportRaw, &export); err != nil {
			return nil, err
		}
		_, summary, err := safeboxstore.QuarantineCharacterSafeboxStateExport(export)
		if err != nil {
			return nil, err
		}
		if len(summary.CharacterIDs) == 0 {
			return nil, fmt.Errorf("%w: wipe scope character_ids is empty", errInvalidSynthesizeWipeExportInput)
		}
		return safeboxstore.CharacterSafeboxStateExport{
			MigrationVersion: safeboxstore.CharacterSafeboxStateMigrationVersion,
			MigrationName:    safeboxstore.CharacterSafeboxStateMigrationName,
			CharacterIDs:     append([]uint32(nil), summary.CharacterIDs...),
			Passwords:        []safeboxstore.CharacterSafeboxPasswordRow{},
			Items:            []safeboxstore.CharacterSafeboxItemRow{},
		}, nil
	case "bootstrap-ground-item-state":
		var export worldruntime.BootstrapGroundItemStateExport
		if err := decodeStrictJSON(exportRaw, &export); err != nil {
			return nil, err
		}
		_, summary, err := worldruntime.QuarantineBootstrapGroundItemStateExport(export)
		if err != nil {
			return nil, err
		}
		if len(summary.VIDs) == 0 {
			return nil, fmt.Errorf("%w: wipe scope vids is empty", errInvalidSynthesizeWipeExportInput)
		}
		return worldruntime.BootstrapGroundItemStateExport{
			MigrationVersion: worldruntime.BootstrapGroundItemStateMigrationVersion,
			MigrationName:    worldruntime.BootstrapGroundItemStateMigrationName,
			VIDs:             append([]uint32(nil), summary.VIDs...),
			GroundItems:      []worldruntime.BootstrapGroundItemStateRow{},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported synthesize-wipe-export kind %q", kind)
	}
}

// normalizeImportExportJSON accepts either a bare migration-shaped export or a
// wrapped quarantine-export result {"summary":...,"export":...} and returns the
// bare export JSON bytes.
func normalizeImportExportJSON(raw []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: export is empty", errInvalidExportQuarantineInput)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &top); err != nil {
		return nil, fmt.Errorf("%w: decode export: %v", errInvalidExportQuarantineInput, err)
	}
	exportRaw, hasExport := top["export"]
	_, hasSummary := top["summary"]
	if hasExport && hasSummary && len(top) == 2 {
		if len(bytes.TrimSpace(exportRaw)) == 0 {
			return nil, fmt.Errorf("%w: wrapped quarantine export is empty", errInvalidExportQuarantineInput)
		}
		return exportRaw, nil
	}
	return trimmed, nil
}

func printSynthesizeWipeExportUsage(w io.Writer) {
	fmt.Fprintln(w, "synthesize-wipe-export usage:")
	fmt.Fprintln(w, "  metin2-migrate synthesize-wipe-export --kind <kind> --export <path|->")
	fmt.Fprintln(w, "kinds:")
	for _, kind := range importExportDrillWipeKinds {
		fmt.Fprintf(w, "  %s\n", kind)
	}
}
