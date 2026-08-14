<template>
  <div class="room-list">
    <div
      v-if="filteredRooms.length === 0"
      style="color:var(--muted);text-align:center;padding:20px;font-family:monospace;"
    >
      暂无群聊
    </div>
    <div
      v-for="room in filteredRooms"
      :key="room.id || room.room_id"
      class="room-item"
      :class="{ active: isActive(room) }"
      @click="$emit('select', room.id || room.room_id)"
      @contextmenu.prevent="openRoomMenu($event, room)"
    >
      <div class="room-item-name">{{ room.name || '未命名群聊' }}</div>
      <div class="room-item-id">#{{ room.id || room.room_id }}</div>
    </div>
  </div>

  <Teleport to="body">
    <div
      v-if="roomMenu.visible"
      class="context-menu"
      :style="{ left: roomMenu.x + 'px', top: roomMenu.y + 'px' }"
      @click.stop
    >
      <button type="button" class="danger" @click="disbandSelectedRoom">
        解散群聊
      </button>
    </div>
  </Teleport>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { state, filteredRooms } from '../store/index.js';
import { disbandRoom } from '../composables/useChat.js';

defineEmits(['select']);

const roomMenu = ref({ visible: false, x: 0, y: 0, room: null });

onMounted(() => {
  window.addEventListener('click', closeRoomMenu);
  window.addEventListener('scroll', closeRoomMenu, true);
  window.addEventListener('keydown', closeRoomMenuOnEscape);
});

onBeforeUnmount(() => {
  window.removeEventListener('click', closeRoomMenu);
  window.removeEventListener('scroll', closeRoomMenu, true);
  window.removeEventListener('keydown', closeRoomMenuOnEscape);
});

function isActive(room) {
  return (room.id || room.room_id) === state.currentRoomId;
}

function openRoomMenu(event, room) {
  roomMenu.value = {
    visible: true,
    x: Math.min(event.clientX, window.innerWidth - 150),
    y: Math.min(event.clientY, window.innerHeight - 70),
    room,
  };
}

function closeRoomMenu() {
  roomMenu.value.visible = false;
}

function closeRoomMenuOnEscape(event) {
  if (event.key === 'Escape') closeRoomMenu();
}

function disbandSelectedRoom() {
  const room = roomMenu.value.room;
  if (!room) return;
  closeRoomMenu();
  disbandRoom(room.id || room.room_id);
}
</script>
