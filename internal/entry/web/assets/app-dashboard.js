// ainovel web — dashboard rendering từ Host.UISnapshot.
// Tách ra để giữ app.js dưới ngưỡng ~600 dòng; nạp sau app.js nên dùng chung $.
'use strict';

// ── Outline ──
function renderOutline(s) {
  const card = $('#outlineCard');
  const list = $('#outlineList');
  const meta = $('#outlineMeta');
  const outline = s.Outline || [];
  if (!outline.length) { card.hidden = true; return; }
  card.hidden = false;

  list.innerHTML = '';
  const done = s.CompletedCount || 0;
  const inProgress = s.InProgressChapter || 0;
  outline.forEach((e) => {
    const li = document.createElement('li');
    li.className = 'outline-item';
    li.dataset.chapter = e.Chapter;
    li.style.cursor = 'pointer';
    li.title = 'Click để đọc chương';
    li.addEventListener('click', () => { if (typeof selectChapter === 'function') selectChapter(e.Chapter); });
    const marker = document.createElement('span');
    marker.className = 'outline-marker';
    const title = document.createElement('span');
    title.className = 'outline-title';
    title.textContent = `${e.Chapter}. ${e.Title || ''}`;
    if (done >= e.Chapter) {
      marker.classList.add('marker-done');
      marker.textContent = '●';
    } else if (inProgress === e.Chapter) {
      marker.classList.add('marker-active');
      title.classList.add('active');
      marker.textContent = '▸';
    } else {
      marker.classList.add('marker-todo');
      marker.textContent = '○';
    }
    li.append(marker, title);
    list.appendChild(li);
  });
  if (typeof highlightOutline === 'function') highlightOutline(typeof selectedChapter !== 'undefined' ? selectedChapter : null);

  const parts = [];
  if (s.Layered && s.CurrentVolumeArc) parts.push(`Arc: ${s.CurrentVolumeArc}`);
  if (s.NextVolumeTitle) parts.push(`Next: ${s.NextVolumeTitle}`);
  if (s.CompassDirection) {
    let d = `→ ${s.CompassDirection}`;
    if (s.CompassScale) d += ` (${s.CompassScale})`;
    parts.push(d);
  }
  if (parts.length) {
    meta.innerHTML = '';
    parts.forEach((p) => {
      const span = document.createElement('span');
      span.className = 'outline-meta-item';
      span.textContent = p;
      meta.appendChild(span);
    });
    meta.hidden = false;
  } else {
    meta.hidden = true;
  }
}

// ── Rewrite queue / Pending steer ──
function renderRuntimeState(s) {
  const rewriteCard = $('#rewriteCard');
  const rewriteBody = $('#rewriteBody');
  const rewrites = s.PendingRewrites || [];
  if (rewrites.length) {
    rewriteCard.hidden = false;
    rewriteBody.innerHTML = '';
    const q = document.createElement('div');
    q.className = 'rewrite-queue';
    q.textContent = `Chương: ${rewrites.join(', ')}`;
    rewriteBody.appendChild(q);
    if (s.RewriteReason) {
      const r = document.createElement('div');
      r.className = 'rewrite-reason';
      r.textContent = `Lý do: ${s.RewriteReason}`;
      rewriteBody.appendChild(r);
    }
  } else {
    rewriteCard.hidden = true;
  }

  const steerCard = $('#steerCard');
  const steerText = $('#steerText');
  if (s.PendingSteer) {
    steerCard.hidden = false;
    steerText.textContent = s.PendingSteer;
  } else {
    steerCard.hidden = true;
  }
}

