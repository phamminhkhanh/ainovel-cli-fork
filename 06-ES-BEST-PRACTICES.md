# 06 — Best Practices: Viết Novel Tiếng Tây Ban Nha (ES) cho Serialized Fiction

> Mục đích: cẩm nang thực chiến cho tác giả dùng `ainovel-cli` để sinh novel tiếng
> Tây Ban Nha, tối ưu cho nền tảng serialized fiction (Booknet, Wattpad, Dreame,
> MyNovel). Dựa trên nghiên cứu thị trường ES (tháng 7/2026) + kinh nghiệm từ VN.

---

## 0. Snapshot thị trường ES (30 giây)

| Metric | Giá trị |
|--------|---------|
| Thị phần ebook số tiếng Tây Ban Nha | España 56% + México 20% = 76% |
| Genre #1 đọc nhiều nhất ở Tây Ban Nha | **Romance** (vượt cả fantasy) |
| Sub-genre hot nhất 2025-2026 | **Romantasy** (romance + fantasy) |
| Tăng trưởng thị trường web novel | CAGR 12.4% → $22.4B năm 2034 |
| Định dạng đọc chủ đạo | **Mobile-first** — 80% Wattpad đọc trên điện thoại |

**Tóm tắt:** Thị trường ES khổng lồ và đang tăng. Romance là vua. Platform
**Booknet** là target #1 cho sản phẩm AI (license 80%, không yêu cầu độc quyền,
dark romance/erotica là chủ đạo). Wattpad cho build community, Dreame cho thu
nhập nhưng hợp đồng gắt gao.

---

## 1. Nền tảng: chọn chỗ xuất bản

### 1.1 Ma trận nền tảng ES

| Nền tảng | Vai trò | Thu nhập | Độc quyền? | Risk | Phù hợp AI novel? |
|----------|---------|----------|-------------|------|-------------------|
| **Booknet** | Kênh chính | 80% (exclusive) / 50% (non-exclusive) | Tùy chọn | **Thấp** — license không perpetual, có thể rút | ✅ **#1 khuyến nghị** |
| **Wattpad** | Build community | Creators Program (500 từ/tuần tối thiểu) | Không | Thấp | ✅ Book 1 FREE kéo readers |
| **Dreame** | Thu nhập cố định | $100-$200 / 50K từ + 50% net (exclusive) | **Exclusive, life+50 năm** | **CAO** — ghost-writing clause, terminate 36 tháng | ⚠️ Chỉ khi chấp nhận lock-in |
| **MyNovel** | Kênh phụ | 80% (claimed) | Non-exclusive | **TRUNG BÌNH** — license perpetual/transferable, liability cap €100 | ⚠️ Chỉ non-exclusive, sách cũ |

### 1.2 Chiến lược xuất bản đề xuất (3 cuốn)

| # | Nền tảng | Giá | Thể loại | Mục tiêu |
|---|----------|:---:|----------|----------|
| **Book 1** | Wattpad (FREE) + Booknet (FREE) | $0 | Contemporary Romance + Second Chance + Slow Burn | Kéo readers, build following, đạt "commercial status" Booknet |
| **Book 2** | Booknet (PAID $2-3) | $2.53 | Dark Romance + Mystery + Enemies-to-Lovers | Convert fans, comment magnet |
| **Book 3** | Booknet (PAID $2-3) | $2.53 | Romantasy + Werewolf + Fated Mates | Blue ocean — ít cạnh tranh ES |

**Funnel:** Book 1 FREE (reach) → Book 2/3 PAID (convert). Upload completed —
độc giả ES thích truyện hoàn thành.

### 1.3 Booknet: yêu cầu "commercial status"

Để bán sách trên Booknet cần:
- **100 followers** tối thiểu
- **1 tác phẩm hoàn thành FREE** ≥ 240.000 ký tự (≈ 40-50K từ ≈ 15-20 chương)

→ Book 1 FREE trên Booknet vừa build community vừa đạt ngưỡng commercial.
Sau khi đạt: Book 2+ có thể tính phí.

### 1.4 MyNovel: lưu ý pháp lý

