// src/modules/agent.js —— Agent 管理：启停、配置、查询
import { state } from '../store/index.js';
import { request } from '../api/api.js';
import { notifyLine } from '../utils/ui.js';

let agentConfigDirty = false;
let agentEnabled = false;

export function toggleAgent() {
  const cb = document.getElementById("agent-enabled-cb");
  if (!cb) return;
  cb.checked ? enableAgent() : disableAgent();
}

async function enableAgent() {
  if (!state.currentRoomId) return;
  try {
    const res = await request("/rooms/" + state.currentRoomId + "/agent/enable", { method: "POST" });
    if (res.ok) {
      agentEnabled = true;
      updateAgentUI(true);
      notifyLine("Agent 已启用", "sys");
    } else {
      document.getElementById("agent-enabled-cb").checked = false;
      alert("启用失败");
    }
  } catch (e) {
    document.getElementById("agent-enabled-cb").checked = false;
  }
}

async function disableAgent() {
  if (!state.currentRoomId) return;
  try {
    const res = await request("/rooms/" + state.currentRoomId + "/agent/disable", { method: "POST" });
    if (res.ok) {
      agentEnabled = false;
      updateAgentUI(false);
      notifyLine("Agent 已暂停", "sys");
    } else {
      document.getElementById("agent-enabled-cb").checked = true;
      alert("暂停失败");
    }
  } catch (e) {
    document.getElementById("agent-enabled-cb").checked = true;
  }
}

export async function removeAgent() {
  if (!state.currentRoomId) return;
  if (!confirm("确定移除 Agent？将清空该群全部 AI 数据（向量库+分块），不可恢复。")) return;
  try {
    const res = await request("/rooms/" + state.currentRoomId + "/agent", { method: "DELETE" });
    if (res.ok) {
      agentEnabled = false;
      updateAgentUI(false);
      document.getElementById("agent-enabled-cb").checked = false;
      notifyLine("Agent 已移除（含数据清理）", "sys");
    } else {
      const data = await res.json().catch(() => ({}));
      alert(data.error || "移除失败");
    }
  } catch (e) {
    alert("网络异常");
  }
}

export function markAgentDirty() {
  agentConfigDirty = true;
  document.getElementById("btn-save-agent").disabled = false;
}

export async function saveAgentConfig() {
  if (!agentConfigDirty || !state.currentRoomId) return;
  const body = {
    trigger_mode: parseInt(document.getElementById("agent-trigger-mode").value),
    trigger_words: document.getElementById("agent-trigger-words").value,
    system_prompt: document.getElementById("agent-system-prompt").value,
    max_history: parseInt(document.getElementById("agent-max-history").value) || 20,
    model_name: document.getElementById("agent-model").value,
    top_k: parseInt(document.getElementById("agent-topk").value) || 5
  };
  try {
    const res = await request("/rooms/" + state.currentRoomId + "/agent/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
    if (res.ok) {
      agentConfigDirty = false;
      document.getElementById("btn-save-agent").disabled = true;
      notifyLine("Agent 配置已保存", "sys");
    } else {
      const data = await res.json().catch(() => ({}));
      alert(data.error || "保存失败");
    }
  } catch (e) {
    alert("网络异常");
  }
}

export async function loadAgentState() {
  if (!state.currentRoomId) {
    const panel = document.getElementById("agent-panel");
    if (panel) panel.style.display = "none";
    return;
  }
  const panel = document.getElementById("agent-panel");
  if (panel) panel.style.display = "block";
  try {
    const res = await request("/rooms/" + state.currentRoomId + "/agent/config");
    if (!res.ok) return;
    const data = await res.json();
    const cfg = data.config || {};
    agentEnabled = cfg.room_id === state.currentRoomId;

    document.getElementById("agent-enabled-cb").checked = agentEnabled;
    updateAgentUI(agentEnabled);

    document.getElementById("agent-trigger-mode").value = cfg.trigger_mode || 1;
    document.getElementById("agent-trigger-words").value = cfg.trigger_words || "[]";
    document.getElementById("agent-system-prompt").value = cfg.system_prompt || "";
    document.getElementById("agent-max-history").value = cfg.max_history || 20;
    document.getElementById("agent-model").value = cfg.model_name || "deepseek-chat";
    document.getElementById("agent-topk").value = cfg.top_k || 5;
    agentConfigDirty = false;
    document.getElementById("btn-save-agent").disabled = true;
  } catch (e) {
    console.error("loadAgentState", e);
  }
}

function updateAgentUI(enabled) {
  document.getElementById("agent-status-label").textContent = enabled ? "已启用" : "未启用";
  document.getElementById("agent-status-label").className = "agent-status " + (enabled ? "on" : "off");
  document.getElementById("agent-config").className = "agent-config" + (enabled ? " show" : "");
  document.getElementById("btn-remove-agent").style.display = enabled ? "" : "none";
}