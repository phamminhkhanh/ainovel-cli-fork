// ainovel web - Production Cockpit tab (San xuat).
// Nap sau app.js de dung $, post, toast.
'use strict';

let productionSelectedRunId = null;
let productionPollTimer = null;
let productionRunsCache = [];
let productionProfilesCache = [];
let productionCreateMode = 'fresh_profile';
let productionWorkspaceSnapshot = null;

function renderProductionTab() {
  const panel = $('#tab-production');
  if (!panel) return;
  if (!panel.dataset.initialized) {
    panel.innerHTML = `
      <div class="production-layout">
        <aside class="run-list">
          <div class="run-list-head">
            <h3>Danh s\u00e1ch job</h3>
            <div class="field-row compact-actions">
              <button class="btn sm primary" id="btnNewFreshRun">+ T\u1ea1o truy\u1ec7n m\u1edbi</button>
              <button class="btn sm" id="btnNewContinueRun">\u21bb Cook ti\u1ebfp</button>
            </div>
          </div>
          <ul class="run-list-items" id="runListItems"><li class="muted">\u0110ang t\u1ea3i\u2026</li></ul>
        </aside>
        <main class="run-detail" id="runDetail">
          <div class="placeholder">Ch\u1ecdn m\u1ed9t job b\u00ean tr\u00e1i \u0111\u1ec3 xem chi ti\u1ebft.</div>
        </main>
      </div>
      <div class="modal-overlay" id="newRunOverlay" hidden>
        <div class="modal" role="dialog" aria-modal="true" aria-labelledby="newRunTitle">
          <header class="modal-head"><h2 id="newRunTitle">T\u1ea1o job s\u1ea3n xu\u1ea5t</h2></header>
          <div class="modal-body">
            <div class="field"><label for="newRunName">T\u00ean job</label><input type="text" id="newRunName" placeholder="vd: Werewolf romantasy 100 ch\u01b0\u01a1ng"></div>
            <div class="field" id="newRunProfileField"><label for="newRunProfile">Profile t\u1ea1o truy\u1ec7n</label><select id="newRunProfile"><option value="">\u0110ang t\u1ea3i\u2026</option></select><small class="muted">File .md trong ./.ainovel/profiles/ ho\u1eb7c ~/.ainovel/profiles/.</small></div>
            <div class="field-row">
              <div class="field"><label for="newRunModel">Model override (t\u00f9y ch\u1ecdn)</label><input type="text" id="newRunModel" placeholder="vd: gpt-4o"></div>
              <div class="field"><label for="newRunProvider">Provider override (t\u00f9y ch\u1ecdn)</label><input type="text" id="newRunProvider" placeholder="vd: openai"></div>
            </div>
            <div class="field-row">
              <div class="field"><label for="newRunTarget" id="newRunTargetLabel">S\u1ed1 ch\u01b0\u01a1ng m\u1ee5c ti\u00eau</label><input type="number" id="newRunTarget" min="1" value="30"><small class="muted" id="newRunTargetHelp"></small></div>
              <div class="field"><label for="newRunBudget">Ng\u00e2n s\u00e1ch (USD)</label><input type="number" id="newRunBudget" min="0.1" step="0.1" value="5"></div>
            </div>
            <p class="muted" id="newRunModeHelp"></p>
          </div>
          <footer class="modal-foot">
            <button class="btn" id="newRunCancel">H\u1ee7y</button>
            <button class="btn primary" id="newRunSubmit">T\u1ea1o job</button>
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
  $('#btnNewFreshRun')?.addEventListener('click', () => openNewRunModal('fresh_profile'));
  $('#btnNewContinueRun')?.addEventListener('click', () => openNewRunModal('continue_workspace'));
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

async function openNewRunModal(mode) {
  productionCreateMode = mode || 'fresh_profile';
  productionWorkspaceSnapshot = await loadProductionSnapshot();
  if (productionCreateMode === 'continue_workspace') {
    const guard = continueWorkspaceGuard(productionWorkspaceSnapshot);
    if (guard) { toast(guard, 'error'); return; }
  } else {
    await populateProfileSelect();
  }
  configureNewRunModal();
  $('#newRunOverlay').hidden = false;
  $('#newRunName').focus();
}
function closeNewRunModal() { $('#newRunOverlay').hidden = true; }

async function loadProductionSnapshot() {
  try {
    if (lastSnapshot) return lastSnapshot;
    const res = await fetch('/api/snapshot');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    return await res.json();
  } catch (e) {
    return null;
  }
}

function continueWorkspaceGuard(snap) {
  if (!snap) return 'Kh\u00f4ng \u0111\u1ecdc \u0111\u01b0\u1ee3c workspace hi\u1ec7n t\u1ea1i.';
  if (snap.IsRunning || snap.RuntimeState === 'running' || snap.RuntimeState === 'pausing') return 'D\u1eebng phi\u00ean hi\u1ec7n t\u1ea1i tr\u01b0\u1edbc khi seed job cook ti\u1ebfp.';
  if ((snap.CompletedCount || 0) <= 0) return 'Workspace hi\u1ec7n t\u1ea1i ch\u01b0a c\u00f3 truy\u1ec7n \u0111\u1ec3 cook ti\u1ebfp.';
  if (snap.Phase === 'complete' || snap.RuntimeState === 'completed') return 'S\u00e1ch \u0111\u00e3 ho\u00e0n th\u00e0nh; d\u00f9ng T\u1ea1o truy\u1ec7n m\u1edbi n\u1ebfu mu\u1ed1n vi\u1ebft cu\u1ed1n kh\u00e1c.';
  return '';
}

function configureNewRunModal() {
  const isContinue = productionCreateMode === 'continue_workspace';
  $('#newRunTitle').textContent = isContinue ? 'Cook ti\u1ebfp workspace hi\u1ec7n t\u1ea1i' : 'T\u1ea1o truy\u1ec7n m\u1edbi t\u1eeb profile';
  $('#newRunProfileField').hidden = isContinue;
  $('#newRunTargetLabel').textContent = isContinue ? 'T\u1ed5ng s\u1ed1 ch\u01b0\u01a1ng mu\u1ed1n \u0111\u1ea1t t\u1edbi' : 'S\u1ed1 ch\u01b0\u01a1ng m\u1ee5c ti\u00eau';
  $('#newRunTargetHelp').textContent = isContinue
    ? 'M\u1ee5c ti\u00eau l\u00e0 t\u1ed5ng s\u1ed1 ch\u01b0\u01a1ng cu\u1ed1i c\u00f9ng, kh\u00f4ng ph\u1ea3i s\u1ed1 ch\u01b0\u01a1ng vi\u1ebft th\u00eam. V\u00ed d\u1ee5 \u0111ang 12 ch\u01b0\u01a1ng, mu\u1ed1n t\u1edbi 100 th\u00ec nh\u1eadp 100.'
    : '';
  $('#newRunModeHelp').textContent = isContinue
    ? 'Job s\u1ebd ch\u1ea1y trong sandbox ri\u00eang. Workspace ch\u00ednh ch\u1ec9 thay \u0111\u1ed5i khi b\u1ea1n b\u1ea5m \u0110\u1ed3ng b\u1ed9.'
    : 'Job m\u1edbi sinh truy\u1ec7n t\u1eeb profile \u0111\u00e3 ch\u1ecdn trong sandbox ri\u00eang.';
  const done = productionWorkspaceSnapshot?.CompletedCount || 0;
  if (isContinue && done > 0) {
    $('#newRunTarget').value = String(Math.max(done + 1, productionWorkspaceSnapshot.TotalChapters || done + 20));
    $('#newRunName').placeholder = `Cook ti\u1ebfp t\u1eeb ${done} ch\u01b0\u01a1ng`;
  } else {
    $('#newRunName').placeholder = 'vd: Werewolf romantasy 100 ch\u01b0\u01a1ng';
  }
}

function productionProfileLabel(profile) {
  const source = profile.source === 'project' ? 'D\u1ef1 \u00e1n' : profile.source === 'global' ? 'Global' : profile.source === 'legacy' ? 'Legacy' : 'Profile';
  return `${source} \u00b7 ${profile.name || profile.path || ''}`;
}

function renderProductionProfileOptions(profiles) {
  if (!profiles.length) return '<option value="">Ch\u01b0a c\u00f3 profile (.md)</option>';
  return profiles.map((p) => `<option value="${escapeHtml(p.path)}">${escapeHtml(productionProfileLabel(p))}</option>`).join('');
}

async function populateProfileSelect() {
  const sel = $('#newRunProfile');
  if (!sel || productionProfilesCache.length) {
    if (sel) sel.innerHTML = renderProductionProfileOptions(productionProfilesCache);
    return;
  }
  try {
    const res = await fetch('/api/profiles');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    productionProfilesCache = await res.json();
    sel.innerHTML = renderProductionProfileOptions(productionProfilesCache);
  } catch (e) {
    sel.innerHTML = '<option value="">L\u1ed7i t\u1ea3i profile</option>';
    toast('L\u1ed7i t\u1ea3i profile: ' + e, 'error');
  }
}

async function submitNewRun() {
  const isContinue = productionCreateMode === 'continue_workspace';
  const body = {
    kind: productionCreateMode,
    name: $('#newRunName').value.trim(),
    profile: isContinue ? undefined : $('#newRunProfile').value,
    model: $('#newRunModel').value.trim() || undefined,
    provider: $('#newRunProvider').value.trim() || undefined,
    targetChapters: parseInt($('#newRunTarget').value, 10) || 30,
    budgetUsd: parseFloat($('#newRunBudget').value) || 5,
  };
  if (!body.name) { toast('Nh\u1eadp t\u00ean job', 'error'); return; }
  if (!isContinue && !body.profile) { toast('Ch\u1ecdn profile', 'error'); return; }
  if (isContinue) {
    const done = productionWorkspaceSnapshot?.CompletedCount || 0;
    if (body.targetChapters <= done) { toast(`T\u1ed5ng m\u1ee5c ti\u00eau ph\u1ea3i l\u1edbn h\u01a1n ${done} ch\u01b0\u01a1ng hi\u1ec7n c\u00f3`, 'error'); return; }
  }
  const res = await post('/api/prodruns', body);
  if (res) {
    closeNewRunModal();
    clearNewRunForm();
    productionSelectedRunId = res.id;
    await loadProductionData();
    toast(isContinue ? '\u0110\u00e3 t\u1ea1o job cook ti\u1ebfp' : '\u0110\u00e3 t\u1ea1o job', 'ok');
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
    toast('L\u1ed7i t\u1ea3i danh s\u00e1ch job: ' + e, 'error');
  }
}

function selectProductionRun(id) {
  productionSelectedRunId = id;
  renderProductionRuns();
  const run = productionRunsCache.find((r) => r.id === id);
  renderProductionDetail(run || null);
}

async function startProductionRun(id) {
  const run = productionRunsCache.find((r) => r.id === id);
  if (run?.kind === 'continue_workspace') {
    const guard = continueWorkspaceGuard(await loadProductionSnapshot());
    if (guard) { toast(guard, 'error'); return; }
  }
  const res = await post(`/api/prodruns/${id}/start`, {});
  if (res) {
    toast('\u0110\u00e3 b\u1eaft \u0111\u1ea7u job', 'ok');
    await loadProductionData();
  }
}

async function stopProductionRun(id) {
  if (!confirm('D\u1eebng job n\u00e0y? Ti\u1ebfn tr\u00ecnh con s\u1ebd b\u1ecb kill ngay l\u1eadp t\u1ee9c.')) return;
  const res = await post(`/api/prodruns/${id}/stop`, {});
  if (res) {
    toast('\u0110\u00e3 d\u1eebng job', 'ok');
    await loadProductionData();
  }
}

async function deleteProductionRun(id) {
  if (!confirm('X\u00f3a job n\u00e0y? Th\u01b0 m\u1ee5c run v\u00e0 log s\u1ebd b\u1ecb x\u00f3a.')) return;
  try {
    const r = await fetch(`/api/prodruns/${id}`, { method: 'DELETE' });
    if (!r.ok) {
      const data = await r.json().catch(() => ({}));
      toast(data.error || ('HTTP ' + r.status), 'error');
      return;
    }
    if (productionSelectedRunId === id) productionSelectedRunId = null;
    toast('\u0110\u00e3 x\u00f3a job', 'ok');
    await loadProductionData();
  } catch (e) {
    toast('L\u1ed7i x\u00f3a job: ' + e, 'error');
  }
}

async function exportProductionRun(id) {
  const res = await post(`/api/prodruns/${id}/export`, { format: 'txt' });
  if (!res) return;
  const a = document.createElement('a');
  a.href = `/api/prodruns/${id}/export.txt`;
  a.download = '';
  document.body.appendChild(a);
  a.click();
  a.remove();
  toast('\u0110\u00e3 xu\u1ea5t TXT', 'ok');
}

async function syncProductionRun(id, force) {
  const run = productionRunsCache.find((r) => r.id === id);
  const isContinue = run?.kind === 'continue_workspace';
  const msg = force
    ? (isContinue ? 'Workspace ch\u00ednh \u0111\u00e3 thay \u0111\u1ed5i sau khi job \u0111\u01b0\u1ee3c seed. \u0110\u1ed3ng b\u1ed9 s\u1ebd ghi \u0111\u00e8 thay \u0111\u1ed5i hi\u1ec7n t\u1ea1i. Ti\u1ebfp t\u1ee5c?' : 'Workspace ch\u00ednh \u0111\u00e3 c\u00f3 d\u1eef li\u1ec7u. Ghi \u0111\u00e8 to\u00e0n b\u1ed9 b\u1eb1ng k\u1ebft qu\u1ea3 job n\u00e0y?')
    : (isContinue ? 'Fast-forward k\u1ebft qu\u1ea3 job v\u00e0o workspace ch\u00ednh?' : '\u0110\u1ed3ng b\u1ed9 k\u1ebft qu\u1ea3 job v\u00e0o workspace ch\u00ednh? D\u1eef li\u1ec7u workspace hi\u1ec7n t\u1ea1i ph\u1ea3i tr\u1ed1ng.');
  if (!confirm(msg)) return;
  try {
    const r = await fetch(`/api/prodruns/${id}/sync`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ force: !!force }),
    });
    if (!r.ok) {
      const data = await r.json().catch(() => ({}));
      const err = data.error || '';
      if (r.status === 409 && !force && (err.includes('already has progress') || err.includes('changed since this continue run was seeded'))) {
        syncProductionRun(id, true);
        return;
      }
      toast(err || ('HTTP ' + r.status), 'error');
      return;
    }
    const data = await r.json().catch(() => ({}));
    const mode = data.fastForward ? 'fast-forward ' : '';
    toast(`\u0110\u00e3 ${mode}\u0111\u1ed3ng b\u1ed9 ${data.copiedFiles || 0} t\u1ec7p v\u00e0o workspace`, 'ok');
    await loadProductionData();
    try {
      const snap = await fetch('/api/snapshot').then((r) => r.json());
      if (typeof renderSnapshot === 'function') renderSnapshot(snap);
    } catch (e) {
      // SSE se tu cap nhat sau.
    }
  } catch (e) {
    toast('L\u1ed7i \u0111\u1ed3ng b\u1ed9: ' + e, 'error');
  }
}

function productionRunKindLabel(run) {
  return run.kind === 'continue_workspace' ? 'Cook ti\u1ebfp workspace' : 'Truy\u1ec7n m\u1edbi t\u1eeb profile';
}

function renderProductionRuns() {
  const ul = $('#runListItems');
  if (!ul) return;
  if (!productionRunsCache.length) {
    ul.innerHTML = '<li class="muted">Ch\u01b0a c\u00f3 job n\u00e0o.</li>';
    return;
  }
  ul.innerHTML = productionRunsCache.map((r) => {
    const selected = r.id === productionSelectedRunId ? ' selected' : '';
    return `<li class="run-item${selected}" data-run-id="${escapeHtml(r.id)}" tabindex="0">
      <div class="run-item-head">
        <span class="run-item-name">${escapeHtml(r.name || r.id)}</span>
        <span class="run-badge run-badge-${r.status}">${escapeHtml(statusLabel(r.status))}</span>
      </div>
      <div class="run-item-meta">${escapeHtml(productionRunKindLabel(r))} \u00b7 ${r.chapters || 0}/${r.targetChapters} ch\u01b0\u01a1ng \u00b7 $${(r.costUsd || 0).toFixed(2)}</div>
    </li>`;
  }).join('');
}

async function renderProductionDetail(run) {
  const detail = $('#runDetail');
  if (!detail) return;
  if (!run) {
    detail.innerHTML = '<div class="placeholder">Ch\u1ecdn m\u1ed9t job b\u00ean tr\u00e1i \u0111\u1ec3 xem chi ti\u1ebft.</div>';
    return;
  }

  const runtime = formatDuration(run.startedAt ? new Date(run.startedAt) : null, run.stoppedAt ? new Date(run.stoppedAt) : null);
  const progress = run.targetChapters > 0 ? Math.min(100, Math.round((run.chapters || 0) / run.targetChapters * 100)) : 0;
  let logHtml = '';
  try {
    const res = await fetch(`/api/prodruns/${run.id}/log?lines=50`);
    const text = res.ok ? await res.text() : '';
    logHtml = text ? `<pre class="run-log">${escapeHtml(text)}</pre>` : '<p class="muted">Ch\u01b0a c\u00f3 log.</p>';
  } catch (e) {
    logHtml = '<p class="muted">L\u1ed7i t\u1ea3i log.</p>';
  }

  const pauseNotice = run.status === 'paused'
    ? '<div class=\"run-pause-notice\">T\u1ea1m d\u1eebng \u2014 ch\u1ec9 c\u00f3 th\u1ec3 D\u1eebng ho\u1eb7c xu\u1ea5t file.</div>'
    : '';
  const seedHtml = run.seededFrom
    ? `<div class="stat"><span class="stat-label">Seed t\u1eeb workspace</span><span class="stat-value">${run.seededFrom.completedChapters || 0} ch\u01b0\u01a1ng</span></div>`
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
        <div class="stat"><span class="stat-label">Ki\u1ec3u job</span><span class="stat-value">${escapeHtml(productionRunKindLabel(run))}</span></div>
        ${seedHtml}
        <div class="stat"><span class="stat-label">Ch\u01b0\u01a1ng</span><span class="stat-value">${run.chapters || 0} / ${run.targetChapters}</span></div>
        <div class="stat"><span class="stat-label">\u0110\u00e1nh gi\u00e1</span><span class="stat-value">${run.reviews || 0}</span></div>
        <div class="stat"><span class="stat-label">Vi\u1ebft l\u1ea1i</span><span class="stat-value">${run.rewrites || 0}</span></div>
        <div class="stat"><span class="stat-label">Chi ph\u00ed</span><span class="stat-value">$${(run.costUsd || 0).toFixed(2)} / $${(run.budgetUsd || 0).toFixed(2)}</span></div>
        <div class="stat"><span class="stat-label">Th\u1eddi gian</span><span class="stat-value">${runtime}</span></div>
        <div class="stat"><span class="stat-label">L\u00fd do d\u1eebng</span><span class="stat-value">${escapeHtml(run.stopReason || '\u2014')}</span></div>
      </div>
      <div class="progress-bar"><div class="progress-fill" style="width:${progress}%"></div></div>
      <div class="run-detail-actions">
        <button class="btn primary" data-action="start" data-run-id="${escapeHtml(run.id)}" ${!canStart ? 'disabled' : ''}>\u25b6 B\u1eaft \u0111\u1ea7u</button>
        <button class="btn danger" data-action="stop" data-run-id="${escapeHtml(run.id)}" ${!canStop ? 'disabled' : ''}>\u25a0 D\u1eebng</button>
        <button class="btn" data-action="export" data-run-id="${escapeHtml(run.id)}" ${!canExport ? 'disabled' : ''}>\u2b07 Xu\u1ea5t TXT</button>
        <button class="btn" data-action="sync" data-run-id="${escapeHtml(run.id)}" ${!canSync ? 'disabled' : ''}>\ud83d\udd04 \u0110\u1ed3ng b\u1ed9</button>
        <button class="btn danger" data-action="delete" data-run-id="${escapeHtml(run.id)}" ${!canDelete ? 'disabled' : ''}>\ud83d\uddd1 X\u00f3a</button>
      </div>
      ${run.possiblyOrphaned ? '<div class="run-orphan-warning">\u26a0 Job n\u00e0y c\u00f3 th\u1ec3 c\u00f2n ti\u1ebfn tr\u00ecnh con m\u1ed3 c\u00f4i. Ki\u1ec3m tra PID ' + (run.childPid || '\u2014') + '.</div>' : ''}
      <h4>Nh\u1eadt k\u00fd</h4>
      ${logHtml}
    </div>`;
}

function statusLabel(status) {
  const map = {
    queued: 'Ch\u1edd',
    running: '\u0110ang ch\u1ea1y',
    paused: 'T\u1ea1m d\u1eebng',
    completed: 'Ho\u00e0n th\u00e0nh',
    failed: 'L\u1ed7i',
    cancelled: '\u0110\u00e3 h\u1ee7y',
  };
  return map[status] || status;
}

function formatDuration(start, stop) {
  if (!start) return '\u2014';
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
