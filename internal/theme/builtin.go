package theme

var builtinOrder = []string{
	"onedark",
	"tokyonight-night",
	"catppuccin-mocha",
	"gruvbox-dark",
}

var builtins = map[string]*Palette{
	"onedark":          onedark(),
	"tokyonight-night": tokyonight(),
	"catppuccin-mocha": catppuccin(),
	"gruvbox-dark":     gruvbox(),
}

// Get returns the named built-in palette, or nil if not found.
func Get(name string) *Palette {
	return builtins[name]
}

// List returns the names of all built-in palettes in display order.
func List() []string {
	out := make([]string, len(builtinOrder))
	copy(out, builtinOrder)
	return out
}

// Default returns the default palette (onedark).
func Default() *Palette {
	return Get("onedark")
}

func onedark() *Palette {
	return &Palette{
		Name:      "onedark",
		Bg:        "#282c34",
		Fg:        "#abb2bf",
		Accent:    "#61afef",
		Muted:     "#5c6370",
		Selection: "#3e4451",
		Border:    "#3e4451",
		Match:     "#e5c07b",
		ANSI16: [16]string{
			"#282c34", "#e06c75", "#98c379", "#e5c07b",
			"#61afef", "#c678dd", "#56b6c2", "#abb2bf",
			"#5c6370", "#e06c75", "#98c379", "#e5c07b",
			"#61afef", "#c678dd", "#56b6c2", "#ffffff",
		},
	}
}

func tokyonight() *Palette {
	return &Palette{
		Name:      "tokyonight-night",
		Bg:        "#1a1b26",
		Fg:        "#c0caf5",
		Accent:    "#7aa2f7",
		Muted:     "#565f89",
		Selection: "#283457",
		Border:    "#27a1b9",
		Match:     "#e0af68",
		ANSI16: [16]string{
			"#15161e", "#f7768e", "#9ece6a", "#e0af68",
			"#7aa2f7", "#bb9af7", "#7dcfff", "#a9b1d6",
			"#414868", "#f7768e", "#9ece6a", "#e0af68",
			"#7aa2f7", "#bb9af7", "#7dcfff", "#c0caf5",
		},
	}
}

func catppuccin() *Palette {
	return &Palette{
		Name:      "catppuccin-mocha",
		Bg:        "#1e1e2e",
		Fg:        "#cdd6f4",
		Accent:    "#89b4fa",
		Muted:     "#6c7086",
		Selection: "#45475a",
		Border:    "#585b70",
		Match:     "#f9e2af",
		ANSI16: [16]string{
			"#45475a", "#f38ba8", "#a6e3a1", "#f9e2af",
			"#89b4fa", "#f5c2e7", "#94e2d5", "#bac2de",
			"#585b70", "#f38ba8", "#a6e3a1", "#f9e2af",
			"#89b4fa", "#f5c2e7", "#94e2d5", "#cdd6f4",
		},
	}
}

func gruvbox() *Palette {
	return &Palette{
		Name:      "gruvbox-dark",
		Bg:        "#282828",
		Fg:        "#ebdbb2",
		Accent:    "#83a598",
		Muted:     "#928374",
		Selection: "#3c3836",
		Border:    "#504945",
		Match:     "#fabd2f",
		ANSI16: [16]string{
			"#282828", "#cc241d", "#98971a", "#d79921",
			"#458588", "#b16286", "#689d6a", "#a89984",
			"#928374", "#fb4934", "#b8bb26", "#fabd2f",
			"#83a598", "#d3869b", "#8ec07c", "#ebdbb2",
		},
	}
}
