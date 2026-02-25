package moshmux

import (
	"testing"
)

func TestParseAliasesToml(t *testing.T) {
	content := `[mc]
session = "minecraft"
dir = "~/workspace/minecraft"

[moshmux]
dir = "~/workspace/moshmux"

# Comment line
[asm]
dir = "~/workspace/mytools"
`

	aliases := ParseAliasesToml(content)
	if len(aliases) != 3 {
		t.Fatalf("expected 3 aliases, got %d", len(aliases))
	}

	// mc: explicit session
	if aliases[0].Name != "mc" || aliases[0].Session != "minecraft" || aliases[0].Dir != "~/workspace/minecraft" {
		t.Errorf("mc alias: got %+v", aliases[0])
	}

	// moshmux: session defaults to name
	if aliases[1].Name != "moshmux" || aliases[1].Session != "moshmux" || aliases[1].Dir != "~/workspace/moshmux" {
		t.Errorf("moshmux alias: got %+v", aliases[1])
	}

	// asm: session defaults to name
	if aliases[2].Name != "asm" || aliases[2].Session != "asm" {
		t.Errorf("asm alias: got %+v", aliases[2])
	}
}

func TestMarshalAliasesToml(t *testing.T) {
	aliases := []Alias{
		{Name: "mc", Session: "minecraft", Dir: "~/workspace/minecraft"},
		{Name: "moshmux", Session: "moshmux", Dir: "~/workspace/moshmux"},
	}

	got := MarshalAliasesToml(aliases)
	expected := "[mc]\nsession = \"minecraft\"\ndir = \"~/workspace/minecraft\"\n\n[moshmux]\ndir = \"~/workspace/moshmux\"\n"
	if got != expected {
		t.Errorf("marshal mismatch:\ngot:\n%s\nexpected:\n%s", got, expected)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	original := []Alias{
		{Name: "mc", Session: "minecraft", Dir: "~/workspace/minecraft"},
		{Name: "moshmux", Session: "moshmux", Dir: "~/workspace/moshmux"},
		{Name: "asm", Session: "asm", Dir: "~/workspace/mytools"},
	}

	content := MarshalAliasesToml(original)
	parsed := ParseAliasesToml(content)

	if len(parsed) != len(original) {
		t.Fatalf("roundtrip: expected %d aliases, got %d", len(original), len(parsed))
	}
	for i := range original {
		if parsed[i] != original[i] {
			t.Errorf("roundtrip[%d]: got %+v, want %+v", i, parsed[i], original[i])
		}
	}
}

func TestAddAliasToml(t *testing.T) {
	aliases := []Alias{{Name: "mc", Session: "minecraft", Dir: "~/mc"}}

	got, err := AddAliasToml(aliases, "test", "test", "~/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Name != "test" {
		t.Errorf("expected 2 aliases, got %d", len(got))
	}

	_, err = AddAliasToml(got, "mc", "mc", "~/mc2")
	if err == nil {
		t.Error("expected duplicate error")
	}
}

func TestUpdateAliasToml(t *testing.T) {
	aliases := []Alias{{Name: "mc", Session: "minecraft", Dir: "~/mc"}}

	got, err := UpdateAliasToml(aliases, "mc", "~/new-mc")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Dir != "~/new-mc" {
		t.Errorf("expected ~/new-mc, got %s", got[0].Dir)
	}

	_, err = UpdateAliasToml(aliases, "nope", "~/x")
	if err == nil {
		t.Error("expected not found error")
	}
}

func TestRemoveAliasToml(t *testing.T) {
	aliases := []Alias{
		{Name: "mc", Session: "minecraft", Dir: "~/mc"},
		{Name: "test", Session: "test", Dir: "~/test"},
	}

	got, err := RemoveAliasToml(aliases, "mc")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "test" {
		t.Errorf("expected [test], got %+v", got)
	}

	_, err = RemoveAliasToml(got, "mc")
	if err == nil {
		t.Error("expected not found error")
	}
}

func TestParseZshAndMarshalToml(t *testing.T) {
	zsh := "alias mc='mux minecraft ~/workspace/minecraft'\nalias moshmux='mux moshmux ~/workspace/moshmux'\nalias asm='mux asm ~/workspace/mytools'\n"
	aliases := ParseAliases(zsh)
	if len(aliases) != 3 {
		t.Fatalf("expected 3 zsh aliases, got %d", len(aliases))
	}

	toml := MarshalAliasesToml(aliases)
	parsed := ParseAliasesToml(toml)
	if len(parsed) != 3 {
		t.Fatalf("expected 3 toml aliases after roundtrip, got %d", len(parsed))
	}

	for i := range aliases {
		if aliases[i] != parsed[i] {
			t.Errorf("[%d] zsh=%+v toml=%+v", i, aliases[i], parsed[i])
		}
	}
}
