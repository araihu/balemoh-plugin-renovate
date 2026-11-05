package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

type rawDependency struct {
	Datasource string            `json:"datasource"`
	Name       string            `json:"depName"`
	Package    string            `json:"packageName"`
	Type       string            `json:"depType"`
	Current    *string           `json:"currentValue"`
	Digest     *string           `json:"currentDigest"`
	Skip       string            `json:"skipReason"`
	Warnings   []json.RawMessage `json:"warnings"`
	Updates    []rawUpdate       `json:"updates"`
}
type rawUpdate struct {
	IsVulnerabilityAlert bool    `json:"isVulnerabilityAlert"`
	Value                *string `json:"newValue"`
	Digest               *string `json:"newDigest"`
	Type                 string  `json:"updateType"`
	Pending              bool    `json:"pendingChecks"`
	Branch               string  `json:"branchName"`
}
type rawFile struct {
	File string          `json:"packageFile"`
	Deps []rawDependency `json:"deps"`
}
type rawUpgrade struct {
	Manager       string  `json:"manager"`
	DepType       string  `json:"depType"`
	Type          string  `json:"updateType"`
	CurrentDigest *string `json:"currentDigest"`
	File          string  `json:"packageFile"`
	Package       string  `json:"packageName"`
	Name          string  `json:"depName"`
	Current       *string `json:"currentValue"`
	Value         *string `json:"newValue"`
	Digest        *string `json:"newDigest"`
}

func (u rawUpgrade) tuple(branch string) string {
	name := u.Package
	if name == "" {
		name = u.Name
	}
	return ID(branch, u.File, name, u.Name, u.Current, u.CurrentDigest, u.Value, u.Digest, u.Type)
}

type rawBranch struct {
	Name     string       `json:"branchName"`
	Result   string       `json:"result"`
	PR       int          `json:"prNo"`
	Upgrades []rawUpgrade `json:"upgrades"`
}
type rawRepo struct {
	Status   string               `json:"status"`
	Problems []json.RawMessage    `json:"problems"`
	Files    map[string][]rawFile `json:"packageFiles"`
	Branches []rawBranch          `json:"branches"`
}
type rawReport struct {
	Problems     []json.RawMessage  `json:"problems"`
	Repositories map[string]rawRepo `json:"repositories"`
}

var branchResult = regexp.MustCompile(`^[a-z][a-z0-9-]{0,79}$`)

