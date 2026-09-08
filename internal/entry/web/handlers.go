package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/entry/startup"
	"github.com/voocel/ainovel-cli/internal/host"
)

// ── 静态资源 ──

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := assetFS.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "index missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *server) handleAsset(path, ctype string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := assetFS.ReadFile(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", ctype)
		_, _ = w.Write(b)
	}
}

// ── 只读 ──

func (s *server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.eng.Snapshot())
}

// handleReplay 把持久化运行时队列归一化成与实时 SSE 同构的信封数组，
// 前端 reconnect/刷新时按同一 handle() 重建历史，无需第二套解析逻辑。
// upstream 重构后队列只持久化事件（流式增量/clear 不再落盘），故回放仅含 event 帧；
// 刷新时会话中正在生成的部分流式文本无法恢复，须等下一条实时增量。
func (s *server) handleReplay(w http.ResponseWriter, r *http.Request) {
	var after int64
	if v := r.URL.Query().Get("after"); v != "" {
		after, _ = strconv.ParseInt(v, 10, 64)
	}
	items, err := s.eng.ReplayQueue(after)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	msgs := make([]sseMessage, 0, len(items))
	for _, it := range items {
		ev := host.Event{Time: it.Time, Category: it.Category, Agent: it.Agent, Summary: it.Summary}
		msgs = append(msgs, sseMessage{Type: "event", Seq: it.Seq, Data: mustJSON(ev)})
	}
	writeJSON(w, http.StatusOK, msgs)
}

// handleEvents 是 SSE 端点：订阅 hub，把信封逐条以 text/event-stream 推给浏览器。
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)

	// hello 让前端确认连接就绪后再拉 snapshot/replay。
	writeSSE(w, flusher, sseMessage{Type: "hello"})

	for {
		select {
		case <-r.Context().Done(): // 客户端断开
			return
		case <-s.ctx.Done(): // 服务端关停
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if !writeSSE(w, flusher, msg) {
				return
			}
		}
	}
}

// ── 控制 ──

func (s *server) handleStart(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	// StartPrepared performs destructive setup writes before its synchronous
	// Arbiter model call returns. Do not queue duplicate clicks behind this
	// potentially long request: reject them immediately, while preserving the
	// existing workspace-wide serialization for the first request.
	if !s.workspaceMu.TryLock() {
		writeErr(w, http.StatusConflict, fmt.Errorf("workspace mutation already in progress"))
		return
	}
	defer s.workspaceMu.Unlock()
	var body struct {
		Prompt string `json:"prompt"`
		Force  bool   `json:"force"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// StartPrepared 会无条件 Reset 检查点 + Init 进度，等于清空可恢复会话。开新书前先过这道闸：
	// 磁盘上存在可恢复进度且未显式 force 时拒绝（前端二次确认后带 force 重试，或改走「恢复」）。
	// 必须在 PrepareQuick/PrepareUserRules 任何落盘之前判断。
	if s.blockIfRecoverable(w, body.Force) {
		return
	}
	prompt, err := startup.PrepareQuick(body.Prompt)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// 启动侧确定性生成本书用户规则快照（用原始 prompt 归一化），须在 StartPrepared 前。
	if err := s.eng.PrepareUserRules(prompt); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.eng.StartPrepared(prompt); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dir": s.eng.Dir()})
}

func (s *server) handleSteer(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("text is required"))
		return
	}
	s.eng.Steer(body.Text)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) handleContinue(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.eng.Continue(body.Text); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) handleAbort(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	// Abort() có thể block nếu coordinator đang chờ LLM response.
	// Chạy nền, return ngay — SSE snapshot/done frame sẽ phản ánh trạng thái.
	go func() { s.eng.Abort() }()
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

func (s *server) handleResume(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	label, err := s.eng.Resume()
	if err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": label != "", "label": label})
}

// ── 章节推进门控 + 重开（fork 新增；mirror TUI /review、/next、/reopen）──
//
// Host API 已就绪（SetAdvanceMode/AdvanceOneChapter/Reopen），这里只做薄封装：
// 校验 + 调用 + 错误映射。业务逻辑全在 Host，符合 entry→host 单向依赖。
// 三个接口都走 workspaceMu，与 steer/continue/start 串行，避免并发改 workspace。

// handleAdvanceMode 切换逐章验收模式（auto / review）。只写运行意图，
// 不调用 Arbiter，也不隐式启动已暂停的 Engine——与 TUI /review on|off 等价。
func (s *server) handleAdvanceMode(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	var body struct {
		Mode string `json:"mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	mode := domain.ChapterAdvanceMode(strings.TrimSpace(body.Mode))
	if !mode.Valid() {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("mode must be \"auto\" or \"review\", got %q", body.Mode))
		return
	}
	if err := s.eng.SetAdvanceMode(mode); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mode": string(mode)})
}

