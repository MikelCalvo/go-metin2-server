package accountstore

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidAccountCharacterRosterExport reports that a retained roster export
// failed the 0002 migration-shaped quarantine contract.
var ErrInvalidAccountCharacterRosterExport = errors.New("invalid account character roster export")

// AccountCharacterRosterQuarantineSummary is the metadata-only result of
// validating or quarantining a retained account/character roster export. It
// never includes account passwords, SQL, DSNs, or account snapshot bytes.
type AccountCharacterRosterQuarantineSummary struct {
	AccountCount   int      `json:"account_count"`
	CharacterCount int      `json:"character_count"`
	AccountIDs     []int64  `json:"account_ids"`
	CharacterIDs   []uint32 `json:"character_ids"`
}

// AccountCharacterRosterQuarantineResult pairs the metadata-only quarantine
// summary with a canonicalized export ready for later offline review or
// backfill tools.
type AccountCharacterRosterQuarantineResult struct {
	Summary AccountCharacterRosterQuarantineSummary `json:"summary"`
	Export  AccountCharacterRosterExport            `json:"export"`
}

// ValidateAccountCharacterRosterExport fails closed when a retained export does
// not match the 0002_account_character_roster shape. It does not open a
// database, write account snapshots, or mutate the supplied export.
func ValidateAccountCharacterRosterExport(export AccountCharacterRosterExport) (AccountCharacterRosterQuarantineSummary, error) {
	canonical, summary, err := canonicalizeAccountCharacterRosterExport(export)
	if err != nil {
		return AccountCharacterRosterQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineAccountCharacterRosterExport validates a retained export and
// returns a canonicalized copy ordered by normalized login / account id and
// account/slot character keys. Declared account_ids merge with account-row-
// derived ids so a listed account with zero account/character rows can
// wipe-to-empty under scoped replace. It never opens a database or mutates
// account snapshots.
func QuarantineAccountCharacterRosterExport(export AccountCharacterRosterExport) (AccountCharacterRosterExport, AccountCharacterRosterQuarantineSummary, error) {
	return canonicalizeAccountCharacterRosterExport(export)
}

func canonicalizeAccountCharacterRosterExport(export AccountCharacterRosterExport) (AccountCharacterRosterExport, AccountCharacterRosterQuarantineSummary, error) {
	if export.MigrationVersion != AccountCharacterRosterMigrationVersion {
		return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidAccountCharacterRosterExport, export.MigrationVersion)
	}
	if export.MigrationName != AccountCharacterRosterMigrationName {
		return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidAccountCharacterRosterExport, export.MigrationName)
	}
	if export.Accounts == nil {
		return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: accounts must be present", ErrInvalidAccountCharacterRosterExport)
	}
	if export.Characters == nil {
		return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: characters must be present", ErrInvalidAccountCharacterRosterExport)
	}

	accountIDs := make(map[int64]struct{}, len(export.Accounts)+len(export.AccountIDs))
	if export.AccountIDs != nil {
		seenDeclaredIDs := make(map[int64]struct{}, len(export.AccountIDs))
		for _, accountID := range export.AccountIDs {
			if accountID <= 0 {
				return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: account_ids entries must be > 0", ErrInvalidAccountCharacterRosterExport)
			}
			if _, exists := seenDeclaredIDs[accountID]; exists {
				return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: duplicate account_ids entry %d", ErrInvalidAccountCharacterRosterExport, accountID)
			}
			seenDeclaredIDs[accountID] = struct{}{}
			accountIDs[accountID] = struct{}{}
		}
	}

	seenAccountIDs := make(map[int64]string, len(export.Accounts))
	seenLoginNormalized := make(map[string]int64, len(export.Accounts))
	accounts := make([]AccountCharacterRosterAccountRow, 0, len(export.Accounts))
	for _, row := range export.Accounts {
		if err := validateQuarantineRosterAccountRow(row); err != nil {
			return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, err
		}
		if previous, ok := seenAccountIDs[row.ID]; ok {
			return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: account id %d is used by %q and %q", ErrInvalidAccountCharacterRosterExport, row.ID, previous, row.Login)
		}
		seenAccountIDs[row.ID] = row.Login
		accountIDs[row.ID] = struct{}{}
		if previousID, ok := seenLoginNormalized[row.LoginNormalized]; ok {
			return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: login_normalized %q is used by account id %d and id %d", ErrInvalidAccountCharacterRosterExport, row.LoginNormalized, previousID, row.ID)
		}
		seenLoginNormalized[row.LoginNormalized] = row.ID
		accounts = append(accounts, row)
	}

	seenCharacterIDs := make(map[uint32]string, len(export.Characters))
	seenCharacterNames := make(map[string]uint32, len(export.Characters))
	seenAccountSlots := make(map[string]struct{}, len(export.Characters))
	characters := make([]AccountCharacterRosterCharacterRow, 0, len(export.Characters))
	for _, row := range export.Characters {
		if err := validateQuarantineRosterCharacterRow(row, seenAccountIDs); err != nil {
			return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, err
		}
		slotKey := fmt.Sprintf("%d:%d", row.AccountID, row.Slot)
		if _, exists := seenAccountSlots[slotKey]; exists {
			return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: duplicate account_id=%d slot %d", ErrInvalidAccountCharacterRosterExport, row.AccountID, row.Slot)
		}
		seenAccountSlots[slotKey] = struct{}{}
		if previous, ok := seenCharacterIDs[row.ID]; ok {
			return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: character id %d is used by %q and %q", ErrInvalidAccountCharacterRosterExport, row.ID, previous, row.Name)
		}
		seenCharacterIDs[row.ID] = row.Name
		if previousID, ok := seenCharacterNames[row.NameNormalized]; ok {
			return AccountCharacterRosterExport{}, AccountCharacterRosterQuarantineSummary{}, fmt.Errorf("%w: name_normalized %q is used by character id %d and id %d", ErrInvalidAccountCharacterRosterExport, row.NameNormalized, previousID, row.ID)
		}
		seenCharacterNames[row.NameNormalized] = row.ID
		characters = append(characters, row)
	}

	sort.SliceStable(accounts, func(i, j int) bool {
		if accounts[i].LoginNormalized != accounts[j].LoginNormalized {
			return accounts[i].LoginNormalized < accounts[j].LoginNormalized
		}
		if accounts[i].Login != accounts[j].Login {
			return accounts[i].Login < accounts[j].Login
		}
		return accounts[i].ID < accounts[j].ID
	})
	sort.SliceStable(characters, func(i, j int) bool {
		if characters[i].AccountID != characters[j].AccountID {
			return characters[i].AccountID < characters[j].AccountID
		}
		if characters[i].Slot != characters[j].Slot {
			return characters[i].Slot < characters[j].Slot
		}
		return characters[i].ID < characters[j].ID
	})

	sortedAccountIDs := make([]int64, 0, len(accountIDs))
	for accountID := range accountIDs {
		sortedAccountIDs = append(sortedAccountIDs, accountID)
	}
	sort.Slice(sortedAccountIDs, func(i, j int) bool { return sortedAccountIDs[i] < sortedAccountIDs[j] })
	characterIDs := make([]uint32, 0, len(seenCharacterIDs))
	for characterID := range seenCharacterIDs {
		characterIDs = append(characterIDs, characterID)
	}
	sort.Slice(characterIDs, func(i, j int) bool { return characterIDs[i] < characterIDs[j] })

	canonical := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		AccountIDs:       append([]int64(nil), sortedAccountIDs...),
		Accounts:         accounts,
		Characters:       characters,
	}
	if canonical.AccountIDs == nil {
		canonical.AccountIDs = []int64{}
	}
	summary := AccountCharacterRosterQuarantineSummary{
		AccountCount:   len(sortedAccountIDs),
		CharacterCount: len(characters),
		AccountIDs:     sortedAccountIDs,
		CharacterIDs:   characterIDs,
	}
	if summary.AccountIDs == nil {
		summary.AccountIDs = []int64{}
	}
	if summary.CharacterIDs == nil {
		summary.CharacterIDs = []uint32{}
	}
	return canonical, summary, nil
}