Xem chi tiết `03-MYNOVEL-REPORT.md`. Tóm tắt risk:
- License perpetual, transferable, sublicensable — ngay cả khi rút, quyền vẫn tồn tại.
- Liability cap €100, terminate 7 ngày.
- DMCA contact = `legal@booknet.com` (cùng group Booknet).
- **Khuyến nghị:** chỉ upload non-exclusive, sách cũ đã xuất bản nơi khác.

---

## 2. Tropes & conventions thể loại cho độc giả ES

### 2.1 Tropes thắng trên thị trường ES (2025-2026)

| Trope | Độ hot | Ghi chú |
|-------|--------|---------|
| **Enemies to Lovers** | 🔥 Hottest | Được tìm kiếm nhiều nhất ở ES |
| **Forced Proximity** (one bed) | 🔥 Hot | Catalyze romance trong không gian hẹp |
| **Fake Dating / Matrimonio por contrato** | ✅ Proven | Booknet staple |
| **Second Chance** | ✅ Proven | New Adult + contemporary ES |
| **Found Family** | ✅ Proven | Emotional resonance cao |
| **Grumpy x Sunshine** | ✅ Trending | "Vaqueros gruñones" đang lên |
| **Slow Burn** | ✅ Bắt buộc | Độc giả ES ưu tiên — phát triển qua hàng trăm trang |
| **Sports Romance** (F1, hockey) | 📈 TikTok viral | Đang tăng nhờ BookTok |

### 2.2 Sub-genre hot

| Sub-genre | Mức độ | Đặc điểm |
|-----------|--------|----------|
| **Romantasy** | 🔥 Lucrative nhất | Romance + epic fantasy + spicy scenes. Fan loyalty cao |
| **Dark Romance** | ✅ Major trend 2026 | Morally grey characters, questionable legality. Booknet chủ đạo |
| **Dark Academia** | ✅ Trend | Boarding school elite, forbidden secrets |
| **Contemporary Romance** | ✅ Volume pillar | 71/100 Kindle bestsellers ES. Sub-themes: Mafia, Millionaires |
| **Mystery + Romance** | 📈 Hybrid đang lên | Romance + police procedural / social criticism |

### 2.3 Kỳ vọng độc giả ES (must-have)

- **Female Gaze bắt buộc** — mutual pleasure, consent, không male-gaze gratuitous.
- **Competence Porn** — protagonist hyper-competent (chính trị, kiếm thuật,
  chuyên môn). Độc giả ES ghét heroine yếu đuối vô cớ.
- **Slow Burn** — emotional satisfaction phát triển dần, không rush chemistry.
- **Cliffhanger per chapter** — giữ engagement, đặc biệt khi serialize.
- **Multi-voice narration** — POV alternating đang trend 2026.

---

## 3. Chiều dài chương & nhịp phát

### 3.1 Wordcount theo nền tảng/thể loại

| Loại | Từ/chương | Rune range (engine) | Ghi chú |
|------|----------|---------------------|---------|
| Ebook completed (Booknet/MyNovel) | 2500-4000 | 14000-22000 | Sweet spot |
| Serialize mobile (Wattpad) | 1500-2000 | 8000-12000 | 80% đọc mobile |
| Thriller/Mystery (ritmo rápido) | 1000-2000 | 6000-12000 | Tension ngắn |
| YA (Young Adult) | 1500-3000 | 8000-16000 | Giữ attention |

> ⚠ **Cài đặt engine:** override `chapter_words` trong `meta/user_rules.json`.
> Mặc định `3000-6000` rune là calibrate cho CJK → chương ES sẽ quá ngắn.
> Đề nghị: `{min: 14000, max: 22000}`.

### 3.2 Nguyên tắc nhịp

- **Serialize:** 2-3 chương/tuần, mỗi chương kết bằng cliffhanger.
- **Completed ebook:** xuất bản hết rồi mới lên — độc giả ES thích completed.
- **Hard limit Wattpad:** 200 chương/story. Nếu truyện dài → chia thành seasons.
- **Split rule:** chương > 7000 từ → chia làm 2. Không vượt 4000 từ cho romance
  thông thường.

