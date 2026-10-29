package interactionstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

var (
	ErrSQLExecutorRequired      = errors.New("interaction SQL executor is required")
	ErrSQLSchemaRequired        = errors.New("interaction SQL schema is not applied")
	ErrSQLDefinitionsReferenced = errors.New("interaction SQL definitions are referenced by static actors")
)

// SQLStore is an opt-in repository for the tip-0013 interaction tables.
// FileStore remains the stock runtime path. The caller supplies a transaction-
// capable executor; this package never registers a driver or opens a DSN.
// Tip-0013 cannot represent open_cube or multi-tab shop_preview definitions:
// Save rejects them instead of silently losing authored fields.
type SQLStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

var _ Store = (*SQLStore)(nil)

func NewSQLStore(executor dbmigrations.SQLMigrationExecutor) *SQLStore {
	return &SQLStore{executor: executor}
}

func SelectRematerializeStore(fileStore Store, sqlStore *SQLStore) Store {
	if sqlStore != nil {
		return sqlStore
	}
	return fileStore
}

func (s *SQLStore) begin(readOnly bool) (*sql.Tx, error) {
	if s == nil || s.executor == nil || (reflect.ValueOf(s.executor).Kind() == reflect.Ptr && reflect.ValueOf(s.executor).IsNil()) {
		return nil, ErrSQLExecutorRequired
	}
	options := &sql.TxOptions{ReadOnly: readOnly}
	return s.executor.BeginTx(context.Background(), options)
}

func requireInteractionSQLSchema(ctx context.Context, tx *sql.Tx) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, tx)
	if err != nil {
		return fmt.Errorf("%w: read ledger: %v", ErrSQLSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == 13 && entry.Name == "static_actor_combat_profile_state" {
			return nil
		}
	}
	return fmt.Errorf("%w: missing 0013 static_actor_combat_profile_state", ErrSQLSchemaRequired)
}

