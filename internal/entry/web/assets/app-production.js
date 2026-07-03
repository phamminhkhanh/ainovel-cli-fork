// ainovel web — Production Cockpit tab (Sản xuất).
// Nạp sau app.js để dùng $, post, toast.
'use strict';

let productionSelectedRunId = null;
let productionPollTimer = null;
let productionRunsCache = [];
let productionProfilesCache = [];

function renderProductionTab() {
  const panel = $('#tab-production');
  if (!panel) return;
  if (!panel.dataset.initialized) {
    panel.innerHTML = `
      <div class="production-layout">
        <aside class="run-list">
          <div class="run-list-head">
            <h3>Danh sách job</h3>
            <button class="btn sm primary" id="btnNewRun">+ Tạo</button>
          </div>
          <ul class="run-list-items" id="runListItems"><li class="muted">Đang tải…</li></ul>
        </aside>
        <main class="run-detail" id="runDetail">
          <div class="placeholder">Chọn một job bên trái để xem chi tiết.</div>
        </main>
      </div>
      <div class="modal-overlay" id="newRunOverlay" hidden>
        <div class="modal" role="dialog" aria-modal="true" aria-labelledby="newRunTitle">
          <header class="modal-head"><h2 id="newRunTitle">Tạo job sản xuất</h2></header>
          <div class="modal-body">
            <div class="field"><label for="newRunName">Tên job</label><input type="text" id="newRunName" placeholder="vd: Werewolf romantasy 50 chương"></div>
            <div class="field"><label for="newRunProfile">Profile tạo truyện</label><select id="newRunProfile"><option value="">Đang tải…</option></select><small class="muted">File .md trong ./.ainovel/profiles/ hoặc ~/.ainovel/profiles/.</small></div>
            <div class="field-row">
              <div class="field"><label for="newRunModel">Model (tùy chọn)</label><input type="text" id="newRunModel" placeholder="vd: gpt-4o"></div>
              <div class="field"><label for="newRunProvider">Provider (tùy chọn)</label><input type="text" id="newRunProvider" placeholder="vd: openai"></div>
            </div>
            <div class="field-row">
              <div class="field"><label for="newRunTarget">Số chương mục tiêu</label><input type="number" id="newRunTarget" min="1" value="30"></div>
              <div class="field"><label for="newRunBudget">Ngân sách (USD)</label><input type="number" id="newRunBudget" min="0.1" step="0.1" value="5"></div>
            </div>
          </div>
          <footer class="modal-foot">
            <button class="btn" id="newRunCancel">Hủy</button>
            <button class="btn primary" id="newRunSubmit">Tạo job</button>
          </footer>
        </div>
      </div>`;
    panel.dataset.initialized = 'true';
    bindProductionEvents();
  }
  loadProductionData();
  startProductionPoll();
}

function bindProductionEvents() {
  $('#btnNewRun')?.addEventListener('click', openNewRunModal);
  $('#newRunCancel')?.addEventListener('click', closeNewRunModal);
  $('#newRunOverlay')?.addEventListener('click', (e) => {
    if (e.target.id === 'newRunOverlay') closeNewRunModal();
  });
  $('#newRunSubmit')?.addEventListener('click', submitNewRun);
  $('#runListItems')?.addEventListener('click', (e) => {
    const li = e.target.closest('[data-run-id]');
    if (!li) return;
    selectProductionRun(li.dataset.runId);
  });
  $('#runDetail')?.addEventListener('click', (e) => {
    const btn = e.target.closest('button');
    if (!btn) return;
    const id = btn.dataset.runId;
    if (!id) return;
    if (btn.dataset.action === 'start') startProductionRun(id);
    else if (btn.dataset.action === 'stop') stopProductionRun(id);
    else if (btn.dataset.action === 'delete') deleteProductionRun(id);
    else if (btn.dataset.action === 'export') exportProductionRun(id);
    else if (btn.dataset.action === 'sync') syncProductionRun(id);
  });
}

