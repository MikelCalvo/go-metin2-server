package worldentry

import (
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase            = errors.New("world-entry flow is not in select/loading phase")
	ErrUnexpectedClientPacket  = errors.New("unexpected client packet during world entry")
	ErrCharacterSelectRejected = errors.New("character select rejected")
)

type CreateCharacterFunc func(worldproto.CharacterCreatePacket) CreateResult

type SelectCharacterFunc func(index uint8) Result

type Config struct {
	CreateCharacter CreateCharacterFunc
	SelectCharacter SelectCharacterFunc
}

type CreateResult struct {
	Accepted    bool
	FailureType uint8
	Player      worldproto.PlayerCreateSuccessPacket
}

type Result struct {
	Accepted      bool
	MainCharacter worldproto.MainCharacterPacket
	PlayerPoints  worldproto.PlayerPointsPacket
}

type Flow struct {
	machine         *session.StateMachine
	createCharacter CreateCharacterFunc
	selectCharacter SelectCharacterFunc
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	creator := cfg.CreateCharacter
	if creator == nil {
		creator = func(worldproto.CharacterCreatePacket) CreateResult {
			return CreateResult{Accepted: false, FailureType: 0}
		}
	}
	selector := cfg.SelectCharacter
	if selector == nil {
		selector = func(uint8) Result { return Result{Accepted: false} }
	}
	return &Flow{machine: machine, createCharacter: creator, selectCharacter: selector}
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	switch f.machine.Current() {
	case session.PhaseSelect:
		switch in.Header {
		case worldproto.HeaderCharacterCreate:
			packet, err := worldproto.DecodeCharacterCreate(in)
			if err != nil {
				return nil, err
			}
			result := f.createCharacter(packet)
			if !result.Accepted {
				return [][]byte{worldproto.EncodePlayerCreateFailure(worldproto.PlayerCreateFailurePacket{Type: result.FailureType})}, nil
			}
			success, err := worldproto.EncodePlayerCreateSuccess(result.Player)
			if err != nil {
				return nil, err
			}
			return [][]byte{success}, nil
		case worldproto.HeaderCharacterSelect:
			packet, err := worldproto.DecodeCharacterSelect(in)
			if err != nil {
				return nil, err
			}
			result := f.selectCharacter(packet.Index)
			if !result.Accepted {
				return nil, ErrCharacterSelectRejected
			}
			phaseLoading, err := control.EncodePhase(session.PhaseLoading)
			if err != nil {
				return nil, err
			}
			mainCharacter, err := worldproto.EncodeMainCharacter(result.MainCharacter)
			if err != nil {
				return nil, err
			}
			playerPoints := worldproto.EncodePlayerPoints(result.PlayerPoints)
			if err := f.machine.Transition(session.PhaseLoading); err != nil {
				return nil, err
			}
			return [][]byte{phaseLoading, mainCharacter, playerPoints}, nil
		default:
			return nil, ErrUnexpectedClientPacket
		}
	case session.PhaseLoading:
		if in.Header != worldproto.HeaderEnterGame {
			return nil, ErrUnexpectedClientPacket
		}
		if err := worldproto.DecodeEnterGame(in); err != nil {
			return nil, err
		}
		phaseGame, err := control.EncodePhase(session.PhaseGame)
		if err != nil {
			return nil, err
		}
		if err := f.machine.Transition(session.PhaseGame); err != nil {
			return nil, err
		}
		return [][]byte{phaseGame}, nil
	default:
		return nil, ErrInvalidPhase
	}
}
