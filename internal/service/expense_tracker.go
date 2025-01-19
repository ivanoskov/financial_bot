package service

import (
	"context"
	"fmt"
	"log"
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
	now := time.Now()
	// Нормализуем дату до начала дня
	transactionDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	transaction := &model.Transaction{
		UserID:      userID,
		CategoryID:  categoryID,
		Amount:      amount,
		Description: description,
		Date:        transactionDate,
		CreatedAt:   now,
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

// calculateTrendPercent вычисляет процент изменения
func calculateTrendPercent(current, previous float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100 // Рост с нуля
		}
		return 0 // Нет изменений, если оба значения нулевые
	}

	// Если значения имеют разные знаки или текущее значение намного меньше предыдущего
	if (current < 0 && previous > 0) || (current > 0 && previous < 0) {
		return -100 // Полное изменение в противоположную сторону
	}

	// Для случаев, когда текущее значение намного меньше предыдущего
	if math.Abs(current) < math.Abs(previous) {
		decrease := ((math.Abs(previous) - math.Abs(current)) / math.Abs(current)) * 100
		return -decrease // Возвращаем отрицательный процент
	}

	// Для случаев, когда текущее значение больше предыдущего
	increase := ((math.Abs(current) - math.Abs(previous)) / math.Abs(previous)) * 100
	return increase
}