func (s *SQLStore) Load() (Snapshot, error) {
	tx, err := s.begin(true)
	if err != nil {
		return Snapshot{}, err
	}
	ctx := context.Background()
	failure := func(err error) (Snapshot, error) { _ = tx.Rollback(); return Snapshot{}, err }
	if err := requireInteractionSQLSchema(ctx, tx); err != nil {
		return failure(err)
	}
	definitions := []Definition{}
	rows, err := tx.QueryContext(ctx, `SELECT kind, ref, text, title, map_index, x, y, size, quest_ref, quest_flag, quest_from, quest_to, reward_experience, reward_gold, consume_gold, consume_experience FROM interaction_definitions ORDER BY kind, ref`)
	if err != nil {
		return failure(fmt.Errorf("query interaction definitions: %w", err))
	}
	for rows.Next() {
		var d Definition
		var mapIndex, x, y sql.NullInt64
		var size, from, to, exp, gold, consumeGold, consumeExp int64
		if err = rows.Scan(&d.Kind, &d.Ref, &d.Text, &d.Title, &mapIndex, &x, &y, &size, &d.QuestRef, &d.QuestFlag, &from, &to, &exp, &gold, &consumeGold, &consumeExp); err != nil {
			break
		}
		if size < 0 || size > math.MaxUint8 || from < 0 || from > math.MaxUint32 || to < 0 || to > math.MaxUint32 || exp < 0 || gold < 0 || consumeGold < 0 || consumeExp < 0 || (mapIndex.Valid != x.Valid || x.Valid != y.Valid) || (mapIndex.Valid && (mapIndex.Int64 < 0 || mapIndex.Int64 > math.MaxUint32 || x.Int64 < math.MinInt32 || x.Int64 > math.MaxInt32 || y.Int64 < math.MinInt32 || y.Int64 > math.MaxInt32)) {
			err = ErrInvalidSnapshot
			break
		}
		d.Size, d.QuestFrom, d.QuestTo = uint8(size), uint32(from), uint32(to)
		d.RewardExperience, d.RewardGold, d.ConsumeGold, d.ConsumeExperience = uint64(exp), uint64(gold), uint64(consumeGold), uint64(consumeExp)
		if mapIndex.Valid {
			d.MapIndex, d.X, d.Y = uint32(mapIndex.Int64), int32(x.Int64), int32(y.Int64)
		}
		definitions = append(definitions, d)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return failure(fmt.Errorf("read interaction definitions: %w", err))
	}
	byKey := make(map[string]*Definition, len(definitions))
	for i := range definitions {
		key := definitions[i].Kind + "\x00" + definitions[i].Ref
		if _, exists := byKey[key]; exists {
			return failure(ErrInvalidSnapshot)
		}
		d := &definitions[i]
		if d.Kind != strings.TrimSpace(d.Kind) || d.Ref != strings.TrimSpace(d.Ref) || d.Text != strings.TrimSpace(d.Text) || d.Title != strings.TrimSpace(d.Title) || d.QuestRef != strings.TrimSpace(d.QuestRef) || d.QuestFlag != strings.TrimSpace(d.QuestFlag) {
			return failure(ErrInvalidSnapshot)
		}
		byKey[key] = d
	}
	catalogRows, err := tx.QueryContext(ctx, `SELECT definition_kind, definition_ref, slot, item_vnum, price, count FROM interaction_merchant_catalog_entries ORDER BY definition_kind, definition_ref, slot`)
	if err != nil {
		return failure(fmt.Errorf("query merchant catalog: %w", err))
	}
	for catalogRows.Next() {
		var kind, ref string
		var slot, vnum, price, count int64
		if err = catalogRows.Scan(&kind, &ref, &slot, &vnum, &price, &count); err != nil {
			break
		}
		parent := byKey[kind+"\x00"+ref]
		if parent == nil || kind != KindShopPreview || slot < 0 || slot > math.MaxUint16 || slot != int64(len(parent.Catalog)) || vnum <= 0 || vnum > math.MaxUint32 || price <= 0 || count <= 0 || count > math.MaxUint16 {
			err = ErrInvalidSnapshot
			break
		}
		parent.Catalog = append(parent.Catalog, MerchantCatalogEntry{Slot: uint16(slot), ItemVnum: uint32(vnum), Price: uint64(price), Count: uint16(count)})
	}
	if err == nil {
		err = catalogRows.Err()
	}
	_ = catalogRows.Close()
	if err != nil {
		return failure(fmt.Errorf("read merchant catalog: %w", err))
	}
	for _, child := range []struct {
		table  string
		reward bool
	}{{"interaction_quest_flag_reward_items", true}, {"interaction_quest_flag_consume_items", false}} {
		// Only fixed, source-controlled identifiers reach this SQL string.
		items, queryErr := tx.QueryContext(ctx, `SELECT definition_kind, definition_ref, position, item_vnum, count FROM `+child.table+` ORDER BY definition_kind, definition_ref, position`)
		if queryErr != nil {
			return failure(fmt.Errorf("query %s: %w", child.table, queryErr))
		}
		for items.Next() {
			var kind, ref string
			var pos, vnum, count int64
			if err = items.Scan(&kind, &ref, &pos, &vnum, &count); err != nil {
				break
			}
			parent := byKey[kind+"\x00"+ref]
			if parent == nil || kind != KindQuestFlag || pos < 0 || pos > math.MaxUint8 || vnum <= 0 || vnum > math.MaxUint32 || count <= 0 || count > math.MaxUint16 {
				err = ErrInvalidSnapshot
				break
			}
			entries := &parent.ConsumeItems
			if child.reward {
				entries = &parent.RewardItems
			}
			if pos != int64(len(*entries)) {
				err = ErrInvalidSnapshot
				break
			}
			*entries = append(*entries, RewardItemEntry{ItemVnum: uint32(vnum), Count: uint16(count)})
		}
		if err == nil {
			err = items.Err()
		}
		_ = items.Close()
		if err != nil {
			return failure(fmt.Errorf("read %s: %w", child.table, err))
		}
	}
	snapshot := normalizeSnapshot(Snapshot{Definitions: definitions})
	if err := validateSQLSnapshot(snapshot); err != nil {
		return failure(err)
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit interaction SQL load: %w", err)
	}
	return snapshot, nil
}

