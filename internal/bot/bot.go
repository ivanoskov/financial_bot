package bot

import (
	"log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"your-module/internal/service"
)

type Bot struct {
	api     *tgbotapi.BotAPI
	service *service.ExpenseTracker
}

func NewBot(token string, service *service.ExpenseTracker) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:     bot,
		service: service,
	}, nil
}

func (b *Bot) Start() error {
	log.Printf("Authorized on account %s", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		go b.handleUpdate(update)
	}

	return nil
} 