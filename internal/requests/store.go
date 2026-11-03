package requests

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"syscall"
)

var identifier = regexp.MustCompile(`^[a-f0-9]{64}$`)

type store struct {
	mu      sync.Mutex
	dir     string
	lock    *os.File
	records map[string]Record
}

func openStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "controller.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, errors.New("controller already owns request storage")
	}
	s := &store{dir: dir, lock: lock, records: map[string]Record{}}
	failed := true
	defer func() {
		if failed {
			s.close()
		}
	}()
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) > 512 {
		return nil, errors.New("request capacity exceeded")
	}
	for _, file := range files {
		b, err := readBounded(file, 1<<20)
		if err != nil || len(b) > 1<<20 {
			return nil, errors.New("invalid request storage")
		}
		var r Record
		if json.Unmarshal(b, &r) != nil || r.Validate() != nil || filepath.Base(file) != r.ID+".json" {
			return nil, errors.New("invalid request storage")
		}
		s.records[r.ID] = r
	}
	failed = false
	return s, nil
}
func (s *store) close() {
	if s.lock != nil {
		_ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		_ = s.lock.Close()
		s.lock = nil
	}
}

// save is called under mu. A durable running marker must precede engine execution.
func (s *store) save(r Record) error {
	if r.Validate() != nil {
		return errors.New("invalid request identity")
	}
	if _, ok := s.records[r.ID]; !ok && len(s.records) >= 512 {
		return errors.New("request capacity exceeded")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	// Leave room for PR/status growth and the SDK's bounded query envelope.
	if _, exists := s.records[r.ID]; !exists {
		size := len(b) + 1024
		for _, stored := range s.records {
			data, _ := json.Marshal(stored)
			size += len(data) + 1024
		}
		if size > 512<<10 {
			return errors.New("request capacity exceeded")
		}
	}

	f, err := os.CreateTemp(s.dir, ".request-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, filepath.Join(s.dir, r.ID+".json")); err != nil {
		return err
	}
	dir, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil {
		return err
	}
	s.records[r.ID] = r
	return nil
}
func (s *store) list() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]Record, 0, len(s.records))
	for _, r := range s.records {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.Before(rows[j].CreatedAt) })
	// Callers cannot mutate retained slices and pointers.
	b, _ := json.Marshal(rows)
	var out []Record
	_ = json.Unmarshal(b, &out)
	return out
}
func readBounded(path string, max int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, max+1))
	if e != nil || int64(len(b)) > max {
		return nil, errors.New("file exceeds bound")
	}
	return b, nil
}
