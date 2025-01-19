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

// TransactionFilter представляет фильтр для транзакций
type TransactionFilter struct {
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
}

// TransactionInfo содержит информацию о транзакции
type TransactionInfo struct {
	Amount      float64
	CategoryID  string
	Date        time.Time
	Description string
}

// CategoryStats содержит статистику по категории
type CategoryStats struct {
	CategoryID  string
	Name       string
	Amount     float64
	Count      int
	AvgAmount  float64
	Share      float64
	TrendPercent float64
}

// CategoryChange представляет изменение в категории
type CategoryChange struct {
	CategoryID    string
	Name         string
	ChangeValue  float64
	ChangePercent float64
}

// CategoryChanges содержит информацию об изменениях в категориях
type CategoryChanges struct {
	FastestGrowingExpense CategoryChange
	FastestGrowingIncome  CategoryChange
	LargestDropExpense    CategoryChange
	LargestDropIncome     CategoryChange
}