function openNewRunModal() {
  populateProfileSelect();
  $('#newRunOverlay').hidden = false;
  $('#newRunName').focus();
}
function closeNewRunModal() { $('#newRunOverlay').hidden = true; }

function productionProfileLabel(profile) {
  const source = profile.source === 'project' ? 'Dự án' : profile.source === 'global' ? 'Global' : profile.source === 'legacy' ? 'Legacy' : 'Profile';
  return `${source} · ${profile.name || profile.path || ''}`;
}

function renderProductionProfileOptions(profiles) {
  if (!profiles.length) {
    return '<option value="">Chưa có profile (.md)</option>';
  }
  return profiles.map((p) => `<option value="${escapeHtml(p.path)}">${escapeHtml(productionProfileLabel(p))}</option>`).join('');
}

async function populateProfileSelect() {
  const sel = $('#newRunProfile');
  if (!sel || productionProfilesCache.length) {
    if (sel) {
      sel.innerHTML = renderProductionProfileOptions(productionProfilesCache);
    }
    return;
  }
  try {
    const res = await fetch('/api/profiles');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    productionProfilesCache = await res.json();
    sel.innerHTML = renderProductionProfileOptions(productionProfilesCache);
  } catch (e) {
    sel.innerHTML = '<option value="">Lỗi tải profile</option>';
    toast('Lỗi tải profile: ' + e, 'error');
  }
}

async function submitNewRun() {
  const body = {
    name: $('#newRunName').value.trim(),
    profile: $('#newRunProfile').value,
    model: $('#newRunModel').value.trim() || undefined,
    provider: $('#newRunProvider').value.trim() || undefined,
    targetChapters: parseInt($('#newRunTarget').value, 10) || 30,
    budgetUsd: parseFloat($('#newRunBudget').value) || 5,
  };
  if (!body.name) { toast('Nhập tên job', 'error'); return; }
  if (!body.profile) { toast('Chọn profile', 'error'); return; }
  const res = await post('/api/prodruns', body);
  if (res) {
    closeNewRunModal();
    clearNewRunForm();
    productionSelectedRunId = res.id;
    await loadProductionData();
    toast('Đã tạo job', 'ok');
  }
}

function clearNewRunForm() {
  $('#newRunName').value = '';
  $('#newRunModel').value = '';
  $('#newRunProvider').value = '';
  $('#newRunTarget').value = '30';
  $('#newRunBudget').value = '5';
}

async function loadProductionData() {
  await Promise.all([loadProductionRuns(), populateProfileSelect()]);
  renderProductionRuns();
  if (productionSelectedRunId) {
    const run = productionRunsCache.find((r) => r.id === productionSelectedRunId);
    renderProductionDetail(run || null);
  }
}

async function loadProductionRuns() {
  try {
    const res = await fetch('/api/prodruns');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    productionRunsCache = await res.json();
  } catch (e) {
    toast('Lỗi tải danh sách job: ' + e, 'error');
  }
}

function selectProductionRun(id) {
  productionSelectedRunId = id;
  renderProductionRuns();
  const run = productionRunsCache.find((r) => r.id === id);
  renderProductionDetail(run || null);
}

async function startProductionRun(id) {
  const res = await post(`/api/prodruns/${id}/start`, {});
  if (res) {
    toast('Đã bắt đầu job', 'ok');
    await loadProductionData();
  }
}

async function stopProductionRun(id) {
  if (!confirm('Dừng job này? Tiến trình con sẽ bị kill ngay lập tức.')) return;
  const res = await post(`/api/prodruns/${id}/stop`, {});
  if (res) {
    toast('Đã dừng job', 'ok');
    await loadProductionData();
  }
}

async function deleteProductionRun(id) {
  if (!confirm('Xóa job này? Thư mục run và log sẽ bị xóa.')) return;
  try {
    const r = await fetch(`/api/prodruns/${id}`, { method: 'DELETE' });
    if (!r.ok) {
      const data = await r.json().catch(() => ({}));
      toast(data.error || ('HTTP ' + r.status), 'error');
      return;
    }
    if (productionSelectedRunId === id) productionSelectedRunId = null;
    toast('Đã xóa job', 'ok');
    await loadProductionData();
  } catch (e) {
    toast('Lỗi xóa job: ' + e, 'error');
  }
}

