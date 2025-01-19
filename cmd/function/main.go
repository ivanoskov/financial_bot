package main

import (
	"context"
	"encoding/json"
	"github.com/ivanoskov/financial_bot/internal/bot"
	"github.com/ivanoskov/financial_bot/internal/config"
	"github.com/ivanoskov/financial_bot/internal/repository"
	"github.com/ivanoskov/financial_bot/internal/service"
)

// Request структура входящего запроса от API Gateway
type Request struct {
	Body string `json:"body"`
}

// Response структура ответа для API Gateway
type Response struct {
	StatusCode int               `json:"statusCode"`
	Body       string           `json:"body"`
	Headers    map[string]string `json:"headers,omitempty"`
}

func Handler(ctx context.Context, request Request) (*Response, error) {
	// Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		return errorResponse(err)
	}

	// Инициализация репозитория
	repo, err := repository.NewSupabaseRepository(cfg.SupabaseURL, cfg.SupabaseKey)
	if err != nil {
		return errorResponse(err)
	}

	// Инициализация сервиса
	service := service.NewExpenseTracker(repo)

	// Инициализация бота
	bot, err := bot.NewBot(cfg.TelegramToken, service)
	if err != nil {
		return errorResponse(err)
	}

	// Обработка webhook-обновления
	if err := bot.HandleWebhook([]byte(request.Body)); err != nil {
		return errorResponse(err)
	}

	return &Response{
		StatusCode: 200,
		Body:       "",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func errorResponse(err error) (*Response, error) {
	return &Response{
		StatusCode: 500,
		Body:       err.Error(),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func main() {
	// Точка входа для локального тестирования
} 