package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/voocel/ainovel-cli/assets"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/diag"
	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/logger"
	"github.com/voocel/ainovel-cli/internal/store"
)

// Options 是 web 入口的可选配置。
type Options struct {
	Addr        string // listen address, default 127.0.0.1:8787
	AllowPublic bool   // allow non-loopback bind; unauthenticated, CLI opt-in only
}

const defaultAddr = "127.0.0.1:8787"

func checkPublicBind(addr string, allowPublic bool) error {
	if !isLoopbackHostPort(addr) && !allowPublic {
		return fmt.Errorf("refusing public web bind %q: web UI has no auth; use --unsafe-public-web to expose it", addr)
	}
	return nil
}

// Run 以本地 Web 模式运行会话内核：把 Host 的事件/流式/完成三通道经 SSE 扇出到浏览器，
// 浏览器经 fetch 反向驱动 start/steer/continue/abort/resume。与 tui/headless 并列为第三个
// 入口适配器，不改动 host/tools/prompt，故对 upstream 近乎零冲突。
func Run(cfg bootstrap.Config, bundle assets.Bundle, opts Options) error {
	addr := opts.Addr
	if addr == "" {
		addr = defaultAddr
	}

	if err := checkPublicBind(addr, opts.AllowPublic); err != nil {
		return err
	}

	// 提示词覆盖：建目录 + 读 ~/.ainovel/prompts/*.md 整篇替换核心 prompt（在 host.New 消费 bundle 前）。
	// 全 additive：只调 assets.Bundle.OverridePrompt 这个 upstream 已导出的 seam，不改 assets/prompts。
	ensurePromptsDir()
	if applied := applyPromptOverrides(&bundle); len(applied) > 0 {
		slog.Info("đã áp prompt override", "files", applied)
	}

	eng, err := host.New(cfg, bundle)
	if err != nil {
		return err
	}
	cleanup := logger.SetupFile(eng.Dir(), "web.log", false)
	defer cleanup()
	defer eng.Close()
	// 运行结束 / 出错返回时落一份脱敏诊断，方便贴 issue。
	defer func() { _, _ = diag.Export(store.NewStore(eng.Dir())) }()

	// Ctrl+C 取消该 ctx：既退出主循环，也让 SSE 长连接处理及时返回，避免 Shutdown 被挂住。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	h := newHub()
	go h.run(eng) // Host 三通道的唯一消费者

	// ask_user 桥：引擎工具线程阻塞等待，浏览器 POST /api/ask 解阻塞。须在任何运行开始前注入。
	ask := newAskBridge(h)
	eng.AskUser().SetHandler(ask.handle)

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	jobsDir := filepath.Join(filepath.Dir(eng.Dir()), "jobs")
	prodRunMgr, err := newProdRunManager(jobsDir, binPath, repoRoot, eng.Dir(), cfg)
	if err != nil {
		return fmt.Errorf("init production cockpit: %w", err)
	}

	srv := &server{
		eng: eng, store: store.NewStore(eng.Dir()), hub: h, ask: ask, ctx: ctx, addr: addr,
		cfg: cfg, repoRoot: repoRoot, prodRunManager: prodRunMgr,
	}
	httpSrv := &http.Server{Addr: addr, Handler: srv.mux()}

	errc := make(chan error, 1)
	go func() { errc <- httpSrv.ListenAndServe() }()

	fmt.Fprintf(os.Stderr, "ainovel web 已启动 → http://%s  (输出目录: %s)\n", addr, eng.Dir())
	fmt.Fprintln(os.Stderr, "按 Ctrl+C 退出。")
	// 本工具无鉴权、可触发文件读写与模型消费。绑定到非回环即对网络暴露，显式警示用户自担风险。
	if !isLoopbackHostPort(addr) {
		fmt.Fprintf(os.Stderr, "⚠ 警告：%s 非回环地址，Web 界面无鉴权将向网络暴露文件读写与模型调用，请仅在可信网络使用。\n", addr)
	}

	select {
	case <-ctx.Done():
	case err := <-errc:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	return nil
}
