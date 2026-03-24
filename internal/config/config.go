package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config хранит значения, которые пробрасываются в остальные модули.
// По умолчанию используются безопасные значения для локальной разработки.
type Config struct {
	HTTPAddress string
	LogLevel    string
}

// Load читает параметры из .env (если файл существует) и переменных окружения.
func Load(envPath string) (Config, error) {
	if envPath == "" {
		envPath = ".env"
	}

	// В учебном/локальном режиме отсутствие .env не должно ломать старт сервера.
	if _, err := os.Stat(envPath); err == nil {
		_ = godotenv.Load(envPath)
	}

	cfg := Config{
		HTTPAddress: getEnv("HTTP_ADDRESS", ":8080"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
