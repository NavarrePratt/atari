# BD Issue Tracking Skill

Use the bd CLI for issue tracking workflow.

## Trigger

Use when:
- Starting work on an issue
- Updating issue status
- Creating new issues
- Closing completed work

## Workflow

1. Check available work: `bd ready --json`
2. Claim issue: `bd update <id> --status in_progress`
3. Work on implementation
4. Checkpoint progress: `bd update <id> --notes "..."`
5. Complete: `bd close <id> --reason "..."`
