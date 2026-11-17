package crudgen

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteSpecOnlyRemovesSpec(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ops_demo.yaml")
	if err := os.WriteFile(path, []byte("name: ops_demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := deleteSpecOnlyIn(dir, "ops_demo")
	var specOnly *SpecOnlyDeletedError
	if !errors.As(err, &specOnly) {
		t.Fatalf("error = %v", err)
	}
	if specOnly.Table != "ops_demo" || specOnly.Path != path {
		t.Fatalf("result = %+v", specOnly)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("spec must be removed, err = %v", statErr)
	}
}

func TestDeleteSpecOnlyWithoutSpecFails(t *testing.T) {
	t.Parallel()
	if err := deleteSpecOnlyIn(t.TempDir(), "ops_missing"); err == nil {
		t.Fatal("missing spec must fail")
	}
}
