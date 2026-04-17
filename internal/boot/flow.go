package boot

import (
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var ErrUnsupportedPhase = errors.New("boot flow phase is not implemented")

type Config struct {
	Handshake handshake.Config
	Login     loginflow.Config
}

type Flow struct {
	machine   *session.StateMachine
	handshake *handshake.Flow
	login     *loginflow.Flow
}

func NewFlow(cfg Config) *Flow {
	machine := session.NewStateMachine()

	return &Flow{
		machine:   machine,
		handshake: handshake.NewFlow(machine, cfg.Handshake),
		login:     loginflow.NewFlow(machine, cfg.Login),
	}
}

func (f *Flow) Start() ([][]byte, error) {
	return f.handshake.Start()
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	switch f.machine.Current() {
	case session.PhaseHandshake:
		return f.handshake.HandleClientFrame(in)
	case session.PhaseLogin:
		return f.login.HandleClientFrame(in)
	default:
		return nil, ErrUnsupportedPhase
	}
}

func (f *Flow) CurrentPhase() session.Phase {
	return f.machine.Current()
}
