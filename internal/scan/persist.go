package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

// LastScanMaxAge is how long a last-scan.json stays valid for delete.
var LastScanMaxAge = 24 * time.Hour

// LastScanPath is $XDG_CACHE_HOME/freedisk/last-scan.json
func LastScanPath() string {
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "freedisk", "last-scan.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "freedisk", "last-scan.json")
}

// WriteLastScan persists the report.
func WriteLastScan(path string, r findings.Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ReadReportFile loads a scan JSON without the last-scan freshness check.
func ReadReportFile(path string) (findings.Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return findings.Report{}, err
	}
	var r findings.Report
	if err := json.Unmarshal(b, &r); err != nil {
		return findings.Report{}, fmt.Errorf("corrupt report: %w", err)
	}
	if err := r.ValidateRisks(); err != nil {
		return findings.Report{}, fmt.Errorf("corrupt report: %w", err)
	}
	for _, f := range r.Findings {
		if f.ID == "" || f.Path == "" {
			return findings.Report{}, fmt.Errorf("corrupt report: finding missing id or path")
		}
	}
	return r, nil
}

// ReadLastScan loads the report used by why/delete. Rejects scans older than LastScanMaxAge.
func ReadLastScan(path string) (findings.Report, error) {
	r, err := ReadReportFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return findings.Report{}, err
		}
		return findings.Report{}, fmt.Errorf("last-scan.json: %w", err)
	}
	if r.GeneratedAt != "" {
		t, err := time.Parse(time.RFC3339, r.GeneratedAt)
		if err == nil && time.Since(t) > LastScanMaxAge {
			return findings.Report{}, fmt.Errorf("stale last-scan.json (%s); run scan again", r.GeneratedAt)
		}
	}
	return r, nil
}
