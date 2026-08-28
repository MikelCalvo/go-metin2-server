package loginticket

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

var (
	ErrStoreDirRequired     = errors.New("login ticket store dir is required")
	ErrTicketExists         = errors.New("login ticket already exists")
	ErrTicketNotFound       = errors.New("login ticket not found")
	ErrTicketLoginMismatch  = errors.New("login ticket login does not match")
	ErrInvalidTicket        = errors.New("invalid login ticket snapshot")
	ErrIssuedBeforeRequired = errors.New("login ticket issued-before cutoff is required")
)

type Character struct {
	ID          uint32                   `json:"id"`
	VID         uint32                   `json:"vid"`
	Name        string                   `json:"name"`
	Job         uint8                    `json:"job"`
	RaceNum     uint16                   `json:"race_num"`
	Level       uint8                    `json:"level"`
	PlayMinutes uint32                   `json:"play_minutes"`
	ST          uint8                    `json:"st"`
	HT          uint8                    `json:"ht"`
	DX          uint8                    `json:"dx"`
	IQ          uint8                    `json:"iq"`
	MainPart    uint16                   `json:"main_part"`
	ChangeName  uint8                    `json:"change_name"`
	HairPart    uint16                   `json:"hair_part"`
	Dummy       [4]byte                  `json:"dummy"`
	X           int32                    `json:"x"`
	Y           int32                    `json:"y"`
	Z           int32                    `json:"z"`
	MapIndex    uint32                   `json:"map_index"`
	Empire      uint8                    `json:"empire"`
	SkillGroup  uint8                    `json:"skill_group"`
	GuildID     uint32                   `json:"guild_id"`
	GuildName   string                   `json:"guild_name"`
	Points      [255]int32               `json:"points"`
	Gold        uint64                   `json:"gold"`
	Inventory   []inventory.ItemInstance `json:"inventory"`
	Equipment   []inventory.ItemInstance `json:"equipment"`
	Quickslots  []Quickslot              `json:"quickslots"`
}

type Quickslot struct {
	Position uint8 `json:"position"`
	Type     uint8 `json:"type"`
	Slot     uint8 `json:"slot"`
}

func (c *Character) NormalizeItemState() {
	if c.Inventory == nil {
		c.Inventory = []inventory.ItemInstance{}
	}
	if c.Equipment == nil {
		c.Equipment = []inventory.ItemInstance{}
	}
	if c.Quickslots == nil {
		c.Quickslots = []Quickslot{}
	}
}

func (c Character) IsEmptySlot() bool {
	if c.ID != 0 || c.VID != 0 || c.Name != "" || c.Job != 0 || c.RaceNum != 0 || c.Level != 0 || c.PlayMinutes != 0 {
		return false
	}
	if c.ST != 0 || c.HT != 0 || c.DX != 0 || c.IQ != 0 || c.MainPart != 0 || c.ChangeName != 0 || c.HairPart != 0 {
		return false
	}
	if c.X != 0 || c.Y != 0 || c.Z != 0 || c.MapIndex != 0 || c.Empire != 0 || c.SkillGroup != 0 || c.GuildID != 0 || c.GuildName != "" || c.Gold != 0 {
		return false
	}
	for _, value := range c.Dummy {
		if value != 0 {
			return false
		}
	}
	for _, value := range c.Points {
		if value != 0 {
			return false
		}
	}
	return len(c.Inventory) == 0 && len(c.Equipment) == 0 && len(c.Quickslots) == 0
}

