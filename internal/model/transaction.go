package model

import (
	"time"
	"github.com/google/uuid"
)

type Transaction struct {
	ID          string    `json:"id"`
	UserID      int64     `json:"user_id"`
	CategoryID  string    `json:"category_id"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}

// GenerateID генерирует новый UUID для транзакции, если он еще не установлен
func (t *Transaction) GenerateID() {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
}