# Session Protocol

## Work on ONE issue at a time

1. Select highest-priority from `bd ready`
2. Implement ONLY that feature
3. Commit with `/commit`
4. Close: `bd close <id> --reason "..." --json`
5. Verify end-to-end
6. Move to next issue

## Quality Gates

Before committing:
- Code compiles/lints without errors
- All tests pass
- No hardcoded secrets
- Changes minimal and focused
