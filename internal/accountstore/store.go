package accountstore

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

var (
	ErrStoreDirRequired = errors.New("account store dir is required")
	ErrLoginRequired    = errors.New("account login is required")
	ErrAccountNotFound  = errors.New("account not found")
	ErrInvalidAccount   = errors.New("invalid account snapshot")
)

type Account struct {
	Login      string                  `json:"login"`
	Empire     uint8                   `json:"empire"`
	Characters []loginticket.Character `json:"characters"`
}

func normalizeAccountCharacters(characters []loginticket.Character) []loginticket.Character {
	cloned := loginticket.CloneCharacters(characters)
	for i := range cloned {
		cloned[i].NormalizeItemState()
	}
	return cloned
}

type Store interface {
	Load(login string) (Account, error)
	Save(account Account) error
}

type FileStore struct {
	dir string
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func (s *FileStore) Load(login string) (Account, error) {
	if s.dir == "" {
		return Account{}, ErrStoreDirRequired
	}
	if login == "" {
		return Account{}, ErrLoginRequired
	}

	raw, err := os.ReadFile(s.accountPath(login))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Account{}, ErrAccountNotFound
		}
		return Account{}, fmt.Errorf("read account: %w", err)
	}

	var account Account
	if err := decodeAccountStrict(raw, &account); err != nil {
		return Account{}, fmt.Errorf("%w: decode account: %v", ErrInvalidAccount, err)
	}
	account.Characters = normalizeAccountCharacters(account.Characters)
	if err := validateLoadedAccountForLogin(login, account); err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *FileStore) Save(account Account) error {
	if s.dir == "" {
		return ErrStoreDirRequired
	}
	if account.Login == "" {
		return ErrLoginRequired
	}
	account.Characters = normalizeAccountCharacters(account.Characters)
	if err := validateAccount(account); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create account store dir: %w", err)
	}

	temp, err := os.CreateTemp(s.dir, ".account-*.json")
	if err != nil {
		return fmt.Errorf("create account temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	if err := json.NewEncoder(temp).Encode(account); err != nil {
		return fmt.Errorf("encode account: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync account temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close account temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), s.accountPath(account.Login)); err != nil {
		return fmt.Errorf("commit account file: %w", err)
	}
	if err := syncStoreDir(s.dir); err != nil {
		return fmt.Errorf("sync account store dir: %w", err)
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

func (s *FileStore) accountPath(login string) string {
	normalized := strings.ToLower(login)
	filename := hex.EncodeToString([]byte(normalized)) + ".json"
	return filepath.Join(s.dir, filename)
}

func decodeAccountStrict(raw []byte, account *Account) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(account); err != nil {
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

func validateAccount(account Account) error {
	for _, character := range account.Characters {
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

func validateLoadedAccountForLogin(requestedLogin string, account Account) error {
	if account.Login == "" {
		return fmt.Errorf("%w: account login is empty", ErrInvalidAccount)
	}
	if !strings.EqualFold(account.Login, requestedLogin) {
		return fmt.Errorf("%w: snapshot login %q does not match requested login %q", ErrInvalidAccount, account.Login, requestedLogin)
	}
	return validateAccount(account)
}

func validateCharacterItemPayloads(character loginticket.Character) error {
	for _, item := range character.Inventory {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("%w: inventory item %d: %v", ErrInvalidAccount, item.ID, err)
		}
		if item.Slot >= inventory.CarriedInventorySlotCount {
			return fmt.Errorf("%w: inventory item %d: slot %d out of range", ErrInvalidAccount, item.ID, item.Slot)
		}
	}
	for _, item := range character.Equipment {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("%w: equipment item %d: %v", ErrInvalidAccount, item.ID, err)
		}
	}
	return nil
}

func validateCharacterUniqueEquipmentSlots(character loginticket.Character) error {
	equipmentSlots := make(map[uint8]uint64, len(character.Equipment))
	for _, item := range character.Equipment {
		if previousID, ok := equipmentSlots[uint8(item.EquipSlot)]; ok {
			return fmt.Errorf("%w: equipment slot %s contains item %d and item %d", ErrInvalidAccount, item.EquipSlot.String(), previousID, item.ID)
		}
		equipmentSlots[uint8(item.EquipSlot)] = item.ID
	}
	return nil
}

func validateCharacterQuickslots(character loginticket.Character) error {
	quickslotPositions := make(map[uint8]loginticket.Quickslot, len(character.Quickslots))
	for _, quickslot := range character.Quickslots {
		if !validQuickslotTuple(quickslot) {
			return fmt.Errorf("%w: quickslot position %d has invalid type %d slot %d", ErrInvalidAccount, quickslot.Position, quickslot.Type, quickslot.Slot)
		}
		if previous, ok := quickslotPositions[quickslot.Position]; ok {
			return fmt.Errorf("%w: quickslot position %d contains type %d slot %d and type %d slot %d", ErrInvalidAccount, quickslot.Position, previous.Type, previous.Slot, quickslot.Type, quickslot.Slot)
		}
		quickslotPositions[quickslot.Position] = quickslot
	}
	return nil
}

func validQuickslotTuple(quickslot loginticket.Quickslot) bool {
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
