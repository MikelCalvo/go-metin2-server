package session

import (
	"errors"
	"testing"
)

func TestNewStateMachineStartsInHandshake(t *testing.T) {
	machine := NewStateMachine()

	if machine.Current() != PhaseHandshake {
		t.Fatalf("expected initial phase %q, got %q", PhaseHandshake, machine.Current())
	}
}

func TestStateMachineAllowsTheBootPathInOrder(t *testing.T) {
	machine := NewStateMachine()

	path := []Phase{
		PhaseLogin,
		PhaseSelect,
		PhaseLoading,
		PhaseGame,
	}

	for _, next := range path {
		if err := machine.Transition(next); err != nil {
			t.Fatalf("transition to %q failed: %v", next, err)
		}
	}

	if machine.Current() != PhaseGame {
		t.Fatalf("expected current phase %q, got %q", PhaseGame, machine.Current())
	}
}

func TestStateMachineAllowsCloseFromAnyPhase(t *testing.T) {
	phases := []Phase{
		PhaseHandshake,
		PhaseLogin,
		PhaseSelect,
		PhaseLoading,
		PhaseGame,
	}

	for _, start := range phases {
		machine := NewStateMachineAt(start)

		if err := machine.Transition(PhaseClose); err != nil {
			t.Fatalf("expected close transition from %q to succeed: %v", start, err)
		}

		if machine.Current() != PhaseClose {
			t.Fatalf("expected current phase %q after close, got %q", PhaseClose, machine.Current())
		}
	}
}

func TestStateMachineRejectsSkippingPhases(t *testing.T) {
	machine := NewStateMachineAt(PhaseLogin)

	err := machine.Transition(PhaseGame)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}

	if machine.Current() != PhaseLogin {
		t.Fatalf("expected phase to remain %q, got %q", PhaseLogin, machine.Current())
	}
}

func TestStateMachineRejectsUnknownTargetPhase(t *testing.T) {
	machine := NewStateMachine()

	err := machine.Transition(Phase("UNKNOWN"))
	if !errors.Is(err, ErrUnknownPhase) {
		t.Fatalf("expected ErrUnknownPhase, got %v", err)
	}
}
