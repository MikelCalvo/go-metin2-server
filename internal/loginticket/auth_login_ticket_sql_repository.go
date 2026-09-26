package loginticket

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// SQLRepository is the named auth login-ticket repository beside FileStore
// issue/load/consume and the tip-0007 export. A caller supplies the
// database/sql executor. This type does not select a driver, load a DSN,
// embed secrets, register a production engine, or mount authd/gamed.
//
// Issue inserts one active (consumed_at IS NULL) tip-0007 row. Load reads
// that active row. Consume returns the same ticket and leaves the SQL row
// untouched: destructive one-shot removal stays on FileStore. Replacing that
// delete with a consumed_at update is explicitly deferred.
//
// Export identity stays tip-0007. Insert-only import and opt-in scoped replace
// remain ImportAuthLoginTicketHandoff.
type SQLRepository struct {
	executor dbmigrations.SQLMigrationExecutor
}

// NewSQLRepository returns an opt-in SQL login-ticket repository. A nil
// executor fails closed on Issue/Load/Consume/Export rather than at
// construction.
func NewSQLRepository(executor dbmigrations.SQLMigrationExecutor) *SQLRepository {
	return &SQLRepository{executor: executor}
}

// Issue inserts one active auth_login_tickets row for a validated pending
// ticket. A second active row for the same login key fails as ErrTicketExists.
// IssuedAt is filled when the caller omits it, matching FileStore.
func (s *SQLRepository) Issue(ticket Ticket) error {
	if ticket.IssuedAt.IsZero() {
		ticket.IssuedAt = time.Now().UTC()
	}
	ticket.Characters = CloneCharacters(ticket.Characters)
	normalizeCharactersItemState(ticket.Characters)
	if err := validateTicket(ticket); err != nil {
		return err
	}
	if s == nil || authLoginTicketHandoffImportExecutorIsNil(s.executor) {
		return ErrAuthLoginTicketHandoffImportExecutorRequired
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin auth login-ticket SQL issue transaction: %w", err)
	}
	if err := requireAuthLoginTicketHandoffSchema(ctx, tx); err != nil {
		return rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}
	active, err := countActiveAuthLoginTickets(ctx, tx, ticket.LoginKey)
	if err != nil {
		return rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}
	if active != 0 {
		return rollbackAfterAuthLoginTicketHandoffImportFailure(tx, ErrTicketExists)
	}
	row, err := authLoginTicketHandoffRowFromTicket(ticket)
	if err != nil {
		return rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}
	if err := insertAuthLoginTicketHandoff(ctx, tx, row); err != nil {
		return rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit auth login-ticket SQL issue transaction: %w", err)
	}
	return nil
}

// Load returns the active tip-0007 ticket for login + login key. Consumed
// historical rows are not a pending handoff. A login that does not match the
// stored row is ErrTicketLoginMismatch, matching FileStore.
func (s *SQLRepository) Load(login string, loginKey uint32) (Ticket, error) {
	return s.readActive(login, loginKey)
}

// Consume returns the active tip-0007 ticket and does not update or delete
// the SQL row. FileStore.Consume remains the destructive one-shot delete.
func (s *SQLRepository) Consume(login string, loginKey uint32) (Ticket, error) {
	return s.readActive(login, loginKey)
}

// ExportAuthLoginTicketHandoff projects active SQL rows onto the tip-0007
// export. Consumed historical rows are omitted, matching the pending FileStore
// set. An empty active set is an empty export.
func (s *SQLRepository) ExportAuthLoginTicketHandoff() (AuthLoginTicketHandoffExport, error) {
	if s == nil || authLoginTicketHandoffImportExecutorIsNil(s.executor) {
		return AuthLoginTicketHandoffExport{}, ErrAuthLoginTicketHandoffImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return AuthLoginTicketHandoffExport{}, fmt.Errorf("begin auth login-ticket SQL export transaction: %w", err)
	}
	if err := requireAuthLoginTicketHandoffSchema(ctx, tx); err != nil {
		return AuthLoginTicketHandoffExport{}, rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}
	tickets, err := loadActiveAuthLoginTickets(ctx, tx)
	if err != nil {
		return AuthLoginTicketHandoffExport{}, rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return AuthLoginTicketHandoffExport{}, fmt.Errorf("commit auth login-ticket SQL export transaction: %w", err)
	}
	return ExportAuthLoginTicketHandoff(tickets)
}

func (s *SQLRepository) readActive(login string, loginKey uint32) (Ticket, error) {
	if strings.TrimSpace(login) == "" {
		return Ticket{}, fmt.Errorf("%w: login is required", ErrInvalidTicket)
	}
	if login != strings.TrimSpace(login) {
		return Ticket{}, fmt.Errorf("%w: login %q has leading or trailing whitespace", ErrInvalidTicket, login)
	}
	if containsNUL(login) {
		return Ticket{}, fmt.Errorf("%w: login contains NUL", ErrInvalidTicket)
	}
	if loginKey == 0 {
		return Ticket{}, fmt.Errorf("%w: login key is required", ErrInvalidTicket)
	}
	if s == nil || authLoginTicketHandoffImportExecutorIsNil(s.executor) {
		return Ticket{}, ErrAuthLoginTicketHandoffImportExecutorRequired
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return Ticket{}, fmt.Errorf("begin auth login-ticket SQL load transaction: %w", err)
	}
	if err := requireAuthLoginTicketHandoffSchema(ctx, tx); err != nil {
		return Ticket{}, rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}
	ticket, found, err := loadActiveAuthLoginTicket(ctx, tx, loginKey)
	if err != nil {
		return Ticket{}, rollbackAfterAuthLoginTicketHandoffImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return Ticket{}, fmt.Errorf("commit auth login-ticket SQL load transaction: %w", err)
	}
	if !found {
		return Ticket{}, ErrTicketNotFound
	}
	if ticket.Login != login || ticket.LoginKey != loginKey {
		return Ticket{}, ErrTicketLoginMismatch
	}
	return ticket, nil
}

