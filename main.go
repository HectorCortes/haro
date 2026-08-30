package main

import (
	"context"
	"os"

	"github.com/HectorCortes/haro/internal/cmd"
)

func main() {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	code := cmd.Execute(ctx, os.Args[1:], cwd, os.Stdout, os.Stderr)
	os.Exit(code)
}
