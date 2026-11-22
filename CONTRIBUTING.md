# Contributing to CommitFence

Start with the [user guide](docs/usage.md) for current CLI behavior and the
[design overview](docs/design.md) for the analysis pipeline, package boundaries,
and current limitations.

## Local setup

Install Git, Make, and Go. See [go.mod](go.mod) for the minimum Go version and
preferred toolchain. Clone the repository and run commands from its root:

```sh
go mod download
make test
make build
bin/commitfence --help
```

Tests run in process and do not need a database, Oracle client, or a built
CommitFence executable.

## Make a change

Keep each change focused on one behavior or problem. Preserve the boundary
between `internal/analysis`, which returns structured reports, and `internal/cli`,
which handles arguments, configuration, rendering, and exit status.

Format changed Go files with `gofmt`. For a focused test run, use the relevant
package and test name, for example:

```sh
go test ./internal/analysis -run TestCheckInputs
go test ./internal/cli -run TestConfigAndCLIOverrides
```

For analysis changes, extend the existing acceptance tests around
`analysis.Check` with small real-file fixtures. Assert the observable report:
diagnostic kinds, locations, routines, evidence, and deterministic ordering.
Reuse the [fixture corpus](internal/analysis/testdata/README.md) where possible;
add a fixture for a distinct supported behavior or a regression. Keep CLI tests
focused on configuration, rendering, and status selection. Fixture tests must not
build or launch the executable.

The CLI help has one reviewed golden file. When an intentional command, flag, or
help change affects it, regenerate only that golden and inspect the diff:

```sh
go test ./internal/cli -run '^TestCommandLine$' -update
git diff -- internal/cli/testdata/commandline.golden
```

Update the user guide for user-visible changes and the design overview when
architecture or supported analysis behavior changes.

## Validate the result

Run the repository checks before submitting:

```sh
make check
make vulncheck
make build
git diff --check
```

Individual targets are available while iterating:

| Target | Purpose |
| --- | --- |
| `make test` | Run the full Go test suite. |
| `make lint` | Check formatting and run `go vet` and Staticcheck. |
| `make test-race` | Run tests with the race detector. |
| `make vulncheck` | Check dependencies for known vulnerabilities. |
| `make check` | Run lint and race tests. |

Staticcheck and govulncheck are declared as tools in `go.mod`; the Make targets
run them with `go tool`. To update a tool, use `go get -tool package@version`
with its package path from the `tool` directive and the chosen version, then run
`go mod tidy` and the repository checks.

The race detector requires a supported platform and C compiler. The vulnerability
check needs network access to download its tool and query the vulnerability
database. Include any checks you could not run and the reason in your pull
request.

## Submit a pull request

Use Conventional Commit subjects, such as `fix: preserve diagnostic ordering`,
`feat: support a new source form`, or `docs: clarify transaction declarations`.
Describe the problem, the resulting behavior, and the checks you ran. Include a
small before/after example when it helps explain a behavior change, and link the
relevant issue or design decision when one exists. Keep generated artifacts and
unrelated cleanup out of the change unless they are needed for it.
