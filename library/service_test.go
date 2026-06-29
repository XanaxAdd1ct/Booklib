package library_test

import (
    "context"
    "errors"
    "log/slog"
    "os"
    "testing"

    "booklib/library"
    "booklib/models"
)

type mockStorage struct {
    books   []models.Book
    loadErr error
    saveErr error
}

func (m *mockStorage) Load(_ context.Context) ([]models.Book, error) {
    return m.books, m.loadErr
}

func (m *mockStorage) Save(_ context.Context, books []models.Book) error {
    if m.saveErr != nil {
        return m.saveErr
    }
    m.books = books
    return nil
}

func newTestService(t *testing.T, books []models.Book) (*library.Service, *mockStorage) {
    t.Helper()
    log := slog.New(slog.NewTextHandler(os.Stderr, nil))
    mock := &mockStorage{books: books}
    svc, err := library.NewService(context.Background(), mock, log, 5.0)
    if err != nil {
        t.Fatalf("failed to create service: %v", err)
    }
    return svc, mock
}

func TestAddBook_Success(t *testing.T) {
    svc, _ := newTestService(t, nil)

    book, err := svc.AddBook(context.Background(), "Master and Margarita", "Bulgakov", "Novel", 5.0)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if book.ID == "" {
        t.Error("ID must not be empty")
    }
    if book.Title != "Master and Margarita" {
        t.Errorf("unexpected title: %s", book.Title)
    }
}

func TestAddBook_InvalidInput(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    cases := []struct {
        name   string
        title  string
        author string
        genre  string
        rating float64
        want   error
    }{
        {"empty title", "", "Author", "Genre", 4.0, library.ErrInvalidInput},
        {"empty author", "Book", "", "Genre", 4.0, library.ErrInvalidInput},
        {"empty genre", "Book", "Author", "", 4.0, library.ErrInvalidInput},
        {"rating below zero", "Book", "Author", "Genre", -1.0, library.ErrInvalidRating},
        {"rating above max", "Book", "Author", "Genre", 6.0, library.ErrInvalidRating},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := svc.AddBook(ctx, tc.title, tc.author, tc.genre, tc.rating)
            if !errors.Is(err, tc.want) {
                t.Errorf("expected %v, got %v", tc.want, err)
            }
        })
    }
}

func TestAddBook_Duplicate(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    _, err := svc.AddBook(ctx, "Book", "Author", "Genre", 4.0)
    if err != nil {
        t.Fatal(err)
    }

    _, err = svc.AddBook(ctx, "book", "AUTHOR", "Genre", 3.0)
    if !errors.Is(err, library.ErrBookExists) {
        t.Errorf("expected ErrBookExists, got %v", err)
    }
}

func TestAddBook_StorageError_RollsBackCache(t *testing.T) {
    svc, mock := newTestService(t, nil)
    ctx := context.Background()

    mock.saveErr = errors.New("disk full")

    _, err := svc.AddBook(ctx, "Book", "Author", "Genre", 4.0)
    if err == nil {
        t.Fatal("expected error, got nil")
    }

    books := svc.ListBooks(library.SortByTitle)
    if len(books) != 0 {
        t.Errorf("cache should be empty after rollback, got %d books", len(books))
    }
}

func TestUpdateBook_Success(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    book, err := svc.AddBook(ctx, "Old Title", "Author", "Genre", 3.0)
    if err != nil {
        t.Fatal(err)
    }

    updated, err := svc.UpdateBook(ctx, book.ID, "New Title", "Author", "Genre", 4.5)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if updated.Title != "New Title" {
        t.Errorf("expected 'New Title', got '%s'", updated.Title)
    }
    if updated.Rating != 4.5 {
        t.Errorf("expected rating 4.5, got %.1f", updated.Rating)
    }
    if updated.AddedAt != book.AddedAt {
        t.Error("AddedAt must not change after update")
    }
}

func TestUpdateBook_NotFound(t *testing.T) {
    svc, _ := newTestService(t, nil)

    _, err := svc.UpdateBook(context.Background(), "nonexistent", "Title", "Author", "Genre", 4.0)
    if !errors.Is(err, library.ErrBookNotFound) {
        t.Errorf("expected ErrBookNotFound, got %v", err)
    }
}

func TestUpdateBook_DuplicateTitle(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    svc.AddBook(ctx, "Book One", "Author", "Genre", 4.0)
    book2, _ := svc.AddBook(ctx, "Book Two", "Author", "Genre", 4.0)

    _, err := svc.UpdateBook(ctx, book2.ID, "Book One", "Author", "Genre", 3.0)
    if !errors.Is(err, library.ErrBookExists) {
        t.Errorf("expected ErrBookExists, got %v", err)
    }
}

func TestUpdateBook_RollbackOnStorageError(t *testing.T) {
    svc, mock := newTestService(t, nil)
    ctx := context.Background()

    book, _ := svc.AddBook(ctx, "Original", "Author", "Genre", 4.0)

    mock.saveErr = errors.New("disk full")

    _, err := svc.UpdateBook(ctx, book.ID, "Changed", "Author", "Genre", 5.0)
    if err == nil {
        t.Fatal("expected error, got nil")
    }

    books := svc.ListBooks(library.SortByTitle)
    if books[0].Title != "Original" {
        t.Errorf("expected rollback to 'Original', got '%s'", books[0].Title)
    }
}

func TestDeleteBook_Success(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    book, err := svc.AddBook(ctx, "Book", "Author", "Genre", 3.0)
    if err != nil {
        t.Fatal(err)
    }

    if err := svc.DeleteBook(ctx, book.ID); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    books := svc.ListBooks(library.SortByTitle)
    if len(books) != 0 {
        t.Errorf("expected 0 books after deletion, got %d", len(books))
    }
}

func TestDeleteBook_NotFound(t *testing.T) {
    svc, _ := newTestService(t, nil)

    err := svc.DeleteBook(context.Background(), "nonexistent_id")
    if !errors.Is(err, library.ErrBookNotFound) {
        t.Errorf("expected ErrBookNotFound, got %v", err)
    }
}

func TestDeleteBook_EmptyID(t *testing.T) {
    svc, _ := newTestService(t, nil)

    err := svc.DeleteBook(context.Background(), "")
    if !errors.Is(err, library.ErrEmptyID) {
        t.Errorf("expected ErrEmptyID, got %v", err)
    }
}

func TestSearch(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    svc.AddBook(ctx, "War and Peace", "Tolstoy", "Novel", 5.0)
    svc.AddBook(ctx, "Crime and Punishment", "Dostoevsky", "Novel", 4.8)
    svc.AddBook(ctx, "Master and Margarita", "Bulgakov", "Mystic", 5.0)

    cases := []struct {
        query string
        want  int
    }{
        {"tolstoy", 1},
        {"novel", 2},
        {"and", 3},
        {"xyz_not_exist", 0},
    }

    for _, tc := range cases {
        t.Run(tc.query, func(t *testing.T) {
            results := svc.Search(tc.query)
            if len(results) != tc.want {
                t.Errorf("query %q: expected %d results, got %d", tc.query, tc.want, len(results))
            }
        })
    }
}

func TestNewService_LoadError(t *testing.T) {
    log := slog.New(slog.NewTextHandler(os.Stderr, nil))
    mock := &mockStorage{loadErr: errors.New("storage unavailable")}

    _, err := library.NewService(context.Background(), mock, log, 5.0)
    if err == nil {
        t.Fatal("expected error when storage is unavailable, got nil")
    }
}