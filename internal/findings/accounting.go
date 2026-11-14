package findings

import (
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func newer(a, b string) string {
	parse := func(s string) time.Time {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
		return time.Time{}
	}
	if parse(b).After(parse(a)) {
		return b
	}
	return a
}

// Normalize keeps inventory parents visible but never offers them as delete
// targets. A fresh copy permits callers to render/filter a persisted report.
func Normalize(r Report) Report {
	out := make([]Finding, 0, len(r.Findings))
	seen := map[string]int{}
	for _, f := range r.Findings {
		key := f.Path
		if filepath.IsAbs(key) {
			key = filepath.Clean(key)
		}
		if i, ok := seen[key]; ok {
			prev := out[i]
			last := newer(prev.LastUsed, f.LastUsed)
			incomingRisk := f.Risk
			// Prefer the owner-aware record over a generic catalog record. Preserve
			// keep/never from either observation of exactly the same resource.
			if f.Reclaim == nil || f.Reclaim.Kind == "" {
				f = prev
			}
			if prev.Risk == RiskKeep || prev.Risk == RiskNever {
				f.Risk = prev.Risk
				f.Reclaim = nil
			}
			if incomingRisk == RiskKeep || incomingRisk == RiskNever {
				f.Risk = incomingRisk
				f.Reclaim = nil
			}
			f.LastUsed = last
			out[i] = f
			continue
		}
		seen[key] = len(out)
		out = append(out, f)
	}
	// Descendants are resolved before parents so recency propagates all the way up.
	order := make([]int, len(out))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return len(out[order[i]].Path) > len(out[order[j]].Path) })
	for _, i := range order {
		f := &out[i]
		if !filepath.IsAbs(f.Path) {
			continue
		}
		parent := filepath.Dir(filepath.Clean(f.Path))
		for parent != "/" && parent != "." {
			if j, ok := seen[parent]; ok && j != i {
				out[j].SummaryOnly = true
				out[j].Reclaim = nil
				out[j].LastUsed = newer(out[j].LastUsed, f.LastUsed)
			}
			parent = filepath.Dir(parent)
		}
	}
	// VM bytes and images describe the same Docker storage at different layers.
	hasImages := false
	for _, f := range out {
		if f.Category == "docker-image" {
			hasImages = true
		}
	}
	if hasImages {
		for i := range out {
			if strings.HasPrefix(out[i].Category, "docker") && out[i].Category != "docker-image" {
				out[i].SummaryOnly = true
				out[i].Reclaim = nil
			}
		}
	}
	total := &ReclaimTotal{Basis: "Minimum filesystem-private bytes, excluding shared clone/snapshot blocks and hard-linked data. Additional shared storage is measured after deletion."}
	for _, f := range out {
		if f.SummaryOnly || f.Risk == RiskKeep || f.Risk == RiskNever {
			continue
		}
		if !filepath.IsAbs(f.Path) && (f.Reclaim == nil || f.Reclaim.Kind == "") {
			continue
		}
		total.Targets++
		if f.ReclaimBytes == nil {
			total.Unknown++
		} else {
			total.KnownPrivateBytes += *f.ReclaimBytes
		}
	}
	if total.Targets == 0 {
		zero := int64(0)
		total.Bytes = &zero
	} else if total.Unknown == 0 {
		known := total.KnownPrivateBytes
		total.Bytes = &known
	}
	r.Findings = out
	r.ReclaimTotal = total
	return r
}
