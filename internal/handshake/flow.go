package handshake

import (
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase           = errors.New("handshake flow is not in handshake phase")
	ErrHandshakeNotStarted    = errors.New("handshake flow has not started")
	ErrHandshakeAlreadyStart  = errors.New("handshake flow has already started")
	ErrUnexpectedClientPacket = errors.New("unexpected client packet during handshake")
	ErrKeyResponseRejected    = errors.New("key response rejected")
)

type VerifyKeyResponseFunc func(control.KeyResponsePacket) bool

type Config struct {
	KeyChallenge      control.KeyChallengePacket
	KeyComplete       control.KeyCompletePacket
	VerifyKeyResponse VerifyKeyResponseFunc
}

type Flow struct {
	machine           *session.StateMachine
	keyChallenge      control.KeyChallengePacket
	keyComplete       control.KeyCompletePacket
	verifyKeyResponse VerifyKeyResponseFunc
	started           bool
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	verify := cfg.VerifyKeyResponse
	if verify == nil {
		verify = func(control.KeyResponsePacket) bool {
			return true
		}
	}

	return &Flow{
		machine:           machine,
		keyChallenge:      cfg.KeyChallenge,
		keyComplete:       cfg.KeyComplete,
		verifyKeyResponse: verify,
	}
}

func (f *Flow) Start() ([][]byte, error) {
	if f.machine.Current() != session.PhaseHandshake {
		return nil, ErrInvalidPhase
	}

	if f.started {
		return nil, ErrHandshakeAlreadyStart
	}

	f.started = true
	return [][]byte{control.EncodeKeyChallenge(f.keyChallenge)}, nil
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if f.machine.Current() != session.PhaseHandshake {
		return nil, ErrInvalidPhase
	}

	if !f.started {
		return nil, ErrHandshakeNotStarted
	}

	switch in.Header {
	case control.HeaderPong:
		if len(in.Payload) != 0 {
			return nil, control.ErrInvalidPayload
		}

		return nil, nil
	case control.HeaderKeyResponse:
		packet, err := control.DecodeKeyResponse(in)
		if err != nil {
			return nil, err
		}

		if !f.verifyKeyResponse(packet) {
			_ = f.machine.Transition(session.PhaseClose)
			return nil, ErrKeyResponseRejected
		}

		if err := f.machine.Transition(session.PhaseLogin); err != nil {
			return nil, err
		}

		phaseLogin, err := control.EncodePhase(session.PhaseLogin)
		if err != nil {
			return nil, err
		}

		return [][]byte{
			control.EncodeKeyComplete(f.keyComplete),
			phaseLogin,
		}, nil
	default:
		return nil, ErrUnexpectedClientPacket
	}
}