func CloneCharacters(characters []Character) []Character {
	if characters == nil {
		return nil
	}
	cloned := make([]Character, len(characters))
	copy(cloned, characters)
	for i := range cloned {
		if cloned[i].Inventory != nil {
			cloned[i].Inventory = append(cloned[i].Inventory[:0:0], cloned[i].Inventory...)
			for j := range cloned[i].Inventory {
				cloned[i].Inventory[j].Sockets = cloned[i].Inventory[j].CloneSockets()
			}
		}
		if cloned[i].Equipment != nil {
			cloned[i].Equipment = append(cloned[i].Equipment[:0:0], cloned[i].Equipment...)
			for j := range cloned[i].Equipment {
				cloned[i].Equipment[j].Sockets = cloned[i].Equipment[j].CloneSockets()
			}
		}
		if cloned[i].Quickslots != nil {
			cloned[i].Quickslots = append(cloned[i].Quickslots[:0:0], cloned[i].Quickslots...)
		}
	}
	return cloned
}

type Ticket struct {
	Login      string      `json:"login"`
	LoginKey   uint32      `json:"login_key"`
	Empire     uint8       `json:"empire"`
	IssuedAt   time.Time   `json:"issued_at"`
	Characters []Character `json:"characters"`
}

type SnapshotSummary struct {
	TicketCount             int        `json:"ticket_count"`
	CharacterCount          int        `json:"character_count"`
	EmptyCharacterSlotCount int        `json:"empty_character_slot_count,omitempty"`
	Logins                  []string   `json:"logins"`
	LoginKeys               []uint32   `json:"login_keys"`
	OldestIssuedAt          *time.Time `json:"oldest_issued_at,omitempty"`
	NewestIssuedAt          *time.Time `json:"newest_issued_at,omitempty"`
	CrashTempCount          int        `json:"crash_temp_count,omitempty"`
	CrashTempFiles          []string   `json:"crash_temp_files,omitempty"`
}

type IssuedBeforePreviewSummary struct {
	IssuedBefore   time.Time       `json:"issued_before"`
	StaleCount     int             `json:"stale_count"`
	StaleLogins    []string        `json:"stale_logins"`
	StaleLoginKeys []uint32        `json:"stale_login_keys"`
	Current        SnapshotSummary `json:"current"`
}

type IssuedBeforeCleanupSummary struct {
	IssuedBefore     time.Time       `json:"issued_before"`
	RemovedCount     int             `json:"removed_count"`
	RemovedLogins    []string        `json:"removed_logins"`
	RemovedLoginKeys []uint32        `json:"removed_login_keys"`
	Remaining        SnapshotSummary `json:"remaining"`
}

func normalizeCharactersItemState(characters []Character) {
	for i := range characters {
		characters[i].NormalizeItemState()
	}
}

type Store interface {
	Issue(Ticket) error
	Load(login string, loginKey uint32) (Ticket, error)
	Consume(login string, loginKey uint32) (Ticket, error)
}

type FileStore struct {
	dir string
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

var durableSyncDisabledForTest bool

// DisableDurableSyncForTest skips fsync calls in this package until the
// returned restore function is called. It is intended for high-volume tests
// that assert runtime behavior, not crash durability.
func DisableDurableSyncForTest() func() {
	previous := durableSyncDisabledForTest
	durableSyncDisabledForTest = true
	return func() { durableSyncDisabledForTest = previous }
}

func (s *FileStore) Dir() string {
	if s == nil {
		return ""
	}
	return s.dir
}

func (s *FileStore) syncStoreDir() error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return syncStoreDir(s.dir)
}

func (s *FileStore) syncFile(file *os.File) error {
	if s == nil || durableSyncDisabledForTest {
		return nil
	}
	return file.Sync()
}

