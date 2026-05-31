# neoclaude

A Neovim-flavored TUI (Go + Bubble Tea) that manages multiple `claude` CLI sessions as
PTY-wrapped "buffers", each rendered through a vt10x terminal emulator and blitted into a
lipgloss view.

**Status:** WIP — P0 + P1 + P2 + P3 landed.

## Completion criteria

1. Manage multiple live `claude` sessions concurrently inside one TUI.
2. Vim-like modal editing (NORMAL / INSERT / COMMAND / VISUAL / SEARCH) with a command line.
3. Buffer navigation: `:bn`/`:bp`/`:bd` plus a `<leader><leader>` fuzzy buffer picker.
4. `:new [path]` spawns a new claude session in a chosen working directory.
5. Named sessions with persistence + resume via claude's own `--session-id`/`-r` machinery.
6. In-buffer search (`/`) over visible output and scrollback with `n`/`N` navigation.
7. Live-grep (`<leader>sg`) across all open buffers, jump to buffer+line.
8. Theming: onedark builtin, GitHub `.lua` colorscheme fetch/parse, ANSI16 remap (truecolor passthrough).

## Build

```sh
go build ./cmd/neoclaude
```

## Run

```sh
go run ./cmd/neoclaude
# or after building:
./neoclaude
```

## Key bindings

Starts in **INSERT** mode — all keys go directly to the active `claude` session.

| Key | Mode | Action |
|-----|------|--------|
| `Esc` `Esc` (≤300 ms) | INSERT | → NORMAL |
| `i` / `a` / `Enter` | NORMAL | → INSERT |
| `:` | NORMAL | → COMMAND (opens command line) |
| `<leader><leader>` | NORMAL | Open fuzzy buffer picker (live buffers) |
| `<leader>sg` | NORMAL | Live-grep across all buffers |
| `<leader>sn` | NORMAL | Named-session picker (live + closed/persisted) |
| `/` | NORMAL | In-buffer incremental search |
| `n` / `N` | SEARCH | Next / previous match |
| `v` | NORMAL | → VISUAL (linewise selection) |
| `y` | VISUAL | Yank selection to clipboard, → NORMAL |
| `Esc` | SEARCH / VISUAL / PICKER | → NORMAL |
| `Ctrl-C` | NORMAL | Quit |

### Commands (`:` in NORMAL)

| Command | Effect |
|---------|--------|
| `:new [path]` | Spawn a new claude session (optionally in `path`). Tab-completes paths. Name defaults to `basename(path)`. |
| `:name <NAME>` | Rename the active buffer's neoclaude label and update `sessions.json`. Claude's own display name (set at spawn via `-n`) is unchanged. |
| `:bn` | Next buffer |
| `:bp` | Previous buffer |
| `:bd` | Kill active buffer |

## Named sessions and resume (P3)

Every buffer spawned by `:new` gets a UUID and is launched as:

```
claude --session-id <uuid> -n <name>
```

The UUID and name are written to `sessions.json` immediately so even a crash preserves the record.

**Resume a closed session:**
Double-Esc → NORMAL → `<leader>sn`. The picker shows:
- **●** Live buffers (switch to them with Enter)
- **○** Closed/persisted sessions (Enter spawns `claude --resume <uuid>` in its stored cwd — full conversation context is restored by claude)

Sessions survive process restart: `<leader>sn` on a fresh launch still lists prior named sessions.

### Sessions file

`$XDG_STATE_HOME/neoclaude/sessions.json`  
(defaults to `~/.local/state/neoclaude/sessions.json` when `XDG_STATE_HOME` is unset)

## Configuration

Config file: `~/.config/neoclaude/config.toml`  
(respects `$XDG_CONFIG_HOME`; missing file → silent defaults)

```toml
# Leader key for NORMAL-mode chords.
# Single character (e.g. ",") or keyname: "space", "comma", "backslash".
# Default: " " (space)
leader = " "

# Future options (not yet active):
# theme = "onedark"
# scrollback_lines = 10000
```

The leader key fires **only in NORMAL mode**. In INSERT, all keys (including space)
are forwarded raw to the active claude session.

## Development

```sh
go build ./...
go vet ./...
go test ./...
```
