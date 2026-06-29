// ainovel web — bộ điều khiển trang đơn, vanilla JS (0 dependency).
// Xuống: EventSource('/api/events') nhận stream/event/snapshot/clear/done.
// Lên: fetch POST start/steer/continue/abort/resume.
'use strict';

const $ = (sel) => document.querySelector(sel);

const THINKING_SEP = '\x02';
let currentState = 'idle';
let roundHasContent = false; // round stream hiện tại đã có chữ chưa (để chèn divider khi clear)
let streamIsThinking = false; // Host uses \x02 to toggle thinking; route it to the thinking pane.
let lastSnapshot = null;     // snapshot mới nhất — app-studio.js đọc để biết tiến độ/trạng thái (Phase 3)
const logIndex = new Map();  // Event.ID -> <li> (cập nhật tại chỗ: đang chạy -> hoàn thành)

// ── Dispatch khung SSE ──
function handle(m) {
  switch (m.type) {
    case 'hello': break;
    case 'stream': if (appendStream(m.text)) roundHasContent = true; break;
    case 'clear': streamIsThinking = false; if (roundHasContent) { appendDivider(); roundHasContent = false; } break;
    case 'event': handleEvent(m.data); break;
    case 'snapshot': renderSnapshot(m.data); break;
    case 'ask': showAsk(m.data); break;            // engine hỏi (payload key thường — tools.Question có json tag)
    case 'ask-cancel': closeAskIf(m.data); break;  // run kết thúc/abort khi đang hỏi
    case 'cocreate': if (typeof onCoCreateProgress === 'function') onCoCreateProgress(m.data); break; // Phase 3 (app-studio.js)
    case 'job': if (typeof onJobEvent === 'function') onJobEvent(m.data); break;                       // Phase 3 (app-studio.js)
    case 'done': break; // snapshot terminal đã được đẩy ngay trước done
  }
}

// ── Stream pane ──
function appendThinking(text) {
  if (!text) return;
  const t = $('#thinkingStream');
  const ph = t.querySelector('.placeholder');
  if (ph) ph.remove();
  t.appendChild(document.createTextNode(text));
  t.scrollTop = t.scrollHeight;
}
function appendDraft(text) {
  if (!text) return false;
  const s = $('#stream');
  const ph = s.querySelector('.placeholder');
  if (ph) ph.remove();
  s.appendChild(document.createTextNode(text));
  s.scrollTop = s.scrollHeight;
  return true;
}
function appendStream(text) {
  if (!text) return false;
  let appendedDraft = false;
  const chunks = String(text).split(THINKING_SEP);
  for (let i = 0; i < chunks.length; i++) {
    const part = chunks[i];
    if (part) {
      if (streamIsThinking) appendThinking(part);
      else if (appendDraft(part)) appendedDraft = true;
    }
    if (i < chunks.length - 1) streamIsThinking = !streamIsThinking;
  }
  return appendedDraft;
}
function appendDivider() {
  const s = $('#stream');
  const hr = document.createElement('hr');
  hr.className = 'divider';
  s.appendChild(hr);
  s.scrollTop = s.scrollHeight;
}

// ── Event log ──
function isZeroTime(t) { return !t || t.startsWith('0001-01-01'); }
function fmtTime(t) {
  if (!t) return '--:--:--';
  const d = new Date(t);
  if (isNaN(d.getTime())) return '--:--:--';
  return d.toTimeString().slice(0, 8);
}

// ── i18n nhật ký sự kiện ──
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
  if (!text) return text;
  if (EVENT_SUMMARY_MAP[text]) return EVENT_SUMMARY_MAP[text];
  const m = text.match(RETRY_RE);
  if (m) return text.replace(RETRY_RE, `Thử lại (${m[1]}/${m[2]}): `);
  for (const [zh, vi] of EVENT_PREFIX_MAP) {
    if (text.startsWith(zh)) return vi + text.slice(zh.length);
  }
  return text;
}

