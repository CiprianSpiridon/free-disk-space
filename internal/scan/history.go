package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

// HistoryEntry is one archived scan, newest-first in ListHistoryDir.
type HistoryEntry struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	GeneratedAt string `json:"generated_at"`
	Mode        string `json:"mode"`
	Findings    int    `json:"findings"`
	InUseBytes  int64  `json:"in_use_bytes"`
	FreeBytes   int64  `json:"free_bytes"`
}

// HistoryDir is $XDG_CACHE_HOME/freedisk/history (else ~/.cache/freedisk/history).
func HistoryDir() string {
	return filepath.Join(filepath.Dir(LastScanPath()), "history")
}

// HistoryFileID is YYYYMMDDTHHMMSSZ-mode from generated_at + mode.
func HistoryFileID(generatedAt, mode string) string {
	mode = sanitizeHistoryMode(mode)
	t, err := time.Parse(time.RFC3339, generatedAt)
	if err != nil {
		t = time.Now().UTC()
	}
	return t.UTC().Format("20060102T150405Z") + "-" + mode
}

func sanitizeHistoryMode(mode string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(mode) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return "full"
	}
	return s
}

func uniqueHistoryPath(dir, id string) (path, useID string, err error) {
	useID = id
	for n := 2; n <= 100; n++ {
		path = filepath.Join(dir, useID+".json")
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return path, useID, nil
		}
		if err != nil {
			return "", "", err
		}
		useID = fmt.Sprintf("%s-%d", id, n)
	}
	return "", "", fmt.Errorf("too many history files for %s", id)
}

// WriteHistory archives a copy of the report next to last-scan.json.
func WriteHistory(r findings.Report) error {
	_, _, err := WriteHistoryTo(HistoryDir(), r)
	return err
}

// WriteHistoryTo writes one timestamped report into dir. Returns the id and path.
func WriteHistoryTo(dir string, r findings.Report) (id, path string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	id = HistoryFileID(r.GeneratedAt, r.Mode)
	path, id, err = uniqueHistoryPath(dir, id)
	if err != nil {
		return "", "", err
	}
	if err := WriteLastScan(path, r); err != nil {
		return "", "", err
	}
	return id, path, nil
}

// ListHistoryDir returns archived scans, newest first. Missing dir is empty, not an error.
func ListHistoryDir(dir string) ([]HistoryEntry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []HistoryEntry
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		r, err := ReadReportFile(p)
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		mode := r.Mode
		if mode == "" {
			mode = modeFromHistoryID(id)
		}
		out = append(out, HistoryEntry{
			ID:          id,
			Path:        p,
			GeneratedAt: r.GeneratedAt,
			Mode:        mode,
			Findings:    len(r.Findings),
			InUseBytes:  r.Volume.InUseBytes,
			FreeBytes:   r.Volume.FreeBytes,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GeneratedAt != out[j].GeneratedAt {
			return out[i].GeneratedAt > out[j].GeneratedAt
		}
		return out[i].ID > out[j].ID
	})
	return out, nil
}

func modeFromHistoryID(id string) string {
	i := strings.Index(id, "Z-")
	if i < 0 || i+2 >= len(id) {
		return ""
	}
	rest := id[i+2:]
	if j := strings.LastIndex(rest, "-"); j >= 0 {
		if _, err := strconv.Atoi(rest[j+1:]); err == nil {
			rest = rest[:j]
		}
	}
	return rest
}

// LookupHistory finds one archived scan. spec is latest, a 1-based index, an id, or a unique prefix.
func LookupHistory(dir, spec string) (HistoryEntry, error) {
	entries, err := ListHistoryDir(dir)
	if err != nil {
		return HistoryEntry{}, err
	}
	if len(entries) == 0 {
		return HistoryEntry{}, fmt.Errorf("no historic reports; run scan first")
	}
	spec = strings.TrimSpace(spec)
	spec = strings.TrimSuffix(spec, ".json")
	if spec == "" {
		return HistoryEntry{}, fmt.Errorf("history show needs an id, latest, or a 1-based index")
	}
	if spec == "latest" {
		return entries[0], nil
	}
	var prefix []HistoryEntry
	for _, e := range entries {
		base := filepath.Base(e.Path)
		if e.ID == spec || base == spec || base == spec+".json" {
			return e, nil
		}
		if strings.HasPrefix(e.ID, spec) {
			prefix = append(prefix, e)
		}
	}
	// 1–3 digit specs are list indexes when in range. "2" must not match
	// every 2026… filename as a prefix.
	if n, ok := historyIndex(spec); ok && n <= len(entries) {
		return entries[n-1], nil
	}
	if len(prefix) == 1 {
		return prefix[0], nil
	}
	if len(prefix) > 1 {
		return HistoryEntry{}, fmt.Errorf("ambiguous historic report %q", spec)
	}
	if _, ok := historyIndex(spec); ok {
		return HistoryEntry{}, fmt.Errorf("unknown historic report %s (have %d)", spec, len(entries))
	}
	return HistoryEntry{}, fmt.Errorf("unknown historic report %s", spec)
}

func historyIndex(spec string) (int, bool) {
	if spec == "" || len(spec) > 3 {
		return 0, false
	}
	for i := 0; i < len(spec); i++ {
		if spec[i] < '0' || spec[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(spec)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}