// Normalize accepts only completed full dry-runs with a verified checkout.
// Empty updates stay unknown: this report version has no universal success bit.
func Normalize(data []byte, m Metadata, diagnostics ...io.Reader) (*Snapshot, error) {
	if len(diagnostics) > 1 {
		return nil, ErrInvalid
	}
	if len(data) > 64<<20 || m.ExitCode != 0 || m.DryRun != "full" || m.CheckoutCommit == "" || m.CheckoutCommit != m.Repository.Commit || m.BeforeCommit != m.CheckoutCommit || m.AfterCommit != m.CheckoutCommit {
		return nil, ErrInvalid
	}
	var raw rawReport
	if json.Unmarshal(data, &raw) != nil || len(raw.Repositories) != 1 {
		return nil, ErrInvalid
	}
	r, ok := raw.Repositories[m.Repository.Name]
	// Activated additionally means a PR has been merged; onboarded can produce valid candidates and PRs too.
	if !ok || (r.Status != "activated" && r.Status != "onboarded") || r.Files == nil {
		return nil, ErrInvalid
	}
	var diagnosticReader io.Reader
	if len(diagnostics) == 1 {
		diagnosticReader = diagnostics[0]
	}
	problems := append(append([]json.RawMessage{}, raw.Problems...), r.Problems...)
	facts, err := diagnosticFacts(diagnosticReader, problems)
	if err != nil {
		return nil, err
	}
	s := &Snapshot{SchemaVersion: 1, RunID: m.RunID, StartedAt: m.StartedAt, CompletedAt: m.CompletedAt, Producer: m.Producer, Repository: m.Repository, Dependencies: []Dependency{}, Coverage: Coverage{Scope: "Explicit image managers; repository ignore paths and implicit chart images remain outside scope", Status: "complete", Reasons: []string{}}}
	if len(raw.Problems)+len(r.Problems) > 0 {
		s.Coverage.Reasons = append(s.Coverage.Reasons, "source_problems")
	}
	groups := map[string][]rawDependency{}
	templates := map[string]Dependency{}
	chartDefaults, buildTemplates := 0, 0
	// Branch projections omit manager/depType, so include even out-of-scope
	// Docker declarations when rejecting tuples shared by native identities.
	owners := map[string]string{}
	for manager, files := range r.Files {
		for _, f := range files {
			for _, d := range f.Deps {
				if d.Datasource != "docker" {
					continue
				}
				name := d.Package
				if name == "" {
					name = d.Name
				}
				keyBytes, _ := json.Marshal([]string{d.Name, d.Type})
				key := string(keyBytes)
				id := ID(m.Repository.Platform, m.Repository.Name, m.Repository.BaseBranch, manager, f.File, key, name)
				for _, u := range d.Updates {
					tuple := (rawUpgrade{File: f.File, Package: name, Name: d.Name, Current: d.Current, CurrentDigest: d.Digest, Value: u.Value, Digest: u.Digest, Type: u.Type}).tuple(u.Branch)
					if owner, exists := owners[tuple]; !exists {
						owners[tuple] = id
					} else if owner != id {
						owners[tuple] = ""
					}
				}
				if !imageManager(manager) {
					if !slices.Contains(s.Coverage.Reasons, "unsupported_manager") {
						s.Coverage.Reasons = append(s.Coverage.Reasons, "unsupported_manager")
					}
					continue
				}
				scopeKind := ""
				if safePath(f.File) {
					scopeKind = outsideImageScope(manager, d)
				}
				switch scopeKind {
				case "chart_defaults":
					chartDefaults++
					continue
				case "build_template":
					buildTemplates++
					continue
				}
				s.Coverage.SourceRecords++
				if !safePackage(name) || !safePath(f.File) {
					if !slices.Contains(s.Coverage.Reasons, "missing_identity") {
						s.Coverage.Reasons = append(s.Coverage.Reasons, "missing_identity")
					}
					continue
				}
				groups[id] = append(groups[id], d)
				templates[id] = Dependency{ID: id, Manager: manager, PackageFile: f.File, DependencyKey: key, PackageName: name, Candidates: []Candidate{}, Reasons: []string{}}
			}
		}
	}
	for id, rows := range groups {
		d := templates[id]
		d.SourceOccurrences = len(rows)
		base := rows[0]
		d.CurrentValue = base.Current
		// Renovate uses an empty tag for digest-only references; the wire contract uses null.
		if d.CurrentValue != nil && *d.CurrentValue == "" {
			d.CurrentValue = nil
		}
		d.CurrentDigest = base.Digest
		ambiguous := false
		for _, row := range rows[1:] {
			if !reflect.DeepEqual(row, base) {
				ambiguous = true
			}
		}
		allDisabled := !slices.ContainsFunc(rows, func(row rawDependency) bool { return row.Skip != "disabled" })
		allInherited := d.Manager == "helm-values" && !slices.ContainsFunc(rows, func(row rawDependency) bool { return !inheritedTag(row) })
		switch {
		case allDisabled:
			d.Assessment = "excluded"
			d.Reasons = append(d.Reasons, "disabled")
			if ambiguous {
				d.CurrentValue, d.CurrentDigest = nil, nil
			}
		case allInherited:
			d.Assessment = "excluded"
			d.Reasons = append(d.Reasons, "chart_inherited")
		case ambiguous:
			d.Assessment = "unknown"
			d.CurrentValue = nil
			d.CurrentDigest = nil
			d.Reasons = append(d.Reasons, "ambiguous_declarations")
		case base.Skip != "" || len(base.Warnings) > 0:
			d.Assessment = "unknown"
			d.Reasons = append(d.Reasons, "lookup_unconfirmed", warningCause(base, d.Manager, d.PackageName, facts))
		case len(base.Updates) == 0:
			d.Assessment = "unknown"
			d.Reasons = append(d.Reasons, "negative_assessment_unproven")
		default:
			d.Assessment = "updates_found"
			seen := map[string]int{}
			for _, u := range base.Updates {
				if !slices.Contains([]string{"major", "minor", "patch", "digest", "pinDigest", "pin"}, u.Type) || u.Value == nil && u.Digest == nil {
					return nil, ErrInvalid
				}
				c := Candidate{NewValue: u.Value, NewDigest: u.Digest, UpdateType: u.Type, Policy: "unknown", Workflow: "unknown", IsVulnerabilityAlert: u.IsVulnerabilityAlert}
				c.ID = ID(d.ID, c.NewValue, c.NewDigest, c.UpdateType)
				if u.Pending {
					c.Policy = "pending_checks"
				}
				if len(u.Branch) <= 256 && !strings.ContainsAny(u.Branch, "\r\n\t") {
					c.BranchName = u.Branch
				}
				key := (rawUpgrade{File: d.PackageFile, Package: d.PackageName, Name: base.Name, Current: base.Current, CurrentDigest: base.Digest, Value: u.Value, Digest: u.Digest, Type: u.Type}).tuple(u.Branch)
				matches := []rawBranch{}
				for _, b := range r.Branches {
					if b.Name != u.Branch || b.Name == "" {
						continue
					}
					for _, up := range b.Upgrades {
						if owners[key] == id && up.tuple(b.Name) == key && (up.Manager == "" || up.Manager == d.Manager) && (up.DepType == "" || up.DepType == base.Type) {
							matches = append(matches, b)
						}
					}
				}
				if len(matches) == 1 {
					b := matches[0]
					if branchResult.MatchString(b.Result) {
						c.BranchResult = b.Result
					}
					switch b.Result {
					case "needs-approval", "needs-pr-approval":
						c.Policy = "approval_required"
					case "branch-limit-reached", "pr-limit-reached":
						c.Policy = "pr_limit"
					case "commit-per-run-limit-reached", "commit-hourly-limit-reached":
						c.Policy = "rate_limited"
					case "not-scheduled":
						c.Policy = "scheduled"
					case "pending":
						c.Policy = "pending_checks"
					}
					if b.PR > 0 {
						c.Workflow = "pr_open"
						c.PR = &PR{Number: b.PR, URL: fmt.Sprintf("https://github.com/%s/pull/%d", m.Repository.Name, b.PR)}
					}
				}
				if index, ok := seen[c.ID]; ok {
					d.Candidates[index].IsVulnerabilityAlert = d.Candidates[index].IsVulnerabilityAlert || c.IsVulnerabilityAlert
					continue
				}
				seen[c.ID] = len(d.Candidates)
				d.Candidates = append(d.Candidates, c)
			}
		}
		s.Dependencies = append(s.Dependencies, d)
		switch d.Assessment {
		case "updates_found", "no_candidate":
			s.Coverage.Assessed++
		case "excluded":
			s.Coverage.Excluded++
		default:
			s.Coverage.Unknown++
		}
	}
	if chartDefaults > 0 {
		s.Coverage.Scope += fmt.Sprintf(" Empty Helm override records outside these counts: %d; their image references require chart inheritance.", chartDefaults)
	}
	if buildTemplates > 0 {
		s.Coverage.Scope += fmt.Sprintf(" Unresolved Dockerfile build-argument records outside these counts: %d.", buildTemplates)
	}
	slices.SortFunc(s.Dependencies, func(a, b Dependency) int { return strings.Compare(a.ID, b.ID) })
	s.Coverage.Extracted = len(s.Dependencies)
	if s.Coverage.Unknown > 0 {
		s.Coverage.Reasons = append(s.Coverage.Reasons, "unassessed_dependencies")
	}
	if len(s.Coverage.Reasons) > 0 {
		s.Coverage.Status = "partial"
	}
	slices.Sort(s.Coverage.Reasons)
	var encoded bytes.Buffer
	if json.NewEncoder(&encoded).Encode(s) != nil {
		return nil, ErrInvalid
	}
	return Decode(encoded.Bytes(), time.Now())
}
