package main

import (
	"context"
	"fmt"

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
	Body       string            `json:"body"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// WebhookHandler обрабатывает входящие обновления от Telegram
func WebhookHandler(ctx context.Context, request Request) (*Response, error) {
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

// DailyReportHandler отправляет ежедневные отчеты всем пользователям
func DailyReportHandler(ctx context.Context, request Request) (*Response, error) {
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
	expenseTracker := service.NewExpenseTracker(repo)

	// Инициализация бота
	bot, err := bot.NewBot(cfg.TelegramToken, expenseTracker)
	if err != nil {
		return errorResponse(err)
	}

	// Получаем список всех пользователей
	users, err := repo.GetAllUsers(ctx)
	if err != nil {
		return errorResponse(err)
	}

	// Отправляем отчеты каждому пользователю
	for _, userID := range users {
		// Получаем отчет за день
		report, err := expenseTracker.GetReport(ctx, userID, service.DailyReport)
		if err != nil {
			continue // Пропускаем пользователя в случае ошибки
		}

		// Отправляем отчет
		bot.SendDailyReport(ctx, userID, report)
	}

	return &Response{
		StatusCode: 200,
		Body:       fmt.Sprintf("Daily reports sent to %d users", len(users)),
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
