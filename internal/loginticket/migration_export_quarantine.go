package loginticket

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalidAuthLoginTicketHandoffExport reports that a retained auth
// login-ticket handoff export failed the 0007 migration-shaped quarantine
// contract.
var ErrInvalidAuthLoginTicketHandoffExport = errors.New("invalid auth login-ticket handoff export")

// AuthLoginTicketHandoffQuarantineSummary is the metadata-only result of
// validating or quarantining a retained auth login-ticket handoff export. It
// never includes ticket secrets, SQL, DSNs, or login-ticket snapshot bytes.
type AuthLoginTicketHandoffQuarantineSummary struct {
	TicketCount       int      `json:"ticket_count"`
	ActiveTicketCount int      `json:"active_ticket_count"`
	LoginKeys         []uint32 `json:"login_keys"`
}

// AuthLoginTicketHandoffQuarantineResult pairs the metadata-only quarantine
// summary with a canonicalized export ready for later offline review or
// backfill tools.
type AuthLoginTicketHandoffQuarantineResult struct {
	Summary AuthLoginTicketHandoffQuarantineSummary `json:"summary"`
	Export  AuthLoginTicketHandoffExport            `json:"export"`
}

// ValidateAuthLoginTicketHandoffExport fails closed when a retained export does
// not match the 0007_auth_login_ticket_handoff shape. It does not open a
// database, write login-ticket snapshots, consume tickets, or mutate the
// supplied export.
func ValidateAuthLoginTicketHandoffExport(export AuthLoginTicketHandoffExport) (AuthLoginTicketHandoffQuarantineSummary, error) {
	canonical, summary, err := canonicalizeAuthLoginTicketHandoffExport(export)
	if err != nil {
		return AuthLoginTicketHandoffQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineAuthLoginTicketHandoffExport validates a retained export and
// returns a canonicalized copy ordered by normalized login, original login,
// then login key. It never opens a database, writes tickets, or consumes them.
func QuarantineAuthLoginTicketHandoffExport(export AuthLoginTicketHandoffExport) (AuthLoginTicketHandoffExport, AuthLoginTicketHandoffQuarantineSummary, error) {
	return canonicalizeAuthLoginTicketHandoffExport(export)
}

func canonicalizeAuthLoginTicketHandoffExport(export AuthLoginTicketHandoffExport) (AuthLoginTicketHandoffExport, AuthLoginTicketHandoffQuarantineSummary, error) {
	if export.MigrationVersion != AuthLoginTicketHandoffMigrationVersion {
		return AuthLoginTicketHandoffExport{}, AuthLoginTicketHandoffQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidAuthLoginTicketHandoffExport, export.MigrationVersion)
	}
	if export.MigrationName != AuthLoginTicketHandoffMigrationName {
		return AuthLoginTicketHandoffExport{}, AuthLoginTicketHandoffQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidAuthLoginTicketHandoffExport, export.MigrationName)
	}
	if export.Tickets == nil {
		return AuthLoginTicketHandoffExport{}, AuthLoginTicketHandoffQuarantineSummary{}, fmt.Errorf("%w: tickets must be present", ErrInvalidAuthLoginTicketHandoffExport)
	}

	seenPrimaryKeys := make(map[string]struct{}, len(export.Tickets))
	seenActiveLoginKeys := make(map[uint32]string, len(export.Tickets))
	loginKeys := make(map[uint32]struct{}, len(export.Tickets))
	tickets := make([]AuthLoginTicketHandoffRow, 0, len(export.Tickets))
	activeTicketCount := 0

	for _, row := range export.Tickets {
		canonicalRow, err := canonicalizeAuthLoginTicketHandoffRow(row)
		if err != nil {
			return AuthLoginTicketHandoffExport{}, AuthLoginTicketHandoffQuarantineSummary{}, err
		}

		primaryKey := fmt.Sprintf("%d\x00%s", canonicalRow.LoginKey, canonicalRow.IssuedAt.UTC().Format(time.RFC3339Nano))
		if _, exists := seenPrimaryKeys[primaryKey]; exists {
			return AuthLoginTicketHandoffExport{}, AuthLoginTicketHandoffQuarantineSummary{}, fmt.Errorf("%w: duplicate login_key=%08x issued_at=%s", ErrInvalidAuthLoginTicketHandoffExport, canonicalRow.LoginKey, canonicalRow.IssuedAt.UTC().Format(time.RFC3339Nano))
		}
		seenPrimaryKeys[primaryKey] = struct{}{}

		if canonicalRow.ConsumedAt == nil {
			if previous, ok := seenActiveLoginKeys[canonicalRow.LoginKey]; ok {
				return AuthLoginTicketHandoffExport{}, AuthLoginTicketHandoffQuarantineSummary{}, fmt.Errorf("%w: active login key %08x is used by %q and %q", ErrInvalidAuthLoginTicketHandoffExport, canonicalRow.LoginKey, previous, canonicalRow.Login)
			}
			seenActiveLoginKeys[canonicalRow.LoginKey] = canonicalRow.Login
			activeTicketCount++
		}

		loginKeys[canonicalRow.LoginKey] = struct{}{}
		tickets = append(tickets, canonicalRow)
	}

	sort.SliceStable(tickets, func(i, j int) bool {
		if tickets[i].LoginNormalized != tickets[j].LoginNormalized {
			return tickets[i].LoginNormalized < tickets[j].LoginNormalized
		}
		if tickets[i].Login != tickets[j].Login {
			return tickets[i].Login < tickets[j].Login
		}
		return tickets[i].LoginKey < tickets[j].LoginKey
	})

	sortedLoginKeys := make([]uint32, 0, len(loginKeys))
	for loginKey := range loginKeys {
		sortedLoginKeys = append(sortedLoginKeys, loginKey)
	}
	sort.Slice(sortedLoginKeys, func(i, j int) bool { return sortedLoginKeys[i] < sortedLoginKeys[j] })

	canonical := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          tickets,
	}
	summary := AuthLoginTicketHandoffQuarantineSummary{
		TicketCount:       len(canonical.Tickets),
		ActiveTicketCount: activeTicketCount,
		LoginKeys:         sortedLoginKeys,
	}
	if summary.LoginKeys == nil {
		summary.LoginKeys = []uint32{}
	}
	return canonical, summary, nil
}

