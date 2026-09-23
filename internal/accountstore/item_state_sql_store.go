package accountstore

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

// SQLItemStateStore is the first live DB-backed inventory/equipment repository.
// It implements Store Load/Save (and AccountCharacterStateExporter's item-state
// method) against already-owned tip-0003 + additive 0024/0027 tables through a
// caller-supplied database/sql executor. Export identity stays tip-0003.
//
// Load returns only inventory, equipment, and quickslots. Roster identity
// (login, slot, name, appearance, points, gold) is reconstructed from the
// already-owned 0002 characters/accounts rows so the result still satisfies
// Store. Save writes only the three item-state tables; it does not insert,
// update, or delete roster rows.
//
// The package does not select a driver, load a DSN, embed secrets, or
// register a production engine. Stock gamed rematerialize stays on FileStore.
// Save replaces the whole item-state snapshot (delete all child rows, then
// insert). That is the FileStore whole-snapshot posture, not insert-only
// import, not scoped replace, and not an upsert.
type SQLItemStateStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

// NewSQLItemStateStore returns an opt-in SQL inventory/equipment repository.
// A nil executor fails closed on Load/Save/Export rather than at construction.
func NewSQLItemStateStore(executor dbmigrations.SQLMigrationExecutor) *SQLItemStateStore {
	return &SQLItemStateStore{executor: executor}
}

// Load projects character_inventory_items, character_equipment_items, and
// character_quickslots onto Account snapshots whose roster shell comes from
// 0002. Empty item tables are an empty carried set, not a missing FileStore.
// Schema preflight requires tip-0003 + additive 0024 + additive 0027.
func (s *SQLItemStateStore) Load(login string) (Account, error) {
	if strings.TrimSpace(login) == "" {
		return Account{}, ErrLoginRequired
	}
	if login != strings.TrimSpace(login) {
		return Account{}, fmt.Errorf("%w: account login %q has leading or trailing whitespace", ErrInvalidAccount, login)
	}
	if containsNUL(login) {
		return Account{}, fmt.Errorf("%w: account login contains NUL", ErrInvalidAccount)
	}
	accounts, err := s.loadAccounts()
	if err != nil {
		return Account{}, err
	}
	for _, account := range accounts {
		if strings.EqualFold(account.Login, login) {
			return account, nil
		}
	}
	return Account{}, ErrAccountNotFound
}

// Save replaces the entire tip-0003 item-state snapshot with the canonicalized
// accounts inside one transaction. Parent character rows must already exist.
// Characters present in 0002 but omitted from the snapshot lose their item
// rows, matching FileStore Save of a whole snapshot rather than a scoped merge.
func (s *SQLItemStateStore) Save(account Account) error {
	return s.SaveAccounts([]Account{account})
}

// SaveAccounts replaces the entire tip-0003 item-state snapshot from the
// supplied account set. The zero-length set clears inventory, equipment, and
// quickslots without touching roster rows.
func (s *SQLItemStateStore) SaveAccounts(accounts []Account) error {
	if s == nil || itemStateImportExecutorIsNil(s.executor) {
		return ErrCharacterItemStateImportExecutorRequired
	}
	normalized := normalizeItemStateAccounts(accounts)
	if err := validateItemStateAccounts(normalized); err != nil {
		return fmt.Errorf("%w: validate item-state accounts", err)
	}
	export, err := ExportCharacterItemState(normalized)
	if err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin character item-state SQL save transaction: %w", err)
	}
	if err := requireCharacterItemStateSchema(ctx, tx); err != nil {
		return rollbackAfterItemStateImportFailure(tx, err)
	}
	if err := replaceCharacterItemStateSnapshot(ctx, tx, export); err != nil {
		return rollbackAfterItemStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit character item-state SQL save transaction: %w", err)
	}
	return nil
}

// List projects every committed SQL account that owns at least one 0002
// character row. Accounts with no item rows are still returned so callers can
// tell an empty warehouse from a missing login.
func (s *SQLItemStateStore) List() ([]Account, error) {
	return s.loadAccounts()
}

// ExportCharacterItemState projects the committed SQL item-state tables onto
// the 0003 migration tip. Empty tables yield an empty export.
func (s *SQLItemStateStore) ExportCharacterItemState() (CharacterItemStateExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterItemStateExport{}, err
	}
	return ExportCharacterItemState(accounts)
}

