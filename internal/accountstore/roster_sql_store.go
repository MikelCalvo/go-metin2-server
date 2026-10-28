package accountstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

// SQLRosterStore is an opt-in tip-0002 account/character roster repository.
// The caller owns the executor, driver, migration and external DB backup.
// Only accounts and characters are read/written; child tables remain separate.
// Save replaces one account's roster atomically, never cascades child rows, and
// fails closed on FK/unique conflicts. Stock login/select still uses FileStore.
// This projection cannot persist item state, points, channels or login tickets.
type SQLRosterStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

func NewSQLRosterStore(executor dbmigrations.SQLMigrationExecutor) *SQLRosterStore {
	return &SQLRosterStore{executor: executor}
}

var (
	_ Store         = (*SQLRosterStore)(nil)
	_ AccountLister = (*SQLRosterStore)(nil)
)

func (s *SQLRosterStore) Load(login string) (Account, error) {
	if strings.TrimSpace(login) == "" {
		return Account{}, ErrLoginRequired
	}
	if login != strings.TrimSpace(login) || containsNUL(login) {
		return Account{}, ErrInvalidAccount
	}
	accounts, err := s.List()
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

func (s *SQLRosterStore) List() ([]Account, error) {
	export, err := s.ExportAccountCharacterRoster()
	if err != nil {
		return nil, err
	}
	accounts := make([]Account, len(export.Accounts))
	index := make(map[int64]int, len(accounts))
	for i, row := range export.Accounts {
		accounts[i] = Account{Login: row.Login, Empire: row.Empire, Characters: make([]loginticket.Character, accountCharacterRosterPlayerSlots)}
		index[row.ID] = i
	}
	for _, row := range export.Characters {
		character := loginticket.Character{
			ID: row.ID, Name: row.Name, Job: row.Job, RaceNum: row.RaceNum,
			Level: row.Level, PlayMinutes: row.PlayMinutes, ST: row.ST, HT: row.HT,
			DX: row.DX, IQ: row.IQ, MainPart: row.MainPart, ChangeName: row.ChangeName,
			HairPart: row.HairPart, X: row.X, Y: row.Y, Z: row.Z, MapIndex: row.MapIndex,
			Empire: row.Empire, SkillGroup: row.SkillGroup, GuildID: row.GuildID,
			GuildName: row.GuildName, Gold: row.Gold,
		}
		accounts[index[row.AccountID]].Characters[row.Slot] = character
	}
	for i := range accounts {
		accounts[i].Characters = normalizeAccountCharacters(accounts[i].Characters)
	}
	return accounts, nil
}

// ExportAccountCharacterRoster reads the committed SQL identity (not a
// re-derived FileStore hash) so imported account IDs survive Save and export.
func (s *SQLRosterStore) ExportAccountCharacterRoster() (AccountCharacterRosterExport, error) {
	if s == nil || rosterImportExecutorIsNil(s.executor) {
		return AccountCharacterRosterExport{}, ErrAccountCharacterRosterImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return AccountCharacterRosterExport{}, fmt.Errorf("begin roster SQL read: %w", err)
	}
	export, err := readSQLRoster(ctx, tx)
	if err != nil {
		return AccountCharacterRosterExport{}, rollbackAfterRosterImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return AccountCharacterRosterExport{}, fmt.Errorf("commit roster SQL read: %w", err)
	}
	return export, nil
}

// Save creates an absent account or replaces only the existing account matching
// login_normalized. The transaction checks the ledger before any write. It does
// not silently merge missing slots or delete another account, and never invokes
// the import's optional item-state cascade. FK dependents refuse replacement.
func (s *SQLRosterStore) Save(account Account) error {
	if s == nil || rosterImportExecutorIsNil(s.executor) {
		return ErrAccountCharacterRosterImportExecutorRequired
	}
	account.Characters = normalizeAccountCharacters(account.Characters)
	if err := validateRosterExportAccount(account); err != nil {
		return err
	}
	// A roster-only write must not silently discard state supplied by a full
	// FileStore snapshot. Empty slots carry normalized empty item slices.
	if len(account.CharacterChannels) != 0 {
		return fmt.Errorf("%w: roster SQL cannot persist channel assignments", ErrInvalidAccount)
	}
	for _, character := range account.Characters {
		if character.VID != 0 || character.Dummy != ([4]byte{}) || character.Points != ([255]int32{}) ||
			len(character.Inventory) != 0 || len(character.Equipment) != 0 || len(character.Quickslots) != 0 || len(character.MyShopUnitPrices) != 0 {
			return fmt.Errorf("%w: roster SQL cannot persist non-roster character state", ErrInvalidAccount)
		}
	}
	export, err := ExportAccountCharacterRoster([]Account{account})
	if err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin roster SQL save: %w", err)
	}
	if err := requireAccountCharacterRosterSchema(ctx, tx); err != nil {
		return rollbackAfterRosterImportFailure(tx, err)
	}
	row := export.Accounts[0]
	var storedID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE login_normalized = ?`, row.LoginNormalized).Scan(&storedID)
	switch {
	case err == nil:
		if storedID <= 0 {
			return rollbackAfterRosterImportFailure(tx, fmt.Errorf("%w: invalid stored account id", ErrInvalidAccount))
		}
		row.ID = storedID
		if err := deleteAccountCharacterRosterForAccount(ctx, tx, storedID); err != nil {
			return rollbackAfterRosterImportFailure(tx, err)
		}
	case err != sql.ErrNoRows:
		return rollbackAfterRosterImportFailure(tx, fmt.Errorf("lookup roster account: %w", err))
	}
	if err := insertAccountCharacterRosterAccount(ctx, tx, row); err != nil {
		return rollbackAfterRosterImportFailure(tx, err)
	}
	for _, character := range export.Characters {
		character.AccountID = row.ID
		if err := insertAccountCharacterRosterCharacter(ctx, tx, character); err != nil {
			return rollbackAfterRosterImportFailure(tx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit roster SQL save: %w", err)
	}
	return nil
}

func readSQLRoster(ctx context.Context, tx *sql.Tx) (AccountCharacterRosterExport, error) {
	empty := AccountCharacterRosterExport{MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName: AccountCharacterRosterMigrationName,
		Accounts:      []AccountCharacterRosterAccountRow{}, Characters: []AccountCharacterRosterCharacterRow{}}
	if err := requireAccountCharacterRosterSchema(ctx, tx); err != nil {
		return empty, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, login, login_normalized, empire FROM accounts ORDER BY login_normalized, id`)
	if err != nil {
		return empty, fmt.Errorf("query roster accounts: %w", err)
	}
	for rows.Next() {
		var id, empire int64
		var login, normalized string
		if err := rows.Scan(&id, &login, &normalized, &empire); err != nil {
			rows.Close()
			return empty, fmt.Errorf("scan roster account: %w", err)
		}
		value, err := sqlPointStateUint8(empire, "empire")
		if err != nil {
			rows.Close()
			return empty, err
		}
		empty.Accounts = append(empty.Accounts, AccountCharacterRosterAccountRow{ID: id, Login: login, LoginNormalized: normalized, Empire: value})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return empty, fmt.Errorf("iterate roster accounts: %w", err)
	}
	if err := rows.Close(); err != nil {
		return empty, fmt.Errorf("close roster accounts: %w", err)
	}

	rows, err = tx.QueryContext(ctx, `SELECT id, account_id, slot, name, name_normalized,
        job, race_num, level, play_minutes, st, ht, dx, iq, main_part, change_name,
        hair_part, x, y, z, map_index, empire, skill_group, guild_id, guild_name, gold
        FROM characters ORDER BY account_id, slot, id`)
	if err != nil {
		return empty, fmt.Errorf("query roster characters: %w", err)
	}
	for rows.Next() {
		var v [22]int64
		var name, normalized, guildName string
		// v holds the numeric fields in the SELECT order, with text scanned separately.
		if err := rows.Scan(&v[0], &v[1], &v[2], &name, &normalized,
			&v[3], &v[4], &v[5], &v[6], &v[7], &v[8], &v[9], &v[10],
			&v[11], &v[12], &v[13], &v[14], &v[15], &v[16], &v[17],
			&v[18], &v[19], &v[20], &guildName, &v[21]); err != nil {
			rows.Close()
			return empty, fmt.Errorf("scan roster character: %w", err)
		}
		row, err := sqlRosterCharacter(v, name, normalized, guildName)
		if err != nil {
			rows.Close()
			return empty, err
		}
		empty.Characters = append(empty.Characters, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return empty, fmt.Errorf("iterate roster characters: %w", err)
	}
	if err := rows.Close(); err != nil {
		return empty, fmt.Errorf("close roster characters: %w", err)
	}
	canonical, _, err := QuarantineAccountCharacterRosterExport(empty)
	return canonical, err
}

