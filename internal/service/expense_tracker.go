package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ivanoskov/financial_bot/internal/model"
)

// ReportType определяет тип отчета
type ReportType int

const (
	DailyReport ReportType = iota
	WeeklyReport
	MonthlyReport
	YearlyReport
)

// ExpenseTracker предоставляет методы для работы с финансовыми данными
type ExpenseTracker struct {
	repo Repository
}

// Repository определяет интерфейс для работы с хранилищем данных
type Repository interface {
	GetTransactions(ctx context.Context, userID int64, filter model.TransactionFilter) ([]model.Transaction, error)
	GetCategories(ctx context.Context, userID int64) ([]model.Category, error)
	CreateTransaction(ctx context.Context, transaction *model.Transaction) error
	DeleteTransaction(ctx context.Context, transactionID string, userID int64) error
	CreateCategory(ctx context.Context, category *model.Category) error
	DeleteCategory(ctx context.Context, categoryID string, userID int64) error
}

// NewExpenseTracker создает новый экземпляр ExpenseTracker
func NewExpenseTracker(repo Repository) *ExpenseTracker {
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

func (s *ExpenseTracker) GetMonthlyReport(ctx context.Context, userID int64) (*BaseReport, error) {
	now := time.Now()
	currentStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	currentEnd := currentStart.AddDate(0, 1, 0).Add(-time.Second)

	// Получаем данные за текущий месяц
	currentTransactions, err := s.repo.GetTransactions(ctx, userID, model.TransactionFilter{
		StartDate: &currentStart,
		EndDate:   &currentEnd,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get current month transactions: %w", err)
	}

	// Получаем данные за предыдущий месяц
	prevStart := currentStart.AddDate(0, -1, 0)
	prevEnd := currentStart.Add(-time.Second)
	prevTransactions, err := s.repo.GetTransactions(ctx, userID, model.TransactionFilter{
		StartDate: &prevStart,
		EndDate:   &prevEnd,
	})
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

	// Анализируем текущий месяц
	currentPeriod := analyzePeriod(currentTransactions, currentStart, currentEnd, categoryNames)

	// Анализируем предыдущий месяц
	prevPeriod := analyzePeriod(prevTransactions, prevStart, prevEnd, categoryNames)

	// Рассчитываем коэффициент сбережений
	savingsRate := 0.0
	if currentPeriod.TotalIncome > 0 {
		savingsRate = (currentPeriod.TotalIncome - currentPeriod.TotalExpenses) / currentPeriod.TotalIncome
	}

	prevSavingsRate := 0.0
	if prevPeriod.TotalIncome > 0 {
		prevSavingsRate = (prevPeriod.TotalIncome - prevPeriod.TotalExpenses) / prevPeriod.TotalIncome
	}

	// Получаем тренды
	expenseTrend, incomeTrend := s.calculateTrends(currentTransactions)

	// Форматируем отчет
	monthNames := []string{
		"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
		"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
	}

	report := &BaseReport{
		Period: fmt.Sprintf("%s %d", monthNames[now.Month()-1], now.Year()),
		Text: fmt.Sprintf(
			"💰 Доходы: %.2f₽%s\n"+
				"💸 Расходы: %.2f₽%s\n"+
				"📊 Баланс: %.2f₽%s\n"+
				"📈 Средний доход в день: %.2f₽%s\n"+
				"📉 Средний расход в день: %.2f₽%s\n"+
				"💹 Коэффициент сбережений: %.1f%%%s",
			currentPeriod.TotalIncome, formatChange(currentPeriod.TotalIncome, prevPeriod.TotalIncome),
			currentPeriod.TotalExpenses, formatChange(currentPeriod.TotalExpenses, prevPeriod.TotalExpenses),
			currentPeriod.Balance, formatChange(currentPeriod.Balance, prevPeriod.Balance),
			currentPeriod.AvgDailyIncome, formatChange(currentPeriod.AvgDailyIncome, prevPeriod.AvgDailyIncome),
			currentPeriod.AvgDailyExpense, formatChange(currentPeriod.AvgDailyExpense, prevPeriod.AvgDailyExpense),
			savingsRate*100, formatChange(savingsRate*100, prevSavingsRate*100),
		),
		CategoryData: CategoryData{
			Expenses: formatCategoryStats(currentPeriod.ExpensesByCategory, prevPeriod.ExpensesByCategory),
			Income:   formatCategoryStats(currentPeriod.IncomeByCategory, prevPeriod.IncomeByCategory),
		},
		Trends: Trends{
			ExpenseTrend: expenseTrend,
			IncomeTrend:  incomeTrend,
			PeriodComparison: PeriodComparison{
				PrevPeriod:    prevPeriod,
				CurrentPeriod: currentPeriod,
			},
		},
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
	filter := model.TransactionFilter{
		Limit: limit,
	}
	return s.repo.GetTransactions(ctx, userID, filter)
}

func (s *ExpenseTracker) DeleteTransaction(ctx context.Context, transactionID string, userID int64) error {
	return s.repo.DeleteTransaction(ctx, transactionID, userID)
}

// BaseReport представляет базовый отчет
type BaseReport struct {
	Period          string
	Text            string
	StartDate       time.Time
	EndDate         time.Time
	TotalIncome     float64
	TotalExpenses   float64
	Balance         float64
	TransactionData struct {
		TotalCount      int
		IncomeCount     int
		ExpenseCount    int
		AvgIncome       float64
		AvgExpense      float64
		DailyAvgIncome  float64
		DailyAvgExpense float64
		MaxIncome       model.TransactionInfo
		MaxExpense      model.TransactionInfo
	}
	CategoryData struct {
		Expenses []model.CategoryStats
		Income   []model.CategoryStats
		Changes  model.CategoryChanges
	}
	Trends struct {
		ExpenseTrend     []TrendPoint
		IncomeTrend      []TrendPoint
		PeriodComparison PeriodComparison
	}
}

// CategoryData содержит данные по категориям
type CategoryData struct {
	Expenses []model.CategoryStats
	Income   []model.CategoryStats
	Changes  model.CategoryChanges
}

// CategoryStat представляет статистику по категории
type CategoryStat struct {
	Name   string
	Amount float64
	Share  string
}

// Trends содержит данные о трендах
type Trends struct {
	ExpenseTrend     []TrendPoint
	IncomeTrend      []TrendPoint
	PeriodComparison PeriodComparison
}

// TrendPoint представляет точку в тренде
type TrendPoint struct {
	Date   time.Time
	Amount float64
	Change float64
}

// PeriodComparison содержит сравнение периодов
type PeriodComparison struct {
	PrevPeriod    PeriodStats
	CurrentPeriod PeriodStats
	ExpenseChange float64
	IncomeChange  float64
	BalanceChange float64
}

// PeriodStats содержит статистику за период
type PeriodStats struct {
	TotalIncome        float64
	TotalExpenses      float64
	Balance            float64
	AvgDailyIncome     float64
	AvgDailyExpense    float64
	DailyAvgIncome     float64
	DailyAvgExpense    float64
	ExpensesByCategory map[string]float64
	IncomeByCategory   map[string]float64
}

// formatChange форматирует изменение значения в процентах
func formatChange(current, previous float64) string {
	if previous == 0 {
		return ""
	}
	change := ((current - previous) / previous) * 100
	if change > 0 {
		return fmt.Sprintf(" `+%.1f%%`", change)
	}
	return fmt.Sprintf(" `%.1f%%`", change)
}

// formatCategoryStats форматирует статистику по категориям с изменениями
func formatCategoryStats(current, previous map[string]float64) []model.CategoryStats {
	stats := make([]model.CategoryStats, 0)
	total := 0.0
	for _, amount := range current {
		total += amount
	}

	for name, amount := range current {
		prevAmount := previous[name]
		share := (amount / total) * 100
		stats = append(stats, model.CategoryStats{
			Name:         name,
			Amount:       amount,
			Share:        share,
			TrendPercent: calculateTrendPercent(amount, prevAmount),
		})
	}

	// Сортируем по убыванию суммы
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Amount > stats[j].Amount
	})
	return stats
}

// calculateTrendPercent вычисляет процент изменения
func calculateTrendPercent(current, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return ((current - previous) / previous) * 100
}

// analyzePeriod анализирует транзакции за период
func analyzePeriod(transactions []model.Transaction, start, end time.Time, categoryNames map[string]string) PeriodStats {
	stats := PeriodStats{
		ExpensesByCategory: make(map[string]float64),
		IncomeByCategory:   make(map[string]float64),
	}

	days := end.Sub(start).Hours() / 24

	for _, t := range transactions {
		categoryName := categoryNames[t.CategoryID]
		if t.Amount > 0 {
			stats.TotalIncome += t.Amount
			stats.IncomeByCategory[categoryName] += t.Amount
		} else {
			stats.TotalExpenses += -t.Amount
			stats.ExpensesByCategory[categoryName] += -t.Amount
		}
	}

	stats.Balance = stats.TotalIncome - stats.TotalExpenses
	stats.AvgDailyIncome = stats.TotalIncome / days
	stats.AvgDailyExpense = stats.TotalExpenses / days
	stats.DailyAvgIncome = stats.TotalIncome / days
	stats.DailyAvgExpense = stats.TotalExpenses / days

	return stats
}

// calculateTrends вычисляет тренды изменений
func (s *ExpenseTracker) calculateTrends(transactions []model.Transaction) ([]TrendPoint, []TrendPoint) {
	// Сортируем транзакции по дате
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Date.Before(transactions[j].Date)
	})

	// Группируем транзакции по дням
	dailyExpenses := make(map[time.Time]float64)
	dailyIncome := make(map[time.Time]float64)

	for _, t := range transactions {
		date := time.Date(t.Date.Year(), t.Date.Month(), t.Date.Day(), 0, 0, 0, 0, time.UTC)
		if t.Amount > 0 {
			dailyIncome[date] += t.Amount
		} else {
			dailyExpenses[date] += -t.Amount
		}
	}

	// Создаем точки трендов
	expenseTrend := make([]TrendPoint, 0)
	incomeTrend := make([]TrendPoint, 0)

	// Получаем все даты
	dates := make([]time.Time, 0)
	for date := range dailyExpenses {
		dates = append(dates, date)
	}
	for date := range dailyIncome {
		if _, exists := dailyExpenses[date]; !exists {
			dates = append(dates, date)
		}
	}

	// Сортируем даты
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})

	// Вычисляем изменения
	var prevExpense, prevIncome float64
	for _, date := range dates {
		expense := dailyExpenses[date]
		income := dailyIncome[date]

		expenseTrend = append(expenseTrend, TrendPoint{
			Date:   date,
			Amount: expense,
			Change: expense - prevExpense,
		})
		incomeTrend = append(incomeTrend, TrendPoint{
			Date:   date,
			Amount: income,
			Change: income - prevIncome,
		})

		prevExpense = expense
		prevIncome = income
	}

	return expenseTrend, incomeTrend
}

