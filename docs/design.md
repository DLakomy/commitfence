# CommitFence design

CommitFence reviews potential transaction effects in local PL/SQL. It runs
without an Oracle connection, SQL execution, or changes to source files.

## Architecture

The CLI resolves project settings, presents diagnostics, and chooses the exit
status. The analysis engine reads the selected source and any external
transaction declarations and returns a structured report. This boundary keeps
diagnostic formatting and invocation defaults independent of analysis.

Project settings and the externals file are versioned separately. Settings
select inputs and presentation for a run. Transaction declarations describe
routine behavior and can be used across projects. Invalid project settings stop
the CLI before analysis. Errors in the externals file can be reported alongside
findings from independent source inputs.

## Analysis approach

The static analysis recognizes a subset of PL/SQL syntax. It uses routine
boundaries to attribute direct effects and builds a call graph to track effects
across routines. The narrow scope keeps the analyzer small and simpler to
maintain. Unrecognized syntax can hide effects, and approximate recognition can
produce false positives. See the
[recognized forms and limits](usage.md#recognized-forms-and-limits).

Call resolution uses lexical and package scope, explicitly written schema
context, and exact routine identities to avoid attributing the behavior of an
unrelated routine. Resolved source takes precedence over built-in classifications.
Transaction declarations can describe routines whose bodies are unavailable.
The analyzer checks available bodies for conflicts with `safe` declarations.

## Transaction model

Permission records a routine's explicit opt-in to transaction effects.
Potential effects describe what it may do, and uncertainty records behavior the
analysis cannot establish. An inline marker or `unsafe` declaration grants
permission and advertises a potential effect even without an observed statement.
Known effects and uncertainty can coexist, including in routines with permission.

Effects and uncertainty propagate to callers across files and recursive calls,
regardless of permission. Routine specifications and bodies share a summary by
qualified name, including overloads. Transaction declarations use the same
qualified name. An overload whose own body is safe can still produce warnings.
Attributing an ordinary nested helper's effects to the enclosing routine can
produce a warning even when the helper is never called. A helper with potential
effects still needs its own permission.

A routine's own `PRAGMA AUTONOMOUS_TRANSACTION` permits `COMMIT`, `ROLLBACK`,
and `ROLLBACK TO` statements that belong to that definition. Calls with
potential or unknown effects and statements in ordinary nested helpers are
checked under the usual rules. Effects and uncertainty stop at the autonomous
transaction boundary. Isolation applies per definition, so ordinary overloads
can still affect callers.

Oracle documents the transaction semantics behind this model.
[Autonomous transactions](https://docs.oracle.com/en/database/oracle/oracle-database/26/lnpls/autonomous-transactions.html),
[COMMIT](https://docs.oracle.com/en/database/oracle/oracle-database/26/sqlrf/COMMIT.html),
[DML statements and transactions](https://docs.oracle.com/en/database/oracle/oracle-database/26/tdddg/dml-and-transactions.html).

## Report guarantees

The report lists successfully read inputs and keeps findings separate from
errors. An input or analysis error can coexist with findings from independent
inputs. Entries appear in deterministic order for reproducible checks.

Diagnostics use stable identifiers and canonical absolute paths. Where
applicable, a diagnostic points to an origin or shows a representative call
chain. Multiple paths to the same finding produce one diagnostic, while distinct
affected routines and call sites retain their own findings.

## Open questions

1. **Marker placement.** The POC allows both spec and body. Should public routines
   prefer or require the spec, should all markers go in the body, or should the
   API contract and implementation permission have separate markers?
2. **Marker name.** Keep `--@commitfence` or choose a name that makes its
   transaction semantics explicit?
3. **Inline `safe`.** Add a source-level equivalent of external `safe`? How should
   it handle available bodies, unknown effects, and conflicting declarations?
