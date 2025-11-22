# Этап 1: Сборка
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копируем файлы модулей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/app

# Этап 2: Финальный образ
FROM alpine:3.18

# Устанавливаем зависимости
RUN apk --no-cache add ca-certificates postgresql-client curl

# Устанавливаем migrate CLI
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz | tar xvz && \
    mv migrate /usr/local/bin/migrate && \
    chmod +x /usr/local/bin/migrate

WORKDIR /app

# Копируем из builder
COPY --from=builder /app/app .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/scripts ./scripts

# Делаем скрипты исполняемыми
RUN chmod +x ./scripts/*.sh

EXPOSE 8080

# Запускаем миграции и приложение
CMD ["sh", "-c", "./scripts/migrate.sh && ./app"]