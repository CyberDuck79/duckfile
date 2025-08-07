# Duckfiles – Configuration Specification (`.duck.yaml`)

The file `.duck.yaml` (or `.duck.yml`, `duck.yaml`, `duck.yml`) is the single source of truth that tells Duckfiles how to fetch, render, cache and execute a remote template.

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
| `version` | Integer | ✔ | Specification version understood by this Duckfiles release. Start with `1`. |
| `default` | Target object | ✔ | First (default) target. Runs when user executes `duck <args>`. |
| `targets` | Mapping <string, Target> | ✖ | Additional named targets executed via `duck <target> <args>`. |
| `settings` | Settings object | ✖ | Global switches (cache dir, log level, allowlist…). |

## 3. Target object

| Key | Type | Required | Description |
|---|---|---|---|
| `name` | String | ✔ (for `default`; auto-derived in `targets`) | Human readable label, used in logs. |
| `binary` | String | ✔ | Executable to launch (e.g. `make`, `task`, `helm`). |
| `fileFlag` | String | ✔ | CLI flag that injects the rendered file (e.g. `-f`, `--taskfile`, `-fvalues`). |
| `optionArgs` | String | ✔ | CLI arguments that customize the executable behavior (inserted before fileFlag). |
| `template` | Template object | ✔ | Where to find the template file. |
| `variables` | Mapping <string, VarValue> | ✖ | Parameters used during template rendering. |
| `cacheFile` | String | ✖ | Destination path in the project after rendering. Default: `.duck/<target>/<basename>`. |
| `args` | String or String[] | ✖ | Default extra arguments always passed to the binary before user-provided ones. |

## 4. Template object

| Key | Type | Required | Description |
|---|---|---|---|
| `repo` | Git URL | ✔ | Remote Git repository (SSH or HTTPS). |
| `ref` | String | ✖ | Git reference (branch, tag or commit). Default `HEAD`. |
| `path` | String | ✔ | Path inside the repo to the template file. |
| `submodules` | Boolean | ✖ | Fetch submodules (`--recurse-submodules`). Default `false`. |
| `shallow` | Boolean | ✖ | Shallow clone (`--depth 1`). Default `true`. |
| `checksum` | SHA-256 | ✖ | Expected hash of the raw template for supply-chain safety. |

## 5. Variable value (`VarValue`)

A variable value is **either** a scalar **or** a tagged scalar beginning with `!`.

| Tag | Meaning | Example | Result |
|---|---|---|---|
| *(no tag)* | Literal string | `REGION: eu-west-3` | `"eu-west-3"` |
| `!env` | Take from environment variable | `GO_VERSION: !env GOVER` | `$GOVER` |
| `!cmd` | Evaluate shell command | `DATE: !cmd date +%F` | `2025-08-07` |
| `!file` | Read entire file | `CERT: !file ./tls.crt` | File contents |

Notes  
• Shell commands are executed with `/bin/sh -c`.  
• Command values are cached per render; they do **not** re-run unless `duck sync` or inputs change.

## 6. Settings object

| Key | Type | Default | Description |
|---|---|---|---|
| `cacheDir` | String | `.duck/objects` | Folder used for immutable cache blobs. |
| `logLevel` | Enum `debug` `info` `warn` `error` | `info` | Verbosity of CLI output. |
| `allowedHosts` | String[] | *(no restriction)* | White-list of Git hostnames. |
| `locked` | Boolean | `false` | If `true`, `duck` exits when template or variables changed instead of updating. |

## 7. Cache-key algorithm (informative)

\[ key = SHA-1( repo + ref + path + renderedVariablesJSON ) \]

The rendered template is stored at  
`.duck/objects/<key>/<basename>`  
and symlinked / copied to `cacheFile`.

## 8. Example config

```yaml
version: 1

default:                # default target ⇒ `duck build`
  name: build
  binary: make
  fileFlag: -f
  template:
    repo: git@gitlab.com:acme/devops-templates.git
    ref: main
    path: make/Makefile.tpl
  variables:
    PROJECT: my-service
    DATE: !cmd date +%Y-%m-%d
  cacheFile: .duck/Makefile

targets:
  test:                  # executed with `duck test`
    binary: task
    fileFlag: --taskfile
    template:
      repo: https://gitlab.com/acme/test-templates.git
      ref: v2.3.0
      path: Taskfile.yml.tpl
    variables:
      GO_VERSION: !env GO_VERSION    # falls back to 1.22 if unset
      PLATFORM: linux/amd64
    args: ["--quiet"]

settings:
  logLevel: debug
  allowedHosts:
    - gitlab.com
```

## 9. JSON-Schema (v7)

Place the following file under `docs/duck.schema.json`; editors can auto-validate.

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Duckfiles configuration",
  "type": "object",
  "required": ["version", "default"],
  "properties": {
    "version": { "type": "integer", "enum": [1] },

    "default": { "$ref": "#/definitions/target" },

    "targets": {
      "type": "object",
      "additionalProperties": { "$ref": "#/definitions/target" }
    },

    "settings": {
      "type": "object",
      "properties": {
        "cacheDir": { "type": "string" },
        "logLevel": { "type": "string", "enum": ["debug","info","warn","error"] },
        "allowedHosts": { "type": "array", "items": { "type": "string" } },
        "locked": { "type": "boolean" }
      },
      "additionalProperties": false
    }
  },

  "definitions": {
    "target": {
      "type": "object",
      "required": ["binary","fileFlag","template"],
      "properties": {
        "name": { "type": "string" },
        "binary": { "type": "string" },
        "fileFlag": { "type": "string" },
		"optionArgs": { "type": "string" },
        "template": { "$ref": "#/definitions/template" },
        "variables": { "type": "object", "additionalProperties": { "type": ["string","number","boolean"] } },
        "cacheFile": { "type": "string" },
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
        "submodules": { "type": "boolean" },
        "shallow": { "type": "boolean" },
        "checksum": { "type": "string", "pattern": "^[A-Fa-f0-9]{64}$" }
      },
      "additionalProperties": false
    }
  }
}
```

---

## 10. Migration rules

| Change | Action for users |
|---|---|
| `version` bump 2→future | Tool prints error: “please run duck migrate”. |

