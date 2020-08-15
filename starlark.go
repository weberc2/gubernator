package main

import (
	"fmt"
	"hash"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"hash/adler32"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

//
// Path
//

// Type implements the starlark.Value.Type() method.
func (p Path) Type() string {
	return fmt.Sprintf("Path")
}

// Freeze implements the starlark.Value.Freeze() method.
func (p Path) Freeze() {}

// Truth implements the starlark.Value.Truth() method.
func (p Path) Truth() starlark.Bool { return starlark.True }

// Hash32 implements the Arg.Hash32() method.
func (p Path) Hash32(h hash.Hash32) { h.Write([]byte(p)) }

// Hash implements the starlark.Value.Hash() method.
func (p Path) Hash() (uint32, error) {
	return adler32.Checksum([]byte(p)), nil
}

// starlarkPath parses Starlark kw/args and returns a corresponding `Path`
func starlarkPath(
	args starlark.Tuple,
	kwargs []starlark.Tuple,
) (starlark.Value, error) {
	if len(args) != 1 {
		return nil, errors.Errorf(
			"Expected exactly 1 positional argument; found %d",
			len(args),
		)
	}

	if len(kwargs) != 0 {
		return nil, errors.Errorf(
			"Expected exactly 0 positional arguments; found %d",
			len(kwargs),
		)
	}

	if s, ok := args[0].(starlark.String); ok {
		return Path(s), nil
	}

	return nil, errors.Errorf(
		"TypeError: Expected a string argument; found %s",
		args[0].Type(),
	)
}

//
// GlobGroup
//

// Type implements the starlark.Value.Type() method.
func (gg GlobGroup) Type() string {
	return fmt.Sprintf("GlobGroup")
}

// Freeze implements the starlark.Value.Freeze() method.
func (gg GlobGroup) Freeze() {}

// Truth implements the starlark.Value.Truth() method.
func (gg GlobGroup) Truth() starlark.Bool { return starlark.True }

// Hash32 implements the Arg.Hash32() method.
func (gg GlobGroup) Hash32(h hash.Hash32) {
	for _, p := range gg {
		h.Write([]byte(p))
	}
}

// Hash implements the starlark.Value.Hash() method.
func (gg GlobGroup) Hash() (uint32, error) {
	h := adler32.New()
	gg.Hash32(h)
	return h.Sum32(), nil
}

//
// String
//

// Hash32 implements the Arg.Hash32() method.
func (s String) Hash32(h hash.Hash32) { h.Write([]byte(s)) }

//
// Sub
//

// Type implements the starlark.Value.Type() method.
func (s *Sub) Type() string { return "Sub" }

// Truth implements the starlark.Value.Truth() method.
func (s *Sub) Truth() starlark.Bool { return s == nil }

// Hash32 implements the Arg.Hash32() method.
func (s *Sub) Hash32(h hash.Hash32) {
	h.Write([]byte(s.Format))
	for _, sub := range s.Substitutions {
		h.Write([]byte(sub.Key))
		sub.Value.Hash32(h)
	}
}

// Hash implements the starlark.Value.Hash() method.
func (s *Sub) Hash() (uint32, error) {
	h := adler32.New()
	s.Hash32(h)
	return h.Sum32(), nil
}

// Freeze implements the starlark.Value.Freeze() method.
func (s *Sub) Freeze() {}

// starlarkSub parses Starlark kw/args and returns a corresponding `*Sub`
// wrapped in a `starlark.Value` interface. This is used in the `sub()`
// starlark predefined/builtin function.
func starlarkSub(
	args starlark.Tuple,
	kwargs []starlark.Tuple,
) (starlark.Value, error) {
	// Expect exactly one positional argument, which represents the format
	// string.
	if len(args) != 1 {
		return nil, errors.Errorf(
			"Expected 1 positional argument 'format'; found %d",
			len(args),
		)
	}

	// Validate that the positional argument is a string.
	format, ok := args[0].(starlark.String)
	if !ok {
		return nil, errors.Errorf(
			"TypeError: Expected argument 'format' has type str; found %s",
			args[0].Type(),
		)
	}

	// Treat the keyword arguments as substitutions, including parsing their
	// values into `Arg`s.
	substitutions := make([]Substitution, len(kwargs))
	for i, kwarg := range kwargs {
		value, err := starlarkValueToArg(kwarg[1])
		if err != nil {
			return nil, err
		}

		substitutions[i] = Substitution{
			Key:   string(kwarg[0].(starlark.String)),
			Value: value,
		}
	}

	// TODO: Error if there are substitution placeholders in the format string
	// (e.g., `${Foo}`) for which there are no corresponding substitutions.
	// This is particularly important since the placeholder syntax is valid
	// bash, for example, if the placeholder is `${PATH}`, it would resolve at
	// runtime to the PATH env var, which would be a different down-the-road
	// error if it errored at all.

	// Build and return the resulting `*Sub` structure.
	return &Sub{Format: string(format), Substitutions: substitutions}, nil
}

//
// Target
//

// Type implements the starlark.Value.Type() method.
func (t *Target) Type() string { return "Target" }

// Freeze implements the starlark.Value.Freeze() method.
func (t *Target) Freeze() {}

// Truth implements the starlark.Value.Truth() method.
func (t *Target) Truth() starlark.Bool { return starlark.Bool(t == nil) }

// Hash32 implements the Arg.Hash32() method.
func (t *Target) Hash32(h hash.Hash32) {
	h.Write([]byte(t.Name))
	h.Write([]byte(t.Builder))
	for _, arg := range t.Args {
		arg.Hash32(h)
	}
	for _, env := range t.Env {
		h.Write([]byte(env))
	}
}

// Hash implements the starlark.Value.Hash() method.
func (t *Target) Hash() (uint32, error) {
	h := adler32.New()
	t.Hash32(h)
	return h.Sum32(), nil
}

// starlarkTarget parses Starlark kw/args and returns a corresponding `*Target`
// wrapped in a `starlark.Value` interface. This is used in the `target()`
// starlark predefined/builtin function.
func starlarkTarget(
	args starlark.Tuple,
	kwargs []starlark.Tuple,
) (starlark.Value, error) {
	// For the sake of simpler parsing, we'll simply require that all args are
	// passed as kwargs (no positional args).
	if len(args) != 0 {
		return nil, errors.Errorf(
			"Expected 0 positional args; found %d",
			len(args),
		)
	}

	// Make sure we have exactly the right number of keyword arguments.
	if len(kwargs) != 4 {
		found := make([]string, len(kwargs))
		for i, kwarg := range kwargs {
			found[i] = string(kwarg[0].(starlark.String))
		}
		return nil, errors.Errorf(
			"Expected kwargs {name, builder, args, env}; found {%s}",
			strings.Join(found, ", "),
		)
	}

	// Iterate through the keyword arguments and grab the values for each
	// kwarg, putting them into the right `starlark.Value` variable. We'll
	// convert these to Go values for the `*Target` struct later.
	var nameKwarg, builderKwarg, argsKwarg, envKwarg starlark.Value
	for _, kwarg := range kwargs {
		switch key := kwarg[0].(starlark.String); key {
		case "name":
			if nameKwarg != nil {
				return nil, errors.Errorf("Duplicate argument 'name' found")
			}
			nameKwarg = kwarg[1]
		case "builder":
			if builderKwarg != nil {
				return nil, errors.Errorf("Duplicate argument 'builder' found")
			}
			builderKwarg = kwarg[1]
		case "args":
			if argsKwarg != nil {
				return nil, errors.Errorf("Duplicate argument 'args' found")
			}
			argsKwarg = kwarg[1]
		case "env":
			if envKwarg != nil {
				return nil, errors.Errorf("Duplicate argument 'env' found")
			}
			envKwarg = kwarg[1]
		default:
			return nil, errors.Errorf("Unexpected argument '%s' found", key)
		}
	}

	// Ok, now we've made sure we have values for the required keyword args and
	// that no additional arguments were passed. Next, we'll convert these
	// `starlark.Value`-typed variables into Go values for the output `*Target`
	// struct.

	// Validate that the `name` kwarg was a string.
	name, ok := nameKwarg.(starlark.String)
	if !ok {
		return nil, errors.Errorf(
			"TypeError: argument 'name': expected str, got %s",
			nameKwarg.Type(),
		)
	}

	// Validate that the `builder` kwarg was a string.
	builder, ok := builderKwarg.(starlark.String)
	if !ok {
		return nil, errors.Errorf(
			"TypeError: argument 'builder': expected str, got %s",
			builderKwarg.Type(),
		)
	}

	// Validate that the `args` kwarg was a list of `Arg`s, and convert it
	// into a `[]Arg` for the `Target.Args` field.
	argsSL, ok := argsKwarg.(*starlark.List)
	if !ok {
		return nil, errors.Errorf(
			"TypeError: argument 'args': expected list, got %s",
			argsKwarg.Type(),
		)
	}
	args_ := make([]Arg, argsSL.Len())
	for i := range args_ {
		arg, err := starlarkValueToArg(argsSL.Index(i))
		if err != nil {
			return nil, errors.Wrapf(err, "Argument 'args[%d]'", i)
		}
		args_[i] = arg
	}

	// Validate that the `env` kwarg was a list of strings, and convert it into
	// a `[]string` for the `Target.Env` field.
	envSL, ok := envKwarg.(*starlark.List)
	if !ok {
		return nil, errors.Errorf(
			"TypeError: argument 'env': expected list, got %s",
			envKwarg.Type(),
		)
	}
	env := make([]string, envSL.Len())
	for i := range env {
		str, ok := envSL.Index(i).(starlark.String)
		if !ok {
			return nil, errors.Errorf(
				"TypeError: argument 'env[%d]': expected string; found %s",
				i,
				envSL.Index(i).Type(),
			)
		}
		env[i] = string(str)
	}

	// By now, all of the fields have been validated, so build and return the
	// final `*Target`.
	return &Target{
		Name:    string(name),
		Builder: string(builder),
		Args:    args_,
		Env:     env,
	}, nil
}

//
// Arg
//

// starlarkValueToArg takes a starlark value and parses it into an `Arg`.
func starlarkValueToArg(v starlark.Value) (Arg, error) {
	switch x := v.(type) {
	case Arg:
		return x, nil
	case starlark.String:
		return String(x), nil
	default:
		return nil, errors.Errorf(
			"Cannot convert %s into a target argument",
			v.Type(),
		)
	}
}

//
// execFile
//

// builtinWrapper DRYs up the error handling boilerplate for a starlark builtin
// function.
func builtinWrapper(
	name string,
	f func(starlark.Tuple, []starlark.Tuple) (starlark.Value, error),
) *starlark.Builtin {
	return starlark.NewBuiltin(
		name,
		func(
			_ *starlark.Thread,
			builtin *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			v, err := f(args, kwargs)
			if err != nil {
				return nil, errors.Wrapf(err, "%s()", builtin.Name())
			}
			return v, nil
		},
	)
}

// loadFunc is a signature for a starlark loader function.
type loadFunc func(*starlark.Thread, string) (starlark.StringDict, error)

// makeLoader makes a load function for a given workspace.
func makeLoader(root string, packages map[string]string) loadFunc {
	return makeLoaderHelper(
		root,
		packages,
		map[string]*cacheEntry{},
		starlark.StringDict{
			"target": builtinWrapper("target", starlarkTarget),
			"sub":    builtinWrapper("sub", starlarkSub),
			"path":   builtinWrapper("path", starlarkPath),
			"glob":   builtinWrapper("glob", starlarkGlob),
		},
	)
}

type cacheEntry struct {
	globals starlark.StringDict
	err     error
}

func makeLoaderHelper(
	root string,
	packages map[string]string,
	cache map[string]*cacheEntry,
	builtins starlark.StringDict,
) loadFunc {
	return func(
		th *starlark.Thread,
		addr string,
	) (starlark.StringDict, error) {
		e, ok := cache[addr]
		if e == nil {
			// Addr is already in the process of being loaded.
			if ok {
				return nil, errors.Errorf("Cycle in load graph")
			}

			// Add a placeholder to indicate that the addr loading is in
			// progress.
			cache[addr] = nil

			// Parse the address into a (package, module) tuple. If the
			// package is an empty string, then it's the same package as the
			// caller module.
			pkg, module := parseModule(addr)

			// Get the file path for the given (pkg, module)
			packageRoot, filePath, err := resolveModule(
				root,
				packages,
				pkg,
				module,
			)
			if err != nil {
				return nil, err
			}

			// Read the target module, if any
			data, err := ioutil.ReadFile(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					err = errors.Errorf(
						"Module '%s' not found in package '%s'",
						module,
						pkg,
					)
				}
				return nil, errors.Wrapf(err, "Loading module '%s'", module)
			}

			// Execute the target module in a new thread.
			globals, err := starlark.ExecFile(
				&starlark.Thread{
					Name: filePath,
					Load: makeLoaderHelper(packageRoot, packages, cache, builtins),
				},
				addr,
				data,
				builtins,
			)
			e = &cacheEntry{globals, err}
			cache[addr] = e
		}
		return e.globals, e.err
	}
}

