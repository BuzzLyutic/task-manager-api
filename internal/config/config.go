package config

import "os"

type Config struct {
	Port string
	DatabaseURL string
	WorkerCount int
}

func Load() Config {
	return Config{
		Port: getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:pass@localhost:5432/taskdb?sslmode=disable"),
		WorkerCount: 3,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
