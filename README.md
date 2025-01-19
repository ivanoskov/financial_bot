# Telegram-бот для учета личных финансов, написанный на Go

Проект демонстрирует практическое применение чистой архитектуры, работу с Telegram Bot API и визуализацию данных. Разработан с целью тестирования использования Golang в serverless архитектуре.

## Режимы работы

Бот поддерживает два режима работы:

### 1. Long Polling Mode

Классический режим работы через long polling:
```bash
go build cmd/bot/main.go
./main
```

### 2. Serverless Mode (AWS Lambda)

Бот может работать в serverless режиме через AWS Lambda или аналогичные сервисы:

- `cmd/function/WebhookHandler` - обработка входящих сообщений через webhook
- `cmd/function/DailyReportHandler` - отправка ежедневных отчетов (триггер по расписанию)

#### Настройка Webhook

1. Разверните функцию в AWS Lambda
2. Создайте API Gateway endpoint
3. Настройте webhook в Telegram:
```bash
curl -X POST https://api.telegram.org/bot<YOUR_BOT_TOKEN>/setWebhook \
     -H "Content-Type: application/json" \
     -d '{"url": "https://your-api-gateway-url/prod/webhook"}'
```

## Архитектура

Проект построен с использованием принципов чистой архитектуры:

```
.
├── cmd/
│   ├── bot/              # Точка входа для long polling режима
│   └── function/         # AWS Lambda handlers
├── internal/
│   ├── bot/             # Telegram бот и обработка команд
│   ├── model/           # Доменные модели
│   ├── repository/      # Работа с данными (Supabase)
│   ├── service/         # Бизнес-логика
│   ├── charts/          # Генерация графиков
│   └── config/          # Конфигурация
└── migrations/          # Миграции бд
```

### Технические решения

#### 1. Управление состоянием

- Паттерн конечного автомата для управления диалогами
- In-memory хранение состояний пользователей
- Автоматический сброс состояния после завершения операций

#### 2. Работа с данными

- **База данных**: Supabase (PostgreSQL)
- **Схема данных**:
  ```sql
  -- Категории доходов/расходов
  CREATE TABLE categories (
      id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
      user_id BIGINT NOT NULL,
      name TEXT NOT NULL,
      type TEXT NOT NULL,
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
  );

  -- Финансовые операции
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

#### 3. Визуализация данных

Использует библиотеку `go-chart` для генерации графиков:

- **Типы графиков**:
  - Линейные графики (динамика доходов/расходов)
  - Круговые диаграммы (распределение по категориям)
  - Столбчатые диаграммы (сравнение периодов)

- **Оптимизации**:
  - Предварительная фильтрация данных
  - Адаптивные размеры для Telegram
  - Группировка малых категорий
  - Оптимизированные форматы изображений

#### 4. Аналитика и отчеты

- **Типы отчетов**:
  ```go
  type ReportType int

  const (
      DailyReport ReportType = iota
      WeeklyReport
      MonthlyReport
      YearlyReport
  )
  ```

- **Метрики**:
  - Основные показатели (доходы, расходы, баланс)
  - Сравнение с предыдущими периодами
  - Тренды и изменения
  - Статистика по категориям

#### 5. Обработка ошибок

- Многоуровневая валидация
- Контекстные ошибки
- Информативные сообщения пользователю
- Логирование для отладки

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
   - Информативные сообщения об ошибках
   - Поддержка частичного ввода (транзакции без описания)

4. **Масштабируемость**
   - Чистая архитектура
   - Независимые модули
   - Легкое добавление новых типов отчетов и графиков
   * Слабое звено - несколько не разделенных файлов, которые можно разделить на модули

## Развертывание

### 1. Подготовка Supabase

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

### 2. Настройка окружения

```bash
# Для обоих режимов работы
export BOT_TOKEN="your_telegram_bot_token"
export SUPABASE_URL="your_supabase_url"
export SUPABASE_KEY="your_supabase_key"
```

### 3. Запуск

#### Long Polling Mode
```bash
go build cmd/bot/main.go
./main
```

#### Serverless Mode
1. Создайте ZIP для AWS Lambda:
```bash
zip function.zip bootstrap
```

2. Загрузите ZIP в AWS Lambda и настройте триггеры:
   - API Gateway для webhook
   - EventBridge для ежедневных отчетов 