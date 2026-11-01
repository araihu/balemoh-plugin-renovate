package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
)

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "request controller stopped; inspect configuration and retained request status")
		os.Exit(1)
	}
}
func run() error {
	c, err := requests.Load()
	if err != nil {
		return err
	}
	if _, err = tls.LoadX509KeyPair(c.TLSCert, c.TLSKey); err != nil {
		return err
	}
	controller, err := requests.New(c)
	if err != nil {
		return err
	}
	defer controller.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	server := &http.Server{Addr: c.Address, Handler: controller.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan struct{})
	go func() { defer close(done); controller.Run(ctx) }()
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	}()
	err = server.ListenAndServeTLS(c.TLSCert, c.TLSKey)
	cancel()
	<-done
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
