# 07 — Vận Hành Hệ Thống Mới (Engine + Arbiter)

> **Áp dụng từ:** merge upstream 2026-07-23 (merge commit `2e78d4e`; loạt commit upstream `964eb06…5127e69`).
> **Thay thế:** kiến trúc Coordinator long-loop trong [01-TONG-QUAN-DU-AN.md](01-TONG-QUAN-DU-AN.md) — Coordinator đã bị upstream **xóa hoàn toàn**, doc 01 chỉ còn giá trị lịch sử về phần kiến trúc.
> **Đối tượng:** người vận hành viết truyện hằng ngày (TUI hoặc Web UI) — cần biết lệnh nào làm gì, khi nào can thiệp, sự cố thì nhìn vào đâu.
> Tài liệu kỹ thuật gốc (中文): [README.md](README.md) §架构 · [docs/engine-rfc.md](docs/engine-rfc.md) · [docs/engine-arbiter.md](docs/engine-arbiter.md) · [docs/chapter-advance-gate.md](docs/chapter-advance-gate.md) · [docs/voice-layer.md](docs/voice-layer.md) · [docs/import.md](docs/import.md). Lịch sử merge: [04-LUU-Y-MERGE-UPSTREAM.md](04-LUU-Y-MERGE-UPSTREAM.md).

---

## 1. Kiến trúc mới trong 5 phút

```
User gõ lệnh / steer
        │
        ▼
   Host (vỏ mỏng: start / resume / observe)
        │
        ▼
   Engine ── vòng lặp tuần tự, MỘT hành động mỗi vòng
        │  1. flow.Route(facts)  ── hàm THUẦN, code định tuyến quy trình
        │  2. Route trả nil      ── tra bảng kịch bản → Arbiter (LLM phán đoán)
        │  3. Gate kiểm tra giấy phép chương (advance gate)
        ▼
   Workers:  Architect (lập kế hoạch) · Writer (viết) · Editor (review)
        │    mỗi worker tự hoàn thành MỘT đơn vị việc rồi thoát
        ▼
   Store: Progress · PendingCommit · PendingRewrites · RunMeta · Checkpoints
          · meta/decisions.jsonl (audit mọi phán đoán của LLM)
```

| Thành phần | Quyết định gì | Ghi chú vận hành |
|---|---|---|
| **Engine** | Thứ tự thực thi, timeout, retry, circuit-breaker | Tuần tự, xác định — không còn long-loop tùy hứng |
| **flow.Route** | Bước tiếp theo (điều phối / giữ nguyên / xin phán đoán) | Hàm thuần từ dữ kiện Store — có test, không phải LLM |
| **Arbiter** | Chỉ các kịch bản mở: mở sách, can thiệp user, worker hỏng, deadlock | Mỗi kịch bản 1 prompt riêng, bị chặt bởi dữ kiện + hành động hợp lệ |
| **Workers** | Tự hoàn thành 1 đơn vị việc của mình | Không giám sát lẫn nhau; chỉ báo cáo về Engine |
| **Store** | Sự thật duy nhất | Mọi thứ (kể cả restart) suy ra từ đây |

### Ba nguyên tắc cốt lõi (khác hẳn hệ cũ)

1. **Bước tiếp theo phải suy ra được từ dữ kiện đã ghi** — không còn "LLM nghĩ nên làm gì tiếp".
2. **Code định tuyến quy trình, LLM chỉ phán đoán ở ranh giới** — mọi phán đoán đều có log audit trong `meta/decisions.jsonl`.
3. **Không fallback nuốt lỗi** — lỗi hệ thống = dừng rõ ràng kèm lý do, không giả vờ chạy tiếp.

### Arbiter chỉ mở đúng 4 kịch bản

| Kịch bản | Khi nào kích hoạt | Prompt |
|---|---|---|
| **开局裁定** `plan_start` | Mở sách mới; hoặc resume mà Store thiếu dữ kiện suy ra bước tiếp | `assets/prompts/arbiter-plan-start.md` |
| **用户干预** `intervention` | User gõ yêu cầu tự do (steer) | `assets/prompts/arbiter-intervention.md` |
| **失败裁定** `worker_failure` | Worker hỏng / structured output không hợp lệ sau retry | `assets/prompts/arbiter-failure.md` |
| **僵局裁定** `deadlock` | Cùng một việc lặp lại nhiều lần không tiến triển (circuit-breaker) | dùng chung failure |
| ~~完本争议~~ `completion_dispute` | **Standby — chưa mở**, không ảnh hưởng production | — |

