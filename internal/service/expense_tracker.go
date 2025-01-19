package service

import (
	"context"
	"time"
	"your-module/internal/model"
	"your-module/internal/repository"
)

type ExpenseTracker struct {
	repo repository.Repository
}

func NewExpenseTracker(repo repository.Repository) *ExpenseTracker {
	return &ExpenseTracker{
		repo: repo,
	}
}

func (s *ExpenseTracker) AddTransaction(ctx context.Context, userID string, categoryID string, amount float64, description string) error {
	transaction := &model.Transaction{
		UserID:      userID,
		CategoryID:  categoryID,
		Amount:      amount,
		Description: description,
		Date:        time.Now(),
		CreatedAt:   time.Now(),
	}
	return s.repo.CreateTransaction(ctx, transaction)
}

func (s *ExpenseTracker) GetMonthlyReport(ctx context.Context, userID string) (*Report, error) {
	startDate := time.Now().AddDate(0, -1, 0)
	filter := repository.TransactionFilter{
		StartDate: &startDate,
		EndDate:   &time.Now(),
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

func (s *ExpenseTracker) CreateDefaultCategories(ctx context.Context, userID string) error {
	defaultCategories := []model.Category{
		{UserID: userID, Name: "Продукты", Type: "expense"},
		{UserID: userID, Name: "Транспорт", Type: "expense"},
		{UserID: userID, Name: "Развлечения", Type: "expense"},
		{UserID: userID, Name: "Зарплата", Type: "income"},
	}

	for _, category := range defaultCategories {
		if err := s.repo.CreateCategory(ctx, &category); err != nil {
			return err
		}
	}
	return nil
}

type Report struct {
	TotalExpenses float64
	TotalIncome   float64
	ByCategory    map[string]float64
	Period        string
} 