package loginticket

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

func TestImportAuthLoginTicketHandoffRejectsTooManyOptions(t *testing.T) {
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          []AuthLoginTicketHandoffRow{},
	}
	_, err := ImportAuthLoginTicketHandoff(
		context.Background(),
		failingAuthLoginTicketHandoffImportExecutor{},
		export,
		ImportAuthLoginTicketHandoffOptions{Replace: true},
		ImportAuthLoginTicketHandoffOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportAuthLoginTicketHandoff(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineAuthLoginTicketHandoffExportMergesDeclaredLoginKeys(t *testing.T) {
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		LoginKeys:        []uint32{0x01020304},
		Tickets:          []AuthLoginTicketHandoffRow{},
	}
	canonical, summary, err := QuarantineAuthLoginTicketHandoffExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.TicketCount != 0 || summary.ActiveTicketCount != 0 {
		t.Fatalf("declared wipe should keep zero ticket counts: %#v", summary)
	}
	if len(summary.LoginKeys) != 1 || summary.LoginKeys[0] != 0x01020304 {
		t.Fatalf("unexpected declared wipe summary login keys: %#v", summary)
	}
	if len(canonical.LoginKeys) != 1 || canonical.LoginKeys[0] != 0x01020304 {
		t.Fatalf("unexpected canonical login_keys: %#v", canonical.LoginKeys)
	}
}

func TestQuarantineAuthLoginTicketHandoffExportRejectsInvalidDeclaredLoginKeys(t *testing.T) {
	base := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          []AuthLoginTicketHandoffRow{},
	}

	zeroKey := base
	zeroKey.LoginKeys = []uint32{0}
	if _, _, err := QuarantineAuthLoginTicketHandoffExport(zeroKey); err == nil || !errors.Is(err, ErrInvalidAuthLoginTicketHandoffExport) {
		t.Fatalf("zero login_keys error = %v, want invalid export", err)
	}

	dupKey := base
	dupKey.LoginKeys = []uint32{0x01020304, 0x01020304}
	if _, _, err := QuarantineAuthLoginTicketHandoffExport(dupKey); err == nil || !errors.Is(err, ErrInvalidAuthLoginTicketHandoffExport) {
		t.Fatalf("duplicate login_keys error = %v, want invalid export", err)
	}
}

type failingAuthLoginTicketHandoffImportExecutor struct{}

func (failingAuthLoginTicketHandoffImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid auth login-ticket handoff exports")
}