func (s *SQLItemStateStore) loadAccounts() ([]Account, error) {
	if s == nil || itemStateImportExecutorIsNil(s.executor) {
		return nil, ErrCharacterItemStateImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin character item-state SQL load transaction: %w", err)
	}
	accounts, err := loadCharacterItemStateAccounts(ctx, tx)
	if err != nil {
		return nil, rollbackAfterItemStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit character item-state SQL load transaction: %w", err)
	}
	return accounts, nil
}

func replaceCharacterItemStateSnapshot(ctx context.Context, tx *sql.Tx, export CharacterItemStateExport) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM character_inventory_items`); err != nil {
		return fmt.Errorf("delete character inventory items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM character_equipment_items`); err != nil {
		return fmt.Errorf("delete character equipment items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM character_quickslots`); err != nil {
		return fmt.Errorf("delete character quickslots: %w", err)
	}
	for _, row := range export.InventoryItems {
		if err := insertCharacterInventoryItem(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.EquipmentItems {
		if err := insertCharacterEquipmentItem(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.Quickslots {
		if err := insertCharacterQuickslot(ctx, tx, row); err != nil {
			return err
		}
	}
	return nil
}

func loadCharacterItemStateAccounts(ctx context.Context, tx *sql.Tx) ([]Account, error) {
	if err := requireCharacterItemStateSchema(ctx, tx); err != nil {
		return nil, err
	}

	rosterQuery := `
SELECT accounts.login, accounts.empire, characters.id, characters.slot, characters.name
FROM characters
JOIN accounts ON accounts.id = characters.account_id
ORDER BY accounts.login_normalized ASC, characters.slot ASC`
	rosterRows, err := tx.QueryContext(ctx, rosterQuery)
	if err != nil {
		return nil, fmt.Errorf("query character item-state roster: %w", err)
	}
	defer rosterRows.Close()

	var roster []itemStateRosterCharacter
	accountIndex := map[string]int{}
	accounts := make([]Account, 0)
	for rosterRows.Next() {
		var (
			login       string
			empire      int64
			characterID int64
			slot        int64
			name        string
		)
		if err := rosterRows.Scan(&login, &empire, &characterID, &slot, &name); err != nil {
			return nil, fmt.Errorf("scan character item-state roster: %w", err)
		}
		id, err := sqlItemStateUint32(characterID, "character_id")
		if err != nil {
			return nil, err
		}
		if slot < 0 || slot >= accountCharacterRosterPlayerSlots {
			return nil, fmt.Errorf("%w: character id %d slot %d outside item-state roster", ErrInvalidAccount, id, slot)
		}
		empireValue, err := sqlItemStateUint8(empire, "empire")
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(login)
		idx, ok := accountIndex[key]
		if !ok {
			idx = len(accounts)
			accountIndex[key] = idx
			accounts = append(accounts, Account{
				Login:      login,
				Empire:     empireValue,
				Characters: make([]loginticket.Character, accountCharacterRosterPlayerSlots),
			})
		} else if accounts[idx].Empire != empireValue {
			return nil, fmt.Errorf("%w: account %q empire drifted between roster rows", ErrInvalidAccount, login)
		}
		roster = append(roster, itemStateRosterCharacter{
			login:   login,
			empire:  empireValue,
			id:      id,
			slot:    int(slot),
			name:    name,
			account: idx,
		})
		character := rosterExportShell(id, name)
		character.Empire = empireValue
		accounts[idx].Characters[slot] = character
	}
	if err := rosterRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate character item-state roster: %w", err)
	}

	characterSlot := make(map[uint32]int, len(roster))
	for i := range roster {
		if _, exists := characterSlot[roster[i].id]; exists {
			return nil, fmt.Errorf("%w: duplicate character_id=%d roster row", ErrInvalidAccount, roster[i].id)
		}
		characterSlot[roster[i].id] = i
	}

	if err := attachInventoryItems(ctx, tx, accounts, roster, characterSlot); err != nil {
		return nil, err
	}
	if err := attachEquipmentItems(ctx, tx, accounts, roster, characterSlot); err != nil {
		return nil, err
	}
	if err := attachQuickslots(ctx, tx, accounts, roster, characterSlot); err != nil {
		return nil, err
	}

	normalized := normalizeItemStateAccounts(accounts)
	if err := validateItemStateAccounts(normalized); err != nil {
		return nil, fmt.Errorf("%w: validate item-state accounts", err)
	}
	if _, err := ExportCharacterItemState(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

type itemStateRosterCharacter struct {
	login   string
	empire  uint8
	id      uint32
	slot    int
	name    string
	account int
}

func attachInventoryItems(ctx context.Context, tx *sql.Tx, accounts []Account, roster []itemStateRosterCharacter, characterSlot map[uint32]int) error {
	rows, err := tx.QueryContext(ctx, `
SELECT id, character_id, slot, vnum, count, locked,
       has_sockets, socket0, socket1, socket2,
       has_attributes,
       attr0_type, attr0_value, attr1_type, attr1_value, attr2_type, attr2_value,
       attr3_type, attr3_value, attr4_type, attr4_value, attr5_type, attr5_value,
       attr6_type, attr6_value
FROM character_inventory_items
ORDER BY character_id ASC, slot ASC, id ASC`)
	if err != nil {
		return fmt.Errorf("query character inventory items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item, characterID, err := scanInventoryItem(rows)
		if err != nil {
			return err
		}
		idx, ok := characterSlot[characterID]
		if !ok {
			return fmt.Errorf("%w: inventory item id %d character_id=%d has no roster parent", ErrInvalidAccount, item.ID, characterID)
		}
		parent := roster[idx]
		accounts[parent.account].Characters[parent.slot].Inventory = append(accounts[parent.account].Characters[parent.slot].Inventory, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate character inventory items: %w", err)
	}
	return nil
}

func attachEquipmentItems(ctx context.Context, tx *sql.Tx, accounts []Account, roster []itemStateRosterCharacter, characterSlot map[uint32]int) error {
	rows, err := tx.QueryContext(ctx, `
SELECT id, character_id, equip_slot, vnum, count, locked,
       has_sockets, socket0, socket1, socket2,
       has_attributes,
       attr0_type, attr0_value, attr1_type, attr1_value, attr2_type, attr2_value,
       attr3_type, attr3_value, attr4_type, attr4_value, attr5_type, attr5_value,
       attr6_type, attr6_value
FROM character_equipment_items
ORDER BY character_id ASC, equip_slot ASC, id ASC`)
	if err != nil {
		return fmt.Errorf("query character equipment items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item, characterID, err := scanEquipmentItem(rows)
		if err != nil {
			return err
		}
		idx, ok := characterSlot[characterID]
		if !ok {
			return fmt.Errorf("%w: equipment item id %d character_id=%d has no roster parent", ErrInvalidAccount, item.ID, characterID)
		}
		parent := roster[idx]
		accounts[parent.account].Characters[parent.slot].Equipment = append(accounts[parent.account].Characters[parent.slot].Equipment, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate character equipment items: %w", err)
	}
	return nil
}

func attachQuickslots(ctx context.Context, tx *sql.Tx, accounts []Account, roster []itemStateRosterCharacter, characterSlot map[uint32]int) error {
	rows, err := tx.QueryContext(ctx, `
SELECT character_id, position, type, slot
FROM character_quickslots
ORDER BY character_id ASC, position ASC`)
	if err != nil {
		return fmt.Errorf("query character quickslots: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var characterID, position, slotType, slot int64
		if err := rows.Scan(&characterID, &position, &slotType, &slot); err != nil {
			return fmt.Errorf("scan character quickslot: %w", err)
		}
		id, err := sqlItemStateUint32(characterID, "character_id")
		if err != nil {
			return err
		}
		idx, ok := characterSlot[id]
		if !ok {
			return fmt.Errorf("%w: quickslot character_id=%d has no roster parent", ErrInvalidAccount, id)
		}
		positionValue, err := sqlItemStateUint8(position, "position")
		if err != nil {
			return err
		}
		typeValue, err := sqlItemStateUint8(slotType, "type")
		if err != nil {
			return err
		}
		slotValue, err := sqlItemStateUint8(slot, "slot")
		if err != nil {
			return err
		}
		parent := roster[idx]
		accounts[parent.account].Characters[parent.slot].Quickslots = append(accounts[parent.account].Characters[parent.slot].Quickslots, loginticket.Quickslot{
			Position: positionValue,
			Type:     typeValue,
			Slot:     slotValue,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate character quickslots: %w", err)
	}
	return nil
}

type itemStateScanner interface {
	Scan(dest ...any) error
}

func scanInventoryItem(rows itemStateScanner) (inventory.ItemInstance, uint32, error) {
	var (
		id            int64
		characterID   int64
		slot          int64
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
	if err := rows.Scan(
		&id, &characterID, &slot, &vnum, &count, &locked,
		&hasSockets, &socket0, &socket1, &socket2,
		&hasAttributes,
		&attr0Type, &attr0Value, &attr1Type, &attr1Value, &attr2Type, &attr2Value,
		&attr3Type, &attr3Value, &attr4Type, &attr4Value, &attr5Type, &attr5Value,
		&attr6Type, &attr6Value,
	); err != nil {
		return inventory.ItemInstance{}, 0, fmt.Errorf("scan character inventory item: %w", err)
	}
	ownerID, err := sqlItemStateUint32(characterID, "character_id")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	itemID, err := sqlItemStateUint64(id, "id")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	slotIndex, err := sqlItemStateSlot(slot)
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	itemVnum, err := sqlItemStateUint32(vnum, "vnum")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	itemCount, err := sqlItemStateUint16(count, "count")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	lockedFlag, err := sqlItemStateBool(locked, "locked")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	sockets, err := sqlItemStateSockets(hasSockets, socket0, socket1, socket2)
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	attributes, err := sqlItemStateAttributes(
		hasAttributes,
		attr0Type, attr0Value, attr1Type, attr1Value, attr2Type, attr2Value,
		attr3Type, attr3Value, attr4Type, attr4Value, attr5Type, attr5Value,
		attr6Type, attr6Value,
	)
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	return inventory.ItemInstance{
		ID:         itemID,
		Vnum:       itemVnum,
		Count:      itemCount,
		Slot:       slotIndex,
		Locked:     lockedFlag,
		Sockets:    sockets,
		Attributes: attributes,
	}, ownerID, nil
}

func scanEquipmentItem(rows itemStateScanner) (inventory.ItemInstance, uint32, error) {
	var (
		id            int64
		characterID   int64
		equipSlot     string
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
	if err := rows.Scan(
		&id, &characterID, &equipSlot, &vnum, &count, &locked,
		&hasSockets, &socket0, &socket1, &socket2,
		&hasAttributes,
		&attr0Type, &attr0Value, &attr1Type, &attr1Value, &attr2Type, &attr2Value,
		&attr3Type, &attr3Value, &attr4Type, &attr4Value, &attr5Type, &attr5Value,
		&attr6Type, &attr6Value,
	); err != nil {
		return inventory.ItemInstance{}, 0, fmt.Errorf("scan character equipment item: %w", err)
	}
	ownerID, err := sqlItemStateUint32(characterID, "character_id")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	itemID, err := sqlItemStateUint64(id, "id")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	parsedSlot, ok := inventory.ParseEquipmentSlot(equipSlot)
	if !ok || !parsedSlot.Valid() {
		return inventory.ItemInstance{}, 0, fmt.Errorf("%w: equipment item id %d has invalid equip_slot %q", ErrInvalidAccount, itemID, equipSlot)
	}
	itemVnum, err := sqlItemStateUint32(vnum, "vnum")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	itemCount, err := sqlItemStateUint16(count, "count")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	lockedFlag, err := sqlItemStateBool(locked, "locked")
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	sockets, err := sqlItemStateSockets(hasSockets, socket0, socket1, socket2)
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	attributes, err := sqlItemStateAttributes(
		hasAttributes,
		attr0Type, attr0Value, attr1Type, attr1Value, attr2Type, attr2Value,
		attr3Type, attr3Value, attr4Type, attr4Value, attr5Type, attr5Value,
		attr6Type, attr6Value,
	)
	if err != nil {
		return inventory.ItemInstance{}, 0, err
	}
	return inventory.ItemInstance{
		ID:         itemID,
		Vnum:       itemVnum,
		Count:      itemCount,
		Equipped:   true,
		EquipSlot:  parsedSlot,
		Locked:     lockedFlag,
		Sockets:    sockets,
		Attributes: attributes,
	}, ownerID, nil
}

func normalizeItemStateAccounts(accounts []Account) []Account {
	if accounts == nil {
		return []Account{}
	}
	cloned := make([]Account, len(accounts))
	for i := range accounts {
		cloned[i] = Account{
			Login:      accounts[i].Login,
			Empire:     accounts[i].Empire,
			Characters: normalizeAccountCharacters(accounts[i].Characters),
		}
	}
	sort.SliceStable(cloned, func(i, j int) bool {
		left := strings.ToLower(cloned[i].Login)
		right := strings.ToLower(cloned[j].Login)
		if left != right {
			return left < right
		}
		return cloned[i].Login < cloned[j].Login
	})
	return cloned
}

func validateItemStateAccounts(accounts []Account) error {
	seenLogins := map[string]string{}
	for _, account := range accounts {
		if strings.TrimSpace(account.Login) == "" {
			return ErrLoginRequired
		}
		if account.Login != strings.TrimSpace(account.Login) {
			return fmt.Errorf("%w: account login %q has leading or trailing whitespace", ErrInvalidAccount, account.Login)
		}
		if containsNUL(account.Login) {
			return fmt.Errorf("%w: account login contains NUL", ErrInvalidAccount)
		}
		normalized := strings.ToLower(account.Login)
		if previous, ok := seenLogins[normalized]; ok {
			return fmt.Errorf("%w: account login %q duplicates %q", ErrInvalidAccount, account.Login, previous)
		}
		seenLogins[normalized] = account.Login
		if err := validateAccount(account); err != nil {
			return err
		}
		if err := validateAccountUniqueInventorySlots(account); err != nil {
			return err
		}
	}
	return nil
}

func rosterExportShell(id uint32, name string) loginticket.Character {
	return loginticket.Character{
		ID:       id,
		Name:     name,
		Level:    1,
		MapIndex: 1,
	}
}

func sqlItemStateBool(value int64, field string) (bool, error) {
	switch value {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
}

func sqlItemStateUint8(value int64, field string) (uint8, error) {
	if value < 0 || value > math.MaxUint8 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return uint8(value), nil
}

func sqlItemStateUint16(value int64, field string) (uint16, error) {
	if value < 0 || value > math.MaxUint16 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return uint16(value), nil
}

func sqlItemStateUint32(value int64, field string) (uint32, error) {
	if value < 0 || value > math.MaxUint32 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return uint32(value), nil
}

func sqlItemStateUint64(value int64, field string) (uint64, error) {
	if value <= 0 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return uint64(value), nil
}

func sqlItemStateSlot(value int64) (inventory.SlotIndex, error) {
	if value < 0 || value >= int64(inventory.CarriedInventorySlotCount) {
		return 0, fmt.Errorf("%w: slot %d", ErrInvalidAccount, value)
	}
	return inventory.SlotIndex(value), nil
}

func sqlItemStateInt32(value int64, field string) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return int32(value), nil
}

func sqlItemStateInt16(value int64, field string) (int16, error) {
	if value < math.MinInt16 || value > math.MaxInt16 {
		return 0, fmt.Errorf("%w: %s %d", ErrInvalidAccount, field, value)
	}
	return int16(value), nil
}

func sqlItemStateSockets(hasSockets, socket0, socket1, socket2 int64) (*inventory.SocketValues, error) {
	present, err := sqlItemStateBool(hasSockets, "has_sockets")
	if err != nil {
		return nil, err
	}
	sock0, err := sqlItemStateInt32(socket0, "socket0")
	if err != nil {
		return nil, err
	}
	sock1, err := sqlItemStateInt32(socket1, "socket1")
	if err != nil {
		return nil, err
	}
	sock2, err := sqlItemStateInt32(socket2, "socket2")
	if err != nil {
		return nil, err
	}
	if !present {
		if sock0 != 0 || sock1 != 0 || sock2 != 0 {
			return nil, fmt.Errorf("%w: non-zero sockets without has_sockets", ErrInvalidAccount)
		}
		return nil, nil
	}
	values := inventory.SocketValues{sock0, sock1, sock2}
	return &values, nil
}

func sqlItemStateAttributes(
	hasAttributes int64,
	attr0Type, attr0Value, attr1Type, attr1Value, attr2Type, attr2Value int64,
	attr3Type, attr3Value, attr4Type, attr4Value, attr5Type, attr5Value int64,
	attr6Type, attr6Value int64,
) (*inventory.AttributeValues, error) {
	present, err := sqlItemStateBool(hasAttributes, "has_attributes")
	if err != nil {
		return nil, err
	}
	types := []int64{attr0Type, attr1Type, attr2Type, attr3Type, attr4Type, attr5Type, attr6Type}
	values := []int64{attr0Value, attr1Value, attr2Value, attr3Value, attr4Value, attr5Value, attr6Value}
	var attrs inventory.AttributeValues
	for i := range attrs {
		attrType, err := sqlItemStateUint8(types[i], fmt.Sprintf("attr%d_type", i))
		if err != nil {
			return nil, err
		}
		attrValue, err := sqlItemStateInt16(values[i], fmt.Sprintf("attr%d_value", i))
		if err != nil {
			return nil, err
		}
		attrs[i] = inventory.Attribute{Type: attrType, Value: attrValue}
	}
	if !present {
		if attrs != (inventory.AttributeValues{}) {
			return nil, fmt.Errorf("%w: non-zero attributes without has_attributes", ErrInvalidAccount)
		}
		return nil, nil
	}
	copied := attrs
	return &copied, nil
}
