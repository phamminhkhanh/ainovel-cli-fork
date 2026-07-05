// ainovel web - Production Cockpit tab (San xuat).
// Nap sau app.js de dung $, post, toast.
'use strict';

let productionSelectedRunId = null;
let productionPollTimer = null;
let productionRunsCache = [];
let productionProfilesCache = [];
let productionCreateMode = 'fresh_profile';
let productionWorkspaceSnapshot = null;
// Cache nội dung preview nền móng theo run id (nội dung tĩnh) → tránh nhấp nháy
// "Đang tải…" và fetch lại mỗi vòng poll 5s.
const foundationPreviewCache = {};

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
              <button class="btn sm" id="btnProfileLibrary">\ud83d\udcda Th\u01b0 vi\u1ec7n Profile</button>
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
      </div>
      <div class="modal-overlay" id="profileLibOverlay" hidden>
        <div class="modal modal-wide" role="dialog" aria-modal="true" aria-labelledby="profileLibTitle">
          <header class="modal-head"><h2 id="profileLibTitle">\ud83d\udcda Th\u01b0 vi\u1ec7n Profile</h2></header>
          <div class="modal-body profile-lib">
            <aside class="profile-lib-list">
              <button class="btn sm primary" id="profileLibNew">+ Profile m\u1edbi</button>
              <ul id="profileLibItems"><li class="muted">\u0110ang t\u1ea3i\u2026</li></ul>
              <small class="muted">Ch\u1ec9 profile trong <code>project</code> (./.ainovel/profiles/) s\u1eeda/x\u00f3a \u0111\u01b0\u1ee3c. global/legacy ch\u1ec9 \u0111\u1ecdc.</small>
            </aside>
            <section class="profile-lib-editor">
              <details class="profile-studio" id="profileStudio">
                <summary>\u2728 Sinh profile t\u1eeb \u00fd t\u01b0\u1edfng (AI)</summary>
                <div class="field"><label for="studioIdea">\u00dd t\u01b0\u1edfng th\u00f4</label><textarea id="studioIdea" rows="2" placeholder="vd: tu ti\u00ean, main mang ki\u1ebfm bi\u1ebft n\u00f3i, m\u1ed7i l\u1ea7n r\u00fat ki\u1ebfm qu\u00ean m\u1ed9t k\u00fd \u1ee9c"></textarea></div>
                <div class="field-row">
                  <div class="field"><label for="studioGenre">Th\u1ec3 lo\u1ea1i</label><input type="text" id="studioGenre" placeholder="vd: tu ti\u00ean huy\u1ec1n huy\u1ec5n"></div>
                  <div class="field"><label for="studioPlatform">N\u1ec1n t\u1ea3ng/th\u1ecb tr\u01b0\u1eddng</label><input type="text" id="studioPlatform" placeholder="vd: WebNovel, KDP"></div>
                </div>
                <div class="field-row">
                  <div class="field"><label for="studioLang">Ng\u00f4n ng\u1eef</label><input type="text" id="studioLang" value="Ti\u1ebfng Vi\u1ec7t"></div>
                  <div class="field"><label for="studioChapters">S\u1ed1 ch\u01b0\u01a1ng d\u1ef1 ki\u1ebfn</label><input type="number" id="studioChapters" min="1" value="60"></div>
                </div>
                <div class="field"><label for="studioStyle">Phong c\u00e1ch / y\u00eau c\u1ea7u b\u1eaft bu\u1ed9c (t\u00f9y ch\u1ecdn)</label><input type="text" id="studioStyle" placeholder="vd: k\u1ebft bittersweet, tr\u00e1nh h\u1eady mono, nh\u1ecbp nhanh"></div>
                <button class="btn primary" id="studioGenerate">\u2728 Sinh profile</button>
                <small class="muted">Sinh v\u00e0o \u00f4 b\u00ean d\u01b0\u1edbi \u0111\u1ec3 b\u1ea1n duy\u1ec7t/s\u1eeda r\u1ed3i L\u01b0u. T\u1ed1n ~$0.01, kh\u00f4ng t\u1ea1o run.</small>
              </details>
              <details class="profile-guide" id="profileGuide">
                <summary>\ud83d\udcd6 H\u01b0\u1edbng d\u1eabn &amp; l\u01b0u \u00fd vi\u1ebft profile</summary>
                <ul class="profile-guide-list">
                  <li><strong>C\u1ee5 th\u1ec3 ho\u00e1:</strong> n\u00eau t\u00ean nh\u00e2n v\u1eadt, chi ti\u1ebft b\u1ed1i c\u1ea3nh, h\u01b0\u1edbng twist \u2014 \u0111\u1eebng \u0111\u1ec3 chung chung, Architect s\u1ebd t\u1ef1 b\u1ecba.</li>
                  <li><strong>Ch\u1ed1t h\u01b0\u1edbng k\u1ebft</strong> (theo ch\u1ee7 \u0111\u1ec1, kh\u00f4ng ph\u1ea3i t\u00ean ch\u01b0\u01a1ng): happy / bittersweet / open.</li>
                  <li><strong>\u0110i\u1ec3m kh\u00e1c bi\u1ec7t \u2265 3</strong>: v\u00ec sao \u0111\u1ecdc gi\u1ea3 \u0111\u1ecdc ti\u1ebfp.</li>
                  <li><strong>Tr\u00e1nh AI-tell:</strong> ghi r\u00f5 c\u1ea5m purple prose, c\u1ea5u tr\u00fac "kh\u00f4ng ph\u1ea3i X m\u00e0 l\u00e0 Y", n\u1ed9i t\u00e2m l\u1eb7p, tho\u1ea1i gi\u1ea3i th\u00edch th\u1eeba.</li>
                  <li><strong>Kh\u1edbp \u0111\u1ed9 d\u00e0i</strong>: s\u1ed1 ch\u01b0\u01a1ng + s\u1ed1 t\u1eeb/ch\u01b0\u01a1ng m\u1ee5c ti\u00eau.</li>
                  <li>D\u00f9ng ngo\u00e0i: copy khung d\u01b0\u1edbi \u0111\u00e2y \u2192 d\u00e1n v\u00e0o LLM kh\u00e1c \u2192 "\u0111i\u1ec1n template n\u00e0y cho \u00fd t\u01b0\u1edfng c\u1ee7a t\u00f4i".</li>
                  <li><strong>\u0110\u1ecdc k\u1ef9 tr\u01b0\u1edbc khi L\u01b0u:</strong> AI (Studio ho\u1eb7c LLM ngo\u00e0i) c\u00f3 th\u1ec3 c\u00f2n s\u00f3t AI-tell \u2014 hay g\u1eb7p nh\u1ea5t l\u00e0 c\u1ea5u tr\u00fac "kh\u00f4ng ph\u1ea3i X m\u00e0 l\u00e0 Y" \u1edf ph\u1ea7n H\u01b0\u1edbng k\u1ebft. S\u1eeda tay c\u00e1c c\u00e2u \u0111\u00f3 tr\u01b0\u1edbc khi l\u01b0u.</li>
                </ul>
              </details>
              <div class="field"><label for="profileLibName">T\u00ean file (.md)</label><input type="text" id="profileLibName" placeholder="vd: werewolf-100c"></div>
              <div class="field profile-lib-content-field"><label for="profileLibContent">N\u1ed9i dung brief (markdown)</label><textarea id="profileLibContent" placeholder="M\u00f4 t\u1ea3 truy\u1ec7n b\u1ea1n mu\u1ed1n engine vi\u1ebft: b\u1ed1i c\u1ea3nh, th\u1ec3 lo\u1ea1i, nh\u00e2n v\u1eadt, \u0111\u1ed9 d\u00e0i, phong c\u00e1ch\u2026"></textarea></div>
              <div class="profile-lib-actions">
                <button class="btn primary" id="profileLibSave">L\u01b0u</button>
                <button class="btn" id="profileLibCopyLLM">\ud83d\udccb Copy cho LLM ngo\u00e0i</button>
                <button class="btn danger" id="profileLibDelete" hidden>\ud83d\uddd1 X\u00f3a</button>
                <span class="muted" id="profileLibHint"></span>
              </div>
            </section>
          </div>
          <footer class="modal-foot">
            <button class="btn" id="profileLibClose">\u0110\u00f3ng</button>
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

  // Profile Library
  $('#btnProfileLibrary')?.addEventListener('click', openProfileLibrary);
  $('#profileLibClose')?.addEventListener('click', () => { $('#profileLibOverlay').hidden = true; });
  $('#profileLibOverlay')?.addEventListener('click', (e) => {
    if (e.target.id === 'profileLibOverlay') $('#profileLibOverlay').hidden = true;
  });
  $('#profileLibNew')?.addEventListener('click', profileLibNew);
  $('#studioGenerate')?.addEventListener('click', generateProfileFromIdea);
  $('#profileLibSave')?.addEventListener('click', profileLibSave);
  $('#profileLibCopyLLM')?.addEventListener('click', profileLibCopyForLLM);
  $('#profileLibDelete')?.addEventListener('click', profileLibDelete);
  $('#profileLibItems')?.addEventListener('click', (e) => {
    const li = e.target.closest('[data-profile-path]');
    if (!li) return;
    profileLibLoad(li.dataset.profilePath, li.dataset.profileSource);
  });

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
    else if (btn.dataset.action === 'approve') approveProductionRun(id);
    else if (btn.dataset.action === 'reject') rejectProductionRun(id);
    else if (btn.dataset.action === 'revise') reviseProductionRun(id);
    else if (btn.dataset.action === 'reveal') revealFoundationFolder(id);
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

