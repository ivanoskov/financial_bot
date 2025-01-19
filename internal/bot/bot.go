package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ivanoskov/financial_bot/internal/charts"
	"github.com/ivanoskov/financial_bot/internal/model"
	"github.com/ivanoskov/financial_bot/internal/service"
)

// UserState хранит текущее состояние пользователя
type UserState struct {
	SelectedCategoryID string
	TransactionType    string // "income" или "expense"
	AwaitingAction     string // "new_category" или пусто
}

type Bot struct {
	api      *tgbotapi.BotAPI
	service  *service.ExpenseTracker
	chartGen *charts.ChartGenerator
}

func NewBot(token string, service *service.ExpenseTracker) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:      bot,
		service:  service,
		chartGen: charts.NewChartGenerator(),
	}, nil
}

// getUserState получает состояние пользователя из БД
func (b *Bot) getUserState(ctx context.Context, userID int64) (*model.UserState, error) {
	return b.service.GetUserState(ctx, userID)
}

// saveUserState сохраняет состояние пользователя в БД
func (b *Bot) saveUserState(ctx context.Context, state *model.UserState) error {
	return b.service.SaveUserState(ctx, state)
}

// deleteUserState удаляет состояние пользователя из БД
func (b *Bot) deleteUserState(ctx context.Context, userID int64) error {
	return b.service.DeleteUserState(ctx, userID)
}

func (b *Bot) handleUpdate(update tgbotapi.Update) error {
	if update.Message == nil && update.CallbackQuery == nil {
		return nil
	}

	if update.Message != nil && update.Message.IsCommand() {
		return b.handleCommand(update.Message)
	}

	if update.CallbackQuery != nil {
		return b.handleCallback(update.CallbackQuery)
	}

	if update.Message != nil {
		return b.handleMessage(update.Message)
	}

	return nil
}

// Start запускает бота в режиме long polling
func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if err := b.handleUpdate(update); err != nil {
			// Логируем ошибку, но продолжаем работу
			fmt.Printf("Error handling update: %v\n", err)
		}
	}

	return nil
}

// HandleWebhook - точка входа для обработки входящих webhook-обновлений
func (b *Bot) HandleWebhook(body []byte) error {
	var update tgbotapi.Update
	if err := json.Unmarshal(body, &update); err != nil {
		return err
	}

	return b.handleUpdate(update)
}

func (b *Bot) handleCommand(message *tgbotapi.Message) error {
	cmd := message.Command()

	switch cmd {
	case "start":
		b.handleStart(message)
	case "add":
		b.handleAddTransaction(message)
	case "report":
		b.handleReport(message)
	case "categories":
		b.handleCategories(message)
	}

	return nil
}

func (b *Bot) handleStart(message *tgbotapi.Message) {
	// Создаем категории по умолчанию при первом запуске
	err := b.service.CreateDefaultCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, fmt.Sprintf("Не удалось создать стандартные категории: %v", err))
		return
	}

	keyboard := b.getMainKeyboard()
	msg := tgbotapi.NewMessage(message.Chat.ID,
		"*Привет! Я помогу вести учет финансов* 💰\n\n"+
			"Вот что я умею:\n"+
			"• Записывать доходы и расходы\n"+
			"• Показывать отчеты по категориям\n"+
			"• Управлять категориями\n\n"+
			"*Выберите нужное действие в меню ниже* 👇")

	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleAddTransaction(message *tgbotapi.Message) {
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при получении категорий")
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Выберите категорию:")
	msg.ReplyMarkup = b.getCategoriesKeyboard(categories)
	b.api.Send(msg)
}

