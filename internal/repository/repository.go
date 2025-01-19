package repository

import (
	"context"
	"your-module/internal/model"
	"time"
)

type Repository interface {
	// Категории
	CreateCategory(ctx context.Context, category *model.Category) error
	GetCategories(ctx context.Context, userID string) ([]model.Category, error)
	UpdateCategory(ctx context.Context, category *model.Category) error
	DeleteCategory(ctx context.Context, id string, userID string) error

	// Транзакции
	CreateTransaction(ctx context.Context, transaction *model.Transaction) error
	GetTransactions(ctx context.Context, userID string, filter TransactionFilter) ([]model.Transaction, error)
	GetTransactionsByCategory(ctx context.Context, userID string, categoryID string) ([]model.Transaction, error)
	DeleteTransaction(ctx context.Context, id string, userID string) error
}

type TransactionFilter struct {
	StartDate *time.Time
	EndDate   *time.Time
	Type      string // "expense" или "income"
} 