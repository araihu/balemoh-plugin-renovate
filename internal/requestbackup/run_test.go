package requestbackup

import (
	"encoding/base64"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func environment() map[string]string {
	return map[string]string{"UPGRADES_REPOSITORY": "example/project", "UPGRADES_BACKUP_URL": "https://controller.example/v1/requests", "UPGRADES_BACKUP_RESTIC_REPOSITORY": "s3:https://s3.example/bucket/requests", "RESTIC_REPOSITORY": "s3:https://s3.example/bucket/requests", "UPGRADES_BACKUP_HOST": "controller", "UPGRADES_BACKUP_TAG": "requests-production"}
}
func TestBackupEnvironment(t *testing.T) {
	base := environment()
	c, e := Parse(base)
	if e != nil || c.KeepDaily != 30 || c.KeepLast != 7 {
		t.Fatal("default policy rejected", e)
	}
	for key, value := range map[string]string{"RESTIC_REPOSITORY": "s3:https://s3.example/bucket/unrelated", "UPGRADES_BACKUP_URL": "http://controller.example/v1/requests", "UPGRADES_BACKUP_TAG": "--all", "UPGRADES_BACKUP_HOST": "*", "UPGRADES_BACKUP_KEEP_DAILY": "0", "UPGRADES_BACKUP_KEEP_LAST": "bad", "UPGRADES_BACKUP_RESTIC_BIN": "relative", "UPGRADES_REPOSITORY": ""} {
		t.Run(key, func(t *testing.T) {
			v := maps.Clone(base)
			v[key] = value
			if _, e := Parse(v); e == nil {
				t.Fatal("invalid backup environment accepted")
			}
		})
	}
	delete(base, "UPGRADES_BACKUP_RESTIC_REPOSITORY")
	if _, e := Parse(base); e == nil {
		t.Fatal("implicit backup repository accepted")
	}
}
func TestRunScopesRetentionAndStopsBeforePruneOnFailure(t *testing.T) {
	for _, failure := range []string{"", "check", "backup"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			reader := strings.Repeat("r", 32)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer "+reader {
					t.Error("wrong export credential")
				}
				_, _ = w.Write([]byte(`{"enabled":false,"requests":[]}`))
			}))
			defer server.Close()
			token := filepath.Join(dir, "reader")
			ca := filepath.Join(dir, "ca")
			if e := os.WriteFile(token, []byte(reader), 0600); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(ca, []byte(base64.StdEncoding.EncodeToString(server.TLS.Certificates[0].Certificate[0])), 0600); e != nil {
				t.Fatal(e)
			}
			binary := filepath.Join(dir, "restic")
			calls := filepath.Join(dir, "calls")
			script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$RESTIC_TEST_LOG"
if [ "$2" = "$RESTIC_TEST_FAIL" ]; then echo private-diagnostic >&2; exit 1; fi
if [ "$2" = backup ]; then
  /bin/cat >/dev/null
  printf '%s\n' '{"message_type":"summary","snapshot_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","total_bytes_processed":2048}'
fi
`
			if e := os.WriteFile(binary, []byte(script), 0700); e != nil {
				t.Fatal(e)
			}
			values := environment()
			values["UPGRADES_BACKUP_URL"] = server.URL + "/v1/requests"
			values["UPGRADES_BACKUP_TOKEN_FILE"] = token
			values["UPGRADES_BACKUP_CA_FILE"] = ca
			values["UPGRADES_BACKUP_WORK_DIR"] = filepath.Join(dir, "work")
			values["UPGRADES_BACKUP_RESTIC_BIN"] = binary
			values["RESTIC_TEST_LOG"] = calls
			values["RESTIC_TEST_FAIL"] = failure
			values["GITHUB_TOKEN"] = "must-not-reach-restic"
			c, e := Parse(values)
			if e != nil {
				t.Fatal(e)
			}
			for _, v := range c.commandEnvironment {
				if strings.HasPrefix(v, "GITHUB_TOKEN=") {
					t.Fatal("unrelated credential inherited")
				}
			}
			result, e := Run(t.Context(), c)
			if failure == "" && (e != nil || result.RepositoryCheck != "passed" || result.Records != 0) {
				t.Fatal("backup failed", e)
			}
			if failure != "" && (e == nil || strings.Contains(e.Error(), "private-diagnostic")) {
				t.Fatal("error not propagated/redacted")
			}
			data, e := os.ReadFile(calls)
			if e != nil {
				t.Fatal(e)
			}
			text := string(data)
			if strings.Contains(text, "init") {
				t.Fatal("backup initialized repository")
			}
			if failure != "" && strings.Contains(text, "forget") {
				t.Fatal("failed backup reached retention")
			}
			if failure == "" && (!strings.Contains(text, "check --read-data") || !strings.Contains(text, "forget --tag requests-production --host controller --group-by host,paths,tags --keep-daily 30 --keep-last 7 --prune")) {
				t.Fatal("retention escaped explicit group")
			}
		})
	}
}
