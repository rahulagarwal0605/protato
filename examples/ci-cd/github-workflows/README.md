# CI/CD Integration with Protato

This directory contains example GitHub Actions workflows for integrating Protato into your CI/CD pipeline. These are **example files** that should be copied to your repository's `.github/workflows/` directory.

## What You'll Learn

- How to set up GitHub Actions with Protato
- How to automatically push protos on changes
- How to verify workspace integrity in pipelines

## Use Cases

### Use Case 1: Auto-Push on Changes

Automatically push proto files to registry when they change in your repository.

### Use Case 2: Verify Before Deploy

Verify workspace integrity before deploying services.

## ⚠️ Important

These files are **examples only**. To use them:

1. **Copy to your repository root**:
   ```bash
   mkdir -p .github/workflows
   cp examples/ci-cd/github-workflows/protato-push-with-pat.yml .github/workflows/
   ```

2. **Do NOT** place these files in `examples/ci-cd/` - they won't work there!

3. **Customize** the workflow for your needs after copying.

## Available Workflows

### Authentication Methods

Protato supports multiple authentication methods for GitHub Actions. Choose the one that best fits your security requirements:

| Workflow | Authentication | Use Case | Security Level |
|----------|---------------|----------|----------------|
| `protato-push-with-github-app.yml` | GitHub App | **RECOMMENDED** - Organization-wide, scalable | ⭐⭐⭐⭐⭐ |
| `protato-push-with-pat.yml` | Personal Access Token | Cross-repo access | ⭐⭐⭐⭐ |
| `protato-push-with-ssh.yml` | SSH Key | Most secure, cross-repo | ⭐⭐⭐⭐⭐ |
| `protato-push-with-deploy-key.yml` | Deploy Key | Repository-specific access | ⭐⭐⭐⭐ |

### protato-push-with-github-app.yml (GitHub App) - RECOMMENDED

**✅ RECOMMENDED** - Uses GitHub App for organization-wide, scalable authentication:
- ✅ Organization-wide access
- ✅ Scalable and maintainable
- ✅ Fine-grained permissions
- ✅ No user tokens required
- ✅ Centralized management
- ⚠️ Requires GitHub App setup

**Setup**:
1. Create GitHub App in organization
2. Install app on repositories
3. Add App ID and Private Key as secrets:
   - `PROTATO_GITHUB_APP_ID`
   - `PROTATO_GITHUB_APP_PRIVATE_KEY`

**Best for**: 
- ✅ Organizations and enterprises
- ✅ Multiple repositories
- ✅ Centralized authentication management
- ✅ Production environments

### protato-push-with-ssh.yml (SSH Key)

Uses SSH key authentication:
- ✅ Most secure method
- ✅ No tokens to manage
- ✅ Works with any Git host
- ⚠️ Requires SSH key setup

**Setup**:
1. Generate SSH key pair
2. Add public key to GitHub (Settings → SSH and GPG keys)
3. Add private key as secret: `PROTATO_SSH_PRIVATE_KEY`
4. Registry URL must use SSH format: `git@github.com:org/repo.git`

**Best for**: Maximum security, production environments

### protato-push-with-pat.yml (Personal Access Token)

Uses a Personal Access Token (PAT) for authentication:
- ✅ Fine-grained permissions
- ✅ **Can access multiple repositories** (unlike GITHUB_TOKEN)
- ✅ Can be scoped to specific operations
- ✅ **Can push to different repository** than where workflow runs
- ⚠️ Requires creating and storing PAT secret

**Setup**:
1. Create PAT with `repo` scope
2. Add as secret: `PROTATO_PAT`
3. Workflow uses PAT instead of GITHUB_TOKEN

**Best for**: 
- ✅ **Pushing to different repository** (most common use case)
- ✅ Registry in separate repository
- ✅ Cross-repository access
- ✅ Specific permissions needed

### protato-push-with-deploy-key.yml (Deploy Key)

Uses repository-specific deploy key:
- ✅ Repository-scoped access
- ✅ Read/write or read-only
- ✅ No user account required
- ⚠️ One key per repository

**Setup**:
1. Generate SSH key pair
2. Add public key as Deploy Key (Settings → Deploy keys)
3. Add private key as secret: `PROTATO_DEPLOY_KEY`
4. Registry URL must use SSH format

**Best for**: Single repository, read-only or write access

## Setup Instructions

### Step 1: Add Registry Secret

**Required for all workflows**:

