package safeboxstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

const (
	// MallSnapshotFilename is the committed mall JSON name in a sibling
	// directory of the safebox FileStore. Mall stays out of safebox.json so
	// safebox backup/restore empty-dir checks and warehouse password/money
	// rows are unchanged.
	MallSnapshotFilename = "mall.json"

	mallCrashTempPrefix = ".mall-"
	mallCrashTempSuffix = ".json"
)

var (
	ErrMallStorePathRequired = errors.New("mall store path is required")
)

// MallCharacterRow is the durable mall contents for one account login + character id.
// Password, warehouse money, cash-shop purchase, and NPC mall open stay out of this snapshot.
type MallCharacterRow struct {
	Login       string `json:"login"`
	CharacterID uint32 `json:"character_id"`
	Cells       []Cell `json:"cells"`
}

// MallSnapshot is the committed durable mall FileStore payload.
type MallSnapshot struct {
	Characters []MallCharacterRow `json:"characters"`
}

// MallStore is the Load/Save seam used by gamed mall rematerialize/persist.
type MallStore interface {
	Load() (MallSnapshot, error)
	Save(MallSnapshot) error
}

// MallFileStore persists durable mall cells beside the safebox FileStore.
type MallFileStore struct {
	path string
}

var (
	_ MallStore = (*MallFileStore)(nil)
)

// NewMallFileStore returns a mall FileStore rooted at path.
func NewMallFileStore(path string) *MallFileStore {
	return &MallFileStore{path: path}
}

// Path returns the configured mall snapshot path.
func (s *MallFileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// MallStorePathBesideSafebox derives a sibling mall.json path for a safebox
// FileStore path so the two stores never share a directory. Crash-temp prefixes
// therefore cannot collide, and safebox backup/restore empty-dir checks stay
// unchanged.
func MallStorePathBesideSafebox(safeboxPath string) string {
	safeboxPath = strings.TrimSpace(safeboxPath)
	if safeboxPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(safeboxPath)+"-mall", MallSnapshotFilename)
}

func (s *MallFileStore) syncStoreDir(dir string) error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return syncStoreDir(dir)
}

func (s *MallFileStore) syncFile(file *os.File) error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return file.Sync()
}

