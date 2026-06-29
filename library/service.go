package library

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "errors"
    "fmt"
    "log/slog"
    "sort"
    "strings"
    "sync"
    "time"

    "booklib/models"
    "booklib/storage"
)

var (
    ErrBookNotFound  = errors.New("book not found")
    ErrInvalidInput  = errors.New("invalid input: title, author, and genre are required")
    ErrInvalidRating = errors.New("invalid rating")
    ErrInputTooLong  = errors.New("input exceeds maximum allowed length")
    ErrBookExists    = errors.New("book already exists in the library")
    ErrEmptyID       = errors.New("ID cannot be empty")
)

type SortCriteria string

const (
    SortByTitle  SortCriteria = "title"
    SortByDate   SortCriteria = "date"
    SortByRating SortCriteria = "rating"
)

type Service struct {
    store     storage.Storage
    cache     []models.Book
    mu        sync.RWMutex
    log       *slog.Logger
    maxRating float64
}

func NewService(ctx context.Context, store storage.Storage, log *slog.Logger, maxRating float64) (*Service, error) {
    books, err := store.Load(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to load initial data: %w", err)
    }

    log.Info("storage loaded", slog.Int("books_count", len(books)))

    return &Service{
        store:     store,
        cache:     books,
        log:       log,
        maxRating: maxRating,
    }, nil
}

func generateID() (string, error) {
    b := make([]byte, 8)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("failed to generate ID: %w", err)
    }
    return hex.EncodeToString(b), nil
}

func (s *Service) validate(title, author, genre string, rating float64) error {
    if strings.TrimSpace(title) == "" || strings.TrimSpace(author) == "" || strings.TrimSpace(genre) == "" {
        return ErrInvalidInput
    }
    if len(title) > 255 || len(author) > 100 || len(genre) > 50 {
        return ErrInputTooLong
    }
    if rating < 0 || rating > s.maxRating {
        return fmt.Errorf("%w (max %.1f)", ErrInvalidRating, s.maxRating)
    }
    return nil
}

func (s *Service) AddBook(ctx context.Context, title, author, genre string, rating float64) (models.Book, error) {
    title = strings.TrimSpace(title)
    author = strings.TrimSpace(author)
    genre = strings.TrimSpace(genre)

    if err := s.validate(title, author, genre, rating); err != nil {
        return models.Book{}, err
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    for _, b := range s.cache {
        if strings.EqualFold(b.Title, title) && strings.EqualFold(b.Author, author) {
            return models.Book{}, ErrBookExists
        }
    }

    id, err := generateID()
    if err != nil {
        return models.Book{}, err
    }

    book := models.Book{
        ID:      id,
        Title:   title,
        Author:  author,
        Genre:   genre,
        Rating:  rating,
        AddedAt: time.Now(),
    }

    s.cache = append(s.cache, book)

    if err := s.store.Save(ctx, s.cache); err != nil {
        s.cache = s.cache[:len(s.cache)-1]
        s.log.Error("failed to save book", slog.String("error", err.Error()))
        return models.Book{}, fmt.Errorf("failed to save book: %w", err)
    }

    s.log.Info("book added", slog.String("id", book.ID))
    return book, nil
}

func (s *Service) UpdateBook(ctx context.Context, id, title, author, genre string, rating float64) (models.Book, error) {
    title = strings.TrimSpace(title)
    author = strings.TrimSpace(author)
    genre = strings.TrimSpace(genre)

    if err := s.validate(title, author, genre, rating); err != nil {
        return models.Book{}, err
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    idx := -1
    for i, b := range s.cache {
        if b.ID == id {
            idx = i
            break
        }
    }

    if idx == -1 {
        return models.Book{}, ErrBookNotFound
    }

    for i, b := range s.cache {
        if i == idx {
            continue
        }
        if strings.EqualFold(b.Title, title) && strings.EqualFold(b.Author, author) {
            return models.Book{}, ErrBookExists
        }
    }

    updated := models.Book{
        ID:      s.cache[idx].ID,
        Title:   title,
        Author:  author,
        Genre:   genre,
        Rating:  rating,
        AddedAt: s.cache[idx].AddedAt,
    }

    old := s.cache[idx]
    s.cache[idx] = updated

    if err := s.store.Save(ctx, s.cache); err != nil {
        s.cache[idx] = old
        s.log.Error("failed to update book", slog.String("id", id), slog.String("error", err.Error()))
        return models.Book{}, fmt.Errorf("failed to update book: %w", err)
    }

    s.log.Info("book updated", slog.String("id", id))
    return updated, nil
}

func (s *Service) Search(query string) []models.Book {
    s.mu.RLock()
    defer s.mu.RUnlock()

    searchWords := strings.Fields(strings.ToLower(query))
    if len(searchWords) == 0 {
        result := make([]models.Book, len(s.cache))
        copy(result, s.cache)
        return result
    }

    result := make([]models.Book, 0)

    for _, b := range s.cache {
        lowerTitle  := strings.ToLower(b.Title)
        lowerAuthor := strings.ToLower(b.Author)
        lowerGenre  := strings.ToLower(b.Genre)

        matchesAll := true
        for _, word := range searchWords {
            if !strings.Contains(lowerTitle, word) &&
                !strings.Contains(lowerAuthor, word) &&
                !strings.Contains(lowerGenre, word) {
                matchesAll = false
                break
            }
        }

        if matchesAll {
            result = append(result, b)
        }
    }

    return result
}

func (s *Service) DeleteBook(ctx context.Context, id string) error {
    if strings.TrimSpace(id) == "" {
        return ErrEmptyID
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    idx := -1
    for i, b := range s.cache {
        if b.ID == id {
            idx = i
            break
        }
    }

    if idx == -1 {
        return ErrBookNotFound
    }

    newCache := make([]models.Book, 0, len(s.cache)-1)
    newCache = append(newCache, s.cache[:idx]...)
    newCache = append(newCache, s.cache[idx+1:]...)

    if err := s.store.Save(ctx, newCache); err != nil {
        s.log.Error("failed to delete book", slog.String("id", id), slog.String("error", err.Error()))
        return fmt.Errorf("failed to delete book: %w", err)
    }

    s.cache = newCache
    s.log.Info("book deleted", slog.String("id", id))
    return nil
}

func (s *Service) ListBooks(sortBy SortCriteria) []models.Book {
    s.mu.RLock()
    defer s.mu.RUnlock()

    result := make([]models.Book, len(s.cache))
    copy(result, s.cache)

    switch sortBy {
    case SortByTitle:
        sort.Slice(result, func(i, j int) bool { return result[i].Title < result[j].Title })
    case SortByDate:
        sort.Slice(result, func(i, j int) bool { return result[i].AddedAt.Before(result[j].AddedAt) })
    case SortByRating:
        sort.Slice(result, func(i, j int) bool { return result[i].Rating > result[j].Rating })
    }

    return result
}

func (s *Service) GetStats() map[string]int {
    s.mu.RLock()
    defer s.mu.RUnlock()

    stats := make(map[string]int, len(s.cache))
    for _, b := range s.cache {
        stats[b.Genre]++
    }
    return stats
}