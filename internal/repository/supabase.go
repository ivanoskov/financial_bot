package repository

import (
	"context"
	"github.com/supabase-community/supabase-go"
	"your-module/internal/model"
)

type SupabaseRepository struct {
	client *supabase.Client
}

func NewSupabaseRepository(url, key string) (*SupabaseRepository, error) {
	client, err := supabase.NewClient(url, key)
	if err != nil {
		return nil, err
	}
	
	return &SupabaseRepository{
		client: client,
	}, nil
}

func (r *SupabaseRepository) CreateCategory(ctx context.Context, category *model.Category) error {
	_, err := r.client.From("categories").Insert(category).Execute()
	return err
}

func (r *SupabaseRepository) GetCategories(ctx context.Context, userID string) ([]model.Category, error) {
	var categories []model.Category
	err := r.client.From("categories").
		Select("*").
		Eq("user_id", userID).
		Execute(&categories)
	return categories, err
}

func (r *SupabaseRepository) CreateTransaction(ctx context.Context, transaction *model.Transaction) error {
	_, err := r.client.From("transactions").Insert(transaction).Execute()
	return err
}

func (r *SupabaseRepository) GetTransactions(ctx context.Context, userID string, filter TransactionFilter) ([]model.Transaction, error) {
	var transactions []model.Transaction
	query := r.client.From("transactions").
		Select("*").
		Eq("user_id", userID)

	if filter.StartDate != nil {
		query = query.Gte("date", filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Lte("date", filter.EndDate)
	}

	err := query.Execute(&transactions)
	return transactions, err
}

func (r *SupabaseRepository) GetTransactionsByCategory(ctx context.Context, userID string, categoryID string) ([]model.Transaction, error) {
	var transactions []model.Transaction
	err := r.client.From("transactions").
		Select("*").
		Eq("user_id", userID).
		Eq("category_id", categoryID).
		Execute(&transactions)
	return transactions, err
}

func (r *SupabaseRepository) DeleteTransaction(ctx context.Context, id string, userID string) error {
	_, err := r.client.From("transactions").
		Delete().
		Eq("id", id).
		Eq("user_id", userID).
		Execute()
	return err
}

// Реализация остальных методов репозитория... 