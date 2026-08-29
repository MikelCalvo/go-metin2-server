package migratecli

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func runImportExport(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("import-export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var kind string
	var exportPath string
	var driverName string
	var dsn string
	var confirmImport bool
	flags.StringVar(&kind, "kind", "", "migration-shaped export kind to import")
	flags.StringVar(&exportPath, "export", "", "path to retained export JSON, or - for stdin")
	flags.StringVar(&driverName, "driver", "", "database/sql driver name for the import target")
	flags.StringVar(&dsn, "dsn", "", "database/sql DSN for the import target")
	flags.BoolVar(&confirmImport, "i-confirm-sql-import", false, "confirm CLI SQL import/backfill mutation against the supplied driver/DSN")
	flags.Usage = func() { printImportExportUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected import-export argument %q\n", flags.Arg(0))
		printImportExportUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(kind) == "" || strings.TrimSpace(exportPath) == "" || strings.TrimSpace(driverName) == "" || strings.TrimSpace(dsn) == "" {
		fmt.Fprintln(stderr, "--kind, --export, --driver, and --dsn are required for import-export")
		printImportExportUsage(stderr)
		return exitUsage
	}
	if !isSupportedExportQuarantineKind(kind) {
		fmt.Fprintf(stderr, "unsupported import-export kind %q\n", kind)
		printImportExportUsage(stderr)
		return exitUsage
	}
	if !confirmImport {
		fmt.Fprintln(stderr, "--i-confirm-sql-import is required for import-export")
		printImportExportUsage(stderr)
		return exitUsage
	}

	reader, closeReader, err := openExportQuarantineReader(exportPath, stdin)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "import-export: %v", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	raw, err := readBoundedExportQuarantine(reader)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "import-export: %v", err)
		return exitError
	}

	decoded, err := decodeImportExportPayload(kind, raw)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "import-export: %v", err)
		return exitError
	}
	if err := quarantineImportExportPayload(kind, decoded); err != nil {
		writeMigrationCommandError(stderr, dsn, "import-export: %v", err)
		return exitError
	}

	db, err := sql.Open(strings.TrimSpace(driverName), strings.TrimSpace(dsn))
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "import-export: open database driver %q: %v", strings.TrimSpace(driverName), err)
		return exitError
	}
	defer db.Close()

	result, err := importExportPayload(context.Background(), db, kind, decoded)
	if err != nil {
		writeMigrationCommandError(stderr, dsn, "import-export: %v", err)
		return exitError
	}
	return writeJSON(stdout, stderr, result)
}

func decodeImportExportPayload(kind string, raw []byte) (any, error) {
	switch kind {
	case "account-character-roster":
		var export accountstore.AccountCharacterRosterExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "character-item-state":
		var export accountstore.CharacterItemStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "character-point-state":
		var export accountstore.CharacterPointStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "character-myshop-unit-prices":
		var export accountstore.CharacterMyShopUnitPricesExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "character-quest-state":
		var export queststate.CharacterQuestStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "character-safebox-state":
		var export safeboxstore.CharacterSafeboxStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "auth-login-ticket-handoff":
		var export loginticket.AuthLoginTicketHandoffExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "item-template-state":
		var export itemstore.ItemTemplateStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "static-actor-content-state":
		var export staticstore.StaticActorContentStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	case "bootstrap-ground-item-state":
		var export worldruntime.BootstrapGroundItemStateExport
		if err := decodeStrictJSON(raw, &export); err != nil {
			return nil, err
		}
		return export, nil
	default:
		return nil, fmt.Errorf("unsupported import-export kind %q", kind)
	}
}

