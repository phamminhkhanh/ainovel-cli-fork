# Journal: Production run validation 61 chương + review-gate/steer session

**Ngày:** 2026-07-09
**Loại:** Validation (end-to-end production) + tổng hợp phiên (feature + docs)
**Kết luận:** Pipeline production **PASS ổn định**. Lever còn mở: steer nội dung (romance ratio).

---

## 1. Ca test: "Werewofl sonnet 5"

- Run: `workspace/novepro_02/output/jobs/run-002/` — `fresh_profile`, target 300 chương, budget $50.
- Profile: `werewolf-sonnet-5.md` (werewolf romance + power progression, thị trường VN).
- Trạng thái lúc chốt: **61/300 chương**, phase=writing, volume 1 arc 5, đang chạy → user tắt (checkpoint-safe).

## 2. Sức khỏe hệ thống — PASS

| Chỉ số | Kết quả | Ý nghĩa |
|---|---|---|
| Chương liền mạch | 1–61 không thủng số | checkpoint/resume chuẩn, tắt/mở lại an toàn |
| Wordcount | 9–15k rune/ch suốt 61 ch (ch61 ≈10.7k ≈2500 từ) | **fix non-CJK giữ vững**, không tụt chương ngắn |
| Đổi model | dùng cả `grok-4.5` + `deepseek-v4-pro` + `glm-5.2`, continuity KHÔNG gãy | engine + Cockpit chịu được model-switch |
| Chi phí | $2.79 / 61 ch ≈ **$0.046/ch**, saved $8.37 nhờ cache (cache_read 18.7M), 5.6% budget | rất hiệu quả |
| Editor review | chấm theo arc (12/28/42…), verdict polish/accept, không critical | vòng review tự động chạy |

→ **Đủ xác nhận pipeline production chạy ngon.**

## 3. Chất lượng nội dung

- **Nền móng: xuất sắc** — hấp thụ đúng bài học review-gate: nhân vật có giới hạn cứng (aura Alpha vô hiệu hoá đọc-vị → đọc sai + trả giá ở mốc cụ thể), cái giá ở ch35-40 & ch148 (point-of-no-return), phản diện có tên (Vũ Lang) + tuyến phản bội gài sẵn (Minh Khang), tỷ lệ "sủng" bắt buộc, market-fit VN + ngưỡng kiểm duyệt, throughline 3 tầng sợ hãi.
- **Prose: tốt, xuất bản được** — ch1 hook mạnh; đấu trí forensic (ch44/50) chất noir cô đọng; continuity chặt.
- **Drift khi đổi model** — Grok (ch1-50) noir cô đọng; DeepSeek (giai đoạn sau) giãn/kể/recap nhiều hơn, **lạm dụng "không phải X mà là Y"** (rule profile cấm), "bond nền ấm đều" lặp 20+ lần (Grok era).

## 4. Lưu ý nội dung (không phải lỗi hệ thống)

`strand_history`: sau ~ch16 **không còn strand `fire`** (romance-forward); ch45-61 gần như toàn `constellation` (plot/ensemble). → tỷ lệ romance/"sủng" mà profile bắt buộc (≥3/10) **đang bị bỏ**, truyện nghiêng chính-trị/bí-ẩn. `run.json` không có `steer_history` → chưa steer lần nào (cố ý để test tự nhiên). Đây là chuyện **steer**, không ảnh hưởng độ ổn định.

## 5. Đã build + verify trong phiên (đều additive, `internal/entry/web/`)

- **Health strip** (`prodrun_health.go`): 5 metric (progress/rewrite_rate/cost_pace/budget/persist), pure fn + test.
- **Soát nền móng Step 4** (profile): checklist + copy prompt review 13 trục cho LLM ngoài.
- **Foundation Gate review**: copy review (LLM ngoài) + copy bundle Review&Edit (agent IDE sửa 5 file nền móng), trục SSOT `FOUNDATION_REVIEW_AXES` (có trục "trung thành profile" bắt Architect drift).
- **Polish prompt sinh** (`profileStudioSystemPrompt`): frame-first + long-novel survival rules + market-fit + anti-AI-tell; generate chuyển sang **SSE streaming**.
- **Steer-on-resume** (đúng seam host `SetPendingSteer`/`Resume`, 0 đụng host): Dừng → Tiếp tục kèm steer → ghi `pending_steer` vào sandbox `run.json` → headless `Resume()` inject. Có guard `runDirHasExistingOutput` + 3 test. **Chưa áp cho run này.**
- **Persist-hardening (P0/P1/P2)**: `saveLocked` retry (Windows lock), `PersistError` + health metric, target-reached luôn kill.
- **Docs**: `05-REVIEW-TRUYEN.md` (playbook review, định khung thể loại+nước+văn hóa cho VN/EN/ES) — tham chiếu từ `AGENTS.md` + `CLAUDE.md` + `01`; cập nhật `02-WEB-UI.md`.

## 6. Còn mở / backlog

- **Áp steer** cho run werewolf để nắn romance ratio + siết giọng (dùng steer-on-resume vừa build).
- **AI soát lỗi profile trong app** (`/api/profiles/review`) — mới có checklist + copy ngoài.
- **Review timeline** khi writing (chương mở đầu / ranh giới arc / drift).
- **Commit hygiene**: working tree gom nhiều mảng (review-gate/health + persist-fix + steer-on-resume + docs) — nên tách commit theo chủ đề (hỏi user trước khi commit).
- Non-CJK wordcount: mới workaround per-run; fix gốc cần upstream.
