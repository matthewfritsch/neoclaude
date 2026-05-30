# neoclaude — Implementation Plan

A Neovim-flavored TUI (Go + Bubble Tea) that manages multiple `claude` CLI sessions as
PTY-wrapped "buffers", each rendered through a vt10x terminal emulator and blitted into a
lipgloss view.

**Status:** architecture is locked (see task brief). This document turns it into an ordered,
dependency-aware task plan. No redesign — only sequencing, concrete files, types/functions, and
per-phase acceptance criteria.

---

## 0. Environment & Verified Facts

| Item | Value | How verified |
|------|-------|--------------|
| Go toolchain | `go1.26.3` present | `go version` |
| git | `2.43.0` present | `git --version` |
| `claude` flags | `-n/--name`, `--session-id <uuid>`, `-r/--resume [value]`, `-c/--continue`, `--fork-session` all present | `claude --help` |
| Suggested module path | `github.com/matthewfritsch/neoclaude` | user email = matthewmfritsch@gmail.com |
| Min Go version in go.mod | `go 1.24` (toolchain is newer; pin a stable floor) | — |

### Library pin recommendations (current as of 2026-05)

| Module | Pin | Notes |
|--------|-----|-------|
| `github.com/charmbracelet/bubbletea` | `v1.3.10` | core loop, `tea.Program`, `program.Send` |
| `github.com/charmbracelet/lipgloss` | `v1.1.0` | chrome styling, blit target |
| `github.com/charmbracelet/bubbles` | `v1.0.0` | `textinput` (command line), `list`/`viewport` helpers. v0.21.1 is the prior tag if v1.0.0 API churn bites. |
| `github.com/creack/pty` | `v1.1.24` | `pty.Start`, `pty.Setsize` |
| `github.com/sahilm/fuzzy` | `v0.1.2` | buffer picker + session ranking |
| `github.com/yuin/gopher-lua` | `v1.1.2` | Neovim theme `.lua` parsing (P4) |
| `github.com/google/uuid` | `v1.6.0` | session-id generation |
| **vt10x** | **SPIKE-GATED** — default `github.com/hinshun/vt10x@v0.0.0-20220301184237-5011da428d02`; fallback `github.com/ActiveState/vt10x` (upstream of hinshun) | **No tagged release on either; both low-activity. Correctness of alt-screen + scroll-region + truecolor MUST be validated in P0 before committing.** This is the single highest library risk. |

**vt10x decision rule:** P0 must exercise alt-screen toggle, scroll-region (claude's spinner/streaming
output), and 256/truecolor SGR. If hinshun mis-handles any, try ActiveState/vt10x; if both fail,
trigger the P0 circuit breaker (tcell-direct buffer pane — see Risk Register R1).

---

## 1. Package Layout (locked)

```
cmd/neoclaude/main.go            entrypoint; wires tea.Program
internal/
  app/        root tea.Model: holds registry, mode, active buffer, dispatches msgs
  session/    PTY-wrapped claude child: spawn, read goroutine, write, resize, kill, session-id/name
  vt/         vt10x wrapper: feed bytes, Resize, snapshot cell grid, cursor, scrollback extraction
  buffer/     a buffer = session + vt + metadata (name, cwd, uuid)
  registry/   ordered collection of buffers; active index; bn/bp/bd ops
  mode/       Mode FSM (NORMAL/INSERT/COMMAND/VISUAL) + double-Esc pending-timer logic
  render/     blit cell grid -> lipgloss string; ANSI16 palette remap; cursor overlay; coalescing
  search/     in-process scrollback search (/), live-grep across buffers, fuzzy ranking
  ui/         statusline, command line, fuzzy picker, grep results pane
  theme/      Theme struct, onedark builtin, GitHub fetch + gopher-lua parse, ANSI16 map
  persist/    sessions.json read/write at ~/.local/state/neoclaude/sessions.json
```

### Custom Bubble Tea messages (locked)
```go
type ptyDataMsg   struct { bufID BufferID; data []byte }
type ptyExitMsg    struct { bufID BufferID; err error }
type sessionStartedMsg struct { bufID BufferID; pid int }
type themeLoadedMsg  struct { theme *theme.Theme; err error }
type grepResultMsg   struct { results []search.Hit }
```

