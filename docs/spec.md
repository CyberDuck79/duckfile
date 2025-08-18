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
| `trackCommitHash` | Boolean | ✖ | Enable commit hash validation and tracking. When `true`, Duckfile stores the actual commit hash after fetching and validates it hasn't changed on subsequent runs. Default `false`. **Note: Cannot be used with commit hash refs (40-character hex strings).** |
| `autoUpdateOnChange` | Boolean | ✖ | Automatically update cache when remote commit hash changes. Only valid when `trackCommitHash` is `true`. Default `false`. When enabled, cache is automatically invalidated and re-fetched if the remote commit hash differs from the stored value. |

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
| `logLevel` | Enum `error` `warn` `info` `debug` | `info` | Verbosity of CLI output. Can be overridden by `--log-level` CLI flag or `DUCK_LOG_LEVEL` environment variable. Precedence: CLI flag > env var > config > default. |
| `locked` | Boolean | `false` | If `true`, `duck` exits when template or variables changed instead of updating. |
| `trackCommitHash` | Boolean | `false` | Global default for commit hash tracking. Can be overridden per-template and by CLI flags. |
| `autoUpdateOnChange` | Boolean | `false` | Global default for auto-update behavior. Can be overridden per-template and by CLI flags. |

## 7. Environment Variables (.env file support)

Duckfile automatically loads environment variables from `.env` files to streamline environment management for both templating and execution contexts.

### File Discovery

Duckfile searches for `.env` files in the following priority order:
1. `.env` (current directory) - **highest priority**
2. `.duck/.env` (duck cache directory)
3. `.env.duck` (duck-specific variant) - **lowest priority**

The first file found is loaded. If multiple files exist, only the highest priority file is used.

### File Format

The `.env` file uses a simple `KEY=VALUE` format:

```bash
# Project configuration
PROJECT_NAME=myapp
VERSION=1.0.0
ENVIRONMENT=development

# Quoted values (optional)
DATABASE_URL="postgres://localhost:5432/myapp"
API_KEY='secret-key-value'

# Values with spaces
DESCRIPTION=My application description

# Empty values
DEBUG_MODE=

# Comments and empty lines are ignored
# This is a comment
```

### Environment Variable Precedence

Environment variables follow this precedence order (highest to lowest):
1. **Existing OS environment variables** (command line exports, shell environment)
2. **Variables from .env file**
3. **Default values** (if any)

This ensures that explicitly set environment variables always override `.env` file values, making the system predictable and secure.

### Loading Behavior

- **.env files are loaded automatically** before any duck command execution
- **Silent loading**: No output is shown when .env files are loaded (to avoid noise)
- **Error handling**: Malformed .env files cause the command to fail with clear error messages
- **Optional**: Not having a .env file is perfectly fine and not an error

### Integration with Variable Resolution

Variables loaded from .env files are available for use with `!env` variable tags:

**`.env` file:**
```bash
GO_VERSION=1.21
DOCKER_REGISTRY=ghcr.io/myorg
BUILD_TAG=latest
```

**`duck.yaml` configuration:**
```yaml
targets:
  build:
    template:
      repo: https://github.com/org/templates.git
      path: Dockerfile.tpl
    variables:
      GO_VERSION: !env GO_VERSION        # Uses value from .env
      REGISTRY: !env DOCKER_REGISTRY     # Uses value from .env
      TAG: !env BUILD_TAG                # Uses value from .env
```

### Example Workflow

1. **Development setup**: Create `.env` with development defaults
2. **Team sharing**: Commit `.env.example` with template values
3. **Local customization**: Copy `.env.example` to `.env` and customize
4. **CI/CD override**: Set critical variables via CI environment, others use .env defaults

**Example `.env.example` (committed to Git):**
```bash
# Copy to .env and customize for your environment
PROJECT_NAME=myapp
GO_VERSION=1.21
ENVIRONMENT=development
# API_KEY=your-api-key-here
# DATABASE_URL=your-database-url
```

**Example `.env` (gitignored, local only):**
```bash
PROJECT_NAME=myapp
GO_VERSION=1.21
ENVIRONMENT=john-dev
API_KEY=john-dev-12345
DATABASE_URL=postgres://localhost:5432/myapp_john
```

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

## 8. Commit Hash Tracking and Validation

Duckfile supports tracking and validating commit hashes to detect when remote templates have changed. This feature helps ensure reproducible builds and provides early warning when templates are updated.

### Configuration

Commit hash tracking can be configured at multiple levels:

1. **Template level** (in `duck.yaml`):
```yaml
targets:
  build:
    template:
      repo: https://github.com/example/templates.git
      ref: main
      path: Makefile.tpl
      trackCommitHash: true
      autoUpdateOnChange: true
```

2. **Global level** (in `duck.yaml` settings):
```yaml
settings:
  trackCommitHash: true
  autoUpdateOnChange: false
```

3. **Environment variables**:
```bash
export DUCK_TRACK_COMMIT_HASH="true"
export DUCK_AUTO_UPDATE_ON_CHANGE="true"
```

4. **CLI flags** (highest precedence):
```bash
# Root command
duck build --track-commit-hash --auto-update-on-change
duck --no-track-commit-hash build

# Sync command  
duck sync --track-commit-hash --no-auto-update-on-change
duck sync target --no-track-commit-hash
```

### Precedence Rules

The commit hash tracking configuration follows this precedence (highest to lowest):

