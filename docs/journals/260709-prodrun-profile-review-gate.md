# Journal: Production Cockpit — Health strip + Profile review gate + prompt polish

**Ngày:** 2026-07-09
**Loại:** Feature (fork additive-only, toàn bộ trong `internal/entry/web/`)
**Trạng thái:** Đã code + build/vet/test pass. Flow chưa khép kín (còn backlog), CHƯA commit.

## Bối cảnh

Đi từng điểm review-gate của flow sản xuất. Nguyên tắc: Cockpit là **automation-first** — chỉ **1 hard-gate** (Foundation Gate, đã có), phần còn lại là **biển báo/nhận biết** (không chặn), giúp user biết chỗ nào đáng review và "duyệt đúng không". Bắt đầu từ **profile** vì nó là foundation: profile sai → nhân lên hàng trăm chương.

## Đã làm

### 1. Health strip (nhận biết run có "đúng nhịp" không)

- `prodrun_health.go`: pure fn `computeRunHealth(*ProdRun) → runHealth` (có test `prodrun_health_test.go`). 5 metric từ dữ liệu đã poll: `progress` (chỉ tham khảo, không kéo overall) · `rewrite_rate` · `cost_pace` (so budget/target) · `budget` · `persist` (Windows file-lock).
- Level `idle/good/warn/bad`; `overall` = mức xấu nhất trong các metric actionable.
- `prodRunView` bọc `health` vào response `GET /api/prodruns` + `/{id}` (embed `*ProdRun` → giữ nguyên field cũ + thêm `health`).
- Frontend `app-production.js`: `renderHealthStrip` map key→nhãn VN + đèn traffic-light; tự ẩn khi mọi metric idle. CSS ở `app.css`.
- Nhãn model Studio hiện rõ `provider/model` kế thừa (thêm `studioDefault` vào `/api/models`), thay chữ "kế thừa" mơ hồ.

> Ghi chú: metric `persist` do phiên fix P0/P1/P2 (journal `260709-prodrun-persist-fail-windows`) bổ sung sau, đã hợp nhất vào cùng health strip.

### 2. Soát nền móng ở Step 4 (Profile Studio)

Khối **🔍 Soát nền móng** hiện dưới ô output khi có profile:
- **Checklist tự review** (11 mục) — bám các lỗi LLM hay mắc ở foundation truyện dài.
- **Nút "📋 Copy profile + prompt review cho LLM ngoài"** — copy *profile đã sinh* + **prompt review 13 trục** vào clipboard để dán GPT/Claude. Phân biệt rõ với "Copy cho LLM ngoài" ở Step 3 (copy *ý tưởng* để **sinh**).
- Prompt review: bối cảnh thị trường (platform+ngôn ngữ) + năm động; bắt reviewer **định khung trước** (thể loại / đã đại trà chưa / đặc trưng / thị trường) rồi soát 13 trục; phản biện thẳng, trích dẫn, chốt "3 việc phải sửa"; có trục thị trường (mã trope bản địa vs ngoại nhập, "sủng"/longing, 18+/kiểm duyệt) + bắt chỉ ra profile tự vi phạm "Điều cần tránh".

### 3. Polish prompt SINH profile (`profile_studio.go`)

