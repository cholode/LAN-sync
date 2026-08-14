<template>
  <div id="chat-view" class="view-container">
    <header class="topbar">
      <div class="topbar-brand">LAN-IM 局域网即时通讯</div>
      <div class="topbar-user">
        <div class="topbar-user-meta">
          <div class="topbar-user-name">{{ state.user?.username || '用户' }}</div>
          <div class="topbar-user-id">#{{ state.user?.id || state.user?.user_id || '—' }}</div>
        </div>
        <button type="button" class="btn-ghost" @click="logout">退出登录</button>
      </div>
    </header>

    <div class="app-body">
      <aside class="sidebar">
        <div class="sidebar-head">群聊列表</div>
        <RoomList @select="selectRoom" />

        <div class="aside-block">
          <h3>群聊搜索</h3>
          <input v-model="state.roomFilter" type="search" placeholder="搜索群聊名称/ID" />
          <p class="hint">支持搜索已加入的群聊，可通过下方功能创建或加入新群聊</p>
          <div class="flex-row" style="margin-top:12px;">
            <input v-model="state.newRoomName" type="text" placeholder="群聊名称" />
            <button type="button" class="primary" @click="createRoom(state.newRoomName)">创建群聊</button>
          </div>
          <div class="flex-row">
            <input v-model="state.joinRoomId" type="number" placeholder="群聊ID" />
            <button type="button" class="primary" @click="joinRoom(state.joinRoomId)">加入群聊</button>
          </div>
        </div>

        <UploadPanel />
        <AgentPanel />
        <MemberPanel />
      </aside>

      <MessagePanel />
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue';
import { state } from '../store/index.js';
import { logout } from '../composables/useAuth.js';
import {
  initChat,
  selectRoom,
  createRoom,
  joinRoom,
} from '../composables/useChat.js';
import RoomList from './RoomList.vue';
import MessagePanel from './MessagePanel.vue';
import UploadPanel from './UploadPanel.vue';
import AgentPanel from './AgentPanel.vue';
import MemberPanel from './MemberPanel.vue';

onMounted(() => {
  initChat();
});
</script>
