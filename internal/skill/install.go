package skill

import (
	"os"

	skilldata "github.com/CiprianSpiridon/free-disk-space/skill"
)

// Markdown is the bundled SKILL.md.
func Markdown() []byte {
	return skilldata.Markdown
}

// Result is one install attempt.
type Result struct {
	Tool    string `json:"tool"`
	Path    string `json:"path,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Error   string `json:"error,omitempty"`
}

// List returns what would be installed vs skipped.
func List(home string, targets []Target) []Result {
	if targets == nil {
		targets = DefaultTargets
	}
	var out []Result
	for _, t := range targets {
		if !t.Available(home) {
			out = append(out, Result{Tool: t.Name, Skipped: true, Reason: "cli not installed"})
			continue
		}
		out = append(out, Result{Tool: t.Name, Path: t.SkillFile(home)})
	}
	return out
}

// Install writes SKILL.md into every available CLI skill directory.
func Install(home string, targets []Target) []Result {
	if targets == nil {
		targets = DefaultTargets
	}
	body := Markdown()
	var out []Result
	for _, t := range targets {
		if !t.Available(home) {
			out = append(out, Result{Tool: t.Name, Skipped: true, Reason: "cli not installed"})
			continue
		}
		dir := t.SkillDir(home)
		file := t.SkillFile(home)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			out = append(out, Result{Tool: t.Name, Path: file, Error: err.Error()})
			continue
		}
		if err := os.WriteFile(file, body, 0o644); err != nil {
			out = append(out, Result{Tool: t.Name, Path: file, Error: err.Error()})
			continue
		}
		out = append(out, Result{Tool: t.Name, Path: file})
	}
	return out
}

// Uninstall removes the skill dir from available CLIs.
func Uninstall(home string, targets []Target) []Result {
	if targets == nil {
		targets = DefaultTargets
	}
	var out []Result
	for _, t := range targets {
		if !t.Available(home) {
			out = append(out, Result{Tool: t.Name, Skipped: true, Reason: "cli not installed"})
			continue
		}
		dir := t.SkillDir(home)
		if err := os.RemoveAll(dir); err != nil {
			out = append(out, Result{Tool: t.Name, Path: dir, Error: err.Error()})
			continue
		}
		out = append(out, Result{Tool: t.Name, Path: dir})
	}
	return out
}