func (s *ExpenseTracker) GetReport(ctx context.Context, userID int64, reportType ReportType) (*BaseReport, error) {
	now := time.Now()
	var startDate, endDate time.Time

	switch reportType {
	case DailyReport:
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endDate = now
	case WeeklyReport:
		startDate = now.AddDate(0, 0, -7)
		endDate = now
	case MonthlyReport:
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endDate = now
	case YearlyReport:
		startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		endDate = now
	}

	// Получаем транзакции за текущий период
	currentFilter := model.TransactionFilter{
		StartDate: &startDate,
		EndDate:   &endDate,
	}
	currentTransactions, err := s.repo.GetTransactions(ctx, userID, currentFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get current period transactions: %w", err)
	}

	// Получаем транзакции за предыдущий период
	prevStartDate := startDate.AddDate(0, -1, 0)
	prevEndDate := endDate.AddDate(0, -1, 0)
	prevFilter := model.TransactionFilter{
		StartDate: &prevStartDate,
		EndDate:   &prevEndDate,
	}
	prevTransactions, err := s.repo.GetTransactions(ctx, userID, prevFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get previous period transactions: %w", err)
	}

	// Получаем категории
	categories, err := s.repo.GetCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	// Создаем базовый отчет
	report := &BaseReport{
		Period:    s.formatPeriod(reportType, startDate, endDate),
		StartDate: startDate,
		EndDate:   endDate,
	}

	// Заполняем данные отчета
	s.fillTransactionStats(report, currentTransactions, categories)
	s.fillCategoryAnalytics(report, currentTransactions, prevTransactions, categories)
	s.fillTrendAnalytics(report, currentTransactions, prevTransactions, categories)

	return report, nil
}

