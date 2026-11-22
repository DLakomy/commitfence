# CommitFence skill

[`commitfence/SKILL.md`](commitfence/SKILL.md) is a small, model- and
harness-independent workflow for checking PL/SQL transaction capabilities. It
uses the ordinary CommitFence CLI; it requires no agent framework, service,
database credentials, or Oracle connection.

## Install

1. Put an installed CommitFence executable on `PATH` or provide its absolute
   path to the assistant. For source builds, see [the user guide](../docs/usage.md).
2. For a harness that discovers `SKILL.md` directories, copy or symlink the
   `skills/commitfence` directory into that harness's documented project or user
   skill location. There is no universal installation directory; use your
   harness's instructions. The directory contains the complete skill.
3. For a harness without skill discovery, load or paste
   `commitfence/SKILL.md` as task instructions. Tell the assistant where to
   run the check in your PL/SQL project and provide the executable path if it
   is outside `PATH`.

Keep installed copies synchronized when upgrading CommitFence. The skill does
not install the binary, alter project configuration, or grant transaction
permission automatically.

## Use

Ask the assistant to use the CommitFence skill to check a named PL/SQL project
or investigate a diagnostic. For example:

> Use the CommitFence skill to check `src/`, trace the warnings through callers,
> and explain which transaction boundaries need a design decision.

Direct CLI use remains available:

```sh
commitfence check src/ --color never
```

Diagnostics go to stderr. Status `0` means no warnings or errors, `1` means
warnings, and `2` means invocation/input/configuration or analysis errors; errors
take precedence. A clean run is not proof of transaction safety. The workflow
requires investigating findings rather than adding permissions or `safe`
declarations simply to make the command pass.
