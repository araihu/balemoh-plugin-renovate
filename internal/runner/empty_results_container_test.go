//go:build container

package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestPinnedEngineEmptyResultsAreNotNegativeProof(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	script, err := os.ReadFile("testdata/renovate-empty-results.mjs")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "-i", "--entrypoint", "node", snapshot.RenovateImage, "--input-type=module", "-")
	command.Stdin = bytes.NewReader(script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native empty-result qualification failed: %v\n%s", err, output)
	}
	_, data, ok := bytes.Cut(output, []byte("PROOF:"))
	var receipt struct {
		Passed     bool              `json:"passed"`
		Serialized bool              `json:"nativeReportSerialized"`
		Cases      []json.RawMessage `json:"cases"`
	}
	if !ok || json.Unmarshal(data, &receipt) != nil || !receipt.Passed || !receipt.Serialized || len(receipt.Cases) != 6 {
		t.Fatalf("missing qualification receipt: %s", output)
	}
}
