// ainovel web — workspace tabs: Stream | Chương | Outline | World.
// Nạp sau app.js để dùng $, post, toast, currentState, lastSnapshot.
'use strict';

const TAB_KEY = 'ainovel-active-tab';
var activeTab = sessionStorage.getItem(TAB_KEY) || 'stream';
var selectedChapter = null;

function initWorkspace() {
  setupTabs();
  renderTabPanel(activeTab);
}

function setupTabs() {
  const bar = $('#workspaceTabs');
  if (!bar) return;
  bar.addEventListener('click', (e) => {
    const btn = e.target.closest('.tab');
    if (!btn) return;
    switchTab(btn.dataset.tab);
  });
  bar.addEventListener('keydown', (e) => {
    const tabs = [...bar.querySelectorAll('.tab')];
    const idx = tabs.findIndex((b) => b.dataset.tab === activeTab);
    if (e.key === 'ArrowRight') {
      e.preventDefault();
      const next = tabs[(idx + 1) % tabs.length];
      switchTab(next.dataset.tab);
      next.focus();
    } else if (e.key === 'ArrowLeft') {
      e.preventDefault();
      const prev = tabs[(idx - 1 + tabs.length) % tabs.length];
      switchTab(prev.dataset.tab);
      prev.focus();
    }
  });
}

function switchTab(name) {
  activeTab = name;
  sessionStorage.setItem(TAB_KEY, name);

  const bar = $('#workspaceTabs');
  bar.querySelectorAll('.tab').forEach((b) => {
    const on = b.dataset.tab === name;
    b.classList.toggle('active', on);
    b.setAttribute('aria-selected', on ? 'true' : 'false');
  });

  document.querySelectorAll('.tab-panel').forEach((p) => {
    p.classList.toggle('active', p.id === 'tab-' + name);
  });

  if (name === 'outline') loadOutlineTab();
  if (name === 'world') loadWorldTab();
  if (name === 'review') loadReviewTab();
}

function focusStreamTab() {
  switchTab('stream');
}

function clearContentCache() {
  outlineCache = null;
  worldCache = null;
  reviewCache = null;
}

function renderTabPanel(name) {
  switchTab(name);
}

// ── Chương ──

function selectChapter(n) {
  selectedChapter = n;
  highlightOutline(n);
  switchTab('chapter');
  loadChapter(n);
}

function highlightOutline(n) {
  document.querySelectorAll('#outlineList li').forEach((li) => {
    li.classList.toggle('selected', Number(li.dataset.chapter) === n);
  });
}

async function loadChapter(n) {
  const meta = $('#chapterMeta');
  const textEl = $('#chapterText');
  meta.textContent = `Chương ${n}`;
  textEl.innerHTML = '<div class="placeholder">Đang tải…</div>';

  try {
    const res = await fetch(`/api/chapters/${n}`);
    if (res.ok) {
      const data = await res.json();
      renderChapter(data, false);
      return;
    }
    if (res.status === 404) {
      const draftRes = await fetch(`/api/chapters/${n}/draft`);
      if (draftRes.ok) {
        const data = await draftRes.json();
        renderChapter(data, true);
        return;
      }
      if (draftRes.status === 404) {
        meta.textContent = '';
        textEl.innerHTML = '<div class="placeholder">Chương chưa có nội dung.</div>';
        return;
      }
      throw new Error('draft HTTP ' + draftRes.status);
    }
    throw new Error('HTTP ' + res.status);
  } catch (e) {
    meta.textContent = '';
    textEl.innerHTML = '<div class="placeholder">Lỗi tải chương.</div>';
    toast('Lỗi tải chương: ' + e, 'error');
  }
}

function renderChapter(data, isDraftFallback) {
  const meta = $('#chapterMeta');
  const textEl = $('#chapterText');
  const badge = isDraftFallback ? ' · Bản nháp' : '';
  meta.textContent = `Chương ${data.chapter}${badge}`;
  textEl.textContent = data.text || '';
}

// ── Outline ──

let outlineCache = null;
async function loadOutlineTab() {
  if (outlineCache) {
    renderOutlineDetail(outlineCache);
    return;
  }
  const detail = $('#outlineDetail');
  detail.innerHTML = '<div class="placeholder">Đang tải outline…</div>';
  try {
    const res = await fetch('/api/outline');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    outlineCache = await res.json();
    renderOutlineDetail(outlineCache);
  } catch (e) {
    $('#outlinePremise').innerHTML = '';
    detail.innerHTML = '<div class="placeholder">Lỗi tải outline.</div>';
    toast('Lỗi tải outline: ' + e, 'error');
  }
}

