<template>
  <section class="main-view">
    <div class="chat-header">
      <div>
        <div id="current-room-title">{{ state.currentRoomName || '请选择群聊' }}</div>
        <div class="chat-header-sub" id="current-room-sub">
          {{ state.currentRoomId ? 'ID: ' + state.currentRoomId : '' }}
        </div>
      </div>
    </div>

    <div class="chat-wrap" ref="chatWrap" @scroll="handleScroll">
      <div class="chat-history-hint" id="history-hint">{{ state.historyHint }}</div>
      <div v-if="state.messages.length === 0" class="chat-placeholder">
        {{ state.currentRoomId ? '正在加载消息...' : '请选择左侧群聊开始聊天' }}
      </div>
      <div
        v-for="message in state.messages"
        :key="messageKey(message)"
        class="msg-row"
        :class="messageClass(message)"
      >
        <div v-if="message.kind !== 'sys' && message.kind !== 'err'" class="msg-meta">
          UID {{ message.sender_id }}<template v-if="formatMessageTime(message.created_at)"> · {{ formatMessageTime(message.created_at) }}</template>
        </div>
        <div class="msg-bubble" v-html="formatChatMessageHtml(message.content)"></div>
      </div>
    </div>

    <div class="input-area">
      <input
        v-model="draft"
        type="text"
        placeholder="输入消息，Enter 发送"
        :disabled="!state.currentRoomId"
        @keydown.enter.prevent="send"
      />
      <button type="button" class="primary" :disabled="!state.currentRoomId" @click="send">
        发送
      </button>
    </div>
  </section>
</template>

<script setup>
import { nextTick, onMounted, ref } from 'vue';
import { state } from '../store/index.js';
import {
  onLiveMessage,
  sendMessage,
  loadMoreHistory,
} from '../composables/useChat.js';
import {
  formatChatMessageHtml,
  formatMessageTime,
} from '../utils/message.js';

const chatWrap = ref(null);
const draft = ref('');

onMounted(() => {
  onLiveMessage(() => {
    nextTick(() => scrollToBottom());
  });
});

function messageKey(message) {
  return (
    message.id ||
    (message.created_at || '') + '-' + (message.sender_id || 'sys') + '-' + (message.content || '')
  );
}

function isOwnMessage(message) {
  const selfId = state.user ? Number(state.user.id || state.user.user_id) : null;
  return selfId != null && Number(message.sender_id) === selfId;
}

function messageClass(message) {
  if (message.kind === 'err') return 'err';
  if (message.kind === 'sys') return 'sys';
  return isOwnMessage(message) ? 'own' : 'other';
}

function send() {
  const content = draft.value;
  sendMessage(content);
  draft.value = '';
}

function scrollToBottom() {
  const el = chatWrap.value;
  if (!el) return;
  el.scrollTop = el.scrollHeight;
}

async function handleScroll() {
  const el = chatWrap.value;
  if (!el || el.scrollTop > 40 || !state.currentRoomId) return;

  const oldHeight = el.scrollHeight;
  const oldTop = el.scrollTop;
  const loaded = await loadMoreHistory();
  if (loaded) {
    await nextTick();
    el.scrollTop = oldTop + (el.scrollHeight - oldHeight);
  }
}
</script>
