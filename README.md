# neoclaude

A Neovim-flavored TUI (Go + Bubble Tea) that manages multiple `claude` CLI sessions as
PTY-wrapped "buffers", each rendered through a vt10x terminal emulator and blitted into a
lipgloss view.

**Status:** WIP. Bootstrap + P0 SPIKE landed.

## Completion criteria

1. Manage multiple live `claude` sessions concurrently inside one TUI.
2. Vim-like modal editing (NORMAL / INSERT / COMMAND / VISUAL) with a command line.
3. Buffer navigation: `:bn`/`:bp`/`:bd` plus a `<leader><leader>` fuzzy buffer picker.
4. `:new [path]` spawns a new claude session in a chosen working directory.
5. Named sessions with persistence + resume via claude's own `--session-id`/`-r` machinery.
6. In-buffer search (`/`) over visible output and scrollback.
7. Live-grep (`<leader>sg`) across all open buffers, jump to buffer+line.
8. Theming: onedark builtin, GitHub `.lua` colorscheme fetch/parse, ANSI16 remap (truecolor passthrough).

## Build

```sh
go build ./cmd/neoclaude
```

## Run

```sh
go run ./cmd/neoclaude
# or
./neoclaude
```

In the current P0 spike: launches a single hardcoded `claude` session in `$PWD`, full-screen.
Press `Esc` twice quickly (or `Ctrl-C`) to quit.

## Development

```sh
go build ./...
go vet ./...
go test ./...
```
