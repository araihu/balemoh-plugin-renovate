// Package runner owns the fixed read-only Renovate execution profile.
package runner

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/caarlos0/env/v11"
)

//go:generate go tool envdoc -output environment.md -format markdown
type Config struct {
	commandEnvironment []string
	// GitHub owner/repository scanned with a dedicated read-only credential.
	Repository string `env:"UPGRADES_REPOSITORY,required,notEmpty"`
	// Single base branch. External presets are rejected by this adapter release.
	BaseBranch string `env:"UPGRADES_BASE_BRANCH" envDefault:"main"`
	// File containing a dedicated read-only GitHub installation token or fine-grained token.
	TokenFile string `env:"UPGRADES_GITHUB_TOKEN_FILE,required,notEmpty"`
	// Disposable working directory. Raw reports never enter the published directory.
	WorkDir string `env:"UPGRADES_WORK_DIR" envDefault:"/work"`
	// Shared directory retaining the last validated snapshot and repository lock.
	PublishDir string `env:"UPGRADES_PUBLISH_DIR" envDefault:"/snapshots"`
}

func Parse(values map[string]string) (Config, error) {
	c, err := env.ParseAsWithOptions[Config](env.Options{Environment: values})
	if err != nil {
		return Config{}, errors.New("invalid collector environment")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(c.Repository) || strings.HasPrefix(c.BaseBranch, "-") || strings.TrimSpace(c.BaseBranch) == "" || strings.ContainsAny(c.BaseBranch, "\r\n\t ~^:?*[\\") || strings.Contains(c.BaseBranch, "..") || !filepath.IsAbs(c.TokenFile) || !filepath.IsAbs(c.WorkDir) || !filepath.IsAbs(c.PublishDir) || c.WorkDir == c.PublishDir {
		return Config{}, errors.New("invalid collector environment")
	}
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL", "NODE_EXTRA_CA_CERTS", "SSL_CERT_FILE", "SSL_CERT_DIR", "TZ"} {
		if value, ok := values[name]; ok {
			c.commandEnvironment = append(c.commandEnvironment, name+"="+value)
		}
	}
	return c, nil
}
func Load() (Config, error) { return Parse(env.ToMap(os.Environ())) }
