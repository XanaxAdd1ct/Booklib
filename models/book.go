package models

import "time"

type Book struct {
    ID      string    `json:"id"`
    Title   string    `json:"title"`
    Author  string    `json:"author"`
    Genre   string    `json:"genre"`
    Rating  float64   `json:"rating"`
    AddedAt time.Time `json:"added_at"`
}