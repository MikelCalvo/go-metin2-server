package loginticket

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestExportAuthLoginTicketHandoffBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	issuedAlpha := time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)
	issuedZeta := time.Date(2026, 8, 12, 10, 45, 0, 0, time.UTC)
	alpha := Ticket{
		Login:    "Alpha",
		LoginKey: 0x01020304,
		Empire:   1,
		IssuedAt: issuedAlpha,
		Characters: []Character{{
			ID:       7,
			Name:     "AlphaWar",
			Level:    1,
			MapIndex: 1,
			Inventory: []inventory.ItemInstance{
				{ID: 1001, Vnum: 27001, Count: 2, Slot: 8},
			},
		}},
	}
	zeta := Ticket{Login: "zeta", LoginKey: 0x02000000, Empire: 3, IssuedAt: issuedZeta}

	export, err := ExportAuthLoginTicketHandoff([]Ticket{zeta, alpha})
	if err != nil {
		t.Fatalf("export auth login-ticket handoff: %v", err)
	}
	if export.MigrationVersion != AuthLoginTicketHandoffMigrationVersion || export.MigrationName != AuthLoginTicketHandoffMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	if len(export.Tickets) != 2 {
		t.Fatalf("expected two ticket rows, got %#v", export.Tickets)
	}
	first := export.Tickets[0]
	if first.Login != "Alpha" || first.LoginNormalized != "alpha" || first.LoginKey != 0x01020304 || first.Empire != 1 || !first.IssuedAt.Equal(issuedAlpha) || first.ConsumedAt != nil {
		t.Fatalf("unexpected first ticket row: %#v", first)
	}
	if strings.TrimSpace(first.CharactersSnapshotJSON) == "" || !json.Valid([]byte(first.CharactersSnapshotJSON)) {
		t.Fatalf("expected valid non-empty characters snapshot JSON, got %q", first.CharactersSnapshotJSON)
	}
	if strings.Contains(first.CharactersSnapshotJSON, "login_key") || strings.Contains(first.CharactersSnapshotJSON, "issued_at") {
		t.Fatalf("characters snapshot JSON must not contain ticket envelope fields, got %s", first.CharactersSnapshotJSON)
	}
	if !strings.Contains(first.CharactersSnapshotJSON, `"name":"AlphaWar"`) || !strings.Contains(first.CharactersSnapshotJSON, `"inventory":[`) || !strings.Contains(first.CharactersSnapshotJSON, `"equipment":[]`) || !strings.Contains(first.CharactersSnapshotJSON, `"quickslots":[]`) {
		t.Fatalf("characters snapshot JSON did not preserve normalized character payload: %s", first.CharactersSnapshotJSON)
	}
	if export.Tickets[1].Login != "zeta" || export.Tickets[1].LoginKey != 0x02000000 || export.Tickets[1].CharactersSnapshotJSON != "[]" {
		t.Fatalf("unexpected second ticket row: %#v", export.Tickets[1])
	}

	exportAgain, err := ExportAuthLoginTicketHandoff([]Ticket{alpha, zeta})
	if err != nil {
		t.Fatalf("export auth login-ticket handoff again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic export independent of input order:\n got: %#v\nwant: %#v", exportAgain, export)
	}
}

func TestExportAuthLoginTicketHandoffRejectsRowsThatCannotTargetMigrationSchema(t *testing.T) {
	valid := Ticket{Login: "Alpha", LoginKey: 0x01020304, IssuedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)}
	for _, tc := range []struct {
		name    string
		tickets []Ticket
	}{
		{name: "zero login key", tickets: []Ticket{{Login: "Alpha", LoginKey: 0, IssuedAt: valid.IssuedAt}}},
		{name: "blank login", tickets: []Ticket{{Login: " ", LoginKey: 0x01020304, IssuedAt: valid.IssuedAt}}},
		{name: "zero issued_at", tickets: []Ticket{{Login: "Alpha", LoginKey: 0x01020304}}},
		{name: "duplicate active login key", tickets: []Ticket{valid, {Login: "Bravo", LoginKey: valid.LoginKey, IssuedAt: valid.IssuedAt.Add(time.Minute)}}},
		{name: "invalid character payload", tickets: []Ticket{{Login: "Alpha", LoginKey: 0x01020304, IssuedAt: valid.IssuedAt, Characters: []Character{{Name: "Ghost"}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExportAuthLoginTicketHandoff(tc.tickets)
			if !errors.Is(err, ErrInvalidTicket) {
				t.Fatalf("expected ErrInvalidTicket, got %v", err)
			}
		})
	}
}

func TestFileStoreExportAuthLoginTicketHandoffReadsCommittedTickets(t *testing.T) {
	store := NewFileStore(t.TempDir())
	issuedAt := time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)
	if err := store.Issue(Ticket{Login: "Alpha", LoginKey: 0x01020304, Empire: 1, IssuedAt: issuedAt, Characters: []Character{{ID: 7, Name: "AlphaWar", Level: 1, MapIndex: 1}}}); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}

	export, err := store.ExportAuthLoginTicketHandoff()
	if err != nil {
		t.Fatalf("file-store auth login-ticket handoff export: %v", err)
	}
	if export.MigrationVersion != AuthLoginTicketHandoffMigrationVersion || export.MigrationName != AuthLoginTicketHandoffMigrationName {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.Tickets) != 1 || export.Tickets[0].Login != "Alpha" || export.Tickets[0].LoginKey != 0x01020304 || export.Tickets[0].LoginNormalized != "alpha" {
		t.Fatalf("unexpected ticket export rows: %#v", export.Tickets)
	}
}

func TestFileStoreExportAuthLoginTicketHandoffTreatsMissingStoreAsEmptyExport(t *testing.T) {
	store := NewFileStore(t.TempDir() + "/missing")

	export, err := store.ExportAuthLoginTicketHandoff()
	if err != nil {
		t.Fatalf("export missing ticket store: %v", err)
	}
	if export.MigrationVersion != AuthLoginTicketHandoffMigrationVersion || len(export.Tickets) != 0 {
		t.Fatalf("expected empty auth login-ticket handoff export for missing store, got %#v", export)
	}
}
