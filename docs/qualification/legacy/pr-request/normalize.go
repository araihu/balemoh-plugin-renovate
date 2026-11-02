// Copy into the plugin's ignored .local directory and run from that module.
// Accepts actual full-dry-run artifacts only; writer reports are never imported.
package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: normalize ARTIFACT_DIRECTORY SNAPSHOT_FILE")
	}
	root := os.Args[1]
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			panic(err)
		}
		return b
	}
	var profile struct {
		Repositories []string `json:"repositories"`
		DryRun       string   `json:"dryRun"`
	}
	if json.Unmarshal(read("profile.json"), &profile) != nil || profile.DryRun != "full" || len(profile.Repositories) != 1 || profile.Repositories[0] != "guilycst/balemoh-renovate-engine-poc-20261101" {
		panic("not an isolated full-dry-run profile")
	}
	commit := strings.TrimSpace(string(read("base-before.txt")))
	if commit != strings.TrimSpace(string(read("base-after.txt"))) {
		panic("base changed during collection")
	}
	stamp := func(name string) time.Time {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(read(name))))
		if err != nil {
			panic(err)
		}
		return t
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	id[6], id[8] = id[6]&0x0f|0x40, id[8]&0x3f|0x80
	runID := fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	m := snapshot.Metadata{
		RunID: runID, StartedAt: stamp("started-at.txt"), CompletedAt: stamp("completed-at.txt"),
		Producer:   snapshot.Producer{RenovateVersion: snapshot.RenovateVersion, RenovateImage: snapshot.RenovateImage, AdapterVersion: snapshot.AdapterVersion, PolicyHash: snapshot.ID(string(read("profile.json")))},
		Repository: snapshot.Repository{Platform: "github", Name: profile.Repositories[0], BaseBranch: "main", Commit: commit},
		DryRun:     "full", CheckoutCommit: commit, BeforeCommit: commit, AfterCommit: commit,
	}
	s, err := snapshot.Normalize(read("report.json"), m)
	if err != nil {
		panic(err)
	}
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	if _, err := snapshot.Decode(b, time.Now()); err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[2]+".next", b, 0600); err != nil {
		panic(err)
	}
	if err := os.Rename(os.Args[2]+".next", os.Args[2]); err != nil {
		panic(err)
	}
	prs := 0
	for _, d := range s.Dependencies {
		for _, c := range d.Candidates {
			if c.PR != nil {
				prs++
			}
		}
	}
	fmt.Printf("normalized groups=%d candidatesWithPR=%d\n", len(s.Dependencies), prs)
}
