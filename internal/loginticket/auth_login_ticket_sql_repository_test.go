package loginticket

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestSQLRepositoryRejectsNilExecutor(t *testing.T) {
	ticket := sampleAuthLoginTicket()
	if err := (*SQLRepository)(nil).Issue(ticket); !errors.Is(err, ErrAuthLoginTicketHandoffImportExecutorRequired) {
		t.Fatalf("nil SQLRepository.Issue error = %v, want %v", err, ErrAuthLoginTicketHandoffImportExecutorRequired)
	}
	if err := NewSQLRepository(nil).Issue(ticket); !errors.Is(err, ErrAuthLoginTicketHandoffImportExecutorRequired) {
		t.Fatalf("NewSQLRepository(nil).Issue error = %v, want %v", err, ErrAuthLoginTicketHandoffImportExecutorRequired)
	}
	if _, err := NewSQLRepository(nil).Load("Alpha", ticket.LoginKey); !errors.Is(err, ErrAuthLoginTicketHandoffImportExecutorRequired) {
		t.Fatalf("NewSQLRepository(nil).Load error = %v, want %v", err, ErrAuthLoginTicketHandoffImportExecutorRequired)
	}
	if _, err := NewSQLRepository(nil).Consume("Alpha", ticket.LoginKey); !errors.Is(err, ErrAuthLoginTicketHandoffImportExecutorRequired) {
		t.Fatalf("NewSQLRepository(nil).Consume error = %v, want %v", err, ErrAuthLoginTicketHandoffImportExecutorRequired)
	}
	if _, err := NewSQLRepository(nil).ExportAuthLoginTicketHandoff(); !errors.Is(err, ErrAuthLoginTicketHandoffImportExecutorRequired) {
		t.Fatalf("NewSQLRepository(nil).Export error = %v, want %v", err, ErrAuthLoginTicketHandoffImportExecutorRequired)
	}
}

func TestSQLRepositoryLoadRejectsBlankLoginBeforeOpeningTransaction(t *testing.T) {
	_, err := NewSQLRepository(failingAuthLoginTicketHandoffImportExecutor{}).Load("  ", 0x01020304)
	if err == nil || !errors.Is(err, ErrInvalidTicket) || !strings.Contains(err.Error(), "login is required") {
		t.Fatalf("Load(blank) error = %v, want invalid ticket login required", err)
	}
}

func TestSQLRepositoryLoadRejectsPaddedOrNULLoginBeforeOpeningTransaction(t *testing.T) {
	store := NewSQLRepository(failingAuthLoginTicketHandoffImportExecutor{})
	_, err := store.Load(" Alpha", 0x01020304)
	if err == nil || !errors.Is(err, ErrInvalidTicket) || !strings.Contains(err.Error(), "leading or trailing whitespace") {
		t.Fatalf("Load(padded) error = %v, want invalid ticket whitespace", err)
	}
	_, err = store.Load("Alpha\x00", 0x01020304)
	if err == nil || !errors.Is(err, ErrInvalidTicket) || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("Load(NUL) error = %v, want invalid ticket NUL", err)
	}
	_, err = store.Consume("Alpha", 0)
	if err == nil || !errors.Is(err, ErrInvalidTicket) || !strings.Contains(err.Error(), "login key is required") {
		t.Fatalf("Consume(zero key) error = %v, want invalid ticket login key", err)
	}
}

func TestSQLRepositoryIssueRejectsInvalidTicketBeforeOpeningTransaction(t *testing.T) {
	err := NewSQLRepository(failingAuthLoginTicketHandoffImportExecutor{}).Issue(Ticket{
		Login:    "Alpha",
		LoginKey: 0,
		IssuedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC),
	})
	if err == nil || !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("Issue(zero key) error = %v, want %v", err, ErrInvalidTicket)
	}
}

func TestSQLRepositorySatisfiesStoreAndExporter(t *testing.T) {
	var store Store = NewSQLRepository(nil)
	var exporter AuthLoginTicketHandoffExporter = store.(AuthLoginTicketHandoffExporter)
	if exporter == nil {
		t.Fatal("SQLRepository must satisfy AuthLoginTicketHandoffExporter")
	}
}

func sampleAuthLoginTicket() Ticket {
	return Ticket{
		Login:    "Alpha",
		LoginKey: 0x01020304,
		Empire:   1,
		IssuedAt: time.Date(2026, 8, 12, 9, 30, 0, 123456789, time.UTC),
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
}
