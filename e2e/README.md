# End-to-end project

This standalone fixture exercises project configuration, package specifications
and bodies, cross-package calls, and transaction declarations for external
routines whose source is unavailable.
It needs no database and does not execute SQL.

From the repository root:

```sh
make build
bin/commitfence check --config e2e/.commitfence.json
```

The check prints two `CF0001 [missing-permission]` warnings and exits with `1`:

- `APP.IMPORT_API.RUN_IMPORT` calls `APP.CUSTOMER_SERVICE.SAVE_CUSTOMER`, which
  commits. The service's specification grants it permission, but the caller needs
  its own permission. The evidence points to the helper's `COMMIT`.
- The same entry procedure calls `EXT.CUSTOMER_API.FLUSH_BATCH`, whose
  `unsafe` declaration advertises transaction effects. The evidence points to
  the declaration in `externals.json`.

`EXT.CUSTOMER_API.FIND_CUSTOMER` has a trusted `safe` declaration, so the
helper's lookup produces no unknown-effect warning.

Source paths and the `externals` path are relative to `.commitfence.json`.
Diagnostics are relative to the working directory. Config discovery also works:

```sh
cd e2e
../bin/commitfence check
```

To make the project clean, add the marker immediately before the entry
procedure's declaration in `src/import_api.pks`:

```sql
CREATE OR REPLACE PACKAGE app.import_api AS
  --@commitfence
  PROCEDURE run_import;
END import_api;
/
```

Rerunning the check then prints nothing and exits with `0`. The marker applies
to the matching package body as well. It declares permission; it does not remove
transaction effects or suppress unknown behavior. A `safe` declaration is a
trusted assertion, and a clean check is not proof of transaction safety.

From the repository root, disable external declarations to see unknown-effect
warnings.

```sh
bin/commitfence check --config e2e/.commitfence.json --externals=
```

## Automated tests

The in-process CLI tests copy this project into a fresh temporary directory for
each case, then call `cli.Run` against the real files. They cover the initial
warnings, permission added to the entry specification, disabled declarations,
and a missing externals file. Keep the checked-in entry specification unannotated
so the initial warning case remains reproducible.

Run just these tests, or include them in the full suite:

```sh
go test ./internal/cli -run '^TestEndToEndProject$' -v
make test
```