// ── Profile Library ──────────────────────────────────────────────
// Profile = SSOT brief authored/reviewed here BEFORE a run exists.
// Only project profiles (./.ainovel/profiles/) are editable/deletable;
// global/legacy are read-only (edit = save-as into project).

let profileLibSelected = null; // { path, source }

// Khung profile chuẩn (khớp các chiều Architect cần để sinh Premise mạnh).
// Điền tay trong UI, hoặc copy ra LLM ngoài. Gợi ý trong ngoặc — thay bằng nội dung thật.
const PROFILE_TEMPLATE = `# [Tên truyện]

<!-- Nguyên tắc: mô tả ĐỊNH HƯỚNG & RÀNG BUỘC, đừng viết sẵn câu văn mẫu.
     Ghi rõ mỗi phần cần ĐẠT gì, để Architect/Writer tự sáng tạo. Xoá dòng gợi ý khi viết xong. -->

## Thể loại & giọng điệu
(thể loại chính + sub-genre; sắc thái cảm xúc chủ đạo; nhịp kể; nền tảng & đối tượng đọc nếu nhắm bán)

## Bối cảnh & thế giới quan
(luật chơi + kết cấu thế giới RÀNG BUỘC cốt truyện: tài nguyên, cái giá, giới hạn, biên các thế lực — đủ để dựng world rules, không chỉ trang trí)

## Nhân vật chính & tuyến quan hệ
(mỗi nhân vật chính: một khát khao, một vết thương, một mâu thuẫn nội tại; quan hệ phải có căng thẳng thường trực, không chỉ là đồng minh)

## Xung đột cốt lõi
(áp lực trung tâm đủ sức nuôi CẢ truyện, không phải một sự việc lẻ)

## Hướng kết (theo chủ đề, không phải tên chương)
(câu hỏi mà cái kết trả lời + lập trường nó chọn; happy / bittersweet / open)

## Điểm bán khác biệt (≥3)
(mỗi điểm phải là thứ mà truyện CÙNG THỂ LOẠI không chắc có)

## Công thức chương
(cách một chương điển hình mở — dựng — kết; kỷ luật giữ người đọc lật trang)

## Điều cần tránh
(cliché của thể loại cần né; VÀ AI-tell nêu dạng nguyên tắc: purple prose, cấu trúc "không phải X mà là Y", nội tâm lặp, thoại giải thích thừa, câu dài đều nhịp)

## Độ dài & phong cách
(số chương mục tiêu + số từ/chương; văn phong; ngôn ngữ viết)
`;