func resolveModule(
	root string,
	packages map[string]string,
	pkg string,
	module string,
) (string, string, error) {
	// Look up the package's root directory in the package map.
	if pkg != "" {
		var found bool
		root, found = packages[pkg]
		if !found {
			return "", "", packageNotFoundErr(pkg)
		}
	}

	// Make sure the module doesn't escape the root (prevent reading
	// files outside of `root`).
	if strings.Contains(module, "..") {
		return "", "", moduleNotFoundErr{pkg: pkg, module: module}
	}

	// If the module doesn't have the suffix '.star', then assume it's
	// the default file in a directory.
	path := filepath.Join(root, module)
	if !strings.HasSuffix(module, ".star") {
		path = filepath.Join(path, "default.star")
	}

	return root, path, nil
}

type packageNotFoundErr string

func (err packageNotFoundErr) Error() string {
	return fmt.Sprintf("Package not found: %s", string(err))
}

type moduleNotFoundErr struct {
	pkg    string
	module string
}

func (err moduleNotFoundErr) Error() string {
	return fmt.Sprintf(
		"Module '%s' not found in package '%s'",
		err.module,
		err.pkg,
	)
}

func starlarkGlob(
	args starlark.Tuple,
	kwargs []starlark.Tuple,
) (starlark.Value, error) {
	if len(kwargs) != 0 {
		return nil, errors.Errorf(
			"Expected exactly 0 positional arguments; found %d",
			len(kwargs),
		)
	}

	globs := make(GlobGroup, len(args))
	for i, arg := range args {
		if s, ok := arg.(starlark.String); ok {
			globs[i] = string(s)
		}
	}

	return globs, nil
}

func parseModule(s string) (pkg, mod string) {
	i := strings.Index(s, ":")
	// If there is no ':', then the package is
	if i < 0 {
		return "", s
	}
	return s[:i], s[i+1:]
}

// execModule executes a module using a given load function and returns the
// global variables.
func execModule(module string, load loadFunc) (starlark.StringDict, error) {
	return load(&starlark.Thread{Name: module, Load: load}, module)
}
