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
          <div class="placeholder">
            <p>Ch\u1ecdn m\u1ed9t job b\u00ean tr\u00e1i \u0111\u1ec3 xem chi ti\u1ebft.</p>
            <p class="muted help-text">
              <strong>Ch\u1ebf \u0111\u1ed9 S\u1ea3n xu\u1ea5t = headless:</strong> job ch\u1ea1y tr\u00ean server, kh\u00f4ng m\u1edf TUI nh\u01b0 ch\u1ebf \u0111\u1ed9 th\u01b0\u1eddng.
              Cook li\u00ean t\u1ee5c \u0111\u1ebfn khi \u0111\u1ea1t m\u1ee5c ti\u00eau ho\u1eb7c h\u1ebft budget. D\u00f9ng \u201c\u0110\u1ed3ng b\u1ed9\u201d \u0111\u1ec3 \u0111\u1ed5 k\u1ebft qu\u1ea3 v\u1ec1 workspace ch\u00ednh, ho\u1eb7c \u201cD\u1eebng\u201d \u0111\u1ec3 xem t\u1eebng ch\u01b0\u01a1ng \u0111\u00e3 vi\u1ebft.
            </p>
          </div>
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
              <!-- Step 1: Brief Template -->
              <div class="studio-step">
                <div class="step-header"><span class="step-num">1</span> Brief c\u01a1 b\u1ea3n</div>
                <div class="step-hint">Ch\u1ecdn template c\u00f3 s\u1eb5n HO\u1eb6C \u0111i\u1ec1n tr\u1ef1c ti\u1ebfp v\u00e0o \u00f4 markdown</div>
                <div class="genre-template-chips">
                  <button class="chip-btn" data-template="werewolf">Werewolf</button>
                  <button class="chip-btn" data-template="dark-romance">Dark Romance</button>
                  <button class="chip-btn" data-template="billionaire">Billionaire</button>
                  <button class="chip-btn" data-template="second-chance">Second Chance</button>
                  <button class="chip-btn" data-template="fantasy">Fantasy</button>
                  <button class="chip-btn" data-template="tien-hiep">Tiên hiệp</button>
                  <button class="chip-btn" data-template="huyen-huyen">Huyền huyễn</button>
                  <button class="chip-btn" data-template="lit-rpg">LitRPG</button>
                  <button class="chip-btn" data-template="trinh-tham">Trinh thám</button>
                  <button class="chip-btn" data-template="xuyen-khong">Xuyên không</button>
                </div>
                <textarea id="studioBriefTemplate" rows="6" placeholder="M\u1eabu brief markdown... (click template \u0111\u1ec3 \u0111i\u1ec1n)"></textarea>
              </div>

              <!-- Step 2: \u00dd t\u01b0\u1edfng th\u00f4 -->
              <div class="studio-step">
                <div class="step-header"><span class="step-num">2</span> \u00dd t\u01b0\u1edfng th\u00f4 <span class="step-optional">(t\u00f9y ch\u1ecdn \u2014 AI d\u00f9ng n\u1ebfu c\u00f3)</span></div>
                <div class="step-fields">
                  <div class="field"><label for="studioIdea">\u00dd t\u01b0\u1edfng th\u00f4</label><textarea id="studioIdea" rows="2" placeholder="vd: b\u1ecb \u0111u\u1ed5i h\u1ecdc, v\u00f4 t\u00ecnh l\u1ea1c v\u00e0o th\u1ebf gi\u1edbi werewolf"></textarea></div>
                  <div class="field-row">
                    <div class="field"><label for="studioGenre">Th\u1ec3 lo\u1ea1i</label><input type="text" id="studioGenre" placeholder="werewolf"></div>
                    <div class="field"><label for="studioPlatform">Platform</label><input type="text" id="studioPlatform" placeholder="WebNovel, KDP"></div>
                  </div>
                  <div class="field-row">
                    <div class="field"><label for="studioLang">Ng\u00f4n ng\u1eef</label><input type="text" id="studioLang" value="Ti\u1ebfng Vi\u1ec7t"></div>
                    <div class="field"><label for="studioChapters">S\u1ed1 ch\u01b0\u01a1ng</label><input type="number" id="studioChapters" min="1" value="300"></div>
                  </div>
                  <div class="field"><label for="studioStyle">Phong c\u00e1ch / y\u00eau c\u1ea7u</label><input type="text" id="studioStyle" placeholder="vd: \u0111\u1ea5u tr\u00ed, main th\u00f4ng minh"></div>
                </div>
              </div>

              <!-- Step 3: Sinh profile -->
              <div class="studio-step">
                <div class="step-header"><span class="step-num">3</span> Sinh profile</div>
                <div class="field-row">
                  <div class="field">
                    <label for="studioModelUse">Model</label>
                    <select id="studioModelUse">
                      <option value="">M\u1eb7c \u0111\u1ecbnh</option>
                    </select>
                  </div>
                  <div class="field" id="studioModelOverride" hidden>
                    <label for="studioModelProvider">Provider</label>
                    <select id="studioModelProvider"><option value="">\u2014</option></select>
                  </div>
                  <div class="field" id="studioModelSelect" hidden>
                    <label for="studioModel">Model</label>
                    <select id="studioModel"><option value="">\u2014</option></select>
                  </div>
                </div>
                <div class="step-actions">
                  <button class="btn primary" id="studioGenerate">\u2728 T\u1ea1o b\u1eb1ng AI</button>
                  <button class="btn" id="studioCopyForLLM">\ud83d\udccb Copy cho LLM ngo\u00e0i</button>
                </div>
                <div class="step-hint" id="studioHint"></div>
              </div>

              <!-- Step 4: Kết quả -->
              <div class="studio-step studio-step-output">
                <div class="step-header"><span class="step-num">4</span> Kết quả <span class="step-optional">(copy từ AI / dán từ LLM ngoài)</span></div>
                <textarea id="studioOutput" rows="12" placeholder="Profile output sẽ xuất hiện ở đây..."></textarea>
                <div class="studio-review" id="studioReview">
                  <div class="review-head">🔍 Soát nền móng trước khi lưu <span class="step-optional">— profile là gốc, sai ở đây nhân lên hàng trăm chương</span></div>
                  <ul class="review-checklist">
                    <li>Nhân vật chính có want + wound + mâu thuẫn rõ? Tên &amp; tính cách nhất quán, phân biệt được?</li>
                    <li>Tài năng lõi (vd trí tuệ) có nguồn gốc + <strong>giới hạn</strong>? (vô hạn → hoá "thánh", mất căng thẳng)</li>
                    <li><strong>Có thua / trả giá thật</strong> dọc truyện, hay chỉ thắng liên tục? (truyện dài cần nhiều setback, không chỉ 1 bước ngoặt)</li>
                    <li>Phản diện xứng tầm, lặp lại, có mưu riêng — không phải "bộ máy vô danh"? Mồi dài hơi có bị bỏ rơi?</li>
                    <li>Thể loại chính có được cấu trúc phục vụ tương xứng, hay bị nhánh khác lấn → lệch mood/kỳ vọng?</li>
                    <li>Cơ chế lõi (vd mate bond) có <strong>luật + giá + giới hạn</strong> rõ? (mơ hồ → deus ex machina)</li>
                    <li>Mid-pivot cụ thể (~40–60%)? Xung đột lõi đủ sức kéo cả truyện?</li>
                    <li>Có ensemble / tuyến nhân vật phụ đủ nuôi truyện dài?</li>
                    <li>Công thức chương có biến thể, không đơn điệu một mô-típ suốt truyện?</li>
                    <li>Kết = câu hỏi chủ đề (không phải plot beat) và có cái giá? Tên truyện khớp tông của kết?</li>
                    <li>AI-tell: no purple prose, no "không phải X mà là Y", nội tâm lặp, nhịp câu đều đều?</li>
                    <li><strong>Hợp gu thị trường mục tiêu (${new Date().getFullYear()}+)?</strong> Trope/độ nóng/nhịp/nội dung khớp văn hoá đọc nước đích? (vd Tây Ban Nha ≠ Việt Nam ≠ Anh–Mỹ)</li>
                  </ul>
                  <div class="step-actions">
                    <button class="btn" id="studioCopyReview">📋 Copy profile + prompt review cho LLM ngoài</button>
                  </div>
                  <div class="step-hint" id="studioReviewHint"></div>
                </div>
              </div>

              <!-- Save section -->
              <div class="studio-save">
                <div class="field"><label for="profileLibName">T\u00ean file (.md)</label><input type="text" id="profileLibName" placeholder="vd: werewolf-100c"></div>
                <div class="studio-save-actions">
                  <button class="btn primary" id="profileLibSave">\u2705 L\u01b0u Profile</button>
                  <button class="btn danger" id="profileLibDelete" hidden>\ud83d\uddd1 X\u00f3a</button>
                  <span class="muted" id="profileLibHint"></span>
                </div>
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
  initStudioModelSelect();
}

