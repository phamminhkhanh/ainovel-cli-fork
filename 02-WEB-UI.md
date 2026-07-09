# Web UI — Ý tưởng, cách làm & quy trình bám upstream

> Sổ tay cho bản fork: vì sao có web UI, nó được dựng/port thế nào, và **làm gì mỗi
> lần kéo bản mới từ upstream** (`voocel/ainovel-cli`). Đọc mục 4–5 trước khi merge.

## 1. Vì sao có web UI (ý tưởng gốc)

Cách Việt hoá kiểu cũ sửa **thẳng** vào TUI + prompt + engine → mỗi lần `git pull`
xung đột hàng chục file, không bám nổi upstream.

Kiến trúc engine cho ta lối thoát: `Host` là engine tự đủ với **API công khai sạch**;
TUI và headless chỉ là 2 adapter **tiêu thụ** API đó (phụ thuộc 1 chiều `entry → host`).

→ Giải pháp: thêm adapter **thứ ba** — web UI — dưới dạng package **mới** `internal/entry/web/`,
**không đụng** `internal/host/`, `internal/tools/`, `assets/prompts/`. Engine giữ nguyên
(kể cả narration tiếng Trung) vẫn chạy đúng; UI muốn tiếng Việt tuỳ ý. Vì additive nên
`git pull upstream` gần như **0 xung đột** — đúng thứ bản fork cũ đánh mất.

## 2. Định hướng UI/UX

Web UI là **trình quản lý truyện + điều khiển engine**, không chỉ là màn hình stream.

- **Không để khung chính trống:** khi engine không stream, khung giữa là *workspace* đọc
  chương, outline, world/characters.
- **Sidebar là dashboard:** hiển thị tiến độ, outline, rewrite queue, steer, agents, usage, cache.
- **Outline clickable:** click chương ở sidebar → mở tab **Chương** và đọc nội dung (final hoặc draft).
- **Stream chỉ khi cần:** auto-switch về **Stream** chỉ khi user bấm Bắt đầu / Tiếp tục / Can thiệp.
  Background streaming không được cướp focus khỏi tab user đang đọc.
- **State lấy từ disk là source-of-truth:** endpoint đọc file trực tiếp qua `internal/store`,
  không gate bằng `UISnapshot.TotalChapters` (tránh 404 sai khi snapshot stale).
- **Read-only trước:** chỉnh sửa nội dung từ UI là out-of-scope cho MVP.
- **Tiếng Việt hoàn toàn ở tầng UI:** nhãn, log, placeholder dịch trong `app-i18n.js`;
  engine/prompt không bị sửa.
- **0 dependency frontend:** JS thuần, embed vào binary; không cần build step.

Chạy: `ainovel-cli --web` → mở `http://127.0.0.1:8787` (chỉ bind localhost).

> ⚠️ **Cấu hình lần đầu:** chế độ `--web` (và `--headless`) **không** có wizard cấu hình.
> Máy chưa từng thiết lập config (`~/.ainovel/config.json`) thì chạy TUI một lần cho xong
> (`ainovel-cli` hoặc `go run ./cmd/ainovel-cli`) — chọn provider / API key / model, mất vài giây.
> Đã có config rồi thì `--web` chạy thẳng. (Quyết định: không port wizard sang web vì setup qua
> terminal nhanh gọn hơn nhiều so với chi phí thêm/bảo trì một trang setup riêng.)

- **Xuống** (SSE `GET /api/events`): stream chữ / event / snapshot / ask / done — 1 kênh đa hợp.
- **Lên** (fetch POST): start / steer / continue / abort / resume / model / thinking /
  cocreate / export / import / diag / job/cancel / reveal.

Frontend = **JS thuần, 0 dependency**, nhúng vào binary bằng `go:embed` → 1 file chạy, không build step.

### File trong `internal/entry/web/` (toàn bộ MỚI → không bao giờ xung đột upstream)

