package snapshot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWarningCausesRequireMatchingNativeEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, pkg, manager   string
		current              *string
		skip                 string
		logs, problem, cause string
	}{
		{"missing latest", "ghcr.io/example/image", "kubernetes", nil, "", `{"msg":"Docker Manifest is unknown","registryHost":"https://ghcr.io","dockerRepository":"example/image","tag":"latest","err":{"response":{"statusCode":404}}}`, "", "cause_latest_tag_missing"},
		{"missing explicit tag", "ghcr.io/example/image", "kubernetes", new("1.0.0"), "", `{"msg":"Docker Manifest is unknown","registryHost":"https://ghcr.io","dockerRepository":"example/image","tag":"1.0.0","err":{"response":{"statusCode":404}}}`, "", "cause_manifest_tag_missing"},
		{"another package", "ghcr.io/example/image", "kubernetes", nil, "", `{"msg":"Docker Manifest is unknown","registryHost":"https://ghcr.io","dockerRepository":"other/image","tag":"latest","err":{"response":{"statusCode":404}}}`, "", "cause_digest_resolution_failed"},
		{"another tag", "ghcr.io/example/image", "kubernetes", new("1.0.0"), "", `{"msg":"Docker Manifest is unknown","registryHost":"https://ghcr.io","dockerRepository":"example/image","tag":"latest","err":{"response":{"statusCode":404}}}`, "", "cause_digest_resolution_failed"},
		{"scoped denied token", "ghcr.io/example/image", "kubernetes", nil, "", `{"msg":"Request failed with status code 401 (Unauthorized)","err":{"options":{"url":"https://ghcr.io/token?scope=repository%3Aexample%2Fimage%3Apull&token=do-not-publish","headers":{"authorization":"do-not-publish"}},"response":{"statusCode":401,"body":"do-not-publish"}}}`, "", "cause_registry_access_denied"},
		{"normal auth challenge", "ghcr.io/example/image", "kubernetes", nil, "", `{"msg":"Request failed with status code 401 (Unauthorized)","err":{"options":{"url":"https://ghcr.io/v2/example/image/manifests/latest"},"response":{"statusCode":401}}}`, "", "cause_digest_resolution_failed"},
		{"credentials in URL ignored", "ghcr.io/example/image", "kubernetes", nil, "", `{"msg":"Request failed with status code 401 (Unauthorized)","err":{"options":{"url":"https://user:do-not-publish@ghcr.io/token?scope=repository%3Aexample%2Fimage%3Apull"},"response":{"statusCode":401}}}`, "", "cause_digest_resolution_failed"},
		{"rate limit", "registry.example/image", "kubernetes", new("1.0.0"), "", `{"msg":"Unknown Error looking up docker manifest","registryHost":"https://registry.example","dockerRepository":"image","tag":"1.0.0","err":{"response":{"statusCode":429}}}`, "", "cause_registry_rate_limited"},
		{"blocked private registry", "10.0.0.1:5000/local/image", "kubernetes", nil, "", "", `{"msg":"Blocked HTTP request to an internal host - example","hostType":"docker","url":"https://10.0.0.1:5000/v2/local/image/tags/list?n=10000&token=do-not-publish"}`, "cause_internal_host_blocked"},
		{"skipped value ignores same-package HTTP evidence", "quay.io/example/image", "helm-values", new(""), "invalid-value", `{"msg":"Unknown Error looking up docker manifest","registryHost":"https://quay.io","dockerRepository":"example/image","tag":"latest","err":{"response":{"statusCode":429}}}`, "", "cause_empty_helm_tag"},
		{"blank Helm tag", "quay.io/example/image", "helm-values", new(""), "invalid-value", "", "", "cause_empty_helm_tag"},
		{"invalid nonblank value", "quay.io/example/image", "helm-values", new("bad value"), "invalid-value", "", "", "cause_invalid_value"},
		{"unknown skip", "quay.io/example/image", "helm-values", new("1"), "future-skip", "", "", "cause_lookup_skipped"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw rawReport
			if err := json.Unmarshal(fixture(t), &raw); err != nil {
				t.Fatal(err)
			}
			repo := raw.Repositories["example/read-only"]
			warning, _ := json.Marshal(map[string]string{"message": "Could not determine new digest for update (docker package " + tc.pkg + ")"})
			repo.Files = map[string][]rawFile{tc.manager: {{File: "k8s/test.yaml", Deps: []rawDependency{{Datasource: "docker", Name: tc.pkg, Package: tc.pkg, Current: tc.current, Skip: tc.skip, Warnings: []json.RawMessage{warning}}}}}}
			repo.Problems = nil
			if tc.problem != "" {
				repo.Problems = []json.RawMessage{json.RawMessage(tc.problem)}
			}
			raw.Repositories["example/read-only"] = repo
			data, _ := json.Marshal(raw)
			s, err := Normalize(data, metadata(), strings.NewReader(tc.logs))
			if err != nil {
				t.Fatal(err)
			}
			d := s.Dependencies[0]
			if d.Assessment != "unknown" || len(d.Reasons) != 2 || d.Reasons[0] != "lookup_unconfirmed" || d.Reasons[1] != tc.cause {
				t.Fatalf("wrong cause: %+v", d.Reasons)
			}
			encoded, _ := json.Marshal(s)
			if strings.Contains(string(encoded), "do-not-publish") || strings.Contains(string(encoded), "authorization") {
				t.Fatal("raw diagnostic data leaked")
			}
		})
	}
}
func TestDiagnosticLimitsAndLegacyFallback(t *testing.T) {
	if _, err := Normalize(fixture(t), metadata(), strings.NewReader(strings.Repeat("x", (1<<20)+1))); err == nil {
		t.Fatal("oversized diagnostic line accepted")
	}
	a, err := Normalize(fixture(t), metadata())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Normalize(fixture(t), metadata(), strings.NewReader("not JSON\n{\"msg\":\"new native message\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if string(ab) != string(bb) {
		t.Fatal("unknown logging changed normalization")
	}
}
