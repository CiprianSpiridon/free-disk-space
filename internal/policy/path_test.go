package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanDeleteUsrLocalVsUsrBin(t *testing.T) {
	if !CanDelete("/usr/local/Homebrew") {
		t.Fatal("expected /usr/local/Homebrew allowed")
	}
	if CanDelete("/usr/bin") {
		t.Fatal("expected /usr/bin refused")
	}
}

func TestCanDeleteSystemAndTmp(t *testing.T) {
	if CanDelete("/System/Volumes/Data/foo") {
		t.Fatal("expected system path refused")
	}
	if !CanDelete("/tmp/proj-foo") {
		t.Fatal("expected /tmp/proj-foo allowed")
	}
	if CanDelete("/tmp") {
		t.Fatal("expected /tmp refused")
	}
	if CanDelete("/private/tmp") {
		t.Fatal("expected /private/tmp refused")
	}
	if CanDelete("/private/var/tmp") {
		t.Fatal("expected /private/var/tmp refused")
	}
	if CanDelete("/var/vm") {
		t.Fatal("expected /var/vm refused")
	}
	if CanDelete("/private/var/vm/swapfile0") {
		t.Fatal("expected /private/var/vm child refused")
	}
}

func TestCanDeleteRelativeAndCargo(t *testing.T) {
	if CanDelete("foo") {
		t.Fatal("relative refused")
	}
	if CanDelete("/tmp/../etc") {
		t.Fatal("dotdot refused")
	}
	if CanDelete("/var/tmp") {
		t.Fatal("/var/tmp root refused")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if CanDelete(filepath.Join(home, ".cargo")) {
		t.Fatal("expected ~/.cargo refused")
	}
	if CanDelete(home) {
		t.Fatal("home refused")
	}
	if CanDelete("/") {
		t.Fatal("root refused")
	}
}

func TestCanDeleteIntermediateSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "keep")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(link, "Documents")
	if err := os.Mkdir(filepath.Join(target, "Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if CanDelete(nested) {
		t.Fatal("intermediate symlink parent must be refused")
	}
	if !CanDelete(filepath.Join(dir, "plain")) {
		t.Fatal("plain child under temp dir should be allowed")
	}
}

func TestCanDeleteTMPDIRRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	if CanDelete(dir) {
		t.Fatal("TMPDIR root refused")
	}
	if !CanDelete(filepath.Join(dir, "child")) {
		t.Fatal("TMPDIR child allowed")
	}
}
