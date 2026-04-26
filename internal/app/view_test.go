package app

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// requireEqualGolden compares got to testdata/<TestName>.golden.
// Run `go test ./... -update` to regenerate the file after intentional UI changes.
//
// Stand-in for github.com/charmbracelet/x/exp/golden until network access
// is available to vendor it; the contract is the same.
func requireEqualGolden(t *testing.T, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", t.Name()+".golden")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("output mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestView_Initial(t *testing.T) {
	requireEqualGolden(t, []byte((model{}).View()))
}