func sqlRosterCharacter(v [22]int64, name, normalized, guildName string) (AccountCharacterRosterCharacterRow, error) {
	var r AccountCharacterRosterCharacterRow
	var err error
	if r.ID, err = sqlPointStateUint32(v[0], "id"); err != nil {
		return r, err
	}
	r.AccountID = v[1]
	if v[2] < 0 || v[2] >= accountCharacterRosterPlayerSlots {
		return r, ErrInvalidAccount
	}
	r.Slot = int(v[2])
	r.Name, r.NameNormalized, r.GuildName = name, normalized, guildName
	for _, field := range []struct {
		value int64
		name  string
		dest  *uint8
	}{
		{v[3], "job", &r.Job}, {v[5], "level", &r.Level},
		{v[7], "st", &r.ST}, {v[8], "ht", &r.HT}, {v[9], "dx", &r.DX}, {v[10], "iq", &r.IQ},
		{v[12], "change_name", &r.ChangeName}, {v[18], "empire", &r.Empire},
		{v[19], "skill_group", &r.SkillGroup},
	} {
		if *field.dest, err = sqlPointStateUint8(field.value, field.name); err != nil {
			return r, err
		}
	}
	for _, field := range []struct {
		value int64
		name  string
		dest  *uint16
	}{
		{v[4], "race_num", &r.RaceNum}, {v[11], "main_part", &r.MainPart}, {v[13], "hair_part", &r.HairPart},
	} {
		if *field.dest, err = sqlItemStateUint16(field.value, field.name); err != nil {
			return r, err
		}
	}
	for _, field := range []struct {
		value int64
		name  string
		dest  *uint32
	}{
		{v[6], "play_minutes", &r.PlayMinutes}, {v[17], "map_index", &r.MapIndex}, {v[20], "guild_id", &r.GuildID},
	} {
		if *field.dest, err = sqlPointStateUint32(field.value, field.name); err != nil {
			return r, err
		}
	}
	for _, field := range []struct {
		value int64
		name  string
		dest  *int32
	}{
		{v[14], "x", &r.X}, {v[15], "y", &r.Y}, {v[16], "z", &r.Z},
	} {
		if *field.dest, err = sqlPointStateInt32(field.value, field.name); err != nil {
			return r, err
		}
	}
	if v[21] < 0 {
		return r, ErrInvalidAccount
	}
	r.Gold = uint64(v[21])
	return r, nil
}
