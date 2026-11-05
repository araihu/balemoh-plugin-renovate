package snapshot

import (
	"bufio"
	"encoding/json"
	"io"
	"net/url"
	"slices"
	"strings"
)

type diagnosticFact struct{ Package, Tag, Cause string }
type diagnosticRecord struct {
	Msg              string `json:"msg"`
	URL              string `json:"url"`
	HostType         string `json:"hostType"`
	RegistryHost     string `json:"registryHost"`
	DockerRepository string `json:"dockerRepository"`
	Tag              string `json:"tag"`
	Err              struct {
		Options struct {
			URL string `json:"url"`
		} `json:"options"`
		Response struct {
			StatusCode int `json:"statusCode"`
		} `json:"response"`
	} `json:"err"`
}

func packageKey(name string) string {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 1 {
		return "docker.io/library/" + name
	}
	if !strings.ContainsAny(parts[0], ".:") && parts[0] != "localhost" {
		return "docker.io/" + name
	}
	if slices.Contains([]string{"index.docker.io", "registry-1.docker.io"}, parts[0]) {
		return "docker.io/" + parts[1]
	}
	return name
}
func dockerURL(value string) (*url.URL, string, string) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Host == "" || u.Fragment != "" {
		return nil, "", ""
	}
	path := strings.TrimPrefix(u.Path, "/v2/")
	if path == u.Path {
		return u, "", ""
	}
	for _, part := range []string{"/manifests/", "/tags/list"} {
		name, tag, ok := strings.Cut(path, part)
		if ok && safePackage(u.Host+"/"+name) {
			return u, packageKey(u.Host + "/" + name), tag
		}
	}
	return u, "", ""
}
func recordFacts(r diagnosticRecord) []diagnosticFact {
	if strings.HasPrefix(r.Msg, "Blocked HTTP request to an internal host") && r.HostType == "docker" {
		_, pkg, _ := dockerURL(r.URL)
		if pkg != "" {
			return []diagnosticFact{{Package: pkg, Cause: "cause_internal_host_blocked"}}
		}
	}
	status := r.Err.Response.StatusCode
	if r.RegistryHost != "" && r.DockerRepository != "" && len(r.Tag) <= 512 {
		_, pkg, tag := dockerURL(strings.TrimRight(r.RegistryHost, "/") + "/v2/" + r.DockerRepository + "/manifests/" + url.PathEscape(r.Tag))
		if pkg != "" && status == 404 && r.Msg == "Docker Manifest is unknown" {
			return []diagnosticFact{{pkg, tag, "cause_manifest_tag_missing"}}
		}
		if pkg != "" && status == 429 {
			return []diagnosticFact{{pkg, tag, "cause_registry_rate_limited"}}
		}
	}
	// Initial Docker 401 challenges are normal. Only a failed scoped token request proves access denial.
	if (status == 401 || status == 403) && strings.HasPrefix(r.Msg, "Request failed with status code ") {
		u, _, _ := dockerURL(r.Err.Options.URL)
		if u != nil && u.Scheme == "https" && u.Host == "ghcr.io" && u.Path == "/token" {
			scope := strings.Split(u.Query().Get("scope"), ":")
			if len(scope) == 3 && scope[0] == "repository" && scope[2] == "pull" && safePackage("ghcr.io/"+scope[1]) {
				return []diagnosticFact{{Package: "ghcr.io/" + scope[1], Cause: "cause_registry_access_denied"}}
			}
		}
	}
	return nil
}
func diagnosticFacts(reader io.Reader, problems []json.RawMessage) ([]diagnosticFact, error) {
	facts := []diagnosticFact{}
	seen := map[diagnosticFact]bool{}
	add := func(record diagnosticRecord) {
		for _, f := range recordFacts(record) {
			if !seen[f] && len(facts) <= 10000 {
				seen[f] = true
				facts = append(facts, f)
			}
		}
	}
	for _, data := range problems {
		var r diagnosticRecord
		if json.Unmarshal(data, &r) == nil {
			add(r)
		}
	}
	if len(facts) > 10000 {
		return nil, ErrInvalid
	}
	if reader == nil {
		return facts, nil
	}
	limited := &io.LimitedReader{R: reader, N: 64<<20 + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var record diagnosticRecord
		if json.Unmarshal(scanner.Bytes(), &record) == nil {
			add(record)
		}
		if len(facts) > 10000 {
			return nil, ErrInvalid
		}
	}
	if scanner.Err() != nil || limited.N == 0 {
		return nil, ErrInvalid
	}
	return facts, nil
}
func warningCause(d rawDependency, manager, pkg string, facts []diagnosticFact) string {
	// Skipped declarations did not perform a lookup; do not borrow another declaration's HTTP failure.
	if d.Skip == "invalid-value" {
		if manager == "helm-values" && (d.Current == nil || *d.Current == "") && d.Digest == nil {
			return "cause_empty_helm_tag"
		}
		return "cause_invalid_value"
	}
	if d.Skip != "" {
		return "cause_lookup_skipped"
	}
	digestFailed := false
	for _, data := range d.Warnings {
		var warning struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(data, &warning) == nil && warning.Message == "Could not determine new digest for update (docker package "+pkg+")" {
			digestFailed = true
		}
	}
	tag := "latest"
	if d.Current != nil && *d.Current != "" {
		tag = *d.Current
	}
	for _, cause := range []string{"cause_registry_access_denied", "cause_registry_rate_limited", "cause_internal_host_blocked", "cause_manifest_tag_missing"} {
		if cause == "cause_manifest_tag_missing" && !digestFailed {
			continue
		}
		for _, f := range facts {
			if f.Package == packageKey(pkg) && f.Cause == cause && (f.Tag == "" || f.Tag == tag) {
				if cause == "cause_manifest_tag_missing" && tag == "latest" && (d.Current == nil || *d.Current == "") {
					return "cause_latest_tag_missing"
				}
				return cause
			}
		}
	}
	if digestFailed {
		return "cause_digest_resolution_failed"
	}
	return "cause_lookup_failed"
}
