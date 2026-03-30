# SsuBench

REST API сервис для размещения заданий и взаимодействия между заказчиками и исполнителями с использованием внутренней системы баллов.

## Описание

Пользователи делятся на три роли:
- `customer` — создаёт задания
- `executor` — выполняет задания
- `admin` — управляет пользователями

Оплата за выполнение происходит через внутренние баллы: после подтверждения задания баллы списываются у заказчика и начисляются исполнителю.

---

## Стек

- Go
- PostgreSQL
- Docker
- Chi
- pgx
- JWT
- bcrypt

---

## Быстрый старт

### 1. Настройка окружения

```bash
cp .env.example .env
```

### 2. Запуск базы данных

```bash
docker compose up -d
```

### 3. Применение миграций

```bash
migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/ssubench?sslmode=disable" up
```

### 4. Запуск сервера

```bash
go run ./cmd/api
```

Сервис будет доступен по адресу:
http://localhost:8080

---

## Как работает сервис

1. Заказчик создаёт задачу  
2. Публикует её  
3. Исполнители откликаются  
4. Заказчик выбирает исполнителя  
5. Исполнитель выполняет задачу  
6. Заказчик подтверждает → перевод баллов  

---

## Основные эндпоинты

### Auth
- POST /auth/register
- POST /auth/login

### Tasks
- POST /tasks
- PATCH /tasks/{id}/publish
- PATCH /tasks/{id}/complete
- PATCH /tasks/{id}/confirm

### Bids
- POST /tasks/{id}/bids
- PATCH /tasks/{id}/bids/{bid_id}/accept

---

## Примеры curl

### Регистрация

```bash
curl -X POST http://localhost:8080/auth/register   -H "Content-Type: application/json"   -d '{"email":"user@example.com","password":"123456","role":"customer"}'
```

### Логин

```bash
curl -X POST http://localhost:8080/auth/login   -H "Content-Type: application/json"   -d '{"email":"user@example.com","password":"123456"}'
```

### Создание задачи

```bash
curl -X POST http://localhost:8080/tasks   -H "Authorization: Bearer <TOKEN>"   -H "Content-Type: application/json"   -d '{"title":"Task","description":"Some task","reward":100}'
```

### Публикация задачи

```bash
curl -X PATCH http://localhost:8080/tasks/<TASK_ID>/publish   -H "Authorization: Bearer <TOKEN>"
```

### Отклик на задачу

```bash
curl -X POST http://localhost:8080/tasks/<TASK_ID>/bids   -H "Authorization: Bearer <TOKEN>"
```

### Принятие отклика

```bash
curl -X PATCH http://localhost:8080/tasks/<TASK_ID>/bids/<BID_ID>/accept   -H "Authorization: Bearer <TOKEN>"
```

### Завершение задачи

```bash
curl -X PATCH http://localhost:8080/tasks/<TASK_ID>/complete   -H "Authorization: Bearer <TOKEN>"
```

### Подтверждение выполнения

```bash
curl -X PATCH http://localhost:8080/tasks/<TASK_ID>/confirm   -H "Authorization: Bearer <TOKEN>"
```

---

## Тесты

```bash
go test ./...
```

---

## Структура

```
cmd/api/
internal/
migrations/
docs/
```

---

## Документация

docs/openapi.yaml
