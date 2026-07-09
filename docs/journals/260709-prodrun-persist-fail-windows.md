# Journal: prodrun persist-fail trên Windows — backlog QA

**Ngày:** 2026-07-09
**Loại:** QA bug report + backlog cho dev
**Trạng thái:** Đã xác minh bằng code, CHƯA sửa (QA mode)
**Phạm vi sửa:** toàn bộ trong `internal/entry/web/` (fork territory, merge-safe)

## Bằng chứng log

```
prodrun: failed to persist target-reached status for run-002: 
rename output\jobs\jobs.json.tmp output\jobs\jobs.json: Access is denied.
```

## Bug 1 (QA báo): persist fail im lặng trên UI

**Vị trí:** 9 chỗ `fmt.Fprintf(os.Stderr, ...)` trong `prodrun_runner.go` (dòng 134, 156, 181, 255, 277, 292, 306, 347, 364).

**Cơ chế:** `prodRunStore.update()` (`prodrun.go:334`) gọi closure `fn(r)` mutate in-memory TRƯỚC, rồi `saveLocked()` (`prodrun.go:236`) persist ra disk. Khi persist fail:
- In-memory: đã mutate (comment dòng 332-333 nói rõ "mutated even if persistence fails").
- Disk `jobs.json`: stale (vẫn status cũ).
- UI đọc in-memory → live session thấy status mới, **nhưng restart engine → load stale jobs.json → run stuck "running" không có process**.
- 9 chỗ fail đều chỉ `Fprintf(os.Stderr)` → UI không bao giờ biết.

**`ProdRun` struct (`prodrun.go:80-114`) KHÔNG có field Error** — chỉ có `StopReason` (string) + `PossiblyOrphaned` (bool). Không có chỗ nào lưu "lần persist gần nhất fail vì gì".

## Bug 3 (QA chưa nêu — NGHIÊM TRỌNG hơn): target-reached persist fail → process TIẾP TỤC TIÊU TIỀN

**Vị trí:** `prodrun_runner.go:298-311`.

```go
targetReached := false
if _, err := rr.store.update(id, func(r *ProdRun) {
    if r.Status == prodRunRunning && r.TargetChapters > 0 && chapters >= r.TargetChapters {
        r.Status = prodRunCompleted
        r.StopReason = stopReasonTargetReached
        r.StoppedAt = time.Now()
        targetReached = true   // ← closure chạy TRƯỚC saveLocked, nên set dù persist fail
    }
}); err != nil {
    fmt.Fprintf(os.Stderr, "prodrun: failed to persist target-reached status for %s: %v\n", id, err)
    return                      // ← SKIP killProcess ở dòng 310
}
if targetReached {
    rr.killProcess(id)          // ← bị bỏ qua khi persist fail
}
```

**Hậu quả:** target-chapters đạt → closure set `targetReached=true` + in-memory completed, nhưng `saveLocked` fail → `return` → `killProcess` không chạy → **child process tiếp tục viết chương, tiêu credit/tiền** sau khi đã đạt target. Đồng thời 3-way inconsistency: in-memory=completed, disk=running, process=alive.

**Đây là code path duy nhất bị lỗi pattern này.** Các path khác đều safe:
- `waitProc` (dòng 166-182): persist fail → chỉ log, vẫn close logFile + free slot + close done. Safe.
- `cancel` (dòng 352-354): `proc.cmd.Process.Kill()` TRƯỚC persist. Process chết dù persist fail. Safe.

Root cause pattern: target-reached persist FIRST rồi kill, các path khác kill FIRST rồi persist (hoặc không cần kill).

## Bug 2 (QA báo): saveLocked không retry dù retry helper đã tồn tại

**Phát hiện then chốt:** `safeWriteFile()` (`prodrun_runner.go:932`) **đã có retry 5 lần × 50ms** cho Windows file lock (comment dòng 931: "retrying a few times to tolerate transient Windows file locks"). Nhưng `prodRunStore.saveLocked()` (`prodrun.go:223-237`) dùng `os.Rename` trần **không retry**:

```go
func (ps *prodRunStore) saveLocked() error {
    // ...
    tmp := ps.path + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return fmt.Errorf("write prodrun store tmp: %w", err)
    }
    return os.Rename(tmp, ps.path)   // ← 0 retry, 1 phát fail luôn
}
```

Retry helper tồn tại trong cùng package nhưng persist path jobs.json không dùng. Mâu thuẫn nội bộ: dev đã biết Windows lock vấn đề (viết safeWriteFile) nhưng không áp dụng cho jobs.json — file dễ bị IDE/editor lock nhất.

`saveLocked` có 4 caller (`prodrun.go:202, 220, 295, 344, 358`) — toàn trong fork territory, sửa merge-safe.

**Health strip (`prodrun_health.go`):** 4 metric (progress, rewrite_rate, cost_pace, budget). KHÔNG có metric persist/file-lock. `computeRunHealth` là pure function derive từ stats — không detect file lock.

## Fix đề xuất — phân ưu tiên

### P0: target-reached kill bất kể persist (Bug 3) — 1 dòng

`prodrun_runner.go:298-311`. Di chuyển `killProcess` ra ngoài nhánh persist-fail, hoặc kill trước persist:

