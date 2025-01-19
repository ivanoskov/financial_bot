package bot

import (
	"context"
	"fmt"
	"strconv"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strings"
)

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil && update.CallbackQuery == nil {
		return
	}

	var chatID int64
	if update.Message != nil {
		chatID = update.Message.Chat.ID
	} else {
		chatID = update.CallbackQuery.Message.Chat.ID
	}

	if update.Message != nil && update.Message.IsCommand() {
		b.handleCommand(update.Message)
		return
	}

	if update.CallbackQuery != nil {
		b.handleCallback(update.CallbackQuery)
		return
	}

	if update.Message != nil {
		b.handleMessage(update.Message)
	}
}

func (b *Bot) handleCommand(message *tgbotapi.Message) {
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
}

func (b *Bot) handleStart(message *tgbotapi.Message) {
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

func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) {
	if strings.HasPrefix(callback.Data, "category_") {
		categoryID := strings.TrimPrefix(callback.Data, "category_")
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, 
			"Введите сумму и описание в формате:\n"+
			"1000 Покупка продуктов")
		b.api.Send(msg)
	}
}

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	if message.Text == "📊 Отчёты" {
		b.handleReport(message)
		return
	}

	// Обработка ввода суммы и описания
	parts := strings.SplitN(message.Text, " ", 2)
	if len(parts) != 2 {
		return
	}

	amount, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return
	}

	err = b.service.AddTransaction(context.Background(), 
		message.From.ID, 
		"category_id", // здесь нужно сохранять ID категории в состоянии пользователя
		amount,
		parts[1])

	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при сохранении транзакции")
		return
	}

	b.api.Send(tgbotapi.NewMessage(message.Chat.ID, "Транзакция сохранена! ✅"))
}

func (b *Bot) handleReport(message *tgbotapi.Message) {
	report, err := b.service.GetMonthlyReport(context.Background(), message.From.ID)
	if err != nil {
		b.sendErrorMessage(message.Chat.ID, "Ошибка при формировании отчета")
		return
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

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	b.api.Send(msg)
} 