### 3.3 Tổng chiều dài

| Loại | Từ | Chương (2500 từ) |
|------|-----|-------------------|
| Novela corta | 25.000-40.000 | 10-16 |
| Novela estándar | 40.000-90.000 | 16-36 |
| Novela larga (serial) | 90.000-150.000 | 36-60 |

---

## 4. Español neutro — chiến lược ngôn ngữ

### 4.1 Tại sao neutro

Thị trường ES chia đôi: độc giả Tây Ban Nha phàn nàn regionalism Mỹ Latinh
"khác lạ"; độc giả Mỹ Latinh thấy tiếng Tây Ban Nha "mệt mỏi" nhưng hiểu được.
→ **Español neutro** = biến thể trung tính dựa trên tiếng México nhưng bỏ
slang địa phương, chấp nhận được ở cả 2 thị trường.

### 4.2 Rules neutro cho ainovel-cli

| Rule | Neutro | Tránh |
|------|--------|-------|
| Đại từ 2 số nhiều | **ustedes** | vosotros (España), vos (Argentina) |
| Đại từ 2 số ít | **tú** | vos (Cono Sur) |
| Thì quá khứ | **pretérito simple** (comí) | compuesto (he comido) — España ưu tiên compuesto |
| "Computer" | **computadora** | ordenador (España) |
| "Gasoline" | **gasolina/nafta** theo ngữ cảnh | — |
| "Sidewalk" | **calle** + ngữ cảnh | vereda (AR), acera (ES) |
| "You all" | **ustedes** | vosotros |
| Voseo | KHÔNG dùng | — |

> **Cho profile engine:** ghi rule "español neutro" trong section ngôn ngữ.
> File `~/.ainovel/rules/lang-es.md` đã có rules chi tiết.

### 4.3 Lỗi ngữ pháp AI phổ biến ở ES

| Lỗi | Ví dụ | Sửa thành |
|-----|-------|-----------|
| Gerundio causal lạm dụng | "X sonrió, sabiendo que..." | "X sonrió. Sabía que..." |
| Frases simétricas | "No es A, es B" lặp lại | Biến thể tự nhiên |
| Sin contracciones | "no lo sé" | "no sé" (natural) |
| Conectores monotonous | "además... sin embargo... por lo tanto" | Biến thể: "y... pero... así que" |
| Hedging words | "posiblemente", "potencialmente" | Khẳng định trực tiếp |
| Metáforas gastadas | "un tapiz de emociones" | Hình ảnh cụ thể, mới |

---

## 5. Anti-AI prose patterns (chống văn AI tiếng Tây Ban Nha)

> File rules đầy đủ: `~/.ainovel/rules/lang-es.md` + `prose-rhythm-es.md`.
> Đây là tóm tắt cho reviewer.

### 5.1 Frases prohibidas (clichés AI)

**Structural:**
- "de alguna manera", "no podía evitar", "ciertamente", "por así decirlo"
- "valga la redundancia", "en cierta medida", "cabe destacar"
- "es importante tener en cuenta que", "esto significa que"
- "una comprensión más profunda", "sin rodeos", "honestamente"
- "no es X, es Y", "no sólo... sino también..."

**Metáforas gastadas:**
- "pintar un cuadro con palabras", "un tapiz de emociones"
- "un viaje de autodescubrimiento", "un torbellino de eventos"
- "entretejida en el tejido de la historia"
- "desbloquear el potencial", "embarcarse en", "transforma tu enfoque"

### 5.2 Frequency limits (mỗi chương tối đa)

Conectores: además 3, sin embargo 3, no obstante 3, por lo tanto 2, entonces 5,
pero 8, porque 6, cuando 6.

Muletillas: de alguna manera 2, no podía evitar 2, ciertamente 2, por así
decirlo 1, en cierta medida 2, cabe destacar 2, posiblemente 2, potencialmente 2,
fundamental 2, vital 2, exhaustivo 1, integral 2.

### 5.3 Gerundio-causal pattern

IA lạm dụng: "A hace B, causando C" / "X sonrió, sabiendo que...".
**Limit: 2 por capítulo.** Reescribir como coordinada o subordinada.

