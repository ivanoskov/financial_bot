package charts

import (
	"bytes"
	"fmt"
	"time"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/ivanoskov/financial_bot/internal/service"
)

// ChartGenerator генерирует различные типы графиков
type ChartGenerator struct{}

// NewChartGenerator создает новый генератор графиков
func NewChartGenerator() *ChartGenerator {
	return &ChartGenerator{}
}

// calculateMovingAverage вычисляет скользящее среднее
func calculateMovingAverage(values []float64, window int) []float64 {
	result := make([]float64, len(values))
	for i := range values {
		count := 0
		sum := 0.0
		for j := max(0, i-window+1); j <= i; j++ {
			sum += values[j]
			count++
		}
		result[i] = sum / float64(count)
	}
	return result
}

// max возвращает максимальное из двух чисел
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GenerateFinancialDashboard создает информационную панель с финансовыми показателями
func (g *ChartGenerator) GenerateFinancialDashboard(report *service.BaseReport) ([]byte, error) {
	// Проверяем наличие данных
	if len(report.Trends.ExpenseTrend) == 0 && len(report.Trends.IncomeTrend) == 0 {
		return nil, nil // Возвращаем nil, если нет данных для графика
	}

	// Подготавливаем данные для графика трат и доходов
	xValues := make([]time.Time, len(report.Trends.ExpenseTrend))
	expenseValues := make([]float64, len(report.Trends.ExpenseTrend))
	incomeValues := make([]float64, len(report.Trends.IncomeTrend))
	balanceValues := make([]float64, len(report.Trends.ExpenseTrend))
	
	// Рассчитываем накопительный баланс и собираем данные
	runningBalance := 0.0
	for i, point := range report.Trends.ExpenseTrend {
		xValues[i] = point.Date
		expenseValues[i] = point.Amount
		incomeValues[i] = report.Trends.IncomeTrend[i].Amount
		runningBalance += incomeValues[i] - expenseValues[i]
		balanceValues[i] = runningBalance
	}

	// Рассчитываем скользящие средние
	maExpenses := calculateMovingAverage(expenseValues, 7) // 7-дневное среднее
	maIncome := calculateMovingAverage(incomeValues, 7)

	// Создаем график
	graph := chart.Chart{
		Width:  1200,
		Height: 600,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   50,
				Right:  50,
				Bottom: 50,
			},
			FillColor: chart.ColorWhite,
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat("02.01"),
			Style: chart.Style{
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		YAxis: chart.YAxis{
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%.0f₽", v.(float64))
			},
			Style: chart.Style{
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		Series: []chart.Series{
			chart.TimeSeries{
				Name:    "Расходы",
				XValues: xValues,
				YValues: expenseValues,
				Style: chart.Style{
					StrokeColor: chart.ColorRed,
					StrokeWidth: 2,
				},
			},
			chart.TimeSeries{
				Name:    "Доходы",
				XValues: xValues,
				YValues: incomeValues,
				Style: chart.Style{
					StrokeColor: chart.ColorGreen,
					StrokeWidth: 2,
				},
			},
			chart.TimeSeries{
				Name:    "Баланс",
				XValues: xValues,
				YValues: balanceValues,
				Style: chart.Style{
					StrokeColor: chart.ColorBlue,
					StrokeWidth: 3,
				},
			},
			chart.TimeSeries{
				Name:    "Тренд расходов (7 дней)",
				XValues: xValues,
				YValues: maExpenses,
				Style: chart.Style{
					StrokeColor:     chart.ColorRed.WithAlpha(100),
					StrokeWidth:     2,
					StrokeDashArray: []float64{5.0, 5.0},
				},
			},
			chart.TimeSeries{
				Name:    "Тренд доходов (7 дней)",
				XValues: xValues,
				YValues: maIncome,
				Style: chart.Style{
					StrokeColor:     chart.ColorGreen.WithAlpha(100),
					StrokeWidth:     2,
					StrokeDashArray: []float64{5.0, 5.0},
				},
			},
		},
	}

	// Добавляем легенду
	graph.Elements = []chart.Renderable{
		chart.Legend(&graph, chart.Style{
			FontSize: 12,
			FontColor: chart.ColorBlack,
		}),
	}

	// Рендерим график
	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render financial dashboard: %w", err)
	}

	return buffer.Bytes(), nil
}

