package safeboxstore

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

// SQLStore is the first live DB-backed safebox repository. It implements Store
// Load/Save (and CharacterSafeboxStateExporter) against already-owned
// tip-0015 + additive 0025/0028 tables through a caller-supplied
// database/sql executor.
//
// The package does not select a driver, load a DSN, embed secrets, or
// register a production engine. Stock gamed rematerialize stays on FileStore.
type SQLStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

// NewSQLStore returns an opt-in SQL safebox repository. A nil executor fails
// closed on Load/Save/Export rather than at construction time.
func NewSQLStore(executor dbmigrations.SQLMigrationExecutor) *SQLStore {
	return &SQLStore{executor: executor}
}

// Load projects character_safebox_passwords + character_safebox_items into a
// normalized Snapshot. Empty tables are an empty warehouse, not a missing
// FileStore snapshot. Schema preflight requires tip-0015 + 0025 + 0028.
func (s *SQLStore) Load() (Snapshot, error) {
	if s == nil || safeboxStateImportExecutorIsNil(s.executor) {
		return Snapshot{}, ErrCharacterSafeboxStateImportExecutorRequired
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin character safebox SQL load transaction: %w", err)
	}

	snapshot, err := loadCharacterSafeboxSnapshot(ctx, tx)
	if err != nil {
		return Snapshot{}, rollbackAfterSafeboxStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit character safebox SQL load transaction: %w", err)
	}
	return snapshot, nil
}

// Save replaces the entire tip-0015 warehouse with the canonicalized snapshot
// inside one transaction (delete all child rows, then insert). This matches
// FileStore Save of a whole JSON snapshot; it is not insert-only import and
// not scoped replace. Parent character rows must already exist.
func (s *SQLStore) Save(snapshot Snapshot) error {
	if s == nil || safeboxStateImportExecutorIsNil(s.executor) {
		return ErrCharacterSafeboxStateImportExecutorRequired
	}

	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate safebox snapshot", err)
	}
	export, err := ExportCharacterSafeboxState(normalized)
	if err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin character safebox SQL save transaction: %w", err)
	}
	if err := requireCharacterSafeboxStateSchema(ctx, tx); err != nil {
		return rollbackAfterSafeboxStateImportFailure(tx, err)
	}
	if err := replaceCharacterSafeboxState(ctx, tx, export); err != nil {
		return rollbackAfterSafeboxStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit character safebox SQL save transaction: %w", err)
	}
	return nil
}

// ExportCharacterSafeboxState projects the committed SQL warehouse onto the
// 0015 migration tip. Empty tables yield an empty export.
func (s *SQLStore) ExportCharacterSafeboxState() (CharacterSafeboxStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		return CharacterSafeboxStateExport{}, err
	}
	return ExportCharacterSafeboxState(snapshot)
}

