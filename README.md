# BookLib — Library Manager

CLI-приложение для управления личной библиотекой книг.

## Требования

- Go 1.21+

## Установка

```
git clone https://github.com/yourname/booklib.git
cd booklib
go mod init booklib
```

## Запуск

```
go run ./...
```

## Работа

```
add Название|Автор|Жанр|Рейтинг
edit ID|Название|Автор|Жанр|Рейтинг
delete ID
list
list title
list date
list rating
search запрос
stats
help
exit
```

## Примеры

```
add Мастер и Маргарита|Булгаков|Роман|5.0
edit a1b2c3d4|Мастер и Маргарита|Булгаков|Мистика|4.8
delete a1b2c3d4
list
list rating
search Булгаков
stats
exit
```
# API документация — BookLib

---

## Что это такое

BookLib — это консольное приложение для управления библиотекой книг. Вся бизнес-логика живёт в пакете `library`, данные читаются и пишутся через интерфейс `storage.Storage`. Все методы можно безопасно вызывать из нескольких горутин одновременно.

---

## Как устроено приложение

Запрос идёт сверху вниз: пользователь вводит команду → CLI разбирает её → сервис выполняет логику → хранилище сохраняет результат.

---

## Из чего состоит проект

| Пакет | Что делает |
|---|---|
| `models` | Описывает структуру книги |
| `storage` | Читает и записывает данные в JSON-файл |
| `library` | Содержит всю бизнес-логику |
| `config` | Загружает настройки из переменных окружения |
| `logger` | Настраивает логгер |
| `cli` | Обрабатывает команды пользователя |

---

## Модель данных — `Book`

Это основная структура, с которой работает всё приложение:

```go
type Book struct {
    ID      string    `json:"id"`
    Title   string    `json:"title"`
    Author  string    `json:"author"`
    Genre   string    `json:"genre"`
    Rating  float64   `json:"rating"`
    AddedAt time.Time `json:"added_at"`
}
```

| Поле | Тип | Описание |
|---|---|---|
| `ID` | `string` | Уникальный идентификатор — 16 hex символов, генерируется автоматически |
| `Title` | `string` | Название книги, не длиннее 255 символов |
| `Author` | `string` | Автор книги, не длиннее 100 символов |
| `Genre` | `string` | Жанр книги, не длиннее 50 символов |
| `Rating` | `float64` | Рейтинг от `0` до `MAX_RATING` |
| `AddedAt` | `time.Time` | Когда книга была добавлена — никогда не меняется при редактировании |

---

## Хранилище — пакет `storage`

### Интерфейс

Любое хранилище должно реализовать два метода — загрузить и сохранить:

```go
type Storage interface {
    Load(ctx context.Context) ([]models.Book, error)
    Save(ctx context.Context, books []models.Book) error
}
```

Благодаря интерфейсу можно легко подменить реализацию — например, в тестах используется `mockStorage` вместо реального файла.

---

### Создание хранилища — `NewJSONStorage`

```go
func NewJSONStorage(filename string) *JSONStorage
```

Просто передаёшь путь к файлу — хранилище готово:

```go
store := storage.NewJSONStorage("library_data.json")
```

---

### Загрузка данных — `Load`

```go
func (s *JSONStorage) Load(ctx context.Context) ([]models.Book, error)
```

Загружает книги из файла. Важные детали поведения:

- Файл не существует → вернёт пустой список, ошибки не будет
- Контекст отменён → сразу вернёт ошибку
- Файл повреждён → вернёт ошибку с описанием

| Что вернёт | Когда |
|---|---|
| `[]models.Book, nil` | Всё хорошо, данные загружены |
| `[]models.Book{}, nil` | Файла нет, но это нормально |
| `nil, error` | Файл есть, но прочитать не получилось |

---

### Сохранение данных — `Save`

```go
func (s *JSONStorage) Save(ctx context.Context, books []models.Book) error
```

Записывает все книги в файл. Чтобы не потерять данные при сбое, запись происходит атомарно:

```
1. Создаётся временный файл  →  library_data.json.tmp
2. Данные записываются в него
3. Файл закрывается
4. os.Rename(.tmp → .json)   ← атомарная операция ОС
```

Если что-то пойдёт не так на шаге 1-3, оригинальный файл останется нетронутым.

---

## Бизнес-логика — пакет `library`

### Ошибки

