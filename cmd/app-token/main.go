package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/tracking"
)

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "App token operation failed; credential values omitted")
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("image-upgrades-app-token run|ready|qualify; see internal/tracking/environment.md")
		return nil
	}
	c, e := tracking.Load()
	if e != nil {
		return e
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("operation required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	switch os.Args[1] {
	case "ready":
		_, e = tracking.State(c.OutputDir, time.Now())
		return e
	case "run":
		return tracking.New(c).Run(ctx, func(event string) {
			message := map[string]any{"event": event}
			if event == "token_refreshed" {
				if metadata, err := tracking.State(c.OutputDir, time.Now()); err == nil {
					message["expiresAt"] = metadata.ExpiresAt
				}
			}
			_ = json.NewEncoder(os.Stdout).Encode(message)
		})
	case "qualify":
		flags := flag.NewFlagSet("qualify", flag.ContinueOnError)
		number := flags.Int("pull-request", 0, "PR number to verify without repository writes")
		receipt := flags.String("receipt", "", "optional qualification receipt path")
		if e = flags.Parse(os.Args[2:]); e != nil {
			return e
		}
		result, e := tracking.New(c).Qualify(ctx, *number)
		if e != nil {
			return e
		}
		data, e := json.MarshalIndent(result, "", "  ")
		if e != nil {
			return e
		}
		if *receipt != "" {
			f, e := os.OpenFile(*receipt, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return e
			}
			_, e = f.Write(append(data, '\n'))
			closed := f.Close()
			if e != nil {
				return e
			}
			if closed != nil {
				return closed
			}
		}
		_, e = os.Stdout.Write(append(data, '\n'))
		return e
	default:
		return fmt.Errorf("unsupported operation")
	}
}
