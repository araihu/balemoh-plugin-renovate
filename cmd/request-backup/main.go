package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/araihu/balemoh-plugin-renovate/internal/requestbackup"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
)

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "Request backup failed; inspect private diagnostics without exposing credentials")
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("image-upgrades-request-backup run|export|restore; see internal/requestbackup/environment.md")
		return nil
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if len(os.Args) < 2 {
		return fmt.Errorf("operation required")
	}
	if os.Args[1] == "run" {
		c, e := requestbackup.Load()
		if e != nil {
			return e
		}
		r, e := requestbackup.Run(ctx, c)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(r)
	}
	flags := flag.NewFlagSet("request-backup", flag.ContinueOnError)
	repository := flags.String("repository", "", "expected owner/repository")
	archive := flags.String("archive", "", "archive file, never overwritten")
	endpoint := flags.String("url", "", "controller HTTPS catalog URL")
	token := flags.String("token-file", "", "reader bearer file")
	ca := flags.String("ca-file", "", "base64 DER CA file")
	destination := flags.String("destination", "", "existing empty parent directory for restored state")
	if e := flags.Parse(os.Args[2:]); e != nil {
		return e
	}
	if *repository == "" || *archive == "" {
		return fmt.Errorf("repository and archive required")
	}
	var m requests.BackupManifest
	var e error
	switch os.Args[1] {
	case "export":
		m, e = requests.ExportRequestBackup(ctx, *endpoint, *token, *ca, *repository, *archive)
	case "restore":
		if *destination == "" {
			return fmt.Errorf("destination required")
		}
		m, e = requests.RestoreRequestBackup(*archive, *destination, *repository)
	default:
		return fmt.Errorf("unsupported operation")
	}
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"records": m.RecordCount, "repository": m.Repository, "writerEnabledAtBackup": m.ControllerWriterEnabled})
}