// Load reads and validates the committed durable mall snapshot.
func (s *MallFileStore) Load() (MallSnapshot, error) {
	if s == nil || s.path == "" {
		return MallSnapshot{}, ErrMallStorePathRequired
	}
	if err := rejectCommittedMallSnapshotSymlink(s.path); err != nil {
		return MallSnapshot{}, err
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MallSnapshot{}, ErrSnapshotNotFound
		}
		return MallSnapshot{}, fmt.Errorf("read mall snapshot: %w", err)
	}
	if !utf8.Valid(raw) {
		return MallSnapshot{}, fmt.Errorf("%w: decode mall snapshot: invalid utf-8", ErrInvalidSnapshot)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return MallSnapshot{}, fmt.Errorf("%w: decode mall snapshot: null root", ErrInvalidSnapshot)
	}

	var rawSnapshot struct {
		Characters json.RawMessage `json:"characters"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawSnapshot); err != nil {
		return MallSnapshot{}, fmt.Errorf("%w: decode mall snapshot: %v", ErrInvalidSnapshot, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return MallSnapshot{}, fmt.Errorf("%w: trailing mall snapshot content", ErrInvalidSnapshot)
	}

	var snapshot MallSnapshot
	if rawSnapshot.Characters != nil {
		if bytes.Equal(bytes.TrimSpace(rawSnapshot.Characters), []byte("null")) {
			return MallSnapshot{}, fmt.Errorf("%w: decode mall snapshot: null characters collection", ErrInvalidSnapshot)
		}
		collectionDecoder := json.NewDecoder(bytes.NewReader(rawSnapshot.Characters))
		collectionDecoder.DisallowUnknownFields()
		if err := collectionDecoder.Decode(&snapshot.Characters); err != nil {
			return MallSnapshot{}, fmt.Errorf("%w: decode mall snapshot: %v", ErrInvalidSnapshot, err)
		}
		if err := collectionDecoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return MallSnapshot{}, fmt.Errorf("%w: trailing mall characters content", ErrInvalidSnapshot)
		}
	}

	normalized := normalizeMallSnapshot(snapshot)
	if err := validateMallSnapshot(normalized); err != nil {
		return MallSnapshot{}, fmt.Errorf("%w: validate mall snapshot", err)
	}
	return normalized, nil
}

// Save writes the durable mall snapshot via crash-temp + rename + fsync.
func (s *MallFileStore) Save(snapshot MallSnapshot) error {
	if s == nil || s.path == "" {
		return ErrMallStorePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create mall store dir: %w", err)
	}
	normalized := normalizeMallSnapshot(snapshot)
	if err := validateMallSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate mall snapshot", err)
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode mall snapshot: %w", err)
	}
	raw = append(raw, '\n')

	temp, err := os.CreateTemp(filepath.Dir(s.path), mallCrashTempPrefix+"*"+mallCrashTempSuffix)
	if err != nil {
		return fmt.Errorf("create mall temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write mall snapshot: %w", err)
	}
	if err := s.syncFile(temp); err != nil {
		return fmt.Errorf("sync mall temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close mall temp file: %w", err)
	}
	storeDir := filepath.Dir(s.path)
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit mall snapshot: %w", err)
	}
	if err := s.syncStoreDir(storeDir); err != nil {
		return fmt.Errorf("sync mall store dir: %w", err)
	}
	return nil
}

// LoadMallOrEmpty returns the committed mall snapshot or an empty one when missing.
func LoadMallOrEmpty(store MallStore) (MallSnapshot, error) {
	if store == nil {
		return MallSnapshot{Characters: []MallCharacterRow{}}, nil
	}
	snapshot, err := store.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return MallSnapshot{Characters: []MallCharacterRow{}}, nil
		}
		return MallSnapshot{}, err
	}
	return snapshot, nil
}

// MallCharacterCells returns the durable mall cells for one login + character id.
func MallCharacterCells(snapshot MallSnapshot, login string, characterID uint32) map[uint8]inventory.ItemInstance {
	login = strings.TrimSpace(login)
	out := make(map[uint8]inventory.ItemInstance)
	for _, row := range normalizeMallSnapshot(snapshot).Characters {
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

// ReplaceMallCharacterCells upserts or removes one character row from the mall snapshot.
// An empty cells map removes the character row.
func ReplaceMallCharacterCells(snapshot MallSnapshot, login string, characterID uint32, cells map[uint8]inventory.ItemInstance) (MallSnapshot, error) {
	login = strings.TrimSpace(login)
	if !validLogin(login) || characterID == 0 {
		return MallSnapshot{}, ErrInvalidSnapshot
	}
	normalized := normalizeMallSnapshot(snapshot)
	filtered := make([]MallCharacterRow, 0, len(normalized.Characters))
	for _, row := range normalized.Characters {
		if row.Login == login && row.CharacterID == characterID {
			continue
		}
		filtered = append(filtered, row)
	}
	if len(cells) == 0 {
		return normalizeMallSnapshot(MallSnapshot{Characters: filtered}), nil
	}
	rowCells := make([]Cell, 0, len(cells))
	for cell, item := range cells {
		if uint8(item.Slot) != cell && item.Slot != 0 {
			// Prefer explicit map key; slot is rewritten to match cell.
		}
		rowCells = append(rowCells, cellFromItemInstance(cell, item))
	}
	filtered = append(filtered, MallCharacterRow{
		Login:       login,
		CharacterID: characterID,
		Cells:       rowCells,
	})
	next := normalizeMallSnapshot(MallSnapshot{Characters: filtered})
	if err := validateMallSnapshot(next); err != nil {
		return MallSnapshot{}, err
	}
	return next, nil
}

func normalizeMallSnapshot(snapshot MallSnapshot) MallSnapshot {
	normalized := MallSnapshot{Characters: cloneMallCharacterRows(snapshot.Characters)}
	if normalized.Characters == nil {
		normalized.Characters = []MallCharacterRow{}
	}
	for i := range normalized.Characters {
		normalized.Characters[i] = normalizeMallCharacterRow(normalized.Characters[i])
	}
	sort.Slice(normalized.Characters, func(i, j int) bool {
		if normalized.Characters[i].Login == normalized.Characters[j].Login {
			return normalized.Characters[i].CharacterID < normalized.Characters[j].CharacterID
		}
		return normalized.Characters[i].Login < normalized.Characters[j].Login
	})
	return normalized
}

func normalizeMallCharacterRow(row MallCharacterRow) MallCharacterRow {
	row.Login = strings.TrimSpace(row.Login)
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

func validateMallSnapshot(snapshot MallSnapshot) error {
	seenCharacters := make(map[string]struct{}, len(snapshot.Characters))
	seenItemIDs := make(map[uint64]struct{})
	for _, row := range snapshot.Characters {
		row = normalizeMallCharacterRow(row)
		if !validLogin(row.Login) || row.CharacterID == 0 {
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

func cloneMallCharacterRows(rows []MallCharacterRow) []MallCharacterRow {
	if len(rows) == 0 {
		return nil
	}
	cloned := make([]MallCharacterRow, len(rows))
	for i, row := range rows {
		cloned[i] = MallCharacterRow{
			Login:       row.Login,
			CharacterID: row.CharacterID,
			Cells:       cloneCells(row.Cells),
		}
	}
	return cloned
}

func rejectCommittedMallSnapshotSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat mall snapshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: mall snapshot %q is a symlink", ErrInvalidSnapshot, filepath.Base(path))
	}
	return nil
}
