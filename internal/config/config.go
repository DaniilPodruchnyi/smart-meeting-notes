package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config хранит значения, которые пробрасываются в остальные модули.
// По умолчанию используются безопасные значения для локальной разработки.
type Config struct {
	HTTPAddress  string
	LogLevel     string
	SaluteSpeech SaluteSpeechConfig
}

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
			AudioEncoding:    getEnv("SALUTE_SPEECH_AUDIO_ENCODING", "MP3"),
			SampleRate:       getEnvInt("SALUTE_SPEECH_SAMPLE_RATE", 44100),
			ChannelsCount:    getEnvInt("SALUTE_SPEECH_CHANNELS_COUNT", 1),
			PollInterval:     getEnvDuration("SALUTE_SPEECH_POLL_INTERVAL", 2*time.Second),
			PollTimeout:      getEnvDuration("SALUTE_SPEECH_POLL_TIMEOUT", 2*time.Minute),
			HTTPTimeout:      getEnvDuration("SALUTE_SPEECH_HTTP_TIMEOUT", 30*time.Second),
		},
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