func quarantineImportExportPayload(kind string, payload any) error {
	switch kind {
	case "account-character-roster":
		_, _, err := accountstore.QuarantineAccountCharacterRosterExport(payload.(accountstore.AccountCharacterRosterExport))
		return err
	case "character-item-state":
		_, _, err := accountstore.QuarantineCharacterItemStateExport(payload.(accountstore.CharacterItemStateExport))
		return err
	case "character-point-state":
		_, _, err := accountstore.QuarantineCharacterPointStateExport(payload.(accountstore.CharacterPointStateExport))
		return err
	case "character-myshop-unit-prices":
		_, _, err := accountstore.QuarantineCharacterMyShopUnitPricesExport(payload.(accountstore.CharacterMyShopUnitPricesExport))
		return err
	case "character-quest-state":
		_, _, err := queststate.QuarantineCharacterQuestStateExport(payload.(queststate.CharacterQuestStateExport))
		return err
	case "character-safebox-state":
		_, _, err := safeboxstore.QuarantineCharacterSafeboxStateExport(payload.(safeboxstore.CharacterSafeboxStateExport))
		return err
	case "auth-login-ticket-handoff":
		_, _, err := loginticket.QuarantineAuthLoginTicketHandoffExport(payload.(loginticket.AuthLoginTicketHandoffExport))
		return err
	case "item-template-state":
		_, _, err := itemstore.QuarantineItemTemplateStateExport(payload.(itemstore.ItemTemplateStateExport))
		return err
	case "static-actor-content-state":
		_, _, err := staticstore.QuarantineStaticActorContentStateExport(payload.(staticstore.StaticActorContentStateExport))
		return err
	case "bootstrap-ground-item-state":
		_, _, err := worldruntime.QuarantineBootstrapGroundItemStateExport(payload.(worldruntime.BootstrapGroundItemStateExport))
		return err
	default:
		return fmt.Errorf("unsupported import-export kind %q", kind)
	}
}

func importExportPayload(ctx context.Context, db *sql.DB, kind string, payload any) (any, error) {
	switch kind {
	case "account-character-roster":
		return accountstore.ImportAccountCharacterRoster(ctx, db, payload.(accountstore.AccountCharacterRosterExport))
	case "character-item-state":
		return accountstore.ImportCharacterItemState(ctx, db, payload.(accountstore.CharacterItemStateExport))
	case "character-point-state":
		return accountstore.ImportCharacterPointState(ctx, db, payload.(accountstore.CharacterPointStateExport))
	case "character-myshop-unit-prices":
		return accountstore.ImportCharacterMyShopUnitPrices(ctx, db, payload.(accountstore.CharacterMyShopUnitPricesExport))
	case "character-quest-state":
		return queststate.ImportCharacterQuestState(ctx, db, payload.(queststate.CharacterQuestStateExport))
	case "character-safebox-state":
		return safeboxstore.ImportCharacterSafeboxState(ctx, db, payload.(safeboxstore.CharacterSafeboxStateExport))
	case "auth-login-ticket-handoff":
		return loginticket.ImportAuthLoginTicketHandoff(ctx, db, payload.(loginticket.AuthLoginTicketHandoffExport))
	case "item-template-state":
		return itemstore.ImportItemTemplateState(ctx, db, payload.(itemstore.ItemTemplateStateExport))
	case "static-actor-content-state":
		return staticstore.ImportStaticActorContentState(ctx, db, payload.(staticstore.StaticActorContentStateExport))
	case "bootstrap-ground-item-state":
		return worldruntime.ImportBootstrapGroundItemState(ctx, db, payload.(worldruntime.BootstrapGroundItemStateExport))
	default:
		return nil, fmt.Errorf("unsupported import-export kind %q", kind)
	}
}

func printImportExportUsage(w io.Writer) {
	fmt.Fprintln(w, "import-export usage:")
	fmt.Fprintln(w, "  metin2-migrate import-export --kind <kind> --export <path|-> --driver <database/sql-driver> --dsn <dsn> --i-confirm-sql-import")
	fmt.Fprintln(w, "kinds:")
	for _, kind := range exportQuarantineKinds {
		fmt.Fprintf(w, "  %s\n", kind)
	}
}
