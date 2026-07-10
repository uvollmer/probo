# Changelog

All notable changes to the `@probo/claude-plugin` package will be documented in
this file.

## Unreleased

### Added

- Codex marketplace at `.agents/plugins/marketplace.json`
- Multi-agent support: `.codex-plugin/plugin.json` and `COMPATIBILITY.md`
- `skills/access-review/SKILL.md` as canonical workflow (Codex, OpenCode, Cursor)
- Portable relative `references/` paths (no `${CLAUDE_PLUGIN_ROOT}` in skills)

### Changed

- `access-review` Claude command delegates to the shared skill

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
