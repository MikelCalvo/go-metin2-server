package worldruntime

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// SQLGroundItemStore is the first live DB-backed pending ground-item repository.
// It loads and whole-snapshot-replaces bootstrap_ground_items through a
// caller-supplied database/sql executor. The row shape is tip-0010 plus the
// additive 0026 instance sockets, 0029 instance attributes, and 0030
// ownership-timer columns already required by ImportBootstrapGroundItemState.
// Export identity stays tip-0010.
//
// Save replaces every pending ground row (delete all, then insert). That is
// the FileStore whole-snapshot posture, not insert-only import, not the
// opt-in per-VID scoped replace, and not an upsert. Parent character rows
// from 0002 must already exist. Process-local item ids are not columns on
// tip-0010, so this store does not invent them; stock gamed restart stays on
// the ground-item FileStore.
//
// The package does not select a driver, load a DSN, embed secrets, register
// a production engine, or expose a daemon mutation route.
type SQLGroundItemStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

// NewSQLGroundItemStore returns an opt-in SQL pending-ground repository.
// A nil executor fails closed on Load/Save/Export rather than at construction.
func NewSQLGroundItemStore(executor dbmigrations.SQLMigrationExecutor) *SQLGroundItemStore {
	return &SQLGroundItemStore{executor: executor}
}

// LoadGroundItems projects bootstrap_ground_items onto pending ground snapshots
// ordered by visible VID. An empty table is an empty pending set, not a missing
// FileStore. Schema preflight requires tip-0010 plus additive 0026, 0029, and
// 0030. Rows that cannot target the migration export fail closed.
func (s *SQLGroundItemStore) LoadGroundItems() ([]GroundItemSnapshot, error) {
	if s == nil || groundItemStateImportExecutorIsNil(s.executor) {
		return nil, ErrBootstrapGroundItemStateImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin bootstrap ground-item SQL load transaction: %w", err)
	}
	snapshots, err := loadBootstrapGroundItemSnapshots(ctx, tx)
	if err != nil {
		return nil, rollbackAfterGroundItemStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit bootstrap ground-item SQL load transaction: %w", err)
	}
	return snapshots, nil
}

// SaveGroundItems replaces the entire tip-0010 pending-ground snapshot with the
// supplied handles inside one transaction. A nil or empty slice clears the
// table. Characters referenced by owner_character_id must already exist.
func (s *SQLGroundItemStore) SaveGroundItems(snapshots []GroundItemSnapshot) error {
	if s == nil || groundItemStateImportExecutorIsNil(s.executor) {
		return ErrBootstrapGroundItemStateImportExecutorRequired
	}
	export, err := ExportBootstrapGroundItemState(snapshots)
	if err != nil {
		return err
	}
	canonical, _, err := QuarantineBootstrapGroundItemStateExport(export)
	if err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bootstrap ground-item SQL save transaction: %w", err)
	}
	if err := requireBootstrapGroundItemStateSchema(ctx, tx); err != nil {
		return rollbackAfterGroundItemStateImportFailure(tx, err)
	}
	if err := replaceBootstrapGroundItemSnapshot(ctx, tx, canonical.GroundItems); err != nil {
		return rollbackAfterGroundItemStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bootstrap ground-item SQL save transaction: %w", err)
	}
	return nil
}

// ExportBootstrapGroundItemState projects the committed SQL rows onto the
// tip-0010 migration export. An empty table yields an empty export.
func (s *SQLGroundItemStore) ExportBootstrapGroundItemState() (BootstrapGroundItemStateExport, error) {
	snapshots, err := s.LoadGroundItems()
	if err != nil {
		return BootstrapGroundItemStateExport{}, err
	}
	return ExportBootstrapGroundItemState(snapshots)
}

func replaceBootstrapGroundItemSnapshot(ctx context.Context, tx *sql.Tx, rows []BootstrapGroundItemStateRow) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM bootstrap_ground_items`); err != nil {
		return fmt.Errorf("delete bootstrap ground items: %w", err)
	}
	for _, row := range rows {
		if err := insertBootstrapGroundItem(ctx, tx, row); err != nil {
			return err
		}
	}
	return nil
}

func loadBootstrapGroundItemSnapshots(ctx context.Context, tx *sql.Tx) ([]GroundItemSnapshot, error) {
	if err := requireBootstrapGroundItemStateSchema(ctx, tx); err != nil {
		return nil, err
	}
	const query = `
SELECT vid, vnum, item_count, gold_amount, owner_login, owner_character_id, owner_vid, owner_name,
       map_index, x, y, z, pickup_range,
       has_sockets, socket0, socket1, socket2,
       has_attributes,
       attr0_type, attr0_value, attr1_type, attr1_value, attr2_type, attr2_value,
       attr3_type, attr3_value, attr4_type, attr4_value, attr5_type, attr5_value,
       attr6_type, attr6_value,
       ownership_exclusive, ownership_expires_at, despawn_at
