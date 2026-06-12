// Package config loads neoclaude configuration from
// ~/.config/neoclaude/config.toml (XDG). Missing file → defaults, no error.
//
// Future fields to add here: Theme, ScrollbackLines, Keybinds.
// Keep the struct growing-friendly: add new optional fields with sane defaults.
//
// Config file location: $XDG_CONFIG_HOME/neoclaude/config.toml
// (falls back to ~/.config/neoclaude/config.toml when XDG_CONFIG_HOME is unset).
//
// Example config.toml:
//
//	leader = ","        # single char, or "space" / "comma"
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

// Config holds all user-configurable settings.
// Add new optional fields here as phases progress; always provide a default in Default().
type Config struct {
	// Leader is the normal-mode leader key (default: " " — space).
	// Accept a literal single character (e.g. ",") or one of the keynames:
	// "space" → ' ', "comma" → ',', "backslash" → '\'.
	Leader string `toml:"leader"`

	// LeaderRune is parsed from Leader and is what the FSM matches against.
	// Not written to disk; populated by Load/Default.
	LeaderRune rune `toml:"-"`

	// Theme is the name of the colour palette (default: "onedark").
	// Built-in: onedark, tokyonight-night, catppuccin-mocha, gruvbox-dark.
	Theme string `toml:"theme"`

	// TODO(P2+): ScrollbackLines int `toml:"scrollback_lines"` — default 10000
	// TODO(P1+): Keybinds map[string]string `toml:"keybinds"` — future keymap overrides
}

// Default returns a Config with all defaults applied.
func Default() *Config {
	c := &Config{Leader: " ", Theme: "onedark"}
	c.LeaderRune = ' '
	return c
}

// configPath returns the platform config file path.
func configPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "neoclaude", "config.toml")
}

// Load reads the config file. If the file does not exist, Default() is returned
// with no error. Parse errors are returned so the caller can surface them.
func Load() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}

	c := Default()
	if _, err := toml.Decode(string(data), c); err != nil {
		return Default(), err
	}
	if err := c.normalize(); err != nil {
		return Default(), err
	}
	return c, nil
}

// normalize validates and fills derived fields (LeaderRune) from string values.
func (c *Config) normalize() error {
	c.LeaderRune = parseLeader(c.Leader)
	return nil
}

// parseLeader converts a leader string to a rune. Empty string or unknown
// keyname falls back to space.
func parseLeader(s string) rune {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "", "space":
		return ' '
	case "comma":
		return ','
	case "backslash":
		return '\\'
	}
	// Accept a single literal character.
	r, size := utf8.DecodeRuneInString(s)
	if r != utf8.RuneError && size == len(s) {
		return r
	}
	return ' '
}
