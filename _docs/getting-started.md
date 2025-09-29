---
layout: docs
title: Getting Started
description: Learn how to install and use Duckfile for the first time
---

# Getting Started with Duckfile

Welcome to Duckfile! This guide will help you get up and running with secure configuration management in just a few minutes.

## What is Duckfile?

Duckfile is a configuration management tool that combines the power of Git-based templates with enterprise-grade security features. It allows you to:

- Manage configuration templates in Git repositories
- Use variables from multiple sources (environment, commands, files)
- Render templates and execute commands in one step
- Implement security policies and digital signatures
- Share configurations across teams and projects

## Prerequisites

Before you begin, make sure you have:

- **Git** installed and configured
- **Go 1.19+** (if building from source)
- Basic familiarity with command line tools
- A text editor for configuration files

## Installation Methods

### Option 1: Download Pre-built Binary (Recommended)

1. Visit the [releases page](https://github.com/CyberDuck79/duckfile/releases)
2. Download the appropriate binary for your platform
3. Move it to a directory in your PATH
4. Make it executable (Unix/macOS): `chmod +x duck`

### Option 2: Install with Go

```bash
go install github.com/CyberDuck79/duckfile/cmd/duck@latest
```

### Option 3: Build from Source

```bash
git clone https://github.com/CyberDuck79/duckfile.git
cd duckfile
go build -o duck cmd/duck/main.go
```

## Verify Installation

Confirm Duck is installed correctly:

```bash
duck version
```

You should see version information displayed.

## Your First Configuration

Let's create a simple configuration to get familiar with Duck:

### Step 1: Initialize a New Project

```bash
mkdir my-duck-project
cd my-duck-project
duck init
```

The interactive wizard will guide you through:
- Choosing a template repository
- Setting up variables
- Configuring the binary to execute

### Step 2: Examine the Configuration

After initialization, you'll have a `duck.yaml` file:

```yaml
targets:
  default:
    remote:
      repo: "https://github.com/example/docker-templates"
      path: "basic-app"
    variables:
      app_name: "my-app"
      port: !env PORT
    binary: "docker-compose"
    args: ["up", "-d"]
```

### Step 3: Run Your Configuration

```bash
duck
```

This will:
1. Fetch the template from Git
2. Render it with your variables
3. Execute the specified binary with arguments

## Understanding the Workflow

1. **Template Fetching**: Duck clones or updates the Git repository
2. **Variable Resolution**: Variables are resolved from environment, commands, or files
3. **Template Rendering**: Go templates are processed with your variables
4. **Binary Execution**: The specified command runs with the rendered configuration

## Common Use Cases

- **Docker Compose**: Render compose files with environment-specific variables
- **Kubernetes**: Generate manifests with dynamic values
- **CI/CD**: Template pipeline configurations
- **Infrastructure as Code**: Generate Terraform configurations
- **Application Config**: Create environment-specific app configurations

## Next Steps

Now that you have Duck installed and working:

1. [**Core Concepts**]({{ '/docs/core-concepts/' | relative_url }}) - Learn about targets, templates, and variables
2. [**Interactive Setup**]({{ '/docs/interactive-setup/' | relative_url }}) - Master the `duck init` and `duck add` commands
3. [**Variable System**]({{ '/docs/variables/' | relative_url }}) - Understand different variable types and sources

## Getting Help

If you run into issues:

- Use `duck --help` for command help
- Check our [Troubleshooting Guide]({{ '/docs/troubleshooting/' | relative_url }})
- Browse the [CLI Reference]({{ '/docs/reference/cli/' | relative_url }})
- Open an issue on [GitHub](https://github.com/CyberDuck79/duckfile/issues)

---

**Ready for the next step?** Learn about [Core Concepts]({{ '/docs/core-concepts/' | relative_url }}) to understand how Duck works under the hood.