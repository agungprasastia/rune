# Rune

**A fast, native, lightweight and extensible AI coding agent for your terminal.**

Rune is a Go-native terminal coding agent. It provides a TUI, headless execution,
multi-provider support, tools, sessions, MCP, permissions, sandboxing, plugins,
skills, hooks, specialists, and Git/worktree integration.

## Install From Source

```bash
git clone https://github.com/agungprasastia/rune.git
cd rune
go build -trimpath -o rune ./cmd/rune
./rune
```

Development run:

```bash
go run ./cmd/rune
```

Core Rune does not require Node.js or npm.

## Usage

```bash
rune
rune --help
rune --version
rune exec "fix the failing test in ./pkg"
rune doctor
```

Headless execution uses the same Go agent runtime as the TUI. Provider setup and
credentials are configured through Rune's existing CLI and configuration system.

## Quality Checks

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

## Architecture

Rune keeps its core layers in Go:

```text
CLI / TUI
    |
Agent runtime
    |
Providers, tools, sessions, context, MCP, permissions, sandbox
```

TUI rendering remains separate from agent execution. Native helpers are used only
where the existing platform integration requires them. Rust is not required.

## Project

- Repository: https://github.com/agungprasastia/rune
- License: MIT
- Upstream attribution: see `LICENSE`

Rune is early production foundation work. APIs, storage details, and release
packaging may change before the first stable release.