1. Go to your GitHub repository
2. Navigate to **Settings → Secrets and variables → Actions**
3. Click **New repository secret**
4. Name: `PROTATO_REGISTRY_URL`
5. Value: Your registry Git URL (e.g., `https://github.com/your-org/proto-registry.git`)
6. Click **Add secret**

### Step 1b: Add Authentication Secrets (if needed)

**For PAT**:
- Secret name: `PROTATO_PAT` (or use `GITHUB_TOKEN`)

**For SSH/Deploy Key**:
- Secret name: `PROTATO_SSH_PRIVATE_KEY` or `PROTATO_DEPLOY_KEY`
- Value: Your SSH private key (entire key including `-----BEGIN OPENSSH PRIVATE KEY-----`)

**For GitHub App**:
- Secret name: `PROTATO_GITHUB_APP_ID` (App ID number)
- Secret name: `PROTATO_GITHUB_APP_PRIVATE_KEY` (App private key)

**Note**: All workflows automatically configure Git user name and email using `${{ github.actor }}` and `${{ github.actor }}@users.noreply.github.com`. This is required for Protato to create commits in the registry.

### Step 2: Choose Authentication Method

Select the workflow that matches your authentication method:

```bash
# From your repository root
mkdir -p .github/workflows

# Option 1: Personal Access Token (RECOMMENDED for cross-repo)
cp examples/ci-cd/github-workflows/protato-push-with-pat.yml .github/workflows/

# Option 2: SSH Key (Most secure)
cp examples/ci-cd/github-workflows/protato-push-with-ssh.yml .github/workflows/

# Option 3: GitHub App (Best for organizations)
cp examples/ci-cd/github-workflows/protato-push-with-github-app.yml .github/workflows/

# Option 4: Deploy Key (Repository-specific)
cp examples/ci-cd/github-workflows/protato-push-with-deploy-key.yml .github/workflows/
```

**Note**: All workflows support pushing to a different repository (cross-repo). GitHub App is recommended for organizations and production use.

### Step 3: Customize

Edit your copied workflow file:
- Adjust trigger conditions (`on:` section)
- Modify steps as needed
- Configure branch names
- Update registry URL format (HTTPS vs SSH) based on auth method
- **Pin Protato version** (see Version Management below)

### Step 4: Version Management

**Important**: All workflows pin Protato to a specific version (`v1.0.0` by default) for reproducibility.

**To use a different version**, you have two options:

**Option 1: Set environment variable** (recommended for flexibility):
```yaml
env:
  PROTATO_VERSION: v0.2.0  # Use specific version
```

**Option 2: Update the install step directly**:
```yaml
- name: Install Protato
  run: |
    PROTATO_VERSION="v0.2.0"  # Change version here
    curl -fsSL https://raw.githubusercontent.com/rahulagarwal0605/protato/main/dl/protato.sh -o /tmp/protato-installer
    chmod +x /tmp/protato-installer
    sudo mv /tmp/protato-installer /usr/local/bin/protato
```

**Why pin versions?**
- ✅ **Reproducibility**: Same version runs every time
- ✅ **Stability**: Avoids unexpected changes from `@latest`
- ✅ **Security**: Know exactly what code is running
- ✅ **Debugging**: Easier to troubleshoot when version is known

**Why use protato download script?**
- ✅ **Simpler**: Single command, handles all complexity
- ✅ **Consistent**: Same installation method as local development
- ✅ **Automatic caching**: Script caches binaries for faster subsequent runs
- ✅ **Cross-platform**: Automatically detects OS and architecture
- ✅ **Maintainable**: Single source of truth for installation logic

**To use latest version** (not recommended for production):
```yaml
- name: Install Protato
  run: |
    # Download latest release (not recommended - use specific version instead)
    # The protato download script defaults to latest if PROTATO_VERSION is not set
    curl -fsSL https://raw.githubusercontent.com/rahulagarwal0605/protato/main/dl/protato.sh -o /tmp/protato-installer
    chmod +x /tmp/protato-installer
    sudo mv /tmp/protato-installer /usr/local/bin/protato
```

## Usage Scenarios

### Scenario 1: Auto-push on Proto Changes

The workflow automatically pushes protos when:
- Files in `protos/` directory change
- `protato.yaml` is modified
- Changes are pushed to `main` or `develop` branches

### Scenario 2: Manual Trigger