function handleEvent(ev) {
  if (!ev || !ev.Summary) return;
  const id = ev.ID || '';
  const running = id !== '' && isZeroTime(ev.FinishedAt);
  const logEl = $('#log');
  let li = id ? logIndex.get(id) : null;
  if (!li) {
    li = document.createElement('li');
    if (id) logIndex.set(id, li);
    logEl.appendChild(li);
  }
  li.dataset.running = running ? 'true' : 'false';
  li.dataset.level = ev.Level || '';
  li.innerHTML = '';
  const ts = document.createElement('span'); ts.className = 'ts'; ts.textContent = fmtTime(ev.Time);
  const cat = document.createElement('span'); cat.className = 'cat'; cat.textContent = ev.Category || '';
  const sum = document.createElement('span'); sum.className = 'sum'; sum.textContent = translateSummary(ev.Summary);
  li.append(ts, cat, sum);
  logEl.scrollTop = logEl.scrollHeight;
  // chặn log phình vô hạn
  while (logEl.children.length > 500) {
    const first = logEl.firstChild;
    for (const [k, v] of logIndex) { if (v === first) { logIndex.delete(k); break; } }
    logEl.removeChild(first);
  }
}

// ── Dashboard (Snapshot) ──
function renderSnapshot(s) {
  if (!s) return;
  lastSnapshot = s;
  currentState = s.RuntimeState || 'idle';

  $('#novelName').textContent = s.NovelName || 'Chưa có tên';
  const badge = $('#stateBadge');
  badge.dataset.state = currentState;
  badge.textContent = s.StatusLabel || currentState;

  const cur = s.CurrentChapter || 0, tot = s.TotalChapters || 0, done = s.CompletedCount || 0;
  $('#chapters').textContent = tot ? `${cur} / ${tot}` : (cur || '—');
  $('#completed').textContent = done || '0';
  $('#words').textContent = (s.TotalWordCount || 0).toLocaleString();
  $('#phase').textContent = s.Phase || '—';
  $('#flow').textContent = s.Flow || '—';
  $('#progressFill').style.width = (tot ? Math.min(100, Math.round((done / tot) * 100)) : 0) + '%';

  const ag = $('#agents');
  ag.innerHTML = '';
  const agents = s.Agents || [];
  if (!agents.length) {
    ag.innerHTML = '<li class="muted">—</li>';
  } else {
    agents.forEach((a) => {
      const li = document.createElement('li');
      const dot = document.createElement('span');
      dot.className = 'agent-dot';
      dot.dataset.on = (a.State && a.State !== 'idle') ? 'true' : 'false';
      const nm = document.createElement('span'); nm.className = 'agent-name'; nm.textContent = a.Name || '?';
      const tk = document.createElement('span'); tk.className = 'agent-task';
      tk.textContent = a.Tool || a.Summary || a.State || '';
      li.append(dot, nm, tk);
      ag.appendChild(li);
    });
  }

  $('#ctx').textContent = s.ContextWindow
    ? `${(s.ContextTokens || 0).toLocaleString()} / ${s.ContextWindow.toLocaleString()}`
    : '—';
  $('#ctxFill').style.width = Math.min(100, s.ContextPercent || 0) + '%';
  $('#model').textContent = s.ModelName || '—';
  $('#cost').textContent = (s.TotalCostUSD != null)
    ? ('$' + Number(s.TotalCostUSD).toFixed(4) + (s.BudgetLimitUSD ? ' / $' + s.BudgetLimitUSD : ''))
    : '—';

  const pc = $('#premiseCard');
  const chars = s.Characters || [];
  if (s.Premise || chars.length) {
    pc.hidden = false;
    $('#premise').textContent = s.Premise || '';
    const ch = $('#characters');
    ch.innerHTML = '';
    chars.slice(0, 12).forEach((c) => { const li = document.createElement('li'); li.textContent = c; ch.appendChild(li); });
  } else {
    pc.hidden = true;
  }

  updateControls(s);
}

