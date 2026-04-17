package auth

import (
	"errors"

	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase           = errors.New("auth flow is not in auth phase")
	ErrUnexpectedClientPacket = errors.New("unexpected client packet during auth")
)

type AuthenticateFunc func(authproto.Login3Packet) Result

type Config struct {
	Authenticate AuthenticateFunc
}

type Result struct {
	Accepted      bool
	FailureStatus string
	LoginKey      uint32
}

type Flow struct {
	machine      *session.StateMachine
	authenticate AuthenticateFunc
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	authenticate := cfg.Authenticate
	if authenticate == nil {
		authenticate = func(authproto.Login3Packet) Result {
			return Result{Accepted: false, FailureStatus: "NOID"}
		}
	}

	return &Flow{machine: machine, authenticate: authenticate}
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if f.machine.Current() != session.PhaseAuth {
		return nil, ErrInvalidPhase
	}

	switch in.Header {
	case control.HeaderPong:
		if len(in.Payload) != 0 {
			return nil, control.ErrInvalidPayload
		}
		return nil, nil
	case authproto.HeaderLogin3:
		packet, err := authproto.DecodeLogin3(in)
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

		return [][]byte{authproto.EncodeAuthSuccess(authproto.AuthSuccessPacket{
			LoginKey: result.LoginKey,
			Result:   1,
		})}, nil
	default:
		return nil, ErrUnexpectedClientPacket
	}
}
