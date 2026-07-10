# Multi-agent compatibility

`@probo/claude-plugin` targets **Claude Code**, **Codex**, **OpenCode**, and
other MCP-capable agents (including **Cursor**). The portable core is Probo
MCP plus Agent Skills–compatible `SKILL.md` files.

## What works where

| Component | Claude Code | Codex | OpenCode | Cursor |
| --- | --- | --- | --- | --- |
| Probo MCP (OAuth) | ✅ Plugin `.mcp.json` | ✅ `.codex-plugin` + `.mcp.json` | ✅ Manual MCP config | ✅ IDE MCP settings |
| Skills (`SKILL.md`) | ✅ `skills/` | ✅ `skills/` via `.codex-plugin` | ✅ `.opencode/skills/` or `.claude/skills/` | ✅ Copy/symlink to `.cursor/skills/` |
| Commands | ✅ `commands/` → `/probo:…` | ⚠️ Use skills instead | ⚠️ Native `skill` tool | ❌ Use skill or rules |
| Plugin manifest | `.claude-plugin/` | `.codex-plugin/` | Discovery paths (no manifest) | No native manifest |

## Probo MCP (all agents)

Set the instance URL:

```bash
export PROBO_BASE_URL="https://your-probo-instance.example.com"
```

Endpoint: `${PROBO_BASE_URL}/mcp/v1` (HTTP, OAuth 2.0). Do not configure a
static bearer token — OAuth discovery uses
`/.well-known/oauth-protected-resource`.

### Claude Code

```bash
claude plugin marketplace add ./packages/claude-plugin/.claude-plugin
claude plugin install probo@probo
claude mcp login probo   # or /mcp in session
/probo:access-review Q3 GitHub review
```

### Codex

Install the plugin directory (contains `.codex-plugin/plugin.json`):

```bash
codex plugin install /path/to/packages/claude-plugin
codex mcp login probo
```

Or add MCP manually in `~/.codex/config.toml` pointing at
`${PROBO_BASE_URL}/mcp/v1`, then `codex mcp login probo`.

Skills load from `./skills/` automatically via the Codex manifest.

### OpenCode

OpenCode discovers skills at `.opencode/skills/`, `.claude/skills/`, and
`~/.config/opencode/skills/`. Options:

**Option A — symlink from this package:**

```bash
mkdir -p .opencode/skills
ln -s ../../packages/claude-plugin/skills/access-review .opencode/skills/access-review
ln -s ../../packages/claude-plugin/skills/open-source-compliance .opencode/skills/open-source-compliance
```

**Option B — Claude Code bridge:** install
[`opencode-claude-code-bridge`](https://www.npmjs.com/package/opencode-claude-code-bridge)
to import Claude plugins and MCP configs into OpenCode.

Configure Probo MCP in `opencode.json` or global OpenCode MCP settings, then
authenticate. Invoke via the native `skill` tool (`access-review`).

### Cursor

1. Add Probo MCP in Cursor settings (HTTP URL: `${PROBO_BASE_URL}/mcp/v1`,
   OAuth).
2. Copy or symlink skills into `.cursor/skills/`:

```bash
mkdir -p .cursor/skills
cp -r packages/claude-plugin/skills/access-review .cursor/skills/
```

Reference the skill in chat or add a Cursor rule pointing at the skill.

## Portable vs agent-specific paths

| Path | Portable? |
| --- | --- |
| `skills/<name>/SKILL.md` | ✅ Agent Skills standard |
| `skills/<name>/references/*.md` | ✅ Relative to skill directory |
| `.mcp.json` with `${PROBO_BASE_URL}` | ✅ Standard env var |
| `${CLAUDE_PLUGIN_ROOT}` | ❌ Claude Code only — avoid in skill bodies |
| `commands/*.md` | Claude Code slash commands only |

Skill bodies use **relative** `references/` paths so they work once the skill
directory is discovered, regardless of which agent loads it.

## npm package layout

```
@probo/claude-plugin/
  .claude-plugin/plugin.json    # Claude Code
  .claude-plugin/marketplace.json
  .codex-plugin/plugin.json     # Codex
  .mcp.json                     # Shared MCP wiring
  skills/                       # Shared skills (all agents)
  commands/                     # Claude Code commands only
```
