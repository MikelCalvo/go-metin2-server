package main

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
    "github.com/MikelCalvo/go-metin2-server/internal/config"
    "github.com/MikelCalvo/go-metin2-server/internal/service"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
        "service", "gamed",
        "version", buildinfo.Version,
        "commit", buildinfo.Commit,
        "build_date", buildinfo.BuildDate,
    )

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    cfg := config.LoadService("gamed", ":6060")
    if err := service.Run(ctx, cfg, logger); err != nil {
        logger.Error("service stopped with error", "err", err)
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
