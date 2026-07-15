package loginticket

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

var (
	ErrStoreDirRequired    = errors.New("login ticket store dir is required")
	ErrTicketExists        = errors.New("login ticket already exists")
	ErrTicketNotFound      = errors.New("login ticket not found")
	ErrTicketLoginMismatch = errors.New("login ticket login does not match")
	ErrInvalidTicket       = errors.New("invalid login ticket snapshot")
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

func CloneCharacters(characters []Character) []Character {
	if characters == nil {
		return nil
	}
	cloned := make([]Character, len(characters))
	copy(cloned, characters)
	for i := range cloned {
		if cloned[i].Inventory != nil {
			cloned[i].Inventory = append(cloned[i].Inventory[:0:0], cloned[i].Inventory...)
		}
		if cloned[i].Equipment != nil {
			cloned[i].Equipment = append(cloned[i].Equipment[:0:0], cloned[i].Equipment...)
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
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync login ticket temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close login ticket temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrTicketExists
		}
		return fmt.Errorf("commit login ticket file: %w", err)
	}
	if err := syncStoreDir(s.dir); err != nil {
		return fmt.Errorf("sync login ticket store dir: %w", err)
	}

	return nil
}

var syncStoreDir = syncDir

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

	path := s.ticketPath(loginKey)
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
	if ticket.Login != login || ticket.LoginKey != loginKey {
		return Ticket{}, ErrTicketLoginMismatch
	}
	if !consume {
		return ticket, nil
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Ticket{}, ErrTicketNotFound
		}
		return Ticket{}, fmt.Errorf("delete consumed login ticket: %w", err)
	}
	if err := syncStoreDir(s.dir); err != nil {
		return Ticket{}, fmt.Errorf("sync consumed login ticket store dir: %w", err)
	}

	return ticket, nil
}

func (s *FileStore) ticketPath(loginKey uint32) string {
	return filepath.Join(s.dir, fmt.Sprintf("%08x.json", loginKey))
}

func decodeTicketStrict(raw []byte, ticket *Ticket) error {
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
	if err := validateUniqueCharacterIdentity(ticket.Characters); err != nil {
		return err
	}
	for _, character := range ticket.Characters {
		if err := validateCharacterItemPayloads(character); err != nil {
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
	names := make(map[string]uint32, len(characters))
	for _, character := range characters {
		if previousName, ok := ids[character.ID]; ok {
			return fmt.Errorf("%w: character id %d is used by %q and %q", ErrInvalidTicket, character.ID, previousName, character.Name)
		}
		ids[character.ID] = character.Name

		normalizedName := strings.ToLower(strings.TrimSpace(character.Name))
		if normalizedName == "" {
			continue
		}
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
		if item.Slot >= inventory.CarriedInventorySlotCount {
			return fmt.Errorf("%w: inventory item %d: slot %d out of range", ErrInvalidTicket, item.ID, item.Slot)
		}
	}
	for _, item := range character.Equipment {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("%w: equipment item %d: %v", ErrInvalidTicket, item.ID, err)
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

func validateCharacterQuickslots(character Character) error {
	quickslotPositions := make(map[uint8]Quickslot, len(character.Quickslots))
	for _, quickslot := range character.Quickslots {
		if !validQuickslotTuple(quickslot) {
			return fmt.Errorf("%w: quickslot position %d has invalid type %d slot %d", ErrInvalidTicket, quickslot.Position, quickslot.Type, quickslot.Slot)
		}
		if previous, ok := quickslotPositions[quickslot.Position]; ok {
			return fmt.Errorf("%w: quickslot position %d contains type %d slot %d and type %d slot %d", ErrInvalidTicket, quickslot.Position, previous.Type, previous.Slot, quickslot.Type, quickslot.Slot)
		}
		quickslotPositions[quickslot.Position] = quickslot
	}
	return nil
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
