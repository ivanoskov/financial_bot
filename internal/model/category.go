package model

import "time"

type Category struct {
    ID          string    `json:"id,omitempty"`
    UserID      int64     `json:"user_id"`
    Name        string    `json:"name"`
    Type        string    `json:"type"` // expense или income
    CreatedAt   time.Time `json:"created_at,omitempty"`
} 