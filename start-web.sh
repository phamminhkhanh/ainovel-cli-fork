#!/usr/bin/env bash
# start-web.sh — Chọn truyện rồi khởi động ainovel-cli ở chế độ Web (--web).
#
# Mỗi truyện gắn với một thư mục chạy; dữ liệu nằm ở  <thư-mục-chạy>/output/novel .
#   - Truyện "dự án gốc": chạy từ gốc repo → dùng lại  ./output/novel  đã có sẵn.
#   - Truyện mới: nằm dưới AINOVEL_HOME (mặc định ./workspace), mỗi cái một thư mục con.
# Một binary vừa chạy engine vừa phục vụ SPA tại http://127.0.0.1:8787 rồi tự mở trình duyệt.
#
# ⚠ Chế độ Web KHÔNG hỗ trợ cấu hình lần đầu. Nếu chưa từng thiết lập config
#    (~/.ainovel/config.json hoặc ./.ainovel/config.json), hãy chạy TUI một lần trước:
#      go run ./cmd/ainovel-cli
#
# Dùng:
#   bash start-web.sh                    # hiện menu chọn / tạo truyện
#   bash start-web.sh my-fantasy         # mở (hoặc tạo) truyện "my-fantasy" trong workspace/
#   ADDR=127.0.0.1:9000 bash start-web.sh my-fantasy   # đổi cổng/địa chỉ
#   AINOVEL_HOME=~/truyen bash start-web.sh            # đổi thư mục chứa truyện mới
#
# Ctrl+C để dừng server.

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
ADDR="${ADDR:-127.0.0.1:8787}"
URL="http://${ADDR}"
BASE="${AINOVEL_HOME:-$PROJECT_DIR/workspace}"

if ! command -v go >/dev/null 2>&1; then
  echo "✗ Không tìm thấy 'go'. Cài Go rồi thử lại: https://go.dev/dl/" >&2
  exit 1
fi

mkdir -p "$BASE"
# Normalize về tuyệt đối: nếu AINOVEL_HOME là đường dẫn tương đối, nó được giải theo THƯ MỤC HIỆN
# TẠI lúc gọi script (không phải thư mục dự án). Đưa về tuyệt đối để mọi so sánh path sau nhất quán.
BASE="$(cd "$BASE" && pwd)"

# sanitize giữ lại chữ/số/._- cho tên thư mục truyện an toàn (chặn path traversal, khoảng trắng).
# LƯU Ý: tr vẫn giữ dấu chấm nên '.' / '..' lọt qua đây → phải validate lại sau khi ghép path.
sanitize() { printf '%s' "$1" | tr -cd '[:alnum:]._-'; }

# build_candidates dựng 2 mảng song song: LABELS (nhãn hiển thị) và DIRS (thư mục chạy tương ứng).
# Truyện "dự án gốc" (./output/novel) luôn đứng đầu nếu tồn tại, để không bỏ sót truyện cũ.
LABELS=()
DIRS=()
build_candidates() {
  LABELS=()
  DIRS=()
  if [ -d "$PROJECT_DIR/output/novel" ]; then
    LABELS+=("(dự án gốc) output/novel")
    DIRS+=("$PROJECT_DIR")
  fi
  local d name
  for d in "$BASE"/*/; do
    [ -d "$d" ] || continue
    name="$(basename "$d")"
    LABELS+=("$name")
    DIRS+=("$BASE/$name")
  done
}

# choose_novel_dir: in ra stdout thư mục chạy đã chọn. Dùng $1 nếu có (tên → workspace/<tên>),
# ngược lại hiện menu. Mọi text hướng dẫn in ra stderr để không lẫn vào giá trị trả về.
choose_novel_dir() {
  local arg="${1:-}"
  build_candidates

  if [ -n "$arg" ]; then
    printf '%s\n' "$BASE/$(sanitize "$arg")"
    return
  fi

  echo "── Chọn truyện ──" >&2
  if [ "${#LABELS[@]}" -eq 0 ]; then
    echo "  (chưa có truyện nào)" >&2
  else
    local i=1 lbl
    for lbl in "${LABELS[@]}"; do
      echo "  $i) $lbl" >&2
      i=$((i + 1))
    done
  fi
  echo "  n) + Tạo truyện mới (trong $BASE)" >&2
  printf "Nhập số, 'n' để tạo mới, hoặc gõ thẳng tên truyện: " >&2

  local ans
  read -r ans
  if [[ "$ans" =~ ^[0-9]+$ ]] && [ "$ans" -ge 1 ] && [ "$ans" -le "${#DIRS[@]}" ]; then
    printf '%s\n' "${DIRS[$((ans - 1))]}"
  elif [ "$ans" = "n" ] || [ "$ans" = "N" ]; then
    local newname
    printf "Tên truyện mới: " >&2
    read -r newname
    printf '%s\n' "$BASE/$(sanitize "$newname")"
  else
    printf '%s\n' "$BASE/$(sanitize "$ans")"
  fi
}

RUN_DIR="$(choose_novel_dir "${1:-}")"
# Chốt an toàn: RUN_DIR chỉ được phép là truyện gốc ($PROJECT_DIR) hoặc con TRỰC TIẾP của $BASE.
# Chặn tên rỗng / '.' / '..' / chứa '/' / bắt đầu bằng dấu chấm → tránh thoát workspace hoặc chạy
# engine ngay tại workspace root (cả hai đều làm lẫn dữ liệu truyện vào chỗ không mong muốn).
case "$RUN_DIR" in
  "$PROJECT_DIR") : ;; # truyện dự án gốc (./output/novel)
  "$BASE"/*)
    rel="${RUN_DIR#"$BASE"/}"
    case "$rel" in
      "" | . | .. | */* | .*)
        echo "✗ Tên truyện không hợp lệ (chỉ dùng chữ/số/._-, không bắt đầu bằng dấu chấm, không chứa '/')." >&2
        exit 1
        ;;
    esac
    ;;
  *)
    echo "✗ Thư mục truyện không hợp lệ: $RUN_DIR" >&2
    exit 1
    ;;