func replaceCharacterSafeboxState(ctx context.Context, tx *sql.Tx, export CharacterSafeboxStateExport) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM character_safebox_items`); err != nil {
		return fmt.Errorf("delete character safebox items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM character_safebox_passwords`); err != nil {
		return fmt.Errorf("delete character safebox passwords: %w", err)
	}
	for _, row := range export.Passwords {
		if err := insertCharacterSafeboxPassword(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.Items {
		if err := insertCharacterSafeboxItem(ctx, tx, row); err != nil {
			return err
		}
	}
	return nil
}

func loadCharacterSafeboxSnapshot(ctx context.Context, tx *sql.Tx) (Snapshot, error) {
	if err := requireCharacterSafeboxStateSchema(ctx, tx); err != nil {
		return Snapshot{}, err
	}

	passwordRows, err := tx.QueryContext(ctx, `
SELECT character_id, login, password, money
FROM character_safebox_passwords
ORDER BY login ASC, character_id ASC`)
	if err != nil {
		return Snapshot{}, fmt.Errorf("query character safebox passwords: %w", err)
	}
	defer passwordRows.Close()

	characters := make([]CharacterRow, 0)
	seen := make(map[uint32]int)
	for passwordRows.Next() {
		var (
			characterID int64
			login       string
			password    string
			money       int64
		)
		if err := passwordRows.Scan(&characterID, &login, &password, &money); err != nil {
			return Snapshot{}, fmt.Errorf("scan character safebox password: %w", err)
		}
		id, err := sqlUint32(characterID, "character_id")
		if err != nil {
			return Snapshot{}, err
		}
		if _, exists := seen[id]; exists {
			return Snapshot{}, fmt.Errorf("%w: duplicate character_id=%d password row", ErrInvalidSnapshot, id)
		}
		seen[id] = len(characters)
		characters = append(characters, CharacterRow{
			Login:       login,
			CharacterID: id,
			Password:    password,
			Money:       money,
			Cells:       []Cell{},
		})
	}
	if err := passwordRows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("iterate character safebox passwords: %w", err)
	}

	itemRows, err := tx.QueryContext(ctx, `
SELECT id, character_id, login, cell, vnum, count, locked,
       has_sockets, socket0, socket1, socket2,
       has_attributes,
       attr0_type, attr0_value, attr1_type, attr1_value, attr2_type, attr2_value,
       attr3_type, attr3_value, attr4_type, attr4_value, attr5_type, attr5_value,
       attr6_type, attr6_value
FROM character_safebox_items
ORDER BY character_id ASC, cell ASC`)
	if err != nil {
		return Snapshot{}, fmt.Errorf("query character safebox items: %w", err)
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var (
			id            int64
			characterID   int64
			login         string
			cell          int64
			vnum          int64
			count         int64
			locked        int64
			hasSockets    int64
			socket0       int64
			socket1       int64
			socket2       int64
			hasAttributes int64
			attr0Type     int64
			attr0Value    int64
			attr1Type     int64
			attr1Value    int64
			attr2Type     int64
			attr2Value    int64
			attr3Type     int64
			attr3Value    int64
			attr4Type     int64
			attr4Value    int64
			attr5Type     int64
			attr5Value    int64
			attr6Type     int64
			attr6Value    int64
		)
		if err := itemRows.Scan(
			&id, &characterID, &login, &cell, &vnum, &count, &locked,
			&hasSockets, &socket0, &socket1, &socket2,
			&hasAttributes,
			&attr0Type, &attr0Value, &attr1Type, &attr1Value, &attr2Type, &attr2Value,
			&attr3Type, &attr3Value, &attr4Type, &attr4Value, &attr5Type, &attr5Value,
			&attr6Type, &attr6Value,
		); err != nil {
			return Snapshot{}, fmt.Errorf("scan character safebox item: %w", err)
		}
		item, err := snapshotCellFromSQL(
			id, characterID, login, cell, vnum, count, locked,
			hasSockets, socket0, socket1, socket2,
			hasAttributes,
			attr0Type, attr0Value, attr1Type, attr1Value, attr2Type, attr2Value,
			attr3Type, attr3Value, attr4Type, attr4Value, attr5Type, attr5Value,
			attr6Type, attr6Value,
		)
		if err != nil {
			return Snapshot{}, err
		}
		parentID, err := sqlUint32(characterID, "character_id")
		if err != nil {
			return Snapshot{}, err
		}
		idx, ok := seen[parentID]
		if !ok {
			return Snapshot{}, fmt.Errorf("%w: safebox item id %d character_id=%d has no password parent", ErrInvalidSnapshot, item.ID, parentID)
		}
		if characters[idx].Login != login {
			return Snapshot{}, fmt.Errorf("%w: safebox item id %d character_id=%d login %q does not match password login %q", ErrInvalidSnapshot, item.ID, parentID, login, characters[idx].Login)
		}
		characters[idx].Cells = append(characters[idx].Cells, item)
	}
	if err := itemRows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("iterate character safebox items: %w", err)
	}

	normalized := normalizeSnapshot(Snapshot{Characters: characters})
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate safebox snapshot", err)
	}
	return normalized, nil
}

