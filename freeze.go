package main

import (
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar"
	"github.com/pkg/errors"
)

func FreezeTarget(
	packageRoot string,
	newHasher func() hash.Hash,
	cache Cache,
	t *Target,
) (*Derivation, error) {
	d, _, err := freezeTarget(&freezer{packageRoot, newHasher, cache}, t)
	return d, err
}

type freezer struct {
	packageRoot string
	newHasher   func() hash.Hash
	cache       Cache
}

func freezeTarget(f *freezer, t *Target) (*Derivation, []byte, error) {
	hasher := f.newHasher()
	hasher.Write([]byte(t.Name))
	hasher.Write([]byte(t.Builder))

	for _, envVar := range t.Env {
		hasher.Write([]byte(envVar))
	}

	var dependencies []*Derivation
	frozenArgs := make([]string, len(t.Args))
	for i, arg := range t.Args {
		argValue, err := arg.freezeArg(f)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "Freezing argument '%s'", arg)
		}

		dependencies = append(dependencies, argValue.Derivations...)

		// Include the arg's hash in the target hash (so changes to the arg
		// invalidate the target)
		hasher.Write(argValue.Hash)

		// Add the value to the list of frozen args
		frozenArgs[i] = argValue.Value
	}

	hash := hasher.Sum(nil)
	return &Derivation{
		ID:           fmt.Sprintf("%s-%s", hex.EncodeToString(hash), t.Name),
		Dependencies: dependencies,
		Builder:      t.Builder,
		Args:         frozenArgs,
		Env:          t.Env,
	}, hash, nil
}

func (t *Target) freezeArg(f *freezer) (ArgValue, error) {
	d, hash, err := freezeTarget(f, t)
	if err != nil {
		return ArgValue{}, err
	}
	return ArgValue{
		Value:       d.ID,
		Derivations: []*Derivation{d},
		Hash:        hash,
	}, nil
}

func (s String) freezeArg(f *freezer) (ArgValue, error) {
	hasher := f.newHasher()
	hasher.Write([]byte(s))
	return ArgValue{Value: string(s), Hash: hasher.Sum(nil)}, nil
}

func (p Path) freezeArg(f *freezer) (ArgValue, error) {
	hasher := f.newHasher()
	cachePath := func() string {
		return filepath.Join(hex.EncodeToString(hasher.Sum(nil)), string(p))
	}
	if err := f.cache.NewFileEntry(
		hashFile(f.packageRoot, string(p), hasher),
		cachePath,
	); err != nil {
		return ArgValue{}, err
	}
	return ArgValue{
		Value:       cachePath(),
		Hash:        hasher.Sum(nil),
		Derivations: nil,
	}, nil
}

func (gg GlobGroup) freezeArg(f *freezer) (ArgValue, error) {
	// Resolve the glob patterns into a list of file paths.
	paths, err := gg.matches(f.packageRoot)
	if err != nil {
		return ArgValue{}, err
	}

	// Hash the paths
	hasher := f.newHasher()
	if err := f.cache.NewDirEntry(
		func(registerFile CacheDir) error {
			for _, path := range paths {
				relPath, err := filepath.Rel(f.packageRoot, path)
				if err != nil {
					return err
				}

				if err := registerFile(
					relPath,
					hashFile(f.packageRoot, relPath, hasher),
				); err != nil {
					return err
				}
			}
			return nil
		},
		func() string { return hex.EncodeToString(hasher.Sum(nil)) },
	); err != nil {
		return ArgValue{}, err
	}
	return ArgValue{
		Value:       hex.EncodeToString(hasher.Sum(nil)),
		Hash:        hasher.Sum(nil),
		Derivations: nil,
	}, nil
}

func (gg GlobGroup) matches(packageRoot string) ([]string, error) {
	// Create a set of globs so we know we aren't looking up any glob multiple
	// times in the event that there are duplicate globs.
	seen := map[string]struct{}{}
	paths := make([]string, 0, 64)
	for _, glob := range gg {
		if _, found := seen[glob]; found {
			continue
		}
		matches, err := doublestar.Glob(filepath.Join(packageRoot, glob))
		if err != nil {
			return nil, errors.Wrapf(err, "Matching pattern '%s'", glob)
		}
		paths = append(paths, matches...)
		seen[glob] = struct{}{}
	}

	// Sort the paths so they're always in the same order for stable hashing.
	sort.Strings(paths)

	return paths, nil
}

func hashFile(root, relPath string, hasher hash.Hash) CacheFileCallback {
	return func(w io.Writer) (os.FileMode, error) {
		f, err := os.Open(filepath.Join(root, relPath))
		if err != nil {
			return 0, err
		}
		defer properClose(f)

		fi, err := f.Stat()
		if err != nil {
			return 0, err
		}
		mode := fi.Mode()

		hasher.Write([]byte(relPath))
		hasher.Write([]byte{
			byte(mode >> 6 & 0o007),
			byte(mode >> 3 & 0o007),
			byte(mode & 0o007),
		})

		_, err = io.Copy(w, &HashingReader{Reader: f, Hasher: hasher})
		return fi.Mode(), err
	}
}

func (s *Sub) freezeArg(f *freezer) (ArgValue, error) {
	hasher := f.newHasher()
	hasher.Write([]byte(s.Format))
	message := s.Format
	var derivations []*Derivation
	for _, substitution := range s.Substitutions {
		value, err := substitution.Value.freezeArg(f)
		if err != nil {
			return ArgValue{}, errors.Wrapf(err, "Freezing substitution '%s'", substitution.Key)
		}
		derivations = append(derivations, value.Derivations...)
		hasher.Write([]byte(substitution.Key))
		hasher.Write(value.Hash)
		message = strings.ReplaceAll(
			message,
			"${"+substitution.Key+"}",
			value.Value,
		)
	}
	return ArgValue{
		Value:       message,
		Derivations: derivations,
		Hash:        hasher.Sum(nil),
	}, nil
}
