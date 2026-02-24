package moshmux

import (
	"fmt"
	"strings"
)

// Alias represents a moshmux alias.
type Alias struct {
	Name    string // alias name (what user types, e.g. "mc")
	Session string // tmux session name (e.g. "minecraft")
	Dir     string
}

// ParseAliases extracts moshmux aliases from moshmux.zsh content.
// Identifies lines of the form: alias <name>='mux <session> <dir>'
func ParseAliases(content string) []Alias {
	var aliases []Alias
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "alias ") {
			continue
		}
		// alias name='mux session ~/workspace/dir'
		rest := strings.TrimPrefix(line, "alias ")
		name, after, ok := strings.Cut(rest, "=")
		if !ok {
			continue
		}
		if (strings.HasPrefix(after, "'") && strings.HasSuffix(after, "'")) ||
			(strings.HasPrefix(after, "\"") && strings.HasSuffix(after, "\"")) {
			after = after[1 : len(after)-1]
		}
		// after = "mux session ~/workspace/dir"
		if !strings.HasPrefix(after, "mux ") {
			continue
		}
		parts := strings.SplitN(after, " ", 3)
		if len(parts) < 3 {
			continue
		}
		// parts[0]="mux", parts[1]=session, parts[2]=dir
		aliases = append(aliases, Alias{Name: name, Session: parts[1], Dir: parts[2]})
	}
	return aliases
}

// FindAlias searches for an alias by name and returns it.
// Returns error if not found.
func FindAlias(content, name string) (*Alias, error) {
	aliases := ParseAliases(content)
	for _, a := range aliases {
		if a.Name == name {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("alias %s not found", name)
}

// AddAliasZshWithSession appends an alias line with explicit session name.
// This allows creating aliases that point to the same session as another alias.
func AddAliasZshWithSession(content, name, session, dir string) (string, error) {
	line := fmt.Sprintf("alias %s='mux %s %s'", name, session, dir)

	// Check for duplicate
	for _, a := range ParseAliases(content) {
		if a.Name == name {
			return "", fmt.Errorf("alias %s already exists", name)
		}
	}

	// Insert before trailing newline
	content = strings.TrimRight(content, "\n")
	return content + "\n" + line + "\n", nil
}

// UpdateAliasZsh replaces the directory of an existing alias in-place,
// preserving the session name. Returns an error if the alias does not exist.
func UpdateAliasZsh(content, name, dir string) (string, error) {
	target := fmt.Sprintf("alias %s=", name)
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, target) {
			continue
		}
		// Parse existing alias to get the session name
		rest := strings.TrimPrefix(trimmed, "alias ")
		_, after, ok := strings.Cut(rest, "=")
		if !ok {
			continue
		}
		if (strings.HasPrefix(after, "'") && strings.HasSuffix(after, "'")) ||
			(strings.HasPrefix(after, "\"") && strings.HasSuffix(after, "\"")) {
			after = after[1 : len(after)-1]
		}
		if !strings.HasPrefix(after, "mux ") {
			continue
		}
		parts := strings.SplitN(after, " ", 3)
		if len(parts) < 3 {
			continue
		}
		session := parts[1]
		lines[i] = fmt.Sprintf("alias %s='mux %s %s'", name, session, dir)
		found = true
		break
	}
	if !found {
		return "", fmt.Errorf("alias %s not found", name)
	}
	return strings.Join(lines, "\n"), nil
}

// RemoveAliasZsh removes an alias line from moshmux.zsh content.
func RemoveAliasZsh(content, name string) (string, error) {
	target := fmt.Sprintf("alias %s=", name)
	lines := strings.Split(content, "\n")
	found := false
	var out []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), target) {
			found = true
			continue
		}
		out = append(out, line)
	}
	if !found {
		return "", fmt.Errorf("alias %s not found", name)
	}
	return strings.Join(out, "\n"), nil
}
