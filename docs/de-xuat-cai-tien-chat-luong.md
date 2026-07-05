# Đề xuất cải tiến — Từ "engine sinh truyện" đến "dây chuyền sản xuất truyện bán được"

> **Tính chất:** Doc thiết kế / bàn luận. KHÔNG phải changelog, chưa có gì được code.
> **Mục tiêu:** Ghi lại các thứ cần thêm/cải tiến để một người dùng có thể **dễ dàng tạo ra
> truyện chất lượng thật sự đọc được, đủ để bán**, với giả định môi trường:
> - Model tháng 6/2026 đủ mạnh, context window ~1M token.
> - Người dùng muốn kiếm tiền bằng serialized fiction (Dreame / GoodNovel / Royal Road / KDP…),
>   tức là **chạy nhiều đầu sách**, không phải nuôi 1 cuốn.
>
> **Ràng buộc kiến trúc bắt buộc tuân theo:** additive-only. Mọi thứ dưới đây phải nằm trong
> `internal/entry/web/` (+ tối đa 1-2 flag ở `cmd/ainovel-cli/main.go`). KHÔNG sửa
> `internal/host/`, `internal/tools/`, `assets/prompts/`, `internal/entry/tui/`.
> Xem [AGENTS.md](../AGENTS.md), [02-WEB-UI.md](../02-WEB-UI.md), [04-LUU-Y-MERGE-UPSTREAM.md](../04-LUU-Y-MERGE-UPSTREAM.md).

---

## 0. Đặt lại bài toán

Khi model đủ mạnh + context 1M, **bottleneck dịch chuyển**. Việc engine hiện tại lo nhất —
giữ nhất quán, không quên 伏笔, không lạc văn phong — model mạnh + context to tự lo phần lớn
(thậm chí có thể nạp gần cả cuốn thay vì nén tóm tắt 4 tầng, ít mất nuance hơn).

Vấn đề mới, mà engine hiện **không đo và không kiểm**, gồm 3 nhóm:

| # | Bottleneck mới | Engine hiện tại có gì? |
|---|----------------|------------------------|
| A | **"Đúng nhưng nhạt"** — truyện nhất quán nhưng người lạ không trả tiền đọc tiếp (hook yếu, nhịp phẳng, "AI-tell"). | Editor review 7 chiều là **QC kỹ thuật** (đo *đúng*), không đo *hấp dẫn*. `anti-ai-tone.md` chỉ là prompt reference, không ai *đo* AI-tell theo thời gian. |
| B | **Kinh tế theo danh mục** — vận hành 5-10 cuốn song song, cap theo tổng budget/rate-limit. | Cockpit chặn cứng **1 job/lần**. UI là book-first (chọn 1 truyện → workspace). |
| C | **Vòng lặp người-duyệt trước khi "chốt"** — không ai để AI tự bấm publish khi có tiền thật; và cần duyệt **nền móng** (outline/thế giới quan/twist/bài học) *trước khi* đổ hàng trăm chương. | Full-auto 100%. Gate foundation→writing là tự động tuyệt đối, không có điểm dừng (xem §4). |

Hệ quả định hướng: **Cockpit nên là trung tâm (fleet), single-book view trở thành công cụ soi
kỹ 1 cuốn khi cần** — ngược lại quan hệ hiện tại (single-book có thêm tab Sản xuất).

---

## 1. Foundation Gate — duyệt nền móng trước khi viết  ⭐ ưu tiên cao nhất

> ✅ **Foundation Gate đã ship** (2026-07-05) — xem
> [journal](journals/260705-foundation-gate.md). Có: Approve, Reject, **Sửa tay + reveal thư mục**,
> và **Revise = regenerate-with-feedback** (chính là lối (c) của §1b bên dưới). CHƯA làm: steer vào
> Host sống (lối (a)/(b) của §1b) — và cố ý không làm vì phá additive-only.

Đây là thứ người dùng thấy đau nhất: **outline, thế giới quan, twist, bài học (lesson) là
những thứ phải chốt ngay từ đầu, nhưng Cockpit gần như không cho xem trước và góp ý.**

### Hiện trạng (đã xác nhận bằng code)

