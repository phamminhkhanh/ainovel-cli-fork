package web

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/store"
)

// server 持有引擎与扇出 hub，注册所有 HTTP 路由。
// ctx 取消（Ctrl+C）即触发 SSE 处理退出，让 http.Server.Shutdown 不被长连接挂住。
type server struct {
	eng   *host.Host
	store *store.Store
	hub   *hub
	ask   *askBridge
	ctx   context.Context
	addr  string // 监听地址，用于 Host 头校验（仅回环绑定时加锁）

	jobMu      sync.Mutex         // 串行化后台任务（import/simulate/importsim）
	jobRunning bool               // 是否有后台任务在跑——guardExclusive 不跟踪它们，故 web 侧自锁
	jobCancel  context.CancelFunc // 取消当前后台任务的 ctx（无任务时为 nil）；imp/sim.Run 在章节边界检查 ctx
}

// Store returns the cached on-disk store for read-only content handlers.
func (s *server) Store() *store.Store { return s.store }

// Snapshot returns the latest Host UI snapshot.
func (s *server) Snapshot() host.UISnapshot { return s.eng.Snapshot() }

func (s *server) mux() http.Handler {
	mux := http.NewServeMux()

	// 静态单页（go:embed）
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/app-i18n.js", s.handleAsset("assets/app-i18n.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/app.js", s.handleAsset("assets/app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/app-dashboard.js", s.handleAsset("assets/app-dashboard.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/app-workspace.js", s.handleAsset("assets/app-workspace.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/app-chapters.js", s.handleAsset("assets/app-chapters.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/app-studio.js", s.handleAsset("assets/app-studio.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/app-input.js", s.handleAsset("assets/app-input.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/app.css", s.handleAsset("assets/app.css", "text/css; charset=utf-8"))

	// 只读
	mux.HandleFunc("/api/snapshot", s.handleSnapshot)
	mux.HandleFunc("/api/replay", s.handleReplay)
	mux.HandleFunc("/api/events", s.handleEvents)

	// 内容读取（workspace tabs）
	mux.HandleFunc("GET /api/chapters/{n}", s.handleChapter)
	mux.HandleFunc("GET /api/chapters/{n}/draft", s.handleChapterDraft)
	mux.HandleFunc("GET /api/outline", s.handleOutline)
	mux.HandleFunc("GET /api/world", s.handleWorld)
	mux.HandleFunc("GET /api/characters", s.handleCharacters)
	mux.HandleFunc("GET /api/reviews", s.handleReviews)
	mux.HandleFunc("GET /api/foreshadow", s.handleForeshadow)

	// 控制
	mux.HandleFunc("/api/start", s.handleStart)
	mux.HandleFunc("/api/steer", s.handleSteer)
	mux.HandleFunc("/api/continue", s.handleContinue)
	mux.HandleFunc("/api/abort", s.handleAbort)
	mux.HandleFunc("/api/resume", s.handleResume)

	// 交互 / 模型 / 推理强度（Phase 2）
	mux.HandleFunc("/api/ask", s.handleAsk)
	mux.HandleFunc("/api/models", s.handleModels)
	mux.HandleFunc("/api/model", s.handleModel)
	mux.HandleFunc("/api/thinking", s.handleThinking)

	// 共创 / 导出 / 导入 / 仿写 / 诊断（Phase 3）
	mux.HandleFunc("/api/cocreate/send", s.handleCoCreateSend)
	mux.HandleFunc("/api/cocreate/pause", s.handleCoCreatePause)
	mux.HandleFunc("/api/cocreate/resume", s.handleCoCreateResume)
	mux.HandleFunc("/api/cocreate/cancel", s.handleCoCreateCancel)
	mux.HandleFunc("/api/cocreate/start", s.handleCoCreateStart)
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/api/import", s.handleImport)
	mux.HandleFunc("/api/simulate", s.handleSimulate)
	mux.HandleFunc("/api/importsim", s.handleImportSim)
	mux.HandleFunc("/api/job/cancel", s.handleJobCancel)
	mux.HandleFunc("/api/reveal", s.handleReveal)
	mux.HandleFunc("/api/diag", s.handleDiag)

	return s.guardHost(mux)
}

// ── Host 头校验：本地单人工具的轻量加固（挡 DNS-rebinding / 跨站 fetch）──
//
// 默认绑定 127.0.0.1，但浏览器里的恶意页面仍可用 DNS-rebinding 把某域名解析到回环、再 fetch 本服务，
// 触发文件读写 / 模型消费。回环绑定时只放行 Host 为回环的请求，挡住此类跨站调用；用户若显式绑定到
// 非回环地址（启动时已警示），说明其有意对外，便不再加锁，以免拦掉其局域网访问。
func (s *server) guardHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host, s.addr) {
			http.Error(w, "forbidden: non-local Host header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hostAllowed(reqHost, bindAddr string) bool {
	if !isLoopbackHostPort(bindAddr) {
		return true // 用户显式绑定到非回环（已警示）：不再以 Host 头加锁
	}
	if reqHost == "" {
		return false
	}
	return isLoopbackHostPort(reqHost)
}

// isLoopbackHostPort 判断 host[:port] 的主机部分是否为回环（localhost / 127.0.0.0/8 / ::1）。
func isLoopbackHostPort(hostport string) bool {
	h := hostport
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		h = host
	}
	h = strings.Trim(strings.TrimSpace(h), "[]")
	if h == "" {
		return false
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// tryStartJob 标记后台任务开始并返回该任务专属的可取消 ctx；已有任务在跑则返回 (nil, false)（调用方应回 409）。
// guardExclusive 只跟踪 running/cocreating，import/simulate 不置 lifecycle=running，故 web 侧自锁串行。
// 返回的 ctx 派生自 s.ctx（服务端关停仍会级联取消），另存 cancel 供 /api/job/cancel 单独中止本任务。
func (s *server) tryStartJob() (context.Context, bool) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	if s.jobRunning {
		return nil, false
	}
	s.jobRunning = true
	jobCtx, cancel := context.WithCancel(s.ctx)
	s.jobCancel = cancel
	return jobCtx, true
}

// endJob 清除后台任务占用标记，并释放/清空 cancel（幂等：cancel 多调无害）。
func (s *server) endJob() {
	s.jobMu.Lock()
	if s.jobCancel != nil {
		s.jobCancel()
		s.jobCancel = nil
	}
	s.jobRunning = false
	s.jobMu.Unlock()
}

// cancelJob 中止当前后台任务的 ctx；无任务在跑则返回 false（调用方应回 409）。
// imp/sim.Run 在章节边界检查 ctx.Err()，故取消会在下一章边界干净停下并 emit「用户取消」。
func (s *server) cancelJob() bool {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	if !s.jobRunning || s.jobCancel == nil {
		return false
	}
	s.jobCancel()
	return true
}
