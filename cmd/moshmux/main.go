package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"golang.org/x/term"

	"github.com/plinde/moshmux"
)

// config holds moshmux settings from config.toml.
type config struct {
	AliasesDir string // where aliases.toml lives (empty = same as config dir)
	GitSync    bool   // auto git add/commit/push on alias changes
}

// configDir returns the XDG config directory for moshmux.
func configDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "moshmux")
}

// configPath returns the path to config.toml.
func configPath() string {
	return filepath.Join(configDir(), "config.toml")
}

// loadConfig reads config.toml and returns the parsed config.
func loadConfig() config {
	var cfg config
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, "\"")

		switch key {
		case "aliases_dir":
			cfg.AliasesDir = val
		case "git_sync":
			cfg.GitSync = val == "true"
		}
	}
	return cfg
}

// writeConfig writes the config struct to config.toml.
func writeConfig(cfg config) {
	cfgPath := configPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		fatal("creating config dir: %s", err)
	}
	var b strings.Builder
	if cfg.AliasesDir != "" {
		fmt.Fprintf(&b, "aliases_dir = \"%s\"\n", cfg.AliasesDir)
	}
	if cfg.GitSync {
		b.WriteString("git_sync = true\n")
	}
	if err := os.WriteFile(cfgPath, []byte(b.String()), 0644); err != nil {
		fatal("writing config: %s", err)
	}
}

// aliasesDir returns the directory containing aliases.toml.
// Priority: MOSHMUX_DIR env > config aliases_dir > XDG config dir.
func aliasesDir() string {
	if d := os.Getenv("MOSHMUX_DIR"); d != "" {
		return d
	}
	cfg := loadConfig()
	if cfg.AliasesDir != "" {
		return expandHome(cfg.AliasesDir)
	}
	return configDir()
}

// aliasesPath returns the full path to aliases.toml.
func aliasesPath() string {
	return filepath.Join(aliasesDir(), "aliases.toml")
}

// readAliasesFile reads and parses aliases.toml.
func readAliasesFile() []moshmux.Alias {
	path := aliasesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		fatal("reading %s: %s", path, err)
	}
	return moshmux.ParseAliasesToml(string(data))
}

// writeAliasesFile writes aliases to aliases.toml, creating the directory if needed.
func writeAliasesFile(aliases []moshmux.Alias) {
	path := aliasesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fatal("creating aliases dir: %s", err)
	}
	content := moshmux.MarshalAliasesToml(aliases)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fatal("writing %s: %s", path, err)
	}
}

// gitRunIn executes a git command in the given directory.
func gitRunIn(dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	switch args[0] {
	case "commit":
		cmd.Stdout = os.Stdout
	case "push":
		out, err := cmd.CombinedOutput()
		if err != nil {
			_, _ = os.Stderr.Write(out)
			fatal("git push: %s", err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "To ") || strings.Contains(trimmed, "->") {
				fmt.Println(trimmed)
			}
		}
		return
	}
	if err := cmd.Run(); err != nil {
		fatal("git %s: %s", args[0], err)
	}
}

// syncIfEnabled runs git add/commit/push on aliases.toml if git_sync is enabled
// and the aliases dir is a git repo.
func syncIfEnabled(msg string) {
	cfg := loadConfig()
	if !cfg.GitSync {
		return
	}
	dir := aliasesDir()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return
	}
	gitRunIn(dir, "add", "aliases.toml")
	gitRunIn(dir, "commit", "-m", msg)
	gitRunIn(dir, "push")
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  moshmux <name>                  Attach to named session
  moshmux                         Interactive session picker (fzf)
  moshmux .                       Attach to session matching cwd
  moshmux list                    Live-updating session status (TUI)
  moshmux list --no-tui           Print session table and exit
  moshmux add .                   Add cwd (name = directory basename)
  moshmux add . <name>            Add cwd with custom alias name
  moshmux add <name> <target>     Add alias (target = dir or alias name)
  moshmux remove .                Remove alias matching cwd
  moshmux remove <name>           Remove <name> alias
  moshmux update <name> <path>    Update path of existing alias (. = cwd)
  moshmux kill <name>             Kill named tmux session (alias: close, reset)
  moshmux termius                 Print Termius startup script (fzf picker)
  moshmux termius <name>          Print Termius startup script (direct attach)
  moshmux join <name>             Join session with independent window focus
  moshmux join .                  Join session matching cwd
  moshmux config                  Show current config
  moshmux config set-aliases-dir  Set aliases directory (. = cwd)
  moshmux config set-git-sync     Enable/disable git sync (on|off)
  moshmux migrate [path]          Migrate moshmux.zsh to aliases.toml
  moshmux upgrade                 Detect and kill old tmux 2.6 server
  moshmux upgrade --force         Kill old server without confirmation
