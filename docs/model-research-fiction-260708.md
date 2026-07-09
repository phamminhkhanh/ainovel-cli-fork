# Model Research — Fiction Writing Pipeline

**Cập nhật:** 2026-07-09
**Mục đích:** Chọn model viết novel cho ainovel-cli-fork. Xài riêng, budget $5/run, engine prompt tiếng Trung, novel dài nhiều chương.
**Constraint:** Không dùng Claude/GPT (quá mắc). Pool: model TQ + Mistral (Pháp) + Grok (xAI).

---

## Pool đã test

| Model | Vendor | Context | Giá output/1M | Đã test | Nguồn data |
|---|---|---|---|---|---|
| **DeepSeek V4 Pro** | DeepSeek (TQ) | 1M | **$0.87** | ✅ 4/4 genre | Kakalot proxy |
| **Grok 4.5** | xAI | 500K | $6.00 | ✅ 3/4 genre | Kakalot proxy |
| **Grok 4.3** | xAI | 500K | $2.50 | ✅ 4/4 genre | Kakalot proxy |
| **Mistral Large 3** | Mistral (Pháp) | 262K | $1.50 | ✅ 4/4 genre | Official API |
| **GLM 5.2** | Zhipu (TQ) | 1M | $4.40 | ⚠️ 1/4 genre | Kakalot proxy (3 bị SSE issue, official key hết balance) |

### Loại bỏ (không test):
- **Qwen 3.7 Max** — $7.50/1M output, prose angular, thiếu emotion. Thế mạnh coding, không phải fiction.
- **Kimi K2.7** — Coding variant, temp cứng 1.0, chưa có fiction benchmark. Friction kỹ thuật quá nhiều.
- **GPT-5.5** — $30/1M output, budget killer. Mạnh fiction theo benchmark nhưng 6–30x đắt hơn pool.

---

## Benchmark thực tế — 4 genre × 5 model

**Prompts:** EN literary dark fantasy, EN action/combat, ES magical realism, ZH xianxia.
**Method:** Cùng prompt, cùng temperature (0.9), max_tokens 4000. Đánh giá chủ quan 1-10.

### Scoring tổng hợp

| Genre | #1 | #2 | #3 | #4 | #5 |
|---|---|---|---|---|---|
| **EN Literary** | Grok 4.5 (9.4) | Mistral (7.8) | Grok 4.3 (6.4) | DS V4 (~8.0 est.*) | GLM (N/A) |
| **EN Action** | DS V4 Pro (8.6) | Grok 4.5 (8.4) | GLM 5.2 (8.3) | Grok 4.3 (6.9) | Mistral (6.7) |
| **ES Costurera** | DS V4 Pro (8.9) | Mistral (8.5) | Grok 4.3 (8.0) | — | — |
| **ZH 断剑楼** | Grok 4.5 (8.9) | Mistral (8.4) | DS V4 Pro (8.2) | Grok 4.3 (6.6) | GLM (N/A) |

*DeepSeek literary bị truncated — 3589/4000 tokens dùng cho reasoning, chỉ output 400 words. Cần set max_tokens ≥ 8000.

### Chi tiết từng model (từ benchmark, không phải giả định)

#### DeepSeek V4 Pro — Overall winner, $0.03/chapter

