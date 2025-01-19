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
		b.sendErrorMessage(message.Chat.ID, "Ошибка при создании категорий")
		return
	}

	keyboard := b.getMainKeyboard()
	msg := tgbotapi.NewMessage(message.Chat.ID,
		"Добро пожаловать в бот учёта расходов! 💰\n\n" +
		"Я помогу вам отслеживать ваши доходы и расходы. Вот что я умею:\n\n" +
		"• Добавлять доходы и расходы\n" +
		"• Показывать отчёты и графики\n" +
		"• Управлять категориями\n\n" +
		"Выберите действие:")
	
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
		msg = tgbotapi.NewMessage(callback.Message.Chat.ID, "Выберите действие:")
		msg.ReplyMarkup = b.getMainKeyboard()
		b.api.Send(msg)
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
			fmt.Sprintf("Категория: %s\nВведите сумму и описание в формате:\n1000 Покупка продуктов", categoryName))
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
		b.sendErrorMessage(message.Chat.ID, "Ошибка при формировании отчета")
		return
	}

	// Получаем категории для отображения их названий
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при получении категорий")
		return
	}

	// Создаем мапу ID -> Name для категорий
	categoryNames := make(map[string]string)
	for _, cat := range categories {
		categoryNames[cat.ID] = cat.Name
	}

	text := fmt.Sprintf(
		"📊 Отчет %s\n\n"+
			"💰 Доходы: %.2f\n"+
			"💸 Расходы: %.2f\n"+
			"💵 Баланс: %.2f\n\n"+
			"По категориям:\n",
		report.Period,
		report.TotalIncome,
		report.TotalExpenses,
		report.TotalIncome-report.TotalExpenses,
	)

	// Добавляем информацию по категориям
	for categoryID, amount := range report.ByCategory {
		categoryName := categoryNames[categoryID]
		emoji := "💸"
		if amount > 0 {
			emoji = "💰"
		}
		text += fmt.Sprintf("%s %s: %.2f\n", emoji, categoryName, amount)
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	b.api.Send(msg)
}

func (b *Bot) handleCategories(message *tgbotapi.Message) {
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при получении категорий")
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

	text := "📋 Ваши категории:\n\n💰 Доходы:\n"
	for _, cat := range incomeCategories {
		text += fmt.Sprintf("• %s\n", cat.Name)
	}

	text += "\n💸 Расходы:\n"
	for _, cat := range expenseCategories {
		text += fmt.Sprintf("• %s\n", cat.Name)
	}

	// Создаем клавиатуру для управления категориями
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить категорию дохода", "add_income_category"),
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить категорию расхода", "add_expense_category"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "action_back"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

// Добавляем новые методы для обработки доходов и расходов
func (b *Bot) handleAddExpense(message *tgbotapi.Message) {
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при получении категорий")
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
		b.sendErrorMessage(message.Chat.ID, "У вас нет категорий расходов. Сначала добавьте их через /categories")
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Выберите категорию расхода:")
	msg.ReplyMarkup = b.getCategoriesKeyboard(expenseCategories)
	b.api.Send(msg)
}

func (b *Bot) handleAddIncome(message *tgbotapi.Message) {
	categories, err := b.service.GetCategories(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при получении категорий")
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
		b.sendErrorMessage(message.Chat.ID, "У вас нет категорий доходов. Сначала добавьте их через /categories")
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Выберите категорию дохода:")
	msg.ReplyMarkup = b.getCategoriesKeyboard(incomeCategories)
	b.api.Send(msg)
}

// Добавляем новые методы для управления категориями
func (b *Bot) handleAddIncomeCategory(message *tgbotapi.Message) {
	b.states[message.From.ID] = &UserState{
		TransactionType: "income",
		AwaitingAction: "new_category",
	}
	msg := tgbotapi.NewMessage(message.Chat.ID, "Введите название новой категории дохода:")
	b.api.Send(msg)
}

func (b *Bot) handleAddExpenseCategory(message *tgbotapi.Message) {
	b.states[message.From.ID] = &UserState{
		TransactionType: "expense",
		AwaitingAction: "new_category",
	}
	msg := tgbotapi.NewMessage(message.Chat.ID, "Введите название новой категории расхода:")
	b.api.Send(msg)
}

func (b *Bot) sendErrorMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, "❌ "+text)
	b.api.Send(msg)
} 