// Package simulator models simctl runtime disk images, whose UUIDs differ from
// com.apple.CoreSimulator.SimRuntime identifiers and mounted volume names.
package simulator

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Image struct {
	Identifier        string `json:"identifier"`
	UUID              string `json:"uuid"`
	RuntimeIdentifier string `json:"runtimeIdentifier"`
	Build             string `json:"build"`
	Path              string `json:"path"`
	RuntimeBundlePath string `json:"runtimeBundlePath"`
	State             string `json:"state"`
	SizeBytes         int64  `json:"sizeBytes"`
	LastUsedAt        string `json:"lastUsedAt"`
}

func ParseImages(raw []byte) ([]Image, error) {
	var doc map[string]Image
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("simctl runtime list: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("simctl runtime list: null inventory")
	}
	out := make([]Image, 0, len(doc))
	for key, img := range doc {
		if img.Identifier == "" {
			img.Identifier = img.UUID
		}
		if img.Identifier == "" {
			img.Identifier = key
		}
		out = append(out, img)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Identifier < out[j].Identifier })
	return out, nil
}
