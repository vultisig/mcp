# Adding Skills & Tool Descriptions

## Architecture

The agent system uses a three-tier prompt architecture:

1. **Core system prompt** (~2,000 tokens, always loaded) — identity, safety rules, general guidelines
2. **Skill files** (loaded on-demand via `get_skill`) — detailed domain instructions
3. **MCP tool descriptions** (per-tool, always visible) — short operational hints

## When to use each tier

| Content type | Where it goes | Example |
|---|---|---|
| Safety rules, general behavior | prompt.go `systemPromptAfterActions` | "NEVER call financial tool without amount" |
| Domain workflow (multi-step) | Skill file in `internal/skills/files/` | Polymarket order flow, swap confirmation |
| Tool-specific hint | Tool description in `mcp.WithDescription()` | "Load 'swap-trading' skill for pre-checks" |

## Adding a new skill file

1. Create `internal/skills/files/your-skill-name.md`
2. Add YAML frontmatter:
   ```yaml
   ---
   name: Your Skill Name
   description: One-line description for the skill index
   tags: [relevant, tags]
   ---
   ```
3. Write the skill content in markdown. Include:
   - Overview (what this skill covers)
   - Step-by-step flows
   - Contract addresses / constants
   - Error handling
   - DO NOTs section
4. The skill auto-registers via `embed.FS` — no code changes needed
5. Reference it in tool descriptions: `"Load the 'your-skill-name' skill for..."`
6. Add it to the domain index in `prompt.go` `systemPromptAfterActions`

## Adding a new tool description

Tool descriptions go in the `newXxxTool()` function in `internal/tools/`:

```go
mcp.WithDescription(
    "What this tool does in 1-2 sentences. "+
        "What it returns. "+
        "Load the 'relevant-skill' skill for workflow details.",
),
```

Keep descriptions to 2-4 sentences. The skill file has the detailed instructions.

## Rules

- Core system prompt MUST stay under 2,000 tokens
- Skill files have no size limit (loaded on demand)
- Tool descriptions: 2-4 sentences max
- Safety-critical rules stay in prompt.go (never in skills — skills can be skipped)
- Every new domain needs: skill file + tool description hints + domain index entry
- Test: restart MCP, verify skill appears in `resources/list`
