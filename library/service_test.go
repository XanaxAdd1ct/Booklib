package library_test

import (
    "context"
    "errors"
    "log/slog"
    "os"
    "testing"

    "booklib/library"
    "booklib/models"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
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
    require.NoError(t, err, "failed to create service")
    return svc, mock
}

func TestAddBook_Success(t *testing.T) {
    svc, _ := newTestService(t, nil)

    book, err := svc.AddBook(context.Background(), "Master and Margarita", "Bulgakov", "Novel", 5.0)

    require.NoError(t, err)
    assert.NotEmpty(t, book.ID, "ID must not be empty")
    assert.Equal(t, "Master and Margarita", book.Title)
    assert.Equal(t, "Bulgakov", book.Author)
    assert.Equal(t, "Novel", book.Genre)
    assert.Equal(t, 5.0, book.Rating)
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
            assert.ErrorIs(t, err, tc.want)
        })
    }
}

func TestAddBook_Duplicate(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    _, err := svc.AddBook(ctx, "Book", "Author", "Genre", 4.0)
    require.NoError(t, err)

    _, err = svc.AddBook(ctx, "book", "AUTHOR", "Genre", 3.0)
    assert.ErrorIs(t, err, library.ErrBookExists)
}

func TestAddBook_StorageError_RollsBackCache(t *testing.T) {
    svc, mock := newTestService(t, nil)
    ctx := context.Background()

    mock.saveErr = errors.New("disk full")

    _, err := svc.AddBook(ctx, "Book", "Author", "Genre", 4.0)
    require.Error(t, err)

    books := svc.ListBooks(library.SortByTitle)
    assert.Empty(t, books, "cache should be empty after rollback")
}

func TestUpdateBook_Success(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    book, err := svc.AddBook(ctx, "Old Title", "Author", "Genre", 3.0)
    require.NoError(t, err)

    updated, err := svc.UpdateBook(ctx, book.ID, "New Title", "Author", "Genre", 4.5)

    require.NoError(t, err)
    assert.Equal(t, "New Title", updated.Title)
    assert.Equal(t, 4.5, updated.Rating)
    assert.Equal(t, book.AddedAt, updated.AddedAt, "AddedAt must not change after update")
}

func TestUpdateBook_NotFound(t *testing.T) {
    svc, _ := newTestService(t, nil)

    _, err := svc.UpdateBook(context.Background(), "nonexistent", "Title", "Author", "Genre", 4.0)
    assert.ErrorIs(t, err, library.ErrBookNotFound)
}

func TestUpdateBook_DuplicateTitle(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    _, err := svc.AddBook(ctx, "Book One", "Author", "Genre", 4.0)
    require.NoError(t, err)

    book2, err := svc.AddBook(ctx, "Book Two", "Author", "Genre", 4.0)
    require.NoError(t, err)

    _, err = svc.UpdateBook(ctx, book2.ID, "Book One", "Author", "Genre", 3.0)
    assert.ErrorIs(t, err, library.ErrBookExists)
}

func TestUpdateBook_RollbackOnStorageError(t *testing.T) {
    svc, mock := newTestService(t, nil)
    ctx := context.Background()

    book, err := svc.AddBook(ctx, "Original", "Author", "Genre", 4.0)
    require.NoError(t, err)

    mock.saveErr = errors.New("disk full")

    _, err = svc.UpdateBook(ctx, book.ID, "Changed", "Author", "Genre", 5.0)
    require.Error(t, err)

    books := svc.ListBooks(library.SortByTitle)
    require.Len(t, books, 1)
    assert.Equal(t, "Original", books[0].Title, "expected rollback to original title")
}

func TestDeleteBook_Success(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    book, err := svc.AddBook(ctx, "Book", "Author", "Genre", 3.0)
    require.NoError(t, err)

    err = svc.DeleteBook(ctx, book.ID)
    require.NoError(t, err)

    books := svc.ListBooks(library.SortByTitle)
    assert.Empty(t, books, "expected 0 books after deletion")
}

func TestDeleteBook_NotFound(t *testing.T) {
    svc, _ := newTestService(t, nil)

    err := svc.DeleteBook(context.Background(), "nonexistent_id")
    assert.ErrorIs(t, err, library.ErrBookNotFound)
}

func TestDeleteBook_EmptyID(t *testing.T) {
    svc, _ := newTestService(t, nil)

    err := svc.DeleteBook(context.Background(), "")
    assert.ErrorIs(t, err, library.ErrEmptyID)
}

func TestSearch(t *testing.T) {
    svc, _ := newTestService(t, nil)
    ctx := context.Background()

    _, err := svc.AddBook(ctx, "War and Peace", "Tolstoy", "Novel", 5.0)
    require.NoError(t, err)
    _, err = svc.AddBook(ctx, "Crime and Punishment", "Dostoevsky", "Novel", 4.8)
    require.NoError(t, err)
    _, err = svc.AddBook(ctx, "Master and Margarita", "Bulgakov", "Mystic", 5.0)
    require.NoError(t, err)

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
            assert.Len(t, results, tc.want, "query %q: unexpected number of results", tc.query)
        })
    }
}

func TestNewService_LoadError(t *testing.T) {
    log := slog.New(slog.NewTextHandler(os.Stderr, nil))
    mock := &mockStorage{loadErr: errors.New("storage unavailable")}

    _, err := library.NewService(context.Background(), mock, log, 5.0)
    assert.Error(t, err, "expected error when storage is unavailable")
}