`)
	os.Exit(1)
}

// Set via -ldflags at build time.
var version = "dev"

func fatal(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+msg+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		cmdPicker()
		return
	}

	switch os.Args[1] {
	case "--version", "-v", "version":
		fmt.Println(version)
		return
	case ".":
		cmdDot()
	case "list":
		explicitNoTUI := len(os.Args) > 2 && os.Args[2] == "--no-tui"
		isPiped := !term.IsTerminal(int(os.Stdout.Fd()))
		cmdList(explicitNoTUI || isPiped)
	case "add":
		handleAdd()
	case "update":
		if len(os.Args) != 4 {
			usage()
		}
		name := os.Args[2]
		dir := os.Args[3]
		if dir == "." {
			dir = cwdTilde()
		}
		cmdUpdate(name, dir)
	case "remove":
		if len(os.Args) != 3 {
			usage()
		}
		name := os.Args[2]
		if name == "." {
			name = resolveDot(readAliases()).Name
		}
		cmdRemove(name)
	case "termius":
		name := ""
		if len(os.Args) == 3 {
			name = os.Args[2]
		} else if len(os.Args) > 3 {
			usage()
		}
		cmdTermius(name)
	case "join":
		if len(os.Args) != 3 {
			usage()
		}
		cmdJoin(os.Args[2])
	case "kill", "close", "reset":
		if len(os.Args) != 3 {
			usage()
		}
		cmdKillSession(os.Args[2])
	case "config":
		cmdConfig()
	case "migrate":
		path := ""
		if len(os.Args) == 3 {
			path = os.Args[2]
		}
		cmdMigrate(path)
	case "upgrade":
		force := len(os.Args) > 2 && os.Args[2] == "--force"
		cmdUpgrade(force)
	default:
		if len(os.Args) == 2 {
			cmdName(os.Args[1])
		} else {
			usage()
		}
	}
}

// isPathLike determines if a string looks like a directory path.
func isPathLike(arg string) bool {
	return arg == "." ||
		strings.Contains(arg, "/") ||
		strings.HasPrefix(arg, "~")
}

// resolveTarget determines if target is a path or alias name.
// Returns (session, dir, error).
// - If target is path-like, returns (name, target, nil) for default behavior
// - If target is alias, looks up and returns (alias.Session, alias.Dir, nil)
func resolveTarget(name, target string, aliases []moshmux.Alias) (session, dir string, err error) {
	// Check for self-reference
	if target == name {
		return "", "", fmt.Errorf("cannot create alias that references itself")
	}

	// If it looks like a path, use it as-is (default behavior)
	if isPathLike(target) {
		return name, target, nil
	}

	// Otherwise, try to resolve as an alias
	for _, a := range aliases {
		if a.Name == target {
			return a.Session, a.Dir, nil
		}
	}

	return "", "", fmt.Errorf("alias %s not found (did you mean a directory? use ~/path or ./path)", target)
}

// handleAdd resolves "moshmux add ." and "moshmux add <name> <target>" forms.
// Target can be a directory path or an existing alias name.
func handleAdd() {
	switch len(os.Args) {
	case 3:
		// moshmux add . — derive name from cwd basename
		target := os.Args[2]
		if target == "." {
			target = cwdTilde()
		}
		name := filepath.Base(target)
		// Strip ~ prefix for Base to work on tilde paths
		if strings.HasPrefix(target, "~") {
			name = filepath.Base(strings.TrimPrefix(target, "~"))
		}
		cmdAdd(name, target)
	case 4:
		name := os.Args[2]
		target := os.Args[3]
		if name == "." {
			// moshmux add . <name> — cwd with custom alias name
			cmdAdd(target, cwdTilde())
		} else {
			// moshmux add <name> <target>
			if target == "." {
				target = cwdTilde()
			}
			cmdAdd(name, target)
		}
	default:
		usage()
	}
}

// cwdTilde returns the current working directory with $HOME replaced by ~.
func cwdTilde() string {
	cwd, err := os.Getwd()
	if err != nil {
		fatal("getting cwd: %s", err)
	}
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(cwd, home) {
		return "~" + strings.TrimPrefix(cwd, home)
	}
	return cwd
}

// readAliases reads aliases from aliases.toml.
func readAliases() []moshmux.Alias {
	return readAliasesFile()
}

// resolveDot resolves "." to an alias via cwd: match by dir path, then by basename.
// Fatals if no match is found.
func resolveDot(aliases []moshmux.Alias) *moshmux.Alias {
	cwd := cwdTilde()

	// Match by directory path
	for i := range aliases {
		if aliases[i].Dir == cwd {
			return &aliases[i]
		}
	}

	// Match by cwd basename as alias name
	base := filepath.Base(cwd)
	for i := range aliases {
		if aliases[i].Name == base {
			return &aliases[i]
		}
	}

	fmt.Fprintf(os.Stderr, "No alias found for %s (tried dir match and name %q).\nRun: moshmux add .\n", cwd, base)
	os.Exit(1)
	return nil
}

// resolveAlias resolves an alias by exact name or unique prefix match.
// Fatals if not found or ambiguous.
func resolveAlias(name string, aliases []moshmux.Alias) *moshmux.Alias {
	// Exact match first
	for i := range aliases {
		if aliases[i].Name == name {
			return &aliases[i]
		}
	}

	// Prefix match fallback
	var matches []moshmux.Alias
	for _, a := range aliases {
		if strings.HasPrefix(a.Name, name) {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		fatal("no alias %s found", name)
	case 1:
		return &matches[0]
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		fatal("ambiguous: %q matches %s", name, strings.Join(names, ", "))
	}
	return nil
}

// findRunningTmux finds a tmux binary that can talk to the running server.
// Returns the binary path, or "" if no server is running.
func findRunningTmux() string {
	socket := tmuxSocketPath()
	for _, bin := range tmuxBinaries() {
		if err := exec.Command(bin, "-S", socket, "list-sessions").Run(); err == nil {
			return bin
		}
	}
	return ""
}

// expandHome expands ~/... to an absolute path.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return path
}

// cmdDot attaches to the tmux session matching cwd, or tells the user
// to register it first. Matches by directory path first, then by name.
func cmdDot() {
	attachSession(resolveDot(readAliases()))
}

// moshEnvPath returns the PATH for mosh-server's environment.
// Uses the current process PATH so it works on any machine.
func moshEnvPath() string {
	if p := os.Getenv("PATH"); p != "" {
		return p
	}
	return "/usr/local/bin:/usr/bin:/bin"
}

// cmdTermius prints a Termius-ready startup script.
// With no name: prints the full mosh-server command for the fzf picker.
// With a name: prints the full mosh-server command for direct session attach.
func cmdTermius(name string) {
	self, _ := os.Executable()
	if name == "" {
		fmt.Printf("mosh-server new -c 256 -s -l LANG=en_US.UTF-8 -- /usr/bin/env PATH=%s %s\n", moshEnvPath(), self)
		return
	}
	alias := resolveAlias(name, readAliases())
	fmt.Printf("mosh-server new -c 256 -s -l LANG=en_US.UTF-8 -- /usr/bin/env PATH=%s tmux new-session -A -s '%s' -c '%s'\n", moshEnvPath(), alias.Session, alias.Dir)
}

// cmdKillSession terminates the tmux session associated with the given alias name.
// Accepts alias name or "." for cwd match. Errors if no session is running.
func cmdKillSession(name string) {
	aliases := readAliases()
	if name == "." {
		name = resolveDot(aliases).Name
	}

	alias := resolveAlias(name, aliases)
	session := alias.Session

	tmuxBin := findRunningTmux()
	if tmuxBin == "" {
		fmt.Fprintf(os.Stderr, "No tmux session '%s' is running.\n", session)
		os.Exit(1)
	}

	socket := tmuxSocketPath()
	if err := exec.Command(tmuxBin, "-S", socket, "has-session", "-t", session).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "No tmux session '%s' is running.\n", session)
		os.Exit(1)
	}

	if err := exec.Command(tmuxBin, "-S", socket, "kill-session", "-t", session).Run(); err != nil {
		fatal("kill-session %s: %s", session, err)
	}
	fmt.Printf("Killed tmux session '%s'.\n", session)
}

// cmdName attaches to the tmux session with the given alias name.
func cmdName(name string) {
	attachSession(resolveAlias(name, readAliases()))
}

// tmuxBinaries returns tmux paths to check, in preference order.
// Includes Linuxbrew path if present, then system paths, then PATH lookup.
func tmuxBinaries() []string {
	var bins []string
	// Linuxbrew tmux (newer version, if installed)
	if _, err := os.Stat("/home/linuxbrew/.linuxbrew/bin/tmux"); err == nil {
		bins = append(bins, "/home/linuxbrew/.linuxbrew/bin/tmux")
	}
	// System tmux
	if _, err := os.Stat("/usr/bin/tmux"); err == nil {
		bins = append(bins, "/usr/bin/tmux")
	}
	// PATH lookup fallback
	if p, err := exec.LookPath("tmux"); err == nil {
		// Avoid duplicates
		dup := false
		for _, b := range bins {
			if b == p {
				dup = true
				break
			}
		}
		if !dup {
			bins = append(bins, p)
		}
	}
	return bins
}

// tmuxSocketPath returns the default tmux socket path for the current user.
func tmuxSocketPath() string {
	return fmt.Sprintf("/tmp/tmux-%d/default", os.Getuid())
}

// getTmuxServerVersion queries the running tmux server version.
// Returns empty string if no server is running.
// Strategy: try each binary in tmuxBinaries order until one can talk to the
// server, then ask the server for its actual version via display-message #{version}.
func getTmuxServerVersion() string {
	socket := tmuxSocketPath()
	for _, bin := range tmuxBinaries() {
		out, err := exec.Command(bin, "-S", socket, "display-message", "-p", "#{version}").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// findTmuxForSession returns the tmux binary to use for the given session.
// If any tmux server is already running, we must use the system tmux (2.6)
// because the newer Linuxbrew client can't talk to the older server.
func findTmuxForSession(name string) string {
	socket := tmuxSocketPath()
	bins := tmuxBinaries()
	sysTmux := bins[len(bins)-1]

	// Check if a server is already running by trying to list sessions with system tmux.
	// If the command succeeds (exit 0), a server is running and we must use system tmux.
	// (list-sessions returns exit 1 with "no server running" if no server exists)
	if _, err := os.Stat(sysTmux); err == nil {
		if err := exec.Command(sysTmux, "-S", socket, "list-sessions").Run(); err == nil {
			return sysTmux
		}
	}

	// No server running — use preferred binary for fresh start
	for _, bin := range bins {
		if _, err := os.Stat(bin); err == nil {
			return bin
		}
	}
	fatal("no tmux binary found")
	return ""
}

// attachSession execs into a tmux session for the given alias.
func attachSession(a *moshmux.Alias) {
	dir := expandHome(a.Dir)
	socket := tmuxSocketPath()
	tmux := findTmuxForSession(a.Session)
	if err := syscall.Exec(tmux, []string{"tmux", "-S", socket, "new-session", "-AD", "-s", a.Session, "-c", dir}, os.Environ()); err != nil {
		fatal("exec tmux: %s", err)
	}
}

// cmdJoin resolves an alias name (with prefix matching and "." support)
// and joins the existing tmux session with independent window focus.
func cmdJoin(name string) {
	aliases := readAliases()
	if name == "." {
		name = resolveDot(aliases).Name
	}

	alias := resolveAlias(name, aliases)

	// Verify the target session is actually running
	tmuxBin := findRunningTmux()
	if tmuxBin == "" {
		fatal("no tmux session '%s' is running — start it first with: moshmux %s", alias.Session, alias.Name)
	}

	socket := tmuxSocketPath()
	if err := exec.Command(tmuxBin, "-S", socket, "has-session", "-t", alias.Session).Run(); err != nil {
		fatal("no tmux session '%s' is running — start it first with: moshmux %s", alias.Session, alias.Name)
	}

	joinSession(alias, tmuxBin, socket)
}

// joinSession creates a grouped tmux session that shares windows with the
// target session but allows independent window focus per client.
// The grouped session auto-destroys on detach.
func joinSession(a *moshmux.Alias, tmuxBin, socket string) {
	joinName := fmt.Sprintf("%s-join-%d", a.Session, os.Getpid())
	if err := syscall.Exec(tmuxBin, []string{
		"tmux", "-S", socket,
		"new-session", "-t", a.Session, "-s", joinName,
		";", "set-option", "destroy-unattached", "on",
	}, os.Environ()); err != nil {
		fatal("exec tmux: %s", err)
	}
}

func cmdPicker() {
	aliases := readAliases()
	if len(aliases) == 0 {
		fatal("no aliases configured")
	}

	// Query session info and sort by activity
	sessions := tmuxSessions()
	aliases = sortAliasesByActivity(aliases, sessions)

	// Compute column width from longest alias name (floor 16)
	nameWidth := 16
	for _, a := range aliases {
		if len(a.Name) > nameWidth {
			nameWidth = len(a.Name)
		}
	}
	nameWidth++ // one space padding

	nameFmt := fmt.Sprintf("%%-%ds", nameWidth)

	// Build lines for fzf
	var lines []string
	for _, a := range aliases {
		status := "-"
		lastActive := "-"

		if info, ok := sessions[a.Session]; ok {
			if info.attached {
				status = "attached"
			} else {
				status = "detached"
			}
			lastActive = formatRelativeTime(info.activity)
		}

		lines = append(lines, fmt.Sprintf(nameFmt+"%-10s %-10s %s", a.Name, status, lastActive, a.Dir))
	}

	// Pipe to fzf
	fzf := exec.Command("fzf", "--prompt=tmux> ", "--reverse", "--header=Sessions ─ select to attach")
	fzf.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	fzf.Stderr = os.Stderr
	out, err := fzf.Output()
	if err != nil {
		os.Exit(0)
	}

	choice := strings.TrimSpace(string(out))
	if choice == "" {
		os.Exit(0)
	}

	fields := strings.Fields(choice)
	cmdName(fields[0])
}

type sessionInfo struct {
	cwd      string
	attached bool
	activity time.Time
}

// tmuxSessions queries the tmux server for running sessions, returning a map of
// session name → session info (cwd and attached status).
func tmuxSessions() map[string]sessionInfo {
	m := make(map[string]sessionInfo)
	home, _ := os.UserHomeDir()

	tmuxBin := findRunningTmux()
	if tmuxBin == "" {
		return m
	}

	socket := tmuxSocketPath()
	out, err := exec.Command(tmuxBin, "-S", socket, "list-sessions", "-F", "#{session_name}\t#{pane_current_path}\t#{session_attached}\t#{session_activity}").Output()
	if err != nil {
		return m
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		dir := parts[1]
		if strings.HasPrefix(dir, home) {
			dir = "~" + strings.TrimPrefix(dir, home)
		}

		// Parse activity timestamp
		activityUnix, _ := strconv.ParseInt(parts[3], 10, 64)
		activity := time.Unix(activityUnix, 0)

		m[parts[0]] = sessionInfo{
			cwd:      dir,
			attached: parts[2] == "1",
			activity: activity,
		}
	}
	return m
}

// --- bubbletea list TUI ---

type tickMsg time.Time

type listModel struct {
	aliases []moshmux.Alias
	rows    [][]string
}

func (m listModel) Init() tea.Cmd {
	return tea.Batch(tea.ClearScreen, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }))
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		return m, tea.Quit
	case tickMsg:
		m.rows = buildRows(m.aliases, tmuxSessions())
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
	}
	return m, nil
}

func (m listModel) View() string {
	return renderTable(m.rows) + "\n" + lipgloss.NewStyle().Faint(true).Render("Press any key to quit • refreshes every 2s")
}

// formatRelativeTime formats a timestamp as relative time (e.g., "2m ago", "5h ago")
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	if days < 7 {
		return fmt.Sprintf("%dd ago", days)
	}
	weeks := days / 7
	if weeks == 1 {
		return "1w ago"
	}
	if weeks < 4 {
		return fmt.Sprintf("%dw ago", weeks)
	}
	months := days / 30
	return fmt.Sprintf("%dmo ago", months)
}

// sortAliasesByActivity sorts aliases by session activity, with most recent first.
// Aliases without running sessions appear at the bottom in their original order.
func sortAliasesByActivity(aliases []moshmux.Alias, sessions map[string]sessionInfo) []moshmux.Alias {
	// Separate active and inactive aliases
	var active, inactive []moshmux.Alias
	for _, a := range aliases {
		if _, ok := sessions[a.Session]; ok {
			active = append(active, a)
		} else {
			inactive = append(inactive, a)
		}
	}

	// Sort active aliases by activity (most recent first)
	sort.Slice(active, func(i, j int) bool {
		ti := sessions[active[i].Session].activity
		tj := sessions[active[j].Session].activity
		return ti.After(tj) // Most recent first
	})

	// Combine: active sessions first, then inactive
	return append(active, inactive...)
}

func buildRows(aliases []moshmux.Alias, sessions map[string]sessionInfo) [][]string {
	// Sort aliases by activity
	aliases = sortAliasesByActivity(aliases, sessions)

	rows := [][]string{}
	for _, a := range aliases {
		status := "-"
		actual := ""
		lastActive := "-"

		if info, ok := sessions[a.Session]; ok {
			if info.attached {
				status = "attached"
			} else {
				status = "detached"
			}
			if info.cwd != a.Dir {
				actual = info.cwd
			}
			// Format relative time
			lastActive = formatRelativeTime(info.activity)
		}

		rows = append(rows, []string{
			a.Name,
			status,
			lastActive,
			a.Dir,
			actual,
		})
	}
	return rows
}

func renderTable(rows [][]string) string {
	dim := lipgloss.NewStyle().Faint(true)

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(dim).
		Headers("ALIAS", "STATUS", "LAST ACTIVE", "CONFIGURED", "ACTUAL CWD").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true)
			}
			if col == 1 && row >= 0 && row < len(rows) {
				switch rows[row][1] {
				case "attached":
					return lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
				case "detached":
					return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
				default:
					return dim
				}
			}
			return lipgloss.NewStyle()
		}).
		Rows(rows...)

	return t.String()
}

func cmdList(noTUI bool) {
	aliases := readAliases()
	if len(aliases) == 0 {
		fmt.Println("No aliases configured.")
		return
	}

	if noTUI {
		fmt.Println(renderTable(buildRows(aliases, tmuxSessions())))
		return
	}

	m := listModel{
		aliases: aliases,
		rows:    buildRows(aliases, tmuxSessions()),
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fatal("tui: %s", err)
	}
}

func cmdAdd(name, target string) {
	if name == "" {
		fatal("alias name cannot be empty")
	}

	aliases := readAliases()

	// Resolve target to session and directory
	session, dir, err := resolveTarget(name, target, aliases)
	if err != nil {
		fatal("%s", err)
	}

	// Add the alias
	aliases, err = moshmux.AddAliasToml(aliases, name, session, dir)
	if err != nil {
		fatal("%s", err)
	}

	writeAliasesFile(aliases)

	// Determine commit message based on whether it's linking
	var msg string
	if session == name {
		msg = fmt.Sprintf("Add %s alias (%s)", name, dir)
	} else {
		msg = fmt.Sprintf("Add %s alias (links to %s session)", name, session)
	}

	syncIfEnabled(msg)

	// Show what was created
	if session == name {
		fmt.Printf("Added %s → %s\n", name, dir)
	} else {
		fmt.Printf("Added %s → %s session (%s)\n", name, session, dir)
	}
}

func cmdUpdate(name, dir string) {
	aliases := readAliases()

	aliases, err := moshmux.UpdateAliasToml(aliases, name, dir)
	if err != nil {
		fatal("%s", err)
	}

	writeAliasesFile(aliases)

	syncIfEnabled(fmt.Sprintf("Update %s path to %s", name, dir))

	fmt.Printf("Updated %s → %s\n", name, dir)
}

func cmdRemove(name string) {
	aliases := readAliases()

	aliases, err := moshmux.RemoveAliasToml(aliases, name)
	if err != nil {
		fatal("%s", err)
	}

	writeAliasesFile(aliases)

	syncIfEnabled(fmt.Sprintf("Remove %s alias", name))

	fmt.Printf("Removed %s\n", name)
}

// cmdUpgrade detects old tmux servers and kills them to allow upgrade.
func cmdUpgrade(force bool) {
	version := getTmuxServerVersion()

	// No server running
	if version == "" {
		fmt.Println("No tmux server running. Next session will use tmux 3.6a ✓")
		return
	}

	// Already on new version
	if !strings.HasPrefix(version, "2.") {
		fmt.Printf("Already running tmux %s ✓\n", version)
		return
	}

	// Old version detected - warn and prompt
	fmt.Printf("Old tmux %s detected. Upgrade required.\n\n", version)

	aliases := readAliases()
	sessions := tmuxSessions()
	rows := buildRows(aliases, sessions)

	// Filter to only show active sessions
	activeRows := [][]string{}
	for _, row := range rows {
		if row[1] != "-" { // status column
			activeRows = append(activeRows, row)
		}
	}

	if len(activeRows) > 0 {
		fmt.Println("These sessions will be killed:")
		fmt.Println(renderTable(activeRows))
		fmt.Println()
	} else {
		fmt.Println("No active sessions.")
		fmt.Println()
	}

	// Prompt for confirmation unless --force
	if !force {
		fmt.Print("Kill old server and upgrade to tmux 3.6a? [y/N]: ")
		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			return
		}
	}

	// Kill the server
	socket := tmuxSocketPath()
	// Use the last tmux binary to kill — it can always talk to the old server
	upgradeBins := tmuxBinaries()
	if err := exec.Command(upgradeBins[len(upgradeBins)-1], "-S", socket, "kill-server").Run(); err != nil {
		fatal("killing tmux server: %s", err)
	}

	fmt.Println("✓ Old tmux server killed.")
	fmt.Println("Next moshmux session will use tmux 3.6a")
}

// cmdConfig shows or modifies moshmux configuration.
func cmdConfig() {
	if len(os.Args) == 2 {
		cfg := loadConfig()
		fmt.Printf("config:     %s\n", configPath())
		fmt.Printf("aliases:    %s\n", aliasesPath())
		if cfg.GitSync {
			fmt.Printf("git_sync:   on\n")
		} else {
			fmt.Printf("git_sync:   off\n")
		}
		return
	}

	if len(os.Args) >= 3 {
		switch os.Args[2] {
		case "set-aliases-dir":
			if len(os.Args) != 4 {
				fatal("usage: moshmux config set-aliases-dir <path>")
			}
			path := os.Args[3]
			if path == "." {
				var err error
				path, err = os.Getwd()
				if err != nil {
					fatal("getting cwd: %s", err)
				}
			} else {
				path = expandHome(path)
			}

			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				fatal("%s is not a directory", path)
			}

			cfg := loadConfig()
			cfg.AliasesDir = path
			writeConfig(cfg)

			fmt.Printf("aliases_dir = %s\n", path)
			fmt.Printf("  → %s\n", configPath())
			return

		case "set-git-sync":
			if len(os.Args) != 4 {
				fatal("usage: moshmux config set-git-sync on|off")
			}
			val := os.Args[3]
			cfg := loadConfig()
			switch val {
			case "on", "true":
				cfg.GitSync = true
			case "off", "false":
				cfg.GitSync = false
			default:
				fatal("expected on or off, got %q", val)
			}
			writeConfig(cfg)
			fmt.Printf("git_sync = %v\n", cfg.GitSync)
			return
		}
	}

	usage()
}

// cmdMigrate converts moshmux.zsh to aliases.toml.
func cmdMigrate(zshPath string) {
	if zshPath == "" {
		candidates := []string{
			filepath.Join(aliasesDir(), "moshmux.zsh"),
		}
		home, _ := os.UserHomeDir()
		candidates = append(candidates, filepath.Join(home, "workspace", "moshmux", "moshmux.zsh"))

		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				zshPath = c
				break
			}
		}
		if zshPath == "" {
			fatal("no moshmux.zsh found; pass path as argument: moshmux migrate /path/to/moshmux.zsh")
		}
	} else {
		zshPath = expandHome(zshPath)
	}

	if _, err := os.Stat(aliasesPath()); err == nil {
		fatal("aliases.toml already exists at %s; remove it first to re-migrate", aliasesPath())
	}

	data, err := os.ReadFile(zshPath)
	if err != nil {
		fatal("reading %s: %s", zshPath, err)
	}

	aliases := moshmux.ParseAliases(string(data))
	if len(aliases) == 0 {
		fatal("no aliases found in %s", zshPath)
	}

	writeAliasesFile(aliases)

	fmt.Printf("Migrated %d aliases from %s\n", len(aliases), zshPath)
	fmt.Printf("  → %s\n", aliasesPath())
}
