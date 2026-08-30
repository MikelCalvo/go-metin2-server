package worldruntime

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
	"time"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

// Durable pending ground-item FileStore for process-restart rematerialization.
// This is intentionally separate from MemoryGroundItemStore (hermetic export seam)
// and from the 0010 migration export (which omits item IDs and absolute timers).

var (
	ErrGroundItemStorePathRequired = errors.New("ground item store path is required")
	ErrGroundItemSnapshotNotFound  = errors.New("ground item snapshot not found")
	ErrInvalidGroundItemSnapshot   = errors.New("invalid ground item snapshot")
)

const (
	groundItemCrashTempPrefix = ".ground-items-"
	groundItemCrashTempSuffix = ".json"
)

// DurableGroundItemRecord is the file-backed pending ground handle shape used for
// process restart. It extends the 0010 migration projection with the minimum
// runtime fields needed for safe rematerialize (stable item id + absolute timers).
// Presence-aware instance sockets mirror carried FileStore / tip-0003+0024:
// HasSockets=false / omitted means nil instance sockets (template fallback);
// HasSockets=true including all-zero is authoritative. Presence-aware instance
// attributes mirror the same rule beside sockets: HasAttributes=false / omitted
// means nil instance attributes (template fallback); HasAttributes=true including
// all-zero / type-zero is authoritative. Gold-shaped rows stay socket-less and
// attribute-less. Tip-0010 SQL attribute companions stay deferred, so the 0010
// projection still omits attribute fields.
type DurableGroundItemRecord struct {
	VID                uint32                     `json:"vid"`
	Vnum               uint32                     `json:"vnum"`
	ItemCount          *uint16                    `json:"item_count,omitempty"`
	GoldAmount         *uint32                    `json:"gold_amount,omitempty"`
	ItemID             uint64                     `json:"item_id,omitempty"`
	HasSockets         bool                       `json:"has_sockets,omitempty"`
	Socket0            int32                      `json:"socket0,omitempty"`
	Socket1            int32                      `json:"socket1,omitempty"`
	Socket2            int32                      `json:"socket2,omitempty"`
	HasAttributes      bool                       `json:"has_attributes,omitempty"`
	Attributes         *inventory.AttributeValues `json:"attributes,omitempty"`
	OwnerLogin         string                     `json:"owner_login"`
	OwnerCharacterID   uint32                     `json:"owner_character_id"`
	OwnerVID           uint32                     `json:"owner_vid"`
	OwnerName          string                     `json:"owner_name"`
	MapIndex           uint32                     `json:"map_index"`
	X                  int32                      `json:"x"`
	Y                  int32                      `json:"y"`
	Z                  int32                      `json:"z"`
	PickupRange        int64                      `json:"pickup_range"`
	OwnershipExclusive bool                       `json:"ownership_exclusive"`
	OwnershipExpiresAt *time.Time                 `json:"ownership_expires_at,omitempty"`
	DespawnAt          time.Time                  `json:"despawn_at"`
}

// DurableGroundItemSnapshot is the committed pending-ground FileStore payload.
type DurableGroundItemSnapshot struct {
	GroundItems []DurableGroundItemRecord `json:"ground_items"`
}

// DurableGroundItemSnapshotSummary is the operator/debug summary for Validate.
type DurableGroundItemSnapshotSummary struct {
	GroundItemCount int      `json:"ground_item_count"`
	ItemShapedCount int      `json:"item_shaped_count"`
	GoldShapedCount int      `json:"gold_shaped_count"`
	VIDs            []uint32 `json:"vids"`
	CrashTempCount  int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles  []string `json:"crash_temp_files,omitempty"`
}

// GroundItemStore is the Load/Save seam used by gamed rematerialize/persist.
type GroundItemStore interface {
	Load() (DurableGroundItemSnapshot, error)
	Save(DurableGroundItemSnapshot) error
}

// FileStore persists durable pending ground handles beside other bootstrap JSON stores.
type FileStore struct {
	path string
}

var durableGroundItemSyncDisabledForTest bool