func snapshotCellFromSQL(
	id, characterID int64,
	login string,
	cell, vnum, count, locked int64,
	hasSockets, socket0, socket1, socket2 int64,
	hasAttributes int64,
	attr0Type, attr0Value, attr1Type, attr1Value, attr2Type, attr2Value int64,
	attr3Type, attr3Value, attr4Type, attr4Value, attr5Type, attr5Value int64,
	attr6Type, attr6Value int64,
) (Cell, error) {
	_ = characterID
	_ = login
	itemID, err := sqlUint64(id, "id")
	if err != nil {
		return Cell{}, err
	}
	cellIndex, err := sqlUint8(cell, "cell")
	if err != nil {
		return Cell{}, err
	}
	itemVnum, err := sqlUint32(vnum, "vnum")
	if err != nil {
		return Cell{}, err
	}
	itemCount, err := sqlUint16(count, "count")
	if err != nil {
		return Cell{}, err
	}
	lockedFlag, err := sqlIntAsBool(locked, "locked")
	if err != nil {
		return Cell{}, err
	}
	socketsPresent, err := sqlIntAsBool(hasSockets, "has_sockets")
	if err != nil {
		return Cell{}, err
	}
	sock0, err := sqlInt32(socket0, "socket0")
	if err != nil {
		return Cell{}, err
	}
	sock1, err := sqlInt32(socket1, "socket1")
	if err != nil {
		return Cell{}, err
	}
	sock2, err := sqlInt32(socket2, "socket2")
	if err != nil {
		return Cell{}, err
	}
	attributesPresent, err := sqlIntAsBool(hasAttributes, "has_attributes")
	if err != nil {
		return Cell{}, err
	}
	attrs, err := sqlAttributeValues(
		attr0Type, attr0Value, attr1Type, attr1Value, attr2Type, attr2Value,
		attr3Type, attr3Value, attr4Type, attr4Value, attr5Type, attr5Value,
		attr6Type, attr6Value,
	)
	if err != nil {
		return Cell{}, err
	}
	out := Cell{
		Cell:          cellIndex,
		ID:            itemID,
		Vnum:          itemVnum,
		Count:         itemCount,
		Locked:        lockedFlag,
		HasSockets:    socketsPresent,
		Socket0:       sock0,
		Socket1:       sock1,
		Socket2:       sock2,
		HasAttributes: attributesPresent,
	}
	if attributesPresent {
		copied := attrs
		out.Attributes = &copied
	}
	return out, nil
}

func sqlIntAsBool(value int64, field string) (bool, error) {
	switch value {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("%w: %s %d", ErrInvalidSnapshot, field, value)
	}
}

func sqlUint8(value int64, field string) (uint8, error) {
	if value < 0 || value > math.MaxUint8 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidSnapshot, field, value)
	}
	return uint8(value), nil
}

func sqlUint16(value int64, field string) (uint16, error) {
	if value < 0 || value > math.MaxUint16 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidSnapshot, field, value)
	}
	return uint16(value), nil
}

func sqlUint32(value int64, field string) (uint32, error) {
	if value < 0 || value > math.MaxUint32 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidSnapshot, field, value)
	}
	return uint32(value), nil
}

func sqlUint64(value int64, field string) (uint64, error) {
	if value <= 0 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidSnapshot, field, value)
	}
	return uint64(value), nil
}

func sqlInt32(value int64, field string) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidSnapshot, field, value)
	}
	return int32(value), nil
}

func sqlInt16(value int64, field string) (int16, error) {
	if value < math.MinInt16 || value > math.MaxInt16 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidSnapshot, field, value)
	}
	return int16(value), nil
}

func sqlAttributeValues(
	attr0Type, attr0Value, attr1Type, attr1Value, attr2Type, attr2Value int64,
	attr3Type, attr3Value, attr4Type, attr4Value, attr5Type, attr5Value int64,
	attr6Type, attr6Value int64,
) (inventory.AttributeValues, error) {
	types := []int64{attr0Type, attr1Type, attr2Type, attr3Type, attr4Type, attr5Type, attr6Type}
	values := []int64{attr0Value, attr1Value, attr2Value, attr3Value, attr4Value, attr5Value, attr6Value}
	var attrs inventory.AttributeValues
	for i := range attrs {
		attrType, err := sqlUint8(types[i], fmt.Sprintf("attr%d_type", i))
		if err != nil {
			return inventory.AttributeValues{}, err
		}
		attrValue, err := sqlInt16(values[i], fmt.Sprintf("attr%d_value", i))
		if err != nil {
			return inventory.AttributeValues{}, err
		}
		attrs[i] = inventory.Attribute{Type: attrType, Value: attrValue}
	}
	return attrs, nil
}
