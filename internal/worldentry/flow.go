package worldentry

import (
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase            = errors.New("world-entry flow is not in select/loading phase")
	ErrUnexpectedClientPacket  = errors.New("unexpected client packet during world entry")
	ErrCharacterSelectRejected = errors.New("character select rejected")
	ErrEmpireSelectRejected    = errors.New("empire select rejected")
)

type EmpireSelectFunc func(empire uint8) EmpireResult

type CreateCharacterFunc func(worldproto.CharacterCreatePacket) CreateResult

type DeleteCharacterFunc func(worldproto.CharacterDeletePacket) DeleteResult

type SelectCharacterFunc func(index uint8) Result

type EnterGameFunc func() EnterGameResult

type Config struct {
	SelectEmpire    EmpireSelectFunc
	CreateCharacter CreateCharacterFunc
	DeleteCharacter DeleteCharacterFunc
	SelectCharacter SelectCharacterFunc
	EnterGame       EnterGameFunc
}

type EmpireResult struct {
	Accepted bool
	Empire   uint8
}

type CreateResult struct {
	Accepted    bool
	FailureType uint8
	Player      worldproto.PlayerCreateSuccessPacket
}

type DeleteResult struct {
	Accepted bool
	Index    uint8
}

type Result struct {
	Accepted      bool
	MainCharacter worldproto.MainCharacterPacket
	PlayerPoints  worldproto.PlayerPointsPacket
}

type EnterGameResult struct {
	Frames [][]byte
}

type Flow struct {
	machine         *session.StateMachine
	selectEmpire    EmpireSelectFunc
	createCharacter CreateCharacterFunc
	deleteCharacter DeleteCharacterFunc
	selectCharacter SelectCharacterFunc
	enterGame       EnterGameFunc
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	empireSelector := cfg.SelectEmpire
	if empireSelector == nil {
		empireSelector = func(uint8) EmpireResult { return EmpireResult{Accepted: false} }
	}
	creator := cfg.CreateCharacter
	if creator == nil {
		creator = func(worldproto.CharacterCreatePacket) CreateResult {
			return CreateResult{Accepted: false, FailureType: 0}
		}
	}
	deleter := cfg.DeleteCharacter
	if deleter == nil {
		deleter = func(worldproto.CharacterDeletePacket) DeleteResult {
			return DeleteResult{Accepted: false}
		}
	}
	selector := cfg.SelectCharacter
	if selector == nil {
		selector = func(uint8) Result { return Result{Accepted: false} }
	}
	enterGame := cfg.EnterGame
	if enterGame == nil {
		enterGame = func() EnterGameResult { return EnterGameResult{} }
	}
	return &Flow{machine: machine, selectEmpire: empireSelector, createCharacter: creator, deleteCharacter: deleter, selectCharacter: selector, enterGame: enterGame}
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	switch f.machine.Current() {
	case session.PhaseSelect:
		switch in.Header {
		case loginproto.HeaderEmpireSelect:
			packet, err := loginproto.DecodeEmpireSelect(in)
			if err != nil {
				return nil, err
			}
			result := f.selectEmpire(packet.Empire)
			if !result.Accepted {
				return nil, ErrEmpireSelectRejected
			}
			return [][]byte{loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: result.Empire})}, nil
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
		case worldproto.HeaderCharacterDelete:
			packet, err := worldproto.DecodeCharacterDelete(in)
			if err != nil {
				return nil, err
			}
			result := f.deleteCharacter(packet)
			if !result.Accepted {
				return [][]byte{worldproto.EncodePlayerDeleteFailure()}, nil
			}
			return [][]byte{worldproto.EncodePlayerDeleteSuccess(worldproto.PlayerDeleteSuccessPacket{Index: result.Index})}, nil
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
		switch in.Header {
		case control.HeaderClientVersion:
			if _, err := control.DecodeClientVersion(in); err != nil {
				return nil, err
			}
			return nil, nil
		case worldproto.HeaderEnterGame:
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
			result := f.enterGame()
			out := make([][]byte, 0, 1+len(result.Frames))
			out = append(out, phaseGame)
			out = append(out, result.Frames...)
			return out, nil
		default:
			return nil, ErrUnexpectedClientPacket
		}
	default:
		return nil, ErrInvalidPhase
	}
}