async function openProfileLibrary() {
  $('#profileLibOverlay').hidden = false;
  profileLibNew();
  await refreshProfileLibList();
}

async function refreshProfileLibList() {
  const ul = $('#profileLibItems');
  if (!ul) return;
  try {
    const res = await fetch('/api/profiles');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const items = await res.json();
    if (!items.length) {
      ul.innerHTML = '<li class="muted">Ch\u01b0a c\u00f3 profile n\u00e0o.</li>';
      return;
    }
    ul.innerHTML = items.map((p) => {
      const sel = profileLibSelected && profileLibSelected.path === p.path ? ' selected' : '';
      const ro = p.source !== 'project' ? ' <span class="profile-lib-ro">read-only</span>' : '';
      return `<li class="profile-lib-item${sel}" data-profile-path="${escapeHtml(p.path)}" data-profile-source="${escapeHtml(p.source)}" tabindex="0">
        <span class="profile-lib-name">${escapeHtml(p.name)}</span>
        <span class="profile-lib-src">${escapeHtml(p.source)}${ro}</span>
      </li>`;
    }).join('');
  } catch (e) {
    ul.innerHTML = '<li class="muted">L\u1ed7i t\u1ea3i: ' + escapeHtml(String(e)) + '</li>';
  }
}

function profileLibNew() {
  profileLibSelected = null;
  $('#profileLibName').value = '';
  $('#profileLibName').readOnly = false;
  $('#profileLibContent').value = PROFILE_TEMPLATE;
  $('#profileLibDelete').hidden = true;
  $('#profileLibHint').textContent = 'Khung m\u1eabu \u0111\u00e3 \u0111i\u1ec1n s\u1eb5n \u2014 thay g\u1ee3i \u00fd b\u1eb1ng n\u1ed9i dung r\u1ed3i L\u01b0u.';
}

async function profileLibLoad(path, source) {
  try {
    const res = await fetch('/api/profiles/content?path=' + encodeURIComponent(path));
    if (!res.ok) { const d = await res.json().catch(() => ({})); toast(d.error || ('HTTP ' + res.status), 'error'); return; }
    const data = await res.json();
    profileLibSelected = { path, source };
    $('#profileLibContent').value = data.content || '';
    // Name is the base filename; project files keep their name, non-project
    // become a save-as (name editable, will land in project).
    const base = (data.name || '').replace(/\.md$/i, '');
    $('#profileLibName').value = base;
    if (source === 'project') {
      $('#profileLibName').readOnly = true;
      $('#profileLibDelete').hidden = false;
      $('#profileLibHint').textContent = 'S\u1eeda tr\u1ef1c ti\u1ebfp profile project n\u00e0y.';
    } else {
      $('#profileLibName').readOnly = false;
      $('#profileLibDelete').hidden = true;
      $('#profileLibHint').textContent = source + ' l\u00e0 read-only \u2014 L\u01b0u s\u1ebd t\u1ea1o b\u1ea3n copy trong project.';
    }
    await refreshProfileLibList();
  } catch (e) {
    toast('L\u1ed7i t\u1ea3i profile: ' + e, 'error');
  }
}

