#!/usr/bin/env bash
# start-web.sh — Khởi động ainovel-cli ở chế độ Web (--web) rồi tự mở trình duyệt vào UI.
#
# Một binary duy nhất vừa chạy engine vừa phục vụ SPA tại http://127.0.0.1:8787.
# Lưu ý: chế độ Web KHÔNG hỗ trợ cấu hình lần đầu. Nếu chưa từng thiết lập, hãy chạy
# TUI một lần để hoàn tất config trước:  go run ./cmd/ainovel-cli
#
# Dùng:
#   bash start-web.sh                  # mặc định 127.0.0.1:8787
#   bash start-web.sh 127.0.0.1:9000   # đổi cổng/địa chỉ
#
# Ctrl+C để dừng server.

set -euo pipefail

ADDR="${1:-127.0.0.1:8787}"
URL="http://${ADDR}"

# Về thư mục gốc dự án để `go run ./cmd/...` luôn resolve đúng dù gọi script từ đâu.
cd "$(dirname "$0")"

if ! command -v go >/dev/null 2>&1; then
  echo "✗ Không tìm thấy 'go'. Cài Go rồi thử lại: https://go.dev/dl/" >&2
  exit 1
fi

# Mở URL bằng trình duyệt mặc định của hệ điều hành (đa nền tảng).
open_url() {
  local url="$1"
  if command -v cmd.exe >/dev/null 2>&1; then            # Windows / Git Bash
    MSYS2_ARG_CONV_EXCL='*' cmd.exe /c start "" "$url" >/dev/null 2>&1 && return
  fi
  if command -v powershell.exe >/dev/null 2>&1; then      # Windows fallback
    powershell.exe -NoProfile -Command "Start-Process '$url'" >/dev/null 2>&1 && return
  fi
  if command -v open >/dev/null 2>&1; then open "$url" && return; fi              # macOS
  if command -v xdg-open >/dev/null 2>&1; then xdg-open "$url" >/dev/null 2>&1 && return; fi  # Linux
  echo "ℹ Không tự mở được trình duyệt — mở thủ công: $url"
}

# Chờ server sẵn sàng (poll /api/snapshot, Host loopback nên qua được guard) rồi mở UI một lần.
wait_then_open() {
  if command -v curl >/dev/null 2>&1; then
    for ((i = 0; i < 60; i++)); do
      if curl -sf -o /dev/null "$URL/api/snapshot" 2>/dev/null; then
        open_url "$URL"
        return
      fi
      sleep 0.5
    done
    echo "⚠ Server chưa phản hồi sau ~30s — mở thủ công: $URL" >&2
  else
    sleep 3
    open_url "$URL"
  fi
}

echo "▶ Khởi động ainovel web tại $URL ..."
echo "ℹ Nếu báo lỗi chưa cấu hình, hãy chạy TUI một lần trước: go run ./cmd/ainovel-cli"
wait_then_open &

# Chạy server ở foreground; mọi lỗi khởi động (vd: chưa cấu hình) sẽ hiện ngay tại đây.
exec go run ./cmd/ainovel-cli --web --addr "$ADDR"
