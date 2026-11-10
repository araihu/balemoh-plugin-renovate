// Package requests owns durable, explicitly approved Renovate execution requests.
package requests

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

type Selection struct {
	DependencyID   string `json:"dependencyId"`
	CandidateID    string `json:"candidateId"`
	SnapshotDigest string `json:"snapshotDigest"`
}

type PullRequest struct {
	Number  int    `json:"number"`
	URL     string `json:"url"`
	HeadSHA string `json:"headSha"`
	State   string `json:"state"`
	Checks  string `json:"checks"`
}

type Record struct {
	SnapshotDigest string              `json:"snapshotDigest"`
	ID             string              `json:"id"`
	Repository     snapshot.Repository `json:"repository"`
	Dependency     snapshot.Dependency `json:"dependency"`
	Candidate      snapshot.Candidate  `json:"candidate"`
	Branch         string              `json:"branch"`
	State          string              `json:"state"`
	Reason         string              `json:"reason,omitempty"`
	CreatedAt      time.Time           `json:"createdAt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
	TrackedAt      *time.Time          `json:"trackedAt,omitempty"`
	TrackingError  bool                `json:"trackingError"`
	PR             *PullRequest        `json:"pr,omitempty"`
}

type Catalog struct {
	GeneratedFingerprintFile string   `json:"generatedFingerprintFile,omitempty"`
	Enabled                  bool     `json:"enabled"`
	Requests                 []Record `json:"requests"`
}

func Digest(s *snapshot.Snapshot) string {
	data, _ := json.Marshal(s)
	// The adapter's canonical ID also keeps input representation unambiguous.
	return snapshot.ID(string(data))
}

func Supported(d snapshot.Dependency, c snapshot.Candidate) bool {
	if (d.Manager != "kubernetes" && d.Manager != "docker-compose" && d.Manager != "dockerfile") ||
		strings.ContainsAny(d.PackageFile, "*?[]{}!\\") || !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9./:_-]*$`).MatchString(d.PackageName) ||
		d.Assessment != "updates_found" || c.Policy == "excluded" || c.Policy == "ignored" {
		return false
	}
	tag := regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	tagged := d.CurrentValue != nil && c.NewValue != nil && tag.MatchString(*d.CurrentValue) && tag.MatchString(*c.NewValue)
	tagUpgrade := c.UpdateType == "major" || c.UpdateType == "minor" || c.UpdateType == "patch"
	if d.CurrentDigest == nil && c.NewDigest == nil {
		return tagged && tagUpgrade
	}
	digest := regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	if d.CurrentDigest == nil || c.NewDigest == nil || !digest.MatchString(*d.CurrentDigest) || !digest.MatchString(*c.NewDigest) {
		return false
	}
	if c.UpdateType == "digest" {
		return *d.CurrentDigest != *c.NewDigest && ((tagged && *d.CurrentValue == *c.NewValue) || (d.CurrentValue == nil && c.NewValue == nil))
	}
	return tagged && tagUpgrade && *d.CurrentValue != *c.NewValue
}

func (r Record) Validate() error {
	states := map[string]bool{"queued": true, "running": true, "awaiting_pr": true, "open": true, "draft": true, "closed": true, "merged": true, "blocked": true, "needs_attention": true}
	if !identifier.MatchString(r.SnapshotDigest) || !states[r.State] || !Supported(r.Dependency, r.Candidate) || !identifier.MatchString(r.ID) || r.ID != snapshot.ID(r.Repository, r.Dependency.ID, r.Candidate.ID) || r.Branch != "balemoh-renovate/"+r.ID || !sha.MatchString(r.Repository.Commit) || r.Repository.Platform != "github" || r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || r.UpdatedAt.After(time.Now().Add(5*time.Minute)) {
		return errors.New("invalid request record")
	}
	d := r.Dependency
	if d.ID != snapshot.ID(r.Repository.Platform, r.Repository.Name, r.Repository.BaseBranch, d.Manager, d.PackageFile, d.DependencyKey, d.PackageName) || r.Candidate.ID != snapshot.ID(d.ID, r.Candidate.NewValue, r.Candidate.NewDigest, r.Candidate.UpdateType) {
		return errors.New("invalid request identity")
	}
	if r.PR != nil && (!sha.MatchString(r.PR.HeadSHA) || r.PR.Number < 1 || r.PR.URL != fmt.Sprintf("https://github.com/%s/pull/%d", r.Repository.Name, r.PR.Number) || !regexp.MustCompile(`^(open|draft|closed|merged)$`).MatchString(r.PR.State) || !regexp.MustCompile(`^(passing|pending|failing|unknown)$`).MatchString(r.PR.Checks)) {
		return errors.New("invalid PR association")
	}
	return nil
}
