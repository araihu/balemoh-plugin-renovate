package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/araihu/balemoh-plugin-renovate/internal/artifact"
	"github.com/araihu/balemoh/plugin-sdk/credential"
	"github.com/caarlos0/env/v11"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:generate go tool envdoc -output environment.md -format markdown
type Config struct {
	// TLS listening address. No plaintext listener exists.
	Address string `env:"UPGRADES_LISTEN" envDefault:":8443"`
	// Read-only normalized snapshot file.
	SnapshotFile string `env:"UPGRADES_SNAPSHOT_FILE" envDefault:"/snapshots/snapshot.json"`
	// File containing the snapshot reader bearer token, at least 32 characters.
	TokenFile string `env:"UPGRADES_READER_TOKEN_FILE,required,notEmpty"`
	// Server certificate chain.
	Certificate string `env:"UPGRADES_TLS_CERT,required,notEmpty"`
	// Server private key.
	Key string `env:"UPGRADES_TLS_KEY,required,notEmpty"`
}

func run() error {
	c, e := env.ParseAs[Config]()
	if e != nil {
		return errors.New("invalid server configuration")
	}
	data, e := os.ReadFile(c.TokenFile)
	if e != nil {
		return e
	}
	token := strings.TrimSpace(string(data))
	if len(token) < 32 || !credential.ValidBearer(token) {
		return errors.New("invalid reader credential")
	}
	server := &http.Server{Addr: c.Address, Handler: artifact.Handler(c.SnapshotFile, token), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		stop, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = server.Shutdown(stop)
	}()
	e = server.ListenAndServeTLS(c.Certificate, c.Key)
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "snapshot server failed")
		os.Exit(1)
	}
}
