package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config хранит конфигурацию приложения
type Config struct {
	HTTPAddress  string
	LogLevel     string
	TLS          TLSConfig
	SaluteSpeech SaluteSpeechConfig
	Telegram     TelegramConfig
	Database     DatabaseConfig
	GigaChat     GigaChatConfig
}

// TLSConfig общие параметры TLS для исходящих HTTP-клиентов (Telegram, SaluteSpeech, GigaChat).
type TLSConfig struct {
	// InsecureSkipVerify отключает проверку сертификата. Только dev/тесты: TLS_INSECURE_SKIP_VERIFY=true
	InsecureSkipVerify bool
}

// GigaChatConfig конфигурация GigaChat API
type GigaChatConfig struct {
	APIKey string
}

// DatabaseConfig конфигурация базы данных
type DatabaseConfig struct {
	DSN             string `json:"dsn"`
	MaxConns        int32  `json:"max_conns"`
	MinConns        int32  `json:"min_conns"`
	MaxConnLifetime string `json:"max_conn_lifetime"`
	MaxConnIdleTime string `json:"max_conn_idle_time"`
	HealthCheckTime string `json:"health_check_period"`
}

// TelegramConfig конфигурация Telegram бота
type TelegramConfig struct {
	BotToken string
}

// IsZero проверяет, что токен бота не задан
func (c TelegramConfig) IsZero() bool {
	return c.BotToken == ""
}

// SaluteSpeechConfig конфигурация SaluteSpeech API
type SaluteSpeechConfig struct {
	AuthURL          string
	AuthorizationKey string
	Scope            string
	APIBaseURL       string
	UploadPath       string
	RecognizePath    string
	StatusPath       string
	DownloadPath     string
	Model            string
	Language         string
	AudioEncoding    string
	SampleRate       int
	ChannelsCount    int
	PollInterval     time.Duration
	PollTimeout      time.Duration
	HTTPTimeout      time.Duration
}

