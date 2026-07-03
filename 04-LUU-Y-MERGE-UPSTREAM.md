# 04 — Lưu Ý Merge Upstream & Big Change History

**Fork:** ainovel-cli-fork  
**Upstream:** https://github.com/voocel/ainovel-cli  
**Mục đích:** Ghi lại quy trình merge an toàn và các thay đổi lớn từ upstream ảnh hưởng đến fork. Dùng để sau mỗi lần merge, dev biết behavior nào đổi và cần test lại.

---

## 1. Quy tắc Merge An Toàn Cho Fork

Fork này theo chiến lược **additive-only**. Chỉ được thêm file mới trong `internal/entry/web/`; không sửa `internal/host/`, `internal/tools/`, `assets/prompts/`, `internal/entry/tui/` trừ khi upstream bắt buộc (và phải ghi chú lý do).

Quy trình mỗi lần merge:

```bash
# 1. Lấy metadata mới nhất
git fetch upstream

# 2. Xem có commits mới không
git log --oneline HEAD..upstream/main

# 3. Dry-run merge để phát hiện conflict (không đụng working tree)
git merge-tree $(git merge-base HEAD upstream/main) HEAD upstream/main

# 4. Merge thật nếu dry-run sạch
git merge upstream/main --no-edit

# 5. Gate bắt buộc sau merge
go build ./...
go vet ./...
go test ./internal/entry/web/...
```

**Lưu ý đặc biệt:** Nếu upstream thay đổi `assets/prompts/`, phải **rebuild binary** — browser refresh không đủ vì prompts được embed qua `go:embed`.

---

## 2. v0.6.1 — Pause Points & Completion Convergence

**Upstream tag:** `v0.6.1`  
**Merge date:** 2026-07-03  
**Merge commit:** `4544b53`  
**Range:** `6342dc3..1a4b5c5`

### Big changes

| Feature | Files chính | Impact |
|---------|-------------|--------|
| **用户验收停靠点** (user acceptance pause point) | `internal/host/pause.go`, `internal/host/flow/pause.go`, `internal/tools/save_pause_point.go`, `internal/domain/runtime.go` | Sau rewrite, engine có thể tự động pause để user kiểm tra. Web UI cần đọc pause state từ snapshot để hiển thị nút Continue thay vì hiển thị idle. |
| **完本收敛** (book completion convergence) | `assets/prompts/architect-long.md`, `internal/tools/commit_chapter.go`, `internal/tools/save_volume_summary.go`, `internal/domain/story.go`, `docs/architecture.md` | Engine quyết định kết thúc sách thông minh hơn. Có khái niệm **收官卷** (final volume): khi story đã đến điểm kết, architect có thể append volume với `"final": true`; sau khi final volume viết xong và qua editor review, hệ thống tự động mark complete. |
| **Notify event mới** | `internal/notify/notify.go`, `config.example.jsonc` | Thêm `pause_point` vào danh sách notify events. |
| **TUI update** | `internal/entry/tui/*.go` | Hiển thị pause state. Không ảnh hưởng Web UI vì Web UI dùng snapshot riêng. |

### Lưu ý sau merge v0.6.1

- **Prompts đã đổi** → rebuild binary trước khi chạy bất kỳ novel nào.
- **Behavior hoàn thành sách khác** → spike test 30 chương phải chạy trên v0.6.1, không dùng kết quả cũ.
- **Pause point** có thể làm engine dừng giữa chừng sau rewrite. Khi làm Production Cockpit sau này, nên tận dụng behavior này thay vì tự xây stop logic phức tạp.
- **Final volume** có nghĩa là `TotalChapters` trong snapshot có thể thay đổi khi story tiến gần kết — automation target phải linh hoạt hoặc dựa trên `phase == complete` thay vì chapter count cứng.
- **Windows file lock với export/rename**: Trong spike test v0.6.1, engine export thất bại trên Windows khi IDE/file watcher giữ file chapter đang mở trong lúc `os.Rename` ghi đè. Lỗi này tồn thiởi trước v0.6.1, không phải regression. Cần đóng watcher/IDE khi chạy production, hoặc upstream cần thêm retry loop/copy-delete fallback cho Windows.

---

## 3. Production Cockpit MVP (tab Sản xuất)

