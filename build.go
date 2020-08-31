package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/pkg/errors"
)

func init() {
	color.Output = os.Stderr
}

// BuildRecursive builds a derivation's dependencies recursively before
// building the derivation itself. Before any derivation is built, the cache is
// first consulted to see if the target needs to be built in the first place.
func BuildRecursive(
	fsc *FileSystemCache,
	d *Derivation,
	tmpDirBase string,
) error {
	exists, err := fsc.Exists(d.ID)
	if err != nil {
		return errors.Wrapf(err, "Checking cache for key '%s'", d.ID)
	}
	if exists {
		color.Green("Already built %s", d.ID)
		return nil
	}

	for _, dependency := range d.Dependencies {
		if err := BuildRecursive(fsc, dependency, tmpDirBase); err != nil {
			return err
		}
	}

	color.Yellow("Rebuilding %s", d.ID)
	return errors.Wrapf(Build(fsc, d, tmpDirBase), "Building '%s'", d.ID)
}

// Build builds a derivation and puts it into the build cache. It does not
// build the derivation's dependencies, and it should not be invoked until
// after the dependencies have been built. Further, it will always rebuild the
// derivation--i.e., it makes no attempt to check the cache before building the
// target derivation. `tmpDirBase` is the base directory to use when creating
// temporary directories. This can be left empty to use the default temporary
// directory; however, this directory must be on the same file system as the
// build cache or else an error will be returned when this function tries to
// move the output artifacts from the temp directory to the build cache (Linux
// distributions seem to put the tmp dir on a tmpfs file system and
// consequently an explicit tmpDirBase value must be passed).
func Build(fsc *FileSystemCache, d *Derivation, tmpDirBase string) error {
	var output bytes.Buffer

	tmpDir, err := ioutil.TempDir(tmpDirBase, "*")
	if err != nil {
		return errors.Wrap(err, "Creating temporary build directory")
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Print("WARN failed to remove temporary build directory:", err)
		}
	}()
	tmpOutPath := filepath.Join(tmpDir, randString())

	cmd := exec.Command(d.Builder, d.Args...)

	// Make a copy of the derivation's env slice and prepend to it the output
	// env copy slice. It's important that we prepend instead of append so that
	// it overrides any other "out" env vars that were present in the
	// original environment.
	envCopy := make([]string, len(d.Env)+2)
	copy(envCopy[:len(envCopy)-1], d.Env)
	envCopy[len(envCopy)-1] = "out=" + tmpOutPath
	envCopy[len(envCopy)-2] = "cachePath=" + fsc.Root()
	cmd.Env = envCopy
	cmd.Dir = tmpDir
	cmd.Stderr = &output
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "OUTPUT: '%s'", &output)
	}

	// Make the artifact immutable before moving it into the cache.
	if err := makeImmutable(tmpOutPath); err != nil {
		return errors.Wrap(err, "Chmodding output artifact")
	}

	// Builder exited OK; move the output file into the cache. If the output
	// file doesn't exist, report a distinct error.
	if err := fsc.MoveFile(tmpOutPath, d.ID); err != nil {
		if os.IsNotExist(err) {
			return errors.Errorf(
				"Builder succeeded but didn't create output file",
			)
		}
		return errors.Wrap(err, "Moving output file into cache")
	}

	return nil
}

func makeImmutable(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	return makeImmutableHelper(path, fi)
}

func makeImmutableHelper(path string, fi os.FileInfo) error {
	if fi.IsDir() {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}

		for _, file := range files {
			if err := makeImmutableHelper(
				filepath.Join(path, file.Name()),
				file,
			); err != nil {
				return err
			}
		}
	}

	// ^os.FileMode(0b111111111) creates a bitmask that has all 1s followed by
	// 9 zeros. This works regardless of 32-bit vs 64-bit. Then we OR that mask
	// with the lower 9 bits 0b101_101_101 which is a mask which prevents
	// writing (it allows reading and executing). The higher N bits must be set
	// to `1` to avoid overwriting valuable information such as the directory
	// bit.
	const immutableMask = ^os.FileMode(0b111111111) | 0b101101101
	return os.Chmod(path, fi.Mode()&immutableMask)
}
