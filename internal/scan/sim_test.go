package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestSimJoinRuntimeIdentifier(t *testing.T) {
	raw := []byte(`{
  "devices": {
    "iOS 26.4": [
      {"udid":"AAA","name":"iPhone","state":"Shutdown","runtimeIdentifier":"com.apple.CoreSimulator.SimRuntime.iOS-26-4-1"}
    ]
  }
}`)
	rep := findings.NewReport("/u")
	if _, err := ParseSimctlDevices(raw, &rep, "/u"); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("%+v", rep.Findings)
	}
	if !strings.Contains(rep.Findings[0].Why, "com.apple.CoreSimulator.SimRuntime.iOS-26-4-1") {
		t.Fatal(rep.Findings[0].Why)
	}
	if strings.Contains(rep.Findings[0].Why, "orphan") {
		t.Fatal("name mismatch should not orphan")
	}
	want := filepath.Join("/u", "Library", "Developer", "CoreSimulator", "Devices", "AAA")
	if rep.Findings[0].Path != want {
		t.Fatalf("path=%s", rep.Findings[0].Path)
	}
}

func TestSimRuntimeUnusedWhenNeverBooted(t *testing.T) {
	rep := findings.NewReport("/u")
	booted, err := ParseSimctlDevices([]byte(`{
  "devices": {
    "iOS 18": [
      {"udid":"AAA","name":"iPhone","state":"Shutdown","runtimeIdentifier":"rt-used","lastBootedAt":"2026-01-01"},
      {"udid":"BBB","name":"iPhone","state":"Shutdown","runtimeIdentifier":"rt-never"}
    ]
  }
}`), &rep, "/u")
	if err != nil {
		t.Fatal(err)
	}
	ParseSimctlRuntimes([]byte(`{
  "runtimes": [
    {"identifier":"rt-used","name":"iOS 18"},
    {"identifier":"rt-never","name":"iOS 17"}
  ]
}`), &rep, booted, "")
	var used, never *findings.Finding
	for i := range rep.Findings {
		f := &rep.Findings[i]
		if f.Category != "simulator-runtime" {
			continue
		}
		if f.Path == "rt-used" {
			used = f
		}
		if f.Path == "rt-never" {
			never = f
		}
	}
	if used == nil || never == nil {
		t.Fatalf("%+v", rep.Findings)
	}
	if used.Risk != findings.RiskAsk {
		t.Fatalf("used risk %s", used.Risk)
	}
	if never.Risk != findings.RiskUnusedRuntime {
		t.Fatalf("never risk %s", never.Risk)
	}
}

func TestSimRuntimeSizesVolume(t *testing.T) {
	vol := t.TempDir()
	img := filepath.Join(vol, "watchOS_23S303")
	_ = os.Mkdir(img, 0o755)
	_ = os.WriteFile(filepath.Join(img, "blob"), make([]byte, 32<<10), 0o644)
	rep := findings.NewReport("/u")
	ParseSimctlRuntimes([]byte(`{
  "runtimes": [
    {"identifier":"com.apple.CoreSimulator.SimRuntime.watchOS-26-2","name":"watchOS 26.2","buildversion":"23S303"}
  ]
}`), &rep, map[string]bool{}, vol)
	if len(rep.Findings) != 1 {
		t.Fatalf("%+v", rep.Findings)
	}
	f := rep.Findings[0]
	if f.Path != img {
		t.Fatalf("path %s", f.Path)
	}
	if f.Bytes <= 0 {
		t.Fatal("expected volume bytes")
	}
	if f.Risk != findings.RiskUnusedRuntime {
		t.Fatal(f.Risk)
	}
}

func TestAndroidImagesWithZeroAVDs(t *testing.T) {
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("ANDROID_HOME", "")
	home := t.TempDir()
	img := filepath.Join(home, "sdk", "system-images")
	_ = os.MkdirAll(img, 0o755)
	_ = os.WriteFile(filepath.Join(img, "x"), []byte("img"), 0o644)
	cat := &catalog.Catalog{
		Android: []catalog.Entry{
			{Path: img, Category: "android-system-images", Risk: "unused-runtime"},
			{Path: filepath.Join(home, "no-avds"), Category: "android-avds", Risk: "unused-runtime"},
		},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Catalog: cat, Home: home, Report: &rep}
	if err := runAndroidSim(ctx); err != nil {
		t.Fatal(err)
	}
	saw := false
	for _, f := range rep.Findings {
		if f.Category == "android-system-images" {
			saw = true
			if f.Risk != findings.RiskUnusedRuntime {
				t.Fatal(f.Risk)
			}
		}
	}
	if !saw {
		t.Fatal("missing image finding")
	}
	ok := false
	for _, n := range rep.NotPresent {
		if n == "android AVDs" {
			ok = true
		}
	}
	if !ok {
		t.Fatal(rep.NotPresent)
	}
}

func TestAndroidSDKRootEnv(t *testing.T) {
	home := t.TempDir()
	sdk := filepath.Join(home, "sdk")
	img := filepath.Join(sdk, "system-images")
	_ = os.MkdirAll(img, 0o755)
	_ = os.WriteFile(filepath.Join(img, "x"), []byte("img"), 0o644)
	t.Setenv("ANDROID_SDK_ROOT", sdk)
	t.Setenv("ANDROID_HOME", "")
	rep := findings.NewReport(home)
	ctx := &Context{Catalog: &catalog.Catalog{}, Home: home, Report: &rep}
	if err := runAndroidSim(ctx); err != nil {
		t.Fatal(err)
	}
	saw := false
	for _, f := range rep.Findings {
		if f.Path == img {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected ANDROID_SDK_ROOT image: %+v", rep.Findings)
	}
}

func TestSimctlBadJSONNotPresent(t *testing.T) {
	old := SimctlJSON
	SimctlJSON = func() ([]byte, error) { return []byte("not-json"), nil }
	defer func() { SimctlJSON = old }()
	rep := findings.NewReport("/u")
	ctx := &Context{Report: &rep, Catalog: &catalog.Catalog{}, Home: "/u"}
	if err := runAppleSim(ctx); err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, n := range rep.NotPresent {
		if n == "simctl-json" || n == "simctl" {
			ok = true
		}
	}
	if !ok && len(rep.Findings) == 0 {
		// xcrun missing → simctl not_present; bad json → simctl-json
		for _, n := range rep.NotPresent {
			if n == "simctl" {
				return
			}
		}
		t.Fatalf("not_present=%v", rep.NotPresent)
	}
}

func TestAppleSimMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	rep := findings.NewReport("/u")
	ctx := &Context{Report: &rep, Catalog: &catalog.Catalog{}}
	if err := runAppleSim(ctx); err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, n := range rep.NotPresent {
		if n == "simctl" {
			ok = true
		}
	}
	if !ok {
		t.Fatal("expected simctl not_present")
	}
}
