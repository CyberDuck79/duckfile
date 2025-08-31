# Duck Security Configuration Examples

This directory contains example security configurations for different use cases and environments.

## Configuration Examples

### 📋 **[basic-security.yaml](basic-security.yaml)**
Simple security configuration perfect for getting started with Duck security features.
- Basic host allowlist (github.com, gitlab.com)
- Deny known malicious hosts
- Strict mode enabled

**Use case:** Individual developers, small teams, getting started with security

### 🏢 **[enterprise-security.yaml](enterprise-security.yaml)**
Comprehensive security policy for production enterprise environments.
- Strict host allowlist with internal Git servers
- Comprehensive policy enforcement
- Audit and compliance metadata
- SOC2 and ISO27001 compliance ready

**Use case:** Large organizations, production environments, compliance requirements

### 🔄 **[ci-cd-security.yaml](ci-cd-security.yaml)**
Maximum security configuration for automated CI/CD environments.
- Zero-trust security model
- Mandatory checksums and commit tracking
- Disabled auto-updates for stability
- Immutable infrastructure support

**Use case:** CI/CD pipelines, automated builds, GitHub Actions, GitLab CI

### 🛠️ **[development-security.yaml](development-security.yaml)**
Balanced security configuration for development environments.
- More permissive than production
- Includes localhost for local development
- Flexible host restrictions
- Development-friendly policies

**Use case:** Local development, testing, development teams

### 📊 **[policy-violations-demo.yaml](policy-violations-demo.yaml)**
Example configuration demonstrating common policy violations for testing.
- Shows what NOT to do for security
- Useful for testing security validation
- Contains intentional security issues

**Use case:** Testing, security validation, training

## Usage

### Copy and Customize
```bash
# Copy a template to your project
cp examples/security/basic-security.yaml .duckfile/security.yaml

# Edit for your needs
vim .duckfile/security.yaml
```

### Validate Configuration
```bash
# Check syntax and security
duck security verify --config .duckfile/security.yaml

# View effective security settings
duck security status --verbose
```

### Sign Configuration (Recommended)
```bash
# Generate keys if needed
duck security generate-keys

# Sign the configuration
duck security sign .duckfile/security.yaml

# Verify signature
duck security verify --config .duckfile/security.yaml
```

## Environment-Specific Deployment

### Development Environment
```bash
cp examples/security/development-security.yaml .duckfile/security.yaml
```

### Production Environment
```bash
cp examples/security/enterprise-security.yaml .duckfile/security.yaml
duck security generate-keys
duck security sign .duckfile/security.yaml
```

### CI/CD Environment
```bash
cp examples/security/ci-cd-security.yaml .duckfile/security.yaml
duck security sign .duckfile/security.yaml
```

## Configuration Guidelines

### Security Levels

**🟢 Development (Low Security)**
- Permissive host policies
- Local development support
- Flexible validation

**🟡 Staging (Medium Security)**
- Production-like restrictions
- Signed configurations recommended
- Audit logging enabled

**🔴 Production (High Security)**
- Strict host allowlists
- Mandatory signed configurations
- Full policy enforcement
- Compliance metadata

### Best Practices

1. **Always use signed configurations in production**
2. **Implement least-privilege host access**
3. **Enable strict mode for production environments**
4. **Use environment-specific configurations**
5. **Regularly review and update security policies**
6. **Include audit metadata for compliance**

## Documentation References

- **[Security User Guide](../../docs/security-user-guide.md)** - Complete security features guide
- **[Configuration Specification](../../docs/spec.md)** - Duck configuration reference
- **[Security Schema](../../docs/security.schema.json)** - JSON schema validation

## Support

For security questions or issues:
1. Check the [Security User Guide](../../docs/security-user-guide.md)
2. Use `duck security --help` for command reference
3. Use `duck security status` to diagnose configuration issues