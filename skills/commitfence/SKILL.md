---
name: commitfence
description: Check local PL/SQL transaction capabilities with CommitFence; trace commit, rollback, and unknown-effect findings before proposing changes.
---

# CommitFence

Use this skill when reviewing or changing PL/SQL transaction boundaries, checking
transaction capabilities through callers, or investigating CommitFence findings.
CommitFence reads local files without connecting to Oracle or executing SQL.

## Check

1. Use `commitfence` on `PATH` or an executable path supplied by the user. If
   neither is available, ask for the path to an installed executable. Run
   `commitfence check --help` for the installed version's options.
2. Work from the PL/SQL project's intended working directory. Inspect its
   `.commitfence.json`, if present, and any selected externals file. CommitFence
   looks for the default configuration only in the working directory. Configured
   `paths` and `externals` resolve from the config file's directory. CLI paths
   resolve from the working directory. Include relevant callers, declarations,
   and bodies in the source scope.
3. Use the command that matches the project's inputs, substituting actual paths.

   ```sh
   commitfence check src/ --color never
   commitfence check src/ extra.sql --externals externals.json --color never
   commitfence check --config path/to/.commitfence.json --color never
   ```

   With no positional paths, CommitFence scans the configured paths if present
   and the working directory otherwise. Explicit paths replace configured paths.
   Directory scans recurse using configured extensions or the defaults `.sql`,
   `.pks`, `.pkb`, `.pls`, `.tps`, and `.tpb`. Explicit files may have any extension.
   `--externals FILE` selects a file of transaction declarations and
   `--externals=` clears a configured one. Capture stderr and preserve the
   command's exit status.

| Exit | Interpretation |
| --- | --- |
| 0 | No warnings or errors in the recognized forms and selected inputs. This is not a transaction-safety proof. |
| 1 | Warnings: investigate missing permission and unknown effects. |
| 2 | Invocation, input, configuration, annotation, or recognized analysis errors, including declaration conflicts. Errors take precedence over warnings. Valid independent inputs may still have findings. |

## Investigate and change

- Follow each diagnostic's location, routine/callee, origin, and representative
  evidence through the relevant source. Commit, full rollback, and rollback to a
  savepoint count as effects; ordinary DML alone does not. Effects propagate
  through calls even when a callee lacks permission.
- For unknown effects, check source coverage, qualified names, and actual
  dependency behavior before choosing a fix. Available source is analyzed even
  when a transaction declaration exists.
- Never add `--@commitfence` or a transaction declaration merely to
  silence findings. Permission declares an intentional API capability; `safe`
  asserts preservation of the caller's transaction and requires evidence.
- If permission is intended, the exact standalone `--@commitfence` line belongs
  in the immediately preceding comment block. It grants both commit and rollback
  permission, advertises potential effects, and covers all overloads of that
  qualified name. It does not grant permission to callers or nested helpers.
- `PRAGMA AUTONOMOUS_TRANSACTION` permits `COMMIT`, `ROLLBACK`, and `ROLLBACK TO`
  written directly in the definition that owns the pragma. Call sites and
  ordinary nested helpers still follow the usual rules. Effects and unknowns
  stop at the autonomous boundary. Do not introduce the pragma merely to clear
  findings.
- After an authorized change, rerun the relevant check with the same configuration
  and enough caller/dependency scope to assess propagation. Report the command,
  scope, status, findings, and any unverified assumptions. Do not claim that a
  clean result validates arbitrary PL/SQL or proves transaction safety.

## External transaction behavior

Use a version-1 JSON file such as `externals.json` for dependencies whose source
is unavailable or uneditable. Each transaction declaration uses a
`SCHEMA.ROUTINE` or `SCHEMA.PACKAGE.ROUTINE` name and covers all overloads of
that identity. Unquoted names are case-insensitive and quoted names retain
their identity.

```json
{
  "version": 1,
  "routines": [
    {"name": "EXT.LOOKUP_API.FIND_CUSTOMER", "transaction": "safe"},
    {"name": "EXT.BATCH_API.FLUSH", "transaction": "unsafe"}
  ]
}
```

`safe` asserts that the routine preserves its caller's transaction. `unsafe`
declares possible effects on that transaction and grants permission in
CommitFence. If available source contradicts a `safe` declaration, CommitFence
reports `CF1004 [declaration-conflict]`.

## Analysis limits

CommitFence recognizes standalone, packaged, and nested procedures and functions.
Unresolved calls and dynamic SQL produce unknown effects. Dynamic SQL stays
unknown even with a literal statement. Resolution uses lexical and package
context and exact qualified names.
Recognized syntax excludes triggers, object-type methods, package initialization
effects, synonyms, database links, and conditional compilation that requires
unavailable context.