func (s *ExpenseTracker) fillTransactionStats(report *BaseReport, transactions []model.Transaction, categories []model.Category) {
	stats := &report.TransactionData
	categoryNames := make(map[string]string)
	for _, cat := range categories {
		categoryNames[cat.ID] = cat.Name
	}

	var totalIncome, totalExpense float64
	for _, t := range transactions {
		if t.Amount > 0 {
			totalIncome += t.Amount
			stats.IncomeCount++
			if t.Amount > stats.MaxIncome.Amount {
				stats.MaxIncome = model.TransactionInfo{
					Amount:      t.Amount,
					CategoryID:  t.CategoryID,
					Date:        t.Date,
					Description: t.Description,
				}
			}
		} else {
			expense := -t.Amount
			totalExpense += expense
			stats.ExpenseCount++
			if expense > stats.MaxExpense.Amount {
				stats.MaxExpense = model.TransactionInfo{
					Amount:      expense,
					CategoryID:  t.CategoryID,
					Date:        t.Date,
					Description: t.Description,
				}
			}
		}
	}

	stats.TotalCount = len(transactions)
	report.TotalIncome = totalIncome
	report.TotalExpenses = totalExpense
	report.Balance = totalIncome - totalExpense

	// Вычисляем средние значения
	days := float64(report.EndDate.Sub(report.StartDate).Hours() / 24)
	if days > 0 {
		stats.DailyAvgIncome = totalIncome / days
		stats.DailyAvgExpense = totalExpense / days
	}
	if stats.IncomeCount > 0 {
		stats.AvgIncome = totalIncome / float64(stats.IncomeCount)
	}
	if stats.ExpenseCount > 0 {
		stats.AvgExpense = totalExpense / float64(stats.ExpenseCount)
	}
}