Use `workflow_dispatch` to manually trigger:
1. Go to Actions tab in GitHub
2. Select "Protato Push Demo"
3. Click "Run workflow"
4. Choose branch and click "Run workflow"

### Scenario 3: Verify Before Deploy

The workflow can:
- Verify workspace integrity
- Check proto file consistency
- Validate registry state

## Customization Examples

### Change Registry URL

```yaml
env:
  PROTATO_REGISTRY_URL: https://github.com/your-org/proto-registry.git
```

### Add Authentication

```yaml
- name: Configure Git credentials
  run: |
    git config --global credential.helper store
    echo "https://${{ secrets.GITHUB_TOKEN }}@github.com" > ~/.git-credentials
```

### Custom Cache Location

```yaml
env:
  PROTATO_REGISTRY_CACHE: /tmp/protato-cache
```

### Use GitHub Actions Cache

```yaml
- name: Cache registry
  uses: actions/cache@v3
  with:
    path: ~/.cache/protato/registry
    key: protato-registry-${{ runner.os }}
```

## Best Practices

1. ✅ **Use Secrets**: Never hardcode registry URLs or credentials
2. ✅ **Verify First**: Always verify workspace before pushing
3. ✅ **Handle Errors**: Add proper error handling for failed pushes
4. ✅ **Use Cache**: Cache registry directory for faster builds
5. ✅ **Notifications**: Add notifications for failed pushes
6. ✅ **Branch Protection**: Use branch protection rules for registry
7. ✅ **Review Workflows**: Review workflow runs regularly

## Troubleshooting

### Workflow Not Triggering

- Check file paths in `on.push.paths`
- Ensure files are in the correct branch
- Verify workflow file is in `.github/workflows/`

### Authentication Errors

- Verify `PROTATO_REGISTRY_URL` secret is set
- Check repository permissions
- Ensure Git credentials are configured

### Build Failures

- Verify protoc is installed (if needed)
- Check Protato version exists in releases
- Review workflow logs for details
- Ensure curl is available (standard on GitHub Actions runners)

## Additional Workflow Examples

### Example 1: Simple Push on Changes

```yaml
name: Push Protos

on:
  push:
    paths:
      - 'protos/**'
      - 'protato.yaml'

jobs:
  push:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          PROTATO_VERSION="${PROTATO_VERSION:-v1.0.0}"
          curl -fsSL https://raw.githubusercontent.com/rahulagarwal0605/protato/main/dl/protato.sh -o /tmp/protato-installer
          chmod +x /tmp/protato-installer
          sudo mv /tmp/protato-installer /usr/local/bin/protato
      - run: protato push
        env:
          PROTATO_REGISTRY_URL: ${{ secrets.PROTATO_REGISTRY_URL }}
```

### Example 2: Verify Before Deploy

```yaml
name: Verify Workspace

on:
  pull_request:
    paths:
      - 'protos/**'

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: |
          PROTATO_VERSION="${PROTATO_VERSION:-v1.0.0}"
          go install github.com/rahulagarwal0605/protato@${PROTATO_VERSION}
      - run: protato verify
        env:
          PROTATO_REGISTRY_URL: ${{ secrets.PROTATO_REGISTRY_URL }}
```

## Local Testing

To test Protato commands locally before setting up CI/CD:

```bash
# Initialize workspace
protato init --project payments/api

# Create proto files
mkdir -p protos/payments/api/v1
# ... create your .proto files ...

# Test push locally
protato push

# Test verify in another directory
protato verify

# Verify workspace
protato verify
```

## Authentication Guide

All workflows support pushing to a different repository (cross-repo). Choose the authentication method that best fits your security requirements:

**Quick reference**:
- **GitHub App**: **RECOMMENDED** - Best for organizations and production
- **PAT**: Personal Access Token - Cross-repo access
- **SSH**: Most secure, key-based authentication
- **Deploy Key**: Repository-specific access

## Related Documentation

- [Main Examples README](../../README.md) - All examples overview
- [Workflow Basic](../../cli/workflow-basic/README.md) - Core commands with auto-discovery: init, new, push, pull
- [Workflow Advanced](../../cli/workflow-advanced/README.md) - Core commands without auto-discovery: explicit project specification
- [Inspect](../../cli/inspect/README.md) - Inspection commands: mine, list, verify
- [Command Reference](../../../docs/COMMAND_REFERENCE.md) - All commands
- [GitHub Actions Docs](https://docs.github.com/en/actions) - Official documentation
