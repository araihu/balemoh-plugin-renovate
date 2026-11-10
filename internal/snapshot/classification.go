package snapshot

func emptyCurrent(d rawDependency) bool { return d.Current == nil || *d.Current == "" }

// These are extraction placeholders, not identifiable image references.
func outsideImageScope(manager string, d rawDependency) string {
	if d.Digest != nil || len(d.Updates) != 0 || len(d.Warnings) != 0 {
		return ""
	}
	if manager == "helm-values" && d.Skip == "invalid-name" && d.Name == "" && d.Package == "" && emptyCurrent(d) {
		return "chart_defaults"
	}
	if manager == "dockerfile" && d.Skip == "contains-variable" {
		return "build_template"
	}
	return ""
}

func inheritedTag(d rawDependency) bool {
	return d.Skip == "invalid-value" && emptyCurrent(d) && d.Digest == nil && len(d.Updates) == 0 && len(d.Warnings) == 0
}
