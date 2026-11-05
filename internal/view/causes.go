package view

import (
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
)

var lookupCauseOrder = []string{"cause_latest_tag_missing", "cause_manifest_tag_missing", "cause_registry_access_denied", "cause_registry_rate_limited", "cause_internal_host_blocked", "cause_empty_helm_tag", "cause_invalid_value", "cause_lookup_skipped", "cause_digest_resolution_failed", "cause_lookup_failed", "cause_not_recorded"}

func causeTitle(code string) string {
	titles := map[string]string{
		"cause_latest_tag_missing":       "Default latest tag not found",
		"cause_manifest_tag_missing":     "Image tag not found",
		"cause_registry_access_denied":   "Registry access denied",
		"cause_registry_rate_limited":    "Registry rate limit",
		"cause_internal_host_blocked":    "Internal registry blocked",
		"cause_empty_helm_tag":           "Empty Helm image tag",
		"cause_invalid_value":            "Unusable image version",
		"cause_lookup_skipped":           "Lookup skipped by Renovate",
		"cause_digest_resolution_failed": "Digest could not be resolved",
		"cause_lookup_failed":            "Lookup failed",
		"cause_not_recorded":             "Cause not recorded in this snapshot",
	}
	if title, ok := titles[code]; ok {
		return title
	}
	return titles["cause_not_recorded"]
}
func causeDescription(code string) string {
	descriptions := map[string]string{
		"cause_latest_tag_missing":       "No image tag was supplied. Renovate checked latest and the registry returned 404. Set an explicit tag alongside the digest.",
		"cause_manifest_tag_missing":     "The registry returned 404 for the requested image tag. Verify the tag before selecting an upgrade.",
		"cause_registry_access_denied":   "The registry denied a scoped pull-token request. Verify the package name, visibility and read credentials; this does not prove the package is private.",
		"cause_registry_rate_limited":    "The registry returned HTTP 429 for the requested image manifest. Retry later or review scoped registry read access.",
		"cause_internal_host_blocked":    "The collector blocked this internal registry under its network policy. Exclude the reference or review a narrowly scoped access grant.",
		"cause_empty_helm_tag":           "These Helm values have no explicit image tag. The chart may supply it; resolve that inheritance before assessing this image.",
		"cause_invalid_value":            "Renovate could not interpret the current image version. Verify the tag or versioning configuration.",
		"cause_lookup_skipped":           "Renovate skipped this lookup. No more specific supported cause was captured. Verify the image reference and collector configuration.",
		"cause_digest_resolution_failed": "Renovate could not determine the target digest. No more specific registry response was captured.",
		"cause_lookup_failed":            "Renovate reported a lookup warning without a more specific supported cause. Verify the image reference and collector configuration.",
		"cause_not_recorded":             "This snapshot did not capture a specific cause. A new collection with diagnostic capture is needed.",
	}
	if description, ok := descriptions[code]; ok {
		return description
	}
	return descriptions["cause_not_recorded"]
}
func displayReasons(d advisory.Item) []string {
	reasons := []string{}
	for _, r := range d.Reasons {
		if r == "lookup_unconfirmed" {
			if advisory.LookupCause(d.Dependency) == "cause_not_recorded" {
				reasons = append(reasons, "cause_not_recorded")
			}
			continue
		}
		reasons = append(reasons, r)
	}
	return reasons
}
