package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

// Options control markdown grouping.
type Options struct {
	ArtifactIdleDays int
	TmpIdleDays      int
}

func idleDays(lastUsed string) int {
	if lastUsed == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", lastUsed)
	if err != nil {
		t, err = time.Parse(time.RFC3339, lastUsed)
		if err != nil {
			return 0
		}
	}
	d := int(time.Since(t).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if isShellSafe(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func isShellSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '/', '.', '-', '_', '=', '@', '+', ',', ':':
			continue
		default:
			return false
		}
	}
	return true
}

func reclaimCmd(f findings.Finding) string {
	if f.Reclaim != nil && strings.TrimSpace(f.Reclaim.Cmd) != "" {
		return f.Reclaim.Cmd
	}
	if f.Path != "" {
		return "rm -rf " + shellQuote(f.Path)
	}
	return ""
}

func displayLast(s string) string {
	if s == "" {
		return "—"
	}
	d := idleDays(s)
	if d <= 0 {
		return s
	}
	return fmt.Sprintf("%s (%dd)", s, d)
}

func humanBytes(n int64) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(k), 0
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	for n/div >= k && exp < len(units)-1 {
		div *= k
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}

func highConfidence(f findings.Finding, opt Options) bool {
	if f.Risk == findings.RiskKeep || f.Risk == findings.RiskNever {
		return false
	}
	switch f.Risk {
	case findings.RiskSafeCache, findings.RiskLeftoverWorktree, findings.RiskUnusedRuntime:
		return true
	case findings.RiskRebuildable:
		return idleDays(f.LastUsed) >= opt.ArtifactIdleDays
	case findings.RiskAsk:
		if f.Category == "tmp" || strings.HasPrefix(f.Category, "tmp") {
			if idleDays(f.LastUsed) >= opt.TmpIdleDays {
				return true
			}
			// Huge tmp children (e.g. leftover /private/tmp/kensi-*) are reclaimable even if recently touched.
			if f.Bytes >= 1<<30 {
				return true
			}
		}
	}
	return false
}

// Markdown writes RECIPE §11 human report.
func Markdown(w io.Writer, r findings.Report, opt Options) error {
	if opt.ArtifactIdleDays == 0 {
		opt.ArtifactIdleDays = 30
	}
	if opt.TmpIdleDays == 0 {
		opt.TmpIdleDays = 7
	}
	v := r.Volume
	pct := 0.0
	if v.ContainerBytes > 0 {
		pct = 100 * float64(v.InUseBytes) / float64(v.ContainerBytes)
	}
	fmt.Fprintf(w, "## Disk\n")
	fmt.Fprintf(w, "- Container: %s in use / %s free (%.0f%%)\n",
		humanBytes(v.InUseBytes), humanBytes(v.FreeBytes), pct)
	if v.DataVolumeUsedBytes > 0 {
		fmt.Fprintf(w, "- Data volume: %s\n", humanBytes(v.DataVolumeUsedBytes))
	}
	if v.DFRootUsedBytes > 0 && v.InUseBytes > v.DFRootUsedBytes*2 {
		fmt.Fprintf(w, "- Note: df / disagrees (sealed snapshot)\n")
	}
	fmt.Fprintf(w, "\n## Reclaimable (high confidence)\n\n")
	fmt.Fprintf(w, "| id | path | size | last used | risk | command |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, f := range r.Findings {
		if !highConfidence(f, opt) {
			continue
		}
		fmt.Fprintf(w, "| `%s` | `%s` | %s | %s | %s | `%s` |\n",
			mdCell(f.ID), mdCell(f.Path), humanBytes(f.Bytes), displayLast(f.LastUsed), f.Risk, mdCell(reclaimCmd(f)))
	}
	fmt.Fprintf(w, "\n## Ask first\n\n")
	fmt.Fprintf(w, "| id | path | size | last used | why | command |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, f := range r.Findings {
		if f.Risk == findings.RiskKeep || f.Risk == findings.RiskNever {
			continue
		}
		if highConfidence(f, opt) {
			continue
		}
		fmt.Fprintf(w, "| `%s` | `%s` | %s | %s | %s | `%s` |\n",
			mdCell(f.ID), mdCell(f.Path), humanBytes(f.Bytes), displayLast(f.LastUsed), mdCell(f.Why), mdCell(reclaimCmd(f)))
	}
	fmt.Fprintf(w, "\n## Keep / not reclaim\n")
	for _, f := range r.Findings {
		if f.Risk == findings.RiskKeep || f.Risk == findings.RiskNever {
			fmt.Fprintf(w, "- %s (%s)\n", f.Path, humanBytes(f.Bytes))
		}
	}
	fmt.Fprintf(w, "\n## Not present\n")
	if len(r.NotPresent) == 0 {
		fmt.Fprintf(w, "- (none)\n")
	}
	for _, n := range r.NotPresent {
		fmt.Fprintf(w, "- %s\n", n)
	}
	return nil
}