Все ошибки объявлены как сентинелы — их удобно проверять через `errors.Is()`:

```go
var (
    ErrBookNotFound  = errors.New("book not found")
    ErrInvalidInput  = errors.New("invalid input: title, author, and genre are required")
    ErrInvalidRating = errors.New("invalid rating")
    ErrInputTooLong  = errors.New("input exceeds maximum allowed length")
    ErrBookExists    = errors.New("book already exists in the library")
    ErrEmptyID       = errors.New("ID cannot be empty")
)
```

Пример проверки:
```go
if errors.Is(err, library.ErrBookNotFound) {
    fmt.Println("книга не найдена")
}
```

---

### Сортировка

```go
type SortCriteria string

const (
    SortByTitle  SortCriteria = "title"
    SortByDate   SortCriteria = "date"
    SortByRating SortCriteria = "rating"
)
```

---

### Создание сервиса — `NewService`

```go
func NewService(
    ctx       context.Context,
    store     storage.Storage,
    log       *slog.Logger,
    maxRating float64,
) (*Service, error)
```

При создании сервис сразу загружает все книги из хранилища в память. Если хранилище недоступно — вернёт ошибку и дальше работать не будет.

```go
svc, err := library.NewService(ctx, store, log, 5.0)
if err != nil {
    log.Error("не удалось запустить сервис", "error", err)
    os.Exit(1)
}
```

---

### Добавление книги — `AddBook`

```go
func (s *Service) AddBook(
    ctx    context.Context,
    title  string,
    author string,
    genre  string,
    rating float64,
) (models.Book, error)
```

Перед сохранением метод проверяет входные данные и убеждается, что такая книга ещё не добавлена. Если что-то пошло не так при записи в файл — кэш автоматически откатится к предыдущему состоянию.

| Что вернёт | Когда |
|---|---|
| `models.Book, nil` | Книга добавлена |
| `ErrInvalidInput` | Одно из полей пустое |
| `ErrInputTooLong` | Поле слишком длинное |
| `ErrInvalidRating` | Рейтинг выходит за допустимый диапазон |
| `ErrBookExists` | Книга с таким названием и автором уже есть |

```go
book, err := svc.AddBook(ctx, "Мастер и Маргарита", "Булгаков", "Роман", 5.0)
if errors.Is(err, library.ErrBookExists) {
    fmt.Println("такая книга уже есть в библиотеке")
}
```

---

### Редактирование книги — `UpdateBook`

```go
func (s *Service) UpdateBook(
    ctx    context.Context,
    id     string,
    title  string,
    author string,
    genre  string,
    rating float64,
) (models.Book, error)
```

Обновляет книгу по ID. Поле `AddedAt` при этом не трогается — дата добавления остаётся оригинальной. Как и в `AddBook`, при сбое записи кэш откатывается.

| Что вернёт | Когда |
|---|---|
| `models.Book, nil` | Книга обновлена |
| `ErrBookNotFound` | Книги с таким ID не существует |
| `ErrBookExists` | Другая книга с таким названием и автором уже есть |
| `ErrInvalidInput` | Одно из полей пустое |
| `ErrInvalidRating` | Рейтинг выходит за допустимый диапазон |

---

### Удаление книги — `DeleteBook`

```go
func (s *Service) DeleteBook(
    ctx context.Context,
    id  string,
) error
```

Удаляет книгу по ID. Пустую строку в качестве ID не принимает.

| Что вернёт | Когда |
|---|---|
| `nil` | Книга удалена |
| `ErrEmptyID` | Передана пустая строка |
| `ErrBookNotFound` | Книги с таким ID не существует |

```go
err := svc.DeleteBook(ctx, "a1b2c3d4e5f6g7h8")
if errors.Is(err, library.ErrBookNotFound) {
    fmt.Println("книга не найдена")
}
```

---

### Поиск — `Search`

```go
func (s *Service) Search(query string) []models.Book
```

Ищет книги по ключевым словам. Несколько особенностей:

- Регистр не важен — `"ТОЛСТОЙ"` и `"толстой"` дадут одинаковый результат
- Поиск идёт сразу по названию, автору и жанру
- Если передать несколько слов — каждое из них должно встретиться хотя бы в одном поле
- Пустой запрос вернёт все книги

```go
// найдёт все книги Толстого
books := svc.Search("толстой")

// найдёт книги, где упоминаются и "война", и "роман"
books := svc.Search("война роман")
```

