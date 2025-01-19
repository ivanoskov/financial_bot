package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ivanoskov/financial_bot/internal/service"
	"github.com/ivanoskov/financial_bot/internal/model"
)

// UserState хранит текущее состояние пользователя
type UserState struct {
	SelectedCategoryID string
	TransactionType    string // "income" или "expense"
	AwaitingAction    string // "new_category" или пусто
}

type Bot struct {
	api     *tgbotapi.BotAPI
	service *service.ExpenseTracker
	states  map[int64]*UserState // состояния пользователей по их ID
}

func NewBot(token string, service *service.ExpenseTracker) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:     bot,
		service: service,
		states:  make(map[int64]*UserState),
	}, nil
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

		// Сохраняем выбранную категорию и тип транзакции в состоянии пользователя
		b.states[callback.From.ID] = &UserState{
			SelectedCategoryID: categoryID,
			TransactionType:    transactionType,
		}
		
		msg = tgbotapi.NewMessage(callback.Message.Chat.ID, 
			fmt.Sprintf("*Категория:* %s\n\n"+
				"Введите сумму и описание в формате:\n"+
				"`1000 Покупка продуктов`", categoryName))
		msg.ParseMode = "Markdown"
		b.api.Send(msg)
	}

	// Отвечаем на callback, чтобы убрать loading indicator
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	b.api.Request(callbackResponse)

	return nil
}

func (b *Bot) handleMessage(message *tgbotapi.Message) error {
	// Проверяем, есть ли выбранная категория или ожидаемое действие
	state, exists := b.states[message.From.ID]
	if !exists {
		// Если нет активного состояния, показываем главное меню
		msg := tgbotapi.NewMessage(message.Chat.ID, "Выберите действие:")
		msg.ReplyMarkup = b.getMainKeyboard()
		b.api.Send(msg)
		return nil
	}

	// Если ожидаем создание новой категории
	if state.AwaitingAction == "new_category" {
		category := model.Category{
			UserID: message.From.ID,
			Name:   message.Text,
			Type:   state.TransactionType,
		}

		if err := b.service.CreateCategory(context.Background(), &category); err != nil {
			b.sendErrorMessage(message.Chat.ID, fmt.Sprintf("Ошибка при создании категории: %v", err))
			return nil
		}

		// Очищаем состояние и показываем обновленный список категорий
		delete(b.states, message.From.ID)
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Категория '%s' успешно создана! ✅", category.Name))
		b.api.Send(msg)
		b.handleCategories(message)
		return nil
	}

	// Обработка ввода суммы и описания транзакции
	parts := strings.SplitN(message.Text, " ", 2)
	if len(parts) != 2 {
		b.sendErrorMessage(message.Chat.ID, "Неверный формат. Используйте: <сумма> <описание>")
		return nil
	}

	amount, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Неверный формат суммы. Используйте число, например: 1000.50")
		return nil
	}

	// Если это расход, делаем сумму отрицательной
	if state.TransactionType == "expense" {
		amount = -amount
	}

	err = b.service.AddTransaction(context.Background(), 
		message.From.ID,
		state.SelectedCategoryID,
		amount,
		parts[1])

	if err != nil {
		b.sendErrorMessage(message.Chat.ID, fmt.Sprintf("Ошибка при сохранении транзакции: %v", err))
		return nil
	}

	// Очищаем состояние после сохранения транзакции
	delete(b.states, message.From.ID)

	// Отправляем сообщение об успехе и показываем главное меню
	msg := tgbotapi.NewMessage(message.Chat.ID, "Транзакция сохранена! ✅")
	msg.ReplyMarkup = b.getMainKeyboard()
	b.api.Send(msg)

	return nil
}

func (b *Bot) handleReport(message *tgbotapi.Message) {
	report, err := b.service.GetMonthlyReport(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Не удалось сформировать отчет")
		return
	}

	text := fmt.Sprintf(
		"📊 *Отчет за %s*\n\n"+
			"💰 *Доходы:* %.2f₽ ", report.Period, report.TotalIncome)
	
	// Добавляем изменение доходов
	if report.IncomeChange != 0 {
		if report.IncomeChange > 0 {
			text += fmt.Sprintf("(+%.1f%%⬆️)", report.IncomeChange)
		} else {
			text += fmt.Sprintf("(%.1f%%⬇️)", report.IncomeChange)
		}
	}

	text += fmt.Sprintf("\n💸 *Расходы:* %.2f₽ ", report.TotalExpenses)
	
	// Добавляем изменение расходов
	if report.ExpensesChange != 0 {
		if report.ExpensesChange > 0 {
			text += fmt.Sprintf("(+%.1f%%⬆️)", report.ExpensesChange)
		} else {
			text += fmt.Sprintf("(%.1f%%⬇️)", report.ExpensesChange)
		}
	}

	// Баланс
	text += fmt.Sprintf("\n💵 *Баланс:* %.2f₽\n", report.Balance)

	// Средние значения
	text += fmt.Sprintf("\n📈 *Средние показатели:*\n"+
		"• В день: %.2f₽\n"+
		"• Средняя транзакция: %.2f₽\n"+
		"• Всего транзакций: %d\n",
		report.AvgDailyExpense,
		report.AvgTransAmount,
		report.TransactionsCount)

	// Топ категорий расходов
	if len(report.TopExpenseCategories) > 0 {
		text += "\n💸 *Топ расходов:*\n"
		for _, cat := range report.TopExpenseCategories {
			text += fmt.Sprintf("• %s: %.2f₽ (%.1f%%)\n",
				cat.Name, cat.Amount, cat.Share)
		}
	}

	// Топ категорий доходов
	if len(report.TopIncomeCategories) > 0 {
		text += "\n💰 *Топ доходов:*\n"
		for _, cat := range report.TopIncomeCategories {
			text += fmt.Sprintf("• %s: %.2f₽ (%.1f%%)\n",
				cat.Name, cat.Amount, cat.Share)
		}
	}

	// Добавляем сравнение с прошлым месяцем
	text += "\n📅 *Сравнение с прошлым месяцем:*\n"
	if report.PrevMonthIncome > 0 || report.PrevMonthExpenses > 0 {
		text += fmt.Sprintf("• Доходы: %.2f₽ → %.2f₽\n"+
			"• Расходы: %.2f₽ → %.2f₽\n",
			report.PrevMonthIncome, report.TotalIncome,
			report.PrevMonthExpenses, report.TotalExpenses)
	} else {
		text += "Нет данных за прошлый месяц"
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
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
	b.states[message.From.ID] = &UserState{
		TransactionType: "income",
		AwaitingAction: "new_category",
	}
	msg := tgbotapi.NewMessage(message.Chat.ID, "*Новая категория дохода*\n\nВведите название:")
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) handleAddExpenseCategory(message *tgbotapi.Message) {
	b.states[message.From.ID] = &UserState{
		TransactionType: "expense",
		AwaitingAction: "new_category",
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

func (b *Bot) sendErrorMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, "❌ "+text)
	b.api.Send(msg)
} 