package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultLeader(t *testing.T) {
	c := Default()
	if c.LeaderRune != ' ' {
		t.Errorf("default leader rune want ' ' got %q", c.LeaderRune)
	}
	if c.Leader != " " {
		t.Errorf("default Leader string want ' ' got %q", c.Leader)
	}
}

func TestMissingFileReturnsDefaults(t *testing.T) {
	// Point config path at a nonexistent dir by temporarily overriding env.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "nonexistent"))
	c, err := Load()
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if c.LeaderRune != ' ' {
		t.Errorf("want default leader got %q", c.LeaderRune)
	}
}

func TestParseLeaderLiteral(t *testing.T) {
	cases := []struct {
		in   string
		want rune
	}{
		{",", ','},
		{"\\", '\\'},
		{" ", ' '},
		{"", ' '},
		{"space", ' '},
		{"comma", ','},
		{"backslash", '\\'},
		{"SPACE", ' '},
		{"z", 'z'},
	}
	for _, tc := range cases {
		got := parseLeader(tc.in)
		if got != tc.want {
			t.Errorf("parseLeader(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgDir := filepath.Join(dir, "neoclaude")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(`leader = ","`), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if c.Leader != "," {
		t.Errorf("Leader want ',' got %q", c.Leader)
	}
	if c.LeaderRune != ',' {
		t.Errorf("LeaderRune want ',' got %q", c.LeaderRune)
	}
}

func TestLoadInvalidToml(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgDir := filepath.Join(dir, "neoclaude")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(`leader = [`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err == nil {
		t.Error("invalid toml should return error")
	}
	// Even on error, a usable default is returned.
	if c == nil {
		t.Error("Load should return a non-nil config even on error")
	}
	if c.LeaderRune != ' ' {
		t.Errorf("error fallback should give default leader, got %q", c.LeaderRune)
	}
}

func TestLeaderNormalizationEmpty(t *testing.T) {
	c := &Config{Leader: ""}
	c.normalize()
	if c.LeaderRune != ' ' {
		t.Errorf("empty leader should normalize to space, got %q", c.LeaderRune)
	}
}
