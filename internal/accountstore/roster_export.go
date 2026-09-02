package accountstore

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

const (
	AccountCharacterRosterMigrationVersion = 2
	AccountCharacterRosterMigrationName    = "account_character_roster"
	accountCharacterRosterPlayerSlots      = 4
)

const maxSignedBigInt = uint64(1<<63 - 1)

// AccountCharacterRosterExport is a deterministic, schema-shaped projection of
// bootstrap JSON account snapshots onto the 0002_account_character_roster
// migration boundary. It is intentionally a data-model/export contract only: it
// does not open a database, emit SQL, apply migrations, or mutate the file store.
type AccountCharacterRosterExport struct {
	MigrationVersion int    `json:"migration_version"`
	MigrationName    string `json:"migration_name"`
	// AccountIDs optionally declares the replace/wipe account scope for
	// ImportAccountCharacterRoster(..., Replace: true). When omitted or empty,
	// quarantine derives the scope from account rows (legacy insert-only exports
	// stay valid). Explicit ids merge with account-row-derived ids so a listed
	// account with zero account/character rows can still wipe-to-empty.
	AccountIDs []int64                              `json:"account_ids,omitempty"`
	Accounts   []AccountCharacterRosterAccountRow   `json:"accounts"`
	Characters []AccountCharacterRosterCharacterRow `json:"characters"`
}

// AccountCharacterRosterAccountRow mirrors the durable accounts table columns
// frozen by the 0002_account_character_roster migration, excluding timestamps
// that are database-owned at insert time.
type AccountCharacterRosterAccountRow struct {
	ID              int64  `json:"id"`
	Login           string `json:"login"`
	LoginNormalized string `json:"login_normalized"`
	Empire          uint8  `json:"empire"`
}

// AccountCharacterRosterCharacterRow mirrors the durable characters table
// columns frozen by the 0002_account_character_roster migration, excluding
// timestamps that are database-owned at insert time. Inventory, equipment,
// quickslots, quest state, and other later persistence domains are deliberately
// omitted from this roster projection.
type AccountCharacterRosterCharacterRow struct {
	ID             uint32 `json:"id"`
	AccountID      int64  `json:"account_id"`
	Slot           int    `json:"slot"`
	Name           string `json:"name"`
	NameNormalized string `json:"name_normalized"`
	Job            uint8  `json:"job"`
	RaceNum        uint16 `json:"race_num"`
	Level          uint8  `json:"level"`
	PlayMinutes    uint32 `json:"play_minutes"`
	ST             uint8  `json:"st"`
	HT             uint8  `json:"ht"`
	DX             uint8  `json:"dx"`
	IQ             uint8  `json:"iq"`
	MainPart       uint16 `json:"main_part"`
	ChangeName     uint8  `json:"change_name"`
	HairPart       uint16 `json:"hair_part"`
	X              int32  `json:"x"`
	Y              int32  `json:"y"`
	Z              int32  `json:"z"`
	MapIndex       uint32 `json:"map_index"`
	Empire         uint8  `json:"empire"`
	SkillGroup     uint8  `json:"skill_group"`
	GuildID        uint32 `json:"guild_id"`
	GuildName      string `json:"guild_name"`
	Gold           uint64 `json:"gold"`
}

// ExportAccountCharacterRoster validates bootstrap account snapshots and returns
// rows ordered exactly as a future backfill/import tool should process them:
// accounts by normalized login, then non-empty characters by account row and
// select-screen slot. All validation fails closed against the 0002 migration
// constraints so malformed bootstrap JSON cannot be silently coerced into a
// future database import.
func ExportAccountCharacterRoster(accounts []Account) (AccountCharacterRosterExport, error) {
	ordered := append([]Account(nil), accounts...)
	sort.Slice(ordered, func(i, j int) bool {
		left := strings.ToLower(ordered[i].Login)
		right := strings.ToLower(ordered[j].Login)
		if left != right {
			return left < right
		}
		return ordered[i].Login < ordered[j].Login
	})

	export := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		Accounts:         make([]AccountCharacterRosterAccountRow, 0, len(ordered)),
		Characters:       []AccountCharacterRosterCharacterRow{},
	}
	seenAccountLogins := make(map[string]string, len(ordered))
	seenAccountIDs := make(map[int64]string, len(ordered))
	seenCharacterIDs := make(map[uint32]string)
	seenCharacterNames := make(map[string]uint32)

	for _, account := range ordered {
		if err := validateRosterExportAccount(account); err != nil {
			return AccountCharacterRosterExport{}, err
		}
		normalizedLogin := strings.ToLower(account.Login)
		if previous, ok := seenAccountLogins[normalizedLogin]; ok {
			return AccountCharacterRosterExport{}, fmt.Errorf("%w: account login %q duplicates %q", ErrInvalidAccount, account.Login, previous)
		}
		seenAccountLogins[normalizedLogin] = account.Login

		accountID := stableRosterAccountID(normalizedLogin, seenAccountIDs)
		seenAccountIDs[accountID] = account.Login
		export.Accounts = append(export.Accounts, AccountCharacterRosterAccountRow{
			ID:              accountID,
			Login:           account.Login,
			LoginNormalized: normalizedLogin,
			Empire:          account.Empire,
		})

		for slot, character := range account.Characters {
			if character.IsEmptySlot() {
				continue
			}
			if err := validateRosterExportCharacter(account, slot, character, seenCharacterIDs, seenCharacterNames); err != nil {
				return AccountCharacterRosterExport{}, err
			}
			seenCharacterIDs[character.ID] = character.Name
			seenCharacterNames[strings.ToLower(character.Name)] = character.ID
			export.Characters = append(export.Characters, AccountCharacterRosterCharacterRow{
				ID:             character.ID,
				AccountID:      accountID,
				Slot:           slot,
				Name:           character.Name,
				NameNormalized: strings.ToLower(character.Name),
				Job:            character.Job,
				RaceNum:        character.RaceNum,
				Level:          character.Level,
				PlayMinutes:    character.PlayMinutes,
				ST:             character.ST,
				HT:             character.HT,
				DX:             character.DX,
				IQ:             character.IQ,
				MainPart:       character.MainPart,
				ChangeName:     character.ChangeName,
				HairPart:       character.HairPart,
				X:              character.X,
				Y:              character.Y,
				Z:              character.Z,
				MapIndex:       character.MapIndex,
				Empire:         character.Empire,
				SkillGroup:     character.SkillGroup,
				GuildID:        character.GuildID,
				GuildName:      character.GuildName,
				Gold:           character.Gold,
			})
		}
	}

	return export, nil
}