// handleAdvanceNext 在逐章验收模式下授权一个精确章节并启动 Engine——与 TUI /next 等价。
// Engine 自身守卫：running/cocreating/exclusive 或非 review 模式时返回错误，映射 409。
func (s *server) handleAdvanceNext(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	if err := s.eng.AdvanceOneChapter(); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleReopen 重开已完结的书为创作态——与 TUI /reopen [续写方向] 等价。
// direction 非空时登记为待处理干预，恢复时先经 Arbiter 裁定注入。
// 与 TUI 一致：Reopen 成功后立即 Resume() 自动续跑，无需用户再点「继续」。
func (s *server) handleReopen(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	var body struct {
		Direction string `json:"direction"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.eng.Reopen(body.Direction); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	// Reopen 只把书重置为创作态，不启动 Engine。与 TUI（commands.go:199 → resumeBook）
	// 一致：立即 Resume() 自动续跑。Resume 自身守卫 running/cocreating/exclusive，
	// 若 Reopen 后因竞态已 running 则返回 409。
	label, err := s.eng.Resume()
	if err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "label": label})
}

// ── 模型 / 推理强度 ──

// webModelRoles 与 TUI 模型面板一致（command_model.go:modelRoleOptions）。
// web 自带一份角色/强度标签，避免跨 adapter（tui）依赖——这些纯属各自 UI 的展示文案。
var webModelRoles = []struct{ Key, Label string }{
	{"default", "默认"},
	{"coordinator", "Coordinator"},
	{"architect", "Architect"},
	{"writer", "Writer"},
	{"editor", "Editor"},
}

var webThinkingLabels = map[string]string{
	"":       "默认(继承)",
	"off":    "关闭",
	"low":    "低",
	"medium": "中",
	"high":   "高",
	"xhigh":  "极高",
	"max":    "最高",
}

type labelKV struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type roleView struct {
	Key             string    `json:"key"`
	Label           string    `json:"label"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	Thinking        string    `json:"thinking"`
	ThinkingOptions []labelKV `json:"thinkingOptions"`
}

func studioDefaultView(cfg bootstrap.Config) map[string]string {
	return map[string]string{
		"provider": cfg.Provider,
		"model":    cfg.ModelName,
	}
}

// handleModels 汇总模型面板所需：可选 provider/model、各角色当前选择与可用推理强度。
func (s *server) handleModels(w http.ResponseWriter, r *http.Request) {
	providers := s.eng.ConfiguredProviders()
	models := make(map[string][]string, len(providers))
	for _, p := range providers {
		models[p] = s.eng.ConfiguredModels(p)
	}
	roles := make([]roleView, 0, len(webModelRoles))
	for _, ro := range webModelRoles {
		provider, model, _ := s.eng.CurrentModelSelection(ro.Key)
		roles = append(roles, roleView{
			Key:             ro.Key,
			Label:           ro.Label,
			Provider:        provider,
			Model:           model,
			Thinking:        s.eng.CurrentThinking(ro.Key),
			ThinkingOptions: s.thinkingOptionsFor(ro.Key),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": providers,
		"models":    models,
		"roles":     roles,
		// studioDefault is the provider/model Profile Studio actually inherits
		// when "Mặc định Studio (kế thừa)" is picked. Studio builds its own
		// ModelSet from the STARTUP config (s.cfg) and is intentionally NOT
		// affected by runtime /api/model switches, so this reports the boot-time
		// default (top-level provider/model) rather than the engine's current
		// selection — otherwise the label would lie after a runtime switch.
		"studioDefault": studioDefaultView(s.cfg),
	})
}

// thinkingOptionsFor 镜像 TUI thinkingOptionsFor：按角色当前模型可用的强度过滤标签表。
func (s *server) thinkingOptionsFor(role string) []labelKV {
	levels := s.eng.AvailableThinking(role)
	out := make([]labelKV, 0, len(levels))
	for _, lv := range levels {
		key := string(lv)
		if label, ok := webThinkingLabels[key]; ok {
			out = append(out, labelKV{Key: key, Label: label})
		}
	}
	if len(out) == 0 {
		out = append(out, labelKV{Key: "", Label: webThinkingLabels[""]})
	}
	return out
}

func (s *server) handleModel(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Role     string `json:"role"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.eng.SwitchModel(body.Role, body.Provider, body.Model); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// SwitchModel 自身 emitEvent，hub 消费后会广播新 snapshot，仪表盘自动刷新。
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) handleThinking(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Role  string `json:"role"`
		Level string `json:"level"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.eng.SetRoleThinking(body.Role, body.Level); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "thinking": s.eng.CurrentThinking(body.Role)})
}

// ── 小工具 ──

// blockIfRecoverable 在存在可恢复会话且未显式 force 时写 409 并返回 true（调用方应直接 return）。
// 任何「开新书」入口（start / cocreate 冷启动）落盘前都要先过它，避免 StartPrepared 静默清空进度。
// 响应带 code=recoverable，让前端无需匹配文案即可识别此冲突并弹二次确认。
func (s *server) blockIfRecoverable(w http.ResponseWriter, force bool) bool {
	code, message, blocked := classifyStartConflict(s.eng.Snapshot(), force)
	if blocked {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": message,
			"code":  code,
		})
		return true
	}
	return false
}

