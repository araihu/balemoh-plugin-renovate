package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/credential"
)

type Controller struct {
	wake           chan struct{}
	config         Config
	store          *store
	github         github
	reader, action string
	execute        func(context.Context, Record) error
}

func New(c Config) (*Controller, error) {
	reader, e := credential.ReadBearerFile(c.ReadBearerFile)
	if e != nil {
		return nil, errors.New("invalid controller reader credential")
	}
	action, e := credential.ReadBearerFile(c.ActionBearerFile)
	if e != nil || reader == action {
		return nil, errors.New("invalid controller action credential")
	}
	readToken, e := credential.ReadBearerFile(c.GitHubReadTokenFile)
	if e != nil {
		return nil, errors.New("invalid controller GitHub credential")
	}
	if c.WriterEnabled {
		if _, err := credential.ReadBearerFile(c.GitHubWriteTokenFile); err != nil {
			return nil, errors.New("invalid controller writer credential")
		}
	}
	s, e := openStore(c.DataDir)
	if e != nil {
		return nil, e
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	controller := &Controller{wake: make(chan struct{}, 1), config: c, store: s, reader: reader, action: action, github: github{fingerprintFile: c.GeneratedFingerprintFile, endpoint: "https://api.github.com", token: readToken, tokenFile: c.GitHubReadTokenFile, author: c.PRAuthor, client: &http.Client{Transport: t, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}}
	for _, r := range s.records {
		if r.Repository.Name != c.Repository || r.Repository.BaseBranch != c.BaseBranch {
			s.close()
			return nil, errors.New("request storage belongs to another repository")
		}
	}
	controller.execute = controller.runEngine
	return controller, nil
}
func (c *Controller) Close() { c.store.close(); c.github.client.CloseIdleConnections() }

func (c *Controller) Submit(ctx context.Context, selection Selection) (Record, error) {
	if !c.config.WriterEnabled {
		return Record{}, backend.Fail("permission_denied")
	}
	if !identifier.MatchString(selection.DependencyID) || !identifier.MatchString(selection.CandidateID) || !identifier.MatchString(selection.SnapshotDigest) {
		return Record{}, backend.Fail("invalid_argument")
	}
	data, e := readBounded(c.config.SnapshotFile, snapshot.MaxBytes)
	if e != nil {
		return Record{}, backend.Fail("unavailable")
	}
	s, e := snapshot.Decode(data, time.Now())
	if e != nil || s.Repository.Name != c.config.Repository || s.Repository.BaseBranch != c.config.BaseBranch {
		return Record{}, backend.Fail("unavailable")
	}
	if Digest(s) != selection.SnapshotDigest || time.Since(s.CompletedAt) > time.Duration(c.config.MaxSnapshotAgeSeconds)*time.Second {
		return Record{}, backend.Fail("conflict")
	}
	var dependency snapshot.Dependency
	var candidate snapshot.Candidate
	for _, d := range s.Dependencies {
		if d.ID == selection.DependencyID {
			dependency = d
			for _, u := range d.Candidates {
				if u.ID == selection.CandidateID {
					candidate = u
				}
			}
		}
	}
	if !Supported(dependency, candidate) {
		return Record{}, backend.Fail("invalid_argument")
	}
	if candidate.PR != nil {
		return Record{}, backend.Fail("conflict")
	}
	id := snapshot.ID(s.Repository, dependency.ID, candidate.ID)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	for _, r := range c.store.records {
		if r.Dependency.ID == dependency.ID && (r.State == "queued" || r.State == "running" || r.State == "awaiting_pr" || r.State == "open" || r.State == "draft" || r.State == "needs_attention") {
			if r.Candidate.ID == candidate.ID {
				return r, nil
			}
			return Record{}, backend.Fail("conflict")
		}
	}
	if r, ok := c.store.records[id]; ok {
		if r.State == "blocked" && r.SnapshotDigest != selection.SnapshotDigest {
			r.State = "queued"
			r.Reason = ""
			r.SnapshotDigest = selection.SnapshotDigest
			r.UpdatedAt = time.Now().UTC()
			if c.store.save(r) != nil {
				return Record{}, backend.Fail("unavailable")
			}
			select {
			case c.wake <- struct{}{}:
			default:
			}
		}
		return r, nil
	}
	now := time.Now().UTC()
	r := Record{ID: id, SnapshotDigest: selection.SnapshotDigest, Repository: s.Repository, Dependency: dependency, Candidate: candidate, Branch: "balemoh-renovate/" + id, State: "queued", CreatedAt: now, UpdatedAt: now}
	if c.store.save(r) != nil {
		return Record{}, backend.Fail("unavailable")
	}
	select {
	case c.wake <- struct{}{}:
	default:
	}
	return r, nil
}

func (c *Controller) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("GET /snapshot.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !credential.MatchBearer(r.Header.Get("Authorization"), c.reader) {
			w.WriteHeader(401)
			return
		}
		data, err := readBounded(c.config.SnapshotFile, snapshot.MaxBytes)
		if err != nil {
			w.WriteHeader(503)
			return
		}
		value, err := snapshot.Decode(data, time.Now())
		if err != nil || value.Repository.Name != c.config.Repository || value.Repository.BaseBranch != c.config.BaseBranch {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method != "HEAD" {
			_, _ = w.Write(data)
		}
	})
	mux.HandleFunc("GET /v1/requests", func(w http.ResponseWriter, r *http.Request) {
		if !credential.MatchBearer(r.Header.Get("Authorization"), c.reader) {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Catalog{GeneratedFingerprintFile: c.config.GeneratedFingerprintFile, Enabled: c.config.WriterEnabled, Requests: c.store.list()})
	})
	mux.HandleFunc("POST /v1/requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if !credential.MatchBearer(r.Header.Get("Authorization"), c.action) {
			w.WriteHeader(401)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		var input Selection
		if d.Decode(&input) != nil || d.Decode(new(any)) != io.EOF {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(backend.Fail("invalid_argument"))
			return
		}
		result, e := c.Submit(r.Context(), input)
		if e != nil {
			code := backend.Code(e)
			status := map[string]int{"invalid_argument": 400, "permission_denied": 403, "conflict": 409}[code]
			if status == 0 {
				status = 503
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(backend.Fail(code))
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})
	return mux
}

// Run is the only execution worker. The storage lock excludes other processes.
func (c *Controller) Run(ctx context.Context) {
	// A surviving running marker may represent a completed external write. Never replay it.
	c.store.mu.Lock()
	for _, r := range c.store.records {
		if r.State == "running" {
			r.State = "needs_attention"
			r.Reason = "execution_interrupted"
			r.UpdatedAt = time.Now().UTC()
			if c.store.save(r) != nil {
				c.store.mu.Unlock()
				return
			}
		}
	}
	c.store.mu.Unlock()
	for {
		if ctx.Err() != nil {
			return
		}
		c.cycle(ctx)
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-c.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}
func (c *Controller) observe(ctx context.Context, r Record) Record {
	pr, err := c.github.pull(ctx, r)
	now := time.Now().UTC()
	r.TrackedAt = &now
	r.UpdatedAt = now
	r.TrackingError = err != nil
	if err != nil && r.PR != nil {
		retained := *r.PR
		retained.Checks = "unknown"
		r.PR = &retained
	}
	if pr != nil {
		r.PR = pr
		r.State = pr.State
		r.Reason = ""
	} else if r.State == "awaiting_pr" {
		// A successful process is not proof of PR creation. Never replay an unconfirmed write.
		r.State = "needs_attention"
		r.Reason = "pr_observation_unavailable"
		if err == nil {
			exists, branchErr := c.github.branchExists(ctx, r)
			r.TrackingError = branchErr != nil
			if branchErr == nil {
				r.Reason = "pr_not_created"
				if exists {
					r.Reason = "branch_exists_without_pr"
				}
			}
		}
	}
	return r
}

func (c *Controller) cycle(ctx context.Context) {
	rows := c.store.list()
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].TrackedAt == nil {
			return rows[j].TrackedAt != nil
		}
		return rows[j].TrackedAt != nil && rows[i].TrackedAt.Before(*rows[j].TrackedAt)
	})
	tracked := 0
	for _, r := range rows {
		if ctx.Err() != nil {
			return
		}
		if r.State == "merged" {
			continue
		}
		if tracked >= 10 {
			break
		}
		tracked++
		r = c.observe(ctx, r)
		c.store.mu.Lock()
		e := c.store.save(r)
		c.store.mu.Unlock()
		if e != nil {
			return
		}
		if r.TrackingError || r.PR != nil || r.State != "queued" || !c.config.WriterEnabled {
			continue
		}
		exists, branchErr := c.github.branchExists(ctx, r)
		if branchErr != nil {
			continue
		}
		if exists {
			r.State = "needs_attention"
			r.Reason = "branch_exists_without_pr"
			r.UpdatedAt = time.Now().UTC()
			c.store.mu.Lock()
			e = c.store.save(r)
			c.store.mu.Unlock()
			if e != nil {
				return
			}
			continue
		}
		base, e := c.github.base(ctx, r.Repository.Name, r.Repository.BaseBranch)
		if e != nil {
			continue
		}
		if base != r.Repository.Commit {
			r.State = "blocked"
			r.Reason = "base_changed"
		} else {
			r.State = "running"
			r.UpdatedAt = time.Now().UTC()
			c.store.mu.Lock()
			e = c.store.save(r)
			c.store.mu.Unlock()
			if e != nil {
				return
			}
			executionCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
			e = c.execute(executionCtx, r)
			cancel()
			if e != nil {
				r.State = "needs_attention"
				r.Reason = "execution_unconfirmed"
				var failure *engineFailure
				if errors.As(e, &failure) && !failure.mayHaveWritten {
					r.State = "blocked"
					r.Reason = failure.phase
				}
			} else {
				r.State = "awaiting_pr"
				r.Reason = ""
				r = c.observe(ctx, r)
			}
		}
		r.UpdatedAt = time.Now().UTC()
		c.store.mu.Lock()
		e = c.store.save(r)
		c.store.mu.Unlock()
		if e != nil {
			return
		}
	}
}

// decodeResponse is shared by the plugin's bounded controller client.
func DecodeCatalog(data []byte) (Catalog, error) {
	var out Catalog
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if len(data) > 1<<20 || d.Decode(&out) != nil || d.Decode(new(any)) != io.EOF || out.Requests == nil || len(out.Requests) > 512 || out.GeneratedFingerprintFile != "" && !validFingerprintPath(out.GeneratedFingerprintFile) {
		return out, errors.New("invalid controller response")
	}
	return out, nil
}
