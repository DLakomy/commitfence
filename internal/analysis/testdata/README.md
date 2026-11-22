# Acceptance fixture map

The external-package tests in `../check_test.go` call `analysis.Check` with real
files. They do not build or invoke an executable. This map records the compact
corpus covering the [documented behavior](../../../docs/usage.md) and
[analysis design](../../../docs/design.md).

| Risk | Existing coverage |
| --- | --- |
| Direct effects, markers, DML/savepoints, comments/literals, named/unnamed boundaries | `extraction`; extraction and statement-context tests |
| Structural ownership across routine/package endings, blocks, labels, control constructs, and CASE expressions; malformed recovery | `extraction/unnamed.sql` and `unnamed_malformed.sql`; exact sites, lexical identities, transitive evidence, isolation, and independent-input recovery |
| Specification/body permission, overloads, nested scopes, calls in SQL | `graph`, `resolution`; exact site and evidence assertions in graph tests |
| Recursion and propagation through unpermitted wrappers | `graph`, `unknown`; generated real-file long-chain test |
| Autonomous effects/unknowns and mixed overloads | `autonomous`, `forward_isolation`, `graph`; internal and outward findings checked separately |
| Dynamic execution, unresolved/declaration-only calls, explicit built-ins and source precedence | `unknown`, `builtin_shadow`, `resolution` |
| JSON transaction declarations, conflicts, unknown preservation, isolation | `externals`; invalid documents written as real temporary files |
| Invalid inputs, aliases, limits and independent valid inputs | `sources` and temporary real files; input tests |
| Multiple call sites and graph-path deduplication | Graph behavior test asserts exact ordered sites, including two calls to the same leaf on one line; graph evidence test asserts the representative diamond chain |
| Reordered paths and complete warning/error reports | Graph/extraction ordering tests and report-severity cases compare complete reports, including repeated paths, findings, errors and evidence |

Report-severity comparisons reverse and repeat inputs, retaining mixed input-error
and declaration-conflict results. Separate in-process CLI tests cover configuration,
rendering, status selection, and generated help; see
[Contributing](../../../CONTRIBUTING.md) for the verification workflow.

Passing fixtures establish behavior for recognized forms, not complete PL/SQL
validation. The [user guide](../../../docs/usage.md#recognized-forms-and-limits)
describes the current limits.
