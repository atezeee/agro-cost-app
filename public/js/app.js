const $ = (id) => document.getElementById(id);
const state = {
  step: 1,
  rows: [],
  currentUser: null,
  authMode: 'login',
  settings: {},
  federalDistricts: [], crops: [], operations: [], operationRules: [], norms: [], conditionCoefficients: [], techMapTemplates: [], materials: [], machines: [], operationMachines: [], costItems: [], prices: [], history: [], lastResult: null, autoSaving: false, selectedDetail: null
};

const titles = {
  calculations: ['Расчёт технологических карт', 'Планирование затрат по культуре, технике, нормам и условиям работы'],
  history: ['Журнал расчётов', 'Сохранённые ведомости с детализацией, источниками цен и экспортом'],
  settings: ['Рабочие настройки', 'Параметры организации, автора отчёта и значения по умолчанию'],
  help: ['Помощь', 'Короткая карта логики расчёта и источников данных']
};

const defaultSettings = {
  organization: '',
  reportAuthor: '',
  defaultDistrictId: '',
  defaultMachineUsage: 'own',
  autosaveHistory: true,
  showHints: true,
  defaultCropId: ''
};

async function api(url, options = {}) {
  const res = await fetch(url, { credentials: 'include', headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }, ...options });
  if (!res.ok) {
    let msg = `Ошибка ${res.status}`;
    try { const data = await res.json(); msg = data.error || msg; } catch (_) {}
    throw new Error(msg);
  }
  return res.json();
}

function num(v, d = 0) { return Number(v || 0).toLocaleString('ru-RU', { maximumFractionDigits: d, minimumFractionDigits: d }); }
function rub(v) { return `${num(v, 0)} руб.`; }
function optionHtml(items, selected) { return (items || []).map(x => `<option value="${x.id}" ${selected && selected == x.id ? 'selected' : ''}>${x.name}</option>`).join(''); }
function machineOptionHtml(items, selected) { return (items || []).map(x => `<option value="${x.id}" ${selected && selected == x.id ? 'selected' : ''}>${x.name}${x.role ? ` — ${x.role}` : ''}</option>`).join(''); }
function nameById(items, id) { return (items || []).find(x => Number(x.id) === Number(id))?.name || ''; }
function itemById(items, id) { return (items || []).find(x => Number(x.id) === Number(id)); }
function sourceLabel(name) { return name && name.toLowerCase().includes('демонстрац') ? 'Резервная база цен' : (name || 'Ценовая база'); }
function toast(msg, error = false) { const t = $('toast'); t.textContent = msg; t.className = `toast show ${error ? 'error' : ''}`; clearTimeout(t._timer); t._timer = setTimeout(() => t.className = 'toast', 3200); }

function loadSettingsState() {
  try {
    state.settings = { ...defaultSettings, ...(JSON.parse(localStorage.getItem('agrocalc.settings') || '{}') || {}) };
  } catch (_) {
    state.settings = { ...defaultSettings };
  }
}

function saveSettingsState() {
  localStorage.setItem('agrocalc.settings', JSON.stringify(state.settings));
}

function selectedOperationName() { return nameById(state.operations, $('operation')?.value); }
function getOperationRule() {
  const opID = Number($('operation')?.value || 0);
  const rule = state.operationRules.find(x => Number(x.operation_id) === opID);
  if (!rule) return { resource: null, title: '', rate: 0, showCrop: true, description: 'Заполните необходимые данные по выбранной работе.' };
  const norm = normFor(opID, rule.resource_material_type);
  return { resource: rule.resource_material_type || null, title: rule.resource_title || '', rate: norm?.rate || 0, showCrop: rule.requires_crop, description: rule.description || 'Заполните необходимые данные по выбранной работе.' };
}

function normFor(operationID, materialType) {
  if (!materialType) return null;
  const cropID = Number($('crop')?.value || 0);
  return state.norms.find(x => Number(x.crop_id) === cropID && Number(x.operation_id) === Number(operationID) && x.material_type === materialType)
    || state.norms.find(x => Number(x.operation_id) === Number(operationID) && x.material_type === materialType)
    || null;
}

function selectedConditionCoefficientIds() {
  return Array.from(document.querySelectorAll('[data-condition-coefficient]')).map(x => Number(x.value || 0)).filter(Boolean);
}

function selectedConditionCoefficientValue() {
  return selectedConditionCoefficientIds().reduce((acc, id) => acc * Number(state.conditionCoefficients.find(x => Number(x.id) === id)?.value || 1), 1);
}

function setView(view) {
  if (!titles[view]) view = 'calculations';
  document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
  $(`view-${view}`)?.classList.add('active');
  document.querySelectorAll('[data-view]').forEach(a => a.classList.toggle('active', a.dataset.view === view));
  $('pageTitle').textContent = titles[view][0];
  $('pageSubtitle').textContent = titles[view][1];
  if (view === 'history') loadHistory();
  if (view === 'settings') renderSettings();
  if (view === 'help') renderHelp();
}

function setStep(step) {
  state.step = step;
  document.querySelectorAll('.wizard-screen').forEach(x => x.classList.remove('active'));
  $(`step${step}`).classList.add('active');
  document.querySelectorAll('[data-step-indicator]').forEach(x => x.classList.toggle('active', Number(x.dataset.stepIndicator) === step));
}

function openModal(id) { $(id).classList.add('active'); $(id).setAttribute('aria-hidden', 'false'); }
function closeModal(id) { $(id).classList.remove('active'); $(id).setAttribute('aria-hidden', 'true'); }

async function refreshAuth() {
  try {
    const data = await api('/api/auth/me');
    state.currentUser = data.authenticated ? data.user : null;
  } catch (_) { state.currentUser = null; }
  renderAuthActions();
}

