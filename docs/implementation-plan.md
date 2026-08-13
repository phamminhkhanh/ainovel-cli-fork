# UI v4 — Implementation Plan (v2)

> **Spec:** `docs/ascii-prototypes/UI-SPEC-FINAL.md`
> **Target:** Phase 1-4 implementation
> **Updated:** 2026-08-13 based on 2 QA reviews

---

## 📋 OVERVIEW

| Phase | Content | Duration | Priority | Dependencies |
|-------|---------|----------|----------|--------------|
| **P0: Verification** | Audit existing code + API endpoints | 0.5 days | Must Do | None |
| **P1: Layout Infrastructure** | 9 tabs shell, collapsible sidebar, state toolbar | 2-3 days | Must Have | P0 |
| **P2: Tab Content — Core** | Stream, Chapters, Outline, World, Review, System Radar | 3-4 days | Must Have | P1.1 |
| **P3: Production Cockpit** | Jobs, Foundation Gate, Profile Library, Profile Studio | 2-3 days | Must Have | P1.1 |
| **P4: Advanced** | Market Radar, Cocreate modal, Import/Simulate/Diag | 1-2 days | Should Have | P1.1 |

---

## 🔴 PHASE 0: Verification (Pre-work)

### P0.1: Existing Files Inventory
**Files:** All in `internal/entry/web/assets/`

| File | Exists? | Status |
|------|----------|--------|
| `app.js` | ✅ | Tab switch logic + SSE handlers |
| `app.css` | ✅ | Full stylesheet with design tokens |
| `app-dashboard.js` | ✅ | Snapshot rendering |
| `app-workspace.js` | ✅ | Outline/World/Review tab content |
| `app-chapters.js` | ✅ | Chapter reader |
| `app-production.js` | ✅ | Production Jobs + Foundation Gate |
| `app-studio.js` | ✅ | Profile Studio |
| `app-radar.js` | ✅ | Market Radar |
| `app-input.js` | ✅ | Input bar + cocreate |
| `app-i18n.js` | ✅ | i18n labels |

### P0.2: API Endpoint Verification
**Source:** `internal/entry/web/server.go`

| Endpoint | Method | Handler | Status |
|----------|--------|---------|--------|
| `/api/diag` | GET | `handleDiag` | ✅ Exists |
| `/api/import` | POST | `handleImport` | ✅ Exists |
| `/api/simulate` | POST | `handleSimulate` | ✅ Exists |
| `/api/importsim` | POST | `handleImportSim` | ✅ Exists |
| `/api/reveal` | POST | `handleReveal` | ✅ Exists |
| `/api/cocreate/send` | POST | `handleCoCreateSend` | ✅ Exists |
| `/api/cocreate/pause` | POST | `handleCoCreatePause` | ✅ Exists |
| `/api/cocreate/resume` | POST | `handleCoCreateResume` | ✅ Exists |
| `/api/cocreate/cancel` | POST | `handleCoCreateCancel` | ✅ Exists |
| `/api/radar/latest` | GET | `handleRadarLatest` | ✅ Exists |
| `/api/radar/scan` | POST | `handleRadarScan` | ✅ Exists |
| `/api/prodruns/{id}/ide-bundle` | GET | `handleProdRunIDEBundle` | ✅ Exists |
| `/api/profiles/generate` | POST | `handleProfileGenerate` | ✅ SSE stream |

### P0.3: Existing State Machine
**Source:** `app.js` + `02-WEB-UI.md`

```
RuntimeState: idle | running | paused | error | cocreating
Phase: init | outline | writing | editing | complete | paused
```

**Full State Matrix:**

