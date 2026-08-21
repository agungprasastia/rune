# Rune Specialists

Specialists are named sub-agents that Rune can delegate focused work to through
the `Task` tool. A specialist is a markdown manifest with YAML-style
frontmatter plus a system prompt body.

Specialists can be built in, user-scoped, or project-scoped:

| Scope | Path | Notes |
| --- | --- | --- |
| Built-in | compiled into Rune | `worker`, `explorer`, and `code-review` ship with the binary. |
| User | `~/.config/rune/specialists/*.md` | Available across local workspaces. |
| Project | `.rune/specialists/*.md` | Shared with the current repository when committed. |

Project specialists override user and built-in specialists with the same name.
User specialists override built-ins.

## CLI Management

```bash
rune specialist list
rune specialist show worker
rune specialist path

rune specialist create api-review \
  --project \
  --description "Reviews API changes" \
  --tools read-only,plan \
  --prompt "Review API changes for compatibility and missing tests."

rune specialist edit api-review --project
rune specialist delete api-review --project
```

Use `--json` with `list`, `show`, `path`, `create`, or `delete` when scripting.
`create --force` replaces an existing manifest, but refuses symlink overwrites.
`edit` also refuses symlink manifests before opening `$VISUAL` or `$EDITOR`.

## Manifest Format

```markdown
---
name: api-review
description: Reviews API changes for compatibility and missing tests.
tools:
  - read-only
  - plan
---

Review API changes for behavior regressions, compatibility breaks, and missing
tests. Report concrete findings with file paths.
```

Supported frontmatter keys:

| Key | Purpose |
| --- | --- |
| `name` | Lowercase specialist id. Use letters, numbers, and dashes. |
| `description` | Short summary shown in listings and task metadata. |
| `extends` | Optional base specialist to inherit prompt/model/tools from. |
| `model` | Optional model override. Empty means inherit the parent model. |
| `reasoningEffort` | Optional reasoning effort override. |
| `tools` | Array of tool categories or tool ids. |

If the body is empty and `description` is set, Rune uses the description as the
system prompt and reports a warning in `rune specialist show`.

## Tool Selection

Known categories:

| Category | Tools |
| --- | --- |
| `read-only` | `read_file`, `list_directory`, `grep`, `glob` |
| `edit` | read-only tools plus `write_file`, `edit_file`, `apply_patch` |
| `execute` | read-only tools plus `bash` |
| `plan` | `update_plan` |

Specialist manifests cannot enable `Task`, `TaskOutput`, `TaskStop`, or
`GenerateSpecialist`, so child specialists cannot spawn more specialists or
author new ones.

## Agent Tools

Rune registers these tools for top-level agent runs:

| Tool | Purpose |
| --- | --- |
| `Task` | Launch a specialist sub-agent for a focused prompt. |
| `TaskOutput` | Read or block on a background specialist task's output. |
| `TaskStop` | Stop a running background specialist task. |
| `GenerateSpecialist` | Create a project-local specialist manifest from a description. |

`GenerateSpecialist` is project-scoped only. It writes to
`.rune/specialists`, not the user specialist directory.

Example LLM-facing `Task` payload:

```json
{
  "name": "explorer",
  "description": "Find session storage code",
  "prompt": "Find the files that create, load, and list sessions."
}
```

Background task payload:

```json
{
  "name": "worker",
  "description": "Audit release docs",
  "prompt": "Check the release docs for stale TypeScript references.",
  "run_in_background": true
}
```

The returned `task_id` is also the child session id. Use it with
`TaskOutput`, `TaskStop`, or `Task` resume.

## Background State

Background specialist output is stored under:

```text
${XDG_DATA_HOME:-~/.local/share}/rune/background/
```

Each task has:

- `<task_id>.ndjson` for the child process stream output
- `<task_id>.json` for task metadata such as status, PID, parent session, and
  timestamps

Persisted metadata lets a new background manager instance read completed task
output or stop a still-running task by id.

If Rune is restarted while a background task is still marked `running`, the new
manager marks that task `error` and clears its PID. This avoids sending
`TaskStop` to a stale PID that may now belong to an unrelated process.

## Recovering an Interrupted Overwrite

Rune writes and flushes a complete temporary file before publishing an overwrite,
so a write failure before publication leaves the existing manifest unchanged
instead of truncating it. On Unix, publication uses a same-directory rename and
preserves the existing file's permission bits.

On Windows, Rune uses `ReplaceFileW` to preserve the destination DACL instead of
silently replacing it with the temporary file's inherited DACL. `ReplaceFileW`
is not observer-atomic: another process can briefly observe the destination path
as absent during replacement. Rune serializes specialist loads and managed
mutations within one process, but cannot synchronize external processes or
editors.

Windows errors 1176 (`ERROR_UNABLE_TO_MOVE_REPLACEMENT`) and 1177
(`ERROR_UNABLE_TO_MOVE_REPLACEMENT_2`) are partial replacement failures. With
Rune's managed backup, 1176 leaves the original names intact and needs no manual
recovery. For 1177, Rune has moved the original aside and tries to move it back.
That rollback almost always succeeds, and the failed write changes nothing.

If the rollback itself fails — typically because another process is holding a
lock on the file — the original is not lost, but it is left under a name Rune
does not read:

```text
<specialist dir>/.rune-replace-<random>.backup
```

Only `*.md` files are loaded as specialists, so until that file is renamed the
specialist will not appear in `rune specialist list` or resolve by name. Rune's
error message includes both the backup path and destination path. Recover by
closing whatever holds the lock and renaming the backup back:

```powershell
Move-Item .rune-replace-<random>.backup <name>.md
```

A `.rune-replace-*.backup` can also linger after a *successful* overwrite if the
backup could not be deleted afterward. That case is reported as a warning rather
than an error — the new manifest is already in place, and the leftover file is
safe to delete.