1. **`ask_user` bị vô hiệu trong headless/Cockpit.** `prodrun_runner.go` spawn child chỉ set
   `Stdout`/`Stderr = logFile`, **không set `Stdin`** → stdin = /dev/null. Khi Architect gọi
   `ask_user`, `terminalAskUser.readLine()` gặp EOF → lỗi → `AskUserTool.Execute` tự degrade:
   *"用户交互失败… 请根据你的判断自行决策并继续"* (tự quyết rồi viết tiếp).
   → **Architect luôn tự quyết một mình, người dùng không được hỏi.**
2. **Gate foundation→writing là tự động tuyệt đối.** Cuối `save_foundation.Execute`, khi
   `FoundationMissing()` rỗng → `UpdatePhase(PhaseWriting)` **ngay trong cùng lượt gọi tool**,
   Writer bắt đầu chương 1 liền. Không có khoảng dừng để đọc outline.
3. **Pause point (v0.6.1) không cứu được** — nó chỉ kích hoạt *sau* khi rewrite queue rỗng
   (duyệt bản đã viết), không phải *trước* khi viết.
4. **Web UI thường** có `/api/outline|world|characters` nhưng chỉ đọc **thụ động sau khi**
   engine đã lưu và **đang viết tiếp** — không phải cổng duyệt trước.

### Vì sao vốn thế

Hệ quả trực tiếp của nguyên tắc `LLM quyết định, Host phục vụ` (Host không được "chờ duyệt")
+ mục tiêu ban đầu của Cockpit là *headless, không người canh, chạy hàng loạt qua đêm*.

### Đề xuất (additive-only) — phân rã 2 milestone

Thêm một **Foundation Gate ở tầng Cockpit**, KHÔNG sửa Host / `ask_user`. Sau QA review, tách
làm 2 milestone vì phần "duyệt & xem" khả thi ngay, còn phần "steer & retry" vướng ràng buộc thật.

**Cơ chế detect (đã xác minh):** KHÔNG có event `foundation_ready` — chuyển phase nằm trong
result JSON của `save_foundation` + ghi vào `progress.json`, `consume()` chỉ đọc Events/Stream/Done
(`headless/run.go`). Nên detect bằng **poll `meta/progress.json`** — thứ Cockpit *đã* poll sẵn
(`readCompletedChapters`). Khi thấy `phase == writing` (hoặc `completed_chapters` vừa > 0) → kill
child, set trạng thái review. Không cần event mới, không cần sửa Host.

**Milestone 1a — Foundation Review (thuần Cockpit) — ✅ ĐÃ LÀM (2026-07-05):**
1. Cockpit poll thấy phase vừa chuyển `writing` → kill child, set `ProdRun.Status = awaiting_review`.
2. Hiển thị premise/outline/world/characters qua GET endpoint tái dùng từ `content.go`.
3. **Approve** (restart cùng run dir, headless native `Resume()`) và **Reject** (hủy job).

> ⚠️ **Đây là best-effort pause, KHÔNG phải hard gate.** Poll mỗi 5s; engine chuyển `phase=writing`
> **đồng bộ** trong `save_foundation` rồi dispatcher `Steer` "viết chương 1" ngay. Nên tệ nhất
> Writer đã kịp bắt đầu/draft dở chương 1 trước khi poll chặn. Thực tế viết trọn 1 chương tốn hàng
> chục giây nhiều lần gọi model → pause gần như luôn chặn *trong lúc* chương 1 đang draft → tệ nhất
> mất **một phần chương 1**, không bao giờ mất hàng trăm chương. Hard gate zero-token cần stop hook
> trong `internal/host`/`headless` (phá additive-only) → **cố ý không làm**. Xem
> [journal](journals/260705-foundation-gate.md).

