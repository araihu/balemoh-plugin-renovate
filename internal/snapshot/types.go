// Package snapshot owns the versioned, credential-free advisory contract.
package snapshot

import "time"

const RenovateVersion = "44.197.0"
const RenovateImage = "ghcr.io/renovatebot/renovate@sha256:d3aa7ee1726eb31bf0b8b006f07bcde28747a820e450cf7f4010c45913f5257f"
const AdapterVersion = "0.0.3"
const MaxBytes = 10 << 20

type Snapshot struct {
	SchemaVersion int          `json:"schemaVersion"`
	RunID         string       `json:"runId"`
	StartedAt     time.Time    `json:"startedAt"`
	CompletedAt   time.Time    `json:"completedAt"`
	Producer      Producer     `json:"producer"`
	Repository    Repository   `json:"repository"`
	Coverage      Coverage     `json:"coverage"`
	Dependencies  []Dependency `json:"dependencies"`
}
type Producer struct {
	RenovateVersion string `json:"renovateVersion"`
	RenovateImage   string `json:"renovateImage"`
	AdapterVersion  string `json:"adapterVersion"`
	PolicyHash      string `json:"policyHash"`
}
type Repository struct {
	Platform   string `json:"platform"`
	Name       string `json:"name"`
	BaseBranch string `json:"baseBranch"`
	Commit     string `json:"commit"`
}
type Coverage struct {
	Scope         string   `json:"scope"`
	Status        string   `json:"status"`
	SourceRecords int      `json:"sourceRecords"`
	Extracted     int      `json:"extracted"`
	Assessed      int      `json:"assessed"`
	Excluded      int      `json:"excluded"`
	Unknown       int      `json:"unknown"`
	Reasons       []string `json:"reasons"`
}
type Dependency struct {
	ID                string      `json:"id"`
	Manager           string      `json:"manager"`
	PackageFile       string      `json:"packageFile"`
	DependencyKey     string      `json:"dependencyKey"`
	PackageName       string      `json:"packageName"`
	RegistryURL       string      `json:"registryUrl,omitempty"`
	SourceOccurrences int         `json:"sourceOccurrences,omitempty"`
	CurrentValue      *string     `json:"currentValue"`
	CurrentDigest     *string     `json:"currentDigest"`
	Assessment        string      `json:"assessment"`
	Candidates        []Candidate `json:"candidates"`
	Reasons           []string    `json:"reasons"`
}
type Candidate struct {
	IsVulnerabilityAlert bool    `json:"isVulnerabilityAlert,omitempty"`
	ID                   string  `json:"id"`
	NewValue             *string `json:"newValue"`
	NewDigest            *string `json:"newDigest"`
	UpdateType           string  `json:"updateType"`
	Policy               string  `json:"policy"`
	Workflow             string  `json:"workflow"`
	BranchName           string  `json:"branchName,omitempty"`
	BranchResult         string  `json:"branchResult,omitempty"`
	PR                   *PR     `json:"pr,omitempty"`
}
type PR struct {
	Number  int     `json:"number"`
	URL     string  `json:"url"`
	HeadSHA *string `json:"headSha"`
}
type Metadata struct {
	RunID          string     `json:"runId"`
	StartedAt      time.Time  `json:"startedAt"`
	CompletedAt    time.Time  `json:"completedAt"`
	Producer       Producer   `json:"producer"`
	Repository     Repository `json:"repository"`
	ExitCode       int        `json:"exitCode"`
	DryRun         string     `json:"dryRun"`
	CheckoutCommit string     `json:"checkoutCommit"`
	BeforeCommit   string     `json:"beforeCommit"`
	AfterCommit    string     `json:"afterCommit"`
}
