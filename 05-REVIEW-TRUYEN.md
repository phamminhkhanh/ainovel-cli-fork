# 05 — Playbook Review Truyện (đọc file này là đủ để review)

> Mục đích: mở thread mới, agent chỉ cần đọc file này + `AGENTS.md` là hiểu **dự án** và **cách review** một cuốn truyện engine sinh ra (profile · thế giới quan · outline · nhân vật · duy trì · độ hay · twist · thị trường). Không cần giải thích lại từ đầu.

---

## 0. Dự án là gì (30 giây)

`ainovel-cli` (fork) = engine viết tiểu thuyết AI dài kỳ (Go). Coordinator điều phối **Architect / Writer / Editor**; Host là vỏ mỏng. Fork thêm **Web UI** (`internal/entry/web/`, additive-only) + **Production Cockpit** (tab Sản xuất) chạy job `--headless` trong sandbox. Chi tiết: `AGENTS.md`, `01-TONG-QUAN-DU-AN.md`, `02-WEB-UI.md`.

**Nguyên tắc code:** chỉ sửa `internal/entry/web/`; KHÔNG đụng `internal/host/`, `internal/tools/`, `assets/prompts/`, `internal/entry/tui/`.

---

## 1. Truyện nằm ở đâu — đọc gì để review

Mỗi job Cockpit ghi vào sandbox riêng:
```
workspace/<tên>/output/jobs/run-XXX/output/novel/   ← hoặc ./output/jobs/... nếu chạy từ root
  premise.md                      # tiền đề (đọc trước)
  world_rules.md | .json          # luật thế giới
  characters.md | .json           # nhân vật + arc
  layered_outline.md | .json      # outline phân tầng (卷/弧/章)
  meta/compass.json               # la bàn:終局/quy mô/tuyến dài
  meta/progress.json              # phase, completed_chapters, chapter_word_counts, strand_history, hook_history, current_volume/arc
  meta/run.json                   # model/provider + steer_history (user đã can thiệp gì) + pending_steer (chưa inject) — đọc để biết truyện bị steer mấy lần, drift do steer hay do model
  reviews/NN.json                 # Editor tự chấm 7 chiều (mốc arc)
  chapters/NN.md                  # bản final
  foreshadow_ledger.* relationship_state.* timeline.*
```

> ⚠ **Source of truth (quan trọng khi sửa file):** `premise.md` → markdown LÀ gốc (không có .json). `compass.json`, `layered_outline.json`, `world_rules.json`, `characters.json` → **JSON là gốc**, file `.md` cùng tên chỉ là render — engine bỏ qua .md → sửa .md vô tác dụng. KHÔNG rename key JSON (engine phụ thuộc field name). Giữ nguyên ngôn ngữ hiện tại của foundation.

Profile gốc (SSOT): `<cwd>/.ainovel/profiles/*.md` (hoặc `~/.ainovel/profiles/`, legacy `./profiles/`).

**Quy trình đọc để review nhanh (dùng `read_files`, KHÔNG dùng shell — tránh lỗi cú pháp + encoding):**
1. Profile gốc + `premise.md` + `world_rules.md` + `characters.md` + `meta/compass.json` → nắm nền móng.
2. `meta/progress.json` → tiến độ, wordcount, `strand_history` (quest/fire/emotion), `hook_history`.
3. `reviews/*.json` → Editor tự chấm (điểm 0-100/chiều, verdict `polish`/`accept`/`rewrite`). Trung thực, đọc trước để tiết kiệm.
4. Đọc trực tiếp prose: **chương 1** (giữ chân) + **1 chương giữa** + **chương mới nhất** (bắt drift). Nếu đổi model giữa chừng → đọc chương ngay trước + ngay sau mốc đổi.

**Wordcount:** engine đếm **rune** (Unicode codepoint) — đúng cho CJK (1 rune ≈ 1 chữ), **sai mật độ** cho VN/Latin (1 từ ≈ 5-7 rune). Cap mặc định `chapter_words: {min:3000, max:6000}` calibrate cho tiếng Trung; áp cho tiếng Việt → chương chỉ ~1000-1500 từ (quá ngắn). **Fix:** override `chapter_words` trong `meta/user_rules.json` — VN đề xuất `min:9000, max:15000` rune (≈ 2000-3500 từ, chuẩn webnovel). Tương tự cho Spanish/English/Latin alphabet. Dài hạn: engine cần word-count mode cho non-CJK (upstream feature request).

