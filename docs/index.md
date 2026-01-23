# Ribbin Documentation

Block direct tool calls and redirect to project-specific alternatives.

## Getting Started

New to Ribbin? Start here:

- [**Getting Started Tutorial**](tutorials/getting-started.md) - Install Ribbin and create your first configuration
- [**Create Your First Wrapper**](tutorials/first-wrapper.md) - Step-by-step guide to blocking a command

## How-To Guides

Solve specific problems:

### Configuration
- [Block Commands](how-to/block-commands.md) - Show error messages for direct tool calls
- [Redirect Commands](how-to/redirect-commands.md) - Execute wrapper scripts instead
- [Allow from Approved Scripts](how-to/passthrough-args.md) - Passthrough matching for parent processes
- [Configure Monorepo Scopes](how-to/monorepo-scopes.md) - Per-directory rules
- [Use Config Inheritance](how-to/config-inheritance.md) - Extend and reuse configurations
- [Create Local Overrides](how-to/local-overrides.md) - Personal config overrides

### Integration
- [Set Up for AI Agents](how-to/integrate-ai-agents.md) - Guide Claude, Copilot, and other assistants

### Operations
- [View Audit Logs](how-to/view-audit-logs.md) - Monitor blocked commands
- [Rotate Audit Logs](how-to/rotate-logs.md) - Manage log file size

## Reference

Look up specific details:

- [CLI Commands](reference/cli-commands.md) - All commands with flags and options
- [Configuration Schema](reference/config-schema.md) - Complete `ribbin.jsonc` format
- [Audit Log Format](reference/audit-log-format.md) - Event structure and types
- [Security Features](reference/security-features.md) - Protection mechanisms
- [Environment Variables](reference/environment-vars.md) - `RIBBIN_BYPASS` and others

## Explanation

Understand how and why:

- [Why Ribbin?](explanation/why-ribbin.md) - Comparison with AGENTS.md, hooks, and alternatives
- [How Ribbin Works](explanation/how-ribbin-works.md) - Architecture and execution flow
- [Security Model](explanation/security-model.md) - Threat model and design decisions
- [Local Development Mode](explanation/local-dev-mode.md) - Repository-scoped protection
- [Performance](explanation/performance.md) - Overhead analysis and benchmarks

## Quick Links

| Task | Guide |
|------|-------|
| Install and set up | [Getting Started](tutorials/getting-started.md) |
| Block a command | [Block Commands](how-to/block-commands.md) |
| Set up for AI agents | [AI Integration](how-to/integrate-ai-agents.md) |
| Configure a monorepo | [Monorepo Scopes](how-to/monorepo-scopes.md) |
| Check what's blocked | [View Audit Logs](how-to/view-audit-logs.md) |
| Understand security | [Security Model](explanation/security-model.md) |

## About This Documentation

This documentation follows the [Diátaxis](https://diataxis.fr/) framework:

- **Tutorials** - Learning-oriented, step-by-step guides
- **How-To Guides** - Goal-oriented, solve specific problems
- **Reference** - Information-oriented, technical details
- **Explanation** - Understanding-oriented, background and context
