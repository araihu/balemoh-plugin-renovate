package requests

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strings"
)

var fingerprintLine = regexp.MustCompile("(?m)^- Source digest: `([a-f0-9]{64})`$")

func validFingerprintPath(name string) bool {
	if len(name) > 512 || !fs.ValidPath(name) || !strings.HasSuffix(name, ".md") || !regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*$`).MatchString(name) {
		return false
	}
	return !slices.Contains(strings.Split(name, "/"), ".git")
}

func fingerprintOnly(before, after []byte) bool {
	return len(before) <= 1<<20 && len(after) <= 1<<20 && len(fingerprintLine.FindAll(before, -1)) == 1 && len(fingerprintLine.FindAll(after, -1)) == 1 && bytes.Equal(fingerprintLine.ReplaceAll(before, []byte("Source digest")), fingerprintLine.ReplaceAll(after, []byte("Source digest")))
}

// RefreshFingerprint hashes Git-visible files without executing repository code.
func RefreshFingerprint(name string) error {
	if !validFingerprintPath(name) {
		return errors.New("invalid fingerprint path")
	}
	directory, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return errors.New("fingerprint requires Git worktree")
	}
	root, err := os.OpenRoot(strings.TrimSpace(string(directory)))
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 != 0 {
		return errors.New("fingerprint must be an existing regular Markdown file")
	}
	input, err := root.Open(name)
	if err != nil {
		return err
	}
	before, readErr := io.ReadAll(io.LimitReader(input, 1<<20+1))
	err = errors.Join(readErr, input.Close())
	if err != nil || len(before) > 1<<20 || len(fingerprintLine.FindAll(before, -1)) != 1 {
		return errors.New("invalid fingerprint artifact")
	}
	command := exec.Command("git", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	command.Dir = root.Name()
	raw, err := command.Output()
	if err != nil || len(raw) > 4<<20 {
		return errors.New("fingerprint file listing unavailable")
	}
	names := strings.Split(string(raw), "\x00")
	if len(names) > 10001 {
		return errors.New("fingerprint file count exceeds bound")
	}
	slices.Sort(names)
	names = slices.Compact(names)
	digest := sha256.New()
	var size int64
	for _, file := range names {
		if file == "" || file == name || strings.HasPrefix(file, ".worktrees/") || slices.Contains(strings.Split(file, "/"), "__pycache__") || strings.HasSuffix(file, ".pyc") || strings.HasSuffix(file, ".pyo") {
			continue
		}
		if !fs.ValidPath(file) {
			return errors.New("invalid fingerprint source path")
		}
		stat, err := root.Lstat(file)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		mode := "100644"
		var link string
		if stat.Mode()&os.ModeSymlink != 0 {
			mode = "120000"
			link, err = root.Readlink(file)
			if err != nil {
				return err
			}
		} else if !stat.Mode().IsRegular() {
			continue
		} else if stat.Mode().Perm()&0111 != 0 {
			mode = "100755"
		}
		fmt.Fprintf(digest, "%s\x00%s\x00", file, mode)
		if mode == "120000" {
			digest.Write([]byte(link))
		} else {
			size += stat.Size()
			if stat.Size() > 32<<20 || size > 256<<20 {
				return errors.New("fingerprint sources exceed bound")
			}
			source, err := root.Open(file)
			if err != nil {
				return err
			}
			count, copyErr := io.Copy(digest, io.LimitReader(source, 32<<20+1))
			closeErr := source.Close()
			if count != stat.Size() {
				return errors.New("fingerprint source changed during read")
			}
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		digest.Write([]byte{0})
	}
	after := fingerprintLine.ReplaceAll(before, fmt.Appendf(nil, "- Source digest: `%x`", digest.Sum(nil)))
	if bytes.Equal(before, after) {
		return nil
	}
	temporary := name + ".tmp-" + rand.Text()
	output, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	_, writeErr := output.Write(after)
	syncErr := output.Sync()
	closeErr := output.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err = root.Rename(temporary, name); err != nil {
		return err
	}
	parent, err := root.Open(path.Dir(name))
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}
