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

## 8. "Cố tình KHÔNG làm" (ranh giới kiến trúc)

Vi phạm = lệch kiến trúc (xem `docs/architecture.md` §10 — 15 điều):

- ❌ Không Task / Job / WorkItem · không Dispatcher / Scheduler
- ❌ Không "空闲续跑" (auto-resume khi idle) — Run kết thúc = Host vào终态
- ❌ Không mô hình 4 tầng (WorkflowInstance / Command + Apply)
- ❌ Không hardcode "bù đắp ảo giác LLM" ở Host
- ❌ Không cho `diag` / tầng quan sát động vào control flow
- ❌ Không gọi LLM trong tool layer (trừ chính agent tool)
- ❌ Không để UI đọc trực tiếp Store
