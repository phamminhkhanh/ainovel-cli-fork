// ainovel web — Market Radar (CN lead signals -> target-market opportunities).
// Manual scan only. Backend owns fetch/analyze/persistence; UI only renders.
'use strict';

let radarLoaded = false;
let radarScanning = false;
let radarReportCache = null;

async function loadRadarTab() {
  if (radarLoaded) return;
  const root = $('#radarRoot');
  if (!root) return;
  root.innerHTML = '<div class="placeholder">Đang tải báo cáo Radar gần nhất…</div>';
  try {
    const res = await fetch('/api/radar/latest');
    if (res.status === 404) {
      renderRadarEmpty();
      radarLoaded = true;
      return;
    }
    if (!res.ok) throw new Error('HTTP ' + res.status);
    renderRadarReport(await res.json());
    radarLoaded = true;
  } catch (e) {
    renderRadarLoadError(e);
    toast('Lỗi tải Radar: ' + e, 'error');
    radarLoaded = false;
  }
}

function renderRadarLoadError(error) {
  const root = $('#radarRoot');
  if (!root) return;
  root.innerHTML = `
    <div class="radar-hero">
      <div><h2>Radar thị trường</h2><p>Không đọc được báo cáo gần nhất. Chưa thực hiện scan mới hoặc gọi model.</p></div>
      <button class="btn" id="radarRetryLatest">Thử tải lại</button>
    </div>
    <div class="radar-empty" role="alert">${escapeHtml(String(error || 'Lỗi không xác định'))}</div>`;
  const retry = $('#radarRetryLatest');
  if (retry) retry.addEventListener('click', () => loadRadarTab());
}

function renderRadarEmpty() {
  const root = $('#radarRoot');
  if (!root) return;
  root.innerHTML = `
    <div class="radar-hero">
      <div><h2>Radar thị trường</h2><p>Tín hiệu bảng xếp hạng Trung Quốc, phân tích khả năng lan sang VN/ES trong 6–12 tháng.</p></div>
      ${radarControls()}
    </div>
    <div class="radar-empty">Chưa có báo cáo. Chọn thị trường đích và quét thủ công.</div>`;
  bindRadarControls();
}

function radarControls(target) {
  const value = target || 'vi';
  return `<div class="radar-controls">
    <label>Thị trường đích
      <select id="radarTarget">
        <option value="vi" ${value === 'vi' ? 'selected' : ''}>Việt Nam</option>
        <option value="es" ${value === 'es' ? 'selected' : ''}>Tây Ban Nha / LATAM</option>
        <option value="en" ${value === 'en' ? 'selected' : ''}>English</option>
      </select>
    </label>
    <button class="btn primary" id="radarScan" ${radarScanning ? 'disabled' : ''}>${radarScanning ? 'Đang quét…' : '📡 Quét Radar'}</button>
  </div>`;
}

function bindRadarControls() {
  const button = $('#radarScan');
  if (button) button.addEventListener('click', scanRadar);
}

