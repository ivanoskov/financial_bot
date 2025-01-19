-- Включаем расширение для UUID
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Создание таблицы категорий
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('expense', 'income')),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Создание таблицы транзакций
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT NOT NULL,
    category_id UUID REFERENCES categories(id),
    amount DECIMAL NOT NULL,
    description TEXT,
    date TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Индексы для оптимизации запросов
CREATE INDEX idx_categories_user_id ON categories(user_id);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_category_id ON transactions(category_id);
CREATE INDEX idx_transactions_date ON transactions(date);

-- Добавление базовых категорий для тестирования
INSERT INTO categories (user_id, name, type) VALUES
    (12345, 'Продукты', 'expense'),
    (12345, 'Транспорт', 'expense'),
    (12345, 'Развлечения', 'expense'),
    (12345, 'Зарплата', 'income'); 