// classifyStartConflict distinguishes disposable startup residue from an
// existing book. Force may clear a failed planning session with zero completed
// chapters, but it must never promise to erase a book that Host intentionally
// protects via refuseNewBookOverExisting.
func classifyStartConflict(snap host.UISnapshot, force bool) (code, message string, blocked bool) {
	if snap.CompletedCount > 0 {
		return "existing_book",
			fmt.Sprintf("Workspace hiện có %d chương hoàn thành. Muốn tạo truyện mới, hãy chọn hoặc tạo workspace khác; dùng Khôi phục để viết tiếp.", snap.CompletedCount), true
	}
	if snap.RecoveryLabel == "" {
		return "", "", false
	}
	if force {
		return "", "", false
	}
	return "recoverable",
		fmt.Sprintf("Có tiến độ khởi tạo có thể khôi phục (%s). Bắt đầu mới sẽ xóa phần khởi tạo chưa có chương này.", snap.RecoveryLabel), true
}

func requirePOST(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return false
	}
	return true
}

// maxBodyBytes 是请求体上限。共创/续写可能携带较长规划历史，1MiB 偏紧，放宽到 8MiB。
const maxBodyBytes = 8 << 20

// decodeJSON 解析请求体；空体视为零值（abort/resume 等无体请求复用 POST 校验）。
func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	err := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes)).Decode(v)
	if err == io.EOF {
		return nil
	}
	return err
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeSSE(w http.ResponseWriter, f http.Flusher, msg sseMessage) bool {
	b, err := json.Marshal(msg)
	if err != nil {
		return true
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return false
	}
	f.Flush()
	return true
}