| State | Phase | Toolbar Actions | Special UI |
|-------|-------|-----------------|------------|
| NO_BOOK | init | `[Bắt đầu]` | Welcome card |
| IDLE | outline/writing/editing | `[Tiếp tục] [Nhập] [Viết tiếp] [Xuất]` | — |
| RUNNING | — | `[Dừng] [Tạm dừng] [Can thiệp]` | Steering input active |
| REVIEW_MODE | — | `[Dừng] [Can thiệp]` + `[▸ Chương kế]` highlighted | Review hold card |
| STEERING | — | `[Dừng]` | Decision preview card |
| COCREATE | — | `[Dừng] [Chấp nhận Draft] [Hủy]` | Chat interface |
| FOUNDATION_GATE | — | `[Foundation cần duyệt]` | Gate card |
| COMPLETE | complete | `[Xuất TXT] [Xuất EPUB] [Viết tiếp] [Mới]` | — |
| ERROR | — | `[Thử lại] [Mở thư mục]` | Error card |

### P0.4: SSE Event Types
**Source:** `app.js:19-29`

```javascript
case 'hello':     // connection established
case 'stream':    // draft text chunk
case 'clear':     // round boundary
case 'event':     // structured event
case 'snapshot':   // full state snapshot
case 'cocreate':  // cocreate progress (→ app-studio.js)
case 'job':       // production job event (→ app-studio.js)
case 'done':      // round complete
```

---

## 🟠 PHASE 1: Layout Infrastructure

### P1.1: 9 Tabs Shell
**Files:** `index.html`, `app.js`, `app-i18n.js`

**Current → Target:**
```
Current:  [Stream][Chương][Outline][World][Đánh giá][Radar][Sản xuất][Hỗ trợ]
Target:   [Stream][Chương][Outline][World][Review][⚡System][📡Market][Sản xuất][Hỗ trợ]
```

**Tab ID Mapping:**

| Display | Tab ID | Panel ID |
|---------|--------|----------|
| Stream | `tab-stream` | `#tab-stream` ✅ |
| Chương | `tab-chapters` | `#tab-chapters` ✅ |
| Outline | `tab-outline` | `#tab-outline` ✅ |
| World | `tab-world` | `#tab-world` ✅ |
| Review | `tab-review` | `#tab-review` (rename from `tab-review`) |
| ⚡System | `tab-system` | `#tab-system` (rename from `tab-radar`) |
| 📡Market | `tab-market` | `#tab-market` (NEW) |
| Sản xuất | `tab-production` | `#tab-production` ✅ |
| Hỗ trợ | `tab-help` | `#tab-guide` ✅ |

**Changes:**
- [ ] Rename tab button "Đánh giá" → "Review"
- [ ] Rename tab button "Radar" → "⚡System"
- [ ] Add new tab button "📡Market" after System
- [ ] Update `app.js` tab routing for new tab IDs
- [ ] Update `app-i18n.js` labels

**Files to edit:**
- `index.html` (tab buttons, tab panels)
- `app.js` (tab switch logic, line ~300+)
- `app-i18n.js` (labels)

### P1.2: Collapsible Sidebar
**Files:** `index.html`, `app.css`, `app.js`

**Current:** Sidebar width 420px (from `--sidebar-w: 420px` in CSS)

**Target:**
- Sidebar toggle button in header (`#sidebarToggle`)
- Collapsed: width 0, overflow hidden
- Floating toggle button appears when collapsed
- CSS transition: 200ms ease-out (width + padding)
- State saved to `localStorage`

**Changes:**
- [ ] Add `#sidebarToggle` button to `.brand` section in header
- [ ] Add `.sidebar.collapsed` CSS class
- [ ] Add `.sidebar.collapsed + .main` layout adjustment
- [ ] Add floating toggle button (`#floatingToggle`)
- [ ] Add `localStorage` persistence
- [ ] Add `sessionStorage` for open tab memory

**CSS changes:**
```css
.sidebar {
  transition: width 0.2s ease-out, padding 0.2s ease-out;
}
.sidebar.collapsed {
  width: 0 !important;
  padding: 0 !important;
  overflow: hidden;
  border-right: none;
}
.floating-toggle {
  position: fixed;
  right: 16px;
  bottom: 80px;
  z-index: 100;
  /* visibility controlled by .sidebar.collapsed sibling */
}
```