func (s *ExpenseTracker) fillCategoryAnalytics(report *BaseReport, currentTransactions, prevTransactions []model.Transaction, categories []model.Category) {
	// Создаем мапы для быстрого доступа
	categoryStats := make(map[string]*model.CategoryStats)
	prevCategoryAmounts := make(map[string]float64)
	categoryNames := make(map[string]string)
	categoryTypes := make(map[string]string)

	for _, cat := range categories {
		categoryNames[cat.ID] = cat.Name
		categoryTypes[cat.ID] = cat.Type
		categoryStats[cat.ID] = &model.CategoryStats{
			CategoryID: cat.ID,
			Name:       cat.Name,
		}
	}

	// Анализируем предыдущий период
	for _, t := range prevTransactions {
		prevCategoryAmounts[t.CategoryID] += t.Amount
	}

	// Анализируем текущий период
	for _, t := range currentTransactions {
		stats := categoryStats[t.CategoryID]
		if stats == nil {
			continue
		}

		amount := t.Amount
		if amount < 0 {
			amount = -amount
		}
		stats.Amount += amount
		stats.Count++
	}

	// Вычисляем статистику по категориям
	for _, stats := range categoryStats {
		if stats.Count > 0 {
			stats.AvgAmount = stats.Amount / float64(stats.Count)
		}

		// Вычисляем тренд
		prevAmount := prevCategoryAmounts[stats.CategoryID]
		if prevAmount != 0 {
			stats.TrendPercent = calculateTrendPercent(stats.Amount, prevAmount)
		}

		// Определяем долю от общей суммы в зависимости от типа категории
		if categoryTypes[stats.CategoryID] == "income" {
			if report.TotalIncome > 0 {
				stats.Share = (stats.Amount / report.TotalIncome) * 100
				report.CategoryData.Income = append(report.CategoryData.Income, *stats)
			}
		} else {
			if report.TotalExpenses > 0 {
				stats.Share = (stats.Amount / report.TotalExpenses) * 100
				report.CategoryData.Expenses = append(report.CategoryData.Expenses, *stats)
			}
		}
	}

	// Сортируем категории по сумме
	sort.Slice(report.CategoryData.Income, func(i, j int) bool {
		return report.CategoryData.Income[i].Amount > report.CategoryData.Income[j].Amount
	})
	sort.Slice(report.CategoryData.Expenses, func(i, j int) bool {
		return report.CategoryData.Expenses[i].Amount > report.CategoryData.Expenses[j].Amount
	})

	// Находим категории с наибольшими изменениями
	s.findCategoryChanges(&report.CategoryData.Changes, categoryStats, prevCategoryAmounts, categoryNames)
}

