# Duckfiles – Configuration Specification (`duck.yaml`)

The file `duck.yaml` (or `duck.yml`, `.duck.yaml`, `.duck.yml`) is the single source of truth that tells Duckfile how to fetch, render, cache, and execute a remote template.

## 1. File format

| Item | Value |
|---|---|
| Encoding | UTF-8 |
| Syntax | YAML 1.2 |
| Root type | Mapping |
| Versioning | Required `version` field (integer) |

## 2. Top-level structure

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | Integer | ✔ | Specification version understood by this release. Start with `1`. |
| `default` | Target object | ✔ | First (default) target. Runs when user executes `duck <args>`. |
| `targets` | Mapping <string, Target> | ✖ | Additional named targets executed via `duck <target> <args>`. |
| `settings` | Settings object | ✖ | Global switches (cache dir, log level, allowlist…). |

## 3. Target object

| Key | Type | Required | Description |
|---|---|---|---|
| `name` | String | ✔ (for `default`; auto-derived in `targets`) | Human readable label, used in logs. |
| `binary` | String | ✔ | Executable to launch (e.g. `make`, `task`, `helm`). |
| `fileFlag` | String | ✔ | CLI flag that injects the rendered file (e.g. `-f`, `--taskfile`, `-fvalues`). |
| `template` | Template object | ✔ | Where to find the template file. |
| `variables` | Mapping <string, VarValue> | ✖ | Parameters used during template rendering. |
| `renderedPath` | String | ✖ | Destination path used by the tool. Default: `.duck/<target>/<basename>`. |
| `args` | String or String[] | ✖ | Default extra arguments always passed to the binary before user-provided ones. |

## 4. Template object

| Key | Type | Required | Description |
|---|---|---|---|
| `repo` | Git URL | ✔ | Remote Git repository (SSH or HTTPS). |
| `ref` | String | ✖ | Git reference (branch, tag or commit). Default `HEAD`. |
| `path` | String | ✔ | Path inside the repo to the template file. |
| `delims` | Object `{left,right}` | ✖ | Override Go template delimiters (`{{` / `}}` by default). |
| `allowMissing` | Boolean | ✖ | If `true`, missing keys render as zero values (empty strings). Default `false` (strict). |
| `submodules` | Boolean | ✖ | Fetch submodules (`--recurse-submodules`). Default `false`. |
| `shallow` | Boolean | ✖ | Shallow clone (`--depth 1`). Default `true`. |
| `checksum` | SHA-256 | ✖ | Expected hash of the raw template for supply-chain safety. |

## 5. Variable value (`VarValue`)

A variable value is either a scalar or a tagged scalar beginning with `!`.

| Tag | Meaning | Example | Result |
|---|---|---|---|
| (no tag) | Literal string/number/bool | `REGION: eu-west-3` | `"eu-west-3"` |
| `!env` | Take from environment variable | `GO_VERSION: !env GOVER` | `$GOVER` |
| `!cmd` | Evaluate shell command | `DATE: !cmd date +%F` | `2025-08-07` |
| `!file` | Read entire file | `CERT: !file ./tls.crt` | File contents |

Notes:
- Shell commands run with `/bin/sh -c`. Trailing newlines are trimmed.
- Values are computed per render.

## 6. Settings object

| Key | Type | Default | Description |
|---|---|---|---|
| `cacheDir` | String | `.duck/objects` | Folder for cache objects. |
| `logLevel` | Enum `debug` `info` `warn` `error` | `info` | Verbosity of CLI output. |
| `allowedHosts` | String[] | *(no restriction)* | Allowlist of Git hostnames. |
| `locked` | Boolean | `false` | If `true`, `duck` exits when template or variables changed instead of updating. |

## 7. Deterministic cache (informative)
Key = `SHA1(repo + ref + path + resolvedVariablesJSON)`.  
Stored at `.duck/objects/<key>/<basename>`.  
A symlink is created at `renderedPath` (or `.duck/<target>/<basename>`) pointing to the object.

## 8. Example config
```yaml
version: 1

default:
  name: build
  binary: make
  fileFlag: -f
  template:
    repo: git@github.com:acme/devops-templates.git
    ref: main
    path: make/Makefile.tpl
  variables:
    PROJECT: my-service
    DATE: !cmd date +%Y-%m-%d
  renderedPath: .duck/Makefile

targets:
  test:
    binary: task
    fileFlag: --taskfile
    template:
      repo: https://github.com/acme/test-templates.git
      ref: v2.3.1
      path: task/Taskfile.yml.tpl
      delims: { left: "[[", right: "]]" }
      allowMissing: true
    variables:
      GO_VERSION: !env GO_VERSION
      PLATFORM: linux/amd64
    args: ["--quiet"]

settings:
  logLevel: debug
  allowedHosts: [github.com]
```

## 9. JSON-Schema (v7) excerpt
```json
{
  "definitions": {
    "target": {
      "type": "object",
      "required": ["binary","fileFlag","template"],
      "properties": {
        "name": { "type": "string" },
        "binary": { "type": "string" },
        "fileFlag": { "type": "string" },
        "template": { "$ref": "#/definitions/template" },
        "variables": { "type": "object", "additionalProperties": { "type": ["string","number","boolean"] } },
        "renderedPath": { "type": "string" },
        "args": {
          "oneOf": [
            { "type": "string" },
            { "type": "array", "items": { "type": "string" } }
          ]
        }
      },
      "additionalProperties": false
    },
    "template": {
      "type": "object",
      "required": ["repo", "path"],
      "properties": {
        "repo": { "type": "string" },
        "ref": { "type": "string" },
        "path": { "type": "string" },
        "delims": {
          "type": "object",
          "properties": { "left": { "type": "string" }, "right": { "type": "string" } },
          "additionalProperties": false
        },
        "allowMissing": { "type": "boolean" },
        "submodules": { "type": "boolean" },
        "shallow": { "type": "boolean" },
        "checksum": { "type": "string", "pattern": "^[A-Fa-f0-9]{64}$" }
      },
      "additionalProperties": false
    }
  }
}
```

## 10. Migration rules
Future changes will be announced with a version bump; for MVP users, no migration is required.