### 5.4 Staccato (prosa fragmentada)

IA có 2 chế độ: (1) băm vụn thành câu 3-7 từ liên tiếp, (2) câu lê thê 1 nhịp.
Cả 2 là AI-tell. **Đích đến: biến thiên độ dài câu có kiểm soát.**

- Phần lớn câu 15-35 từ, có subordinadas + conjunciones.
- Câu ngắn (<8 từ) chỉ ở điểm nhấn, tối đa 1-2 liên tiếp.
- Xen dài/vừa/ngắn trong mỗi paragraph.

Xem `~/.ainovel/rules/prose-rhythm-es.md` cho rules đầy đủ.

---

## 6. Setup ainovel-cli cho ES

### 6.1 Rules (đã có sẵn)

```
~/.ainovel/rules/
  lang-es.md           ← ngôn ngữ, wordcount, frases prohibidas, frequency
  prose-rhythm-es.md   ← staccato anti-pattern
  lang-vi.md           ← (VN) — VẪN được load kể cả khi viết ES!
  prose-rhythm-vi.md   ← (VN) — VẪN được load kể cả khi viết ES!
```

> ✅ **Đã có auto-select theo ngôn ngữ (từ bản vá per-language rules).** Bạn
> **giữ tất cả rule của mọi ngôn ngữ trong `~/.ainovel/rules/`** như thường —
> mỗi run tự chọn đúng bộ:
>
> - Mỗi production run mang một **mã ngôn ngữ** (`vi`/`es`/`en`). Khi tạo run,
>   `prepareRunDir` **chỉ copy** rule trung tính + rule khớp mã đó vào sandbox;
>   bộ ngôn ngữ khác bị bỏ qua.
> - Child headless được **re-root HOME về sandbox**, nên đường "global rules"
>   của engine (`~/.ainovel/rules`) trỏ vào bản đã lọc — **không còn lây nhiễm
>   chéo** (run ES không nạp `lang-vi.md` nữa).
> - Run lưu `language` + `ruleFiles` (danh sách file thực sự nạp), hiện ngay ở
>   panel chi tiết run trong Cockpit → **trực quan, khỏi đoán**.
>
> **Quy ước đặt tên (load-bearing):** rule theo ngôn ngữ phải kết thúc bằng
> `-<mã>.md` (`lang-es.md`, `prose-rhythm-es.md`, `lang-vi.md`…). Rule **trung
> tính** (không hậu tố mã) nạp cho **mọi** run. Mã hỗ trợ: `vi`, `es`, `en`.
>
> **Cách chọn ngôn ngữ cho run:**
> 1. Chọn ô "Ngôn ngữ viết" trong modal tạo run (Tiếng Việt / Español / English), **hoặc**
> 2. Để "Tự động" — hệ thống đọc marker `<!-- ainovel:lang=xx -->` (Studio tự
>    chèn khi sinh profile) rồi tới hậu tố tên file profile (`*-es.md`, `*-vn.md`).

### 6.2 Profile mẫu (tạo per-novel)

Khi viết novel ES, tạo profile theo template VN hiện có nhưng:
1. Đổi ngôn ngữ sang español.
2. Đặt bối cảnh ở thế giới Hispano (España hoặc Latinoamérica — chọn 1, nhất quán).
3. Dùng tropes từ mục 2.
4. Thêm section "Español neutro" tham chiếu `lang-es.md`.
5. Override `chapter_words: {min: 14000, max: 22000}`.

### 6.3 Checklist trước khi chạy production

- [ ] Profile ghi rõ: sub-genre, tropes, target platform, HEA/HFN/bittersweet.
- [ ] `lang-es.md` đã trong `~/.ainovel/rules/`.
- [ ] `prose-rhythm-es.md` đã trong `~/.ainovel/rules/`.
- [ ] `chapter_words` override: min 14000, max 22000 rune.
- [ ] Bối cảnh nhất quán (España hoặc Latinoamérica, không trộn).
- [ ] Español neutro: tú/ustedes, pretérito simple, vocab universal.
- [ ] Chọn "Ngôn ngữ viết = Español" (hoặc để Tự động với profile `*-es.md`) khi
      tạo run; sau khi start, kiểm tra "Rule đã nạp" ở panel run chỉ gồm
      `lang-es.md` + `prose-rhythm-es.md` + rule trung tính.

