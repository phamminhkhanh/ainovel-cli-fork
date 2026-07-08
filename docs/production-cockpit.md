# Production Cockpit — Hướng Dẫn Sử Dụng

Production Cockpit là tab **Sản xuất** trong Web UI của `ainovel-cli-fork`. Nó cho phép bạn xếp hàng đợi và chạy các generation job dạng headless, giám sát tiến độ + chi phí, xem lịch sử, và xuất file TXT — thay vì phải để máy đứng chờ ở TUI.

## Mục Lục

1. [Khi nào dùng](#khi-nào-dùng)
2. [Mở tab Sản xuất](#mở-tab-sản-xuất)
3. [Tạo job mới](#tạo-job-mới)
4. [Chạy và dừng job](#chạy-và-dừng-job)
5. [Theo dõi tiến độ](#theo-dõi-tiến-độ)
6. [Xuất TXT](#xuất-txt)
7. [Lưu ý quan trọng](#lưu-ý-quan-trọng)
8. [Giới hạn MVP](#giới-hạn-mvp)
9. [Gỡ lỗi nhanh](#gỡ-lỗi-nhanh)

---

## Khi nào dùng

- Bạn muốn chạy một bộ tiểu thuyết dài (ví dụ 30–100 chương) mà không cần giữ TUI mở.
- Bạn muốn chạy nhiều profile khác nhau và so sánh kết quả.
- Bạn cần giám sát chi phí API theo thờii gian thực.
- Bạn muốn xuất file TXT từ các chương đã hoàn thành.

## Mở tab Sản xuất

1. Khởi động Web UI:
   ```bash
   go run ./cmd/ainovel-cli --web
   ```
2. Mở trình duyệt tại địa chỉ hiển thị (thường là `http://localhost:8080`).
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

> Chỉ có job đang ở trạng thái **Chờ** mới hiển thị nút **Bắt đầu**. Các job **Hoàn thành**, **Lỗi**, **Đã hủy** không thể chạy lại từ UI; bạn cần tạo job mới.

## Theo dõi tiến độ

Khi job đang chạy, bảng chi tiết tự động làm mới mỗi 5 giây và hiển thị:

| Chỉ số | Ý nghĩa |
|--------|---------|
| **Chương** | Số chương đã hoàn thành / số chương mục tiêu. |
| **Đánh giá** | Số lần Editor review được ghi nhận từ `reviews/*.json`. |
| **Viết lại** | Số lần review kết luận `verdict == "rewrite"`. |
| **Chi phí** | Chi phí hiện tại / ngân sách đặt ra. |
| **Thờii gian** | Thờii gian đã chạy. |
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

`overall` là mức xấu nhất trong các chip hành động (`rewrite_rate`, `cost_pace`, `budget`). `progress` là thông tin, không phải cảnh báo chất lượng. API `create/list/get/start/stop/approve/revise` đều trả `ProdRun` kèm `health` ở top-level response.

### Các trạng thái

| Trạng thái | Ý nghĩa |
|------------|---------|
| `Chờ` | Job đã tạo, chưa chạy. |
| `Đang chạy` | Tiến trình con đang viết. |
| `Tạm dừng` | Engine pause (ví dụ pause point). Chỉ có thể Dừng hoặc Xuất TXT. |
| `Hoàn thành` | Đạt số chương mục tiêu hoặc engine tự kết thúc. |
| `Lỗi` | Tiến trình con thoát với lỗi. |
| `Đã hủy` | Ngườii dùng nhấn Dừng. |

## Xuất TXT

Khi job đã có ít nhất một chương hoàn thành:

1. Chọn job.
2. Nhấn **⬇ Xuất TXT**.
3. Trình duyệt sẽ tải về file TXT được nối từ các file `chapters/*.md` trong thư mục của run.

> Xuất TXT là server-side concatenation, không dùng `s.eng.Export()`. Do đó nó vẫn thành công ngay cả khi bạn đang mở chapter file trong IDE/file watcher trên Windows.

## Lưu ý quan trọng

- **Rebuild binary**: sau mỗi lần sửa file trong `internal/entry/web/assets/`, bạn phải chạy `go build ./...` rồi khởi động lại binary. Browser refresh không đủ vì asset được embed qua `go:embed`.
- **Một job chạy một lúc**: MVP không hỗ trợ chạy song song nhiều job. Nếu cần chạy nhiều, tạo job và chạy từng cái.
- **Tiến trình con bị kill khi đạt target**: Cockpit poll `meta/progress.json` và kill child khi `len(completed_chapters) >= targetChapters`, vì engine chưa có config `max_chapters`.
- **Khôi phục sau crash**: khi Web UI khởi động lại, các job trước đó đang `running` sẽ bị đánh dấu `failed` với cờ `PossiblyOrphaned`. Bạn nên kiểm tra PID cũ trên hệ thống và kill tay nếu cần.
- **Pause là read-only**: khi engine pause, UI chỉ hiển thị thông báo. MVP chưa có nút **Tiếp tục**; bạn chỉ có thể Dừng hoặc Xuất TXT.
- **Spike test Unix-only**: `scripts/model-spike-test.sh` dùng bash / `kill` / `find` / python3; trên Windows cần chạy trong Git Bash hoặc WSL.

## Giới hạn MVP

- Chỉ hỗ trợ xuất định dạng **TXT**.
- Không có scheduling / queue tự động.
- Không chạy song song nhiều job.
- Không có nút **Tiếp tục** cho paused child.
- Không hiển thị real-time streaming; chỉ poll mỗi 5 giây.

## Gỡ lỗi nhanh

| Triệu chứng | Cách xử lý |
|-------------|------------|
| Job tạo xong không chạy được | Kiểm tra profile path có tồn tại không; xem log server. |
| Chương không tăng nhưng vẫn `Đang chạy` | Kiểm tra `run.log` xem engine có đang pause hoặc lỗi loop. |
| Chi phí không cập nhật | Kiểm tra `meta/progress.json` và `run.log` có ghi cost không. |
| Xuất TXT lỗi | Đảm bảo thư mục `{runDir}/output/novel/chapters/` tồn tại và có file `.md`. |
| `PossiblyOrphaned` | Kiểm tra PID cũ trong Task Manager / `ps` và kill nếu còn. |

---

## Liên Kết

- Kiến trúc Web UI: [`02-WEB-UI.md`](02-WEB-UI.md)
- Lưu ý merge upstream: [`04-LUU-Y-MERGE-UPSTREAM.md`](04-LUU-Y-MERGE-UPSTREAM.md)
- Code backend: `internal/entry/web/prodrun*.go`
- Code frontend: `internal/entry/web/assets/app-production.js`

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
- **Copy rules** — `~/.ainovel/rules/*.md` (lang-vi.md...) được copy vào runDir → engine tự nạp rule tiếng Việt.
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
| Cocreate (đồng sáng tác) | ❌ | ✅ | ✅ |
| Xuất TXT | ✅ (server-side) | ✅ (eng.Export) | ✅ |
| Xuất EPUB | ❌ | ✅ | ✅ |
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

- **Mỗi job = 1 cuốn sách mới** — vì `--prompt-file` luôn truyền prompt → engine luôn `StartPrepared` (sách mới), không bao giờ `Resume` (tiếp tục dở).
- **Sách thủ công không bị đụng** — vì job chạy ở `output/jobs/run-XXX/`, tách biệt với workspace chính `output/novel/`.
- **Sync ngược về workspace** — nếu workspace đã có chương, sync bị chặn (409). Cần `force: true` → **xóa sạch** chương thủ công + toàn bộ meta, rồi copy sách của job vào. **Không có merge** — đây là overwrite.
- **Crash Web UI** → run đang `running` bị `failed` + `PossiblyOrphaned` → kiểm tra PID cũ và kill tay nếu cần.