func authLoginTicketHandoffRowFromTicket(ticket Ticket) (AuthLoginTicketHandoffRow, error) {
	export, err := ExportAuthLoginTicketHandoff([]Ticket{ticket})
	if err != nil {
		return AuthLoginTicketHandoffRow{}, err
	}
	if len(export.Tickets) != 1 {
		return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: auth login-ticket export produced %d rows", ErrInvalidTicket, len(export.Tickets))
	}
	return export.Tickets[0], nil
}

func countActiveAuthLoginTickets(ctx context.Context, tx *sql.Tx, loginKey uint32) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM auth_login_tickets
WHERE login_key = ? AND consumed_at IS NULL`, int64(loginKey)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active auth login tickets login_key=%08x: %w", loginKey, err)
	}
	return count, nil
}

func loadActiveAuthLoginTicket(ctx context.Context, tx *sql.Tx, loginKey uint32) (Ticket, bool, error) {
	var (
		gotLoginKey               int64
		gotIssuedAt               string
		gotLogin                  string
		gotEmpire                 int64
		gotCharactersSnapshotJSON string
	)
	err := tx.QueryRowContext(ctx, `
SELECT login_key, issued_at, login, empire, characters_snapshot_json
FROM auth_login_tickets
WHERE login_key = ? AND consumed_at IS NULL`, int64(loginKey)).Scan(
		&gotLoginKey, &gotIssuedAt, &gotLogin, &gotEmpire, &gotCharactersSnapshotJSON,
	)
	if err == sql.ErrNoRows {
		return Ticket{}, false, nil
	}
	if err != nil {
		return Ticket{}, false, fmt.Errorf("load active auth login ticket login_key=%08x: %w", loginKey, err)
	}
	ticket, err := ticketFromAuthLoginTicketColumns(gotLoginKey, gotIssuedAt, gotLogin, gotEmpire, gotCharactersSnapshotJSON)
	if err != nil {
		return Ticket{}, false, err
	}
	return ticket, true, nil
}

func loadActiveAuthLoginTickets(ctx context.Context, tx *sql.Tx) ([]Ticket, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT login_key, issued_at, login, empire, characters_snapshot_json
FROM auth_login_tickets
WHERE consumed_at IS NULL
ORDER BY login_normalized ASC, login ASC, login_key ASC`)
	if err != nil {
		return nil, fmt.Errorf("query active auth login tickets: %w", err)
	}
	defer rows.Close()

	tickets := make([]Ticket, 0)
	for rows.Next() {
		var (
			gotLoginKey               int64
			gotIssuedAt               string
			gotLogin                  string
			gotEmpire                 int64
			gotCharactersSnapshotJSON string
		)
		if err := rows.Scan(&gotLoginKey, &gotIssuedAt, &gotLogin, &gotEmpire, &gotCharactersSnapshotJSON); err != nil {
			return nil, fmt.Errorf("scan active auth login ticket: %w", err)
		}
		ticket, err := ticketFromAuthLoginTicketColumns(gotLoginKey, gotIssuedAt, gotLogin, gotEmpire, gotCharactersSnapshotJSON)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active auth login tickets: %w", err)
	}
	return tickets, nil
}

func ticketFromAuthLoginTicketColumns(loginKey int64, issuedAt, login string, empire int64, charactersSnapshotJSON string) (Ticket, error) {
	if loginKey <= 0 || loginKey > int64(^uint32(0)) {
		return Ticket{}, fmt.Errorf("%w: auth login ticket login_key %d is outside uint32", ErrInvalidTicket, loginKey)
	}
	if empire < 0 || empire > 255 {
		return Ticket{}, fmt.Errorf("%w: auth login ticket empire %d is outside uint8", ErrInvalidTicket, empire)
	}
	parsedIssuedAt, err := time.Parse(time.RFC3339Nano, issuedAt)
	if err != nil {
		return Ticket{}, fmt.Errorf("%w: auth login ticket issued_at %q: %v", ErrInvalidTicket, issuedAt, err)
	}
	var characters []Character
	if err := json.Unmarshal([]byte(charactersSnapshotJSON), &characters); err != nil {
		return Ticket{}, fmt.Errorf("%w: decode characters snapshot for login key %08x: %v", ErrInvalidTicket, uint32(loginKey), err)
	}
	ticket := Ticket{
		Login:      login,
		LoginKey:   uint32(loginKey),
		Empire:     uint8(empire),
		IssuedAt:   parsedIssuedAt.UTC(),
		Characters: CloneCharacters(characters),
	}
	normalizeCharactersItemState(ticket.Characters)
	if err := validateTicket(ticket); err != nil {
		return Ticket{}, err
	}
	return ticket, nil
}
