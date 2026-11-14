package reclaim

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/rustusage"
	"github.com/CiprianSpiridon/free-disk-space/internal/simulator"
)

// Command is injectable; commands are fixed argument arrays, never report shells.
var Command = func(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if name == "rustup" {
		cmd.Env = append(os.Environ(), "RUSTUP_AUTO_INSTALL=0")
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		return b, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return b, nil
}
var WaitTimeout = 2 * time.Minute
var PollInterval = time.Second
var uuid = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)

func applyManaged(f findings.Finding) error {
	a := f.Reclaim
	switch a.Kind {
	case "simulator-device", "simulator-runtime":
		if !uuid.MatchString(a.Target) {
			return fmt.Errorf("invalid simulator UUID; rescan required: %s", a.Target)
		}
		if a.Kind == "simulator-device" {
			return deleteDevice(a.Target)
		}
		return deleteRuntime(a.Target)
	case "rustup-toolchain":
		if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]+$`).MatchString(a.Target) {
			return fmt.Errorf("invalid rustup toolchain")
		}
		show, err := Command("rustup", "show")
		if err != nil {
			return err
		}
		overrides, err := Command("rustup", "override", "list")
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(show), "\n") {
			if strings.Contains(line, a.Target) && (strings.Contains(line, "(default)") || strings.Contains(line, "(active") || strings.Contains(line, "name:")) {
				return fmt.Errorf("refused active/default rustup toolchain %s", a.Target)
			}
		}
		if strings.Contains(string(overrides), a.Target) {
			return fmt.Errorf("refused overridden rustup toolchain %s", a.Target)
		}
		pins, complete := rustusage.Collect(a.Roots)
		if !complete {
			return fmt.Errorf("rustup project pins could not be rechecked")
		}
		for pin, path := range pins {
			if rustusage.Matches(a.Target, pin) {
				return fmt.Errorf("refused rustup toolchain pinned by %s", path)
			}
		}
		_, err = Command("rustup", "toolchain", "uninstall", a.Target)
		return err
	case "docker-image":
		if !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(a.Target) {
			return fmt.Errorf("invalid Docker image id")
		}
		// Docker performs the final reference check atomically; never force removal.
		_, err := Command("docker", "image", "rm", a.Target)
		return err
	default:
		return fmt.Errorf("unknown owner action %q", a.Kind)
	}
}

func deviceState(id string) (string, bool, error) {
	b, err := Command("xcrun", "simctl", "list", "devices", "-j")
	if err != nil {
		return "", false, err
	}
	var doc struct {
		Devices map[string][]struct{ UDID, State string } `json:"devices"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return "", false, err
	}
	if doc.Devices == nil {
		return "", false, fmt.Errorf("invalid simctl devices response")
	}
	for _, devices := range doc.Devices {
		for _, d := range devices {
			if d.UDID == id {
				return d.State, true, nil
			}
		}
	}
	return "", false, nil
}

func waitGone(check func() (bool, error)) error {
	deadline := time.Now().Add(WaitTimeout)
	for {
		present, err := check()
		if err != nil {
			return err
		}
		if !present {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("deletion still pending after %s; not confirmed complete", WaitTimeout)
		}
		time.Sleep(PollInterval)
	}
}

func deleteDevice(id string) error {
	state, present, err := deviceState(id)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	if state != "Shutdown" {
		if _, err := Command("xcrun", "simctl", "shutdown", id); err != nil {
			return err
		}
		if err := waitGone(func() (bool, error) { s, present, err := deviceState(id); return present && s != "Shutdown", err }); err != nil {
			return err
		}
	}
	if _, err := Command("xcrun", "simctl", "delete", id); err != nil {
		return err
	}
	return waitGone(func() (bool, error) { _, present, err := deviceState(id); return present, err })
}

func runtimeState(id string) (string, bool, error) {
	b, err := Command("xcrun", "simctl", "runtime", "list", "-j")
	if err != nil {
		return "", false, err
	}
	images, err := simulator.ParseImages(b)
	if err != nil {
		return "", false, err
	}
	for _, img := range images {
		if img.Identifier == id {
			return img.State, true, nil
		}
	}
	return "", false, nil
}

func deleteRuntime(id string) error {
	state, present, err := runtimeState(id)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	if state != "Deleting" {
		if _, err := Command("xcrun", "simctl", "runtime", "delete", id); err != nil {
			return err
		}
	}
	return waitGone(func() (bool, error) { _, present, err := runtimeState(id); return present, err })
}
