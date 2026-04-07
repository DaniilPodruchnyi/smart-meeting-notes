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
		envPath = flag.String("env", ".env", "path to .env")
		text    = flag.String("text", "", "text to embed")
		timeout = flag.Duration("timeout", 60*time.Second, "timeout")
	)
	flag.Parse()

	if *text == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/embeddings-check -text \"...\"")
		os.Exit(2)
	}

	cfg, err := config.Load(*envPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	client := gigachat.NewClient(cfg.GigaChat)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	vecs, err := client.Embed(ctx, []string{*text})
	if err != nil {
		fmt.Fprintf(os.Stderr, "embed: %v\n", err)
		os.Exit(1)
	}
	if len(vecs) == 0 || vecs[0] == nil {
		fmt.Fprintln(os.Stderr, "empty vector")
		os.Exit(1)
	}
	v := vecs[0]
	fmt.Printf("dim=%d first5=", len(v))
	for i := 0; i < len(v) && i < 5; i++ {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%.6f", v[i])
	}
	fmt.Println()
}