function renderOutlineDetail(data) {
  const premise = $('#outlinePremise');
  const detail = $('#outlineDetail');
  premise.textContent = data.premise || '';
  premise.hidden = !data.premise;

  if (!data.outline || !data.outline.length) {
    detail.innerHTML = '<div class="placeholder">Chưa có outline.</div>';
    return;
  }
  const ul = document.createElement('ul');
  ul.className = 'outline-detail-list';
  data.outline.forEach((e) => {
    const li = document.createElement('li');
    const num = document.createElement('span');
    num.className = 'outline-detail-num';
    num.textContent = e.Chapter;
    const title = document.createElement('span');
    title.className = 'outline-detail-title';
    title.textContent = e.Title || '(chưa có tiêu đề)';
    const evt = document.createElement('div');
    evt.className = 'outline-detail-event';
    evt.textContent = e.CoreEvent || '';
    li.append(num, title, evt);
    ul.appendChild(li);
  });
  detail.innerHTML = '';
  detail.appendChild(ul);
}

// ── World / Characters ──

let worldCache = null;
async function loadWorldTab() {
  if (worldCache) {
    renderWorldTab(worldCache);
    return;
  }
  const rulesEl = $('#worldRules');
  rulesEl.innerHTML = '<div class="placeholder">Đang tải…</div>';
  try {
    const [charsRes, worldRes, fsRes] = await Promise.all([
      fetch('/api/characters'),
      fetch('/api/world'),
      fetch('/api/foreshadow'),
    ]);
    if (!charsRes.ok || !worldRes.ok || !fsRes.ok) throw new Error('HTTP ' + charsRes.status + '/' + worldRes.status + '/' + fsRes.status);
    worldCache = {
      characters: await charsRes.json(),
      world: await worldRes.json(),
      foreshadow: await fsRes.json(),
    };
    renderWorldTab(worldCache);
  } catch (e) {
    $('#worldChars').innerHTML = '';
    $('#worldForeshadow').innerHTML = '';
    rulesEl.innerHTML = '<div class="placeholder">Lỗi tải world/characters.</div>';
    toast('Lỗi tải world/characters: ' + e, 'error');
  }
}

function renderWorldTab(data) {
  renderCharacters(data.characters);
  renderForeshadow(data.foreshadow);
  renderWorldRules(data.world);
}

// ── 伏笔 (Foreshadow) ──

const FORESHADOW_STATUS_LABEL = { planted: 'Đã cài', advanced: 'Đang tiến triển', resolved: 'Đã thu hồi' };
const FORESHADOW_STATUS_ICON = { planted: '🌱', advanced: '🔗', resolved: '✅' };

function renderForeshadow(data) {
  const container = $('#worldForeshadow');
  container.innerHTML = '';
  const h = document.createElement('h4');
  h.textContent = 'Cài cắm (伏笔)';
  container.appendChild(h);

  const entries = (data && data.entries) || [];
  if (!entries.length) {
    appendPlaceholder(container, 'Chưa cài cắm sợi truyện nào.');
    return;
  }
  const ul = document.createElement('ul');
  ul.className = 'foreshadow-list';
  entries.forEach((e) => {
    const li = document.createElement('li');
    li.className = 'foreshadow-item foreshadow-' + (e.status || 'planted');
    const icon = document.createElement('span');
    icon.className = 'foreshadow-icon';
    icon.textContent = FORESHADOW_STATUS_ICON[e.status] || '•';
    const body = document.createElement('div');
    body.className = 'foreshadow-body';
    const desc = document.createElement('div');
    desc.className = 'foreshadow-desc';
    desc.textContent = e.description || e.id || '(không có mô tả)';
    const meta = document.createElement('div');
    meta.className = 'foreshadow-meta';
    const statusLabel = FORESHADOW_STATUS_LABEL[e.status] || e.status || '';
    let metaText = `${statusLabel} · cài ở chương ${e.planted_at}`;
    if (e.status === 'resolved' && e.resolved_at) metaText += ` → thu hồi ở chương ${e.resolved_at}`;
    meta.textContent = metaText;
    body.append(desc, meta);
    li.append(icon, body);
    ul.appendChild(li);
  });
  container.appendChild(ul);
}

