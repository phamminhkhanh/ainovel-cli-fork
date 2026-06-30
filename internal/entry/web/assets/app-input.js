// ainovel web — input history, inline command palette, Esc behavior, startup mode selector.
// Nạp sau app-studio.js nên dùng chung $, COMMANDS, allOverlaysHidden, openCoCreate.
// app.js gọi hook: renderStartupSelector(s) sau mỗi snapshot, onInputSent(text) sau mỗi lệnh gửi thành công.
'use strict';

// ── Input history ──
const HISTORY_KEY = 'ainovel-input-history';
const HISTORY_CAP = 100;
let inputHistory = [];
let historyIdx = -1;
let historyDraft = '';

try {
  const raw = localStorage.getItem(HISTORY_KEY);
  if (raw) inputHistory = JSON.parse(raw);
  if (!Array.isArray(inputHistory)) inputHistory = [];
} catch (e) {
  inputHistory = [];
}

function saveHistory() {
  try { localStorage.setItem(HISTORY_KEY, JSON.stringify(inputHistory)); } catch (e) {}
}
function pushHistory(text) {
  text = text.trim();
  if (!text) return;
  if (inputHistory.length > 0 && inputHistory[0] === text) return;
  inputHistory.unshift(text);
  if (inputHistory.length > HISTORY_CAP) inputHistory.pop();
  saveHistory();
}
function historyUp(input) {
  if (!inputHistory.length) return;
  if (historyIdx === -1) historyDraft = input.value;
  historyIdx = Math.min(historyIdx + 1, inputHistory.length - 1);
  input.value = inputHistory[historyIdx];
}
function historyDown(input) {
  if (historyIdx <= 0) {
    historyIdx = -1;
    input.value = historyDraft;
    return;
  }
  historyIdx--;
  input.value = inputHistory[historyIdx];
}
function resetHistory() {
  historyIdx = -1;
  historyDraft = '';
}

// Hook được app.js gọi sau khi gửi thành công.
function onInputSent(text) {
  pushHistory(text);
  resetHistory();
}

// ── Inline command palette ──
let inlinePaletteVisible = false;
let inlineSelected = 0;

function inputCommandQuery(value) {
  if (!value.startsWith('/')) return null;
  const rest = value.slice(1);
  if (/\s/.test(rest)) return null;
  return rest.toLowerCase();
}
function filteredInlineCommands(q) {
  if (!q) return COMMANDS;
  return COMMANDS.filter((c) => (c.key + ' ' + c.label + ' ' + c.desc).toLowerCase().includes(q));
}
function renderInlinePalette(list) {
  let ul = $('#inlinePalette');
  if (!ul) {
    ul = document.createElement('ul');
    ul.id = 'inlinePalette';
    ul.className = 'inline-palette';
    const composer = $('.composer');
    if (composer) composer.appendChild(ul);
  }
  ul.innerHTML = '';
  if (!list.length) { ul.hidden = true; inlinePaletteVisible = false; return; }
  list.forEach((c, i) => {
    const li = document.createElement('li');
    li.className = 'inline-palette-item';
    if (i === inlineSelected) li.classList.add('selected');
    const k = document.createElement('span'); k.className = 'inline-key'; k.textContent = '/' + c.key;
    const l = document.createElement('span'); l.className = 'inline-label'; l.textContent = c.label;
    const d = document.createElement('span'); d.className = 'inline-desc'; d.textContent = c.desc;
    li.append(k, l, d);
    li.addEventListener('click', () => runInlineCommand(c));
    ul.appendChild(li);
  });
  ul.hidden = false;
  inlinePaletteVisible = true;
}
function closeInlinePalette() {
  const ul = $('#inlinePalette');
  if (ul) ul.hidden = true;
  inlinePaletteVisible = false;
  inlineSelected = 0;
}
function acceptInlinePalette() {
  const input = $('#input');
  const q = inputCommandQuery(input.value);
  const list = filteredInlineCommands(q);
  if (!list.length) return;
  if (inlineSelected < 0 || inlineSelected >= list.length) inlineSelected = 0;
  runInlineCommand(list[inlineSelected]);
}
function runInlineCommand(c) {
  closeInlinePalette();
  $('#input').value = '';
  c.run();
}
function updateInlinePalette(input) {
  const q = inputCommandQuery(input.value);
  if (q === null) { closeInlinePalette(); return; }
  inlineSelected = 0;
  renderInlinePalette(filteredInlineCommands(q));
}

// ── Startup mode selector ──
function renderStartupSelector() {
  const card = $('#startupCard');
  if (!card) return;
  const s = lastSnapshot;
  const idle = currentState === 'idle';
  const noProgress = !s || ((s.CompletedCount || 0) === 0 && (s.CurrentChapter || 0) === 0 && (s.TotalChapters || 0) === 0);
  card.hidden = !(idle && noProgress);
}
function setupStartupSelector() {
  const btnStart = $('#startupQuick');
  const btnCoCreate = $('#startupCoCreate');
  if (btnStart) btnStart.addEventListener('click', () => { $('#input').focus(); });
  if (btnCoCreate) btnCoCreate.addEventListener('click', () => { if (typeof openCoCreate === 'function') openCoCreate(); });
}

// ── Wiring ──
function setupInput() {
  const input = $('#input');
  if (!input) return;

  input.addEventListener('input', () => {
    updateInlinePalette(input);
  });

  input.addEventListener('keydown', (e) => {
    if (inlinePaletteVisible && ['ArrowUp', 'ArrowDown', 'Enter', 'Tab', 'Escape'].includes(e.key)) {
      const q = inputCommandQuery(input.value);
      const list = filteredInlineCommands(q);
      if (e.key === 'ArrowUp') { e.preventDefault(); inlineSelected = (inlineSelected - 1 + list.length) % list.length; renderInlinePalette(list); }
      else if (e.key === 'ArrowDown') { e.preventDefault(); inlineSelected = (inlineSelected + 1) % list.length; renderInlinePalette(list); }
      else if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); acceptInlinePalette(); }
      else if (e.key === 'Escape') { e.preventDefault(); closeInlinePalette(); }
      return;
    }

    if (e.key === 'ArrowUp' && input.selectionStart === input.value.length && inputHistory.length) {
      e.preventDefault();
      historyUp(input);
      return;
    }
    if (e.key === 'ArrowDown' && input.selectionStart === input.value.length) {
      e.preventDefault();
      historyDown(input);
      return;
    }
  });

  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !allOverlaysHidden()) return;
    if (document.activeElement === input) input.blur();
    if (currentState === 'running' || currentState === 'pausing') {
      e.preventDefault();
      post('/api/abort', {});
    } else if (!input.value.trim()) {
      return;
    } else {
      e.preventDefault();
      input.value = '';
      closeInlinePalette();
    }
  });
}

setupStartupSelector();
setupInput();
renderStartupSelector();