// Profile Studio (C-lite): one-shot generate a profile from a rough idea into
// the editor for review. Does not save or run anything.
async function generateProfileFromIdea() {
  const idea = $('#studioIdea').value.trim();
  if (!idea) { toast('Nh\u1eadp \u00fd t\u01b0\u1edfng th\u00f4 tr\u01b0\u1edbc', 'error'); return; }
  const btn = $('#studioGenerate');
  const body = {
    idea,
    genre: $('#studioGenre').value.trim() || undefined,
    platform: $('#studioPlatform').value.trim() || undefined,
    language: $('#studioLang').value.trim() || undefined,
    styleNotes: $('#studioStyle').value.trim() || undefined,
    targetChapters: parseInt($('#studioChapters').value, 10) || undefined,
  };
  const prev = btn.textContent;
  btn.disabled = true;
  btn.textContent = '\u23f3 \u0110ang sinh\u2026';
  try {
    const res = await post('/api/profiles/generate', body);
    if (res && res.content) {
      // Treat as a fresh, unsaved project profile (editable, no delete yet).
      profileLibSelected = null;
      $('#profileLibName').readOnly = false;
      $('#profileLibDelete').hidden = true;
      $('#profileLibContent').value = res.content;
      if (!$('#profileLibName').value.trim()) {
        $('#profileLibName').value = 'profile-' + Date.now();
      }
      $('#profileLibHint').textContent = '\u0110\u00e3 sinh \u2014 duy\u1ec7t/s\u1eeda r\u1ed3i b\u1ea5m L\u01b0u.';
      toast('\u0110\u00e3 sinh profile, duy\u1ec7t r\u1ed3i L\u01b0u', 'ok');
    }
  } finally {
    btn.disabled = false;
    btn.textContent = prev;
  }
}

async function profileLibSave() {
  const name = $('#profileLibName').value.trim();
  const content = $('#profileLibContent').value;
  if (!name) { toast('Nh\u1eadp t\u00ean file', 'error'); return; }
  if (!content.trim()) { toast('N\u1ed9i dung tr\u1ed1ng', 'error'); return; }
  // Editing an existing project profile → overwrite expected. New/generated →
  // don't clobber silently: backend returns 409, we confirm then retry.
  const editingProject = !!(profileLibSelected && profileLibSelected.source === 'project');
  const res = await saveProfileRequest(name, content, editingProject);
  if (res) {
    profileLibSelected = { path: res.path, source: res.source };
    $('#profileLibName').readOnly = true;
    $('#profileLibDelete').hidden = false;
    $('#profileLibHint').textContent = '\u0110\u00e3 l\u01b0u ' + res.path;
    toast('\u0110\u00e3 l\u01b0u profile', 'ok');
    await refreshProfileLibList();
    // Refresh the New Run dropdown so the new profile is immediately usable.
    productionProfilesCache = [];
    await populateProfileSelect();
  }
}

// saveProfileRequest posts a save; on 409 (exists) confirms once then retries
// with overwrite=true. Returns the saved item, or null if cancelled/failed.
async function saveProfileRequest(name, content, overwrite) {
  const r = await fetch('/api/profiles/save', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, content, overwrite: !!overwrite }),
  });
  if (r.status === 409 && !overwrite) {
    if (confirm('Profile "' + name + '.md" \u0111\u00e3 t\u1ed3n t\u1ea1i. Ghi \u0111\u00e8?')) {
      return saveProfileRequest(name, content, true);
    }
    return null;
  }
  if (!r.ok) {
    const d = await r.json().catch(() => ({}));
    toast(d.error || ('HTTP ' + r.status), 'error');
    return null;
  }
  return r.json();
}

// Copy the current template/brief + a ready instruction to the clipboard, so
// the user can paste it into an external LLM (ChatGPT/Claude), have it filled,
// then paste the result back. Reduces the manual "select textarea + write
// instruction" friction of the external-LLM path.
async function profileLibCopyForLLM() {
  const tpl = $('#profileLibContent').value || PROFILE_TEMPLATE;
  const payload =
    'B\u1ea1n l\u00e0 tr\u1ee3 l\u00fd so\u1ea1n "profile" (brief) cho engine vi\u1ebft ti\u1ec3u thuy\u1ebft d\u00e0i.\n' +
    '\u0110i\u1ec1n \u0111\u1ea7y \u0111\u1ee7 template Markdown b\u00ean d\u01b0\u1edbi cho \u00fd t\u01b0\u1edfng c\u1ee7a t\u00f4i: [\u0110I\u1ec0N \u00dd T\u01af\u1edeNG TH\u00d4 C\u1ee6A B\u1ea0N \u1ede \u0110\u00c2Y]\n\n' +
    'Nguy\u00ean t\u1eafc:\n' +
    '- Vi\u1ebft C\u1ee4 TH\u1ec2 (t\u00ean nh\u00e2n v\u1eadt, chi ti\u1ebft b\u1ed1i c\u1ea3nh, h\u01b0\u1edbng twist), t\u1ef1 quy\u1ebft \u0111\u1ecbnh khi t\u00f4i b\u1ecf tr\u1ed1ng.\n' +
    '- M\u00f4 t\u1ea3 theo \u0110\u1ecaNH H\u01af\u1edaNG & R\u00c0NG BU\u1ed8C, \u0110\u1eebNG vi\u1ebft s\u1eb5n c\u00e2u v\u0103n m\u1eabu (tr\u00e1nh anchor).\n' +
    '- Tr\u00e1nh AI-tell: no purple prose, no "kh\u00f4ng ph\u1ea3i X m\u00e0 l\u00e0 Y", n\u1ed9i t\u00e2m l\u1eb7p, tho\u1ea1i gi\u1ea3i th\u00edch th\u1eeba.\n' +
    '- Ch\u1ec9 xu\u1ea5t Markdown c\u1ee7a profile, kh\u00f4ng gi\u1ea3i th\u00edch th\u00eam.\n\n' +
    '--- TEMPLATE ---\n' + tpl;
  try {
    await navigator.clipboard.writeText(payload);
    toast('\u0110\u00e3 copy template + c\u00e2u l\u1ec7nh. D\u00e1n v\u00e0o ChatGPT/Claude, \u0111i\u1ec1n \u00fd t\u01b0\u1edfng, r\u1ed3i d\u00e1n k\u1ebft qu\u1ea3 v\u1ec1 \u00f4 n\u00e0y.', 'ok');
  } catch (e) {
    toast('Tr\u00ecnh duy\u1ec7t ch\u1eb7n clipboard \u2014 h\u00e3y b\u00f4i \u0111en \u00f4 n\u1ed9i dung r\u1ed3i copy tay.', 'error');
  }
}