| File | Vai trò |
|---|---|
| `run.go` | `web.Run`: dựng Host, set ask handler, goroutine tiêu thụ event → SSE, `http.Server` |
| `server.go` | đăng ký route + middleware (Host-header allowlist chống CSRF/DNS-rebinding); giữ `*store.Store` cache |
| `sse.go` | SSE hub: 1 consumer fan-out, đa hợp khung stream/event/snapshot/ask/done |
| `handlers.go` | các POST handler → gọi method Host |
| `ask.go` | cầu nối block→channel cho `ask_user` (engine block tới khi client trả lời) |
| `phase3.go` | cocreate / export / import / diag (Phase 3) |
| `content.go` | read-only content endpoints: chapters, outline, world, characters |
| `content_reviews.go` | read-only: đánh giá 7 chiều của Editor (`/api/reviews`) + sổ 伏笔 (`/api/foreshadow`) |
| `prodrun.go` | `ProdRun` model (`SeededFrom`/`FoundationApproved`/`RevisionNote`/`PersistError`) + JSON store `jobs.json` (persist có retry chống Windows lock) |
| `prodrun_runner.go` | spawn `ainovel-cli --headless`, poll progress/reviews/cost, target-kill, Foundation Gate detect, reap+retry (persist chống Windows lock) |
| `prodrun_handlers.go` | HTTP handlers `/api/profiles*` + `/api/prodruns*` (create/start/stop/sync/foundation/approve/reject/revise/resume/reveal/ide-bundle/export) |
| `prodrun_health.go` | Health strip: pure fn `computeRunHealth` → 5 metric (progress/rewrite_rate/cost_pace/budget/persist); `prodRunView` bọc `health` vào response |
| `prodrun_profiles.go` | resolver profile 3 nguồn (project/global/legacy) + validate path |
| `prodrun_workspace.go` | continue_workspace: fingerprint + seed workspace vào sandbox |
| `prodrun_sync.go` | sync kết quả run về workspace chính (fast-forward / force + backup) |
| `prodrun_export.go` | server-side TXT concatenation |
| `profiles_library.go` | Profile Library: CRUD `.md` (`content`/`save`/`delete`), ghi/xóa **project-only**, guard 409 + traversal |
| `profile_studio.go` | Profile Studio: `POST /api/profiles/generate` — **stream qua SSE** (`profileDelta`/`profileThinking`/`profileDone`/`profileError`; fallback 1-shot JSON khi không flush được) qua `bootstrap.NewModelSet(s.cfg)`; system prompt principle-based (frame-first · long-novel survival rules · market-fit) |
| `prompts.go` | prompt override loader: đọc `~/.ainovel/prompts/*.md` → `Bundle.OverridePrompt` trước `host.New` |
| `reveal.go` | mở thư mục bằng file manager của OS (loopback-only) |
| `embed.go` | `go:embed` assets |
| `assets/{index.html,app.css,app-i18n.js,app.js,app-dashboard.js,app-workspace.js,app-chapters.js,app-studio.js,app-production.js,app-input.js}` | SPA 1 trang: dashboard, workspace tabs, chapters, studio/profile modals, Production Cockpit, input UX |
| `*_test.go` | guard hồi quy (ask, sse, server, assets, content, prodrun*, profile*, health) |

### Content workspace (khung chính)

Khung giữa không còn trống khi engine không stream. Nó là workspace có 7 tab:

| Tab | Nguồn | Hành vi |
|---|---|---|
| **Stream** | SSE `events` | hiển thị thinking + draft đang chảy (giữ nguyên). |
| **Chương** | `GET /api/chapters/{n}` | đọc chương đã commit; nếu chưa có thì fallback `/api/chapters/{n}/draft` với nhãn “Bản nháp”. |
| **Outline** | `GET /api/outline` | premise + danh sách chương (chapter / title / core event). |
| **World** | `GET /api/world` + `GET /api/characters` + `GET /api/foreshadow` | world rules, timeline, compass, nhân vật, sổ 伏笔 (cài/thu hồi). |
| **Đánh giá** | `GET /api/reviews` | review 7 chiều của Editor theo chương + review vòng cung: verdict, điểm từng chiều, issue có trích dẫn, contract status. |
| **Sản xuất** | `GET /api/prodruns` + `/api/profiles` | queue, start, monitor, stop, and export headless novel-generation runs. |
| **Hỗ trợ** | embedded guide | hướng dẫn nhanh, giải thích các tab và thể loại phù hợp. |