```go
targetReached := false
if _, err := rr.store.update(id, func(r *ProdRun) {
    if r.Status == prodRunRunning && r.TargetChapters > 0 && chapters >= r.TargetChapters {
        r.Status = prodRunCompleted
        r.StopReason = stopReasonTargetReached
        r.StoppedAt = time.Now()
        targetReached = true
    }
}); err != nil {
    fmt.Fprintf(os.Stderr, "prodrun: failed to persist target-reached status for %s: %v\n", id, err)
    // KHÔNG return — vẫn phải kill
}
if targetReached {
    rr.killProcess(id)
}
```

Risk: LOW. Process chết đúng lúc target — đó là intent gốc. In-memory đã completed. Chỉ disk stale (P1 xử lý).

### P1: saveLocked dùng retry (Bug 2 root) — copy safeWriteFile logic

`prodrun.go:223-237`. Thay `os.Rename` trần bằng retry loop (hoặc gọi thẳng `safeWriteFile` nếu refactor được — cùng package):

```go
func (ps *prodRunStore) saveLocked() error {
    if err := os.MkdirAll(filepath.Dir(ps.path), 0o755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(ps.listLocked(), "", "  ")
    if err != nil {
        return fmt.Errorf("marshal prodrun store: %w", err)
    }
    tmp := ps.path + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return fmt.Errorf("write prodrun store tmp: %w", err)
    }
    var lastErr error
    for i := 0; i < 5; i++ {
        if lastErr = os.Rename(tmp, ps.path); lastErr == nil {
            return nil
        }
        time.Sleep(50 * time.Millisecond)
    }
    _ = os.Remove(tmp)
    return lastErr
}
```

Risk: LOW. Giữ nguyên语义, chỉ thêm retry. Test `prodrun_test.go:48` (persist/recover) vẫn pass — retry thành công nhanh trên test temp dir (không lock).

### P2: surface persist error lên UI (Bug 1)

Thêm field `PersistError string` + `PersistErrorAt time.Time` vào `ProdRun` struct. Khi `saveLocked` fail, `update` ghi error vào run (in-memory) để UI đọc được. Nhưng cẩn thận: `update` gọi `fn` rồi `saveLocked` — nếu saveLocked fail, cần ghi PersistError mà không trigger đệ quy persist. Pattern:

```go
func (ps *prodRunStore) update(id string, fn func(*ProdRun)) (*ProdRun, error) {
    ps.mu.Lock()
    defer ps.mu.Unlock()
    r, ok := ps.runs[id]
    if !ok { return nil, fmt.Errorf("run %q not found", id) }
    fn(r)
    if err := ps.saveLocked(); err != nil {
        r.PersistError = err.Error()
        r.PersistErrorAt = time.Now()
        // best-effort persist lại để ghi luôn error — nếu lại fail thì bỏ qua
        _ = ps.saveLocked()
        cp := *r
        return &cp, err
    }
    r.PersistError = ""  // clear lỗi cũ khi persist thành công
    cp := *r
    return &cp, nil
}
```

UI: `prodrun_health.go` thêm metric `persist`:

```go
func persistMetric(r *ProdRun) healthMetric {
    if r.PersistError == "" {
        return healthMetric{Key: "persist", Value: "ok", Level: healthGood}
    }
    age := time.Since(r.PersistErrorAt)
    lvl := healthBad
    if age > 5*time.Minute { lvl = healthWarn }  // lỗi cũ chưa tái diễn
    return healthMetric{Key: "persist", Value: "file lock", Level: lvl}
}
```

Frontend `app-production.js`: map key `persist` → label "Lưu trữ" + toast khi level bad: "File bị khóa bởi IDE/editor. Tắt editor, mở lại Web UI."

Risk: MEDIUM. Thêm field struct → thay JSON schema jobs.json (cần backward-compat: omitempty, loader cũ bỏ qua field mới). Thêm metric → thay health strip shape (frontend phải handle key mới).

### P3 (QA đề xuất): health endpoint dry-run file lock probe

QA gợi ý `/api/prodruns/{id}/health` thêm `os.OpenFile(target, WRONLY)` probe. **Khuyến nghị KHÔNG làm** — dry-run probe trên Windows có thể本身 cause lock/flakiness, và metric `persist` (P2) đã cover: nếu persist fail thì metric báo bad, không cần probe chủ động. P2 reactive đủ, P3 proactive rủi ro hơn lợi.

## Thứ tự thực hiện

1. **P0** (Bug 3 — process tiêu tiền): sửa ngay, 1 dòng xóa `return`. Test: target-reached + persist-fail mock → verify killProcess vẫn gọi.
2. **P1** (Bug 2 root — retry): sửa saveLocked. Test: mock os.Rename fail N lần → verify retry + eventual success.
3. **P2** (Bug 1 — UI): thêm PersistError field + health metric + frontend toast. Test: persist fail → view có PersistError + health.bad.
4. P3: bỏ (P2 đủ).

## Tương tác phiên này

QA mode, read-only. Không sửa code engine. Backlog này là deliverable cho dev proceed.

## Ghi chú

- Toàn bộ fix trong `internal/entry/web/` → fork additive-only, `git merge upstream/main` không conflict.
- Windows-specific: Linux/Mac `os.Rename` atomic, ít bị lock behavior này. Nhưng retry loop không hại cross-platform (rename thành công lần đầu → return ngay).
- Bug 3 (process không killed) có thể đã tiêu tiền thực trong run-002 nếu target-chapters đạt ngay lúc IDE đang mở jobs.json. Kiểm log run-002 xem có chương vượt target không.
