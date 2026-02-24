# moshmux

Named tmux session manager with shell aliases and an interactive picker.

Define aliases like `alias myproject='mux myproject ~/workspace/myproject'`, source the file, and type `myproject` to create or reattach to the tmux session. Works locally by default, or over SSH/mosh to a remote host.

## Quick Start (Local)

```bash
# Install the CLI
go install github.com/plinde/moshmux/cmd/moshmux@latest

# Source the aliases file
mkdir -p ~/.zsh
cp moshmux.zsh ~/.zsh/
echo 'source ~/.zsh/moshmux.zsh' >> ~/.zshrc
source ~/.zsh/moshmux.zsh

# Add your first alias
moshmux add .              # uses cwd basename as name
moshmux add myproject ~/workspace/myproject
```

## Remote Usage

To use moshmux as a remote session manager (SSH/mosh to a server), set `MOSHMUX_HOST`:

```bash
export MOSHMUX_HOST=myserver.local
```

With `MOSHMUX_HOST` set, the `mux()` function connects via SSH instead of running tmux locally. Unset it to return to local mode.

## The `mux()` Function

The base function sourced from `moshmux.zsh`:

```bash
mux <session-name> <directory>
```

- Creates a new tmux session (or reattaches to an existing one) in the given directory.
- Each alias is just a thin wrapper: `alias foo='mux foo ~/workspace/foo'`

## CLI Commands

```
moshmux                       Interactive session picker (fzf)
moshmux <name>                Attach to named session (prefix match supported)
moshmux .                     Attach to session matching cwd
moshmux list                  Live-updating session status (TUI)
moshmux list --no-tui         Print session table and exit
moshmux add .                 Add cwd (name = directory basename)
moshmux add . <name>          Add cwd with custom alias name
moshmux add <name> <target>   Add alias (target = dir or existing alias name)
moshmux remove .              Remove alias matching cwd
moshmux remove <name>         Remove specific alias
moshmux update <name> <path>  Update path of existing alias (. = cwd)
moshmux join <name>           Join session with independent window focus
moshmux join .                Join session matching cwd
moshmux kill <name>           Kill named tmux session
moshmux termius               Print Termius startup script (fzf picker)
moshmux termius <name>        Print Termius startup script (direct attach)
moshmux upgrade               Detect and kill old tmux server
moshmux upgrade --force       Kill old server without confirmation
```

## Adding Aliases

```bash
# Add current directory
moshmux add .

# Add with explicit name and path
moshmux add myproject ~/workspace/myproject

# Link to existing alias (shares the same tmux session)
moshmux add shortname existingalias
```

To see all configured aliases, check `moshmux.zsh` or run `moshmux list`.

## Build from Source

```bash
git clone https://github.com/plinde/moshmux.git
cd moshmux
make build
```

Or with plain Go:

```bash
go build -o moshmux ./cmd/moshmux
```
