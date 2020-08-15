package main

import (
	"testing"
)

func TestResolveModule(t *testing.T) {
	const root = "/root"
	for _, testCase := range []struct {
		name       string
		pkg        string
		module     string
		packages   map[string]string
		wantedRoot string
		wantedPath string
		wantedErr  error
	}{{
		name:       "current package, default module",
		pkg:        "",
		module:     "",
		packages:   nil,
		wantedRoot: "/root",
		wantedPath: "/root/default.star",
		wantedErr:  nil,
	}, {
		name:       "current package, explicit module",
		pkg:        "",
		module:     "foo.star",
		packages:   nil,
		wantedRoot: "/root",
		wantedPath: "/root/foo.star",
		wantedErr:  nil,
	}, {
		name:       "current package, invalid module",
		pkg:        "",
		module:     "../foo.star",
		packages:   nil,
		wantedRoot: "",
		wantedPath: "",
		wantedErr:  moduleNotFoundErr{pkg: "", module: "../foo.star"},
	}, {
		name:       "foreign package",
		pkg:        "foo",
		module:     "",
		packages:   map[string]string{"foo": "/modules/foo"},
		wantedRoot: "/modules/foo",
		wantedPath: "/modules/foo/default.star",
		wantedErr:  nil,
	}, {
		name:       "unknown package",
		pkg:        "bar",
		module:     "",
		packages:   nil,
		wantedRoot: "",
		wantedPath: "",
		wantedErr:  packageNotFoundErr("bar"),
	}} {
		t.Run(testCase.name, func(t *testing.T) {
			packages := testCase.packages
			if packages == nil {
				packages = map[string]string{}
			}

			packageRoot, path, err := resolveModule(
				root,
				testCase.packages,
				testCase.pkg,
				testCase.module,
			)
			if err != testCase.wantedErr {
				t.Errorf(
					"Wanted error '%v'; got '%v'",
					testCase.wantedErr,
					err,
				)
			}
			if packageRoot != testCase.wantedRoot {
				t.Errorf(
					"Wanted root '%s'; got '%s'",
					testCase.wantedRoot,
					packageRoot,
				)
			}
			if path != testCase.wantedPath {
				t.Errorf(
					"Wanted path '%s'; got '%s'",
					testCase.wantedPath,
					path,
				)
			}
		})
	}
}
