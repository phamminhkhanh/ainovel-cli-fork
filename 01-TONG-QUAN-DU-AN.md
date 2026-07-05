# Tổng quan dự án — ainovel-cli

> Bản tóm tắt tiếng Việt: **dự án là gì · triết lý thiết kế · flow hoạt động**. Đọc file này là đủ nắm cốt lõi, khỏi đọc lại toàn bộ source.
> Chi tiết kỹ thuật đầy đủ: [README.md](README.md) (中文) · [docs/architecture.md](docs/architecture.md).

---

## 1. Dự án là gì

**Engine sáng tác tiểu thuyết dài AI toàn tự động.** Nhập 1 câu yêu cầu → ra cả cuốn tiểu thuyết hoàn chỉnh (200~500 chương), không cần can thiệp thủ công.

- Một **Coordinator (LLM)** điều phối 3 sub-agent **Architect / Writer / Editor** trong một vòng lặp dài.
- **Host** chỉ làm vỏ mỏng: khởi động, khôi phục, quan sát, định tuyến.
- Viết bằng **Go 1.25**, giao diện **TUI (Bubble Tea)**, hỗ trợ headless (Docker/server).
- Đa nhà cung cấp LLM: OpenRouter / Anthropic / Gemini / OpenAI / DeepSeek / Qwen / GLM / Grok / Ollama / Bedrock + proxy tùy biến.

---

## 2. Triết lý thiết kế

> 🎯 **把复杂度从代码搬到模型里 — Dời độ phức tạp từ code vào model. Code càng ít, chỗ hỏng càng ít.**

```
┌─────────────────────────────────────────────────────────────┐
│  Quyết định SÁNG TÁC & PHÁN QUYẾT  ──────────►  giao LLM      │
│  (hiểu ngữ nghĩa / chất lượng / ý đồ)    model mạnh = tự tốt  │
│                                                               │
│  Định tuyến QUY TRÌNH  ──────────────────►  giao CODE         │
│  (đọc fact → tra bảng)              hàm thuần + test, lỗi ≈ 0 │
└─────────────────────────────────────────────────────────────┘
```

**5 trụ cột:**

| # | Nguyên tắc | Ý nghĩa |
|---|-----------|---------|
| 1 | **LLM quyết định, Host phục vụ** | LLM giữ **sáng tác + phán quyết**. `flow.Route` (code, Host) định tuyến **quy trình**: chương kế / hàng đợi rewrite / hậu kỳ cuối arc; trả `nil` ở ca裁定 (chọn planner, xử lý user steer, xuất tóm tắt) để LLM tự quyết |
| 2 | **Host nhỏ nhất · Prompt béo nhất · Tool mạnh nhất** | `host.go` ~1.2k dòng (cả package host ~4.7k); tri thức sáng tác nằm trong prompt (đổi văn phong = đổi prompt) |
| 3 | **Tách Fact ⇄ Instruction** | Tool chỉ trả sự thật (JSON), KHÔNG kèm lệnh; chỉ thị do `reminder` tính lại mỗi vòng |
| 4 | **Tầng fact phẳng** | Chỉ 3 loại sự thật: `Progress` + `Checkpoint` + `Artifact`. Không Task/Job/Scheduler/Workflow |
| 5 | **Quan sát chỉ được quan sát** | UI / `diag` đọc fact, không tạo fact, không động control flow. Không tự sửa |

**Logic kinh tế:** quyền quyết định nằm trong prompt + ngữ nghĩa tool → **model nâng cấp = cả hệ thống tự tốt lên**, Host không sửa 1 dòng. Hardcode logic ở Host → thành **lợi nhuận âm** khi model mạnh lên.

> 🔑 **Kỷ luật cốt lõi:** Khi muốn *"làm Host thông minh hơn"*, hỏi trước *"tại sao không làm LLM thông minh hơn?"*. Không trả lời được lý do *"Host bắt buộc"* thì đừng thêm code vào Host.

---

## 3. Kiến trúc phân tầng

