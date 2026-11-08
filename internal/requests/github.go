package requests

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/araihu/balemoh/plugin-sdk/credential"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type github struct {
	fingerprintFile         string
	client                  *http.Client
	endpoint, token, author string
	tokenFile               string
}

var sha = regexp.MustCompile(`^[a-f0-9]{40}$`)

func (g github) get(ctx context.Context, path string, out any) error {
	r, e := http.NewRequestWithContext(ctx, "GET", g.endpoint+path, nil)
	if e != nil {
		return errors.New("GitHub unavailable")
	}
	token := g.token
	if g.tokenFile != "" {
		value, err := credential.ReadBearerFile(g.tokenFile)
		if err != nil {
			return errors.New("GitHub credential unavailable")
		}
		token = value
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Accept", "application/vnd.github+json")
	r.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, e := g.client.Do(r)
	if e != nil {
		return errors.New("GitHub unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("GitHub unavailable")
	}
	d := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		return errors.New("invalid GitHub response")
	}
	return nil
}
func (g github) base(ctx context.Context, repository, branch string) (string, error) {
	var r struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	e := g.get(ctx, "/repos/"+repository+"/git/ref/heads/"+url.PathEscape(branch), &r)
	if e != nil || !sha.MatchString(r.Object.SHA) {
		return "", errors.New("base unavailable")
	}
	return r.Object.SHA, nil
}

func (g github) branchExists(ctx context.Context, r Record) (bool, error) {
	var refs []struct {
		Ref string `json:"ref"`
	}
	if err := g.get(ctx, "/repos/"+r.Repository.Name+"/git/matching-refs/heads/"+url.PathEscape(r.Branch), &refs); err != nil {
		return false, err
	}
	for _, ref := range refs {
		if ref.Ref == "refs/heads/"+r.Branch {
			return true, nil
		}
	}
	return false, nil
}

type nativePR struct {
	Number   int        `json:"number"`
	URL      string     `json:"html_url"`
	State    string     `json:"state"`
	Draft    bool       `json:"draft"`
	MergedAt *time.Time `json:"merged_at"`
	User     struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref  string `json:"ref"`
		SHA  string `json:"sha"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Base struct {
		SHA  string `json:"sha"`
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"base"`
	AutoMerge json.RawMessage `json:"auto_merge"`
}

func (g github) pull(ctx context.Context, r Record) (*PullRequest, error) {
	query := url.Values{"head": {strings.SplitN(r.Repository.Name, "/", 2)[0] + ":" + r.Branch}, "base": {r.Repository.BaseBranch}, "state": {"all"}, "per_page": {"100"}}
	var prs []nativePR
	if e := g.get(ctx, "/repos/"+r.Repository.Name+"/pulls?"+query.Encode(), &prs); e != nil {
		return nil, e
	}
	if len(prs) == 0 {
		return nil, nil
	}
	if len(prs) != 1 {
		return nil, errors.New("ambiguous PR association")
	}
	p := prs[0]
	if p.Number < 1 || p.User.Login != g.author || p.Head.Ref != r.Branch || p.Head.Repo.FullName != r.Repository.Name || p.Base.Repo.FullName != r.Repository.Name || p.Base.Ref != r.Repository.BaseBranch || !sha.MatchString(p.Head.SHA) || p.URL != "https://github.com/"+r.Repository.Name+"/pull/"+strconv.Itoa(p.Number) || len(p.AutoMerge) > 0 && string(p.AutoMerge) != "null" {
		return nil, errors.New("unexpected PR ownership or automerge")
	}
	state := "open"
	if p.Draft {
		state = "draft"
	}
	if p.State == "closed" {
		state = "closed"
	}
	if p.MergedAt != nil {
		state = "merged"
	}
	if p.State != "open" && p.State != "closed" {
		return nil, errors.New("unknown PR state")
	}
	if err := g.verifyDiff(ctx, r, p.Number, p.Head.SHA, p.Base.SHA); err != nil {
		return nil, err
	}
	checks, checksErr := g.checks(ctx, r.Repository.Name, p.Head.SHA)
	if checksErr != nil {
		checks = "unknown"
	}
	// A changed head invalidates both the verified diff and its check evidence.
	var latest nativePR
	if e := g.get(ctx, "/repos/"+r.Repository.Name+"/pulls/"+strconv.Itoa(p.Number), &latest); e != nil {
		return &PullRequest{Number: p.Number, URL: p.URL, HeadSHA: p.Head.SHA, State: state, Checks: "unknown"}, e
	}
	if latest.Head.SHA != p.Head.SHA || g.fingerprintFile != "" && latest.Base.SHA != p.Base.SHA {
		return nil, errors.New("PR head changed during verification")
	}
	return &PullRequest{Number: p.Number, URL: p.URL, HeadSHA: p.Head.SHA, State: state, Checks: checks}, checksErr
}

func (g github) verifyDiff(ctx context.Context, r Record, number int, head, base string) error {
	var files []struct {
		Name   string `json:"filename"`
		Status string `json:"status"`
	}
	if err := g.get(ctx, "/repos/"+r.Repository.Name+"/pulls/"+strconv.Itoa(number)+"/files?per_page=100", &files); err != nil {
		return err
	}
	if len(files) < 1 || len(files) > 2 {
		return errors.New("PR contains unexpected files")
	}
	imageFound, fingerprintFound := false, false
	for _, file := range files {
		switch {
		case file.Name == r.Dependency.PackageFile && !imageFound:
			imageFound = true
		case g.fingerprintFile != "" && validFingerprintPath(g.fingerprintFile) && file.Name == g.fingerprintFile && !fingerprintFound && file.Status == "modified":
			fingerprintFound = true
		default:
			return errors.New("PR contains unexpected files")
		}
	}
	if !imageFound {
		return errors.New("PR does not contain approved image file")
	}
	read := func(file, ref string) ([]byte, error) {
		var body struct {
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
		}
		if err := g.get(ctx, "/repos/"+r.Repository.Name+"/contents/"+url.PathEscape(file)+"?ref="+url.QueryEscape(ref), &body); err != nil {
			return nil, err
		}
		if body.Encoding != "base64" {
			return nil, errors.New("file is not bounded Git content")
		}
		data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(body.Content, "\n", ""))
		if err != nil || len(data) > 1<<20 {
			return nil, errors.New("file exceeds bound")
		}
		return data, nil
	}
	if fingerprintFound {
		if !sha.MatchString(base) {
			return errors.New("PR base unavailable")
		}
		before, err := read(g.fingerprintFile, base)
		if err != nil {
			return err
		}
		after, err := read(g.fingerprintFile, head)
		if err != nil {
			return err
		}
		if !fingerprintOnly(before, after) {
			return errors.New("PR fingerprint content differs from approval")
		}
		var tree struct {
			Truncated bool `json:"truncated"`
			Entries   []struct {
				Path string `json:"path"`
				Mode string `json:"mode"`
				Type string `json:"type"`
			} `json:"tree"`
		}
		if err := g.get(ctx, "/repos/"+r.Repository.Name+"/git/trees/"+head+"?recursive=1", &tree); err != nil {
			return err
		}
		regular := false
		for _, entry := range tree.Entries {
			if entry.Path == g.fingerprintFile {
				regular = entry.Mode == "100644" && entry.Type == "blob"
				break
			}
		}
		if tree.Truncated || !regular {
			return errors.New("PR fingerprint is not a bounded regular file")
		}
	}
	before, err := read(r.Dependency.PackageFile, r.Repository.Commit)
	if err != nil {
		return err
	}
	after, err := read(r.Dependency.PackageFile, head)
	if err != nil {
		return err
	}
	var identity []string
	if json.Unmarshal([]byte(r.Dependency.DependencyKey), &identity) != nil || len(identity) != 2 || !Supported(r.Dependency, r.Candidate) {
		return errors.New("invalid approval identity")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9./:_-]*$`).MatchString(identity[0]) {
		return errors.New("invalid source image")
	}
	reference := func(tag, digest *string) []byte {
		value := identity[0]
		if tag != nil {
			value += ":" + *tag
		}
		if digest != nil {
			value += "@" + *digest
		}
		return []byte(value)
	}
	old := reference(r.Dependency.CurrentValue, r.Dependency.CurrentDigest)
	next := reference(r.Candidate.NewValue, r.Candidate.NewDigest)
	for offset := 0; offset < len(before); {
		index := bytes.Index(before[offset:], old)
		if index < 0 {
			break
		}
		start := offset + index
		end := start + len(old)
		// Tag/digest prefixes and embedded repository names are not complete references.
		delimiter := func(b byte) bool { return strings.ContainsRune(" \t\r\n\"'[]=,", rune(b)) }
		if (start > 0 && !delimiter(before[start-1])) || (end < len(before) && !delimiter(before[end])) {
			return errors.New("ambiguous image reference boundary")
		}
		offset = end
	}
	count := max(r.Dependency.SourceOccurrences, 1)
	expected := bytes.ReplaceAll(before, old, next)
	if bytes.Count(before, old) != count {
		if r.Dependency.Manager != "kubernetes" {
			return errors.New("PR content differs from approval")
		}
		// Match the pinned Kubernetes manager's line-based image selector, including embedded manifests.
		selector := regexp.MustCompile(`(?m)^[ \t]*-?[ \t]*image:[ \t]*['"]?` + regexp.QuoteMeta(string(old)))
		matches := selector.FindAllIndex(before, -1)
		if len(matches) != count {
			return errors.New("PR content differs from approval")
		}
		var content bytes.Buffer
		offset := 0
		for _, match := range matches {
			content.Write(before[offset : match[1]-len(old)])
			content.Write(next)
			offset = match[1]
		}
		content.Write(before[offset:])
		expected = content.Bytes()
	}
	if !bytes.Equal(expected, after) {
		return errors.New("PR content differs from approval")
	}
	return nil
}
func (g github) checks(ctx context.Context, repository, head string) (string, error) {
	var checks struct {
		Total int `json:"total_count"`
		Runs  []struct {
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if e := g.get(ctx, "/repos/"+repository+"/commits/"+head+"/check-runs?per_page=100", &checks); e != nil {
		return "unknown", e
	}
	var status struct {
		State    string            `json:"state"`
		Total    int               `json:"total_count"`
		Statuses []json.RawMessage `json:"statuses"`
	}
	if e := g.get(ctx, "/repos/"+repository+"/commits/"+head+"/status?per_page=100", &status); e != nil {
		return "unknown", e
	}
	if checks.Total > len(checks.Runs) || status.Total > len(status.Statuses) || checks.Total+status.Total == 0 {
		return "unknown", nil
	}
	value := "passing"
	if status.Total > 0 {
		switch status.State {
		case "failure", "error":
			return "failing", nil
		case "pending":
			value = "pending"
		case "success":
		default:
			value = "unknown"
		}
	}
	for _, c := range checks.Runs {
		if c.Status != "completed" {
			if value != "unknown" {
				value = "pending"
			}
			continue
		}
		if c.Conclusion == nil {
			value = "unknown"
			continue
		}
		switch *c.Conclusion {
		case "failure", "timed_out", "cancelled", "action_required", "startup_failure":
			return "failing", nil
		case "success":
		default:
			value = "unknown"
		}
	}
	return value, nil
}
