package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/simulator"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func TestRuntimeWithoutDiskImageMappingCannotOfferDelete(t *testing.T) {
	r := findings.NewReport("/u")
	ParseSimctlRuntimes([]byte(`{"runtimes":[{"identifier":"com.apple.CoreSimulator.SimRuntime.iOS-26-2","name":"iOS 26.2","buildversion":"23C54"}]}`), &r, nil, "")
	if len(r.Findings) != 1 {
		t.Fatal(r.Findings)
	}
	if r.Findings[0].Reclaim != nil {
		t.Fatal("runtime identifier is not a disk image delete id", r.Findings[0])
	}
}

func TestCriticalQuickScanIncludesSimulatorPhase(t *testing.T) {
	phase := Phase{Name: "apple-sim"}
	for _, used := range []int64{94, 95, 99} {
		r := findings.NewReport("/u")
		r.Volume = findings.Volume{ContainerBytes: 100, InUseBytes: used}
		ctx := &Context{Mode: "quick", Catalog: &catalog.Catalog{}, Report: &r}
		if got := wantPhase(ctx, phase); got != (used >= 95) {
			t.Fatalf("%d%% full: apple-sim=%v", used, got)
		}
	}
}

func TestRuntimeDeleteUsesDiskImageIDAndBackingFile(t *testing.T) {
	root := t.TempDir()
	image := filepath.Join(root, "image.dmg")
	os.WriteFile(image, make([]byte, 32768), 0644)
	id := "7ADE2658-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
	r := findings.NewReport(root)
	images, err := simulator.ParseImages([]byte(`{"` + id + `":{"identifier":"` + id + `","runtimeIdentifier":"rt-ios","build":"23C54","path":"` + image + `","state":"Ready"}}`))
	if err != nil {
		t.Fatal(err)
	}
	parseSimctlRuntimes([]byte(`{"runtimes":[{"identifier":"rt-ios","buildversion":"23C54","name":"iOS 26.2"}]}`), &r, map[string]bool{}, "", images, size.NewScanner().Of)
	if len(r.Findings) != 1 || r.Findings[0].Path != image || r.Findings[0].Reclaim == nil || r.Findings[0].Reclaim.Target != id {
		t.Fatal(r.Findings)
	}
}

func TestBootedDeviceIsNotNeverBooted(t *testing.T) {
	r := findings.NewReport("/u")
	_, err := ParseSimctlDevices([]byte(`{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-26-2":[{"udid":"AAA","name":"iPhone 17 Pro Max","state":"Booted"}]}}`), &r, "/u")
	if err != nil {
		t.Fatal(err)
	}
	if r.Findings[0].Risk == findings.RiskUnusedRuntime {
		t.Fatal("booted device classified as never used")
	}
}

func TestPinnedRustToolchainIsKept(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "work", "hermes", "buzz-cli")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "rust-toolchain.toml"), []byte("[toolchain]\nchannel = \"1.95.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	old := RustupList
	RustupList = func() (string, error) {
		return "stable-aarch64-apple-darwin (active, default)\n1.95.0-aarch64-apple-darwin\n", nil
	}
	defer func() { RustupList = old }()
	p := filepath.Join(home, ".rustup", "toolchains", "1.95.0-aarch64-apple-darwin")
	r := findings.NewReport(home)
	r.Findings = []findings.Finding{{ID: "pinned", Path: p, Category: "rust-toolchains-child", Risk: findings.RiskAsk}}
	ctx := &Context{Home: home, Report: &r, Catalog: &catalog.Catalog{WorkRoots: []catalog.Entry{{Path: filepath.Join(home, "work")}}}}
	classifyRustup(ctx)
	if r.Findings[0].Risk != findings.RiskKeep {
		t.Fatal("project pin ignored", r.Findings[0])
	}
}
