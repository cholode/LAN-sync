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
      <div
        v-for="member in state.members"
        :key="member.user_id || member.id"
        class="member-item"
        @contextmenu.prevent="openMemberMenu($event, member)"
      >
        <span class="member-name">
          {{ member.username || '用户' }}
          <span v-if="member.is_creator || Number(member.role) === 3" class="member-badge owner">群主</span>
          <span v-else-if="Number(member.role) === 2" class="member-badge manager">管理</span>
        </span>
        <span class="member-id">#{{ member.user_id || member.id }}</span>
      </div>
    </div>
  </div>

  <Teleport to="body">
    <div
      v-if="memberMenu.visible"
      class="context-menu"
      :style="{ left: memberMenu.x + 'px', top: memberMenu.y + 'px' }"
      @click.stop
    >
      <button type="button" @click="kickSelectedMember">
        {{ isSelf(memberMenu.member) ? '退出群聊' : '移出群聊' }}
      </button>
    </div>
  </Teleport>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { state } from '../store/index.js';
import { removeMember } from '../composables/useChat.js';

const memberMenu = ref({ visible: false, x: 0, y: 0, member: null });

onMounted(() => {
  window.addEventListener('click', closeMemberMenu);
  window.addEventListener('scroll', closeMemberMenu, true);
  window.addEventListener('keydown', closeMemberMenuOnEscape);
});

onBeforeUnmount(() => {
  window.removeEventListener('click', closeMemberMenu);
  window.removeEventListener('scroll', closeMemberMenu, true);
  window.removeEventListener('keydown', closeMemberMenuOnEscape);
});

function openMemberMenu(event, member) {
  memberMenu.value = {
    visible: true,
    x: Math.min(event.clientX, window.innerWidth - 150),
    y: Math.min(event.clientY, window.innerHeight - 70),
    member,
  };
}

function closeMemberMenu() {
  memberMenu.value.visible = false;
}

function closeMemberMenuOnEscape(event) {
  if (event.key === 'Escape') closeMemberMenu();
}

function isSelf(member) {
  if (!member) return false;
  const selfId = state.user ? Number(state.user.id || state.user.user_id) : null;
  return selfId != null && Number(member.user_id || member.id) === selfId;
}

function kickSelectedMember() {
  const member = memberMenu.value.member;
  if (!member) return;
  closeMemberMenu();
  removeMember(Number(member.user_id || member.id));
}
</script>