function renderAuthActions() {
  if (state.currentUser) {
    $('authActions').innerHTML = `<div class="auth-user"><div><strong>${state.currentUser.full_name || 'Пользователь'}</strong><span>Личный кабинет</span></div><button class="outline compact" id="logoutTop">Выход</button></div>`;
    $('logoutTop').addEventListener('click', logout);
  } else {
    $('authActions').innerHTML = `<button class="outline compact" id="openLogin">Вход</button><button class="compact" id="openRegister">Регистрация</button>`;
    $('openLogin').addEventListener('click', () => openAuth('login'));
    $('openRegister').addEventListener('click', () => openAuth('register'));
  }
}

function openAuth(mode) {
  state.authMode = mode;
  $('authTitle').textContent = mode === 'register' ? 'Регистрация' : 'Вход';
  $('authName').classList.toggle('hide', mode !== 'register');
  $('authStatus').textContent = mode === 'register' ? 'Создайте учётную запись для синхронизации истории расчётов' : 'Войдите в учётную запись';
  $('loginBtn').textContent = mode === 'register' ? 'У меня есть аккаунт' : 'Войти';
  $('registerBtn').textContent = mode === 'register' ? 'Зарегистрироваться' : 'Регистрация';
  openModal('authModal');
}

async function login() {
  try {
    const user = await api('/api/auth/login', { method: 'POST', body: JSON.stringify({ email: $('authEmail').value, password: $('authPassword').value }) });
    state.currentUser = user; closeModal('authModal'); renderAuthActions(); loadHistory(); toast('Вход выполнен');
  } catch (e) { toast(e.message, true); }
}
async function register() {
  try {
    const user = await api('/api/auth/register', { method: 'POST', body: JSON.stringify({ fullName: $('authName').value, email: $('authEmail').value, password: $('authPassword').value }) });
    state.currentUser = user; closeModal('authModal'); renderAuthActions(); loadHistory(); toast('Регистрация выполнена');
  } catch (e) { toast(e.message, true); }
}
async function logout() { try { await api('/api/auth/logout', { method: 'POST', body: '{}' }); } catch (_) {} state.currentUser = null; renderAuthActions(); await loadHistory(); toast('Вы вышли'); }

function activeFDName() { return nameById(state.federalDistricts, Number($('federalDistrict')?.value)); }

async function loadPrices() {
  const fdID = Number($('federalDistrict').value || 0);
  state.prices = await api(`/api/prices?federal_district_id=${fdID}`);
  renderPriceInfo();
}

function machineRolePriority(role) {
  const value = String(role || '').toLowerCase();
  if (value.includes('энергет')) return 0;
  if (value.includes('убороч')) return 1;
  if (value.includes('транспорт')) return 2;
  if (value.includes('посев')) return 3;
  return 4;
}

function machineUsageLabel(val) { return val === 'rent' ? 'Аренда' : 'Своя техника'; }

async function loadOperationMachines() {
  const opID = Number($('operation').value || 0);
  state.operationMachines = await api(`/api/operation-machines?operation_id=${opID}`);
  state.operationMachines.sort((a, b) => machineRolePriority(a.role) - machineRolePriority(b.role) || String(a.name).localeCompare(String(b.name), 'ru'));
  const list = state.operationMachines.length ? state.operationMachines : state.machines;
  $('machine').innerHTML = machineOptionHtml(list);
  if (list.length) $('machine').value = list[0].id;
  updateMachineFields();
}

async function loadTechMapTemplates() {
  const cropID = Number($('crop')?.value || 0);
  if (!cropID) return;
  state.techMapTemplates = await api(`/api/tech-map-templates?crop_id=${cropID}`);
  renderTechMapTemplates();
}

function renderTechMapTemplates() {
  const box = $('techMapList');
  if (!box) return;
  if ($('calcMode')?.value !== 'tech_map') {
    box.innerHTML = '';
    return;
  }
  const rows = (state.techMapTemplates || []).map(t => `<label class="tech-map-item"><input type="checkbox" data-tech-template value="${t.id}" ${t.is_required ? 'checked disabled' : 'checked'}><span><strong>${t.operation_name}</strong><small>${t.phase || 'Этап работ'}${t.is_required ? ' · обязательно' : ' · опционально'}</small></span><small>${num(t.area_factor || 1, 2)}x</small></label>`).join('');
  box.innerHTML = rows || '<div class="empty-state">Для выбранной культуры пока нет шаблона технологической карты.</div>';
}

function renderConditionCoefficientFields() {
  const box = $('conditionCoefficientFields');
  if (!box) return;
  const groups = {};
  (state.conditionCoefficients || []).forEach(item => { (groups[item.group_code] ||= { name: item.group_name, items: [] }).items.push(item); });
  box.innerHTML = Object.entries(groups).map(([code, group]) => `<label class="field-card"><span>${group.name}</span><select data-condition-coefficient="${code}">${group.items.map(x => `<option value="${x.id}">${x.name} (${num(x.value, 2)})</option>`).join('')}</select></label>`).join('');
  box.querySelectorAll('select').forEach(x => x.addEventListener('change', updateConditionCoefficientTotal));
  updateConditionCoefficientTotal();
}

function updateConditionCoefficientTotal() {
  const el = $('conditionCoefficientTotal');
  if (el) el.textContent = num(selectedConditionCoefficientValue(), 2);
}

