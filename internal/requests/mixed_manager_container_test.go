//go:build container

package requests

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestMixedManagerSelectorsMatchPinnedNativeExtraction(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	folder := t.TempDir()
	if err := os.Chmod(folder, 0755); err != nil {
		t.Fatal(err)
	}
	prefix := "apiVersion: batch/v1\nkind: Job\n"
	cases := []struct {
		Content string `json:"content"`
		Count   int    `json:"count"`
	}{
		{prefix + "imageName: nginx:1.25.3\nimage: nginx:1.25.3\n", 1},
		{prefix + "imageName: nginx:1.25.3\nscript: |\n  cat <<EOF\n    image: nginx:1.25.3\n  EOF\n", 1},
		{prefix + "imageName: nginx:1.25.3\ncontainers:\n  - image: nginx:1.25.3\ninitContainers:\n  - image: nginx:1.25.3\n", 2},
		{prefix + "imageName: nginx:1.25.3\n# image: nginx:1.25.3\n", 0},
		{prefix + "imageName: nginx:1.25.3\n'image': nginx:1.25.3\n", 0},
		{prefix + "imageName: nginx:1.25.3\ncontainer: {image: nginx:1.25.3}\n", 0},
		{prefix + "imageName: nginx:1.25.3\n  - image:\t'127.0.0.1:5500/fixture/image:1.25.3@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'\r\n", 1},
	}
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(folder, "cases.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {extractPackageFile} from '/usr/local/renovate/dist/modules/manager/kubernetes/extract.js';
const cases=JSON.parse(await readFile('/proof/cases.json','utf8'));
for(const c of cases){
 const result=extractPackageFile(c.content,'fixture.yaml',{});
 assert.equal(result?.deps.filter(d=>d.datasource==='docker').length??0,c.count);
}
`
	if err = os.WriteFile(filepath.Join(folder, "proof.mjs"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "--entrypoint", "/usr/local/renovate/node", "-v", folder+":/proof:ro", snapshot.RenovateImage, "/proof/proof.mjs")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native extractor parity: %v\n%s", err, out)
	}
}