func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) error {
	var msg tgbotapi.MessageConfig

	switch {
	case callback.Data == "action_add_income":
		b.handleAddIncome(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case callback.Data == "action_add_expense":
		b.handleAddExpense(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case callback.Data == "action_report":
		b.handleReport(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case callback.Data == "action_categories":
		b.handleCategories(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case callback.Data == "action_transactions":
		b.handleTransactions(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case callback.Data == "add_income_category":
		b.handleAddIncomeCategory(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case callback.Data == "add_expense_category":
		b.handleAddExpenseCategory(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case callback.Data == "action_back":
		msg = tgbotapi.NewMessage(callback.Message.Chat.ID, "*Главное меню*\nВыберите нужное действие 👇")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = b.getMainKeyboard()
		b.api.Send(msg)
	case strings.HasPrefix(callback.Data, "delete_transaction_"):
		transactionID := strings.TrimPrefix(callback.Data, "delete_transaction_")
		err := b.service.DeleteTransaction(context.Background(), transactionID, callback.From.ID)
		if err != nil {
			return fmt.Errorf("error deleting transaction: %w", err)
		}
		// Обновляем список транзакций
		b.handleTransactions(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case strings.HasPrefix(callback.Data, "delete_category_"):
		categoryID := strings.TrimPrefix(callback.Data, "delete_category_")
		err := b.service.DeleteCategory(context.Background(), categoryID, callback.From.ID)
		if err != nil {
			return fmt.Errorf("error deleting category: %w", err)
		}
		// Обновляем список категорий
		b.handleCategories(&tgbotapi.Message{
			From: callback.From,
			Chat: callback.Message.Chat,
		})
	case strings.HasPrefix(callback.Data, "category_"):
		categoryID := strings.TrimPrefix(callback.Data, "category_")

		// Получаем категорию для определения типа транзакции
		categories, err := b.service.GetCategories(context.Background(), callback.From.ID)
		if err != nil {
			return fmt.Errorf("error getting categories: %w", err)
		}

		var transactionType string
		var categoryName string
		for _, cat := range categories {
			if cat.ID == categoryID {
				transactionType = cat.Type
				categoryName = cat.Name
				break
			}
		}

		// Сохраняем состояние в БД
		state := &model.UserState{
			UserID:           callback.From.ID,
			SelectedCategory: categoryID,
			TransactionType:  transactionType,
		}
		if err := b.saveUserState(context.Background(), state); err != nil {
			return fmt.Errorf("error saving user state: %w", err)
		}

		msg = tgbotapi.NewMessage(callback.Message.Chat.ID,
			fmt.Sprintf("*Категория:* %s\n\n"+
				"Введите сумму и описание в формате:\n"+
				"`1000 Покупка продуктов`", categoryName))
		msg.ParseMode = "Markdown"
		b.api.Send(msg)
	case callback.Data == "report_daily":
		b.sendReport(callback.Message.Chat.ID, callback.From.ID, service.DailyReport)
	case callback.Data == "report_weekly":
		b.sendReport(callback.Message.Chat.ID, callback.From.ID, service.WeeklyReport)
	case callback.Data == "report_monthly":
		b.sendReport(callback.Message.Chat.ID, callback.From.ID, service.MonthlyReport)
	case callback.Data == "report_yearly":
		b.sendReport(callback.Message.Chat.ID, callback.From.ID, service.YearlyReport)
	case callback.Data == "report_charts":
		// Получаем отчет для графиков
		report, err := b.service.GetReport(context.Background(), callback.From.ID, service.MonthlyReport)
		if err != nil {
			b.sendErrorMessage(callback.Message.Chat.ID, "Не удалось сформировать отчет для графиков")
			return nil
		}
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "📊 Графический анализ...")
		b.api.Send(msg)
		err = b.sendCharts(context.Background(), callback.Message.Chat.ID, report)
		if err != nil {
			b.sendErrorMessage(callback.Message.Chat.ID, fmt.Sprintf("Не удалось сгенерировать графики: %v", err))
		}
	}

	// Отвечаем на callback, чтобы убрать loading indicator
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	b.api.Request(callbackResponse)

	return nil
}

func (b *Bot) handleMessage(message *tgbotapi.Message) error {
	// Проверяем состояние пользователя в БД
	state, err := b.getUserState(context.Background(), message.From.ID)
	if err != nil {
		return fmt.Errorf("error getting user state: %w", err)
	}

	fmt.Printf("Current user state: %+v\n", state)

	if state == nil {
		// Если нет активного состояния, показываем главное меню
		msg := tgbotapi.NewMessage(message.Chat.ID, "Выберите действие:")
		msg.ReplyMarkup = b.getMainKeyboard()
		b.api.Send(msg)
		return nil
	}

	// Если ожидаем создание новой категории
	if state.AwaitingAction == "new_category" {
		fmt.Printf("Creating new category: %s, type: %s\n", message.Text, state.TransactionType)
		category := model.Category{
			UserID: message.From.ID,
			Name:   message.Text,
			Type:   state.TransactionType,
		}

		if err := b.service.CreateCategory(context.Background(), &category); err != nil {
			b.sendErrorMessage(message.Chat.ID, fmt.Sprintf("Ошибка при создании категории: %v", err))
			return nil
		}

		// Очищаем состояние
		if err := b.deleteUserState(context.Background(), message.From.ID); err != nil {
			return fmt.Errorf("error deleting user state: %w", err)
		}

		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Категория '%s' успешно создана! ✅", category.Name))
		b.api.Send(msg)
		b.handleCategories(message)
		return nil
	}

	// Обработка ввода суммы и описания транзакции
	parts := strings.SplitN(message.Text, " ", 2)
	amount, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Неверный формат суммы. Используйте число, например: 1000.50")
		return nil
	}

	// Если это расход, делаем сумму отрицательной
	if state.TransactionType == "expense" {
		amount = -amount
	}

	// Получаем описание, если оно есть
	description := ""
	if len(parts) > 1 {
		description = parts[1]
	}

	err = b.service.AddTransaction(context.Background(),
		message.From.ID,
		state.SelectedCategory,
		amount,
		description)

	if err != nil {
		b.sendErrorMessage(message.Chat.ID, fmt.Sprintf("Ошибка при сохранении транзакции: %v", err))
		return nil
	}

	// Очищаем состояние после сохранения транзакции
	if err := b.deleteUserState(context.Background(), message.From.ID); err != nil {
		return fmt.Errorf("error deleting user state: %w", err)
	}

	// Отправляем сообщение об успехе и показываем главное меню
	msg := tgbotapi.NewMessage(message.Chat.ID, "Транзакция сохранена! ✅")
	msg.ReplyMarkup = b.getMainKeyboard()
	b.api.Send(msg)

	return nil
}

func (b *Bot) handleReport(message *tgbotapi.Message) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 За день", "report_daily"),
			tgbotapi.NewInlineKeyboardButtonData("📈 За неделю", "report_weekly"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 За месяц", "report_monthly"),
			tgbotapi.NewInlineKeyboardButtonData("📅 За год", "report_yearly"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Графики", "report_charts"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Назад", "action_back"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID,
		"*Выберите период для отчета:*\n\n"+
			"• За день - детальный анализ расходов за текущий день\n"+
			"• За неделю - анализ трендов за последние 7 дней\n"+
			"• За месяц - полный анализ за текущий месяц\n"+
			"• За год - годовая статистика и тренды\n"+
			"• Графики - визуальный анализ ваших финансов")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleCategories(message *tgbotapi.Message) {
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Не удалось загрузить категории")
		return
	}

	// Группируем категории по типу
	incomeCategories := make([]model.Category, 0)
	expenseCategories := make([]model.Category, 0)
	for _, cat := range categories {
		if cat.Type == "income" {
			incomeCategories = append(incomeCategories, cat)
		} else {
			expenseCategories = append(expenseCategories, cat)
		}
	}

	text := "*Ваши категории*\n\n"
	if len(incomeCategories) > 0 {
		text += "💰 *Доходы:*\n"
		for _, cat := range incomeCategories {
			text += fmt.Sprintf("• %s\n", cat.Name)
		}
	}

	if len(expenseCategories) > 0 {
		if len(incomeCategories) > 0 {
			text += "\n"
		}
		text += "💸 *Расходы:*\n"
		for _, cat := range expenseCategories {
			text += fmt.Sprintf("• %s\n", cat.Name)
		}
	}

	text += "\nНажмите на категорию для добавления транзакции или 🗑 для удаления"

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = b.getCategoriesKeyboard(categories)
	b.api.Send(msg)
}

// Добавляем новые методы для обработки доходов и расходов
func (b *Bot) handleAddExpense(message *tgbotapi.Message) {
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Не удалось загрузить категории")
		return
	}

	// Фильтруем только категории расходов
	expenseCategories := make([]model.Category, 0)
	for _, cat := range categories {
		if cat.Type == "expense" {
			expenseCategories = append(expenseCategories, cat)
		}
	}

	if len(expenseCategories) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID,
			"*У вас нет категорий расходов*\n\nСначала создайте хотя бы одну категорию:")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = b.getCategoriesKeyboard(categories)
		b.api.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "*Добавление расхода*\n\nВыберите категорию:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = b.getSelectCategoryKeyboard(expenseCategories)
	b.api.Send(msg)
}

func (b *Bot) handleAddIncome(message *tgbotapi.Message) {
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Не удалось загрузить категории")
		return
	}

	// Фильтруем только категории доходов
	incomeCategories := make([]model.Category, 0)
	for _, cat := range categories {
		if cat.Type == "income" {
			incomeCategories = append(incomeCategories, cat)
		}
	}

	if len(incomeCategories) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID,
			"*У вас нет категорий доходов*\n\nСначала создайте хотя бы одну категорию:")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = b.getCategoriesKeyboard(categories)
		b.api.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "*Добавление дохода*\n\nВыберите категорию:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = b.getSelectCategoryKeyboard(incomeCategories)
	b.api.Send(msg)
}

// Добавляем новые методы для управления категориями
func (b *Bot) handleAddIncomeCategory(message *tgbotapi.Message) {
	state := &model.UserState{
		UserID:          message.From.ID,
		TransactionType: "income",
		AwaitingAction:  "new_category",
	}
	if err := b.saveUserState(context.Background(), state); err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при сохранении состояния")
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "*Новая категория дохода*\n\nВведите название:")
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) handleAddExpenseCategory(message *tgbotapi.Message) {
	state := &model.UserState{
		UserID:          message.From.ID,
		TransactionType: "expense",
		AwaitingAction:  "new_category",
	}
	if err := b.saveUserState(context.Background(), state); err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при сохранении состояния")
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "*Новая категория расхода*\n\nВведите название:")
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) handleTransactions(message *tgbotapi.Message) {
	// Получаем последние 10 транзакций
	transactions, err := b.service.GetRecentTransactions(context.Background(), message.From.ID, 10)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Не удалось загрузить транзакции")
		return
	}

	if len(transactions) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "*История транзакций*\n\nУ вас пока нет транзакций")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = b.getMainKeyboard()
		b.api.Send(msg)
		return
	}

	// Получаем категории для отображения их названий
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Не удалось загрузить категории")
		return
	}

	categoryNames := make(map[string]string)
	for _, cat := range categories {
		categoryNames[cat.ID] = cat.Name
	}

	text := "*Последние транзакции*\nНажмите на транзакцию для её удаления\n\n"
	var buttons [][]tgbotapi.InlineKeyboardButton

	for _, t := range transactions {
		categoryName := categoryNames[t.CategoryID]
		emoji := "💸"
		amountStr := fmt.Sprintf("%.2f₽", -t.Amount)
		if t.Amount > 0 {
			emoji = "💰"
			amountStr = fmt.Sprintf("%.2f₽", t.Amount)
		}

		text += fmt.Sprintf("%s *%s*: %s _%s_\n",
			emoji, categoryName, amountStr, t.Description)

		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s %s: %s", emoji, categoryName, amountStr),
				"delete_transaction_"+t.ID,
			),
		})
	}

	// Добавляем кнопку "Назад"
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("« Назад", "action_back"),
	})

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	b.api.Send(msg)
}

