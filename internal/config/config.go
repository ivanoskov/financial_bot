package config

import (
    "os"
    "github.com/joho/godotenv"
)

type Config struct {
    SupabaseURL    string
    SupabaseKey    string
    TelegramToken  string
}

func LoadConfig() (*Config, error) {
    if err := godotenv.Load(); err != nil {
        return nil, err
    }

    return &Config{
        SupabaseURL:    os.Getenv("SUPABASE_URL"),
        SupabaseKey:    os.Getenv("SUPABASE_KEY"),
        TelegramToken:  os.Getenv("TELEGRAM_TOKEN"),
    }, nil
} 