**Ngày thêm:** 2026-07-03  
**Files mới toàn bộ trong `internal/entry/web/`:**
- `prodrun.go`, `prodrun_runner.go`, `prodrun_handlers.go`, `prodrun_export.go`
- `prodrun*_test.go`
- `assets/app-production.js`
- Cập nhật `assets/index.html`, `assets/app-workspace.js`, `assets/app-i18n.js`, `assets/app.css`, `embed.go`, `assets_test.go`
- Tiếp tục đụng fork-exception files: `server.go` (route registration), `run.go` (start wiring)

### Lưu ý sau merge khi có Production Cockpit

| Vùng | Tác động / Cần làm |
|------|--------------------|
| **Conflict** | Các file `prodrun*.go` và `app-production.js` nằm trong `internal/entry/web/`, không nằm trong upstream → merge upstream/main sẽ **không conflict** trên chúng. |
| **Rebuild binary** | Mọi thay đổi ở `assets/` đều cần `go build ./...` — browser refresh không đủ vì file được embed qua `go:embed`. |
| **Fork-exception files** | Nếu upstream sửa `internal/entry/web/server.go` hoặc `internal/entry/web/run.go`, phải reconcile lại việc đăng ký route `/api/prodruns*` và wiring `startWeb`/`web.NewServer`. Giữ additive-only: không xóa route cũ. |
| **`cmd/ainovel-cli/main.go`** | Upstream chưa có `--web` / `--headless`. Nếu upstream thêm CLI flags, đảm bảo `--web` vẫn start Web UI và `--headless` vẫn dùng được cho Production Cockpit runner. |
| **`meta/progress.json` schema** | Runner đọc field `completed_chapters` để biết khi nào đạt `targetChapters`. Nếu upstream đổi tên field hoặc format, cần cập nhật `readCompletedChapters`. |
| **`reviews/*.json` shape** | Runner đếm rewrites qua `verdict == "rewrite"`. Nếu upstream đổi cấu trúc review JSON, cần cập nhật `countReviewsAndRewrites`. |
| **Engine output path** | Cockpit dựa vào cwd-based output: mỗi run spawn `ainovel-cli --headless` với `Cmd.Dir = {runDir}`, nên output rơi vào `{runDir}/output/novel`. Nếu upstream thay đổi `bootstrap.Config.OutputDir` / `FillDefaults()` behavior, phải kiểm tra lại đường dẫn chapter/log/meta. |
| **Pause point v0.6.1** | Cockpit chỉ đọc pause marker từ `run.log` (read-only). Nếu upstream cung cấp API pause tốt hơn (ví dụ snapshot pause state hoặc RPC), có thể thay thế polling log. |
| **Windows file lock** | Export TXT của Cockpit là server-side concat, không dùng `os.Rename` nên không gặp lock của IDE/file watcher. Giữ behavior này; đừng chuyển sang gọi `s.eng.Export()` vì sẽ re-introduce Windows lock. |

---

## 4. Checklist Sau Mỗi Lần Merge

- [ ] `git fetch upstream` xong.
- [ ] Dry-run merge không báo conflict.
- [ ] `git merge upstream/main` thành công.
- [ ] `go build ./...` pass.
- [ ] `go vet ./...` pass.
- [ ] `go test ./internal/entry/web/...` pass.
- [ ] Đọc diff của `docs/architecture.md` nếu upstream thêm section mới.
- [ ] Kiểm tra `assets/prompts/` có thay đổi gì không.
- [ ] Rebuild binary nếu prompts/assets thay đổi.
- [ ] Chạy smoke test 1 chapter nếu prompts thay đổi nhiều.

---

## 5. Lịch Sử Thay Đổi Lớn

| Version | Date | Key Change | Action Required |
|---------|------|------------|-----------------|
| v0.6.1 | 2026-07-03 | Pause points + completion convergence | Rebuild binary; rerun spike test; update Web UI pause handling |
| post-v0.6.1 (fork) | 2026-07-03 | Production Cockpit MVP (tab Sản xuất) | Rebuild binary; check `server.go`/`run.go` after upstream merge; verify `progress.json`/`reviews/*.json` schema |
---

## 6. Link

- Report merge test chi tiết: [`plans/reports/upstream-merge-test-260703-1121-v061-pause-convergence-report.md`](plans/reports/upstream-merge-test-260703-1121-v061-pause-convergence-report.md)
- Architecture upstream: [`docs/architecture.md`](docs/architecture.md)
