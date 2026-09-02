package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestImportAccountCharacterRosterRejectsNilExecutor(t *testing.T) {
	export, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
			},
		},
	})
	if err != nil {
		t.Fatalf("export sample roster: %v", err)
	}

	_, err = ImportAccountCharacterRoster(context.Background(), nil, export)
	if !errors.Is(err, ErrAccountCharacterRosterImportExecutorRequired) {
		t.Fatalf("ImportAccountCharacterRoster(nil) error = %v, want %v", err, ErrAccountCharacterRosterImportExecutorRequired)
	}
}

func TestImportAccountCharacterRosterRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := AccountCharacterRosterExport{
		MigrationVersion: 99,
		MigrationName:    "not-roster",
		Accounts:         []AccountCharacterRosterAccountRow{},
		Characters:       []AccountCharacterRosterCharacterRow{},
	}

	_, err := ImportAccountCharacterRoster(context.Background(), failingRosterImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidAccountCharacterRosterExport) {
		t.Fatalf("ImportAccountCharacterRoster(invalid) error = %v, want %v", err, ErrInvalidAccountCharacterRosterExport)
	}
}

func TestImportAccountCharacterRosterRejectsTooManyOptions(t *testing.T) {
	export := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		Accounts:         []AccountCharacterRosterAccountRow{},
		Characters:       []AccountCharacterRosterCharacterRow{},
	}
	_, err := ImportAccountCharacterRoster(
		context.Background(),
		failingRosterImportExecutor{},
		export,
		ImportAccountCharacterRosterOptions{Replace: true},
		ImportAccountCharacterRosterOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportAccountCharacterRoster(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineAccountCharacterRosterExportMergesDeclaredAccountIDs(t *testing.T) {
	export := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		AccountIDs:       []int64{42},
		Accounts:         []AccountCharacterRosterAccountRow{},
		Characters:       []AccountCharacterRosterCharacterRow{},
	}
	canonical, summary, err := QuarantineAccountCharacterRosterExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.AccountCount != 1 || len(summary.AccountIDs) != 1 || summary.AccountIDs[0] != 42 {
		t.Fatalf("unexpected declared wipe summary: %#v", summary)
	}
	if len(canonical.AccountIDs) != 1 || canonical.AccountIDs[0] != 42 {
		t.Fatalf("unexpected canonical account_ids: %#v", canonical.AccountIDs)
	}
	if summary.CharacterCount != 0 || len(canonical.Accounts) != 0 || len(canonical.Characters) != 0 {
		t.Fatalf("declared wipe should keep zero roster rows: %#v %#v", summary, canonical)
	}
}

func TestQuarantineAccountCharacterRosterExportRejectsInvalidDeclaredAccountIDs(t *testing.T) {
	base := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		Accounts:         []AccountCharacterRosterAccountRow{},
		Characters:       []AccountCharacterRosterCharacterRow{},
	}

	zeroID := base
	zeroID.AccountIDs = []int64{0}
	if _, _, err := QuarantineAccountCharacterRosterExport(zeroID); err == nil || !errors.Is(err, ErrInvalidAccountCharacterRosterExport) {
		t.Fatalf("zero account_ids error = %v, want invalid export", err)
	}

	dupID := base
	dupID.AccountIDs = []int64{7, 7}
	if _, _, err := QuarantineAccountCharacterRosterExport(dupID); err == nil || !errors.Is(err, ErrInvalidAccountCharacterRosterExport) {
		t.Fatalf("duplicate account_ids error = %v, want invalid export", err)
	}
}

type failingRosterImportExecutor struct{}

func (failingRosterImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid roster exports")
}
