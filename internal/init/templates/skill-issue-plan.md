---
name: issue-plan
description: Plan issues using AI debate between Claude (haiku) and Codex (mini). Best for standard multi-step work requiring discovery and collaborative refinement.
---

# Planning Issues

Plan and create issues for multi-step work requiring discovery and collaborative debate.

## Context Sources

This command receives context from two sources:
1. **Conversation history** - All messages above inform requirements, decisions, and scope
2. **Arguments** - Additional instructions passed when invoking the command (see $ARGUMENTS at end)

## Tools Used

- **Task (Explore subagent)** - Codebase exploration with model: "haiku"
- **Task (Plan subagent)** - Implementation design with model: "haiku"
- **mcp__codex__codex** - Cross-reference discovery with model: "gpt-5.1-codex-mini"
- **br CLI** - Issue creation, status management, and dependencies

---

## Overview

This is a two-phase process: discovery first, then planning with collaborative debate.

## Phase 1: Discovery

Use BOTH approaches for comprehensive discovery:

### Claude Explore Agents
Use the Explore subagent with "very thorough" setting and **model: "haiku"** to understand:
1. All code related to this work (run up to 3 parallel explorations)
2. Current architecture, patterns, and conventions

{{ SHARED_VERIFICATION_DISCOVERY }}

### Codex Discovery
Use the codex MCP tool for additional discovery:
```
mcp__codex__codex with model: "gpt-5.1-codex-mini"
prompt: "Explore [topic]. Find all relevant code, patterns, edge cases, and potential issues. Report findings comprehensively."
```
Cross-reference Codex findings with Explore results to ensure nothing is missed.

## Phase 1.5: Discovery Synthesis

Before planning, consolidate findings into a brief summary:
- **Architecture overview**: Key patterns, conventions, and constraints discovered
- **Testing setup**: Where tests live, how to run them, what coverage exists
- **Verification commands**: Exact commands for lint, static analysis, test, e2e (from discovery)
- **Known risks**: Edge cases, gotchas, or blockers identified during discovery

This summary becomes the input for Phase 2.

## Phase 2: Planning with Collaborative Debate

Use multi-round refinement for thorough planning:

### Step 1: Initial Plan
Use the Plan subagent with **model: "haiku"** to design implementation approach based on discovery synthesis.

### Step 2: Collaborative Debate (2-4 rounds, until consensus or escalation)
Claude (Haiku) and Codex (gpt-5.1-codex-mini) debate back-and-forth to refine the plan:

**Round 1 - Dual Critique**:
- **Claude (Haiku)**: List 5-10 specific gaps, risks, or edge cases in the plan. For each, explain why it matters.
- **Codex**: Use `mcp__codex__codex` with model "gpt-5.1-codex-mini":
  ```
  prompt: "Review this implementation plan: [plan]. List 5-10 specific gaps, conflicts, or risks. For each issue: (1) What could break? (2) What assumption might be wrong? (3) Suggest a concrete mitigation."
  ```
- Synthesize both critiques. If >3 critical issues overlap, they are high-priority fixes.

**Round 2 - Address & Counter**:
- **Claude (Haiku)**: Propose specific revisions for each Round 1 concern. State which you accept, reject (with rationale), or defer.
- **Codex**: Use `mcp__codex__codex` with model "gpt-5.1-codex-mini":
  ```
  prompt: "Claude proposes these revisions: [revisions]. For each: (1) Does it actually solve the concern? (2) What breaks if Claude's assumption is wrong? (3) Suggest 1-2 concrete alternatives for weak points."
  ```
- Integrate valid counterpoints. If fundamental disagreement on architecture, pause and re-examine discovery findings.

**Round 3 - Final Consensus** (skip if Round 2 achieved consensus):
- **Claude (Haiku)**: Present refined plan with all incorporated feedback. List any unresolved disagreements.
- **Codex**: Use `mcp__codex__codex` with model "gpt-5.1-codex-mini":
  ```
  prompt: "Final plan review: [plan]. Verify: (1) All discovered edge cases addressed or explicitly deferred? (2) Error/failure paths defined? (3) Testing strategy clear? (4) Dependencies sequenced correctly? List any gaps."
  ```
- If consensus: Proceed. If disagreement on implementation detail: Choose simpler/safer option, note as future optimization.

### Quality Gate
Before creating issues, confirm:
- [ ] All discovered edge cases addressed or explicitly deferred with rationale
- [ ] Error paths defined (what happens when X fails?)
- [ ] Testing strategy covers new code
- [ ] Trade-offs documented with reasoning

{{ SHARED_BEAD_CREATION }}

## Handling Failures

When discovery or planning reveals blocking issues:
1. Create a P0 meta issue titled: "Create plan for [blocker-topic]"
2. Description must include:
   - What was blocking and why it matters
   - Instruction to use Explore subagent for discovery
   - Instruction to use Plan subagent to design fix
   - Instruction to create implementation issues via issue-tracking skill
3. Any implementation issues spawned from meta issues are also P0

---

## Additional Instructions

$ARGUMENTS
