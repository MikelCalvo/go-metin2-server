package loginticket

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

var (
	ErrStoreDirRequired    = errors.New("login ticket store dir is required")
	ErrTicketExists        = errors.New("login ticket already exists")
	ErrTicketNotFound      = errors.New("login ticket not found")
	ErrTicketLoginMismatch = errors.New("login ticket login does not match")
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
}

func (c *Character) NormalizeItemState() {
	if c.Inventory == nil {
		c.Inventory = []inventory.ItemInstance{}
	}
	if c.Equipment == nil {
		c.Equipment = []inventory.ItemInstance{}
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
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close login ticket temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrTicketExists
		}
		return fmt.Errorf("commit login ticket file: %w", err)
	}

	return nil
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
	if err := json.Unmarshal(raw, &ticket); err != nil {
		return Ticket{}, fmt.Errorf("decode login ticket: %w", err)
	}
	normalizeCharactersItemState(ticket.Characters)
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

	return ticket, nil
}

func (s *FileStore) ticketPath(loginKey uint32) string {
	return filepath.Join(s.dir, fmt.Sprintf("%08x.json", loginKey))
}
