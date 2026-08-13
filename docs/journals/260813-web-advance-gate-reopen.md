# 260813 — Port advance gate + reopen sang Web UI

> Loại: feature port (TUI → Web UI) · Additive-only · ~139 LOC code + ~100 LOC test

## Bối cảnh

Web UI (fork) thiếu 2 tính năng TUI có: **advance gate** (`/review on|off`, `/next`) và **reopen** (`/reopen [hướng]`). User dùng 100% UI → khoảng trống thật. Host API cả 3 method sẵn sàng từ lâu (`SetAdvanceMode`/`AdvanceOneChapter`/`Reopen`), chỉ thiếu endpoint HTTP + nút UI.

## Đã làm

3 endpoint mới trong `internal/entry/web/` (handlers.go + server.go):

| Path | Host call |
|---|---|
| `POST /api/advance/mode` | `SetAdvanceMode(mode)` |
| `POST /api/advance/next` | `AdvanceOneChapter()` |
| `POST /api/reopen` | `Reopen(direction)` → `Resume()` (auto-resume như TUI) |

UI: 3 nút trong toolbar inputbar (`#advanceToggleBtn` toggle mode, `#advanceNextBtn` chỉ hiện review+idle, `#reopenBtn` chỉ hiện khi `phase=complete`) + 1 card `#advanceHoldCard` hiển thị advance hold reason. Render trong `updateControls()`, click wire trong `boot()`.

Test: 6 guard test (input validation, method, decode) — all pass. Full web suite 6.8s green.

## Quyết định

1. **Reopen auto-resume**: TUI làm vậy (`commands.go:199` → `resumeBook` → `Resume`). Web làm tương tự — đỡ 1 bước user, nhất quán. Nếu `Reopen` thành công mà `Resume` fail (race hiếm) → sách reopen-but-paused, user bấm Khôi phục (giống TUI).
2. **Nút toggle ở toolbar** (không modal): frequent-use control, luôn thấy.
3. **i18n**: nhãn hardcode VN inline trong `app.js` — theo convention toolbar có sẵn (`resumeBtn`/`abortBtn` cũng vậy), không dùng `app-i18n.js` map. Reviewer confirm đây là pattern đúng.
4. **409 over-broad mapping** trong `handleAdvanceNext`: tất cả error → 409 (kể cả store error). Consistent với `handleContinue`/`handleResume` sibling. Để nguyên — strict status code split là refactor riêng, không scope PR này.

## Thách thức / bài học

- **Test deadlock ban đầu**: thử test `workspaceMu` guard bằng lock-then-call → `sync.Mutex` không try-lock → deadlock 5 phút timeout. Fix: chỉ test input validation path (chạy trước Lock). Pattern này áp dụng cho mọi handler dùng `workspaceMu` — không test được concurrent-guard bằng cách đó (khác `handleStart` có `blockIfRecoverable` logic riêng trước Lock).
- **GitNexus HIGH risk false positive**: `detect_changes` báo HIGH do 4 file sửa, nhưng symbol "touched" đều là helper có sẵn không đổi signature. 3 handler mới chưa index (stale). Build + test confirm no regression. Bài: HIGH risk cần phân tích symbol-level, không dừng ở con số.

## Kết quả

- Build + vet + full web test pass.
- Code-review DONE_WITH_CONCERNS, no blocker.
- Docs cập nhật: [07-VAN-HANH-HE-THONG-MOI.md](../../07-VAN-HANH-HE-THONG-MOI.md) (bảng parity ✅→✅), [02-WEB-UI.md](../../02-WEB-UI.md) (mục API + UI mới).
- Không động upstream (`internal/host/`, `internal/entry/tui/`, `assets/prompts/`, `internal/domain/`).

## Follow-up (optional, deferred)

- Mock-Host success-path test cho response shape contract (hiện chỉ guard test).
- Strict status code split cho 5 mutation handler (nếu cần semantic chính xác).

## Post-review fix (2026-08-13)

- Sửa mixed-language toast "engine tự续跑" → "engine tự động tiếp tục".
- Strip CRLF → LF trong `app.js` (working tree khớp `.gitattributes` `eol=lf`).
- Rút gọn comment test vòng vo.
