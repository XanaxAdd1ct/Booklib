package cli

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "log/slog"
    "strconv"
    "strings"

    "booklib/library"
)

type CLI struct {
    svc    *library.Service
    log    *slog.Logger
    reader *bufio.Scanner
    writer io.Writer
}

func New(svc *library.Service, log *slog.Logger, reader io.Reader, writer io.Writer) *CLI {
    return &CLI{
        svc:    svc,
        log:    log,
        reader: bufio.NewScanner(reader),
        writer: writer,
    }
}

func (c *CLI) Run(ctx context.Context) {
    c.printf("Library manager started.\n")
    c.printf("Type 'help' for the list of commands.\n")

    inputCh := make(chan string)

    go func() {
        defer close(inputCh)
        for c.reader.Scan() {
            select {
            case inputCh <- c.reader.Text():
            case <-ctx.Done():
                return
            }
        }
    }()

    for {
        c.printf("> ")

        select {
        case <-ctx.Done():
            c.printf("\nShutting down...\n")
            return
        case input, ok := <-inputCh:
            if !ok {
                return
            }

            input = strings.TrimSpace(input)
            if input == "" {
                continue
            }

            if input == "exit" {
                c.printf("Goodbye.\n")
                return
            }

            parts := strings.SplitN(input, " ", 2)
            cmd := parts[0]
            args := ""
            if len(parts) > 1 {
                args = parts[1]
            }

            c.dispatch(ctx, cmd, args)
        }
    }
}

func (c *CLI) dispatch(ctx context.Context, cmd, args string) {
    switch cmd {
    case "add":
        c.handleAdd(ctx, args)
    case "edit":
        c.handleEdit(ctx, args)
    case "list":
        c.handleList(args)
    case "search":
        c.handleSearch(args)
    case "delete":
        c.handleDelete(ctx, args)
    case "stats":
        c.handleStats()
    case "help":
        c.handleHelp()
    default:
        c.printf("Unknown command. Type 'help'.\n")
    }
}

func (c *CLI) handleAdd(ctx context.Context, args string) {
    data := strings.Split(args, "|")
    if len(data) < 4 {
        c.printf("Usage: add Title|Author|Genre|Rating\n")
        return
    }

    ratingStr := strings.TrimSpace(data[len(data)-1])
    genre     := strings.TrimSpace(data[len(data)-2])
    author    := strings.TrimSpace(data[len(data)-3])
    title     := strings.TrimSpace(strings.Join(data[:len(data)-3], "|"))

    rating, err := strconv.ParseFloat(ratingStr, 64)
    if err != nil {
        c.printf("Error: rating must be a number (e.g., 4.5)\n")
        return
    }

    book, err := c.svc.AddBook(ctx, title, author, genre, rating)
    if err != nil {
        c.printf("Error: %v\n", err)
        return
    }

    c.printf("Book added successfully. ID: %s\n", book.ID)
}

func (c *CLI) handleEdit(ctx context.Context, args string) {
    data := strings.Split(args, "|")
    if len(data) < 5 {
        c.printf("Usage: edit <ID>|<Title>|<Author>|<Genre>|<Rating>\n")
        return
    }

    id       := strings.TrimSpace(data[0])
    ratingStr := strings.TrimSpace(data[len(data)-1])
    genre    := strings.TrimSpace(data[len(data)-2])
    author   := strings.TrimSpace(data[len(data)-3])
    title    := strings.TrimSpace(strings.Join(data[1:len(data)-3], "|"))

    rating, err := strconv.ParseFloat(ratingStr, 64)
    if err != nil {
        c.printf("Error: rating must be a number (e.g., 4.5)\n")
        return
    }

    book, err := c.svc.UpdateBook(ctx, id, title, author, genre, rating)
    if err != nil {
        c.printf("Error: %v\n", err)
        return
    }

    c.printf("Book updated successfully. ID: %s\n", book.ID)
}

func (c *CLI) handleList(args string) {
    sortMap := map[string]library.SortCriteria{
        "date":   library.SortByDate,
        "rating": library.SortByRating,
        "title":  library.SortByTitle,
    }

    sortBy := library.SortByTitle
    if s, ok := sortMap[args]; ok {
        sortBy = s
    }

    books := c.svc.ListBooks(sortBy)
    if len(books) == 0 {
        c.printf("Library is empty.\n")
        return
    }

    for _, b := range books {
        c.printf("[%s] %s - %s (%s) | Rating: %.1f\n", b.ID, b.Title, b.Author, b.Genre, b.Rating)
    }
}

func (c *CLI) handleSearch(args string) {
    if strings.TrimSpace(args) == "" {
        c.printf("Usage: search <query>\n")
        return
    }

    books := c.svc.Search(args)
    if len(books) == 0 {
        c.printf("No matches found.\n")
        return
    }

    for _, b := range books {
        c.printf("[%s] %s - %s (%s)\n", b.ID, b.Title, b.Author, b.Genre)
    }
}

func (c *CLI) handleDelete(ctx context.Context, args string) {
    if err := c.svc.DeleteBook(ctx, args); err != nil {
        c.printf("Error: %v\n", err)
        return
    }
    c.printf("Book deleted successfully.\n")
}

func (c *CLI) handleStats() {
    stats := c.svc.GetStats()
    c.printf("Genre statistics:\n")
    if len(stats) == 0 {
        c.printf("No data available.\n")
        return
    }
    for genre, count := range stats {
        c.printf("- %s: %d\n", genre, count)
    }
}

func (c *CLI) handleHelp() {
    c.printf("Available commands:\n")
    c.printf("  add <Title>|<Author>|<Genre>|<Rating>        - Add a new book\n")
    c.printf("  edit <ID>|<Title>|<Author>|<Genre>|<Rating>  - Edit existing book\n")
    c.printf("  list [title|date|rating]                     - List all books\n")
    c.printf("  search <query>                               - Search by any keyword\n")
    c.printf("  delete <ID>                                  - Delete book by ID\n")
    c.printf("  stats                                        - Show genre statistics\n")
    c.printf("  exit                                         - Exit application\n")
}

func (c *CLI) printf(format string, args ...any) {
    fmt.Fprintf(c.writer, format, args...)
}