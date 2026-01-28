# User Interview Planning

Plan work by interviewing the user in-depth about their plans using probing, non-obvious questions.

## Context Sources

This command receives context from two sources:
1. **Conversation history** - All messages above inform requirements, decisions, and scope
2. **Arguments** - Additional instructions passed when invoking the command (see $ARGUMENTS at end)

## Tools Used

- **AskUserQuestion** - Primary tool for in-depth user interviews
- **Task (Explore subagent)** - Codebase exploration to inform questions
- **br CLI** - Issue creation after planning is complete

---

## Overview

This is an interactive planning process where you interview the user to fully flesh out their plan before creating beads. Unlike issue-plan and issue-plan-codex which use AI-to-AI debate, this command uses direct user dialogue to refine requirements.

## Step 1: Understand the Plan

Review the entire conversation to understand what plan is being discussed. Identify:
- The core goal or feature being planned
- Any constraints or requirements already mentioned
- Technical context from prior discussion
- Open questions or unclear areas

## Step 2: Initial Exploration (Optional)

If the plan involves code changes and you need context to ask better questions, run a **quick** Explore query (model: "haiku") to understand:
- Relevant existing code and patterns
- How similar features are implemented
- Potential integration points

Skip this step if the conversation already provides sufficient technical context.

## Step 3: Interview the User

Interview the user about this plan in detail using the AskUserQuestion tool. Probe across multiple dimensions:

**Technical implementation:**
- How should this integrate with existing code?
- What patterns or conventions should it follow?
- Are there performance considerations?

**UI and UX (if applicable):**
- How should users interact with this?
- What feedback should the system provide?
- What happens in error states?

**Scope and boundaries:**
- What is explicitly out of scope?
- Are there phases or increments to consider?
- What is the minimum viable implementation?

**Edge cases and assumptions:**
- What inputs or states could cause problems?
- What assumptions are we making about the environment?
- How should the system behave in unexpected situations?

**Risks and dependencies:**
- What could go wrong?
- What does this depend on?
- What other work might be affected?

**Testing and verification:**
- How will we know this works correctly?
- What should be tested manually vs automatically?
- Are there integration concerns?

### Interview Guidelines

- **Ask non-obvious questions** - Probe deeper into things the user might not have considered
- **Challenge assumptions** - Question unstated beliefs about how things should work
- **Ask about the hard parts** - Focus on areas that seem complex or risky
- **Use multiple rounds** - Continue interviewing until the plan is fully fleshed out
- **Be very in-depth** - This is not a superficial checklist; dig into specifics

### Using AskUserQuestion Effectively

Structure questions with clear options when possible:
```
questions:
  - question: "How should errors be surfaced to the user?"
    header: "Error UX"
    options:
      - label: "Toast notification"
        description: "Non-blocking notification that auto-dismisses"
      - label: "Modal dialog"
        description: "Blocking dialog requiring user acknowledgment"
      - label: "Inline error"
        description: "Error displayed next to the relevant input"
      - label: "Status bar"
        description: "Persistent status area showing current state"
    multiSelect: false
```

For open-ended questions, provide example answers as options with "Other" for custom input.

## Step 4: Synthesize the Plan

After the interview is complete, synthesize everything discussed into a comprehensive plan:

1. **Summary** - What we are building and why
2. **Scope** - What is included and excluded
3. **Technical approach** - How it will be implemented
4. **Key decisions** - Important choices made during the interview
5. **Edge cases addressed** - How we handle unusual situations
6. **Testing strategy** - How we verify correctness
7. **Risks and mitigations** - What could go wrong and how we prevent it

Present this synthesis to the user and confirm it accurately captures the plan.

## Step 5: Discover Verification Commands

{{ SHARED_VERIFICATION_DISCOVERY }}

## Step 6: Create Beads

{{ SHARED_BEAD_CREATION }}

## Step 7: Output Summary

After creating and publishing beads, output a clear summary:

```
Created X bead(s) from user interview:

- bd-xxx: [title] (P2, open)
- bd-xxx: [title] (P2, open, blocked by bd-xxx)
- bd-xxx: [title] - final verification (P2, open, blocked by all above)

Epic: bd-xxx - [epic title]

Key decisions from interview:
- [Decision 1]
- [Decision 2]

Ready for implementation.
```

---

## Additional Instructions

$ARGUMENTS
