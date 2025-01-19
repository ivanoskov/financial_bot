package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ivanoskov/financial_bot/internal/model"
)

func (b *Bot) getMainKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Добавить доход", "action_add_income"),
			tgbotapi.NewInlineKeyboardButtonData("💸 Добавить расход", "action_add_expense"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Отчёты", "action_report"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Категории", "action_categories"),
		),
	)
}

func (b *Bot) getCategoriesKeyboard(categories []model.Category) tgbotapi.InlineKeyboardMarkup {
	var buttons [][]tgbotapi.InlineKeyboardButton
	
	for _, category := range categories {
		emoji := "💸"
		if category.Type == "income" {
			emoji = "💰"
		}
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				emoji + " " + category.Name,
				"category_" + category.ID,
			),
		})
	}

	// Добавляем кнопку "Назад"
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "action_back"),
	})
	
	return tgbotapi.NewInlineKeyboardMarkup(buttons...)
} 