### Core loop (locked)
per-session read goroutine → `program.Send(ptyDataMsg)` → `app.Update` writes bytes into that
buffer's `vt` → `app.View` blits the active buffer's grid. Read goroutines are started after
`tea.Program` exists so `program.Send` is valid.

---

## 2. Repo Bootstrap (do first, before any phase)

**Files:** `go.mod`, `.gitignore`, `README.md`, `cmd/neoclaude/main.go` (stub)

Tasks:
1. `git init` in `/home/matthew/Programming/NeoClaude`.
2. `go mod init github.com/matthewfritsch/neoclaude` (confirm module path with user if it will be published elsewhere).
3. Add `go 1.24` line; run `go mod tidy` after first deps added.
4. `.gitignore`: `/neoclaude`, `*.test`, `dist/`, `.DS_Store`, `*.log`, `coverage.out`.
5. `README.md` stub: one-paragraph what/why, build (`go build ./cmd/neoclaude`), run (`./neoclaude`), status = WIP.
6. `cmd/neoclaude/main.go` stub that prints version and exits 0 (replaced in P0).

**Acceptance:** `go build ./...` succeeds; `git log` has one bootstrap commit; `./neoclaude` runs.

---

## P0 — SPIKE (highest risk, build first)

**Goal:** prove a real `claude` session is fully usable inside Bubble Tea. One hardcoded child,
PTY-wrapped, vt10x grid blitted into the view, INSERT keystrokes forwarded, resize correct, cursor
visible. Standalone runnable.

**Files:**
- `internal/session/session.go` — minimal: `Start`, read goroutine, `Write`, `Resize`, `Kill`.
- `internal/vt/vt.go` — wrap vt10x: `New(cols,rows)`, `Write([]byte)`, `Resize`, `Snapshot() Grid`, `Cursor() (x,y,visible)`.
- `internal/render/blit.go` — `Blit(Grid, cursor) string` (truecolor passthrough; no remap yet).
- `cmd/neoclaude/main.go` — wire one session + vt + `tea.Program` with `tea.WithAltScreen()`.

**Types/funcs to create:**
```go
// session
type Session struct { cmd *exec.Cmd; ptmx *os.File; ... }
func Start(argv []string, cols, rows uint16) (*Session, error)   // exec.Command("claude", ...); pty.Start
func (s *Session) ReadLoop(send func(ptyDataMsg))                // goroutine; emits ptyDataMsg/ptyExitMsg
func (s *Session) Write(p []byte) error
func (s *Session) Resize(cols, rows uint16) error                // pty.Setsize
func (s *Session) Kill() error

// vt
type Grid struct { Cells [][]Cell; Cols, Rows int }
type Cell  struct { Rune rune; FG, BG Color; Attrs Attr }
func New(cols, rows int) *VT
func (v *VT) Write(p []byte)
func (v *VT) Resize(cols, rows int)
func (v *VT) Snapshot() Grid
func (v *VT) Cursor() (x, y int, visible bool)
```

**Update/View contract:** on `tea.WindowSizeMsg` call `vt.Resize` AND `session.Resize` (both, in that
order). On `ptyDataMsg` write into vt and request re-render. In NORMAL-less spike mode all key
events forward raw to `session.Write` except `Ctrl+Q`/`q`-while-not-typing to quit (keep it crude;
FSM lands in P1).

**P0 acceptance (manual, MUST validate all):**
1. `go run ./cmd/neoclaude` opens a single live `claude` session, full-screen, usable.
2. Typing a prompt and getting a streamed response renders correctly (validates **scroll-region** + spinner repaint).
3. Resizing the terminal reflows claude's UI without corruption (validates `vt.Resize` + `pty.Setsize`).
4. Cursor appears at the correct cell and blinks/positions with claude's input.
5. Colors render (claude uses 256/truecolor) — validates SGR handling and truecolor passthrough.
6. `q`/Ctrl+Q tears down: child killed, terminal restored, no orphan PID (`pgrep claude`).

**Circuit breaker:** 3 attempts to make rendering smooth. Failure conditions: persistent corruption
on stream/resize, unfixable color loss, or flicker that coalescing can't solve. On breaker →
escalate R1 (evaluate tcell-direct buffer pane embedded in the bubbletea frame).