Метод никогда не вернёт `nil` — в худшем случае вернёт пустой срез.

---

### Список книг — `ListBooks`

```go
func (s *Service) ListBooks(sortBy SortCriteria) []models.Book
```

Возвращает все книги с нужной сортировкой:

| Значение | Как сортирует |
|---|---|
| `SortByTitle` | По названию от А до Я |
| `SortByDate` | По дате добавления, старые первые |
| `SortByRating` | По рейтингу, лучшие первые |

Метод возвращает копию данных — оригинальный кэш сервиса не затрагивается.

```go
books := svc.ListBooks(library.SortByRating)
for _, b := range books {
    fmt.Printf("%s — %.1f\n", b.Title, b.Rating)
}
```

---

### Статистика — `GetStats`

```go
func (s *Service) GetStats() map[string]int
```

Возвращает сколько книг каждого жанра есть в библиотеке:

```go
map[string]int{
    "Роман":    3,
    "Мистика":  1,
    "Детектив": 2,
}
```

---

## Конфигурация — пакет `config`

```go
func Load() (*Config, error)
```

Читает настройки из переменных окружения. Если переменная не задана — берётся значение по умолчанию:

| Переменная | По умолчанию | Что делает |
|---|---|---|
| `STORAGE_FILE` | `library_data.json` | Путь к файлу с данными |
| `LOG_LEVEL` | `info` | Уровень логов: `debug`, `info`, `warn`, `error` |
| `MAX_RATING` | `5.0` | Максимальный рейтинг, должен быть больше нуля |

Если `MAX_RATING` передан некорректно — функция вернёт ошибку и приложение не запустится.

---

## Логгер — пакет `logger`

```go
func New(level string) *slog.Logger
```

Создаёт логгер с JSON-выводом в `stderr`. Уровень логирования задаётся строкой:

| Что передать | Уровень |
|---|---|
| `"debug"` | Всё подряд |
| `"warn"` | Только предупреждения и ошибки |
| `"error"` | Только ошибки |
| что угодно другое | `info` — стандартный режим |

Пример того, как выглядят логи:
```json
{"time":"2026-06-29T10:00:00Z","level":"INFO","msg":"book added","id":"a1b2c3d4"}
```

---

## Потокобезопасность

Сервис можно использовать из нескольких горутин без дополнительной синхронизации — внутри всё уже защищено:

| Метод | Тип блокировки |
|---|---|
| `AddBook` | Полная блокировка на запись |
| `UpdateBook` | Полная блокировка на запись |
| `DeleteBook` | Полная блокировка на запись |
| `Search` | Блокировка только на чтение |
| `ListBooks` | Блокировка только на чтение |
| `GetStats` | Блокировка только на чтение |

Методы чтения не блокируют друг друга — несколько горутин могут искать книги одновременно.

---

## Пример — полный сценарий работы

```go
package main

import (
    "context"
    "errors"
    "fmt"

    "booklib/library"
    "booklib/logger"
    "booklib/storage"
)

func main() {
    ctx   := context.Background()
    log   := logger.New("info")
    store := storage.NewJSONStorage("library_data.json")

    // Создаём сервис — он сразу загрузит данные из файла
    svc, err := library.NewService(ctx, store, log, 5.0)
    if err != nil {
        fmt.Println("не удалось запустить приложение:", err)
        return
    }

    // Добавляем книгу
    book, err := svc.AddBook(ctx, "Мастер и Маргарита", "Булгаков", "Роман", 5.0)
    if errors.Is(err, library.ErrBookExists) {
        fmt.Println("книга уже есть в библиотеке")
        return
    }
    fmt.Println("добавлена книга с ID:", book.ID)

    // Ищем книги Булгакова
    results := svc.Search("Булгаков")
    fmt.Println("найдено книг:", len(results))

    // Смотрим все книги, отсортированные по рейтингу
    books := svc.ListBooks(library.SortByRating)
    for _, b := range books {
        fmt.Printf("[%s] %s — %.1f\n", b.ID, b.Title, b.Rating)
    }

    // Смотрим статистику по жанрам
    stats := svc.GetStats()
    for genre, count := range stats {
        fmt.Printf("%s: %d книг\n", genre, count)
    }

    // Удаляем книгу
    if err := svc.DeleteBook(ctx, book.ID); err != nil {
        fmt.Println("не удалось удалить книгу:", err)
    }
}
```
