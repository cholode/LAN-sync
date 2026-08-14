<template>
  <div class="aside-block aside-members-panel">
    <h3>群成员</h3>
    <p class="members-room-hint">{{ state.membersHint }}</p>
    <div class="member-list">
      <div
        v-if="state.members.length === 0"
        style="color:var(--muted);font-size:11px;padding:8px 12px;"
      >
        暂无成员
      </div>
      <div v-for="member in state.members" :key="member.user_id || member.id" class="member-item">
        <span class="member-name">
          {{ member.username || '用户' }}
          <span v-if="member.is_creator || Number(member.role) === 3" class="member-badge owner">群主</span>
          <span v-else-if="Number(member.role) === 2" class="member-badge manager">管理</span>
        </span>
        <span class="member-id">#{{ member.user_id || member.id }}</span>
      </div>
    </div>

    <div class="flex-row" style="margin-top:12px;">
      <input
        v-model="state.kickUserId"
        type="number"
        placeholder="用户ID（退出群聊输入自己ID）"
        style="margin-bottom:0;"
      />
      <button type="button" class="primary danger" @click="remove">
        暂无成员
      </button>
    </div>

    <div class="aside-block aside-disband-panel" style="border-top:1px solid var(--border-dim);">
      <h3>解散群聊</h3>
      <p class="hint">警告：仅群主可操作，解散后群聊将被删除，所有成员将被移出。</p>
      <button
        type="button"
        class="primary danger"
        :disabled="!state.currentRoomId"
        style="width:100%;"
        @click="disband"
      >
        暂无成员
      </button>
    </div>
  </div>
</template>

<script setup>
import { state } from '../store/index.js';
import { removeMember, disbandCurrentRoom } from '../composables/useChat.js';

function remove() {
  removeMember(Number(state.kickUserId));
}

function disband() {
  disbandCurrentRoom();
}
</script>