---

## P1 — MVP: Registry, Mode FSM, multi-buffer, command line
**Criteria covered:** 1, 2, 3 (partial: bn/bp), 4.

**Files:**
- `internal/buffer/buffer.go` — `Buffer{ ID, Name, Cwd, UUID, *session.Session, *vt.VT }`.
- `internal/registry/registry.go` — ordered `[]*Buffer`, `Active`, `Next/Prev/Add/Delete/ByID`.
- `internal/mode/mode.go` — FSM + double-Esc timer.
- `internal/ui/statusline.go` — mode indicator, buffer name, index `[2/5]`, cwd.
- `internal/ui/cmdline.go` — `:`-command input (wraps bubbles `textinput`); path completion for `:new`.
- `internal/app/app.go` — promote spike `main` into root model holding the registry + mode + cmdline.
- `internal/app/commands.go` — parse/dispatch `:new`, `:bn`, `:bp`, `:bd`.

**Types/funcs:**
```go
// mode
type Mode int // Normal, Insert, Command, Visual
type FSM struct { mode Mode; escPending bool; escAt time.Time }
func (f *FSM) HandleKey(k tea.KeyMsg, now time.Time) (Action, Mode)
// double-Esc: in Insert, first Esc sets escPending+escAt and FORWARDS Esc to child;
// second Esc within escDelay(~300ms) -> switch to Normal, swallow. Timeout clears pending.
const escDelay = 300 * time.Millisecond

// registry
type BufferID int
func (r *Registry) Add(b *Buffer); func (r *Registry) Delete(id BufferID) error
func (r *Registry) Next(); func (r *Registry) Prev(); func (r *Registry) Active() *Buffer

// commands
func (a *App) cmdNew(path string) tea.Cmd      // mkdir-check, spawn session (with --session-id/-n in P3), add buffer
func completePath(prefix string) []string       // filesystem dir/file completion for :new
```

**Behavior:**
- In Insert mode all keys (except the double-Esc detector) forward to active buffer's session.
- In Normal mode keys are command bindings; `i`/`a` → Insert; `:` → Command.
- `:new [path]` spawns a new buffer (cwd = path or `.`); switches to it. Path completion via `<Tab>`.
- `:bn`/`:bp` cycle active buffer; statusline updates.
- `:bd` kills active session, removes buffer, falls back to previous (or empty state).

**P1 acceptance:**
1. Open 3 buffers via `:new`, `:new ~/x`, `:new /tmp`; each is a live claude in its cwd.
2. `:bn`/`:bp` switch the rendered buffer and the statusline `[n/N]`.
3. Single `Esc` while typing reaches claude (claude reacts); double-Esc (<300ms) returns to Normal without claude seeing the second Esc.
4. `:bd` kills the active session (verify with `pgrep`), removes it, switches to a neighbor.
5. `:new ./sr<Tab>` completes a path.
- Unit tests: FSM transitions incl. double-Esc timing boundary; registry add/delete/next/prev ordering; `completePath`.

---

## P2 — Picker, in-buffer search, live-grep, scrollback
**Criteria covered:** 3 (fuzzy picker), 6, 7.

**Files:**
- `internal/vt/scrollback.go` — ring buffer of evicted lines; `ExtractText() []string` (visible grid + scrollback).
- `internal/search/search.go` — `SearchBuffer(lines, query) []Hit`; `Grep(buffers, query) []Hit`.
- `internal/search/fuzzy.go` — wrap sahilm/fuzzy for picker + session ranking.
- `internal/ui/picker.go` — `<leader><leader>` fuzzy buffer picker overlay.
- `internal/ui/searchbar.go` — `/` in-buffer search; match highlight; `n`/`N` navigation; Visual selection (`v`) over cells.
- `internal/ui/greppane.go` — `<leader>sg` results list across all buffers; Enter jumps to buffer+line.

**Types/funcs:**
```go
type Hit struct { BufID BufferID; Line int; Col int; Text string }
func (v *VT) ExtractText() []string
type Ring struct { lines []string; cap, head int }
func SearchBuffer(lines []string, q string) []Hit
func Grep(bufs []*buffer.Buffer, q string) tea.Cmd   // async -> grepResultMsg
```

