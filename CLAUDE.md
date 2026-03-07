# CLAUDE.md

## Project Overview

Named tmux session manager with TOML-based alias config and an interactive fzf-based session picker. Works locally by default; set `MOSHMUX_HOST` for remote SSH/mosh usage.

## Key Files

- `cmd/moshmux/main.go` — CLI binary for managing aliases and launching sessions via fzf.
- `parser.go` — Library for parsing/editing alias entries (TOML format, with legacy zsh parser for migration).

## Config Layout

```
~/.config/moshmux/
  config.toml       — settings (aliases_dir, git_sync)
  aliases.toml      — default aliases location
```

### config.toml

```toml
# Optional: where aliases.toml lives. Default = same dir as config.toml.
# aliases_dir = "/home/user/workspace/moshmux-config"

# Auto git add/commit/push on alias changes. Default = false.
# git_sync = false
```

### aliases.toml

```toml
[mc]
session = "minecraft"
dir = "~/workspace/minecraft"

[moshmux]
dir = "~/workspace/moshmux"

# When session is omitted, defaults to the alias (section) name
```

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
moshmux config                Show current config
moshmux config set-aliases-dir  Set aliases directory
moshmux config set-git-sync   Enable/disable git sync (on|off)
moshmux suggest               Suggest aliases from zoxide/atuin/history
moshmux suggest --source X    Only use specific source (zoxide|atuin|history|aliases)
moshmux suggest --count N     Show top N suggestions (default: 10)
moshmux migrate [path]        Migrate moshmux.zsh to aliases.toml
moshmux upgrade               Detect and kill old tmux server
```

## Adding Aliases

Use the CLI:

```bash
moshmux add myproject ~/workspace/myproject
moshmux add .   # uses cwd basename as name
```

Or manually edit `~/.config/moshmux/aliases.toml`:

```toml
[myproject]
dir = "~/workspace/myproject"
```

## Migration from moshmux.zsh

```bash
moshmux migrate                          # auto-detect moshmux.zsh
moshmux migrate ~/workspace/moshmux/moshmux.zsh  # explicit path
```

## Build

```bash
go build -o moshmux ./cmd/moshmux
```

## Notes

- Aliases dir resolution: `MOSHMUX_DIR` env > `aliases_dir` in config.toml > `~/.config/moshmux/`
- Git sync is opt-in: set `git_sync = true` in config.toml and ensure aliases dir is a git repo
- Local mode (default): `mux()` runs tmux directly
- Remote mode: `export MOSHMUX_HOST=server` makes `mux()` use SSH/mosh