### P1.3: Status Bar (Sidebar Top)
**Files:** `index.html`, `app.css`, `app-dashboard.js`

**Location:** TOP of sidebar (inside `.sidebar`), NOT in header.

**Format:** `Engine: Drafting · Ch.23/100 · Health: 87% · $2.34 · 45% ctx`

**Changes:**
- [ ] Create `#statusBar` element in sidebar (below `.brand`)
- [ ] Format: `Engine: {state} · Ch.{n}/{total} · Health: {pct}% · ${cost} · {ctx}% ctx`
- [ ] Click `#statusBar` → expand sidebar (toggle)
- [ ] Update via SSE `snapshot` event

### P1.4: State-Based Toolbar
**Files:** `app.js`, `app-input.js`, `app.css`

**Implementation:** Create `getToolbarActions(state, phase)` function.

**State → Actions Mapping:**

```javascript
function getToolbarActions(state, phase) {
  const actions = [];
  switch (state) {
    case 'idle':
      if (phase === 'init') {
        actions.push({ id: 'start', label: 'Bắt đầu', primary: true });
      } else {
        actions.push({ id: 'continue', label: 'Tiếp tục' });
        actions.push({ id: 'import', label: 'Nhập truyện' });
        actions.push({ id: 'reopen', label: 'Viết tiếp' });
        actions.push({ id: 'export', label: 'Xuất' });
      }
      break;
    case 'running':
      actions.push({ id: 'abort', label: 'Dừng', danger: true });
      actions.push({ id: 'pause', label: 'Tạm dừng' });
      break;
    case 'cocreating':
      actions.push({ id: 'abort', label: 'Dừng', danger: true });
      actions.push({ id: 'acceptDraft', label: 'Chấp nhận Draft' });
      actions.push({ id: 'cancelCoCreate', label: 'Hủy' });
      break;
    case 'error':
      actions.push({ id: 'retry', label: 'Thử lại' });
      actions.push({ id: 'reveal', label: 'Mở thư mục' });
      break;
  }
  if (phase === 'complete') {
    actions.push({ id: 'exportTxt', label: 'Xuất TXT' });
    actions.push({ id: 'exportEpub', label: 'Xuất EPUB' });
    actions.push({ id: 'reopen', label: 'Viết tiếp' });
    actions.push({ id: 'new', label: 'Truyện mới' });
  }
  return actions;
}
```

**Changes:**
- [ ] Create `getToolbarActions()` function
- [ ] Add state-specific cards (STEERING decision, COCREATE chat, FOUNDATION_GATE)
- [ ] Wire up to `renderSnapshot()` in `app.js`
- [ ] Add state classes to `#toolbar` for CSS styling

### P1.5: Keyboard Shortcuts
**Files:** `app.js`

**Shortcuts (only fire when input NOT focused):**

| Key | Action | Condition |
|-----|--------|-----------|
| `S` | Stream tab | Input unfocused |
| `C` | Chapters tab | Input unfocused |
| `O` | Outline tab | Input unfocused |
| `W` | World tab | Input unfocused |
| `R` | Review tab | Input unfocused |
| `D` | System Radar tab | Input unfocused |
| `M` | Market Radar tab | Input unfocused |
| `P` | Production tab | Input unfocused |
| `H` | Help tab | Input unfocused |
| `/` | Command palette | Any |
| `Esc` | Close modal | Modal open |
| `Ctrl+1-9` | Direct tab (optional) | Any |