**Leader handling:** add `<leader>` (default `Space`) chord detection in mode FSM: Normal-mode
pending-leader state, similar to escPending. `<leader><leader>` → picker; `<leader>sg` → grep.

**P2 acceptance:**
1. `<leader><leader>` opens picker; typing fuzzy-filters buffer names; Enter switches.
2. `/foo` in a buffer highlights matches; `n`/`N` cycle; `v` selects a region (highlight visible).
3. `<leader>sg bar` lists hits across all buffers; Enter jumps to the right buffer + scrolls to line.
4. Scrollback retained after output scrolls off (search finds off-screen text).
- Unit tests: `ExtractText` against a scripted vt byte stream; `SearchBuffer` hit offsets; fuzzy ranking order; ring eviction.

---

## P3 — Named sessions, persistence, resume (leverage claude's own machinery)
**Criteria covered:** 5.

**Files:**
- `internal/persist/persist.go` — `~/.local/state/neoclaude/sessions.json` load/save (atomic write via temp+rename).
- `internal/session/session.go` — extend `Start` to pass `--session-id <uuid>` and `-n <name>`; add `Resume(uuid)` using `-r <uuid>`.
- `internal/app/commands.go` — `:rename <name>` (re-launch/relabel), `:new --name <n>`; `<leader>sn` session picker over closed named sessions.

**Types/funcs:**
```go
type Record struct { UUID string `json:"uuid"`; Name string `json:"name"`; Cwd string `json:"cwd"`; LastSeen time.Time `json:"last_seen"` }
type Store struct { path string; Records []Record }
func Load() (*Store, error); func (s *Store) Save() error
func (s *Store) Upsert(r Record); func (s *Store) Closed(openUUIDs map[string]bool) []Record

// session
func Start(opts Opts) (*Session, error) // Opts{ UUID, Name, Cwd } -> claude --session-id <uuid> -n <name>
func Resume(uuid string, cols, rows uint16) (*Session, error) // claude -r <uuid>
```

**Flow:** on `:new`, neoclaude generates `uuid.NewString()`, spawns
`claude --session-id <uuid> -n <name>`, and `Upsert`s `{uuid,name,cwd}`. `<leader>sn` lists records
not currently open and re-opens via `Resume` (real context restore).

**P3 acceptance:**
1. `:new --name work` creates a buffer; `sessions.json` gains `{uuid,name:"work",cwd}`.
2. `:bd` the buffer, then `<leader>sn` lists "work" as a closed session.
3. Selecting it runs `claude -r <uuid>` and the prior conversation context is restored (scroll up to confirm earlier turns).
4. Restart neoclaude → `<leader>sn` still lists prior named sessions (persistence survives process exit).
- Unit tests: persist round-trip (Load/Save), atomic write, `Closed` filtering, uuid validity.

---

## P4 — Theming: onedark builtin → GitHub fetch + lua parse → ANSI16 remap
**Criteria covered:** 8.

**Files:**
- `internal/theme/theme.go` — `Theme` struct (chrome colors + ANSI16 palette).
- `internal/theme/onedark.go` — bundled onedark **darker** builtin (chrome exact, load first).
- `internal/theme/fetch.go` — GitHub raw fetch of a Neovim colorscheme `.lua`.
- `internal/theme/lua.go` — gopher-lua sandbox: run the colorscheme, read `highlight`/`vim.api.nvim_set_hl` calls into a color map.
- `internal/render/remap.go` — map child ANSI16 indices to active theme palette during blit; truecolor passes through untouched.

**Types/funcs:**
```go
type Theme struct { Chrome ChromeColors; ANSI16 [16]Color; Name string }
func Onedark() *Theme
func FetchTheme(githubRawURL string) tea.Cmd      // -> themeLoadedMsg
func ParseLua(src []byte) (*Theme, error)          // sandboxed gopher-lua
func Remap(c Color, t *Theme) Color                // ANSI16-index -> theme; truecolor untouched
```

