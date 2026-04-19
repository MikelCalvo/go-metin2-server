package handshake

import (
	"errors"
	"fmt"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/securecipher"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase           = errors.New("handshake flow is not in handshake phase")
	ErrHandshakeNotStarted    = errors.New("handshake flow has not started")
	ErrHandshakeAlreadyStart  = errors.New("handshake flow has already started")
	ErrUnexpectedClientPacket = errors.New("unexpected client packet during handshake")
	ErrKeyResponseRejected    = errors.New("key response rejected")
	ErrInvalidNextPhase       = errors.New("invalid handshake next phase")
)

type VerifyKeyResponseFunc func(control.KeyResponsePacket) bool

type Config struct {
	KeyChallenge      control.KeyChallengePacket
	KeyComplete       control.KeyCompletePacket
	VerifyKeyResponse VerifyKeyResponseFunc
	NextPhase         session.Phase
	SecureSession     *securecipher.ServerSession
}

type Flow struct {
	machine           *session.StateMachine
	keyChallenge      control.KeyChallengePacket
	keyComplete       control.KeyCompletePacket
	verifyKeyResponse VerifyKeyResponseFunc
	nextPhase         session.Phase
	secureSession     *securecipher.ServerSession
	configErr         error
	started           bool
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	verify := cfg.VerifyKeyResponse
	if verify == nil {
		verify = func(control.KeyResponsePacket) bool {
			return true
		}
	}

	nextPhase := cfg.NextPhase
	configErr := error(nil)
	if nextPhase == "" {
		nextPhase = session.PhaseLogin
	} else if !isValidNextPhase(nextPhase) {
		configErr = ErrInvalidNextPhase
	}

	return &Flow{
		machine:           machine,
		keyChallenge:      cfg.KeyChallenge,
		keyComplete:       cfg.KeyComplete,
		verifyKeyResponse: verify,
		nextPhase:         nextPhase,
		secureSession:     cfg.SecureSession,
		configErr:         configErr,
	}
}

func (f *Flow) Start() ([][]byte, error) {
	if f.configErr != nil {
		return nil, f.configErr
	}

	if f.machine.Current() != session.PhaseHandshake {
		return nil, ErrInvalidPhase
	}

	if f.started {
		return nil, ErrHandshakeAlreadyStart
	}

	f.started = true
	if f.secureSession != nil {
		challenge, err := f.secureSession.Start()
		if err != nil {
			return nil, err
		}
		return [][]byte{control.EncodeKeyChallenge(challenge)}, nil
	}
	return [][]byte{control.EncodeKeyChallenge(f.keyChallenge)}, nil
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if f.configErr != nil {
		return nil, f.configErr
	}

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

		if f.secureSession != nil {
			complete, err := f.secureSession.HandleKeyResponse(packet)
			if err != nil {
				_ = f.machine.Transition(session.PhaseClose)
				return nil, fmt.Errorf("%w: %v", ErrKeyResponseRejected, err)
			}
			if err := f.machine.Transition(f.nextPhase); err != nil {
				return nil, err
			}
			phaseNext, err := control.EncodePhase(f.nextPhase)
			if err != nil {
				return nil, err
			}
			return [][]byte{
				control.EncodeKeyComplete(complete),
				phaseNext,
			}, nil
		}

		if !f.verifyKeyResponse(packet) {
			_ = f.machine.Transition(session.PhaseClose)
			return nil, ErrKeyResponseRejected
		}

		if err := f.machine.Transition(f.nextPhase); err != nil {
			return nil, err
		}

		phaseNext, err := control.EncodePhase(f.nextPhase)
		if err != nil {
			return nil, err
		}

		return [][]byte{
			control.EncodeKeyComplete(f.keyComplete),
			phaseNext,
		}, nil
	default:
		return nil, ErrUnexpectedClientPacket
	}
}

func isValidNextPhase(phase session.Phase) bool {
	return phase == session.PhaseLogin || phase == session.PhaseAuth
}

func (f *Flow) EncryptLegacyOutgoing(raw []byte) ([]byte, error) {
	if f == nil || f.secureSession == nil {
		return append([]byte(nil), raw...), nil
	}
	return f.secureSession.EncryptOutgoing(raw)
}

func (f *Flow) DecryptLegacyIncoming(raw []byte) ([]byte, error) {
	if f == nil || f.secureSession == nil {
		return append([]byte(nil), raw...), nil
	}
	return f.secureSession.DecryptIncoming(raw)
}