func (s *FileStore) List() ([]Ticket, error) {
	if s.dir == "" {
		return nil, ErrStoreDirRequired
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Ticket{}, nil
		}
		return nil, fmt.Errorf("read login ticket store dir: %w", err)
	}

	tickets := make([]Ticket, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == BackupManifestFilename {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		loginKey, err := decodeTicketFilenameLoginKey(entry.Name())
		if err != nil {
			return nil, err
		}
		if canonicalFilename := filepath.Base(s.ticketPath(loginKey)); entry.Name() != canonicalFilename {
			return nil, fmt.Errorf("%w: login ticket filename %q is not canonical for login key %08x", ErrInvalidTicket, entry.Name(), loginKey)
		}
		ticket, err := s.readTicketFile(loginKey)
		if err != nil {
			return nil, err
		}
		if ticket.LoginKey != loginKey {
			return nil, fmt.Errorf("%w: login ticket filename key %08x does not match snapshot key %08x", ErrInvalidTicket, loginKey, ticket.LoginKey)
		}
		tickets = append(tickets, ticket)
	}
	sort.Slice(tickets, func(i, j int) bool {
		leftLogin := strings.ToLower(tickets[i].Login)
		rightLogin := strings.ToLower(tickets[j].Login)
		if leftLogin != rightLogin {
			return leftLogin < rightLogin
		}
		if tickets[i].Login != tickets[j].Login {
			return tickets[i].Login < tickets[j].Login
		}
		return tickets[i].LoginKey < tickets[j].LoginKey
	})
	return tickets, nil
}