function renderCharacters(data) {
  const container = $('#worldChars');
  container.innerHTML = '';
  const h = document.createElement('h4');
  h.textContent = 'Nhân vật';
  container.appendChild(h);

  const main = data.characters || [];
  if (!main.length) {
    appendPlaceholder(container, 'Chưa có nhân vật.');
  } else {
    main.forEach((c) => {
      const details = document.createElement('details');
      details.className = 'char-item';
      const summary = document.createElement('summary');
      summary.textContent = c.Name || c.name || '?';
      details.appendChild(summary);
      const body = document.createElement('div');
      body.className = 'char-body';
      renderCharBody(body, c);
      details.appendChild(body);
      container.appendChild(details);
    });
  }

  const supp = (data.supporting || []).slice(0, 20);
  if (supp.length) {
    const sh = document.createElement('h4');
    sh.textContent = 'Supporting';
    container.appendChild(sh);
    const ul = document.createElement('ul');
    ul.className = 'supporting-list';
    supp.forEach((c) => {
      const li = document.createElement('li');
      li.textContent = c.Name || c.name || String(c);
      ul.appendChild(li);
    });
    container.appendChild(ul);
  }
}

function appendPlaceholder(container, text) {
  const div = document.createElement('div');
  div.className = 'placeholder';
  div.textContent = text;
  container.appendChild(div);
}

function renderCharBody(container, c) {
  const dl = document.createElement('dl');
  dl.className = 'char-attrs';
  Object.entries(c).forEach(([k, v]) => {
    const dt = document.createElement('dt');
    dt.textContent = k;
    const dd = document.createElement('dd');
    if (v === null || v === undefined) {
      dd.textContent = '—';
    } else if (typeof v === 'object') {
      dd.textContent = JSON.stringify(v, null, 2);
    } else {
      dd.textContent = String(v);
    }
    dl.appendChild(dt);
    dl.appendChild(dd);
  });
  container.appendChild(dl);
}

function renderWorldRules(data) {
  const container = $('#worldRules');
  container.innerHTML = '';

  const h = document.createElement('h4');
  h.textContent = 'World rules';
  container.appendChild(h);
  const rules = data.rules || [];
  if (!rules.length) {
    appendPlaceholder(container, 'Chưa có world rules.');
  } else {
    const ul = document.createElement('ul');
    ul.className = 'world-rule-list';
    rules.forEach((r) => {
      const li = document.createElement('li');
      li.textContent = r.Rule || r.rule || r.Name || r.name || JSON.stringify(r);
      ul.appendChild(li);
    });
    container.appendChild(ul);
  }

  if (data.compass) {
    const ch = document.createElement('h4');
    ch.textContent = 'Compass';
    container.appendChild(ch);
    const pre = document.createElement('pre');
    pre.className = 'compass-block';
    pre.textContent = JSON.stringify(data.compass, null, 2);
    container.appendChild(pre);
  }

  if (data.timeline && data.timeline.length) {
    const th = document.createElement('h4');
    th.textContent = 'Timeline';
    container.appendChild(th);
    const ul = document.createElement('ul');
    ul.className = 'timeline-list';
    data.timeline.forEach((ev) => {
      const li = document.createElement('li');
      li.textContent = ev.Event || ev.event || JSON.stringify(ev);
      ul.appendChild(li);
    });
    container.appendChild(ul);
  }
}

// ── Đánh giá (Editor review) ──

const DIMENSION_LABEL = {
  consistency: 'Nhất quán thiết lập',
  character: 'Hành vi nhân vật',
  pacing: 'Nhịp truyện',
  continuity: 'Mạch tự sự',
  foreshadow: 'Cài cắm (伏笔)',
  hook: 'Móc câu (钩子)',
  aesthetic: 'Thẩm mỹ',
};
const VERDICT_LABEL = { accept: 'Chấp nhận', polish: 'Cần mài giũa', rewrite: 'Phải viết lại' };
const SEVERITY_LABEL = { critical: 'Nghiêm trọng', error: 'Lỗi', warning: 'Cảnh báo' };

let reviewCache = null;
async function loadReviewTab() {
  if (reviewCache) {
    renderReviewTab(reviewCache);
    return;
  }
  const list = $('#reviewList');
  list.innerHTML = '<div class="placeholder">Đang tải đánh giá…</div>';
  try {
    const res = await fetch('/api/reviews');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    reviewCache = await res.json();
    renderReviewTab(reviewCache);
  } catch (e) {
    list.innerHTML = '<div class="placeholder">Lỗi tải đánh giá.</div>';
    toast('Lỗi tải đánh giá: ' + e, 'error');
  }
}

