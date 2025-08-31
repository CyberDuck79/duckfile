# Duck Security System - User Guide

This comprehensive guide covers Duck's enterprise-grade security features designed to protect your DevOps workflows from supply chain attacks and ensure configuration integrity.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Security Configuration](#security-configuration)
4. [Digital Signatures](#digital-signatures)
5. [Host Access Control](#host-access-control)
6. [Configuration Precedence](#configuration-precedence)
7. [CLI Security Commands](#cli-security-commands)
8. [File Permissions](#file-permissions)
9. [Environment Variables](#environment-variables)
10. [Troubleshooting](#troubleshooting)
11. [Best Practices](#best-practices)
12. [Advanced Usage](#advanced-usage)

---

## Overview

Duck's security system provides multiple layers of protection:

- **🔐 Digital Signatures**: Cryptographic verification of configuration integrity
- **🛡️ Host Access Control**: Allow/deny lists for Git repositories  
- **📁 File Permissions**: Secure configuration file handling
- **⚡ Precedence System**: 5-tier configuration hierarchy with signed configs taking priority
- **🔧 CLI Security Tools**: Complete security management command suite
- **🌍 Environment Integration**: Secure environment variable handling

### Security Architecture

```
🔒 Signed Security Config Files    (Highest Priority)
⚡ CLI Flags                       (High Priority)  
🌍 Environment Variables           (Medium Priority)
📄 Unsigned Security Config Files  (Low Priority)
🔓 No Restrictions                 (Lowest Priority)
```

---

## Quick Start

### 1. Enable Basic Security

Create a security configuration file:

```bash
# Create security config directory
mkdir -p .duckfile

# Create basic security configuration
cat > .duckfile/security.yaml << EOF
version: 1
allowedHosts:
  - github.com
  - gitlab.com
  - bitbucket.org
deniedHosts:
  - malicious-host.com
strictMode: true
EOF
```

### 2. Check Security Status

```bash
# View current security configuration
duck security status

# Get detailed security information
duck security status --verbose
```

### 3. Optional: Add Digital Signatures

```bash
# Generate cryptographic key pair
duck security generate-keys

# Sign your security configuration
duck security sign .duckfile/security.yaml

# Verify the signed configuration
duck security verify --config .duckfile/security.yaml
```

---

## Security Configuration

### Configuration File Structure

Security configurations use YAML format with the following structure:

```yaml
version: 1                    # Required: Configuration schema version

# Host Access Control
allowedHosts:                 # Optional: Permitted Git hosts
  - github.com
  - gitlab.internal.com
  - bitbucket.org

deniedHosts:                  # Optional: Blocked Git hosts  
  - malicious-host.com
  - untrusted-source.net

strictMode: true              # Optional: Fail if no restrictions configured (default: false)

# Signature Information (auto-generated when signed)
signature:
  keyId: "duck-key-abc123"    # Key identifier used for signing
  timestamp: "2025-08-31T10:30:00Z"  # When configuration was signed
```

### Configuration Discovery

Duck searches for security configurations in this order:

1. **Project Level**: `./.duckfile/security.{yaml,yml}`
2. **User Level**: `~/.duck/security.{yaml,yml}`
3. **System Level**: `/etc/duck/security.{yaml,yml}` (Unix) or `%PROGRAMDATA%\duck\security.{yaml,yml}` (Windows)

### Configuration Validation

```bash
# Validate configuration syntax and security
duck security verify

# Validate specific configuration file
duck security verify --config /path/to/security.yaml

# Verbose validation with detailed checks
duck security verify --config security.yaml --verbose
```

---

## Digital Signatures

Digital signatures ensure configuration integrity and authenticity using Ed25519 cryptography.

### Key Management

#### Generate Key Pair

```bash
# Generate keys in default location (~/.duck/keys/)
duck security generate-keys

# Generate keys in custom directory
duck security generate-keys --output-dir ./my-keys

# Overwrite existing keys
duck security generate-keys --overwrite
```

This creates:
- `private.key`: Ed25519 private key (keep secure!)
- `public.key`: Ed25519 public key (shareable)
- Key files are named with format: `{keyId}.{priv|pub}`

#### Key Distribution

**For teams:**
1. Generate keys on secure system
2. Distribute public keys to team members
3. Store private keys securely (password managers, HSMs)

**Public key sharing:**
```bash
# Share public key with team
cp ~/.duck/keys/duck-key-abc123.pub /shared/duck-keys/
```

### Signing Configurations

#### Sign Configuration

```bash
# Sign with default private key
duck security sign .duckfile/security.yaml

# Sign with specific key file
duck security sign security.yaml --key-file ./my-keys/private.key

# Specify output directory for signature
duck security sign security.yaml --output-dir ./signatures
```

This creates a `.sig` file (e.g., `security.yaml.sig`) containing the Ed25519 signature.

#### Signature Format

Signature files contain base64-encoded Ed25519 signatures:
```
# security.yaml.sig
iQEcBAABCAAGBQJhEt2dAAoJEKGj... (base64-encoded signature)
```

### Verification Process

```bash
# Verify signed configuration
duck security verify --config security.yaml

# Detailed verification output
duck security verify --config security.yaml --verbose
```

**Verification checks:**
1. ✅ Configuration file exists and is readable
2. ✅ Signature file exists (`config.yaml.sig`)
3. ✅ Public key available for verification
4. ✅ Signature cryptographically valid
5. ✅ Configuration content matches signature

---

## Host Access Control

Protect against supply chain attacks by controlling which Git repositories Duck can access.

### Host Configuration

#### Allowed Hosts (Allowlist)

```yaml
allowedHosts:
  - github.com
  - gitlab.internal.com
  - bitbucket.org
  - git.company.com
```

- Only repositories from these hosts are permitted
- Exact host matching (subdomains not included unless specified)
- Empty list allows all hosts (unless `deniedHosts` specified)

#### Denied Hosts (Denylist)

```yaml
deniedHosts:
  - malicious-host.com
  - compromised-git.net
  - untrusted-source.org
```

- Repositories from these hosts are blocked
- Takes precedence over `allowedHosts`
- Useful for blocking specific known threats

#### Strict Mode

```yaml
strictMode: true
```

- **true**: Fails if no host restrictions are configured
- **false** (default): Allows all hosts when no restrictions specified
- Recommended for production environments

### Host Validation Examples

#### Example 1: Corporate Environment
```yaml
version: 1
allowedHosts:
  - github.com
  - gitlab.internal.corp.com
  - git.corp.com
deniedHosts:
  - github.personal.com  # Block personal accounts
strictMode: true
```

#### Example 2: Open Source Project
```yaml
version: 1
allowedHosts:
  - github.com
  - gitlab.com
  - bitbucket.org
strictMode: false  # Allow flexibility for contributors
```

#### Example 3: High Security Environment
```yaml
version: 1
allowedHosts:
  - git.secure-internal.com  # Only internal Git server
deniedHosts:
  - "*"  # Block everything else
strictMode: true
```

### CLI Host Override

Override host restrictions via command line:

```bash
# Allow specific hosts for one execution
duck --allowed-hosts github.com,gitlab.com sync

# Block specific hosts
duck --denied-hosts malicious-host.com build

# Enable strict mode
duck --strict-hosts status
```

---

## Configuration Precedence

Duck uses a 5-tier precedence system ensuring security configurations take priority:

### Precedence Hierarchy

```
🔒 1. Signed Security Config Files    (HIGHEST)
   └── Cryptographically verified configurations
   
⚡ 2. CLI Flags                       (HIGH)
   └── --allowed-hosts, --denied-hosts, --strict-hosts
   
🌍 3. Environment Variables           (MEDIUM)
   └── DUCK_ALLOWED_HOSTS, DUCK_DENIED_HOSTS, DUCK_STRICT_MODE
   
📄 4. Unsigned Security Config Files  (LOW)
   └── Regular security.yaml files without signatures
   
🔓 5. No Restrictions                 (LOWEST)
   └── Default permissive behavior
```

### Precedence Examples

#### Example 1: Signed Config Override

```bash
# Environment sets: DUCK_ALLOWED_HOSTS=evil-host.com
# Signed config contains: allowedHosts: [github.com]
# Result: Only github.com allowed (signed config wins)

duck security status
# Shows: "Effective Configuration Source: signed"
```

#### Example 2: CLI Flag Override

```bash
# Unsigned config: allowedHosts: [old-host.com]  
# CLI flag: --allowed-hosts new-host.com
# Result: Only new-host.com allowed (CLI wins)

duck --allowed-hosts new-host.com security status
```

#### Example 3: Environment Fallback

```bash
# No config files or CLI flags
# Environment: DUCK_ALLOWED_HOSTS=github.com
# Result: Only github.com allowed (environment used)

export DUCK_ALLOWED_HOSTS=github.com
duck security status
```

### Viewing Effective Configuration

```bash
# See which configuration source is active
duck security status

# Detailed precedence information
duck security status --verbose
```

Output shows:
```
📊 Effective Configuration Source: signed

📋 Precedence Hierarchy (🔒 = highest, 🔓 = lowest):
   🔒 Signed Security Config Files: ✅ ACTIVE
   ⚡ CLI flags: ❌ Not configured  
   🌍 Environment variables: ❌ Not configured
   📄 Unsigned Security Config Files: ❌ Not configured
   🔓 No restrictions: ❌ Not configured
```

---

## CLI Security Commands

Duck provides comprehensive CLI tools for security management:

### `duck security status`

Display current security configuration and policy enforcement status.

```bash
# Basic security status
duck security status

# Include detailed permission information  
duck security status --include-permissions

# Show comprehensive security details
duck security status --verbose
```

**Example output:**
```
[duck] 🔍 Security Configuration Status

📊 Effective Configuration Source: signed

🌐 Host Access Control:
   ✅ Allowed Hosts: github.com, gitlab.internal.com
   🚫 Denied Hosts: malicious-host.com
   🔒 Strict Mode: true

📄 Configuration Files:
   ./.duckfile/security.yaml: ✅ Found (signed)
   ~/.duck/security.yaml: ❌ Not found
   
🔐 Digital Signatures:
   Key ID: duck-key-abc123
   Signature: ✅ Valid
   Signed: 2025-08-31T10:30:00Z
```

### `duck security verify`

Verify security configuration integrity and signatures.

```bash
# Verify discovered security configurations
duck security verify

# Verify specific configuration file
duck security verify --config security.yaml

# Detailed verification output
duck security verify --config security.yaml --verbose
```

**Verification process:**
1. Configuration file syntax validation
2. Digital signature verification (if present)
3. File permission checks
4. Policy compliance validation

### `duck security generate-keys`

Generate Ed25519 cryptographic key pairs for signing.

```bash
# Generate keys in default location (~/.duck/keys/)
duck security generate-keys

# Generate keys in custom directory
duck security generate-keys --output-dir ./team-keys

# Overwrite existing keys
duck security generate-keys --overwrite
```

**Generated files:**
- `duck-key-{id}.priv`: Private key (Ed25519, 64 bytes)
- `duck-key-{id}.pub`: Public key (Ed25519, 32 bytes)

### `duck security sign`

Sign security configuration files with cryptographic signatures.

```bash
# Sign with default private key
duck security sign .duckfile/security.yaml

# Sign with specific key file  
duck security sign security.yaml --key-file ./keys/private.key

# Specify output directory for signature
duck security sign security.yaml --output-dir ./signatures
```

**Creates:** `{config}.sig` file with base64-encoded Ed25519 signature

### `duck security check-permissions`

Validate file permissions for security configurations.

```bash
# Check all discovered security configurations
duck security check-permissions

# Automatically fix permission violations
duck security check-permissions --fix

# Detailed permission analysis
duck security check-permissions --verbose
```

**Permission requirements:**
- **System configs** (`/etc/duck/`): `644` (readable by all)
- **User configs** (`~/.duck/`): `600` (user-only)
- **Project configs** (`./.duckfile/`): `644` (readable by all)

### `duck security fix-permissions`

Automatically repair security configuration file permissions.

```bash
# Fix all permission violations
duck security fix-permissions --all

# Fix only system-wide configurations
duck security fix-permissions --system

# Fix only user-specific configurations  
duck security fix-permissions --user

# Fix only project-specific configurations
duck security fix-permissions --project

# Preview changes without applying them
duck security fix-permissions --all --dry-run
```

---

## File Permissions

Proper file permissions ensure security configuration integrity and prevent unauthorized modifications.

### Permission Requirements

| Scope | Path | Required Permissions | Rationale |
|-------|------|---------------------|-----------|
| **System** | `/etc/duck/security.yaml` | `644` (rw-r--r--) | Readable by all users, writable by admin only |
| **User** | `~/.duck/security.yaml` | `600` (rw-------) | Readable/writable by owner only |
| **Project** | `./.duckfile/security.yaml` | `644` (rw-r--r--) | Readable by team members |

### Permission Validation

Duck automatically validates file permissions during:
- Configuration loading
- Security verification
- Status checks

**Invalid permissions cause:**
- Warning messages during normal operation
- Failure in strict security modes
- Exclusion from configuration precedence

### Permission Management

#### Check Current Permissions

```bash
# Check all security configuration permissions
duck security check-permissions

# Verbose permission analysis
duck security check-permissions --verbose
```

**Example output:**
```
[duck] 🔍 Security Configuration File Permissions

📁 System Configuration (/etc/duck/security.yaml):
   ❌ INVALID: 600 (should be 644)
   
📁 User Configuration (~/.duck/security.yaml):  
   ✅ VALID: 600
   
📁 Project Configuration (./.duckfile/security.yaml):
   ✅ VALID: 644
```

#### Fix Permissions Automatically

```bash
# Fix all permission violations
duck security fix-permissions --all

# Preview changes first
duck security fix-permissions --all --dry-run
```

#### Manual Permission Fixes

```bash
# System configuration (needs sudo)
sudo chmod 644 /etc/duck/security.yaml

# User configuration
chmod 600 ~/.duck/security.yaml

# Project configuration  
chmod 644 ./.duckfile/security.yaml
```

---

## Environment Variables

Control Duck security through environment variables for automation and CI/CD integration.

### Security Environment Variables

| Variable | Type | Description | Example |
|----------|------|-------------|---------|
| `DUCK_ALLOWED_HOSTS` | String | Comma-separated allowed hosts | `github.com,gitlab.com` |
| `DUCK_DENIED_HOSTS` | String | Comma-separated denied hosts | `malicious-host.com` |
| `DUCK_STRICT_MODE` | Boolean | Enable strict host validation | `true` |
| `DUCK_KEYS_PATH` | String | Custom key directory path | `/secure/duck-keys` |

### CI/CD Integration

#### GitHub Actions Example

```yaml
name: Duck Security Build
on: [push, pull_request]

jobs:
  secure-build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Duck Security
        env:
          DUCK_ALLOWED_HOSTS: "github.com"
          DUCK_STRICT_MODE: "true"
        run: |
          duck security status
          duck security verify
          
      - name: Execute Build
        env:
          DUCK_ALLOWED_HOSTS: "github.com"
          DUCK_STRICT_MODE: "true"  
        run: duck build
```

#### GitLab CI Example

```yaml
security-build:
  variables:
    DUCK_ALLOWED_HOSTS: "gitlab.com,github.com"
    DUCK_STRICT_MODE: "true"
  script:
    - duck security status
    - duck security verify
    - duck build
  only:
    - main
    - develop
```

#### Docker Environment

```bash
# Docker container with security restrictions
docker run -e DUCK_ALLOWED_HOSTS="github.com" \
           -e DUCK_STRICT_MODE="true" \
           my-duck-image duck build
```

### Environment Variable Precedence

Environment variables have **medium precedence** in Duck's hierarchy:

```
🔒 Signed Configs > ⚡ CLI Flags > 🌍 Environment > 📄 Unsigned Configs > 🔓 No Restrictions
```

**Example precedence interaction:**
```bash
# Environment setting
export DUCK_ALLOWED_HOSTS="env-host.com"

# CLI override
duck --allowed-hosts cli-host.com build
# Result: cli-host.com used (CLI wins)

# Signed config override
# If signed config has allowedHosts: [signed-host.com]
duck build
# Result: signed-host.com used (signed config wins)
```

---

## Troubleshooting

### Common Issues and Solutions

#### Issue: "No security restrictions configured"

**Symptoms:**
```
[duck] ⚠️  No security restrictions configured
```

**Solutions:**
1. **Create security configuration:**
   ```bash
   mkdir -p .duckfile
   cat > .duckfile/security.yaml << EOF
   version: 1
   allowedHosts:
     - github.com
   strictMode: true
   EOF
   ```

2. **Use environment variables:**
   ```bash
   export DUCK_ALLOWED_HOSTS="github.com"
   export DUCK_STRICT_MODE="true"
   ```

3. **Use CLI flags:**
   ```bash
   duck --allowed-hosts github.com --strict-hosts build
   ```

#### Issue: "File permissions are invalid"

**Symptoms:**
```
[duck] ❌ File permissions are invalid: security.yaml should be 644, found 600
```

**Solutions:**
1. **Automatic fix:**
   ```bash
   duck security fix-permissions --all
   ```

2. **Manual fix:**
   ```bash
   chmod 644 .duckfile/security.yaml  # Project config
   chmod 600 ~/.duck/security.yaml   # User config
   sudo chmod 644 /etc/duck/security.yaml  # System config
   ```

#### Issue: "Signature verification failed"

**Symptoms:**
```
[duck] ❌ Signature verification failed: invalid signature
```

**Solutions:**
1. **Re-sign configuration:**
   ```bash
   duck security sign .duckfile/security.yaml
   ```

2. **Check key files:**
   ```bash
   ls -la ~/.duck/keys/
   # Ensure private.key and public.key exist
   ```

3. **Verify configuration hasn't been modified:**
   ```bash
   duck security verify --config security.yaml --verbose
   ```

#### Issue: "Host not allowed"

**Symptoms:**
```
[duck] ❌ Repository host 'untrusted-host.com' is not in allowed hosts list
```

**Solutions:**
1. **Add host to allowlist:**
   ```yaml
   # In security.yaml
   allowedHosts:
     - github.com
     - untrusted-host.com  # Add this
   ```

2. **Temporary CLI override:**
   ```bash
   duck --allowed-hosts github.com,untrusted-host.com build
   ```

3. **Environment override:**
   ```bash
   export DUCK_ALLOWED_HOSTS="github.com,untrusted-host.com"
   duck build
   ```

#### Issue: "Key not found"

**Symptoms:**
```
[duck] ❌ No suitable private key found for signing
```

**Solutions:**
1. **Generate keys:**
   ```bash
   duck security generate-keys
   ```

2. **Specify key location:**
   ```bash
   duck security sign config.yaml --key-file /path/to/private.key
   ```

3. **Set keys directory:**
   ```bash
   export DUCK_KEYS_PATH="/custom/keys/path"
   ```

### Debug Mode

Enable verbose logging for detailed troubleshooting:

```bash
# Enable debug logging
duck --log-level debug security status

# Trace security configuration resolution
duck --log-level debug --verbose security status
```

### Security Status Investigation

```bash
# Get comprehensive security status
duck security status --verbose --include-permissions

# Check effective configuration
duck security verify --verbose

# Validate all discovered configurations
duck security check-permissions --verbose
```

---

## Best Practices

### 1. Production Security Setup

#### Recommended Configuration
```yaml
# .duckfile/security.yaml
version: 1

# Explicit allowlist of trusted hosts
allowedHosts:
  - github.com
  - gitlab.internal.company.com
  - git.company.com

# Block known malicious hosts
deniedHosts:
  - malicious-git-host.com
  - compromised-server.net

# Enforce restrictions (fail if no security config)
strictMode: true
```

#### Signing for Production
```bash
# 1. Generate dedicated production keys
duck security generate-keys --output-dir ./production-keys

# 2. Sign all security configurations
duck security sign .duckfile/security.yaml --key-file ./production-keys/private.key

# 3. Distribute public keys to team
cp ./production-keys/public.key /shared/team-keys/prod-duck.pub

# 4. Verify configuration
duck security verify --config .duckfile/security.yaml --verbose
```

### 2. Team Collaboration

#### Key Management
1. **Centralized public keys**: Store team public keys in shared location
2. **Distributed signing**: Each team member can sign with their own keys
3. **Key rotation**: Regular key rotation for enhanced security

#### Configuration Sharing
```bash
# Team member 1: Create and sign config
duck security generate-keys
duck security sign .duckfile/security.yaml

# Team member 2: Add public key and verify
mkdir -p ~/.duck/keys/
cp /shared/keys/teammate1.pub ~/.duck/keys/
duck security verify --config .duckfile/security.yaml
```

### 3. CI/CD Integration

#### Secure Pipeline Configuration
```yaml
# .github/workflows/secure-build.yml
name: Secure Duck Build

on: [push, pull_request]

jobs:
  security-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Validate Security Configuration
        env:
          DUCK_ALLOWED_HOSTS: "github.com"
          DUCK_STRICT_MODE: "true"
        run: |
          # Verify security configuration
          duck security verify
          
          # Check file permissions
          duck security check-permissions
          
          # Display security status
          duck security status --verbose

      - name: Execute Secure Build
        env:
          DUCK_ALLOWED_HOSTS: "github.com"
          DUCK_STRICT_MODE: "true"
        run: duck build
```

#### Key Management in CI
```bash
# Store public keys as CI secrets/variables
# Use environment variables for host restrictions
# Never store private keys in CI - use external signing
```

### 4. Security Monitoring

#### Regular Security Audits
```bash
#!/bin/bash
# security-audit.sh

echo "🔍 Duck Security Audit - $(date)"
echo

# Check security status
echo "📊 Current Security Status:"
duck security status --verbose

echo "📁 File Permissions:"
duck security check-permissions --verbose

echo "🔐 Configuration Verification:"
duck security verify --verbose

# Check for unsigned configurations
find . -name "security.yaml" -o -name "security.yml" | while read config; do
  if [ ! -f "${config}.sig" ]; then
    echo "⚠️  Unsigned configuration: $config"
  fi
done
```

#### Security Alerting
Monitor for:
- Unsigned security configurations in production
- Invalid file permissions
- Failed signature verifications
- Attempts to access denied hosts

### 5. Development Workflows

#### Local Development
```yaml
# .duckfile/security.yaml (development)
version: 1
allowedHosts:
  - github.com
  - gitlab.com
  - localhost  # Allow local Git servers
strictMode: false  # More permissive for development
```

#### Staging Environment
```yaml
# .duckfile/security.yaml (staging)
version: 1
allowedHosts:
  - github.com
  - gitlab.internal.company.com
strictMode: true  # Production-like restrictions
```

#### Production Environment
```yaml
# .duckfile/security.yaml (production)
version: 1
allowedHosts:
  - gitlab.internal.company.com  # Only internal Git
strictMode: true
# This file MUST be signed for production use
```

---

## Advanced Usage

### 1. Custom Key Management

#### Multiple Key Support
```bash
# Generate multiple keys for different purposes
duck security generate-keys --output-dir ./dev-keys
duck security generate-keys --output-dir ./prod-keys

# Sign with specific keys
duck security sign config.yaml --key-file ./prod-keys/private.key
```

#### Hardware Security Modules (HSM)
```bash
# Export key to HSM format (implementation-specific)
# Use external tools to sign with HSM
# Import signature back to Duck format
```

### 2. Integration with External Systems

#### LDAP/AD Integration
```bash
# Use LDAP groups for key distribution
# Map LDAP users to Duck key IDs
# Implement custom key discovery logic
```

#### Certificate Authority Integration
```bash
# Use CA-signed certificates for key verification
# Implement certificate chain validation
# Integrate with PKI infrastructure
```

### 3. Custom Security Policies

#### Policy as Code
```yaml
# Custom security policy definition
version: 1

# Host access policies
policies:
  - name: "production"
    allowedHosts: ["git.internal.com"]
    strictMode: true
    
  - name: "development"  
    allowedHosts: ["github.com", "gitlab.com"]
    strictMode: false

# Apply policy based on environment
environmentPolicy:
  production: "production"
  staging: "production"
  development: "development"
```

#### Dynamic Policy Resolution
```bash
# Environment-based policy selection
DUCK_ENVIRONMENT=production duck build

# Custom policy validation
duck security verify --policy production
```

### 4. Audit and Compliance

#### Compliance Reporting
```bash
#!/bin/bash
# compliance-report.sh

echo "Duck Security Compliance Report"
echo "Generated: $(date)"
echo

# Security configuration inventory
echo "📋 Security Configurations:"
find . -name "security.yaml" -o -name "security.yml" | while read config; do
  echo "  - $config"
  if [ -f "${config}.sig" ]; then
    echo "    ✅ Signed"
  else
    echo "    ❌ Unsigned"
  fi
done

# Host access audit
echo "🌐 Host Access Controls:"
duck security status | grep -A 5 "Host Access Control"

# Permission audit
echo "📁 File Permissions:"
duck security check-permissions
```

#### Change Tracking
```bash
# Track security configuration changes
git log --oneline --follow .duckfile/security.yaml

# Verify signature history
git log --oneline --follow .duckfile/security.yaml.sig
```

### 5. Performance Optimization

#### Configuration Caching
```bash
# Cache security configurations for performance
# Implement configuration cache invalidation
# Use cache for frequent security checks
```

#### Parallel Verification
```bash
# Verify multiple configurations in parallel
# Optimize signature verification performance
# Batch file permission checks
```

---

This completes the comprehensive Duck Security System User Guide. The guide covers all aspects of Duck's security features, from basic setup to advanced enterprise usage patterns, providing users with everything they need to implement secure DevOps workflows.
