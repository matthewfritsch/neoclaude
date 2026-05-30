# Open Questions

## neoclaude implementation plan - 2026-05-30
- [ ] Module path: confirm `github.com/matthewfritsch/neoclaude` vs a publishing org — affects `go mod init` and import paths everywhere.
- [ ] `<leader>` default key: `Space` assumed — confirm vs `,` or `\`; affects mode FSM chord detection.
- [ ] onedark "darker" color-of-record: which upstream repo/commit defines the 1:1 chrome target for P4.
- [ ] vt10x library choice is spike-gated (hinshun vs ActiveState vs tcell-direct fallback) — resolved by P0 acceptance, not before.
