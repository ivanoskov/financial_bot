package repository

import (
	"context"
	"time"

	"github.com/ivanoskov/financial_bot/internal/model"
)

type Repository interface {
	// Категории
	CreateCategory(ctx context.Context, category *model.Category) error
	GetCategories(ctx context.Context, userID int64) ([]model.Category, error)
	UpdateCategory(ctx context.Context, category *model.Category) error
	DeleteCategory(ctx context.Context, id string, userID int64) error

	// Транзакции
	CreateTransaction(ctx context.Context, transaction *model.Transaction) error
	GetTransactions(ctx context.Context, userID int64, filter model.TransactionFilter) ([]model.Transaction, error)
	GetTransactionsByCategory(ctx context.Context, userID int64, categoryID string) ([]model.Transaction, error)
	DeleteTransaction(ctx context.Context, id string, userID int64) error

	// Методы для работы с состояниями пользователей
	GetUserState(ctx context.Context, userID int64) (*model.UserState, error)
	SaveUserState(ctx context.Context, state *model.UserState) error
	DeleteUserState(ctx context.Context, userID int64) error

	// Добавленные методы
	GetAllUsers(ctx context.Context) ([]int64, error)
}

type TransactionFilter struct {
	StartDate *time.Time
	EndDate   *time.Time
	Type      string // "expense" или "income"
	Limit     int    // ограничение количества результатов
}
