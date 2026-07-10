# @probo/claude-plugin

Multi-agent plugin for open-source compliance workflows. Ships Agent
Skills–compatible instructions and wires agents to the [Probo MCP
API](https://github.com/getprobo/probo/tree/main/pkg/server/api/mcp/v1) via
OAuth 2.0.

**Supported agents:** Claude Code, Codex, OpenCode, Cursor (MCP + skills).

Marketplace catalogs: `.claude-plugin/marketplace.json` (Claude Code),
`.agents/plugins/marketplace.json` at the repo root or under
`packages/claude-plugin/` (Codex). See [COMPATIBILITY.md](./COMPATIBILITY.md).

## Install

### From npm

Add the marketplace catalog, then install the plugin:

```bash
claude plugin marketplace add ./packages/claude-plugin/.claude-plugin
claude plugin install probo@probo
```

When consuming the published package, the marketplace entry resolves
`@probo/claude-plugin` from npm (see `.claude-plugin/marketplace.json`).

### Configure Probo MCP

Set your Probo instance URL before starting Claude Code:

```bash
export PROBO_BASE_URL="https://your-probo-instance.example.com"
```

The plugin `.mcp.json` connects to `${PROBO_BASE_URL}/mcp/v1`. Probo MCP
authenticates with **OAuth 2.0** — no API token or bearer header is required in
the plugin config. On first use, sign in from Claude Code:

```text
/mcp
```

Or from your shell:

```bash
claude mcp login probo
```

Claude Code discovers Probo's authorization server via
`/.well-known/oauth-protected-resource` and stores tokens securely.

### Local development

```bash
claude --plugin-dir ./packages/claude-plugin
```

## What's included

| Component | Location | Purpose |
| --- | --- | --- |
| MCP | `.mcp.json` | Probo API connection |
| Skills | `skills/` | Compliance workflows |
| Commands | `commands/` | `access-review` — semi-auto campaign review |
| Agents | `agents/` | Reserved |
| Hooks | `hooks/` | Reserved |

Skills: `/probo:<skill-name>` (e.g. `/probo:open-source-compliance`).

Commands: `/probo:<command-name>` (e.g. `/probo:access-review Q3 GitHub review`).

## Adding content

See [`contrib/claude/claude-plugin.md`](../../contrib/claude/claude-plugin.md).

```bash
npm --workspace @probo/claude-plugin run validate
claude --plugin-dir ./packages/claude-plugin
```

## Release

Published to npm as `@probo/claude-plugin`. See
[`contrib/claude/release/claude-plugin.md`](../../contrib/claude/release/claude-plugin.md).
