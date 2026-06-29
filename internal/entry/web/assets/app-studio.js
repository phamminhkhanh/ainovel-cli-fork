// ainovel web · Phase 3 — studio: command palette + đồng sáng tác + export/import/simulate/diag.
// Tách khỏi app.js để giữ mỗi file dưới ngưỡng ~600 dòng. Là classic script nạp SAU app.js nên
// dùng chung các binding toàn cục: $, post, toast, openSettings, currentState, lastSnapshot.
// Hook SSE: app.js gọi onCoCreateProgress(data) cho frame "cocreate", onJobEvent(j) cho frame "job".
'use strict';

// STAGE_OPENER là câu mở màn cộng tác-giai đoạn GỬI CHO LLM (engine tiếng Trung) — bản sao
// của host.stageCoCreateOpener, phải giữ nguyên tiếng Trung để khớp 1:1 với TUI. UI hiển thị
// một dòng hệ thống tiếng Việt thay cho câu này (không giả làm lời người dùng).
const STAGE_OPENER = '我先暂停一下，想和你一起规划接下来的走向。';

// ── Command palette ──
function isTyping(el) {
  const t = (el && el.tagName) || '';
  return t === 'TEXTAREA' || t === 'INPUT' || (el && el.isContentEditable);
}
function allOverlaysHidden() {
  return ['#cmdOverlay', '#ccOverlay', '#expOverlay', '#impOverlay', '#simimpOverlay', '#diagOverlay', '#askOverlay', '#setOverlay']
    .every((id) => { const el = $(id); return !el || el.hidden; });
}
function openCmd() { $('#cmdFilter').value = ''; renderCmdList(); $('#cmdOverlay').hidden = false; $('#cmdFilter').focus(); }
function closeCmd() { $('#cmdOverlay').hidden = true; }
function filteredCmds() {
  const q = $('#cmdFilter').value.trim().toLowerCase();
  if (!q) return COMMANDS;
  return COMMANDS.filter((c) => (c.key + ' ' + c.label + ' ' + c.desc).toLowerCase().includes(q));
}
function renderCmdList() {
  const ul = $('#cmdList');
  ul.innerHTML = '';
  const list = filteredCmds();
  if (!list.length) { ul.innerHTML = '<li class="muted">Không có lệnh khớp</li>'; return; }
  list.forEach((c) => {
    const li = document.createElement('li');
    li.className = 'cmd-item';
    const k = document.createElement('span'); k.className = 'cmd-key'; k.textContent = '/' + c.key;
    const l = document.createElement('span'); l.className = 'cmd-label'; l.textContent = c.label;
    const d = document.createElement('span'); d.className = 'cmd-desc'; d.textContent = c.desc;
    li.append(k, l, d);
    li.addEventListener('click', () => c.run());
    ul.appendChild(li);
  });
}
function runFirstCmd() { const list = filteredCmds(); if (list.length) list[0].run(); }

// ── Đồng sáng tác (cocreate) ──
// Backend không giữ trạng thái hội thoại; frontend giữ lịch sử và mỗi lượt gửi nguyên history lên.
// Mirror startup.CoCreateSession: assistant lưu raw (có [DRAFT]) để lượt sau model thấy nháp cũ;
// draft chỉ ghi đè khi reply.prompt khác rỗng; suggestions thay mới mỗi lượt.
let ccHistory = [];
let ccStage = false;     // true=cộng tác-giai đoạn (đang/đã viết); false=khởi tạo (chưa viết)
let ccDraft = '';
let ccReady = false;
let ccBusy = false;      // đang chờ một lượt stream
let ccLiveBubble = null; // bong bóng "đang gõ" trong lúc reply stream

function ccBubble(role, text) {
  const div = document.createElement('div');
  div.className = 'cc-msg ' + role; // user | assistant | system | "assistant live"
  div.textContent = text;
  const conv = $('#ccConv');
  conv.appendChild(div);
  conv.scrollTop = conv.scrollHeight;
  return div;
}
function setCcBusy(on) {
  ccBusy = on;
  $('#ccSend').disabled = on;
  $('#ccInput').disabled = on;
  updateCcFinish();
}
function updateCcFinish() { $('#ccFinish').disabled = ccBusy || !ccDraft.trim(); }