async function exportProductionRun(id) {
  const res = await post(`/api/prodruns/${id}/export`, { format: 'txt' });
  if (!res) return;
  // Trigger browser download via the dedicated GET endpoint.
  const a = document.createElement('a');
  a.href = `/api/prodruns/${id}/export.txt`;
  a.download = '';
  document.body.appendChild(a);
  a.click();
  a.remove();
  toast('Đã xuất TXT', 'ok');
}

async function syncProductionRun(id, force) {
  const msg = force
    ? 'Workspace chính đã có dữ liệu. Ghi đè toàn bộ bằng kết quả job này?'
    : 'Đồng bộ kết quả job vào workspace chính? Dữ liệu workspace hiện tại phải trống.';
  if (!confirm(msg)) return;
  try {
    const r = await fetch(`/api/prodruns/${id}/sync`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ force: !!force }),
    });
    if (!r.ok) {
      const data = await r.json().catch(() => ({}));
      if (r.status === 409 && data.error && data.error.includes('already has progress') && !force) {
        syncProductionRun(id, true);
        return;
      }
      toast(data.error || ('HTTP ' + r.status), 'error');
      return;
    }
    const data = await r.json().catch(() => ({}));
    toast('Đã đồng bộ ' + (data.copiedFiles || 0) + ' tệp vào workspace', 'ok');
    await loadProductionData();
    // Refresh the main workspace sidebar/tabs immediately.
    try {
      const snap = await fetch('/api/snapshot').then((r) => r.json());
      if (typeof renderSnapshot === 'function') renderSnapshot(snap);
    } catch (e) {
      // Ignore refresh errors; the SSE poll will catch up.
    }
  } catch (e) {
    toast('Lỗi đồng bộ: ' + e, 'error');
  }
}

function renderProductionRuns() {
  const ul = $('#runListItems');
  if (!ul) return;
  if (!productionRunsCache.length) {
    ul.innerHTML = '<li class="muted">Chưa có job nào.</li>';
    return;
  }
  ul.innerHTML = productionRunsCache.map((r) => {
    const selected = r.id === productionSelectedRunId ? ' selected' : '';
    return `<li class="run-item${selected}" data-run-id="${escapeHtml(r.id)}" tabindex="0">
      <div class="run-item-head">
        <span class="run-item-name">${escapeHtml(r.name || r.id)}</span>
        <span class="run-badge run-badge-${r.status}">${escapeHtml(statusLabel(r.status))}</span>
      </div>
      <div class="run-item-meta">${r.chapters || 0}/${r.targetChapters} chương · $${(r.costUsd || 0).toFixed(2)}</div>
    </li>`;
  }).join('');
}