**Milestone 1b — Steer & retry (cần chốt cơ chế, KHÔNG hiển nhiên):**
- Vấn đề thật: `Steer` là method của Host, cần **một Host đang sống** để đẩy text vào vòng lặp.
  Nhưng ở `awaiting_review`, child đã exit → không có Host để Steer. Ba lối, đều có giá:
  - (a) giữ Host sống ở chế độ chờ → **đụng Host**, phá additive-only.
  - (b) Cockpit sửa file outline/JSON rồi restart → **vi phạm "không sửa JSON tay"**.
  - (c) lưu góp ý thành feedback, spawn lại một run **regenerate foundation** với prompt kèm feedback
    → additive, nhưng **không phải "Steer" đúng nghĩa** mà là "sinh lại nền móng có phản hồi".
  → Khuyến nghị chọn (c), và gọi đúng tên nó trong UI để không hứa sai.

**Resume (Approve) cũng cần chốt cơ chế** — child đã exit, "resume" thực chất là spawn lại. Dùng
đúng đường `continue_workspace` sẵn có (seed workspace đã có foundation → `--headless` → native
`Resume()` vào writing). Đây là con đường additive rõ nhất, không cần signal/pipe vào child.

**Về token:** "Reject không tốn token Writer" — đúng, nhưng **Architect đã tiêu token** để sinh
foundation. Nói chính xác: *tiết kiệm toàn bộ chi phí Writer* (phần đắt nhất), không phải zero-cost.

**Phạm vi:** áp cho `fresh_profile` (chặn trước ch.1). Với `expand_arc`/`append_volume` sang
vòng cung/tập mới thì **khó hơn** vì lúc đó engine đã ở phase `writing` liên tục — detect "vừa mở
arc mới" phải dựa mốc khác (vd `progress.VolumeArc` đổi), để milestone sau.

> Ghi chú "lesson/theme": hiện `save_foundation` không có field `theme`. Nếu nhét vào `compass`
> (EndingDirection) thì **Market Judge/Editor không đọc compass sẽ miss** — nên để trong `premise.md`
> (là thứ mọi role đọc), hoặc chấp nhận đụng tool để thêm field (ngoài phạm vi additive).

---

## 2. Market Judge — người đọc lạ, không phải biên tập viên  ⭐ ưu tiên cao

### Vấn đề

Editor review 7 chiều (nhất quán / hành vi nhân vật / nhịp / mạch tự sự / 伏笔 / 钩子 / thẩm mỹ)
là QC kỹ thuật, và nó **tự chấm bài do chính hệ thống viết ra** → lỏng tay ở đúng chỗ quan
trọng nhất: người lạ trả tiền có đọc tiếp không.

### Đề xuất

Thêm **role thứ 5: Market Judge**, độc lập, **không đọc outline/world** (tránh thiên vị nội bộ),
chỉ đọc như một reader thật:
- Chấm chapter 1-3 theo tiêu chí "tôi có bỏ tiền đọc tiếp không".
- Chấm **hook cuối mỗi chương** độc lập với review nội bộ.
- Soi **AI-tell**: cấu trúc "không phải X mà là Y", motif lặp, purple prose (dựa `anti-ai-tone.md`).

Khớp triết lý repo: chỉ thêm 1 sub-agent, đưa quyết định vào LLM, KHÔNG hardcode logic chấm ở Host.

> Cân nhắc additive: nếu thêm sub-agent phải đụng `internal/agents/build.go` (upstream). Cách
> giữ additive-only: chạy Market Judge như một **pass thứ hai ở tầng Cockpit** — Cockpit tự gọi
> model (qua chính binary hoặc API) trên các file chương đã commit, không nhét vào vòng lặp Host.
> Đây là lựa chọn thiết kế cần chốt: role trong engine (mạnh hơn, nhưng đụng upstream) vs
> post-pass ở Cockpit (thuần additive, nhưng tách rời vòng viết).

---

## 3. Cockpit — fleet & vận hành sản xuất

Xếp theo ưu tiên cao → thấp:

1. **Resume thật cho pause, không chỉ Stop/Export.** v0.6.1 đã có pause point ở engine, nhưng
   Cockpit hiện chỉ dò marker trong `run.log` rồi để trạng thái `paused` read-only. Chạy hàng loạt
   không người canh mà mỗi lần pause phải dựng lại continue-workspace bằng tay → mất hết lợi ích
   tự động. Cần nút **Tiếp tục** đẩy tín hiệu resume vào child.
