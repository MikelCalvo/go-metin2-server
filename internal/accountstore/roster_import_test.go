package accountstore

import (
	"context"
	"database/sql"
	"errors"
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

type failingRosterImportExecutor struct{}

func (failingRosterImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid roster exports")
}