function materialByType(type, fallback) {
  const items = state.materials || [];
  const cropName = nameById(state.crops, $('crop').value).toLowerCase();
  if (type === 'seed') {
    if (cropName.includes('ячмен')) return items.find(x => x.name === 'Семена ячменя') || items.find(x => x.name.includes('Семена'));
    if (cropName.includes('подсол')) return items.find(x => x.name === 'Семена подсолнечника') || items.find(x => x.name.includes('Семена'));
    if (cropName.includes('кукуруз')) return items.find(x => x.name === 'Семена кукурузы') || items.find(x => x.name.includes('Семена'));
    return items.find(x => x.name === 'Семена пшеницы') || items.find(x => x.name.includes('Семена'));
  }
  if (type === 'fertilizer') return items.find(x => ['Аммиачная селитра', 'Карбамид', 'Азофоска NPK'].includes(x.name)) || items.find(x => x.description === 'fertilizer');
  if (type === 'pesticide') return items.find(x => ['Гербицид', 'Фунгицид', 'Инсектицид'].includes(x.name)) || items.find(x => x.description === 'pesticide');
  return items.find(x => x.name === fallback);
}
function costItemByName(part) { return state.costItems.find(x => x.name.toLowerCase().includes(part.toLowerCase())); }
function materialNamed(name) { return state.materials.find(x => x.name === name); }
function priceForMaterial(materialID) {
  const fdID = Number($('federalDistrict').value || 0);
  const exact = state.prices.find(p => Number(p.material_id) === Number(materialID) && Number(p.federal_district_id) === fdID);
  return exact || state.prices.find(p => Number(p.material_id) === Number(materialID));
}

function applyCropVisibility() {
  if ($('calcMode')?.value === 'tech_map') {
    $('cropFieldWrap').classList.remove('hide');
    updateSelectionSummary();
    return;
  }
  const rule = getOperationRule();
  const wrap = $('cropFieldWrap');
  wrap.classList.toggle('hide', !rule.showCrop);
  if (!rule.showCrop) {
    const fallbackCrop = state.settings.defaultCropId || state.crops.find(x => x.name === 'Кукуруза')?.id || state.crops[0]?.id;
    if (fallbackCrop) $('crop').value = fallbackCrop;
  }
  updateSelectionSummary();
}

function updateMachineFields() {
  const list = state.operationMachines.length ? state.operationMachines : state.machines;
  const m = itemById(list, $('machine').value) || itemById(state.machines, $('machine').value);
  $('productivity').value = m?.productivity || m?.productivity_ha_per_hour || '';
  $('fuelRate').value = m?.fuel_rate || m?.fuel_rate_l_per_ha || defaultFuelRate();
  $('machineMetaText').textContent = m ? `${m.name}${m.role ? ` — ${m.role}` : ''}. Производительность и норма топлива подставлены автоматически. Тип использования: ${machineUsageLabel($('machineUsage').value)}.` : 'Подберите технику для расчёта — значения нормы топлива и производительности появятся автоматически.';
}

function defaultFuelRate() {
  const name = selectedOperationName().toLowerCase();
  if (name.includes('вспаш')) return 25;
  if (name.includes('культивац')) return 12;
  if (name.includes('лущ')) return 10;
  if (name.includes('борон')) return 6;
  if (name.includes('посев')) return 8;
  if (name.includes('удобр')) return 7;
  if (name.includes('сзр')) return 4;
  if (name.includes('уборк')) return 18;
  if (name.includes('междуряд')) return 9;
  return 8;
}

function updateSelectionSummary() {
  const rule = getOperationRule();
  const isTechMap = $('calcMode')?.value === 'tech_map';
  $('summaryOperationName').textContent = isTechMap ? 'Технологическая карта культуры' : (selectedOperationName() || 'Выберите операцию');
  $('summaryOperationDescription').textContent = isTechMap ? 'Сервис сформирует цепочку операций по шаблону выбранной культуры.' : (rule.description || 'После выбора операции сервис покажет только релевантные параметры.');
  $('summaryFD').textContent = activeFDName() || '—';
  $('summaryArea').textContent = `${num($('area')?.value || 0, 2)} га`;
  $('summaryCrop').textContent = (isTechMap || rule.showCrop) ? (nameById(state.crops, $('crop')?.value) || '—') : 'Не требуется';
  $('workSummaryOp').textContent = isTechMap ? 'Технологическая карта' : (selectedOperationName() || '—');
  $('workSummaryFD').textContent = activeFDName() || '—';
  $('workSummaryArea').textContent = `${num($('area')?.value || 0, 2)} га`;
  $('workSummaryCrop').textContent = (isTechMap || rule.showCrop) ? (nameById(state.crops, $('crop')?.value) || '—') : 'Не требуется';
}

function updateSmartFields() {
  updateModeUI();
  $('workDataTitle').textContent = $('calcMode')?.value === 'tech_map' ? 'Исходные данные технологической карты' : (selectedOperationName() || 'Исходные данные по работе');
  const rule = getOperationRule();
  $('workDataSubtitle').textContent = rule.description || 'Поля подобраны исходя из выбранной агротехнической работы.';
  const resourceCard = $('resourceCard');
  if ($('calcMode')?.value === 'tech_map') {
    resourceCard.classList.add('hide');
    $('resourceRate').value = 0;
  } else if (rule.resource) {
    resourceCard.classList.remove('hide');
    $('resourceTitle').textContent = rule.title;
    const mat = materialByType(rule.resource);
    if (mat) $('material').value = mat.id;
    $('resourceRate').value = rule.rate;
  } else {
    resourceCard.classList.add('hide');
    $('resourceRate').value = 0;
  }
  applyCropVisibility();
  updateMachineFields();
  updateSelectionSummary();
}

function updateModeUI() {
  const isTechMap = $('calcMode')?.value === 'tech_map';
  document.querySelectorAll('.operation-mode-only').forEach(x => x.classList.toggle('hide', isTechMap));
  document.querySelector('.smart-layout')?.classList.toggle('tech-map-active', isTechMap);
  $('techMapCard')?.classList.toggle('hide', !isTechMap);
  renderTechMapTemplates();
}

function validateStep1() {
  if ($('calcMode')?.value !== 'tech_map' && !$('operation').value) return toast('Выберите агротехническую работу', true), false;
  if ($('calcMode')?.value === 'tech_map' && !$('crop').value) return toast('Выберите культуру для технологической карты', true), false;
  if (Number($('area').value || 0) <= 0) return toast('Площадь должна быть больше нуля', true), false;
  return true;
}