**P4 acceptance:**
1. Default chrome matches onedark darker exactly (statusline/borders/cmdline) — visual check.
2. `:theme <github-url>` fetches a `.lua` colorscheme, parses it, applies chrome 1:1.
3. ANSI16 child output recolors to the active palette; truecolor claude output is unchanged.
- Unit tests: `ParseLua` against fixture `.lua` files (assert specific extracted colors); `Remap` table correctness; gopher-lua sandbox rejects filesystem/os access.

---

## P5 — Polish
- Mouse passthrough into the active child (forward SGR mouse to PTY).
- OSC handling (title/clipboard) surfaced or safely swallowed.
- Render coalescing: batch rapid `ptyDataMsg` within a frame tick to cut redraws.
- Theme hot-swap without restart.

**Acceptance (manual):** mouse works inside claude; no flicker under heavy streaming; `:theme` swaps live.

---

## 3. Risk Register & Circuit Breakers

| ID | Risk | Likelihood | Trigger / Breaker | Mitigation |
|----|------|-----------|-------------------|------------|
| R1 | **vt10x correctness** (alt-screen, scroll-region, truecolor) — unmaintained, untagged | High | P0: 3 attempts to get smooth render; corruption on stream/resize that coalescing can't fix | Fallback to `ActiveState/vt10x`; if both fail, embed a tcell-direct buffer pane inside the bubbletea frame |
| R2 | bubbles `v1.0.0` API churn vs v0.21.x | Med | build/API breakage on `textinput`/`list` | Pin v0.21.1 as fallback; isolate bubbles use behind `internal/ui` |
| R3 | claude flag semantics drift (`--session-id`, `-r`) | Low | resume restores wrong/empty context in P3 | Flags verified now; pin behavior in P3 acceptance; degrade `<leader>sn` to name-only relaunch |
| R4 | PTY orphan children on crash | Med | `pgrep claude` shows orphans after quit | `defer Kill()`; process-group kill; teardown test in P0/P1 |
| R5 | Double-Esc timing feels wrong (300ms) | Low | user reports laggy/eager Normal switch | make `escDelay` configurable; FSM unit-tested at boundary |
| R6 | gopher-lua can't run real Neovim colorschemes (they call `vim.*` APIs) | Med | P4 parse fails on real-world `.lua` | Provide a `vim` shim table in the sandbox; fall back to builtin themes; document supported subset |
| R7 | Render perf under streaming | Med | flicker/CPU spike | P5 coalescing; only re-blit on dirty + frame tick |

---

## 4. Test Strategy

**Unit-testable (prioritize):**
- `mode.FSM` transitions incl. double-Esc and leader-chord timing boundaries.
- `vt.ExtractText` / scrollback ring against scripted byte streams.
- `search.SearchBuffer` hit offsets; `search.fuzzy` ranking order.
- `theme.ParseLua` against fixture `.lua` files; `render.Remap` table; lua sandbox denies os/io.
- `persist` round-trip, atomic write, `Closed` filtering, uuid validity.
- `registry` add/delete/next/prev ordering; `completePath`.

**Manual-TUI (documented run scripts per phase):** rendering correctness, resize reflow, color
fidelity, cursor placement, resume context restore, theme application, mouse passthrough. Each phase
above lists explicit manual steps.

**CI suggestion:** `go vet ./...`, `go test ./...`, `gofmt -l`. TUI manual checks gated by a
`MANUAL.md` checklist mirroring each phase's acceptance list.

---

## 5. Dependency Order (execute top-to-bottom)

```
Bootstrap ──► P0 SPIKE ──► P1 MVP ──► P2 search/picker ──► P3 sessions ──► P4 theme ──► P5 polish
                 │
                 └─ gates vt10x choice (R1) before any other phase commits to it
```

P3 depends on P1 (registry/buffer). P2 depends on P1 (multi-buffer + vt scrollback). P4 depends on
P0 blit + P1 render path. Nothing after P0 starts until P0 acceptance passes.

---

## 6. Open Questions
- Module path: confirm `github.com/matthewfritsch/neoclaude` (or a different org for publishing).
- `<leader>` default: `Space` assumed — confirm vs `,` or `\`.
- onedark "darker" exact source: which upstream repo/commit is the color-of-record for 1:1 chrome.