---

## 2. Nguyên tắc review chung

- **Sai ở nền móng nhân lên hàng trăm chương** → review profile/foundation nghiêm nhất.
- **Genre-agnostic**: áp cho mọi thể loại, chỉ đổi từ vựng.
- **Phản biện thẳng, trích dẫn nguyên văn, không khen xã giao.** Kết bằng "3 việc phải sửa trước tiên" xếp theo mức sát thương truyện dài.

> ⚠️ **BƯỚC 0 — BẮT BUỘC trước mọi review hay sinh profile. Xác định rõ 4 thứ, mọi nhận xét sau phải bám vào đây:**
> 1. **Thể loại & sub-genre** chính xác (romance / werewolf / cultivation / mystery / LitRPG…). Tôn trọng đúng thứ user muốn, không trôi về genre mặc định.
> 2. **Đặc thù thể loại**: payoff/beat/khế ước ngầm mà độc giả thể loại này coi là bắt buộc (thiếu = hụt). Đã **đại trà** chưa → cliché phải tránh; nếu niche → phải nail gì cho fan cứng.
> 3. **Nhu cầu độc giả của thể loại đó**: họ đọc để lấy cảm giác gì (đấu trí thắng lợi? "sủng"? longing? leo cấp? giải đố?), nhịp và độ dài kỳ vọng.
> 4. **Quốc gia mục tiêu + VĂN HÓA nước đó + năm hiện tại** — quyết định mã trope, ngưỡng 18+/bạo lực, điều cấm kỵ, gu cảm xúc. **Cùng một thể loại nhưng khác nước là khác truyện.**
>
> **Dự án viết đa ngôn ngữ (VN · EN · ES) → thị trường/văn hóa KHÁC NHAU rõ rệt, không suy từ nước này sang nước khác:**
> - **VN** (NovelToon/Dreame VN/Waka/WebNovel): mã ngôn tình Hoa-hoá (xuyên không, cung đấu, trọng sinh, tổng tài); **"sủng" gần như bắt buộc**; werewolf/Alpha là mã ngoại nhập kén hơn; 18+ bị kiểm duyệt (giữ "suggestive").
> - **EN / Anglo** (Royal Road, Amazon KDP, WebNovel EN, Wattpad): fated-mate/longing cho werewolf; progression/LitRPG mạnh; slow-burn chấp nhận; ngưỡng spice cao hơn tùy nền tảng.
> - **ES / Mỹ Latinh** (Booknet, Dreame ES, Wattpad ES): dark romance / mafia / werewolf rất mạnh; nhịp cảm xúc cao, kịch tính gia đình; taboo văn hóa-tôn giáo riêng; kỳ vọng độ dài/nhịp khác EN.
> - Nếu **không chắc gu nước đích ở năm hiện tại** → nói rõ + khuyên kiểm chứng bằng bảng bestseller/đề xuất đang chạy của chính nền tảng nước đó, đừng đoán.

---

## 3. Các trục review (checklist — "tốt" vs "cờ đỏ")

**A. Nhân vật**
- want + wound + mâu thuẫn nội tâm rõ, không tự mâu thuẫn; tên cố định; **giọng phân biệt** được giữa các nhân vật.
- 🔴 nhân vật hoá "thánh": tài năng lõi (vd trí tuệ) **không có nguồn gốc + giới hạn/điểm mù** → out-think mọi thứ, mất căng.

**B. Cái giá & thất bại thật** (quan trọng nhất cho truyện dài)
- Phân biệt **twist** (đẩy plot) vs **cái giá** (đánh vào nhân vật, để lại dấu vết dài: mất đồng minh/niềm tin/hy sinh không lấy lại).
- 🔴 chỉ thắng liên tục, đúng 1 cú vấp giữa truyện → độc giả hết lo từ ~ch.100. Cần nhiều cái giá rải dọc, không reset mỗi arc.

**C. Phản diện**
- Có (các) đối thủ **CÓ TÊN**, lặp lại, mưu riêng, trí tuệ ngang cơ; mối đe doạ leo thang; gài + trả tuyến phản bội.
- 🔴 "bộ máy/thế lực vô danh"; 伏笔/tuyến phản bội ban đầu bị bỏ rơi.

