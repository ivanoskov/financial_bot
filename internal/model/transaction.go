package model

import "time"

type Transaction struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    CategoryID  string    `json:"category_id"`
    Amount      float64   `json:"amount"`
    Description string    `json:"description"`
    Date        time.Time `json:"date"`
    CreatedAt   time.Time `json:"created_at"`
} 