function renderReviewTab(data) {
  const list = $('#reviewList');
  list.innerHTML = '';

  const reviews = (data && data.reviews) || [];
  const global = data && data.global;

  if (global) {
    list.appendChild(renderReviewCard(global, true));
  }
  if (!reviews.length) {
    if (!global) appendPlaceholder(list, 'Editor chưa đánh giá chương nào.');
    return;
  }
  // mới nhất lên trước — dễ xem review gần đây nhất
  reviews.slice().reverse().forEach((rv) => {
    list.appendChild(renderReviewCard(rv, false));
  });
}

function renderReviewCard(rv, isGlobal) {
  const card = document.createElement('div');
  card.className = 'review-card review-verdict-' + (rv.verdict || 'accept');

  const head = document.createElement('div');
  head.className = 'review-head';
  const title = document.createElement('span');
  title.className = 'review-title';
  title.textContent = isGlobal ? `Đánh giá vòng cung (đến chương ${rv.chapter})` : `Chương ${rv.chapter}`;
  const verdict = document.createElement('span');
  verdict.className = 'review-badge';
  verdict.textContent = VERDICT_LABEL[rv.verdict] || rv.verdict || '';
  head.append(title, verdict);
  card.appendChild(head);

  if (rv.summary) {
    const summary = document.createElement('div');
    summary.className = 'review-summary';
    summary.textContent = rv.summary;
    card.appendChild(summary);
  }

  const dims = rv.dimensions || [];
  if (dims.length) {
    const dimList = document.createElement('div');
    dimList.className = 'review-dims';
    dims.forEach((d) => {
      const row = document.createElement('div');
      row.className = 'review-dim review-dim-' + (d.verdict || 'pass');
      const name = document.createElement('span');
      name.className = 'review-dim-name';
      name.textContent = DIMENSION_LABEL[d.dimension] || d.dimension;
      const score = document.createElement('span');
      score.className = 'review-dim-score';
      score.textContent = (d.score != null ? d.score : '—') + '/100';
      row.append(name, score);
      if (d.comment) {
        const comment = document.createElement('span');
        comment.className = 'review-dim-comment';
        comment.textContent = d.comment;
        row.appendChild(comment);
      }
      dimList.appendChild(row);
    });
    card.appendChild(dimList);
  }

  if (rv.contract_status) {
    const contract = document.createElement('div');
    contract.className = 'review-contract';
    let text = `Hợp đồng chương: ${rv.contract_status}`;
    if (rv.contract_notes) text += ` — ${rv.contract_notes}`;
    contract.textContent = text;
    card.appendChild(contract);
    if ((rv.contract_misses || []).length) {
      const misses = document.createElement('ul');
      misses.className = 'review-contract-misses';
      rv.contract_misses.forEach((m) => {
        const li = document.createElement('li');
        li.textContent = m;
        misses.appendChild(li);
      });
      card.appendChild(misses);
    }
  }

  const issues = rv.issues || [];
  if (issues.length) {
    const issueList = document.createElement('ul');
    issueList.className = 'review-issues';
    issues.forEach((iss) => {
      const li = document.createElement('li');
      li.className = 'review-issue review-issue-' + (iss.severity || 'warning');
      const head2 = document.createElement('div');
      head2.className = 'review-issue-head';
      const sev = document.createElement('span');
      sev.className = 'review-issue-sev';
      sev.textContent = SEVERITY_LABEL[iss.severity] || iss.severity || '';
      const type = document.createElement('span');
      type.className = 'review-issue-type';
      type.textContent = DIMENSION_LABEL[iss.type] || iss.type || '';
      head2.append(sev, type);
      const desc = document.createElement('div');
      desc.className = 'review-issue-desc';
      desc.textContent = iss.description || '';
      li.append(head2, desc);
      if (iss.evidence) {
        const ev = document.createElement('blockquote');
        ev.className = 'review-issue-evidence';
        ev.textContent = iss.evidence;
        li.appendChild(ev);
      }
      if (iss.suggestion) {
        const sug = document.createElement('div');
        sug.className = 'review-issue-suggestion';
        sug.textContent = 'Gợi ý: ' + iss.suggestion;
        li.appendChild(sug);
      }
      issueList.appendChild(li);
    });
    card.appendChild(issueList);
  }

  if ((rv.affected_chapters || []).length) {
    const affected = document.createElement('div');
    affected.className = 'review-affected';
    affected.textContent = 'Chương liên quan: ' + rv.affected_chapters.join(', ');
    card.appendChild(affected);
  }

  return card;
}

initWorkspace();
