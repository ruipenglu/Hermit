'use strict';
const $ = id => document.getElementById(id);
const fragment = location.hash.slice(1);
if (fragment) { sessionStorage.setItem('hermit-token', fragment); history.replaceState(null, '', '/'); }
const token = sessionStorage.getItem('hermit-token') || '';
let state = null, nodes = [], editing = null, busy = false, stopped = false, toastTimer;
const titles = {mappings: '端口映射', nodes: '活动节点', tests: '测试记录', settings: '环境与恢复'};
const statuses = {verified: ['身份已确认', 'good'], failed: ['连接失败', 'bad'], unconfirmed: ['身份未确认', 'warn'], mismatch: ['节点不匹配', 'bad']};
function el(tag, className, text) { const element = document.createElement(tag); if (className) element.className = className; if (text !== undefined) element.textContent = text; return element; }
function pill(text, tone = '') { return el('span', `pill ${tone}`, text); }
function notify(message, error = false) { clearTimeout(toastTimer); $('toast').textContent = message; $('toast').className = error ? 'error' : ''; $('toast').hidden = false; toastTimer = setTimeout(() => $('toast').hidden = true, error ? 12000 : 5000); }
async function api(path, data) {
  const response = await fetch('/api/' + path, {method: data === undefined ? 'GET' : 'POST', headers: {'X-Hermit-Token': token, ...(data === undefined ? {} : {'Content-Type': 'application/json'})}, ...(data === undefined ? {} : {body: JSON.stringify(data)})});
  const result = await response.json(); if (!response.ok) throw new Error(result.error || '操作失败'); return result;
}
async function action(task) {
  if (busy) return; busy = true; document.body.setAttribute('aria-busy', 'true');
  document.querySelectorAll('button').forEach(button => button.disabled = true);
  try { await task(); } catch (error) { notify(error.message || '无法连接 Hermit，请重新启动应用', true); }
  finally { busy = false; document.body.removeAttribute('aria-busy'); document.querySelectorAll('button').forEach(button => button.disabled = false); await refresh(); }
}
function view(name) { document.querySelectorAll('.view').forEach(section => section.hidden = section.id !== 'view-' + name); document.querySelectorAll('.nav').forEach(button => button.classList.toggle('active', button.dataset.view === name)); $('page-title').textContent = titles[name]; }
function friendly(name) { const mapping = state?.mappings.find(item => 'hermit-' + item.id === name); return mapping ? `手动 VLESS · ${mapping.port}` : name; }
function mappingState(item) {
  if (!item.enabled) return ['已停用', ''];
  if (!state.enabled) return ['未启用', ''];
  if (state.revision !== state.appliedRevision) return ['待应用', 'warn'];
  if (state.status !== 'applied') return ['需检查', 'bad'];
  return ['已应用', 'good'];
}
function renderMappings() {
  $('mapping-rows').replaceChildren(); $('empty-mappings').hidden = state.mappings.length > 0;
  $('mapping-count').textContent = `${state.mappings.length} 条映射`;
  for (const item of [...state.mappings].sort((a,b) => a.port - b.port)) {
    const row = el('tr'), portCell = el('td'), portWrap = el('div', 'port-cell'), [label,tone] = mappingState(item);
    const dot = el('i', 'dot ' + (tone === 'good' ? 'green' : tone === 'bad' ? 'red' : ''));
    const port = el('div'); port.append(el('span', 'port-number', item.port), el('span', 'subtle', '127.0.0.1')); portWrap.append(dot, port); portCell.append(portWrap);
    const target = el('td'); target.append(el('span', 'node-name', item.source === 'manual-uri' ? '手动 VLESS 节点' : item.target), el('span', 'subtle', item.source === 'manual-uri' ? '加密保存在本机' : (nodes.find(node => node.name === item.target)?.type || '节点需重新检查')));
    const status = el('td'); status.append(pill(label, tone));
    const operations = el('td'), buttons = el('div', 'row-actions'), test = el('button', 'small-button', '▷ 测试'), edit = el('button', 'edit-button', '编辑');
    test.disabled = !state.enabled || !item.enabled || state.revision !== state.appliedRevision || busy;
    test.addEventListener('click', () => runTest(item.id)); edit.addEventListener('click', () => editMapping(item)); buttons.append(test, edit); operations.append(buttons); row.append(portCell,target,status,operations); $('mapping-rows').append(row);
  }
}
function renderTests(container, tests) {
  container.replaceChildren(); if (!tests.length) { container.append(el('p','test-empty','还没有测试记录。应用映射后，点击「测试」验证固定出口。')); return; }
  const list = el('div', 'test-list');
  for (const test of tests) {
    const details = el('details','test-entry'), summary = el('summary','test-summary'), left = el('div','test-main'), info = el('div');
    info.append(el('div','test-port',`127.0.0.1:${test.port}`), el('div','test-meta',new Date(test.time).toLocaleString('zh-CN', {month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'})));
    left.append(el('i','dot '+(test.status === 'verified' ? 'green' : 'red')), info);
    const right = el('div','test-result'), [label,tone] = statuses[test.status] || ['未知状态','warn']; right.append(pill(label,tone),el('span','subtle',`${test.exitIp || '未取得出口 IP'} · ${test.latencyMs ?? '—'} ms`)); summary.append(left,right);
    const detail = el('div','test-detail'); detail.append(el('div','',test.message),el('div','',`预期节点：${friendly(test.expectedProxy)}`),el('div','',`实际代理链：${test.actualChain.length ? test.actualChain.map(friendly).join(' → ') : '未捕获'}`)); details.append(summary,detail); list.append(details);
  }
  container.append(list);
}
function render() {
  if (!state) return;
  const labels = {disabled:['固定端口尚未启用','映射已保存在本机，开启后应用到 Clash。'],applied:['固定端口已应用','脚本与监听就绪，可点击测试核验指定出口。'],'saved-pending-apply':['映射已保存，等待应用','当前运行的仍是上次应用的配置。'],degraded:['运行状态需要检查','请查看下方诊断，测试连接后确认实际出口。'],unavailable:['Clash 暂不可用','请启动 Clash Verge Rev，并重新检查环境。'],conflict:['全局脚本存在冲突','请先处理外部修改，Hermit 已暂停写入。'],'recovery-required':['上次操作尚未完成','请恢复到变更前状态，再继续配置。']};
  const [title,detail] = labels[state.status] || ['状态未知','请重新检查环境']; $('status-title').textContent = title; $('status-detail').textContent = detail;
  $('power').setAttribute('aria-checked', String(state.enabled)); $('power').setAttribute('aria-label',state.enabled ? '关闭并恢复原脚本' : '启用固定端口映射');
  $('power').disabled = busy || state.recoveryRequired || state.status === 'conflict';
  $('apply').hidden = !state.enabled || state.revision === state.appliedRevision || state.recoveryRequired;
  $('recover').hidden = !state.recoveryRequired; $('notice').hidden = !state.errors.length; $('notice').textContent = state.errors.join('；');
  renderMappings(); const history = [...state.history].reverse(); renderTests($('recent-tests'),history.slice(0,4)); renderTests($('test-history'),history);
  $('editor-test').disabled = busy || !state.enabled || state.revision !== state.appliedRevision;
}
async function refresh() { if (busy || stopped) return; try { state = await api('state'); render(); } catch (error) { $('notice').hidden = false; $('notice').textContent = error.message; } }
function renderNodeOptions(selected = $('target').value) {
  $('target').replaceChildren(); const placeholder = el('option','',nodes.length ? '请选择一个节点' : '没有可用节点'); placeholder.value = ''; $('target').append(placeholder);
  for (const node of nodes) { const option = el('option','',node.name + ' · ' + node.type); option.value = node.name; $('target').append(option); }
  if (selected && !nodes.some(node => node.name === selected)) { const option = el('option','',selected + '（已失效）'); option.value = selected; $('target').append(option); }
  $('target').value = selected;
}
async function refreshNodes() {
  nodes = await api('nodes'); renderNodeOptions(); $('node-list').replaceChildren();
  for (const node of nodes) {
    const card = el('div','node-card'), info = el('div'); info.append(el('div','node-name',node.name),el('span','subtle',node.type));
    const use = el('button','text-button','绑定'); use.addEventListener('click',() => { resetEditor(); $('target').value = node.name; view('mappings'); $('port').focus(); }); card.append(el('span','node-symbol','↗'),info,use); $('node-list').append(card);
  }
  if (!nodes.length) $('node-list').append(el('p','test-empty','当前没有可用的原子代理节点。'));
  render();
}
async function checkEnvironment() {
  try {
    const environment = await api('doctor'); $('connection').replaceChildren(el('i','dot green'),document.createTextNode('已连接 Clash Verge Rev')); $('environment').replaceChildren();
    for (const [key,value] of [['Clash Verge Rev',environment.clashVersion],['Mihomo',environment.mihomoVersion],['活动节点',`${environment.atomicNodeCount} 个`],['Clash 数据目录',environment.dataDirectory],['全局脚本',environment.script]]) { $('environment').append(el('dt','',key),el('dd','',value)); }
  } catch(error) { $('connection').replaceChildren(el('i','dot red'),document.createTextNode('Clash 环境需检查')); $('environment').replaceChildren(el('dt','','环境检查'),el('dd','',error.message)); }
}
function updateSource() { const manual = document.querySelector('input[name=source]:checked').value === 'manual-uri'; $('manual-fields').hidden = !manual; $('active-fields').hidden = manual; $('target').required = !manual; $('uri').required = manual && !(editing?.source === 'manual-uri' && editing.hasCredential); }
function resetEditor() {
  editing = null; $('mapping-form').reset(); $('editor-title').textContent = '新增端口映射'; $('delete').hidden = $('editor-test').hidden = true; $('uri').value = ''; $('uri').type = 'password'; $('uri').placeholder = 'vless://…'; $('show-uri').textContent = '显示输入内容';
  let port = 18080; while(state?.mappings.some(item => item.port === port)) port++; $('port').value = port; $('target').value = ''; updateSource();
}
function editMapping(item) { resetEditor(); editing = item; $('editor-title').textContent = '编辑端口映射'; $('port').value = item.port; document.querySelector(`input[name=source][value="${item.source}"]`).checked = true; renderNodeOptions(item.target || ''); $('mapping-enabled').checked = item.enabled; $('uri').placeholder = item.hasCredential ? '凭据已保存，留空保留原节点' : 'vless://…'; $('delete').hidden = $('editor-test').hidden = false; updateSource(); render(); }
function runTest(id) { return action(async () => { const result = await api('test',{id}); notify(result.message,result.status !== 'verified'); }); }
document.querySelectorAll('.nav').forEach(button => button.addEventListener('click',() => view(button.dataset.view)));
document.querySelectorAll('input[name=source]').forEach(input => input.addEventListener('change',updateSource));
$('add').addEventListener('click',() => {resetEditor();$('port').focus();}); $('empty-add').addEventListener('click',() => {resetEditor();$('port').focus();}); $('reset-editor').addEventListener('click',resetEditor);
$('all-tests').addEventListener('click',() => view('tests'));
$('power').addEventListener('click',() => action(async () => { await api(state.enabled ? 'disable' : 'enable',{}); notify(state.enabled ? '已关闭固定端口并恢复原脚本' : '已应用固定端口，可开始测试'); }));
$('apply').addEventListener('click',() => action(async () => {await api('apply',{});notify('映射更改已应用');}));
$('recover').addEventListener('click',() => action(async () => {await api('recover',{});notify('已恢复变更前状态');}));
$('mapping-form').addEventListener('submit', event => { event.preventDefault(); action(async () => { const source = document.querySelector('input[name=source]:checked').value; const data = {port:Number($('port').value),source,enabled:$('mapping-enabled').checked}; if (editing) data.id = editing.id; if(source === 'active-profile') data.target = $('target').value; else if($('uri').value) data.uri = $('uri').value; await api('mappings/save',data); $('uri').value = ''; resetEditor(); notify('映射已保存，请应用更改或开启总开关'); }); });
$('delete').addEventListener('click',() => action(async () => {await api('mappings/delete',{id:editing.id});resetEditor();notify('映射已删除，请应用更改使其生效');}));
$('editor-test').addEventListener('click',() => editing && runTest(editing.id));
$('show-uri').addEventListener('click',() => {const show = $('uri').type === 'password';$('uri').type = show ? 'text' : 'password';$('show-uri').textContent = show ? '隐藏输入内容' : '显示输入内容';});
$('refresh-nodes').addEventListener('click',() => action(refreshNodes)); $('check-environment').addEventListener('click',() => action(checkEnvironment));
$('quit').addEventListener('click',async () => {if(busy) return;try{await api('quit',{});stopped = true;$('notice').hidden = false;$('notice').textContent = 'Hermit 已退出，可以关闭此标签页。';document.querySelectorAll('button').forEach(button => button.disabled = true);}catch(error){notify(error.message,true);}});
(async () => {await refresh(); await Promise.allSettled([refreshNodes(),checkEnvironment()]); resetEditor();})();
setInterval(() => {if(!busy && !stopped) refresh();},8000);
