package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

type testHash struct {
	output string
	bytes.Buffer
}

func (th *testHash) Sum(b []byte) []byte {
	return append(b, []byte(th.output)...)
}

func (th *testHash) Size() int {
	return len(th.output)
}

func (th *testHash) BlockSize() int { return len(th.output) }

type entry interface{ entry() }

type dirEntry struct {
	files map[string]*fileEntry
}

func (de *dirEntry) entry() {}

type fileEntry struct {
	mode os.FileMode
	buf  bytes.Buffer
}

func (fe *fileEntry) entry() {}

type testCache struct {
	entries map[string]entry
}

func newTestCache() *testCache {
	return &testCache{map[string]entry{}}
}

func (tc *testCache) NewDirEntry(
	cacheDirCallback CacheDirCallback,
	nameCallback NameCallback,
) error {
	files := map[string]*fileEntry{}
	if err := cacheDirCallback(
		func(relpath string, callback CacheFileCallback) error {
			var buf bytes.Buffer
			mode, err := callback(&buf)
			if err != nil {
				return err
			}
			files[relpath] = &fileEntry{mode: mode, buf: buf}
			return nil
		},
	); err != nil {
		return err
	}
	tc.entries[nameCallback()] = &dirEntry{files: files}
	return nil
}

func (tc *testCache) NewFileEntry(
	cacheFileCallback CacheFileCallback,
	nameCallback NameCallback,
) error {
	var buf bytes.Buffer
	mode, err := cacheFileCallback(&buf)
	if err != nil {
		return err
	}
	tc.entries[nameCallback()] = &fileEntry{mode: mode, buf: buf}
	return nil
}

func derivationID(hash, targetName string) string {
	return fmt.Sprintf("%s-%s", hex.EncodeToString([]byte(hash)), targetName)
}

func fmtList(ss []string) string {
	return "[" + strings.Join(ss, ", ") + "]"
}

func stringList(ss []string) string {
	tmp := make([]string, len(ss))
	for i, s := range ss {
		tmp[i] = "'" + s + "'"
	}
	return fmtList(tmp)
}

func argList(args []Arg) string {
	tmp := make([]string, len(args))
	for i, arg := range args {
		tmp[i] = "'" + arg.String() + "'"
	}
	return fmtList(tmp)
}

func derivationList(derivations []*Derivation) string {
	tmp := make([]string, len(derivations))
	for i, d := range derivations {
		tmp[i] = d.String()
	}
	return fmtList(tmp)
}

func expectDerivation(wanted, got *Derivation) error {
	if wanted == nil && got == nil {
		return nil
	}

	if wanted == nil || got == nil {
		return fmt.Errorf("Wanted %v; got %v", wanted, got)
	}

	if wanted.ID != got.ID {
		return fmt.Errorf(
			"Wanted derivation ID '%s'; got '%s'",
			wanted.ID,
			got.ID,
		)
	}

	if len(wanted.Args) != len(got.Args) {
		return fmt.Errorf(
			"Wanted %s; got %s",
			stringList(wanted.Args),
			stringList(got.Args),
		)
	}
	for i := range wanted.Args {
		if wanted.Args[i] != got.Args[i] {
			return fmt.Errorf(
				"Mismatching args at %d; wanted '%s', got '%s'",
				i,
				wanted.Args[i],
				got.Args[i],
			)
		}
	}

	if len(wanted.Dependencies) != len(got.Dependencies) {
		return fmt.Errorf(
			"Wanted %s; got %s",
			derivationList(wanted.Dependencies),
			derivationList(got.Dependencies),
		)
	}
	for i := range wanted.Dependencies {
		if err := expectDerivation(
			wanted.Dependencies[i],
			got.Dependencies[i],
		); err != nil {
			return errors.Wrapf(err, "Dependencies[%d]", i)
		}
	}
	return nil
}

func expectHashed(hasher *testHash, wanted ...string) error {
	hashed := hasher.Buffer.String()
	for _, wanted := range wanted {
		if !strings.Contains(hashed, wanted) {
			return fmt.Errorf("Expected '%s' was hashed but it wasn't", wanted)
		}
	}
	return nil
}

