---
name: issue-plan-hybrid
description: Plan issues by combining user interview with Codex technical review. Iterates until consensus. Best for important work requiring both user input and AI validation.
---

# Hybrid Planning (User Interview + Codex Review)

Plan work by combining user interview with Codex technical review. Adapts depth based on complexity.

## Context Sources

This command receives context from two sources:
1. **Conversation history** - All messages above inform requirements, decisions, and scope
2. **Arguments** - Additional instructions passed when invoking the command (see $ARGUMENTS at end)

## Tools Used

- **AskUserQuestion** - User interview and decision points
- **Task (Explore subagent, haiku)** - Optional codebase exploration
- **mcp__codex__codex (gpt-5.2-codex)** - Plan review for technical gaps
- **br CLI** - Bead creation after planning is complete

---

## Overview

This is an iterative planning process that combines:
1. **User interview** - Gather requirements directly from the user
2. **Codex review** - Technical critique focused on gaps, not feasibility
3. **User decisions** - User resolves conflicts and approves changes

The process adapts to task complexity: simple tasks get streamlined interviews, complex tasks get thorough exploration across all dimensions.

## Step 1: Understand Context

Review the entire conversation to understand what plan is being discussed. Identify:
- The core goal or feature being planned
- Any constraints or requirements already mentioned
- Technical context from prior discussion
- Open questions or unclear areas
- Questions that have ALREADY been answered (do not re-ask these)

## Step 2: Judge Complexity

Assess whether this task is **simple** or **complex**:

**Simple tasks** (streamlined interview):
- Single-file or few-file changes
- Configuration tweaks
- Clear, well-defined requirements
- Following existing patterns
- No external dependencies

**Complex tasks** (thorough interview):
- Data migrations or schema changes
- New infrastructure or services
- External integrations (APIs, third-party services)
- Ambiguous scope or requirements
- Cross-cutting changes affecting multiple systems
- Security-sensitive changes
- Performance-critical paths

State your complexity assessment before proceeding to the interview.

## Step 3: Initial Exploration (Optional)

If the plan involves code changes and you need context to ask better questions, run a **quick** Explore query (model: "haiku") to understand:
- Relevant existing code and patterns
- How similar features are implemented
- Potential integration points

Skip this step if the conversation already provides sufficient technical context.

## Step 4: Interview the User

Interview the user about this plan using the AskUserQuestion tool. Adapt depth based on complexity:

### For Simple Tasks

Focus only on dimensions where the conversation has gaps:
- Skip questions already answered in conversation
- Ask 1-3 targeted questions on unclear points
- Proceed quickly to synthesis

### For Complex Tasks

Probe across all dimensions systematically:

**Technical implementation:**
- How should this integrate with existing code?
- What patterns or conventions should it follow?
- Are there performance considerations?

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

- **Skip answered questions** - Do not re-ask what the conversation already clarifies
- **Ask non-obvious questions** - Probe deeper into things the user might not have considered
- **Challenge assumptions** - Question unstated beliefs about how things should work
- **Use multiple rounds if needed** - Continue until unclear points are resolved

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
    multiSelect: false
```

For open-ended questions, provide example answers as options with "Other" for custom input.

## Step 5: Synthesize the Plan

After the interview is complete, synthesize everything into a comprehensive plan document:

1. **Summary** - What we are building and why
2. **Scope** - What is included and excluded
3. **Technical approach** - How it will be implemented
4. **Key decisions** - Important choices made during the interview
5. **Edge cases addressed** - How we handle unusual situations
6. **Testing strategy** - How we verify correctness
7. **Risks and mitigations** - What could go wrong and how we prevent it

Present this synthesis to the user briefly before sending to Codex for review.

## Step 6: Codex Review

Send the synthesized plan to Codex for technical review. The review focuses on **technical gaps**, NOT feasibility.

Use `mcp__codex__codex` with model "gpt-5.2-codex":
```
prompt: "Review this implementation plan for technical gaps:

[Insert synthesized plan]

Focus your review on:
1. Technical gaps - What's missing that implementers will need?
2. Edge cases - What unusual inputs or states aren't handled?
3. Missing dependencies - What libraries, services, or code doesn't exist yet?
4. Error handling - What failure modes aren't addressed?
5. Testing strategy - What test coverage is missing?

For EACH concern you identify, provide:
- What specifically is the gap or risk?
- Why does it matter for implementation?
- A concrete mitigation or addition to the plan

Do NOT review feasibility or whether this should be done. Assume the user has decided to proceed. Focus only on making the plan technically complete."
```

## Step 7: Present Feedback to User

Summarize the Codex feedback for the user:

1. **List each concern** with a brief explanation
2. **Propose specific changes** to address each concern
3. **Identify decision points** where user input is needed
4. **Note any conflicts** between Codex suggestions and user requirements

If there are conflicts between Codex suggestions and user decisions:
- **User wins** - The user's explicit decision takes precedence
- **Document the trade-off** - Note what was suggested and why user chose differently

Ask the user to confirm the proposed changes or provide alternative direction.

## Step 8: Iterate

Repeat Steps 6-7 until EITHER:
- **User signals ready** - Natural language like "ready", "looks good", "create beads", "let's proceed"
- **Codex has no new concerns** - Review returns with no additional gaps to address

Typically this takes 1-2 rounds. Do not over-iterate on minor issues.

### Stop Signal Detection

Watch for these user signals that the plan is ready:
- "ready" / "looks ready"
- "looks good" / "this is good"
- "create beads" / "create issues" / "create the beads"
- "let's proceed" / "proceed"
- "go ahead" / "ship it"
- Explicit approval of the final plan summary

When detected, proceed immediately to verification discovery and bead creation.

## Step 9: Discover Verification Commands

{{ SHARED_VERIFICATION_DISCOVERY }}

## Step 10: Create Beads

{{ SHARED_BEAD_CREATION }}

## Step 11: Output Summary

After creating and publishing beads, output a clear summary:

```
Created X bead(s) from hybrid planning:

- bd-xxx: [title] (P2, open)
- bd-xxx: [title] (P2, open, blocked by bd-xxx)
- bd-xxx: [title] - final verification (P2, open, blocked by all above)

Epic: bd-xxx - [epic title]

Key decisions from interview:
- [Decision 1]
- [Decision 2]

Codex review addressed:
- [Gap 1] - [how resolved]
- [Gap 2] - [how resolved]

Trade-offs documented:
- [Trade-off 1] - User chose X over Codex suggestion Y because Z

Ready for implementation.
```

---

## Additional Instructions

$ARGUMENTS
