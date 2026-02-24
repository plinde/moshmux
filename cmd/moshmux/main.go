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

func repoDir() string {
	if d := os.Getenv("MOSHMUX_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "workspace", "moshmux")
}

func readFile(name string) string {
	data, err := os.ReadFile(filepath.Join(repoDir(), name))
	if err != nil {
		fatal("reading %s: %s", name, err)
	}
	return string(data)
}

func writeFile(name, content string) {
	if err := os.WriteFile(filepath.Join(repoDir(), name), []byte(content), 0644); err != nil {
		fatal("writing %s: %s", name, err)
	}
}

// gitRun executes a git command quietly. Commit shows its summary line,
// push shows only the "To ..." and ref lines.
func gitRun(args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir()
	switch args[0] {
	case "commit":
		cmd.Stdout = os.Stdout
	case "push":
		out, err := cmd.CombinedOutput()
		if err != nil {
			os.Stderr.Write(out)
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

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  moshmux <name>                Attach to named session
  moshmux                       Interactive session picker (fzf)
  moshmux .                     Attach to session matching cwd
  moshmux list                  Live-updating session status (TUI)
  moshmux list --no-tui         Print session table and exit
  moshmux add .                 Add cwd (name = directory basename)
  moshmux add . <name>          Add cwd with custom alias name
  moshmux add <name> <target>   Add alias (target = dir or alias name)
  moshmux remove .              Remove alias matching cwd
  moshmux remove <name>         Remove <name> alias
  moshmux update <name> <path>  Update path of existing alias (. = cwd)
  moshmux kill <name>           Kill named tmux session (alias: close, reset)
  moshmux termius               Print Termius startup script (fzf picker)
  moshmux termius <name>        Print Termius startup script (direct attach)
  moshmux join <name>           Join session with independent window focus
  moshmux join .                Join session matching cwd
  moshmux upgrade               Detect and kill old tmux 2.6 server
  moshmux upgrade --force       Kill old server without confirmation
`)
	os.Exit(1)
}

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
func resolveTarget(name, target, zshContent string) (session, dir string, err error) {
	// Check for self-reference
	if target == name {
		return "", "", fmt.Errorf("cannot create alias that references itself")
	}

	// If it looks like a path, use it as-is (default behavior)
	if isPathLike(target) {
		return name, target, nil
	}

	// Otherwise, try to resolve as an alias
	alias, err := moshmux.FindAlias(zshContent, target)
	if err != nil {
		return "", "", fmt.Errorf("alias %s not found (did you mean a directory? use ~/path or ./path)", target)
	}

	return alias.Session, alias.Dir, nil
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

// readAliases parses moshmux.zsh and returns all configured aliases.
func readAliases() []moshmux.Alias {
	return moshmux.ParseAliases(readFile("moshmux.zsh"))
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

// gitCommitAndPush stages moshmux.zsh, commits with the given message, and pushes.
func gitCommitAndPush(msg string) {
	gitRun("add", "moshmux.zsh")
	gitRun("commit", "-m", msg)
	gitRun("push")
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
	alias, err := moshmux.FindAlias(readFile("moshmux.zsh"), name)
	if err != nil {
		fatal("%s", err)
	}
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

	// Compute column width from longest alias name (floor 16, includes command shortcuts)
	nameWidth := 16
	for _, a := range aliases {
		if len(a.Name) > nameWidth {
			nameWidth = len(a.Name)
		}
	}
	nameWidth++ // one space padding

	nameFmt := fmt.Sprintf("%%-%ds", nameWidth)

	// Build lines for fzf: sessions first, then command shortcuts
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
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf(nameFmt+"%-10s %-10s %s", "add", "", "", "moshmux add <name> <dir>"))
	lines = append(lines, fmt.Sprintf(nameFmt+"%-10s %-10s %s", "add .", "", "", "moshmux add . (use cwd)"))
	lines = append(lines, fmt.Sprintf(nameFmt+"%-10s %-10s %s", "list", "", "", "moshmux list"))
	lines = append(lines, fmt.Sprintf(nameFmt+"%-10s %-10s %s", "join", "", "", "moshmux join <name> (independent windows)"))
	lines = append(lines, fmt.Sprintf(nameFmt+"%-10s %-10s %s", "upgrade", "", "", "moshmux upgrade (kill old tmux 2.6)"))
	lines = append(lines, fmt.Sprintf(nameFmt+"%-10s %-10s %s", "remove", "", "", "moshmux remove <name>"))

	// Pipe to fzf
	fzf := exec.Command("fzf", "--prompt=tmux> ", "--reverse", "--header=Sessions ─ select to attach, or see commands below")
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
	action := fields[0]

	// Handle command shortcuts
	switch action {
	case "add":
		if len(fields) > 1 && fields[1] == "." {
			// "add ." shortcut: register cwd using cwd basename as alias name
			cwd := cwdTilde()
			name := filepath.Base(cwd)
			if strings.HasPrefix(cwd, "~") {
				name = filepath.Base(strings.TrimPrefix(cwd, "~"))
			}
			cmdAdd(name, cwd)
		} else {
			fmt.Println("Run: moshmux add <name> <dir>")
		}
		return
	case "join":
		if len(fields) > 1 {
			cmdJoin(fields[1])
		} else {
			fmt.Println("Run: moshmux join <name>")
		}
		return
	case "list":
		cmdList(true)
		return
	case "upgrade":
		cmdUpgrade(false)
		return
	case "remove":
		if len(fields) > 1 {
			cmdRemove(fields[1])
		} else {
			fmt.Println("Run: moshmux remove <name>")
		}
		return
	}

	// Session selected — attach by name
	cmdName(action)
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

	zshContent := readFile("moshmux.zsh")

	// Resolve target to session and directory
	session, dir, err := resolveTarget(name, target, zshContent)
	if err != nil {
		fatal("%s", err)
	}

	// Add the alias
	newZsh, err := moshmux.AddAliasZshWithSession(zshContent, name, session, dir)
	if err != nil {
		fatal("%s", err)
	}

	writeFile("moshmux.zsh", newZsh)

	// Determine commit message based on whether it's linking
	var msg string
	if session == name {
		msg = fmt.Sprintf("Add %s alias (%s)", name, dir)
	} else {
		msg = fmt.Sprintf("Add %s alias (links to %s session)", name, session)
	}

	gitCommitAndPush(msg)

	// Show what was created
	if session == name {
		fmt.Printf("Added %s → %s\n", name, dir)
	} else {
		fmt.Printf("Added %s → %s session (%s)\n", name, session, dir)
	}
}

func cmdUpdate(name, dir string) {
	zshContent := readFile("moshmux.zsh")

	newZsh, err := moshmux.UpdateAliasZsh(zshContent, name, dir)
	if err != nil {
		fatal("%s", err)
	}

	writeFile("moshmux.zsh", newZsh)

	gitCommitAndPush(fmt.Sprintf("Update %s path to %s", name, dir))

	fmt.Printf("Updated %s → %s\n", name, dir)
}

func cmdRemove(name string) {
	zshContent := readFile("moshmux.zsh")

	newZsh, err := moshmux.RemoveAliasZsh(zshContent, name)
	if err != nil {
		fatal("%s", err)
	}

	writeFile("moshmux.zsh", newZsh)

	gitCommitAndPush(fmt.Sprintf("Remove %s alias", name))

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
		fmt.Scanln(&response)
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
