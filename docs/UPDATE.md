# Update Flow

`rune update --check` checks the latest GitHub release and compares it with the
local CLI version. `rune update --apply` (or its shorthand, `rune upgrade`)
downloads, verifies, and installs it.

```bash
rune update --check
rune update --check --json
rune update --check --repo agungprasastia/rune
rune update --check --target windows-x64

rune upgrade
rune update --apply
```

`--check` and `--apply` are mutually exclusive. `rune update` requires one of
them explicitly; `rune upgrade` is `rune update` with `--apply` implied.

`--check` is check-only:

- It does not replace the running binary.
- It exits with code `0` when the check succeeds, even when an update is
  available.
- It exits with code `1` when the release check cannot be completed.
- `--json` prints the same result in a machine-readable format for scripts and
  CI.

`--apply` installs the update in place:

- Native Rune builds use GitHub Releases for self-update.
- Standalone installs download the release archive, verify its checksum,
  extract it, and atomically replace the running binary plus any installed
  optional sandbox helpers.
- On Windows, the running executable is renamed aside and cleaned up on the
  next `rune update --apply` or `rune upgrade`, since it can't be overwritten
  while running.
- `--target` cannot be combined with `--apply`; it only applies to `--check`,
  since applying always installs onto the current machine.
- `--repo` and `--endpoint` select the GitHub Release source used by native
  update checks and applies. They apply to
  `--check` there.
- `--json` serializes Rune's final result. Native installs do not require npm.
  also write progress output to stdout, so neither
  `rune update --apply --json` nor `rune upgrade --json` is guaranteed to
  produce a single parseable JSON document. Use `--check --json` for
  machine-readable automation.

Useful flags:

| Flag | Purpose |
|---|---|
| `--repo <owner/repo>` | Use another GitHub repository for `--check` and `--apply`/`upgrade`. |
| `--endpoint <url\|owner/repo>` | Use a specific release API URL or repository slug for `--check` and `--apply`/`upgrade`. |
| `--timeout <duration>` | Override the default release check timeout. |
| `--target <platform-arch>` | Validate release metadata for another supported target (`--check` only). |

Supported targets are `linux-x64`, `linux-arm64`, `macos-x64`, `macos-arm64`,
`windows-x64`, and `windows-arm64`. Without `--target`, Rune checks the current
platform.

Endpoint resolution order:

1. `--endpoint`
2. `RUNE_UPDATE_RELEASE_URL`
3. `--repo`
4. `https://api.github.com/repos/agungprasastia/rune/releases/latest`

Installer scripts download the matching release asset for the local platform and
verify its `.sha256` file. If Rune is already installed, run `rune upgrade`
instead of reinstalling.

## Windows recovery state (standalone installs)

This section describes Windows only. On Linux and macOS a standalone update
renames the staged file directly over the executable path through the
installation directory's file descriptor, so the replacement is atomic and no
aside copy, marker, or recovery record is ever created — a failed update leaves
the previous binary in place and nothing to resolve.

On Windows, a running executable cannot be replaced in place, so the previous
binary is moved aside to `<binary>.rune-update-<random>.old` first. The updater
records that exact file — bound to its filesystem identity, in per-user state
outside the installation directory — and deletes only that recorded copy after
the next update is verified in place. Backups it did not create are never
removed, so a file such as `rune.exe.before-manual-patch.old` is left alone. A
recorded copy that something else holds open (an on-access scanner, an editor)
stays recorded and is removed by a later update instead.

An update **refuses to run** while unresolved recovery state exists beside the
binary:

| State on disk | Meaning |
|---|---|
| `<binary>.old` (or `<binary>.<suffix>.old`) plus a `.keep` marker | A previous update could not restore the original binary. The `.old` file may be the last binary the updater verified; the installed one may be unverified. |
| `<binary>.…old.<suffix>.recovery` | A previous update could not even write the marker, so it moved the last verified binary to that name. |
| The binary is missing and one or more `*.old` files exist | The previous attempt was interrupted between moves. |

The refusal names the paths involved and the two moves that end the state:
either move the recovery binary back over the executable path, or — if the
installed binary is the one you want — delete the `.keep` marker (or the
`.recovery` copy, once you have verified the installed binary) and update again.

This is deliberately fail-closed and differs from older releases, which deleted
`<binary>.old` and proceeded. The trade-off is that anyone who can write in the
installation directory can plant `<binary>.old` and `<binary>.old.keep` there
and make every subsequent update refuse until an operator clears them. That is
the safe direction: the alternative is an update that overwrites the only
verified copy of the previous binary. If updates start refusing, inspect those
files before removing them — an installation directory that a lower-privileged
account can write to is itself worth fixing.