// Load загружает конфигурацию из .env файла и переменных окружения
func Load(envPath string) (Config, error) {
	if envPath == "" {
		envPath = ".env"
	}

	// Попытка загрузить .env файл
	if _, err := os.Stat(envPath); err == nil {
		_ = godotenv.Load(envPath)
	}

	cfg := Config{
		HTTPAddress: getEnv("HTTP_ADDRESS", ":8080"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		TLS: TLSConfig{
			InsecureSkipVerify: getEnvBool("TLS_INSECURE_SKIP_VERIFY", false),
		},
		Database: DatabaseConfig{
			DSN:             getEnv("DATABASE_DSN", "postgres://smartmeet:smartmeet123@localhost:5432/smartmeet?sslmode=disable"),
			MaxConns:        int32(getEnvInt("DB_MAX_CONNS", 10)),
			MinConns:        int32(getEnvInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime: getEnv("DB_MAX_CONN_LIFETIME", "1h"),
			MaxConnIdleTime: getEnv("DB_MAX_CONN_IDLE_TIME", "30m"),
			HealthCheckTime: getEnv("DB_HEALTHCHECK_PERIOD", "1m"),
		},
		GigaChat: GigaChatConfig{
			APIKey: getEnv("GIGACHAT_API_KEY", ""),
		},
		SaluteSpeech: SaluteSpeechConfig{
			AuthURL:          getEnv("SALUTE_SPEECH_AUTH_URL", "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"),
			AuthorizationKey: getEnv("SALUTE_SPEECH_AUTHORIZATION_KEY", ""),
			Scope:            getEnv("SALUTE_SPEECH_SCOPE", "SALUTE_SPEECH_PERS"),
			APIBaseURL:       getEnv("SALUTE_SPEECH_API_BASE_URL", "https://smartspeech.sber.ru/rest/v1"),
			UploadPath:       getEnv("SALUTE_SPEECH_UPLOAD_PATH", "/data:upload"),
			RecognizePath:    getEnv("SALUTE_SPEECH_RECOGNIZE_PATH", "/speech:async_recognize"),
			StatusPath:       getEnv("SALUTE_SPEECH_STATUS_PATH", "/task:get"),
			DownloadPath:     getEnv("SALUTE_SPEECH_DOWNLOAD_PATH", "/data:download"),
			Model:            getEnv("SALUTE_SPEECH_MODEL", "general"),
			Language:         getEnv("SALUTE_SPEECH_LANGUAGE", "ru-RU"),
			AudioEncoding:    getEnv("SALUTE_SPEECH_AUDIO_ENCODING", "PCM_S16LE"),
			SampleRate:       getEnvInt("SALUTE_SPEECH_SAMPLE_RATE", 16000),
			ChannelsCount:    getEnvInt("SALUTE_SPEECH_CHANNELS_COUNT", 1),
			PollInterval:     getEnvDuration("SALUTE_SPEECH_POLL_INTERVAL", 2*time.Second),
			PollTimeout:      getEnvDuration("SALUTE_SPEECH_POLL_TIMEOUT", 2*time.Minute),
			HTTPTimeout:      getEnvDuration("SALUTE_SPEECH_HTTP_TIMEOUT", 30*time.Second),
		},
		Telegram: TelegramConfig{
			BotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		},
	}

	return cfg, nil
}

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt возвращает целочисленное значение переменной окружения
func getEnvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// getEnvDuration возвращает значение времени из переменной окружения
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// JSONConfig структура для загрузки конфига из JSON файла
type JSONConfig struct {
	HTTPAddress  string             `json:"http_address"`
	LogLevel     string             `json:"log_level"`
	TLS          *TLSConfig         `json:"tls,omitempty"`
	Database     DatabaseConfig     `json:"database"`
	GigaChat     GigaChatConfig     `json:"gigachat"`
	SaluteSpeech SaluteSpeechConfig `json:"salutespeech"`
	Telegram     TelegramConfig     `json:"telegram"`
}

// LoadFromFile загружает конфигурацию из JSON файла
func LoadFromFile(filename string) (Config, error) {
	if filename == "" {
		return Config{}, errors.New("имя файла не может быть пустым")
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, err
	}

	var jsonCfg JSONConfig
	if err := json.Unmarshal(data, &jsonCfg); err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddress: getEnv("HTTP_ADDRESS", jsonCfg.HTTPAddress),
		LogLevel:    getEnv("LOG_LEVEL", jsonCfg.LogLevel),
		Database: DatabaseConfig{
			DSN:             getEnv("DATABASE_DSN", jsonCfg.Database.DSN),
			MaxConns:        int32(getEnvInt("DB_MAX_CONNS", int(jsonCfg.Database.MaxConns))),
			MinConns:        int32(getEnvInt("DB_MIN_CONNS", int(jsonCfg.Database.MinConns))),
			MaxConnLifetime: getEnv("DB_MAX_CONN_LIFETIME", jsonCfg.Database.MaxConnLifetime),
			MaxConnIdleTime: getEnv("DB_MAX_CONN_IDLE_TIME", jsonCfg.Database.MaxConnIdleTime),
			HealthCheckTime: getEnv("DB_HEALTHCHECK_PERIOD", jsonCfg.Database.HealthCheckTime),
		},
		GigaChat:     jsonCfg.GigaChat,
		Telegram:     jsonCfg.Telegram,
		SaluteSpeech: jsonCfg.SaluteSpeech,
	}
	if jsonCfg.TLS != nil {
		cfg.TLS = *jsonCfg.TLS
	}
	cfg.TLS.InsecureSkipVerify = getEnvBool("TLS_INSECURE_SKIP_VERIFY", cfg.TLS.InsecureSkipVerify)

	if cfg.HTTPAddress == "" {
		cfg.HTTPAddress = ":8080"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = "postgres://smartmeet:smartmeet123@localhost:5432/smartmeet?sslmode=disable"
	}
	if cfg.Database.MaxConns == 0 {
		cfg.Database.MaxConns = 10
	}
	if cfg.Database.MinConns == 0 {
		cfg.Database.MinConns = 2
	}
	if cfg.Database.MaxConnLifetime == "" {
		cfg.Database.MaxConnLifetime = "1h"
	}
	if cfg.Database.MaxConnIdleTime == "" {
		cfg.Database.MaxConnIdleTime = "30m"
	}
	if cfg.Database.HealthCheckTime == "" {
		cfg.Database.HealthCheckTime = "1m"
	}

	return cfg, nil
}
