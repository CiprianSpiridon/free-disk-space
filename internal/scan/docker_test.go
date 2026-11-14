package scan

import (
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestUntaggedImageUsedByContainerIsKeep(t *testing.T) {
	images := []byte(`[{"Id":"sha256:111","Size":27900000000,"RepoTags":[]},{"Id":"sha256:222","Size":27900000000,"RepoTags":[]}]`)
	containers := []byte(`[{"Image":"sha256:111","Name":"/litellm","State":{"Running":true}}]`)
	r := findings.NewReport("/u")
	if err := parseDockerInventory(images, containers, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Findings) != 2 {
		t.Fatal(r.Findings)
	}
	for _, f := range r.Findings {
		if f.Path == "docker-image:sha256:111" && (f.Risk != findings.RiskKeep || len(f.Containers) != 1 || f.Containers[0] != "litellm" || f.Reclaim != nil) {
			t.Fatal("container image offered for deletion", f)
		}
		if f.Path == "docker-image:sha256:222" && (f.Reclaim == nil || f.Reclaim.Kind != "docker-image" || strings.Contains(f.Reclaim.Cmd, "prune")) {
			t.Fatal("expected explicit image removal", f)
		}
	}
	r = findings.Normalize(r)
	if r.ReclaimTotal.Bytes != nil {
		t.Fatal("shared layers presented as exact host reclaim")
	}
}