// formatChange форматирует изменение значения в процентах
func formatChange(current, previous float64) string {
	if previous == 0 {
		return ""
	}

	change := calculateTrendPercent(current, previous)

	// Ограничиваем отображение процентов разумными пределами
	if change < -1000 {
		change = -1000
	} else if change > 1000 {
		change = 1000
	}

	if change > 0 {
		return fmt.Sprintf(" (+%.1f%%⬆️)", change)
	}
	return fmt.Sprintf(" (%.1f%%⬇️)", change)
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
		// Устанавливаем начало дня (00:00:00) и конец дня (23:59:59)
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	case WeeklyReport:
		// Начало недели (7 дней назад)
		startDate = time.Date(now.Year(), now.Month(), now.Day()-7, 0, 0, 0, 0, now.Location())
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	case MonthlyReport:
		// Начало текущего месяца
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		// Конец текущего месяца
		endDate = time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 999999999, now.Location())
	case YearlyReport:
		// Начало текущего года
		startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		// Конец текущего года
		endDate = time.Date(now.Year(), 12, 31, 23, 59, 59, 999999999, now.Location())
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
	log.Printf("Получено транзакций за текущий период: %d", len(currentTransactions))

	// Получаем транзакции за предыдущий период такой же длительности
	var prevStartDate, prevEndDate time.Time
	periodDuration := endDate.Sub(startDate)
	prevEndDate = startDate.Add(-time.Nanosecond)
	prevStartDate = prevEndDate.Add(-periodDuration).Add(time.Nanosecond)

	prevFilter := model.TransactionFilter{
		StartDate: &prevStartDate,
		EndDate:   &prevEndDate,
	}
	prevTransactions, err := s.repo.GetTransactions(ctx, userID, prevFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get previous period transactions: %w", err)
	}
	log.Printf("Получено транзакций за предыдущий период: %d", len(prevTransactions))

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
	log.Printf("Начинаем анализ транзакций. Всего транзакций: %d, период: %s - %s",
		len(transactions), report.StartDate.Format("2006-01-02"), report.EndDate.Format("2006-01-02"))

	stats := &report.TransactionData
	categoryNames := make(map[string]string)
	for _, cat := range categories {
		categoryNames[cat.ID] = cat.Name
	}

	var totalIncome, totalExpense float64
	var incomeCount, expenseCount int

	// Фильтруем и считаем транзакции только за указанный период
	for _, t := range transactions {
		// Пропускаем транзакции вне периода
		if t.Date.Before(report.StartDate) || t.Date.After(report.EndDate) {
			continue
		}

		log.Printf("Обработка транзакции: ID=%s, Сумма=%.2f, Дата=%s, Категория=%s, Описание=%s",
			t.ID, t.Amount, t.Date.Format("2006-01-02"), categoryNames[t.CategoryID], t.Description)

		if t.Amount > 0 {
			totalIncome += t.Amount
			incomeCount++
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
			expenseCount++
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

	stats.TotalCount = incomeCount + expenseCount
	stats.IncomeCount = incomeCount
	stats.ExpenseCount = expenseCount
	report.TotalIncome = totalIncome
	report.TotalExpenses = totalExpense
	report.Balance = totalIncome - totalExpense

	// Вычисляем средние значения
	days := float64(report.EndDate.Sub(report.StartDate).Hours()/24) + 1 // +1 чтобы включить текущий день
	if days < 1 {
		days = 1
	}

	stats.DailyAvgIncome = totalIncome / days
	stats.DailyAvgExpense = totalExpense / days

	if incomeCount > 0 {
		stats.AvgIncome = totalIncome / float64(incomeCount)
	}
	if expenseCount > 0 {
		stats.AvgExpense = totalExpense / float64(expenseCount)
	}

	log.Printf("Итоги анализа за %d дней:", int(days))
	log.Printf("Доходы=%.2f (среднее в день=%.2f), Кол-во=%d, Средний доход=%.2f",
		totalIncome, stats.DailyAvgIncome, incomeCount, stats.AvgIncome)
	log.Printf("Расходы=%.2f (среднее в день=%.2f), Кол-во=%d, Средний расход=%.2f",
		totalExpense, stats.DailyAvgExpense, expenseCount, stats.AvgExpense)
	log.Printf("Баланс=%.2f", report.Balance)
}

func (s *ExpenseTracker) fillCategoryAnalytics(report *BaseReport, currentTransactions, prevTransactions []model.Transaction, categories []model.Category) {
	log.Printf("Начинаем анализ категорий. Текущих транзакций: %d, Предыдущих транзакций: %d",
		len(currentTransactions), len(prevTransactions))

	// Создаем мапы для быстрого доступа
	categoryStats := make(map[string]*model.CategoryStats)
	prevCategoryAmounts := make(map[string]float64)
	categoryTypes := make(map[string]string)

	// Инициализируем мапы категорий
	for _, cat := range categories {
		categoryTypes[cat.ID] = cat.Type
		categoryStats[cat.ID] = &model.CategoryStats{
			CategoryID: cat.ID,
			Name:       cat.Name,
			Amount:     0,
			Count:      0,
		}
	}

	// Анализируем текущий период
	for _, t := range currentTransactions {
		// Проверяем, что транзакция входит в текущий период
		if t.Date.Before(report.StartDate) || t.Date.After(report.EndDate) {
			// log.Printf("Пропускаем транзакцию вне периода: %s (сумма: %.2f)", t.Date.Format("2006-01-02"), t.Amount)
			continue
		}

		if stats, ok := categoryStats[t.CategoryID]; ok {
			stats.Amount += t.Amount // Сохраняем оригинальное значение (положительное для доходов, отрицательное для расходов)
			stats.Count++
			log.Printf("Добавлена транзакция в категорию %s: %.2f (всего: %.2f)", stats.Name, t.Amount, stats.Amount)
		}
	}

	// Получаем даты для предыдущего периода
	periodDuration := report.EndDate.Sub(report.StartDate)
	prevPeriodEnd := report.StartDate.Add(-time.Nanosecond)
	prevPeriodStart := prevPeriodEnd.Add(-periodDuration).Add(time.Nanosecond)

	// Анализируем предыдущий период
	for _, t := range prevTransactions {
		// Проверяем, что транзакция входит в предыдущий период
		if t.Date.Before(prevPeriodStart) || t.Date.After(prevPeriodEnd) {
			continue
		}

		if _, ok := categoryStats[t.CategoryID]; ok {
			prevCategoryAmounts[t.CategoryID] += t.Amount
		}
	}

	// Вычисляем статистику по категориям
	var totalIncome, totalExpense float64
	for _, stats := range categoryStats {
		if stats.Count > 0 {
			stats.AvgAmount = stats.Amount / float64(stats.Count)

			// Определяем тип категории и считаем общие суммы
			if categoryTypes[stats.CategoryID] == "income" {
				totalIncome += stats.Amount
			} else {
				totalExpense += math.Abs(stats.Amount)
			}
			log.Printf("Категория %s: сумма=%.2f, количество=%d, средняя=%.2f",
				stats.Name, stats.Amount, stats.Count, stats.AvgAmount)
		}
	}

	// Вычисляем доли и формируем итоговые списки
	for _, stats := range categoryStats {
		if stats.Count == 0 {
			continue // Пропускаем категории без транзакций
		}

		// Вычисляем тренд
		prevAmount := prevCategoryAmounts[stats.CategoryID]
		if prevAmount != 0 {
			stats.TrendPercent = calculateTrendPercent(stats.Amount, prevAmount)
		}

		if categoryTypes[stats.CategoryID] == "income" {
			if totalIncome > 0 {
				stats.Share = (stats.Amount / totalIncome) * 100
			}
			report.CategoryData.Income = append(report.CategoryData.Income, *stats)
			log.Printf("Добавлен доход %s: сумма=%.2f, доля=%.2f%%", stats.Name, stats.Amount, stats.Share)
		} else {
			if totalExpense > 0 {
				stats.Share = (math.Abs(stats.Amount) / totalExpense) * 100
			}
			report.CategoryData.Expenses = append(report.CategoryData.Expenses, *stats)
			log.Printf("Добавлен расход %s: сумма=%.2f, доля=%.2f%%", stats.Name, stats.Amount, stats.Share)
		}
	}

	// Сортируем категории по абсолютному значению суммы
	sort.Slice(report.CategoryData.Income, func(i, j int) bool {
		return report.CategoryData.Income[i].Amount > report.CategoryData.Income[j].Amount
	})
	sort.Slice(report.CategoryData.Expenses, func(i, j int) bool {
		return math.Abs(report.CategoryData.Expenses[i].Amount) > math.Abs(report.CategoryData.Expenses[j].Amount)
	})

	// Создаем мапу имен категорий для findCategoryChanges
	categoryNames := make(map[string]string)
	for _, cat := range categories {
		categoryNames[cat.ID] = cat.Name
	}

	// Находим значительные изменения
	s.findCategoryChanges(&report.CategoryData.Changes, categoryStats, prevCategoryAmounts, categoryNames)

	log.Printf("Итоги по категориям: Доходы=%d категорий, Расходы=%d категорий",
		len(report.CategoryData.Income), len(report.CategoryData.Expenses))
}

func (s *ExpenseTracker) fillTrendAnalytics(report *BaseReport, currentTransactions, prevTransactions []model.Transaction, categories []model.Category) {
	// Группируем транзакции по дням
	currentDaily := s.groupTransactionsByDay(currentTransactions)

	// Создаем тренды для доходов и расходов
	report.Trends.ExpenseTrend = make([]TrendPoint, 0)
	report.Trends.IncomeTrend = make([]TrendPoint, 0)

	// Вычисляем средние значения за период
	var totalIncome, totalExpense float64
	var daysWithIncome, daysWithExpense int
	for _, stats := range currentDaily {
		if stats.income > 0 {
			totalIncome += stats.income
			daysWithIncome++
		}
		if stats.expense > 0 {
			totalExpense += stats.expense
			daysWithExpense++
		}
	}

	// Вычисляем средние значения только для дней с транзакциями
	avgDailyIncome := 0.0
	if daysWithIncome > 0 {
		avgDailyIncome = totalIncome / float64(daysWithIncome)
	}

	avgDailyExpense := 0.0
	if daysWithExpense > 0 {
		avgDailyExpense = totalExpense / float64(daysWithExpense)
	}

	log.Printf("Средние значения: доход=%.2f (%d дней), расход=%.2f (%d дней)",
		avgDailyIncome, daysWithIncome, avgDailyExpense, daysWithExpense)

	// Заполняем тренды для текущего периода
	for date := report.StartDate; !date.After(report.EndDate); date = date.AddDate(0, 0, 1) {
		dayKey := date.Format("2006-01-02")
		dayStats := currentDaily[dayKey]

		// Тренд доходов: отклонение от среднего в процентах
		incomeChange := calculateTrendPercent(dayStats.income, avgDailyIncome)
		incomeTrend := TrendPoint{
			Date:   date,
			Amount: dayStats.income,
			Change: incomeChange,
		}
		report.Trends.IncomeTrend = append(report.Trends.IncomeTrend, incomeTrend)

		// Тренд расходов: отклонение от среднего в процентах
		expenseChange := calculateTrendPercent(dayStats.expense, avgDailyExpense)
		expenseTrend := TrendPoint{
			Date:   date,
			Amount: -dayStats.expense, // Сохраняем расходы как отрицательные значения
			Change: expenseChange,
		}
		report.Trends.ExpenseTrend = append(report.Trends.ExpenseTrend, expenseTrend)

		// log.Printf("Тренды за %s: доход=%.2f (%.1f%%), расход=%.2f (%.1f%%)",
		// 	dayKey, dayStats.income, incomeChange, -dayStats.expense, expenseChange)
	}

	// Заполняем сравнение периодов
	var currentPeriod, prevPeriod PeriodStats
	days := float64(report.EndDate.Sub(report.StartDate).Hours() / 24)
	if days < 1 {
		days = 1
	}

	// Считаем текущий период
	for _, t := range currentTransactions {
		if t.Date.Before(report.StartDate) || t.Date.After(report.EndDate) {
			continue
		}
		if t.Amount > 0 {
			currentPeriod.TotalIncome += t.Amount
		} else {
			currentPeriod.TotalExpenses += -t.Amount
		}
	}
	currentPeriod.Balance = currentPeriod.TotalIncome - currentPeriod.TotalExpenses
	currentPeriod.DailyAvgIncome = currentPeriod.TotalIncome / days
	currentPeriod.DailyAvgExpense = currentPeriod.TotalExpenses / days

	// Получаем даты для предыдущего периода
	periodDuration := report.EndDate.Sub(report.StartDate)
	prevPeriodEnd := report.StartDate.Add(-time.Nanosecond)
	prevPeriodStart := prevPeriodEnd.Add(-periodDuration).Add(time.Nanosecond)

	// Считаем предыдущий период
	for _, t := range prevTransactions {
		if t.Date.Before(prevPeriodStart) || t.Date.After(prevPeriodEnd) {
			continue
		}
		if t.Amount > 0 {
			prevPeriod.TotalIncome += t.Amount
		} else {
			prevPeriod.TotalExpenses += -t.Amount
		}
	}
	prevPeriod.Balance = prevPeriod.TotalIncome - prevPeriod.TotalExpenses
	prevPeriod.DailyAvgIncome = prevPeriod.TotalIncome / days
	prevPeriod.DailyAvgExpense = prevPeriod.TotalExpenses / days

	// Вычисляем изменения с ограничением в пределах [-100%, +200%]
	if prevPeriod.TotalExpenses > 0 {
		expenseChange := calculateTrendPercent(currentPeriod.TotalExpenses, prevPeriod.TotalExpenses)
		report.Trends.PeriodComparison.ExpenseChange = math.Max(math.Min(expenseChange, 200), -100)
	}
	if prevPeriod.TotalIncome > 0 {
		incomeChange := calculateTrendPercent(currentPeriod.TotalIncome, prevPeriod.TotalIncome)
		report.Trends.PeriodComparison.IncomeChange = math.Max(math.Min(incomeChange, 200), -100)
	}
	if prevPeriod.Balance != 0 {
		balanceChange := calculateTrendPercent(currentPeriod.Balance, prevPeriod.Balance)
		report.Trends.PeriodComparison.BalanceChange = math.Max(math.Min(balanceChange, 200), -100)
	}

	report.Trends.PeriodComparison.CurrentPeriod = currentPeriod
	report.Trends.PeriodComparison.PrevPeriod = prevPeriod

	log.Printf("Сравнение периодов: Текущий (Доходы=%.2f, Расходы=%.2f, Баланс=%.2f), Предыдущий (Доходы=%.2f, Расходы=%.2f, Баланс=%.2f)",
		currentPeriod.TotalIncome, currentPeriod.TotalExpenses, currentPeriod.Balance,
		prevPeriod.TotalIncome, prevPeriod.TotalExpenses, prevPeriod.Balance)
	log.Printf("Изменения: Доходы=%.1f%%, Расходы=%.1f%%, Баланс=%.1f%%",
		report.Trends.PeriodComparison.IncomeChange,
		report.Trends.PeriodComparison.ExpenseChange,
		report.Trends.PeriodComparison.BalanceChange)
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

func (s *ExpenseTracker) findCategoryChanges(changes *model.CategoryChanges, currentStats map[string]*model.CategoryStats, prevAmounts map[string]float64, categoryNames map[string]string) {
	var maxGrowthExpense, maxGrowthIncome, maxDropExpense, maxDropIncome model.CategoryChange

	for catID, stats := range currentStats {
		prevAmount := prevAmounts[catID]
		change := stats.Amount - prevAmount
		if prevAmount != 0 {
			changePercent := calculateTrendPercent(change, prevAmount)

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
