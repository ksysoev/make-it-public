package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ksysoev/make-it-public/pkg/cmd/server"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	cmd := server.InitCommand()

	err := cmd.ExecuteContext(ctx)
	if err != nil {
		fmt.Println(err)
		cancel()

		os.Exit(1)
	}

	cancel()
}
