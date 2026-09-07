package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestClassifyDeviceSupportKeepsNewest(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "Library", "Developer", "Xcode", "iOS DeviceSupport")
	old := filepath.Join(root, "iPhone16,1 18.4 (22E240)")
	cur := filepath.Join(root, "iPhone17,2 26.4 (23E246)")
	_ = os.MkdirAll(old, 0o755)
	_ = os.MkdirAll(cur, 0o755)
	rep := findings.NewReport(home)
	rep.Findings = []findings.Finding{
		{ID: "ds", Path: root, Category: "xcode-devicesupport", Risk: findings.RiskAsk},
		{ID: "old", Path: old, Category: "xcode-devicesupport-child", Risk: findings.RiskAsk},
		{ID: "cur", Path: cur, Category: "xcode-devicesupport-child", Risk: findings.RiskAsk},
	}
	ctx := &Context{Catalog: &catalog.Catalog{}, Home: home, Report: &rep}
	classifyDeviceSupport(ctx)
	var oldF, curF *findings.Finding
	for i := range rep.Findings {
		switch rep.Findings[i].Path {
		case old:
			oldF = &rep.Findings[i]
		case cur:
			curF = &rep.Findings[i]
		}
	}
	if curF == nil || curF.Risk != findings.RiskKeep {
		t.Fatalf("current should be keep: %+v", curF)
	}
	if oldF == nil || oldF.Risk != findings.RiskUnusedRuntime {
		t.Fatalf("old should be unused: %+v", oldF)
	}
	if oldF.Reclaim == nil || !strings.Contains(oldF.Reclaim.Cmd, old) {
		t.Fatalf("old reclaim %v", oldF.Reclaim)
	}
}

func TestClassifyRustupDefaultVsExtra(t *testing.T) {
	old := RustupList
	RustupList = func() (string, error) {
		return "stable-aarch64-apple-darwin (default)\nnightly-aarch64-apple-darwin\n1.94.0-aarch64-apple-darwin\n", nil
	}
	defer func() { RustupList = old }()
	home := t.TempDir()
	tc := filepath.Join(home, ".rustup", "toolchains")
	stable := filepath.Join(tc, "stable-aarch64-apple-darwin")
	night := filepath.Join(tc, "nightly-aarch64-apple-darwin")
	rep := findings.NewReport(home)
	rep.Findings = []findings.Finding{
		{Path: stable, Category: "rust-toolchains-child", Risk: findings.RiskAsk},
		{Path: night, Category: "rust-toolchains-child", Risk: findings.RiskAsk},
	}
	ctx := &Context{Catalog: &catalog.Catalog{}, Home: home, Report: &rep}
	classifyRustup(ctx)
	for _, f := range rep.Findings {
		switch f.Path {
		case stable:
			if f.Risk != findings.RiskKeep {
				t.Fatalf("stable %s", f.Risk)
			}
		case night:
			if f.Risk != findings.RiskUnusedRuntime {
				t.Fatalf("nightly %s", f.Risk)
			}
			if f.Reclaim == nil || !strings.Contains(f.Reclaim.Cmd, "rustup toolchain uninstall") {
				t.Fatalf("nightly cmd %v", f.Reclaim)
			}
		}
	}
}

func TestClassifyNvmHomebrewNodeMarksAllUnused(t *testing.T) {
	oldW, oldV := WhichNode, NodeVersion
	WhichNode = func() string { return "/opt/homebrew/bin/node" }
	NodeVersion = func() string { return "v25.8.1" }
	defer func() { WhichNode, NodeVersion = oldW, oldV }()
	home := t.TempDir()
	v18 := filepath.Join(home, ".nvm", "versions", "node", "v18.20.8")
	rep := findings.NewReport(home)
	rep.Findings = []findings.Finding{
		{Path: v18, Category: "nvm-versions-child", Risk: findings.RiskAsk},
	}
	ctx := &Context{Catalog: &catalog.Catalog{}, Home: home, Report: &rep}
	classifyNvm(ctx)
	f := rep.Findings[0]
	if f.Risk != findings.RiskUnusedRuntime {
		t.Fatal(f.Risk)
	}
	if f.Reclaim == nil || !strings.Contains(f.Reclaim.Cmd, "nvm uninstall 18.20.8") {
		t.Fatalf("%v", f.Reclaim)
	}
}
