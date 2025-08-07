# duckfile
Universal remote templating for DevOps tools

## spec
[specification version 1](docs/spec.md)

Below is a concrete, copy-paste-ready “scaffolding kit” that gets you from an empty repo to a compiling MVP branch (`feat/mvp-run`) in ≈ 10 minutes.

---

## Project tree

| Path | Purpose |
|---|---|
| `cmd/duck/` | Cobra entry point (`main.go`, `root.go`) |
| `internal/config/` | Tiny parser for `duck.yaml` |
| `internal/git/` | Thin wrapper that shells out to `git` (MVP) |
| `internal/run/` | Renders file, caches in `.duck`, then `os/exec` the tool |
| `go.mod` | Module metadata |
| `README.md` | Quickly document how to build/run |
| `.gitignore` | Standard Go + cache ignores |

---

## Install
```
go install github.com/CyberDuck79/duckfile/cmd/duck@latest
```