---

## 2. Bảng lệnh TUI đầy đủ

Nguồn: `internal/entry/tui/commands.go`. Lệnh có **(idle)** chỉ chạy khi engine đang rảnh.

### Nhóm system

| Lệnh | Tác dụng |
|---|---|
| `/help` | Danh sách lệnh |
| `/model [role]` | Đổi model + reasoning strength theo role (thinking/fast/...) |
| `/config` | Thêm/sửa Provider, model, context window |

### Nhóm analysis

| Lệnh | Tác dụng |
|---|---|
| `/diag` | Chẩn đoán sức khỏe truyện đang viết |

### Nhóm writing

| Lệnh | Tác dụng | Ghi chú |
|---|---|---|
| `/review on\|off` | Bật/tắt **chế độ duyệt từng chương** | Mặc định `off` (auto). Xem §3 |
| `/next` **(idle)** | Cấp phép 1 chương mới sau khi duyệt xong | Chỉ dùng trong review mode |
| `/import <path> [--yes] [--story=open\|closed] [--continue] [--guide=<hd>]` **(idle)** | Import truyện ngoài (pipeline mới) | Không tham số = tiếp tục import dở. Xem §5 |
| `/reopen [hướng viết tiếp]` **(idle)** | Mở lại truyện **đã hoàn** để viết tiếp | Hướng đi qua Arbiter rồi tự resume. Xem §6 |
| `/cocreate` (alias `/plan`) | Tạm dừng, cộng tác hoạch định giai đoạn tiếp theo | Chỉ khi đang chạy |
| `/simulate` **(idle)** | Đọc `./simulate` → sinh/cập nhật hồ sơ phỏng tác | |
| `/importsim <profile.json>` **(idle)** | Import hồ sơ phỏng tác có sẵn | |
| `/export [path] [from=N] [to=M] [--overwrite]` | Xuất TXT/EPUB các chương đã hoàn | |

### Web UI hỗ trợ gì?

| Tính năng | TUI | Web UI |
|---|---|---|
| Start / Pause / Continue / Export / đổi model | ✅ | ✅ (như cũ) |
| Cocreate | ✅ | ✅ (`/api/cocreate/*`) |
| Import | ✅ | ✅ (endpoint fork riêng, `internal/entry/web/import.go`) |
| **Advance gate (`/review`, `/next`)** | ✅ | ✅ (`/api/advance/mode`, `/api/advance/next` — nút trong toolbar) |
| **`/reopen`** | ✅ | ✅ (`/api/reopen` — nút "Viết tiếp" chỉ hiện khi `phase=complete`, auto-resume) |

> Đã port 2026-08-13. Chi tiết endpoint + UI: [02-WEB-UI.md](02-WEB-UI.md). Host API dùng thẳng: `SetAdvanceMode` / `AdvanceOneChapter` / `Reopen` (trong `internal/host/host.go`). Reopen auto-resume như TUI (`commands.go:199` → `resumeBook`).

---

## 3. Duyệt từng chương — Advance Gate

### Hai chế độ

| Mode | Hành vi | Khi nào dùng |
|---|---|---|
| `auto` (mặc định) | Engine tự viết tiếp như cũ | Truyện đã ổn định, chạy production dài |
| `review` | Mỗi chương **forward mới** cần giấy phép; engine viết xong thì dừng chờ | 20-30 chương đầu của truyện mới; sau can thiệp lớn; sau import |

### Thao tác

```
/review on     → bật review mode (được lưu vào RunMeta, sống qua restart)
... engine viết xong chương → dừng chờ ...
/ngườ dùng đọc chương, quyết định/
/next          → cấp phép 1 chương mới (chỉ khi idle)
/review off    → về auto; xóa giấy phép đang có nhưng GIỮ nguyên hold một lần
```

### Ngữ nghĩa quan trọng

- Giấy phép **chỉ chặn chương forward mới**. Planning / review / rework / polish / resume đã hợp lệ vẫn chạy bình thường — bật review mode **không** làm engine đứng hình.
- **Hold một lần** do Arbiter đặt khi user steer, ví dụ:
  - "viết xong đợt này thì dừng cho tôi xem" → hold `boundary` (dừng ở ranh giới worker hiện tại)
  - "sửa xong mớ rewrite đó rồi dừng" → hold `rewrites_drained` (dừng khi hàng rework cạn)
