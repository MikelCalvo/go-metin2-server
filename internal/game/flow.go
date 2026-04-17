package game

import (
	"errors"

	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase           = errors.New("game flow is not in game phase")
	ErrUnexpectedClientPacket = errors.New("unexpected client packet during game")
)

type HandleMoveFunc func(movep.MovePacket) Result

type Config struct {
	HandleMove HandleMoveFunc
}

type Result struct {
	Accepted    bool
	Replication movep.MoveAckPacket
}

type Flow struct {
	machine    *session.StateMachine
	handleMove HandleMoveFunc
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	handler := cfg.HandleMove
	if handler == nil {
		handler = func(movep.MovePacket) Result { return Result{Accepted: false} }
	}
	return &Flow{machine: machine, handleMove: handler}
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if f.machine.Current() != session.PhaseGame {
		return nil, ErrInvalidPhase
	}
	if in.Header != movep.HeaderMove {
		return nil, ErrUnexpectedClientPacket
	}
	packet, err := movep.DecodeMove(in)
	if err != nil {
		return nil, err
	}
	result := f.handleMove(packet)
	if !result.Accepted {
		return nil, nil
	}
	return [][]byte{movep.EncodeMoveAck(result.Replication)}, nil
}
