// Package requestbackup runs the versioned request export and Restic policy configured by the operator.
package requestbackup

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/caarlos0/env/v11"
)

//go:generate go tool envdoc -types Config -files config.go -output environment.md -format markdown
type Config struct {
	// Repository whose request catalog may be backed up.
	Repository string `env:"UPGRADES_REPOSITORY,required,notEmpty"`
	// Private authenticated HTTPS endpoint ending in /v1/requests.
	URL string `env:"UPGRADES_BACKUP_URL,required,notEmpty"`
	// Reader bearer file; never the action or GitHub writer credential.
	TokenFile string `env:"UPGRADES_BACKUP_TOKEN_FILE" envDefault:"/credentials/reader/token"`
	// Base64-encoded DER trust anchor used by the private controller endpoint.
	CAFile string `env:"UPGRADES_BACKUP_CA_FILE" envDefault:"/credentials/reader/ca-base64"`
	// Writable disposable work directory, without the production request PVC.
	WorkDir string `env:"UPGRADES_BACKUP_WORK_DIR" envDefault:"/work"`
	// Pinned Restic executable supplied by the deployment.
	ResticBin string `env:"UPGRADES_BACKUP_RESTIC_BIN" envDefault:"/tools/restic"`
	// Explicit expected isolated repository; must equal RESTIC_REPOSITORY before any operation.
	ExpectedRepository string `env:"UPGRADES_BACKUP_RESTIC_REPOSITORY,required,notEmpty"`
	// Restic backend repository selected by the deployment.
	ResticRepository string `env:"RESTIC_REPOSITORY,required,notEmpty"`
	// Snapshot host used by both backup and retention selection.
	Host string `env:"UPGRADES_BACKUP_HOST,required,notEmpty"`
	// Snapshot tag used by both backup and retention selection.
	Tag string `env:"UPGRADES_BACKUP_TAG,required,notEmpty"`
	// Number of daily snapshots to retain within this exact host/tag group.
	KeepDaily int `env:"UPGRADES_BACKUP_KEEP_DAILY" envDefault:"30"`
	// Minimum number of latest snapshots to retain within this exact host/tag group.
	KeepLast           int `env:"UPGRADES_BACKUP_KEEP_LAST" envDefault:"7"`
	commandEnvironment []string
}

func Parse(values map[string]string) (Config, error) {
	c, e := env.ParseAsWithOptions[Config](env.Options{Environment: values})
	u, ue := url.Parse(c.URL)
	label := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	if e != nil || ue != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "/v1/requests" || u.RawQuery != "" || u.Fragment != "" || !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(c.Repository) || c.ResticRepository != c.ExpectedRepository || strings.TrimSpace(c.ResticRepository) == "" || !label.MatchString(c.Host) || !label.MatchString(c.Tag) || c.KeepDaily < 1 || c.KeepDaily > 3650 || c.KeepLast < 1 || c.KeepLast > 3650 {
		return Config{}, errors.New("invalid request backup environment")
	}
	for _, part := range strings.Split(c.Repository, "/") {
		if part == "." || part == ".." {
			return Config{}, errors.New("invalid request backup environment")
		}
	}
	for _, v := range []string{c.WorkDir, c.TokenFile, c.CAFile, c.ResticBin} {
		if !filepath.IsAbs(v) {
			return Config{}, errors.New("invalid request backup environment")
		}
	}
	for name, value := range values {
		if strings.HasPrefix(name, "RESTIC_") || strings.HasPrefix(name, "AWS_") || name == "HOME" || name == "TMPDIR" || name == "SSL_CERT_FILE" || name == "SSL_CERT_DIR" {
			c.commandEnvironment = append(c.commandEnvironment, name+"="+value)
		}
	}
	return c, nil
}
func Load() (Config, error) { return Parse(env.ToMap(os.Environ())) }
