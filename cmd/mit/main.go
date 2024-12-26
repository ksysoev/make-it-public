package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/ksysoev/make-it-public/pkg/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err := cmd.RunClientCommand(ctx)
	if err != nil {
		panic(err)
	}
}
