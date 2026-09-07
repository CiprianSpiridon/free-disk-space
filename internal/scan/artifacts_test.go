package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestArtifactsNodeModulesAndNestedAndQuick(t *testing.T) {
	home := t.TempDir()
	app := filepath.Join(home, "work", "app")
	nm := filepath.Join(app, "node_modules")
	nested := filepath.Join(nm, "pkg", "node_modules")
	_ = os.MkdirAll(nested, 0o755)
	_ = os.WriteFile(filepath.Join(nm, "x"), []byte("1"), 0o644)
	ex := filepath.Join(home, "work", "app", "examples", "foo")
	_ = os.MkdirAll(ex, 0o755)
	_ = os.WriteFile(filepath.Join(ex, ".tsbuildinfo"), []byte("x"), 0o644)
	_ = os.Mkdir(filepath.Join(ex, ".tsx-cache"), 0o755)
	cat := &catalog.Catalog{
		WorkRoots: []catalog.Entry{{Path: filepath.Join(home, "work"), WalkArtifacts: true, Scans: []string{"dev", "full"}}},
		Artifacts: []catalog.Artifact{
			{Name: "node_modules", Ecosystem: "node", Risk: "rebuildable"},
			{Name: ".tsx-cache", Ecosystem: "tsx", Risk: "safe-cache"},
			{Name: ".tsbuildinfo", Ecosystem: "tsc", Risk: "safe-cache", File: true},
		},
		WalkPrune:  []string{"node_modules"},
		Thresholds: catalog.Thresholds{MaxWalkDepth: 8, ArtifactIdleDays: 30},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Mode: "dev", Catalog: cat, Home: home, Report: &rep}
	if err := runArtifacts(ctx); err != nil {
		t.Fatal(err)
	}
	var nmCount, tsx, tsb int
	for _, f := range rep.Findings {
		if f.Path == nm {
			nmCount++
		}
		if f.Path == nested {
			t.Fatal("nested node_modules walked")
		}
		if filepath.Base(f.Path) == ".tsx-cache" {
			tsx++
		}
		if filepath.Base(f.Path) == ".tsbuildinfo" {
			tsb++
		}
	}
	if nmCount != 1 {
		t.Fatalf("node_modules count %d %+v", nmCount, rep.Findings)
	}
	if tsx < 1 || tsb < 1 {
		t.Fatalf("tsx=%d tsb=%d", tsx, tsb)
	}
}

func TestArtifactsVendorComposerVsRust(t *testing.T) {
	home := t.TempDir()
	php := filepath.Join(home, "work", "phpapp")
	_ = os.MkdirAll(filepath.Join(php, "vendor"), 0o755)
	_ = os.WriteFile(filepath.Join(php, "composer.json"), []byte("{}"), 0o644)
	rustv := filepath.Join(home, "work", "crate", "vendor", "paste")
	_ = os.MkdirAll(filepath.Join(rustv, "src"), 0o755)
	_ = os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(rustv)), "Cargo.toml"), []byte("[package]"), 0o644)
	_ = os.MkdirAll(filepath.Join(rustv, "target"), 0o755)
	_ = os.WriteFile(filepath.Join(rustv, "Cargo.toml"), []byte("[package]"), 0o644)
	cat := &catalog.Catalog{
		WorkRoots:  []catalog.Entry{{Path: filepath.Join(home, "work"), WalkArtifacts: true}},
		Artifacts:  []catalog.Artifact{{Name: "vendor", Ecosystem: "composer-or-go", Risk: "rebuildable"}, {Name: "target", Ecosystem: "rust", Risk: "rebuildable"}},
		Thresholds: catalog.Thresholds{MaxWalkDepth: 8},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Catalog: cat, Home: home, Report: &rep}
	_ = runArtifacts(ctx)
	var composer, rustSrc, rustTarget bool
	for _, f := range rep.Findings {
		if f.Path == filepath.Join(php, "vendor") {
			composer = true
		}
		if f.Path == filepath.Join(rustv, "src") {
			rustSrc = true
		}
		if f.Path == filepath.Join(rustv, "target") {
			rustTarget = true
		}
	}
	if !composer {
		t.Fatal("composer vendor")
	}
	if rustSrc {
		t.Fatal("rust vendor src should not be finding")
	}
	if !rustTarget {
		t.Fatal("rust vendor/paste/target")
	}
}