**D. Cơ chế lõi (thế giới quan)**
- Cơ chế đặc thù (bond/phép/quyền lực) có **LUẬT + giá + giới hạn** rõ; world rules **ràng buộc plot** (tài nguyên/giá/giới hạn), không trang trí.
- 🔴 mơ hồ → **deus ex machina** giải mọi thứ; luật chỉ để tả cho đẹp.

**E. Xung đột lõi & mid-pivot**
- Xung đột trung tâm đủ kéo **CẢ truyện**; có **một chương "không thể quay đầu" cụ thể** (~40-60%) buộc đổi chiến lược thật.
- 🔴 mid-pivot là dải chương mơ hồ / chỉ leo thang tuyến tính.

**F. Outline / phân bổ tải**
- Tải rải đều theo 卷/弧; mỗi arc ≥1 cái giá dai dẳng; **伏笔 gài sớm có chỗ trả**.
- 🔴 outline cạn ý giữa chừng; arc toàn 1 loại strand (xem `strand_history`).

**G. Thể loại chính vs cấu trúc + nhịp**
- Thể loại được hứa (vd Romance) có **beat riêng phục vụ** không, hay bị nhánh khác (chính trị/hành động) lấn → lệch mood/kỳ vọng.
- Nếu profile khai **tỷ lệ chương bắt buộc** (vd ≥3 romance-forward/10, có "sủng"): đối chiếu `strand_history` xem có tụt không.
- 🔴 romance/"sủng" tụt ở arc điều tra; công thức chương lặp cứng suốt N chương → đơn điệu.

**H. Kết & lời hứa**
- Định hướng kết = **câu hỏi chủ đề** (không phải plot beat); kết có **cái giá không đảo ngược** (không utopia); **tên truyện khớp kết**.

**I. Ensemble & duy trì**
- ≥3 nhân vật phụ có tên + arc riêng; hồ sơ nhân vật đồng bộ với diễn biến thật (vd sự kiện bị đẩy sớm/muộn so plan → cần sync).
- 🔴 **Nhân vật tự sinh**: Writer có thể tạo nhân vật mới không có trong `characters.json` (vd "Tuấn" — beta trẻ bảo vệ An Nhiên). Không phải lỗi, nhưng nếu nhân vật tự sinh đóng vai quan trọng mà không có arc trong foundation → sẽ bị bỏ rơi hoặc inconsistent. **Kiểm tra:** grep tên lạ trong chapters, đối chiếu `characters.json`. Nếu quan trọng → thêm vào foundation.

**J. AI-tell (soi kỹ prose)**
- 🔴 **cấu trúc "không phải X mà là Y" / "Không phải… Mà là…"** (kể cả trong worldbuilding/premise) — lỗi phổ biến nhất, nhiều model vi phạm; purple prose; nội tâm lặp một vòng; nhịp câu đều đều; câu văn mẫu; **văn quá vụn** (câu cụt không động từ thành register mặc định → đọc như sổ tay); **recap độn** (điểm lại cả arc thay vì dựng cảnh).
- 🔴 **Từ/cụm khoá lặp máy móc** — grep toàn bộ chapters: nếu 1 cụm xuất hiện >10 lần → cờ đỏ. Ví dụ thực tế: `"bond nền ấm đều"` (Grok 4.5 lặp gần như mỗi chương mở đầu), `"không khỏi"`, `"dường như"`.
  - Sửa qua `meta/user_rules.json` (schema `internal/rules/types.go`), phân biệt đúng field:
    - `forbidden_phrases: []string` → **cụm cấm tuyệt đối** (vd `"bond nền ấm đều"`, `"không phải X mà là Y"` nếu cụ thể hoá được).
    - `fatigue_words: map[string]int` → **từ thường chỉ xấu khi lặp dày**, đặt ngưỡng/chương (vd `{"im": 3, "khẽ": 2, "dường như": 2}`) — không cấm hẳn.
    - `forbidden_chars: []string` → ký tự cấm.
  - Hoặc dùng **steer-on-resume** nếu run đang chạy dở (§5).

**K. Thị trường mục tiêu** (đối chiếu với **BƯỚC 0** ở §2 — nước/văn hóa/năm đã xác định)
- Truyện có thật sự khớp gu nước đích không: mã trope bản địa vs ngoại nhập, kỳ vọng cảm xúc đặc trưng ("sủng" VN / fated-mate-longing EN / dark-romance-kịch-tính ES), ngưỡng 18+/kiểm duyệt nền tảng.
- 🔴 Lựa chọn đi **ngược trend đang thắng** mà không bù đắp → coi là "điểm mới lạ an toàn" (thực ra kén độc giả). Phải gọi tên là risk có chủ đích + cách bù.

