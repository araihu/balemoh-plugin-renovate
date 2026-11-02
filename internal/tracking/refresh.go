package tracking

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Metadata struct {
	IssuedAt  int64     `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func atomic(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".token-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	closed := f.Close()
	if e != nil {
		return e
	}
	if closed != nil {
		return closed
	}
	return os.Rename(name, path)
}
func State(dir string, now time.Time) (Metadata, error) {
	var m Metadata
	b, e := boundedFile(filepath.Join(dir, "expiry.json"), 4096)
	if e != nil || json.Unmarshal(b, &m) != nil || m.IssuedAt <= 0 || m.IssuedAt > now.Unix() || m.ExpiresAt.Sub(now) <= time.Minute {
		return m, unavailable
	}
	token, e := boundedFile(filepath.Join(dir, "token"), 4096)
	if e != nil || len(token) < 32 || !bearer.Match(token) {
		return m, unavailable
	}
	info, e := os.Stat(filepath.Join(dir, "token"))
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return m, unavailable
	}
	return m, nil
}
func (a *App) cycle(ctx context.Context, force bool) (bool, error) {
	current, e := State(a.config.OutputDir, a.now())
	if !force && e == nil && a.now().Unix()-current.IssuedAt < int64(a.config.RefreshSeconds) {
		return false, nil
	}
	issued, e := a.Issue(ctx)
	if e == nil {
		m := Metadata{IssuedAt: a.now().Unix(), ExpiresAt: issued.ExpiresAt}
		data, _ := json.Marshal(m)
		if e = atomic(filepath.Join(a.config.OutputDir, "token"), []byte(issued.Token)); e == nil {
			e = atomic(filepath.Join(a.config.OutputDir, "expiry.json"), data)
		}
		if e == nil {
			return true, nil
		}
	}
	if _, invalid := State(a.config.OutputDir, a.now()); invalid != nil {
		e = errors.Join(e, remove(filepath.Join(a.config.OutputDir, "token")), remove(filepath.Join(a.config.OutputDir, "expiry.json")))
	}
	return false, e
}
func remove(path string) error {
	e := os.Remove(path)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
func (a *App) Run(ctx context.Context, event func(string)) error {
	if e := os.MkdirAll(a.config.OutputDir, 0700); e != nil {
		return unavailable
	}
	lock, e := os.OpenFile(filepath.Join(a.config.OutputDir, ".refresh.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return unavailable
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return errors.New("App token directory already owned")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	first := true
	for {
		changed, e := a.cycle(ctx, first)
		first = false
		if ctx.Err() != nil {
			return nil
		}
		if e != nil {
			event("token_refresh_failed")
		} else if changed {
			event("token_refreshed")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