// ── Usage breakdown ──
function renderUsageBreakdown(s) {
  const container = $('#usageBreakdown');
  const rows = [];
  if (s.TotalInputTokens != null && s.TotalInputTokens > 0) rows.push(['Input tokens', s.TotalInputTokens.toLocaleString()]);
  if (s.TotalOutputTokens != null && s.TotalOutputTokens > 0) rows.push(['Output tokens', s.TotalOutputTokens.toLocaleString()]);
  if (s.TotalCacheReadTokens != null && s.TotalCacheReadTokens > 0) rows.push(['Cache read', s.TotalCacheReadTokens.toLocaleString()]);
  if (s.TotalCacheWriteTokens != null && s.TotalCacheWriteTokens > 0) rows.push(['Cache write', s.TotalCacheWriteTokens.toLocaleString()]);
  if (s.TotalSavedUSD != null && s.TotalSavedUSD > 0) rows.push(['Saved', '$' + Number(s.TotalSavedUSD).toFixed(4)]);
  if (s.BudgetLimitUSD != null && s.BudgetLimitUSD > 0) rows.push(['Budget', '$' + Number(s.BudgetLimitUSD).toFixed(2)]);

  if (!rows.length) { container.innerHTML = ''; return; }
  container.innerHTML = '';
  const dl = document.createElement('dl');
  dl.className = 'kv';
  rows.forEach(([dt, dd]) => {
    const dTerm = document.createElement('dt');
    dTerm.textContent = dt;
    const dDesc = document.createElement('dd');
    dDesc.textContent = dd;
    dl.append(dTerm, dDesc);
  });
  container.appendChild(dl);
}

// ── Cache sidebar ──
function renderCache(s) {
  const card = $('#cacheCard');
  const body = $('#cacheBody');
  const stats = [];
  const recentInput = s.OverallRecentInput || 0;
  const recentRead = s.OverallRecentCacheRead || 0;
  const hasUsage =
    (s.TotalInputTokens || 0) > 0 ||
    (s.TotalCacheReadTokens || 0) > 0 ||
    (s.TotalCacheWriteTokens || 0) > 0 ||
    (s.MissingAssistantUsage || 0) > 0;
  if (recentInput > 0) {
    const rate = recentRead / recentInput;
    stats.push(['Recent hit rate', `${(rate * 100).toFixed(1)}%`]);
  } else if (s.OverallCacheCapable) {
    stats.push(['Recent hit rate', '0%']);
  } else if (hasUsage) {
    stats.push(['Cache', '未启用 · model/provider không hỗ trợ']);
  }
  if (s.MissingAssistantUsage != null && s.MissingAssistantUsage > 0) {
    stats.push(['Missing usage', s.MissingAssistantUsage.toString()]);
  }

  const perAgent = (s.CachePerAgent || []).slice(0, 5);
  const perModel = (s.CachePerModel || []).slice(0, 5);

  if (!stats.length && !perAgent.length && !perModel.length) { card.hidden = true; return; }
  card.hidden = false;
  body.innerHTML = '';

  if (stats.length) {
    const dl = document.createElement('dl');
    dl.className = 'kv';
    stats.forEach(([dt, dd]) => {
      const t = document.createElement('dt');
      t.textContent = dt;
      const d = document.createElement('dd');
      d.textContent = dd;
      dl.append(t, d);
    });
    body.appendChild(dl);
  }

  function listSection(title, items, keyField) {
    if (!items.length) return;
    const h = document.createElement('h4');
    h.className = 'cache-subtitle';
    h.textContent = title;
    body.appendChild(h);
    const ul = document.createElement('ul');
    ul.className = 'cache-list';
    items.forEach((it) => {
      const input = it.Input || 0;
      const read = it.CacheRead || 0;
      const rate = input > 0 ? (read / input * 100).toFixed(1) : '0.0';
      const li = document.createElement('li');
      const name = document.createElement('span');
      name.className = 'cache-name';
      name.textContent = it[keyField] || '?';
      const hit = document.createElement('span');
      hit.className = 'cache-hit';
      hit.textContent = `${rate}%`;
      li.append(name, hit);
      ul.appendChild(li);
    });
    body.appendChild(ul);
  }

  listSection('Per agent', perAgent, 'Role');
  listSection('Per model', perModel, 'Model');
}

// ── Entry point (được app.js renderSnapshot gọi) ──
function renderDashboard(s) {
  renderOutline(s);
  renderRuntimeState(s);
  renderUsageBreakdown(s);
  renderCache(s);
}
