load("modules/std", "bashTarget")

def fmtCheck(goRoot, name, sources):
    return bashTarget(
        name = name,
        script = sub(
            """
            set -eo pipefail
            badFiles=$(${GoRoot}/bin/gofmt -l $cachePath/${Sources})
            if [[ -n $badFiles ]]; then
                >&2 echo '`gofmt` needs to be run on the following files:'
                >&2 echo "$badFiles"
                exit 1
            fi
            touch $out
            """,
            GoRoot = goRoot,
            Sources = sources,
        ),
        env = [],
    )


def test(goTool, name, dependencies, sources):
    return bashTarget(
        name = name,
        script = sub(
            """
            set -eo pipefail
            cd "$cachePath/${Sources}"
            GOCACHE="$cachePath/${Dependencies}/gocache" \
                GOPATH="$cachePath/${Dependencies}/gopath" \
                $cachePath/${GoTool} test -v | tee $out
            """,
            GoTool = goTool,
            Sources = sources,
            Dependencies = dependencies,
        ),
        env = [],
    )

def build(goTool, name, dependencies, sources):
    """Builds a Go package.

    This uses the go.mod and go.sum files to build an intermediate
    "dependencies" target, which represents the downloading and caching of the
    dependencies. Since downloading the dependencies is expensive, we want to
    do it as infrequently as possible, and this implementation only downloads
    dependencies when the go.mod or go.sum files have changed (as opposed to
    the more frequently changed Go source files). Ideally the system would only
    download the individual dependencies which have been modified; however,
    that either requires an update to the build tool to allow Starlark modules
    to parse go.mod files into individual targets per dependency (this is
    probably a reasonable addition, but it needs design and implementation *or*
    it would require users to model the dependency tree explicitly as targets
    (too tedious).

    Args:
        goTool: The Go tool target which is used to build the target.
        name: The name of the target.
        dependencies: The target containing the project state after downloading
            the dependencies. See `dependencies()` for more information.
        sources: The source files including the go.mod and go.sum files.

    Returns: A target whose output is the binary build artifact.
    """

    return bashTarget(
        name = name,
        script = sub(
            """
            set -eo pipefail
            cd "$cachePath/${Sources}"
            GOCACHE="$cachePath/${Dependencies}/gocache" \
                GOPATH="$cachePath/${Dependencies}/gopath" \
                $cachePath/${GoTool} build -o $out
            """,
            GoTool = goTool,
            Sources = sources,
            # We want to cache the dependencies locally so we only rebuild them
            # when the go.mod or go.sum files change (as opposed to the more
            # frequent changes to the source files). The output of this target
            # is a directory containing 2 subdirectories (gopath and cache)
            # which represent the GOPATH and GOCACHE environment variables
            # for the toplevel 'build' target.
            Dependencies = dependencies,
        ),
        env = [],
    )

def dependencies(goTool, name, moduleRoot):
    return bashTarget(
        name = name,
        script = sub(
            """
            set -eo pipefail
            mkdir -p state/gopath
            mkdir -p state/cache
            (
                cd "$cachePath/${GoModSum}" && \
                GOPATH="$cachePath/${Dependencies}/state/gopath" \
                GOCACHE="$cachePath/${Dependencies}/cache" \
                $cachePath/${GoTool} mod download
            )
            mv state $out
            """,
            GoModSum = glob(
                "{}/go.mod".format(moduleRoot),
                "{}/go.sum".format(moduleRoot),
            ),
            GoTool = goTool,
        ),
        env = [],
    )