1. **CLI flags** (`--track-commit-hash`, `--no-track-commit-hash`, `--auto-update-on-change`, `--no-auto-update-on-change`)
2. **Environment variables** (`DUCK_TRACK_COMMIT_HASH`, `DUCK_AUTO_UPDATE_ON_CHANGE`)
3. **Template-level configuration** (`template.trackCommitHash`, `template.autoUpdateOnChange`)
4. **Global configuration** (`settings.trackCommitHash`, `settings.autoUpdateOnChange`)
5. **Default** (`false` for both settings)

### Validation Rules

- **Commit hash refs not allowed**: When `trackCommitHash` is enabled, the template `ref` cannot be a commit hash (40-character hex string). Use branch or tag names instead.
- **Auto-update requires tracking**: `autoUpdateOnChange` can only be `true` when `trackCommitHash` is also `true`.
- **Network resilience**: If network errors occur during validation, Duckfile continues with cached templates and logs a warning.

### Behavior

**With tracking enabled (`trackCommitHash: true`)**:
- On first fetch: Duckfile resolves the branch/tag to a commit hash and stores it alongside the cached template
- On subsequent runs: Duckfile checks if the remote commit hash has changed
- If unchanged: Uses cached template (fast)
- If changed: Logs the change and either updates automatically or warns the user

**With auto-update enabled (`autoUpdateOnChange: true`)**:
- Automatically invalidates cache and re-fetches when commit hash changes
- Provides seamless updates while tracking what changed
- Logs the old and new commit hashes for audit trails

**Example workflows**:

```bash
# Enable tracking with manual updates (default)
duck sync --track-commit-hash
# Output: "🔄 commit hash changed for repo@main: abc123 -> def456"

# Enable tracking with automatic updates  
duck sync --track-commit-hash --auto-update-on-change
# Output: "📦 Updating template cache: repo@main" (transparent update)

# Disable tracking entirely (backward compatible)
duck sync --no-track-commit-hash
# No commit hash checking, cache based only on template config
```

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

## 9. Deterministic cache (informative)
Key = `SHA1(repo + ref + path + resolvedVariablesJSON + commitHashTracking)`.  
When commit hash tracking is enabled, the actual resolved commit hash is included in the cache key computation to ensure cache invalidation when commits change.  
Stored at `.duck/objects/<key>/<basename>`.  
A symlink is created at `renderedPath` (or `.duck/<target>/<basename>`) pointing to the object.

## 10. Example config

**Example `.env` file:**
```bash
# Development environment variables
PROJECT_NAME=my-service
GO_VERSION=1.21
DOCKER_REGISTRY=ghcr.io/myorg
BUILD_DATE=2025-08-18
DEBUG=true
```

**Example `duck.yaml` configuration:**
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
      trackCommitHash: true
      autoUpdateOnChange: true
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
      trackCommitHash: true  # Track without auto-update for tags
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
  trackCommitHash: false  # Global default (can be overridden per-template)
  autoUpdateOnChange: false
```

## 11. CLI subcommands

- `duck sync [target] [-f]`: render into cache and update symlinks without executing the tool. With `-f/--force`, ignore cache and re-render. If no target is provided, syncs all (default + named) targets.
  - `--track-commit-hash` / `--no-track-commit-hash`: Override commit hash tracking setting
  - `--auto-update-on-change` / `--no-auto-update-on-change`: Override auto-update behavior
- `duck clean [target]`: purge cache. If no target provided, removes all cached objects and per-target directories; otherwise only that target.
- `duck [target] [args...]`: render template, create symlink, and execute the binary with the rendered file. Supports the same commit hash tracking flags as `sync`.
  - `--track-commit-hash` / `--no-track-commit-hash`: Override commit hash tracking setting  
  - `--auto-update-on-change` / `--no-auto-update-on-change`: Override auto-update behavior

When a target lacks `binary`, `duck` will refuse to execute it with the root command. Use `duck sync` and `duck clean` instead.

## 12. Checksum validation and warnings

When a template config includes a `checksum` property, Duckfile will validate the fetched template file against the provided SHA-256 checksum. If the checksum does not match, Duckfile will abort and print an error message showing the expected and actual checksum.

If the template config changes (`repo`, `ref`, or `path`) but the `checksum` remains unchanged, Duckfile will print a warning that the checksum may be stale and should be updated.

Checksum validation is optional. If no checksum is provided, Duckfile will proceed without validation.

## 13. JSON-Schema (v7) excerpt
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
        "checksum": { "type": "string", "pattern": "^[A-Fa-f0-9]{64}$" },
        "trackCommitHash": { "type": "boolean" },
        "autoUpdateOnChange": { "type": "boolean" }
      },
      "additionalProperties": false,
      "allOf": [
        {
          "if": { "properties": { "trackCommitHash": { "const": true } } },
          "then": { 
            "not": { 
              "properties": { 
                "ref": { "pattern": "^[A-Fa-f0-9]{40}$" } 
              } 
            } 
          }
        },
        {
          "if": { "properties": { "autoUpdateOnChange": { "const": true } } },
          "then": { "properties": { "trackCommitHash": { "const": true } } }
        }
      ]
    },
    "settings": {
      "type": "object",
      "properties": {
        "cacheDir": { "type": "string" },
        "logLevel": { "type": "string", "enum": ["error","warn","info","debug"] },
        "locked": { "type": "boolean" },
        "trackCommitHash": { "type": "boolean" },
        "autoUpdateOnChange": { "type": "boolean" }
      },
      "additionalProperties": false,
      "allOf": [
        {
          "if": { "properties": { "autoUpdateOnChange": { "const": true } } },
          "then": { "properties": { "trackCommitHash": { "const": true } } }
        }
      ]
    }
  }
}
```

## 14. Migration rules
Future changes will be announced with a version bump; for MVP users, no migration is required.

