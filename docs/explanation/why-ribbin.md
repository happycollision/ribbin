# Why Ribbin?

You might wonder why ribbin exists when there are other ways to guide AI agents. The short answer: **ribbin delivers instructions exactly when the agent needs them, without cluttering every conversation with setup details it might never use.**

## The Problem with Upfront Instructions (AGENTS.md, CLAUDE.md)

These files load instructions at session start. If your project has a complex TypeScript setup, custom test runners, and specific deployment scripts, all of that context sits in every conversation—even when the agent is just renaming a variable.

Ribbin takes a different approach: instructions arrive at the moment of the mistake, as the error message itself.

## The Problem with AI-Specific Hooks (Claude Code hooks, AgentGuard)

Claude Code's hook system and tools like AgentGuard can intercept commands—but only for their specific AI tool. Your carefully crafted guardrails won't help when a teammate uses Cursor, Cline, or Windsurf.

Ribbin works at the shell level. Any AI agent (or human) that runs a command hits the same wrapper.

## The Problem with Git Hooks (Lefthook, Husky)

Git hooks like Lefthook and Husky only run at commit time. By then, the agent has already made the mistake, run the wrong tests, or used the wrong build command. You want to catch the error when it happens, not after a pile of changes are ready to commit.

Ribbin intercepts commands in real-time, before they execute.

## The Problem with Shell Config (direnv, shell aliases)

You might think "I'll just use direnv and shell aliases." But AI agents spawn fresh shells:

```bash
# Your .bashrc/.zshrc never runs in agent shells
eval "$(direnv hook bash)"  # Never executes
alias npm="echo 'use pnpm'"  # Never defined
```

Ribbin's PATH shims are baked into the filesystem—they work regardless of how the shell was spawned.

## When to Use What

| Approach | Best For |
|----------|----------|
| `AGENTS.md` / `CLAUDE.md` | Always-relevant project context (architecture, coding standards) |
| Claude Code hooks / AgentGuard | Claude Code-specific workflows, security policies |
| Lefthook / Husky | Enforcing standards at commit time |
| **ribbin** | Tool-specific instructions that only matter when the agent reaches for the wrong tool |

Ribbin complements these approaches rather than replacing them. Use `AGENTS.md` for "here's how this codebase is structured" and ribbin for "if you try to run `tsc` directly, here's why that won't work and what to do instead."

## See Also

- [How Ribbin Works](how-ribbin-works.md) - Architecture and execution flow
- [Security Model](security-model.md) - Threat model and design decisions
