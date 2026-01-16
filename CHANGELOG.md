# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.2] - 2026-01-16

### Fixed
- Fix CI/CD workflow failures by removing dependency on non-existent `GITHUB_ACTOR_EMAIL` environment variable
- Simplify `GetUser()` to rely solely on git config (`user.name` and `user.email`)
- Improve reliability in GitHub Actions workflows by using git config that workflows already configure

### Changed
- `GetUser()` now only reads from git config, removing GitHub Actions environment variable checks
- More consistent behavior across all environments (local, CI/CD, etc.)

## [1.0.1] - 2026-01-14

### Fixed
- Fix init force behavior to do complete override instead of merge
- Move already-initialized check earlier in init flow for better UX
- Simplify workspace initialization logic

## [1.0.0] - 2026-01-14

### Added
- Initial release of Protato
- Core CLI commands: `init`, `new`, `pull`, `push`, `verify`, `list`, `mine`
- Git-based registry for protobuf definitions
- Local-first workflow with ownership model
- Automatic dependency resolution
- Version tracking via Git commits
- Workspace configuration via `protato.yaml`
- Support for project path patterns and ignores
- Protoc integration for import resolution
- Comprehensive logging with zerolog
- Cross-platform support (Linux, macOS, Windows)
- Comprehensive test suite with unit, integration, and e2e tests
- Debug logging throughout the codebase for troubleshooting
- Support for parallel test execution
- Test helpers and utilities for test development
- BSR dependency resolution via buf export
- File lock to prevent concurrent cache access
- Configurable import path mapping
- GitHub Actions workflows for CI/CD
- Test coverage reporting
- Auto-discovery of projects from directory structure
- Project ownership model with single owner per project
- Workspace verification and integrity checks
- Snapshot tracking via Git commit hashes

### Features
- **Workspace Management**: Initialize and manage protato workspaces
- **Project Ownership**: Claim and manage project ownership
- **Registry Operations**: Push and pull projects from Git-based registry
- **Dependency Resolution**: Automatic transitive dependency discovery
- **Version Tracking**: Git commits as version identifiers
- **File Management**: Track owned files and manage ignores
- **Verification**: Verify workspace integrity

[Unreleased]: https://github.com/rahulagarwal0605/protato/compare/v1.0.2...HEAD
[1.0.2]: https://github.com/rahulagarwal0605/protato/releases/tag/v1.0.2
[1.0.1]: https://github.com/rahulagarwal0605/protato/releases/tag/v1.0.1
[1.0.0]: https://github.com/rahulagarwal0605/protato/releases/tag/v1.0.0
