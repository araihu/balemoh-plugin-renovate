package requestbackup

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
)

type Receipt struct {
	SnapshotID      string `json:"snapshotID"`
	ArchiveBytes    int64  `json:"archiveBytes"`
	Records         int    `json:"records"`
	WriterEnabled   bool   `json:"writerEnabledAtBackup"`
	RepositoryCheck string `json:"repositoryCheck"`
	KeepDaily       int    `json:"keepDaily"`
	KeepLast        int    `json:"keepLast"`
}

func Run(ctx context.Context, c Config) (Receipt, error) {
	if c.ResticRepository == "" || c.ResticRepository != c.ExpectedRepository {
		return Receipt{}, errors.New("Restic repository differs from explicit backup target")
	}
	// Init is deliberately absent. Repository creation and credentials stay operator-owned.
	if e := os.MkdirAll(filepath.Join(c.WorkDir, "tmp"), 0700); e != nil {
		return Receipt{}, e
	}
	run := func(log string, stdin io.Reader, args ...string) error {
		f, e := os.OpenFile(filepath.Join(c.WorkDir, log), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		defer f.Close()
		command := exec.CommandContext(ctx, c.ResticBin, append([]string{"--no-cache"}, args...)...)
		command.Env = append([]string{}, c.commandEnvironment...)
		command.Stdin = stdin
		command.Stdout = f
		stderr, e := os.OpenFile(filepath.Join(c.WorkDir, log+".stderr"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		defer stderr.Close()
		command.Stderr = stderr
		if command.Run() != nil {
			return errors.New("Restic operation failed; private diagnostic log retained")
		}
		return f.Sync()
	}
	if e := run("config.log", nil, "cat", "config"); e != nil {
		return Receipt{}, e
	}
	archive := filepath.Join(c.WorkDir, "requests.tar")
	manifest, e := requests.ExportRequestBackup(ctx, c.URL, c.TokenFile, c.CAFile, c.Repository, archive)
	if e != nil {
		return Receipt{}, e
	}
	f, e := os.Open(archive)
	if e != nil {
		return Receipt{}, e
	}
	e = run("backup.jsonl", f, "backup", "--stdin", "--stdin-filename", "requests.tar", "--host", c.Host, "--tag", c.Tag, "--json")
	closed := f.Close()
	if e != nil {
		return Receipt{}, e
	}
	if closed != nil {
		return Receipt{}, closed
	}
	log, e := os.Open(filepath.Join(c.WorkDir, "backup.jsonl"))
	if e != nil {
		return Receipt{}, e
	}
	defer log.Close()
	info, e := log.Stat()
	if e != nil || info.Size() > 2<<20 {
		return Receipt{}, errors.New("Restic summary exceeds bound")
	}
	scanner := bufio.NewScanner(io.LimitReader(log, 2<<20))
	scanner.Buffer(make([]byte, 4096), 2<<20)
	var receipt Receipt
	found := false
	for scanner.Scan() {
		var row struct {
			Type       string `json:"message_type"`
			SnapshotID string `json:"snapshot_id"`
			Bytes      int64  `json:"total_bytes_processed"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			return Receipt{}, errors.New("invalid Restic summary")
		}
		if row.Type == "summary" {
			if found {
				return Receipt{}, errors.New("ambiguous Restic summary")
			}
			receipt.SnapshotID = row.SnapshotID
			receipt.ArchiveBytes = row.Bytes
			found = true
		}
	}
	_, snapshotError := hex.DecodeString(receipt.SnapshotID)
	if snapshotError != nil || scanner.Err() != nil || !found || len(receipt.SnapshotID) != 64 || receipt.ArchiveBytes <= 0 {
		return Receipt{}, errors.New("invalid Restic summary")
	}
	if e = run("check.log", nil, "check", "--read-data"); e != nil {
		return Receipt{}, e
	}
	if e = run("retention.log", nil, "forget", "--tag", c.Tag, "--host", c.Host, "--group-by", "host,paths,tags", "--keep-daily", strconv.Itoa(c.KeepDaily), "--keep-last", strconv.Itoa(c.KeepLast), "--prune"); e != nil {
		return Receipt{}, e
	}
	receipt.Records = manifest.RecordCount
	receipt.WriterEnabled = manifest.ControllerWriterEnabled
	receipt.RepositoryCheck = "passed"
	receipt.KeepDaily = c.KeepDaily
	receipt.KeepLast = c.KeepLast
	return receipt, nil
}
