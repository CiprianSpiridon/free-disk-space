package reclaim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestSimulatorNeverFallsBackToRemoveAll(t *testing.T) {
	p := filepath.Join(t.TempDir(), "device")
	if err := os.Mkdir(p, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	err := ApplyOne(findings.Finding{ID: "sim-AAA", Path: p, Category: "simulator-device", Risk: findings.RiskUnusedRuntime})
	if err == nil {
		t.Fatal("simulator was removed without its owner API")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("device path removed", err)
	}
}

func TestDeviceShutdownPrecedesOwnerDelete(t *testing.T) {
	old := Command
	defer func() { Command = old }()
	id := "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
	state := "Booted"
	deleted := false
	var calls []string
	Command = func(name string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		calls = append(calls, cmd)
		switch cmd {
		case "simctl list devices -j":
			if deleted {
				return []byte(`{"devices":{}}`), nil
			}
			return []byte(`{"devices":{"runtime":[{"udid":"` + id + `","state":"` + state + `"}]}}`), nil
		case "simctl shutdown " + id:
			state = "Shutdown"
			return nil, nil
		case "simctl delete " + id:
			if state != "Shutdown" {
				t.Fatal("deleted before shutdown")
			}
			deleted = true
			return nil, nil
		default:
			t.Fatalf("unexpected owner command: %s %s", name, cmd)
			return nil, nil
		}
	}
	p := filepath.Join(t.TempDir(), "device")
	os.Mkdir(p, 0755)
	err := ApplyOne(findings.Finding{ID: "device", Path: p, Category: "simulator-device", Risk: findings.RiskAsk, Reclaim: &findings.Reclaim{Kind: "simulator-device", Target: id}})
	if err != nil || !deleted {
		t.Fatal(err, calls)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("owner action fell back to filesystem removal")
	}
}

func TestRuntimeWaitsThroughDeletingForDiskImageUUID(t *testing.T) {
	old, oldPoll, oldTimeout := Command, PollInterval, WaitTimeout
	defer func() { Command, PollInterval, WaitTimeout = old, oldPoll, oldTimeout }()
	PollInterval = time.Millisecond
	WaitTimeout = time.Second
	id := "7ADE2658-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
	deleted := false
	polls := 0
	Command = func(name string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		if cmd == "simctl runtime delete "+id {
			deleted = true
			return nil, nil
		}
		if cmd != "simctl runtime list -j" {
			t.Fatal("wrong runtime command", name, cmd)
		}
		state := "Ready"
		if deleted {
			polls++
			state = "Deleting"
			if polls >= 3 {
				return []byte(`{}`), nil
			}
		}
		return []byte(`{"` + id + `":{"identifier":"` + id + `","state":"` + state + `"}}`), nil
	}
	err := ApplyOne(findings.Finding{ID: "runtime", Path: "simulator-runtime:" + id, Category: "simulator-runtime", Risk: findings.RiskUnusedRuntime, Reclaim: &findings.Reclaim{Kind: "simulator-runtime", Target: id}})
	if err != nil || !deleted || polls != 3 {
		t.Fatal("runtime completion not awaited", err, polls)
	}
}
