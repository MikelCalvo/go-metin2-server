package safeboxstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

var (
	ErrStorePathRequired = errors.New("safebox store path is required")
	ErrSnapshotNotFound  = errors.New("safebox snapshot not found")
	ErrInvalidSnapshot   = errors.New("invalid safebox snapshot")
)

const (
	// MaxDurableCellExclusive is the exclusive upper bound for durable safebox
	// cells (bootstrap open size 1..3 × 5 cells/page). Open presentation still
	// capacity-gates mutations and SAFEBOX_SET emission to the currently opened
	// size; cells beyond that stay on disk for a later larger reopen.
	MaxDurableCellExclusive = 15

	safeboxCrashTempPrefix = ".safebox-"
	safeboxCrashTempSuffix = ".json"
)

// Cell is one durable safebox slot for a selected character.
// Presence-aware instance sockets mirror carried FileStore / tip-0003+0024 /
// ground-item FileStore: HasSockets=false / omitted means nil instance sockets
// (template fallback); HasSockets=true including all-zero is authoritative.
// Presence-aware instance attributes mirror the same rule beside sockets:
// HasAttributes=false / omitted means nil instance attributes (template
// fallback); HasAttributes=true including all-zero / type-zero is authoritative.
// Tip-0015 SQL attribute companions stay deferred.
type Cell struct {
	Cell          uint8                      `json:"cell"`
	ID            uint64                     `json:"id"`
	Vnum          uint32                     `json:"vnum"`
	Count         uint16                     `json:"count"`
	Locked        bool                       `json:"locked,omitempty"`
	HasSockets    bool                       `json:"has_sockets,omitempty"`
	Socket0       int32                      `json:"socket0,omitempty"`
	Socket1       int32                      `json:"socket1,omitempty"`
	Socket2       int32                      `json:"socket2,omitempty"`
	HasAttributes bool                       `json:"has_attributes,omitempty"`
	Attributes    *inventory.AttributeValues `json:"attributes,omitempty"`
}

const (
	// DefaultPassword is the bootstrap effective password when a character row
	// omits password or stores an empty string (legacy oracle default "000000").
	DefaultPassword = "000000"
	// MaxPasswordLen mirrors SAFEBOX_PASSWORD_MAX_LEN from the external oracle.
	MaxPasswordLen = 6
)

// CharacterRow is the durable safebox contents for one account login + character id.
type CharacterRow struct {
	Login       string `json:"login"`
	CharacterID uint32 `json:"character_id"`
	// Password is the optional durable safebox password. Omitted / empty means
	// DefaultPassword for challenge matching; persisted JSON omits the field
	// when empty so existing cell-only snapshots stay byte-compatible.
	Password string `json:"password,omitempty"`
	// Money is the optional durable warehouse gold. Omitted / zero means 0;
	// persisted JSON omits the field when zero so existing password/cell-only
	// snapshots stay byte-compatible. Store validation rejects negatives and
	// values above math.MaxInt32 (SAFEBOX_MONEY_CHANGE wire is signed int32).
	Money int64  `json:"money,omitempty"`
	Cells []Cell `json:"cells"`
}

// Snapshot is the committed durable safebox FileStore payload.
type Snapshot struct {
	Characters []CharacterRow `json:"characters"`
}

// SnapshotSummary is the operator/debug summary for Validate.
type SnapshotSummary struct {
	CharacterCount int      `json:"character_count"`
	CellCount      int      `json:"cell_count"`
	Logins         []string `json:"logins"`
	CharacterKeys  []string `json:"character_keys"`
	CrashTempCount int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles []string `json:"crash_temp_files,omitempty"`
}

// Store is the Load/Save seam used by gamed rematerialize/persist.
type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

// FileStore persists durable safebox cells beside other bootstrap JSON stores.
type FileStore struct {
	path string
}

var durableSyncDisabledForTest bool

// DisableDurableSyncForTest skips fsync calls until restore runs.
func DisableDurableSyncForTest() func() {
	previous := durableSyncDisabledForTest
	durableSyncDisabledForTest = true
	return func() { durableSyncDisabledForTest = previous }
}

