package main

import (
	"os"
	"testing"

	"github.com/pkg/errors"
)

func TestBuild(t *testing.T) {
	d := Derivation{
		ID:           "foo",
		Hash:         []byte("barhash"),
		Builder:      "/bin/bash",
		Args:         []string{"-c", "echo 'hello' > $out"},
		Dependencies: nil,
		Env:          os.Environ(),
	}

	if err := withTempDir(func(tmpDir string) error {
		fsc, err := FileSystemCacheFromTempDir(tmpDir)
		if err != nil {
			return errors.Wrap(err, "Creating temp FileSystemCache directory")
		}
		return errors.Wrap(Build(fsc, &d, tmpDir), "Building test derivation")
	}); err != nil {
		t.Fatal(err)
	}
}