---

## 4. Riêng khi ĐỔI MODEL giữa truyện

Đọc chương ngay trước + ngay sau mốc đổi, so:
- **Cốt truyện/continuity**: model mới có giữ伏笔/nhân vật/nhất quán không (thường GIỮ được).
- **Voice drift**: register có đổi không (vd noir cô đọng → giãn/giải thích/recap nhiều)? Người đọc tinh cảm nhận được.
- **AI-tell regression**: model mới có lạm dụng "không phải X mà là Y", recap, purple prose hơn không.
→ Nếu drift: hoặc **steer siết giọng**, hoặc đổi lại model writer.

---

## 5. Can thiệp (sau khi review ra vấn đề)

Cockpit là **automation-first**, chỉ 1 hard-gate:
1. **Foundation Gate** (`awaiting_review`, run `fresh_profile`): tự dừng khi nền móng xong. **Duyệt** (`approve`) / **Từ chối** (`reject`) / **Sửa & tạo lại** (`revise` — sinh lại nền móng theo góp ý, ~$0.01) / mở nền móng (`reveal`/`ide-bundle`). Có sẵn **copy review** (LLM ngoài) + **copy bundle Review&Edit** (agent IDE sửa 5 file nền móng trực tiếp).
2. **Steer khi đang viết**: headless KHÔNG có ô input. Cách đúng seam: **Dừng job → nút ↻ Tiếp tục kèm ô "Steer khi tiếp tục"** → ghi `pending_steer` vào `meta/run.json`, headless `Resume()` inject vào Coordinator ở chương kế (chỉ ăn ở ranh giới resume, không khi child đang chạy). Steer **mềm** (Coordinator đánh giá & áp) → viết **cụ thể** (vd "mỗi 4-5 chương 1 nhịp sủng cụ thể; siết câu ngắn; cấm 'không phải X mà là Y'; giảm 'bond nền ấm đều'").
   - **Edge**: steer-on-resume **KHÔNG** tác dụng nếu run chưa có output (fail trước Foundation Gate) — child chạy `--prompt-file` (fresh `StartPrepared`, không `Resume`) → `pending_steer` không được đọc. Chỉ steer-resume được khi run đã viết ≥1 chương.
   - Mỗi steer ghi vào `steer_history` (`meta/run.json`) → reviewer xem đây để biết truyện đã bị can thiệp mấy lần, nội dung gì. `pending_steer` bị xóa ngay sau khi inject (1 lần, không lặp).
3. **Model theo vai**: Writer nên model mạnh nhất; đổi trong ⚙ Model hoặc field `roles` config. Đổi giữa truyện → coi §4.
4. **Rules**: `meta/user_rules.json` / `~/.ainovel/rules/*.md` nắn **cấu trúc** (số từ/cấm từ), KHÔNG ép được "thêm romance".

---

## 6. Tooling review đã có trong code (tham chiếu, khỏi viết lại)

- Prompt **sinh** profile: `internal/entry/web/profile_studio.go` `profileStudioSystemPrompt` — frame-first + long-novel survival rules + market-fit + anti-AI-tell.
- Prompt **review** (SSOT trục, dùng chung): `assets/app-production.js` — `buildProfileReviewPrompt` (profile, 13 trục) · `buildFoundationReviewPrompt` + `FOUNDATION_REVIEW_AXES`/`buildReviewAxes` (foundation, có trục "trung thành profile" bắt Architect drift).
- Nút UI: Step 4 Studio "Copy profile + prompt review"; Foundation Gate "Copy review" + "Copy bundle Review&Edit cho IDE".
- Health strip (`prodrun_health.go`): progress/rewrite_rate/cost_pace/budget/persist — nhìn nhanh run có "đúng nhịp" không.

---

## 7. Mẫu kết luận review

```
Nền móng: [đánh giá 1-2 câu]
Prose/chương: [tốt/khá/kém + bằng chứng trích dẫn]
Trục Cần sửa: [liệt kê ngắn, mỗi cái 1 dòng + cách sửa]
3 việc PHẢI sửa trước (theo mức sát thương truyện dài):
  1. ...
  2. ...
  3. ...
Can thiệp đề xuất: [Foundation revise / steer-on-resume câu cụ thể / đổi model]
```
