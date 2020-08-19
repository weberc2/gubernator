# README

## Introduction

Gubernator, aka g8r (informally pronounced "gator"), is an experimental,
general-purpose build tool. The primary goals of g8r are to allow project
maintainers to easily specify instructions for building projects and to allow
project users to build the project easily with minimal installation of system
dependencies, etc (reproducibility).

The existing landscape of build systems seems to be
divisible into two camps: reproducible and nonreproducible. Existing
reproducible build systems (Bazel, Buck, Nix, etc) impose a steep learning
curve on maintainers and defining a build target is often a tedious,
time-consuming ordeal with the upside being a high degree of reproducibility
beyond what is valuable to many projects. Existing non-reproducible build tools
(e.g., Make, CMake, Autotools, etc) make it very easy to define targets, but
they leave it completely up to users to install the right versions of
dependency packages onto the system (where they might conflict with other
versions that other user programs depend on if they're even available in the
system package manager at all). These build tools are more common and they
impose such a steep learning curve on users that the ecosystem has evolved
package maintainers who specialize in building these packages for distribution
via package managers (e.g., apt, yum, etc) for end-users (note that package
maintainers aren't specialists in 'building Autotools projects' as much as they
specialize in 'building a *particular* Autotools project').

To oversimplify, the landscape offers reproducible build tools which are 100%
friendly for users but 0% friendly for maintainers *or* non-reproducible build
tools which are 100% friendly for maintainers but 0% friendly for users. G8r
represents the belief that this isn't a zero-sum game--that build tools can
exist which are 80% friendly for maintainers and 80% friendly for users.

G8r aspires to achieve greater friendliness for maintainers by allowing
maintainers to reference existing third-party definitions for dependency
packages or else to punt on definitions for these dependency packages
altogether thereby shifting the burden for certain dependencies onto the user.
Often these can be the most difficult-to-package, low-level dependencies which
are very likely to be installed on the user's system already (e.g., coreutils)
or readily available in the user's package manager (e.g., gcc, clang, etc).
Over time if the project in question is successful and attracts a wide
user-base, the maintainer can go through the trouble of creating package
definitions for these low-level packages to increase the reproducibility.

To reduce the burden for users versus the non-reproducible category, g8r
encourages package developers to provide or reference package definitions for
as many packages as possible, especially those packages which are unlikely to
be installed or available in the user's package manager at the right version.
Further, over time (as previously discussed) as the user base grows, the
maintainer should provide or reference package definitions that cover ever-more
of the dependency tree such that the scope of packages for which the user is
responsible approaches zero.

For both groups, g8r aspires to furnish a friendlier interface than tools in
either category (Bazel, Nix, CMake, Autotools, Make, etc).

## Contact

**G8r is still in its infancy, and may never take off; however, if you're
interested in contributing or just talking about the project (or if you have
questions), feel free to reach out at weberc2@gmail.com.**

## Design

At its core, g8r has a notion of targets which are an abstract definition for
how to build a given artifact. These targets may depend on each other such that
the artifact for one target is used to build another. For example, consider a
static-site blog--one target might represent the static-site-generator itself
while the downstream target might represent invoking the static-site-generator
on some markdown, template, and config files to produce the output static HTML
payload.

Artifacts are stored in a build cache with a key that is based on the hash of
the target and any files or targets it references. Targets are only rebuilt if
there is no artifact in the build cache for the current hash, which is to say
that builds are incremental.

Targets are defined in a Python-like language called Starlark.

```star
# Results in a single-file artifact whose contents are 'hello, world'
hello = target(
    name = "hello",
    builder = "bash",
    env = [],
    args = [ "-c", "echo 'hello, world' > $out" ],
)
```

In addition to defining individual targets, we can also use Starlark functions
which allow us to stamp out many targets of a particular 'type':

```star
def singleFileTarget(name, contents):
    """Defines a single-file target.

    Args:
        name: The name of the target.
        contents: The contents of the single-file artifact.

    Returns: A target representing a single-file artifact.
    """
    return target(
        name = name,
        builder = "bash",
        env = [],
        # Use the `sub()` builtin to render `contents` into the bash script.
        args = ["-c", sub("echo '${Contents}' > $out", Contents=contents)],
    )

# Define two single-file targets with contents 'hello, world' and 'goodbye,
# world', respectively.
hello = singleFileTarget("hello", "hello, world")
goodbye = singleFileTarget("goodbye", "goodbye, world")
```

Eventually, as our project becomes larger, it makes sense to separate out our
Starlark code into many separate files. In this case, we'll put the definition
for `singleFileTarget()` into `single-file-target.star` and reference it:

```star
load("single-file-target.star", "singleFileTarget")

hello = singleFileTarget("hello", "hello, world")
goodbye = singleFileTarget("goodbye", "goodbye, world")
```

Similarly, we can use the output of one target to build another. Consider this
example where we load the definition for a Go tool chain target and use it to
build a Go project:

```star
load("modules/go", goTool="toolchain")

my_project = target(
    name = "my-project",
    builder = "bash",
    args = [
        "-c",
        sub(
            """
            set -eo pipefail
            # cd to the source/project directory
            cd $cachePath/${Sources}

            # Invoke `go build` and write the result to the special `$out`
            # file location.
            $cachePath/${GoTool} build -o $out
            """,
            GoTool=goTool,
            Sources=glob("go.mod", "go.sum", "./**/*.go"),
        ),
    ],
    env = [],
)
```

Based on this definition, g8r is able to determine that `goTool` is a
dependency of `my_project` and thus it must be built before `my_project` can be
built.

Note that g8r has no notion of static-site-generators or Go projects--only
targets expressed in Starlark files. g8r is responsible for determining when a
given target needs to be rebuilt, but the actual definition for a target and
its relationship to other targets is determined by evaluating the Starlark
scripts.
