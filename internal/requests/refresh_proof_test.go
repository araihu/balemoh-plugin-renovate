//go:build refreshproof

package requests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

var refreshProof = flag.Bool("refresh-proof", false, "isolated read-only App consumer")

func TestAppTokenRefreshProof(t *testing.T) {
	if !*refreshProof {
		t.Skip("explicit homelab proof only")
	}
	write := func(name, value string) string {
		path := filepath.Join("/work", name)
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal("fixture credential write failed")
		}
		return path
	}
	cfg, err := Parse(map[string]string{
		"UPGRADES_REPOSITORY": "guilycst/home-lab", "UPGRADES_CONTROLLER_WRITER_ENABLED": "false",
		"UPGRADES_CONTROLLER_DATA_DIR": "/work/state", "UPGRADES_CONTROLLER_WORK_DIR": "/work/engine",
		"UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE": "/github-read/token", "UPGRADES_CONTROLLER_PR_AUTHOR": "guilycst",
		"UPGRADES_CONTROLLER_READ_BEARER_FILE":   write("reader", strings.Repeat("r", 32)),
		"UPGRADES_CONTROLLER_ACTION_BEARER_FILE": write("action", strings.Repeat("a", 32)),
		"UPGRADES_CONTROLLER_TLS_CERT":           "/work/cert", "UPGRADES_CONTROLLER_TLS_KEY": "/work/key",
	})
	if err != nil || cfg.WriterEnabled || cfg.GitHubWriteTokenFile != "" {
		t.Fatal("disabled configuration rejected")
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal("controller startup failed")
	}
	defer c.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()
	c.execute = func(context.Context, Record) error { t.Error("executor called"); return nil }
	go c.Run(ctx)
	request := httptest.NewRequest(http.MethodPost, "/v1/requests", bytes.NewBufferString(`{}`))
	request.Header.Set("Authorization", "Bearer "+c.action)
	response := httptest.NewRecorder()
	c.Handler().ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal("writer admission enabled")
	}
	seen := map[[32]byte]bool{}
	head, err := os.ReadFile("/work/head")
	if err != nil {
		t.Fatal("qualification head missing")
	}
	successes, unavailable := 0, 0
	for {
		data, readErr := os.ReadFile(cfg.GitHubReadTokenFile)
		_, baseErr := c.github.base(ctx, cfg.Repository, "main")
		checks, checksErr := c.github.checks(ctx, cfg.Repository, strings.TrimSpace(string(head)))
		if readErr == nil && baseErr == nil && checksErr == nil {
			seen[sha256.Sum256(data)] = true
			successes++
		} else {
			unavailable++
			checks = "unknown"
		}
		receipt := map[string]any{"pid": os.Getpid(), "observedTokens": len(seen), "successfulReads": successes,
			"unavailableReads": unavailable, "checks": checks, "writerEnabled": false, "actionStatus": response.Code}
		encoded, _ := json.Marshal(receipt)
		if err := os.WriteFile("/work/consumer.json", encoded, 0600); err != nil {
			t.Fatal("receipt write failed")
		}
		fmt.Println(string(encoded))
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