// ── Chế độ input theo trạng thái ──
function sendModeFor(state) {
  if (state === 'running' || state === 'pausing') return 'steer';
  if (state === 'paused' || state === 'completed') return 'continue';
  return 'start'; // idle / unknown
}
function updateControls(s) {
  const st = s.RuntimeState || 'idle';
  const mode = sendModeFor(st);
  $('#sendBtn').textContent = mode === 'start' ? 'Bắt đầu' : mode === 'steer' ? 'Can thiệp' : 'Tiếp tục';
  $('#modeHint').textContent = 'Chế độ: ' + (mode === 'start' ? 'Bắt đầu' : mode === 'steer' ? 'Can thiệp (đang chạy)' : 'Tiếp tục');
  $('#abortBtn').hidden = !(st === 'running' || st === 'pausing' || st === 'paused');
  const resumeBtn = $('#resumeBtn');
  resumeBtn.hidden = !(st === 'idle' && s.RecoveryLabel);
  if (s.RecoveryLabel) resumeBtn.textContent = 'Khôi phục: ' + s.RecoveryLabel;
}

// ── Gọi API ──
async function post(url, body) {
  try {
    const r = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    });
    const data = await r.json().catch(() => ({}));
    if (!r.ok) { toast(data.error || ('HTTP ' + r.status), 'error'); return null; }
    return data;
  } catch (e) {
    toast(String(e), 'error');
    return null;
  }
}

// startNovel gọi /api/start. Nếu BE chặn vì còn phiên khôi phục được (409 code=recoverable),
// hỏi xác nhận rồi thử lại với force=true — tránh xoá nhầm tiến độ cũ (StartPrepared reset sạch).
async function startNovel(prompt, force) {
  try {
    const r = await fetch('/api/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ prompt, force: !!force }),
    });
    const data = await r.json().catch(() => ({}));
    if (r.status === 409 && data.code === 'recoverable' && !force) {
      if (confirm((data.error || 'Có tiến độ khôi phục được.') + '\n\nVẫn tạo truyện MỚI và xoá tiến độ này?')) {
        return startNovel(prompt, true);
      }
      return null;
    }
    if (!r.ok) { toast(data.error || ('HTTP ' + r.status), 'error'); return null; }
    return data;
  } catch (e) {
    toast(String(e), 'error');
    return null;
  }
}

async function send() {
  const input = $('#input');
  const text = input.value.trim();
  const mode = sendModeFor(currentState);
  let res;
  if (mode === 'start') {
    if (!text) { toast('Nhập yêu cầu trước khi bắt đầu', 'error'); return; }
    res = await startNovel(text, false);
  } else if (mode === 'steer') {
    if (!text) { toast('Nhập nội dung can thiệp', 'error'); return; }
    res = await post('/api/steer', { text });
  } else {
    res = await post('/api/continue', { text });
  }
  if (res) {
    input.value = '';
    if (mode === 'steer') toast('Đã gửi can thiệp', 'ok');
  }
}

// ── Toast ──
let toastTimer;
function toast(msg, kind) {
  const t = $('#toast');
  t.textContent = msg;
  t.dataset.kind = kind || '';
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.hidden = true; }, 4000);
}

// ── Modal: ask_user ──
// LƯU Ý: payload ask dùng key THƯỜNG (id/questions; question/header/options/multiSelect;
// label/description) vì tools.Question có json tag — khác với event/snapshot (PascalCase).
let currentAsk = null; // {id, questions}

function showAsk(data) {
  if (!data || !data.id || !Array.isArray(data.questions)) return;
  currentAsk = data;
  const body = $('#askBody');
  body.innerHTML = '';
  data.questions.forEach((q, qi) => {
    const block = document.createElement('div');
    block.className = 'ask-q';

    const head = document.createElement('div');
    head.className = 'ask-q-head';
    const chip = document.createElement('span'); chip.className = 'badge'; chip.textContent = q.header || '';
    const title = document.createElement('span'); title.className = 'ask-q-title'; title.textContent = q.question || '';
    head.append(chip, title);

    const opts = document.createElement('div');
    opts.className = 'ask-options';
    const type = q.multiSelect ? 'checkbox' : 'radio';
    (q.options || []).forEach((opt) => {
      const row = document.createElement('label');
      row.className = 'ask-opt';
      const input = document.createElement('input');
      input.type = type; input.name = 'ask-' + qi; input.value = opt.label || '';
      const txt = document.createElement('span');
      const lab = document.createElement('span'); lab.className = 'ask-opt-label'; lab.textContent = opt.label || '';
      const desc = document.createElement('span'); desc.className = 'ask-opt-desc';
      desc.textContent = opt.description ? (' — ' + opt.description) : '';
      txt.append(lab, desc);
      row.append(input, txt);
      opts.appendChild(row);
    });

    const custom = document.createElement('div');
    custom.className = 'ask-custom';
    const clab = document.createElement('label'); clab.textContent = 'Hoặc tự nhập:';
    const cinput = document.createElement('input');
    cinput.type = 'text'; cinput.className = 'ask-note'; cinput.placeholder = 'Nhập câu trả lời của bạn…';
    custom.append(clab, cinput);

    block.append(head, opts, custom);
    body.appendChild(block);
  });
  $('#askOverlay').hidden = false;
}

