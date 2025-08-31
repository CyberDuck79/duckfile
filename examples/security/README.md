# Duck Security Configuration Examples

This directory contains example security configurations for different environments and use cases.

## Files

- `basic-security.yaml` - Simple host restrictions for getting started
- `enterprise-security.yaml` - Comprehensive enterprise security policy
- `development-security.yaml` - Development environment with relaxed policies
- `ci-cd-security.yaml` - CI/CD environment with strict enforcement
- `team-security.yaml` - Team-specific configuration example
- `project-security.yaml` - Project-specific overrides

## Usage

1. **Copy** an example that matches your environment
2. **Customize** the hosts and policies for your organization
3. **Place** the file in the appropriate location:
   - System-wide: `/etc/duckfile/security.yaml`
   - User-specific: `~/.duckfile/security.yaml` 
   - Project-specific: `./.duckfile/security.yaml`
4. **Sign** the configuration for tamper-proof enforcement:
   ```bash
   duck security generate-keys --output-dir ~/.duck/keys
   duck security sign ~/.duckfile/security.yaml
   ```
5. **Verify** the configuration works:
   ```bash
   duck security verify --config ~/.duckfile/security.yaml
   duck security status
   ```

## Configuration Hierarchy

Remember that signed configurations take precedence over unsigned ones, and the discovery order is:

1. **System** (`/etc/duckfile/security.yaml`) - Highest precedence
2. **User** (`~/.duckfile/security.yaml`) - Medium precedence  
3. **Project** (`./.duckfile/security.yaml`) - Lowest precedence

See the [User Guide](../../README.md#security-user-guide) for complete documentation.