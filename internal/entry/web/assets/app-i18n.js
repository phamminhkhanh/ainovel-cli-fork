// ainovel web — bản dịch UI: nhãn model/suy luận + event log tiếng Trung.
// Nạp trước app.js để renderSettings() và handleEvent() dùng.
'use strict';

// ── Model / reasoning labels ──
const UI_LABEL_MAP = {
  '默认': 'Mặc định',
  '默认(继承)': 'Mặc định (kế thừa)',
  '关闭': 'Tắt',
  '低': 'Thấp',
  '中': 'Trung bình',
  '高': 'Cao',
  '极高': 'Rất cao',
  '最高': 'Tối đa',
};

function trLabel(s) {
  return UI_LABEL_MAP[s] || s;
}

// ── Event log translation ──
// Engine phát Event.Summary bằng tiếng Trung (host.go / observer.go). Đây là lớp dịch CHỈ Ở UI
// cho các chuỗi SYSTEM/USER cố định — additive, KHÔNG đụng engine (giữ git pull upstream sạch).
// Chỉ phủ chuỗi cố định + tiền tố ổn định; đuôi động (tên/nội dung) và nhãn dashboard giữ nguyên.
const EVENT_SUMMARY_MAP = {
  '开始创作': 'Bắt đầu sáng tác',
  '进入阶段共创': 'Vào đồng sáng tác theo giai đoạn',
  '进入阶段共创，创作已暂停': 'Vào đồng sáng tác theo giai đoạn — đã tạm dừng',
  '阶段共创完成，已注入后续方向并恢复创作': 'Đồng sáng tác xong — đã chèn hướng đi tiếp và tiếp tục sáng tác',
  '已退出阶段共创，创作保持暂停（可在输入框继续）': 'Đã thoát đồng sáng tác — vẫn tạm dừng (gõ ở ô nhập để tiếp tục)',
  '干预已保存，下次启动时生效': 'Đã lưu can thiệp — áp dụng ở lần khởi động kế tiếp',
  '用户手动暂停当前创作': 'Người dùng tạm dừng sáng tác thủ công',
};
const EVENT_PREFIX_MAP = [
  ['指令重复: ', 'Lệnh trùng lặp: '],
  ['恢复创作: ', 'Tiếp tục sáng tác: '],
  ['一致性告警: ', 'Cảnh báo nhất quán: '],
  ['[继续] ', '[Tiếp tục] '],
  ['[用户干预] ', '[Can thiệp] '],
];
const RETRY_RE = /^重试 \((\d+)\/(\d+)\): /; // observer.go: "重试 (n/m): "
function translateSummary(text) {
  if (!text) return '';
  if (EVENT_SUMMARY_MAP[text]) return EVENT_SUMMARY_MAP[text];
  const m = text.match(RETRY_RE);
  if (m) return text.replace(RETRY_RE, `Thử lại (${m[1]}/${m[2]}): `);
  for (const [zh, vi] of EVENT_PREFIX_MAP) {
    if (text.startsWith(zh)) return vi + text.slice(zh.length);
  }
  return text;
}
