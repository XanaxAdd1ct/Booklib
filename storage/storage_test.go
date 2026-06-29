package storage_test

import (
    "context"
    "os"
    "testing"
    "time"

    "booklib/models"
    "booklib/storage"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestJSONStorage_LoadNotExist(t *testing.T) {
    s := storage.NewJSONStorage("non_existent_file.json")

    books, err := s.Load(context.Background())

    require.NoError(t, err)
    assert.Empty(t, books, "expected empty list for non-existent file")
}

func TestJSONStorage_SaveAndLoad(t *testing.T) {
    f, err := os.CreateTemp("", "booklib_test_*.json")
    require.NoError(t, err)
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

    err = s.Save(ctx, want)
    require.NoError(t, err)

    got, err := s.Load(ctx)
    require.NoError(t, err)

    require.Len(t, got, len(want))
    assert.Equal(t, want[0].ID, got[0].ID)
    assert.Equal(t, want[0].Title, got[0].Title)
    assert.Equal(t, want[0].Author, got[0].Author)
    assert.Equal(t, want[0].Rating, got[0].Rating)
}

func TestJSONStorage_CorruptedFile(t *testing.T) {
    f, err := os.CreateTemp("", "booklib_corrupt_*.json")
    require.NoError(t, err)

    _, err = f.WriteString("not valid json {{{")
    require.NoError(t, err)
    f.Close()
    defer os.Remove(f.Name())

    s := storage.NewJSONStorage(f.Name())

    _, err = s.Load(context.Background())
    assert.Error(t, err, "expected error for corrupted file")
}

func TestJSONStorage_CancelledContext(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    s := storage.NewJSONStorage("any.json")

    _, err := s.Load(ctx)
    assert.Error(t, err, "expected error for cancelled context")
}