// collectAsk gom đáp án theo ngữ nghĩa tools.AskUserResponse (key = nguyên văn câu hỏi):
// có chọn option → answer=label(join 、); chỉ tự nhập → answer="自定义"+note. Thiếu → null.
function collectAsk() {
  const answers = {}, notes = {};
  const blocks = $('#askBody').querySelectorAll('.ask-q');
  for (let qi = 0; qi < currentAsk.questions.length; qi++) {
    const q = currentAsk.questions[qi];
    const block = blocks[qi];
    const checked = [...block.querySelectorAll('input:checked')].map((i) => i.value);
    const note = block.querySelector('.ask-note').value.trim();
    if (checked.length) {
      answers[q.question] = checked.join('、');
      if (note) notes[q.question] = note;
    } else if (note) {
      answers[q.question] = '自定义';
      notes[q.question] = note;
    } else {
      return null;
    }
  }
  return { answers, notes };
}

async function submitAsk() {
  if (!currentAsk) return;
  const collected = collectAsk();
  if (!collected) { toast('Vui lòng trả lời tất cả câu hỏi', 'error'); return; }
  const res = await post('/api/ask', { id: currentAsk.id, answers: collected.answers, notes: collected.notes });
  if (res) closeAsk();
}

async function skipAsk() {
  if (!currentAsk) return;
  await post('/api/ask', { id: currentAsk.id, answers: {}, notes: {} }); // rỗng → engine tự quyết (formatAnswers)
  closeAsk();
}

function closeAsk() { $('#askOverlay').hidden = true; currentAsk = null; }
function closeAskIf(data) {
  if (currentAsk && data && data.id === currentAsk.id) {
    closeAsk();
    toast('Phiên hỏi đã hủy (run kết thúc/abort)', 'error');
  }
}

// ── Modal: settings (đổi model + mức suy luận theo vai trò) ──
let modelData = null;

async function openSettings() {
  $('#setOverlay').hidden = false;
  $('#setBody').innerHTML = '<p class="muted">Đang tải…</p>';
  try {
    modelData = await fetch('/api/models').then((r) => r.json());
  } catch (e) { $('#setBody').innerHTML = ''; toast('Tải model lỗi: ' + e, 'error'); return; }
  renderSettings();
}

function renderSettings() {
  const body = $('#setBody');
  body.innerHTML = '';
  const providers = modelData.providers || [];
  const models = modelData.models || {};
  (modelData.roles || []).forEach((role) => {
    const row = document.createElement('div');
    row.className = 'role-row';
    const h = document.createElement('h3'); h.textContent = role.label; row.appendChild(h);

    // hàng provider + model + nút Đổi
    const line1 = document.createElement('div'); line1.className = 'role-line';
    const pLbl = document.createElement('span'); pLbl.className = 'lbl'; pLbl.textContent = 'Provider';
    const pSel = document.createElement('select');
    providers.forEach((p) => { const o = document.createElement('option'); o.value = p; o.textContent = p; if (p === role.provider) o.selected = true; pSel.appendChild(o); });
    const mSel = document.createElement('select');
    const fillModels = (provider, selected) => {
      mSel.innerHTML = '';
      const list = models[provider] || [];
      list.forEach((m) => { const o = document.createElement('option'); o.value = m; o.textContent = m; if (m === selected) o.selected = true; mSel.appendChild(o); });
      if (!list.length) { const o = document.createElement('option'); o.value = ''; o.textContent = '(không có)'; mSel.appendChild(o); }
    };
    fillModels(role.provider, role.model);
    pSel.addEventListener('change', () => fillModels(pSel.value, null));
    const mApply = document.createElement('button'); mApply.className = 'btn sm primary'; mApply.textContent = 'Đổi';
    mApply.addEventListener('click', () => applyModel(role.key, pSel.value, mSel.value));
    line1.append(pLbl, pSel, mSel, mApply);

    // hàng mức suy luận + nút Đặt
    const line2 = document.createElement('div'); line2.className = 'role-line';
    const tLbl = document.createElement('span'); tLbl.className = 'lbl'; tLbl.textContent = 'Suy luận';
    const tSel = document.createElement('select');
    (role.thinkingOptions || []).forEach((opt) => { const o = document.createElement('option'); o.value = opt.key; o.textContent = opt.label; if (opt.key === role.thinking) o.selected = true; tSel.appendChild(o); });
    const tApply = document.createElement('button'); tApply.className = 'btn sm'; tApply.textContent = 'Đặt';
    tApply.addEventListener('click', () => applyThinking(role.key, tSel.value));
    line2.append(tLbl, tSel, tApply);

    row.append(line1, line2);
    body.appendChild(row);
  });
}

