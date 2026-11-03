package requests

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
)

var previewFile = flag.String("request-preview", "", "private output file for an isolated controller preview")

// Explicit test fixture. No production flag enables this executor or GitHub fixture.
func TestControlledRequestPreview(t *testing.T) {
	if *previewFile == "" {
		t.Skip("opt-in local preview")
	}
	c, s := setup(t)
	r, e := c.Submit(t.Context(), s)
	if e != nil {
		t.Fatal(e)
	}
	c.store.mu.Lock()
	delete(c.store.records, r.ID)
	e = os.Remove(filepath.Join(c.config.DataDir, r.ID+".json"))
	c.store.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	_, present := githubFixture(t, c, r)
	var effects atomic.Int32
	c.execute = func(context.Context, Record) error { effects.Add(1); *present = true; return nil }
	server := httptest.NewTLSServer(c.Handler())
	defer server.Close()
	dir := filepath.Dir(*previewFile)
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		return p
	}
	cert, e := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	if e != nil {
		t.Fatal(e)
	}
	ca := write("preview-ca", []byte(base64.StdEncoding.EncodeToString(cert.Raw)))
	output := map[string]any{"snapshotURL": server.URL + "/snapshot.json", "controllerURL": server.URL, "repository": "test/repo", "snapshotToken": write("preview-reader", []byte(c.reader)), "actionToken": write("preview-action", []byte(c.action)), "ca": ca, "dependencyId": s.DependencyID, "candidateId": s.CandidateID, "store": c.config.DataDir}
	b, _ := json.Marshal(output)
	if e = os.WriteFile(*previewFile, b, 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); c.Run(ctx) }()
	<-ctx.Done()
	<-done
	t.Logf("controlled executions=%d", effects.Load())
}