function renderSugs(sugs) {
  const ul = $('#ccSugs');
  ul.innerHTML = '';
  (sugs || []).forEach((s) => {
    const li = document.createElement('li');
    li.textContent = s;
    li.addEventListener('click', () => { $('#ccInput').value = s; $('#ccInput').focus(); });
    ul.appendChild(li);
  });
}

async function openCoCreate() {
  closeCmd();
  // Chế độ: đã có tiến độ hoặc engine không rảnh → giai đoạn; idle & chưa viết gì → khởi tạo.
  const started = !!(lastSnapshot && ((lastSnapshot.CompletedCount || 0) > 0 || (lastSnapshot.CurrentChapter || 0) > 0)) || (currentState !== 'idle');
  ccStage = started;
  ccHistory = []; ccDraft = ''; ccReady = false; ccLiveBubble = null;
  $('#ccConv').innerHTML = '';
  $('#ccThinking').textContent = ''; $('#ccThinkingWrap').hidden = true;
  $('#ccDraftWrap').hidden = true; $('#ccDraft').textContent = '';
  $('#ccSugs').innerHTML = '';
  $('#ccInput').value = '';
  setCcBusy(false);

  if (ccStage) {
    // Vào cộng tác-giai đoạn: đặt cờ + tạm dừng coordinator trước (409 → không mở).
    const ok = await post('/api/cocreate/pause', {});
    if (!ok) return;
    $('#ccTitle').textContent = 'Đồng sáng tác · định hướng tiếp theo';
    $('#ccFinish').textContent = 'Áp dụng & tiếp tục';
    ccBubble('system', 'Đã tạm dừng — AI sẽ dựa trên tiến độ hiện tại cùng bạn lên kế hoạch hướng đi tiếp theo.');
    ccHistory.push({ role: 'user', content: STAGE_OPENER }); // kickoff gửi cho LLM, không hiện như lời người dùng
    $('#ccOverlay').hidden = false;
    ccSend(''); // tự chạy lượt đầu
  } else {
    $('#ccTitle').textContent = 'Đồng sáng tác · khởi tạo';
    $('#ccFinish').textContent = 'Bắt đầu viết';
    ccBubble('system', 'Mô tả ý tưởng cốt lõi của bạn. AI sẽ hỏi lại để cùng chốt chỉ thị sáng tác.');
    $('#ccOverlay').hidden = false;
    $('#ccInput').focus();
  }
  updateCcFinish();
}

async function ccSend(text) {
  if (ccBusy) return;
  text = (text != null ? text : $('#ccInput').value).trim();
  // Stage kickoff: history đã có opener (1 phần tử user) → cho phép gửi với text rỗng.
  const isStageKickoff = ccStage && ccHistory.length === 1 && ccHistory[0].role === 'user';
  if (!text && !isStageKickoff) { toast('Nhập nội dung gửi AI', 'error'); return; }
  if (text) {
    ccHistory.push({ role: 'user', content: text });
    ccBubble('user', text);
    $('#ccInput').value = '';
  }
  $('#ccSugs').innerHTML = '';
  $('#ccThinking').textContent = ''; $('#ccThinkingWrap').hidden = true;
  ccLiveBubble = ccBubble('assistant live', '…');
  setCcBusy(true);

  const reply = await post('/api/cocreate/send', { stage: ccStage, history: ccHistory });
  setCcBusy(false);
  if (!reply) { // lỗi (toast đã hiện) → gỡ bong bóng sống, giữ history để thử lại
    if (ccLiveBubble) { ccLiveBubble.remove(); ccLiveBubble = null; }
    return;
  }
  // Áp reply (mirror CoCreateSession.ApplyReply): history giữ raw; draft chỉ ghi đè khi prompt khác rỗng.
  const raw = (reply.raw || '').trim();
  const msg = (reply.message || '').trim();
  ccHistory.push({ role: 'assistant', content: raw || msg });
  if ((reply.prompt || '').trim()) ccDraft = reply.prompt.trim();
  ccReady = !!reply.ready;
  if (ccLiveBubble) {
    ccLiveBubble.className = 'cc-msg assistant';
    ccLiveBubble.textContent = msg || '(không có nội dung)';
    ccLiveBubble = null;
  }
  if (ccDraft) { $('#ccDraftWrap').hidden = false; $('#ccDraft').textContent = ccDraft; }
  renderSugs(reply.suggestions);
  updateCcFinish();
}

