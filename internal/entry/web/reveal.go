package web

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// revealOpen là điểm mở thư mục, tách ra biến để test thay thế (không mở file manager thật).
var revealOpen = openInFileManager

// handleReveal 在系统文件管理器中打开一个已知目录。
// 安全边界：
//   - 仅回环绑定可用。public bind（--unsafe-public-web）下 guardHost 会放行所有请求，
//     故这里额外自锁：非回环绑定直接 403，绝不让 LAN 客户端触发本机进程启动。
//   - 只接受固定 target（novel = 引擎输出目录 / prompts = 全局覆盖目录），不接收客户端路径。
func (s *server) handleReveal(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	if !isLoopbackHostPort(s.addr) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("mở thư mục chỉ khả dụng khi bind loopback (không dùng ở chế độ public)"))
		return
	}
	var body struct {
		Target string `json:"target"` // "novel" | "prompts"
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var dir string
	switch body.Target {
	case "novel":
		dir = s.eng.Dir()
	case "prompts":
		dir = ensurePromptsDir() // 顺带建目录 + README，保证按钮总能打开
	default:
		writeErr(w, http.StatusBadRequest, fmt.Errorf("target không hợp lệ: %q (chỉ nhận novel|prompts)", body.Target))
		return
	}
	if err := revealOpen(dir); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dir": dir})
}

// openInFileManager 用当前 OS 的文件管理器打开目录（Windows explorer / macOS open / Linux xdg-open）。
// 用 Start 不 Wait：explorer 即便成功也返回退出码 1，Wait 会误报错；且不阻塞请求。
// Start 成功后 Release 释放进程句柄，避免长生命周期 server 泄漏子进程资源。
func openInFileManager(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("thư mục trống")
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("không mở được thư mục %q: %w", dir, err)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
