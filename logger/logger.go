package logger

import (
    "log/slog"
    "os"
)

func New(level string) *slog.Logger {
    var l slog.Level
    switch level {
    case "debug":
        l = slog.LevelDebug
    case "warn":
        l = slog.LevelWarn
    case "error":
        l = slog.LevelError
    default:
        l = slog.LevelInfo
    }

    opts := &slog.HandlerOptions{Level: l}
    return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}