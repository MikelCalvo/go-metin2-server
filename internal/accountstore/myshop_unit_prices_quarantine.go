package accountstore

import (
	"errors"
	"fmt"
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

// ErrInvalidCharacterMyShopUnitPricesExport reports that a retained myshop
// unit-prices export failed the 0023 migration-shaped quarantine contract.
var ErrInvalidCharacterMyShopUnitPricesExport = errors.New("invalid character myshop unit-prices export")

// CharacterMyShopUnitPricesQuarantineSummary is the metadata-only result of
// validating or quarantining a retained character myshop unit-prices export. It
// never includes unit prices, SQL, DSNs, or account snapshot bytes.
type CharacterMyShopUnitPricesQuarantineSummary struct {
	CharacterCount int      `json:"character_count"`
	PriceRowCount  int      `json:"price_row_count"`
	CharacterIDs   []uint32 `json:"character_ids"`
}

// CharacterMyShopUnitPricesQuarantineResult pairs the metadata-only quarantine
// summary with a canonicalized export ready for later offline review or backfill
// tools.
type CharacterMyShopUnitPricesQuarantineResult struct {
	Summary CharacterMyShopUnitPricesQuarantineSummary `json:"summary"`
	Export  CharacterMyShopUnitPricesExport            `json:"export"`
}

// ValidateCharacterMyShopUnitPricesExport fails closed when a retained export
// does not match the 0023_character_myshop_unit_prices shape. It does not open a
// database, write account snapshots, or mutate the supplied export.
func ValidateCharacterMyShopUnitPricesExport(export CharacterMyShopUnitPricesExport) (CharacterMyShopUnitPricesQuarantineSummary, error) {
	canonical, summary, err := canonicalizeCharacterMyShopUnitPricesExport(export)
	if err != nil {
		return CharacterMyShopUnitPricesQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineCharacterMyShopUnitPricesExport validates a retained export and
// returns a canonicalized copy grouped by ascending character_id then ascending
// vnum. Declared character_ids merge with unit-price row-derived ids so a listed
// character with zero price rows can wipe-to-empty under scoped replace. It never
// opens a database or mutates account snapshots.
func QuarantineCharacterMyShopUnitPricesExport(export CharacterMyShopUnitPricesExport) (CharacterMyShopUnitPricesExport, CharacterMyShopUnitPricesQuarantineSummary, error) {
	return canonicalizeCharacterMyShopUnitPricesExport(export)
}

func canonicalizeCharacterMyShopUnitPricesExport(export CharacterMyShopUnitPricesExport) (CharacterMyShopUnitPricesExport, CharacterMyShopUnitPricesQuarantineSummary, error) {
	if export.MigrationVersion != CharacterMyShopUnitPricesMigrationVersion {
		return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidCharacterMyShopUnitPricesExport, export.MigrationVersion)
	}
	if export.MigrationName != CharacterMyShopUnitPricesMigrationName {
		return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidCharacterMyShopUnitPricesExport, export.MigrationName)
	}
	if export.UnitPrices == nil {
		return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: unit_prices must be present", ErrInvalidCharacterMyShopUnitPricesExport)
	}

	characterIDs := make(map[uint32]struct{}, len(export.UnitPrices)+len(export.CharacterIDs))
	if export.CharacterIDs != nil {
		seenDeclaredIDs := make(map[uint32]struct{}, len(export.CharacterIDs))
		for _, characterID := range export.CharacterIDs {
			if characterID == 0 {
				return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: character_ids entries must be > 0", ErrInvalidCharacterMyShopUnitPricesExport)
			}
			if _, exists := seenDeclaredIDs[characterID]; exists {
				return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: duplicate character_ids entry %d", ErrInvalidCharacterMyShopUnitPricesExport, characterID)
			}
			seenDeclaredIDs[characterID] = struct{}{}
			characterIDs[characterID] = struct{}{}
		}
	}

	byCharacter := make(map[uint32]map[uint32]uint32)
	for _, row := range export.UnitPrices {
		if row.CharacterID == 0 {
			return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: character_id must be > 0", ErrInvalidCharacterMyShopUnitPricesExport)
		}
		if row.Vnum == 0 {
			return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: character %d has myshop_unit_prices row with zero vnum", ErrInvalidCharacterMyShopUnitPricesExport, row.CharacterID)
		}
		prices, ok := byCharacter[row.CharacterID]
		if !ok {
			prices = make(map[uint32]uint32)
			byCharacter[row.CharacterID] = prices
		}
		if _, exists := prices[row.Vnum]; exists {
			return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: duplicate character_id=%d vnum=%d", ErrInvalidCharacterMyShopUnitPricesExport, row.CharacterID, row.Vnum)
		}
		prices[row.Vnum] = row.UnitPrice
		characterIDs[row.CharacterID] = struct{}{}
	}

	for characterID, prices := range byCharacter {
		if len(prices) > loginticket.MyShopUnitPriceMax {
			return CharacterMyShopUnitPricesExport{}, CharacterMyShopUnitPricesQuarantineSummary{}, fmt.Errorf("%w: character %d has %d myshop_unit_prices rows (max %d)", ErrInvalidCharacterMyShopUnitPricesExport, characterID, len(prices), loginticket.MyShopUnitPriceMax)
		}
	}

	sortedCharacterIDs := make([]uint32, 0, len(characterIDs))
	for characterID := range characterIDs {
		sortedCharacterIDs = append(sortedCharacterIDs, characterID)
	}
	sort.Slice(sortedCharacterIDs, func(i, j int) bool { return sortedCharacterIDs[i] < sortedCharacterIDs[j] })

	canonical := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		CharacterIDs:     append([]uint32(nil), sortedCharacterIDs...),
		UnitPrices:       make([]CharacterMyShopUnitPriceRow, 0, len(export.UnitPrices)),
	}
	if canonical.CharacterIDs == nil {
		canonical.CharacterIDs = []uint32{}
	}
	for _, characterID := range sortedCharacterIDs {
		prices, ok := byCharacter[characterID]
		if !ok {
			continue
		}
		vnums := make([]uint32, 0, len(prices))
		for vnum := range prices {
			vnums = append(vnums, vnum)
		}
		sort.Slice(vnums, func(i, j int) bool { return vnums[i] < vnums[j] })
		for _, vnum := range vnums {
			canonical.UnitPrices = append(canonical.UnitPrices, CharacterMyShopUnitPriceRow{
				CharacterID: characterID,
				Vnum:        vnum,
				UnitPrice:   prices[vnum],
			})
		}
	}

	summary := CharacterMyShopUnitPricesQuarantineSummary{
		CharacterCount: len(sortedCharacterIDs),
		PriceRowCount:  len(canonical.UnitPrices),
		CharacterIDs:   sortedCharacterIDs,
	}
	if summary.CharacterIDs == nil {
		summary.CharacterIDs = []uint32{}
	}
	return canonical, summary, nil
}
