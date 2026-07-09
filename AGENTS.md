# ainovel-cli-fork — Project Overview

Fork of [voocel/ainovel-cli](https://github.com/voocel/ainovel-cli) — an AI-powered novel writing engine (Go).

## What This Fork Adds

A **Web UI adapter** (`internal/entry/web/`) that lets users control the engine from a browser instead of the terminal TUI. The fork strategy is **additive-only**: no upstream files are modified except `cmd/ainovel-cli/main.go` (to add `--web` flag). This keeps `git merge upstream/main` near-zero-conflict.

## Architecture (must-know)

```
cmd/ainovel-cli/main.go   ← only upstream file we touch
    ├── internal/entry/tui/    ← upstream TUI (DON'T MODIFY)
    ├── internal/entry/web/    ← OUR Web UI adapter (all new files)
    └── internal/host/         ← engine core (DON'T MODIFY)
```

- **`internal/host/Host`** is the engine. It exposes public API: `StartPrepared`, `Steer`, `CoCreateStream`, `Export`, `SwitchModel`, `Snapshot()`, etc.
- **Entry adapters** (TUI, Web, headless) are thin consumers of Host API. Dependency is one-way: `entry → host`.
- **Web adapter** files are 100% new → never conflict on merge.
- **Tiếng Việt** is UI-only (`app-i18n.js`). Engine/prompts stay in Chinese. Novel language controlled via `~/.ainovel/rules/lang-vi.md` (outside repo).

## Key References

| Document | What it covers |
|---|---|
| [01-TONG-QUAN-DU-AN.md](01-TONG-QUAN-DU-AN.md) | Tổng quan tiếng Việt: kiến trúc, flow, thể loại hỗ trợ, lưu ý sử dụng |
| [02-WEB-UI.md](02-WEB-UI.md) | **Deep dive**: Web UI architecture, file map, API endpoints, content workspace tabs, i18n strategy, upstream merge workflow |
| [03-MYNOVEL-REPORT.md](03-MYNOVEL-REPORT.md) | Phân tích nền tảng MyNovel (mynovel.net/pro): pháp lý, traffic, rủi ro, so sánh platform |
| [05-REVIEW-TRUYEN.md](05-REVIEW-TRUYEN.md) | **Playbook review truyện engine sinh ra**: đọc file này là đủ để review 1 cuốn (nền móng/prose/11 trục/can thiệp). Mở thread mới review → đọc 05 trước |
| [docs/production-cockpit.md](docs/production-cockpit.md) | Production Cockpit (tab Sản xuất): hướng dẫn dùng + [journal MVP kèm sơ đồ tương tác](docs/journals/260703-production-cockpit-mvp.md) |
| [start-web.sh](start-web.sh) | How to run the web UI locally |

## Rules for This Codebase

1. **NEVER modify** `internal/host/`, `internal/tools/`, `assets/prompts/`, `internal/entry/tui/` — these are upstream.
2. **All Web UI code** lives in `internal/entry/web/`. New files only.
3. **Only upstream touch point**: `cmd/ainovel-cli/main.go` (the `--web` flag).
4. **CSS**: use design tokens (`var(--color-*)`, `var(--space-*)`) — no hardcoded colors.
5. **JS**: vanilla, no dependencies, `'use strict'`, embed via `go:embed`.
6. After changing any asset: rebuild binary (`go build ./...`) — browser refresh alone won't pick up embedded changes.
7. Before commit: `go build ./... && go vet ./... && go test ./internal/entry/web/...`

---

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **ainovel-cli-fork** (6926 symbols, 25885 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/ainovel-cli-fork/context` | Codebase overview, check index freshness |
| `gitnexus://repo/ainovel-cli-fork/clusters` | All functional areas |
| `gitnexus://repo/ainovel-cli-fork/processes` | All execution flows |
| `gitnexus://repo/ainovel-cli-fork/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
