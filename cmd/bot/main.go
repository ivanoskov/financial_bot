package main

import (
	"log"
	"your-module/internal/bot"
	"your-module/internal/config"
	"your-module/internal/service"
	"your-module/internal/repository"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	repo, err := repository.NewSupabaseRepository(cfg.SupabaseURL, cfg.SupabaseKey)
	if err != nil {
		log.Fatal(err)
	}

	service := service.NewExpenseTracker(repo)
	
	bot, err := bot.NewBot(cfg.TelegramToken, service)
	if err != nil {
		log.Fatal(err)
	}

	if err := bot.Start(); err != nil {
		log.Fatal(err)
	}
} 