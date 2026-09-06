package scan

import "testing"

const fixture = `
APFS Container (1 found)
|
+-- Container disk3  GUID
    ====================================================
    APFS Container Reference:     disk3
    Size (Capacity Ceiling):      926.0 GB (926000000000 Bytes)
    Capacity In Use By Volumes:   785.2 GB (785200000000 Bytes) (84.8% used)
    Capacity Not Allocated:       140.8 GB (140800000000 Bytes) (15.2% free)
`

func TestParseVolumeFixture(t *testing.T) {
	v, err := ParseVolume(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if v.InUseBytes != 785200000000 {
		t.Fatalf("in_use=%d", v.InUseBytes)
	}
	if v.FreeBytes != 140800000000 {
		t.Fatalf("free=%d", v.FreeBytes)
	}
	if v.ContainerBytes != 926000000000 {
		t.Fatalf("container=%d", v.ContainerBytes)
	}
}

func TestParseVolumeDoesNotUseDFRoot(t *testing.T) {
	// System snapshot ~17GB must not become in_use when container figures exist.
	s := fixture + "\nFilesystem  17Gi  16Gi  1Gi  94% /\n"
	v, err := ParseVolume(s)
	if err != nil {
		t.Fatal(err)
	}
	if v.InUseBytes < 100_000_000_000 {
		t.Fatalf("used df snapshot? in_use=%d", v.InUseBytes)
	}
}

func TestParseVolumeGarbage(t *testing.T) {
	if _, err := ParseVolume("not diskutil output"); err == nil {
		t.Fatal("expected error")
	}
}
