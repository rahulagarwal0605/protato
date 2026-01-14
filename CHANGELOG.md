# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive test suite with unit, integration, and e2e tests
- Debug logging throughout the codebase for troubleshooting
- Support for parallel test execution
- Test helpers and utilities for test development

### Fixed
- Fixed parallel test failures by setting working directory for bare Git repositories
- Resolved HEAD detection issues in shallow clones
- Fixed protoc resolver test paths to match service prefix structure

### Changed
- Simplified Snapshot() function to use FETCH_HEAD then HEAD fallback
- Improved Git command execution for bare repositories
- Enhanced error handling and logging

## [0.1.0] - 2024-01-XX

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

### Features
- **Workspace Management**: Initialize and manage protato workspaces
- **Project Ownership**: Claim and manage project ownership
- **Registry Operations**: Push and pull projects from Git-based registry
- **Dependency Resolution**: Automatic transitive dependency discovery
- **Version Tracking**: Git commits as version identifiers
- **File Management**: Track owned files and manage ignores
- **Verification**: Verify workspace integrity

[Unreleased]: https://github.com/rahulagarwal0605/protato/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/rahulagarwal0605/protato/releases/tag/v0.1.0