// DisableDurableGroundItemSyncForTest skips fsync calls until restore runs.
func DisableDurableGroundItemSyncForTest() func() {
	previous := durableGroundItemSyncDisabledForTest
	durableGroundItemSyncDisabledForTest = true
	return func() { durableGroundItemSyncDisabledForTest = previous }
}

// NewGroundItemFileStore returns a FileStore rooted at path.
func NewGroundItemFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Path returns the configured snapshot path.
func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load reads and validates the committed durable ground-item snapshot.
func (s *FileStore) Load() (DurableGroundItemSnapshot, error) {
	if s == nil || s.path == "" {
		return DurableGroundItemSnapshot{}, ErrGroundItemStorePathRequired
	}
	if err := rejectGroundItemCommittedSnapshotSymlink(s.path); err != nil {
		return DurableGroundItemSnapshot{}, err
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DurableGroundItemSnapshot{}, ErrGroundItemSnapshotNotFound
		}
		return DurableGroundItemSnapshot{}, fmt.Errorf("read ground item snapshot: %w", err)
	}
	if !utf8.Valid(raw) {
		return DurableGroundItemSnapshot{}, fmt.Errorf("%w: decode ground item snapshot: invalid utf-8", ErrInvalidGroundItemSnapshot)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return DurableGroundItemSnapshot{}, fmt.Errorf("%w: decode ground item snapshot: null root", ErrInvalidGroundItemSnapshot)
	}

	var rawSnapshot struct {
		GroundItems json.RawMessage `json:"ground_items"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawSnapshot); err != nil {
		return DurableGroundItemSnapshot{}, fmt.Errorf("%w: decode ground item snapshot: %v", ErrInvalidGroundItemSnapshot, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return DurableGroundItemSnapshot{}, fmt.Errorf("%w: trailing ground item snapshot content", ErrInvalidGroundItemSnapshot)
	}

	var snapshot DurableGroundItemSnapshot
	if rawSnapshot.GroundItems != nil {
		if bytes.Equal(bytes.TrimSpace(rawSnapshot.GroundItems), []byte("null")) {
			return DurableGroundItemSnapshot{}, fmt.Errorf("%w: decode ground item snapshot: null ground_items collection", ErrInvalidGroundItemSnapshot)
		}
		collectionDecoder := json.NewDecoder(bytes.NewReader(rawSnapshot.GroundItems))
		collectionDecoder.DisallowUnknownFields()
		if err := collectionDecoder.Decode(&snapshot.GroundItems); err != nil {
			return DurableGroundItemSnapshot{}, fmt.Errorf("%w: decode ground item snapshot: %v", ErrInvalidGroundItemSnapshot, err)
		}
		if err := collectionDecoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return DurableGroundItemSnapshot{}, fmt.Errorf("%w: trailing ground_items content", ErrInvalidGroundItemSnapshot)
		}
	}

	normalized := NormalizeDurableGroundItemSnapshot(snapshot)
	if err := ValidateDurableGroundItemSnapshot(normalized); err != nil {
		return DurableGroundItemSnapshot{}, fmt.Errorf("%w: validate ground item snapshot", err)
	}
	return normalized, nil
}

// Validate loads the committed snapshot when present and reports crash-temp residue.
func (s *FileStore) Validate() (DurableGroundItemSnapshotSummary, error) {
	if s == nil || s.path == "" {
		return DurableGroundItemSnapshotSummary{}, ErrGroundItemStorePathRequired
	}
	summary := DurableGroundItemSnapshotSummary{VIDs: []uint32{}}
	snapshot, err := s.Load()
	if err != nil {
		if !errors.Is(err, ErrGroundItemSnapshotNotFound) {
			return DurableGroundItemSnapshotSummary{}, err
		}
	} else {
		summary = SummarizeDurableGroundItemSnapshot(snapshot)
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return DurableGroundItemSnapshotSummary{}, err
	}
	summary.CrashTempCount = len(crashTempFiles)
	summary.CrashTempFiles = crashTempFiles
	if err := s.validateActiveBackupManifest(); err != nil {
		return DurableGroundItemSnapshotSummary{}, err
	}
	return summary, nil
}

// CleanupCrashTempFiles removes hidden .ground-items-*.json crash temps after Validate succeeds.
func (s *FileStore) CleanupCrashTempFiles() (DurableGroundItemSnapshotSummary, error) {
	if s == nil || s.path == "" {
		return DurableGroundItemSnapshotSummary{}, ErrGroundItemStorePathRequired
	}
	if _, err := s.Validate(); err != nil {
		return DurableGroundItemSnapshotSummary{}, err
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return DurableGroundItemSnapshotSummary{}, err
	}
	if len(crashTempFiles) == 0 {
		return s.Validate()
	}
	storeDir := filepath.Dir(s.path)
	for _, filename := range crashTempFiles {
		if err := os.Remove(filepath.Join(storeDir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return DurableGroundItemSnapshotSummary{}, fmt.Errorf("remove ground item crash temp file %q: %w", filename, err)
		}
	}
	if err := syncGroundItemStoreDir(storeDir); err != nil {
		return DurableGroundItemSnapshotSummary{}, fmt.Errorf("sync ground item store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

// Save atomically commits a validated durable ground-item snapshot.
func (s *FileStore) Save(snapshot DurableGroundItemSnapshot) error {
	if s == nil || s.path == "" {
		return ErrGroundItemStorePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create ground item store dir: %w", err)
	}
	normalized := NormalizeDurableGroundItemSnapshot(snapshot)
	if err := ValidateDurableGroundItemSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate ground item snapshot", err)
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ground item snapshot: %w", err)
	}
	raw = append(raw, '\n')

	temp, err := os.CreateTemp(filepath.Dir(s.path), groundItemCrashTempPrefix+"*"+groundItemCrashTempSuffix)
	if err != nil {
		return fmt.Errorf("create ground item temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write ground item snapshot: %w", err)
	}
	if !durableGroundItemSyncDisabledForTest {
		if err := temp.Sync(); err != nil {
			return fmt.Errorf("sync ground item temp file: %w", err)
		}
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close ground item temp file: %w", err)
	}
	storeDir := filepath.Dir(s.path)
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit ground item snapshot: %w", err)
	}
	if err := removeBackupManifest(storeDir); err != nil {
		return fmt.Errorf("remove stale ground item backup manifest: %w", err)
	}
	if !durableGroundItemSyncDisabledForTest {
		if err := syncGroundItemStoreDir(storeDir); err != nil {
			return fmt.Errorf("sync ground item store dir: %w", err)
		}
	}
	return nil
}

// ExportBootstrapGroundItemState projects durable records onto the 0010 migration
// shape without timers or item IDs.
func (s *FileStore) ExportBootstrapGroundItemState() (BootstrapGroundItemStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrGroundItemSnapshotNotFound) {
			return ExportBootstrapGroundItemState(nil)
		}
		return BootstrapGroundItemStateExport{}, err
	}
	return ExportBootstrapGroundItemState(DurableGroundItemRecordsToSnapshots(snapshot.GroundItems))
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	dir := filepath.Dir(s.path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read ground item store crash temp files: %w", err)
	}
	committed := filepath.Base(s.path)
	files := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == committed {
			continue
		}
		if strings.HasPrefix(name, groundItemCrashTempPrefix) && strings.HasSuffix(name, groundItemCrashTempSuffix) {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%w: ground item crash temp file %q is a symlink", ErrInvalidGroundItemSnapshot, name)
			}
			if entry.IsDir() {
				continue
			}
			files = append(files, name)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, nil
	}
	return files, nil
}

// NormalizeDurableGroundItemSnapshot clones and sorts records by ascending VID.
func NormalizeDurableGroundItemSnapshot(snapshot DurableGroundItemSnapshot) DurableGroundItemSnapshot {
	normalized := DurableGroundItemSnapshot{GroundItems: cloneDurableGroundItemRecords(snapshot.GroundItems)}
	if normalized.GroundItems == nil {
		normalized.GroundItems = []DurableGroundItemRecord{}
	}
	for i := range normalized.GroundItems {
		normalized.GroundItems[i] = normalizeDurableGroundItemRecord(normalized.GroundItems[i])
	}
	sort.Slice(normalized.GroundItems, func(i, j int) bool {
		return normalized.GroundItems[i].VID < normalized.GroundItems[j].VID
	})
	return normalized
}

// ValidateDurableGroundItemSnapshot fails closed against the durable restart contract.
func ValidateDurableGroundItemSnapshot(snapshot DurableGroundItemSnapshot) error {
	seen := make(map[uint32]struct{}, len(snapshot.GroundItems))
	for _, record := range snapshot.GroundItems {
		if err := validateDurableGroundItemRecord(record); err != nil {
			return err
		}
		if _, ok := seen[record.VID]; ok {
			return fmt.Errorf("%w: duplicate ground vid %d", ErrInvalidGroundItemSnapshot, record.VID)
		}
		seen[record.VID] = struct{}{}
	}
	return nil
}

// SummarizeDurableGroundItemSnapshot returns deterministic operator metadata.
func SummarizeDurableGroundItemSnapshot(snapshot DurableGroundItemSnapshot) DurableGroundItemSnapshotSummary {
	normalized := NormalizeDurableGroundItemSnapshot(snapshot)
	summary := DurableGroundItemSnapshotSummary{
		GroundItemCount: len(normalized.GroundItems),
		VIDs:            make([]uint32, 0, len(normalized.GroundItems)),
	}
	for _, record := range normalized.GroundItems {
		summary.VIDs = append(summary.VIDs, record.VID)
		if record.GoldAmount != nil {
			summary.GoldShapedCount++
		} else {
			summary.ItemShapedCount++
		}
	}
	return summary
}

// FilterDurableGroundItemSnapshotForRestore drops despawned rows and publicizes
// exclusive ownership whose absolute expiry has already passed.
func FilterDurableGroundItemSnapshotForRestore(snapshot DurableGroundItemSnapshot, now time.Time) DurableGroundItemSnapshot {
	normalized := NormalizeDurableGroundItemSnapshot(snapshot)
	now = now.UTC()
	kept := make([]DurableGroundItemRecord, 0, len(normalized.GroundItems))
	for _, record := range normalized.GroundItems {
		if !record.DespawnAt.After(now) {
			continue
		}
		if record.OwnershipExclusive {
			if record.OwnershipExpiresAt == nil || !record.OwnershipExpiresAt.After(now) {
				record.OwnershipExclusive = false
				record.OwnershipExpiresAt = nil
			}
		} else {
			record.OwnershipExpiresAt = nil
		}
		kept = append(kept, record)
	}
	return DurableGroundItemSnapshot{GroundItems: kept}
}

// DurableGroundItemRecordsToSnapshots projects durable records onto operator snapshots.
func DurableGroundItemRecordsToSnapshots(records []DurableGroundItemRecord) []GroundItemSnapshot {
	out := make([]GroundItemSnapshot, 0, len(records))
	for _, record := range records {
		snapshot := GroundItemSnapshot{
			VID:              record.VID,
			Vnum:             record.Vnum,
			OwnerName:        record.OwnerName,
			OwnerLogin:       record.OwnerLogin,
			OwnerCharacterID: record.OwnerCharacterID,
			OwnerVID:         record.OwnerVID,
			PickupRange:      record.PickupRange,
			MapIndex:         record.MapIndex,
			X:                record.X,
			Y:                record.Y,
			Z:                record.Z,
			HasSockets:       record.HasSockets,
			Socket0:          record.Socket0,
			Socket1:          record.Socket1,
			Socket2:          record.Socket2,
		}
		if record.GoldAmount != nil {
			snapshot.GoldAmount = *record.GoldAmount
			snapshot.HasSockets = false
			snapshot.Socket0 = 0
			snapshot.Socket1 = 0
			snapshot.Socket2 = 0
		} else if record.ItemCount != nil {
			snapshot.Count = *record.ItemCount
		}
		out = append(out, snapshot)
	}
	return out
}

func normalizeDurableGroundItemRecord(record DurableGroundItemRecord) DurableGroundItemRecord {
	record.OwnerLogin = strings.TrimSpace(record.OwnerLogin)
	record.OwnerName = strings.TrimSpace(record.OwnerName)
	if record.OwnershipExpiresAt != nil {
		utc := record.OwnershipExpiresAt.UTC()
		record.OwnershipExpiresAt = &utc
	}
	if !record.DespawnAt.IsZero() {
		record.DespawnAt = record.DespawnAt.UTC()
	}
	if !record.OwnershipExclusive {
		record.OwnershipExpiresAt = nil
	}
	if record.HasAttributes {
		if record.Attributes == nil {
			zero := inventory.AttributeValues{}
			record.Attributes = &zero
		} else {
			copied := *record.Attributes
			record.Attributes = &copied
		}
	} else if durableGroundItemAttributesNonZero(record.Attributes) {
		// Keep the non-zero payload so validation can fail closed; do not
		// silently coerce malformed presence into template fallback.
		copied := *record.Attributes
		record.Attributes = &copied
	} else {
		record.Attributes = nil
	}
	return record
}

func validateDurableGroundItemRecord(record DurableGroundItemRecord) error {
	record = normalizeDurableGroundItemRecord(record)
	if record.VID == 0 {
		return fmt.Errorf("%w: ground vid must be positive", ErrInvalidGroundItemSnapshot)
	}
	if record.Vnum == 0 {
		return fmt.Errorf("%w: ground vid %d has zero vnum", ErrInvalidGroundItemSnapshot, record.VID)
	}
	if !validBootstrapGroundOwnerMetadata(record.OwnerLogin) {
		return fmt.Errorf("%w: ground vid %d has invalid owner login", ErrInvalidGroundItemSnapshot, record.VID)
	}
	if !validBootstrapGroundOwnerMetadata(record.OwnerName) || len(record.OwnerName) > bootstrapGroundItemOwnerNameMaxBytes {
		return fmt.Errorf("%w: ground vid %d has invalid owner name", ErrInvalidGroundItemSnapshot, record.VID)
	}
	if record.OwnerCharacterID == 0 {
		return fmt.Errorf("%w: ground vid %d has zero owner character id", ErrInvalidGroundItemSnapshot, record.VID)
	}
	if record.OwnerVID == 0 {
		return fmt.Errorf("%w: ground vid %d has zero owner vid", ErrInvalidGroundItemSnapshot, record.VID)
	}
	if record.MapIndex == 0 {
		return fmt.Errorf("%w: ground vid %d has zero map index", ErrInvalidGroundItemSnapshot, record.VID)
	}
	if record.PickupRange <= 0 {
		return fmt.Errorf("%w: ground vid %d has non-positive pickup range", ErrInvalidGroundItemSnapshot, record.VID)
	}
	if record.DespawnAt.IsZero() {
		return fmt.Errorf("%w: ground vid %d missing despawn_at", ErrInvalidGroundItemSnapshot, record.VID)
	}
	if record.OwnershipExclusive {
		if record.OwnershipExpiresAt == nil || record.OwnershipExpiresAt.IsZero() {
			return fmt.Errorf("%w: ground vid %d exclusive ownership missing ownership_expires_at", ErrInvalidGroundItemSnapshot, record.VID)
		}
		if !record.OwnershipExpiresAt.Before(record.DespawnAt) && !record.OwnershipExpiresAt.Equal(record.DespawnAt) {
			// expiry may equal despawn; later than despawn is nonsense
			if record.OwnershipExpiresAt.After(record.DespawnAt) {
				return fmt.Errorf("%w: ground vid %d ownership_expires_at after despawn_at", ErrInvalidGroundItemSnapshot, record.VID)
			}
		}
	} else if record.OwnershipExpiresAt != nil {
		return fmt.Errorf("%w: ground vid %d public ownership must omit ownership_expires_at", ErrInvalidGroundItemSnapshot, record.VID)
	}

	hasItem := record.ItemCount != nil
	hasGold := record.GoldAmount != nil
	switch {
	case hasItem && hasGold:
		return fmt.Errorf("%w: ground vid %d has both item count and gold amount", ErrInvalidGroundItemSnapshot, record.VID)
	case hasGold:
		if record.ItemID != 0 {
			return fmt.Errorf("%w: ground vid %d gold-shaped row must omit item_id", ErrInvalidGroundItemSnapshot, record.VID)
		}
		if record.Vnum != bootstrapGroundGoldVnum {
			return fmt.Errorf("%w: ground vid %d has gold amount with non-gold vnum %d", ErrInvalidGroundItemSnapshot, record.VID, record.Vnum)
		}
		if *record.GoldAmount == 0 || *record.GoldAmount > bootstrapGroundGoldMaxAmount {
			return fmt.Errorf("%w: ground vid %d gold amount out of bounds", ErrInvalidGroundItemSnapshot, record.VID)
		}
		if record.HasSockets || record.Socket0 != 0 || record.Socket1 != 0 || record.Socket2 != 0 {
			return fmt.Errorf("%w: ground vid %d gold-shaped row must omit instance sockets", ErrInvalidGroundItemSnapshot, record.VID)
		}
		if record.HasAttributes || durableGroundItemAttributesNonZero(record.Attributes) {
			return fmt.Errorf("%w: ground vid %d gold-shaped row must omit instance attributes", ErrInvalidGroundItemSnapshot, record.VID)
		}
	case hasItem:
		if record.ItemID == 0 {
			return fmt.Errorf("%w: ground vid %d item-shaped row requires item_id", ErrInvalidGroundItemSnapshot, record.VID)
		}
		if *record.ItemCount == 0 || *record.ItemCount > bootstrapGroundItemMaxCount {
			return fmt.Errorf("%w: ground vid %d item count out of bounds", ErrInvalidGroundItemSnapshot, record.VID)
		}
		if err := validateDurableGroundItemInstanceSockets(record); err != nil {
			return err
		}
		if err := validateDurableGroundItemInstanceAttributes(record); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: ground vid %d has neither item count nor gold amount", ErrInvalidGroundItemSnapshot, record.VID)
	}
	return nil
}

func validateDurableGroundItemInstanceSockets(record DurableGroundItemRecord) error {
	if record.HasSockets {
		return nil
	}
	if record.Socket0 != 0 || record.Socket1 != 0 || record.Socket2 != 0 {
		return fmt.Errorf("%w: ground vid %d has non-zero sockets without has_sockets", ErrInvalidGroundItemSnapshot, record.VID)
	}
	return nil
}

func validateDurableGroundItemInstanceAttributes(record DurableGroundItemRecord) error {
	if record.HasAttributes {
		return nil
	}
	if durableGroundItemAttributesNonZero(record.Attributes) {
		return fmt.Errorf("%w: ground vid %d has non-zero attributes without has_attributes", ErrInvalidGroundItemSnapshot, record.VID)
	}
	return nil
}

func durableGroundItemAttributesNonZero(attributes *inventory.AttributeValues) bool {
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

func cloneDurableGroundItemRecords(records []DurableGroundItemRecord) []DurableGroundItemRecord {
	if records == nil {
		return nil
	}
	out := make([]DurableGroundItemRecord, len(records))
	copy(out, records)
	for i := range out {
		if records[i].ItemCount != nil {
			count := *records[i].ItemCount
			out[i].ItemCount = &count
		}
		if records[i].GoldAmount != nil {
			amount := *records[i].GoldAmount
			out[i].GoldAmount = &amount
		}
		if records[i].OwnershipExpiresAt != nil {
			expires := records[i].OwnershipExpiresAt.UTC()
			out[i].OwnershipExpiresAt = &expires
		}
		if records[i].Attributes != nil {
			copied := *records[i].Attributes
			out[i].Attributes = &copied
		}
	}
	return out
}

func rejectGroundItemCommittedSnapshotSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat ground item snapshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: ground item snapshot %q is a symlink", ErrInvalidGroundItemSnapshot, filepath.Base(path))
	}
	return nil
}

func syncGroundItemStoreDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// Ensure FileStore satisfies the exporter seam when used as a durable source.
var _ BootstrapGroundItemStateExporter = (*FileStore)(nil)
var _ GroundItemStore = (*FileStore)(nil)
