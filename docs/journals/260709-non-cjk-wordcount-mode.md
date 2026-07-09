# Journal: Non-CJK word count mode — feature request upstream

**Ngày:** 2026-07-09
**Loại:** Bug analysis + upstream feature request
**Trạng thái:** Workaround áp dụng locally (fork additive-only), chờ upstream

## Triệu chứng

Run-002 (sách ngôn tình Việt) sinh chương ngắn bất thường (~1200-1500 từ thay vì 2000-3500 từ chuẩn webnovel Việt). Engine cắt cấu trúc để lọt cap 6000 rune.

## Root cause

Engine đếm độ dài chương bằng `utf8.RuneCountInString` / `len([]rune(text))` — **rune count**, không phải word count.

Cap mặc định `chapter_words: {min: 3000, max: 6000}` (`internal/rules/snapshot.go:187`, `SystemDefaults`) được calibrate cho **tiếng Trung (CJK)**: 6000 rune ≈ 6000 chữ Hán = chương đầy.

Khi áp cho ngôn ngữ Latin (alphabet + dấu), mật độ rune cao gấp 3-5x:

| Ngôn ngữ | Runes/từ (TB) | 6000 rune ≈ | Chương webnovel chuẩn |
|---|---|---|---|
| Trung (CJK) | ~1-2 | 3000-6000 từ ✅ | 3000-5000 chữ |
| Việt | ~5-7 | 850-1200 từ ❌ | 2000-3500 từ |
| Tây Ban Nha | ~5-7 | 850-1200 từ ❌ | 2500-4000 từ |
| Anh | ~5-6 | 1000-1200 từ ❌ | 2000-4000 từ |

→ Mọi ngôn ngữ alphabet đều bị ép chương ngắn. Bug không giới hạn Việt — ảnh hưởng Tây Ban Nha, Anh, Pháp, Đức, v.v.

## Vị trí code gốc (upstream — KHÔNG sửa trong fork)

| File | Dòng | Vai trò |
|---|---|---|
| `internal/store/drafts.go` | 77 | `LoadChapterContent` trả `utf8.RuneCountInString(draft)` |
| `internal/rules/checker.go` | 25 | `Check` tự đếm rune khi `wordCount < 0` |
| `internal/tools/draft_chapter.go` | 111, 128 | trả `word_count: utf8.RuneCountInString(...)` |
| `internal/rules/snapshot.go` | 187 | `SystemDefaults` cap 3000-6000 (CJK-centric) |

`stylestat.go` cũng dùng `[]rune` nhưng chỉ để đo `EndingStat.MedianRunes` / short-ending — không phải cap enforcement, ít hậu quả hơn.

## Fix đề xuất cho upstream

Thêm **effective word count mode** phát theo script dominant:

```go
// internal/rules/wordcount.go (mới)
func EffectiveWordCount(text string) int {
    runes := []rune(text)
    if isCJKDominant(runes) {
        return len(runes) // CJK: rune ≈ chữ, giữ nguyên
    }
    // Latin: đếm từ (whitespace token), bỏ punctuation
    return countWhitespaceTokens(text)
}
```

- `isCJKDominant`: ratio rune trong khoảng CJK (0x4E00-0x9FFF + 0x3000-0x303F) > 60% → CJK mode, else Latin mode.
- Cap `chapter_words` trở thành "đơn vị hiệu quả" universally: chương Trung 5000 chữ và chương Tây Ban Nha 5000 từ đều pass — UX nhất quán mọi ngôn ngữ.
- `SystemDefaults` không cần đổi số (3000-6000 giờ là effective units).

**Ưu:** auto cho mọi ngôn ngữ, 0 config user. **Nhược:** đụng 4 file upstream → conflict khi merge upstream.

## Workaround áp dụng trong fork (additive-only, không đụng upstream)

Dùng cơ chế override đã có sẵn (`rules.BuildSnapshot` merge theo priority, `user_rules.json` là runtime fact source):

1. **Per-run:** sửa `meta/user_rules.json` trực tiếp — `chapter_words: {min: 9000, max: 15000}` cho Việt.
2. **Per-language global:** `~/.ainovel/rules/lang-vi.md` khai báo `chapter_words` rõ ràng để LLM normalize (`internal/userrules/normalize.go`) nhặt được — prompt yêu cầu "明确区间/上限/下限/目标字数" mới nâng lên structured.
3. **Template ngôn ngữ khác:** `~/.ainovel/rules/lang-es.md` (Tây Ban Nha min 10000 max 16000), tương tự cho lang-en/lang-fr.

`GetOrBuild` (`internal/userrules/service.go:48`) đọc snapshot có sẵn → dùng luôn, không rebuild — nên sửa `user_rules.json` trực tiếp an toàn cho run đang chạy. Chỉ khi user "refresh rules" (gọi `Build`) mới normalize lại từ rule file.

## Kết quả

- Run-002: `chapter_words` giờ 9000-15000 rune (≈ 2000-3500 từ Việt). Engine không còn ép cắt cấu trúc. Forbidden/fatigue cũng Việt hoá (bộ cũ toàn tiếng Trung, vô tác dụng với sách Việt).
- Rule file global: sách Việt mới sau này auto nhận cap đúng khi normalize thành công.
- Tây Ban Nha: có sẵn template `lang-es.md`, test sau này chỉ việc đổi file.

## Tương tác trong phiên này

- Sửa `meta/user_rules.json` run-002 (structured + preferences + sources).
- Update `~/.ainovel/rules/lang-vi.md` (thêm section mật độ + forbidden + fatigue).
- Tạo `~/.ainovel/rules/lang-es.md` (template Tây Ban Nha).

## Còn dở

- Chưa PR upstream (fork policy: additive-only). Ghi nhận đây làm note cho lúc prepare upstream PR.
- `lang-en.md` / `lang-fr.md` chưa tạo — thêm khi cần.
- Engine vẫn đếm rune ở `LoadChapterContent` / `draft_chapter` trả về `word_count` — số này sẽ "ảo cao" với Việt (15000 rune hiện thị như 15000 "字" trong UI). UI label tiếng Trung "字数" gây hiểu nhầm cho sách Việt. Đây là vấn đề hiển thị, không ảnh hưởng enforcement (checker dùng structured cap đúng).
