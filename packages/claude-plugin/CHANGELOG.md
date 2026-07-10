# Changelog

All notable changes to the `@probo/claude-plugin` package will be documented in
this file.

## Unreleased

### Added

- `access-review` command for semi-automated campaign review via Probo MCP
- Reference docs for MCP tools, decision rubric, and `.probo/access-reviews/` notes format

### Changed

- Refocus the plugin on open-source compliance workflows powered by Probo MCP
- Replace the dev `commit` skill with `open-source-compliance`

### Changed

- Use OAuth 2.0 for Probo MCP instead of bearer token configuration

### Added

- Probo MCP wiring via `.mcp.json` (`PROBO_BASE_URL`, OAuth sign-in via `/mcp`)

## [0.1.0] - 2026-07-01

### Added

- Initial Claude Code plugin scaffold with manifest and marketplace metadata
- Package validation script and npm publish configuration