2. **Dashboard AI-tell / hook / style-drift theo thời gian.** `internal/stylestat.Compute` là
   **hàm thuần deterministic**, trả `Stats` JSON (patterns per-chapter, top phrases, repeated
   sentences, ending shape, opening-time rate, title-format mix). Đã được gọi **online** trong
   `novel_context_builders.go` (nạp vào context Editor) + offline trong `internal/eval`. Cockpit
   lộ ra UI rất dễ theo additive: **tự gọi `Compute` trên file chương của run** (pure fn, không
   cần Host). ⚠️ **Nhưng toàn bộ regex là tiếng Trung** (`不是…而是`, `第N章`, 明喻…) → với truyện
   **tiếng Việt gần như bắt được 0 pattern**. Muốn AI-tell tiếng Việt phải viết bộ pattern mới
   (nằm được trong `internal/entry/web/` hoặc package mới, không đụng `stylestat` upstream).
3. **Bỏ giới hạn 1-job**, thay bằng hàng đợi có cap theo **tổng budget danh mục / rate-limit
   provider**, không cap theo số job. (`prodRunRunner.start()` hiện `if len(rr.running) > 0 → từ chối`.)
4. **Hàng đợi QA người.** Chương nào Market Judge chấm thấp, hoặc rewrite-loop > 2 lần → tự rơi
   vào "cần người đọc trước khi publish". Không tin full-auto 100% khi có tiền thật.
5. **Xuất drip + đa định dạng.** Hiện chỉ gộp 1 file TXT (`prodrun_export.go`). Platform
   serialized sống nhờ ra chương đều đặn → cần export theo lịch nhỏ giọt + EPUB (đang defer).
6. **Template hoá dòng sản phẩm.** Từ 1 profile mẫu, sinh biến thể (đổi tên/thế giới/nhân vật) để
   đẻ nhanh 5-10 cuốn cùng công thức đã bán được. Hiện mỗi profile viết tay.
7. **Cảnh báo chủ động** (Discord/Telegram webhook): khi pause, vượt ngân sách, AI-tell tăng vọt.
   Vận hành nhiều cuốn song song thì không ai ngồi F5 canh log. (`internal/notify` đã có khung.)

---

## 4. UI — Fleet-first

Home hiện tại: "chọn 1 truyện → workspace 7 tab". Đề xuất **màn hình danh mục (fleet) làm home**:

```
┌─ Fleet ──────────────────────────────────────────────────────┐
│ 📗 Cửu Tiêu Tôn      writing   ch.87/150  $12.40  hook:7.2↓  │
│ 💗 CEO's Contract    paused    ch.34/60   $4.10   AI-tell:LOW │
│ 🔍 Vòng Xoáy Đen     needs-QA  ch.12/40   $1.90   flagged×3  │
│ 📗 Đế Vương Chi Lộ   queued    —          —                   │
└────────────────────────────────────────────────────────────────┘
```

- Mỗi thẻ = 1 cuốn (không phải 1 job kỹ thuật). Click mới rơi vào workspace hiện tại.
- Cột chỉ số sức khoẻ: chương/mục tiêu, chi phí, xu hướng hook, mức AI-tell, số chương bị flag QA.
- Đảo quan hệ: Cockpit *chứa* single-book view, không phải single-book *có thêm* tab Cockpit.

---

## 5. Điểm căng thẳng cần thừa nhận thẳng

Kiến trúc gốc cấm rõ: **không Task/Job/Scheduler**, Host không giữ trạng thái orchestration
(`01-TONG-QUAN-DU-AN.md` §10, `docs/architecture.md` §10).

Fleet nhiều cuốn song song + lịch drip-release + hàng đợi QA **chính là một scheduler**. Cách giữ
hợp lệ: đặt toàn bộ trong `internal/entry/web/` (Cockpit đã là một store + runner riêng), KHÔNG
đụng `internal/host/`. Vậy nguyên tắc "Host ngu" vẫn giữ.

