package moshmux

import (
	"testing"
)

func TestParseAliases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Alias
	}{
		{
			name:  "single alias",
			input: "alias mc='mux minecraft ~/workspace/minecraft'",
			want:  []Alias{{Name: "mc", Session: "minecraft", Dir: "~/workspace/minecraft"}},
		},
		{
			name:  "multiple aliases",
			input: "alias mc='mux minecraft ~/workspace/minecraft'\nalias dev='mux dev ~/workspace/dev'",
			want: []Alias{
				{Name: "mc", Session: "minecraft", Dir: "~/workspace/minecraft"},
				{Name: "dev", Session: "dev", Dir: "~/workspace/dev"},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "non-alias lines ignored",
			input: "# comment\nexport FOO=bar\nalias mc='mux minecraft ~/workspace/minecraft'\nsome other line",
			want:  []Alias{{Name: "mc", Session: "minecraft", Dir: "~/workspace/minecraft"}},
		},
		{
			name:  "non-mux alias ignored",
			input: "alias ll='ls -la'",
			want:  nil,
		},
		{
			name:  "double-quoted value",
			input: `alias mc="mux minecraft ~/workspace/minecraft"`,
			want:  []Alias{{Name: "mc", Session: "minecraft", Dir: "~/workspace/minecraft"}},
		},
		{
			name:  "malformed no equals",
			input: "alias mc mux minecraft ~/workspace/minecraft",
			want:  nil,
		},
		{
			name:  "mux with only two parts",
			input: "alias mc='mux minecraft'",
			want:  nil,
		},
		{
			name:  "dir with spaces in path",
			input: "alias mc='mux minecraft ~/workspace/my project dir'",
			want:  []Alias{{Name: "mc", Session: "minecraft", Dir: "~/workspace/my project dir"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAliases(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseAliases() returned %d aliases, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("alias[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindAlias(t *testing.T) {
	content := "alias mc='mux minecraft ~/workspace/minecraft'\nalias dev='mux dev ~/workspace/dev'"

	t.Run("found", func(t *testing.T) {
		a, err := FindAlias(content, "mc")
		if err != nil {
			t.Fatalf("FindAlias() error = %v", err)
		}
		if a.Name != "mc" || a.Session != "minecraft" || a.Dir != "~/workspace/minecraft" {
			t.Errorf("FindAlias() = %+v", a)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := FindAlias(content, "nonexistent")
		if err == nil {
			t.Fatal("FindAlias() expected error for missing alias")
		}
	})
}

func TestAddAliasZshWithSession(t *testing.T) {
	base := "alias mc='mux minecraft ~/workspace/minecraft'\n"

	t.Run("success", func(t *testing.T) {
		got, err := AddAliasZshWithSession(base, "dev", "dev", "~/workspace/dev")
		if err != nil {
			t.Fatalf("AddAliasZshWithSession() error = %v", err)
		}
		aliases := ParseAliases(got)
		if len(aliases) != 2 {
			t.Fatalf("expected 2 aliases, got %d", len(aliases))
		}
		if aliases[1].Name != "dev" || aliases[1].Session != "dev" || aliases[1].Dir != "~/workspace/dev" {
			t.Errorf("new alias = %+v", aliases[1])
		}
	})

	t.Run("duplicate error", func(t *testing.T) {
		_, err := AddAliasZshWithSession(base, "mc", "minecraft", "~/workspace/minecraft")
		if err == nil {
			t.Fatal("expected error for duplicate alias")
		}
	})

	t.Run("preserves existing", func(t *testing.T) {
		got, err := AddAliasZshWithSession(base, "new", "newsess", "~/workspace/new")
		if err != nil {
			t.Fatalf("AddAliasZshWithSession() error = %v", err)
		}
		aliases := ParseAliases(got)
		if aliases[0].Name != "mc" {
			t.Errorf("existing alias modified: %+v", aliases[0])
		}
	})
}

func TestUpdateAliasZsh(t *testing.T) {
	base := "alias mc='mux minecraft ~/workspace/minecraft'\nalias dev='mux dev ~/workspace/dev'\n"

	t.Run("success", func(t *testing.T) {
		got, err := UpdateAliasZsh(base, "mc", "~/workspace/minecraft-new")
		if err != nil {
			t.Fatalf("UpdateAliasZsh() error = %v", err)
		}
		a, err := FindAlias(got, "mc")
		if err != nil {
			t.Fatalf("FindAlias() error = %v", err)
		}
		if a.Dir != "~/workspace/minecraft-new" {
			t.Errorf("dir = %q, want ~/workspace/minecraft-new", a.Dir)
		}
	})

	t.Run("preserves session name", func(t *testing.T) {
		got, err := UpdateAliasZsh(base, "mc", "~/workspace/other")
		if err != nil {
			t.Fatalf("UpdateAliasZsh() error = %v", err)
		}
		a, _ := FindAlias(got, "mc")
		if a.Session != "minecraft" {
			t.Errorf("session = %q, want minecraft", a.Session)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := UpdateAliasZsh(base, "nonexistent", "~/workspace/x")
		if err == nil {
			t.Fatal("expected error for missing alias")
		}
	})
}

func TestRemoveAliasZsh(t *testing.T) {
	base := "alias mc='mux minecraft ~/workspace/minecraft'\nalias dev='mux dev ~/workspace/dev'\n"

	t.Run("success", func(t *testing.T) {
		got, err := RemoveAliasZsh(base, "mc")
		if err != nil {
			t.Fatalf("RemoveAliasZsh() error = %v", err)
		}
		aliases := ParseAliases(got)
		if len(aliases) != 1 {
			t.Fatalf("expected 1 alias, got %d", len(aliases))
		}
		if aliases[0].Name != "dev" {
			t.Errorf("wrong alias remaining: %+v", aliases[0])
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := RemoveAliasZsh(base, "nonexistent")
		if err == nil {
			t.Fatal("expected error for missing alias")
		}
	})

	t.Run("preserves other lines", func(t *testing.T) {
		input := "# header\nalias mc='mux minecraft ~/workspace/minecraft'\nalias dev='mux dev ~/workspace/dev'\n# footer\n"
		got, err := RemoveAliasZsh(input, "mc")
		if err != nil {
			t.Fatalf("RemoveAliasZsh() error = %v", err)
		}
		aliases := ParseAliases(got)
		if len(aliases) != 1 || aliases[0].Name != "dev" {
			t.Fatalf("unexpected aliases: %+v", aliases)
		}
		// Verify non-alias lines preserved
		if !contains(got, "# header") || !contains(got, "# footer") {
			t.Error("non-alias lines were not preserved")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
