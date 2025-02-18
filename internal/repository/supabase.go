package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/ivanoskov/financial_bot/internal/model"
	"github.com/supabase-community/supabase-go"
)

type SupabaseRepository struct {
	client *supabase.Client
}

func NewSupabaseRepository(url, key string) (*SupabaseRepository, error) {
	client, err := supabase.NewClient(url, key, &supabase.ClientOptions{})
	if err != nil {
		return nil, err
	}

	return &SupabaseRepository{
		client: client,
	}, nil
}

func (r *SupabaseRepository) CreateCategory(ctx context.Context, category *model.Category) error {
	fmt.Printf("Creating category: %+v\n", category)
	data, count, err := r.client.From("categories").Insert(category, true, "", "", "").Execute()
	if err != nil {
		fmt.Printf("Error creating category: %v\n", err)
		return fmt.Errorf("failed to create category: %w", err)
	}
	fmt.Printf("Category created successfully. Response data: %s, count: %d\n", string(data), count)

	// Парсим ответ для получения ID
	var createdCategories []model.Category
	if err := json.Unmarshal(data, &createdCategories); err != nil {
		return fmt.Errorf("failed to parse created category: %w", err)
	}
	if len(createdCategories) > 0 {
		category.ID = createdCategories[0].ID
		category.CreatedAt = createdCategories[0].CreatedAt
	}
	return nil
}

