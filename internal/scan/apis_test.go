package scan

import (
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestAPIsDockerAndBrew(t *testing.T) {
	oldD, oldB := DockerSystemDF, BrewAutoremoveDry
	DockerSystemDF = func() (string, error) {
		return "TYPE  TOTAL  ACTIVE  SIZE  RECLAIMABLE\nImages  2  1  8GB  3GB\n", nil
	}
	BrewAutoremoveDry = func() (string, error) {
		return "Would uninstall:\nfoo\n", nil
	}
	defer func() { DockerSystemDF, BrewAutoremoveDry = oldD, oldB }()
	rep := findings.NewReport(t.TempDir())
	ctx := &Context{Catalog: &catalog.Catalog{}, Report: &rep, Home: t.TempDir()}
	if err := runAPIs(ctx); err != nil {
		t.Fatal(err)
	}
	var docker, brew bool
	for _, f := range rep.Findings {
		if f.ID == "docker-system-df" {
			docker = true
			if f.Reclaim == nil || !strings.Contains(f.Reclaim.Cmd, "docker system prune") {
				t.Fatalf("%v", f.Reclaim)
			}
		}
		if f.ID == "brew-autoremove" {
			brew = true
		}
	}
	if !docker || !brew {
		t.Fatalf("docker=%v brew=%v %+v", docker, brew, rep.Findings)
	}
}

func TestAPIsSkipEmptyBrew(t *testing.T) {
	oldD, oldB := DockerSystemDF, BrewAutoremoveDry
	DockerSystemDF = func() (string, error) { return "", nil }
	BrewAutoremoveDry = func() (string, error) { return "Nothing to uninstall\n", nil }
	defer func() { DockerSystemDF, BrewAutoremoveDry = oldD, oldB }()
	rep := findings.NewReport(t.TempDir())
	ctx := &Context{Catalog: &catalog.Catalog{}, Report: &rep}
	_ = runAPIs(ctx)
	if len(rep.Findings) != 0 {
		t.Fatalf("%+v", rep.Findings)
	}
}
