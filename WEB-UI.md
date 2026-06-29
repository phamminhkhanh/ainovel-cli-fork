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

## 2. Cách làm (kiến trúc)

Chạy: `ainovel-cli --web` → mở `http://127.0.0.1:8787` (chỉ bind localhost).

- **Xuống** (SSE `GET /api/events`): stream chữ / event / snapshot / ask / done — 1 kênh đa hợp.
- **Lên** (fetch POST): start / steer / continue / abort / resume / model / thinking /
  cocreate / export / import / diag.

Frontend = **JS thuần, 0 dependency**, nhúng vào binary bằng `go:embed` → 1 file chạy, không build step.

### File trong `internal/entry/web/` (toàn bộ MỚI → không bao giờ xung đột upstream)

| File | Vai trò |
|---|---|
| `run.go` | `web.Run`: dựng Host, set ask handler, goroutine tiêu thụ event → SSE, `http.Server` |
| `server.go` | đăng ký route + middleware (Host-header allowlist chống CSRF/DNS-rebinding) |
| `sse.go` | SSE hub: 1 consumer fan-out, đa hợp khung stream/event/snapshot/ask/done |
| `handlers.go` | các POST handler → gọi method Host |
| `ask.go` | cầu nối block→channel cho `ask_user` (engine block tới khi client trả lời) |
| `phase3.go` | cocreate / export / import / diag (Phase 3) |
| `embed.go` | `go:embed` assets |
| `assets/{index.html,app.css,app.js,app-studio.js}` | SPA 1 trang |
| `*_test.go` | guard hồi quy (ask, sse, server, assets) |

### Việt hoá (nằm gọn trong file của TA — không đụng engine)

- **Nhãn UI + log:** dịch trong `assets/app.js` (hàm `translateSummary` + map `EVENT_SUMMARY_MAP`).
  ⚠️ Chỉ dịch được **chuỗi cố định** (Summary của event). Suy luận **tự sinh** của coordinator
  (tiếng Trung, từ `assets/prompts/coordinator.md`) **không** dịch được bằng map — đó là backstage,
  không vào truyện.
- **Nội dung truyện tiếng Việt:** KHÔNG sửa prompt engine. Dùng cơ chế **user-rules**:
  đặt `~/.ainovel/rules/lang-vi.md` (ngoài repo) → engine tự nạp cho writer/architect/editor
  khi **mở sách mới**. Sách cũ đã có `meta/user_rules.json` thì không đọc lại.

## 3. Điểm chạm duy nhất vào file upstream

Chỉ **`cmd/ainovel-cli/main.go`** bị sửa (mirror nhánh `--headless` sẵn có):
- import `internal/entry/web`
- struct `cliOptions`: thêm `Web`, `Addr`, `UnsafePublicWeb`
- `parseCLIOptions`: thêm case `--web` / `--addr` / `--unsafe-public-web` + validation
- `runWithConfig`: nhánh `if opts.Web { web.Run(...) }`

Ngoài ra (nếu đã sửa): `README.md`, `.gitignore`.

→ Đây là **tất cả** bề mặt có thể xung đột khi merge. Mọi thứ trong `internal/entry/web/`
là file mới → không bao giờ đụng nhau.

## 4. Port: biến repo này thành fork (LÀM 1 LẦN)

**Hiện trạng:** `origin` đang trỏ **thẳng** vào voocel (`git remote -v` → `origin = github.com/voocel/ainovel-cli`).
Tức là **chưa fork** — không push được.

```bash
# 1. Đã fork voocel/ainovel-cli → github.com/phamminhkhanh/ainovel-cli-fork
# 2. Đổi remote cũ (voocel) thành upstream — chỉ để kéo về:
git remote rename origin upstream
# 3. Thêm fork của bạn làm origin:
git remote add origin https://github.com/phamminhkhanh/ainovel-cli-fork.git
# 4. Đẩy main (gồm toàn bộ web UI) lên fork:
git push -u origin main
```

Sau bước này: `origin` = fork của bạn (push/pull thoải mái), `upstream` = voocel (chỉ kéo về).
Solo-dev thì cứ để hết trên `main` của fork cho gọn — không cần nhánh riêng.

## 5. Lưu ý mỗi lần update từ upstream

Dùng **merge** (không rebase — khỏi force-push, giải xung đột 1 lần):

```bash
git fetch upstream
git merge upstream/main
# giải xung đột nếu có (xem dưới) → build/test → push
go build ./... && go vet ./... && go test ./internal/entry/web/...
git push origin main
```

**Xung đột CHỈ có thể ở** (theo thứ tự khả năng):
1. `cmd/ainovel-cli/main.go` — nếu upstream sửa đúng vùng cờ/parse. Giải: **giữ cả hai** —
   phần upstream mới + các dòng `--web` của bạn (chúng độc lập, chỉ cần đặt cạnh nhau).
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