Nhưng phải nói thật: khi đó **Cockpit không còn là "tính năng phụ nhỏ"** — nó thành **một hệ
thống điều phối thứ hai sống cạnh Host**, có state riêng, lifecycle riêng, có thể có scheduler
riêng. Đó là cái giá thật của việc biến "công cụ sáng tác cá nhân" thành "dây chuyền sản xuất
bán hàng". Không sai với chiến lược additive-only, nhưng đừng giả vờ nó vẫn nhỏ như MVP hiện tại.

---

## 6. Thứ tự ưu tiên đề xuất

| Ưu tiên | Hạng mục | Vì sao trước | Đụng upstream? |
|---------|----------|--------------|----------------|
| P0 | **Foundation Gate** (§1) | Chặn lãng phí lớn nhất: viết 100 chương trên nền móng sai. Người dùng đau nhất. | 1 flag ở `main.go` (điểm chạm hợp lệ) + web-only |
| P0 | **Resume-on-pause** (§3.1) | Không có nó thì "tự động hàng loạt" là giả. | Web-only |
| P1 | **Market Judge** (§2) | Đo "hấp dẫn" thay vì chỉ "đúng". | Cần chốt: post-pass (additive) vs role (đụng build.go) |
| P1 | **Bỏ 1-job + fleet UI** (§3.3, §4) | Mô hình kinh doanh là đa đầu sách. | Web-only |
| P2 | **AI-tell dashboard** (§3.2) | Lộ `stylestat` sẵn có ra UI. | Web-only |
| P2 | **QA queue người** (§3.4) | An toàn trước publish. | Web-only |
| P3 | Drip export + EPUB (§3.5) | Hợp mô hình engagement platform. | Web-only |
| P3 | Template dòng sản phẩm (§3.6) | Tăng throughput danh mục. | Web-only |
| P3 | Alert webhook (§3.7) | Vận hành không cần canh. | Web-only (`notify` sẵn có) |

---

## 7. Nguyên tắc chọn model (nhắc lại, vì nó quyết định chất lượng hơn mọi feature trên)

> ⚠️ Model mạnh = truyện tốt. Không feature nào bù được model yếu. Đầu tư budget vào **Writer**
> (prose, tâm lý, đối thoại — cần sáng tạo nhất), tiết kiệm ở Coordinator/Editor. Chọn theo
> **tier hiệu suất tại thời điểm dùng**, không theo tên model cụ thể (LLM tiến hoá quá nhanh).
> Xem `01-TONG-QUAN-DU-AN.md` §9.1.

---

*Nguồn: tổng hợp từ phiên thảo luận thiết kế + đọc code `internal/entry/web/prodrun_*.go`,
`internal/entry/headless/`, `internal/tools/{ask_user,save_foundation}.go`, `internal/host/pause.go`.*

---

## Phụ lục A — QA review: xác minh, câu hỏi mở, quyết định cần chốt

> Kết quả rà lại toàn bộ đề xuất với code hiện tại. Mục tiêu: tách "đã chứng minh" khỏi "giả định",
> và phân rã P0 tới mức code được.

### A.1 Đã xác minh bằng code

| Claim | Nguồn | Kết luận |
|-------|-------|----------|
| `save_foundation` tự đẩy `PhaseWriting` khi foundation đủ | `internal/tools/save_foundation.go` (cuối `Execute`) | ✅ đúng |
| Headless spawn child KHÔNG set `Stdin` → `ask_user` degrade | `internal/entry/web/prodrun_runner.go` (`start`) | ✅ đúng |
| `consume()` chỉ đọc Events/Stream/Done, không expose tool result | `internal/entry/headless/run.go` | ✅ đúng → không có event `foundation_ready` |
| Không có event phase-change riêng | `internal/host/**` (chỉ `UpdatePhase` ghi progress, `router.Dispatch` đọc) | ✅ đúng → phải detect qua file |
| `ProdRun.Status` = queued/running/paused/completed/failed/cancelled | `internal/entry/web/prodrun.go` | ✅ thêm `awaiting_review` = schema change |
| `PausePointSentinel` chỉ pause sau khi rewrite queue rỗng | `internal/host/pause.go` | ✅ đúng, khác foundation-review |
| `prodRunRunner.start()` chặn cứng 1 job | `internal/entry/web/prodrun_runner.go` | ✅ đúng |
| `stylestat.Compute` là pure fn, output `Stats` JSON, regex tiếng Trung | `internal/stylestat/stylestat.go` | ✅ đúng → gap tiếng Việt |
| `CostUSD` đọc từ `usage.json` (`state.Overall.Cost`) | `internal/entry/web/prodrun_runner.go` (`readCostUSD`) | ✅ có tổng chi phí; **chưa có cost theo từng chương** |

