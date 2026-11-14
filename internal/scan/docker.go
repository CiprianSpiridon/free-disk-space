package scan

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

// Inspect objects in batches. Layer-heavy `system df --verbose` can take
// minutes; exact image Size and container references need no layer walk.
var DockerInventory = func() ([]byte, []byte, error) {
	inspect := func(kind string) ([]byte, error) {
		ids, err := execTimeout("docker", kind, "ls", "-aq", "--no-trunc")
		if err != nil {
			return nil, err
		}
		names := strings.Fields(ids)
		if len(names) == 0 {
			return []byte("[]"), nil
		}
		if len(names) > 4096 {
			return nil, fmt.Errorf("Docker %s inventory exceeds 4096 objects", kind)
		}
		unique := make([]string, 0, len(names))
		seen := map[string]bool{}
		for _, n := range names {
			if !seen[n] {
				unique = append(unique, n)
				seen[n] = true
			}
		}
		args := append([]string{kind, "inspect"}, unique...)
		out, err := execTimeout("docker", args...)
		return []byte(out), err
	}
	images, err := inspect("image")
	if err != nil {
		return nil, nil, err
	}
	containers, err := inspect("container")
	return images, containers, err
}

func dockerImages(ctx *Context) {
	images, containers, err := DockerInventory()
	if err != nil {
		ctx.logf("Docker image references unavailable: %v", err)
		return
	}
	if err := parseDockerInventory(images, containers, ctx.Report); err != nil {
		ctx.logf("Docker inventory: %v", err)
	}
}

func parseDockerInventory(imagesRaw, containersRaw []byte, rep *findings.Report) error {
	var images []struct {
		ID       string `json:"Id"`
		Size     int64
		RepoTags []string
		Created  string
	}
	var containers []struct {
		Image, Name string
		State       struct{ Running bool }
	}
	if err := json.Unmarshal(imagesRaw, &images); err != nil {
		return err
	}
	if err := json.Unmarshal(containersRaw, &containers); err != nil {
		return err
	}
	if images == nil || containers == nil {
		return fmt.Errorf("incomplete Docker inventory")
	}
	refs := map[string][]string{}
	for _, c := range containers {
		refs[c.Image] = append(refs[c.Image], strings.TrimPrefix(c.Name, "/"))
	}
	sort.Slice(images, func(i, j int) bool { return images[i].ID < images[j].ID })
	for _, img := range images {
		risk := findings.RiskKeep
		why := "tagged image; layer bytes may be shared"
		var action *findings.Reclaim
		if len(refs[img.ID]) > 0 {
			sort.Strings(refs[img.ID])
			why = "used by containers: " + strings.Join(refs[img.ID], ", ")
		} else if len(img.RepoTags) == 0 || (len(img.RepoTags) == 1 && img.RepoTags[0] == "<none>:<none>") {
			risk = findings.RiskSafeCache
			why = "untagged and unreferenced image; layer bytes may be shared; host VM reclaim is unknown"
			action = &findings.Reclaim{Kind: "docker-image", Target: img.ID, Cmd: "docker image rm " + img.ID}
		}
		rep.Findings = append(rep.Findings, findings.Finding{ID: findings.IDSlug("docker-image", img.ID), Path: "docker-image:" + img.ID, Bytes: img.Size, Category: "docker-image", Risk: risk, SizeBasis: "docker-image-layers", Why: why, Containers: refs[img.ID], Reclaim: action})
	}
	return nil
}
