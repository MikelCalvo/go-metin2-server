package loginticket

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestImportAuthLoginTicketHandoffRejectsNilExecutor(t *testing.T) {
	export, err := ExportAuthLoginTicketHandoff([]Ticket{
		{
			Login:    "Alpha",
			LoginKey: 0x01020304,
			Empire:   1,
			IssuedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC),
			Characters: []Character{{
				ID:       7,
				Name:     "AlphaWar",
				Level:    1,
				MapIndex: 1,
				Inventory: []inventory.ItemInstance{
					{ID: 1001, Vnum: 27001, Count: 2, Slot: 8},
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("export sample auth login-ticket handoff: %v", err)
	}

	_, err = ImportAuthLoginTicketHandoff(context.Background(), nil, export)
	if !errors.Is(err, ErrAuthLoginTicketHandoffImportExecutorRequired) {
		t.Fatalf("ImportAuthLoginTicketHandoff(nil) error = %v, want %v", err, ErrAuthLoginTicketHandoffImportExecutorRequired)
	}
}

func TestImportAuthLoginTicketHandoffRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: 99,
		MigrationName:    "not-auth-login-ticket-handoff",
		Tickets:          []AuthLoginTicketHandoffRow{},
	}

	_, err := ImportAuthLoginTicketHandoff(context.Background(), failingAuthLoginTicketHandoffImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidAuthLoginTicketHandoffExport) {
		t.Fatalf("ImportAuthLoginTicketHandoff(invalid) error = %v, want %v", err, ErrInvalidAuthLoginTicketHandoffExport)
	}
}

func TestImportAuthLoginTicketHandoffRejectsNilTicketsBeforeOpeningTransaction(t *testing.T) {
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          nil,
	}

	_, err := ImportAuthLoginTicketHandoff(context.Background(), failingAuthLoginTicketHandoffImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidAuthLoginTicketHandoffExport) {
		t.Fatalf("ImportAuthLoginTicketHandoff(nil tickets) error = %v, want %v", err, ErrInvalidAuthLoginTicketHandoffExport)
	}
}

type failingAuthLoginTicketHandoffImportExecutor struct{}

func (failingAuthLoginTicketHandoffImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid auth login-ticket handoff exports")
}
