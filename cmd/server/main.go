package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"smart-meeting-notes/internal/adapters/gigachat"
	"smart-meeting-notes/internal/adapters/persistence/postgres"
	"smart-meeting-notes/internal/adapters/salutespeech"
	"smart-meeting-notes/internal/adapters/telegram"
	"smart-meeting-notes/internal/app/usecase"
	"smart-meeting-notes/internal/buildinfo"
	"smart-meeting-notes/internal/config"
	"smart-meeting-notes/internal/logger"
	"smart-meeting-notes/internal/queue"
	"smart-meeting-notes/internal/server"
)

var (
	version string
	date    string
	commit  string
)

func init() {
	buildinfo.SetBuildInfo(version, date, commit)
}

func main() {
	// Обработка сигналов завершения
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Парсинг флага конфига
	cfgFile := flag.String("c", "", "путь к JSON конфигу")
	flag.Parse()

	var cfg config.Config
	var err error

	// Загрузка конфига из файла или .env
	if *cfgFile != "" {
		cfg, err = config.LoadFromFile(*cfgFile)
		if err != nil {
			println("ошибка загрузки конфига: " + err.Error())
			os.Exit(1)
		}
	} else {
		cfg, _ = config.Load(".env")
	}

	// Вывод информации о сборке
	buildinfo.PrintBuildInfo()

	// Инициализация логгера
	lg, err := logger.New(cfg)
	if err != nil {
		panic(err)
	}
	defer func() { _ = lg.Sync() }()

	lg.Info("конфигурация загружена",
		zap.String("http_address", cfg.HTTPAddress),
		zap.String("log_level", cfg.LogLevel),
		zap.String("db_dsn", cfg.Database.DSN),
	)

	// Инициализация очереди сообщений
	q := queue.New(ctx, 4)

	// Подключение к базе данных
	var db *postgres.DB
	if cfg.Database.DSN != "" {
		db, err = postgres.New(cfg.Database.DSN)
		if err != nil {
			lg.Warn("подключение к базе данных не удалось", zap.Error(err))
		} else {
			if err := db.Migrate(); err != nil {
				lg.Warn("миграция базы данных не удалась", zap.Error(err))
			}
			lg.Info("база данных инициализирована")
		}
	} else {
		lg.Warn("DSN базы данных не задан")
	}

	// Инициализация клиента SaluteSpeech
	var saluteClient *salutespeech.Client
	if cfg.SaluteSpeech.AuthorizationKey != "" {
		saluteClient = salutespeech.NewClient(cfg.SaluteSpeech, lg)
		lg.Info("клиент salutespeech инициализирован")
	}

	// Инициализация клиента GigaChat
	var gigaClient *gigachat.Client
	if cfg.GigaChat.APIKey != "" {
		gigaClient = gigachat.New(cfg.GigaChat)
		lg.Info("клиент gigachat инициализирован")
	}

	// Инициализация Telegram бота
	var tgClient *telegram.Client
	var sender func(chatID int64, text string)

	userRepo := postgres.NewUserRepo(db)
	meetingRepo := postgres.NewMeetingRepo(db)

	telegramCfg := cfg.Telegram
	if !telegramCfg.IsZero() {
		tgClient, err = telegram.New(telegramCfg, lg, q, nil)
		if err != nil {
			lg.Warn("инициализация telegram клиента не удалась", zap.Error(err))
		} else {
			sender = func(chatID int64, text string) {
				_ = tgClient.SendToUser(chatID, text)
			}
			go func() {
				if err := tgClient.Run(ctx); err != nil {
					lg.Error("ошибка telegram бота", zap.Error(err))
				}
			}()
		}
	}

	// Создание сервиса встреч
	meetingSvc := usecase.NewMeetingService(meetingRepo, userRepo, saluteClient, gigaClient, q, lg, sender)

	if tgClient != nil {
		tgClient.SetMeetingService(meetingSvc)
	}

	// Создание HTTP сервера
	pingSvc := usecase.NewPingService()
	srv := server.New(cfg, lg, pingSvc)

	// Запуск сервера
	if err := srv.Run(ctx); err != nil {
		lg.Fatal("сервер остановлен с ошибкой", zap.Error(err))
	}
}
