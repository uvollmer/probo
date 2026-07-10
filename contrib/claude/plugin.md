# Agent plugin (`packages/plugin`)

npm package [`@probo/plugin`](../../packages/plugin) ships a multi-agent plugin
for open-source compliance workflows powered by the Probo MCP API. Compatible
with **Claude Code**, **Codex**, **OpenCode**, and **Cursor** (via MCP +
skills). See [`COMPATIBILITY.md`](../../packages/plugin/COMPATIBILITY.md).

## What this plugin is

A **multi-agent plugin** (the installable unit) bundling:

| Component | Role |
| --- | --- |
| `.mcp.json` | Connects agents to Probo (`/mcp/v1`, OAuth 2.0) |
| `skills/` | Workflow instructions and reference docs |
| `commands/` | Explicit slash commands (e.g. `access-review`, Claude Code only) |
| `agents/` | Optional specialized subagents |
| `hooks/` | Optional event automation |

Individual capabilities are namespaced under `probo`:

- Skills: `/probo:<skill-name>` (e.g. `/probo:open-source-compliance`)
- Commands: `/probo:<command-name>` (e.g. `/probo:access-review`)

Published to npm as `@probo/plugin`. Agent-specific manifests (`.claude-plugin/`,
`.codex-plugin/`) ship inside the same package.

## Directory structure

```
.agents/plugins/marketplace.json   # repo root — Codex catalog for getprobo/probo

packages/plugin/
  .claude-plugin/
    plugin.json           # Claude Code manifest (required)
    marketplace.json      # Claude marketplace catalog (npm)
  .agents/plugins/
    marketplace.json      # Codex catalog when marketplace root is the package
  .codex-plugin/
    plugin.json           # Codex manifest
  .mcp.json               # Probo MCP server wiring
  skills/
    <skill-name>/
      SKILL.md
      references/
  commands/
  agents/
  hooks/
  scripts/validate.mjs
  package.json
  CHANGELOG.md
```

Only `plugin.json` belongs inside `.claude-plugin/`. All other directories
must sit at the plugin root.

## plugin.json rules

Claude Code validates the manifest strictly. Common pitfalls:

| Field | Expected type | Notes |
| --- | --- | --- |
| `name` | string | Skill namespace (`probo` → `/probo:open-source-compliance`) |
| `repository` | string URL | **Not** the npm-style `{ type, url }` object |
| `bugs` | string URL | **Not** the npm-style `{ url }` object |
| `version` | string | Bump on every release when using explicit versioning |

Run `npm --workspace @probo/plugin run validate` before publishing.

## Probo MCP configuration

The plugin `.mcp.json` expects one environment variable:

- `PROBO_BASE_URL` — instance root URL

Authentication is OAuth 2.0 only. Users complete sign-in via `/mcp` or
`claude mcp login probo`. Do not document API keys or bearer tokens in the
plugin config — a pre-set `Authorization` header prevents Claude Code from
starting the OAuth flow.

## Adding a skill

1. Create `skills/<name>/SKILL.md` with YAML frontmatter (`name`, `description`).
2. Add `references/` for detailed workflow docs loaded on demand.
3. Validate and test:

```bash
npm --workspace @probo/plugin run validate
claude --plugin-dir ./packages/plugin
/probo:<name>
```

4. Update `packages/plugin/CHANGELOG.md` under `## Unreleased`.

Skills must be self-contained — npm installs do not include `contrib/claude/`
from the monorepo.

## Adding a command

Use commands for explicit, user-invoked workflows on Claude Code only. Pair a
thin `commands/<name>.md` with a shared `skills/<name>/SKILL.md` so Codex and
OpenCode load the same workflow. Reference docs live under
`skills/<name>/references/` using paths relative to the skill directory (not
`${CLAUDE_PLUGIN_ROOT}`).

1. Create `commands/<name>.md` with frontmatter (`description`,
   `argument-hint`, `disable-model-invocation: true` when writes are involved).
2. Add reference docs under `skills/<name>/references/`.
3. Register paths in `scripts/validate.mjs`.
4. Test: `/probo:<name> <args>` after `claude --plugin-dir ./packages/plugin`.

## Distribution

Published to npm as `@probo/plugin`. Claude marketplace entry:

```json
{
  "source": {
    "source": "npm",
    "package": "@probo/plugin"
  }
}
```

Release process: [`contrib/claude/release/plugin.md`](release/plugin.md).
