package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

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
	return os.WriteFile(path, b, 0o644)
}

// ReadLastScan loads the report.
func ReadLastScan(path string) (findings.Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return findings.Report{}, err
	}
	var r findings.Report
	if err := json.Unmarshal(b, &r); err != nil {
		return findings.Report{}, fmt.Errorf("corrupt last-scan.json: %w", err)
	}
	return r, nil
}
