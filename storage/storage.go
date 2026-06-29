package storage

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    "booklib/models"
)

type Storage interface {
    Load(ctx context.Context) ([]models.Book, error)
    Save(ctx context.Context, books []models.Book) error
}

type JSONStorage struct {
    filename string
}

func NewJSONStorage(filename string) *JSONStorage {
    return &JSONStorage{filename: filename}
}

func (s *JSONStorage) Load(ctx context.Context) ([]models.Book, error) {
    if err := ctx.Err(); err != nil {
        return nil, fmt.Errorf("operation canceled: %w", err)
    }

    file, err := os.Open(s.filename)
    if err != nil {
        if os.IsNotExist(err) {
            return []models.Book{}, nil
        }
        return nil, fmt.Errorf("failed to open storage file: %w", err)
    }
    defer file.Close()

    var books []models.Book
    if err := json.NewDecoder(file).Decode(&books); err != nil {
        return nil, fmt.Errorf("storage file is corrupted: %w", err)
    }

    return books, nil
}

func (s *JSONStorage) Save(ctx context.Context, books []models.Book) error {
    if err := ctx.Err(); err != nil {
        return fmt.Errorf("operation canceled: %w", err)
    }

    tmpFile := s.filename + ".tmp"

    file, err := os.Create(tmpFile)
    if err != nil {
        return fmt.Errorf("failed to create temp file: %w", err)
    }

    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "  ")

    if err := encoder.Encode(books); err != nil {
        file.Close()
        os.Remove(tmpFile)
        return fmt.Errorf("failed to encode data: %w", err)
    }

    if err := file.Close(); err != nil {
        os.Remove(tmpFile)
        return fmt.Errorf("failed to flush data to disk: %w", err)
    }

    if err := os.Rename(tmpFile, s.filename); err != nil {
        os.Remove(tmpFile)
        return fmt.Errorf("failed to atomically replace file: %w", err)
    }

    return nil
}