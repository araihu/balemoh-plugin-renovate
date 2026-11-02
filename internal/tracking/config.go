// Package tracking owns the read-only GitHub App token used by the request controller.
package tracking

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
	// Single repository allowed by the installation, in owner/name form.
	Repository string `env:"UPGRADES_REPOSITORY,required,notEmpty"`
	// Expected GitHub App slug; checked before each token is issued.
	AppSlug string `env:"UPGRADES_APP_SLUG,required,notEmpty"`
	// Expected owner login of the GitHub App registration.
	AppOwner string `env:"UPGRADES_APP_OWNER,required,notEmpty"`
	// Secret directory containing app-id, installation-id and private-key.pem.
	CredentialsDir string `env:"UPGRADES_APP_CREDENTIALS_DIR" envDefault:"/credentials/app"`
	// Memory-backed directory shared with the controller; contains token and expiry.json.
	OutputDir string `env:"UPGRADES_APP_TOKEN_DIR" envDefault:"/github-read"`
	// Renewal interval in seconds; shorter intervals are for isolated qualification only.
	RefreshSeconds int `env:"UPGRADES_APP_REFRESH_SECONDS" envDefault:"2400"`
}

func Parse(values map[string]string) (Config, error) {
	c, e := env.ParseAsWithOptions[Config](env.Options{Environment: values})
	if e != nil || !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(c.Repository) || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(c.AppSlug) || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(c.AppOwner) || !filepath.IsAbs(c.CredentialsDir) || !filepath.IsAbs(c.OutputDir) || filepath.Clean(c.CredentialsDir) == filepath.Clean(c.OutputDir) || c.RefreshSeconds < 30 || c.RefreshSeconds > 2400 {
		return Config{}, errors.New("invalid App token environment")
	}
	for _, p := range strings.Split(c.Repository, "/") {
		if p == "." || p == ".." {
			return Config{}, errors.New("invalid App token environment")
		}
	}
	return c, nil
}
func Load() (Config, error) { return Parse(env.ToMap(os.Environ())) }
