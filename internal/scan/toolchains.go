package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/rustusage"
)

func toolchainCmd(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if name == "rustup" {
		cmd.Env = append(os.Environ(), "RUSTUP_AUTO_INSTALL=0")
	}
	b, err := cmd.Output()
	return string(b), err
}

func init() {
	Register(Phase{Name: "toolchains", Quick: true, Dev: true, Run: runToolchains})
}

// RustupList is injected in tests.
var RustupList = func() (string, error) {
	if _, err := exec.LookPath("rustup"); err != nil {
		return "", err
	}
	return toolchainCmd("rustup", "toolchain", "list")
}

// WhichNode is injected in tests.
var WhichNode = func() string {
	p, err := exec.LookPath("node")
	if err != nil {
		return ""
	}
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		return rp
	}
	return p
}

// NodeVersion is injected in tests (e.g. "v25.8.1").
var NodeVersion = func() string {
	s, err := toolchainCmd("node", "-v")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func runToolchains(ctx *Context) error {
	classifyDeviceSupport(ctx)
	classifyRustup(ctx)
	classifyNvm(ctx)
	return nil
}

var reIOSVer = regexp.MustCompile(`(\d+)\.(\d+)`)

func parseDottedVersion(name string) (int, int, bool) {
	ms := reIOSVer.FindAllStringSubmatch(name, -1)
	if len(ms) == 0 {
		return 0, 0, false
	}
	m := ms[len(ms)-1]
	maj, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	return maj, min, true
}

func classifyDeviceSupport(ctx *Context) {
	groups := map[string][]int{}
	for i, f := range ctx.Report.Findings {
		dir := filepath.Dir(f.Path)
		if !strings.HasSuffix(dir, "DeviceSupport") {
			if strings.HasSuffix(f.Path, "DeviceSupport") {
				ctx.Report.Findings[i].Risk = findings.RiskAsk
			}
			continue
		}
		groups[dir] = append(groups[dir], i)
	}
	for _, idxs := range groups {
		bestI := -1
		bestMaj, bestMin := -1, -1
		for _, i := range idxs {
			maj, min, ok := parseDottedVersion(filepath.Base(ctx.Report.Findings[i].Path))
			if !ok {
				continue
			}
			if maj > bestMaj || (maj == bestMaj && min > bestMin) {
				bestMaj, bestMin, bestI = maj, min, i
			}
		}
		if bestI < 0 {
			continue
		}
		for _, i := range idxs {
			f := &ctx.Report.Findings[i]
			if i == bestI {
				f.Risk = findings.RiskKeep
				f.Why = "current DeviceSupport " + filepath.Base(f.Path)
				f.Reclaim = nil
				continue
			}
			f.Risk = findings.RiskUnusedRuntime
			f.Why = "older DeviceSupport; current is " + filepath.Base(ctx.Report.Findings[bestI].Path)
			f.Reclaim = &findings.Reclaim{Cmd: rmRf(f.Path)}
		}
	}
}

var RustupShow = func() (string, error) { return toolchainCmd("rustup", "show") }
var RustupOverrides = func() (string, error) { return toolchainCmd("rustup", "override", "list") }
var RustcVersion = func(name string) (string, error) { return toolchainCmd("rustup", "run", name, "rustc", "-vV") }

func classifyRustup(ctx *Context) {
	// Do not launch tools or inspect project roots when there are no Rust findings.
	have := false
	for _, f := range ctx.Report.Findings {
		if filepath.Base(filepath.Dir(f.Path)) == "toolchains" {
			have = true
		}
	}
	if !have {
		return
	}
	keep := rustupKeepNames()
	if keep == nil {
		keep = map[string]bool{}
	}
	show, showErr := RustupShow()
	overrides, overrideErr := RustupOverrides()
	for _, line := range strings.Split(show, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && (strings.Contains(line, "(default)") || strings.Contains(line, "(active")) {
			keep[fields[0]] = true
		}
		if len(fields) == 2 && fields[0] == "name:" {
			keep[fields[1]] = true
		}
	}
	for _, line := range strings.Split(overrides, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] != "no" {
			keep[fields[len(fields)-1]] = true
		}
	}
	if env := os.Getenv("RUSTUP_TOOLCHAIN"); env != "" {
		keep[env] = true
	}
	roots := discoverWorkRoots(ctx.Catalog, ctx.Home)
	refs, complete := map[string]string{}, true
	if ctx.Sizer != nil {
		if files, ok := ctx.Sizer.PinFiles(roots); ok {
			refs, complete = rustusage.ReadPins(files)
		} else {
			refs, complete = rustusage.Collect(roots)
		}
	} else {
		refs, complete = rustusage.Collect(roots)
	}
	versions := map[string]bool{}
	for name := range keep {
		if v, err := RustcVersion(name); err == nil && v != "" {
			versions[v] = true
		}
	}
	for i := range ctx.Report.Findings {
		f := &ctx.Report.Findings[i]
		name := filepath.Base(f.Path)
		if filepath.Base(filepath.Dir(f.Path)) != "toolchains" {
			continue
		}
		protected, reason := false, ""
		for k := range keep {
			if rustusage.Matches(name, k) {
				protected = true
				reason = "active/default/overridden rustup toolchain"
			}
		}
		for pin, path := range refs {
			if rustusage.Matches(name, pin) {
				protected = true
				reason = "pinned by " + path
			}
		}
		if protected {
			f.Risk = findings.RiskKeep
			f.Why = reason
			f.Reclaim = nil
			continue
		}
		if !complete || showErr != nil || overrideErr != nil {
			f.Risk = findings.RiskAsk
			f.Why = "rustup usage could not be fully checked"
			f.Reclaim = nil
			f.SummaryOnly = true
			continue
		}
		f.Risk = findings.RiskUnusedRuntime
		f.Why = "no active selection, override or project pin in scanned roots"
		if v, err := RustcVersion(name); err == nil && versions[v] {
			f.Why = "duplicate of the active compiler; active directory is kept"
		}
		f.Reclaim = &findings.Reclaim{Kind: "rustup-toolchain", Target: name, Roots: roots, Cmd: "rustup toolchain uninstall " + shellQuote(name)}
	}
}

