# moshmux

Named tmux session manager with an interactive fzf picker.

```bash
go install github.com/plinde/moshmux/cmd/moshmux@latest

cd ~/workspace/myproject
moshmux add .       # register current directory
moshmux myproject   # create or reattach to tmux session
```

Run `moshmux` with no arguments for an interactive session picker.

## Commands

```
moshmux                       Interactive session picker (fzf)
moshmux <name>                Attach to named session (prefix match supported)
moshmux .                     Attach to session matching cwd
moshmux list                  Live-updating session status (TUI)
moshmux list --no-tui         Print session table and exit
moshmux add .                 Add cwd (name = directory basename)
moshmux add <name> <dir>      Add alias with explicit name and path
moshmux remove <name>         Remove alias
moshmux update <name> <path>  Update path of existing alias
moshmux join <name>           Join session with independent window focus
moshmux kill <name>           Kill named tmux session
moshmux config                Show current config
moshmux migrate [path]        Migrate from legacy moshmux.zsh format
moshmux --version             Show version
```

## Config

Aliases are stored in TOML at `~/.config/moshmux/aliases.toml`:

```toml
[myproject]
dir = "~/workspace/myproject"

[mc]
session = "minecraft"
dir = "~/workspace/minecraft"
```

When `session` is omitted, it defaults to the alias name.

### Syncing across machines

Point aliases at a separate git repo and enable auto-push:

```bash
moshmux config set-aliases-dir ~/workspace/moshmux-config
moshmux config set-git-sync on
```

Now `moshmux add/remove/update` will auto-commit and push to that repo.

## Build from Source

```bash
git clone https://github.com/plinde/moshmux.git
cd moshmux
go build -o moshmux ./cmd/moshmux
```
