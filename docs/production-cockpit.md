# Production Cockpit — Hướng Dẫn Sử Dụng

Production Cockpit là tab **Sản xuất** trong Web UI của `ainovel-cli-fork`. Nó cho phép bạn xếp hàng đợi và chạy các generation job dạng headless, giám sát tiến độ + chi phí, xem lịch sử, và xuất file TXT — thay vì phải để máy đứng chờ ở TUI.

> ⚠️ **Sau merge Engine+Arbiter (2026-07-23):** engine mới **không còn pause point**. Khi engine tự pause (deadlock / worker failure) thì **child process thoát luôn với exit 0**. Cockpit phân biệt được trường hợp này: job chuyển `Tạm dừng` với lý do `engine_paused`, **không** gán nhầm `Hoàn thành`. Đọc [07-VAN-HANH-HE-THONG-MOI.md §9](../07-VAN-HANH-HE-THONG-MOI.md#9-production-cockpit--tab-sản-xuất-headless) trước khi chạy batch.

## Mục Lục

1. [Khi nào dùng](#khi-nào-dùng)
2. [Mở tab Sản xuất](#mở-tab-sản-xuất)
3. [Tạo job mới](#tạo-job-mới)
4. [Chạy và dừng job](#chạy-và-dừng-job)
5. [Tiếp tục job](#tiếp-tục-job)
6. [Theo dõi tiến độ](#theo-dõi-tiến-độ)
7. [Xuất TXT / EPUB](#xuất-txt--epub)
8. [Lưu ý quan trọng](#lưu-ý-quan-trọng)
9. [Giới hạn MVP](#giới-hạn-mvp)
10. [Gỡ lỗi nhanh](#gỡ-lỗi-nhanh)

---

## Khi nào dùng

- Bạn muốn chạy một bộ tiểu thuyết dài (ví dụ 30–100 chương) mà không cần giữ TUI mở.
- Bạn muốn chạy nhiều profile khác nhau và so sánh kết quả.
- Bạn cần giám sát chi phí API theo thời gian thực.
- Bạn muốn xuất file TXT từ các chương đã hoàn thành.

## Mở tab Sản xuất

1. Khởi động Web UI:
   ```bash
   go run ./cmd/ainovel-cli --web
   ```
2. Mở trình duyệt tại địa chỉ hiển thị (mặc định `http://127.0.0.1:8787`).
3. Trong thanh tab, chọn **Sản xuất**. Tab này nằm sau **Đánh giá**, trước **Hỗ trợ**.

Giao diện chia làm hai vùng:
- **Bên trái**: danh sách job.
- **Bên phải**: chi tiết job đang chọn.

## Tạo job mới

1. Nhấn **+ Tạo** ở góc trên bên trái.
2. Điền form:
   - **Tên job**: tên gợi nhớ, ví dụ `Werewolf romantasy 50 chương`.
   - **Profile**: chọn profile tạo truyện từ `./.ainovel/profiles/`, `~/.ainovel/profiles/`, hoặc legacy `./profiles/`. Xem mục [Profile là gì](#profile-là-gì) bên dưới.
   - **Model (tùy chọn)**: ghi đè model, ví dụ `gpt-4o`.
   - **Provider (tùy chọn)**: ghi đè provider, ví dụ `openai`.
   - **Số chương mục tiêu**: số chương tối đa muốn chạy.
   - **Ngân sách (USD)**: ngân sách tối đa. Hệ thống luôn bật `HardStop`.
3. Nhấn **Tạo job**. Job mới sẽ xuất hiện trong danh sách với trạng thái **Chờ**.

## Chạy và dừng job

- Chọn job trong danh sách bên trái.
- Nhấn **▶ Bắt đầu**. Hệ thống spawn một tiến trình con `ainovel-cli --headless`.
- Để dừng, nhấn **■ Dừng**. Tiến trình con bị kill ngay lập tức; trạng thái chuyển thành **Đã hủy**.

> Chỉ có job đang ở trạng thái **Chờ** mới hiển thị nút **Bắt đầu**. Job **Lỗi** và **Đã hủy** có thể **Tiếp tục** (resume) — xem mục [Tiếp tục job](#tiếp-tục-job) bên dưới. Job **Hoàn thành** không chạy lại được; cần tạo job mới.

## Tiếp tục job

Job ở trạng thái **Lỗi** hoặc **Đã hủy** có thể tiếp tục (resume) mà không mất tiến độ đã viết:

1. Chọn job **Lỗi** hoặc **Đã hủy** trong danh sách.
2. Nhấn **▶ Tiếp tục**. Backend copy home rules mới nhất vào sandbox rồi spawn `ainovel-cli --headless` không `--prompt-file` → engine native `Resume()` từ checkpoint đã có.
3. (Tùy chọn) Kèm **steer** — ghi干预文本 vào `meta/run.json` trước khi start → engine inject ngay chương kế. Steer là干预 mềm (Coordinator đánh giá & áp), không phải structural rule cứng.

> Resume chỉ chạy khi run dir đã có output (đã qua Foundation Gate). Run fail trước foundation (0 chương, chưa seed) sẽ chạy `--prompt-file` lại từ đầu — không phải resume.

## Theo dõi tiến độ

Khi job đang chạy, bảng chi tiết tự động làm mới mỗi 5 giây và hiển thị:

| Chỉ số | Ý nghĩa |
|--------|---------|
| **Chương** | Số chương đã hoàn thành / số chương mục tiêu. |
| **Đánh giá** | Số lần Editor review được ghi nhận từ `reviews/*.json`. |
| **Viết lại** | Số lần review kết luận `verdict == "rewrite"`. |
| **Chi phí** | Chi phí hiện tại / ngân sách đặt ra. |
| **Thời gian** | Thời gian đã chạy. |
| **Lý do dừng** | Lý do khi job kết thúc (đạt target, bị dừng tay, lỗi, v.v.). |

Dưới cùng là **Nhật ký** (`run.log`) của tiến trình con.

### Health strip

Mỗi job có một dải health dạng traffic-light, được backend tính từ các số liệu runner đã poll — không gọi thêm engine/model:

| Chip | Nguồn dữ liệu | Cách hiểu |
|---|---|---|
| `progress` | `Chapters / TargetChapters` | Chỉ để nhìn tiến độ; không kéo overall xuống khi job còn sớm. |
| `rewrite_rate` | `Rewrites / Reviews` | `idle` khi chưa đủ 3 review; `warn` khi >25%, `bad` khi >50%. |
| `cost_pace` | `CostUSD / Chapters` so với `BudgetUSD / TargetChapters` | `idle` trước 2 chương; `warn` khi >1.2x pace dự kiến, `bad` khi >2x. |
| `budget` | `CostUSD / BudgetUSD` | `warn` từ 80%, `bad` khi chạm/vượt ngân sách. |
| `persist` | `PersistError` trên `jobs.json` | `idle` khi persist OK; `bad` (đỏ) khi lỗi file lock mới (<5 phút, thường do IDE mở `jobs.json`), `warn` (vàng) khi lỗi cũ không tái diễn. |

`overall` là mức xấu nhất trong các chip hành động (`rewrite_rate`, `cost_pace`, `budget`, `persist`). `progress` là thông tin, không phải cảnh báo chất lượng. Mọi API prodrun (`create/list/get/start/stop/approve/reject/revise/resume/sync`) đều trả `ProdRun` kèm `health` ở top-level response.

### Các trạng thái

| Trạng thái | Ý nghĩa |
|------------|---------|
| `Chờ` | Job đã tạo, chưa chạy. |
| `Đang chạy` | Tiến trình con đang viết. |
| `Tạm dừng` | Engine tự pause giữa chừng (deadlock / worker failure / gate lỗi) rồi child exit 0. Runner gán `stopReason=engine_paused`. Child đã chết — chưa có nút **Tiếp tục** cho paused, xem `run.log` rồi tạo job mới. |
| `Hoàn thành` | Truyện thật sự xong (`phase=complete`) hoặc đạt số chương mục tiêu. |
| `Lỗi` | Tiến trình con thoát với lỗi. Có thể **Tiếp tục** (resume) sau khi sửa nguyên nhân transient. |
| `Đã hủy` | Người dùng nhấn Dừng. Có thể **Tiếp tục** (resume) nếu muốn nấu tiếp. |

## Xuất TXT / EPUB

Khi job đã có ít nhất một chương hoàn thành:

1. Chọn job.
2. Nhấn **⬇ Xuất TXT** (file nối từ `chapters/*.md`) hoặc **📕 Xuất EPUB** (EPUB 3 chuẩn: cover + mục lục + XHTML từng chương, đọc sạch trên điện thoại).
3. Trình duyệt tải về file tương ứng trong `{runDir}/export/`.

> **TXT** là server-side concatenation. **EPUB** build web-side (`prodrun_export_epub.go`) dùng **header gốc của writer** làm nhãn chương (không dùng `internal/host/exp` vì nó hardcode nhãn Trung "第 N 章"/`zh-CN`, sai cho truyện VN/EN/ES). Cả hai chỉ ĐỌC file chương + atomic-write file output riêng (không rename file chương) → vẫn chạy khi bạn đang mở chapter file trong IDE/file watcher trên Windows.

## Lưu ý quan trọng

- **Rebuild binary**: sau mỗi lần sửa file trong `internal/entry/web/assets/`, bạn phải chạy `go build ./...` rồi khởi động lại binary. Browser refresh không đủ vì asset được embed qua `go:embed`.
- **Một job chạy một lúc**: MVP không hỗ trợ chạy song song nhiều job. Nếu cần chạy nhiều, tạo job và chạy từng cái.
- **Tiến trình con bị kill khi đạt target**: Cockpit poll `meta/progress.json` và kill child khi `len(completed_chapters) >= targetChapters`, vì engine chưa có config `max_chapters`.
- **Khôi phục sau crash**: khi Web UI khởi động lại, các job trước đó đang `running` sẽ bị đánh dấu `failed` với cờ `PossiblyOrphaned`. Bạn nên kiểm tra PID cũ trên hệ thống và kill tay nếu cần.
- **Pause là read-only**: khi engine tự pause (deadlock / worker failure), child exit 0 và runner gán trạng thái `Tạm dừng` + lý do `engine_paused` (dựa trên `meta/progress.json` thực tế — không còn gán nhầm `Hoàn thành`). Paused child chưa có nút **Tiếp tục**; mở `run.log` tìm `已暂停` / `引擎停止` để biết lý do pause rồi tạo job mới. (Job **Lỗi**/**Đã hủy** thì có resume — xem [Tiếp tục job](#tiếp-tục-job).)
- **Spike test Unix-only**: `scripts/model-spike-test.sh` dùng bash / `kill` / `find` / python3; trên Windows cần chạy trong Git Bash hoặc WSL.

## Giới hạn MVP

- Hỗ trợ xuất **TXT + EPUB 3** (EPUB build web-side, nhãn chương theo header gốc của writer — đúng ngôn ngữ VN/EN/ES).
- Không có scheduling / queue tự động.
- Không chạy song song nhiều job.
- Không có nút **Tiếp tục** cho paused child (engine self-pause). Job **Lỗi**/**Đã hủy** thì có resume.
- Không hiển thị real-time streaming; chỉ poll mỗi 5 giây.

## Gỡ lỗi nhanh

| Triệu chứng | Cách xử lý |
|-------------|------------|
| Job tạo xong không chạy được | Kiểm tra profile path có tồn tại không; xem log server. |
| Chương không tăng nhưng vẫn `Đang chạy` | Kiểm tra `run.log` xem engine có đang pause (`已暂停`) hoặc lỗi loop. |
| Job hiển thị `Tạm dừng` với lý do `engine_paused` | Engine đã pause giữa chừng (deadlock / worker failure / gate lỗi) rồi child exit 0. Mở `run.log` tìm `已暂停` / `引擎停止 (已完成 N 章)`, đọc `meta/decisions.jsonl` để biết arbiter đã phán gì. |
| Chi phí không cập nhật | Kiểm tra `meta/progress.json` và `run.log` có ghi cost không. |
| Xuất TXT lỗi | Đảm bảo thư mục `{runDir}/output/novel/chapters/` tồn tại và có file `.md`. |
| `PossiblyOrphaned` | Kiểm tra PID cũ trong Task Manager / `ps` và kill nếu còn. |

---

## Liên Kết

- Kiến trúc Web UI: [`02-WEB-UI.md`](../02-WEB-UI.md)
- Lưu ý merge upstream: [`04-LUU-Y-MERGE-UPSTREAM.md`](../04-LUU-Y-MERGE-UPSTREAM.md)
- State machine kỹ thuật: [`docs/prodrun-state-machine.md`](prodrun-state-machine.md)
- Code backend: `internal/entry/web/prodrun*.go`
- Code frontend: `internal/entry/web/assets/app-production.js`

---

## Foundation Gate (duyệt nền móng)

Khi tạo job `fresh_profile`, Cockpit **tự động dừng** sau khi Architect xong nền móng (premise/outline/world/characters), trước khi Writer bắt đầu viết hàng trăm chương. Bạn xem nền móng rồi quyết định:

| Hành động | Nút | Cơ chế | Chi phí |
|---|---|---|---|
| **Duyệt** | **✓ Duyệt** | restart cùng run dir → native `Resume()` vào writing | tốn token viết (như bình thường) |
| **Từ chối** | **✕ Từ chối** | xoá run + run dir | 0 token Writer |
| **Sửa tay + Duyệt** | **📂 Mở thư mục** | sửa 5 file nền móng (`premise.md`, `compass.json`, `layered_outline.json`, `world_rules.json`, `characters.json`) rồi Duyệt | 0 token — phẫu thuật chính xác |
| **Sửa lại (AI)** | **↻ Sửa lại** | ghép góp ý vào `profile.md` → tạo run MỚI regenerate; run cũ giữ lại làm dự phòng | ~$0.01 sinh nền móng |
| **Copy cho IDE** | **📋 Copy cho IDE** | bundle "Review & Edit" cho agent IDE (Kilo/Cursor) soi theo trục chết người rồi sửa trực tiếp 5 file | 0 token regenerate |

> **Best-effort (poll 5s):** engine flip `phase=writing` đồng bộ trong `save_foundation` rồi dispatch "viết chương 1", nên tệ nhất Writer kịp draft dở chương 1 trước khi poll tick lands. Không mất hàng trăm chương.

Xem chi tiết: [`docs/journals/260705-foundation-gate.md`](journals/260705-foundation-gate.md).

---

## Profile Library & Studio

### Thư viện Profile

Modal **📚 Thư viện Profile** list profile từ 3 nguồn (project/global/legacy). Chỉ profile **project** (`./.ainovel/profiles/`) sửa/xoá được; global/legacy read-only.

| Hành động | API | Ghi chú |
|---|---|---|
| Xem nội dung | `GET /api/profiles/content?path=project/foo.md` | |
| Lưu (project-only) | `POST /api/profiles/save` | trùng tên chưa `overwrite` → 409 |
| Xoá (project-only) | `POST /api/profiles/delete` | global/legacy → 403 |

### Profile Studio (sinh profile từ ý tưởng)

Studio 4 bước trong modal Thư viện:

1. **Brief** — chọn template hoặc gõ tay.
2. **Ý tưởng** — nhập ý tưởng thô + field (thể loại, platform, ngôn ngữ, số chương, phong cách).
3. **Sinh profile** — stream realtime qua SSE (`profileDelta`/`profileThinking`/`profileDone`/`profileError`; fallback 1-shot JSON khi không flush được) qua model **default trong config lúc khởi động** (nhãn hiện rõ `provider/model`). Hoặc **📋 Copy cho LLM ngoài** để LLM ngoài sinh.
4. **Kết quả** — sửa → **Lưu**.

Prompt sinh genre-agnostic, principle-based: **frame-first** (tự xác định thể loại / đã đại trà chưa / đặc trưng / thị trường *trước khi viết*), **long-novel survival rules**, **market-fit**, anti-AI-tell. Studio **không tự lưu/chạy** — bạn lưu thủ công rồi tạo job.

---

## Continue workspace (cook tiếp truyện đang dở)

Ngoài `fresh_profile` (truyện mới từ profile), Cockpit hỗ trợ **cook tiếp** workspace chính:

1. Nhấn **Cook tiếp** trong tab Sản xuất.
2. Backend tạo job `continue_workspace` và lưu `SeededFrom` (chương đã hoàn thành + fingerprint SHA-256 của workspace).
3. Seed thật xảy ra ở **Start** (không phải Create): re-fingerprint workspace, nếu khác seed ban đầu → lỗi `workspace changed since the continue run was created`; copy workspace sang sandbox `output/jobs/{id}/output/novel/`; re-fingerprint lần nữa sau copy để bắt race.
4. Runner spawn `ainovel-cli --headless` **không `--prompt-file`** → engine native `Resume()` từ checkpoint đã seed.

> `targetChapters` là **tổng số chương tuyệt đối cuối cùng**, không phải số chương viết thêm. Ví dụ workspace đang có 12 chương, muốn viết đến 100 chương thì nhập `100`.

> Cockpit không hiểu/điều phối logic viết tiếp; nó chỉ là file-plumber + process spawner. Continue sync về workspace chính theo kiểu `git fast-forward` (fingerprint khớp → copy file-by-file; diverge → 409, hỏi `force`).

---

## Profile là gì

**Profile = file `.md` chứa "công thức" (prompt) để tạo 1 cuốn sách mới.** Về bản chất, nó chính là **prompt** mà bạn truyền vào `--prompt-file` khi chạy headless. Nó là **đầu vào duy nhất** của một job Sản xuất, quyết định thể loại, nội dung, quy mô.

### Nằm ở đâu

Production Cockpit đọc profile theo 3 nguồn:

| Ưu tiên | Nguồn | Dùng khi nào | API value |
|---|---|---|---|
| 1 | `./.ainovel/profiles/` | Profile riêng của project hiện tại | `project/foo.md` |
| 2 | `~/.ainovel/profiles/` | Profile cá nhân dùng lại giữa nhiều project | `global/foo.md` |
| 3 | `./profiles/` | Legacy/sample cũ, vẫn hỗ trợ nhưng không khuyến nghị | `legacy/foo.md` hoặc old `profiles/foo.md` |

Ghi chú:

- `./.ainovel/` là thư mục bạn chạy `ainovel-cli --web`; giống project config.
- `~/.ainovel/` là thư mục home của máy (`/Users/.../.ainovel` trên macOS, `C:\Users\...\.ainovel` trên Windows).
- Có thể đặt profile trong thư mục con, ví dụ `~/.ainovel/profiles/romance/werewolf-50ch.md`.
- Chỉ nhận file `.md`; path traversal bị chặn.

### Cấu trúc

Profile là file `.md` thuần, **không có YAML frontmatter**. Nội dung là **prompt tự nhiên** (câu yêu cầu). Engine đọc nó và chạy `StartPrepared` — giống hệt bạn gõ prompt vào ô input "Bắt đầu" của Web UI.

### Đặc điểm

- **Override model/provider** (tùy chọn) — `ProdRun.Model`/`ProdRun.Provider` đè lên `config.json` của job.
- **Override budget** — `BudgetUSD` (default $5, `HardStop: true`).
- **Copy rules (lọc theo ngôn ngữ)** — rule từ `~/.ainovel/rules/*.md` được copy vào sandbox sau khi lọc theo ngôn ngữ của run (`vi`/`es`/`en`): chỉ rule trung tính (không hậu tố mã) + rule khớp mã ngôn ngữ được copy, rule ngôn ngữ khác bị bỏ. Kết hợp re-root HOME của child về sandbox (`withSandboxHome`) để engine đọc "global rules" từ bản đã lọc, không đọc `~/.ainovel/rules` thật. Danh sách file thực copy hiện trong panel run (`RuleFiles`).
- **Không có override** cho `style` (fantasy/romance/suspense) — nó phải nằm trong prompt.

### Ví dụ

Tạo file `~/.ainovel/profiles/werewolf-50ch.md` hoặc `./.ainovel/profiles/werewolf-50ch.md`:

```markdown
Viết một cuốn truyện werewolf romantasy 50 chương, tiếng Việt.
Bối cảnh: rừng Alpine, mate bond.
Nhân vật chính: cô gái con người, có thể biến hình.
Hướng kết: happy end.
```

→ Trong Web UI, chọn profile này → tạo job → chạy → sách mới.

---

## Config override

Config cũng dùng mô hình `.ainovel` 2 lớp. Engine đọc theo thứ tự dưới đây; cái sau ghi đè cái trước:

| Ưu tiên | File | Ý nghĩa |
|---|---|---|
| 1 | `~/.ainovel/config.json` | Global config mặc định: provider, API key, model |
| 2 | `./.ainovel/config.json` | Project override cho thư mục đang chạy |
| 3 | `--config path` | Override cao nhất khi chạy CLI |

`./.ainovel/` là optional; nếu bạn chưa tạo thì engine chỉ dùng global config.

---

## Sản xuất vs TUI/Web UI

Sản xuất = cùng engine, không có gì bị skip. Sản xuất chỉ thiếu **Steer** (can thiệp) và **Cocreate** (chat) vì headless không có ô input.

| Tính năng | Sản xuất | TUI | Web UI |
|---|:---:|:---:|:---:|
| Tạo nhân vật | ✅ | ✅ | ✅ |
| Xây thế giới | ✅ | ✅ | ✅ |
| Review 7 chiều | ✅ | ✅ | ✅ |
| Quy hoạch cuốn chiếu | ✅ | ✅ | ✅ |
| Khôi phục checkpoint | ✅ | ✅ | ✅ |
| Can thiệp realtime (Steer) | ❌ | ✅ | ✅ |
| Steer-on-resume (干预 mềm) | ✅ | — | — |
| Cocreate (đồng sáng tác) | ❌ | ✅ | ✅ |
| Foundation Gate (duyệt nền móng) | ✅ (auto) | ⚠️ (`/review on`) | ❌ |
| Profile Studio (sinh profile) | ✅ | ❌ | ❌ |
| Resume failed/cancelled | ✅ | — | — |
| Xuất TXT | ✅ (server-side) | ✅ (eng.Export) | ✅ |
| Xuất EPUB | ✅ (web-side, nhãn theo header writer) | ✅ | ✅ |
| Đọc chương | ✅ (log) | ✅ (stream) | ✅ (stream + tabs) |

### Cấu trúc thư mục — 2 thư mục tách biệt

```
output/
├── novel/                          ← SÁCH THỦ CÔNG (workspace chính, eng.Dir())
│   ├── chapters/01.md, 02.md ...    ← bạn đã viết bằng mode thủ công
│   └── meta/progress.json
│
└── jobs/                           ← SẢN XUẤT (jobsDir)
    ├── jobs.json                   ← danh sách ProdRun
    └── run-001/                    ← runDir của job
        ├── profile.md              ← copy từ project/global/legacy profile
        ├── .ainovel/config.json    ← override config
        ├── .ainovel/rules/         ← copy từ ~/.ainovel/rules/
        ├── run.log
        └── output/novel/           ← SÁCH MỚI CỦA JOB (tách biệt)
            ├── chapters/01.md ...  ← chương mới, không liên quan sách thủ công
            └── meta/progress.json
```

### Lưu ý quan trọng

- **Mỗi job `fresh_profile` = 1 cuốn sách mới** — vì `--prompt-file` luôn truyền prompt → engine luôn `StartPrepared` (sách mới). Job `continue_workspace` thì ngược lại: **không** `--prompt-file` → engine native `Resume()` (tiếp tục dở).
- **Sách thủ công không bị đụng** — vì job chạy ở `output/jobs/run-XXX/`, tách biệt với workspace chính `output/novel/`.
- **Sync ngược về workspace** — nếu workspace đã có chương, sync bị chặn (409). Cần `force: true` → **xóa sạch** chương thủ công + toàn bộ meta, rồi copy sách của job vào. **Không có merge** — đây là overwrite.
- **Job continue luôn chạy chế độ tự động** — job `continue_workspace` copy cả `meta/` của workspace chính sang sandbox, trong đó có chế độ duyệt chương (`advance_mode`). Nếu bạn từng bật `/review on` trên TUI, sandbox sẽ thừa hưởng chế độ duyệt và job đứng chờ `/next` mãi (Cockpit không có nút này). Vì vậy sau khi seed, Cockpit **tự ép sandbox về `auto`** và xoá lệnh tạm dừng một lần. Workspace chính của bạn **không bị đổi** — muốn duyệt từng chương thì làm trên TUI.
- **Crash Web UI** → run đang `running` bị `failed` + `PossiblyOrphaned` → kiểm tra PID cũ và kill tay nếu cần.
