package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/voocel/ainovel-cli/internal/diag"
	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/host/exp"
	"github.com/voocel/ainovel-cli/internal/host/imp"
	"github.com/voocel/ainovel-cli/internal/host/sim"
	"github.com/voocel/ainovel-cli/internal/store"
)

// ── 共创（cocreate）──
//
// 设计取舍：服务端无状态，对话历史由前端持有，每轮把完整 history 传上来跑一次流。
// 解析（[REPLY]/[DRAFT]/[READY]/[SUGGESTIONS]）已在 Host.CoCreateStream 内完成，
// 返回结构化 CoCreateReply，前端只需累积——避免在 JS 里重写协议解析（守 DRY/单一真相源）。
// 唯一的服务端状态是引擎的 cocreating 占用标记（PauseForCoCreate/Cancel/Resume，仅阶段共创用）。

// coCreateReplyView 把 host.CoCreateReply（无 json tag → PascalCase）转成小写键，
// 与 ask 帧一致，让共创前端统一读小写。
type coCreateReplyView struct {
	Message     string   `json:"message"`
	Prompt      string   `json:"prompt"`
	Ready       bool     `json:"ready"`
	Suggestions []string `json:"suggestions"`
	Raw         string   `json:"raw"`
}

// coCreateProgress 是流式过程帧：kind=thinking|reply（见 host.CoCreateProgress*）。
type coCreateProgress struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// handleCoCreateSend 同步跑一轮共创流：onProgress 经 SSE 实时下推，最终回复作为 HTTP 响应返回。
// stage=true 走阶段共创（带故事状态摘要），false 走冷启动共创（从零澄清需求）。两者签名一致。
// ctx 用 r.Context()：浏览器中止 fetch 即取消本轮 LLM 调用。
func (s *server) handleCoCreateSend(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Stage   bool                   `json:"stage"`
		History []host.CoCreateMessage `json:"history"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if len(body.History) == 0 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("history is required"))
		return
	}
	stream := s.eng.CoCreateStream
	if body.Stage {
		stream = s.eng.StageCoCreateStream
	}
	reply, err := stream(r.Context(), body.History, func(kind, text string) {
		s.hub.broadcast(sseMessage{Type: "cocreate", Data: mustJSON(coCreateProgress{Kind: kind, Text: text})})
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, coCreateReplyView{
		Message:     reply.Message,
		Prompt:      reply.Prompt,
		Ready:       reply.Ready,
		Suggestions: reply.Suggestions,
		Raw:         reply.Raw,
	})
}

// handleCoCreatePause 进入阶段共创（运行中暂停 coordinator + 置占用标记）。
// 返回 false（全书完成 / 已在共创中）→ 409。
func (s *server) handleCoCreatePause(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	if !s.eng.PauseForCoCreate() {
		writeErr(w, http.StatusConflict, fmt.Errorf("无法进入阶段共创：全书已完成或已在共创中"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleCoCreateResume 结束阶段共创：把后续方向 brief 作为干预注入并恢复创作。
func (s *server) handleCoCreateResume(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Draft string `json:"draft"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.eng.ResumeFromCoCreate(body.Draft); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleCoCreateCancel 放弃阶段共创：清占用标记、保持暂停（冷启动共创无标记，为空操作）。
func (s *server) handleCoCreateCancel(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	s.eng.CancelCoCreate()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleCoCreateStart 冷启动共创产出的创作指令直接开写：与 TUI BuildPlan→startRuntime 一致
// （PrepareUserRules(raw) + StartPrepared(BuildStartPrompt(raw))）。引擎在跑时 StartPrepared 报错 → 409。
func (s *server) handleCoCreateStart(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Prompt string `json:"prompt"`
		Force  bool   `json:"force"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	draft := strings.TrimSpace(body.Prompt)
	if draft == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("prompt is required"))
		return
	}
	// 与 /api/start 同闸：冷启动共创开新书同样会触发 StartPrepared 清空可恢复进度。
	if s.blockIfRecoverable(w, body.Force) {
		return
	}
	if err := s.eng.PrepareUserRules(draft); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.eng.StartPrepared(host.BuildStartPrompt(draft)); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dir": s.eng.Dir()})
}

// ── 导出 ──

// handleExport 导出已完成章节为 TXT/EPUB。只读操作，写作中途也可随时导出现阶段成品。
func (s *server) handleExport(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Path      string `json:"path"`
		From      int    `json:"from"`
		To        int    `json:"to"`
		Format    string `json:"format"`
		Overwrite bool   `json:"overwrite"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.eng.Export(r.Context(), exp.Options{
		Format:    exp.Format(strings.ToLower(strings.TrimSpace(body.Format))),
		OutPath:   strings.TrimSpace(body.Path),
		From:      body.From,
		To:        body.To,
		Overwrite: body.Overwrite,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// *exp.Result 无 json tag → PascalCase（Path/Chapters/Bytes/Skipped），与 snapshot 一致。
	writeJSON(w, http.StatusOK, res)
}

// ── 导入 / 仿写（长任务，进度经 SSE "job" 帧下推）──

// jobEvent 是 import/simulate/importsim 的统一进度帧。Done=true 标记任务结束。
type jobEvent struct {
	Name    string `json:"name"` // import | simulate | importsim
	Stage   string `json:"stage"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Done    bool   `json:"done"`
}

// handleImport 反推外部小说续写。任务在 s.ctx（后台）上跑——HTTP 立即返回，进度走 SSE。
// guardExclusive 只挡运行中/共创中，不跟踪后台任务彼此并发；故先用 tryStartJob 串行化 job-vs-job
// （两个并发 import 会同时写同一 store 的 Progress/检查点/章节文件），再交给 guardExclusive。
func (s *server) handleImport(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Path string `json:"path"`
		From int    `json:"from"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Path) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("path is required"))
		return
	}
	jobCtx, ok := s.tryStartJob()
	if !ok {
		writeErr(w, http.StatusConflict, fmt.Errorf("已有后台任务在运行，请等待其完成"))
		return
	}
	ch, err := s.eng.ImportFrom(jobCtx, imp.Options{SourcePath: strings.TrimSpace(body.Path), ResumeFrom: body.From})
	if err != nil {
		s.endJob()
		writeErr(w, http.StatusConflict, err)
		return
	}
	go s.streamImportJob(ch)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleSimulate 读取 ./simulate 生成或增量更新仿写画像。
func (s *server) handleSimulate(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	jobCtx, ok := s.tryStartJob()
	if !ok {
		writeErr(w, http.StatusConflict, fmt.Errorf("已有后台任务在运行，请等待其完成"))
		return
	}
	ch, err := s.eng.Simulate(jobCtx)
	if err != nil {
		s.endJob()
		writeErr(w, http.StatusConflict, err)
		return
	}
	go s.streamSimJob("simulate", ch)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleImportSim 导入已有仿写画像并按语料指纹合并。
func (s *server) handleImportSim(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Path) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("path is required"))
		return
	}
	jobCtx, ok := s.tryStartJob()
	if !ok {
		writeErr(w, http.StatusConflict, fmt.Errorf("已有后台任务在运行，请等待其完成"))
		return
	}
	ch, err := s.eng.ImportSimulationProfile(jobCtx, strings.TrimSpace(body.Path))
	if err != nil {
		s.endJob()
		writeErr(w, http.StatusConflict, err)
		return
	}
	go s.streamSimJob("importsim", ch)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// streamImportJob / streamSimJob 消费各自返回的事件通道（与主 Events 通道无关，独立 goroutine 安全），
// 逐条经 hub 广播；通道关闭即收尾。imp/sim.Event 字段同构，分两函数仅因类型不同。
func (s *server) streamImportJob(ch <-chan imp.Event) {
	var lastErr string
	for ev := range ch {
		je := jobEvent{Name: "import", Stage: string(ev.Stage), Current: ev.Current, Total: ev.Total, Message: ev.Message}
		if ev.Err != nil {
			je.Error = ev.Err.Error()
			lastErr = je.Error // 取消/失败会 emit StageError 后 return，故终结帧即最后一个错误
		}
		s.hub.broadcast(sseMessage{Type: "job", Data: mustJSON(je)})
	}
	s.finishJob("import", lastErr)
}

func (s *server) streamSimJob(name string, ch <-chan sim.Event) {
	var lastErr string
	for ev := range ch {
		je := jobEvent{Name: name, Stage: string(ev.Stage), Current: ev.Current, Total: ev.Total, Message: ev.Message}
		if ev.Err != nil {
			je.Error = ev.Err.Error()
			lastErr = je.Error
		}
		s.hub.broadcast(sseMessage{Type: "job", Data: mustJSON(je)})
	}
	s.finishJob(name, lastErr)
}

// finishJob 收尾：释放后台任务占用，推 done 帧 + 一份新快照（导入会改写 Progress，仪表盘随之刷新）。
// errMsg 非空表示任务被取消或失败（终结帧带 error），前端据此渲染「已停止」而非「已完成」。
func (s *server) finishJob(name, errMsg string) {
	s.endJob()
	done := jobEvent{Name: name, Done: true}
	if errMsg != "" {
		done.Error = errMsg
	}
	s.hub.broadcast(sseMessage{Type: "job", Data: mustJSON(done)})
	s.hub.broadcast(sseMessage{Type: "snapshot", Data: mustJSON(s.eng.Snapshot())})
}

// handleJobCancel 中止当前后台任务（import/simulate/importsim）。各 runner 在安全检查点检查 ctx
// （import 按章节、simulate 按素材源、importsim 在导入前），故取消会在下一个检查点干净停下并
// emit「用户取消」，已落盘数据不受影响。无任务在跑 → 409。
func (s *server) handleJobCancel(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	if !s.cancelJob() {
		writeErr(w, http.StatusConflict, fmt.Errorf("没有正在运行的后台任务"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ── 诊断 ──

// handleDiag 创作诊断 + 运行时检测，渲染脱敏 Markdown 返回并落盘一份。镜像 TUI loadReport。
func (s *server) handleDiag(w http.ResponseWriter, r *http.Request) {
	st := store.NewStore(s.eng.Dir())
	rep, rc := diag.Diagnose(st)
	path, _ := diag.WriteExport(st, rep, rc) // 落盘失败不影响屏上报告
	writeJSON(w, http.StatusOK, map[string]any{
		"path":     path,
		"markdown": string(diag.RenderExport(rep, rc)),
	})
}