func (b *Bot) sendReport(chatID int64, userID int64, reportType service.ReportType) {
	report, err := b.service.GetReport(context.Background(), userID, reportType)
	if err != nil {
		b.sendErrorMessage(chatID, "Не удалось сформировать отчет")
		return
	}

	// Формируем текст отчета
	text := fmt.Sprintf("📊 *Отчет за %s*\n\n", report.Period)

	// Основные показатели
	text += "*Основные показатели:*\n"
	text += fmt.Sprintf("💰 Доходы: *%.0f₽*", report.TotalIncome)
	if report.Trends.PeriodComparison.IncomeChange != 0 {
		if report.Trends.PeriodComparison.IncomeChange > 0 {
			text += fmt.Sprintf(" (+%.1f%%⬆️)", report.Trends.PeriodComparison.IncomeChange)
		} else {
			text += fmt.Sprintf(" (%.1f%%⬇️)", report.Trends.PeriodComparison.IncomeChange)
		}
	}
	text += "\n"

	text += fmt.Sprintf("💸 Расходы: *%.0f₽*", report.TotalExpenses)
	if report.Trends.PeriodComparison.ExpenseChange != 0 {
		if report.Trends.PeriodComparison.ExpenseChange > 0 {
			text += fmt.Sprintf(" (+%.1f%%⬆️)", report.Trends.PeriodComparison.ExpenseChange)
		} else {
			text += fmt.Sprintf(" (%.1f%%⬇️)", report.Trends.PeriodComparison.ExpenseChange)
		}
	}
	text += "\n"

	text += fmt.Sprintf("💵 Баланс: *%.0f₽*", report.Balance)
	if report.Trends.PeriodComparison.BalanceChange != 0 {
		if report.Trends.PeriodComparison.BalanceChange > 0 {
			text += fmt.Sprintf(" (+%.1f%%⬆️)", report.Trends.PeriodComparison.BalanceChange)
		} else {
			text += fmt.Sprintf(" (%.1f%%⬇️)", report.Trends.PeriodComparison.BalanceChange)
		}
	}
	text += "\n\n"

	// Статистика транзакций
	text += "*Статистика транзакций:*\n"
	text += fmt.Sprintf("• Всего: *%.d* (💰 *%d*, 💸 *%d*)\n",
		report.TransactionData.TotalCount,
		report.TransactionData.IncomeCount,
		report.TransactionData.ExpenseCount)
	text += fmt.Sprintf("• Средний доход: *%.0f₽*\n", report.TransactionData.AvgIncome)
	text += fmt.Sprintf("• Средний расход: *%.0f₽*\n", report.TransactionData.AvgExpense)
	text += fmt.Sprintf("• В день (доходы): *%.0f₽*\n", report.TransactionData.DailyAvgIncome)
	text += fmt.Sprintf("• В день (расходы): *%.0f₽*\n\n", report.TransactionData.DailyAvgExpense)

	// Максимальные транзакции
	text += "*Крупнейшие транзакции:*\n"
	if report.TransactionData.MaxIncome.Amount > 0 {
		text += fmt.Sprintf("💰 +*%.0f₽*: %s\n",
			report.TransactionData.MaxIncome.Amount,
			report.TransactionData.MaxIncome.Description)
	}
	if report.TransactionData.MaxExpense.Amount > 0 {
		text += fmt.Sprintf("💸 -*%.0f₽*: %s\n\n",
			report.TransactionData.MaxExpense.Amount,
			report.TransactionData.MaxExpense.Description)
	}

	// Категории расходов
	if len(report.CategoryData.Expenses) > 0 {
		text += "*Топ категорий расходов:*\n"
		for _, cat := range report.CategoryData.Expenses {
			text += fmt.Sprintf("• *%s*: *%.0f₽* (%.1f%%)",
				cat.Name, cat.Amount, cat.Share)
			if cat.TrendPercent != 0 {
				if cat.TrendPercent > 0 {
					text += fmt.Sprintf(" (+%.1f%%⬆️)", cat.TrendPercent)
				} else {
					text += fmt.Sprintf(" (%.1f%%⬇️)", cat.TrendPercent)
				}
			}
			text += "\n"
		}
		text += "\n"
	}

	// Категории доходов
	if len(report.CategoryData.Income) > 0 {
		text += "*Топ категорий доходов:*\n"
		for _, cat := range report.CategoryData.Income {
			text += fmt.Sprintf("• *%s*: *%.0f₽* (%.1f%%)",
				cat.Name, cat.Amount, cat.Share)
			if cat.TrendPercent != 0 {
				if cat.TrendPercent > 0 {
					text += fmt.Sprintf(" (+%.1f%%⬆️)", cat.TrendPercent)
				} else {
					text += fmt.Sprintf(" (%.1f%%⬇️)", cat.TrendPercent)
				}
			}
			text += "\n"
		}
		text += "\n"
	}

	// Значительные изменения
	text += "*Значительные изменения:*\n"
	if report.CategoryData.Changes.FastestGrowingExpense.Name != "" {
		text += fmt.Sprintf("📈 *Быстрее всего растут расходы в категории '%s': %.1f%%*\n",
			report.CategoryData.Changes.FastestGrowingExpense.Name,
			report.CategoryData.Changes.FastestGrowingExpense.ChangePercent)
	}
	if report.CategoryData.Changes.LargestDropExpense.Name != "" {
		text += fmt.Sprintf("📉 *Сильнее всего снизились расходы в '%s': %.1f%%*\n",
			report.CategoryData.Changes.LargestDropExpense.Name,
			report.CategoryData.Changes.LargestDropExpense.ChangePercent)
	}
	if report.CategoryData.Changes.FastestGrowingIncome.Name != "" {
		text += fmt.Sprintf("📈 *Быстрее всего растут доходы в '%s': %.1f%%*\n",
			report.CategoryData.Changes.FastestGrowingIncome.Name,
			report.CategoryData.Changes.FastestGrowingIncome.ChangePercent)
	}
	if report.CategoryData.Changes.LargestDropIncome.Name != "" {
		text += fmt.Sprintf("📉 *Сильнее всего снизились доходы в '%s': %.1f%%*\n",
			report.CategoryData.Changes.LargestDropIncome.Name,
			report.CategoryData.Changes.LargestDropIncome.ChangePercent)
	}

	// Добавляем кнопки
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Графики", "report_charts"),
			tgbotapi.NewInlineKeyboardButtonData("« В меню", "action_back"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) sendCharts(ctx context.Context, chatID int64, report *service.BaseReport) error {
	// Отправляем сообщение о начале генерации
	msg := tgbotapi.NewMessage(chatID, "📊 Генерация графиков...")
	b.api.Send(msg)

	// Генерируем все графики
	log.Printf("Generating financial dashboard...")
	dashboardData, err := b.chartGen.GenerateFinancialDashboard(report)
	if err != nil {
		return fmt.Errorf("failed to generate financial dashboard: %w", err)
	}

	log.Printf("Generating expense categories analysis...")
	expenseCategoriesData, err := b.chartGen.GenerateCategoryPieChart(report, true)
	if err != nil {
		return fmt.Errorf("failed to generate expense categories chart: %w", err)
	}

	log.Printf("Generating income categories analysis...")
	incomeCategoriesData, err := b.chartGen.GenerateCategoryPieChart(report, false)
	if err != nil {
		return fmt.Errorf("failed to generate income categories chart: %w", err)
	}

	log.Printf("Generating trends chart...")
	trendsData, err := b.chartGen.GenerateTrendChart(report)
	if err != nil {
		return fmt.Errorf("failed to generate trends chart: %w", err)
	}

	log.Printf("Generating balance chart...")
	balanceData, err := b.chartGen.GenerateBalanceChart(report)
	if err != nil {
		return fmt.Errorf("failed to generate balance chart: %w", err)
	}

	// Собираем все графики в одно сообщение
	var media []interface{}

	if len(dashboardData) > 0 {
		media = append(media, tgbotapi.NewInputMediaPhoto(tgbotapi.FileBytes{
			Name:  "1_dashboard.png",
			Bytes: dashboardData,
		}))
	}

	if len(expenseCategoriesData) > 0 {
		media = append(media, tgbotapi.NewInputMediaPhoto(tgbotapi.FileBytes{
			Name:  "2_expenses.png",
			Bytes: expenseCategoriesData,
		}))
	}

	if len(incomeCategoriesData) > 0 {
		media = append(media, tgbotapi.NewInputMediaPhoto(tgbotapi.FileBytes{
			Name:  "3_income.png",
			Bytes: incomeCategoriesData,
		}))
	}

	if len(trendsData) > 0 {
		media = append(media, tgbotapi.NewInputMediaPhoto(tgbotapi.FileBytes{
			Name:  "4_trends.png",
			Bytes: trendsData,
		}))
	}

	if len(balanceData) > 0 {
		media = append(media, tgbotapi.NewInputMediaPhoto(tgbotapi.FileBytes{
			Name:  "5_balance.png",
			Bytes: balanceData,
		}))
	}

	if len(media) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Недостаточно данных для построения графиков")
		b.api.Send(msg)
		return nil
	}

	// Добавляем описание к первому изображению
	if mediaPhoto, ok := media[0].(*tgbotapi.InputMediaPhoto); ok {
		mediaPhoto.Caption = "📊 *Графический анализ*\n\n" +
			"1. Динамика доходов и расходов\n" +
			"2. Распределение расходов по категориям\n" +
			"3. Распределение доходов по категориям\n" +
			"4. Тренды изменений\n" +
			"5. Сравнение периодов"
		mediaPhoto.ParseMode = "Markdown"
	}

	// Отправляем все графики одним сообщением
	mediaGroup := tgbotapi.NewMediaGroup(chatID, media)
	_, err = b.api.SendMediaGroup(mediaGroup)
	if err != nil {
		return fmt.Errorf("failed to send charts: %w", err)
	}

	// Добавляем кнопки навигации
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 К отчетам", "action_report"),
			tgbotapi.NewInlineKeyboardButtonData("« В меню", "action_back"),
		),
	)

	msg = tgbotapi.NewMessage(chatID, "Выберите действие:")
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)

	return nil
}

