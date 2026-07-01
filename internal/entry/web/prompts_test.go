package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/assets"
)

// TestApplyPromptOverridesFrom: file non-empty được áp; file rỗng bị bỏ qua; role khác giữ nguyên baseline.
func TestApplyPromptOverridesFrom(t *testing.T) {
	dir := t.TempDir()
	const marker = "PROMPT WRITER TIẾNG VIỆT ĐỘC NHẤT"
	if err := os.WriteFile(filepath.Join(dir, "writer.md"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	// editor.md rỗng → phải bị bỏ qua, không xoá trắng prompt editor.
	if err := os.WriteFile(filepath.Join(dir, "editor.md"), []byte("   \n  "), 0o644); err != nil {
		t.Fatal(err)
	}

	b := assets.Load("default")
	baselineEditor := b.Prompts.Editor
	applied := applyPromptOverridesFrom(dir, &b)

	if len(applied) != 1 || applied[0] != "writer.md" {
		t.Fatalf("chỉ writer.md nên được áp, got %v", applied)
	}
	if !strings.Contains(b.Prompts.Writer, marker) {
		t.Fatal("prompt writer chưa được ghi đè bằng nội dung file")
	}
	if b.Prompts.Editor != baselineEditor {
		t.Fatal("file editor.md rỗng không được đổi prompt editor")
	}
}

// TestApplyPromptOverridesFrom_EmptyDir: thư mục không có file → không áp gì, không panic.
func TestApplyPromptOverridesFrom_EmptyDir(t *testing.T) {
	b := assets.Load("default")
	if applied := applyPromptOverridesFrom(t.TempDir(), &b); len(applied) != 0 {
		t.Fatalf("thư mục rỗng nên không áp gì, got %v", applied)
	}
	if applied := applyPromptOverridesFrom("", &b); applied != nil {
		t.Fatalf("dir rỗng nên trả nil, got %v", applied)
	}
}