// ExportAccountCharacterRoster validates and projects the committed file-store
// snapshots onto the 0002 account/character roster migration shape. It reads the
// same committed snapshot set as List and applies no mutations.
func (s *FileStore) ExportAccountCharacterRoster() (AccountCharacterRosterExport, error) {
	accounts, err := s.List()
	if err != nil {
		return AccountCharacterRosterExport{}, err
	}
	return ExportAccountCharacterRoster(accounts)
}

func validateRosterExportAccount(account Account) error {
	if strings.TrimSpace(account.Login) == "" {
		return ErrLoginRequired
	}
	if account.Login != strings.TrimSpace(account.Login) {
		return fmt.Errorf("%w: account login %q has leading or trailing whitespace", ErrInvalidAccount, account.Login)
	}
	if containsNUL(account.Login) {
		return fmt.Errorf("%w: account login contains NUL", ErrInvalidAccount)
	}
	if len(account.Characters) > accountCharacterRosterPlayerSlots {
		return fmt.Errorf("%w: account %q has %d character slots; migration roster supports %d", ErrInvalidAccount, account.Login, len(account.Characters), accountCharacterRosterPlayerSlots)
	}
	if err := validateAccount(account); err != nil {
		return err
	}
	if err := validateAccountUniqueInventorySlots(account); err != nil {
		return err
	}
	return nil
}

func validateRosterExportCharacter(account Account, slot int, character loginticket.Character, seenIDs map[uint32]string, seenNames map[string]uint32) error {
	if slot < 0 || slot >= accountCharacterRosterPlayerSlots {
		return fmt.Errorf("%w: account %q character slot %d outside migration roster", ErrInvalidAccount, account.Login, slot)
	}
	if character.ID == 0 {
		return fmt.Errorf("%w: account %q character slot %d has zero id", ErrInvalidAccount, account.Login, slot)
	}
	if strings.TrimSpace(character.Name) == "" {
		return fmt.Errorf("%w: character id %d has empty name", ErrInvalidAccount, character.ID)
	}
	if character.Name != strings.TrimSpace(character.Name) {
		return fmt.Errorf("%w: character name %q has leading or trailing whitespace", ErrInvalidAccount, character.Name)
	}
	if containsNUL(character.Name) || containsNUL(character.GuildName) {
		return fmt.Errorf("%w: character %q contains NUL text", ErrInvalidAccount, character.Name)
	}
	if character.Level == 0 {
		return fmt.Errorf("%w: character %q has level 0", ErrInvalidAccount, character.Name)
	}
	if character.MapIndex == 0 {
		return fmt.Errorf("%w: character %q has map_index 0", ErrInvalidAccount, character.Name)
	}
	if character.Gold > maxSignedBigInt {
		return fmt.Errorf("%w: character %q gold %d exceeds signed BIGINT", ErrInvalidAccount, character.Name, character.Gold)
	}
	if previous, ok := seenIDs[character.ID]; ok {
		return fmt.Errorf("%w: character id %d is used by %q and %q", ErrInvalidAccount, character.ID, previous, character.Name)
	}
	normalizedName := strings.ToLower(character.Name)
	if previousID, ok := seenNames[normalizedName]; ok {
		return fmt.Errorf("%w: character name %q is used by id %d and id %d", ErrInvalidAccount, character.Name, previousID, character.ID)
	}
	return nil
}

func stableRosterAccountID(normalizedLogin string, seen map[int64]string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(normalizedLogin))
	candidate := int64(h.Sum64() & maxSignedBigInt)
	if candidate == 0 {
		candidate = 1
	}
	for {
		if _, exists := seen[candidate]; !exists {
			return candidate
		}
		candidate++
		if candidate <= 0 {
			candidate = 1
		}
	}
}