**Implementation:**
```javascript
document.addEventListener('keydown', (e) => {
  // Skip if typing in input/textarea
  const tag = document.activeElement?.tagName;
  const isTyping = tag === 'INPUT' || tag === 'TEXTAREA';
  
  if (!isTyping) {
    const shortcuts = { s:'stream', c:'chapters', o:'outline', w:'world', r:'review', d:'system', m:'market', p:'production', h:'help' };
    if (shortcuts[e.key.toLowerCase()]) {
      switchTab(shortcuts[e.key.toLowerCase()]);
    }
  }
  
  if (e.key === '/') { e.preventDefault(); openCommandPalette(); }
  if (e.key === 'Escape') { closeModal(); }
});
```

**Changes:**
- [ ] Add global keydown listener
- [ ] Add focus check before firing shortcuts
- [ ] Show shortcut hints in tab tooltips

### P1.6: Event Log Collapsible
**Files:** `app.css`, `app.js`

**Changes:**
- [ ] Add `.log-pane.collapsed` CSS class
- [ ] Add toggle button in log header
- [ ] Remember collapsed state in `sessionStorage`

---

## 🟡 PHASE 2: Tab Content — Core

### P2.1: Stream Tab Enhancement
**Files:** `app.js`, `app-workspace.js`, `app.css`

**Changes:**
- [ ] Add `#pinStream` button (prevent auto-scroll)
- [ ] Update token usage display in `#streamStats`
- [ ] Add Quick Stats bar at bottom: `Ch.{n}/{total} · {words} words · ${cost} · {ctx}% ctx`

### P2.2: Chapters Tab — Full Reader
**Files:** `app-chapters.js`, `app.css`, `index.html`

