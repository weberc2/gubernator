load("modules/std", "bashTarget")
load(
    "modules/go",
    goBuild="build",
    goDependencies="dependencies",
    goTest="test",
    goFmtCheck="fmtCheck",
)

GOTOOL = bashTarget(
    name = "go-tool",
    script = sub(
        """
        set -eo pipefail
        # Create a temporary directory for our test hello.go file and
        # Go cache
        tmpDir=$(mktemp -d)
        mkdir $tmpDir/cache
        export GOCACHE=$tmpDir/cache
        echo "package main" >> $tmpDir/hello.go
        echo 'import "fmt"' >> $tmpDir/hello.go
        echo 'func main() { fmt.Println("Hello") }' >> $tmpDir/hello.go

        # Run the hello.go file with the provided Go tool, saving off the
        # exit code
        output=$($cachePath/${GoTool} run $tmpDir/hello.go)
        exitCode="$?"

        # Clean up the temporary directory
        rm -rf $tmpDir

        # Validate that the exit code was zero
        if [[ $exitCode != 0 ]]; then
            >&2 echo "FAILURE hello.go: wanted exit code 0; got '$exitCode'"
            exit 1
        fi

        # Validate that the output was as expected
        if [[ $output != "Hello" ]]; then
            >&2 echo "FAILURE hello.go: wanted output 'Hello'; got '$output'"
            exit 1
        fi

        # Write a link to the provided path into the build cache
        ln $cachePath/${GoTool} $out
        """,
        GoTool=path(".impure/go"),
    ),
    env = [],
)

dependencies = goDependencies(GOTOOL, "g8r-dependencies", ".")
sources = glob("go.mod", "go.sum", "**/*.go")
binary = goBuild(GOTOOL, "g8r-binary", dependencies, sources)
tests = goTest(GOTOOL, "g8r-tests", dependencies, sources)
gofmt = goFmtCheck("/Users/weberc2/.nix-profile/", "g8r-gofmt-check", sources)

ci = bashTarget(
    name = "ci",
    script = sub(
        """
        set -eo pipefail
        echo 'Tests: ${Tests}' >> $out
        echo 'Gofmt: ${Gofmt}' >> $out
        """,
        Tests = tests,
        Gofmt = gofmt,
    ),
    env = [],
)

__DEFAULT__ = binary