async function profileLibDelete() {
  if (!profileLibSelected || profileLibSelected.source !== 'project') return;
  if (!confirm('X\u00f3a profile "' + profileLibSelected.path + '"?')) return;
  try {
    const r = await fetch('/api/profiles/delete', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: profileLibSelected.path }),
    });
    if (!r.ok) { const d = await r.json().catch(() => ({})); toast(d.error || ('HTTP ' + r.status), 'error'); return; }
    toast('\u0110\u00e3 x\u00f3a profile', 'ok');
    profileLibNew();
    await refreshProfileLibList();
    productionProfilesCache = [];
    await populateProfileSelect();
  } catch (e) {
    toast('L\u1ed7i x\u00f3a: ' + e, 'error');
  }
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
    // Prune foundation preview cache: drop entries for runs no longer present
    // (deleted/rejected/revised) so the cache can't grow without bound.
    const liveIds = new Set(productionRunsCache.map((r) => r.id));
    for (const cachedId of Object.keys(foundationPreviewCache)) {
      if (!liveIds.has(cachedId)) delete foundationPreviewCache[cachedId];
    }
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

// Foundation Gate (Milestone 1a): approve resumes the seeded run natively
// (headless Resume(), no --prompt-file), reject deletes it. This is a
// best-effort auto-pause on a 5s poll — the engine flips to writing the moment
// the foundation is saved, so in the worst case the Writer may already have
// started chapter 1 before the pause lands. It reliably stops BEFORE the bulk
// of the book, not with a guaranteed zero-token boundary.
async function approveProductionRun(id) {
  if (!confirm('X\u00e1c nh\u1eadn n\u1ec1n m\u00f3ng (outline/th\u1ebf gi\u1edbi/nh\u00e2n v\u1eadt) \u1ecfn v\u00e0 ti\u1ebfp t\u1ee5c vi\u1ebft?')) return;
  const res = await post(`/api/prodruns/${id}/approve`, {});
  if (res) {
    toast('\u0110\u00e3 duy\u1ec7t n\u1ec1n m\u00f3ng, ti\u1ebfp t\u1ee5c vi\u1ebft', 'ok');
    await loadProductionData();
  }
}

async function rejectProductionRun(id) {
  if (!confirm('T\u1eeb ch\u1ed1i n\u1ec1n m\u00f3ng n\u00e0y v\u00e0 x\u00f3a job? Ti\u1ebft ki\u1ec7m to\u00e0n b\u1ed9 chi ph\u00ed vi\u1ebft truy\u1ec7n (t\u1ec7 nh\u1ea5t ch\u1ec9 t\u1ed1n m\u1ed9t ph\u1ea7n ch\u01b0\u01a1ng 1 n\u1ebfu Writer \u0111\u00e3 k\u1ecbp b\u1eaft \u0111\u1ea7u).')) return;
  try {
    const r = await fetch(`/api/prodruns/${id}/reject`, { method: 'POST' });
    if (!r.ok) {
      const data = await r.json().catch(() => ({}));
      toast(data.error || ('HTTP ' + r.status), 'error');
      return;
    }
    if (productionSelectedRunId === id) productionSelectedRunId = null;
    toast('\u0110\u00e3 t\u1eeb ch\u1ed1i n\u1ec1n m\u00f3ng, x\u00f3a job', 'ok');
    await loadProductionData();
  } catch (e) {
    toast('L\u1ed7i t\u1eeb ch\u1ed1i: ' + e, 'error');
  }
}

