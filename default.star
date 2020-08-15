load("modules/std", "bashTarget")
load(
    "modules/go",
    goBuild="build",
    goDependencies="dependencies",
    goTest="test",
    goFmtCheck="fmtCheck",
)

GOTOOL = "/Users/weberc2/.nix-profile/bin/go"
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
