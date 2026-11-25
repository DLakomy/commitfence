# Using CommitFence

## CLI

After [installing CommitFence](../README.md#install), run a check on your source.

```sh
commitfence check src/ extra.sql
commitfence check src/ --externals externals.json
commitfence check --help
```

Directories are scanned recursively for `.sql`, `.pks`, `.pkb`, `.pls`,
`.tps`, and `.tpb`, ignoring case. This is the default list.
You can configure other extensions.

Diagnostics go to standard error. They include a location and, where
applicable, a call chain to the source of the effect.

| Exit status | Meaning |
| --- | --- |
| `0` | No warnings or errors |
| `1` | Warnings, such as missing permission or unknown behavior |
| `2` | Errors in the invocation, inputs, configuration, or analysis. Takes precedence over warnings. |

### Project configuration

`commitfence check` reads an optional `.commitfence.json` in the working
directory. Use `--config FILE` to select another file. Only `version` is required.
For now, the only possible `version` is `0`. Stability of these file formats is not
guaranteed for this version.

```json
{
  "version": 0,
  "paths": ["src"],
  "externals": "externals.json",
  "extensions": ["sql", "pks", "pkb", "pls", "tps", "tpb"],
  "path_style": "relative",
  "color": "auto"
}
```

Paths in the file are relative to its directory. CLI paths are relative to the
working directory. CLI flags override settings and positional paths replace the
`paths` list. When neither source of paths is set, CommitFence scans the working
directory.

| Flag | Behavior |
| --- | --- |
| `--externals FILE` | Load transaction declarations. Use `--externals=` to clear the setting. |
| `--extensions sql,pks` | Replace the directory-scan extension list. |
| `--path-style absolute` | Show absolute paths. The default is relative to the working directory. |
| `--color auto\|always\|never` | Control diagnostic colors. `auto` uses color on a terminal unless `NO_COLOR` is set to a nonempty value or `TERM=dumb`. |

Relative diagnostics use the resolved working directory when it is reached
through a symbolic link.

## Basic case

Save this as `batch.sql` and run `commitfence check batch.sql`.

```sql
--@commitfence
CREATE OR REPLACE PROCEDURE app.flush_batch IS
BEGIN
  COMMIT;
END;
/

CREATE OR REPLACE PROCEDURE app.run_batch IS
BEGIN
  app.flush_batch;
END;
/
```

The check exits with `1` and reports `missing-permission` at the call in
`APP.RUN_BATCH`. If that procedure is meant to commit, add `--@commitfence`
immediately above its declaration. The example then exits with `0`.

Put the exact marker on its own line in the comment block immediately above
the routine declaration. It grants commit and rollback permission and advertises
a potential effect to callers, even with an empty body. It covers all overloads
of that name and matching package specification and body declarations.
Callers and nested helpers need their own permission. Unknown behavior still
produces a warning.

## Autonomous transactions

```sql
CREATE OR REPLACE PROCEDURE app.write_audit IS
  PRAGMA AUTONOMOUS_TRANSACTION;
BEGIN
  INSERT INTO app.audit_log (message) VALUES ('Batch started');
  COMMIT;
END;
/

CREATE OR REPLACE PROCEDURE app.start_batch IS
BEGIN
  app.write_audit;
END;
/
```

`APP.WRITE_AUDIT` commits its own transaction. Its pragma permits `COMMIT`,
`ROLLBACK`, and `ROLLBACK TO` statements written directly in its body. Effects
and unknown behavior stop at the autonomous transaction boundary. The example
passes `commitfence check` with exit status `0`.

Inside an autonomous routine, calls that can affect its transaction still
require permission, and unknown behavior still produces warnings. Ordinary
nested helpers and overloads need their own permission for transaction effects.

## Rollback and savepoints

In ordinary routines, both `ROLLBACK` and `ROLLBACK TO [SAVEPOINT] name` require
permission and count as potential effects for callers. For example, a routine
containing `ROLLBACK TO before_batch;` needs the same marker as a routine
containing `COMMIT`. Setting a savepoint or running ordinary DML alone needs no
marker. Effects in branches and exception handlers count too.

## External routines

A call to a dependency with unavailable source is generally reported as
`unknown-effect`. CommitFence recognizes a small [set of built-ins](../internal/analysis/graph.go).
For other dependencies, put their transaction behavior in `externals.json`
and pass `--externals externals.json`.

```json
{
  "version": 0,
  "safe": ["EXT.CUSTOMER_API.LOOKUP"],
  "unsafe": ["EXT.BATCH_API.FLUSH"]
}
```

- `safe` asserts that the routine preserves the caller's transaction. It can
  still perform DML.
- `unsafe` grants the external routine permission and declares a potential
  effect. A caller of `EXT.BATCH_API.FLUSH` needs its own permission.

Either array can be omitted when empty. Use `[]` for an explicit empty array.
Use `SCHEMA.ROUTINE` or `SCHEMA.PACKAGE.ROUTINE`. Each entry covers all
overloads. Unquoted names are case-insensitive. If source is available,
CommitFence checks it and reports `declaration-conflict` when an outward effect
or unknown behavior contradicts a `safe` declaration.

## Recognized forms and limits

CommitFence checks standalone, packaged, and nested procedures and functions.
An ordinary nested helper can contribute effects to its enclosing routine even
if it is never called. Overloads share one transaction summary per name.

- **Malformed package or routine.** Recognized boundary problems, such as a
  missing or mismatched `END`, report `incomplete-analysis` and exit with `2`.
  Analysis continues on independent inputs and can recover at the next
  `CREATE` statement or standalone `/`. Both `END;` and `END name;` are
  supported.
- **Dynamic SQL.** `EXECUTE IMMEDIATE` and dynamic `OPEN ... FOR` are reported
  as unknown, even when the SQL is a string literal.
- **Coverage.** This proof of concept recognizes a subset of PL/SQL. Syntax
  outside the supported forms can lead to missed effects or false positives.
  Triggers, object-type methods, package initialization effects, synonyms, and
  database links are outside its scope.

### Schema names

Include the schema in standalone and package declarations, for example
`CREATE PACKAGE app.batch_api` and `CREATE PACKAGE BODY app.batch_api`.
Use `app.batch_api.flush` for calls that need to identify the schema explicitly.
CommitFence resolves shorter calls using lexical scope, package scope, or the
schema written on the enclosing object. It reads schema names from the SQL
source. It treats declarations with an omitted schema and their schema-qualified
forms as distinct names. Unresolved calls report `unknown-effect`.

See the [end-to-end example](../e2e/README.md) for a runnable project and the
[design notes](design.md) for details on the analysis.
