package loginticket

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryStore is a hermetic login-ticket store for tests and repository-seam
// proofs. It implements Store and AuthLoginTicketHandoffExporter without
// touching the filesystem or opening a database. It deliberately omits backup,
// restore, crash-temp cleanup, and issued-before cleanup: those remain
// FileStore operator primitives.
type MemoryStore struct {
	mu      sync.RWMutex
	tickets map[uint32]Ticket // keyed by LoginKey
}

// NewMemoryStore returns an empty hermetic login-ticket store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{tickets: make(map[uint32]Ticket)}
}

func (s *MemoryStore) Issue(ticket Ticket) error {
	if s == nil {
		return fmt.Errorf("%w: memory login ticket store is nil", ErrInvalidTicket)
	}
	if ticket.IssuedAt.IsZero() {
		ticket.IssuedAt = time.Now().UTC()
	}
	ticket.Characters = CloneCharacters(ticket.Characters)
	normalizeCharactersItemState(ticket.Characters)
	if err := validateTicket(ticket); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tickets == nil {
		s.tickets = make(map[uint32]Ticket)
	}
	if _, exists := s.tickets[ticket.LoginKey]; exists {
		return ErrTicketExists
	}
	s.tickets[ticket.LoginKey] = cloneTicket(ticket)
	return nil
}

func (s *MemoryStore) Load(login string, loginKey uint32) (Ticket, error) {
	return s.read(login, loginKey, false)
}

func (s *MemoryStore) Consume(login string, loginKey uint32) (Ticket, error) {
	return s.read(login, loginKey, true)
}

func (s *MemoryStore) List() ([]Ticket, error) {
	if s == nil {
		return []Ticket{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	tickets := make([]Ticket, 0, len(s.tickets))
	for _, ticket := range s.tickets {
		tickets = append(tickets, cloneTicket(ticket))
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

// ExportAuthLoginTicketHandoff projects committed in-memory tickets onto the
// 0007 auth login-ticket handoff migration shape without filesystem I/O. An
// empty pending set is treated as an empty export, matching FileStore.
func (s *MemoryStore) ExportAuthLoginTicketHandoff() (AuthLoginTicketHandoffExport, error) {
	tickets, err := s.List()
	if err != nil {
		return AuthLoginTicketHandoffExport{}, err
	}
	return ExportAuthLoginTicketHandoff(tickets)
}

func (s *MemoryStore) read(login string, loginKey uint32, consume bool) (Ticket, error) {
	if s == nil {
		return Ticket{}, ErrTicketNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tickets == nil {
		return Ticket{}, ErrTicketNotFound
	}
	ticket, ok := s.tickets[loginKey]
	if !ok {
		return Ticket{}, ErrTicketNotFound
	}
	if ticket.Login != login || ticket.LoginKey != loginKey {
		return Ticket{}, ErrTicketLoginMismatch
	}
	cloned := cloneTicket(ticket)
	if consume {
		delete(s.tickets, loginKey)
	}
	return cloned, nil
}

func cloneTicket(ticket Ticket) Ticket {
	return Ticket{
		Login:      ticket.Login,
		LoginKey:   ticket.LoginKey,
		Empire:     ticket.Empire,
		IssuedAt:   ticket.IssuedAt,
		Characters: CloneCharacters(ticket.Characters),
	}
}
