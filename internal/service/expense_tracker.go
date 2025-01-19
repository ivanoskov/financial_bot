package service

import (
	"context"
	"fmt"
	"time"
	"github.com/ivanoskov/financial_bot/internal/model"
	"github.com/ivanoskov/financial_bot/internal/repository"
)

type ExpenseTracker struct {
	repo repository.Repository
}

func NewExpenseTracker(repo repository.Repository) *ExpenseTracker {
	return &ExpenseTracker{
		repo: repo,
	}
}

func (s *ExpenseTracker) AddTransaction(ctx context.Context, userID int64, categoryID string, amount float64, description string) error {
	transaction := &model.Transaction{
		UserID:      userID,
		CategoryID:  categoryID,
		Amount:      amount,
		Description: description,
		Date:        time.Now(),
		CreatedAt:   time.Now(),
	}
	transaction.GenerateID()
	return s.repo.CreateTransaction(ctx, transaction)
}

func (s *ExpenseTracker) GetMonthlyReport(ctx context.Context, userID int64) (*Report, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, -1, 0)
	filter := repository.TransactionFilter{
		StartDate: &startDate,
		EndDate:   &endDate,
	}

	transactions, err := s.repo.GetTransactions(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	report := &Report{
		ByCategory: make(map[string]float64),
		Period:     "За последний месяц",
	}

	for _, t := range transactions {
		if t.Amount > 0 {
			report.TotalIncome += t.Amount
		} else {
			report.TotalExpenses += -t.Amount
		}
		report.ByCategory[t.CategoryID] += t.Amount
	}

	return report, nil
}

func (s *ExpenseTracker) CreateDefaultCategories(ctx context.Context, userID int64) error {
	// Проверяем, есть ли уже категории у пользователя
	existingCategories, err := s.repo.GetCategories(ctx, userID)
	if err != nil {
		return fmt.Errorf("error getting existing categories: %w", err)
	}

	if len(existingCategories) > 0 {
		// У пользователя уже есть категории, не создаем новые
		return nil
	}

	defaultCategories := []model.Category{
		{
			UserID: userID,
			Name:   "Продукты",
			Type:   "expense",
		},
		{
			UserID: userID,
			Name:   "Транспорт",
			Type:   "expense",
		},
		{
			UserID: userID,
			Name:   "Развлечения",
			Type:   "expense",
		},
		{
			UserID: userID,
			Name:   "Зарплата",
			Type:   "income",
		},
	}

	for _, category := range defaultCategories {
		if err := s.repo.CreateCategory(ctx, &category); err != nil {
			return fmt.Errorf("error creating category %s: %w", category.Name, err)
		}
	}

	return nil
}

func (s *ExpenseTracker) GetCategories(ctx context.Context, userID int64) ([]model.Category, error) {
	return s.repo.GetCategories(ctx, userID)
}

func (s *ExpenseTracker) CreateCategory(ctx context.Context, category *model.Category) error {
	return s.repo.CreateCategory(ctx, category)
}

func (s *ExpenseTracker) DeleteCategory(ctx context.Context, categoryID string, userID int64) error {
	return s.repo.DeleteCategory(ctx, categoryID, userID)
}

func (s *ExpenseTracker) GetRecentTransactions(ctx context.Context, userID int64, limit int) ([]model.Transaction, error) {
	filter := repository.TransactionFilter{
		Limit: limit,
	}
	return s.repo.GetTransactions(ctx, userID, filter)
}

func (s *ExpenseTracker) DeleteTransaction(ctx context.Context, transactionID string, userID int64) error {
	return s.repo.DeleteTransaction(ctx, transactionID, userID)
}

type Report struct {
	TotalExpenses float64
	TotalIncome   float64
	ByCategory    map[string]float64
	Period        string
} 