- Click outline item ở sidebar → chuyển tab **Chương** và load nội dung.
- Tab cuối được ghi nhớ theo phiên (`sessionStorage`).
- Chỉ khi người **Bắt đầu / Tiếp tục / Can thiệp** mới tự động chuyển về **Stream**; background streaming không cướp focus.
- Nội dung chương render vào `#chapterText` là `<div>` (không phải `<pre>`), dùng CSS `white-space: pre-wrap` để giữ format.

### API read-only thêm vào

| Method | Path | Response |
|---|---|---|
| GET | `/api/chapters/{n}` | `{ "chapter": n, "kind": "final", "text": "..." }` |
| GET | `/api/chapters/{n}/draft` | `{ "chapter": n, "kind": "draft", "text": "..." }` |
| GET | `/api/outline` | `{ "premise": "...", "outline": [...], "layered": {...}, "compass": {...} }` |
| GET | `/api/world` | `{ "rules": [...], "timeline": [...], "compass": {...} }` |
| GET | `/api/characters` | `{ "characters": [...], "supporting": [...] }` |
| GET | `/api/foreshadow` | `{ "entries": [{ id, description, planted_at, status, resolved_at }] }` |
| GET | `/api/reviews` | `{ "reviews": [ReviewEntry...], "global": ReviewEntry }` |

Quy tắc:

- `{n}` phải là số nguyên dương; ngược lại trả `400`.
- Handler đọc file trực tiếp qua `server.store` (một `*store.Store` cache khởi tạo lúc startup),
  không tạo store mới mỗi request.
- File missing/empty → `404` với JSON error.
- File corrupt / lỗi đọc đĩa → `500` với JSON error (không nuốt lỗi).
- Frontend cache outline/world trong `sessionStorage`-scope; cache được xóa sau mỗi hành động
  thay đổi truyện (Start/Continue/Steer/Cocreate finish) để tránh hiển thị data cũ.

### Việt hoá (nằm gọn trong file của TA — không đụng engine)

- **Nhãn UI + log:** dịch trong `assets/app-i18n.js` (map `UI_LABEL_MAP` + hàm `translateSummary` + map `EVENT_SUMMARY_MAP`).
  ⚠️ Chỉ dịch được **chuỗi cố định** (Summary của event). Suy luận **tự sinh** của coordinator
  (tiếng Trung, từ `assets/prompts/coordinator.md`) **không** dịch được bằng map — đó là backstage,
  không vào truyện.
- **Nội dung truyện tiếng Việt:** KHÔNG sửa prompt engine. Dùng cơ chế **user-rules**:
  đặt `~/.ainovel/rules/lang-vi.md` (ngoài repo) → engine tự nạp cho writer/architect/editor
  khi **mở sách mới**. Sách cũ đã có `meta/user_rules.json` thì không đọc lại.
- **Ghi đè hẳn prompt (tuỳ chọn nâng cao):** bỏ file `.md` vào `~/.ainovel/prompts/`
  (tên đúng: `coordinator.md` / `architect-short.md` / `architect-long.md` / `writer.md` / `editor.md`)
  → `web.Run` gọi `assets.Bundle.OverridePrompt` (seam upstream sẵn có) để **thay nguyên** prompt role đó
  trước `host.New`. Additive, 0 sửa `assets/prompts`. Sidebar có nút **🧩 Thư mục prompt** mở thẳng thư mục này;
  đổi prompt xong phải **restart `--web`** (prompt nạp một lần lúc dựng engine). Role đã override thì
  không còn ăn cải tiến prompt từ upstream — cân nhắc chỉ override chọn lọc, phần còn lại dùng user-rules.

## 3. Tab Sản xuất (Production Cockpit)

Tab thứ 6 trong workspace (ngay trước tab **Hỗ trợ**), dùng để xếp hàng và chạy các job `ainovel-cli --headless` trong sandbox riêng `output/jobs/{id}/`.

Cockpit có 2 kiểu job:

| Kind | Khi dùng | Cách chạy |
|---|---|---|
| `fresh_profile` | Tạo truyện mới từ profile `.md` | copy `profile.md` vào run dir, spawn `ainovel-cli --headless --prompt-file profile.md` |
| `continue_workspace` | Cook tiếp workspace hiện tại | seed `output/novel/` sang `output/jobs/{id}/output/novel/`, spawn `ainovel-cli --headless` **không prompt** để engine native `Resume()` tiếp tục |

