package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) getMainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboardMarkup(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("�� Добавить доход"),
			tgbotapi.NewKeyboardButton("💸 Добавить расход"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Отчёты"),
			tgbotapi.NewKeyboardButton("📋 Категории"),
		),
	)
}

func (b *Bot) getCategoriesKeyboard(categories []model.Category) tgbotapi.InlineKeyboardMarkup {
	var buttons [][]tgbotapi.InlineKeyboardButton
	
	for _, category := range categories {
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				category.Name,
				"category_"+category.ID,
			),
		})
	}
	
	return tgbotapi.NewInlineKeyboardMarkup(buttons...)
} 