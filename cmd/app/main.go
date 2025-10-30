package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BuzzLyutic/task-manager-api/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	// Подключаем логгер
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Загрузка конфигурации
	cfg := config.Load()

	// Подключаем БД
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL) // Создаем новое соединение к БД
	if err != nil {
		log.Fatal("Failed to connect to Database.") // Fatal потому что дальнейшая работа теряет смысл
	}
	defer pool.Close() // Запланированное закрытие соединения

	if err := pool.Ping(context.Background()); err != nil { // Пытаемся пингануть БД
		log.Fatal("Failed to ping the Database.")
	}
	logger.Info("Successfully connected to the Database!")

	r := chi.NewRouter() // Создаем роутер
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// TODO: хэндлеры

	srv := http.Server{ // Создаем сервер
		Addr: ":" + cfg.Port,
		Handler: r,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func ()  { // Запуск сервера и обработка ошибок
		logger.Info("Server started at ", zap.String("port", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed: ", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Shutdown error: ", zap.Error(err))
	}
	logger.Info("Server stopped succsessfully!")
}
