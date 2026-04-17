package service

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "net/http"
    "time"

    "github.com/MikelCalvo/go-metin2-server/internal/config"
    "github.com/MikelCalvo/go-metin2-server/internal/ops"
)

func Run(ctx context.Context, cfg config.Service, logger *slog.Logger) error {
    mux := ops.NewPprofMux(cfg.Name)
    server := &http.Server{
        Addr:              cfg.PprofAddr,
        Handler:           mux,
        ReadHeaderTimeout: 5 * time.Second,
    }

    errCh := make(chan error, 1)

    go func() {
        logger.Info("ops server listening", "addr", cfg.PprofAddr)
        if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- fmt.Errorf("listen and serve: %w", err)
        }
    }()

    select {
    case <-ctx.Done():
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        logger.Info("shutdown requested")
        return server.Shutdown(shutdownCtx)
    case err := <-errCh:
        return err
    }
}
