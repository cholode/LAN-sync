import { state } from '../store/index.js';
import { request } from '../api/api.js';
import { notifyLine } from '../utils/systemMessage.js';

export async function toggleAgent() {
  if (!state.currentRoomId) return;
  if (state.agent.enabled) {
    await disableAgent();
  } else {
    await enableAgent();
  }
}

async function enableAgent() {
  try {
    const res = await request('/rooms/' + state.currentRoomId + '/agent/enable', {
      method: 'POST',
    });
    if (res.ok) {
      state.agent.enabled = true;
      notifyLine('Agent 已启用', 'sys');
    } else {
      alert('启用失败');
    }
  } catch (e) {
    console.error('enableAgent', e);
  }
}

async function disableAgent() {
  try {
    const res = await request('/rooms/' + state.currentRoomId + '/agent/disable', {
      method: 'POST',
    });
    if (res.ok) {
      state.agent.enabled = false;
      notifyLine('Agent 已暂停', 'sys');
    } else {
      alert('暂停失败');
    }
  } catch (e) {
    console.error('disableAgent', e);
  }
}

export function markAgentDirty() {
  state.agent.dirty = true;
}

export async function saveAgentConfig() {
  if (!state.agent.dirty || !state.currentRoomId) return;
  const cfg = state.agent.config;
  const body = {
    trigger_mode: Number(cfg.trigger_mode),
    trigger_words: cfg.trigger_words,
    system_prompt: cfg.system_prompt,
    max_history: Number(cfg.max_history) || 20,
    model_name: cfg.model_name,
    top_k: Number(cfg.top_k) || 5,
  };

  try {
    const res = await request('/rooms/' + state.currentRoomId + '/agent/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (res.ok) {
      state.agent.dirty = false;
      notifyLine('Agent 配置已保存', 'sys');
    } else {
      const data = await res.json().catch(() => ({}));
      alert(data.error || '保存失败');
    }
  } catch (e) {
    alert('网络异常');
  }
}

export async function removeAgent() {
  if (!state.currentRoomId) return;
  if (!confirm('确定移除 Agent？将清空该群全部 AI 数据（向量库+分块），不可恢复。')) {
    return;
  }

  try {
    const res = await request('/rooms/' + state.currentRoomId + '/agent', {
      method: 'DELETE',
    });
    if (res.ok) {
      state.agent.enabled = false;
      notifyLine('Agent 已移除（含数据清理）', 'sys');
    } else {
      const data = await res.json().catch(() => ({}));
      alert(data.error || '移除失败');
    }
  } catch (e) {
    alert('网络异常');
  }
}

export async function loadAgentState() {
  if (!state.currentRoomId) {
    state.agent.visible = false;
    return;
  }

  state.agent.visible = true;
  try {
    const res = await request('/rooms/' + state.currentRoomId + '/agent/config');
    if (!res.ok) return;

    const data = await res.json();
    const cfg = data.config || {};
    state.agent.enabled = cfg.room_id === state.currentRoomId;
    state.agent.config = {
      trigger_mode: cfg.trigger_mode || 1,
      trigger_words: cfg.trigger_words || '[]',
      system_prompt: cfg.system_prompt || '',
      max_history: cfg.max_history || 20,
      model_name: cfg.model_name || 'deepseek-chat',
      top_k: cfg.top_k || 5,
    };
    state.agent.dirty = false;
  } catch (e) {
    console.error('loadAgentState', e);
  }
}
