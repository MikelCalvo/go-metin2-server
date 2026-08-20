package loginticket

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestMemoryStoreIssueLoadConsumeWithoutFilesystem(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore()
	issuedAt := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	seed := Ticket{
		Login:    "Alpha",
		LoginKey: 0x01020304,
		Empire:   1,
		IssuedAt: issuedAt,
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

	if err := store.Issue(seed); err != nil {
		t.Fatalf("issue memory ticket: %v", err)
	}
	if err := store.Issue(seed); !errors.Is(err, ErrTicketExists) {
		t.Fatalf("expected ErrTicketExists on duplicate issue, got %v", err)
	}

	loaded, err := store.Load("Alpha", 0x01020304)
	if err != nil {
		t.Fatalf("load memory ticket: %v", err)
	}
	if loaded.Login != "Alpha" || loaded.LoginKey != 0x01020304 || loaded.Empire != 1 || !loaded.IssuedAt.Equal(issuedAt) {
		t.Fatalf("unexpected loaded ticket envelope: %#v", loaded)
	}
	if len(loaded.Characters) != 1 || loaded.Characters[0].Name != "AlphaWar" || len(loaded.Characters[0].Inventory) != 1 {
		t.Fatalf("unexpected loaded characters: %#v", loaded.Characters)
	}
	loaded.Characters[0].Inventory[0].Count = 99
	reloaded, err := store.Load("Alpha", 0x01020304)
	if err != nil {
		t.Fatalf("reload memory ticket: %v", err)
	}
	if reloaded.Characters[0].Inventory[0].Count != 2 {
		t.Fatalf("memory store leaked caller mutation: %#v", reloaded.Characters[0].Inventory[0])
	}

	if _, err := store.Load("Bravo", 0x01020304); !errors.Is(err, ErrTicketLoginMismatch) {
		t.Fatalf("expected ErrTicketLoginMismatch, got %v", err)
	}

	consumed, err := store.Consume("Alpha", 0x01020304)
	if err != nil {
		t.Fatalf("consume memory ticket: %v", err)
	}
	if consumed.Login != "Alpha" || consumed.LoginKey != 0x01020304 {
		t.Fatalf("unexpected consumed ticket: %#v", consumed)
	}
	if _, err := store.Load("Alpha", 0x01020304); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound after consume, got %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("memory store wrote filesystem entries: %#v", entries)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".ticket-*.json")); err != nil || len(matches) != 0 {
		t.Fatalf("memory store created crash-temp shaped files: matches=%v err=%v", matches, err)
	}
}

func TestMemoryStoreRejectsInvalidIssue(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Issue(Ticket{Login: "Alpha", LoginKey: 0}); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for zero login key, got %v", err)
	}
	tickets, err := store.List()
	if err != nil {
		t.Fatalf("list after invalid issue: %v", err)
	}
	if len(tickets) != 0 {
		t.Fatalf("invalid issue must leave store empty, got %#v", tickets)
	}
}

func TestMemoryStoreExportsMatchFileStoreAndPassQuarantine(t *testing.T) {
	issuedAlpha := time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC)
	issuedZeta := time.Date(2026, 8, 20, 10, 45, 0, 0, time.UTC)
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
		}},
	}
	zeta := Ticket{Login: "zeta", LoginKey: 0x02000000, Empire: 3, IssuedAt: issuedZeta}

	fileStore := NewFileStore(t.TempDir())
	memoryStore := NewMemoryStore()
	for _, ticket := range []Ticket{zeta, alpha} {
		if err := fileStore.Issue(ticket); err != nil {
			t.Fatalf("issue file ticket: %v", err)
		}
		if err := memoryStore.Issue(ticket); err != nil {
			t.Fatalf("issue memory ticket: %v", err)
		}
	}

	fileExport, err := fileStore.ExportAuthLoginTicketHandoff()
	if err != nil {
		t.Fatalf("file auth login-ticket handoff export: %v", err)
	}
	memoryExport, err := memoryStore.ExportAuthLoginTicketHandoff()
	if err != nil {
		t.Fatalf("memory auth login-ticket handoff export: %v", err)
	}
	if !reflect.DeepEqual(fileExport, memoryExport) {
		t.Fatalf("auth login-ticket handoff export mismatch:\n file: %#v\nmemory: %#v", fileExport, memoryExport)
	}
	if _, err := ValidateAuthLoginTicketHandoffExport(memoryExport); err != nil {
		t.Fatalf("quarantine memory auth login-ticket handoff export: %v", err)
	}
}

func TestMemoryStoreExportTreatsEmptyStoreAsEmpty(t *testing.T) {
	store := NewMemoryStore()
	export, err := store.ExportAuthLoginTicketHandoff()
	if err != nil {
		t.Fatalf("export empty memory store: %v", err)
	}
	if export.MigrationVersion != AuthLoginTicketHandoffMigrationVersion || export.MigrationName != AuthLoginTicketHandoffMigrationName || len(export.Tickets) != 0 {
		t.Fatalf("expected empty auth login-ticket handoff export, got %#v", export)
	}
}

func TestMemoryStoreSatisfiesAuthLoginTicketHandoffExporter(t *testing.T) {
	var exporter AuthLoginTicketHandoffExporter = NewMemoryStore()
	if _, err := exporter.ExportAuthLoginTicketHandoff(); err != nil {
		t.Fatalf("empty auth login-ticket handoff export: %v", err)
	}
}