### A.2 Câu hỏi chưa giải quyết (phải chốt trước khi code)

1. **Resume/Approve chính xác chạy lại thế nào?** Khuyến nghị: tái dùng `continue_workspace` (seed
   workspace đã có foundation → native `Resume()`), không phát minh signal/pipe vào child.
2. **Steer & retry khi không có Host sống** → chấp nhận định nghĩa lại thành "regenerate foundation
   với feedback" (lối (c))? Nếu không, phải đụng Host.
3. **Market Judge output là gì** — điểm số hay nhị phân đạt/không? Ngưỡng nào trigger rewrite, ngưỡng
   nào đẩy vào QA-queue người? Nếu rewrite thì **ai rewrite** (Writer theo feedback, hay Coordinator)?
   Nếu Market Judge tự trigger rewrite thì nó **vẫn can thiệp vòng viết**, chỉ ở tầng khác.
4. **Budget/rate-limit danh mục lấy từ đâu?** Hiện chỉ có `BudgetUSD` per-job, không có portfolio
   budget hay provider rate-limit ở store. Nhiều process song song **cùng 1 API key** sẽ tự đánh
   rate-limit vào nhau; model local thì đụng trần RAM/VRAM. Cần một lớp cap thật, không chỉ đếm job.
5. **Hook/AI-tell cho fleet dashboard** — `hook:7.2` chưa có nguồn dữ liệu (cần Market Judge hoặc
   phân tích riêng); `AI-tell` cần bộ pattern tiếng Việt mới. Mockup fleet ở §4 có 2 cột **chưa có
   backing data**, đừng vẽ UI trước khi có nguồn.

### A.3 Quyết định kiến trúc: khi nào tách `internal/entry/web/cockpit/`

`internal/entry/web/` hiện là HTTP adapter + store + child-runner. Khi thêm fleet scheduler + QA
queue + drip export + alert, nó vượt vai trò "entry adapter" và dễ vượt soft-cap ~600 dòng/file.
→ Khi fleet thành thật, **tách module con `internal/entry/web/cockpit/`** (job queue / scheduler /
state-machine mỗi cuốn) để giữ file nhỏ và trách nhiệm rõ. Vẫn nằm trong `web/` nên vẫn additive.

### A.4 Sửa nhỏ / chỉnh lại so với bản đầu

- **P0 Foundation Gate** phải làm **Milestone 1a trước** (detect-phase + `awaiting_review` +
  Approve/Reject). Steer & retry (1b) để sau, và gọi đúng tên "regenerate với feedback".
- **`theme/lesson`**: để trong `premise.md` (mọi role đọc), KHÔNG nhét vào `compass` (Market
  Judge/Editor không đọc compass → miss).
- **Market Judge**: chốt **post-pass ở Cockpit** để giữ additive-only; chốt output + ngưỡng +
  người rewrite *trước* khi code. Lưu ý chi phí: fleet 5-10 cuốn × 30-150 chương = rất nhiều token
  → cần budget cap riêng cho Market Judge.
- **Template dòng sản phẩm** có thể độc lập với fleet → có thể kéo lên **P2** nếu muốn tăng
  throughput sớm mà chưa cần multi-job.
- Link `../AGENTS.md`, `../02-WEB-UI.md`, `../04-LUU-Y-MERGE-UPSTREAM.md`: đã xác nhận tồn tại ở
  repo root, đường dẫn tương đối từ `docs/` đúng.

### A.5 Nguyên tắc thực thi

**Không nhảy vào implement toàn bộ Foundation Gate một lúc.** Thứ tự an toàn:
Milestone 1a (detect + review + approve/reject) → dùng thử → mới tới 1b (feedback loop) và
Market Judge. Mỗi bước phải trả lời xong câu hỏi tương ứng ở A.2.
