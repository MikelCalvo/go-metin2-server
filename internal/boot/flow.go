package boot

import (
	"errors"

	gameflow "github.com/MikelCalvo/go-metin2-server/internal/game"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
	worldentry "github.com/MikelCalvo/go-metin2-server/internal/worldentry"
)

var (
	ErrUnsupportedPhase              = errors.New("boot flow phase is not implemented")
	ErrConflictingHandshakeNextPhase = errors.New("boot flow requires handshake next phase LOGIN")
)

type Config struct {
	Handshake  handshake.Config
	Login      loginflow.Config
	WorldEntry worldentry.Config
	Game       gameflow.Config
}

type Flow struct {
	machine    *session.StateMachine
	handshake  *handshake.Flow
	login      *loginflow.Flow
	worldEntry *worldentry.Flow
	game       *gameflow.Flow
	configErr  error
}

func NewFlow(cfg Config) *Flow {
	machine := session.NewStateMachine()

	handshakeCfg := cfg.Handshake
	configErr := error(nil)
	if handshakeCfg.NextPhase != "" && handshakeCfg.NextPhase != session.PhaseLogin {
		configErr = ErrConflictingHandshakeNextPhase
	}
	handshakeCfg.NextPhase = session.PhaseLogin

	return &Flow{
		machine:    machine,
		handshake:  handshake.NewFlow(machine, handshakeCfg),
		login:      loginflow.NewFlow(machine, cfg.Login),
		worldEntry: worldentry.NewFlow(machine, cfg.WorldEntry),
		game:       gameflow.NewFlow(machine, cfg.Game),
		configErr:  configErr,
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
	case session.PhaseLogin:
		return f.login.HandleClientFrame(in)
	case session.PhaseSelect, session.PhaseLoading:
		return f.worldEntry.HandleClientFrame(in)
	case session.PhaseGame:
		return f.game.HandleClientFrame(in)
	default:
		return nil, ErrUnsupportedPhase
	}
}

func (f *Flow) CurrentPhase() session.Phase {
	return f.machine.Current()
}
