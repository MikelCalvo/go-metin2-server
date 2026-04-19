package login

import (
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase           = errors.New("login flow is not in login phase")
	ErrUnexpectedClientPacket = errors.New("unexpected client packet during login")
)

type AuthenticateFunc func(loginproto.Login2Packet) Result

type Config struct {
	Authenticate AuthenticateFunc
}

type Result struct {
	Accepted      bool
	FailureStatus string
	Empire        uint8
	LoginSuccess4 loginproto.LoginSuccess4Packet
}

type Flow struct {
	machine      *session.StateMachine
	authenticate AuthenticateFunc
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	authenticate := cfg.Authenticate
	if authenticate == nil {
		authenticate = func(loginproto.Login2Packet) Result {
			return Result{Accepted: false, FailureStatus: "NOID"}
		}
	}

	return &Flow{
		machine:      machine,
		authenticate: authenticate,
	}
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if f.machine.Current() != session.PhaseLogin {
		return nil, ErrInvalidPhase
	}

	if in.Header != loginproto.HeaderLogin2 {
		return nil, ErrUnexpectedClientPacket
	}

	packet, err := loginproto.DecodeLogin2(in)
	if err != nil {
		return nil, err
	}

	result := f.authenticate(packet)
	if !result.Accepted {
		failure, err := loginproto.EncodeLoginFailure(loginproto.LoginFailurePacket{Status: result.FailureStatus})
		if err != nil {
			return nil, err
		}

		return [][]byte{failure}, nil
	}

	empire := loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: result.Empire})
	phaseSelect, err := control.EncodePhase(session.PhaseSelect)
	if err != nil {
		return nil, err
	}

	loginSuccess, err := loginproto.EncodeLoginSuccess4(result.LoginSuccess4)
	if err != nil {
		return nil, err
	}

	if err := f.machine.Transition(session.PhaseSelect); err != nil {
		return nil, err
	}

	return [][]byte{loginSuccess, empire, phaseSelect}, nil
}
