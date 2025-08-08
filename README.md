# duckfile
Universal remote templating for DevOps tools

Duckfile lets you keep your Makefiles, Taskfiles, Helm values, and other config as remote templates, render them locally with variables, and run the tool seamlessly.

## Features
- Git-sourced templates: repo + ref + path
- Variable tags: !env, !cmd, !file, and literals
- Go templates with Sprig functions
- Custom delimiters to avoid collisions (e.g., Taskfile)
- Deterministic caching with stable symlinks
- Simple CLI that forwards args to your tool (make, task, helm, …)

## Install
```sh
go install github.com/CyberDuck79/duckfile/cmd/duck@latest
```

Go 1.21+ recommended.

## Quick start
1) Create duck.yaml at the repo root:
```yaml
version: 1

default:
  name: build
  binary: make
  fileFlag: -f
  template:
    repo: https://github.com/CyberDuck79/duckfile-test-templates.git
    ref: main
    path: Makefile.tpl
  variables:
    PROJECT: my-service
    DATE: !cmd date +%Y-%m-%d
  renderedPath: Makefile

targets:
  test:
    binary: task
    fileFlag: --taskfile
    template:
      repo: https://github.com/CyberDuck79/duckfile-test-templates.git
      ref: v2.3.1
      path: task/Taskfile.yml.tpl
      delims: { left: "[[", right: "]]" }  # avoid Task's {{ }}
      allowMissing: true                   # missing vars => ""
    variables:
      GO_VERSION: !env GO_VERSION
      PLATFORM: linux/amd64
    args: ["--quiet"]
```

2) Run
```sh
# print version
go run ./cmd --version

# run default target (renders Makefile and calls make -f Makefile)
go run ./cmd

# run a named target and pass additional args after --
go run ./cmd test --
```

## How it works (MVP)
- Clone/fetch the template repo at the requested ref.
- Resolve variables:
  - !env NAME → os.Getenv(NAME)
  - !cmd SHELL → /bin/sh -c SHELL (trimmed)
  - !file PATH → file contents
  - literal scalars (string/number/bool)
- Render the template using Go text/template + Sprig.
- Deterministic caching:
  - key = SHA1(repo + ref + path + resolvedVarsJSON)
  - rendered file stored under .duck/objects/<key>/<basename>
  - a symlink at renderedPath (or .duck/<target>/<basename>) points to the object
- Execute the tool: binary fileFlag renderedPath [args …]

## Templating tips
- Use Sprig to transform values: {{ .PROJECT | upper }}
- Add now/env helpers: {{ now }} and {{ env "HOME" }}
- When the generated file itself uses Go templates (e.g., Taskfile), set `delims` so our engine renders only your placeholders and leaves the downstream engine’s `{{ ... }}` intact.
- If you want missing variables to become empty strings, set `allowMissing: true`. Default is strict.

## Project layout
| Path | Purpose |
|---|---|
| `cmd/` | Entry point (`main.go`) |
| `cmd/duck/` | Cobra command (`root.go`) |
| `internal/config/` | Parser for `duck.yaml` |
| `internal/git/` | Git wrapper for clone/fetch/checkout |
| `internal/run/` | Render + cache + exec |

## Troubleshooting
- git exit status 128: usually wrong ref or network; error message includes git’s stderr.
- “map has no entry …” during rendering: you are missing a variable and `allowMissing` is false, or your delimiters collide with the target tool (set `delims`).
- On macOS, if a symlink isn’t resolving, remove it and re-run; Duck recreates it.

## Spec
See the full specification: [docs/spec.md](docs/spec.md)
