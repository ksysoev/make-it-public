package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ksysoev/make-it-public/pkg/cmd"
)

var defaultServer = "loalhost:8081"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	command := cmd.InitCommand()

	err := command.ExecuteContext(ctx)
	if err != nil {
		fmt.Println(err)
		cancel()

		os.Exit(1)
	}

	cancel()
}
