package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "booklib/cli"
    "booklib/config"
    "booklib/library"
    "booklib/logger"
    "booklib/storage"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
        os.Exit(1)
    }

    log := logger.New(cfg.LogLevel)

    store := storage.NewJSONStorage(cfg.StorageFile)

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    svc, err := library.NewService(ctx, store, log, cfg.MaxRating)
    if err != nil {
        log.Error("failed to initialize service", "error", err)
        os.Exit(1)
    }

    app := cli.New(svc, log, os.Stdin, os.Stdout)
    app.Run(ctx)
}