// GenerateCategoryAnalysis создает анализ категорий расходов и доходов
func (g *ChartGenerator) GenerateCategoryAnalysis(report *service.BaseReport) ([]byte, error) {
	// Проверяем наличие данных
	if len(report.CategoryData.Expenses) == 0 && len(report.CategoryData.Income) == 0 {
		return nil, nil // Возвращаем nil, если нет данных для графика
	}

	// Подготавливаем данные для расходов
	expenseValues := make([]chart.Value, 0)
	totalExpenses := 0.0
	for _, cat := range report.CategoryData.Expenses {
		totalExpenses += cat.Amount
	}

	// Добавляем только категории с существенной долей (>1%)
	for _, cat := range report.CategoryData.Expenses {
		percentage := (cat.Amount / totalExpenses) * 100
		if percentage > 1.0 {
			expenseValues = append(expenseValues, chart.Value{
				Label: fmt.Sprintf("%s: %.0f₽ (%.1f%%)", cat.Name, cat.Amount, percentage),
				Value: cat.Amount,
			})
		}
	}

	// Создаем круговую диаграмму
	pie := chart.PieChart{
		Width:  1200,
		Height: 600,
		Values: expenseValues,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   50,
				Right:  50,
				Bottom: 50,
			},
			FillColor: chart.ColorWhite,
		},
	}

	// Рендерим график
	buffer := bytes.NewBuffer([]byte{})
	err := pie.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render category analysis: %w", err)
	}

	return buffer.Bytes(), nil
}

// GenerateExpenseChart создает график расходов
func (g *ChartGenerator) GenerateExpenseChart(report *service.BaseReport) ([]byte, error) {
	// Подготавливаем данные
	xValues := make([]time.Time, len(report.Trends.ExpenseTrend))
	expenseValues := make([]float64, len(report.Trends.ExpenseTrend))
	incomeValues := make([]float64, len(report.Trends.IncomeTrend))

	for i, point := range report.Trends.ExpenseTrend {
		xValues[i] = point.Date
		expenseValues[i] = point.Amount
	}

	for i, point := range report.Trends.IncomeTrend {
		incomeValues[i] = point.Amount
	}

	graph := chart.Chart{
		Width:  800,
		Height: 400,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    20,
				Left:   20,
				Right:  20,
				Bottom: 20,
			},
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat("02.01"),
		},
		YAxis: chart.YAxis{
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%.0f₽", v.(float64))
			},
		},
		Series: []chart.Series{
			chart.TimeSeries{
				Name:    "Расходы",
				XValues: xValues,
				YValues: expenseValues,
				Style: chart.Style{
					StrokeColor: chart.ColorRed,
				},
			},
			chart.TimeSeries{
				Name:    "Доходы",
				XValues: xValues,
				YValues: incomeValues,
				Style: chart.Style{
					StrokeColor: chart.ColorGreen,
				},
			},
		},
	}

	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render expense chart: %w", err)
	}

	return buffer.Bytes(), nil
}

// GenerateCategoryPieChart создает круговую диаграмму распределения по категориям
func (g *ChartGenerator) GenerateCategoryPieChart(report *service.BaseReport, isExpense bool) ([]byte, error) {
	// Подготавливаем данные
	categories := report.CategoryData.Expenses
	title := "Распределение расходов"
	if !isExpense {
		categories = report.CategoryData.Income
		title = "Распределение доходов"
	}

	if len(categories) == 0 {
		return nil, nil
	}

	values := make([]chart.Value, 0)
	total := 0.0
	for _, cat := range categories {
		total += cat.Amount
	}

	// Добавляем только категории с существенной долей (>1%)
	for _, cat := range categories {
		percentage := (cat.Amount / total) * 100
		if percentage > 1.0 {
			values = append(values, chart.Value{
				Label: fmt.Sprintf("%s: %.0f₽ (%.1f%%)", cat.Name, cat.Amount, percentage),
				Value: cat.Amount,
				Style: chart.Style{
					FontSize: 12,
					FontColor: chart.ColorBlack,
				},
			})
		}
	}

	pie := chart.PieChart{
		Title:  title,
		Width:  800,
		Height: 800,
		Values: values,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   50,
				Right:  50,
				Bottom: 50,
			},
			FillColor: chart.ColorWhite,
		},
	}

	buffer := bytes.NewBuffer([]byte{})
	err := pie.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render category pie chart: %w", err)
	}

	return buffer.Bytes(), nil
}