**Đã chứng minh:**
- **ES fiction gần native quality.** Multi-layered twist (bones too clean → Faustian deal → child's finger with birthmark). Schweblin-level horror. Score 8.9 — cao hơn Mistral (8.5).
- **EN action mạnh nhất pool.** Tactical depth (child becomes rear guard), sound design (*"The ambush began with a silence"*), longest output (~1800w). Score 8.6.
- **ZH xianxia emotional depth.** *"阿诀，不要寻仇"* — punch mạnh. Score 8.2.
- **Giá rẻ nhất** — $0.87/1M output, ~$0.03/chapter.

**Cần lưu ý:**
- Reasoning token trap: max_tokens=4000 → 3589 reasoning + 411 content. **Phải set max_tokens ≥ 8000** cho creative tasks.
- Reddit report: *"falls apart across a long project"* — consistency qua nhiều chapter chưa verify.

#### Grok 4.5 — Prose ceiling king, $0.20/chapter

**Đã chứng minh:**
- **EN literary tốt nhất** — recursive map + narrator already dead. Twist recontextualizes toàn bộ. Score 9.4.
- **ZH xianxia prose đẹp nhất** — 古风 natural, identity twist *"你不是无尘。无尘已死。你……是谁？"* Score 8.9.
- **Sound design EN action** — onomatopoeia riêng (*thock, whiss-thock*), phantom arm motif.

**Cần lưu ý:**
- $6/1M output — 7x đắt hơn DeepSeek. Chỉ dùng cho premium chapters.
- Proxy timeout cho ES prompt (2 attempts fail 502).

#### Mistral Large 3 — Cultural consultant, không phải primary writer

**Đã chứng minh:**
- **ES: tốt nhưng KHÔNG phải #1.** Galician names (Maruxa, Xoán), cultural detail authentic. Twist (beating heart in jersey) visceral nhưng đơn giản hơn DeepSeek. Score 8.5 vs DeepSeek 8.9.
- **EN action: yếu nhất pool (6.7).** "Obedient" confirmed — follows prompt literally, thiếu surprise, tactical logic, subtext. Child bị thương = predictable.
- **EN literary: competent (7.8)** nhưng thiếu twist/unreliable narrator. *"The sea was rising"* atmospheric nhưng không recontextualize.
- **ZH: bất ngờ dài (2700字)**, plot phức tạp (邪剑假mạo sư phụ). Nhưng prose modern, không 古风.
- **Reliable** — 4/4 test thành công, fast, không proxy issue.

**Vai trò đúng:** ES cultural fallback + Editor backup + ZH volume. **KHÔNG primary writer.**

#### GLM 5.2 — Reasoning layer, chưa test đủ fiction

**Đã chứng minh (1 test):**
- EN action (proxy): Opening *"The first arrow killed Jiro three steps ahead"* — shock value mạnh, score 8.3.
- Child empress: *"I am the sovereign. You will lower your bow."* — character dynamic tốt nhất trong pool.

**Chưa chứng minh:**
- Fiction quality 3 genre còn lại (official key hết balance, proxy SSE issue).
- Preserved Thinking cho reasoning/coordination (community report tích cực nhưng chưa test trực tiếp).

**Vai trò dự kiến:** Coordinator + Editor (reasoning, không phải creative). Cần nạp tiền BigModel để verify.

#### Grok 4.3 — Mid-tier backup, $0.05/chapter

**Đã chứng minh:**
- Prose clean, professional, reliable (4/4 test ok).
- ES Galician atmosphere đúng (8.0), nhưng twist đơn giản.
- **Thiếu punch ở mọi test** — không có moment nào nổi bật.
- $2.50/1M output — 3x đắt hơn DeepSeek mà không tốt hơn.

**Vai trò:** Không recommend cho pipeline chính. Backup nếu DeepSeek down.

---

## Pipeline khuyến nghị

### 4 model, 6 role

| Role | Model | Giá/chapter | Lý do (từ benchmark) |
|---|---|---|---|
| **Coordinator** | GLM 5.2 | ~$0.05 | 1M context + Preserved Thinking cho quản lý logic xuyên suốt (reasoning, không gen truyện) |
| **Architect** | DeepSeek V4 Pro | ~$0.03 | 1M context, rẻ cho brainstorm nhiều lần |
| **Writer (volume)** | DeepSeek V4 Pro | ~$0.03 | Prose EN/ES/ZH đều tốt, output dài, giá rẻ nhất |
| **Writer (premium)** | Grok 4.5 | ~$0.20 | Climax, opening, closing chapters. Prose ceiling + twist không ai bằng |
| **Writer ES (cultural)** | Mistral Large 3 | ~$0.04 | Fallback khi cần Galician/LatAm cultural nuance. DeepSeek là default |
| **Editor** | GLM 5.2 | ~$0.05 | Reasoning mạnh cho structural review. Mistral backup (clean, reliable) |

### Simplified: 2 model

| Kịch bản | Combo | Giá/chapter |
|---|---|---|
| **EN novel** | DeepSeek V4 Pro + Grok 4.5 (premium) | $0.03 – $0.20 |
| **ES novel** | DeepSeek V4 Pro + Mistral L3 (cultural) | $0.03 – $0.04 |
| **ZH novel** | DeepSeek V4 Pro + Grok 4.5 (literary) | $0.03 – $0.20 |

### Chi phí ước tính per run (10 chapters)

| Role | Calls | Cost |
|---|---|---|
| Coordinator (GLM) | 10 | ~$0.50 |
| Architect (DS) | 3 | ~$0.09 |
| Writer volume (DS) | 7 chapters | ~$0.21 |
| Writer premium (Grok 4.5) | 3 chapters | ~$0.60 |
| Editor (GLM) | 10 | ~$0.50 |
| **Total** | | **~$1.90** |

→ Dưới budget $5/run. Còn room cho retry + iteration.

---

## Caveats

1. **1 prompt/genre** — consistency qua 5+ chapter chưa verify. Reddit nói DeepSeek *"falls apart across a long project"* — cần test.
2. **GLM 5.2 chưa test đủ** — chỉ 1/4 genre. Role Coordinator/Editor dựa trên spec + community report, chưa verify trực tiếp.
3. **DeepSeek reasoning overhead** — phải set `max_tokens ≥ 8000` cho creative tasks. Nếu không, reasoning ăn hết budget.
4. **Grok 4.5 proxy instability** — ES prompt timeout 2 lần. Nếu dùng production cần reliable endpoint.
5. **Benchmark bias** — single evaluator (AI), single prompt. Hemingway-bench (Surge AI) báo EQ-Bench chỉ đồng ý expert writer 43%.

---

## Sources

- [lechmazur/writing](https://github.com/lechmazur/writing) — head-to-head story benchmark
- [eqbench.com](https://eqbench.com/creative_writing.html) — EQ-Bench Creative Writing
- [surgehq Hemingway-bench](https://surgehq.ai/blog/hemingway-bench-ai-writing-leaderboard) — benchmark criticism
- Internal benchmark: `grok-fiction-test-comparison.md` (artifact) — full prose samples + scoring matrix
- Reddit: r/LocalLLaMA, r/SillyTavernAI, r/DeepSeek — community sentiment