function initStudioModelSelect() {
  const useSelect = $('#studioModelUse');
  const providerSelect = $('#studioModelProvider');
  const modelSelect = $('#studioModel');
  const providerField = $('#studioModelOverride');
  const modelField = $('#studioModelSelect');
  if (!useSelect || useSelect.dataset.initialized === 'true') return;
  useSelect.dataset.initialized = 'true';

  let modelData = { providers: [], models: {}, roles: [] };
  const CUSTOM = '__custom';
  const ROLE_PREFIX = 'role:';

  const addOption = (select, value, label, dataset) => {
    const opt = document.createElement('option');
    opt.value = value;
    opt.textContent = label;
    if (dataset) {
      Object.entries(dataset).forEach(([k, v]) => { opt.dataset[k] = v || ''; });
    }
    select.appendChild(opt);
    return opt;
  };

  const setModelFieldsVisible = (visible) => {
    providerField.hidden = !visible;
    modelField.hidden = !visible;
  };

  const populateModelsForProvider = (provider, selectedModel) => {
    modelSelect.textContent = '';
    addOption(modelSelect, '', '-');
    (modelData.models?.[provider] || []).forEach((m) => addOption(modelSelect, m, m));
    modelSelect.value = selectedModel || '';
  };

  const populateProviderOptions = () => {
    providerSelect.textContent = '';
    addOption(providerSelect, '', '-');
    (modelData.providers || []).forEach((p) => addOption(providerSelect, p, p));
  };

  const applyMode = () => {
    if (useSelect.value === CUSTOM) {
      setModelFieldsVisible(true);
      if (!providerSelect.value && modelData.providers?.length) {
        providerSelect.value = modelData.providers[0];
      }
      populateModelsForProvider(providerSelect.value, modelSelect.value);
      return;
    }
    setModelFieldsVisible(false);
  };

  fetch('/api/models')
    .then(r => r.json())
    .then(data => {
      modelData = data || modelData;
      useSelect.textContent = '';
      // Nhãn mặc định hiện rõ model thực sự được kế thừa (default trong config
      // lúc server khởi động — Studio không đổi theo ⚙ Model runtime).
      const sd = modelData.studioDefault || {};
      const sdModel = sd.model ? `${sd.provider ? sd.provider + '/' : ''}${sd.model}` : '';
      const defaultLabel = sdModel
        ? `M\u1eb7c \u0111\u1ecbnh config: ${sdModel}`
        : 'M\u1eb7c \u0111\u1ecbnh Studio (k\u1ebf th\u1eeba)';
      addOption(useSelect, '', defaultLabel);
      (modelData.roles || [])
        .filter(role => role.key && role.key !== 'default')
        .forEach(role => {
          const label = `${role.label || role.key} \u2014 ${role.provider || '?'} / ${role.model || '?'}`;
          addOption(useSelect, ROLE_PREFIX + role.key, label, {
            provider: role.provider || '',
            model: role.model || '',
          });
        });
      addOption(useSelect, CUSTOM, 'T\u00f9y ch\u1ecdn provider/model...');
      populateProviderOptions();
      applyMode();
    })
    .catch(() => {});

  useSelect.addEventListener('change', applyMode);
  providerSelect.addEventListener('change', () => {
    populateModelsForProvider(providerSelect.value, '');
  });
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
  $('#profileLibNew')?.addEventListener('click', profileLibNewWithConfirm);
  $('#studioGenerate')?.addEventListener('click', generateProfileFromIdea);
  $('#profileLibSave')?.addEventListener('click', profileLibSave);
  $('#studioCopyForLLM')?.addEventListener('click', profileLibCopyForLLM);
  $('#studioCopyReview')?.addEventListener('click', profileLibCopyForReview);
  $('#profileLibDelete')?.addEventListener('click', profileLibDelete);
  $('#profileLibItems')?.addEventListener('click', (e) => {
    const li = e.target.closest('[data-profile-path]');
    if (!li) return;
    if (!confirmDiscardProfileDraft()) return;
    profileLibLoad(li.dataset.profilePath, li.dataset.profileSource);
  });

  // Genre template chips
  document.querySelectorAll('.chip-btn').forEach((btn) => {
    btn.addEventListener('click', () => applyGenreTemplate(btn.dataset.template));
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
    ? 'Job ch\u1ea1y \u1edf ch\u1ebf \u0111\u1ed9 headless (server). Kh\u00f4ng m\u1edf c\u1eeda s\u1ed5 TUI. Kh\u00e1c ch\u1ebf \u0111\u1ed9 th\u01b0\u1eddng \u1edf: kh\u00f4ng c\u00f3 t\u01b0\u01a1ng t\u00e1c tr\u1ef1c ti\u1ebfp khi vi\u1ebft, kh\u00f4ng steer \u0111\u01b0\u1ee3c gi\u1eefa ch\u1eebng, ch\u1ec9 \u0110\u1ed3ng b\u1ed9 k\u1ebft qu\u1ea3 khi xong ho\u1eb7c D\u1eebng \u0111\u1ec3 xem t\u1eebng ph\u1ea7n.'
    : 'Job ch\u1ea1y \u1edf ch\u1ebf \u0111\u1ed9 headless (server) \u2014 kh\u00f4ng m\u1edf c\u1eeda s\u1ed5 TUI nh\u01b0 ch\u1ebf \u0111\u1ed9 th\u01b0\u1eddng. Sinh truy\u1ec7n li\u00ean t\u1ee5c trong sandbox, \u0110\u1ed3ng b\u1ed9 k\u1ebft qu\u1ea3 khi xong.';
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

// Genre-specific profile templates based on MyNovel/KDP best practices.
const GENRE_TEMPLATES = {
  werewolf: `# [Tên truyện Werewolf Romance]

<!-- Template cho truyện Werewolf/Romantasy — thể loại HOT trên MyNovel (ít cạnh tranh, engagement cao) -->

## Thể loại & giọng điệu
Werewolf Romance + Paranormal Fantasy. Giọng điệu: primal passion xen lẫn ngọt ngào. Nhịp: slow burn 30 chương đầu, payoff rõ ở nửa sau. Đối tượng: phụ nữ 18-35 tuổi thích alpha male + supernatural twist.

## Bối cảnh & thế giới quan
Thế giới ngầm werewolf tồn tại song song xã hội loài người. Alpha cai trị bầy bằng sức mạnh + mate bond (ràng buộc định mệnh). Con người không biết werewolf tồn tại trừ khi bị tiết lộ. Có Hội đồng các thế lực siêu nhiên giữ hòa bình. Vi phạm luật bị trừng phạt nghiêm khắc.

## Nhân vật chính & tuyến quan hệ
- Nữ chính: [tên], [nghề nghiệp: VD: bác sĩ/luật sư/nhân viên bình thường], tính cách mạnh mẽ nhưng có vết thương [VD: mồ côi, từng bị phản bội]. Khát khao: được yêu thật sự, không phải vì định mệnh ép buộc. Mâu thuẫn: không tin vào mate bond nhưng không thể cưỡng lại.
- Nam chính: Alpha [tên], [bí mật: VD: giàu có nhưng cô đơn, bị nguyền không thể kiểm soát sức mạnh]. Khát khao: tìm được người chấp nhận cả người lẫn sói. Mâu thuẫn: bản năng sói muốn chiếm hữu nhưng tình yêu đòi hỏi tôn trọng.
- Quan hệ: Căng thẳng từ mate bond không mong muốn + class difference (con người vs Alpha).

## Xung đột cốt lõi
Mate bond là định mệnh nhưng cả hai đều chống lại. Kẻ thù của bầy (rogue werewolf, vampire council, human hunter) buộc họ phải hợp tác. Câu hỏi: tình yêu có thể vượt qua định mệnh ép buộc không?

## Hướng kết (theo chủ đề, không phải tên chương)
HEA — họ chấp nhận mate bond, coi đó là khởi đầu cho chặng đường mới. Cách họ chiến thắng kẻ thù + hiểu nhau tạo nên payoff thực sự.

## Điểm bán khác biệt (≥3)
- [VD: Mate bond là quá trình chấp nhận dần — tình cảm xây qua thời gian, né trope "love at first sight"]
- [VD: Alpha không possessive tức thì mà phải học cách tôn trọng]
- [VD: World-building có Hội đồng với luật lệ cụ thể, không phải thế giới hỗn loạn]
- [VD: Có mystery subplot về nguồn gốc mate bond]

## Công thức chương
Mở: Hậu quả cliffhanger nhẹ từ chương trước + POV nội tâm. Giữa: Phát triển mối quan hệ / subplot. Kết: Mở ra tension mới HOẶC hoàn thành một arc nhỏ.

## Điều cần tránh
- Alpha "ảo tưởng" — không kiểm soát được bản thân, ghen tuông vô lý.
- Nữ chính "yếu đuối bất ngờ" — không có backbone riêng.
- Mate bond "easy" — không đủ conflict để justify slow burn.
- Purple prose mô tả bối cảnh; dialogue exposition thừa.

## Độ dài & phong cách
60-80 chương, ~2500 từ/chương. Văn phong: immersive, sensory, nội tâm sâu. Ngôn ngữ: tiếng Việt tự nhiên, không cứng nhắc.`,

  'dark-romance': `# [Tên truyện Dark Romance]

<!-- Template cho Dark Romance — engagement cao nhất trên MyNovel, đặc biệt với mystery + kidnapping tropes -->

## Thể loại & giọng điệu
Dark Romance + Psychological Thriller. Giọng điệu: bóng tối, nguy hiểm, tension căng thẳng. Nhịp: build đều, không rushing, payoff mạnh ở climax. Đối tượng: độc giả thích "morally grey" characters, cao trào cảm xúc.

## Bối cảnh & thế giới quan
[VD: Thế giới ngầm mafia / crime syndicate / kidnap ring]. Nam chính có quyền lực tuyệt đối trong lãnh địa của hắn. Nữ chính bị bắt / sáp nhập / ép buộc phải ở đó. Luật ngầm: kẻ yếu không có quyền lên tiếng.

## Nhân vật chính & tuyến quan hệ
- Nữ chính: [tên], [hoàn cảnh: VD: con gái người tốt nhưng bị nhầm là "của cải" của antagonist]. Tính cách: thông minh, kiên cường, không đầu hàng dù bị ép. Vết thương: [VD: mất gia đình, từng bị phản bội]. Khát khao: sống sót + tìm lại quyền tự quyết. Mâu thuẫn: bắt đầu sợ hãi nhưng dần hiểu kẻ bắt có lý do riêng.
- Nam chính: [tên], [vai trò: VD: mafia boss / kidnapper chuyên nghiệp]. Tính cách: lạnh lùng, tính toán, nhưng có mã não xấu bảo vệ [VD: người anh đã chết vì bị phản bội]. Khát khao: kiểm soát tuyệt đối nhưng thực ra sợ hãi mất kiểm soát. Mâu thuẫn: biết hành động của mình sai nhưng không thể dừng.
- Quan hệ: Bắt đầu là captor/captive, dần chuyển thành obsessive devotion. Tension: physical danger + psychological manipulation + forbidden attraction.

## Xung đột cốt lõi
Anh bắt nhầm / cô bị lôi vào thế giới ngầm nguy hiểm. Họ phát triển connection bất chấp hoàn cảnh. Nhưng: có kẻ thù chung đòi hỏi họ phải chọn — tự do hay nhau? Câu hỏi: tình yêu có thể nở từ đất đen không?

## Hướng kết (theo chủ đề, không phải tên chương)
Có thể HEA hoặc bittersweet — tuỳ mức dark bạn muốn. Nếu HEA: anh từ bỏ quyền lực để ở bên cô. Nếu bittersweet: cô thoát ra nhưng phải chấp nhận góc khuất trong anh không bao giờ thay đổi hoàn toàn.

## Điểm bán khác biệt (≥3)
- [VD: Anti-hero không "redeemed" dễ dàng — phải đấu tranh thật sự]
- [VD: Nữ chính không "broken bird" — cô chiến đấu bằng trí tuệ không phải sự yếu đuối]
- [VD: Mystery subplot đan xen — ai là kẻ thù thật sự?]
- [VD: Psychological depth — khám phá trauma và cách nó shape hành vi]`

,

  billionaire: `# [Tên truyện Billionaire Romance]

<!-- Template cho Romance + Billionaire/Millionaire — thể loại phổ biến nhất trên MyNovel, dễ tiếp cận độc giả mới -->

## Thể loại & giọng điệu
Contemporary Romance + Billionaire/Millionaire. Giọng điều: ngọt ngào xen lẫn witty banter. Nhịp: meet-cute → conflict → attraction → obstacle → resolution. Đối tượng: độc giả thích power fantasy (gái thường gặp hoàng tử).

## Bối cảnh & thế giới quan
Thế giới doanh nhân giàu có: công ty lớn, sự kiện thượng lưu, mâu thuẫn gia tộc. Nữ chính thường ở "bottom rung" — assistant, nghèo khó, hoặc bị giáng chức. Bối cảnh cụ thể: [VD: tập đoàn đối thủ, công ty startup bị nuốt, gia đình cần tiền].

## Nhân vật chính & tuyến quan hệ
- Nữ chính: [tên], [nghề nghiệp: VD: trợ lý/đầu bếp/nhân viên bình thường]. Tính cách: độc lập, có nguyên tắc, không vị lợi. Vết thương: [VD: cha mẹ ly dị khiến cô không tin hôn nhân]. Khát khao: thành công trong sự nghiệp mà không phải hy sinh giá trị. Mâu thuẫn: bị hấp dẫn bởi quyền lực nhưng sợ bị mất chính mình.
- Nam chính: [tên], [vai trò: VD: CEO trẻ nổi tiếng / người thừa kế]. Tính cách: lạnh lùng bề ngoài, ấm áp bên trong. Bí mật: [VD: bị ép buộc phải cưới vì lợi ích gia đình]. Khát khao: được yêu vì con người thật, không phải tài khoản. Mâu thuẫn: cam kết ngăn cản tình cảm, nhưng không thể cưỡng lại cô.
- Quan hệ: Bắt đầu vì hợp đồng / circumstance không mong muốn → phát triển attraction thật. Trope phổ biến: Contract Marriage, Fake Relationship, Marriage of Convenience.

## Xung đột cốt lõi
Họ phải giả vờ [yêu/cưới/hợp tác] vì lý do ngoài tình yêu. Trong quá trình giả, họ phát hiện đối phương không như stereotype. Nhưng: gia đình / đối thủ / hoàn cảnh cũ ngăn cản họ ở bên nhau thật. Câu hỏi: tiền có thể mua hạnh phúc không?

## Hướng kết (theo chủ đề, không phải tên chương)
HEA rõ ràng — họ chọn nhau bất chấp áp lực bên ngoài. Điểm payoff: nam chính từ bỏ [điều gì đó quan trọng với anh] để chứng minh tình yêu quan trọng hơn.

## Điểm bán khác biệt (≥3)
- [VD: Hero không "fixed" bằng tình yêu — cô giúp anh thay đổi từ bên trong]
- [VD: Có subplot về self-worth — cô không cần anh để được công nhận]
- [VD: Supporting characters đáng nhớ — best friend/sassy secretary/comic relief]
- [VD: World-building thực sự — business drama không chỉ là backdrop]`

,

  'second-chance': `# [Tên truyện Second Chance Romance]

<!-- Template cho Romance + Second Chance / Second Chance at Love — trope có engagement cao, emotional depth sâu -->

## Thể loại & giọng điệu
Contemporary Romance + Second Chance / Friends to Lovers to Enemies to Lovers. Giọng điệu: nostalgic, bittersweet, emotional payoff mạnh. Nhịp: backstory song song với present, gradual revelation. Đối tượng: độc giả thích emotional journey, không chỉ physical attraction.

## Bối cảnh & thế giới quan
Thiết lập present: [VD: thành phố nhỏ / thành phố lớn / hometown họ từng rời đi]. Quá khứ: [VD: college, first job, summer fling] nơi họ yêu nhau lần đầu. Present: họ vô tình gặp lại sau nhiều năm.

## Nhân vật chính & tuyến quan hệ
- Nữ chính: [tên], [cuộc sống hiện tại: VD: thành công nhưng cô đơn, quay về hometown vì lý do]. Vết thương từ breakup: [VD: anh chọn [điều khác] thay vì cô, cô mất [điều quan trọng] vì anh]. Khát khao: closure hoặc second chance. Mâu thuẫn: muốn tin lại nhưng sợ bị tổn thương cùng một cách.
- Nam chính: [tên], [cuộc sống hiện tại: VD: ở lại hometown, thành công theo cách khác]. Vết thương từ breakup: [VD: cô rời đi, anh không theo được]. Khát khao: được tha thứ, được cơ hội sửa sai. Mâu thuẫn: tự hào không cho phép anh thừa nhận sai lầm.
- Quan hệ: Họ đã có history — memories đan xen present. Căng thẳng từ: chưa tha thứ + still attracted + hoàn cảnh buộc họ tương tác.

## Xung đột cốt lõi
Nguyên nhân chia tay ngày xưa vẫn tồn tại dưới dạng [VD: unfulfilled dream, different life goals, external pressure]. Họ phải đối mặt: điều gì thực sự chia cách họ? Họ đã thay đổi chưa? Họ có thể thay đổi để ở bên nhau không?

## Hướng kết (theo chủ đề, không phải tên chương)
HEA — họ nhận ra [điều khiến họ chia tay] không còn là rào cản vì họ đã thay đổi / vì hoàn cảnh đã khác. Payoff emotional đến từ một khoảnh khắc nhỏ phơi bày sự yếu lòng, né khuôn "grand gesture".

## Điểm bán khác biệt (≥3)
- [VD: Không "misunderstanding" rẻ tiền — nguyên nhân chia tay thật sự và đáng weight]
- [VD: Backstory reveal gradual, không dump tất cả một lần]
- [VD: Supporting characters phản ánh main couple's journey]
- [VD: Emotional growth từ cả hai phía, không chỉ một người thay đổi]`

,

  fantasy: `# [Tên truyện Fantasy Romance]

<!-- Template cho Fantasy/Adventure + Romance elements — cho thể loại Romantasy, Epic Fantasy romance, Magical Academy -->

## Thể loại & giọng điệu
Fantasy + Romance (Romantasy / Epic Fantasy với romance core). Giọng điệu: epic world-building xen lẫn intimate character moments. Nhịp: alternating giữa high stakes plot và romantic development. Đối tượng: độc giả thích world-building sâu + slow burn romance.

## Bối cảnh & thế giới quan
Thế giới fantasy với [VD: magic system có giới hạn rõ ràng, political factions, prophecy cần hoàn thành]. Địa lý cụ thể: [VD: kingdom đang bị đe dọa, academy đào tạo mages, borderland giữa territories]. Quy tắc magic/physics của thế giới phải nhất quán.

## Nhân vật chính & tuyến quan hệ
- Nữ chính: [tên], [vai trò: VD: orphan với hidden power, princess không muốn ngai vàng, apprentice mới vào academy]. Khát khao: [VD: tìm identity, bảo vệ những người yêu thương, hoàn thành prophecy]. Mâu thuẫn: [VD: power của cô có thể save hoặc destroy, cô không biếttrust ai].
- Nam chính: [tên], [vai trò: VD: rival/broody mage, prince với secret, mentor có agendar]. Khát khao: [VD: revenge, redemption, protecting kingdom]. Mâu thuẫn: [VD: feelings for cô conflict với duty/past trauma].
- Quan hệ: Often start as rivals/unequal power dynamic → enemies-to-allies → slow burn attraction.

## Xung đột cốt lõi
[Major plot conflict: prophecy cần fulfill, war sắp xảy ra, kingdom đang bị threaten]. Romance xảy ra trong context này. Câu hỏi: khi duty và love conflict, họ chọn gì?

## Hướng kết (theo chủ đề, không phải tên chương)
Tuỳ scope: có thể series hoặc standalone. Nếu standalone: main plot resolve + romance payoff. Nếu series: main conflict resolve, romance open-ended for continuation.

## Điểm bán khác biệt (≥3)
- [VD: Magic system có rules, không phải deus ex machina]
- [VD: World-building phục vụ plot, không dump lore không cần thiết]
- [VD: Romance enhances plot, không phải distraction from plot]
- [VD: Supporting cast đa chiều, không chỉ là billboard cho main couple]`,

  'tien-hiep': `# [Tên truyện Tiên hiệp / Tu tiên]

<!-- Template Tiên hiệp / Tu tiên (Cultivation). Hợp gu WebNovel, KDP; VN: webnovel.vn/Waka/TruyenFull. Engine style: fantasy -->

## Thể loại & giọng điệu
Tiên hiệp / Tu tiên (Cultivation / Xianxia). Giọng điệu: hào hùng, kỳ vĩ. Nhịp: progression đều — mỗi cảnh giới là một arc, đột phá là cao trào. Đối tượng: độc giả thích hệ thống năng lượng rõ, main từ đáy vươn lên đỉnh.

## Bối cảnh & thế giới quan
Thế giới tu tiên chia cảnh giới rõ (luyện khí → trúc cơ → kim đan → nguyên anh → ...). Linh khí hữu hạn; tranh tài nguyên là nguồn xung đột chính. Ba lớp thế lực: tông môn, thế gia, tán tu. Thiên kiếp ràng buộc mỗi lần thăng cấp — đột phá luôn có cái giá.

## Nhân vật chính & tuyến quan hệ
- Main: [tên], xuất thân thấp (tán tu / phế thể / tông môn ngoại biên). Vết thương: [VD: gia tộc bị diệt, cơ duyên bị đoạt]. Khát khao: chứng minh bản thân, đạt đỉnh cao tu đạo. Điểm mù: kiêu ngạo sau đột phá, hoặc quá tín một đạo pháp tới mức mù quáng.
- Đối thủ ngang cơ: [tên] — tài năng tương đương, đạo khác (chính vs ma), đối đầu lặp lại xuyên nhiều cảnh giới.
- Quan hệ: sư đồ có căng thẳng (ân tình xen lợi dụng); đạo lữ đồng tiến nhưng có thể phân kỳ về đạo.

## Xung đột cốt lõi
Đạo tâm bị thử thách xuyên suốt: tài nguyên khan hiếm buộc tranh đoạt, mỗi đột phá là thiên kiếp có thể ngã. Câu hỏi: lên đỉnh cao bằng đạo nào mà còn giữ được bản tâm?

## Hướng kết (theo chủ đề)
Main đột phá cảnh giới tối thượng nhưng nhận ra đỉnh cao ấy để lại cái giá thật (mất nhân duyên, cắt rễ phàm, hoặc giác ngộ rằng tu tiên là buông bỏ). Trả lời câu hỏi đạo tâm.

## Điểm bán khác biệt (≥3)
- [VD: Hệ thống cảnh giới có sáng tạo, khác formula cũ]
- [VD: Thiên kiếp có luật riêng — đột phá là high stakes, có trả giá]
- [VD: Đạo tâm là xung đột thật — main từng thất đạo rồi tìm lại]

## Công thức chương
Mở: hệ quả lần tu hành trước. Giữa: gặp cơ duyên / đối đầu / bế quan. Kết: đột phá nhỏ hoặc hé lộ thế lực lớn hơn phía trước.

## Điều cần tránh
- Main "nghịch thiên" vô lý — đột phá nhờ deus ex machina, không trả giá.
- Phản diện chỉ làm nền, không có mưu đồ riêng.
- Dump setting cảnh giới dài không phục vụ plot.
- Cliché lặp: "khí thế ngút trời", "sắc mặt đại biến".

## Độ dài & phong cách
200-400 chương, ~2500-3000 từ/chương. Văn phong: kỳ vĩ, tiết chế. Ngôn ngữ: tiếng Việt.`,

  'huyen-huyen': `# [Tên truyện Huyền huyễn / Đông phương huyền bí]

<!-- Template Huyền huyễn (Eastern Fantasy). Hợp gu WebNovel, KDP; VN: webnovel.vn/Waka/TruyenFull. Engine style: fantasy -->

## Thể loại & giọng điệu
Huyền huyễn / Đông phương huyền bí (Eastern Fantasy). Giọng điệu: bí ẩn, kỳ ảo, đôi lúc u ám. Nhịp: khám phá + leo thang sức mạnh đan xen. Đối tượng: độc giả thích thế giới bí ẩn, dị thú, hệ thống lực lượng có quy tắc.

## Bối cảnh & thế giới quan
Thế giới huyền bí nơi lực lượng siêu nhiên (võ hồn, linh thú, huyết mạch, thần thông) tuân theo quy tắc rõ. Có dị vực / cấm địa / di tích cổ chứa bảo vật và hiểm nguy. Thế lực giang hồ và tổ chức ẩn vận hành ngầm. Mỗi năng lực có giới hạn và cái giá khi lạm dụng.

## Nhân vật chính & tuyến quan hệ
- Main: [tên], [VD: thức tỉnh võ hồn biến dị / mang huyết mạch cấm / nhặt được bí tịch cổ]. Vết thương: [VD: bị dòng họ ruồng bỏ, mất người thân vì cấm địa]. Khát khao: tìm chân tướng nguồn sức mạnh, vươn lên. Điểm mù: tin quá vào lực lượng mới có được, khinh thường thế lực cũ.
- Đối thủ: [tên] — nắm giữ thế lực / bí mật liên quan nguồn gốc sức mạnh của main, đối đầu lặp lại.
- Quan hệ: đồng bạn vong niên, sư truyền ẩn danh; mỗi người mang bí mật riêng sẽ nổ ra sau.

## Xung đột cốt lõi
Sức mạnh main có được gắn với một bí mật / lời nguyền lớn — càng mạnh càng gần chân tướng nguy hiểm. Câu hỏi: sức mạnh ấy là ân hay nghiệt, và main sẽ dùng nó cho ai?

## Hướng kết (theo chủ đề)
Main nắm được chân tướng cội nguồn, phải chọn: giữ sức mạnh với cái giá nặng, hay buông bỏ để bảo vệ điều mình trân trọng. Câu hỏi chủ đề được trả lời.

## Điểm bán khác biệt (≥3)
- [VD: Hệ thống lực lượng có quy tắc sáng tạo, cân bằng]
- [VD: Cấm địa / di tích là high stakes thật, vào là có thương vong]
- [VD: Bí mật main đuổi theo dẫn tới phản diện cá nhân xứng tầm]

## Công thức chương
Mở: dấu hiệu thế lực / hiện tượng huyền bí mới. Giữa: khám phá + đối đầu + thi triển lực. Kết: hé lộ tầng bí mật sâu hơn.

## Điều cần tránh
- Năng lực vô hạn — phá quy tắc thế giới lúc cần.
- Cấm địa "miễn phí" — vào ra lành không trả giá.
- Dump lore thần thông không phục vụ plot.
- Cliché: chỉ xếp hàng lực lượng, thiếu tâm lý nhân vật.

## Độ dài & phong cách
200-400 chương, ~2500-3000 từ/chương. Văn phong: kỳ ảo, gợi hình. Ngôn ngữ: tiếng Việt.`,

  'lit-rpg': `# [Tên truyện LitRPG / Progression Fantasy]

<!-- Template LitRPG / Progression Fantasy. Hợp gu Royal Road → Patreon, KDP. Engine style: fantasy -->

## Thể loại & giọng điệu
LitRPG / Progression Fantasy. Giọng điệu: năng động, cuốn hút, có giọng hệ thống. Nhịp: grind + level-up + challenge leo thang rõ ràng. Đối tượng: độc giả thích số liệu, build, power curve rõ ràng.

## Bối cảnh & thế giới quan
Thế giới vận hành theo "hệ thống": cấp độ, chỉ số, kỹ năng, dungeon, phần thưởng. Có thể là VR game, apocalypse hệ thống, hoặc thế giới khác bị áp đặt luật hệ thống. Quy tắc hệ thống phải rõ, nhất quán; có PvP / bảng xếp hạng / kỳ thi để so sánh.

## Nhân vật chính & tuyến quan hệ
- Main: [tên], [VD: class hiếm / build ngoài lệ / bắt đầu với bất lợi]. Vết thương: [VD: bị phát hiện sức mạnh ẩn, mất đội cũ]. Khát khao: leo top, sinh tồn, tìm lối ra. Điểm mù: quá tin build của mình tới mức khinh địch trước build khắc.
- Đối thủ: [tên] — build khắc hệ / đang giữ top, đối đầu trong dungeon và kỳ thi.
- Quan hệ: đội hình cố định có căng thẳng (phân chia tài nguyên, rời đội).

## Xung đột cốt lõi
Hệ thống buộc leo thang — dừng lại là bị bỏ lại hoặc chết. Mỗi tier mới có rào cản thật (dungeon boss, kỳ thi, PvP). Câu hỏi: leo lên đỉnh hệ thống bằng giá nào, và hệ thống giấu chân tướng gì?

## Hướng kết (theo chủ đề)
Main đạt top hoặc phá vỡ hệ thống, nhưng nhận ra đỉnh đó có cái giá (mất đồng đội, phát hiện chân tối của hệ thống). Trả lời câu hỏi chủ đề.

## Điểm bán khác biệt (≥3)
- [VD: Build main sáng tạo, dựa vào chiến thuật thay vì stat brute-force]
- [VD: Hệ thống có luật bất ngờ, dungeon khó đoán]
- [VD: Cái giá thật của mỗi level — mất mát đi kèm]

## Công thức chương
Mở: hệ quả lần grind trước. Giữa: vào dungeon / PvP / học kỹ năng. Kết: level-up hoặc mở khóa tầng thử thách lớn hơn.

## Điều cần tránh
- Main "bất bại" — level-up nhờ luck / deus ex machina, không build thật.
- Dump bảng stat dài không phục vụ plot.
- Hệ thống mâu thuẫn luật (chỉ số không khớp).
- Cliché: đột phá qua đêm, phản diện phẳng.

## Độ dài & phong cách
150-300 chương, ~2500-3000 từ/chương. Văn phong: năng động, rõ số liệu. Ngôn ngữ: tiếng Việt.`,

  'trinh-tham': `# [Tên truyện Trinh thám / Thriller]

<!-- Template Trinh thám / Thriller (Mystery). Hợp gu KDP, Royal Road. Engine style: suspense -->

## Thể loại & giọng điệu
Trinh thám / Thriller (Mystery / Psychological Thriller). Giọng điệu: rùng rợn, căng thẳng, logic. Nhịp: mỗi chương một manh mối + một hook, mở màn dần. Đối tượng: độc giả thích đoán, nghi vấn, twist công bằng.

## Bối cảnh & thế giới quan
Bối cảnh cụ thể [VD: thành phố lớn, cộng đồng khép kín, thời đại cụ thể]. Có một vụ việc (án mạng / mất tích / bí ẩn) phá vỡ bề ngoài yên bình. Luật chơi: mọi manh mối phải công bằng — độc giả có cơ hội đoán đúng.

## Nhân vật chính & tuyến quan hệ
- Main: [tên], [VD: thám tử / phóng viên / người có liên quan cá nhân]. Vết thương: [VD: một án chưa giải ở quá khứ, mất người thân]. Khát khao: tìm chân tướng. Điểm mù: thiên kiến cá nhân khiến nghi ngờ sai người, hoặc ám ảnh quá khiến bỏ qua nguy cơ cá nhân.
- Đối thủ: [tên] — thủ phạm / puppeteer ẩn, thông minh ngang main, để lại thử thách trực tiếp.
- Quan hệ: đồng nghiệp / người thân có thể là nghi phạm; lòng tin bị thử.

## Xung đột cốt lõi
Mỗi lớp bí mật mở ra lại dẫn tới tầng sâu hơn; thủ phạm luôn hơn một bước. Câu hỏi: chân tướng có đáng cái giá main phải trả (an toàn, danh dự, người thân)?

## Hướng kết (theo chủ đề)
Main giải được án nhưng phát hiện chân tướng đụng chạm người thân / đòi hy sinh cá nhân. Twist cuối trả lời câu hỏi chủ đề, công bằng với độc giả.

## Điểm bán khác biệt (≥3)
- [VD: Manh mối công bằng — twist có thể đoán được nếu tinh]
- [VD: Thriller tâm lý — thủ phạm ngang cơ, có động cơ thật]
- [VD: Cái giá giải án — main trả giá thật, không toàn thắng]

## Công thức chương
Mở: manh mối / phát hiện mới. Giữa: nghi vấn + đối đầu + loại trừ. Kết: hook (nghi vấn mới hoặc nguy cơ cá nhân).

## Điều cần tránh
- Deus ex machina — thông tin vụt hiện lúc kết, không có manh mối trước.
- Red herring rẻ tiền — đánh lạc hướng bằng lừa dối thay vì logic.
- Dump procedure điều tra khô khan.
- Cliché: thám tử "thần thánh", không bao giờ sai bước.

## Độ dài & phong cách
60-150 chương, ~2500-3000 từ/chương. Văn phong: cắt gọn, gợi sợ. Ngôn ngữ: tiếng Việt.`,

  'xuyen-khong': `# [Tên truyện Xuyên không / Cổ trang]

<!-- Template Xuyên không / Cổ trang (Isekai / Historical Romance). Hợp gu webnovel.vn, Waka, TruyenFull (VN). Engine style: romance hoặc default -->

## Thể loại & giọng điệu
Xuyên không / Cổ trang (Isekai / Historical Romance). Giọng điệu: kịch tính, mưu trí, xen ngọt ngào. Nhịp: main dùng tri thức hiện đại giải nguy + leo quyền lực. Đối tượng: độc giả Việt thích xuyên việt / cung đấu / tổng tài / cổ đại.

## Bối cảnh & thế giới quan
Main xuyên tới [VD: cổ đại / thế giới song song / vào thân thể kẻ khác]. Quy tắc thế giới mới khắc nghiệt (cung đình, gia tộc, hắc bang). Tri thức hiện đại là lợi thế nhưng có giới hạn (không dùng được nếu thiếu tài nguyên / thế lực).

## Nhân vật chính & tuyến quan hệ
- Nữ chính: [tên], mang ký ức hiện đại, phải thích nghi thân phận mới. Vết thương: [VD: nguyên chủ bị ức hiếp, bị hứa hôn ép buộc]. Khát khao: tự chủ, sinh tồn, kiến tạo cuộc đời mới. Điểm mù: quá tin tri thức hiện đại, khinh thường quy tắc ngầm thời đại đó.
- Nam chính: [tên], [VD: vương gia lạnh lùng / tổng tài / hoàng đế]. Có quyền lực + bí mật. Mâu thuẫn: bị thu hút vì nữ chính khác thường nhưng duty / gia tộc cản.
- Quan hệ: đối đầu → đồng minh → tình cảm; có yếu tố "sủng" rõ (chiều chuộng qua hành động bảo vệ).

## Xung đột cốt lõi
Nữ chính phải sống sót và leo lên trong thế giới có luật khắc nghiệt, dùng mưu trí thay vì sức mạnh để đối phó đối thủ. Câu hỏi: kiến tạo cuộc đời mới có giữ được bản ngã người hiện đại không?

## Hướng kết (theo chủ đề)
Nữ chính giành tự chủ / địa vị, trả lời câu hỏi bản ngã. Có HEA với nam chính nhưng kèm cái giá (cắt đứt với quá khứ, hy sinh điều gì đó). Khớp tên / lời hứa truyện.

## Điểm bán khác biệt (≥3)
- [VD: Nữ chính thắng bằng mưu trí, có điểm mù rõ — không "não tàn vô đối"]
- [VD: Yếu tố "sủng" rõ — nhịp ngọt xen mưu đồ]
- [VD: Quy tắc thế giới mới ràng buộc thật — tri thức hiện đại có giới hạn]

## Công thức chương
Mở: hệ quả mưu chương trước. Giữa: đối phó đối thủ + tiến triển tình cảm. Kết: hé lộ mưu lớn hơn hoặc khoảnh khắc "sủng".

## Điều cần tránh
- Nữ chính "bất bại" — tri thức hiện đại giải quyết mọi thứ, không có giới hạn.
- Thiếu nhịp "sủng" — độc giả Việt ngôn tình cần chiều chuộng rõ.
- Nam chính "tổng tài mẫu bản" — không có chiều sâu riêng.
- Cliché: hiểu lầm rẻ tiền kéo dài, mary sue.

## Độ dài & phong cách
150-300 chương, ~2500-3000 từ/chương. Văn phong: kịch tính + ngọt. Ngôn ngữ: tiếng Việt.`
};

// Apply genre template to Step 1 brief textarea
function applyGenreTemplate(genre) {
  const tpl = GENRE_TEMPLATES[genre];
  if (!tpl) return;
  const existing = $('#studioBriefTemplate').value.trim();
  // Chip-to-chip switching is free: only prompt when the brief holds genuine
  // user content — not empty, not the blank skeleton, and not any chip template
  // (including one already applied). Editing a chip's placeholders makes the
  // text diverge from its template, so an edited brief still prompts.
  const isPristine = existing === ''
    || existing === PROFILE_TEMPLATE.trim()
    || Object.values(GENRE_TEMPLATES).some(t => existing === t.trim());
  if (!isPristine && !confirm('Ghi đè brief hiện tại?')) return;
  $('#studioBriefTemplate').value = tpl;
  // Mark active chip
  document.querySelectorAll('.chip-btn').forEach(b => b.classList.remove('active'));
  document.querySelector(`.chip-btn[data-template="${genre}"]`)?.classList.add('active');
  $('#studioHint').textContent = 'Template "' + genre + '" đã điền. Bổ sung ý tưởng ở Step 2 nếu muốn.';
}

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

function profileLibHasDraft() {
  return !!($('#studioOutput').value.trim() ||
    ($('#studioBriefTemplate').value.trim() && $('#studioBriefTemplate').value.trim() !== PROFILE_TEMPLATE.trim()) ||
    $('#studioIdea').value.trim() ||
    $('#studioGenre').value.trim() ||
    $('#studioPlatform').value.trim() ||
    $('#studioStyle').value.trim());
}

function confirmDiscardProfileDraft(action) {
  return !profileLibHasDraft() || confirm(action + ' s\u1ebd x\u00f3a draft hi\u1ec7n t\u1ea1i \u1edf Profile Studio. Ti\u1ebfp t\u1ee5c?');
}

async function openProfileLibrary() {
  if (!confirmDiscardProfileDraft('M\u1edf Th\u01b0 vi\u1ec7n Profile')) return;
  $('#profileLibOverlay').hidden = false;
  profileLibNew();
  await refreshProfileLibList();
}

function profileLibNewWithConfirm() {
  if (!confirmDiscardProfileDraft('T\u1ea1o profile m\u1edbi')) return;
  profileLibNew();
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
  $('#profileLibDelete').hidden = true;
  $('#profileLibHint').textContent = '';
  // Reset all studio fields
  $('#studioBriefTemplate').value = PROFILE_TEMPLATE;
  $('#studioIdea').value = '';
  $('#studioGenre').value = '';
  $('#studioPlatform').value = '';
  $('#studioLang').value = 'Ti\u1ebfng Vi\u1ec7t';
  $('#studioChapters').value = '300';
  $('#studioStyle').value = '';
  $('#studioOutput').value = '';
  $('#studioHint').textContent = '';
  document.querySelectorAll('.chip-btn').forEach(b => b.classList.remove('active'));
}

async function profileLibLoad(path, source) {
  try {
    const res = await fetch('/api/profiles/content?path=' + encodeURIComponent(path));
    if (!res.ok) { const d = await res.json().catch(() => ({})); toast(d.error || ('HTTP ' + res.status), 'error'); return; }
    const data = await res.json();
    profileLibSelected = { path, source };
    // Load into Step 4 output textarea
    $('#studioOutput').value = data.content || '';
    // Name is the base filename
    const base = (data.name || '').replace(/\.md$/i, '');
    $('#profileLibName').value = base;
    if (source === 'project') {
      $('#profileLibName').readOnly = true;
      $('#profileLibDelete').hidden = false;
      $('#profileLibHint').textContent = 'Profile \u0111\u00e3 t\u1ea3i. S\u1eeda n\u1ebfu c\u1ea7n r\u1ed3i b\u1ea5m "L\u01b0u Profile".';
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

// Generate profile via AI \u2192 output to Step 4 textarea
async function generateProfileFromIdea() {
  const btn = $('#studioGenerate');
  const hint = $('#studioHint');
  const brief = $('#studioBriefTemplate').value.trim();
  const ideaField = $('#studioIdea').value.trim();
  const hasBrief = brief && brief !== PROFILE_TEMPLATE.trim();
  // Guard: need Step 1 brief (non-default) or Step 2 idea
  if (!hasBrief && !ideaField) {
    toast('\u0110i\u1ec1n \u00fd t\u01b0\u1edfng \u1edf Step 1 (template/brief) ho\u1eb7c Step 2 tr\u01b0\u1edbc', 'error');
    return;
  }
  const body = {
    idea: ideaField || undefined,
    genre: $('#studioGenre').value.trim() || undefined,
    platform: $('#studioPlatform').value.trim() || undefined,
    language: $('#studioLang').value.trim() || undefined,
    styleNotes: $('#studioStyle').value.trim() || undefined,
    targetChapters: parseInt($('#studioChapters').value, 10) || undefined,
  };
  const modelMode = $('#studioModelUse')?.value || '';
  if (modelMode === '__custom') {
    body.provider = $('#studioModelProvider')?.value || '';
    body.model = $('#studioModel')?.value || '';
    if (!body.provider || !body.model) {
      toast('Ch\u1ecdn \u0111\u1ee7 provider v\u00e0 model cho Profile Studio', 'error');
      return;
    }
  } else if (modelMode.startsWith('role:')) {
    const sel = $('#studioModelUse').options[$('#studioModelUse').selectedIndex];
    body.provider = sel?.dataset.provider || '';
    body.model = sel?.dataset.model || '';
    if (!body.provider || !body.model) {
      toast('Role model ch\u01b0a c\u00f3 provider/model h\u1ee3p l\u1ec7', 'error');
      return;
    }
  }
  // Feed Step 1 brief into idea if present and not default template
  if (hasBrief) {
    body.idea = [brief, ideaField].filter(Boolean).join('\n\n');
  }
  const prev = btn.textContent;
  btn.disabled = true;
  btn.textContent = '\u23f3 \u0110ang sinh...';
  hint.textContent = '';
  try {
    const res = await post('/api/profiles/generate', body);
    if (res && res.content) {
      $('#studioOutput').value = res.content;
      hint.textContent = '\u2705 \u0110\u00e3 sinh xong! S\u1eeda n\u1ebfu c\u1ea7n r\u1ed3i b\u1ea5m "L\u01b0u Profile".';
      toast('\u0110\u00e3 sinh profile th\u00e0nh c\u00f4ng', 'ok');
      // Auto-fill name from content title
      const match = res.content.match(/^#\s+(.+)/m);
      if (match && !$('#profileLibName').value.trim()) {
        $('#profileLibName').value = match[1].replace(/[^\w\-]/g, '-').toLowerCase().slice(0, 40);
      }
    }
  } catch (e) {
    hint.textContent = '\u274c L\u1ed7i: ' + e;
  } finally {
    btn.disabled = false;
    btn.textContent = prev;
  }
}

async function profileLibSave() {
  const name = $('#profileLibName').value.trim();
  const content = $('#studioOutput').value;
  if (!name) { toast('Nh\u1eadp t\u00ean file', 'error'); return; }
  if (!content.trim()) { toast('N\u1ed9i dung tr\u1ed1ng \u2014 \u0111i\u1ec1n v\u00e0o Step 4 tr\u01b0\u1edbc', 'error'); return; }
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

// Build prompt from all form fields and copy for external LLM
async function profileLibCopyForLLM() {
  const hint = $('#studioHint');
  const brief = $('#studioBriefTemplate').value.trim();
  const idea = $('#studioIdea').value.trim();
  const genre = $('#studioGenre').value.trim();
  const platform = $('#studioPlatform').value.trim();
  const lang = $('#studioLang').value.trim() || 'Ti\u1ebfng Vi\u1ec7t';
  const chapters = $('#studioChapters').value.trim();
  const style = $('#studioStyle').value.trim();

  let payload = `B\u1ea1n l\u00e0 tr\u1ee3 l\u00fd so\u1ea1n "profile" (brief) cho engine vi\u1ebft ti\u1ec3u thuy\u1ebft d\u00e0i.\n\n`;

  if (idea) payload += `## \u00dd t\u01b0\u1edfng th\u00f4\n${idea}\n\n`;
  if (genre) payload += `## Th\u1ec3 lo\u1ea1i\n${genre}\n\n`;
  if (platform) payload += `## N\u1ec1n t\u1ea3ng / th\u1ecb tr\u01b0\u1eddng\n${platform}\n\n`;
  if (chapters) payload += `## S\u1ed1 ch\u01b0\u01a1ng d\u1ef1 ki\u1ebfn\n${chapters}\n\n`;
  if (lang) payload += `## Ng\u00f4n ng\u1eef\n${lang}\n\n`;
  if (style) payload += `## Phong c\u00e1ch / y\u00eau c\u1ea7u\n${style}\n\n`;

  // Sync note: keep the long-novel survival rules here aligned with
  // profile_studio.go (profileStudioSystemPrompt) and buildProfileReviewPrompt.
  const year = new Date().getFullYear();
  payload += `## Trước khi viết (tự xác định, KHÔNG in ra — output chỉ là profile)
- Thể loại & sub-genre ĐÚNG yêu cầu (tôn trọng, đừng trôi về genre mặc định như romance/werewolf)
- Thể loại này đã ĐẠI TRÀ chưa? cliché cần tránh; nếu niche thì phải nail điều gì cho fan cứng
- Đặc trưng / quy ước độc giả thể loại này kỳ vọng (payoff bắt buộc)
- Thị trường mục tiêu & gu tại năm ${year}
Để khung này chi phối toàn bộ profile.

## Nguyên tắc
- Viết CỤ THỂ (tên nhân vật, chi tiết bối cảnh, hướng twist), theo ĐỊNH HƯỚNG & RÀNG BUỘC, ĐỪNG viết sẵn câu văn mẫu
- Phù hợp VĂN HOÁ ĐỌC & THỊ HIẾU thị trường mục tiêu tại năm ${year} (trope/độ "nóng"/bạo lực/nhịp/độ dài/điều cấm kỵ khác nhau theo nước — vd Tây Ban Nha/Mỹ Latinh ≠ Việt Nam ≠ Anh–Mỹ; chú ý mã trope bản địa vs ngoại nhập và kỳ vọng cảm xúc đặc trưng như "sủng" ở ngôn tình VN); tránh trope cũ/nội dung nhạy cảm. Đừng ghi năm/số liệu vào profile

## Nguyên tắc truyện dài (bắt buộc phản ánh)
- CÁI GIÁ thật, không chỉ twist: rải nhiều mất mát để lại dấu vết dài, không chỉ 1 cú vấp giữa truyện (thắng mãi → độc giả hết lo)
- Nhân vật chính có GIỚI HẠN/điểm mù rõ + nguồn gốc năng lực (không "thánh")
- Phản diện có TÊN, có mưu riêng, đối đầu lặp lại; gài ≥1 tuyến phản bội có trả
- Phục vụ THỂ LOẠI CHÍNH bằng nhịp riêng (vd romance cần beat tình cảm/longing/"sủng" riêng, không để nhánh khác lấn)
- Cơ chế lõi có LUẬT + giá + giới hạn; tiên tri/mồi dài hơi gài sớm phải có chỗ trả
- Mỗi nhân vật chính một GIỌNG riêng (nguyên tắc, không câu mẫu)
- Ensemble: đặt tên + 1 dòng arc cho ≥3 nhân vật phụ
- Mid-pivot: một chương "không thể quay đầu" CỤ THỂ (~40–60%)
- Kết có CÁI GIÁ không đảo ngược (không utopia) + khớp tên truyện
- Đa dạng nhịp: đừng khóa mọi chương vào 1 khuôn / 1 kiểu nội tâm

## Chống AI-tell
- No purple prose, no "không phải X mà là Y" (kể cả trong worldbuilding), nội tâm lặp, nhịp câu đều đều
- Chỉ xuất Markdown của profile, không giải thích thêm\n\n`;

  if (brief) {
    payload += `--- BRIEF TEMPLATE ---\n${brief}\n`;
  } else {
    payload += `--- DEFAULT TEMPLATE ---\n${PROFILE_TEMPLATE}\n`;
  }

  try {
    await navigator.clipboard.writeText(payload);
    hint.textContent = '\ud83d\udccb \u0110\u00e3 copy prompt! D\u00e1n v\u00e0o LLM ngo\u00e0i, \u0111i\u1ec1n \u00fd t\u01b0\u1edfng, r\u1ed3i d\u00e1n k\u1ebft qu\u1ea3 v\u00e0o \u00f4 Step 4.';
    toast('\u0110\u00e3 copy prompt cho LLM ngo\u00e0i', 'ok');
  } catch (e) {
    hint.textContent = '\u274c L\u1ed7i clipboard';
  }
}

// buildProfileReviewPrompt: prompt SOÁT LỖI profile đã sinh, dán kèm chính
// profile cho LLM ngoài (GPT/Claude). Bám sát các lỗi LLM hay mắc ở foundation
// truyện dài: thánh-hoá nhân vật, thiếu cái giá, phản diện vô danh, lệch
// mood/thể loại, cơ chế lõi mơ hồ, đơn điệu, kết telegraph. Prompt càng tốt →
// review càng bắt đúng chỗ chết người của cả trăm chương phía sau.
// Sync note: keep these review axes aligned with profile_studio.go
// (profileStudioSystemPrompt) and profileLibCopyForLLM.
function buildProfileReviewPrompt(profile, opts) {
  opts = opts || {};
  const chapters = opts.chapters || 0;
  const year = opts.year || new Date().getFullYear();
  const n = chapters && chapters > 0 ? `~${chapters}` : 'dài kỳ (vài trăm)';
  const marketParts = [];
  if (opts.platform) marketParts.push(`nền tảng mục tiêu: ${opts.platform}`);
  if (opts.language) marketParts.push(`ngôn ngữ / thị trường: ${opts.language}`);
  const marketLine = marketParts.length
    ? marketParts.join('; ')
    : '(chưa khai báo — hãy suy ra từ nền tảng/độc giả nêu trong profile; nếu chưa rõ quốc gia, nêu giả định của bạn)';
  return `Bạn là biên tập viên kỳ cựu chuyên tiểu thuyết mạng dài kỳ (serialized web fiction), rành cả craft lẫn THỊ TRƯỜNG từng nước. Nhiệm vụ: SOÁT LỖI một "profile" (bản brief nền móng) mà một cuốn ${n} chương sẽ dựa vào để triển khai. Profile sai ở đây sẽ nhân lên thành hàng trăm chương sai — hãy nghiêm khắc.

Bối cảnh thị trường mục tiêu: ${marketLine}. Thời điểm đánh giá: ${year}.

Yêu cầu: phản biện thẳng, KHÔNG khen xã giao, KHÔNG tóm tắt lại. Chỉ nêu điểm yếu + cách sửa cụ thể, trích dẫn nguyên văn phần có vấn đề. Nếu một trục ổn thì ghi "Đạt" ngắn gọn rồi qua trục khác.

TRƯỚC KHI SOÁT, nêu ngắn (3–5 dòng) khung tham chiếu để việc soát bám đúng thể loại — rồi mới soát 13 trục DỰA TRÊN khung này:
- Thể loại & sub-genre của profile này là gì?
- Thể loại/sub-genre này đã ĐẠI TRÀ chưa? Nếu rồi: cliché nào phải tránh; nếu niche: phải làm tốt điều gì cho fan cứng?
- Đặc trưng / quy ước độc giả thể loại này KỲ VỌNG (payoff bắt buộc, thiếu là hụt)?
- Thị trường mục tiêu & gu hiện tại?

Soát theo 13 trục (đánh dấu Đạt / Cần sửa cho từng trục):
1. Nhất quán nhân vật — mỗi nhân vật chính có want + wound + mâu thuẫn nội tâm rõ và không tự mâu thuẫn? Tên cố định? Tính cách/giọng phân biệt được giữa các nhân vật?
2. Nguồn & giới hạn năng lực chính — tài năng cốt lõi (vd trí tuệ) có được neo nguồn gốc và có GIỚI HẠN rõ? (năng lực vô hạn → nhân vật hoá "thánh", mất căng thẳng)
3. Cái giá & thất bại thật — phân biệt TWIST (đẩy plot) với CÁI GIÁ (đánh vào nhân vật và để lại dấu vết dài: mất đồng minh, mất niềm tin, hy sinh không lấy lại được). Nhân vật chính có nhiều cái giá thật rải DỌC truyện không, hay chỉ thắng liên tục với đúng một cú vấp giữa truyện? Truyện ${n} chương chỉ thua một lần → độc giả hết lo từ khoảng chương 100.
4. Phản diện xứng tầm — có (các) đối thủ CỤ THỂ, lặp lại, mưu đồ và trí tuệ ngang cơ không, hay chỉ là "bộ máy/thế lực vô danh"? Mối đe doạ có leo thang? Có mồi dài hơi/tuyến phản bội ban đầu nào bị bỏ rơi?
5. Thể loại chính vs cấu trúc — thể loại được hứa (vd Romance) có được cấu trúc truyện phục vụ tương xứng, hay bị nhánh khác (chính trị/hành động) lấn át → lệch kỳ vọng độc giả & lệch mood?
6. Cơ chế lõi có RÀNG BUỘC — cơ chế siêu nhiên/đặc thù trung tâm (vd mate bond) có luật rõ, có giá, có giới hạn? Hay mơ hồ → dễ thành deus ex machina giải quyết mọi thứ?
7. Xung đột lõi & mid-pivot — xung đột trung tâm đủ sức kéo CẢ truyện? Bước ngoặt giữa có chốt được MỘT chương "không thể quay đầu" cụ thể (~40–60%) và thực sự buộc đổi chiến lược, hay chỉ là một dải chương mơ hồ / leo thang tuyến tính?
8. Bền cho truyện dài — có tuyến nhân vật phụ / ensemble đủ nuôi ${n} chương? Có tuyến dài hơi để tránh cạn ý giữa chừng?
9. Chống đại trà — reader promise cụ thể (không sáo)? Có ≥3 điểm khác biệt THẬT mà truyện cùng thể loại không có sẵn? Có trope mòn nào lọt vào?
10. Nhịp & lặp — công thức chương có biến thể không, hay lặp một mô-típ suốt ${n} chương → đơn điệu, mỏi?
11. Kết & lời hứa — định hướng kết là CÂU HỎI CHỦ ĐỀ (không phải plot beat/tên chương)? Kết có cái giá hay quá dễ/utopia? Tên truyện có khớp tông và kỳ vọng của kết không?
12. AI-tell — có purple prose, cấu trúc "không phải X mà là Y", nội tâm lặp, câu văn mẫu, nhịp câu đều đều không?
13. Phù hợp văn hoá đọc & thị hiếu thị trường mục tiêu tại ${year}+ — thể loại/trope/độ "nóng"/bạo lực/nhịp/độ dài/nội dung có khớp gu độc giả thị trường HIỆN TẠI không? Chú ý cụ thể: (i) MÃ TROPE bản địa vs ngoại nhập (vd werewolf/Alpha/pack là mã Tây, không phải mã ngôn tình Hoa quen thuộc với độc giả Việt); (ii) KỲ VỌNG CẢM XÚC đặc trưng của thị trường (vd yếu tố "sủng"/chiều chuộng gần như bắt buộc ở ngôn tình VN; "longing"/fated-mate ở werewolf romance quốc tế) — thiếu là rủi ro rớt độc giả; (iii) ngưỡng 18+/bạo lực và kiểm duyệt nền tảng. Nêu: (a) điểm khớp gu, (b) điểm lệch gu / rủi ro văn hoá–pháp lý / rủi ro thương mại, (c) đề xuất điều chỉnh để bán được. Nếu một lựa chọn đi NGƯỢC xu hướng đang thắng, coi đó là risk có chủ đích cần bù đắp, không phải "điểm mới lạ an toàn". Nếu KHÔNG chắc xu hướng ${year}+, nói rõ và khuyên kiểm chứng bằng bảng bestseller/đề xuất đang chạy của chính nền tảng mục tiêu.

Ngoài 13 trục: chỉ ra nếu profile TỰ VI PHẠM mục "Điều cần tránh" của chính nó (kể cả cấu trúc "không phải X mà là Y" lọt vào worldbuilding).

Định dạng trả lời:
- Mỗi trục: [Đạt/Cần sửa] + 1–2 câu vấn đề (trích nguyên văn) + đề xuất sửa cụ thể.
- Kết: "3 việc PHẢI sửa trước tiên", xếp theo mức sát thương với truyện dài.

--- PROFILE CẦN SOÁT ---
${profile}`;
}

// Copy chính profile đã sinh + prompt review cho LLM ngoài (GPT/Claude...).
// Khác studioCopyForLLM (copy Ý TƯỞNG để LLM ngoài SINH profile).
async function profileLibCopyForReview() {
  const hint = $('#studioReviewHint');
  const profile = $('#studioOutput').value.trim();
  if (!profile) {
    if (hint) hint.textContent = '⚠ Chưa có profile ở Step 4 để review. Sinh hoặc dán profile trước.';
    toast('Chưa có profile để review', 'error');
    return;
  }
  const payload = buildProfileReviewPrompt(profile, {
    chapters: parseInt($('#studioChapters').value, 10) || 0,
    platform: $('#studioPlatform').value.trim(),
    language: $('#studioLang').value.trim(),
    year: new Date().getFullYear(),
  });
  try {
    await navigator.clipboard.writeText(payload);
    if (hint) hint.textContent = '📋 Đã copy profile + prompt review! Dán vào GPT/Claude để soát, rồi sửa lại ở Step 4 trước khi Lưu.';
    toast('Đã copy prompt review cho LLM ngoài', 'ok');
  } catch (e) {
    if (hint) hint.textContent = '❌ Lỗi clipboard';
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
    detail.innerHTML = '<div class="placeholder"><p>Ch\u1ecdn m\u1ed9t job b\u00ean tr\u00e1i \u0111\u1ec3 xem chi ti\u1ebft.</p><p class="muted" style="margin-top: var(--space-3); font-size: 0.9em;"><strong>Ch\u1ebf \u0111\u1ed9 S\u1ea3n xu\u1ea5t = headless:</strong> job ch\u1ea1y tr\u00ean server, kh\u00f4ng m\u1edf TUI nh\u01b0 ch\u1ebf \u0111\u1ed9 th\u01b0\u1eddng. Cook li\u00ean t\u1ee5c \u0111\u1ebfn khi \u0111\u1ea1t m\u1ee5c ti\u00eau ho\u1eb7c h\u1ebft budget. D\u00f9ng \u201c\u0110\u1ed3ng b\u1ed9\u201d \u0111\u1ec3 \u0111\u1ed5 k\u1ebft qu\u1ea3 v\u1ec1 workspace ch\u00ednh, ho\u1eb7c \u201cD\u1eebng\u201d \u0111\u1ec3 xem t\u1eebng ch\u01b0\u01a1ng \u0111\u00e3 vi\u1ebft.</p></div>';
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

  const healthHtml = renderHealthStrip(run.health);

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
      ${healthHtml}
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

// Health strip: at-a-glance "đúng nhịp / nên xem / cần chú ý" cho một run.
// Dữ liệu (level + value) tính ở backend (prodrun_health.go, có test); ở đây
// chỉ map key -> nhãn tiếng Việt và tô màu. Ẩn khi chưa đủ dữ liệu (queued).
const HEALTH_METRIC_LABELS = {
  progress: 'Ti\u1ebfn \u0111\u1ed9',
  rewrite_rate: 'Vi\u1ebft l\u1ea1i',
  cost_pace: 'Chi ph\u00ed/ch\u01b0\u01a1ng',
  budget: 'Ng\u00e2n s\u00e1ch',
};

const HEALTH_OVERALL = {
  good: { icon: '\ud83d\udfe2', text: '\u0110ang \u0111\u00fang nh\u1ecbp' },
  warn: { icon: '\ud83d\udfe1', text: 'N\u00ean xem l\u1ea1i' },
  bad: { icon: '\ud83d\udd34', text: 'C\u1ea7n ch\u00fa \u00fd' },
  idle: { icon: '\u26aa', text: 'Ch\u01b0a \u0111\u1ee7 d\u1eef li\u1ec7u' },
};

// Backend health enum; allowlist before interpolating into a class attribute so
// a malformed/future field can't break out of the attribute context.
const HEALTH_LEVELS = ['good', 'warn', 'bad', 'idle'];

function renderHealthStrip(health) {
  if (!health || !Array.isArray(health.metrics)) return '';
  // Chưa có dữ liệu thực (mọi metric idle) → không hiện, tránh nhiễu cho job Chờ.
  // Allowlist level (backend enum) + bỏ entry malform để payload hỏng không crash detail panel.
  const metrics = health.metrics.filter((m) => m && HEALTH_LEVELS.includes(m.level));
  if (!metrics.some((m) => m.level !== 'idle')) return '';
  const overallLevel = HEALTH_LEVELS.includes(health.overall) ? health.overall : 'idle';
  const overall = HEALTH_OVERALL[overallLevel];
  const chips = metrics.map((m) => {
    const label = HEALTH_METRIC_LABELS[m.key] || m.key;
    return `<span class="health-chip health-${m.level}"><span class="health-dot"></span>${escapeHtml(label)}: <strong>${escapeHtml(m.value || '\u2014')}</strong></span>`;
  }).join('');
  return `<div class="health-strip health-overall-${overallLevel}">
    <span class="health-summary">${overall.icon} ${escapeHtml(overall.text)}</span>
    <div class="health-chips">${chips}</div>
  </div>`;
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