func validateSQLSnapshot(snapshot Snapshot) error {
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	for _, d := range snapshot.Definitions {
		if d.Kind == KindOpenCube || len(d.Tabs) != 0 {
			return fmt.Errorf("%w: interaction kind or tabs not representable by tip-0013", ErrInvalidSnapshot)
		}
	}
	return nil
}

// Save replaces the complete interaction snapshot in one transaction. It leaves
// static actors untouched; existing actor bindings prevent any replacement even
// when driver FK checks are disabled. It never performs a cascade or upsert.
func (s *SQLStore) Save(snapshot Snapshot) error {
	normalized := normalizeSnapshot(snapshot)
	if err := validateSQLSnapshot(normalized); err != nil {
		return err
	}
	tx, err := s.begin(false)
	if err != nil {
		return err
	}
	ctx := context.Background()
	failure := func(err error) error { _ = tx.Rollback(); return err }
	if err := requireInteractionSQLSchema(ctx, tx); err != nil {
		return failure(err)
	}
	// Never depend on the selected driver's FK enforcement to protect actor
	// bindings. Replacing a referenced definition, even with the same key,
	// needs a separate actor/content migration policy.
	var bound int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM static_actors WHERE interaction_kind IS NOT NULL AND interaction_ref IS NOT NULL`).Scan(&bound); err != nil {
		return failure(fmt.Errorf("check static-actor interaction references: %w", err))
	}
	if bound != 0 {
		return failure(ErrSQLDefinitionsReferenced)
	}
	for _, table := range []string{"interaction_merchant_catalog_entries", "interaction_quest_flag_reward_items", "interaction_quest_flag_consume_items", "interaction_definitions"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return failure(fmt.Errorf("delete %s: %w", table, err))
		}
	}
	for _, d := range normalized.Definitions {
		var mapIndex, x, y any
		if d.Kind == KindWarp {
			mapIndex, x, y = int64(d.MapIndex), int64(d.X), int64(d.Y)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO interaction_definitions (kind, ref, text, title, map_index, x, y, size, quest_ref, quest_flag, quest_from, quest_to, reward_experience, reward_gold, consume_gold, consume_experience) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, d.Kind, d.Ref, d.Text, d.Title, mapIndex, x, y, int64(d.Size), d.QuestRef, d.QuestFlag, int64(d.QuestFrom), int64(d.QuestTo), int64(d.RewardExperience), int64(d.RewardGold), int64(d.ConsumeGold), int64(d.ConsumeExperience))
		if err != nil {
			return failure(fmt.Errorf("insert interaction definition: %w", err))
		}
		for _, entry := range d.Catalog {
			_, err := tx.ExecContext(ctx, `INSERT INTO interaction_merchant_catalog_entries (definition_kind, definition_ref, slot, item_vnum, price, count) VALUES (?, ?, ?, ?, ?, ?)`, d.Kind, d.Ref, int64(entry.Slot), int64(entry.ItemVnum), int64(entry.Price), int64(entry.Count))
			if err != nil {
				return failure(fmt.Errorf("insert merchant catalog entry: %w", err))
			}
		}
		for _, child := range []struct {
			table   string
			entries []RewardItemEntry
		}{{"interaction_quest_flag_reward_items", d.RewardItems}, {"interaction_quest_flag_consume_items", d.ConsumeItems}} {
			for pos, entry := range child.entries {
				_, err := tx.ExecContext(ctx, `INSERT INTO `+child.table+` (definition_kind, definition_ref, position, item_vnum, count) VALUES (?, ?, ?, ?, ?)`, d.Kind, d.Ref, pos, int64(entry.ItemVnum), int64(entry.Count))
				if err != nil {
					return failure(fmt.Errorf("insert %s: %w", child.table, err))
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit interaction SQL save: %w", err)
	}
	return nil
}