func (s *ExpenseTracker) fillTrendAnalytics(report *BaseReport, currentTransactions, prevTransactions []model.Transaction, categories []model.Category) {
	// Группируем транзакции по дням
	currentDaily := s.groupTransactionsByDay(currentTransactions)

	// Создаем тренды для доходов и расходов
	report.Trends.ExpenseTrend = make([]TrendPoint, 0)
	report.Trends.IncomeTrend = make([]TrendPoint, 0)

	// Заполняем тренды
	var prevDayIncome, prevDayExpense float64
	for date := report.StartDate; !date.After(report.EndDate); date = date.AddDate(0, 0, 1) {
		dayKey := date.Format("2006-01-02")
		dayStats := currentDaily[dayKey]

		// Тренд доходов
		incomeTrend := TrendPoint{
			Date:   date,
			Amount: dayStats.income,
			Change: dayStats.income - prevDayIncome,
		}
		report.Trends.IncomeTrend = append(report.Trends.IncomeTrend, incomeTrend)
		prevDayIncome = dayStats.income

		// Тренд расходов
		expenseTrend := TrendPoint{
			Date:   date,
			Amount: dayStats.expense,
			Change: dayStats.expense - prevDayExpense,
		}
		report.Trends.ExpenseTrend = append(report.Trends.ExpenseTrend, expenseTrend)
		prevDayExpense = dayStats.expense
	}

	// Заполняем сравнение периодов
	report.Trends.PeriodComparison = s.comparePeriods(currentTransactions, prevTransactions, report.StartDate, report.EndDate)
}

type dailyStats struct {
	income  float64
	expense float64
}

func (s *ExpenseTracker) groupTransactionsByDay(transactions []model.Transaction) map[string]dailyStats {
	daily := make(map[string]dailyStats)
	for _, t := range transactions {
		day := t.Date.Format("2006-01-02")
		stats := daily[day]
		if t.Amount > 0 {
			stats.income += t.Amount
		} else {
			stats.expense += -t.Amount
		}
		daily[day] = stats
	}
	return daily
}