function row(operationID, material, costItem, rate, opts = {}) {
  if (!material || !costItem || Number(rate) <= 0) return null;
  const r = { operation_id: Number(operationID), material_id: Number(material.id), machine_id: Number($('machine').value || 0), machine_usage_type: $('machineUsage').value, cost_item_id: Number(costItem.id), rate: Number(rate), coefficient: Number($('coefficient').value || 1) };
  if (opts.manualPrice !== undefined && opts.manualPrice !== null && opts.manualPrice !== '') r.manual_price = Number(opts.manualPrice);
  return r;
}

async function buildSmartRows() {
  if (!validateStep1()) return;
  try {
    const payload = {
      crop_id: Number($('crop').value),
      federal_district_id: Number($('federalDistrict').value),
      area_ha: Number($('area').value),
      calculation_date: $('calcDate')?.value || '',
      calculation_mode: $('calcMode').value,
      operation_id: Number($('operation').value || 0),
      machine_id: Number($('machine').value || 0),
      machine_usage_type: $('machineUsage').value,
      productivity_ha_per_hour: Number($('productivity').value || 0),
      fuel_rate_l_per_ha: Number($('fuelRate').value || 0),
      resource_material_id: Number($('material').value || 0),
      resource_rate: Number($('resourceRate').value || 0),
      manual_price: $('manualPrice').value ? Number($('manualPrice').value) : undefined,
      condition_coefficient_ids: selectedConditionCoefficientIds(),
      selected_template_ids: Array.from(document.querySelectorAll('[data-tech-template]:checked')).map(x => Number(x.value)),
      include_comparison: $('includeComparison').checked
    };
    const draft = await api('/api/calculation-draft', { method: 'POST', body: JSON.stringify(payload) });
    state.rows = draft.rows || [];
    renderRows();
    await calculate(false, { silent: true });
    setStep(3);
    if (state.settings.autosaveHistory) await autoSaveCalculation();
  } catch (e) { toast(e.message, true); }
}

async function autoSaveCalculation() {
  if (!state.rows.length || state.autoSaving) return;
  state.autoSaving = true;
  $('metricStatus').textContent = 'Сохранение...';
  try {
    await calculate(true, { silent: true });
    $('metricStatus').textContent = 'Расчёт сохранён';
  } catch (e) {
    $('metricStatus').textContent = 'Ошибка сохранения';
    toast(e.message, true);
  } finally {
    state.autoSaving = false;
  }
}

function goHome() {
  state.rows = []; state.lastResult = null;
  renderRows(); updateMetrics(); setStep(1);
}

function rowEstimate(r) {
  const area = Number($('area').value || 0);
  const p = priceForMaterial(r.material_id);
  const machine = itemById(state.machines, r.machine_id) || itemById(state.operationMachines, r.machine_id);
  const costItemName = nameById(state.costItems, r.cost_item_id).toLowerCase();
  let price = r.manual_price ?? p?.price ?? 0;
  let src = p ? sourceLabel(p.source_name) : 'Резервная база цен';
  let date = p?.effective_date ? new Date(p.effective_date).toLocaleDateString('ru-RU') : '—';
  if (costItemName.includes('техник') || costItemName.includes('амортизац') || costItemName.includes('ремонт') || costItemName.includes('обслуж')) {
    if (r.machine_usage_type === 'rent') { price = machine?.rent_price || price; src = 'Арендная ставка техники'; }
    else {
      const base = machine?.default_price || price;
      if (costItemName.includes('амортизац')) price = base * 0.5;
      else if (costItemName.includes('ремонт')) price = base * 0.3;
      else if (costItemName.includes('обслуж')) price = base * 0.2;
      else price = base;
      src = 'Справочник техники';
    }
    date = $('calcDate')?.value ? new Date($('calcDate').value).toLocaleDateString('ru-RU') : new Date().toLocaleDateString('ru-RU');
  }
  if (r.manual_price !== undefined) { src = 'Ручной ввод'; date = $('calcDate')?.value ? new Date($('calcDate').value).toLocaleDateString('ru-RU') : new Date().toLocaleDateString('ru-RU'); }
  const quantity = area * Number(r.area_factor || 1) * Number(r.rate || 0) * Number(r.coefficient || 1);
  return { quantity, price, amount: quantity * Number(price || 0), source: src, date };
}

function renderRows() {
  if (!state.rows.length) { $('rowsBox').innerHTML = `<div class="empty-state">Расчёт ещё не сформирован. На предыдущем шаге выберите технику и необходимые параметры.</div>`; return; }
  let total = 0;
  const rows = state.rows.map((r, i) => {
    const est = rowEstimate(r); total += est.amount;
    const usage = r.machine_usage_type === 'rent' ? '<span class="badge rent">Аренда</span>' : '<span class="badge own">Своя техника</span>';
    return `<tr><td>${i + 1}</td><td>${nameById(state.operations, r.operation_id)}</td><td>${nameById(state.machines, r.machine_id) || nameById(state.operationMachines, r.machine_id) || '—'}</td><td>${usage}</td><td>${nameById(state.costItems, r.cost_item_id)}</td><td>${nameById(state.materials, r.material_id)}</td><td>${num(r.rate, 2)}</td><td>${num(est.price, 2)}<div class="price-source">${est.source}</div></td><td>${est.date}</td><td>${num(est.quantity, 2)}</td><td><strong>${rub(est.amount)}</strong></td></tr>`;
  }).join('');
  const comparison = comparisonHtml(state.lastResult?.comparison);
  $('rowsBox').innerHTML = `${comparison}<div class="table-wrap"><table><thead><tr><th>№</th><th>Работа</th><th>Техника</th><th>Тип</th><th>Статья затрат</th><th>Ресурс</th><th>Норма</th><th>Цена / источник</th><th>Дата цены</th><th>Количество</th><th>Сумма</th></tr></thead><tbody>${rows}</tbody><tfoot><tr><td colspan="10">Итого</td><td>${rub(total)}</td></tr></tfoot></table></div>`;
}

