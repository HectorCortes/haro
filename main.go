package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/HectorCortes/haro/internal/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	code := cmd.Execute(ctx, os.Args[1:], cwd, os.Stdout, os.Stderr)
	os.Exit(code)
}
