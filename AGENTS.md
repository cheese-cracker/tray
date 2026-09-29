# Working on tray

Read [README.md](README.md) for what it is, [DECISIONS.md](DECISIONS.md) for why, [FLOWS.md](FLOWS.md)
for what must keep working, and [CONTRIBUTING.md](CONTRIBUTING.md) for the bar (`make check`).

Everything committed here is generic. A maintainer's own setup — where their data lives, which
remote store their config points at, how they build the binary they use day to day — belongs in
`PERSONAL_INSTRUCTIONS.md` at the repo root, which is gitignored and may not exist in your checkout.
Read it if it is there; never copy from it into a tracked file, a test, a golden or a screenshot.

When you run the binary while working, use a scratch home and config so the maintainer's are never
touched or read:

```sh
export TRAY_HOME=/tmp/tray-scratch TRAY_CONFIG=/tmp/tray-scratch/config.yaml
```

`scripts/check-tray.sh` already does this; `go test ./...` does too.