- Hold là một lần: dùng xong tự xóa. Đổi mode không xóa hold.
- Không cần migrate dữ liệu cũ: sách viết bằng bản trước mở ra mặc định `auto`.

> Lưu ý: cơ chế **pause point của v0.6.1** (tool `save_pause_point`) đã bị upstream xóa — pause giữa chừng giờ do advance hold + cơ chế pause tự động của engine đảm nhiệm.

---

## 4. Chỉnh văn phong — Voice Layer

Chỉnh prompt văn phong **không cần sửa repo, không cần rebuild**. File override đọc lúc process khởi động → sửa xong **restart** là áp dụng (không hot reload).

### Thứ tự ưu tiên

```
<outputDir>/style/   (theo cuốn sách — bind outputDir)
        >  ~/.ainovel/style/   (toàn cục cho mọi sách)
        >  mặc định embed trong binary
```

### File nào làm gì

| File | Hành vi | Dùng khi |
|---|---|---|
| `voice.md` | **Nối thêm** vào prompt writer | Tiêu chuẩn viết, chống AI味, cấm nhắc lại chương trước, kỳ vọng tối thiểu |
| `anti-ai-tone.md` | **Nối thêm** vào tiêu chí bắt bài của editor | Thêm dấu hiệu AI cần phạt (vd: cấm "hắn nhếch môi") |
| `styles/<tên>.md` | **Thay thế/thêm mới** style | Trùng tên embed (`default`...) = thay thế; tên mới `[a-z0-9-]+` = style mới |
| `genres/<thể-loại>/style-references.md` | Tùy chọn, tham chiếu theo thể loại | |

### Khác biệt với `rules/`

| | `rules/` | `style/` (voice) |
|---|---|---|
| Bind | **cwd** (project) | **outputDir** (theo sách) |
| Nội dung | Luật nghiệp vụ (vd: `lang-vi.md` bắt viết tiếng Việt) | Văn phong / anti-AI |

### Tự động sẵn có

`stylestat` thống kê tics câu thực tế của truyện và trả về cho writer — không cần thao tác gì.

---

## 5. Import truyện ngoài (pipeline mới)

Pipeline viết lại hoàn toàn, segment-based, chính xác hơn cho sách dài: **phân tích → khoanh vùng → đoạn → tổng hợp**.

```
/import D:\truyen\nguyen-tac.txt --yes
/import D:\truyen\nguyen-tac.txt --story=closed --guide="chia theo hồi, không theo chương"
/import            ← không tham số = tiếp tục phiên import đang dở
```

- **Không còn `from=N`** — resume tự suy ra từ workspace.
- `--guide` nhận ngôn ngữ tự nhiên để chỉnh cách chia đoạn.
- Web UI có endpoint import riêng của fork (`internal/entry/web/import.go`) — dùng như trước.

---

## 6. Viết tiếp truyện đã hoàn — `/reopen`

```
/reopen viết phần 2: nhân vật chính xây dựng thế lực mới ở ngoại vực
```

- Chỉ chạy khi idle và truyện đã `complete`.
- Hướng viết tiếp đi qua **Arbiter phán đoán → inject → tự resume**, không phải nối thẳng vào.
- Use case: phần 2, ngoại truyện, mở rộng kết.

---

## 7. Can thiệp giữa chừng (steer)

Gõ yêu cầu tự do như cũ — nhưng giờ đi qua kịch bản `intervention` của Arbiter, phân loại rõ:

| Loại yêu cầu | Arbiter làm gì |
|---|---|
| Câu hỏi thuần ("hiện có bao nhiêu 伏笔 chưa thu?") | Trả lỗi, không đổi gì |
| Rule dài hạn ("từ giờ cấm dùng từ X") | Ghi rule, áp cho các chương sau |
| Đổi cấu trúc ("đổi mục tiêu về 80 chương") | Điều phối lại planning |
| Rework ("chương 12 tệ, viết lại") | Tạo `PendingRewrite` + có thể đặt hold chờ user duyệt |

- `PendingSteer` được bảo vệ chống crash: xử lý xong mới xóa — restart không mất yêu cầu.
- "Sửa xong cho tôi xem" = rework + hold `rewrites_drained` → engine tự dừng đúng lúc để user đọc.

---

## 8. Sự cố & tự phục hồi

