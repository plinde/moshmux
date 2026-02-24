# CLAUDE.md

## Project Overview

Named tmux session manager with shell aliases and an interactive fzf-based session picker. Works locally by default; set `MOSHMUX_HOST` for remote SSH/mosh usage.

## Key Files

- `moshmux.zsh` — Shell aliases file. Contains the `mux()` function and all aliases. Sourced from `~/.zshrc`.
- `cmd/moshmux/main.go` — CLI binary for managing aliases and launching sessions via fzf.
- `parser.go` — Library for parsing/editing `moshmux.zsh` alias entries.

## Usage

```
moshmux                       Interactive session picker (fzf)
moshmux .                     Attach to session matching cwd
moshmux list                  Live-updating session status (TUI)
moshmux list --no-tui         Print session table and exit
moshmux add .                 Add cwd (name = directory basename)
moshmux add <name> <dir>      Add a named alias
moshmux remove .              Remove alias matching cwd basename
moshmux remove <name>         Remove <name> alias
moshmux update <name> <path>  Update path of existing alias
moshmux join <name>           Join session with independent window focus
moshmux join .                Join session matching cwd
moshmux kill <name>           Kill named tmux session
moshmux termius               Print Termius startup script
moshmux upgrade               Detect and kill old tmux server
```

## Adding Aliases

Use the CLI:

```bash
moshmux add myproject ~/workspace/myproject
moshmux add .   # uses cwd basename as name
```

Or manually add to `moshmux.zsh`:

```bash
alias name='mux name ~/workspace/project-dir'
```

## Build

```bash
go build -o moshmux ./cmd/moshmux
```

## Notes

- `moshmux.zsh` should be POSIX/zsh compatible (sourced on various platforms)
- Local mode (default): `mux()` runs tmux directly
- Remote mode: `export MOSHMUX_HOST=server` makes `mux()` use SSH/mosh