async function renderProductionDetail(run) {
  const detail = $('#runDetail');
  if (!detail) return;
  if (!run) {
    detail.innerHTML = '<div class="placeholder">Chọn một job bên trái để xem chi tiết.</div>';
    return;
  }

  const runtime = formatDuration(run.startedAt ? new Date(run.startedAt) : null, run.stoppedAt ? new Date(run.stoppedAt) : null);
  const progress = run.targetChapters > 0 ? Math.min(100, Math.round((run.chapters || 0) / run.targetChapters * 100)) : 0;
  let logHtml = '';
  try {
    const res = await fetch(`/api/prodruns/${run.id}/log?lines=50`);
    const text = res.ok ? await res.text() : '';
    logHtml = text ? `<pre class="run-log">${escapeHtml(text)}</pre>` : '<p class="muted">Chưa có log.</p>';
  } catch (e) {
    logHtml = '<p class="muted">Lỗi tải log.</p>';
  }

  const pauseNotice = run.status === 'paused'
    ? '<div class="run-pause-notice">Tạm dừng — chỉ có thể Dừng hoặc xuất file.</div>'
    : '';

  const canStart = run.status === 'queued';
  const canStop = run.status === 'running' || run.status === 'paused';
  const canExport = run.chapters > 0;
  const canDelete = run.status !== 'running' && run.status !== 'paused';
  const canSync = run.chapters > 0 && run.status !== 'running' && run.status !== 'paused';

  detail.innerHTML = `
    <div class="run-detail-card">
      <div class="run-detail-head">
        <h2>${escapeHtml(run.name || run.id)}</h2>
        <span class="run-badge run-badge-${run.status}">${escapeHtml(statusLabel(run.status))}</span>
      </div>
      ${pauseNotice}
      <div class="run-detail-stats">
        <div class="stat"><span class="stat-label">Chương</span><span class="stat-value">${run.chapters || 0} / ${run.targetChapters}</span></div>
        <div class="stat"><span class="stat-label">Đánh giá</span><span class="stat-value">${run.reviews || 0}</span></div>
        <div class="stat"><span class="stat-label">Viết lại</span><span class="stat-value">${run.rewrites || 0}</span></div>
        <div class="stat"><span class="stat-label">Chi phí</span><span class="stat-value">$${(run.costUsd || 0).toFixed(2)} / $${(run.budgetUsd || 0).toFixed(2)}</span></div>
        <div class="stat"><span class="stat-label">Thời gian</span><span class="stat-value">${runtime}</span></div>
        <div class="stat"><span class="stat-label">Lý do dừng</span><span class="stat-value">${escapeHtml(run.stopReason || '—')}</span></div>
      </div>
      <div class="progress-bar"><div class="progress-fill" style="width:${progress}%"></div></div>
      <div class="run-detail-actions">
        <button class="btn primary" data-action="start" data-run-id="${escapeHtml(run.id)}" ${!canStart ? 'disabled' : ''}>▶ Bắt đầu</button>
        <button class="btn danger" data-action="stop" data-run-id="${escapeHtml(run.id)}" ${!canStop ? 'disabled' : ''}>■ Dừng</button>
        <button class="btn" data-action="export" data-run-id="${escapeHtml(run.id)}" ${!canExport ? 'disabled' : ''}>⬇ Xuất TXT</button>
        <button class="btn" data-action="sync" data-run-id="${escapeHtml(run.id)}" ${!canSync ? 'disabled' : ''}>🔄 Đồng bộ</button>
        <button class="btn danger" data-action="delete" data-run-id="${escapeHtml(run.id)}" ${!canDelete ? 'disabled' : ''}>🗑 Xóa</button>
      </div>
      ${run.possiblyOrphaned ? '<div class="run-orphan-warning">⚠ Job này có thể còn tiến trình con mồ côi. Kiểm tra PID ' + (run.childPid || '—') + '.</div>' : ''}
      <h4>Nhật ký</h4>
      ${logHtml}
    </div>`;
}

function statusLabel(status) {
  const map = {
    queued: 'Chờ',
    running: 'Đang chạy',
    paused: 'Tạm dừng',
    completed: 'Hoàn thành',
    failed: 'Lỗi',
    cancelled: 'Đã hủy',
  };
  return map[status] || status;
}

function formatDuration(start, stop) {
  if (!start) return '—';
  const end = stop || new Date();
  const sec = Math.floor((end - start) / 1000);
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `${h}g ${m}p ${s}s`;
  if (m > 0) return `${m}p ${s}s`;
  return `${s}s`;
}

function startProductionPoll() {
  stopProductionPoll();
  productionPollTimer = setInterval(() => {
    if (activeTab !== 'production') return;
    loadProductionData();
  }, 5000);
}

function stopProductionPoll() {
  if (productionPollTimer) {
    clearInterval(productionPollTimer);
    productionPollTimer = null;
  }
}

function loadProductionTab() {
  renderProductionTab();
}

function escapeHtml(s) {
  if (s == null) return '';
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
