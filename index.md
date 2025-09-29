---
layout: default
title: Home
---

<div class="text-center">
  <img src="{{ '/assets/logo_text.svg' | relative_url }}" alt="Duckfile" style="max-width: 400px; margin: 2em 0;">
  
  <h1 class="mb-3">Secure Configuration Management Made Simple</h1>
  
  <p class="lead mb-3" style="font-size: 1.2em; color: #666; max-width: 600px; margin: 0 auto 2em;">
    Duckfile is a powerful configuration management tool that provides secure, 
    templated configurations with Git-based workflows and enterprise security features.
  </p>
  
  <div class="mb-3">
    <a href="{{ '/docs/getting-started/' | relative_url }}" class="btn btn-primary" style="margin-right: 10px;">Get Started</a>
    <a href="https://github.com/{{ site.repository }}" class="btn btn-secondary">View on GitHub</a>
  </div>
</div>

<div style="max-width: 1000px; margin: 3em auto;">
  <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 2em; margin: 3em 0;">
    <div style="background: #f8f9fa; padding: 2em; border-radius: 8px; border-left: 4px solid #08363E;">
      <h3 style="margin-top: 0; color: #08363E;">🚀 Easy to Use</h3>
      <p>Get started in minutes with our interactive setup wizard. Create your first configuration with <code>duck init</code> and start managing templates immediately.</p>
    </div>
    
    <div style="background: #f8f9fa; padding: 2em; border-radius: 8px; border-left: 4px solid #08363E;">
      <h3 style="margin-top: 0; color: #08363E;">🔒 Enterprise Security</h3>
      <p>Built-in security features including digital signatures, access control, and file permission management. Perfect for compliance-critical environments.</p>
    </div>
    
    <div style="background: #f8f9fa; padding: 2em; border-radius: 8px; border-left: 4px solid #08363E;">
      <h3 style="margin-top: 0; color: #08363E;">⚡ Git-Native</h3>
      <p>Seamless Git integration with template repositories, commit tracking, and automatic updates. Leverage your existing Git workflows.</p>
    </div>
  </div>
</div>

## Key Features

- **Template Management**: Git-based templates with Go template syntax support
- **Variable System**: Environment variables, command outputs, and file-based configuration
- **Security First**: Digital signatures, access control, and permission management
- **Interactive Setup**: Wizard-driven configuration creation and management
- **CLI Integration**: Seamless integration with existing development workflows
- **Enterprise Ready**: Multi-scope security policies and compliance features

## Quick Example

```yaml
# duck.yaml
targets:
  default:
    remote:
      repo: "https://github.com/example/templates"
      path: "docker-compose"
    variables:
      app_name: "my-app"
      port: !env PORT
    binary: "docker-compose"
    args: ["up"]
```

```bash
# Initialize and run
duck init
duck  # Renders template and runs docker-compose up
```

## What's Next?

1. [**Getting Started**]({{ '/docs/getting-started/' | relative_url }}) - Install Duck and create your first configuration
2. [**Core Concepts**]({{ '/docs/core-concepts/' | relative_url }}) - Learn about targets, templates, and variables
3. [**Security Guide**]({{ '/docs/security/' | relative_url }}) - Implement enterprise security features
4. [**CLI Reference**]({{ '/docs/reference/cli/' | relative_url }}) - Complete command reference

---

<div class="text-center" style="margin: 3em 0;">
  <p style="color: #666;">Ready to get started? Follow our step-by-step tutorial!</p>
  <a href="{{ '/docs/getting-started/installation/' | relative_url }}" class="btn btn-primary">Install Duckfile</a>
</div>