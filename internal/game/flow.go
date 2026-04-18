package game

import (
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase           = errors.New("game flow is not in game phase")
	ErrUnexpectedClientPacket = errors.New("unexpected client packet during game")
)

type HandleMoveFunc func(movep.MovePacket) Result

type HandleSyncPositionFunc func(movep.SyncPositionPacket) SyncPositionResult

type Config struct {
	HandleMove         HandleMoveFunc
	HandleSyncPosition HandleSyncPositionFunc
}

type Result struct {
	Accepted    bool
	Replication movep.MoveAckPacket
}

type SyncPositionResult struct {
	Accepted        bool
	Synchronization movep.SyncPositionAckPacket
}

type Flow struct {
	machine            *session.StateMachine
	handleMove         HandleMoveFunc
	handleSyncPosition HandleSyncPositionFunc
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	handler := cfg.HandleMove
	if handler == nil {
		handler = func(movep.MovePacket) Result { return Result{Accepted: false} }
	}
	syncHandler := cfg.HandleSyncPosition
	if syncHandler == nil {
		syncHandler = func(movep.SyncPositionPacket) SyncPositionResult { return SyncPositionResult{Accepted: false} }
	}
	return &Flow{machine: machine, handleMove: handler, handleSyncPosition: syncHandler}
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if f.machine.Current() != session.PhaseGame {
		return nil, ErrInvalidPhase
	}
	switch in.Header {
	case control.HeaderPong:
		if _, err := control.DecodePong(in); err != nil {
			return nil, err
		}
		return nil, nil
	case movep.HeaderMove:
		packet, err := movep.DecodeMove(in)
		if err != nil {
			return nil, err
		}
		result := f.handleMove(packet)
		if !result.Accepted {
			return nil, nil
		}
		return [][]byte{movep.EncodeMoveAck(result.Replication)}, nil
	case movep.HeaderSyncPosition:
		packet, err := movep.DecodeSyncPosition(in)
		if err != nil {
			return nil, err
		}
		result := f.handleSyncPosition(packet)
		if !result.Accepted {
			return nil, nil
		}
		return [][]byte{movep.EncodeSyncPositionAck(result.Synchronization)}, nil
	default:
		return nil, ErrUnexpectedClientPacket
	}
}