### 6.4 Chuẩn hóa rule/profile (ĐÃ vá)

Phân loại theo phạm vi để hết "rối":

| Loại | Phạm vi | Cơ chế |
|---|---|---|
| **Profile** (title, tropes, nhân vật, wordcount của 1 cuốn) | **Per-project** — `<repo>/.ainovel/profiles/*.md` | Sinh/lưu tại Studio; là SSOT của 1 cuốn, không tái dùng cross-book |
| **Rule trung tính** (không hậu tố mã ngôn ngữ) | **Global** — `~/.ainovel/rules/` | Nạp cho MỌI run |
| **Rule ngôn ngữ** (`lang-<mã>.md`, `prose-rhythm-<mã>.md`) | **Global, chọn theo run** | Vẫn để trong `~/.ainovel/rules/`; run chỉ nạp bộ khớp `language` |

**Đã implement (all additive, chỉ trong `internal/entry/web/`):**
1. `ProdRun.Language` + `ProdRun.RuleFiles` — set khi tạo run (UI chọn hoặc
   auto-detect từ profile marker/tên file), lọc tại `prepareRunDir`.
2. `copyLangFilteredRules` — copy rule trung tính + rule khớp mã, bỏ bộ khác.
3. Child re-root HOME về sandbox (`withSandboxHome`) → engine không đọc được
   `~/.ainovel/rules` thật, chỉ thấy bản đã lọc. Hết lây nhiễm chéo.
4. Studio chèn `<!-- ainovel:lang=xx -->` vào profile sinh ra để tự khai báo.
5. Cockpit hiện `language` + "Rule đã nạp" ở panel run.

Chi tiết code/flow: xem [02-WEB-UI.md](02-WEB-UI.md) §"Language-aware rule selection".
- [ ] Female gaze + consent cho romance scenes.
- [ ] Cliffhanger mỗi chương.
- [ ] Slow burn — không rush chemistry trước chương 10.

---

## 7. Quick reference: so sánh VN vs ES

| Khía cạnh | Tiếng Việt | Español |
|-----------|-----------|---------|
| Rune/word | ~5-7 | ~5.5 |
| chapter_words đề nghị | 9000-15000 | 14000-22000 |
| Platform #1 | MyNovel (niche) | **Booknet** (established ES) |
| Genre chủ đạo | Romance/Dark Romance | Romance/Romantasy/Dark Romance |
| Trope hot | Second Chance, Enemies-to-Lovers | Enemies-to-Lovers, Fake Dating, Slow Burn |
| Neutro cần? | Không (1 quốc gia) | **Có** (España vs LatAm) |
| Staccato rule | `prose-rhythm-vi.md` | `prose-rhythm-es.md` |
| Gerundio abuse | Ít gặp | **Phổ biến** — limit 2/chapter |

---

## 8. Cảnh báo pháp lý (tóm tắt)

- **Booknet:** license không perpetual, có thể rút → an toàn nhất. Flat Fee
  cần transfer quyền 3 năm — cân nhắc kỹ.
- **Dreame:** exclusive life+50 năm, ghost-writing clause, terminate 36 tháng →
  **chỉ dùng khi chấp nhận lock-in hoàn toàn**.
- **MyNovel:** license perpetual/transferable/sublicensable, liability €100,
  DMCA = booknet.com → chỉ non-exclusive, sách cũ.
- **Wattpad:** an toàn, nhưng Creators Program gated, monetization hạn chế.
- **Tổng:** KHÔNG đăng tác phẩm mới (chưa xuất bản nơi khác) lên Dreame hoặc
  MyNovel exclusive. Booknet non-exclusive là an toàn nhất.

---

*Disclaimer: Dữ liệu thị trường từ nghiên cứu tháng 7/2026. Điều khoản nền
tảng có thể thay đổi. Đọc Terms of Use trước khi ký hợp đồng.*
