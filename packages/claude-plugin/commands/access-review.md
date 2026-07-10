---
description: Run a semi-automated access review on a Probo campaign. Uses MCP tools to list pending entries, records clear decisions, and persists working notes to .probo/access-reviews/.
argument-hint: [campaign name or id]
disable-model-invocation: true
---

# Access review command

Run a **semi-automated** access review for campaign `$ARGUMENTS`. Review
entries only — do not create, start, cancel, or close campaigns.

Load reference docs from `${CLAUDE_PLUGIN_ROOT}/skills/access-review/references/`
before executing:

- `mcp-tools.md` — MCP tool names, inputs, pagination
- `decision-rubric.md` — semi-auto decision rules
- `notes-format.md` — working memory file schema

## Preconditions

1. Probo MCP must be connected. If tools fail with auth errors, stop and tell
   the user to run `/mcp` or `claude mcp login probo`.
2. Resolve the campaign from `$ARGUMENTS` (name match or GID). If ambiguous,
   list `listAccessReviewCampaigns` results and ask the user to pick one.
3. Campaign `status` must be `IN_PROGRESS` or `PENDING_ACTIONS`. Stop with a
   clear message for `DRAFT`, `COMPLETED`, or `CANCELLED`.

## Working notes file

Create or resume `.probo/access-reviews/<campaign-slug>.md` per
`notes-format.md`. Use the campaign slug from the campaign name (lowercase,
hyphens). On resume, read `last_cursor` and pending counts from the file.

Create `.probo/access-reviews/` if missing.

## Workflow

### 1. Orient

- Call `getAccessReviewStatistics` for the campaign.
- Summarize totals and pending count for the user.
- If no pending entries, report completion and stop (do not close the campaign).

### 2. Fetch batch

- Call `listAccessEntries` with `campaign_id`, `filter.decision: PENDING`, and
  `size: 50`.
- Use `last_cursor` from the notes file when resuming; omit cursor on a fresh run
  unless the user asks to continue a specific checkpoint.

### 3. Classify each entry

For every entry in the batch, apply `decision-rubric.md`:

| Class | Action |
| --- | --- |
| **Auto** | Queue for `recordAccessReviewEntryDecisions` |
| **Ambiguous** | Add to a review table for the user; do not write yet |
| **Skip** | Log in notes only (e.g. missing required context) |

Before writing, append each auto decision to the notes file **Entry notes**
table with rationale.

### 4. Write auto decisions

- Call `recordAccessReviewEntryDecisions` for all auto-classified entries in one
  batch when possible.
- Non-`APPROVED` decisions **must** include `decision_note` (see rubric).
- On MCP error, stop, log the error in the session log, and do not advance
  `last_cursor`.

### 5. Present ambiguous entries

Show a concise table: email, roles, flags, incremental_tag, is_admin, active,
proposed decision, rationale. Ask the user how to proceed. Only call
`recordAccessReviewEntryDecision` after explicit confirmation.

Optionally call `flagAccessReviewEntry` when the user agrees flags are missing.

### 6. Checkpoint

Update the notes file:

- `last_cursor` from `listAccessEntries` `next_cursor` (null when done)
- Session log line with batch stats (approved, revoked, deferred, escalated,
  ambiguous, skipped)
- `updated_at` timestamp

If `next_cursor` is set, ask whether to continue the next batch.

## Output

End each run with:

1. Batch summary (counts by decision)
2. Path to the notes file
3. Remaining pending count (from statistics or cursor state)
4. List of ambiguous entries still awaiting user input

## Hard rules

- Never call `closeAccessReviewCampaign`, `cancelAccessReviewCampaign`,
  `startAccessReviewCampaign`, `createAccessReviewCampaign`, or source/campaign
  admin mutations unless the user explicitly requests setup work outside this
  command.
- Never invent entry IDs, emails, or decisions — use MCP responses only.
- Never record a non-`APPROVED` decision without `decision_note`.
- Do not process `COMPLETED` or `CANCELLED` campaigns.
