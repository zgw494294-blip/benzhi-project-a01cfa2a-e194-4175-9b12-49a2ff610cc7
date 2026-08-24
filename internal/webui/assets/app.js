"use strict";

const state = { cases: [], selected: null, audit: [], activeTab: "revisions", busy: false };
const $ = (selector) => document.querySelector(selector);
const escapeHTML = (value) => String(value ?? "").replace(/[&<>'"]/g, (char) => ({"&":"&amp;","<":"&lt;",">":"&gt;","'":"&#39;",'"':"&quot;"})[char]);
const statusLabels = {draft:"草稿",pending_check:"待完整性检查",pending_review:"待人工判读",retake:"待返拍",reinspection:"待复验",frozen:"已冻结",released:"已放行"};
const role = () => $("#role").value;
const actor = () => $("#actor").value.trim();
const key = (operation) => `${operation}-${Date.now()}-${crypto.randomUUID()}`;
const showBusy = (visible) => { state.busy = visible; $("#busy").hidden = !visible; };

async function api(path, options = {}) {
  const headers = new Headers(options.headers || {});
  headers.set("X-Actor", actor() || "未登记操作人");
  headers.set("X-Role", role());
  if (options.json !== undefined) {
    headers.set("Content-Type", "application/json");
    options.body = JSON.stringify(options.json);
  }
  const response = await fetch(path, {...options, headers});
  const contentType = response.headers.get("Content-Type") || "";
  const payload = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    const error = new Error(payload.message || `请求失败（${response.status}）`);
    error.code = payload.code;
    error.field = payload.field;
    throw error;
  }
  return payload;
}

function notify(message, isError = false) {
  const notice = $("#notice");
  notice.textContent = message;
  notice.classList.toggle("error", isError);
  notice.hidden = false;
  window.setTimeout(() => { if (notice.textContent === message) notice.hidden = true; }, 6000);
}

async function refreshCases(keepSelection = true) {
  try {
    const result = await api("/api/cases");
    state.cases = result.items;
    $("#serviceState").textContent = "服务正常 · 数据已同步";
    renderQueue();
    if (keepSelection && state.selected) {
      const updated = state.cases.find((item) => item.id === state.selected.id);
      if (updated) selectCase(updated.id, false);
    }
  } catch (error) {
    $("#serviceState").textContent = "服务连接异常";
    if (state.selected) notify(error.message, true);
  }
}

function renderQueue() {
  const filter = $("#caseFilter").value.trim().toLowerCase();
  const visible = state.cases.filter((item) => `${item.workpieceCode} ${item.inspectionZone} ${statusLabels[item.status]}`.toLowerCase().includes(filter));
  $("#caseCount").textContent = `${visible.length} 项`;
  $("#caseList").innerHTML = visible.length ? visible.map((item) => `
    <button class="case-item ${state.selected?.id === item.id ? "active" : ""}" data-case-id="${escapeHTML(item.id)}">
      <strong>${escapeHTML(item.workpieceCode)}</strong>
      <span>${escapeHTML(item.inspectionZone)}</span>
      <span class="item-foot"><b>${escapeHTML(statusLabels[item.status] || item.status)}</b><i>v${item.version}</i></span>
    </button>`).join("") : `<div class="empty-list">没有匹配的检测任务</div>`;
  document.querySelectorAll("[data-case-id]").forEach((button) => button.addEventListener("click", () => selectCase(button.dataset.caseId)));
}

async function selectCase(id, fetchLatest = true) {
  try {
    const item = fetchLatest ? await api(`/api/cases/${encodeURIComponent(id)}`) : state.cases.find((candidate) => candidate.id === id);
    if (!item) return;
    state.selected = item;
    state.audit = [];
    $("#emptyState").hidden = true;
    $("#caseDetail").hidden = false;
    renderQueue();
    renderDetail();
  } catch (error) { notify(error.message, true); }
}

function renderDetail() {
  const item = state.selected;
  $("#caseTitle").textContent = item.workpieceCode;
  $("#caseStatus").textContent = statusLabels[item.status] || item.status;
  $("#caseMeta").textContent = `任务 ${item.id} · 版本 ${item.version} · 更新 ${formatTime(item.updatedAt)}`;
  $("#caseFacts").innerHTML = [
    ["检测区域", item.inspectionZone],
    ["射线工艺", `${item.techniqueParameters.sourceType} / ${item.techniqueParameters.voltageKV} kV`],
    ["验收规则", `${item.acceptanceRuleSet.id} / v${item.acceptanceRuleSet.version}`],
    ["候选底片", `${activeRevisions(item).length} 张（历史 ${item.revisions.length}）`]
  ].map(([label, value]) => `<div class="fact"><span>${escapeHTML(label)}</span><strong>${escapeHTML(value)}</strong></div>`).join("");
  renderActions(); renderRevisions(); renderFindings(); renderRules(); renderEvidence();
  if (state.activeTab === "audit") loadAudit();
}

function activeRevisions(item) {
  const replaced = new Set(item.revisions.filter((rev) => rev.supersedesRevisionId).map((rev) => rev.supersedesRevisionId));
  return item.revisions.filter((rev) => !replaced.has(rev.id));
}

function renderActions() {
  const item = state.selected;
  const actions = [];
  if (role() === "operator" && ["draft","pending_check","retake","reinspection"].includes(item.status)) actions.push(["上传底片", "upload", "primary"]);
  if (role() === "reviewer" && ["pending_check","reinspection"].includes(item.status)) actions.push(["执行完整性检查", "check", "primary"]);
  if (role() === "reviewer" && item.status === "pending_review") {
    actions.push(["标注缺陷", "finding", "secondary"], ["填写规则结论", "conclusions", "secondary"]);
    if (item.findings.some((finding) => finding.status === "open" && finding.severity === "blocking")) actions.push(["退回返拍", "retake", "danger"]);
    actions.push(["冻结证据", "freeze", "primary"]);
  }
  if (role() === "quality" && item.status === "frozen") actions.push(["签发放行凭据", "issue", "primary"]);
  $("#actionBar").innerHTML = actions.map(([label, action, style]) => `<button class="${style}" data-action="${action}">${label}</button>`).join("");
  document.querySelectorAll("[data-action]").forEach((button) => button.addEventListener("click", () => handleAction(button.dataset.action)));
}

function renderRevisions() {
  const item = state.selected;
  const coverage = item.acceptanceRuleSet.rules.flatMap((rule) => rule.requiredViews.map((view) => { const revision = activeRevisions(item).find((candidate) => candidate.viewCode === view && (!rule.requiredZones.length || rule.requiredZones.includes(candidate.coveredZone))); return `<span class="coverage ${revision ? "covered" : "missing"}">${escapeHTML(view)} · ${escapeHTML(rule.requiredZones[0] || item.inspectionZone)} · ${revision ? `修订 ${revision.revisionNumber}` : "缺失"}</span>`; })).join("");
  $("#revisionList").innerHTML = `<div class="coverage-list">${coverage || "暂无规则覆盖项"}</div>` + (item.revisions.length ? item.revisions.map((rev) => `
    <div class="data-row">
      <div><strong>修订 ${rev.revisionNumber} · ${escapeHTML(rev.viewCode)}</strong><span>${escapeHTML(rev.captureBatch)} / ${escapeHTML(rev.coveredZone)}</span></div>
      <div><strong>${rev.exposureParameters.voltageKV} kV · ${rev.exposureParameters.exposureSeconds} s</strong><small>${rev.exposureParameters.currentMA} mA / 距离 ${rev.exposureParameters.sourceDistanceMM} mm</small></div>
      <div><strong>${formatBytes(rev.sizeBytes)}</strong><small title="${escapeHTML(rev.contentDigest)}">SHA-256 ${escapeHTML(rev.contentDigest.slice(0, 16))}…</small></div>
      <div><a href="/api/cases/${encodeURIComponent(item.id)}/revisions/${encodeURIComponent(rev.id)}/content">下载</a>${rev.supersedesRevisionId ? `<small>替代 ${escapeHTML(rev.supersedesRevisionId)}</small>` : ""}</div>
    </div>`).join("") : `<div class="empty-list">尚未提交底片修订</div>`);
}

function renderFindings() {
  const findings = state.selected.findings;
  const open = findings.filter((item) => item.status === "open");
  const batch = state.selected.checkBatches?.at(-1);
  $("#findingSummary").innerHTML = `<strong>检查序次 ${state.selected.checkSequence}</strong><span>${state.selected.lastCheckPassed ? "完整性检查通过" : "尚未通过检查"}</span><span>未关闭 ${open.length} 项</span><span>阻断 ${open.filter((item) => item.severity === "blocking").length} 项</span>${batch ? `<span>新增 ${batch.differences.filter((item) => item.state === "新增").length} · 持续 ${batch.differences.filter((item) => item.state === "持续").length} · 已解决 ${batch.differences.filter((item) => item.state === "已解决").length}</span>` : ""}`;
  $("#findingList").innerHTML = findings.length ? findings.map((finding) => `
    <div class="data-row">
      <div><strong>${escapeHTML(finding.findingType)}</strong><span>${escapeHTML(finding.location || "全局")}</span></div>
      <div><span class="severity ${escapeHTML(finding.severity)}">${escapeHTML(finding.severity)}</span><small>规则 ${escapeHTML(finding.ruleReference)} · ${finding.measuredSize || 0} mm</small></div>
      <div><strong>${escapeHTML(finding.basis)}</strong><span>${escapeHTML(finding.disposition)}</span>${finding.closureNote ? `<small>关闭：${escapeHTML(finding.closureNote)}</small>` : ""}</div>
      <div>${finding.status === "open" && role() === "reviewer" ? `<button class="secondary" data-close-id="${escapeHTML(finding.id)}">关闭问题</button>` : `<span>${finding.status === "closed" ? "已关闭" : "未关闭"}</span>`}</div>
    </div>`).join("") : `<div class="empty-list">尚无检查问题或人工缺陷</div>`;
  document.querySelectorAll("[data-close-id]").forEach((button) => button.addEventListener("click", () => openCloseFinding(button.dataset.closeId)));
}

function renderRules() {
  const conclusions = new Map(state.selected.ruleConclusions.map((item) => [item.ruleId, item]));
  $("#ruleList").innerHTML = state.selected.acceptanceRuleSet.rules.map((rule) => {
    const result = conclusions.get(rule.id);
    return `<div class="data-row"><div><strong>${escapeHTML(rule.name)}</strong><span>${escapeHTML(rule.id)}</span></div><div><strong>视图 ${escapeHTML(rule.requiredViews.join("、") || "无")}</strong><span>区域 ${escapeHTML(rule.requiredZones.join("、") || "无")}</span></div><div><strong>电压 ${rule.minVoltageKV || 0}–${rule.maxVoltageKV || "不限"} kV</strong><span>最大缺陷 ${rule.maxDefectSizeMM || "未限定"} mm</span></div><div>${result ? `<strong>${result.conclusion === "pass" ? "通过" : "不通过"}</strong><small>${escapeHTML(result.basis)}</small>` : "待判定"}</div></div>`;
  }).join("");
}

function renderEvidence() {
  const item = state.selected;
  if (!item.frozen) { $("#evidenceView").innerHTML = `<div class="empty-list">任务尚未冻结证据</div>`; return; }
  $("#evidenceView").innerHTML = `<div class="evidence-block"><h3>冻结快照</h3><p>冻结版本：${item.frozen.frozenVersion} · 冻结人：${escapeHTML(item.frozen.frozenBy)} · ${formatTime(item.frozen.frozenAt)}</p><p>规则：${escapeHTML(item.frozen.ruleSetId)} / v${item.frozen.ruleSetVersion}</p><p>证据摘要：<code>${escapeHTML(item.frozen.evidenceDigest)}</code></p><p>底片 ${item.frozen.revisions.length} 张 · 发现 ${item.frozen.findings.length} 项</p></div>${item.credential ? `<div class="evidence-block"><h3>放行凭据</h3><p>编号：<strong>${escapeHTML(item.credential.credentialNumber)}</strong></p><p>签发人：${escapeHTML(item.credential.issuer)} · ${formatTime(item.credential.issuedAt)}</p><p>校验摘要：<code>${escapeHTML(item.credential.verificationDigest)}</code></p><button class="secondary" id="verifyCurrent">验证此凭据</button></div>` : `<div class="empty-list">冻结证据尚未签发凭据</div>`}`;
  $("#verifyCurrent")?.addEventListener("click", () => verifyCredential(item.credential.credentialNumber));
}

async function loadAudit() {
  if (!state.selected) return;
  try {
    const result = await api(`/api/cases/${encodeURIComponent(state.selected.id)}/audit`);
    state.audit = result.items;
    $("#auditList").innerHTML = state.audit.length ? state.audit.map((event) => `<div class="data-row"><div><strong>${escapeHTML(event.action)}</strong><span>版本 ${event.version}</span></div><div><strong>${escapeHTML(event.actor)}</strong><small>${escapeHTML(event.role)}</small></div><div><span>${formatTime(event.at)}</span></div><div><small>${escapeHTML(JSON.stringify(event.details || {}))}</small></div></div>`).join("") : `<div class="empty-list">暂无审计事件</div>`;
  } catch (error) { notify(error.message, true); }
}

function openDialog(title, body, submit) {
  $("#dialogTitle").textContent = title;
  $("#dialogBody").innerHTML = body;
  $("#dialogError").hidden = true;
  const dialog = $("#formDialog");
  const handler = async (event) => {
    if (event.target.value !== "default") return;
    event.preventDefault();
    try { showBusy(true); await submit(new FormData(dialog.querySelector("form"))); dialog.close(); await refreshCases(); }
    catch (error) { $("#dialogError").textContent = error.message; $("#dialogError").hidden = false; }
    finally { showBusy(false); }
  };
  $("#dialogSubmit").onclick = handler;
  dialog.showModal();
}

function field(label, name, type = "text", value = "", options = "", wide = false) {
  if (type === "select") return `<label class="${wide ? "wide" : ""}">${label}<select name="${name}" required>${options}</select></label>`;
  if (type === "textarea") return `<label class="${wide ? "wide" : ""}">${label}<textarea name="${name}" required>${escapeHTML(value)}</textarea></label>`;
  return `<label class="${wide ? "wide" : ""}">${label}<input name="${name}" type="${type}" value="${escapeHTML(value)}" ${type === "number" ? 'step="any"' : ""} required></label>`;
}

function handleAction(action) {
  if (action === "upload") return openUpload();
  if (action === "finding") return openFinding();
  if (action === "conclusions") return openConclusions();
  if (action === "retake") return openRetake();
  if (action === "check") return simpleCommand("执行完整性检查", "run-check", `/api/cases/${state.selected.id}/checks`, {});
  if (action === "freeze") return simpleCommand("冻结当前证据集合", "freeze", `/api/cases/${state.selected.id}/freeze`, {});
  if (action === "issue") return simpleCommand("签发不可变放行凭据", "issue", `/api/cases/${state.selected.id}/release`, {frozenVersion: state.selected.frozen.frozenVersion});
}

function simpleCommand(title, operation, path, extra) {
  openDialog(title, `<p class="wide">确认对任务 <strong>${escapeHTML(state.selected.workpieceCode)}</strong> 执行此操作。系统将记录当前操作人与任务版本。</p>`, async () => {
    await api(path, {method:"POST", json:{expectedVersion:state.selected.version,idempotencyKey:key(operation),...extra}});
  });
}

function openNewCase() {
  const row = (id = "R-1") => `<div class="rule-row"><input name="ruleId" required placeholder="规则标识" value="${id}"><input name="ruleName" required placeholder="规则名称" value="底片覆盖与缺陷验收"><input name="requiredViews" required placeholder="必需视图" value="FRONT"><input name="requiredZones" required placeholder="必需区域" value="WELD-A"><input name="minVoltageKV" required type="number" value="100"><input name="maxVoltageKV" required type="number" value="300"><input name="maxDefectSizeMM" required type="number" step="any" value="2"><input name="blockingLevels" required value="blocking"><button type="button" class="icon-button remove-rule" title="删除规则" aria-label="删除规则">×</button></div>`;
  const body = field("工件标识","workpieceCode") + field("检测区域","inspectionZone") + field("射线源类型","sourceType","text","X-ray") + field("管电压 kV","voltageKV","number","180") + field("管电流 mA","currentMA","number","5") + field("曝光时间 s","exposureSeconds","number","2") + field("焦距 mm","sourceDistanceMM","number","600") + field("规则集标识","ruleSetId","text","RT-BASE") + `<label class="wide">验收规则（可增删）<div id="ruleEditor">${row()}</div><button type="button" id="addRule" class="secondary">＋ 添加规则</button></label>`;
  openDialog("新建检测任务", body, async (data) => {
    const split = (name) => String(data.get(name)).split(",").map((v) => v.trim()).filter(Boolean);
    const ids = data.getAll("ruleId"), names = data.getAll("ruleName"), views = data.getAll("requiredViews"), zones = data.getAll("requiredZones"), mins = data.getAll("minVoltageKV"), maxs = data.getAll("maxVoltageKV"), sizes = data.getAll("maxDefectSizeMM"), levels = data.getAll("blockingLevels");
    const rules = ids.map((id, index) => ({id, name:names[index], requiredViews:split(views[index]), requiredZones:split(zones[index]), minVoltageKV:+mins[index], maxVoltageKV:+maxs[index], maxDefectSizeMM:+sizes[index], blockingLevels:split(levels[index])}));
    const payload = {idempotencyKey:key("create-case"),workpieceCode:data.get("workpieceCode"),inspectionZone:data.get("inspectionZone"),techniqueParameters:{sourceType:data.get("sourceType"),voltageKV:+data.get("voltageKV"),currentMA:+data.get("currentMA"),exposureSeconds:+data.get("exposureSeconds"),sourceDistanceMM:+data.get("sourceDistanceMM")},acceptanceRuleSet:{id:data.get("ruleSetId"),version:1,rules}};
    const created = await api("/api/cases", {method:"POST",json:payload}); state.selected = created;
  });
  document.getElementById("addRule")?.addEventListener("click", () => { const count = document.querySelectorAll("#ruleEditor .rule-row").length + 1; document.getElementById("ruleEditor").insertAdjacentHTML("beforeend", row(`R-${count}`)); bindRuleButtons(); });
  function bindRuleButtons() { document.querySelectorAll(".remove-rule").forEach((button) => { button.onclick = () => { const rows = document.querySelectorAll("#ruleEditor .rule-row"); if (rows.length > 1) button.closest(".rule-row").remove(); }; }); }
  bindRuleButtons();
}

function openUpload() {
  const revisions = state.selected.revisions.map((rev) => `<option value="${escapeHTML(rev.id)}">修订 ${rev.revisionNumber} · ${escapeHTML(rev.viewCode)}</option>`).join("");
  const body = field("底片文件","file","file") + field("拍摄批次","captureBatch") + field("视图代码","viewCode","text",state.selected.acceptanceRuleSet.rules[0]?.requiredViews[0] || "FRONT") + field("覆盖区域","coveredZone","text",state.selected.inspectionZone) + field("管电压 kV","voltageKV","number",String(state.selected.techniqueParameters.voltageKV)) + field("管电流 mA","currentMA","number",String(state.selected.techniqueParameters.currentMA)) + field("曝光时间 s","exposureSeconds","number",String(state.selected.techniqueParameters.exposureSeconds)) + field("焦距 mm","sourceDistanceMM","number",String(state.selected.techniqueParameters.sourceDistanceMM)) + `<label class="wide">替代修订（可选）<select name="supersedesRevisionId"><option value="">不替代历史修订</option>${revisions}</select></label>`;
  openDialog("提交不可变底片修订", body, async (data) => {
    const file = data.get("file"); if (!(file instanceof File) || !file.size) throw new Error("请选择非空底片文件");
    const digest = [...new Uint8Array(await crypto.subtle.digest("SHA-256", await file.arrayBuffer()))].map((b) => b.toString(16).padStart(2,"0")).join("");
    const metadata = {expectedVersion:state.selected.version,idempotencyKey:key("submit-revision"),captureBatch:data.get("captureBatch"),viewCode:data.get("viewCode"),coveredZone:data.get("coveredZone"),exposureParameters:{voltageKV:+data.get("voltageKV"),currentMA:+data.get("currentMA"),exposureSeconds:+data.get("exposureSeconds"),sourceDistanceMM:+data.get("sourceDistanceMM")},contentDigest:digest,supersedesRevisionId:data.get("supersedesRevisionId")};
    const form = new FormData(); form.set("metadata",JSON.stringify(metadata)); form.set("file",file); await api(`/api/cases/${state.selected.id}/revisions`,{method:"POST",body:form});
  });
}

function openFinding() {
  const revisions = activeRevisions(state.selected).map((rev) => `<option value="${escapeHTML(rev.id)}">修订 ${rev.revisionNumber} · ${escapeHTML(rev.viewCode)}</option>`).join("");
  const rules = state.selected.acceptanceRuleSet.rules.map((rule) => `<option value="${escapeHTML(rule.id)}">${escapeHTML(rule.name)}</option>`).join("");
  const row = () => `<div class="finding-row"><select name="revisionId" required>${revisions}</select><select name="ruleReference" required>${rules}</select><input name="findingType" required placeholder="缺陷类型"><input name="location" required placeholder="位置"><input name="measuredSize" required type="number" min="0" step="any" value="0"><select name="severity"><option value="info">提示</option><option value="warning">警告</option><option value="blocking">阻断</option></select><input name="basis" required placeholder="判读依据"><input name="disposition" required placeholder="处置意见"><button type="button" class="icon-button remove-finding" title="删除缺陷" aria-label="删除缺陷">×</button></div>`;
  const body = `<label class="wide">批量缺陷（可增删）<div id="findingEditor">${row()}</div><button type="button" id="addFinding" class="secondary">＋ 添加缺陷</button></label>`;
  openDialog("批量标注人工判读缺陷", body, async (data) => { const values = (name) => data.getAll(name); const findings = values("revisionId").map((_, index) => ({revisionId:values("revisionId")[index],ruleReference:values("ruleReference")[index],findingType:values("findingType")[index],location:values("location")[index],measuredSize:+values("measuredSize")[index],severity:values("severity")[index],basis:values("basis")[index],disposition:values("disposition")[index]})); await api(`/api/cases/${state.selected.id}/findings`,{method:"POST",json:{expectedVersion:state.selected.version,idempotencyKey:key("finding"),findings}}); });
  document.getElementById("addFinding")?.addEventListener("click", () => { document.getElementById("findingEditor").insertAdjacentHTML("beforeend", row()); bindFindingButtons(); });
  function bindFindingButtons() { document.querySelectorAll(".remove-finding").forEach((button) => { button.onclick = () => { const rows = document.querySelectorAll("#findingEditor .finding-row"); if (rows.length > 1) button.closest(".finding-row").remove(); }; }); }
  bindFindingButtons();
}

function openConclusions() {
  const current = new Map(state.selected.ruleConclusions.map((item) => [item.ruleId,item]));
  const body = state.selected.acceptanceRuleSet.rules.map((rule,index) => { const saved=current.get(rule.id); return `<label>${escapeHTML(rule.name)}<select name="result-${index}"><option value="pass" ${saved?.conclusion === "pass" ? "selected" : ""}>通过</option><option value="fail" ${saved?.conclusion === "fail" ? "selected" : ""}>不通过</option></select></label>${field("结论依据",`basis-${index}`,"textarea",saved?.basis || "符合验收规则","",true)}`; }).join("");
  openDialog("填写全部规则结论", body, async (data) => { const conclusions=state.selected.acceptanceRuleSet.rules.map((rule,index)=>({ruleId:rule.id,conclusion:data.get(`result-${index}`),basis:data.get(`basis-${index}`)})); await api(`/api/cases/${state.selected.id}/conclusions`,{method:"POST",json:{expectedVersion:state.selected.version,idempotencyKey:key("conclusions"),conclusions}}); });
}

function openRetake() {
  const blockers = state.selected.findings.filter((finding) => finding.status === "open" && finding.severity === "blocking");
  const choices = blockers.map((finding) => `<label class="wide"><input type="checkbox" name="findingIds" value="${escapeHTML(finding.id)}" checked> ${escapeHTML(finding.findingType)} · ${escapeHTML(finding.location || "全局")}</label>`).join("");
  openDialog("退回返拍", `<fieldset class="wide"><legend>选择返拍问题</legend>${choices || "没有可返拍的阻断问题"}</fieldset>` + field("返拍要求","requirement","textarea","请针对所选阻断问题重新拍摄并提交替代修订。","",true), async (data) => { const ids = data.getAll("findingIds"); if (!ids.length) throw new Error("至少选择一项未关闭阻断问题"); await api(`/api/cases/${state.selected.id}/retake`,{method:"POST",json:{expectedVersion:state.selected.version,idempotencyKey:key("retake"),requirement:data.get("requirement"),items:ids.map((findingId) => ({findingId}))}}); });
}

function openCloseFinding(findingId) {
  openDialog("关闭判读问题", field("关闭说明","closureNote","textarea","替代修订已复验，问题不再出现。","",true), async (data) => api(`/api/cases/${state.selected.id}/findings/${findingId}/close`,{method:"POST",json:{expectedVersion:state.selected.version,idempotencyKey:key("close-finding"),closureNote:data.get("closureNote")}}));
}

async function verifyCredential(number) {
  const result = $("#credentialResult");
  try { const verification=await api(`/api/credentials/${encodeURIComponent(number)}`); result.innerHTML=`<strong>${verification.valid ? "校验通过" : "校验失败"}</strong><br>${escapeHTML(verification.message)}<br>${escapeHTML(verification.credential.issuer)} · ${formatTime(verification.credential.issuedAt)}<div>${(verification.evidence || []).map((item) => `<span class="evidence-item ${item.valid ? "ok" : "bad"}">${escapeHTML(item.type)} · ${escapeHTML(item.message)} · ${escapeHTML(item.reference)}</span>`).join("")}</div>`; }
  catch (error) { result.textContent=error.message; }
}

function formatTime(value) { return value ? new Intl.DateTimeFormat("zh-CN",{dateStyle:"medium",timeStyle:"short"}).format(new Date(value)) : "—"; }
function formatBytes(value) { if (value < 1024) return `${value} B`; if (value < 1048576) return `${(value/1024).toFixed(1)} KiB`; return `${(value/1048576).toFixed(1)} MiB`; }

$("#newCaseButton").addEventListener("click", openNewCase);
$("#refreshButton").addEventListener("click", () => refreshCases());
$("#caseFilter").addEventListener("input", renderQueue);
$("#role").addEventListener("change", () => { localStorage.setItem("rt-role",role()); if (state.selected) renderDetail(); });
$("#actor").addEventListener("change", () => localStorage.setItem("rt-actor",actor()));
$("#credentialForm").addEventListener("submit", (event) => { event.preventDefault(); verifyCredential($("#credentialNumber").value.trim()); });
document.querySelectorAll(".tab").forEach((tab) => tab.addEventListener("click", () => { state.activeTab=tab.dataset.tab; document.querySelectorAll(".tab").forEach((item)=>item.classList.toggle("active",item===tab)); document.querySelectorAll(".tab-panel").forEach((panel)=>panel.hidden=panel.id!==`tab-${state.activeTab}`); if(state.activeTab==="audit")loadAudit(); }));
$("#role").value = localStorage.getItem("rt-role") || "operator";
$("#actor").value = localStorage.getItem("rt-actor") || "张工";
refreshCases(false);
