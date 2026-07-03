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
| `prodrun.go` | Production Cockpit: `ProdRun` model + JSON store |
| `prodrun_runner.go` | Production Cockpit: spawn `ainovel-cli --headless`, poll progress, target kill |
| `prodrun_handlers.go` | Production Cockpit: `/api/profiles`, `/api/prodruns*` endpoints |
| `prodrun_export.go` | Production Cockpit: server-side TXT concatenation |
| `prompts.go` | prompt override loader: đọc `~/.ainovel/prompts/*.md` → `Bundle.OverridePrompt` trước `host.New` |
| `reveal.go` | mở thư mục (novel output / prompt override) bằng file manager của OS |
| `embed.go` | `go:embed` assets |
| `assets/{index.html,app.css,app-i18n.js,app.js,app-dashboard.js,app-workspace.js,app-studio.js,app-input.js}` | SPA 1 trang: i18n labels, dashboard, workspace tabs, studio modals, input UX |
| `*_test.go` | guard hồi quy (ask, sse, server, assets, content) |

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

## 3. Tab Sản xuất (Production Cockpit MVP)

Tab thứ 6 trong workspace (ngay trước tab **Hỗ trợ** ở vị trí thứ 7), dùng để xếp hàng và chạy các job tạo truyện tự động bằng `ainovel-cli --headless`.

### API endpoints

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/api/profiles` | — | `[{name, path}]` |
| GET | `/api/prodruns` | — | `[ProdRun]` |
| POST | `/api/prodruns` | `{name, profile, model?, provider?, targetChapters?}` | `ProdRun` |
| GET | `/api/prodruns/{id}` | — | `ProdRun` |
| POST | `/api/prodruns/{id}/start` | — | `ProdRun` |
| POST | `/api/prodruns/{id}/stop` | — | `ProdRun` |
| GET | `/api/prodruns/{id}/log` | — | text/plain tail |
| POST | `/api/prodruns/{id}/export` | `{format}` | `{path}` |
| GET | `/api/prodruns/{id}/export.txt` | — | `text/plain` download |

### Luồng chạy

1. User tạo job (`POST /api/prodruns`) → trạng thái `queued`.
2. User bấm **Bắt đầu** → backend spawn một tiến trình con `ainovel-cli --headless --prompt-file profile.md`, với `Cmd.Dir` là thư mục của job (`output/jobs/{id}/`).
3. Tiến trình con tự tạo `output/jobs/{id}/output/novel/`; runner poll mỗi 5 giây:
   - `meta/progress.json` → số chương hoàn thành.
   - `reviews/*.json` → số review + số rewrite verdict.
   - `meta/usage.json` → chi phí.
   - `run.log` → phát hiện pause.
4. Khi `completed_chapters >= targetChapters`, runner kill tiến trình con và đánh dấu `completed` với `stopReason: target_reached`.
5. User có thể **Dừng** bất kỳ lúc nào → hard kill, trạng thái `cancelled`.
6. User **Xuất TXT** → backend nối các file `output/novel/chapters/*.md` theo thứ tự số và trả về `export.txt`.

### Hạn chế MVP

- Chỉ chạy một job tại một thờ điểm (mỗi job một tiến trình con).
- Không hẹn giờ, không chạy song song nhiều job.
- Pause là read-only: hiện thông báo, chỉ có thể Dừng hoặc xuất file; không có nút Tiếp tục.
- Chỉ xuất TXT; EPUB deferred.
- Khi Web UI crash/restart, các job đang `running` bị đánh dấu `failed`/`unclean_shutdown` + `PossiblyOrphaned`; user cần tự kill PID nếu còn sót.

### Tính additive

Toàn bộ code Production Cockpit nằm trong `internal/entry/web/` (file mới) hoặc là thêm route vào `server.go`/`run.go` — hai file upstream đã được fork sửa trước đó. Không đụng `internal/host/`, `internal/tools/`, `assets/prompts/`.

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