async function scanRadar() {
  if (radarScanning) return;
  radarScanning = true;
  const target = ($('#radarTarget') && $('#radarTarget').value) || 'vi';
  renderRadarBusy(target);
  try {
    const res = await fetch('/api/radar/scan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ targetMarket: target }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
    renderRadarReport(data);
    toast('Radar đã lưu snapshot + report', 'success');
  } catch (e) {
    if (radarReportCache) renderRadarReport(radarReportCache);
    else renderRadarEmpty();
    toast('Quét Radar lỗi: ' + e, 'error');
  } finally {
    radarScanning = false;
    const button = $('#radarScan');
    if (button) { button.disabled = false; button.textContent = '📡 Quét Radar'; }
  }
}

function renderRadarBusy(target) {
  const root = $('#radarRoot');
  root.innerHTML = `<div class="radar-hero"><div><h2>Radar thị trường</h2><p>Đang lấy Fanqie + Qidian rồi phân tích CN → ${escapeHtml(radarMarketLabel(target))}…</p></div>${radarControls(target)}</div>
    <div class="radar-scanning" role="status" aria-live="polite"><span class="spinner"></span> Quét nguồn và gọi model analyst. Có thể mất 1–3 phút.</div>`;
}

function renderRadarReport(report) {
  const root = $('#radarRoot');
  if (!root) return;
  radarReportCache = report;
  const mode = report.analysisMode || 'unknown';
  const recommendations = Array.isArray(report.recommendations) ? report.recommendations : [];
  const failed = Array.isArray(report.failedSources) ? report.failedSources : [];
  const snapshots = Array.isArray(report.snapshots) ? report.snapshots : [];
  root.innerHTML = `
    <div class="radar-hero">
      <div><h2>Radar thị trường</h2><p>CN → ${escapeHtml(radarMarketLabel(report.targetMarket))} · lead time 6–12 tháng</p></div>
      ${radarControls(report.targetMarket)}
    </div>
    <div class="radar-status radar-mode-${escapeHtml(mode)}">
      <strong>${escapeHtml(radarModeLabel(mode))}</strong>
      <span>Nguồn live: ${Number(report.liveSources || 0)}/${Number(report.totalSources || 0)}</span>
      <span>Tuổi snapshot: ${escapeHtml(report.snapshotAge || '0s')}</span>
      ${failed.length ? `<span>Nguồn lỗi: ${failed.map(escapeHtml).join(', ')}</span>` : ''}
    </div>
    <section class="radar-sources"><h3>Nguồn Trung Quốc</h3><div class="radar-source-grid">${snapshots.map(renderRadarSource).join('') || '<div class="placeholder">Không có snapshot nguồn.</div>'}</div></section>
    <section class="radar-summary"><h3>Tổng quan</h3><p>${escapeHtml(report.marketSummary || '—')}</p></section>
    <section><h3>Cơ hội đề xuất</h3><div class="radar-grid">${recommendations.map(renderRadarRecommendation).join('') || '<div class="placeholder">Không có đề xuất.</div>'}</div></section>`;
  bindRadarControls();
}

function renderRadarSource(snapshot) {
  const entries = Array.isArray(snapshot.entries) ? snapshot.entries : [];
  const degraded = Boolean(snapshot.error) && entries.length > 0;
  const state = entries.length ? (degraded ? 'degraded' : 'live') : 'failed';
  const stateLabel = state === 'live' ? 'Live' : state === 'degraded' ? 'Live một phần' : 'Lỗi';
  return `<article class="radar-source radar-source-${state}">
    <div class="radar-source-head">
      <div><strong>${escapeHtml(radarSourceLabel(snapshot.source))}</strong><small>${escapeHtml(snapshot.market || 'cn').toUpperCase()}</small></div>
      <span>${stateLabel} · ${entries.length} mục</span>
    </div>
    ${snapshot.error ? `<p class="radar-source-error">${escapeHtml(snapshot.error)}</p>` : ''}
    ${entries.length ? `<details><summary>Xem bảng xếp hạng đã lấy</summary><ol>${entries.map((entry) => `<li><b>#${Number(entry.rank || 0)}</b> ${escapeHtml(entry.title || '')}${entry.category ? ` <small>${escapeHtml(entry.category)}</small>` : ''}${entry.list ? ` <em>${escapeHtml(radarListLabel(entry.list))}</em>` : ''}</li>`).join('')}</ol></details>` : '<p class="muted">Nguồn không trả dữ liệu ranking.</p>'}
  </article>`;
}

function renderRadarRecommendation(rec, index) {
  const benchmarks = Array.isArray(rec.benchmarkTitles) ? rec.benchmarkTitles : [];
  const risks = Array.isArray(rec.localizationRisks) ? rec.localizationRisks : [];
  const evidence = Array.isArray(rec.evidence) ? rec.evidence : [];
  const lag = Array.isArray(rec.estimatedLagMonths) ? rec.estimatedLagMonths.join('–') : '6–12';
  return `<article class="radar-card">
    <div class="radar-card-head"><span class="radar-rank">#${index + 1}</span><h4>${escapeHtml(rec.genre || 'Chưa phân loại')}</h4></div>
    <p class="radar-concept">${escapeHtml(rec.concept || '')}</p>
    <div class="radar-scores">
      ${radarScore('Đà tăng', rec.momentum)}
      ${radarScore('Bão hòa', rec.saturation)}
      ${radarScore('Khả năng lan', rec.transferability)}
    </div>
    <p><strong>Độ trễ:</strong> ${escapeHtml(lag)} tháng</p>
    <p>${escapeHtml(rec.reasoning || '')}</p>
    ${benchmarks.length ? `<p><strong>Đối chiếu:</strong> ${benchmarks.map(escapeHtml).join(' · ')}</p>` : ''}
    ${risks.length ? `<details><summary>Rủi ro bản địa hóa</summary><ul>${risks.map((x) => `<li>${escapeHtml(x)}</li>`).join('')}</ul></details>` : ''}
    ${evidence.length ? `<details><summary>Bằng chứng dữ liệu</summary><ul>${evidence.map((x) => `<li>${escapeHtml(x)}</li>`).join('')}</ul></details>` : ''}
    <button class="btn radar-copy" data-radar-index="${index}">📋 Copy concept</button>
  </article>`;
}

function radarScore(label, value) {
  const pct = Math.max(0, Math.min(100, Math.round(Number(value || 0) * 100)));
  return `<div><span>${escapeHtml(label)}</span><strong>${pct}%</strong><div class="radar-meter"><i style="width:${pct}%"></i></div></div>`;
}

function radarModeLabel(mode) {
  return ({ live: 'Dữ liệu live', partial_live: 'Live một phần', model_knowledge_fallback: 'Fallback kiến thức model' })[mode] || mode;
}

function radarMarketLabel(market) {
  return ({ vi: 'Việt Nam', es: 'Tây Ban Nha / LATAM', en: 'English' })[market] || market || 'Việt Nam';
}

function radarSourceLabel(source) {
  return ({ fanqie: '番茄小说 · Fanqie', qidian: '起点中文网 · Qidian' })[source] || source || 'Nguồn không rõ';
}

function radarListLabel(list) {
  return ({ hot: '热门榜 · Hot', dark_horse: '黑马榜 · Dark Horse', rank: '热榜 · Ranking' })[list] || list;
}

document.addEventListener('click', async (e) => {
  const button = e.target.closest('.radar-copy');
  if (!button) return;
  try {
    const index = Number(button.dataset.radarIndex);
    const recommendations = radarReportCache && Array.isArray(radarReportCache.recommendations)
      ? radarReportCache.recommendations : [];
    await navigator.clipboard.writeText((recommendations[index] && recommendations[index].concept) || '');
    toast('Đã copy concept', 'success');
  } catch (err) { toast('Không copy được: ' + err, 'error'); }
});