// NewFileStore returns a FileStore rooted at path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Path returns the configured snapshot path.
func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *FileStore) syncStoreDir(dir string) error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return syncStoreDir(dir)
}

func (s *FileStore) syncFile(file *os.File) error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return file.Sync()
}

// NormalizeSnapshot returns the deterministic safebox snapshot shape.
func NormalizeSnapshot(snapshot Snapshot) Snapshot {
	return normalizeSnapshot(snapshot)
}

// ValidSnapshot reports whether the snapshot is well-formed after normalize.
func ValidSnapshot(snapshot Snapshot) bool {
	return validateSnapshot(normalizeSnapshot(snapshot)) == nil
}

// SummarizeSnapshot returns the operator summary for a valid snapshot.
func SummarizeSnapshot(snapshot Snapshot) (SnapshotSummary, error) {
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return SnapshotSummary{}, err
	}
	return summarizeSnapshot(normalized), nil
}

// Load reads and validates the committed durable safebox snapshot.
func (s *FileStore) Load() (Snapshot, error) {
	if s == nil || s.path == "" {
		return Snapshot{}, ErrStorePathRequired
	}
	if err := rejectCommittedSnapshotSymlink(s.path); err != nil {
		return Snapshot{}, err
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, ErrSnapshotNotFound
		}
		return Snapshot{}, fmt.Errorf("read safebox snapshot: %w", err)
	}
	if !utf8.Valid(raw) {
		return Snapshot{}, fmt.Errorf("%w: decode safebox snapshot: invalid utf-8", ErrInvalidSnapshot)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Snapshot{}, fmt.Errorf("%w: decode safebox snapshot: null root", ErrInvalidSnapshot)
	}

	var rawSnapshot struct {
		Characters json.RawMessage `json:"characters"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawSnapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode safebox snapshot: %v", ErrInvalidSnapshot, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Snapshot{}, fmt.Errorf("%w: trailing safebox snapshot content", ErrInvalidSnapshot)
	}

	var snapshot Snapshot
	if rawSnapshot.Characters != nil {
		if bytes.Equal(bytes.TrimSpace(rawSnapshot.Characters), []byte("null")) {
			return Snapshot{}, fmt.Errorf("%w: decode safebox snapshot: null characters collection", ErrInvalidSnapshot)
		}
		collectionDecoder := json.NewDecoder(bytes.NewReader(rawSnapshot.Characters))
		collectionDecoder.DisallowUnknownFields()
		if err := collectionDecoder.Decode(&snapshot.Characters); err != nil {
			return Snapshot{}, fmt.Errorf("%w: decode safebox snapshot: %v", ErrInvalidSnapshot, err)
		}
		if err := collectionDecoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return Snapshot{}, fmt.Errorf("%w: trailing safebox characters content", ErrInvalidSnapshot)
		}
	}

	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate safebox snapshot", err)
	}
	return normalized, nil
}

// Validate loads the committed snapshot when present and reports crash-temp residue.
func (s *FileStore) Validate() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	summary := SnapshotSummary{Logins: []string{}, CharacterKeys: []string{}}
	snapshot, err := s.Load()
	if err != nil {
		if !errors.Is(err, ErrSnapshotNotFound) {
			return SnapshotSummary{}, err
		}
	} else {
		summary = summarizeSnapshot(snapshot)
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	summary.CrashTempCount = len(crashTempFiles)
	summary.CrashTempFiles = crashTempFiles
	if err := s.validateActiveBackupManifest(); err != nil {
		return SnapshotSummary{}, err
	}
	return summary, nil
}

// CleanupCrashTempFiles removes hidden .safebox-*.json crash temps after Validate succeeds.
func (s *FileStore) CleanupCrashTempFiles() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	if _, err := s.Validate(); err != nil {
		return SnapshotSummary{}, err
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	if len(crashTempFiles) == 0 {
		return s.Validate()
	}
	storeDir := filepath.Dir(s.path)
	for _, filename := range crashTempFiles {
		if err := os.Remove(filepath.Join(storeDir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return SnapshotSummary{}, fmt.Errorf("remove safebox crash temp file %q: %w", filename, err)
		}
	}
	if err := s.syncStoreDir(storeDir); err != nil {
		return SnapshotSummary{}, fmt.Errorf("sync safebox store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

// Save writes the durable safebox snapshot via crash-temp + rename + fsync.
func (s *FileStore) Save(snapshot Snapshot) error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create safebox store dir: %w", err)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate safebox snapshot", err)
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode safebox snapshot: %w", err)
	}
	raw = append(raw, '\n')

	temp, err := os.CreateTemp(filepath.Dir(s.path), safeboxCrashTempPrefix+"*"+safeboxCrashTempSuffix)
	if err != nil {
		return fmt.Errorf("create safebox temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write safebox snapshot: %w", err)
	}
	if err := s.syncFile(temp); err != nil {
		return fmt.Errorf("sync safebox temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close safebox temp file: %w", err)
	}
	storeDir := filepath.Dir(s.path)
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit safebox snapshot: %w", err)
	}
	if err := removeBackupManifest(storeDir); err != nil {
		return fmt.Errorf("remove stale safebox backup manifest: %w", err)
	}
	if err := s.syncStoreDir(storeDir); err != nil {
		return fmt.Errorf("sync safebox store dir: %w", err)
	}
	return nil
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	return crashTempFilesInDir(filepath.Dir(s.path), filepath.Base(s.path))
}

// CharacterCells returns the durable cells for one login + character id as a map.
func CharacterCells(snapshot Snapshot, login string, characterID uint32) map[uint8]inventory.ItemInstance {
	login = strings.TrimSpace(login)
	out := make(map[uint8]inventory.ItemInstance)
	for _, row := range normalizeSnapshot(snapshot).Characters {
		if row.Login != login || row.CharacterID != characterID {
			continue
		}
		for _, cell := range row.Cells {
			out[cell.Cell] = inventory.ItemInstance{
				ID:         cell.ID,
				Vnum:       cell.Vnum,
				Count:      cell.Count,
				Slot:       inventory.SlotIndex(cell.Cell),
				Locked:     cell.Locked,
				Sockets:    cellInstanceSockets(cell),
				Attributes: cellInstanceAttributes(cell),
			}
		}
		return out
	}
	return out
}

// ReplaceCharacterCells upserts or removes one character row from the snapshot.
// An empty cells map removes the character row when password and money are also
// blank/zero; otherwise the password/money-only row is preserved.
func ReplaceCharacterCells(snapshot Snapshot, login string, characterID uint32, cells map[uint8]inventory.ItemInstance) (Snapshot, error) {
	login = strings.TrimSpace(login)
	if !validLogin(login) || characterID == 0 {
		return Snapshot{}, ErrInvalidSnapshot
	}
	normalized := normalizeSnapshot(snapshot)
	existingPassword := ""
	existingMoney := int64(0)
	filtered := make([]CharacterRow, 0, len(normalized.Characters))
	for _, row := range normalized.Characters {
		if row.Login == login && row.CharacterID == characterID {
			existingPassword = row.Password
			existingMoney = row.Money
			continue
		}
		filtered = append(filtered, row)
	}
	if len(cells) == 0 {
		if existingPassword == "" && existingMoney == 0 {
			return normalizeSnapshot(Snapshot{Characters: filtered}), nil
		}
		filtered = append(filtered, CharacterRow{
			Login:       login,
			CharacterID: characterID,
			Password:    existingPassword,
			Money:       existingMoney,
			Cells:       nil,
		})
		next := normalizeSnapshot(Snapshot{Characters: filtered})
		if err := validateSnapshot(next); err != nil {
			return Snapshot{}, err
		}
		return next, nil
	}
	rowCells := make([]Cell, 0, len(cells))
	for cell, item := range cells {
		if uint8(item.Slot) != cell && item.Slot != 0 {
			// Prefer explicit map key; slot is rewritten to match cell.
		}
		rowCells = append(rowCells, cellFromItemInstance(cell, item))
	}
	filtered = append(filtered, CharacterRow{
		Login:       login,
		CharacterID: characterID,
		Password:    existingPassword,
		Money:       existingMoney,
		Cells:       rowCells,
	})
	next := normalizeSnapshot(Snapshot{Characters: filtered})
	if err := validateSnapshot(next); err != nil {
		return Snapshot{}, err
	}
	return next, nil
}

// CharacterPassword returns the durable password for one login + character id.
// Missing rows and blank stored passwords resolve to DefaultPassword.
func CharacterPassword(snapshot Snapshot, login string, characterID uint32) string {
	login = strings.TrimSpace(login)
	for _, row := range normalizeSnapshot(snapshot).Characters {
		if row.Login == login && row.CharacterID == characterID {
			return EffectivePassword(row.Password)
		}
	}
	return DefaultPassword
}

// CharacterMoney returns the durable warehouse gold for one login + character id.
// Missing rows resolve to 0.
func CharacterMoney(snapshot Snapshot, login string, characterID uint32) int64 {
	login = strings.TrimSpace(login)
	for _, row := range normalizeSnapshot(snapshot).Characters {
		if row.Login == login && row.CharacterID == characterID {
			return row.Money
		}
	}
	return 0
}

// EffectivePassword returns the challenge password for a stored value.
func EffectivePassword(password string) string {
	if strings.TrimSpace(password) == "" {
		return DefaultPassword
	}
	return password
}

// ReplaceCharacterPassword upserts password for one character row while
// preserving durable cells and money. Missing rows create an empty-cells row.
func ReplaceCharacterPassword(snapshot Snapshot, login string, characterID uint32, password string) (Snapshot, error) {
	login = strings.TrimSpace(login)
	if !validLogin(login) || characterID == 0 {
		return Snapshot{}, ErrInvalidSnapshot
	}
	if !validPassword(password) {
		return Snapshot{}, ErrInvalidSnapshot
	}
	normalized := normalizeSnapshot(snapshot)
	cells := CharacterCells(normalized, login, characterID)
	existingMoney := CharacterMoney(normalized, login, characterID)
	rowCells := make([]Cell, 0, len(cells))
	for cell, item := range cells {
		rowCells = append(rowCells, cellFromItemInstance(cell, item))
	}
	filtered := make([]CharacterRow, 0, len(normalized.Characters)+1)
	for _, row := range normalized.Characters {
		if row.Login == login && row.CharacterID == characterID {
			continue
		}
		filtered = append(filtered, row)
	}
	filtered = append(filtered, CharacterRow{
		Login:       login,
		CharacterID: characterID,
		Password:    password,
		Money:       existingMoney,
		Cells:       rowCells,
	})
	next := normalizeSnapshot(Snapshot{Characters: filtered})
	if err := validateSnapshot(next); err != nil {
		return Snapshot{}, err
	}
	return next, nil
}

// ReplaceCharacterMoney upserts warehouse gold for one character row while
// preserving durable password and cells. Missing rows create an empty-cells row.
func ReplaceCharacterMoney(snapshot Snapshot, login string, characterID uint32, money int64) (Snapshot, error) {
	login = strings.TrimSpace(login)
	if !validLogin(login) || characterID == 0 || !validMoney(money) {
		return Snapshot{}, ErrInvalidSnapshot
	}
	normalized := normalizeSnapshot(snapshot)
	cells := CharacterCells(normalized, login, characterID)
	existingPassword := ""
	for _, row := range normalized.Characters {
		if row.Login == login && row.CharacterID == characterID {
			existingPassword = row.Password
			break
		}
	}
	rowCells := make([]Cell, 0, len(cells))
	for cell, item := range cells {
		rowCells = append(rowCells, cellFromItemInstance(cell, item))
	}
	filtered := make([]CharacterRow, 0, len(normalized.Characters)+1)
	for _, row := range normalized.Characters {
		if row.Login == login && row.CharacterID == characterID {
			continue
		}
		filtered = append(filtered, row)
	}
	if money == 0 && existingPassword == "" && len(rowCells) == 0 {
		return normalizeSnapshot(Snapshot{Characters: filtered}), nil
	}
	filtered = append(filtered, CharacterRow{
		Login:       login,
		CharacterID: characterID,
		Password:    existingPassword,
		Money:       money,
		Cells:       rowCells,
	})
	next := normalizeSnapshot(Snapshot{Characters: filtered})
	if err := validateSnapshot(next); err != nil {
		return Snapshot{}, err
	}
	return next, nil
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := Snapshot{Characters: cloneCharacterRows(snapshot.Characters)}
	if normalized.Characters == nil {
		normalized.Characters = []CharacterRow{}
	}
	for i := range normalized.Characters {
		normalized.Characters[i] = normalizeCharacterRow(normalized.Characters[i])
	}
	sort.Slice(normalized.Characters, func(i, j int) bool {
		if normalized.Characters[i].Login == normalized.Characters[j].Login {
			return normalized.Characters[i].CharacterID < normalized.Characters[j].CharacterID
		}
		return normalized.Characters[i].Login < normalized.Characters[j].Login
	})
	return normalized
}

func normalizeCharacterRow(row CharacterRow) CharacterRow {
	row.Login = strings.TrimSpace(row.Login)
	row.Password = strings.TrimSpace(row.Password)
	row.Cells = cloneCells(row.Cells)
	if row.Cells == nil {
		row.Cells = []Cell{}
	}
	for i := range row.Cells {
		row.Cells[i] = normalizeCell(row.Cells[i])
	}
	sort.Slice(row.Cells, func(i, j int) bool {
		return row.Cells[i].Cell < row.Cells[j].Cell
	})
	return row
}

func normalizeCell(cell Cell) Cell {
	if cell.HasAttributes {
		if cell.Attributes == nil {
			zero := inventory.AttributeValues{}
			cell.Attributes = &zero
		} else {
			copied := *cell.Attributes
			cell.Attributes = &copied
		}
	} else if safeboxAttributesNonZero(cell.Attributes) {
		// Keep the non-zero payload so validation can fail closed; do not
		// silently coerce malformed presence into template fallback.
		copied := *cell.Attributes
		cell.Attributes = &copied
	} else {
		cell.Attributes = nil
	}
	return cell
}

func summarizeSnapshot(snapshot Snapshot) SnapshotSummary {
	summary := SnapshotSummary{
		CharacterCount: len(snapshot.Characters),
		Logins:         []string{},
		CharacterKeys:  make([]string, 0, len(snapshot.Characters)),
	}
	loginsSeen := make(map[string]struct{})
	for _, row := range normalizeSnapshot(snapshot).Characters {
		summary.CellCount += len(row.Cells)
		summary.CharacterKeys = append(summary.CharacterKeys, characterKey(row.Login, row.CharacterID))
		loginsSeen[row.Login] = struct{}{}
	}
	for login := range loginsSeen {
		summary.Logins = append(summary.Logins, login)
	}
	sort.Strings(summary.Logins)
	return summary
}

func validateSnapshot(snapshot Snapshot) error {
	seenCharacters := make(map[string]struct{}, len(snapshot.Characters))
	seenItemIDs := make(map[uint64]struct{})
	for _, row := range snapshot.Characters {
		row = normalizeCharacterRow(row)
		if !validLogin(row.Login) || row.CharacterID == 0 || !validPassword(row.Password) || !validMoney(row.Money) {
			return ErrInvalidSnapshot
		}
		key := characterKey(row.Login, row.CharacterID)
		if _, ok := seenCharacters[key]; ok {
			return ErrInvalidSnapshot
		}
		seenCharacters[key] = struct{}{}
		seenCells := make(map[uint8]struct{}, len(row.Cells))
		for _, cell := range row.Cells {
			if cell.Cell >= MaxDurableCellExclusive {
				return ErrInvalidSnapshot
			}
			if _, ok := seenCells[cell.Cell]; ok {
				return ErrInvalidSnapshot
			}
			seenCells[cell.Cell] = struct{}{}
			if err := validateCellInstanceSockets(cell); err != nil {
				return err
			}
			if err := validateCellInstanceAttributes(cell); err != nil {
				return err
			}
			item := inventory.ItemInstance{
				ID:         cell.ID,
				Vnum:       cell.Vnum,
				Count:      cell.Count,
				Slot:       inventory.SlotIndex(cell.Cell),
				Locked:     cell.Locked,
				Sockets:    cellInstanceSockets(cell),
				Attributes: cellInstanceAttributes(cell),
			}
			if err := item.Validate(); err != nil {
				return ErrInvalidSnapshot
			}
			if item.Equipped {
				return ErrInvalidSnapshot
			}
			if _, ok := seenItemIDs[cell.ID]; ok {
				return ErrInvalidSnapshot
			}
			seenItemIDs[cell.ID] = struct{}{}
		}
	}
	return nil
}

func validLogin(login string) bool {
	login = strings.TrimSpace(login)
	if login == "" || !utf8.ValidString(login) || strings.ContainsRune(login, '\x00') {
		return false
	}
	for _, r := range login {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validPassword(password string) bool {
	if password == "" {
		return true
	}
	if !utf8.ValidString(password) || strings.ContainsRune(password, '\x00') {
		return false
	}
	if len(password) > MaxPasswordLen {
		return false
	}
	for _, r := range password {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func validMoney(money int64) bool {
	return money >= 0 && money <= math.MaxInt32
}

func characterKey(login string, characterID uint32) string {
	return strings.TrimSpace(login) + ":" + fmt.Sprintf("%d", characterID)
}

func cloneCharacterRows(rows []CharacterRow) []CharacterRow {
	if len(rows) == 0 {
		return nil
	}
	cloned := make([]CharacterRow, len(rows))
	for i, row := range rows {
		cloned[i] = CharacterRow{
			Login:       row.Login,
			CharacterID: row.CharacterID,
			Password:    row.Password,
			Money:       row.Money,
			Cells:       cloneCells(row.Cells),
		}
	}
	return cloned
}

func cloneCells(cells []Cell) []Cell {
	if len(cells) == 0 {
		return nil
	}
	cloned := make([]Cell, len(cells))
	copy(cloned, cells)
	for i := range cloned {
		if cells[i].Attributes != nil {
			copied := *cells[i].Attributes
			cloned[i].Attributes = &copied
		}
	}
	return cloned
}

func cellFromItemInstance(cell uint8, item inventory.ItemInstance) Cell {
	out := Cell{
		Cell:   cell,
		ID:     item.ID,
		Vnum:   item.Vnum,
		Count:  item.Count,
		Locked: item.Locked,
	}
	if item.HasSockets() {
		values := *item.Sockets
		out.HasSockets = true
		out.Socket0 = values[0]
		out.Socket1 = values[1]
		out.Socket2 = values[2]
	}
	if item.HasAttributes() {
		values := *item.Attributes
		out.HasAttributes = true
		out.Attributes = &values
	}
	return out
}

func cellInstanceSockets(cell Cell) *inventory.SocketValues {
	if !cell.HasSockets {
		return nil
	}
	values := inventory.SocketValues{cell.Socket0, cell.Socket1, cell.Socket2}
	return &values
}

func cellInstanceAttributes(cell Cell) *inventory.AttributeValues {
	if !cell.HasAttributes {
		return nil
	}
	values := inventory.AttributeValues{}
	if cell.Attributes != nil {
		values = *cell.Attributes
	}
	return &values
}

func validateCellInstanceSockets(cell Cell) error {
	if cell.HasSockets {
		return nil
	}
	if cell.Socket0 != 0 || cell.Socket1 != 0 || cell.Socket2 != 0 {
		return fmt.Errorf("%w: safebox cell %d has non-zero sockets without has_sockets", ErrInvalidSnapshot, cell.Cell)
	}
	return nil
}

func validateCellInstanceAttributes(cell Cell) error {
	if cell.HasAttributes {
		return nil
	}
	if safeboxAttributesNonZero(cell.Attributes) {
		return fmt.Errorf("%w: safebox cell %d has non-zero attributes without has_attributes", ErrInvalidSnapshot, cell.Cell)
	}
	return nil
}

func safeboxAttributesNonZero(attributes *inventory.AttributeValues) bool {
	if attributes == nil {
		return false
	}
	for _, attribute := range *attributes {
		if attribute.Type != 0 || attribute.Value != 0 {
			return true
		}
	}
	return false
}

func rejectCommittedSnapshotSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat safebox snapshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: safebox snapshot %q is a symlink", ErrInvalidSnapshot, filepath.Base(path))
	}
	return nil
}

var syncStoreDir = syncDir

func syncDir(path string) error {
	if durableSyncDisabledForTest {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
