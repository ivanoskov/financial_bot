package model

import "time"

// UserState представляет текущее состояние пользователя
type UserState struct {
	UserID           int64     `json:"user_id"`
	SelectedCategory string    `json:"selected_category_id"`
	TransactionType  string    `json:"transaction_type"`
	AwaitingAction   string    `json:"awaiting_action"`
	UpdatedAt        time.Time `json:"updated_at"`
}
