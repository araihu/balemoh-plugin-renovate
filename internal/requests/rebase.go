package requests

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/credential"
)

// updatePRBase only maintains a previously verified native PR; it never creates an upgrade.
func (c *Controller) updatePRBase(ctx context.Context, r Record) error {
	base, err := c.github.base(ctx, r.Repository.Name, r.Repository.BaseBranch)
	if err != nil {
		return nil
	}
	_, behind, err := c.github.mergeBase(ctx, r.Repository.Name, base, r.PR.HeadSHA)
	if err != nil || behind == 0 {
		return nil
	}
	workCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	// The saved record shares this pointer; preparation must not mutate catalog readers.
	pr := *r.PR
	r.PR = &pr
	err = c.rebase(workCtx, r, base)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	current := c.store.records[r.ID]
	// Keep the durable expected head if a push may have completed.
	if err != nil && current.RebaseHead == "" {
		current.State, current.Reason = "needs_attention", "rebase_blocked"
	}
	current.UpdatedAt = time.Now().UTC()
	return c.store.save(current)
}

func (c *Controller) rebasePR(ctx context.Context, r Record, base string) error {
	if !c.config.WriterEnabled || c.config.GeneratedFingerprintFile == "" || r.PR == nil || (r.State != "open" && r.State != "draft") {
		return errors.New("rebase unavailable")
	}
	if err := c.github.sameApprovedSource(ctx, r, base); err != nil {
		return err
	}
	token, err := credential.ReadBearerFile(c.config.GitHubWriteTokenFile)
	if err != nil {
		return errors.New("writer credential unavailable")
	}
	if err = os.MkdirAll(c.config.WorkDir, 0700); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(c.config.WorkDir, "rebase-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	env := append(append([]string{}, c.config.commandEnvironment...),
		"GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraHeader",
		"GIT_CONFIG_VALUE_0=Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token)))
	fork, _, err := c.github.mergeBase(ctx, r.Repository.Name, base, r.PR.HeadSHA)
	if err != nil {
		return err
	}
	head, err := rebaseCommit(ctx, r, base, fork, "https://github.com/"+r.Repository.Name+".git", directory, c.config.GeneratedFingerprintFile, env)
	if err != nil {
		return err
	}
	// Recheck PR ownership and both immutable inputs immediately before publication.
	if err = c.rebaseGuard(ctx, r, base); err != nil {
		return err
	}
	r.RebaseHead, r.State, r.Reason = head, "needs_attention", "rebase_pending"
	r.PR.Checks = "unknown"
	r.UpdatedAt = time.Now().UTC()
	c.store.mu.Lock()
	err = c.store.save(r)
	c.store.mu.Unlock()
	if err != nil {
		return err
	}
	_, err = rebaseGit(ctx, directory, env, "push", "--force-with-lease=refs/heads/"+r.Branch+":"+r.PR.HeadSHA, "origin", "HEAD:refs/heads/"+r.Branch)
	return err
}

func (c *Controller) rebaseGuard(ctx context.Context, r Record, base string) error {
	var p nativePR
	if err := c.github.get(ctx, "/repos/"+r.Repository.Name+"/pulls/"+strconv.Itoa(r.PR.Number), &p); err != nil {
		return err
	}
	if p.Number != r.PR.Number || p.State != "open" || p.MergedAt != nil || p.User.Login != c.github.author || p.Head.SHA != r.PR.HeadSHA || p.Head.Ref != r.Branch || p.Head.Repo.FullName != r.Repository.Name || p.Base.Repo.FullName != r.Repository.Name || p.Base.Ref != r.Repository.BaseBranch || len(p.AutoMerge) > 0 && string(p.AutoMerge) != "null" {
		return errors.New("PR changed before rebase")
	}
	latest, err := c.github.base(ctx, r.Repository.Name, r.Repository.BaseBranch)
	if err != nil || latest != base {
		return errors.New("base changed before rebase")
	}
	return nil
}

type gitOutput struct{ bytes.Buffer }

func (b *gitOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 2<<20 {
		return 0, errors.New("Git output exceeds bound")
	}
	return b.Buffer.Write(p)
}

func rebaseGit(ctx context.Context, directory string, env []string, args ...string) ([]byte, error) {
	fixed := []string{"--no-pager", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "core.autocrlf=false", "-c", "commit.gpgSign=false"}
	cmd := exec.CommandContext(ctx, "git", append(fixed, args...)...)
	cmd.Dir, cmd.Env = directory, env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var output gitOutput
	cmd.Stdout = &output
	// Git errors can contain transport credentials. Expose only a stable error.
	if cmd.Run() != nil {
		return nil, errors.New("Git rebase operation failed")
	}
	return output.Bytes(), nil
}

// rebaseCommit transports the already-approved native image bytes onto the new base.
func rebaseCommit(ctx context.Context, r Record, base, fork, remote, directory, fingerprint string, env []string) (string, error) {
	image := r.Dependency.PackageFile
	if r.PR == nil || !sha.MatchString(base) || !sha.MatchString(fork) || !sha.MatchString(r.PR.HeadSHA) || !sha.MatchString(r.Repository.Commit) || !fs.ValidPath(image) || slices.Contains(strings.Split(image, "/"), ".git") || !validFingerprintPath(fingerprint) || image == fingerprint {
		return "", errors.New("invalid rebase input")
	}
	git := func(args ...string) ([]byte, error) { return rebaseGit(ctx, directory, env, args...) }
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", remote}, {"fetch", "--no-tags", "--depth=1", "origin", base, fork, r.PR.HeadSHA, r.Repository.Commit}, {"checkout", "--detach", base}} {
		if _, err := git(args...); err != nil {
			return "", err
		}
	}
	read := func(ref, file string) ([]byte, error) {
		entry, err := git("ls-tree", ref, "--", file)
		if err != nil || !strings.HasPrefix(string(entry), "100644 blob ") {
			return nil, errors.New("rebase requires regular files")
		}
		data, err := git("show", ref+":"+file)
		if err != nil || len(data) > 1<<20 {
			return nil, errors.New("rebase file unavailable")
		}
		return data, nil
	}
	approved, err := read(r.Repository.Commit, image)
	if err != nil {
		return "", err
	}
	latest, err := read(base, image)
	if err != nil || !bytes.Equal(approved, latest) {
		return "", errors.New("approved image file changed")
	}
	upgraded, err := read(r.PR.HeadSHA, image)
	if err != nil {
		return "", err
	}
	expected, err := approvedImageContent(r, approved)
	if err != nil || !bytes.Equal(expected, upgraded) {
		return "", errors.New("native image differs from approval")
	}
	// Verify the immutable branch tree as well as the API association before discarding its old parent.
	priorDiff, err := git("diff", "--name-status", fork, r.PR.HeadSHA, "--")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(priorDiff)), "\n")
	if len(lines) < 1 || len(lines) > 2 {
		return "", errors.New("unexpected native PR diff")
	}
	for _, line := range lines {
		if line != "M\t"+image && line != "M\t"+fingerprint {
			return "", errors.New("unexpected native PR diff")
		}
	}
	oldFingerprint, err := read(fork, fingerprint)
	if err != nil {
		return "", err
	}
	branchFingerprint, err := read(r.PR.HeadSHA, fingerprint)
	if err != nil || !fingerprintOnly(oldFingerprint, branchFingerprint) {
		return "", errors.New("native fingerprint differs from approval")
	}
	before, err := read(base, fingerprint)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err = root.WriteFile(image, upgraded, 0600); err != nil {
		return "", err
	}
	if err = refreshFingerprint(ctx, directory, fingerprint, env); err != nil {
		return "", err
	}
	after, err := root.ReadFile(fingerprint)
	if err != nil || !fingerprintOnly(before, after) {
		return "", errors.New("fingerprint content changed")
	}
	if _, err = git("add", "--", image, fingerprint); err != nil {
		return "", err
	}
	changed, err := git("diff", "--cached", "--name-only", base, "--")
	if err != nil {
		return "", err
	}
	names := strings.Fields(string(changed))
	if len(names) < 1 || len(names) > 2 || !slices.Contains(names, image) || slices.ContainsFunc(names, func(n string) bool { return n != image && n != fingerprint }) {
		return "", errors.New("unexpected rebase diff")
	}
	for _, args := range [][]string{{"config", "user.name", "Balemoh Renovate"}, {"config", "user.email", "renovate@balemoh.local"}, {"commit", "--no-gpg-sign", "-C", r.PR.HeadSHA}} {
		if _, err = git(args...); err != nil {
			return "", err
		}
	}
	result, err := git("rev-parse", "HEAD")
	head := strings.TrimSpace(string(result))
	if err != nil || !sha.MatchString(head) {
		return "", errors.New("rebase commit unavailable")
	}
	return head, nil
}
