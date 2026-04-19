package authboot

import (
	"errors"

	authflow "github.com/MikelCalvo/go-metin2-server/internal/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrUnsupportedPhase              = errors.New("auth boot flow phase is not implemented")
	ErrConflictingHandshakeNextPhase = errors.New("auth boot flow requires handshake next phase AUTH")
)

type Config struct {
	Handshake handshake.Config
	Auth      authflow.Config
}

type Flow struct {
	machine   *session.StateMachine
	handshake *handshake.Flow
	auth      *authflow.Flow
	configErr error
}

func NewFlow(cfg Config) *Flow {
	machine := session.NewStateMachine()

	handshakeCfg := cfg.Handshake
	configErr := error(nil)
	if handshakeCfg.NextPhase != "" && handshakeCfg.NextPhase != session.PhaseAuth {
		configErr = ErrConflictingHandshakeNextPhase
	}
	handshakeCfg.NextPhase = session.PhaseAuth

	return &Flow{
		machine:   machine,
		handshake: handshake.NewFlow(machine, handshakeCfg),
		auth:      authflow.NewFlow(machine, cfg.Auth),
		configErr: configErr,
	}
}

func (f *Flow) Start() ([][]byte, error) {
	if f.configErr != nil {
		return nil, f.configErr
	}

	return f.handshake.Start()
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if f.configErr != nil {
		return nil, f.configErr
	}

	switch f.machine.Current() {
	case session.PhaseHandshake:
		return f.handshake.HandleClientFrame(in)
	case session.PhaseAuth:
		return f.auth.HandleClientFrame(in)
	default:
		return nil, ErrUnsupportedPhase
	}
}

func (f *Flow) CurrentPhase() session.Phase {
	return f.machine.Current()
}

func (f *Flow) EncryptLegacyOutgoing(raw []byte) ([]byte, error) {
	if f == nil || f.handshake == nil {
		return append([]byte(nil), raw...), nil
	}
	return f.handshake.EncryptLegacyOutgoing(raw)
}

func (f *Flow) DecryptLegacyIncoming(raw []byte) ([]byte, error) {
	if f == nil || f.handshake == nil {
		return append([]byte(nil), raw...), nil
	}
	return f.handshake.DecryptLegacyIncoming(raw)
}