func (b *Bot) sendErrorMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, "❌ "+text)
	b.api.Send(msg)
}

// SendDailyReport отправляет ежедневный отчет пользователю
func (b *Bot) SendDailyReport(ctx context.Context, userID int64, report *service.BaseReport) error {
	// Формируем текст отчета
	text := "*Ваша финансовая сводка за прошедший день:*\n\n"

	// Основные показатели
	text += "*Основные показатели:*\n"
	text += fmt.Sprintf("💰 Доходы: %.2f₽", report.TotalIncome)
	if report.Trends.PeriodComparison.IncomeChange != 0 {
		if report.Trends.PeriodComparison.IncomeChange > 0 {
			text += fmt.Sprintf(" (+%.1f%%⬆️)", report.Trends.PeriodComparison.IncomeChange)
		} else {
			text += fmt.Sprintf(" (%.1f%%⬇️)", report.Trends.PeriodComparison.IncomeChange)
		}
	}
	text += "\n"

	text += fmt.Sprintf("💸 Расходы: %.2f₽", report.TotalExpenses)
	if report.Trends.PeriodComparison.ExpenseChange != 0 {
		if report.Trends.PeriodComparison.ExpenseChange > 0 {
			text += fmt.Sprintf(" (+%.1f%%⬆️)", report.Trends.PeriodComparison.ExpenseChange)
		} else {
			text += fmt.Sprintf(" (%.1f%%⬇️)", report.Trends.PeriodComparison.ExpenseChange)
		}
	}
	text += "\n"

	text += fmt.Sprintf("💵 Баланс: %.2f₽", report.Balance)
	if report.Trends.PeriodComparison.BalanceChange != 0 {
		if report.Trends.PeriodComparison.BalanceChange > 0 {
			text += fmt.Sprintf(" (+%.1f%%⬆️)", report.Trends.PeriodComparison.BalanceChange)
		} else {
			text += fmt.Sprintf(" (%.1f%%⬇️)", report.Trends.PeriodComparison.BalanceChange)
		}
	}
	text += "\n\n"

	// Добавляем кнопки
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Подробный отчет", "report_daily"),
			tgbotapi.NewInlineKeyboardButtonData("📈 Графики", "report_charts"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	_, err := b.api.Send(msg)

	return err
}