// Foundation Gate revise: regenerate the (cheap) foundation with a steering
// note. Creates a NEW run and selects it; the old run is kept as a fallback
// (delete it manually when happy). Cost ~$0.01; same best-effort gate applies.
async function reviseProductionRun(id) {
  const ta = $('#runReviseNote');
  const feedback = ta ? ta.value.trim() : '';
  if (!feedback) { toast('Nh\u1eadp g\u00f3p \u00fd mu\u1ed1n \u0111i\u1ec1u ch\u1ec9nh tr\u01b0\u1edbc', 'error'); return; }
  if (!confirm('T\u1ea1o job M\u1edaI v\u1edbi n\u1ec1n m\u00f3ng vi\u1ebft l\u1ea1i theo g\u00f3p \u00fd? Job hi\u1ec7n t\u1ea1i \u0111\u01b0\u1ee3c GI\u1eees L\u1ea0I l\u00e0m b\u1ea3n d\u1ef1 ph\u00f2ng (t\u1ef1 x\u00f3a khi h\u00e0i l\u00f2ng). Chi ph\u00ed ~$0.01 sinh n\u1ec1n m\u00f3ng.')) return;
  const res = await post(`/api/prodruns/${id}/revise`, { feedback });
  if (res) {
    productionSelectedRunId = res.id;
    toast('\u0110\u00e3 t\u1ea1o job m\u1edbi; job c\u0169 gi\u1eef l\u1ea1i l\u00e0m d\u1ef1 ph\u00f2ng', 'ok');
    await loadProductionData();
  }
}

// Mở thư mục nền móng của run để sửa tay (chính xác) trước khi Duyệt.
async function revealFoundationFolder(id) {
  const res = await post(`/api/prodruns/${id}/reveal`, {});
  if (res) {
    // Người dùng sắp sửa file tay → bỏ cache để lần render sau nạp lại nội dung mới.
    delete foundationPreviewCache[id];
    toast('\u0110\u00e3 m\u1edf th\u01b0 m\u1ee5c. S\u1eeda xong file r\u1ed3i b\u1ea5m Duy\u1ec7t (preview s\u1ebd t\u1ef1 c\u1eadp nh\u1eadt).', 'ok');
  }
}