| Triệu chứng | Nguyên nhân | Hệ thống làm gì | User làm gì |
|---|---|---|---|
| Model trả sai format | structured output không hợp lệ | `llmcontract` validate → `llmretry` retry → replay → phán đoán thất bại | Thường không cần làm gì; nếu lặp hoài → kiểm tra model/network |
| Worker lỗi không phục hồi được | lỗi hệ thống (API chết, config sai...) | **Dừng rõ ràng**, không nuốt lỗi | Đọc log lỗi, sửa nguyên nhân, chạy lại — resume từ checkpoint, không mất dữ liệu |
| Cùng việc lặp không tiến triển | deadlock | Circuit-breaker → phán đoán deadlock → pause chờ can thiệp | Đọc lý do pause, steer hướng gỡ hoặc chỉnh plan |
| Engine tự pause | deadlock / failure abort / gate lỗi | `pauseWithNotify` (lifecycle=paused) + notify | Đọc event log trong TUI (hoặc `meta/`), xử lý rồi Continue |
| Bị content filter chặn | provider chặn nội dung | Retry → phán đoán → pause kèm gợi ý đường thoát | Chỉnh nội dung nhạy cảm hoặc đổi provider |
| Import dở giữa chừng | tắt máy/crash | Resume tự suy ra từ workspace | `/import` không tham số |

**Nguyên tắc vàng của hệ mới:** engine dừng ≠ hỏng. Dừng rõ ràng + lý do tốt hơn nhiều so với loop vô hạn nuốt lỗi như hệ cũ.

---

## 9. Production Cockpit — tab Sản xuất (headless)

Cockpit = tab **Sản xuất** trong Web UI: xếp job headless, giám sát tiến độ/chi phí, xuất TXT/EPUB. Hướng dẫn dùng đầy đủ: [docs/production-cockpit.md](docs/production-cockpit.md). Mục này chỉ ghi **thay đổi vận hành sau merge Engine+Arbiter** (verify trong code 2026-07-23).

### Vẫn hoạt động như cũ

- Spawn child `--headless`, poll `meta/progress.json` (chương/chi phí), kill child khi đạt target chương.
- Đếm review/rewrite từ `reviews/*.json` — tool `save_review` còn nguyên trong hệ mới.
- Xuất TXT/EPUB, sync workspace, health strip.

### Thay đổi do engine mới

| Điểm | Hệ cũ (Coordinator) | Hệ mới (Engine+Arbiter) | Hệ quả cho Cockpit |
|---|---|---|---|
| Nguồn pause | Tool `save_pause_point` (v0.6.1, đã xóa) | Engine tự pause: deadlock circuit-breaker, worker-failure abort, gate lỗi (`pauseWithNotify`) | Trạng thái `Tạm dừng` đổi ý nghĩa: không còn "pause point sau rewrite" |
| Khi engine pause | Process chờ input (headless treo ở pause point) | `runEnded` → gửi `done` → **child process exit code 0** | Cockpit **không còn job paused sống** — engine pause = process thoát |
| Log pause | `等待用户输入` / `用户暂停` | `已暂停等待人工介入` → `引擎停止 (已完成 N 章)` | **Marker cũ không match nữa** (xem bẫy bên dưới) |
| Advance gate | (không có) | Chỉ gắn TUI; state persist trong `meta/run.json` | Job Cockpit luôn `auto` — sandbox bị ép auto sau khi seed (xem bẫy 2), không duyệt từng chương |

### Bẫy 1 — job pause bị gán nhãn sai "Hoàn thành" (ĐÃ FIX)

1. Runner poll marker pause trong `run.log`: `等待用户输入 / 等待输入 / paused / 用户暂停` — engine mới log `已暂停等待人工介入` rồi `引擎停止 (已完成 N 章)`, **không marker nào khớp**.
2. Child exit 0 → `waitProc` thấy `err == nil` → gán job **`Hoàn thành`** dù truyện mới xong ví dụ 5/50 chương.

**Fix (commit `963b848`):** `waitProc` phân loại theo tiến độ thật qua helper `runFinished()` — `phase=complete` **hoặc** `chapters >= target` → `completed`; ngược lại → `paused` + stopReason `engine_paused`. Marker list thêm `已暂停`.

→ Job dừng giữa chừng giờ hiện **`Tạm dừng` / lý do `engine_paused`**. Mở `run.log` tìm `已暂停`, và `meta/decisions.jsonl` trong runDir để biết arbiter đã phán gì.

### Bẫy 2 — job `continue_workspace` bế tắc vĩnh viễn vì advance gate (ĐÃ FIX)