func withTempDir(f func(string) error) error {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			panic(fmt.Sprintf("Error removing directory '%s': %v", dir, err))
		}
	}()

	return f(dir)
}

func cachePath(hash string, relpath string) string {
	return filepath.Join(hex.EncodeToString([]byte(hash)), relpath)
}

func TestFreezeTarget(t *testing.T) {
	hasher := testHash{output: "hash"}

	d, err := FreezeTarget(
		"package-root",
		func() hash.Hash { return &hasher },
		newTestCache(),
		&Target{
			Name:    "toplevel-target",
			Builder: "toplevel-builder",
			Args:    []Arg{String("arg1"), String("arg2")},
			Env:     []string{"ABC=def", "123=456"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected err: %v", err)
	}

	if err := expectDerivation(
		&Derivation{
			ID:           derivationID("hash", "toplevel-target"),
			Dependencies: nil,
			Builder:      "toplevel-builder",
			Args:         []string{"arg1", "arg2"},
			Env:          []string{"ABC=def", "123=456"},
		},
		d,
	); err != nil {
		t.Fatal(err)
	}

	if err := expectHashed(
		&hasher,
		"toplevel-target",
		"toplevel-builder",
		"arg1",
		"arg2",
		"ABC=def",
		"123=456",
	); err != nil {
		t.Fatal(err)
	}
}

func TestFreezeTarget_withDependencyArg(t *testing.T) {
	hasher := testHash{output: "toplevel-hash"}
	nestedHasher := testHash{output: "nested-hash"}

	h := &hasher
	d, err := FreezeTarget(
		"package-root",
		func() hash.Hash {
			tmp := h
			h = &nestedHasher
			return tmp
		},
		newTestCache(),
		&Target{
			Name:    "toplevel-target",
			Builder: "toplevel-builder",
			Args: []Arg{&Target{
				Name:    "nested-target",
				Builder: "nested-builder",
				Args:    nil,
				Env:     nil,
			}},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected err: %v", err)
	}

	if err := expectDerivation(
		&Derivation{
			ID:      derivationID("toplevel-hash", "toplevel-target"),
			Builder: "toplevel-builder",
			Args:    []string{derivationID("nested-hash", "nested-target")},
			Env:     nil,
			Dependencies: []*Derivation{{
				ID:           derivationID("nested-hash", "nested-target"),
				Builder:      "nested-builder",
				Args:         nil,
				Env:          nil,
				Dependencies: nil,
			}},
		},
		d,
	); err != nil {
		t.Fatal(err)
	}

	if err := expectHashed(
		&hasher,
		"toplevel-target",
		"toplevel-builder",
		"nested-hash",
	); err != nil {
		t.Fatal(err)
	}

	if err := expectHashed(
		&nestedHasher,
		"nested-target",
		"nested-builder",
	); err != nil {
		t.Fatal(err)
	}
}

func TestFreezeTarget_withPathArg(t *testing.T) {
	toplevelHasher := testHash{output: "toplevel-hash"}
	argHasher := testHash{output: "arg-hash"}
	cache := newTestCache()
	h := &toplevelHasher

	var got *Derivation
	if err := withTempDir(func(packageRoot string) error {
		if err := ioutil.WriteFile(
			filepath.Join(packageRoot, "foo.yml"),
			[]byte("foo-yml-contents"),
			0644,
		); err != nil {
			return err
		}

		d, err := FreezeTarget(
			packageRoot,
			func() hash.Hash {
				tmp := h
				h = &argHasher
				return tmp
			},
			cache,
			&Target{
				Name:    "toplevel-target",
				Builder: "toplevel-builder",
				Args:    []Arg{Path("foo.yml")},
			},
		)
		got = d
		return err
	}); err != nil {
		t.Fatalf("Unexpected err: %v", err)
	}

	if err := expectDerivation(
		&Derivation{
			ID:      derivationID("toplevel-hash", "toplevel-target"),
			Builder: "toplevel-builder",
			Args:    []string{cachePath("arg-hash", "foo.yml")},
			Env:     nil,
		},
		got,
	); err != nil {
		t.Fatal(err)
	}

	if err := expectHashed(
		&argHasher,
		"foo.yml",
		"foo-yml-contents",
	); err != nil {
		t.Fatal(err)
	}

	entry := cache.entries[cachePath("arg-hash", "foo.yml")].(*fileEntry)
	if "foo-yml-contents" != entry.buf.String() {
		t.Fatalf(
			"Wanted contents 'foo-yml-contents', got %s",
			entry.buf.String(),
		)
	}
}

func TestFreezeTarget_OneDependency(t *testing.T) {
	dependency := Target{
		Name:    "target<dependency>",
		Builder: "bash",
		Args: []Arg{
			String("arg<dependency>-1"),
			String("arg<dependency>-2"),
		},
		Env: []string{"env<dependency>-1", "env<dependency>-2"},
	}

	toplevel := Target{
		Name:    "target<toplevel>",
		Builder: "builder<bash>",
		Args: []Arg{&Sub{
			Format: "Dependency ${Dependency}",
			Substitutions: []Substitution{{
				Key:   "Dependency",
				Value: &dependency,
			}},
		}},
		Env: []string{"env<toplevel>-1", "env<toplevel>-2"},
	}

	hasher := testHash{output: "hash<toplevel>"}
	dependencyHasher := testHash{output: "hash<dependency>"}
	cache := newTestCache()
	h := &hasher

	var got *Derivation
	if err := withTempDir(func(packageRoot string) error {
		d, err := FreezeTarget(
			packageRoot,
			func() hash.Hash {
				tmp := h
				h = &dependencyHasher
				return tmp
			},
			cache,
			&toplevel,
		)
		got = d
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if err := expectDerivation(
		&Derivation{
			ID:   derivationID("hash<toplevel>", "target<toplevel>"),
			Hash: []byte("hash<toplevel>"),
			Dependencies: []*Derivation{{
				ID: derivationID(
					"hash<dependency>",
					"target<dependency>",
				),
				Hash:    []byte("hash<dependency>"),
				Builder: "builder<dependency>",
				Args:    []string{"arg<dependency>-1", "arg<dependency>-2"},
				Env:     []string{"env<dependency>-1", "env<dependency>-2"},
			}},
			Builder: "builder<toplevel>",
			Args: []string{fmt.Sprintf(
				"Dependency %s",
				derivationID("hash<dependency>", "target<dependency>"),
			)},
			Env: []string{"env<toplevel>-1", "env<toplevel>-2"},
		},
		got,
	); err != nil {
		t.Fatal(err)
	}
}

func TestPathFreezeArg(t *testing.T) {
	if err := withTempDir(func(dir string) error {
		// Prepare test file
		if err := ioutil.WriteFile(
			filepath.Join(dir, "test"),
			[]byte("hi!"),
			0644,
		); err != nil {
			return errors.Wrap(err, "Unexpected error writing test file")
		}

		h := testHash{output: "hash"}
		cache := newTestCache()

		argValue, err := Path("test").freezeArg(&freezer{
			packageRoot: dir,
			newHasher:   func() hash.Hash { return &h },
			cache:       cache,
		})
		if err != nil {
			return errors.Wrap(err, "Unexpected error freezing test file")
		}

		hashStr := h.Buffer.String()
		if !strings.Contains(hashStr, "test") {
			return errors.Errorf(
				"Expected the hash to include the path ('test')",
			)
		}
		if !strings.Contains(hashStr, string([]byte{6, 4, 4})) {
			return errors.Errorf(
				"Expected the hash to include the file mode bits (0644)",
			)
		}
		if !strings.Contains(hashStr, "hi!") {
			return errors.Errorf(
				"Expected the hash to include the file body ('hi!')",
			)
		}

		testEntry, found := cache.entries[argValue.Value]
		if !found {
			return errors.Errorf(
				"Missing expected cache entry '%s'",
				argValue.Value,
			)
		}
		testFileEntry, ok := testEntry.(*fileEntry)
		if !ok {
			return errors.Errorf(
				"Expected file entry at '%s'; found dir entry",
				argValue.Value,
			)
		}
		if actual := testFileEntry.mode; actual != 0644 {
			return errors.Errorf(
				"Expected %s mode 0644; found "+
					"'%s'",
				argValue.Value,
				actual,
			)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGlobGroupFreezeArg(t *testing.T) {
	files := map[string][]byte{
		"foo/bar": []byte("hello"),
		"foo/baz": []byte("world"),
		"qux":     []byte("asdf"),
	}
	if err := withTempDir(func(dir string) error {
		for relPath, contents := range files {
			filePath := filepath.Join(dir, relPath)
			if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
				return errors.Wrapf(
					err,
					"Unexpected error creating intermediate directories for "+
						"test file '%s'",
					filePath,
				)
			}
			if err := ioutil.WriteFile(
				filePath,
				contents,
				0644,
			); err != nil {
				return errors.Wrapf(
					err,
					"Unexpected error writing test file '%s'",
					filePath,
				)
			}
		}

		globGroup := GlobGroup{"foo/ba*"}
		h := testHash{output: "hash"}
		cache := newTestCache()

		argValue, err := globGroup.freezeArg(&freezer{
			packageRoot: dir,
			newHasher:   func() hash.Hash { return &h },
			cache:       cache,
		})
		if err != nil {
			return errors.Wrap(err, "Unexpected error freezing glob group")
		}

		if actual := string(argValue.Hash); actual != "hash" {
			return errors.Errorf("Wanted hash 'hash'; got '%s'", actual)
		}

		if argValue.Derivations != nil {
			return errors.Errorf(
				"Wanted `<nil>` derivations; got %v",
				argValue.Derivations,
			)
		}

		cacheEntry, found := cache.entries[argValue.Value]
		if !found {
			return errors.Errorf(
				"Coulding find entry in cache at key '%s'",
				argValue.Value,
			)
		}

		entry, ok := cacheEntry.(*dirEntry)
		if !ok {
			return errors.Errorf(
				"Expected directory cache entry for key '%s' but found file",
				argValue.Value,
			)
		}

		if len(entry.files) != 2 {
			return errors.Errorf(
				"Expected 2 files in the cache dir entry '%s'; found %d",
				argValue.Value,
				len(entry.files),
			)
		}

		// Make sure the foo/bar file was grabbed properly
		fooBarEntry, found := entry.files["foo/bar"]
		if !found {
			return errors.Errorf(
				"Missing expected file in dir entry (key='%s'): foo/bar",
				argValue.Value,
			)
		}
		if actual := fooBarEntry.mode; actual != 0644 {
			return errors.Errorf(
				"In cache dir entry '%s': Expected foo/bar mode 0644; found "+
					"'%s'",
				argValue.Value,
				actual,
			)
		}
		if actual := fooBarEntry.buf.String(); actual != "hello" {
			return errors.Errorf(
				"In cache dir entry '%s': Expected foo/bar contents "+
					"'hello'; found '%s'",
				argValue.Value,
				actual,
			)
		}

		// Make sure the foo/baz file was grabbed properly
		fooBazEntry, found := entry.files["foo/baz"]
		if !found {
			return errors.Errorf(
				"Missing expected file in dir entry (key='%s'): foo/baz",
				argValue.Value,
			)
		}
		if actual := fooBazEntry.mode; actual != 0644 {
			return errors.Errorf(
				"In cache dir entry '%s': Expected foo/bar mode 0644; found "+
					"'%s'",
				argValue.Value,
				actual,
			)
		}
		if actual := fooBazEntry.buf.String(); actual != "world" {
			return errors.Errorf(
				"In cache dir entry '%s': Expected foo/baz contents "+
					"'world'; found '%s'",
				argValue.Value,
				actual,
			)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TODO: Find a better way to assert that things were hashed. Specifically, we
// don't want tests to start failing if we change the order in which things are
// hashed in the implementation. We also don't want to fail if we decide that
// the toplevel hasher will hash the contents of nested things directly rather
// than hashing the hash of the contents. We also want to avoid false-positives
// associated with string-contains checks (e.g., we actually hash foo/bar.yml
// but the check passes because we're just expecting the hash input contains
// bar.yml).
