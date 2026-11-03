# Environment Variables

## probeEnvironment

 - `GITHUB_REPOSITORY` (**required**, non-empty) - Exact private fixture repository, checked before execution.
 - `GITHUB_SHA` (**required**, non-empty) - GitHub checkout commit used as the immutable approved base.
 - `RENOVATE_TOKEN` (**required**, non-empty) - Short-lived Actions token scoped only to the private fixture. Never printed.
 - `PATH` (**required**, non-empty) - Executable search path inherited by the isolated engine.

