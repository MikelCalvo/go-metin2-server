package accountstore

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

// MemoryStore is a hermetic account store for tests and repository-seam proofs.
// It implements Store, AccountLister, and AccountCharacterStateExporter without
// touching the filesystem or opening a database. It deliberately omits backup,
// restore, and crash-temp cleanup: those remain FileStore operator primitives.
type MemoryStore struct {
	mu       sync.RWMutex
	accounts map[string]Account // keyed by strings.ToLower(login)
}

// NewMemoryStore returns an empty hermetic account store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{accounts: make(map[string]Account)}
}

func (s *MemoryStore) Load(login string) (Account, error) {
	if s == nil {
		return Account{}, ErrAccountNotFound
	}
	if strings.TrimSpace(login) == "" {
		return Account{}, ErrLoginRequired
	}
	if login != strings.TrimSpace(login) {
		return Account{}, fmt.Errorf("%w: account login %q has leading or trailing whitespace", ErrInvalidAccount, login)
	}
	if containsNUL(login) {
		return Account{}, fmt.Errorf("%w: account login contains NUL", ErrInvalidAccount)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.accounts[strings.ToLower(login)]
	if !ok {
		return Account{}, ErrAccountNotFound
	}
	cloned := cloneAccount(account)
	if err := validateLoadedAccountForLogin(login, cloned); err != nil {
		return Account{}, err
	}
	return cloned, nil
}

func (s *MemoryStore) Save(account Account) error {
	if s == nil {
		return fmt.Errorf("%w: memory account store is nil", ErrInvalidAccount)
	}
	if strings.TrimSpace(account.Login) == "" {
		return ErrLoginRequired
	}
	if account.Login != strings.TrimSpace(account.Login) {
		return fmt.Errorf("%w: account login %q has leading or trailing whitespace", ErrInvalidAccount, account.Login)
	}
	if containsNUL(account.Login) {
		return fmt.Errorf("%w: account login contains NUL", ErrInvalidAccount)
	}
	account.Characters = normalizeAccountCharacters(account.Characters)
	if err := validateAccount(account); err != nil {
		return err
	}
	if err := validateAccountUniqueInventorySlots(account); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accounts == nil {
		s.accounts = make(map[string]Account)
	}
	s.accounts[strings.ToLower(account.Login)] = cloneAccount(account)
	return nil
}

func (s *MemoryStore) List() ([]Account, error) {
	if s == nil {
		return []Account{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts := make([]Account, 0, len(s.accounts))
	seenLogins := make(map[string]string, len(s.accounts))
	for _, account := range s.accounts {
		cloned := cloneAccount(account)
		normalizedLogin := strings.ToLower(cloned.Login)
		if previousLogin, ok := seenLogins[normalizedLogin]; ok {
			return nil, fmt.Errorf("%w: account login %q duplicates %q", ErrInvalidAccount, cloned.Login, previousLogin)
		}
		seenLogins[normalizedLogin] = cloned.Login
		accounts = append(accounts, cloned)
	}
	sort.Slice(accounts, func(i, j int) bool {
		return strings.ToLower(accounts[i].Login) < strings.ToLower(accounts[j].Login)
	})
	return accounts, nil
}

// ExportAccountCharacterRoster projects committed in-memory snapshots onto the
// 0002 account/character roster migration shape without filesystem I/O.
func (s *MemoryStore) ExportAccountCharacterRoster() (AccountCharacterRosterExport, error) {
	accounts, err := s.List()
	if err != nil {
		return AccountCharacterRosterExport{}, err
	}
	return ExportAccountCharacterRoster(accounts)
}

// ExportCharacterItemState projects committed in-memory snapshots onto the
// 0003 character item-state migration shape without filesystem I/O.
func (s *MemoryStore) ExportCharacterItemState() (CharacterItemStateExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterItemStateExport{}, err
	}
	return ExportCharacterItemState(accounts)
}

// ExportCharacterPointState projects committed in-memory snapshots onto the
// 0011 character point-state migration shape without filesystem I/O.
func (s *MemoryStore) ExportCharacterPointState() (CharacterPointStateExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterPointStateExport{}, err
	}
	return ExportCharacterPointState(accounts)
}

// ExportCharacterMyShopUnitPrices projects committed in-memory snapshots onto
// the 0023 character myshop unit-prices migration shape without filesystem I/O.
func (s *MemoryStore) ExportCharacterMyShopUnitPrices() (CharacterMyShopUnitPricesExport, error) {
	accounts, err := s.List()
	if err != nil {
		return CharacterMyShopUnitPricesExport{}, err
	}
	return ExportCharacterMyShopUnitPrices(accounts)
}

func cloneAccount(account Account) Account {
	return Account{
		Login:      account.Login,
		Empire:     account.Empire,
		Characters: loginticket.CloneCharacters(account.Characters),
	}
}
