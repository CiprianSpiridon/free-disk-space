package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/version"
)

func TestRootHelpAndUnknownAndVersion(t *testing.T) {
	ensureHelp()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := Execute([]string{"--help"})
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("freedisk")) {
		t.Fatalf("help=%s", b)
	}

	stderr := &bytes.Buffer{}
	g, rest, _ := parseGlobal([]string{"definitely-not-a-command"})
	g.Stderr = stderr
	g.Stdout = io.Discard
	if lookup(rest[0]) != nil {
		t.Fatal("should be unknown")
	}
	if err := Execute([]string{"definitely-not-a-command"}); !errors.Is(err, ErrUsage) {
		t.Fatalf("err=%v", err)
	}

	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	if err := Execute([]string{"--version"}); err != nil {
		t.Fatal(err)
	}
	w2.Close()
	os.Stdout = old
	vb, _ := io.ReadAll(r2)
	if !bytes.Contains(vb, []byte(version.Version)) {
		t.Fatalf("version=%s", vb)
	}
}