**Changes:**
- [ ] Add filter dropdown (All / Final / Draft)
- [ ] Add sort dropdown (# / Name / Date)
- [ ] Add 2-column layout (list left, content right)
- [ ] Add prev/next chapter navigation
- [ ] Add "View Final" / "View Draft" toggle
- [ ] **NOTE:** "Edit in IDE" → refers to `GET /api/prodruns/{id}/ide-bundle` for Production Jobs, NOT chapter reader. Remove from plan.

### P2.3: Outline Tab Enhancement
**Files:** `app-workspace.js`, `app.css`

**API Response:** `GET /api/outline` returns:
```json
{
  "premise": "...",
  "outline": [...],
  "layered": { "volumes": [...] },
  "compass": {
    "final_goal": "...",
    "active_arc": "...",
    "progress": 77,
    "current": "..."
  }
}
```

**Changes:**
- [ ] Add collapsible Premise section
- [ ] Add Compass panel (Final Goal, Active Arc, Progress %)
- [ ] Add Layered Outline view (Volume → Arc → Chapter)
- [ ] Add Arc/Chapter expand/collapse
- [ ] Add legend (✓ Done, ⏳ Current, 🎯 Active Arc, 📋 Planned)

### P2.4: World Tab Enhancement
**Files:** `app-workspace.js`, `app.css`

**API:** `GET /api/characters` returns characters with `want`, `wound`, `hp` fields.

**Changes:**
- [ ] Add sub-tabs: Characters | Foreshadow | Rules
- [ ] Add character cards with Want/Wound/HP
- [ ] Add Foreshadow section (Planted/Advanced/Resolved)
- [ ] **NOTE:** "Add Foreshadow" button → API is read-only (`GET /api/foreshadow`). Button is backlog/placeholder only. Mark as `[BACKLOG]`.

### P2.5: Review Tab Enhancement
**Files:** `app-workspace.js`, `app.css`

**Changes:**
- [ ] Add filter by Chapter/Arc
- [ ] Add grid view of all chapters
- [ ] Add score bars (progress bars with %)
- [ ] Add findings list (warnings, approved)
- [ ] Add "View Full Review" link

### P2.6: System Radar Tab
**Files:** `app-workspace.js`, `app.css`

**Note:** System Radar is NOT in sidebar currently — it's a placeholder tab. Move content from sidebar to this tab.

**Sidebar stays:**
- Status bar (compact)
- Chapter list
- Quick tools

**Changes:**
- [ ] Rename `#tab-radar` → `#tab-system`
- [ ] Add Health gauge (87% with status)
- [ ] Add Agents panel with status icons
- [ ] Add Costs panel with progress
- [ ] Add Recent Events log

---

## 🟢 PHASE 3: Production Cockpit

### P3.1: Production Tab — Jobs List
**Files:** `app-production.js`, `app.css`

**Changes:**
- [ ] Add "Active Jobs" section
- [ ] Add job cards with health strip
- [ ] Add job actions (Pause/Stop/Reveal/Export)
- [ ] Add "Completed Jobs" section (last 5)
- [ ] Add **separate** "Foundation Gate" section (not nested in Jobs)

### P3.2: Foundation Gate Modal
**Files:** `app-production.js`, `app.css`, `index.html`

**Changes:**
- [ ] Create `#foundationGateModal` in index.html
- [ ] Add Premise section (editable)
- [ ] Add Outline section (collapsible)
- [ ] Add Characters & World section
- [ ] Add 11-axis checklist with checkboxes (state in localStorage)
- [ ] Add action buttons (Approve/Revise/Reject/IDE Bundle)
- [ ] Wire up to `/api/prodruns/{id}/approve|revise|reject`
- [ ] Wire up to `/api/prodruns/{id}/ide-bundle`

**Checklist State:** Stored in `localStorage` per job ID. Auto-save on change.

### P3.3: Profile Library
**Files:** `app-production.js`, `app-studio.js`, `app.css`

**Changes:**
- [ ] Add "My Profiles" section
- [ ] Add profile cards with last run info
- [ ] Add actions: Run/Edit/Copy/Duplicate/Delete
- [ ] Add [+ New Profile] button → opens Profile Studio

### P3.4: Profile Studio (Full-page View)
**Files:** `app-studio.js`, `app.css`, `index.html`

**SSE Events for generate:**
```javascript
case 'profileThinking': // reasoning updates
case 'profileDelta':   // content chunk
case 'profileDone':    // complete
case 'profileError':   // error
```

**Changes:**
- [ ] Create `#profileStudioModal` (full-page overlay)
- [ ] Step indicator (Brief → Fields → Generate → Review)
- [ ] Step 1: Brief textarea + genre tags
- [ ] Step 2: Fields (language, genre, platform, target, style)
- [ ] Step 3: Generate (SSE stream + Cancel)
- [ ] Step 4: Review (editable + 11-axis checklist + Save)
- [ ] Wire up to `POST /api/profiles/generate`

---

## 🔵 PHASE 4: Advanced Features

### P4.1: Market Radar Tab
**Files:** `app-radar.js`, `app.css`

**Changes:**
- [ ] Create `#tab-market` panel (NEW tab)
- [ ] Add market selector (Vietnam/Spain-LATAM/English)
- [ ] Add "Scan Now" button → `POST /api/radar/scan`
- [ ] Add source cards (Fanqie, Qidian with status)
- [ ] Add hot lists display (热门榜, 黑马榜)
- [ ] Add AI Recommendations section
- [ ] Add "Copy as Profile Seed" button
- [ ] Add last scan timestamp

### P4.2: Cocreate Modal
**Files:** `app-input.js`, `app.css`, `index.html`

**API:** `/api/cocreate/send|pause|resume|cancel` (all exist)

**Changes:**
- [ ] Create `#cocreateModal` in index.html
- [ ] Add chat message history
- [ ] Add input textarea
- [ ] Add Send/Request Draft buttons
- [ ] Wire up to cocreate API endpoints
- [ ] Handle SSE `cocreate` events

### P4.3: Import/Simulate/Diag Tools
**Files:** `app.js`, `app-input.js`

**API (all exist):**
- `GET /api/diag` → diagnostic report
- `POST /api/import` → import external story
- `POST /api/simulate` → simulate from `./simulate`
- `POST /api/importsim` → import simulate profile

**Changes:**
- [ ] Import → `POST /api/import` → modal with file upload
- [ ] Simulate → `POST /api/simulate` → select source dialog
- [ ] Diag → `GET /api/diag` → modal with markdown display
- [ ] Add progress indicator for Import/Simulate

---

## 📁 FILE MAP

```
internal/entry/web/assets/
├── index.html          # P1.1, P1.3, P3.2, P4.2
├── app.css             # P1.2, P1.3, P1.6, P2.1-P2.6, P3.1-P3.4, P4.1
├── app.js              # P1.1, P1.2, P1.4, P1.5, P1.6, P4.3
├── app-i18n.js         # P1.1 (labels)
├── app-dashboard.js    # P1.3 (status bar)
├── app-workspace.js    # P2.1-P2.6
├── app-chapters.js     # P2.2
├── app-production.js   # P3.1-P3.3
├── app-studio.js       # P3.4
├── app-radar.js        # P4.1
├── app-input.js        # P1.4, P4.2, P4.3
```

---

## 🔗 DEPENDENCY CHAIN

```
P0 (Verification)
  ↓
P1.1 (9 tabs) ──┬── P1.4 (toolbar needs tab IDs)
  │             └── P2.x (all tabs need tab routing)
  │
P1.2 (sidebar) ─┬── P1.3 (status bar)
  │              └── P1.5 (shortcuts need tab refs)
  │
P1.4 (toolbar) ─┴── P2.x (state affects tabs)
  │
P3.1 (Production) ── P3.2 (Foundation Gate in Production)
P3.3 (Library) ───── P3.4 (Studio opens from Library)
```

---

## ✅ SUCCESS CRITERIA

### Phase 1
- [ ] 9 tabs visible and navigable (S/C/O/W/R/D/M/P/H shortcuts work)
- [ ] Sidebar collapses/expands with 200ms animation
- [ ] Status bar shows: `Engine: {state} · Ch.{n} · Health: {pct}% · ${cost} · {ctx}% ctx`
- [ ] Toolbar adapts to engine state (9 states covered)
- [ ] Event log collapsible

### Phase 2
- [ ] Stream: Pin/Clear buttons + Quick Stats
- [ ] Chapters: 2-column reader with filter/sort/nav
- [ ] Outline: Premise + Compass + Layered view
- [ ] World: Characters grid + Foreshadow tracking
- [ ] Review: 7-dimension scores with bars
- [ ] System Radar: Health + Agents + Costs + Events

### Phase 3
- [ ] Production jobs visible with health strip
- [ ] Foundation Gate modal with 11-axis checklist
- [ ] Profile Library shows profiles with actions
- [ ] Profile Studio 4-step wizard with SSE generate

### Phase 4
- [ ] Market Radar shows scan results
- [ ] Cocreate modal chat works
- [ ] Import/Simulate/Diag tools work

---

## 🚨 RISK ASSESSMENT

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing tab routing | High | Test each tab switch after P1.1 |
| CSS conflicts with existing styles | Medium | Use BEM naming, test in isolation |
| SSE event format change from upstream | Medium | Guard against missing fields |
| go:embed requires rebuild | Low | Remind: `go build ./...` after CSS/JS changes |
| Market Radar partial data | Low | Show "No data" state gracefully |

---

## 📝 BUILD & TEST CHECKLIST

After each phase:
```bash
# 1. Rebuild binary
go build ./...

# 2. Verify no vet errors
go vet ./...

# 3. Test in browser
ainovel-cli --web
# or
go run ./cmd/ainovel-cli --web

# 4. Check for console errors
# 5. Verify tab navigation
# 6. Verify SSE events flowing
```

---

## 📋 BACKLOG (Out of Scope for Now)

- [ ] "Add Foreshadow" button — API is read-only, no POST endpoint
- [ ] Edit Chapter inline — read-only per spec
- [ ] Auto-save Chapter edits — out of scope
- [ ] Mobile bottom nav — responsive breakpoints exist but not implemented
- [ ] Ctrl+1-9 direct tab — optional, low priority