func (s *FileStore) Validate() (SnapshotSummary, error) {
	tickets, err := s.List()
	if err != nil {
		return SnapshotSummary{}, err
	}
	if err := s.validateActiveBackupManifestForTickets(tickets); err != nil {
		return SnapshotSummary{}, err
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	return summarizeTickets(tickets, crashTempFiles), nil
}

func summarizeTickets(tickets []Ticket, crashTempFiles []string) SnapshotSummary {
	summary := SnapshotSummary{
		TicketCount:    len(tickets),
		Logins:         make([]string, 0, len(tickets)),
		LoginKeys:      make([]uint32, 0, len(tickets)),
		CrashTempCount: len(crashTempFiles),
		CrashTempFiles: crashTempFiles,
	}
	for _, ticket := range tickets {
		summary.Logins = append(summary.Logins, ticket.Login)
		summary.LoginKeys = append(summary.LoginKeys, ticket.LoginKey)
		summary.CharacterCount += len(ticket.Characters)
		for _, character := range ticket.Characters {
			if character.IsEmptySlot() {
				summary.EmptyCharacterSlotCount++
			}
		}
		issuedAt := ticket.IssuedAt.UTC()
		if summary.OldestIssuedAt == nil || issuedAt.Before(*summary.OldestIssuedAt) {
			oldest := issuedAt
			summary.OldestIssuedAt = &oldest
		}
		if summary.NewestIssuedAt == nil || issuedAt.After(*summary.NewestIssuedAt) {
			newest := issuedAt
			summary.NewestIssuedAt = &newest
		}
	}
	return summary
}

func (s *FileStore) CleanupCrashTempFiles() (SnapshotSummary, error) {
	if s.dir == "" {
		return SnapshotSummary{}, ErrStoreDirRequired
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
	for _, filename := range crashTempFiles {
		if err := os.Remove(filepath.Join(s.dir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return SnapshotSummary{}, fmt.Errorf("remove login ticket crash temp file %q: %w", filename, err)
		}
	}
	if err := syncStoreDir(s.dir); err != nil {
		return SnapshotSummary{}, fmt.Errorf("sync login ticket store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

func (s *FileStore) PreviewIssuedBefore(issuedBefore time.Time) (IssuedBeforePreviewSummary, error) {
	if s.dir == "" {
		return IssuedBeforePreviewSummary{}, ErrStoreDirRequired
	}
	if issuedBefore.IsZero() {
		return IssuedBeforePreviewSummary{}, ErrIssuedBeforeRequired
	}
	tickets, err := s.List()
	if err != nil {
		return IssuedBeforePreviewSummary{}, err
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return IssuedBeforePreviewSummary{}, err
	}
	summary := IssuedBeforePreviewSummary{
		IssuedBefore:   issuedBefore,
		StaleLogins:    []string{},
		StaleLoginKeys: []uint32{},
		Current:        summarizeTickets(tickets, crashTempFiles),
	}
	for _, ticket := range tickets {
		if !ticket.IssuedAt.Before(issuedBefore) {
			continue
		}
		summary.StaleLogins = append(summary.StaleLogins, ticket.Login)
		summary.StaleLoginKeys = append(summary.StaleLoginKeys, ticket.LoginKey)
	}
	summary.StaleCount = len(summary.StaleLoginKeys)
	return summary, nil
}

func (s *FileStore) CleanupIssuedBefore(issuedBefore time.Time) (IssuedBeforeCleanupSummary, error) {
	if s.dir == "" {
		return IssuedBeforeCleanupSummary{}, ErrStoreDirRequired
	}
	if issuedBefore.IsZero() {
		return IssuedBeforeCleanupSummary{}, ErrIssuedBeforeRequired
	}
	tickets, err := s.List()
	if err != nil {
		return IssuedBeforeCleanupSummary{}, err
	}
	if _, err := s.crashTempFiles(); err != nil {
		return IssuedBeforeCleanupSummary{}, err
	}
	summary := IssuedBeforeCleanupSummary{
		IssuedBefore:     issuedBefore,
		RemovedLogins:    []string{},
		RemovedLoginKeys: []uint32{},
	}
	for _, ticket := range tickets {
		if !ticket.IssuedAt.Before(issuedBefore) {
			continue
		}
		if err := os.Remove(s.ticketPath(ticket.LoginKey)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return IssuedBeforeCleanupSummary{}, fmt.Errorf("remove login ticket %08x issued before %s: %w", ticket.LoginKey, issuedBefore.Format(time.RFC3339Nano), err)
		}
		summary.RemovedLogins = append(summary.RemovedLogins, ticket.Login)
		summary.RemovedLoginKeys = append(summary.RemovedLoginKeys, ticket.LoginKey)
	}
	summary.RemovedCount = len(summary.RemovedLoginKeys)
	if summary.RemovedCount != 0 {
		if err := removeBackupManifest(s.dir); err != nil {
			return IssuedBeforeCleanupSummary{}, fmt.Errorf("remove stale login ticket backup manifest: %w", err)
		}
		if err := s.syncStoreDir(); err != nil {
			return IssuedBeforeCleanupSummary{}, fmt.Errorf("sync login ticket store dir after issued-before cleanup: %w", err)
		}
	}
	remaining, err := s.Validate()
	if err != nil {
		return IssuedBeforeCleanupSummary{}, err
	}
	summary.Remaining = remaining
	return summary, nil
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read login ticket store crash temp files: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".ticket-") && strings.HasSuffix(name, ".json") {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%w: login ticket crash temp file %q is a symlink", ErrInvalidTicket, name)
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

func (s *FileStore) Issue(ticket Ticket) error {
	if s.dir == "" {
		return ErrStoreDirRequired
	}
	if ticket.IssuedAt.IsZero() {
		ticket.IssuedAt = time.Now().UTC()
	}
	ticket.Characters = CloneCharacters(ticket.Characters)
	normalizeCharactersItemState(ticket.Characters)
	if err := validateTicket(ticket); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create login ticket store dir: %w", err)
	}

	path := s.ticketPath(ticket.LoginKey)
	if _, err := os.Stat(path); err == nil {
		return ErrTicketExists
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat login ticket: %w", err)
	}

	temp, err := os.CreateTemp(s.dir, ".ticket-*.json")
	if err != nil {
		return fmt.Errorf("create login ticket temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	if err := json.NewEncoder(temp).Encode(ticket); err != nil {
		return fmt.Errorf("encode login ticket: %w", err)
	}
	if err := s.syncFile(temp); err != nil {
		return fmt.Errorf("sync login ticket temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close login ticket temp file: %w", err)
	}
	if err := linkTicketFile(temp.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrTicketExists
		}
		return fmt.Errorf("commit login ticket file: %w", err)
	}
	_ = os.Remove(temp.Name())
	if err := removeBackupManifest(s.dir); err != nil {
		return fmt.Errorf("remove stale login ticket backup manifest: %w", err)
	}
	if err := s.syncStoreDir(); err != nil {
		return fmt.Errorf("sync login ticket store dir: %w", err)
	}

	return nil
}

var syncStoreDir = syncDir

var linkTicketFile = os.Link

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *FileStore) Load(login string, loginKey uint32) (Ticket, error) {
	return s.read(login, loginKey, false)
}

func (s *FileStore) Consume(login string, loginKey uint32) (Ticket, error) {
	return s.read(login, loginKey, true)
}

func (s *FileStore) read(login string, loginKey uint32, consume bool) (Ticket, error) {
	if s.dir == "" {
		return Ticket{}, ErrStoreDirRequired
	}

	ticket, err := s.readTicketFile(loginKey)
	if err != nil {
		return Ticket{}, err
	}
	if ticket.Login != login || ticket.LoginKey != loginKey {
		return Ticket{}, ErrTicketLoginMismatch
	}
	if !consume {
		return ticket, nil
	}
	if err := os.Remove(s.ticketPath(loginKey)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Ticket{}, ErrTicketNotFound
		}
		return Ticket{}, fmt.Errorf("delete consumed login ticket: %w", err)
	}
	if err := removeBackupManifest(s.dir); err != nil {
		return Ticket{}, fmt.Errorf("remove stale login ticket backup manifest: %w", err)
	}
	if err := s.syncStoreDir(); err != nil {
		return Ticket{}, fmt.Errorf("sync consumed login ticket store dir: %w", err)
	}

	return ticket, nil
}

func (s *FileStore) readTicketFile(loginKey uint32) (Ticket, error) {
	path := s.ticketPath(loginKey)
	if err := rejectCommittedTicketSnapshotSymlink(path); err != nil {
		return Ticket{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Ticket{}, ErrTicketNotFound
		}
		return Ticket{}, fmt.Errorf("read login ticket: %w", err)
	}

	var ticket Ticket
	if err := decodeTicketStrict(raw, &ticket); err != nil {
		return Ticket{}, fmt.Errorf("%w: decode login ticket: %v", ErrInvalidTicket, err)
	}
	normalizeCharactersItemState(ticket.Characters)
	if err := validateTicket(ticket); err != nil {
		return Ticket{}, err
	}
	return ticket, nil
}

func (s *FileStore) ticketPath(loginKey uint32) string {
	return filepath.Join(s.dir, fmt.Sprintf("%08x.json", loginKey))
}

func decodeTicketFilenameLoginKey(filename string) (uint32, error) {
	encoded := strings.TrimSuffix(filename, ".json")
	if len(encoded) != 8 {
		return 0, fmt.Errorf("%w: login ticket filename %q is not 8-digit hex JSON", ErrInvalidTicket, filename)
	}
	loginKey, err := strconv.ParseUint(encoded, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: login ticket filename %q is not 8-digit hex JSON", ErrInvalidTicket, filename)
	}
	return uint32(loginKey), nil
}

func rejectCommittedTicketSnapshotSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat login ticket: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: login ticket snapshot %q is a symlink", ErrInvalidTicket, filepath.Base(path))
	}
	return nil
}

func decodeTicketStrict(raw []byte, ticket *Ticket) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid utf-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(ticket); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func validateTicket(ticket Ticket) error {
	if strings.TrimSpace(ticket.Login) == "" {
		return fmt.Errorf("%w: login is required", ErrInvalidTicket)
	}
	if ticket.Login != strings.TrimSpace(ticket.Login) {
		return fmt.Errorf("%w: login %q has leading or trailing whitespace", ErrInvalidTicket, ticket.Login)
	}
	if containsNUL(ticket.Login) {
		return fmt.Errorf("%w: login contains NUL", ErrInvalidTicket)
	}
	if ticket.LoginKey == 0 {
		return fmt.Errorf("%w: login key is required", ErrInvalidTicket)
	}
	if ticket.IssuedAt.IsZero() {
		return fmt.Errorf("%w: issued_at is required", ErrInvalidTicket)
	}
	if err := validateUniqueCharacterIdentity(ticket.Characters); err != nil {
		return err
	}
	for _, character := range ticket.Characters {
		if err := validateCharacterItemPayloads(character); err != nil {
			return err
		}
		if err := validateCharacterUniqueItemInstanceIDs(character); err != nil {
			return err
		}
		if err := validateCharacterUniqueInventorySlots(character); err != nil {
			return err
		}
		if err := validateCharacterUniqueEquipmentSlots(character); err != nil {
			return err
		}
		if err := validateCharacterQuickslots(character); err != nil {
			return err
		}
	}
	return nil
}

func validateUniqueCharacterIdentity(characters []Character) error {
	ids := make(map[uint32]string, len(characters))
	vids := make(map[uint32]string, len(characters))
	names := make(map[string]uint32, len(characters))
	for _, character := range characters {
		if character.ID == 0 {
			if !character.IsEmptySlot() {
				return fmt.Errorf("%w: character slot with zero id contains non-empty state", ErrInvalidTicket)
			}
			continue
		}
		trimmedName := strings.TrimSpace(character.Name)
		if trimmedName == "" {
			return fmt.Errorf("%w: character id %d has empty name", ErrInvalidTicket, character.ID)
		}
		if trimmedName != character.Name {
			return fmt.Errorf("%w: character name %q has leading or trailing whitespace", ErrInvalidTicket, character.Name)
		}
		if containsNUL(character.Name) {
			return fmt.Errorf("%w: character name contains NUL", ErrInvalidTicket)
		}
		if containsNUL(character.GuildName) {
			return fmt.Errorf("%w: character guild name contains NUL", ErrInvalidTicket)
		}
		if previousName, ok := ids[character.ID]; ok {
			return fmt.Errorf("%w: character id %d is used by %q and %q", ErrInvalidTicket, character.ID, previousName, character.Name)
		}
		ids[character.ID] = character.Name
		if character.VID != 0 {
			if previousName, ok := vids[character.VID]; ok {
				return fmt.Errorf("%w: character vid %d is used by %q and %q", ErrInvalidTicket, character.VID, previousName, character.Name)
			}
			vids[character.VID] = character.Name
		}

		normalizedName := strings.ToLower(character.Name)
		if previousID, ok := names[normalizedName]; ok {
			return fmt.Errorf("%w: character name %q is used by id %d and id %d", ErrInvalidTicket, character.Name, previousID, character.ID)
		}
		names[normalizedName] = character.ID
	}
	return nil
}

func validateCharacterItemPayloads(character Character) error {
	for _, item := range character.Inventory {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("%w: inventory item %d: %v", ErrInvalidTicket, item.ID, err)
		}
		if item.Equipped {
			return fmt.Errorf("%w: inventory item %d is marked equipped", ErrInvalidTicket, item.ID)
		}
		if item.Slot >= inventory.CarriedInventorySlotCount {
			return fmt.Errorf("%w: inventory item %d: slot %d out of range", ErrInvalidTicket, item.ID, item.Slot)
		}
	}
	for _, item := range character.Equipment {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("%w: equipment item %d: %v", ErrInvalidTicket, item.ID, err)
		}
		if !item.Equipped {
			return fmt.Errorf("%w: equipment item %d is not marked equipped", ErrInvalidTicket, item.ID)
		}
	}
	return nil
}

