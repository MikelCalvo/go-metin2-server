package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/ops"
)

func Run(ctx context.Context, cfg config.Service, logger *slog.Logger, newSession SessionFactory) error {
	return RunWithOpsHandler(ctx, cfg, logger, newSession, nil)
}

func RunWithOpsHandler(ctx context.Context, cfg config.Service, logger *slog.Logger, newSession SessionFactory, opsHandler http.Handler) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := serveOps(runCtx, cfg, logger, opsHandler); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	if cfg.LegacyAddr != "" && newSession != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ListenAndServeLegacy(runCtx, cfg.LegacyAddr, logger, newSession); err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}()
	}

	select {
	case <-ctx.Done():
		cancel()
		wg.Wait()
		select {
		case err := <-errCh:
			return err
		default:
			return nil
		}
	case err := <-errCh:
		cancel()
		wg.Wait()
		return err
	}
}

func serveOps(ctx context.Context, cfg config.Service, logger *slog.Logger, handler http.Handler) error {
	if handler == nil {
		handler = ops.NewPprofMux(cfg.Name)
	}
	server := &http.Server{
		Addr:              cfg.PprofAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		if logger != nil {
			logger.Info("ops server listening", "addr", cfg.PprofAddr)
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if logger != nil {
			logger.Info("shutdown requested")
		}
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
