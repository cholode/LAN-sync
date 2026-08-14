<template>
  <section v-if="state.messageSearch.visible" class="message-search-panel">
    <div class="message-search-bar">
      <input
        v-model="state.messageSearch.query"
        type="search"
        :placeholder="state.currentRoomId ? '搜索当前群消息' : '请先选择群聊'"
        :disabled="!state.currentRoomId"
        @keydown.enter.prevent="runSearch"
      />
      <button
        type="button"
        class="primary"
        :disabled="!state.currentRoomId || state.messageSearch.loading"
        @click="runSearch"
      >
        {{ state.messageSearch.loading ? '搜索中...' : '搜索' }}
      </button>
      <button type="button" class="btn-ghost" @click="closePanel">关闭</button>
    </div>

    <div v-if="state.messageSearch.error" class="message-search-error">
      {{ state.messageSearch.error }}
    </div>

    <div v-if="!state.messageSearch.loading && state.messageSearch.query" class="message-search-meta">
      共 {{ state.messageSearch.total }} 条结果
    </div>

    <div class="message-search-results">
      <div v-if="state.messageSearch.results.length === 0 && !state.messageSearch.loading && state.messageSearch.query" class="message-search-empty">
        未找到匹配消息
      </div>

      <div
        v-for="(message, index) in state.messageSearch.results"
        :key="message.id || index"
        class="search-hit"
      >
        <div class="search-hit-meta">
          UID {{ message.sender_id }}
          <template v-if="formatMessageTime(message.created_at)">
            · {{ formatMessageTime(message.created_at) }}
          </template>
        </div>
        <div class="search-hit-content" v-html="formatHitContent(message)"></div>
      </div>
    </div>

    <div v-if="state.messageSearch.results.length < state.messageSearch.total" class="message-search-more">
      <button type="button" class="btn-ghost" @click="loadMore">加载更多</button>
    </div>
  </section>
</template>

<script setup>
import { watch } from 'vue';
import { state } from '../store/index.js';
import {
  resetMessageSearch,
  searchRoomMessages,
} from '../composables/useSearch.js';
import {
  escapeHtml,
  formatMessageTime,
} from '../utils/message.js';

watch(
  () => state.currentRoomId,
  () => resetMessageSearch(),
);

function closePanel() {
  resetMessageSearch();
}

async function runSearch() {
  const keyword = String(state.messageSearch.query || '').trim();
  if (!keyword) return;

  state.messageSearch.loading = true;
  state.messageSearch.error = '';
  state.messageSearch.results = [];
  state.messageSearch.total = 0;
  state.messageSearch.from = 0;

  try {
    const data = await searchRoomMessages(keyword, 0, 20);
    state.messageSearch.results = data.messages;
    state.messageSearch.total = data.total;
    state.messageSearch.from = data.messages.length;
  } catch (error) {
    state.messageSearch.error = error.message || '搜索失败';
  } finally {
    state.messageSearch.loading = false;
  }
}

async function loadMore() {
  if (state.messageSearch.loading) return;

  const keyword = String(state.messageSearch.query || '').trim();
  if (!keyword) return;

  state.messageSearch.loading = true;
  state.messageSearch.error = '';

  try {
    const data = await searchRoomMessages(
      keyword,
      state.messageSearch.results.length,
      20,
    );
    const existing = new Set(
      state.messageSearch.results
        .filter((item) => item.id != null)
        .map((item) => String(item.id)),
    );
    const added = data.messages.filter(
      (item) => item.id == null || !existing.has(String(item.id)),
    );
    state.messageSearch.results = state.messageSearch.results.concat(added);
    state.messageSearch.total = data.total;
    state.messageSearch.from = state.messageSearch.results.length;
  } catch (error) {
    state.messageSearch.error = error.message || '搜索失败';
  } finally {
    state.messageSearch.loading = false;
  }
}

function formatHitContent(message) {
  if (!message) return '';
  if (Array.isArray(message.highlight) && message.highlight.length > 0) {
    return message.highlight[0];
  }
  return escapeHtml(message.content);
}
</script>
