# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for
receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| Latest  | :white_check_mark: |
| < Latest | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

- **Email**: [rahulagarwal0605@gmail.com]
- **GitHub Security Advisory**: Use the [Security tab](https://github.com/rahulagarwal0605/protato/security/advisories/new) in the repository

### What to Include

When reporting a security vulnerability, please include:

- **Type of issue** (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
- **Full paths of source file(s) related to the manifestation of the issue**
- **The location of the affected source code** (tag/branch/commit or direct URL)
- **Step-by-step instructions to reproduce the issue**
- **Proof-of-concept or exploit code** (if possible)
- **Impact of the issue**, including how an attacker might exploit the issue

This information will help us triage your report more quickly.

## Disclosure Policy

When we receive a security bug report, we will assign it to a primary handler.
This person will coordinate the fix and release process, involving the following steps:

1. **Confirm the problem** and determine the affected versions
2. **Audit code** to find any potential similar problems
3. **Prepare fixes** for all releases still under maintenance
4. **Release fixes** as soon as possible

We follow a **coordinated disclosure** process:

- **Initial Response**: We will acknowledge receipt within 48 hours
- **Status Updates**: We will provide updates every 7 days until resolution
- **Resolution**: We aim to resolve critical issues within 30 days

## Security Best Practices

### For Users

- **Keep Protato updated** to the latest version
- **Use HTTPS** for registry URLs when possible
- **Review registry access** permissions regularly
- **Validate proto files** before pushing to registry
- **Use `.gitignore`** to exclude sensitive files

### For Developers

- **Never commit secrets** or credentials
- **Validate all inputs** from external sources
- **Use secure defaults** in configuration
- **Follow secure coding practices**
- **Keep dependencies updated**

## Known Security Considerations

### Git Repository Access

- Protato clones Git repositories locally
- Ensure registry URLs are from trusted sources
- Be cautious with local file:// URLs in untrusted environments

### File System Access

- Protato reads and writes files in your workspace
- Ensure proper file permissions are set
- Be cautious when running in shared environments

### Network Operations

- Protato performs Git operations over network
- Use HTTPS or SSH for secure transport
- Verify SSL certificates when using HTTPS

## Security Updates

Security updates will be:

- **Announced** in release notes
- **Tagged** with security labels
- **Documented** in CHANGELOG.md
- **Backported** to supported versions when applicable

## Credits

We appreciate responsible disclosure and will credit security researchers who
report valid vulnerabilities (with their permission) in:

- Release notes
- Security advisories
- Project documentation

Thank you for helping keep Protato and its users safe!