func validateCharacterUniqueEquipmentSlots(character Character) error {
	equipmentSlots := make(map[uint8]uint64, len(character.Equipment))
	for _, item := range character.Equipment {
		if previousID, ok := equipmentSlots[uint8(item.EquipSlot)]; ok {
			return fmt.Errorf("%w: equipment slot %s contains item %d and item %d", ErrInvalidTicket, item.EquipSlot.String(), previousID, item.ID)
		}
		equipmentSlots[uint8(item.EquipSlot)] = item.ID
	}
	return nil
}

func validateCharacterUniqueItemInstanceIDs(character Character) error {
	itemIDs := make(map[uint64]string, len(character.Inventory)+len(character.Equipment))
	for _, item := range character.Inventory {
		if previous, ok := itemIDs[item.ID]; ok {
			return fmt.Errorf("%w: item instance id %d appears in %s and inventory slot %d", ErrInvalidTicket, item.ID, previous, item.Slot)
		}
		itemIDs[item.ID] = fmt.Sprintf("inventory slot %d", item.Slot)
	}
	for _, item := range character.Equipment {
		if previous, ok := itemIDs[item.ID]; ok {
			return fmt.Errorf("%w: item instance id %d appears in %s and equipment slot %s", ErrInvalidTicket, item.ID, previous, item.EquipSlot.String())
		}
		itemIDs[item.ID] = fmt.Sprintf("equipment slot %s", item.EquipSlot.String())
	}
	return nil
}

