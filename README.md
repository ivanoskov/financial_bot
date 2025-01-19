# Financial Bot

Telegram-бот для учета личных финансов, написанный на Go. Проект демонстрирует практическое применение чистой архитектуры, работу с Telegram Bot API и визуализацию данных.

## Архитектура

Проект следует принципам чистой архитектуры с четким разделением на слои:

```
.
├── cmd/
│   └── bot/            # Точка входа приложения
├── internal/
│   ├── bot/           # Обработка Telegram-взаимодействий
│   ├── model/         # Доменные модели
│   ├── repository/    # Слой данных (Supabase)
│   ├── service/       # Бизнес-логика
│   └── charts/        # Генерация графиков
```

### Ключевые особенности реализации

#### 1. Управление состоянием

Бот использует паттерн конечного автомата для управления диалогами:

```go
type UserState struct {
    SelectedCategoryID string
    TransactionType    string    // "income" или "expense"
    AwaitingAction    string    // "new_category" или пусто
}
```

#### 2. Работа с данными

- **База данных**: Supabase (PostgreSQL)
- **Структура таблиц**:
  - `categories`: Категории доходов/расходов
  - `transactions`: Финансовые операции

Пример работы с Supabase:
```go
func (r *SupabaseRepository) GetTransactions(ctx context.Context, userID int64, filter model.TransactionFilter) ([]model.Transaction, error) {
    query := r.client.From("transactions").
        Select("*", "", false).
        Eq("user_id", strconv.FormatInt(userID, 10))
    
    // Применение фильтров и сортировка
    if filter.StartDate != nil {
        query = query.Gte("date", filter.StartDate)
    }
    query = query.Order("created_at", nil)
    
    // In-memory сортировка для корректного отображения
    sort.Slice(transactions, func(i, j int) bool { 
        return transactions[i].Date.After(transactions[j].Date) 
    })
}
```

#### 3. Визуализация данных

Использует библиотеку `go-chart` для генерации графиков:

- Динамика доходов/расходов (линейный график)
- Распределение по категориям (круговые диаграммы)
- Тренды изменений (линейный график с процентами)
- Сравнение периодов (столбчатая диаграмма)

Особенности:
- Автоматическое масштабирование
- Поддержка легенд и подписей
- Оптимизированные размеры для Telegram
- Фильтрация незначительных категорий (<1%)

#### 4. Аналитика и отчеты

Реализованы различные типы отчетов:
```go
type ReportType int

const (
    DailyReport ReportType = iota
    WeeklyReport
    MonthlyReport
    YearlyReport
)
```

Каждый отчет включает:
- Основные показатели (доходы, расходы, баланс)
- Сравнение с предыдущим периодом
- Статистику по категориям
- Тренды и изменения

#### 5. Обработка ошибок

Реализована многоуровневая обработка ошибок:
- Валидация на уровне бота
- Бизнес-логика в сервисном слое
- Обработка ошибок БД
- Информативные сообщения пользователю

### Используемые библиотеки

- `github.com/go-telegram-bot-api/telegram-bot-api/v5` - Telegram Bot API
- `github.com/wcharczuk/go-chart/v2` - Генерация графиков
- `github.com/nedpals/supabase-go` - Работа с Supabase

### Особенности и оригинальные решения

1. **Умная агрегация данных**
   - Автоматическое определение периодов
   - Расчет трендов и изменений
   - Выявление значимых изменений

2. **Оптимизация графиков**
   - Предварительная фильтрация данных
   - Адаптивные размеры
   - Оптимизированные форматы для Telegram

3. **UX-решения**
   - Интуитивная навигация
   - Информативные сообщения об ошибках
   - Поддержка частичного ввода (транзакции без описания)

4. **Масштабируемость**
   - Чистая архитектура
   - Независимые модули
   - Легкое добавление новых типов отчетов и графиков

## Развертывание

1. Создайте проект в Supabase и настройте таблицы:
```sql
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT NOT NULL,
    category_id UUID REFERENCES categories(id),
    amount DECIMAL NOT NULL,
    description TEXT,
    date TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

2. Настройте переменные окружения:
```bash
export BOT_TOKEN="your_telegram_bot_token"
export SUPABASE_URL="your_supabase_url"
export SUPABASE_KEY="your_supabase_key"
```

3. Запустите бота:
```bash
go build cmd/bot/main.go
./main
``` 