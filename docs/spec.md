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
| `default` | String | ✔ | Key of the default target inside `targets`. Runs when user executes `duck <args>`. |
| `targets` | Mapping <string, Target> | ✔ | Declared targets executed via `duck <target> <args>`. Must contain the default key. |
| `settings` | Settings object | ✖ | Global switches (cache dir, log level, locked mode). **Security settings like host allow/deny lists are configured via environment variables or CLI flags, not in this file**. |

## 3. Target object

| Key | Type | Required | Description |
|---|---|---|---|
| (removed) |  |  | The `name` field has been removed. The map key is the target identifier. |
| `description` | String | ✖ | Optional longer explanation shown in `duck list`. |
| `binary` | String | ✖ | Executable to launch (e.g. `make`, `task`, `helm`). Optional for sync/clean-only workflows. |
| `fileFlag` | String | Cond. | Required when `binary` is set. CLI flag that injects the rendered file (e.g. `-f`, `--taskfile`, `-fvalues`). |
| `template` | Template object | ✔ | Where to find the template file. |
| `variables` | Mapping <string, VarValue> | ✖ | Parameters used during template rendering. |
| `renderedPath` | String | ✖ | Destination path used by the tool. Default: `.duck/<target>/<basename>`. |
| `args` | String or String[] | Cond. | Allowed only when `binary` is set. Default extra arguments always passed to the binary before user-provided ones. |

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
| `checksum` | SHA-256 | ✖ | Expected hash of the raw template for supply-chain safety. If provided, Duckfile will validate the fetched template file against this checksum. |

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
- Values are computed per sync (each binary exec through duck calls a sync).

## 6. Settings object

| Key | Type | Default | Description |
|---|---|---|---|
| `cacheDir` | String | `.duck/objects` | Folder for cache objects. |
| `logLevel` | Enum `debug` `info` `warn` `error` | `info` | Verbosity of CLI output. |
| `locked` | Boolean | `false` | If `true`, `duck` exits when template or variables changed instead of updating. |

**Security Configuration (Host Allow/Deny Lists)**

For supply-chain security, Duckfile supports restricting which Git hosts can be accessed for templates. **These restrictions must be configured outside of the `duck.yaml` file** to prevent attackers from modifying both the target repositories and the security policy in the same commit.

**JSON Schema**: See [`docs/security.schema.json`](security.schema.json) for the complete security configuration schema.

### Configuration Methods (in order of precedence):

1. **CLI flags** (highest precedence):
   ```bash
   # Root command flags
   duck build --allowed-hosts github.com,gitlab.internal.com
   duck --denied-hosts malicious-host.com --strict-hosts build
   
   # Sync command flags
   duck sync --allowed-hosts github.com --strict-hosts
   duck sync target --denied-hosts bad-host.com
   ```

2. **Environment variables** (medium precedence):
   ```bash
   export DUCK_ALLOWED_HOSTS="github.com,gitlab.internal.com"  
   export DUCK_DENIED_HOSTS="malicious-host.com"
   export DUCK_STRICT_MODE="true"  # Fail if no restrictions are configured
   ```

3. **System configuration files** (lowest precedence, future enhancement):
   ```bash
   # Future: /etc/duckfile/security.yaml or ~/.duckfile/security.yaml
   # Not yet implemented but planned for enterprise environments
   ```

### Security Rules:
- **Precedence**: CLI flags override environment variables
- **Default behavior**: If no restrictions are configured, all hosts are allowed (backward compatibility)
- **Deny takes precedence**: Denied hosts are blocked even if they're in the allow list
- **Strict mode**: Use `--strict-hosts` or `DUCK_STRICT_MODE=true` to fail if no restrictions are configured
- **Case insensitive**: Host matching is case-insensitive (`GitHub.COM` matches `github.com`)
- **Exact matching**: Currently supports exact hostname matching (wildcards planned for future)
- **Validation timing**: Host validation occurs before git operations, providing fast feedback

### Security Best Practices:
- Set restrictions in environment variables that require elevated privileges to modify
- Use deny lists for known malicious hosts
- Use allow lists in high-security environments to limit to trusted hosts only  
- Enable strict mode in CI/CD environments to ensure policies are always applied
- Regularly audit allowed hosts and remove unused entries

### Supported Git URL Formats:
- HTTPS: `https://github.com/user/repo.git`
- SSH (SCP-style): `git@github.com:user/repo.git`  
- SSH (URL-style): `ssh://git@github.com:22/user/repo.git`

## 7. Deterministic cache (informative)
Key = `SHA1(repo + ref + path + resolvedVariablesJSON)`.  
Stored at `.duck/objects/<key>/<basename>`.  
A symlink is created at `renderedPath` (or `.duck/<target>/<basename>`) pointing to the object.

## 8. Example config
```yaml
version: 1

default: build

targets:
  build:
    binary: make
    fileFlag: -f
    template:
      repo: https://github.com/CyberDuck79/duckfile-test-templates.git
      ref: main
      path: Makefile.tpl
      checksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    variables:
      PROJECT: my-service
      DATE: !cmd date +%Y-%m-%d
    renderedPath: Makefile

  test:
    binary: task
    fileFlag: --taskfile
    template:
      repo: https://github.com/CyberDuck79/duckfile-test-templates.git
      ref: v2.3.3
      path: task/Taskfile.yml.tpl
      delims: { left: "[[", right: "]]" }
      allowMissing: true
    variables:
      GO_VERSION: !env GO_VERSION
      PLATFORM: linux/amd64
    args: ["--silent"]

  docs:
    # No binary: this target is sync-only. Use `duck sync docs` to render.
    template:
      repo: https://github.com/CyberDuck79/duckfile-test-docs-templates.git
      ref: main
      path: index.md.tpl
    variables:
      AUTHOR: Cyberduck

settings:
  logLevel: debug
  allowedHosts: [github.com]
```

## 9. CLI subcommands

- `duck sync [target] [-f]`: render into cache and update symlinks without executing the tool. With `-f/--force`, ignore cache and re-render. If no target is provided, syncs all (default + named) targets.
- `duck clean [target]`: purge cache. If no target provided, removes all cached objects and per-target directories; otherwise only that target.

When a target lacks `binary`, `duck` will refuse to execute it with the root command. Use `duck sync` and `duck clean` instead.

## 11. Checksum validation and warnings

When a template config includes a `checksum` property, Duckfile will validate the fetched template file against the provided SHA-256 checksum. If the checksum does not match, Duckfile will abort and print an error message showing the expected and actual checksum.

If the template config changes (`repo`, `ref`, or `path`) but the `checksum` remains unchanged, Duckfile will print a warning that the checksum may be stale and should be updated.

Checksum validation is optional. If no checksum is provided, Duckfile will proceed without validation.

## 10. JSON-Schema (v7) excerpt
```json
{
  "definitions": {
    "target": {
      "type": "object",
  "required": ["template"],
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
      "additionalProperties": false,
      "allOf": [
        { "if": { "not": { "required": ["binary"] } }, "then": { "not": { "anyOf": [ { "required": ["fileFlag"] }, { "required": ["args"] } ] } } },
        { "if": { "required": ["binary"] }, "then": { "required": ["fileFlag"] } }
      ]
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

