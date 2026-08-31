# Contributing

Issues are welcome. For a change of any size, open an issue before you open a pull
request, so neither of us writes something the other was going to reject.

## Build and test

You need Go 1.24 or later. Nothing else — no database, no services, no network.

Clone the repo and run the whole suite:

```sh
git clone https://github.com/cheese-cracker/tray
cd tray
make check
```

`make check` runs four things, and all four have to pass:

| Target | What it does |
|---|---|
| `make fmt` | `gofmt -l`, which fails if any file is unformatted |
| `make vet` | `go vet ./...` |
| `make test` | `go test -timeout 120s ./...`, including the terminal-interface tests |
| `make flows` | `scripts/check-tray.sh`, which drives the built binary against real files |

To run tray against scratch data rather than your own, point `TRAY_HOME` somewhere else:

```sh
make build
TRAY_HOME=/tmp/tray-scratch ./build/tray init
TRAY_HOME=/tmp/tray-scratch ./build/tray
```

## Before you push

Enable the hook that refuses a direct push to `main`:

```sh
make hooks
```

GitHub cannot enforce this for the repo, so the hook is the only guard. Open a pull
request instead. To push to `main` deliberately, set `ALLOW_MAIN_PUSH=1`.

## What a change needs

**A flow test, if it changes behaviour.** [FLOWS.md](FLOWS.md) lists what must keep
working and names the test that holds each promise. The `internal/flows` package parses
both and fails the build when they disagree, so a new promise without a test — or a new
`TestFlow…` without a row — breaks `make test`.

Add the test first, then the row:

- Terminal interface: a `TestFlow…` function in `internal/ui/flows_test.go`
- Command line: a `head_ "F… · …"` block in `scripts/check-tray.sh`

**A check that the test can fail.** Break the code on purpose and confirm the new test
goes red. Three assertions in this repo's history passed for the wrong reason, and each
one was caught this way and no other.

**A regenerated golden, if it changes what a screen looks like.** Run
`go test ./internal/ui -run TestScreens -update` and read the diff before you commit it.
Goldens record what happened; they do not object to it.

## What a change does not need

Comments explaining what the code does, or a docstring on every function. The code here
comments *why* — a decision, a trade-off, a bug that a shape prevents — and says nothing
where there is nothing to say.

## Decisions

[DECISIONS.md](DECISIONS.md) is a numbered log of every choice, including the reversed
ones, with the reasoning attached. If you are about to argue with something in the code,
it is probably already in there, and the row will tell you whether the argument is new.

Reopening a settled decision is allowed. That is why the rejected ones are still written
down.