async function applyModel(role, provider, model) {
  if (!model) { toast('Chọn model trước', 'error'); return; }
  const res = await post('/api/model', { role, provider, model });
  if (res) { toast('Đã đổi model: ' + role + ' → ' + provider + '/' + model, 'ok'); await refreshSettings(); }
}
async function applyThinking(role, level) {
  const res = await post('/api/thinking', { role, level });
  if (res) { toast('Đã đặt suy luận: ' + role, 'ok'); await refreshSettings(); }
}
async function refreshSettings() {
  try { modelData = await fetch('/api/models').then((r) => r.json()); renderSettings(); } catch (e) { /* giữ panel cũ */ }
}
function closeSettings() { $('#setOverlay').hidden = true; }

// ── Boot ──
async function boot() {
  $('#sendBtn').addEventListener('click', send);
  $('#abortBtn').addEventListener('click', () => post('/api/abort', {}));
  $('#resumeBtn').addEventListener('click', async () => {
    const d = await post('/api/resume', {});
    if (d && d.ok === false) toast('Không có phiên để khôi phục', 'error');
  });
  $('#clearStream').addEventListener('click', () => { $('#stream').innerHTML = '<div class="placeholder">Đã xóa bản thảo hiển thị.</div>'; $('#thinkingStream').innerHTML = '<div class="placeholder">Đã xóa thinking hiển thị.</div>'; roundHasContent = false; streamIsThinking = false; });
  $('#input').addEventListener('keydown', (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); send(); }
  });

  // ask_user modal
  $('#askSubmit').addEventListener('click', submitAsk);
  $('#askSkip').addEventListener('click', skipAsk);
  // settings modal
  $('#settingsBtn').addEventListener('click', openSettings);
  $('#setClose').addEventListener('click', closeSettings);
  $('#setOverlay').addEventListener('click', (e) => { if (e.target === $('#setOverlay')) closeSettings(); });

  // Open SSE first so the server registers this browser before snapshot/replay fetches.
  // Live frames arriving during replay are buffered in pendingLive; no replay-to-connect gap.
  // Event IDs are updated in place by handleEvent, so duplicate finish events do not duplicate log rows.
  const pendingLive = [];
  let bootReady = false;
  let booting = false;
  const finishBoot = async () => {
    if (booting || bootReady) return;
    booting = true;
    try {
      const snap = await fetch('/api/snapshot').then((r) => r.json());
      renderSnapshot(snap);
      const replay = await fetch('/api/replay?after=0').then((r) => r.json());
      (replay || []).forEach(handle);
      bootReady = true;
      while (pendingLive.length) handle(pendingLive.shift());
    } catch (e) {
      toast('Tai trang thai loi: ' + e, 'error');
      bootReady = true;
      while (pendingLive.length) handle(pendingLive.shift());
    }
  };

  const es = new EventSource('/api/events');
  es.onmessage = (e) => {
    let m;
    try { m = JSON.parse(e.data); } catch { return; }
    if (m.type === 'hello') { finishBoot(); return; }
    if (!bootReady) { pendingLive.push(m); return; }
    handle(m);
  };
}

boot();
