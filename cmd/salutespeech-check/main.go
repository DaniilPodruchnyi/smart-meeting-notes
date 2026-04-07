package main

import (
	"context"
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"time"

	"smart-meeting-notes/internal/adapters/salutespeech"
	"smart-meeting-notes/internal/config"
)

func main() {
	var (
		envPath     = flag.String("env", ".env", "path to .env file")
		audioPath   = flag.String("audio", "", "path to audio file for transcription")
		contentType = flag.String("content-type", "", "content type for upload, auto by extension if empty")
		timeout     = flag.Duration("timeout", 5*time.Minute, "global timeout for one transcription run")
	)
	flag.Parse()

	if *audioPath == "" {
		fmt.Fprintln(os.Stderr, "error: -audio is required")
		os.Exit(2)
	}

	cfg, err := config.Load(*envPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load error: %v\n", err)
		os.Exit(1)
	}

	audio, err := os.ReadFile(*audioPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read audio error: %v\n", err)
		os.Exit(1)
	}

	ct := *contentType
	if ct == "" {
		ct = mime.TypeByExtension(filepath.Ext(*audioPath))
	}
	if ct == "" {
		ct = "application/octet-stream"
	}

	client := salutespeech.NewClient(cfg.SaluteSpeech, nil)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	startedAt := time.Now()
	text, err := client.Transcribe(ctx, audio, ct)
	if err != nil {
		fmt.Fprintf(os.Stderr, "transcribe error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Transcription success (%s)\n", time.Since(startedAt).Round(time.Millisecond))
	fmt.Println("----- TEXT START -----")
	fmt.Println(text)
	fmt.Println("----- TEXT END -----")
}
