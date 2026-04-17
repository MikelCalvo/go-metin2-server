package accountstore

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

var (
	ErrStoreDirRequired = errors.New("account store dir is required")
	ErrLoginRequired    = errors.New("account login is required")
	ErrAccountNotFound  = errors.New("account not found")
)

type Account struct {
	Login      string                  `json:"login"`
	Empire     uint8                   `json:"empire"`
	Characters []loginticket.Character `json:"characters"`
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
	if err := json.Unmarshal(raw, &account); err != nil {
		return Account{}, fmt.Errorf("decode account: %w", err)
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
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close account temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), s.accountPath(account.Login)); err != nil {
		return fmt.Errorf("commit account file: %w", err)
	}
	return nil
}

func (s *FileStore) accountPath(login string) string {
	normalized := strings.ToLower(login)
	filename := hex.EncodeToString([]byte(normalized)) + ".json"
	return filepath.Join(s.dir, filename)
}
