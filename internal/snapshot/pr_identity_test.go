package snapshot

import (
	"encoding/json"
	"testing"
)

func TestRejectsPRFromDifferentNativeIdentity(t *testing.T) {
	report := []byte(`{
		"repositories": {
			"example/read-only": {
				"status": "activated",
				"packageFiles": {
					"kubernetes": [{
						"packageFile": "a.yaml",
						"deps": [{
							"datasource": "docker",
							"depName": "nginx",
							"depType": "container",
							"currentValue": "1",
							"updates": [{
								"newValue": "2",
								"updateType": "major",
								"branchName": "renovate/shared"
							}]
						}]
					}]
				},
				"branches": [{
					"branchName": "renovate/shared",
					"result": "pr-edited",
					"prNo": 42,
					"upgrades": [{
						"manager": "regex",
						"depType": "custom",
						"updateType": "patch",
						"packageFile": "a.yaml",
						"packageName": "nginx",
						"depName": "different-native-dependency",
						"currentValue": "1",
						"newValue": "2"
					}]
				}]
			}
		}
	}`)

	s, err := Normalize(report, metadata())
	if err != nil {
		t.Fatal(err)
	}
	candidate := s.Dependencies[0].Candidates[0]
	if candidate.PR != nil || candidate.Workflow != "unknown" {
		t.Fatalf("foreign native upgrade attributed PR/workflow: manager=kubernetes candidateType=major got workflow=%q PR=%+v", candidate.Workflow, candidate.PR)
	}
}

func TestPRBranchTupleRejectsEachNativeMismatch(t *testing.T) {
	value, target := "1", "2"
	base := rawDependency{Datasource: "docker", Name: "nginx", Type: "container", Current: &value, Updates: []rawUpdate{{Value: &target, Type: "major", Branch: "renovate/shared"}}}
	upgrade := rawUpgrade{Manager: "kubernetes", DepType: "container", Type: "major", File: "a.yaml", Package: "nginx", Name: "nginx", Current: &value, Value: &target}
	for _, tc := range []struct {
		name   string
		change func(*rawUpgrade)
	}{
		{"exact", func(*rawUpgrade) {}},
		{"manager", func(u *rawUpgrade) { u.Manager = "regex" }},
		{"dependency name", func(u *rawUpgrade) { u.Name = "other" }},
		{"dependency type", func(u *rawUpgrade) { u.DepType = "initContainer" }},
		{"update type", func(u *rawUpgrade) { u.Type = "patch" }},
		{"current digest", func(u *rawUpgrade) {
			u.CurrentDigest = new("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := upgrade
			tc.change(&other)
			report := rawReport{Repositories: map[string]rawRepo{"example/read-only": {Status: "activated", Files: map[string][]rawFile{"kubernetes": {{File: "a.yaml", Deps: []rawDependency{base}}}}, Branches: []rawBranch{{Name: "renovate/shared", Result: "needs-approval", PR: 42, Upgrades: []rawUpgrade{other}}}}}}
			data, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			s, err := Normalize(data, metadata())
			if err != nil {
				t.Fatal(err)
			}
			c := s.Dependencies[0].Candidates[0]
			if tc.name == "exact" {
				if c.PR == nil || c.Policy != "approval_required" {
					t.Fatal("exact branch evidence lost")
				}
			} else if c.PR != nil || c.Policy != "unknown" || c.IsVulnerabilityAlert || c.BranchResult != "" {
				t.Fatal("foreign branch evidence transferred")
			}
		})
	}
}

func TestProjectedTupleMustHaveUniqueOwnerAndUpgrade(t *testing.T) {
	value, target := "1", "2"
	base := rawDependency{Datasource: "docker", Name: "nginx", Type: "container", Current: &value, Updates: []rawUpdate{{Value: &target, Type: "major", Branch: "renovate/shared", IsVulnerabilityAlert: true}}}
	upgrade := rawUpgrade{File: "a.yaml", Package: "nginx", Name: "nginx", Current: &value, Value: &target, Type: "major"}
	for _, mode := range []string{"unique", "other manager", "other depType", "unsupported manager", "duplicate branch upgrade"} {
		t.Run(mode, func(t *testing.T) {
			repo := rawRepo{Status: "activated", Files: map[string][]rawFile{"kubernetes": {{File: "a.yaml", Deps: []rawDependency{base}}}}, Branches: []rawBranch{{Name: "renovate/shared", Result: "needs-approval", PR: 42, Upgrades: []rawUpgrade{upgrade}}}}
			switch mode {
			case "other manager":
				repo.Files["regex"] = []rawFile{{File: "a.yaml", Deps: []rawDependency{base}}}
			case "unsupported manager":
				repo.Files["argocd"] = []rawFile{{File: "a.yaml", Deps: []rawDependency{base}}}
			case "other depType":
				other := base
				other.Type = "initContainer"
				repo.Files["kubernetes"][0].Deps = append(repo.Files["kubernetes"][0].Deps, other)
			case "duplicate branch upgrade":
				repo.Branches[0].Upgrades = append(repo.Branches[0].Upgrades, upgrade)
			}
			data, _ := json.Marshal(rawReport{Repositories: map[string]rawRepo{"example/read-only": repo}})
			s, err := Normalize(data, metadata())
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range s.Dependencies {
				c := d.Candidates[0]
				if (c.PR != nil) != (mode == "unique") {
					t.Fatalf("ambiguous association in %s: %+v", mode, c)
				}
				if !c.IsVulnerabilityAlert {
					t.Fatal("explicit candidate flag lost")
				}
			}
		})
	}
}