// onCoCreateProgress nhận frame stream (text là TÍCH LŨY, không phải delta → ghi đè).
function onCoCreateProgress(d) {
  if (!d || $('#ccOverlay').hidden) return;
  if (d.kind === 'thinking') {
    $('#ccThinking').textContent = d.text || '';
    $('#ccThinkingWrap').hidden = !((d.text || '').trim());
  } else if (d.kind === 'reply') {
    if (ccLiveBubble) { ccLiveBubble.textContent = d.text || '…'; $('#ccConv').scrollTop = $('#ccConv').scrollHeight; }
  }
}

async function ccFinish() {
  const draft = ccDraft.trim();
  if (!draft) { toast('Chưa có chỉ thị sáng tác', 'error'); return; }
  if (ccStage) {
    const ok = await post('/api/cocreate/resume', { draft });
    if (ok) { toast('Đã áp dụng định hướng, tiếp tục sáng tác', 'ok'); closeCc(true); }
  } else {
    const ok = await startNovel(draft, false);
    if (ok) { toast('Bắt đầu sáng tác từ chỉ thị đồng sáng tác', 'ok'); closeCc(true); }
  }
}

// closeCc: nếu thoát giữa cộng tác-giai đoạn mà chưa áp dụng → gỡ cờ (giữ trạng thái tạm dừng).
async function closeCc(finished) {
  $('#ccOverlay').hidden = true;
  const wasStage = ccStage;
  ccStage = false; ccHistory = []; ccDraft = ''; ccReady = false; ccLiveBubble = null;
  if (wasStage && !finished) await post('/api/cocreate/cancel', {});
}

// ── Xuất bản (export) ──
function openExport() {
  closeCmd();
  $('#expResult').hidden = true; $('#expResult').textContent = '';
  $('#expOverlay').hidden = false;
}
async function submitExport() {
  const res = await post('/api/export', {
    format: $('#expFormat').value,
    path: $('#expPath').value.trim(),
    from: parseInt($('#expFrom').value, 10) || 0,
    to: parseInt($('#expTo').value, 10) || 0,
    overwrite: $('#expOverwrite').checked,
  });
  if (!res) return; // *exp.Result → key PascalCase (Path/Chapters/Bytes/Skipped)
  const skipped = (res.Skipped && res.Skipped.length) ? ` · bỏ qua ${res.Skipped.length} chương chưa xong` : '';
  const el = $('#expResult');
  el.hidden = false;
  el.textContent = `✓ ${res.Chapters || 0} chương → ${res.Path || '?'} (${(res.Bytes || 0).toLocaleString()} bytes)${skipped}`;
  toast('Xuất bản xong', 'ok');
}

// ── Nhập / Mô phỏng (chạy nền, tiến trình qua frame "job") ──
function openImport() { closeCmd(); $('#impOverlay').hidden = false; }
async function submitImport() {
  const path = $('#impPath').value.trim();
  if (!path) { toast('Nhập đường dẫn nguồn', 'error'); return; }
  const res = await post('/api/import', { path, from: parseInt($('#impFrom').value, 10) || 0 });
  if (res) { toast('Đã bắt đầu nhập — xem tiến trình ở thanh dưới', 'ok'); $('#impOverlay').hidden = true; }
}
async function runSimulate() {
  closeCmd();
  const res = await post('/api/simulate', {});
  if (res) toast('Đã bắt đầu tạo hồ sơ mô phỏng — xem tiến trình ở thanh dưới', 'ok');
}
function openImportSim() { closeCmd(); $('#simimpOverlay').hidden = false; }
async function submitImportSim() {
  const path = $('#simimpPath').value.trim();
  if (!path) { toast('Nhập đường dẫn hồ sơ .json', 'error'); return; }
  const res = await post('/api/importsim', { path });
  if (res) { toast('Đã bắt đầu nhập hồ sơ — xem tiến trình ở thanh dưới', 'ok'); $('#simimpOverlay').hidden = true; }
}

// ── Chẩn đoán (diag) — chỉ-đọc, dùng GET ──
async function openDiag() {
  closeCmd();
  $('#diagBody').textContent = 'Đang tải…';
  $('#diagPath').textContent = '';
  $('#diagOverlay').hidden = false;
  try {
    const r = await fetch('/api/diag');
    const data = await r.json().catch(() => ({}));
    if (!r.ok) { $('#diagBody').textContent = data.error || ('HTTP ' + r.status); return; }
    $('#diagBody').textContent = data.markdown || '(trống)';
    $('#diagPath').textContent = data.path ? ('Đã lưu: ' + data.path) : '';
  } catch (e) {
    $('#diagBody').textContent = String(e);
  }
}