func TestArtifactsGitTrackedAndEnv(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "work", "r")
	_ = os.MkdirAll(repo, 0o755)
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init: %s %v", out, err)
	}
	_ = os.WriteFile(filepath.Join(repo, "dist"), []byte("x"), 0o644)
	_ = exec.Command("git", "-C", repo, "add", "dist").Run()
	_ = exec.Command("git", "-C", repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "x").Run()
	_ = os.Mkdir(filepath.Join(repo, "env"), 0o755)
	cat := &catalog.Catalog{
		WorkRoots: []catalog.Entry{{Path: filepath.Join(home, "work"), WalkArtifacts: true}},
		Artifacts: []catalog.Artifact{
			{Name: "dist", Ecosystem: "js-build", Risk: "rebuildable"},
			{Name: "env", Ecosystem: "python", Risk: "rebuildable"},
		},
		Thresholds: catalog.Thresholds{MaxWalkDepth: 8, ArtifactIdleDays: 30},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Catalog: cat, Home: home, Report: &rep}
	_ = runArtifacts(ctx)
	for _, f := range rep.Findings {
		if filepath.Base(f.Path) == "env" {
			t.Fatal("env without pyvenv.cfg")
		}
		if filepath.Base(f.Path) == "dist" && f.Risk != findings.RiskKeep {
			t.Fatalf("tracked dist risk %s", f.Risk)
		}
	}
}

func TestDiscoverTwoGitRepos(t *testing.T) {
	home := t.TempDir()
	acme := filepath.Join(home, "work_acme")
	a := filepath.Join(acme, "a")
	b := filepath.Join(acme, "b")
	_ = os.MkdirAll(filepath.Join(a, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(b, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(a, "node_modules"), 0o755)
	cat := &catalog.Catalog{
		WorkRootDiscover: catalog.WorkRootDiscover{Enabled: true, MinChildRepos: 2, SkipNames: []string{"Library"}},
		Artifacts:        []catalog.Artifact{{Name: "node_modules", Ecosystem: "node", Risk: "rebuildable"}},
		WalkPrune:        []string{"node_modules"},
		Thresholds:       catalog.Thresholds{MaxWalkDepth: 8},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Catalog: cat, Home: home, Report: &rep}
	_ = runArtifacts(ctx)
	ok := false
	for _, f := range rep.Findings {
		if filepath.Base(f.Path) == "node_modules" {
			ok = true
		}
	}
	if !ok {
		t.Fatal("discover walk missed node_modules")
	}
}

func TestArtifactsSkipsWorkRootWithoutWalkFlag(t *testing.T) {
	home := t.TempDir()
	goRoot := filepath.Join(home, "go", "pkg")
	_ = os.MkdirAll(filepath.Join(goRoot, "node_modules"), 0o755)
	cat := &catalog.Catalog{
		WorkRoots:  []catalog.Entry{{Path: filepath.Join(home, "go"), WalkArtifacts: false}},
		Artifacts:  []catalog.Artifact{{Name: "node_modules", Ecosystem: "node", Risk: "rebuildable"}},
		Thresholds: catalog.Thresholds{MaxWalkDepth: 8},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Catalog: cat, Home: home, Report: &rep}
	_ = runArtifacts(ctx)
	for _, f := range rep.Findings {
		if filepath.Base(f.Path) == "node_modules" {
			t.Fatal("walked work root with walk_artifacts false")
		}
	}
}

func TestArtifactsCopyBackupDir(t *testing.T) {
	home := t.TempDir()
	cp := filepath.Join(home, "work", "ulpi-v4 copy")
	bk := filepath.Join(home, "work", "ulpi-v4-bk-1")
	if err := os.MkdirAll(cp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cp, "blob"), make([]byte, 6<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bk, "blob"), make([]byte, 6<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{
		WorkRoots:  []catalog.Entry{{Path: filepath.Join(home, "work"), WalkArtifacts: true, Scans: []string{"dev", "full"}}},
		Thresholds: catalog.Thresholds{MaxWalkDepth: 8, ArtifactListMinBytes: 5 << 20},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Mode: "dev", Catalog: cat, Home: home, Report: &rep}
	if err := runArtifacts(ctx); err != nil {
		t.Fatal(err)
	}
	var sawCopy, sawBk bool
	for _, f := range rep.Findings {
		if f.Path == cp && f.Category == "copy-backup" && f.Risk == findings.RiskAsk {
			sawCopy = true
			if f.Reclaim == nil || !strings.Contains(f.Reclaim.Cmd, "rm -rf") {
				t.Fatalf("%v", f.Reclaim)
			}
		}
		if f.Path == bk && f.Category == "copy-backup" {
			sawBk = true
		}
	}
	if !sawCopy || !sawBk {
		t.Fatalf("copy=%v bk=%v %+v", sawCopy, sawBk, rep.Findings)
	}
}