```
┌──────────────────────────────────────────────────────┐
│  Entry      TUI / headless / startup                 │  nhận input, hiển thị
├──────────────────────────────────────────────────────┤
│  Host       vỏ mỏng: start / resume / route / observe│  KHÔNG quyết nghiệp vụ
│   ├── observer       event → UI/log                  │
│   ├── flow.Dispatcher  Route(fact) → Steer           │  ← hàm thuần định tuyến
│   └── budget / usage / notify                        │
├──────────────────────────────────────────────────────┤
│  Coordinator   1 LLM long-loop (MaxTurns=100_000)    │  bộ não điều phối DUY NHẤT
├──────────────────────────────────────────────────────┤
│  SubAgents   Architect · Writer · Editor             │  context + model ĐỘC LẬP
├──────────────────────────────────────────────────────┤
│  Tools  11 tool IO nghiệp vụ + tool điều phối/phụ trợ│  chỉ trả sự thật
├──────────────────────────────────────────────────────┤
│  Store       file system (tmp + rename, nguyên tử)   │
├──────────────────────────────────────────────────────┤
│  Domain      data thuần (Phase / Flow / Progress...) │
└──────────────────────────────────────────────────────┘
        Phụ thuộc MỘT CHIỀU: entry → host → agents → tools → store → domain
```

**4 vai trò Agent:**

| Agent | Nhiệm vụ | Tool chính |
|-------|----------|-----------|
| **Coordinator** | Điều phối toàn cục, xử lý phán quyết review + can thiệp user | `subagent` `novel_context` |
| **Architect** | Sinh premise, đại cương, nhân vật, world rules; 展开 vòng cung/tập theo nhu cầu | `save_foundation` `novel_context` |
| **Writer** | Tự chủ viết 1 chương: nghĩ → viết → tự kiểm → nộp | `plan_chapter` `draft_chapter` `check_consistency` `commit_chapter` |
| **Editor** | Review 7 chiều, sinh tóm tắt vòng cung/tập | `save_review` `save_arc_summary` `save_volume_summary` |

---

## 4. Flow hoạt động

### 4.1 Vòng đời tổng

```
1 câu yêu cầu
   │
   ▼
Architect ─ quy hoạch骨架 + chương vòng cung đầu
   │
   ▼
Writer ─ viết từng chương ──────────────┐
   │                                     │ (vòng lặp)
   ▼                                     │
Editor ─ review cấp vòng cung            │
   │                                     │
   ├── cần viết lại / mài giũa ──────────┘
   │
   ▼
Architect ─ 展开 vòng cung/tập kế tiếp (tham chiếu tóm tắt + snapshot)
   │
   ▼
... lặp đến hết ...  →  complete (cả cuốn hoàn thành)
```

### 4.2 Writer viết 1 chương (thứ tự tool BẮT BUỘC, nội dung tự chủ)

```
① novel_context     nạp ngữ cảnh (tóm tắt前情, 伏笔, trạng thái nhân vật, gợi ý chương liên quan)
② read_chapter      đọc lại前文 lấy lại giọng văn & nhịp
③ plan_chapter      构思 mục tiêu / xung đột / cung cảm xúc
④ draft_chapter     viết nguyên văn cả chương
⑤ check_consistency đối chiếu dữ liệu trạng thái (PHẢI sau draft)
⑥ commit_chapter    nộp终稿 → trả fact (arc_end_reached / next_chapter...)
```

Mỗi tool thành công → **ghi checkpoint** → crash có thể khôi phục chính xác đến từng bước.

### 4.3 State machine

```
Phase (chỉ TIẾN không LÙI):
   init ─► premise ─► outline ─► writing ─► complete

Flow (chỉ trong giai đoạn writing, chuyển qua lại):
   writing ⇄ reviewing ⇄ rewriting ⇄ polishing ⇄ steering
```

| Flow | Ý nghĩa |
|------|---------|
| `writing` | Viết chương kế tiếp bình thường |
| `reviewing` | Editor đang review |
| `rewriting` | Xử lý chương buộc phải viết lại |
| `polishing` | Xử lý chương chỉ cần mài giũa |
| `steering` | Đang đánh giá & xử lý can thiệp của user |

### 4.4 Tách Fact ⇄ Instruction (cơ chế cốt lõi)

Tool chỉ trả **FACT** (JSON). Chỉ thị do **2 kênh song song** tính lại từ fact mỗi vòng, KHÔNG nhúng trong tool:

```
        Tool trả FACT (arc_end_reached / pending_rewrites / final_verdict...)
                          │
        ┌─────────────────┴───────────────────┐
        ▼                                      ▼
① Reminder  (host/reminder/)         ② Flow Router  (host/flow/)
  HÀM THUẦN · MỖI pre-turn             HÀM THUẦN · tại sync tool boundary
  đọc Progress + Outline               Route(state) → Instruction
  → <system-reminder>                  → Steer "[Host 下达指令]" vào run hiện tại
  (flow / queue_guard / book_complete) (chương kế / rewrite / hậu kỳ arc)
        │                                      │
  KHÔNG vào lịch sử, tính lại mỗi turn   trả nil = ca裁定 → để LLM tự quyết
```

