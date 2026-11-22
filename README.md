# CommitFence

Your first line of defence against accidental commits in PL/SQL.

In PL/SQL, a function can commit the caller's pending transaction with a single
`COMMIT` statement. From the call site, it can be hard to tell that the transaction
has ended (the resulting bugs are really not funny...). CommitFence is a linter that
runs locally, without a database connection, and can help you avoid such accidents.

What inspired me is the way Scala can track side effects. In Eezo, for example,
the type system distinguishes a read-only `read` scope from a `transact` scope
that also allows writes. That gave me an idea: can we track commits
through PL/SQL calls? Vide [Introducing Eezo](https://rockthejvm.com/articles/introducing-eezo)

[Full documentation of CommitFence](docs/usage.md)

## Install

Use `go install` to install CommitFence. See [go.mod](go.mod) for the minimum Go
version and preferred toolchain.

```sh
go install github.com/DLakomy/commitfence/cmd/commitfence@latest
```

With Go's bin directory on your `PATH`, run `commitfence check path/to/src`.

To install from a checkout, run:

```sh
git clone https://github.com/DLakomy/commitfence.git
cd commitfence
go install ./cmd/commitfence
```

To build a local binary, use `make build` or
`go build -o bin/commitfence ./cmd/commitfence` from the checkout.

CommitFence is still a proof of concept. I’ll revisit prebuilt binaries once it’s
more stable and has had more testing.

## Why Go?

I considered writing it in PL/SQL and running it inside Oracle, where it could
inspect database objects directly. For this proof of concept I chose a static
linter that can run against a checkout. Go gives it fast startup, easy
cross-compilation, and a short dependency chain.

[Contributing](CONTRIBUTING.md) · [Design](docs/design.md) · [MIT License](LICENSE)