func validateCharacterUniqueInventorySlots(character Character) error {
	inventorySlots := make(map[inventory.SlotIndex]uint64, len(character.Inventory))
	for _, item := range character.Inventory {
		if previousID, ok := inventorySlots[item.Slot]; ok {
			return fmt.Errorf("%w: inventory slot %d contains item %d and item %d", ErrInvalidTicket, item.Slot, previousID, item.ID)
		}
		inventorySlots[item.Slot] = item.ID
	}
	return nil
}

func validateCharacterQuickslots(character Character) error {
	quickslotPositions := make(map[uint8]Quickslot, len(character.Quickslots))
	quickslotTuples := make(map[quickslotTuple]uint8, len(character.Quickslots))
	for _, quickslot := range character.Quickslots {
		if !validQuickslotTuple(quickslot) {
			return fmt.Errorf("%w: quickslot position %d has invalid type %d slot %d", ErrInvalidTicket, quickslot.Position, quickslot.Type, quickslot.Slot)
		}
		if previous, ok := quickslotPositions[quickslot.Position]; ok {
			return fmt.Errorf("%w: quickslot position %d contains type %d slot %d and type %d slot %d", ErrInvalidTicket, quickslot.Position, previous.Type, previous.Slot, quickslot.Type, quickslot.Slot)
		}
		if quickslot.Type == quickslotproto.TypeSkill || quickslot.Type == quickslotproto.TypeCommand {
			tuple := quickslotTuple{Type: quickslot.Type, Slot: quickslot.Slot}
			if previousPosition, ok := quickslotTuples[tuple]; ok {
				return fmt.Errorf("%w: quickslot type %d slot %d is bound at positions %d and %d", ErrInvalidTicket, quickslot.Type, quickslot.Slot, previousPosition, quickslot.Position)
			}
			quickslotTuples[tuple] = quickslot.Position
		}
		quickslotPositions[quickslot.Position] = quickslot
	}
	return nil
}

type quickslotTuple struct {
	Type uint8
	Slot uint8
}

func validQuickslotTuple(quickslot Quickslot) bool {
	if quickslot.Position >= 36 {
		return false
	}
	switch quickslot.Type {
	case quickslotproto.TypeNone:
		return quickslot.Slot == 0
	case quickslotproto.TypeItem:
		return quickslot.Slot < uint8(inventory.CarriedInventorySlotCount)
	case quickslotproto.TypeSkill:
		return quickslot.Slot < 200
	case quickslotproto.TypeCommand:
		return quickslot.Slot < 60
	default:
		return false
	}
}

func containsNUL(value string) bool {
	return strings.ContainsRune(value, '\x00')
}
