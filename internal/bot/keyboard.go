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
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 История транзакций", "action_transactions"),
		),
	)
}

// Клавиатура для управления категориями (с кнопками удаления)
func (b *Bot) getCategoriesKeyboard(categories []model.Category) tgbotapi.InlineKeyboardMarkup {
	var buttons [][]tgbotapi.InlineKeyboardButton
	
	for _, category := range categories {
		emoji := "💸"
		if category.Type == "income" {
			emoji = "💰"
		}
		// Добавляем кнопку выбора категории и кнопку удаления в одном ряду
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				emoji + " " + category.Name,
				"category_" + category.ID,
			),
			tgbotapi.NewInlineKeyboardButtonData(
				"🗑",
				"delete_category_" + category.ID,
			),
		})
	}

	// Добавляем кнопки управления категориями
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("➕ Доход", "add_income_category"),
		tgbotapi.NewInlineKeyboardButtonData("➕ Расход", "add_expense_category"),
	})

	// Добавляем кнопку "Назад"
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("« Назад", "action_back"),
	})
	
	return tgbotapi.NewInlineKeyboardMarkup(buttons...)
}

// Клавиатура для выбора категории при добавлении транзакции (без кнопок удаления)
func (b *Bot) getSelectCategoryKeyboard(categories []model.Category) tgbotapi.InlineKeyboardMarkup {
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

	// Добавляем кнопки управления
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⚙️ Управление категориями", "action_categories"),
	})

	// Добавляем кнопку "Назад"
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("« Назад", "action_back"),
	})
	
	return tgbotapi.NewInlineKeyboardMarkup(buttons...)
}