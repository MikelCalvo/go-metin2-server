package session

import "errors"

type Phase string

const (
	PhaseHandshake Phase = "HANDSHAKE"
	PhaseLogin     Phase = "LOGIN"
	PhaseSelect    Phase = "SELECT"
	PhaseLoading   Phase = "LOADING"
	PhaseGame      Phase = "GAME"
	PhaseDead      Phase = "DEAD"
	PhaseClose     Phase = "CLOSE"
)

var (
	ErrUnknownPhase      = errors.New("unknown phase")
	ErrInvalidTransition = errors.New("invalid phase transition")
)

type StateMachine struct {
	current Phase
}

func NewStateMachine() *StateMachine {
	return NewStateMachineAt(PhaseHandshake)
}

func NewStateMachineAt(initial Phase) *StateMachine {
	return &StateMachine{current: initial}
}

func (m *StateMachine) Current() Phase {
	return m.current
}

func (m *StateMachine) Transition(next Phase) error {
	if !isKnownPhase(next) {
		return ErrUnknownPhase
	}

	if !isKnownPhase(m.current) {
		return ErrUnknownPhase
	}

	if !m.CanTransition(next) {
		return ErrInvalidTransition
	}

	m.current = next
	return nil
}

func (m *StateMachine) CanTransition(next Phase) bool {
	if next == PhaseClose {
		return isKnownPhase(m.current)
	}

	switch m.current {
	case PhaseHandshake:
		return next == PhaseLogin
	case PhaseLogin:
		return next == PhaseSelect
	case PhaseSelect:
		return next == PhaseLoading
	case PhaseLoading:
		return next == PhaseGame
	case PhaseGame, PhaseDead, PhaseClose:
		return false
	default:
		return false
	}
}

func isKnownPhase(phase Phase) bool {
	switch phase {
	case PhaseHandshake, PhaseLogin, PhaseSelect, PhaseLoading, PhaseGame, PhaseDead, PhaseClose:
		return true
	default:
		return false
	}
}
