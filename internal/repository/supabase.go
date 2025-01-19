package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"github.com/supabase-community/supabase-go"
	"github.com/ivanoskov/financial_bot/internal/model"
	"time"
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
	data, count, err := r.client.From("transactions").Insert(transaction, true, "", "", "").Execute()
	if err != nil {
		return err
	}
	_ = data
	_ = count
	return nil
}

func (r *SupabaseRepository) GetTransactions(ctx context.Context, userID int64, filter TransactionFilter) ([]model.Transaction, error) {
	var transactions []model.Transaction
	query := r.client.From("transactions").
		Select("*", "", false).
		Eq("user_id", strconv.FormatInt(userID, 10))

	if filter.StartDate != nil {
		query = query.Gte("date", filter.StartDate.Format(time.RFC3339))
	}
	if filter.EndDate != nil {
		query = query.Lte("date", filter.EndDate.Format(time.RFC3339))
	}

	data, count, err := query.Execute()
	if err != nil {
		return nil, err
	}
	_ = count

	if err := json.Unmarshal(data, &transactions); err != nil {
		return nil, err
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
	_, count, err := r.client.From("transactions").
		Delete("", "").
		Eq("id", id).
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Execute()
	if err != nil {
		return err
	}
	_ = count
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
	_, count, err := r.client.From("categories").
		Delete("", "").
		Eq("id", id).
		Eq("user_id", strconv.FormatInt(userID, 10)).
		Execute()
	if err != nil {
		return err
	}
	_ = count
	return nil
}

// Реализация остальных методов репозитория... 