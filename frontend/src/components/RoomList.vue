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
    >
      <div class="room-item-name">{{ room.name || '未命名群聊' }}</div>
      <div class="room-item-id">#{{ room.id || room.room_id }}</div>
    </div>
  </div>
</template>

<script setup>
import { state, filteredRooms } from '../store/index.js';

defineEmits(['select']);

function isActive(room) {
  return (room.id || room.room_id) === state.currentRoomId;
}
</script>