→ **StopGuard** là phanh vật lý: chặn `end_turn` khi `Phase ≠ Complete`. Sửa bug = thêm 1 generator/nhánh router + 1 test.

---

## 5. Cơ chế đặc biệt

| Cơ chế | Mô tả ngắn |
|--------|-----------|
| **Khôi phục cấp Step** | Mỗi tool xong ghi `meta/checkpoints.jsonl`; chạy lại cùng thư mục → tự khôi phục đến bước plan/draft/check/commit. Idempotent qua digest |
| **Quy hoạch cuốn chiếu** | Không quy hoạch hết 500 chương 1 lần. Compass (chỉ nam终局) + 展开 dần khi viết tới |
| **Context 3 tầng** | 卷 → 弧 → 章 tóm tắt phân tầng; pipeline nén 4 cấp: `MicroCompact → LightTrim → StoreSummaryCompact → FullSummary` |
| **Review 7 chiều** | Editor chấm: nhất quán设定 / hành vi nhân vật / nhịp / mạch tự sự / 伏笔 / 钩子 / thẩm mỹ — mỗi điểm phải trích dẫn nguyên văn |
| **Can thiệp realtime** | User gõ ý kiến vào ô input giữa chừng (không cần dừng); hệ thống tự đánh giá phạm vi ảnh hưởng & viết lại chương liên quan |
| **StopGuard** | `Phase ≠ Complete` thì Coordinator KHÔNG thể `end_turn` về mặt vật lý |
| **diag (`/diag`)** | Chẩn đoán chỉ-đọc 4 chiều (流程/质量/规划/上下文) + export脱敏 để dán issue. KHÔNG bao giờ tự sửa |

---

## 6. Cấu trúc code

```
cmd/ainovel-cli/        entry point CLI
internal/
  domain/               data thuần: Phase / Flow / Progress / Checkpoint / Scope
  store/                lưu file (tmp+rename + nguyên tử 3 bước)
  tools/                11 tool IO nghiệp vụ (write-tool: artifact+Progress+checkpoint)
                        + tool điều phối/phụ trợ: subagent (ở agents/), reopen_book, save_user_rules, ask_user
  agents/               build.go lắp ráp Coordinator + 3 sub-agent; ctxpack/ nén context
  host/                 host.go (~1219) + resume + observer + usage
    flow/               router.go (hàm thuần) + dispatcher + state
    reminder/           stop_guard (Coordinator) + subagent_guards
    imp/ exp/ sim/      import反推 / export TXT·EPUB / 仿写画像
  entry/                tui (~7000, lớn nhất) / headless / startup
  bootstrap/            config + ModelSet + provider failover + setup向导
  diag/                 chẩn đoán chỉ-đọc
assets/
  prompts/              coordinator / architect-(short|long) / writer / editor / import-* / simulation-*
  references/           kỹ thuật viết + template thể loại + quy hoạch dài
  styles/               default / fantasy / romance / suspense
```

---

## 7. Tech stack