func validateQuarantineRosterAccountRow(row AccountCharacterRosterAccountRow) error {
	if row.ID <= 0 {
		return fmt.Errorf("%w: account id must be > 0", ErrInvalidAccountCharacterRosterExport)
	}
	if strings.TrimSpace(row.Login) == "" {
		return fmt.Errorf("%w: account id %d has empty login", ErrInvalidAccountCharacterRosterExport, row.ID)
	}
	if row.Login != strings.TrimSpace(row.Login) {
		return fmt.Errorf("%w: account login %q has leading or trailing whitespace", ErrInvalidAccountCharacterRosterExport, row.Login)
	}
	if containsNUL(row.Login) {
		return fmt.Errorf("%w: account login contains NUL", ErrInvalidAccountCharacterRosterExport)
	}
	if row.LoginNormalized == "" {
		return fmt.Errorf("%w: account id %d has empty login_normalized", ErrInvalidAccountCharacterRosterExport, row.ID)
	}
	if row.LoginNormalized != strings.ToLower(row.Login) {
		return fmt.Errorf("%w: account id %d login_normalized %q does not match login %q", ErrInvalidAccountCharacterRosterExport, row.ID, row.LoginNormalized, row.Login)
	}
	return nil
}

func validateQuarantineRosterCharacterRow(row AccountCharacterRosterCharacterRow, accountIDs map[int64]string) error {
	if row.ID == 0 {
		return fmt.Errorf("%w: character id must be > 0", ErrInvalidAccountCharacterRosterExport)
	}
	if _, ok := accountIDs[row.AccountID]; !ok {
		return fmt.Errorf("%w: character id %d references unknown account_id %d", ErrInvalidAccountCharacterRosterExport, row.ID, row.AccountID)
	}
	if row.Slot < 0 || row.Slot >= accountCharacterRosterPlayerSlots {
		return fmt.Errorf("%w: character id %d slot %d outside migration roster", ErrInvalidAccountCharacterRosterExport, row.ID, row.Slot)
	}
	if strings.TrimSpace(row.Name) == "" {
		return fmt.Errorf("%w: character id %d has empty name", ErrInvalidAccountCharacterRosterExport, row.ID)
	}
	if row.Name != strings.TrimSpace(row.Name) {
		return fmt.Errorf("%w: character name %q has leading or trailing whitespace", ErrInvalidAccountCharacterRosterExport, row.Name)
	}
	if containsNUL(row.Name) || containsNUL(row.GuildName) {
		return fmt.Errorf("%w: character %q contains NUL text", ErrInvalidAccountCharacterRosterExport, row.Name)
	}
	if row.NameNormalized == "" {
		return fmt.Errorf("%w: character id %d has empty name_normalized", ErrInvalidAccountCharacterRosterExport, row.ID)
	}
	if row.NameNormalized != strings.ToLower(row.Name) {
		return fmt.Errorf("%w: character id %d name_normalized %q does not match name %q", ErrInvalidAccountCharacterRosterExport, row.ID, row.NameNormalized, row.Name)
	}
	if row.Level == 0 {
		return fmt.Errorf("%w: character %q has level 0", ErrInvalidAccountCharacterRosterExport, row.Name)
	}
	if row.MapIndex == 0 {
		return fmt.Errorf("%w: character %q has map_index 0", ErrInvalidAccountCharacterRosterExport, row.Name)
	}
	if row.Gold > maxSignedBigInt {
		return fmt.Errorf("%w: character %q gold %d exceeds signed BIGINT", ErrInvalidAccountCharacterRosterExport, row.Name, row.Gold)
	}
	return nil
}