// GenerateTrendChart создает график трендов
func (g *ChartGenerator) GenerateTrendChart(report *service.BaseReport) ([]byte, error) {
	// Подготавливаем данные
	xValues := make([]time.Time, len(report.Trends.ExpenseTrend))
	expenseChanges := make([]float64, len(report.Trends.ExpenseTrend))
	incomeChanges := make([]float64, len(report.Trends.IncomeTrend))

	for i, point := range report.Trends.ExpenseTrend {
		xValues[i] = point.Date
		expenseChanges[i] = point.Change
		incomeChanges[i] = report.Trends.IncomeTrend[i].Change
	}

	graph := chart.Chart{
		Title: "Тренды изменений",
		Width:  1200,
		Height: 600,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   50,
				Right:  50,
				Bottom: 50,
			},
			FillColor: chart.ColorWhite,
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat("02.01"),
			Style: chart.Style{
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		YAxis: chart.YAxis{
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%.0f%%", v.(float64))
			},
			Style: chart.Style{
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		Series: []chart.Series{
			chart.TimeSeries{
				Name:    "Изменение расходов",
				XValues: xValues,
				YValues: expenseChanges,
				Style: chart.Style{
					StrokeColor: chart.ColorRed,
					StrokeWidth: 2,
				},
			},
			chart.TimeSeries{
				Name:    "Изменение доходов",
				XValues: xValues,
				YValues: incomeChanges,
				Style: chart.Style{
					StrokeColor: chart.ColorGreen,
					StrokeWidth: 2,
				},
			},
		},
	}

	// Добавляем легенду
	graph.Elements = []chart.Renderable{
		chart.Legend(&graph, chart.Style{
			FontSize: 12,
			FontColor: chart.ColorBlack,
		}),
	}

	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render trend chart: %w", err)
	}

	return buffer.Bytes(), nil
}

// GenerateBalanceChart создает график баланса
func (g *ChartGenerator) GenerateBalanceChart(report *service.BaseReport) ([]byte, error) {
	// Подготавливаем данные
	bars := []chart.Value{
		{
			Label: fmt.Sprintf("Баланс (пред.): %.0f₽", report.Trends.PeriodComparison.PrevPeriod.Balance),
			Value: report.Trends.PeriodComparison.PrevPeriod.Balance,
			Style: chart.Style{
				StrokeColor: chart.ColorBlue,
				FillColor:   chart.ColorBlue.WithAlpha(100),
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		{
			Label: fmt.Sprintf("Баланс (тек.): %.0f₽", report.Trends.PeriodComparison.CurrentPeriod.Balance),
			Value: report.Trends.PeriodComparison.CurrentPeriod.Balance,
			Style: chart.Style{
				StrokeColor: chart.ColorBlue,
				FillColor:   chart.ColorBlue,
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		{
			Label: fmt.Sprintf("Расходы (пред.): %.0f₽", report.Trends.PeriodComparison.PrevPeriod.TotalExpenses),
			Value: -report.Trends.PeriodComparison.PrevPeriod.TotalExpenses,
			Style: chart.Style{
				StrokeColor: chart.ColorRed,
				FillColor:   chart.ColorRed.WithAlpha(100),
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		{
			Label: fmt.Sprintf("Расходы (тек.): %.0f₽", report.Trends.PeriodComparison.CurrentPeriod.TotalExpenses),
			Value: -report.Trends.PeriodComparison.CurrentPeriod.TotalExpenses,
			Style: chart.Style{
				StrokeColor: chart.ColorRed,
				FillColor:   chart.ColorRed,
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		{
			Label: fmt.Sprintf("Доходы (пред.): %.0f₽", report.Trends.PeriodComparison.PrevPeriod.TotalIncome),
			Value: report.Trends.PeriodComparison.PrevPeriod.TotalIncome,
			Style: chart.Style{
				StrokeColor: chart.ColorGreen,
				FillColor:   chart.ColorGreen.WithAlpha(100),
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		{
			Label: fmt.Sprintf("Доходы (тек.): %.0f₽", report.Trends.PeriodComparison.CurrentPeriod.TotalIncome),
			Value: report.Trends.PeriodComparison.CurrentPeriod.TotalIncome,
			Style: chart.Style{
				StrokeColor: chart.ColorGreen,
				FillColor:   chart.ColorGreen,
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
	}

	graph := chart.BarChart{
		Title:      "Сравнение периодов",
		TitleStyle: chart.Style{
			FontSize: 14,
			FontColor: chart.ColorBlack,
		},
		Width:      1200,
		Height:     600,
		BarWidth:   60,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   50,
				Right:  50,
				Bottom: 50,
			},
			FillColor: chart.ColorWhite,
		},
		YAxis: chart.YAxis{
			ValueFormatter: func(v interface{}) string {
				return fmt.Sprintf("%.0f₽", v.(float64))
			},
			Style: chart.Style{
				FontSize: 12,
				FontColor: chart.ColorBlack,
			},
		},
		Bars: bars,
	}

	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render balance chart: %w", err)
	}

	return buffer.Bytes(), nil
} 