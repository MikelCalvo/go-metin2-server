package loginticket

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	AuthLoginTicketHandoffMigrationVersion = 7
	AuthLoginTicketHandoffMigrationName    = "auth_login_ticket_handoff"
)

// AuthLoginTicketHandoffExport is a deterministic, schema-shaped projection of
// committed bootstrap JSON login tickets onto the 0007_auth_login_ticket_handoff
// migration boundary. It is intentionally a data-model/export contract only: it
// does not open a database, emit SQL, apply migrations, mutate the ticket store,
// or consume one-shot tickets.
type AuthLoginTicketHandoffExport struct {
	MigrationVersion int                         `json:"migration_version"`
	MigrationName    string                      `json:"migration_name"`
	Tickets          []AuthLoginTicketHandoffRow `json:"tickets"`
}

// AuthLoginTicketHandoffRow mirrors the durable auth_login_tickets columns
// frozen by the 0007_auth_login_ticket_handoff migration, excluding timestamps
// that are database-owned at insert/update time. ConsumedAt is nil for the
// current file-backed pending-ticket store because committed JSON tickets are
// removed on consume rather than retained as historical rows.
type AuthLoginTicketHandoffRow struct {
	LoginKey               uint32     `json:"login_key"`
	IssuedAt               time.Time  `json:"issued_at"`
	Login                  string     `json:"login"`
	LoginNormalized        string     `json:"login_normalized"`
	Empire                 uint8      `json:"empire"`
	ConsumedAt             *time.Time `json:"consumed_at,omitempty"`
	CharactersSnapshotJSON string     `json:"characters_snapshot_json"`
}

// ExportAuthLoginTicketHandoff validates bootstrap login-ticket snapshots and
// returns rows ordered exactly as a future backfill/import tool should process
// them: normalized login, original login, then login key. All validation fails
// closed against the 0007 migration constraints so malformed bootstrap JSON
// cannot be silently coerced into a future database import.
func ExportAuthLoginTicketHandoff(tickets []Ticket) (AuthLoginTicketHandoffExport, error) {
	ordered := append([]Ticket(nil), tickets...)
	sort.Slice(ordered, func(i, j int) bool {
		left := strings.ToLower(ordered[i].Login)
		right := strings.ToLower(ordered[j].Login)
		if left != right {
			return left < right
		}
		if ordered[i].Login != ordered[j].Login {
			return ordered[i].Login < ordered[j].Login
		}
		return ordered[i].LoginKey < ordered[j].LoginKey
	})

	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          []AuthLoginTicketHandoffRow{},
	}
	seenActiveLoginKeys := make(map[uint32]string, len(ordered))
	for _, ticket := range ordered {
		ticket.Characters = CloneCharacters(ticket.Characters)
		normalizeCharactersItemState(ticket.Characters)
		if err := validateTicket(ticket); err != nil {
			return AuthLoginTicketHandoffExport{}, err
		}
		if previous, ok := seenActiveLoginKeys[ticket.LoginKey]; ok {
			return AuthLoginTicketHandoffExport{}, fmt.Errorf("%w: login key %08x is used by %q and %q", ErrInvalidTicket, ticket.LoginKey, previous, ticket.Login)
		}
		seenActiveLoginKeys[ticket.LoginKey] = ticket.Login

		characters := ticket.Characters
		if characters == nil {
			characters = []Character{}
		}
		charactersSnapshot, err := json.Marshal(characters)
		if err != nil {
			return AuthLoginTicketHandoffExport{}, fmt.Errorf("%w: encode characters snapshot for login key %08x: %v", ErrInvalidTicket, ticket.LoginKey, err)
		}
		if len(charactersSnapshot) == 0 || string(charactersSnapshot) == "null" {
			return AuthLoginTicketHandoffExport{}, fmt.Errorf("%w: empty characters snapshot for login key %08x", ErrInvalidTicket, ticket.LoginKey)
		}

		export.Tickets = append(export.Tickets, AuthLoginTicketHandoffRow{
			LoginKey:               ticket.LoginKey,
			IssuedAt:               ticket.IssuedAt.UTC(),
			Login:                  ticket.Login,
			LoginNormalized:        strings.ToLower(ticket.Login),
			Empire:                 ticket.Empire,
			CharactersSnapshotJSON: string(charactersSnapshot),
		})
	}
	return export, nil
}

// ExportAuthLoginTicketHandoff validates and projects the committed file-store
// tickets onto the 0007 auth-login-ticket handoff migration shape. It reads the
// same committed pending ticket set as List and applies no mutations.
func (s *FileStore) ExportAuthLoginTicketHandoff() (AuthLoginTicketHandoffExport, error) {
	tickets, err := s.List()
	if err != nil {
		return AuthLoginTicketHandoffExport{}, err
	}
	return ExportAuthLoginTicketHandoff(tickets)
}
