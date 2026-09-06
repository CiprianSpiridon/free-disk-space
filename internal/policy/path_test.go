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
}
