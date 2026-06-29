package storage_test

import (
    "context"
    "os"
    "testing"
    "time"

    "booklib/models"
    "booklib/storage"
)

func TestJSONStorage_LoadNotExist(t *testing.T) {
    s := storage.NewJSONStorage("non_existent_file.json")

    books, err := s.Load(context.Background())
    if err != nil {
        t.Fatalf("expected nil error, got: %v", err)
    }
    if len(books) != 0 {
        t.Fatalf("expected empty list, got %d books", len(books))
    }
}

func TestJSONStorage_SaveAndLoad(t *testing.T) {
    f, err := os.CreateTemp("", "booklib_test_*.json")
    if err != nil {
        t.Fatal(err)
    }
    f.Close()
    defer os.Remove(f.Name())

    s := storage.NewJSONStorage(f.Name())
    ctx := context.Background()

    want := []models.Book{
        {
            ID:      "abc123",
            Title:   "War and Peace",
            Author:  "Tolstoy",
            Genre:   "Novel",
            Rating:  5.0,
            AddedAt: time.Now().Truncate(time.Second),
        },
    }

    if err := s.Save(ctx, want); err != nil {
        t.Fatalf("Save returned error: %v", err)
    }

    got, err := s.Load(ctx)
    if err != nil {
        t.Fatalf("Load returned error: %v", err)
    }
    if len(got) != len(want) {
        t.Fatalf("expected %d books, got %d", len(want), len(got))
    }
    if got[0].ID != want[0].ID || got[0].Title != want[0].Title {
        t.Errorf("data mismatch: got %+v, want %+v", got[0], want[0])
    }
}

func TestJSONStorage_CorruptedFile(t *testing.T) {
    f, err := os.CreateTemp("", "booklib_corrupt_*.json")
    if err != nil {
        t.Fatal(err)
    }
    f.WriteString("not valid json {{{")
    f.Close()
    defer os.Remove(f.Name())

    s := storage.NewJSONStorage(f.Name())

    _, err = s.Load(context.Background())
    if err == nil {
        t.Fatal("expected error for corrupted file, got nil")
    }
}

func TestJSONStorage_CancelledContext(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    s := storage.NewJSONStorage("any.json")

    _, err := s.Load(ctx)
    if err == nil {
        t.Fatal("expected error for cancelled context, got nil")
    }
}