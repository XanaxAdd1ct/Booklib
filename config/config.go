package config

import (
    "fmt"
    "os"
    "strconv"
)

type Config struct {
    StorageFile string
    LogLevel    string
    MaxRating   float64
}

func Load() (*Config, error) {
    maxRating := 5.0

    if v := os.Getenv("MAX_RATING"); v != "" {
        parsed, err := strconv.ParseFloat(v, 64)
        if err != nil {
            return nil, fmt.Errorf("invalid MAX_RATING value: %w", err)
        }
        if parsed <= 0 {
            return nil, fmt.Errorf("MAX_RATING must be positive, got %.1f", parsed)
        }
        maxRating = parsed
    }

    return &Config{
        StorageFile: getEnv("STORAGE_FILE", "library_data.json"),
        LogLevel:    getEnv("LOG_LEVEL", "info"),
        MaxRating:   maxRating,
    }, nil
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}