> Với `continue_workspace`, `targetChapters` là **tổng số chương tuyệt đối cuối cùng**, không phải số chương viết thêm. Ví dụ workspace đang có 12 chương, muốn viết đến 100 chương thì nhập `100`.

### API endpoints

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/api/profiles` | — | `[{name, path, source}]`, với `path` dạng `project/foo.md`, `global/foo.md`, hoặc `legacy/foo.md` |
| GET | `/api/prodruns` | — | `[ProdRunView]` (`ProdRun` + `health`) |
| POST | `/api/prodruns` | `{kind:"fresh_profile", name, profile, model?, provider?, targetChapters?, budgetUsd?}` | `ProdRunView` |
| POST | `/api/prodruns` | `{kind:"continue_workspace", name, model?, provider?, targetChapters, budgetUsd?}` | `ProdRunView` kèm `seededFrom` |
| GET | `/api/prodruns/{id}` | — | `ProdRunView` |
| POST | `/api/prodruns/{id}/start` | — | `ProdRunView` |
| POST | `/api/prodruns/{id}/stop` | — | `ProdRunView` |
| POST | `/api/prodruns/{id}/sync` | `{force?: boolean}` | `{copiedFiles, mode, fastForward}` |
| GET | `/api/prodruns/{id}/log` | — | text/plain tail |
| POST | `/api/prodruns/{id}/export` | `{format}` | `{path}` |
| GET | `/api/prodruns/{id}/export.txt` | — | `text/plain` download |
| POST | `/api/prodruns/{id}/resume` | — | `ProdRunView` — chỉ khi status `failed` (vd sau unclean shutdown / persist fail), khởi động lại run dir |
| GET | `/api/prodruns/{id}/foundation` | `?section=world\|characters` | premise/outline/world/characters của run (Foundation Gate preview) |
| POST | `/api/prodruns/{id}/approve` | — | duyệt nền móng (`awaiting_review`) → resume native headless |
| POST | `/api/prodruns/{id}/reject` | — | `204` — xoá run chờ duyệt, khỏi tốn token Writer/Editor |
| POST | `/api/prodruns/{id}/revise` | `{feedback}` | sinh lại nền móng theo góp ý → tạo run MỚI (giữ run cũ) |
| POST | `/api/prodruns/{id}/reveal` | — | mở thư mục nền móng bằng OS file manager (loopback-only) |
| GET | `/api/prodruns/{id}/ide-bundle` | — | `{jobId, dir, files[]}` — đường dẫn + danh sách file nền móng để mở trong IDE (dùng được cả khi bind non-loopback) |
| GET | `/api/profiles/content` | `?path=project/foo.md` | `{path, name, source, content}` |
| POST | `/api/profiles/save` | `{name, content, overwrite?}` | lưu profile **project-only**; trùng tên chưa `overwrite` → `409` |
| POST | `/api/profiles/delete` | `{path}` | xoá profile **project-only** (global/legacy → `403`) |
| POST | `/api/profiles/generate` | `{idea, language?, genre?, platform?, styleNotes?, targetChapters?, model?, provider?}` | **SSE** (`profileDelta`/`profileThinking`/`profileDone`/`profileError`); fallback `{content}` JSON khi không flush được. KHÔNG tự lưu/chạy |

`ProdRunView` là `ProdRun` được wrap thêm `health: { overall, metrics[] }`. Các metric ổn định gồm `progress`, `rewrite_rate`, `cost_pace`, `budget`, `persist`; level là `idle/good/warn/bad`. Health tính backend-side từ dữ liệu đã poll (`progress.json`, `reviews/*.json`, `usage.json`) + `PersistError`, để mọi response prodrun có cùng shape. `progress` chỉ để tham khảo (không kéo `overall`); `persist` báo lỗi lưu `jobs.json` (Windows file lock) → chip đỏ/vàng.

### Luồng fresh_profile

1. User tạo job từ profile (`POST /api/prodruns`) → trạng thái `queued`.
2. User bấm **Bắt đầu** → backend copy profile/rules/config vào run dir và spawn `ainovel-cli --headless --prompt-file profile.md`.
3. Tiến trình con tạo `output/jobs/{id}/output/novel/`; runner poll mỗi 5 giây:
   - `meta/progress.json` → số chương hoàn thành.
   - `reviews/*.json` → số review + số rewrite verdict.
   - `meta/usage.json` → chi phí.
   - `run.log` → phát hiện pause.
4. Khi `completed_chapters >= targetChapters`, runner kill tiến trình con và đánh dấu `completed` với `stopReason: target_reached`.

### Luồng continue_workspace

1. User bấm **Cook tiếp**. Backend chỉ tạo job và lưu `seededFrom` gồm:
   - `completedChapters = len(progress.CompletedChapters)`.
   - `fingerprint` SHA-256 content của workspace, bỏ qua noise như `logs/` và `*.log`.
   - thời điểm capture.
2. Seed thật xảy ra ở **Start**, không phải Create:
   - từ chối nếu host engine đang chạy;
   - re-fingerprint workspace, nếu khác seed ban đầu → lỗi `workspace changed since the continue run was created`;
   - copy workspace sang sandbox `output/jobs/{id}/output/novel/` bằng exclude list an toàn;
   - re-fingerprint lần nữa sau copy để bắt race trong lúc seed.
3. Runner spawn `ainovel-cli --headless` **không `--prompt-file`**. Headless đi vào nhánh `eng.Resume()` và tự resume từ checkpoint đã seed.
4. Cockpit không hiểu/điều phối logic viết tiếp; nó chỉ là file-plumber + process spawner.

### Đồng bộ kết quả về workspace chính

- `fresh_profile`: mặc định chỉ sync vào workspace trống; `force` cho phép ghi đè theo cơ chế cũ.
- `continue_workspace`: sync giống `git fast-forward`:
  - workspace hiện tại vẫn đúng fingerprint lúc seed → `fastForward: true`, copy file-by-file về host;
  - workspace đã diverge → trả 409, UI hỏi lại `force`;
  - `force` bắt buộc backup `output/backups/pre-sync-*` trước khi ghi.

Trên Windows, continue sync **không** clear/rename cả thư mục host. Nó dùng copy file-by-file qua `safeWriteFile` để tránh hỏng workspace khi có file lock.

### Profile sources

- `./.ainovel/profiles/` → profile riêng project hiện tại (`project/foo.md`).
- `~/.ainovel/profiles/` → profile global dùng lại giữa nhiều project (`global/foo.md`).
- `./profiles/` → legacy/sample (`legacy/foo.md`); old value `profiles/foo.md` vẫn resolve về legacy để tương thích job cũ.

### Profile Library & Studio (soạn + duyệt profile trước khi tạo run)

Profile là **SSOT** — soạn/duyệt ngay trong UI *trước* khi có run, không tự sinh ngầm lúc tạo job. Modal **📚 Thư viện Profile**:

- **Thư viện**: list `project/global/legacy`; chỉ `project` (`./.ainovel/profiles/`) sửa/xoá được (global/legacy read-only).
- **Studio 4 bước**: (1) brief template hoặc gõ tay; (2) ý tưởng thô + field (thể loại, platform, ngôn ngữ, số chương, phong cách); (3) **Sinh profile** — stream realtime qua SSE (chống proxy 504; reasoning model gửi `profileThinking` heartbeat) qua model **default trong config lúc khởi động** (nhãn hiện rõ `provider/model`, không đổi theo ⚙ Model runtime), hoặc **📋 Copy cho LLM ngoài** để LLM ngoài sinh; (4) **Kết quả** → sửa → **Lưu**.
- Prompt sinh (`profile_studio.go`) genre-agnostic, principle-based: **frame-first** (tự xác định thể loại / đã đại trà chưa / đặc trưng / thị trường *trước khi viết*), **long-novel survival rules** (cái giá thật ≠ twist · nhân vật có giới hạn/điểm mù · phản diện có tên + tuyến phản bội có trả · phục vụ thể loại chính · mid-pivot point-of-no-return · ensemble có tên · kết có giá + khớp title · đa dạng nhịp/nội tâm), **market-fit** (hợp văn hoá đọc nước đích tại năm hiện tại), anti-AI-tell (kể cả trong worldbuilding).

### Soát nền móng ở Step 4 (điểm review foundation cấp profile)

Profile là gốc — sai ở đây nhân lên hàng trăm chương. Step 4 có khối **🔍 Soát nền móng**:

- **Checklist tự review** (11 mục): nhân vật nhất quán + giới hạn, cái giá thật, phản diện xứng tầm, thể loại chính không bị lấn, cơ chế lõi có luật, mid-pivot cụ thể, ensemble, kết có giá + khớp title, hợp thị trường, AI-tell.
- **📋 Copy profile + prompt review cho LLM ngoài**: copy *profile đã sinh* + prompt review 13 trục (kèm bối cảnh thị trường + năm động + bắt reviewer định khung thể loại/độ đại trà trước) để dán vào GPT/Claude. Khác với "Copy cho LLM ngoài" ở Step 3 (copy *ý tưởng* để **sinh**).
- *(Backlog)* "🔍 AI soát lỗi trong app" — review ngay bằng 1 LLM call nội bộ (seam `bootstrap.NewModelSet`), chưa làm.

### Foundation Gate + Health strip

- **Foundation Gate**: run `fresh_profile` tự dừng ở `awaiting_review` khi nền móng xong (poll thấy phase→writing, 0 chương). Duyệt (`approve` → resume native) · Từ chối (`reject` → xoá) · Sửa & tạo lại (`revise` → run mới, giữ run cũ) · mở nền móng (`reveal` loopback-only). Best-effort (poll 5s). Chi tiết: [04-LUU-Y-MERGE-UPSTREAM.md](04-LUU-Y-MERGE-UPSTREAM.md).
  - **Lớp review nền móng** (tại gate): **📋 Copy review** (`copyFoundationForReview`) — copy foundation + profile gốc + prompt review (có trục "trung thành profile" bắt Architect drift) cho LLM ngoài → nhận revision note ngắn → dán vào Revise. **📋 Copy cho IDE** (`copyFoundationForIDE`) — bundle "Review & Edit" cho agent IDE (Kilo/Cursor) soi theo trục chết người rồi **sửa trực tiếp 5 file nền móng** (surgical, không regenerate, không tốn token); dùng `GET /api/prodruns/{id}/ide-bundle` (abs dir + file tồn tại). Trục review là SSOT chung (`FOUNDATION_REVIEW_AXES`/`buildReviewAxes`) cho cả 2.
- **Resume failed** (`POST .../resume`): tiếp tục run `failed`/`cancelled` (lỗi transient đã khắc phục) — copy home rules mới nhất rồi headless `Resume()` native.
- **Health strip** (panel chi tiết run): dải "🟢 đúng nhịp / 🟡 nên xem / 🔴 cần chú ý" tổng hợp 5 metric — trả lời nhanh "run có ổn / có cần xem không" mà không phải đọc log; chip `persist` đỏ khi `jobs.json` bị khóa (nhắc tắt IDE).

### Hạn chế hiện tại

- Chỉ chạy một production job tại một thời điểm.
- Không cho seed/start continue run khi host engine đang chạy.
- Không cho continue run nếu workspace chưa có chương hoàn thành hoặc đã `phase=complete`.
- Không hẹn giờ, không chạy song song nhiều job.
- Pause là read-only: hiện thông báo, chỉ có thể Dừng hoặc xuất file; không có nút Tiếp tục.
- Chỉ xuất TXT; EPUB deferred.
- Khi Web UI crash/restart, các job đang `running` bị đánh dấu `failed`/`unclean_shutdown` + `PossiblyOrphaned`; user cần tự kill PID cũ nếu còn sót, rồi `POST /api/prodruns/{id}/resume` để chạy lại.
- Chưa có review profile bằng AI ngay trong app (mới có checklist + copy cho LLM ngoài).

### Tính additive

Toàn bộ code Production Cockpit nằm trong `internal/entry/web/` (file mới) hoặc là thêm route vào `server.go`/`run.go` — hai file upstream đã được fork sửa trước đó. Không đụng `internal/host/`, `internal/tools/`, `assets/prompts/`.

### State machine (tham chiếu kỹ thuật)

Vòng đời ProdRun (7 trạng thái, race analysis đầy đủ) document ở [`docs/prodrun-state-machine.md`](docs/prodrun-state-machine.md). Dùng khi cần hiểu Foundation Gate, T6/T9 kill-race, lock scope của `start()`, hay T17 sibling semantics. Phần "Hạn chế hiện tại" ở trên là bản tóm tắt rút gọn; doc kỹ thuật là nguồn sự thật.

## 4. Điểm chạm duy nhất vào file upstream

Chỉ **`cmd/ainovel-cli/main.go`** bị sửa (mirror nhánh `--headless` sẵn có):
- import `internal/entry/web`
- struct `cliOptions`: thêm `Web`, `Addr`, `UnsafePublicWeb`
- `parseCLIOptions`: thêm case `--web` / `--addr` / `--unsafe-public-web` + validation
- `runWithConfig`: nhánh `if opts.Web { web.Run(...) }`

Ngoài ra (nếu đã sửa): `README.md`, `.gitignore`.

→ Đây là **tất cả** bề mặt có thể xung đột khi merge. Mọi thứ trong `internal/entry/web/`
là file mới → không bao giờ đụng nhau.

## 5. Port: biến repo này thành fork (ĐÃ XONG — 2026-06-30)

**Trạng thái remote** (đã set up, xác nhận bằng `git remote -v`):
- `origin`   = `github.com/phamminhkhanh/ainovel-cli-fork` — fork của bạn, push/pull thoải mái
- `upstream` = `github.com/voocel/ainovel-cli` — gốc, chỉ kéo về

Web UI đã đẩy lên fork (commit đầu `c793074`). **Flow thật của lần đẩy đầu** — web UI lúc đó
**chưa commit**, và fork đã có sẵn lịch sử voocel **mới hơn** local nên không push thẳng được:

```bash
git add -A
git commit -m "feat: add web UI adapter (internal/entry/web) + docs"
git merge upstream/main          # kéo voocel mới nhất vào local → mới đủ điều kiện push
#   → kẹt đúng 1 dòng import trong cmd/ainovel-cli/main.go → giải keep-both (xem mục 5)
go build ./... && go vet ./... && go test ./internal/entry/web/...
git push -u origin main
```

Solo-dev: để hết trên `main` của fork cho gọn — không cần nhánh riêng. Từ giờ update upstream theo mục 5.

## 6. Lưu ý mỗi lần update từ upstream

Dùng **merge** (không rebase — khỏi force-push, giải xung đột 1 lần):

```bash
git fetch upstream
git merge upstream/main
# giải xung đột nếu có (xem dưới) → build/test → push
go build ./... && go vet ./... && go test ./internal/entry/web/...
git push origin main
```

**Xung đột CHỈ có thể ở** (theo thứ tự khả năng):
1. `cmd/ainovel-cli/main.go` — nếu upstream sửa đúng vùng cờ/parse/import. Giải: **giữ cả hai** —
   phần upstream mới + các dòng `--web` của bạn (chúng độc lập, chỉ cần đặt cạnh nhau).
   *(Ví dụ thật — merge tính năng `eval` ngày 2026-06-30: chỉ kẹt 1 dòng import, giữ cả
   `internal/entry/web` lẫn `internal/eval`; thân hàm git tự merge sạch.)*
2. `README.md`, `.gitignore` — nếu cả hai cùng sửa. Giải: gộp tay.
3. *(Hiếm)* Upstream đổi **API công khai của Host** (`Steer`/`Snapshot`/`SwitchModel`/…):
   vá các call-site trong `internal/entry/web/` cho khớp chữ ký mới. `go build` sẽ chỉ ngay chỗ hỏng.

**Không bao giờ phải sửa khi merge:**
- File trong `internal/entry/web/**` (toàn của bạn)
- `~/.ainovel/rules/lang-vi.md` (ngoài repo)

**Nhắc kỹ thuật:**
- Assets nhúng bằng `go:embed` → sửa `app.js`/`app.css`/`index.html` xong phải **rebuild binary**
  rồi restart server (refresh trình duyệt KHÔNG đủ — nó phục vụ asset đã nhúng lúc compile).
- Sau mỗi merge, tối thiểu: `go build ./...` + `go vet ./...` + test package web.
- `.gitattributes` ép LF mọi nơi (kể cả Windows) → hết warning CRLF khi `git add`, `start-web.sh` không vỡ shebang trên Linux.
