// ainovel web — chapter list + action buttons (sidebar upgrade).
// Nạp sau app-workspace.js (cần selectChapter), trước app-dashboard.js.
'use strict';

// ── Chapter list ──

function chapterStatus(n, snap) {
  var rewrites = snap.PendingRewrites || [];
  for (var i = 0; i < rewrites.length; i++) { if (rewrites[i] === n) return 'rewrite'; }
  if (n === (snap.InProgressChapter || 0)) return 'writing';
  if (n <= (snap.CompletedCount || 0)) return 'done';
  return 'todo';
}

var STATUS_ICON = { done: '✅', writing: '📝', rewrite: '🔄', todo: '—' };

function chapterCount(snap) {
  var candidates = [
    snap.TotalChapters || 0,
    (snap.Outline || []).length,
    snap.CurrentChapter || 0,
    snap.InProgressChapter || 0,
  ];
  var rewrites = snap.PendingRewrites || [];
  for (var i = 0; i < rewrites.length; i++) { candidates.push(rewrites[i]); }
  var max = 0;
  for (var j = 0; j < candidates.length; j++) { if (candidates[j] > max) max = candidates[j]; }
  return max;
}

var _selectedChapterN = null;

function renderChapterList(snap) {
  var container = $('#chapterItems');
  if (!container) return;

  var total = chapterCount(snap);
  var outline = snap.Outline || [];

  container.innerHTML = '';
  if (!total) {
    container.innerHTML = '<li class="muted">Chưa có chương</li>';
    return;
  }

  for (var n = 1; n <= total; n++) {
    var status = chapterStatus(n, snap);
    var title = '';
    for (var k = 0; k < outline.length; k++) {
      if (outline[k].Chapter === n) { title = outline[k].Title || ''; break; }
    }

    var li = document.createElement('li');
    li.className = 'chapter-item';
    if (status === 'writing') li.classList.add('is-writing');
    if (n === _selectedChapterN) li.classList.add('selected');
    li.dataset.chapter = n;

    var icon = document.createElement('span');
    icon.className = 'chapter-icon';
    icon.textContent = STATUS_ICON[status];
    icon.title = status === 'done' ? 'Hoàn thành' : status === 'writing' ? 'Đang viết' : status === 'rewrite' ? 'Chờ rewrite' : 'Chưa viết';

    var label = document.createElement('span');
    label.className = 'chapter-title';
    label.textContent = title ? n + '. ' + title : 'Chương ' + n;

    li.append(icon, label);
    li.addEventListener('click', (function (num) {
      return function () {
        _selectedChapterN = num;
        if (typeof selectChapter === 'function') selectChapter(num);
        highlightChapterItem(num);
      };
    })(n));
    container.appendChild(li);
  }

  // Auto-scroll to in-progress chapter
  if (snap.InProgressChapter) {
    var active = container.querySelector('.is-writing');
    if (active) active.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }
}

function highlightChapterItem(n) {
  var items = document.querySelectorAll('#chapterItems .chapter-item');
  items.forEach(function (li) {
    li.classList.toggle('selected', Number(li.dataset.chapter) === n);
  });
}

// ── Action buttons ──

function renderActions(snap) {
  var container = $('#actionsStack');
  if (!container) return;

  var state = snap.RuntimeState || 'idle';
  var hasNovel = (snap.CompletedCount || 0) > 0 || (snap.CurrentChapter || 0) > 0 || (snap.TotalChapters || 0) > 0;
  var rewrites = snap.PendingRewrites || [];

  container.innerHTML = '';

  if (state === 'running' || state === 'pausing') {
    addAction(container, '■ Dừng', 'danger', function () { post('/api/abort', {}); });
  } else if (state === 'paused') {
    addAction(container, '▶ Tiếp tục', 'primary', function () { post('/api/continue', {}); });
    addAction(container, '↗ Can thiệp', '', function () { focusInputAsSteer(); });
  } else if (state === 'completed') {
    addAction(container, '📦 Xuất bản', 'primary', function () { if (typeof openExport === 'function') openExport(); });
  } else {
    // idle
    if (hasNovel) {
      addAction(container, '▶ Tiếp tục', 'primary', function () { post('/api/continue', {}); });
      addAction(container, '↗ Can thiệp', '', function () { focusInputAsSteer(); });
      addAction(container, '📦 Xuất bản', '', function () { if (typeof openExport === 'function') openExport(); });
      addAction(container, '💬 Đồng sáng tác', '', function () { if (typeof openCoCreate === 'function') openCoCreate(); });
    } else {
      addAction(container, '✍ Bắt đầu', 'primary', function () { $('#input').focus(); });
      addAction(container, '💬 Đồng sáng tác', '', function () { if (typeof openCoCreate === 'function') openCoCreate(); });
    }
  }

  if (rewrites.length) {
    var hint = document.createElement('div');
    hint.className = 'rewrite-hint';
    hint.textContent = '🔄 Rewrite chờ: Ch ' + rewrites.join(', ');
    container.appendChild(hint);
  }
}

function addAction(parent, label, variant, handler) {
  var btn = document.createElement('button');
  btn.className = 'btn' + (variant ? ' ' + variant : '');
  btn.textContent = label;
  btn.addEventListener('click', handler);
  parent.appendChild(btn);
}

function focusInputAsSteer() {
  pendingMode = 'steer';
  var input = $('#input');
  if (input) {
    input.focus();
    input.placeholder = 'Nhập nội dung can thiệp…';
  }
}

// ── Compact progress ──

function renderProgress(snap) {
  var el = $('#progressSummary');
  if (!el) return;
  var done = snap.CompletedCount || 0;
  var total = chapterCount(snap);
  var words = (snap.TotalWordCount || 0).toLocaleString();
  var model = snap.ModelName || '';
  var cost = snap.TotalCostUSD != null ? '$' + Number(snap.TotalCostUSD).toFixed(4) : '';
  var parts = [];
  if (total) parts.push('✅ ' + done + '/' + total + ' chương');
  if (snap.TotalWordCount) parts.push(words + ' từ');
  if (model) parts.push(model);
  if (cost) parts.push(cost);

  // Thêm agent đang chạy
  var agents = snap.Agents || [];
  var activeAgents = agents.filter(function(a) { return a.State && a.State !== 'idle'; });
  if (activeAgents.length) {
    var agentStr = activeAgents.map(function(a) { return a.Name + ':' + a.State; }).join(', ');
    parts.push(agentStr);
  }

  el.textContent = parts.join(' · ') || '—';
}