1. `advance_mode` / `advance_permit_chapter` / `advance_hold` nằm trong **`meta/run.json`** (`domain/runtime.go`).
2. `shouldExcludeWorkspaceSeed` không loại `meta/run.json` → seed mang nguyên mode từ workspace chính vào sandbox.
3. Engine init **giữ** mode đã persist (`store/run_meta.go`: `meta.AdvanceMode = existing.AdvanceMode`).
4. Nếu bạn từng gõ `/review on` ở TUI → job pause ở chương forward đầu tiên, chờ `/next` mà **web không có endpoint nào gửi được** → treo mãi.

**Fix (cùng loạt):** sau khi seed, `forceSandboxAutoAdvance()` ép sandbox về `auto` + clear permit/hold. Chỉ ghi trong sandbox — workspace chính giữ nguyên `/review` của bạn. Không exclude `meta/run.json` khỏi seed được vì `plan_start` là dữ kiện khôi phục duy nhất khi crash ở giai đoạn quy hoạch.

### Checklist chạy batch sau merge

- [ ] Chạy thử 1 job nhỏ (2-3 chương) — xác nhận spawn/poll/kill/export bình thường.
- [ ] Job dừng sớm: đọc lý do dừng — `engine_paused` = engine tự pause, không phải viết xong.
- [ ] Muốn duyệt từng chương thì dùng TUI (`/review on`) — Cockpit/headless luôn chạy `auto` (cố ý).

---

## 10. Audit — "ai đã quyết định gì?"

Mọi phán đoán của LLM trong runtime ghi append-only vào:

```
<outputDir>/meta/decisions.jsonl
```

Mỗi dòng 1 `DecisionRecord`: kịch bản, lập luận, hành động được chọn, dữ kiện tham chiếu (không copy nguyên văn). Khi truyện đi lệch hướng, đọc file này để truy vết quyết định đã dẫn tới.

---

## 11. Checklist nâng cấp từ bản cũ (Coordinator)

- [ ] `go build ./...` — rebuild binary (prompts đổi nhiều: writer/architect/editor + 3 prompt arbiter mới).
- [ ] Chạy thử 1 truyện mới 3-5 chương ở mode `auto` — xác nhận pipeline bình thường.
- [ ] Thử `/review on` → viết 1 chương → engine dừng chờ → `/next` → chạy tiếp.
- [ ] Truyện đang viết dở bằng bản cũ: **backup thư mục output** trước lần chạy đầu với binary mới, sau đó resume bình thường (kịch bản `run_resume` đã được upstream validate).
- [ ] Nếu có file `~/.ainovel/rules/` custom: vẫn hoạt động như cũ (rules bind cwd, không đổi).
- [ ] Nếu muốn chỉnh văn phong: tạo `~/.ainovel/style/voice.md` hoặc `<outputDir>/style/anti-ai-tone.md` rồi restart — không sửa `assets/prompts/` nữa.
- [ ] Production Cockpit: đọc §9 (bẫy gán nhãn "Hoàn thành" khi engine pause) trước khi chạy batch.

---

## 12. Khác biệt lớn nhất so với hệ Coordinator cũ

| | Coordinator cũ (đã xóa) | Engine + Arbiter mới |
|---|---|---|
| Điều phối | LLM long-loop tự quyết mọi bước (tới 100k turn) | Code xác định (`flow.Route`); LLM chỉ 4 kịch bản |
| Nhìn thấy quyết định | Khó audit | `meta/decisions.jsonl` append-only |
| Lỗi hệ thống | Có fallback nuốt lỗi | Dừng rõ ràng + lý do |
| Dừng chờ duyệt chương | Không có | Advance gate `/review` + `/next` + hold một lần |
| Pause sau rewrite (v0.6.1) | Tool `save_pause_point` | Bị xóa — thay bằng hold `rewrites_drained` |
| Chỉnh văn phong | Sửa `assets/prompts/` + rebuild | Voice layer file override + restart |
| Import | Pipeline cũ, resume qua `from=N` | Segment pipeline, resume tự suy ra |
| Truyện đã hoàn | Hết đường viết | `/reopen` viết tiếp |
| 模型/网络异常吞错 | Có | Đã xóa hoàn toàn |

---

*Cập nhật lần cuối: 2026-07-23 (sau merge `2e78d4e`). Khi upstream đổi behavior, cập nhật doc này + ghi merge note vào [04](04-LUU-Y-MERGE-UPSTREAM.md).*