// ── Thanh tiến trình job ──
let jobHideTimer;
function jobLabel(name) {
  return name === 'import' ? 'Nhập truyện'
    : name === 'simulate' ? 'Hồ sơ mô phỏng'
    : name === 'importsim' ? 'Nhập hồ sơ' : (name || 'Tác vụ');
}
function onJobEvent(j) {
  if (!j) return;
  const bar = $('#jobBar');
  clearTimeout(jobHideTimer);
  if (j.done) {
    bar.hidden = false;
    bar.dataset.kind = 'ok';
    bar.textContent = `✓ ${jobLabel(j.name)} hoàn thành`;
    jobHideTimer = setTimeout(() => { bar.hidden = true; }, 4000);
    return;
  }
  bar.hidden = false;
  bar.dataset.kind = j.error ? 'error' : '';
  const prog = j.total ? ` ${j.current}/${j.total}` : '';
  const stage = j.stage ? ` · ${j.stage}` : '';
  const tail = j.error ? ` · ${j.error}` : (j.message ? ` · ${j.message}` : '');
  bar.textContent = `${jobLabel(j.name)}${stage}${prog}${tail}`;
}

// ── Catalog lệnh (mirror command_registry.go của TUI) ──
const COMMANDS = [
  { key: 'cocreate', label: 'Đồng sáng tác', desc: 'Trò chuyện với AI để lên kế hoạch rồi viết / định hướng tiếp', run: openCoCreate },
  { key: 'export', label: 'Xuất bản', desc: 'Xuất các chương đã hoàn thành ra TXT/EPUB', run: openExport },
  { key: 'import', label: 'Nhập tiểu thuyết ngoài', desc: 'Phản suy từ truyện có sẵn để viết tiếp (cần engine rảnh)', run: openImport },
  { key: 'simulate', label: 'Tạo hồ sơ mô phỏng', desc: 'Đọc thư mục ./simulate dựng hồ sơ văn phong (cần engine rảnh)', run: runSimulate },
  { key: 'importsim', label: 'Nhập hồ sơ mô phỏng', desc: 'Nhập hồ sơ văn phong .json có sẵn (cần engine rảnh)', run: openImportSim },
  { key: 'diag', label: 'Chẩn đoán', desc: 'Báo cáo chẩn đoán sáng tác + runtime', run: openDiag },
  { key: 'model', label: 'Model & suy luận', desc: 'Đổi model / mức suy luận theo vai trò', run: () => { closeCmd(); openSettings(); } },
];

// ── Wiring ──
function bootStudio() {
  $('#cmdBtn').addEventListener('click', openCmd);
  $('#cmdClose').addEventListener('click', closeCmd);
  $('#cmdFilter').addEventListener('input', renderCmdList);
  $('#cmdFilter').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); runFirstCmd(); }
    else if (e.key === 'Escape') closeCmd();
  });
  $('#cmdOverlay').addEventListener('click', (e) => { if (e.target === $('#cmdOverlay')) closeCmd(); });
  document.addEventListener('keydown', (e) => {
    if (e.key === '/' && !isTyping(e.target) && allOverlaysHidden()) { e.preventDefault(); openCmd(); }
  });

  // cocreate
  $('#ccSend').addEventListener('click', () => ccSend());
  $('#ccInput').addEventListener('keydown', (e) => { if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); ccSend(); } });
  $('#ccFinish').addEventListener('click', ccFinish);
  $('#ccCancel').addEventListener('click', () => closeCc(false));
  $('#ccClose').addEventListener('click', () => closeCc(false));

  // export
  $('#expSubmit').addEventListener('click', submitExport);
  $('#expCancel').addEventListener('click', () => { $('#expOverlay').hidden = true; });
  $('#expClose').addEventListener('click', () => { $('#expOverlay').hidden = true; });

  // import / importsim
  $('#impSubmit').addEventListener('click', submitImport);
  $('#impClose').addEventListener('click', () => { $('#impOverlay').hidden = true; });
  $('#simimpSubmit').addEventListener('click', submitImportSim);
  $('#simimpClose').addEventListener('click', () => { $('#simimpOverlay').hidden = true; });

  // diag
  $('#diagClose').addEventListener('click', () => { $('#diagOverlay').hidden = true; });
}

bootStudio();