FROM bootstrap_ground_items
ORDER BY vid ASC`
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query bootstrap ground items: %w", err)
	}
	defer rows.Close()

	snapshots := make([]GroundItemSnapshot, 0)
	for rows.Next() {
		row, err := scanBootstrapGroundItemStateRow(rows)
		if err != nil {
			return nil, err
		}
		snapshot, err := groundItemSnapshotFromExportRow(row)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bootstrap ground items: %w", err)
	}
	if _, err := ExportBootstrapGroundItemState(snapshots); err != nil {
		return nil, err
	}
	if snapshots == nil {
		snapshots = []GroundItemSnapshot{}
	}
	return snapshots, nil
}

type bootstrapGroundItemRowScanner interface {
	Scan(dest ...any) error
}

func scanBootstrapGroundItemStateRow(scanner bootstrapGroundItemRowScanner) (BootstrapGroundItemStateRow, error) {
	var (
		vid                int64
		vnum               int64
		itemCount          sql.NullInt64
		goldAmount         sql.NullInt64
		ownerLogin         string
		ownerCharacterID   int64
		ownerVID           int64
		ownerName          string
		mapIndex           int64
		x                  int64
		y                  int64
		z                  int64
		pickupRange        int64
		hasSockets         int64
		socket0            int64
		socket1            int64
		socket2            int64
		hasAttributes      int64
		attr0Type          int64
		attr0Value         int64
		attr1Type          int64
		attr1Value         int64
		attr2Type          int64
		attr2Value         int64
		attr3Type          int64
		attr3Value         int64
		attr4Type          int64
		attr4Value         int64
		attr5Type          int64
		attr5Value         int64
		attr6Type          int64
		attr6Value         int64
		ownershipExclusive int64
		ownershipExpiresAt sql.NullString
		despawnAt          sql.NullString
	)
	if err := scanner.Scan(
		&vid, &vnum, &itemCount, &goldAmount, &ownerLogin, &ownerCharacterID, &ownerVID, &ownerName,
		&mapIndex, &x, &y, &z, &pickupRange,
		&hasSockets, &socket0, &socket1, &socket2,
		&hasAttributes,
		&attr0Type, &attr0Value, &attr1Type, &attr1Value, &attr2Type, &attr2Value,
		&attr3Type, &attr3Value, &attr4Type, &attr4Value, &attr5Type, &attr5Value,
		&attr6Type, &attr6Value,
		&ownershipExclusive, &ownershipExpiresAt, &despawnAt,
	); err != nil {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("scan bootstrap ground item: %w", err)
	}

	rowVID, err := sqlGroundUint32(vid, "vid")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	if rowVID == 0 {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid must be positive", ErrInvalidBootstrapGroundItemStateExport)
	}
	rowVnum, err := sqlGroundUint32(vnum, "vnum")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowOwnerCharacterID, err := sqlGroundUint32(ownerCharacterID, "owner_character_id")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowOwnerVID, err := sqlGroundUint32(ownerVID, "owner_vid")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowMapIndex, err := sqlGroundUint32(mapIndex, "map_index")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowX, err := sqlGroundInt32(x, "x")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowY, err := sqlGroundInt32(y, "y")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowZ, err := sqlGroundInt32(z, "z")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowSocket0, err := sqlGroundInt32(socket0, "socket0")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowSocket1, err := sqlGroundInt32(socket1, "socket1")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowSocket2, err := sqlGroundInt32(socket2, "socket2")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr0Type, err := sqlGroundUint8(attr0Type, "attr0_type")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr0Value, err := sqlGroundInt16(attr0Value, "attr0_value")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr1Type, err := sqlGroundUint8(attr1Type, "attr1_type")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr1Value, err := sqlGroundInt16(attr1Value, "attr1_value")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr2Type, err := sqlGroundUint8(attr2Type, "attr2_type")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr2Value, err := sqlGroundInt16(attr2Value, "attr2_value")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr3Type, err := sqlGroundUint8(attr3Type, "attr3_type")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr3Value, err := sqlGroundInt16(attr3Value, "attr3_value")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr4Type, err := sqlGroundUint8(attr4Type, "attr4_type")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr4Value, err := sqlGroundInt16(attr4Value, "attr4_value")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr5Type, err := sqlGroundUint8(attr5Type, "attr5_type")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr5Value, err := sqlGroundInt16(attr5Value, "attr5_value")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr6Type, err := sqlGroundUint8(attr6Type, "attr6_type")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowAttr6Value, err := sqlGroundInt16(attr6Value, "attr6_value")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowHasSockets, err := sqlGroundBool(hasSockets, "has_sockets")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowHasAttributes, err := sqlGroundBool(hasAttributes, "has_attributes")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowExclusive, err := sqlGroundBool(ownershipExclusive, "ownership_exclusive")
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowItemCount, err := sqlGroundItemCount(itemCount)
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowGoldAmount, err := sqlGroundGoldAmount(goldAmount)
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	if rowItemCount == nil && rowGoldAmount == nil {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has neither item count nor gold amount", ErrInvalidBootstrapGroundItemStateExport, rowVID)
	}
	rowOwnershipExpiresAt, err := sqlGroundTime(ownershipExpiresAt, "ownership_expires_at", rowVID)
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	rowDespawnAt, err := sqlGroundTime(despawnAt, "despawn_at", rowVID)
	if err != nil {
		return BootstrapGroundItemStateRow{}, err
	}

	return BootstrapGroundItemStateRow{
		VID:                rowVID,
		Vnum:               rowVnum,
		ItemCount:          rowItemCount,
		GoldAmount:         rowGoldAmount,
		OwnerLogin:         ownerLogin,
		OwnerCharacterID:   rowOwnerCharacterID,
		OwnerVID:           rowOwnerVID,
		OwnerName:          ownerName,
		MapIndex:           rowMapIndex,
		X:                  rowX,
		Y:                  rowY,
		Z:                  rowZ,
		PickupRange:        pickupRange,
		HasSockets:         rowHasSockets,
		Socket0:            rowSocket0,
		Socket1:            rowSocket1,
		Socket2:            rowSocket2,
		HasAttributes:      rowHasAttributes,
		Attr0Type:          rowAttr0Type,
		Attr0Value:         rowAttr0Value,
		Attr1Type:          rowAttr1Type,
		Attr1Value:         rowAttr1Value,
		Attr2Type:          rowAttr2Type,
		Attr2Value:         rowAttr2Value,
		Attr3Type:          rowAttr3Type,
		Attr3Value:         rowAttr3Value,
		Attr4Type:          rowAttr4Type,
		Attr4Value:         rowAttr4Value,
		Attr5Type:          rowAttr5Type,
		Attr5Value:         rowAttr5Value,
		Attr6Type:          rowAttr6Type,
		Attr6Value:         rowAttr6Value,
		OwnershipExclusive: rowExclusive,
		OwnershipExpiresAt: rowOwnershipExpiresAt,
		DespawnAt:          rowDespawnAt,
	}, nil
}

func sqlGroundUint32(value int64, field string) (uint32, error) {
	if value < 0 || value > math.MaxUint32 {
		return 0, fmt.Errorf("%w: %s %d out of range", ErrInvalidBootstrapGroundItemStateExport, field, value)
	}
	return uint32(value), nil
}

func sqlGroundInt32(value int64, field string) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%w: %s %d out of range", ErrInvalidBootstrapGroundItemStateExport, field, value)
	}
	return int32(value), nil
}

func sqlGroundUint8(value int64, field string) (uint8, error) {
	if value < 0 || value > math.MaxUint8 {
		return 0, fmt.Errorf("%w: %s %d out of range", ErrInvalidBootstrapGroundItemStateExport, field, value)
	}
	return uint8(value), nil
}

func sqlGroundInt16(value int64, field string) (int16, error) {
	if value < math.MinInt16 || value > math.MaxInt16 {
		return 0, fmt.Errorf("%w: %s %d out of range", ErrInvalidBootstrapGroundItemStateExport, field, value)
	}
	return int16(value), nil
}

func sqlGroundBool(value int64, field string) (bool, error) {
	switch value {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("%w: %s %d out of range", ErrInvalidBootstrapGroundItemStateExport, field, value)
	}
}

func sqlGroundItemCount(value sql.NullInt64) (*uint16, error) {
	if !value.Valid {
		return nil, nil
	}
	if value.Int64 < 1 || value.Int64 > int64(bootstrapGroundItemMaxCount) {
		return nil, fmt.Errorf("%w: item_count %d out of range", ErrInvalidBootstrapGroundItemStateExport, value.Int64)
	}
	count := uint16(value.Int64)
	return &count, nil
}

func sqlGroundGoldAmount(value sql.NullInt64) (*uint32, error) {
	if !value.Valid {
		return nil, nil
	}
	if value.Int64 < 1 || value.Int64 > int64(bootstrapGroundGoldMaxAmount) {
		return nil, fmt.Errorf("%w: gold_amount %d out of range", ErrInvalidBootstrapGroundItemStateExport, value.Int64)
	}
	amount := uint32(value.Int64)
	return &amount, nil
}

func sqlGroundTime(value sql.NullString, field string, vid uint32) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value.String)
	if err != nil {
		return nil, fmt.Errorf("%w: ground vid %d %s %q", ErrInvalidBootstrapGroundItemStateExport, vid, field, value.String)
	}
	utc := parsed.UTC()
	return &utc, nil
}
