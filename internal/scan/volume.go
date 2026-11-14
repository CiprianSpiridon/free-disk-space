package scan

import (
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

// ReadVolume measures the boot disk's APFS container, not the sealed root
// snapshot or the first unrelated container in diskutil's full inventory.
func ReadVolume() (findings.Volume, error) {
	out, err := readAPFS()
	if err != nil {
		return findings.Volume{}, err
	}
	v, err := ParseVolume(out)
	if err == nil {
		v.MeasuredAt = time.Now().UTC().Format(time.RFC3339)
	}
	return v, err
}

func containerReference(raw []byte) (string, error) {
	d := xml.NewDecoder(strings.NewReader(string(raw)))
	key := ""
	for {
		tok, err := d.Token()
		if err != nil {
			return "", fmt.Errorf("APFS container reference missing: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "key" {
			if err := d.DecodeElement(&key, &start); err != nil {
				return "", err
			}
		} else if key == "APFSContainerReference" && start.Name.Local == "string" {
			var ref string
			if err := d.DecodeElement(&ref, &start); err != nil {
				return "", err
			}
			if !regexp.MustCompile(`^disk[0-9]+$`).MatchString(ref) {
				return "", fmt.Errorf("invalid APFS container reference")
			}
			return ref, nil
		}
	}
}

func readAPFS() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "diskutil", "info", "-plist", "/").Output()
	if err != nil {
		return "", err
	}
	ref, err := containerReference(b)
	if err != nil {
		return "", err
	}
	b, err = exec.CommandContext(ctx, "diskutil", "apfs", "list", ref).Output()
	return string(b), err
}

var (
	reInUse        = regexp.MustCompile(`Capacity In Use By Volumes:\s+(\d+)\s*B`)
	reFree         = regexp.MustCompile(`Capacity Not Allocated:\s+(\d+)\s*B`)
	reCeiling      = regexp.MustCompile(`Size \(Capacity Ceiling\):\s+(\d+)\s*B`)
	reInUseLoose   = regexp.MustCompile(`Capacity In Use By Volumes:\s+([0-9.]+)\s*([KMGT]i?B)`)
	reFreeLoose    = regexp.MustCompile(`Capacity Not Allocated:\s+([0-9.]+)\s*([KMGT]i?B)`)
	reCeilingLoose = regexp.MustCompile(`(?:Size \(Capacity Ceiling\)|Capacity Ceiling):\s+([0-9.]+)\s*([KMGT]i?B)`)
)

func unitMul(u string) int64 {
	switch strings.ToUpper(u) {
	case "KB":
		return 1_000
	case "MB":
		return 1_000_000
	case "GB":
		return 1_000_000_000
	case "TB":
		return 1_000_000_000_000
	case "KIB":
		return 1024
	case "MIB":
		return 1024 * 1024
	case "GIB":
		return 1024 * 1024 * 1024
	case "TIB":
		return 1024 * 1024 * 1024 * 1024
	default:
		return 1
	}
}

func parseHuman(re *regexp.Regexp, s string) int64 {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return -1
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return int64(f * float64(unitMul(m[2])))
}

func parseBytesParen(re *regexp.Regexp, s string) int64 {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return -1
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return -1
	}
	return n
}

// ParseVolume extracts APFS container figures from `diskutil apfs list` text.
func ParseVolume(diskutilOut string) (findings.Volume, error) {
	var v findings.Volume
	if m := regexp.MustCompile(`APFS Container Reference:\s*(disk[0-9]+)`).FindStringSubmatch(diskutilOut); len(m) > 1 {
		v.ContainerID = m[1]
	}
	v.InUseBytes = parseBytesParen(reInUse, diskutilOut)
	v.FreeBytes = parseBytesParen(reFree, diskutilOut)
	v.ContainerBytes = parseBytesParen(reCeiling, diskutilOut)
	if v.InUseBytes < 0 {
		v.InUseBytes = parseHuman(reInUseLoose, diskutilOut)
	}
	if v.FreeBytes < 0 {
		v.FreeBytes = parseHuman(reFreeLoose, diskutilOut)
	}
	if v.ContainerBytes < 0 {
		v.ContainerBytes = parseHuman(reCeilingLoose, diskutilOut)
	}
	if v.InUseBytes < 0 || v.FreeBytes < 0 {
		return findings.Volume{}, fmt.Errorf("diskutil apfs list: could not parse Capacity In Use / Not Allocated")
	}
	if v.ContainerBytes < 0 {
		v.ContainerBytes = v.InUseBytes + v.FreeBytes
	}
	v.DataVolumeUsedBytes = parseDataVolume(diskutilOut)
	return v, nil
}

var (
	reVolSplit = regexp.MustCompile(`(?m)^\s*Volume disk`)
	reVolName  = regexp.MustCompile(`Name:\s+(.+)`)
	reVolUsed  = regexp.MustCompile(`(?i)Capacity in use by this volume:\s+(\d+)\s*B`)
)

func parseDataVolume(s string) int64 {
	parts := reVolSplit.Split(s, -1)
	var best int64
	for _, part := range parts {
		name := ""
		if m := reVolName.FindStringSubmatch(part); len(m) > 1 {
			name = strings.TrimSpace(m[1])
		}
		if !strings.Contains(strings.ToLower(name), "data") {
			continue
		}
		if strings.Contains(strings.ToLower(name), "preboot") {
			continue
		}
		n := parseBytesParen(reVolUsed, part)
		if n > best {
			best = n
		}
	}
	return best
}

// VolumePhaseName is the registered phase name.
const VolumePhaseName = "volume"