func (r *SupabaseRepository) GetCategories(ctx context.Context, userID int64) ([]model.Category, error) {
	var categories []model.Category
	data, count, err := r.client.From("categories").
		Select("*", "", false).
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Execute()
	if err != nil {
		return nil, err
	}
	_ = count

	if err := json.Unmarshal(data, &categories); err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *SupabaseRepository) CreateTransaction(ctx context.Context, transaction *model.Transaction) error {
	fmt.Printf("Creating transaction: %+v\n", transaction)
	data, count, err := r.client.From("transactions").Insert(transaction, true, "", "", "").Execute()
	if err != nil {
		fmt.Printf("Error creating transaction: %v\n", err)
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	fmt.Printf("Transaction created successfully. Response data: %s, count: %d\n", string(data), count)

	// Парсим ответ для получения ID
	var createdTransactions []model.Transaction
	if err := json.Unmarshal(data, &createdTransactions); err != nil {
		return fmt.Errorf("failed to parse created transaction: %w", err)
	}
	if len(createdTransactions) > 0 {
		transaction.ID = createdTransactions[0].ID
		transaction.CreatedAt = createdTransactions[0].CreatedAt
	}
	return nil
}

func (r *SupabaseRepository) GetTransactions(ctx context.Context, userID int64, filter model.TransactionFilter) ([]model.Transaction, error) {
	query := r.client.From("transactions").
		Select("*", "", false).
		Eq("user_id", strconv.FormatInt(userID, 10))

	if filter.StartDate != nil {
		query = query.Gte("date", filter.StartDate.Format(time.RFC3339))
	}
	if filter.EndDate != nil {
		query = query.Lte("date", filter.EndDate.Format(time.RFC3339))
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit, "")
	}

	// Сортируем по дате транзакции, а не по дате создания
	query = query.Order("date", nil)

	data, _, err := query.Execute()
	if err != nil {
		log.Printf("Error getting transactions: %v", err)
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	// log.Printf("Got %d transactions. Response data: %s", count, string(data))

	var transactions []model.Transaction
	if err := json.Unmarshal(data, &transactions); err != nil {
		log.Printf("Error parsing transactions: %v", err)
		return nil, fmt.Errorf("failed to parse transactions: %w", err)
	}

	return transactions, nil
}

func (r *SupabaseRepository) GetTransactionsByCategory(ctx context.Context, userID int64, categoryID string) ([]model.Transaction, error) {
	var transactions []model.Transaction
	data, count, err := r.client.From("transactions").
		Select("*", "", false).
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Eq("category_id", categoryID).
		Execute()
	if err != nil {
		return nil, err
	}
	_ = count

	if err := json.Unmarshal(data, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *SupabaseRepository) DeleteTransaction(ctx context.Context, id string, userID int64) error {
	fmt.Printf("Deleting transaction %s for user %d\n", id, userID)
	data, count, err := r.client.From("transactions").
		Delete("", "").
		Eq("id", id).
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Execute()
	if err != nil {
		fmt.Printf("Error deleting transaction: %v\n", err)
		return fmt.Errorf("failed to delete transaction: %w", err)
	}
	fmt.Printf("Transaction deleted successfully. Response data: %s, count: %d\n", string(data), count)
	return nil
}

func (r *SupabaseRepository) UpdateCategory(ctx context.Context, category *model.Category) error {
	_, count, err := r.client.From("categories").
		Update(category, "", "").
		Eq("id", category.ID).
		Eq("user_id", strconv.FormatInt(category.UserID, 10)).
		Execute()
	if err != nil {
		return err
	}
	_ = count
	return nil
}

func (r *SupabaseRepository) DeleteCategory(ctx context.Context, id string, userID int64) error {
	fmt.Printf("Deleting category %s for user %d\n", id, userID)

	// Сначала удаляем все транзакции, связанные с этой категорией
	data, count, err := r.client.From("transactions").
		Delete("", "").
		Eq("category_id", id).
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Execute()
	if err != nil {
		fmt.Printf("Error deleting related transactions: %v\n", err)
		return fmt.Errorf("failed to delete related transactions: %w", err)
	}
	fmt.Printf("Deleted %d related transactions. Response data: %s\n", count, string(data))

	// Теперь удаляем саму категорию
	data, count, err = r.client.From("categories").
		Delete("", "").
		Eq("id", id).
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Execute()
	if err != nil {
		fmt.Printf("Error deleting category: %v\n", err)
		return fmt.Errorf("failed to delete category: %w", err)
	}
	fmt.Printf("Category deleted successfully. Response data: %s, count: %d\n", string(data), count)
	return nil
}

// GetAllUsers возвращает список ID всех пользователей
func (r *SupabaseRepository) GetAllUsers(ctx context.Context) ([]int64, error) {
	// Получаем уникальные user_id из таблицы transactions
	query := r.client.From("transactions").
		Select("user_id", "", false).
		Not("user_id", "is", "null")

	var data []byte
	var err error
	if data, _, err = query.Execute(); err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	// Парсим результат
	var result []struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse users: %w", err)
	}

	// Создаем map для уникальности
	usersMap := make(map[int64]bool)
	for _, r := range result {
		usersMap[r.UserID] = true
	}

	// Преобразуем map в slice
	users := make([]int64, 0, len(usersMap))
	for userID := range usersMap {
		users = append(users, userID)
	}

	return users, nil
}

// GetUserState возвращает текущее состояние пользователя
func (r *SupabaseRepository) GetUserState(ctx context.Context, userID int64) (*model.UserState, error) {
	fmt.Printf("Getting state for user %d\n", userID)
	data, count, err := r.client.From("user_states").
		Select("*", "", false).
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get user state: %w", err)
	}
	fmt.Printf("Got response data: %s, count: %d\n", string(data), count)

	var states []model.UserState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, fmt.Errorf("failed to parse user state: %w", err)
	}
	fmt.Printf("Parsed states: %+v\n", states)
	if len(states) == 0 {
		return nil, nil
	}
	return &states[0], nil
}

// SaveUserState сохраняет состояние пользователя
func (r *SupabaseRepository) SaveUserState(ctx context.Context, state *model.UserState) error {
	fmt.Printf("Saving user state: %+v\n", state)
	state.UpdatedAt = time.Now()
	data, count, err := r.client.From("user_states").
		Upsert(map[string]interface{}{
			"user_id":              state.UserID,
			"selected_category_id": state.SelectedCategory,
			"transaction_type":     state.TransactionType,
			"awaiting_action":      state.AwaitingAction,
			"updated_at":           state.UpdatedAt,
		}, "", "", "user_id").
		Execute()
	if err != nil {
		return fmt.Errorf("failed to save user state: %w", err)
	}
	fmt.Printf("User state saved successfully. Response data: %s, count: %d\n", string(data), count)
	return nil
}

// DeleteUserState удаляет состояние пользователя
func (r *SupabaseRepository) DeleteUserState(ctx context.Context, userID int64) error {
	fmt.Printf("Deleting user state for user %d\n", userID)
	data, count, err := r.client.From("user_states").
		Delete("", "").
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to delete user state: %w", err)
	}
	fmt.Printf("User state deleted successfully. Response data: %s, count: %d\n", string(data), count)
	return nil
}

// Реализация остальных методов репозитория...
