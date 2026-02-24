# moshmux: Named tmux session manager with shell aliases
# Source this file from your ~/.zshrc
#
# Usage: mux <session-name> <directory>
#
# Local mode (default): creates/attaches tmux sessions directly.
# Remote mode: set MOSHMUX_HOST to SSH/mosh to a server.
#
#   export MOSHMUX_HOST=myserver.local   # optional — for remote sessions

mux() {
  if [ $# -eq 0 ]; then
    if [ -n "$MOSHMUX_HOST" ]; then
      mosh "$MOSHMUX_HOST"
    else
      tmux
    fi
    return
  fi
  local name="$1" dir="$2"
  shift 2

  if [ -n "$MOSHMUX_HOST" ]; then
    # Pre-flight check: fix stale root-owned tmux socket directory
    ssh "$MOSHMUX_HOST" 'test -d /tmp/tmux-$(id -u) && ! test -w /tmp/tmux-$(id -u)/default 2>/dev/null && sudo mv /tmp/tmux-$(id -u) ~/trash/tmux-$(id -u).$(date +%s) 2>/dev/null; true'
    ssh -t "$MOSHMUX_HOST" "tmux new-session -A -s '$name' -c '$dir' $@ \; send-keys 'cd $dir && clear' Enter"
  else
    tmux new-session -A -s "$name" -c "$dir" "$@" \; send-keys "cd $dir && clear" Enter
  fi
}

# Add your aliases below, or use: moshmux add <name> <dir>
# alias myproject='mux myproject ~/workspace/myproject'
