package report

import (
	"encoding/json"
	"io"
	"unicode/utf8"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func sanitize(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return stringsToValid(s)
}

func stringsToValid(s string) string {
	return stringsMap(s)
}

func stringsMap(s string) string {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 {
			out = append(out, '�')
			i++
			continue
		}
		out = append(out, r)
		i += n
	}
	return string(out)
}

func cleanReport(r findings.Report) findings.Report {
	r.Host.Home = sanitize(r.Host.Home)
	for i := range r.Findings {
		r.Findings[i].Path = sanitize(r.Findings[i].Path)
		r.Findings[i].Why = sanitize(r.Findings[i].Why)
		if r.Findings[i].Reclaim != nil && r.Findings[i].Reclaim.Cmd == "" {
			r.Findings[i].Reclaim = nil
		}
	}
	if r.Findings == nil {
		r.Findings = []findings.Finding{}
	}
	return r
}

// JSON writes a schema-shaped report.
func JSON(w io.Writer, r findings.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(cleanReport(r))
}
