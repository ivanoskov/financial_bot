package service

import (
	"context"
	"fmt"
	"sort"
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
	now := time.Now()
	
	// Текущий месяц
	endDate := now
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	
	// Прошлый месяц
	prevEndDate := startDate.Add(-time.Second)
	prevStartDate := time.Date(prevEndDate.Year(), prevEndDate.Month(), 1, 0, 0, 0, 0, prevEndDate.Location())

	// Получаем транзакции за текущий месяц
	currentFilter := repository.TransactionFilter{
		StartDate: &startDate,
		EndDate:   &endDate,
	}
	currentTransactions, err := s.repo.GetTransactions(ctx, userID, currentFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get current month transactions: %w", err)
	}

	// Получаем транзакции за прошлый месяц
	prevFilter := repository.TransactionFilter{
		StartDate: &prevStartDate,
		EndDate:   &prevEndDate,
	}
	prevTransactions, err := s.repo.GetTransactions(ctx, userID, prevFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get previous month transactions: %w", err)
	}

	// Получаем категории для имен
	categories, err := s.repo.GetCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	categoryNames := make(map[string]string)
	for _, cat := range categories {
		categoryNames[cat.ID] = cat.Name
	}

	report := &Report{
		ByCategory: make(map[string]float64),
		Period:     fmt.Sprintf("%s %d", startDate.Month(), startDate.Year()),
	}

	// Анализируем текущий месяц
	categoryAmounts := make(map[string]float64)
	for _, t := range currentTransactions {
		if t.Amount > 0 {
			report.TotalIncome += t.Amount
		} else {
			report.TotalExpenses += -t.Amount
		}
		report.ByCategory[t.CategoryID] += t.Amount
		categoryAmounts[t.CategoryID] += t.Amount
	}
	report.Balance = report.TotalIncome - report.TotalExpenses
	report.TransactionsCount = len(currentTransactions)

	// Анализируем прошлый месяц
	for _, t := range prevTransactions {
		if t.Amount > 0 {
			report.PrevMonthIncome += t.Amount
		} else {
			report.PrevMonthExpenses += -t.Amount
		}
	}

	// Вычисляем изменения
	if report.PrevMonthExpenses > 0 {
		report.ExpensesChange = ((report.TotalExpenses - report.PrevMonthExpenses) / report.PrevMonthExpenses) * 100
	}
	if report.PrevMonthIncome > 0 {
		report.IncomeChange = ((report.TotalIncome - report.PrevMonthIncome) / report.PrevMonthIncome) * 100
	}

	// Вычисляем средние значения
	daysInMonth := float64(now.Sub(startDate).Hours() / 24)
	if daysInMonth > 0 {
		report.AvgDailyExpense = report.TotalExpenses / daysInMonth
		report.AvgDailyIncome = report.TotalIncome / daysInMonth
	}
	if report.TransactionsCount > 0 {
		report.AvgTransAmount = (report.TotalIncome + report.TotalExpenses) / float64(report.TransactionsCount)
	}

	// Формируем топ категорий
	var expenseStats, incomeStats []CategoryStat
	for catID, amount := range categoryAmounts {
		stat := CategoryStat{
			Name:   categoryNames[catID],
			Amount: amount,
		}
		if amount > 0 {
			stat.Share = (amount / report.TotalIncome) * 100
			incomeStats = append(incomeStats, stat)
		} else {
			stat.Amount = -amount
			stat.Share = (stat.Amount / report.TotalExpenses) * 100
			expenseStats = append(expenseStats, stat)
		}
	}

	// Сортируем категории по убыванию суммы
	sort.Slice(expenseStats, func(i, j int) bool {
		return expenseStats[i].Amount > expenseStats[j].Amount
	})
	sort.Slice(incomeStats, func(i, j int) bool {
		return incomeStats[i].Amount > incomeStats[j].Amount
	})

	// Берем топ-3 категории
	if len(expenseStats) > 3 {
		report.TopExpenseCategories = expenseStats[:3]
	} else {
		report.TopExpenseCategories = expenseStats
	}
	if len(incomeStats) > 3 {
		report.TopIncomeCategories = incomeStats[:3]
	} else {
		report.TopIncomeCategories = incomeStats
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

	now := time.Now()
	defaultCategories := []model.Category{
		{
			UserID:    userID,
			Name:      "Продукты",
			Type:      "expense",
			CreatedAt: now,
		},
		{
			UserID:    userID,
			Name:      "Транспорт",
			Type:      "expense",
			CreatedAt: now,
		},
		{
			UserID:    userID,
			Name:      "Развлечения",
			Type:      "expense",
			CreatedAt: now,
		},
		{
			UserID:    userID,
			Name:      "Зарплата",
			Type:      "income",
			CreatedAt: now,
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
	category.CreatedAt = time.Now()
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
	// Общие показатели
	TotalExpenses float64
	TotalIncome   float64
	Balance       float64
	ByCategory    map[string]float64

	// Сравнение с прошлым периодом
	PrevMonthExpenses float64
	PrevMonthIncome   float64
	ExpensesChange    float64 // в процентах
	IncomeChange      float64 // в процентах

	// Средние значения
	AvgDailyExpense  float64
	AvgDailyIncome   float64
	AvgTransAmount   float64

	// Статистика
	TopExpenseCategories []CategoryStat
	TopIncomeCategories  []CategoryStat
	TransactionsCount    int
	Period              string
}

type CategoryStat struct {
	Name   string
	Amount float64
	Share  float64 // доля в процентах
} 