func rustupKeepNames() map[string]bool {
	out := map[string]bool{}
	s, err := RustupList()
	if err != nil || strings.TrimSpace(s) == "" {
		return nil
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, _, _ := strings.Cut(line, " ")
		if strings.Contains(line, "(default)") || strings.Contains(line, "(active") {
			out[name] = true
		}
	}
	return out
}

func rustupKeep(base string, listed map[string]bool) bool {
	if listed == nil {
		return strings.Contains(base, "stable")
	}
	if listed[base] {
		return true
	}
	for n := range listed {
		if strings.HasPrefix(base, n) || strings.HasPrefix(n, base) {
			return true
		}
	}
	return false
}

func rustupUninstallName(base string) string {
	if i := strings.Index(base, "-"); i > 0 {
		head := base[:i]
		if head == "stable" || head == "nightly" || head == "beta" {
			return head
		}
		if _, err := strconv.Atoi(head[:1]); err == nil {
			return head
		}
	}
	return base
}

func classifyNvm(ctx *Context) {
	nodePath := WhichNode()
	nodeVer := strings.TrimPrefix(NodeVersion(), "v")
	nvmCurrent := strings.Contains(nodePath, string(filepath.Separator)+".nvm"+string(filepath.Separator))
	for i := range ctx.Report.Findings {
		f := &ctx.Report.Findings[i]
		if f.Category == "nvm" || f.Category == "nvm-versions" {
			if f.Risk == findings.RiskUnusedRuntime {
				f.Risk = findings.RiskAsk
			}
		}
		parent := filepath.Base(filepath.Dir(f.Path))
		grand := filepath.Base(filepath.Dir(filepath.Dir(f.Path)))
		if parent != "node" || grand != "versions" {
			continue
		}
		if !strings.Contains(f.Path, string(filepath.Separator)+".nvm"+string(filepath.Separator)) {
			continue
		}
		ver := strings.TrimPrefix(filepath.Base(f.Path), "v")
		if nvmCurrent && nodeVer != "" && (ver == nodeVer || strings.HasPrefix(nodeVer, ver+".") || strings.HasPrefix(ver, nodeVer)) {
			f.Risk = findings.RiskKeep
			f.Why = "nvm current " + filepath.Base(f.Path)
			f.Reclaim = nil
			continue
		}
		f.Risk = findings.RiskUnusedRuntime
		if nvmCurrent {
			f.Why = "extra nvm version; current " + nodeVer
		} else {
			f.Why = "nvm version unused; node is " + nodePath
		}
		f.Reclaim = &findings.Reclaim{Cmd: "nvm uninstall " + ver}
	}
}
