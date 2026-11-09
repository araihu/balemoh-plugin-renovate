package requests

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/caarlos0/env/v11"
)

//go:generate go tool envdoc -types Config -files config.go -output environment.md -format markdown
type Config struct {
	// Single allowed GitHub owner/repository. No client-supplied repository is accepted.
	Repository string `env:"UPGRADES_REPOSITORY,required,notEmpty"`
	// The repository base branch. A newer base requires an unchanged approved image file.
	BaseBranch string `env:"UPGRADES_BASE_BRANCH" envDefault:"main"`
	// Normalized snapshot published by the existing read-only collector.
	SnapshotFile string `env:"UPGRADES_CONTROLLER_SNAPSHOT_FILE" envDefault:"/snapshots/snapshot.json"`
	// Durable private request directory; requires working advisory flock and atomic rename.
	DataDir string `env:"UPGRADES_CONTROLLER_DATA_DIR" envDefault:"/requests"`
	// Disposable per-execution workspace, separate from durable state.
	WorkDir string `env:"UPGRADES_CONTROLLER_WORK_DIR" envDefault:"/work"`
	// Private HTTPS listen address.
	Address string `env:"UPGRADES_CONTROLLER_ADDRESS" envDefault:":8443"`
	// Reader bearer credential file for authenticated status queries.
	ReadBearerFile string `env:"UPGRADES_CONTROLLER_READ_BEARER_FILE,required,notEmpty"`
	// Separate bearer credential file for explicit request submission.
	ActionBearerFile string `env:"UPGRADES_CONTROLLER_ACTION_BEARER_FILE,required,notEmpty"`
	// Read-only GitHub token for PR reconciliation; never passed to the UI.
	GitHubReadTokenFile string `env:"UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE,required,notEmpty"`
	// Expected GitHub login creating PRs, including the [bot] suffix for a GitHub App.
	PRAuthor string `env:"UPGRADES_CONTROLLER_PR_AUTHOR,required,notEmpty"`
	// Dedicated GitHub writer token file, required only after an authorized writer migration.
	GitHubWriteTokenFile string `env:"UPGRADES_CONTROLLER_GITHUB_WRITE_TOKEN_FILE"`
	// Disabled by default. Enabling requires a separate writer handover and credential.
	WriterEnabled bool `env:"UPGRADES_CONTROLLER_WRITER_ENABLED" envDefault:"false"`
	// Optional Markdown fingerprint artifact. Only its Source digest line may change.
	GeneratedFingerprintFile string `env:"UPGRADES_CONTROLLER_GENERATED_FINGERPRINT_FILE"`
	// Maximum accepted collection age in seconds.
	MaxSnapshotAgeSeconds int `env:"UPGRADES_CONTROLLER_MAX_SNAPSHOT_AGE_SECONDS" envDefault:"3600"`
	// HTTPS certificate path.
	TLSCert string `env:"UPGRADES_CONTROLLER_TLS_CERT,required,notEmpty"`
	// HTTPS private key path.
	TLSKey             string `env:"UPGRADES_CONTROLLER_TLS_KEY,required,notEmpty"`
	commandEnvironment []string
}

func Parse(values map[string]string) (Config, error) {
	c, err := env.ParseAsWithOptions[Config](env.Options{Environment: values})
	if err != nil {
		return Config{}, errors.New("invalid request controller environment")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(c.Repository) || c.Repository == "../.." ||
		!regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\[bot\])?$`).MatchString(c.PRAuthor) ||
		strings.TrimSpace(c.BaseBranch) == "" || strings.HasPrefix(c.BaseBranch, "-") || strings.ContainsAny(c.BaseBranch, "\r\n\t ~^:?*[\\") || strings.Contains(c.BaseBranch, "..") ||
		c.GeneratedFingerprintFile != "" && !validFingerprintPath(c.GeneratedFingerprintFile) ||
		c.MaxSnapshotAgeSeconds < 60 || c.MaxSnapshotAgeSeconds > 43200 || c.DataDir == c.WorkDir {
		return Config{}, errors.New("invalid request controller environment")
	}
	for _, part := range strings.Split(c.Repository, "/") {
		if part == "." || part == ".." {
			return Config{}, errors.New("invalid request controller environment")
		}
	}
	for _, path := range []string{c.SnapshotFile, c.DataDir, c.WorkDir, c.ReadBearerFile, c.ActionBearerFile, c.GitHubReadTokenFile, c.TLSCert, c.TLSKey} {
		if !filepath.IsAbs(path) {
			return Config{}, errors.New("invalid request controller environment")
		}
	}
	if c.WriterEnabled && !filepath.IsAbs(c.GitHubWriteTokenFile) {
		return Config{}, errors.New("invalid request controller environment")
	}
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL", "NODE_EXTRA_CA_CERTS", "SSL_CERT_FILE", "SSL_CERT_DIR", "TZ"} {
		if value, ok := values[name]; ok {
			c.commandEnvironment = append(c.commandEnvironment, name+"="+value)
		}
	}
	return c, nil
}
func Load() (Config, error) { return Parse(env.ToMap(os.Environ())) }
