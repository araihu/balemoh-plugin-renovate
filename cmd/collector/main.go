package main

import (
	"context"
	"fmt"
	"github.com/araihu/balemoh-plugin-renovate/internal/runner"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	c, e := runner.Load()
	if e == nil {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		ctx, deadline := context.WithTimeout(ctx, 19*time.Minute)
		defer deadline()
		e = runner.Run(ctx, c)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "collector failed; last snapshot retained")
		os.Exit(1)
	}
}
