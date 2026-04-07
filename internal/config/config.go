package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config хранит конфигурацию приложения
type Config struct {
	HTTPAddress  string
	LogLevel     string
	SaluteSpeech SaluteSpeechConfig
	Telegram     TelegramConfig
	Database     DatabaseConfig
	GigaChat     GigaChatConfig
}

// GigaChatConfig конфигурация GigaChat API
type GigaChatConfig struct {
	APIKey string
}

// DatabaseConfig конфигурация базы данных
type DatabaseConfig struct {
	DSN string
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
		Database: DatabaseConfig{
			DSN: getEnv("DATABASE_DSN", "postgres://smartmeet:smartmeet123@localhost:5432/smartmeet?sslmode=disable"),
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
		HTTPAddress:  getEnv("HTTP_ADDRESS", jsonCfg.HTTPAddress),
		LogLevel:     getEnv("LOG_LEVEL", jsonCfg.LogLevel),
		Database:     jsonCfg.Database,
		GigaChat:     jsonCfg.GigaChat,
		Telegram:     jsonCfg.Telegram,
		SaluteSpeech: jsonCfg.SaluteSpeech,
	}

	if cfg.HTTPAddress == "" {
		cfg.HTTPAddress = ":8080"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	return cfg, nil
}
