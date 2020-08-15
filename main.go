package main

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

func main() {
	root, err := findRoot(".")
	if err != nil {
		panic(err)
	}

	home := os.Getenv("HOME")
	if home == "" {
		panic("`$HOME` environment variable unset")
	}

	cacheDir := filepath.Join(home, ".cache", "gubernator")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		panic(err)
	}

	cache, err := FileSystemCacheFromTempDir(cacheDir)
	if err != nil {
		panic(err)
	}

	module := "."
	if len(os.Args) > 1 {
		module = os.Args[1]
	}
	target := "__DEFAULT__"
	if len(os.Args) > 2 {
		target = os.Args[2]
	}

	if err := buildTarget(
		sha256.New,
		cache,
		cacheDir, // use the cache dir as the base dir for temp dirs
		root,
		module,
		target,
	); err != nil {
		if err, ok := err.(*starlark.EvalError); ok {
			panic(err.Backtrace())
		}
		panic(err)
	}
}

func buildTarget(
	newHash func() hash.Hash,
	cache *FileSystemCache,
	tmpDirBase string,
	root string,
	module string,
	target string,
) error {
	packages, err := loadPackages(root)
	if err != nil {
		return errors.Wrap(err, "Loading packages")
	}

	globals, err := execModule(module, makeLoader(root, packages))
	if err != nil {
		return err
	}

	tval, found := globals[target]
	if !found {
		return errors.Errorf("Missing target `%s`", target)
	}

	t, ok := tval.(*Target)
	if !ok {
		return errors.Errorf(
			"`%s` must be a target; found %s",
			target,
			t.Type(),
		)
	}

	d, err := FreezeTarget(root, newHash, cache, t)
	if err != nil {
		return errors.Wrap(err, "Freezing target")
	}

	if err := BuildRecursive(cache, d, tmpDirBase); err != nil {
		return err
	}

	fmt.Println(filepath.Join(cache.root, d.ID))
	return nil
}

func findRoot(dir string) (string, error) {
	if dir == "." {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = wd
	}
	if _, err := os.Stat(filepath.Join(dir, workspaceFileName)); err != nil {
		if os.IsNotExist(err) {
			if dir == "/" || dir == "" {
				return "", errors.Errorf(
					"Current directory isn't inside of a workspace "+
						"(%s file not found in any parent directories)",
					workspaceFileName,
				)
			}
			return findRoot(filepath.Dir(dir))
		}
		return "", err
	}
	return dir, nil
}

func loadPackages(root string) (map[string]string, error) {
	fileInfos, err := ioutil.ReadDir(filepath.Join(root, vendorDirectoryName))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "Error reading directory '%s'", root)
		}
	}

	packages := map[string]string{}
	for _, fi := range fileInfos {
		if fi.IsDir() {
			packageDirectory := filepath.Join(
				root,
				vendorDirectoryName,
				fi.Name(),
			)
			if _, err := os.Stat(
				filepath.Join(packageDirectory, workspaceFileName),
			); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			packages[fi.Name()] = packageDirectory
		}
	}

	return packages, nil
}

const (
	workspaceFileName   = "WORKSPACE"
	vendorDirectoryName = ".vendor"
)