function comparisonHtml(comparison) {
  if (!comparison) return '';
  const cheaper = comparison.cheaper_usage_type === 'rent' ? 'аренда' : 'своя техника';
  return `<div class="comparison-box"><div><small>Своя техника</small><strong>${rub(comparison.own_total)}</strong></div><div><small>Аренда</small><strong>${rub(comparison.rent_total)}</strong></div><div><small>Выгоднее</small><strong>${cheaper}, разница ${rub(Math.abs(comparison.delta))}</strong></div></div>`;
}

function requestPayload() {
  return {
    crop_id: Number($('crop').value),
    federal_district_id: Number($('federalDistrict').value),
    region_id: 0,
    area_ha: Number($('area').value),
    calculation_date: $('calcDate')?.value || '',
    calculation_mode: $('calcMode')?.value || 'single_operation',
    include_comparison: Boolean($('includeComparison')?.checked),
    condition_coefficient_ids: selectedConditionCoefficientIds(),
    rows: state.rows
  };
}
async function calculate(save, opts = {}) {
  try {
    if (!state.rows.length) return;
    const result = await api(save ? '/api/calculations' : '/api/calculate', { method: 'POST', body: JSON.stringify(requestPayload()) });
    state.lastResult = result;
    renderRows();
    updateMetrics(result);
    if (save) { await loadHistory(); if (!opts.silent) toast('Расчёт сохранён'); }
    else if (!opts.silent) toast('Расчёт выполнен');
  } catch (e) { toast(e.message, true); }
}

function updateMetrics(result = state.lastResult) {
  const total = result?.total_cost ?? state.rows.reduce((s, r) => s + rowEstimate(r).amount, 0);
  const area = Number($('area').value || 0);
  $('metricTotal').textContent = rub(total);
  $('metricPerHa').textContent = `${rub(area > 0 ? total / area : 0)}/га`;
  $('metricRows').textContent = String(state.rows.length);
  $('metricStatus').textContent = result?.id ? 'Расчёт сохранён' : (state.rows.length ? 'Расчёт сформирован' : 'Готов к расчёту');
}

async function loadHistory() {
  try { state.history = await api('/api/calculations'); } catch (_) { state.history = []; }
  renderHistory();
}

function renderHistory() {
  const rows = (state.history || []).map(c => `<tr class="history-row" data-calc-id="${c.id}"><td>${new Date(c.created_at).toLocaleString('ru-RU')}</td><td>${c.crop_name || '—'}</td><td>${c.federal_district || c.region_name || '—'}</td><td>${num(c.area_ha, 2)}</td><td><strong>${rub(c.total_cost)}</strong></td><td>${rub(c.cost_per_ha)}/га</td><td class="row-actions"><button class="outline compact detail-btn" data-id="${c.id}">Открыть</button><button class="outline compact export-one-btn" data-id="${c.id}">Excel</button><button class="danger compact delete-calc-btn" data-id="${c.id}">Удалить</button></td></tr>`).join('');
  $('view-history').innerHTML = `<article class="panel"><div class="history-toolbar"><div><h2>История расчётов</h2><p>${state.currentUser ? 'Показаны расчёты текущей учётной записи.' : 'Показаны расчёты, сохранённые в этом браузере.'}</p></div><div class="export-actions"><button class="outline" id="exportAllDetailedExcel">Выгрузить все расчёты</button></div></div><div class="table-wrap"><table><thead><tr><th>Дата</th><th>Культура</th><th>Федеральный округ</th><th>Площадь, га</th><th>Итог</th><th>На 1 га</th><th>Действия</th></tr></thead><tbody>${rows || '<tr><td colspan="7">История пока пустая</td></tr>'}</tbody></table></div><p class="source-note">Нажмите на строку, чтобы открыть детализацию расчёта, выгрузить его в Excel или удалить.</p></article>`;
  $('exportAllDetailedExcel')?.addEventListener('click', exportAllDetailedExcel);
  document.querySelectorAll('.history-row').forEach(row => row.addEventListener('click', (e) => {
    if (e.target.closest('button')) return;
    openCalculationDetail(row.dataset.calcId);
  }));
  document.querySelectorAll('.detail-btn').forEach(btn => btn.addEventListener('click', () => openCalculationDetail(btn.dataset.id)));
  document.querySelectorAll('.export-one-btn').forEach(btn => btn.addEventListener('click', async () => exportOneDetailedExcel(btn.dataset.id)));
  document.querySelectorAll('.delete-calc-btn').forEach(btn => btn.addEventListener('click', async () => deleteCalculation(btn.dataset.id)));
}

async function openCalculationDetail(id) {
  try {
    const detail = await api(`/api/calculations/${id}`);
    state.selectedDetail = detail;
    const rows = detailRowsHtml(detail);
    $('detailModalBody').innerHTML = `<div class="detail-summary"><div><small>Дата</small><strong>${new Date(detail.created_at).toLocaleString('ru-RU')}</strong></div><div><small>Культура</small><strong>${detail.crop_name || '—'}</strong></div><div><small>Федеральный округ</small><strong>${detail.federal_district || detail.region_name || '—'}</strong></div><div><small>Площадь</small><strong>${num(detail.area_ha, 2)} га</strong></div><div><small>Общая стоимость</small><strong>${rub(detail.total_cost)}</strong></div><div><small>Стоимость на 1 га</small><strong>${rub(detail.cost_per_ha)}/га</strong></div></div>${comparisonHtml(detail.comparison)}<div class="table-wrap"><table><thead><tr><th>№</th><th>Работа</th><th>Техника</th><th>Тип</th><th>Статья</th><th>Ресурс</th><th>Норма</th><th>Цена</th><th>Источник</th><th>Дата цены</th><th>Количество</th><th>Сумма</th></tr></thead><tbody>${rows}</tbody><tfoot><tr><td colspan="11">Итого</td><td>${rub(detail.total_cost)}</td></tr></tfoot></table></div>`;
    $('exportDetailBtn')?.remove();
    const btn = document.createElement('button');
    btn.id = 'exportDetailBtn';
    btn.className = 'outline';
    btn.textContent = 'Выгрузить расчёт в Excel';
    btn.addEventListener('click', () => exportDetailExcel(detail));
    document.querySelector('#detailModal .modal-actions').prepend(btn);
    openModal('detailModal');
  } catch (e) { toast(e.message, true); }
}

