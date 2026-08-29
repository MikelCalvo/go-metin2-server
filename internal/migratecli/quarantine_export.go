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
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const maxExportQuarantineBytes = 1 << 20

var errInvalidExportQuarantineInput = errors.New("invalid export quarantine input")

var exportQuarantineKinds = []string{
	"account-character-roster",
	"character-item-state",
	"character-point-state",
	"character-myshop-unit-prices",
	"character-quest-state",
	"character-safebox-state",
	"auth-login-ticket-handoff",
	"item-template-state",
	"static-actor-content-state",
	"bootstrap-ground-item-state",
}

func runQuarantineExport(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("quarantine-export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var kind string
	var exportPath string
	flags.StringVar(&kind, "kind", "", "migration-shaped export kind to quarantine")
	flags.StringVar(&exportPath, "export", "", "path to retained export JSON, or - for stdin")
	flags.Usage = func() { printQuarantineExportUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected quarantine-export argument %q\n", flags.Arg(0))
		printQuarantineExportUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(kind) == "" || strings.TrimSpace(exportPath) == "" {
		fmt.Fprintln(stderr, "--kind and --export are required for quarantine-export")
		printQuarantineExportUsage(stderr)
		return exitUsage
	}
	if !isSupportedExportQuarantineKind(kind) {
		fmt.Fprintf(stderr, "unsupported quarantine-export kind %q\n", kind)
		printQuarantineExportUsage(stderr)
		return exitUsage
	}

	reader, closeReader, err := openExportQuarantineReader(exportPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "quarantine-export: %v\n", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	raw, err := readBoundedExportQuarantine(reader)
	if err != nil {
		fmt.Fprintf(stderr, "quarantine-export: %v\n", err)
		return exitError
	}

	result, err := quarantineExportJSON(kind, raw)
	if err != nil {
		fmt.Fprintf(stderr, "quarantine-export: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, result)
}

func isSupportedExportQuarantineKind(kind string) bool {
	for _, supported := range exportQuarantineKinds {
		if kind == supported {
			return true
		}
	}
	return false
}

func openExportQuarantineReader(path string, stdin io.Reader) (io.Reader, func(), error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "-" {
		return stdin, nil, nil
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: stat export: %v", errInvalidExportQuarantineInput, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("%w: export must not be a symlink: %s", errInvalidExportQuarantineInput, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%w: export must be a regular file: %s", errInvalidExportQuarantineInput, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: open export: %v", errInvalidExportQuarantineInput, err)
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%w: stat opened export: %v", errInvalidExportQuarantineInput, err)
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%w: opened export must be a regular file: %s", errInvalidExportQuarantineInput, trimmed)
	}
	return file, func() { _ = file.Close() }, nil
}

func readBoundedExportQuarantine(reader io.Reader) ([]byte, error) {
	if reader == nil {
		reader = strings.NewReader("")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxExportQuarantineBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read export: %v", errInvalidExportQuarantineInput, err)
	}
	if len(raw) > maxExportQuarantineBytes {
		return nil, fmt.Errorf("%w: export exceeds %d bytes", errInvalidExportQuarantineInput, maxExportQuarantineBytes)
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("%w: export is not valid UTF-8", errInvalidExportQuarantineInput)
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%w: export is empty", errInvalidExportQuarantineInput)
	}
	return raw, nil
}

func decodeStrictJSON(raw []byte, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("%w: decode export: %v", errInvalidExportQuarantineInput, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%w: export has trailing JSON", errInvalidExportQuarantineInput)
	}
	return nil
}

func quarantineExportJSON(kind string, raw []byte) (any, error) {
	switch kind {
	case "account-character-roster":
		var export accountstore.AccountCharacterRosterExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := accountstore.QuarantineAccountCharacterRosterExport(export)
		if err != nil {
			return nil, err
		}
		return accountstore.AccountCharacterRosterQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "character-item-state":
		var export accountstore.CharacterItemStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := accountstore.QuarantineCharacterItemStateExport(export)
		if err != nil {
			return nil, err
		}
		return accountstore.CharacterItemStateQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "character-point-state":
		var export accountstore.CharacterPointStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := accountstore.QuarantineCharacterPointStateExport(export)
		if err != nil {
			return nil, err
		}
		return accountstore.CharacterPointStateQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "character-myshop-unit-prices":
		var export accountstore.CharacterMyShopUnitPricesExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := accountstore.QuarantineCharacterMyShopUnitPricesExport(export)
		if err != nil {
			return nil, err
		}
		return accountstore.CharacterMyShopUnitPricesQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "character-quest-state":
		var export queststate.CharacterQuestStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := queststate.QuarantineCharacterQuestStateExport(export)
		if err != nil {
			return nil, err
		}
		return queststate.CharacterQuestStateQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "character-safebox-state":
		var export safeboxstore.CharacterSafeboxStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := safeboxstore.QuarantineCharacterSafeboxStateExport(export)
		if err != nil {
			return nil, err
		}
		return safeboxstore.CharacterSafeboxStateQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "auth-login-ticket-handoff":
		var export loginticket.AuthLoginTicketHandoffExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := loginticket.QuarantineAuthLoginTicketHandoffExport(export)
		if err != nil {
			return nil, err
		}
		return loginticket.AuthLoginTicketHandoffQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "item-template-state":
		var export itemstore.ItemTemplateStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := itemstore.QuarantineItemTemplateStateExport(export)
		if err != nil {
			return nil, err
		}
		return itemstore.ItemTemplateStateQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "static-actor-content-state":
		var export staticstore.StaticActorContentStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := staticstore.QuarantineStaticActorContentStateExport(export)
		if err != nil {
			return nil, err
		}
		return staticstore.StaticActorContentStateQuarantineResult{Summary: summary, Export: quarantined}, nil
	case "bootstrap-ground-item-state":
		var export worldruntime.BootstrapGroundItemStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		quarantined, summary, err := worldruntime.QuarantineBootstrapGroundItemStateExport(export)
		if err != nil {
			return nil, err
		}
		return worldruntime.BootstrapGroundItemStateQuarantineResult{Summary: summary, Export: quarantined}, nil
	default:
		return nil, fmt.Errorf("unsupported quarantine-export kind %q", kind)
	}
}

func printQuarantineExportUsage(w io.Writer) {
	fmt.Fprintln(w, "quarantine-export usage:")
	fmt.Fprintln(w, "  metin2-migrate quarantine-export --kind <kind> --export <path|->")
	fmt.Fprintln(w, "kinds:")
	for _, kind := range exportQuarantineKinds {
		fmt.Fprintf(w, "  %s\n", kind)
	}
}
