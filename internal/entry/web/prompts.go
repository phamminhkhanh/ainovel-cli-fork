package web

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/voocel/ainovel-cli/assets"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
)

// overridablePrompts 是 assets.Bundle.OverridePrompt 支持整篇替换的核心提示词文件名。
// 与 assets.promptRole 的键保持一致（coordinator / architect x2 / writer / editor）。
var overridablePrompts = []string{
	"coordinator.md",
	"architect-short.md",
	"architect-long.md",
	"writer.md",
	"editor.md",
}

// promptsOverrideDir 返回全局提示词覆盖目录 ~/.ainovel/prompts。
// 与配置同源（DefaultConfigPath 的父目录），无家目录时返回 ""。
func promptsOverrideDir() string {
	p := bootstrap.DefaultConfigPath()
	if p == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p), "prompts")
}

// ensurePromptsDir 创建覆盖目录并写入一次 README（缺失时），
// 让「Mở thư mục prompt」按钮总能落到一个自解释的位置。返回目录路径（失败仍返回路径，尽力而为）。
func ensurePromptsDir() string {
	dir := promptsOverrideDir()
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("tạo thư mục prompt override thất bại", "dir", dir, "err", err)
		return dir
	}
	readme := filepath.Join(dir, "README.txt")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		if err := os.WriteFile(readme, []byte(promptsReadme), 0o644); err != nil {
			slog.Warn("ghi README prompt override thất bại", "err", err)
		}
	}
	return dir
}

// applyPromptOverrides 读取全局覆盖目录中的核心提示词并整篇替换。见 applyPromptOverridesFrom。
func applyPromptOverrides(bundle *assets.Bundle) []string {
	return applyPromptOverridesFrom(promptsOverrideDir(), bundle)
}

// applyPromptOverridesFrom 从指定目录读取存在且非空的核心提示词文件，用 Bundle.OverridePrompt 整篇替换
// （走与 assets.Load 相同的 WithSimulationGuidance 包装，保证与 baseline 等价）。返回已应用的文件名。
// 覆盖发生在 host.New 之前（bundle 之后被引擎消费），故改 prompt 需重启 --web 才生效。
// 错误处理：仅「文件不存在」静默跳过（= 该角色沿用内置 prompt）；权限/是目录/IO 错误都 slog.Warn，
// 避免用户以为已覆盖但实际没生效。dir 抽参数便于测试。
func applyPromptOverridesFrom(dir string, bundle *assets.Bundle) []string {
	if dir == "" {
		return nil
	}
	var applied []string
	for _, f := range overridablePrompts {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("đọc file prompt override lỗi (bỏ qua)", "file", f, "err", err)
			}
			continue
		}
		raw := strings.TrimSpace(string(data))
		if raw == "" {
			continue // 空文件视为未覆盖，避免误把角色 prompt 清空
		}
		if err := bundle.OverridePrompt(f, raw); err != nil {
			slog.Warn("bỏ qua prompt override", "file", f, "err", err)
			continue
		}
		applied = append(applied, f)
	}
	return applied
}

const promptsReadme = `# Thư mục ghi đè prompt (~/.ainovel/prompts)

Bỏ file .md vào đây để THAY HẲN prompt gốc (đang là tiếng Trung) của từng role.
Chỉ các file dưới đây được nhận (đúng tên):

  coordinator.md      — điều phối tổng
  architect-short.md  — architect (truyện ngắn)
  architect-long.md   — architect (truyện dài)
  writer.md           — viết chương
  editor.md           — biên tập / review

Quy tắc:
- File nào KHÔNG có ở đây → role đó dùng prompt gốc của upstream (vẫn nhận cải tiến khi merge).
- File rỗng → coi như không ghi đè (tránh vô tình xoá trắng prompt).
- Ghi đè xong phải KHỞI ĐỘNG LẠI --web thì mới có hiệu lực (prompt nạp một lần lúc dựng engine).
- Role bạn ghi đè là bạn SỞ HỮU: cải tiến prompt từ upstream sẽ không tự áp cho role đó nữa.

Mẹo: muốn tinh chỉnh nhẹ (giọng văn, độ dài, cấm từ) mà vẫn ăn cải tiến upstream thì
dùng ~/.ainovel/rules/*.md (chèn thêm lên trên) thay vì ghi đè cả prompt ở đây.
`