Học từ một lượt review thật của Claude (bắt đúng: chỉ 1 thất bại/toàn thắng, phản diện vô danh, romance bị chính trị lấn, cơ chế lõi mơ hồ, ensemble vô danh, kết utopia, tự phạm "không phải X mà là Y"). Đưa các failure-mode đó thành **nguyên tắc genre-agnostic** để chặn từ gốc:
- **Frame-first (ngầm)**: trước khi viết, tự xác định thể loại (tôn trọng yêu cầu, không trôi về genre mặc định) · đã đại trà chưa + cliché cần tránh · đặc trưng độc giả kỳ vọng · thị trường + năm.
- **Sinh stream qua SSE** (`handleProfileGenerate`): `profileDelta`/`profileThinking`/`profileDone`/`profileError`, fallback 1-shot JSON khi không flush được (chống proxy 504, reasoning model heartbeat). *(commit `4013462`, hợp nhất vào phiên này)*
- **Long-novel survival rules**: cái giá thật ≠ twist · nhân vật có nguồn + giới hạn · phản diện có tên + tuyến phản bội có trả · phục vụ thể loại chính (longing/"sủng") · gọi tên mâu thuẫn đạo đức · ensemble có tên · đa dạng nhịp/nội tâm · cơ chế lõi có luật+giá+giới hạn · giọng riêng từng nhân vật.
- Output mở rộng 12→15 mục (thêm Costs & stakes, Antagonists tách riêng, voice trong characters, mid-pivot point-of-no-return, ending cost + title payoff, differentiation as intentional-risk).
- **Market-fit** + năm hiện tại (`time.Now().Year()` truyền vào user message).
- Payload "Copy cho LLM ngoài" (JS) mirror cùng bộ nguyên tắc.

## Bản đồ gate của flow (điểm review)

| Bước | Gate/nhận biết | Trạng thái |
|---|---|---|
| Sinh profile | sinh hợp thị trường + survival rules | ✅ |
| Review profile (Step 4) | checklist · copy cho LLM ngoài · **AI soát trong app** | 🟡 2/3 (thiếu AI-in-app) |
| Foundation Gate | approve/reject/revise/reveal/ide-bundle **+ copy review (LLM ngoài) + copy bundle Review&Edit (agent IDE)** | ✅ (đã mirror lớp review, có trục "trung thành profile" bắt Architect drift) |
| Writing | health strip · timeline signals | 🟡 (health xong, timeline chưa) |
| Arc/volume展开 | gate khi mở arc/卷 mới | ⚪ (để sau) |
| Hoàn thành → sync | guard 409/force | ✅ |

## Backlog (để khép kín flow trước khi commit)

1. **AI soát lỗi trong app** cho profile — `POST /api/profiles/review` dùng seam `bootstrap.NewModelSet(s.cfg)` + prompt review, trả findings; nút + ô hiển thị ở Step 4. → đóng gate profile (hiện mới có checklist + copy cho LLM ngoài).
2. **Review timeline** khi writing: chương mở đầu / ranh giới arc / drift chất lượng.

## Cập nhật hiện trạng (sau khi review uncommitted)

Backlog cũ #2 "mirror review sang Foundation Gate" **đã được làm** (uncommitted) — vượt dự kiến:
- `copyFoundationForReview` → copy foundation + profile gốc + prompt review 8 trục (có **trục "trung thành profile"** so foundation vs profile để bắt Architect drift) cho LLM ngoài → nhận revision note ngắn → dán vào Revise.
- `copyFoundationForIDE` → copy **bundle "Review & Edit"** cho agent IDE (Kilo/Cursor): persona + workflow 2 bước (review theo trục chết người → sửa trực tiếp 5 file `premise.md`/`compass.json`/`layered_outline.json`/`world_rules.json`/`characters.json`, surgical, không regenerate, không tốn token). Backend `GET /api/prodruns/{id}/ide-bundle` trả abs dir + file tồn tại.
- Trục review là SSOT chung (`FOUNDATION_REVIEW_AXES` + `buildReviewAxes`) cho cả copyReview và copyIde.
- Kèm persist-hardening (P0/P1/P2 từ journal `260709-prodrun-persist-fail`) + `ResumeFailed` + health dot ở list view.

## Verify

`go build ./...` · `go vet ./internal/entry/web/...` · `go test ./internal/entry/web/...` — pass. Assets embed `go:embed` → cần rebuild binary (`start-web.sh` tự build).

## Tính additive

Toàn bộ trong `internal/entry/web/` (file mới) hoặc fork-exception `server.go`/`handlers.go`. Không đụng `internal/host/`, `internal/tools/`, `assets/prompts/`.