function detailRowsHtml(detail) {
  return (detail.rows || []).map((r, i) => `<tr><td>${i + 1}</td><td>${r.operation_name || '—'}</td><td>${r.machine_name || '—'}</td><td>${r.machine_usage_type === 'rent' ? 'Аренда' : 'Своя'}</td><td>${r.cost_item_name || '—'}</td><td>${r.material_name || '—'}</td><td>${num(r.rate, 2)}</td><td>${num(r.price, 2)}</td><td>${r.price_source || r.machine_price_source || '—'}</td><td>${formatDate(r.price_date)}</td><td>${num(r.quantity, 2)}</td><td><strong>${rub(r.amount)}</strong></td></tr>`).join('') || '<tr><td colspan="12">Нет строк расчёта</td></tr>';
}

async function deleteCalculation(id) {
  if (!confirm('Удалить этот расчёт из истории?')) return;
  try {
    await api(`/api/calculations/${id}`, { method: 'DELETE' });
    if (state.selectedDetail && Number(state.selectedDetail.id) === Number(id)) closeModal('detailModal');
    await loadHistory();
    toast('Расчёт удалён');
  } catch (e) { toast(e.message, true); }
}

function download(name, content, type) { const a = document.createElement('a'); a.href = URL.createObjectURL(new Blob([content], { type })); a.download = name; a.click(); URL.revokeObjectURL(a.href); }
function esc(v) { return String(v ?? '').replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;'); }
function formatDate(v) { return v ? new Date(v).toLocaleDateString('ru-RU') : '—'; }
function excelHeader(title) { return `<html><head><meta charset="utf-8"></head><body><h2>${esc(title)}</h2>`; }
function exportDetailExcel(detail) {
  const summary = `<table><tr><th>Дата</th><td>${new Date(detail.created_at).toLocaleString('ru-RU')}</td></tr><tr><th>Культура</th><td>${esc(detail.crop_name || '')}</td></tr><tr><th>Федеральный округ</th><td>${esc(detail.federal_district || detail.region_name || '')}</td></tr><tr><th>Площадь, га</th><td>${num(detail.area_ha, 2)}</td></tr><tr><th>Общая стоимость</th><td>${num(detail.total_cost, 2)}</td></tr><tr><th>Стоимость на 1 га</th><td>${num(detail.cost_per_ha, 2)}</td></tr></table><br>`;
  const rows = (detail.rows || []).map((r, i) => `<tr><td>${i + 1}</td><td>${esc(r.operation_name || '')}</td><td>${esc(r.machine_name || '')}</td><td>${r.machine_usage_type === 'rent' ? 'Аренда' : 'Своя'}</td><td>${esc(r.cost_item_name || '')}</td><td>${esc(r.material_name || '')}</td><td>${num(r.rate, 2)}</td><td>${num(r.price, 2)}</td><td>${esc(r.price_source || r.machine_price_source || '')}</td><td>${formatDate(r.price_date)}</td><td>${num(r.quantity, 2)}</td><td>${num(r.amount, 2)}</td></tr>`).join('');
  const table = `<table border="1"><thead><tr><th>№</th><th>Работа</th><th>Техника</th><th>Тип техники</th><th>Статья затрат</th><th>Ресурс</th><th>Норма</th><th>Цена</th><th>Источник цены</th><th>Дата цены</th><th>Количество</th><th>Сумма</th></tr></thead><tbody>${rows}</tbody><tfoot><tr><td colspan="11">Итого</td><td>${num(detail.total_cost, 2)}</td></tr></tfoot></table>`;
  download(`agrocalc-calculation-${detail.id}.xls`, excelHeader(`Детализация расчёта №${detail.id}`) + summary + table + '</body></html>', 'application/vnd.ms-excel;charset=utf-8');
}
async function exportOneDetailedExcel(id) {
  try {
    const detail = await api(`/api/calculations/${id}`);
    exportDetailExcel(detail);
  } catch (e) { toast(e.message, true); }
}
async function exportAllDetailedExcel() {
  if (!state.history.length) return toast('История пока пуста', true);
  try {
    const details = [];
    for (const c of state.history) details.push(await api(`/api/calculations/${c.id}`));
    let html = excelHeader('Детализированная выгрузка всех расчётов');
    html += `<table border="1"><thead><tr><th>№ расчёта</th><th>Дата</th><th>Культура</th><th>Федеральный округ</th><th>Площадь, га</th><th>Общая стоимость</th><th>Стоимость на 1 га</th><th>№ строки</th><th>Работа</th><th>Техника</th><th>Тип техники</th><th>Статья затрат</th><th>Ресурс</th><th>Норма</th><th>Цена</th><th>Источник цены</th><th>Дата цены</th><th>Количество</th><th>Сумма</th></tr></thead><tbody>`;
    for (const d of details) {
      if (!d.rows || !d.rows.length) {
        html += `<tr><td>${d.id}</td><td>${new Date(d.created_at).toLocaleString('ru-RU')}</td><td>${esc(d.crop_name || '')}</td><td>${esc(d.federal_district || d.region_name || '')}</td><td>${num(d.area_ha, 2)}</td><td>${num(d.total_cost, 2)}</td><td>${num(d.cost_per_ha, 2)}</td><td colspan="12">Нет строк</td></tr>`;
      }
      (d.rows || []).forEach((r, i) => {
        html += `<tr><td>${d.id}</td><td>${new Date(d.created_at).toLocaleString('ru-RU')}</td><td>${esc(d.crop_name || '')}</td><td>${esc(d.federal_district || d.region_name || '')}</td><td>${num(d.area_ha, 2)}</td><td>${num(d.total_cost, 2)}</td><td>${num(d.cost_per_ha, 2)}</td><td>${i + 1}</td><td>${esc(r.operation_name || '')}</td><td>${esc(r.machine_name || '')}</td><td>${r.machine_usage_type === 'rent' ? 'Аренда' : 'Своя'}</td><td>${esc(r.cost_item_name || '')}</td><td>${esc(r.material_name || '')}</td><td>${num(r.rate, 2)}</td><td>${num(r.price, 2)}</td><td>${esc(r.price_source || r.machine_price_source || '')}</td><td>${formatDate(r.price_date)}</td><td>${num(r.quantity, 2)}</td><td>${num(r.amount, 2)}</td></tr>`;
      });
    }
    html += '</tbody></table></body></html>';
    download('agrocalc-all-calculations-detailed.xls', html, 'application/vnd.ms-excel;charset=utf-8');
  } catch (e) { toast(e.message, true); }
}

function renderPriceInfo() {
  const count = state.prices.length;
  const fuelPrice = state.prices.find(x => String(x.material_name || '').toLowerCase().includes('дизель'));
  const fd = activeFDName() || 'Выбранный федеральный округ';
  $('priceInfo').textContent = fuelPrice ? `${fd}: цена ДТ ${num(fuelPrice.price, 2)} руб./${fuelPrice.unit_name || 'л'} — источник ${sourceLabel(fuelPrice.source_name)}` : (count ? `${fd}: найдено ${count} ценовых записей` : `${fd}: используется последняя сохранённая база цен`);
}

function renderSettings() {
  $('view-settings').innerHTML = `<div class="page-grid settings-grid"><article class="panel"><div class="panel-head wide"><div><h2>Профиль расчёта</h2><p>Эти значения подставляются в новые ведомости и помогают быстрее собирать расчёт.</p></div></div><div class="settings-form"><label class="field-card"><span>Организация</span><input id="settingsOrganization" value="${esc(state.settings.organization)}" placeholder="Например, ООО Агрофирма"/></label><label class="field-card"><span>Автор отчёта</span><input id="settingsAuthor" value="${esc(state.settings.reportAuthor || state.currentUser?.full_name || '')}" placeholder="Например, Иванов И.И."/></label><label class="field-card"><span>Федеральный округ</span><select id="settingsDefaultDistrict">${optionHtml(state.federalDistricts, state.settings.defaultDistrictId)}</select></label><label class="field-card"><span>Культура по умолчанию</span><select id="settingsDefaultCrop">${optionHtml(state.crops, state.settings.defaultCropId || state.crops.find(x => x.name === 'Кукуруза')?.id)}</select></label><label class="field-card"><span>Техника по умолчанию</span><select id="settingsMachineUsage"><option value="own" ${state.settings.defaultMachineUsage === 'own' ? 'selected' : ''}>Своя техника</option><option value="rent" ${state.settings.defaultMachineUsage === 'rent' ? 'selected' : ''}>Аренда</option></select></label><label class="switch-row"><input type="checkbox" id="settingsAutosave" ${state.settings.autosaveHistory ? 'checked' : ''}><span>Сохранять сформированные расчёты в журнал</span></label><label class="switch-row"><input type="checkbox" id="settingsShowHints" ${state.settings.showHints ? 'checked' : ''}><span>Показывать пояснения в рабочих карточках</span></label><div class="settings-actions"><button id="saveSettingsBtn">Сохранить настройки</button></div></div></article><article class="panel"><div class="panel-head wide"><div><h2>ПолеПлан в работе</h2><p>Сервис настроен как рабочий кабинет экономиста: минимум лишних переходов, максимум прозрачности по источникам.</p></div></div><div class="directory-grid two-col"><div class="directory-card"><h3>Быстрый старт</h3><p>Округ, культура и тип техники применяются при первом открытии мастера и в типовых расчётах.</p></div><div class="directory-card"><h3>Сравнение техники</h3><p>Собственная техника раскладывается на амортизацию, ремонт и ТО, аренда считается отдельной ставкой.</p></div><div class="directory-card"><h3>Журнал</h3><p>Каждый сохранённый расчёт можно открыть, проверить источники цен и выгрузить в Excel.</p></div><div class="directory-card"><h3>Компактный режим</h3><p>Скрывает вспомогательные пояснения, когда интерфейс нужен для регулярной работы.</p></div></div></article></div>`;
  $('saveSettingsBtn')?.addEventListener('click', () => {
    state.settings.organization = $('settingsOrganization').value.trim();
    state.settings.reportAuthor = $('settingsAuthor').value.trim();
    state.settings.defaultDistrictId = $('settingsDefaultDistrict').value;
    state.settings.defaultCropId = $('settingsDefaultCrop').value;
    state.settings.defaultMachineUsage = $('settingsMachineUsage').value;
    state.settings.autosaveHistory = $('settingsAutosave').checked;
    state.settings.showHints = $('settingsShowHints').checked;
    saveSettingsState();
    applySettingsToForm();
    toast('Настройки сохранены');
  });
}

function renderHelp() {
  $('view-help').innerHTML = `<article class="panel"><div class="panel-head"><div><h2>Как устроен расчёт</h2><p>ПолеПлан собирает строки затрат из справочников, норм расхода, коэффициентов условий и текущих цен.</p></div></div><div class="directory-grid"><div class="directory-card"><h3>1. Режим</h3><p>Одна операция подходит для быстрой оценки. Технологическая карта собирает набор операций по выбранной культуре.</p></div><div class="directory-card"><h3>2. Условия</h3><p>Коэффициенты влажности, рельефа, каменистости, удалённости и сложности перемножаются в общий поправочный коэффициент.</p></div><div class="directory-card"><h3>3. Техника</h3><p>Для собственной техники отдельно считаются амортизация, ремонт и ТО. Аренда использует арендную ставку машино-часа.</p></div><div class="directory-card"><h3>4. Цены</h3><p>В детализации отображаются источник и дата актуальности цены, чтобы ведомость можно было проверить позже.</p></div><div class="directory-card"><h3>5. Журнал</h3><p>Сохранённые расчёты доступны в истории: открывайте детализацию, экспортируйте Excel или удаляйте устаревшие записи.</p></div><div class="directory-card"><h3>6. Настройки</h3><p>Сохраните организацию, автора, округ, культуру и тип техники, чтобы не повторять ввод в каждом расчёте.</p></div></div></article>`;
}

function applySettingsToForm() {
  const defaultFD = state.settings.defaultDistrictId || state.federalDistricts.find(x => x.code === 'ЮФО')?.id;
  if (defaultFD) $('federalDistrict').value = defaultFD;
  const defaultCrop = state.settings.defaultCropId || state.crops.find(x => x.name === 'Пшеница')?.id;
  if (defaultCrop) $('crop').value = defaultCrop;
  $('machineUsage').value = state.settings.defaultMachineUsage || 'own';
  document.body.classList.toggle('compact-hints', !state.settings.showHints);
  updateSelectionSummary();
}

async function init() {
  try {
    loadSettingsState();
    $('calcDate').valueAsDate = new Date();
    await refreshAuth();
    const [fds, crops, operations, operationRules, norms, conditionCoefficients, materials, machines, costItems] = await Promise.all([api('/api/federal-districts'), api('/api/crops'), api('/api/operations'), api('/api/operation-rules'), api('/api/norms'), api('/api/condition-coefficients'), api('/api/materials'), api('/api/machines'), api('/api/cost-items')]);
    Object.assign(state, { federalDistricts: fds || [], crops: crops || [], operations: operations || [], operationRules: operationRules || [], norms: norms || [], conditionCoefficients: conditionCoefficients || [], materials: materials || [], machines: machines || [], costItems: costItems || [] });
    $('federalDistrict').innerHTML = optionHtml(state.federalDistricts);
    $('crop').innerHTML = optionHtml(state.crops);
    $('operation').innerHTML = optionHtml(state.operations);
    $('material').innerHTML = optionHtml(state.materials);
    applySettingsToForm();
    if (!state.settings.defaultDistrictId) {
      const yufo = state.federalDistricts.find(x => x.code === 'ЮФО');
      if (yufo) $('federalDistrict').value = yufo.id;
    }
    await loadPrices();
    renderConditionCoefficientFields();
    await loadTechMapTemplates();
    await loadOperationMachines();
    updateSmartFields();
    renderRows(); updateMetrics(); loadHistory(); renderSettings(); renderHelp();
    setView((location.hash || '#calculations').replace('#', ''));
  } catch (e) { toast(e.message, true); }
}

document.addEventListener('click', (e) => {
  const historyBtn = e.target.closest('#historyShortcut');
  if (historyBtn) { e.preventDefault(); location.hash = 'history'; setView('history'); return; }
  const viewBtn = e.target.closest('[data-view]'); if (viewBtn) { const view = viewBtn.dataset.view; location.hash = view; setView(view); setSidebarOpen(false); }
  const closeBtn = e.target.closest('[data-close]'); if (closeBtn) closeModal(closeBtn.dataset.close);
  if (e.target.classList.contains('modal')) closeModal(e.target.id);
});
window.addEventListener('hashchange', () => setView((location.hash || '#calculations').replace('#', '')));
function setSidebarOpen(open) {
  document.querySelector('.sidebar')?.classList.toggle('open', open);
  document.body.classList.toggle('sidebar-open', open);
}
$('menuToggle').addEventListener('click', () => setSidebarOpen(!document.querySelector('.sidebar')?.classList.contains('open')));
document.addEventListener('keydown', (e) => { if (e.key === 'Escape') setSidebarOpen(false); });
$('loginBtn').addEventListener('click', () => state.authMode === 'register' ? openAuth('login') : login());
$('registerBtn').addEventListener('click', () => state.authMode === 'register' ? register() : openAuth('register'));
$('goStep2').addEventListener('click', async () => { if (!validateStep1()) return; if ($('calcMode').value === 'tech_map') await loadTechMapTemplates(); else await loadOperationMachines(); updateSmartFields(); setStep(2); });
$('backStep1').addEventListener('click', () => setStep(1));
$('homeStep').addEventListener('click', goHome);
$('editRows').addEventListener('click', () => setStep(2));
$('buildRows').addEventListener('click', () => buildSmartRows());
$('calcMode').addEventListener('change', async () => { await loadTechMapTemplates(); updateSmartFields(); updateSelectionSummary(); });
$('operation').addEventListener('change', async () => { await loadOperationMachines(); updateSmartFields(); });
$('crop').addEventListener('change', async () => { await loadTechMapTemplates(); updateSmartFields(); });
$('machine').addEventListener('change', updateMachineFields);
$('machineUsage').addEventListener('change', updateMachineFields);
$('federalDistrict').addEventListener('change', async () => { await loadPrices(); renderRows(); updateMetrics(); updateSelectionSummary(); });
$('area').addEventListener('input', () => { renderRows(); updateMetrics(); updateSelectionSummary(); });
$('historyShortcut')?.addEventListener('click', () => { location.hash = 'history'; setView('history'); });
init();
