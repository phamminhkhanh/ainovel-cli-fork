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
Profile gốc (SSOT): `<cwd>/.ainovel/profiles/*.md` (hoặc `~/.ainovel/profiles/`, legacy `./profiles/`).

**Quy trình đọc để review nhanh (dùng `read_files`, KHÔNG dùng shell — tránh lỗi cú pháp + encoding):**
1. Profile gốc + `premise.md` + `world_rules.md` + `characters.md` + `meta/compass.json` → nắm nền móng.
2. `meta/progress.json` → tiến độ, wordcount, `strand_history` (quest/fire/emotion), `hook_history`.
3. `reviews/*.json` → Editor tự chấm (điểm 0-100/chiều, verdict `polish`/`accept`/`rewrite`). Trung thực, đọc trước để tiết kiệm.
4. Đọc trực tiếp prose: **chương 1** (giữ chân) + **1 chương giữa** + **chương mới nhất** (bắt drift). Nếu đổi model giữa chừng → đọc chương ngay trước + ngay sau mốc đổi.

**Wordcount:** engine đếm **rune**. VN/Latin ~9-15k rune/chương ≈ 2000-3500 từ (chuẩn webnovel). Nếu chương ngắn bất thường (~1-1.5k từ) → cap CJK-centric chưa fix; xem `docs/journals/260709-non-cjk-wordcount-mode.md` (workaround: `meta/user_rules.json` field `chapter_words`).

---

## 2. Nguyên tắc review chung

- **Sai ở nền móng nhân lên hàng trăm chương** → review profile/foundation nghiêm nhất.
- **Genre-agnostic**: áp cho mọi thể loại, chỉ đổi từ vựng.
- **Định khung trước khi soi**: thể loại & sub-genre? đã **đại trà** chưa (cliché phải tránh) / niche (phải nail gì cho fan cứng)? đặc trưng độc giả kỳ vọng? thị trường mục tiêu + năm hiện tại?
- **Phản biện thẳng, trích dẫn nguyên văn, không khen xã giao.** Kết bằng "3 việc phải sửa trước tiên" xếp theo mức sát thương truyện dài.

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

**J. AI-tell (soi kỹ prose)**
- 🔴 **cấu trúc "không phải X mà là Y" / "Không phải… Mà là…"** (kể cả trong worldbuilding) — lỗi phổ biến nhất, nhiều model vi phạm; purple prose; nội tâm lặp một vòng; nhịp câu đều đều; câu văn mẫu; **văn quá vụn** (câu cụt không động từ thành register mặc định → đọc như sổ tay); **recap độn** (điểm lại cả arc thay vì dựng cảnh); từ khoá lặp máy móc (vd "bond nền ấm đều" 20+ lần).

**K. Thị trường mục tiêu (năm hiện tại)**
- Mã trope **bản địa vs ngoại nhập** (vd werewolf/Alpha là mã Tây, không phải mã ngôn tình Hoa quen với độc giả Việt); kỳ vọng cảm xúc đặc trưng (VN: yếu tố "sủng"; werewolf quốc tế: fated-mate/longing); ngưỡng 18+/kiểm duyệt nền tảng.
- Lựa chọn đi **ngược trend đang thắng** = risk có chủ đích, phải bù đắp — không coi là "điểm mới lạ an toàn".
- Không chắc xu hướng năm nay → nói rõ + khuyên kiểm chứng bằng bảng bestseller nền tảng đích.

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
