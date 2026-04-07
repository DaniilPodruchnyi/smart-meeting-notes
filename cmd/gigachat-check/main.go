package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"smart-meeting-notes/internal/adapters/gigachat"
	"smart-meeting-notes/internal/config"
)

func main() {
	var (
		envPath = flag.String("env", ".env", "path to .env file")
		system  = flag.String("system", "", "system prompt (инструкция для модели; опционально)")
		user    = flag.String("user", "", "user message — контент или вопрос")
		prompt  = flag.String("prompt", "", "синоним -user (если -user не задан)")
		timeout = flag.Duration("timeout", 2*time.Minute, "request timeout")
	)
	flag.Parse()

	userText := *user
	if userText == "" {
		userText = *prompt
	}
	if userText == "" {
		fmt.Fprintln(os.Stderr, "error: нужен -user или -prompt (текст пользователя)")
		os.Exit(2)
	}

	cfg, err := config.Load(*envPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load error: %v\n", err)
		os.Exit(1)
	}

	client := gigachat.NewClient(cfg.GigaChat)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	started := time.Now()
	text, err := client.Chat(ctx, *system, userText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gigachat error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("OK (%s)\n", time.Since(started).Round(time.Millisecond))
	fmt.Println(text)
}