| Thành phần | Vai trò |
|-----------|---------|
| **Go 1.25** | Ngôn ngữ chính |
| [agentcore](https://github.com/voocel/agentcore) | Kernel Agent tối giản (tool-calling + streaming) |
| [litellm](https://github.com/voocel/litellm) | Adapter thống nhất giao diện LLM |
| [Bubble Tea](https://github.com/charmbracelet/bubbletea) | Framework TUI terminal |

---

## 8. Thể loại tiểu thuyết phù hợp

### 8.1 Engine tối ưu cho thể loại nào?

Engine sinh ra để viết **web-novel / light-novel dài** (200–500 chương), cốt truyện tuyến tính tiến về phía trước, arc nối arc — mô hình serialized fiction kiểu Á Đông. Cơ chế quy hoạch cuốn chiếu + compass + review theo arc được thiết kế đúng cho dạng này.

### 8.2 Bốn style dựng sẵn

Config field `style` trong `config.json` chọn template phong cách. Mỗi style có arc-template, style-reference, và hướng dẫn viết riêng (nằm trong `assets/styles/` và `assets/references/genres/`):

| Style | Thể loại chính | Sub-genre phù hợp | Vì sao hợp |
|-------|----------------|-------------------|------------|
| `fantasy` | Huyền huyễn · Tiên hiệp · Tu tiên · Võ hiệp | Progression Fantasy, LitRPG, Cultivation, Epic Fantasy, Isekai | Arc-template "đột phá cảnh giới / đấu pháp / thăng cấp", hệ thống năng lực có quy tắc rõ ràng |
| `romance` | Ngôn tình · Lãng mạn | Billionaire Romance, Werewolf, CEO, Romantasy, Contemporary Romance, Office Romance | Template tuyến tình cảm: tiếp xúc → hảo cảm → xung đột → hòa giải → sâu sắc; quản lý đối thủ tình cảm + hiểu lầm |
| `suspense` | Trinh thám · Kỳ án · Hồi hộp | Mystery, Thriller, Noir, Psychological Suspense, Crime Fiction, Sci-fi Mystery | Quản lý 伏笔 (manh mối/foreshadow) chặt, hook cuối chương, đa tuyến tự sự, kỹ thuật red herring |
| `default` | Chung, không chuyên biệt | Slice of Life, Adventure, Military, Historical, Sci-fi (general), Horror | Dùng khi thể loại không nằm trong 3 cái trên, hoặc truyện lai nhiều thể loại |

### 8.3 Thể loại kém hợp

| Dạng truyện | Lý do | Workaround |
|-------------|-------|------------|
| **Văn học nghệ thuật** (dòng ý thức, cấu trúc phi tuyến, thẩm mỹ câu chữ cực cao) | Engine giỏi giữ nhất quán chứ không giỏi thẩm mỹ tinh vi | Dùng tính năng 仿写 (simulation profile) nạp mẫu văn, hoặc override prompt |
| **Truyện ngắn** (< 15–20 chương) | Phí bộ máy multi-agent, viết tay bằng LLM còn nhanh hơn | Viết trực tiếp bằng LLM, không cần engine |
| **Truyện giọng văn cực kỳ cá nhân** | Engine tạo giọng "nhất quán" nhưng trung tính | Nạp simulation profile hoặc custom prompt trong `assets/prompts/` |
| **Non-fiction / Self-help / Kỹ thuật** | Engine thiết kế cho fiction, không có cơ chế quản lý luận điểm/chứng cứ | Không phù hợp |

### 8.4 Đối chiếu thể loại × thị trường monetization

> Tham khảo khi chọn thể loại để viết kiếm tiền trên các nền tảng.

| Thể loại | Độ hot thị trường (2026) | Style engine | Platform phù hợp nhất |
|----------|------------------------|-------------|----------------------|
| Romance / Romantasy | 🔥🔥🔥🔥🔥 (#1 toàn cầu) | `romance` | Dreame, GoodNovel, KDP |
| Progression Fantasy / LitRPG | 🔥🔥🔥🔥 (#2, niche lớn) | `fantasy` | Royal Road → Patreon, KDP |
| Werewolf / Billionaire | 🔥🔥🔥🔥 (hot trên serialized) | `romance` | Dreame, GoodNovel |
| Thriller / Psychological | 🔥🔥🔥 (đang lên) | `suspense` | KDP, Royal Road |
| Cultivation / Tiên hiệp | 🔥🔥🔥 (niche Á Đông) | `fantasy` | WebNovel, KDP |
| Sci-fi Noir / Mystery | 🔥🔥 (niche nhỏ, ít cạnh tranh) | `suspense` | KDP, Royal Road |
| Cozy Fantasy / Solarpunk | 🔥🔥 (xu hướng mới) | `default` | KDP, Royal Road |

---

## 9. Lưu ý thực tế khi sử dụng

### 9.1 Model LLM — yếu tố quyết định chất lượng

> ⚠️ **Model mạnh = truyện tốt. Không có cách tắt.** Engine chỉ là khung — chất lượng prose, nhân vật, cốt truyện phụ thuộc hoàn toàn vào LLM.

**Nguyên tắc chọn model:**

| Vai trò agent | Nên dùng | Lý do |
|--------------|---------|-------|
| **Writer** | Model mạnh nhất có thể (tier cao nhất trong budget) | Viết prose, tâm lý nhân vật, đối thoại — cần sáng tạo tốt nhất |
| **Architect** | Model trung-mạnh | Quy hoạch cốt truyện, xây dựng thế giới — cần reasoning tốt |
| **Editor** | Model trung-rẻ | Review 7 chiều, tóm tắt — reasoning đủ, không cần sáng tạo |
| **Coordinator** | Model trung-rẻ | Điều phối quy trình — logic flow, ít sáng tạo |

**Có thể set model khác nhau theo vai trò** — chỉnh trong modal ⚙ Model hoặc field `roles` trong config. Chiến lược: đầu tư budget vào Writer, tiết kiệm ở Coordinator/Editor.

> 💡 Model ngành LLM tiến hóa cực nhanh — không nên chọn model theo tên cụ thể mà theo **tier hiệu suất tại thời điểm dùng**. Kiểm tra [leaderboard](https://lmarena.ai/) hoặc benchmark mới nhất để chọn model phù hợp budget.

### 9.2 Viết tiếng Việt

Engine gốc là tiếng Trung. Muốn viết tiếng Việt:
1. Tạo file `~/.ainovel/rules/lang-vi.md` với nội dung đại ý: *"Viết toàn bộ truyện bằng tiếng Việt tự nhiên"*
2. Engine tự nạp rule khi mở sách mới
3. ⚠️ Sách cũ đã chạy rồi thì **không đọc lại rule mới** (đã chốt `user_rules` lúc tạo)

### 9.3 Viết yêu cầu ban đầu

**Càng rõ càng ít drift.** Đừng chỉ gõ "viết truyện tu tiên". Nêu:
- Bối cảnh / thế giới quan
- Kiểu nhân vật chính (tính cách, động lực, điểm yếu)
- Hướng kết thúc (happy end? bittersweet? open?)
- Điều muốn tránh (cliché, trope không thích)
- Hoặc dùng **Đồng sáng tác** để chat chốt chỉ thị trước

> Yêu cầu mơ hồ → AI tự bịa hướng → 50 chương sau mới phát hiện lệch → tốn token sửa.

### 9.4 Can thiệp sớm

Thấy lệch hướng → gõ can thiệp **ngay** vào ô input. Ví dụ:
- "nhịp chậm quá, đẩy nhanh"
- "đừng biến main thành thánh mẫu"
- "cần thêm tension giữa A và B"

Sửa muộn (50+ chương) → phải viết lại nhiều chương → tốn token + thời gian.

**Can thiệp sớm nhất — Foundation Gate (tab Sản xuất, bản fork):** khi chạy job sinh
truyện mới, engine tự dừng ngay sau khi lập xong **nền móng** (premise/outline/thế giới/nhân
vật) — trước khi viết phần lớn truyện — để bạn duyệt. Ở đó có 3 lựa chọn: **Duyệt** (viết
tiếp), **Sửa tay** (mở thư mục sửa file nền móng rồi Duyệt), hoặc **Sửa & tạo lại** (nhờ AI
viết lại nền móng theo góp ý — tạo job mới, giữ job cũ làm dự phòng, ~$0.01 sinh nền móng). Đây
là chốt chặn rẻ nhất chống đúng cái bẫy "50 chương sau mới phát hiện lệch" ở §9.3 (dừng theo
poll 5s nên là best-effort — tệ nhất mất một phần chương 1, không mất hàng trăm chương). Chi tiết:
[docs/journals/260705-foundation-gate.md](docs/journals/260705-foundation-gate.md).

### 9.5 Chi phí & thời gian

- Truyện dài = hàng trăm lượt gọi LLM = **vài giờ đến vài ngày** + chi phí API
- Theo dõi ô **Chi phí · Context** ở sidebar
- Chạy nền được, không cần ngồi canh
- Crash không mất gì — checkpoint theo bước, chạy lại là khôi phục

### 9.6 Quy tắc vệ sinh

- **Mỗi truyện = 1 thư mục** — đừng chạy 2 truyện trong cùng thư mục
- **Đừng sửa tay file chương** — UI hiện chỉ đọc, engine tin vào Store. Sửa tay `.md` có thể lệch với `progress.json`. Muốn đổi → can thiệp cho AI viết lại
- **Crash → chạy lại** — `bash start-web.sh` → chọn truyện → Khôi phục

---

## 10. "Cố tình KHÔNG làm" (ranh giới kiến trúc)

Vi phạm = lệch kiến trúc (xem `docs/architecture.md` §10 — 15 điều):

- ❌ Không Task / Job / WorkItem · không Dispatcher / Scheduler
- ❌ Không "空闲续跑" (auto-resume khi idle) — Run kết thúc = Host vào终态
- ❌ Không mô hình 4 tầng (WorkflowInstance / Command + Apply)
- ❌ Không hardcode "bù đắp ảo giác LLM" ở Host
- ❌ Không cho `diag` / tầng quan sát động vào control flow
- ❌ Không gọi LLM trong tool layer (trừ chính agent tool)
- ❌ Không để UI đọc trực tiếp Store