esac
mkdir -p "$RUN_DIR"

# Cấu hình: dùng ./.ainovel/config.json ở gốc dự án nếu có (giữ khoá đã thiết lập ở đó dù chạy
# engine từ thư mục truyện khác). Cấu hình toàn cục ~/.ainovel/config.json luôn được nạp nên
# khoá API vẫn hoạt động dù cwd nào.
CONFIG_ARGS=()
if [ -f "$PROJECT_DIR/.ainovel/config.json" ]; then
  CONFIG_ARGS=(--config "$PROJECT_DIR/.ainovel/config.json")
fi

# Biên dịch binary (asset nhúng bằng go:embed → phải build lại mới ăn thay đổi UI; build có cache
# nên các lần sau rất nhanh). Chạy từ thư mục truyện nên cần binary tuyệt đối, không dùng `go run .`.
BIN="$PROJECT_DIR/ainovel-cli"
case "$(uname -s 2>/dev/null)" in
  MINGW* | MSYS* | CYGWIN*) BIN="$BIN.exe" ;;
esac

echo "▶ Biên dịch ainovel-cli ..."
( cd "$PROJECT_DIR" && go build -o "$BIN" ./cmd/ainovel-cli )

# Mở URL bằng trình duyệt mặc định của hệ điều hành (đa nền tảng).
open_url() {
  local url="$1"
  if command -v cmd.exe >/dev/null 2>&1; then            # Windows / Git Bash
    MSYS2_ARG_CONV_EXCL='*' cmd.exe /c start "" "$url" >/dev/null 2>&1 && return
  fi
  if command -v powershell.exe >/dev/null 2>&1; then      # Windows fallback
    powershell.exe -NoProfile -Command "Start-Process '$url'" >/dev/null 2>&1 && return
  fi
  if command -v open >/dev/null 2>&1; then open "$url" && return; fi                          # macOS
  if command -v xdg-open >/dev/null 2>&1; then xdg-open "$url" >/dev/null 2>&1 && return; fi   # Linux
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

echo "▶ Thư mục chạy: $RUN_DIR"
echo "▶ Dữ liệu     : $RUN_DIR/output/novel"
echo "▶ Web tại     : $URL"
echo "ℹ Nếu báo lỗi chưa cấu hình, hãy chạy TUI một lần trước: go run ./cmd/ainovel-cli"
wait_then_open &

# Chạy engine ở foreground TỪ thư mục truyện: OutputDir mặc định là ./output/novel (theo cwd),
# nên mỗi truyện ghi vào thư mục riêng của nó. Mọi lỗi khởi động hiện ngay tại đây.
cd "$RUN_DIR"
exec "$BIN" --web --addr "$ADDR" "${CONFIG_ARGS[@]}"
