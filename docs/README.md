# Duck Security Documentation

This directory contains comprehensive documentation for Duck's enterprise-grade security features.

## Documentation Index

### 📖 **[Security User Guide](security-user-guide.md)**
Comprehensive guide covering all security features including:
- Quick start and basic setup
- Digital signatures and cryptographic verification
- Host access control and supply chain protection
- Configuration precedence and security hierarchy
- CLI security commands and management tools
- File permissions and security validation
- Environment variables and CI/CD integration
- Troubleshooting and best practices
- Advanced usage patterns

### 📋 **[Configuration Specification](spec.md)**
Complete duck.yaml configuration reference including security-related settings

### 🔧 **[Security Schema](security.schema.json)**
JSON schema for security configuration validation

## Quick Security Setup

### Basic Security Configuration
```bash
# Create security configuration
mkdir -p .duckfile
cat > .duckfile/security.yaml << EOF
version: 1
allowedHosts:
  - github.com
  - gitlab.com
strictMode: true
EOF

# Check security status
duck security status
```

### Advanced Setup with Digital Signatures
```bash
# Generate cryptographic keys
duck security generate-keys

# Sign security configuration
duck security sign .duckfile/security.yaml

# Verify signed configuration
duck security verify --config .duckfile/security.yaml
```

## Security Architecture

Duck implements a comprehensive 5-tier security precedence system:

```
🔒 Signed Security Config Files    (Highest Priority)
⚡ CLI Flags                       (High Priority)  
🌍 Environment Variables           (Medium Priority)
📄 Unsigned Security Config Files  (Low Priority)
🔓 No Restrictions                 (Lowest Priority)
```

## Security Features Overview

- **🔐 Digital Signatures**: Ed25519 cryptographic verification
- **🛡️ Host Access Control**: Allow/deny lists for Git repositories
- **📁 File Permissions**: Secure configuration file handling
- **⚡ Precedence System**: Multi-tier security hierarchy
- **🔧 CLI Security Tools**: Complete security management commands
- **🌍 Environment Integration**: CI/CD and automation support

## Getting Help

- Read the **[Security User Guide](security-user-guide.md)** for detailed instructions
- Use `duck security --help` for CLI command reference
- Use `duck security status` to check current security configuration
- Use `duck security verify` to validate security settings

For security issues or questions, please refer to the troubleshooting section in the user guide.
