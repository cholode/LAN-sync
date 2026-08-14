<template>
  <div v-if="state.agent.visible" class="aside-block agent-panel">
    <div class="agent-panel-head">
      <div>
        <h3>群 Agent 助手</h3>
        <span class="agent-status" :class="state.agent.enabled ? 'on' : 'off'">
          {{ state.agent.enabled ? '已启用' : '未启用' }}
        </span>
      </div>
      <button
        type="button"
        class="icon-btn"
        title="配置 Agent 助手"
        @click="openAgentConfig"
      >
        ⚙
      </button>
    </div>

    <label class="agent-toggle-line">
      <input
        type="checkbox"
        :checked="state.agent.enabled"
        @change="toggleAgent"
      />
      <span>启用 AI 助手</span>
    </label>
  </div>

  <Teleport to="body">
    <div
      v-if="state.agent.configVisible"
      class="modal-backdrop"
      @click.self="closeAgentConfig"
    >
      <div class="agent-config-modal">
        <div class="modal-head">
          <h3>Agent 助手配置</h3>
          <button type="button" class="icon-btn" title="关闭" @click="closeAgentConfig">
            ×
          </button>
        </div>

        <div class="agent-config show">
          <label>触发模式</label>
          <select v-model="state.agent.config.trigger_mode" @change="markAgentDirty">
            <option value="1">@提及 / @agent</option>
            <option value="2">全部消息</option>
            <option value="3">关键词</option>
          </select>

          <label>触发关键词（JSON 数组）</label>
          <input
            v-model="state.agent.config.trigger_words"
            placeholder='["hello","你好"]'
            @input="markAgentDirty"
          />

          <label>系统提示词</label>
          <textarea
            v-model="state.agent.config.system_prompt"
            placeholder="你是本群的 AI 助手..."
            @input="markAgentDirty"
          ></textarea>

          <label>上下文消息条数</label>
          <input
            v-model="state.agent.config.max_history"
            type="number"
            min="1"
            max="100"
            @input="markAgentDirty"
          />

          <label>LLM 模型</label>
          <input v-model="state.agent.config.model_name" @input="markAgentDirty" />

          <label>Top-K 检索</label>
          <input
            v-model="state.agent.config.top_k"
            type="number"
            min="1"
            max="20"
            @input="markAgentDirty"
          />

          <div class="btn-row">
            <button type="button" class="primary" :disabled="!state.agent.dirty" @click="saveAgentConfig">
              保存配置
            </button>
            <button
              v-if="state.agent.enabled"
              type="button"
              class="primary danger"
              @click="removeAgent"
            >
              移除 Agent
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { state } from '../store/index.js';
import {
  toggleAgent,
  saveAgentConfig,
  removeAgent,
  markAgentDirty,
} from '../composables/useAgent.js';

function openAgentConfig() {
  state.agent.configVisible = true;
}

function closeAgentConfig() {
  state.agent.configVisible = false;
}
</script>