func (s *ExpenseTracker) comparePeriods(current, prev []model.Transaction, startDate, endDate time.Time) PeriodComparison {
	var comp PeriodComparison
	days := float64(endDate.Sub(startDate).Hours() / 24)

	// Текущий период
	for _, t := range current {
		if t.Amount > 0 {
			comp.CurrentPeriod.TotalIncome += t.Amount
		} else {
			comp.CurrentPeriod.TotalExpenses += -t.Amount
		}
	}
	comp.CurrentPeriod.Balance = comp.CurrentPeriod.TotalIncome - comp.CurrentPeriod.TotalExpenses
	if days > 0 {
		comp.CurrentPeriod.DailyAvgIncome = comp.CurrentPeriod.TotalIncome / days
		comp.CurrentPeriod.DailyAvgExpense = comp.CurrentPeriod.TotalExpenses / days
	}

	// Предыдущий период
	for _, t := range prev {
		if t.Amount > 0 {
			comp.PrevPeriod.TotalIncome += t.Amount
		} else {
			comp.PrevPeriod.TotalExpenses += -t.Amount
		}
	}
	comp.PrevPeriod.Balance = comp.PrevPeriod.TotalIncome - comp.PrevPeriod.TotalExpenses
	if days > 0 {
		comp.PrevPeriod.DailyAvgIncome = comp.PrevPeriod.TotalIncome / days
		comp.PrevPeriod.DailyAvgExpense = comp.PrevPeriod.TotalExpenses / days
	}

	// Вычисляем изменения
	if comp.PrevPeriod.TotalExpenses > 0 {
		comp.ExpenseChange = ((comp.CurrentPeriod.TotalExpenses - comp.PrevPeriod.TotalExpenses) / comp.PrevPeriod.TotalExpenses) * 100
	}
	if comp.PrevPeriod.TotalIncome > 0 {
		comp.IncomeChange = ((comp.CurrentPeriod.TotalIncome - comp.PrevPeriod.TotalIncome) / comp.PrevPeriod.TotalIncome) * 100
	}
	if comp.PrevPeriod.Balance != 0 {
		comp.BalanceChange = ((comp.CurrentPeriod.Balance - comp.PrevPeriod.Balance) / math.Abs(comp.PrevPeriod.Balance)) * 100
	}

	return comp
}

func (s *ExpenseTracker) findCategoryChanges(changes *model.CategoryChanges, currentStats map[string]*model.CategoryStats, prevAmounts map[string]float64, categoryNames map[string]string) {
	var maxGrowthExpense, maxGrowthIncome, maxDropExpense, maxDropIncome model.CategoryChange

	for catID, stats := range currentStats {
		prevAmount := prevAmounts[catID]
		change := stats.Amount - prevAmount
		if prevAmount != 0 {
			changePercent := (change / math.Abs(prevAmount)) * 100

			categoryChange := model.CategoryChange{
				CategoryID:    catID,
				Name:          categoryNames[catID],
				ChangeValue:   change,
				ChangePercent: changePercent,
			}

			if stats.Amount >= 0 { // Доходы
				if changePercent > maxGrowthIncome.ChangePercent {
					maxGrowthIncome = categoryChange
				} else if changePercent < maxDropIncome.ChangePercent {
					maxDropIncome = categoryChange
				}
			} else { // Расходы
				if changePercent > maxGrowthExpense.ChangePercent {
					maxGrowthExpense = categoryChange
				} else if changePercent < maxDropExpense.ChangePercent {
					maxDropExpense = categoryChange
				}
			}
		}
	}

	changes.FastestGrowingExpense = maxGrowthExpense
	changes.FastestGrowingIncome = maxGrowthIncome
	changes.LargestDropExpense = maxDropExpense
	changes.LargestDropIncome = maxDropIncome
}

func (s *ExpenseTracker) formatPeriod(reportType ReportType, start, end time.Time) string {
	switch reportType {
	case DailyReport:
		return start.Format("02.01.2006")
	case WeeklyReport:
		return fmt.Sprintf("%s - %s",
			start.Format("02.01.2006"),
			end.Format("02.01.2006"))
	case MonthlyReport:
		return start.Format("January 2006")
	case YearlyReport:
		return start.Format("2006")
	default:
		return fmt.Sprintf("%s - %s",
			start.Format("02.01.2006"),
			end.Format("02.01.2006"))
	}
}