async function loadFoundationPreview(id) {
  try {
    const [outlineRes, worldRes, charRes] = await Promise.all([
      fetch(`/api/prodruns/${id}/foundation`),
      fetch(`/api/prodruns/${id}/foundation?section=world`),
      fetch(`/api/prodruns/${id}/foundation?section=characters`),
    ]);
    const outline = outlineRes.ok ? await outlineRes.json() : null;
    const world = worldRes.ok ? await worldRes.json() : null;
    const chars = charRes.ok ? await charRes.json() : null;
    if (!outline && !world && !chars) return '<p class="muted">Ch\u01b0a \u0111\u1ecdc \u0111\u01b0\u1ee3c n\u1ec1n m\u00f3ng.</p>';

    const premise = outline?.premise ? escapeHtml(outline.premise) : '(ch\u01b0a c\u00f3 premise)';

    // Layered outline (Volume -> Arc) holds the volume/arc themes where the
    // big twists live. Show it first when present; the flat outline below is
    // just the currently-expanded arc's chapters.
    const volumes = Array.isArray(outline?.layered) ? outline.layered : [];
    const layeredHtml = volumes.length
      ? volumes.map((v) => {
          const vnum = v.index != null ? v.index : '?';
          const vtitle = escapeHtml(v.title || '');
          const vtheme = v.theme ? `<div class="foundation-theme">${escapeHtml(String(v.theme))}</div>` : '';
          const vfinal = v.final ? ' <span class="foundation-final">\u0111\u1ee9c k\u1ebft</span>' : '';
          const arcs = Array.isArray(v.arcs) ? v.arcs : [];
          const arcsHtml = arcs.length
            ? '<ul class="foundation-list">' + arcs.map((a) => {
                const at = escapeHtml(a.title || '');
                const goal = a.goal ? ` \u2014 ${escapeHtml(String(a.goal))}` : (a.theme ? ` \u2014 ${escapeHtml(String(a.theme))}` : '');
                const est = a.estimated_chapters ? ` (${a.estimated_chapters} ch\u01b0\u01a1ng)` : '';
                return `<li><strong>Arc ${escapeHtml(String(a.index != null ? a.index : '?'))}:</strong> ${at}${est}${goal}</li>`;
              }).join('') + '</ul>'
            : '';
          return `<div class="foundation-vol"><strong>Volume ${escapeHtml(String(vnum))}: ${vtitle}</strong>${vfinal}${vtheme}${arcsHtml}</div>`;
        }).join('')
      : '';

    const outlineList = Array.isArray(outline?.outline) ? outline.outline : [];
    const outlineHtml = outlineList.length
      ? '<ul class="foundation-list">' + outlineList.map((c) => {
          const num = c.chapter != null ? c.chapter : (c.index != null ? c.index : '?');
          const title = escapeHtml(c.title || c.core_event || c.event || '');
          return `<li><span class="foundation-num">${escapeHtml(String(num))}</span> ${title}</li>`;
        }).join('') + '</ul>'
      : '<p class="muted">(ch\u01b0a c\u00f3 outline chi ti\u1ebft)</p>';

    const rules = Array.isArray(world?.rules) ? world.rules : [];
    const rulesHtml = rules.length
      ? '<ul class="foundation-list">' + rules.map((r) => {
          const t = typeof r === 'string' ? r : (r.rule || r.description || r.name || JSON.stringify(r));
          return `<li>${escapeHtml(String(t))}</li>`;
        }).join('') + '</ul>'
      : '<p class="muted">(ch\u01b0a c\u00f3 world rules)</p>';

    const charList = Array.isArray(chars?.characters) ? chars.characters : [];
    const charsHtml = charList.length
      ? '<ul class="foundation-list">' + charList.map((c) => {
          const name = escapeHtml(c.name || c.id || '?');
          const role = c.role || c.archetype || c.description || '';
          return `<li><strong>${name}</strong>${role ? ' \u2014 ' + escapeHtml(String(role)) : ''}</li>`;
        }).join('') + '</ul>'
      : '<p class="muted">(ch\u01b0a c\u00f3 nh\u00e2n v\u1eadt)</p>';

    const layeredSection = volumes.length
      ? `<div class="foundation-section"><h5>C\u1ea5u tr\u00fac truy\u1ec7n (${volumes.length} volume \u2014 theme/twist)</h5>${layeredHtml}</div>`
      : '';

    return `<div class="foundation-section"><h5>Premise</h5><div>${premise}</div></div>
      ${layeredSection}
      <div class="foundation-section"><h5>Ch\u01b0\u01a1ng \u0111\u00e3 tri\u1ec3n khai (${outlineList.length})</h5>${outlineHtml}</div>
      <div class="foundation-section"><h5>Nh\u00e2n v\u1eadt (${charList.length})</h5>${charsHtml}</div>
      <div class="foundation-section"><h5>World rules (${rules.length})</h5>${rulesHtml}</div>`;
  } catch (e) {
    return '<p class="muted">L\u1ed7i t\u1ea3i n\u1ec1n m\u00f3ng: ' + escapeHtml(String(e)) + '</p>';
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
  const reviewNotice = run.status === 'awaiting_review'
    ? '<div class="run-review-notice">\ud83d\udd0d N\u1ec1n m\u00f3ng (premise/outline/th\u1ebf gi\u1edbi/nh\u00e2n v\u1eadt) \u0111\u00e3 xong, engine \u0111\u00e3 t\u1ea1m d\u1eebng \u0111\u1ec3 b\u1ea1n duy\u1ec7t tr\u01b0\u1edbc khi vi\u1ebft ph\u1ea7n l\u1edbn truy\u1ec7n. \u0110\u1ecdc b\u00ean d\u01b0\u1edbi r\u1ed1i Duy\u1ec7t, ho\u1eb7c T\u1eeb ch\u1ed1i n\u1ebfu sai h\u01b0\u1edbng. (T\u1ea1m d\u1eebng ki\u1ec3m m\u1ed7i 5 gi\u00e2y \u2014 t\u1ec7 nh\u1ea5t Writer c\u00f3 th\u1ec3 \u0111\u00e3 k\u1ecbp b\u1eaft \u0111\u1ea7u ch\u01b0\u01a1ng 1.)</div>'
    : '';
  const seedHtml = run.seededFrom
    ? `<div class="stat"><span class="stat-label">Seed t\u1eeb workspace</span><span class="stat-value">${run.seededFrom.completedChapters || 0} ch\u01b0\u01a1ng</span></div>`
    : '';

  const canStart = run.status === 'queued';
  const canStop = run.status === 'running' || run.status === 'paused';
  const canExport = run.chapters > 0;
  const canDelete = run.status !== 'running' && run.status !== 'paused' && run.status !== 'awaiting_review';
  const canSync = run.chapters > 0 && run.status !== 'running' && run.status !== 'paused';
  const canApprove = run.status === 'awaiting_review';
  const canReject = run.status === 'awaiting_review';

  const previewInner = foundationPreviewCache[run.id] || '\u0110ang t\u1ea3i\u2026';
  const foundationHtml = run.status === 'awaiting_review'
    ? '<h4>N\u1ec1n m\u00f3ng \u0111\u1ec3 duy\u1ec7t</h4>'
      + `<div class="run-foundation-preview" id="runFoundationPreview">${previewInner}</div>`
      // Cách 1: sửa tay (chính xác) — khuyên dùng cho chỉnh nhỏ.
      + '<div class="run-manual-box">'
      + '<div class="run-edit-title">\u270f\ufe0f S\u1eeda ch\u00ednh x\u00e1c b\u1eb1ng tay (khuy\u00ean d\u00f9ng)</div>'
      + '<p class="muted">M\u1edf th\u01b0 m\u1ee5c, s\u1eeda file r\u1ed3i b\u1ea5m <strong>Duy\u1ec7t</strong>. An to\u00e0n: <code>premise.md</code>, <code>characters.json</code>, <code>world_rules.json</code>, goal/theme trong <code>layered_outline.json</code>. \u26a0 \u0110\u1eebng th\u00eam/b\u1edbt l\u1ebb s\u1ed1 ch\u01b0\u01a1ng \u2014 <code>outline.json</code> \u21d4 <code>layered_outline.json</code> \u21d4 <code>progress.json</code> ph\u1ea3i kh\u1edbp.</p>'
      + `<button class="btn" data-action="reveal" data-run-id="${escapeHtml(run.id)}">\ud83d\udcc2 M\u1edf th\u01b0 m\u1ee5c n\u1ec1n m\u00f3ng</button>`
      + '</div>'
      // Cách 2: nhờ AI viết lại (KHÔNG phải sửa từng dòng).
      + '<div class="run-revise-box">'
      + '<div class="run-edit-title">\ud83e\udd16 Nh\u1edd AI vi\u1ebft l\u1ea1i theo g\u00f3p \u00fd</div>'
      + '<div class="run-revise-warn">\u26a0 <strong>Vi\u1ebft n\u1ec1n m\u00f3ng M\u1edaI HO\u00c0N TO\u00c0N</strong> t\u1eeb profile + g\u00f3p \u00fd n\u00e0y \u2014 KH\u00d4NG s\u1eeda t\u1eebng d\u00f2ng. T\u00ean nh\u00e2n v\u1eadt, ch\u01b0\u01a1ng, c\u1ea3 volume kh\u00e1c \u0111\u1ec1u c\u00f3 th\u1ec3 \u0111\u1ed5i. T\u1ea1o job M\u1edaI (job c\u0169 gi\u1eef l\u1ea1i l\u00e0m d\u1ef1 ph\u00f2ng). Chi ph\u00ed ~$0.01 sinh n\u1ec1n m\u00f3ng. D\u00f9ng khi mu\u1ed1n \u0111\u1ed5i \u0111\u1ecbnh h\u01b0\u1edbng t\u1ed5ng th\u1ec3, kh\u00f4ng ph\u1ea3i ch\u1ec9nh v\u1eb7t.</div>'
      + '<textarea id="runReviseNote" rows="3" placeholder="vd: r\u00fat c\u00f2n 3 volume, k\u1ebft bi k\u1ecbch h\u01a1n, B\u1ea1ch D\u1ea1 c\u00f3 \u0111\u1ed9ng c\u01a1 \u0111\u1ed3ng c\u1ea3m\u2026"></textarea>'
      + `<button class="btn" data-action="revise" data-run-id="${escapeHtml(run.id)}">\u270e S\u1eeda &amp; t\u1ea1o l\u1ea1i (vi\u1ebft m\u1edbi)</button>`
      + '</div>'
    : '';

  detail.innerHTML = `
    <div class="run-detail-card">
      <div class="run-detail-head">
        <h2>${escapeHtml(run.name || run.id)}</h2>
        <span class="run-badge run-badge-${run.status}">${escapeHtml(statusLabel(run.status))}</span>
      </div>
      ${pauseNotice}
      ${reviewNotice}
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
        <button class="btn primary" data-action="approve" data-run-id="${escapeHtml(run.id)}" ${!canApprove ? 'disabled' : ''}>\u2713 Duy\u1ec7t, ti\u1ebfp t\u1ee5c vi\u1ebft</button>
        <button class="btn danger" data-action="reject" data-run-id="${escapeHtml(run.id)}" ${!canReject ? 'disabled' : ''}>\u2715 T\u1eeb ch\u1ed1i</button>
        <button class="btn danger" data-action="stop" data-run-id="${escapeHtml(run.id)}" ${!canStop ? 'disabled' : ''}>\u25a0 D\u1eebng</button>
        <button class="btn" data-action="export" data-run-id="${escapeHtml(run.id)}" ${!canExport ? 'disabled' : ''}>\u2b07 Xu\u1ea5t TXT</button>
        <button class="btn" data-action="sync" data-run-id="${escapeHtml(run.id)}" ${!canSync ? 'disabled' : ''}>\ud83d\udd04 \u0110\u1ed3ng b\u1ed9</button>
        <button class="btn danger" data-action="delete" data-run-id="${escapeHtml(run.id)}" ${!canDelete ? 'disabled' : ''}>\ud83d\uddd1 X\u00f3a</button>
      </div>
      ${run.possiblyOrphaned ? '<div class="run-orphan-warning">\u26a0 Job n\u00e0y c\u00f3 th\u1ec3 c\u00f2n ti\u1ebfn tr\u00ecnh con m\u1ed3 c\u00f4i. Ki\u1ec3m tra PID ' + (run.childPid || '\u2014') + '.</div>' : ''}
      ${foundationHtml}
      <h4>Nh\u1eadt k\u00fd</h4>
      ${logHtml}
    </div>`;

  // Chỉ fetch preview một lần cho mỗi run (nội dung tĩnh). Poll 5s re-render lại
  // panel nhưng dùng cache nên không nhấp nháy "Đang tải…" và không fetch lại.
  if (run.status === 'awaiting_review' && !foundationPreviewCache[run.id]) {
    const html = await loadFoundationPreview(run.id);
    foundationPreviewCache[run.id] = html;
    const previewEl = $('#runFoundationPreview');
    if (previewEl) previewEl.innerHTML = html;
  }
}

function statusLabel(status) {
  const map = {
    queued: 'Ch\u1edd',
    running: '\u0110ang ch\u1ea1y',
    paused: 'T\u1ea1m d\u1eebng',
    awaiting_review: 'Ch\u1edd duy\u1ec7t n\u1ec1n m\u00f3ng',
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
