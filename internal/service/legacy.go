package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

type SessionFlow interface {
	Start() ([][]byte, error)
	HandleClientFrame(frame.Frame) ([][]byte, error)
}

type ServerFrameSource interface {
	FlushServerFrames() ([][]byte, error)
}

type SessionFactory func() SessionFlow

type secureLegacySessionFlow interface {
	EncryptLegacyOutgoing([]byte) ([]byte, error)
	DecryptLegacyIncoming([]byte) ([]byte, error)
}

type phaseAwareSessionFlow interface {
	CurrentPhase() session.Phase
}

func ListenAndServeLegacy(ctx context.Context, addr string, logger *slog.Logger, newSession SessionFactory) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen legacy tcp: %w", err)
	}

	if logger != nil {
		logger.Info("legacy server listening", "addr", listener.Addr().String())
	}

	return ServeLegacy(ctx, listener, logger, newSession)
}

func ServeLegacy(ctx context.Context, listener net.Listener, logger *slog.Logger, newSession SessionFactory) error {
	if newSession == nil {
		defer listener.Close()
		<-ctx.Done()
		return nil
	}

	defer listener.Close()
	tracker := newConnTracker()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
		tracker.closeAll()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				tracker.wait()
				return nil
			}
			tracker.closeAll()
			tracker.wait()
			return fmt.Errorf("accept legacy tcp: %w", err)
		}

		tracker.add(conn)
		go func(conn net.Conn) {
			defer conn.Close()
			defer tracker.done(conn)

			if err := serveLegacyConn(ctx, conn, logger, newSession()); err != nil && logger != nil {
				logger.Warn("legacy session closed with error", "remote_addr", conn.RemoteAddr().String(), "err", err)
			}
		}(conn)
	}
}

var ErrPipelinedPlaintextAfterSecureHandshake = errors.New("pipelined plaintext frame after secure handshake activation")

func serveLegacyConn(ctx context.Context, conn net.Conn, logger *slog.Logger, flow SessionFlow) error {
	if closer, ok := flow.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}

	out, err := flow.Start()
	if err != nil {
		return fmt.Errorf("start session flow: %w", err)
	}

	remoteAddr := conn.RemoteAddr().String()
	lastPhase, hasPhase := currentSessionPhase(flow)
	secureFlow, hasSecureFlow := flow.(secureLegacySessionFlow)
	if hasPhase && logger != nil {
		logger.Info("legacy session started", "remote_addr", remoteAddr, "phase", lastPhase)
	}

	for _, raw := range out {
		if hasSecureFlow {
			raw, err = secureFlow.EncryptLegacyOutgoing(raw)
			if err != nil {
				return fmt.Errorf("encrypt session start frame: %w", err)
			}
		}
		if err := writeLegacyFrame(conn, raw); err != nil {
			return fmt.Errorf("write session start frame: %w", err)
		}
	}

	decoder := frame.NewDecoder(8192)
	buffer := make([]byte, 8192)

	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := flushServerFrames(conn, flow); err != nil {
			return err
		}

		if err := conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		n, err := conn.Read(buffer)
		if n > 0 {
			incomingBytes := buffer[:n]
			if hasSecureFlow {
				incomingBytes, err = secureFlow.DecryptLegacyIncoming(incomingBytes)
				if err != nil {
					return fmt.Errorf("decrypt legacy bytes: %w", err)
				}
			}
			frames, decodeErr := decoder.Feed(incomingBytes)
			if decodeErr != nil {
				return fmt.Errorf("decode legacy frame: %w", decodeErr)
			}

			for idx, incoming := range frames {
				phaseBefore, hadPhaseBefore := currentSessionPhase(flow)
				out, handleErr := flow.HandleClientFrame(incoming)
				if handleErr != nil {
					return handleErr
				}

				phaseAfter, hadPhaseAfter := currentSessionPhase(flow)
				if hadPhaseAfter {
					if !hasPhase || phaseAfter != lastPhase {
						if logger != nil {
							logger.Info("legacy session phase changed", "remote_addr", remoteAddr, "from_phase", lastPhase, "to_phase", phaseAfter)
						}
						lastPhase = phaseAfter
						hasPhase = true
					}
				}

				for _, raw := range out {
					if hasSecureFlow {
						raw, err = secureFlow.EncryptLegacyOutgoing(raw)
						if err != nil {
							return fmt.Errorf("encrypt legacy frame: %w", err)
						}
					}
					if writeErr := writeLegacyFrame(conn, raw); writeErr != nil {
						return fmt.Errorf("write legacy frame: %w", writeErr)
					}
				}

				if hasSecureFlow && crossedSecureLegacyBoundary(hadPhaseBefore, phaseBefore, hadPhaseAfter, phaseAfter) {
					if idx < len(frames)-1 || decoder.BufferedLen() > 0 {
						return ErrPipelinedPlaintextAfterSecureHandshake
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}

			return fmt.Errorf("read legacy frame: %w", err)
		}
	}
}

func flushServerFrames(conn net.Conn, flow SessionFlow) error {
	source, ok := flow.(ServerFrameSource)
	if !ok {
		return nil
	}

	out, err := source.FlushServerFrames()
	if err != nil {
		return fmt.Errorf("flush server frames: %w", err)
	}

	for _, raw := range out {
		if secureFlow, ok := flow.(secureLegacySessionFlow); ok {
			var err error
			raw, err = secureFlow.EncryptLegacyOutgoing(raw)
			if err != nil {
				return fmt.Errorf("encrypt server frame: %w", err)
			}
		}
		if err := writeLegacyFrame(conn, raw); err != nil {
			return fmt.Errorf("write server frame: %w", err)
		}
	}

	return nil
}

func currentSessionPhase(flow SessionFlow) (session.Phase, bool) {
	phaseAware, ok := flow.(phaseAwareSessionFlow)
	if !ok {
		return "", false
	}
	return phaseAware.CurrentPhase(), true
}

func crossedSecureLegacyBoundary(hadBefore bool, before session.Phase, hadAfter bool, after session.Phase) bool {
	return hadBefore && hadAfter && before == session.PhaseHandshake && after != session.PhaseHandshake
}

func writeLegacyFrame(conn net.Conn, raw []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	for len(raw) > 0 {
		n, err := conn.Write(raw)
		if err != nil {
			return err
		}
		raw = raw[n:]
	}

	return nil
}

type connTracker struct {
	mu    sync.Mutex
	conns map[net.Conn]struct{}
	wg    sync.WaitGroup
}

func newConnTracker() *connTracker {
	return &connTracker{conns: make(map[net.Conn]struct{})}
}

func (t *connTracker) add(conn net.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.conns[conn] = struct{}{}
	t.wg.Add(1)
}

func (t *connTracker) done(conn net.Conn) {
	t.mu.Lock()
	delete(t.conns, conn)
	t.mu.Unlock()
	t.wg.Done()
}

func (t *connTracker) closeAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for conn := range t.conns {
		_ = conn.Close()
	}
}

func (t *connTracker) wait() {
	t.wg.Wait()
}