func canonicalizeAuthLoginTicketHandoffRow(row AuthLoginTicketHandoffRow) (AuthLoginTicketHandoffRow, error) {
	login := strings.TrimSpace(row.Login)
	if row.LoginKey == 0 {
		return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: login_key must be > 0", ErrInvalidAuthLoginTicketHandoffExport)
	}
	if row.IssuedAt.IsZero() {
		return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: issued_at is required", ErrInvalidAuthLoginTicketHandoffExport)
	}
	if login == "" {
		return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: login is required", ErrInvalidAuthLoginTicketHandoffExport)
	}
	if row.Login != login {
		return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: login %q has leading or trailing whitespace", ErrInvalidAuthLoginTicketHandoffExport, row.Login)
	}
	if containsNUL(login) {
		return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: login contains NUL", ErrInvalidAuthLoginTicketHandoffExport)
	}
	if strings.TrimSpace(row.LoginNormalized) == "" {
		return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: login_normalized is required", ErrInvalidAuthLoginTicketHandoffExport)
	}
	if row.LoginNormalized != strings.ToLower(login) {
		return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: login_normalized %q does not match login %q", ErrInvalidAuthLoginTicketHandoffExport, row.LoginNormalized, login)
	}
	if row.ConsumedAt != nil {
		if row.ConsumedAt.IsZero() {
			return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: consumed_at must be non-zero when present", ErrInvalidAuthLoginTicketHandoffExport)
		}
		if row.ConsumedAt.Before(row.IssuedAt) {
			return AuthLoginTicketHandoffRow{}, fmt.Errorf("%w: consumed_at %s is before issued_at %s", ErrInvalidAuthLoginTicketHandoffExport, row.ConsumedAt.UTC().Format(time.RFC3339Nano), row.IssuedAt.UTC().Format(time.RFC3339Nano))
		}
	}
	if err := validateAuthLoginTicketCharactersSnapshotJSON(row.CharactersSnapshotJSON); err != nil {
		return AuthLoginTicketHandoffRow{}, err
	}

	canonical := AuthLoginTicketHandoffRow{
		LoginKey:               row.LoginKey,
		IssuedAt:               row.IssuedAt.UTC(),
		Login:                  login,
		LoginNormalized:        row.LoginNormalized,
		Empire:                 row.Empire,
		CharactersSnapshotJSON: row.CharactersSnapshotJSON,
	}
	if row.ConsumedAt != nil {
		consumedAt := row.ConsumedAt.UTC()
		canonical.ConsumedAt = &consumedAt
	}
	return canonical, nil
}

func validateAuthLoginTicketCharactersSnapshotJSON(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("%w: characters_snapshot_json is required", ErrInvalidAuthLoginTicketHandoffExport)
	}
	if !utf8.ValidString(raw) {
		return fmt.Errorf("%w: characters_snapshot_json is not valid UTF-8", ErrInvalidAuthLoginTicketHandoffExport)
	}
	if !json.Valid([]byte(raw)) || strings.TrimSpace(raw) == "null" {
		return fmt.Errorf("%w: characters_snapshot_json must be a non-null JSON array", ErrInvalidAuthLoginTicketHandoffExport)
	}

	var characters []Character
	if err := json.Unmarshal([]byte(raw), &characters); err != nil {
		return fmt.Errorf("%w: decode characters_snapshot_json: %v", ErrInvalidAuthLoginTicketHandoffExport, err)
	}
	normalizeCharactersItemState(characters)
	if err := validateUniqueCharacterIdentity(characters); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidAuthLoginTicketHandoffExport, err)
	}
	for _, character := range characters {
		if err := validateCharacterItemPayloads(character); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidAuthLoginTicketHandoffExport, err)
		}
		if err := validateCharacterUniqueItemInstanceIDs(character); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidAuthLoginTicketHandoffExport, err)
		}
		if err := validateCharacterUniqueInventorySlots(character); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidAuthLoginTicketHandoffExport, err)
		}
		if err := validateCharacterUniqueEquipmentSlots(character); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidAuthLoginTicketHandoffExport, err)
		}
		if err := validateCharacterQuickslots(character); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidAuthLoginTicketHandoffExport, err)
		}
	}
	return nil
}
