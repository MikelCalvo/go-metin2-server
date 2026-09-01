package accountstore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

const (
	CharacterMyShopUnitPricesMigrationVersion = 23
	CharacterMyShopUnitPricesMigrationName    = "character_myshop_unit_prices"
)

// CharacterMyShopUnitPricesExport is a deterministic, schema-shaped projection of
// durable silk-bag remembered unit prices onto the
// 0023_character_myshop_unit_prices migration boundary. It is intentionally a
// data-model/export contract only: it does not open a database, emit SQL, apply
// migrations, or mutate the file store.
type CharacterMyShopUnitPricesExport struct {
	MigrationVersion int    `json:"migration_version"`
	MigrationName    string `json:"migration_name"`
	// CharacterIDs optionally declares the replace/wipe character scope for
	// ImportCharacterMyShopUnitPrices(..., Replace: true). When omitted or empty,
	// quarantine derives the scope from unit-price rows (legacy insert-only
	// exports stay valid). Explicit ids merge with unit-price row-derived ids so
	// a listed character with zero price rows can still wipe-to-empty.
	CharacterIDs []uint32                      `json:"character_ids,omitempty"`
	UnitPrices   []CharacterMyShopUnitPriceRow `json:"unit_prices"`
}

// CharacterMyShopUnitPriceRow mirrors one remembered private-shop unit-price
// row frozen by the 0023_character_myshop_unit_prices migration.
type CharacterMyShopUnitPriceRow struct {
	CharacterID uint32 `json:"character_id"`
	Vnum        uint32 `json:"vnum"`
	UnitPrice   uint32 `json:"unit_price"`
}

// ExportCharacterMyShopUnitPrices validates bootstrap account snapshots and
// returns rows ordered exactly as a future backfill/import tool should process
// them: accounts by normalized login, characters by select-screen slot, then
// unit-price rows by ascending vnum. Characters with an empty remembered map
// emit no rows. All validation fails closed against the 0002 roster and FileStore
// myshop_unit_prices bounds so malformed bootstrap JSON cannot be silently
// coerced into a future database import.
func ExportCharacterMyShopUnitPrices(accounts []Account) (CharacterMyShopUnitPricesExport, error) {
	if _, err := ExportAccountCharacterRoster(accounts); err != nil {
		return CharacterMyShopUnitPricesExport{}, err
	}

	ordered := append([]Account(nil), accounts...)
	sort.Slice(ordered, func(i, j int) bool {
		left := strings.ToLower(ordered[i].Login)
		right := strings.ToLower(ordered[j].Login)
		if left != right {
			return left < right
		}
		return ordered[i].Login < ordered[j].Login
	})

	export := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	}

	for _, account := range ordered {
		for slot, character := range account.Characters {
			if character.IsEmptySlot() {
				continue
			}
			if slot < 0 || slot >= accountCharacterRosterPlayerSlots {
				return CharacterMyShopUnitPricesExport{}, fmt.Errorf("%w: account %q character slot %d outside myshop-unit-prices roster", ErrInvalidAccount, account.Login, slot)
			}
			if err := validateRosterExportCharacter(account, slot, character, map[uint32]string{}, map[string]uint32{}); err != nil {
				return CharacterMyShopUnitPricesExport{}, err
			}
			if err := validateCharacterMyShopUnitPrices(character); err != nil {
				return CharacterMyShopUnitPricesExport{}, err
			}
			for _, row := range loginticket.CanonicalMyShopUnitPrices(character.MyShopUnitPrices) {
				export.UnitPrices = append(export.UnitPrices, CharacterMyShopUnitPriceRow{
					CharacterID: character.ID,
					Vnum:        row.Vnum,
					UnitPrice:   row.UnitPrice,
				})
			}
		}
	}

	return export, nil
}

// ExportCharacterMyShopUnitPrices validates and projects the committed file-store
// snapshots onto the 0023 character myshop unit-prices migration shape. It reads
// the same committed snapshot set as List and applies no mutations.
func (s *FileStore) ExportCharacterMyShopUnitPrices() (CharacterMyShopUnitPricesExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterMyShopUnitPricesExport{}, err
	}
	return ExportCharacterMyShopUnitPrices(accounts)
}
